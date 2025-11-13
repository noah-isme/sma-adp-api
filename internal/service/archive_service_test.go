package service

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/noah-isme/sma-adp-api/internal/dto"
	"github.com/noah-isme/sma-adp-api/internal/models"
	"github.com/noah-isme/sma-adp-api/pkg/storage"
)

type archiveRepoStub struct {
	items  map[string]*models.ArchiveItem
	filter models.ArchiveFilter
}

func newArchiveRepoStub() *archiveRepoStub {
	return &archiveRepoStub{items: make(map[string]*models.ArchiveItem)}
}

func (r *archiveRepoStub) Create(ctx context.Context, item *models.ArchiveItem) error {
	if item.ID == "" {
		item.ID = fmt.Sprintf("arch-%d", len(r.items)+1)
	}
	if item.UploadedAt.IsZero() {
		item.UploadedAt = time.Now()
	}
	r.items[item.ID] = item
	return nil
}

func (r *archiveRepoStub) GetByID(ctx context.Context, id string) (*models.ArchiveItem, error) {
	if item, ok := r.items[id]; ok {
		copy := *item
		return &copy, nil
	}
	return nil, fmt.Errorf("not found")
}

func (r *archiveRepoStub) List(ctx context.Context, filter models.ArchiveFilter) ([]models.ArchiveItem, error) {
	r.filter = filter
	result := make([]models.ArchiveItem, 0, len(r.items))
	for _, item := range r.items {
		result = append(result, *item)
	}
	return result, nil
}

func (r *archiveRepoStub) SoftDelete(ctx context.Context, id string, deletedAt time.Time) error {
	if item, ok := r.items[id]; ok {
		item.DeletedAt = &deletedAt
		return nil
	}
	return fmt.Errorf("not found")
}

type storageStub struct {
	saved map[string][]byte
	files map[string]string
}

func newStorageStub() *storageStub {
	return &storageStub{
		saved: make(map[string][]byte),
		files: make(map[string]string),
	}
}

func (s *storageStub) SaveStream(filename string, r io.Reader) (string, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return "", err
	}
	s.saved[filename] = data
	path := filepath.Join(os.TempDir(), "archive-test-"+filepath.Base(filename))
	if err := os.WriteFile(path, data, 0o600); err != nil {
		return "", err
	}
	s.files[filename] = path
	return filename, nil
}

func (s *storageStub) Open(filename string) (*os.File, error) {
	path, ok := s.files[filename]
	if !ok {
		return nil, fmt.Errorf("not found")
	}
	return os.Open(path)
}

func (s *storageStub) Delete(filename string) error {
	if path, ok := s.files[filename]; ok {
		_ = os.Remove(path)
		delete(s.files, filename)
	}
	delete(s.saved, filename)
	return nil
}

type archiveAssignmentStub struct {
	assignments []models.TeacherAssignmentDetail
}

func (a archiveAssignmentStub) ListByTeacher(ctx context.Context, teacherID string) ([]models.TeacherAssignmentDetail, error) {
	return a.assignments, nil
}

type enrollmentAccessStub struct {
	byStudent map[string][]models.Enrollment
	byTerm    map[string][]models.Enrollment
}

func (e enrollmentAccessStub) FindActiveByStudentAndTerm(ctx context.Context, studentID, termID string) ([]models.Enrollment, error) {
	if e.byTerm == nil {
		return nil, nil
	}
	key := studentID + "|" + termID
	return e.byTerm[key], nil
}

func (e enrollmentAccessStub) ListActiveByStudent(ctx context.Context, studentID string) ([]models.Enrollment, error) {
	if e.byStudent == nil {
		return nil, nil
	}
	return e.byStudent[studentID], nil
}

func TestArchiveServiceUpload(t *testing.T) {
	repo := newArchiveRepoStub()
	store := newStorageStub()
	audit := &auditStub{}
	svc := NewArchiveService(
		repo,
		nil,
		nil,
		store,
		nil,
		audit,
		nil,
		ArchiveServiceConfig{
			MaxFileSize:  1024 * 1024,
			AllowedMIMEs: []string{"application/pdf"},
			APIPrefix:    "/api/v1",
		},
	)

	meta := dto.CreateArchiveRequest{
		Title:    "Policy",
		Category: "OPS",
		Scope:    models.ArchiveScopeGlobal,
	}
	content := bytes.NewReader([]byte("hello world"))
	item, err := svc.Upload(context.Background(), meta, ArchiveUpload{
		Filename: "policy.pdf",
		Size:     int64(content.Len()),
		MimeType: "application/pdf",
		Content:  content,
	}, &models.JWTClaims{UserID: "admin-1", Role: models.RoleAdmin})
	require.NoError(t, err)
	t.Cleanup(func() { _ = store.Delete(item.FilePath) })
	require.NotEmpty(t, item.ID)
	require.Contains(t, store.saved, item.FilePath)
	require.Len(t, audit.logs, 1)
}

func TestArchiveServiceListTeacherFilters(t *testing.T) {
	repo := newArchiveRepoStub()
	repo.items["arch-1"] = &models.ArchiveItem{ID: "arch-1", Scope: models.ArchiveScopeGlobal}
	classID := "class-1"
	repo.items["arch-2"] = &models.ArchiveItem{ID: "arch-2", Scope: models.ArchiveScopeClass, RefClassID: &classID}
	repo.items["arch-3"] = &models.ArchiveItem{ID: "arch-3", Scope: models.ArchiveScopeClass}

	assignments := archiveAssignmentStub{assignments: []models.TeacherAssignmentDetail{
		{TeacherAssignment: models.TeacherAssignment{ClassID: classID, TermID: "term-1"}},
	}}
	svc := NewArchiveService(
		repo,
		assignments,
		nil,
		newStorageStub(),
		nil,
		&auditStub{},
		nil,
		ArchiveServiceConfig{APIPrefix: "/api/v1"},
	)

	items, err := svc.List(context.Background(), dto.ArchiveFilter{}, &models.JWTClaims{UserID: "teacher-1", Role: models.RoleTeacher})
	require.NoError(t, err)
	require.Len(t, items, 2)
}

func TestArchiveServiceStudentScopeEnrollmentFallback(t *testing.T) {
	repo := newArchiveRepoStub()
	studentID := "stu-1"
	classID := "class-x"
	termID := "term-1"
	repo.items["arch-s"] = &models.ArchiveItem{
		ID:           "arch-s",
		Scope:        models.ArchiveScopeStudent,
		RefStudentID: &studentID,
		RefTermID:    &termID,
	}
	assignments := archiveAssignmentStub{assignments: []models.TeacherAssignmentDetail{
		{TeacherAssignment: models.TeacherAssignment{ClassID: classID, TermID: termID}},
	}}
	enrollments := &enrollmentAccessStub{
		byTerm: map[string][]models.Enrollment{
			studentID + "|" + termID: {{ClassID: classID}},
		},
		byStudent: map[string][]models.Enrollment{
			studentID: {{ClassID: classID}},
		},
	}
	svc := NewArchiveService(
		repo,
		assignments,
		enrollments,
		newStorageStub(),
		nil,
		&auditStub{},
		nil,
		ArchiveServiceConfig{APIPrefix: "/api/v1"},
	)

	items, err := svc.List(context.Background(), dto.ArchiveFilter{}, &models.JWTClaims{UserID: "teacher-1", Role: models.RoleTeacher})
	require.NoError(t, err)
	require.Len(t, items, 1)
}

func TestArchiveServiceDownload(t *testing.T) {
	repo := newArchiveRepoStub()
	store := newStorageStub()
	audit := &auditStub{}
	signer := storage.NewSignedURLSigner("secret", time.Minute)
	item := &models.ArchiveItem{
		ID:         "arch-1",
		Title:      "Policy",
		Category:   "OPS",
		Scope:      models.ArchiveScopeGlobal,
		FilePath:   "archive/policy.pdf",
		MimeType:   "application/pdf",
		SizeBytes:  5,
		UploadedBy: "admin",
	}
	repo.items[item.ID] = item
	_, err := store.SaveStream(item.FilePath, bytes.NewReader([]byte("hello")))
	require.NoError(t, err)
	t.Cleanup(func() { _ = store.Delete(item.FilePath) })

	svc := NewArchiveService(
		repo,
		nil,
		nil,
		store,
		signer,
		audit,
		nil,
		ArchiveServiceConfig{APIPrefix: "/api/v1"},
	)

	url, err := svc.GetDownloadURL(context.Background(), item.ID, &models.JWTClaims{UserID: "admin", Role: models.RoleAdmin})
	require.NoError(t, err)
	require.Contains(t, url, "token=")
	parts := strings.SplitN(url, "token=", 2)
	require.Len(t, parts, 2)
	token := parts[1]

	download, err := svc.Download(context.Background(), item.ID, token, &models.JWTClaims{UserID: "admin", Role: models.RoleAdmin})
	require.NoError(t, err)
	require.Equal(t, "application/pdf", download.MimeType)
	download.File.Close() //nolint:errcheck
}
