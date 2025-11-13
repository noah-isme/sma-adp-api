package service

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"go.uber.org/zap"

	"github.com/noah-isme/sma-adp-api/internal/dto"
	"github.com/noah-isme/sma-adp-api/internal/models"
	appErrors "github.com/noah-isme/sma-adp-api/pkg/errors"
)

type archiveStore interface {
	Create(ctx context.Context, item *models.ArchiveItem) error
	GetByID(ctx context.Context, id string) (*models.ArchiveItem, error)
	List(ctx context.Context, filter models.ArchiveFilter) ([]models.ArchiveItem, error)
	SoftDelete(ctx context.Context, id string, deletedAt time.Time) error
}

type archiveAssignmentLister interface {
	ListByTeacher(ctx context.Context, teacherID string) ([]models.TeacherAssignmentDetail, error)
}

type archiveEnrollmentResolver interface {
	FindActiveByStudentAndTerm(ctx context.Context, studentID, termID string) ([]models.Enrollment, error)
	ListActiveByStudent(ctx context.Context, studentID string) ([]models.Enrollment, error)
}

type archiveFileStorage interface {
	SaveStream(filename string, r io.Reader) (string, error)
	Open(filename string) (*os.File, error)
	Delete(filename string) error
}

type archiveSignedURLSigner interface {
	Generate(id, relPath string) (string, time.Time, error)
	Parse(token string, allowExpired bool) (id, relPath string, expiresAt time.Time, err error)
}

// ArchiveUpload carries upload metadata and stream reader.
type ArchiveUpload struct {
	Filename string
	Size     int64
	MimeType string
	Content  io.ReadSeeker
}

// ArchiveDownload bundles file reader metadata for streaming.
type ArchiveDownload struct {
	File      *os.File
	Filename  string
	MimeType  string
	SizeBytes int64
	ExpiresAt time.Time
}

// ArchiveServiceConfig holds feature toggles and validation parameters.
type ArchiveServiceConfig struct {
	MaxFileSize  int64
	AllowedMIMEs []string
	APIPrefix    string
}

// ArchiveService manages archive metadata and storage IO.
type ArchiveService struct {
	repo        archiveStore
	assignments archiveAssignmentLister
	enrollments archiveEnrollmentResolver
	storage     archiveFileStorage
	signer      archiveSignedURLSigner
	audit       auditLogger
	logger      *zap.Logger
	cfg         ArchiveServiceConfig
	mimeSet     map[string]struct{}
}

// NewArchiveService constructs the service with defaults.
func NewArchiveService(repo archiveStore, assignments archiveAssignmentLister, enrollments archiveEnrollmentResolver, storage archiveFileStorage, signer archiveSignedURLSigner, audit auditLogger, logger *zap.Logger, cfg ArchiveServiceConfig) *ArchiveService {
	if logger == nil {
		logger = zap.NewNop()
	}
	if cfg.MaxFileSize <= 0 {
		cfg.MaxFileSize = 10 * 1024 * 1024
	}
	if len(cfg.AllowedMIMEs) == 0 {
		cfg.AllowedMIMEs = []string{
			"application/pdf",
			"application/vnd.openxmlformats-officedocument.wordprocessingml.document",
			"application/vnd.openxmlformats-officedocument.spreadsheetml.sheet",
			"application/zip",
		}
	}
	if cfg.APIPrefix == "" {
		cfg.APIPrefix = "/api/v1"
	}
	mimeSet := make(map[string]struct{}, len(cfg.AllowedMIMEs))
	for _, mt := range cfg.AllowedMIMEs {
		mimeSet[strings.ToLower(mt)] = struct{}{}
	}
	return &ArchiveService{
		repo:        repo,
		assignments: assignments,
		enrollments: enrollments,
		storage:     storage,
		signer:      signer,
		audit:       audit,
		logger:      logger,
		cfg:         cfg,
		mimeSet:     mimeSet,
	}
}

// Upload persists metadata and physical file for a new archive entry.
func (s *ArchiveService) Upload(ctx context.Context, meta dto.CreateArchiveRequest, upload ArchiveUpload, actor *models.JWTClaims) (*models.ArchiveItem, error) {
	if actor == nil {
		return nil, appErrors.ErrUnauthorized
	}
	if actor.Role != models.RoleAdmin && actor.Role != models.RoleSuperAdmin {
		return nil, appErrors.ErrForbidden
	}
	if err := s.validateUploadMeta(meta); err != nil {
		return nil, err
	}
	if upload.Content == nil || upload.Size <= 0 {
		return nil, appErrors.Clone(appErrors.ErrValidation, "file is required")
	}
	if upload.Size > s.cfg.MaxFileSize {
		return nil, appErrors.Clone(appErrors.ErrValidation, fmt.Sprintf("file exceeds %d bytes limit", s.cfg.MaxFileSize))
	}
	mimeType, err := s.detectMime(upload)
	if err != nil {
		return nil, err
	}
	if _, allowed := s.mimeSet[strings.ToLower(mimeType)]; !allowed {
		return nil, appErrors.Clone(appErrors.ErrValidation, "mime type not allowed")
	}
	filename := s.generateFilename(meta.Category, upload.Filename, mimeType)
	if _, err := upload.Content.Seek(0, io.SeekStart); err != nil {
		return nil, appErrors.Wrap(err, appErrors.ErrInternal.Code, appErrors.ErrInternal.Status, "failed to reset upload stream")
	}
	path, err := s.storage.SaveStream(filename, upload.Content)
	if err != nil {
		return nil, appErrors.Wrap(err, appErrors.ErrInternal.Code, appErrors.ErrInternal.Status, "failed to persist archive file")
	}
	item := &models.ArchiveItem{
		Title:        meta.Title,
		Category:     meta.Category,
		Scope:        models.ArchiveScope(strings.ToUpper(string(meta.Scope))),
		RefTermID:    normalizeRef(meta.RefTermID),
		RefClassID:   normalizeRef(meta.RefClassID),
		RefStudentID: normalizeRef(meta.RefStudentID),
		FilePath:     path,
		MimeType:     mimeType,
		SizeBytes:    upload.Size,
		UploadedBy:   actor.UserID,
	}
	if err := s.repo.Create(ctx, item); err != nil {
		_ = s.storage.Delete(path)
		return nil, appErrors.Wrap(err, appErrors.ErrInternal.Code, appErrors.ErrInternal.Status, "failed to create archive metadata")
	}
	s.emitAudit(ctx, &models.AuditLog{
		UserID:     &actor.UserID,
		Action:     models.AuditActionArchiveUpload,
		Resource:   "archive",
		ResourceID: &item.ID,
		NewValues:  []byte(fmt.Sprintf(`{"title":"%s","category":"%s"}`, item.Title, item.Category)),
	})
	return item, nil
}

// List returns archives permitted for the actor respecting scope filters.
func (s *ArchiveService) List(ctx context.Context, filter dto.ArchiveFilter, actor *models.JWTClaims) ([]models.ArchiveItem, error) {
	if actor == nil {
		return nil, appErrors.ErrUnauthorized
	}
	repoFilter := models.ArchiveFilter{
		Scope:    filter.Scope,
		Category: filter.Category,
		TermID:   filter.TermID,
		ClassID:  filter.ClassID,
	}
	items, err := s.repo.List(ctx, repoFilter)
	if err != nil {
		return nil, appErrors.Wrap(err, appErrors.ErrInternal.Code, appErrors.ErrInternal.Status, "failed to list archives")
	}
	if actor.Role == models.RoleSuperAdmin || actor.Role == models.RoleAdmin {
		return items, nil
	}
	scope, err := s.teacherScope(ctx, actor.UserID)
	if err != nil {
		return nil, err
	}
	filtered := make([]models.ArchiveItem, 0, len(items))
	for _, item := range items {
		if s.canTeacherAccess(ctx, scope, &item) {
			filtered = append(filtered, item)
		}
	}
	return filtered, nil
}

// Get returns archive metadata enforcing permissions.
func (s *ArchiveService) Get(ctx context.Context, id string, actor *models.JWTClaims) (*models.ArchiveItem, error) {
	item, err := s.repo.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, appErrors.ErrNotFound
		}
		return nil, appErrors.Wrap(err, appErrors.ErrInternal.Code, appErrors.ErrInternal.Status, "failed to load archive")
	}
	if item.DeletedAt != nil {
		return nil, appErrors.ErrNotFound
	}
	if err := s.ensureAccess(ctx, item, actor); err != nil {
		return nil, err
	}
	return item, nil
}

// GetDownloadURL generates a signed URL for downloading the file.
func (s *ArchiveService) GetDownloadURL(ctx context.Context, id string, actor *models.JWTClaims) (string, error) {
	if s.signer == nil {
		return "", appErrors.Clone(appErrors.ErrInternal, "download signer unavailable")
	}
	item, err := s.Get(ctx, id, actor)
	if err != nil {
		return "", err
	}
	token, _, err := s.signer.Generate(item.ID, item.FilePath)
	if err != nil {
		return "", appErrors.Wrap(err, appErrors.ErrInternal.Code, appErrors.ErrInternal.Status, "failed to generate download token")
	}
	base := strings.TrimRight(s.cfg.APIPrefix, "/")
	url := fmt.Sprintf("%s/archives/%s/download?token=%s", base, item.ID, token)
	return url, nil
}

// Download validates token and opens the archive file.
func (s *ArchiveService) Download(ctx context.Context, id, token string, actor *models.JWTClaims) (*ArchiveDownload, error) {
	if s.signer == nil {
		return nil, appErrors.Clone(appErrors.ErrInternal, "download signer unavailable")
	}
	item, err := s.Get(ctx, id, actor)
	if err != nil {
		return nil, err
	}
	archiveID, relPath, expiresAt, err := s.signer.Parse(token, false)
	if err != nil {
		return nil, appErrors.Clone(appErrors.ErrForbidden, "invalid or expired token")
	}
	if archiveID != item.ID || relPath != item.FilePath {
		return nil, appErrors.Clone(appErrors.ErrForbidden, "token mismatch")
	}
	file, err := s.storage.Open(relPath)
	if err != nil {
		return nil, appErrors.Wrap(err, appErrors.ErrInternal.Code, appErrors.ErrInternal.Status, "failed to open archive file")
	}
	info, err := file.Stat()
	if err != nil {
		file.Close() //nolint:errcheck
		return nil, appErrors.Wrap(err, appErrors.ErrInternal.Code, appErrors.ErrInternal.Status, "failed to read archive metadata")
	}
	return &ArchiveDownload{
		File:      file,
		Filename:  filepath.Base(relPath),
		MimeType:  item.MimeType,
		SizeBytes: info.Size(),
		ExpiresAt: expiresAt,
	}, nil
}

// Delete marks an archive as deleted (soft delete).
func (s *ArchiveService) Delete(ctx context.Context, id string, actor *models.JWTClaims) error {
	if actor == nil {
		return appErrors.ErrUnauthorized
	}
	if actor.Role != models.RoleSuperAdmin {
		return appErrors.ErrForbidden
	}
	if err := s.repo.SoftDelete(ctx, id, time.Now().UTC()); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return appErrors.ErrNotFound
		}
		return appErrors.Wrap(err, appErrors.ErrInternal.Code, appErrors.ErrInternal.Status, "failed to delete archive")
	}
	s.emitAudit(ctx, &models.AuditLog{
		UserID:     &actor.UserID,
		Action:     models.AuditActionArchiveDelete,
		Resource:   "archive",
		ResourceID: &id,
	})
	return nil
}

func (s *ArchiveService) ensureAccess(ctx context.Context, item *models.ArchiveItem, actor *models.JWTClaims) error {
	if actor == nil {
		return appErrors.ErrUnauthorized
	}
	switch actor.Role {
	case models.RoleSuperAdmin, models.RoleAdmin:
		return nil
	case models.RoleTeacher:
		scope, err := s.teacherScope(ctx, actor.UserID)
		if err != nil {
			return err
		}
		if s.canTeacherAccess(ctx, scope, item) {
			return nil
		}
		return appErrors.ErrForbidden
	default:
		return appErrors.ErrForbidden
	}
}

type teacherScope struct {
	ClassIDs map[string]struct{}
	TermIDs  map[string]struct{}
}

func (s *ArchiveService) teacherScope(ctx context.Context, teacherID string) (*teacherScope, error) {
	result := &teacherScope{
		ClassIDs: map[string]struct{}{},
		TermIDs:  map[string]struct{}{},
	}
	if s.assignments == nil {
		return result, nil
	}
	assignments, err := s.assignments.ListByTeacher(ctx, teacherID)
	if err != nil {
		return nil, appErrors.Wrap(err, appErrors.ErrInternal.Code, appErrors.ErrInternal.Status, "failed to load teacher assignments")
	}
	for _, assignment := range assignments {
		result.ClassIDs[assignment.ClassID] = struct{}{}
		result.TermIDs[assignment.TermID] = struct{}{}
	}
	return result, nil
}

func (s *ArchiveService) canTeacherAccess(ctx context.Context, scope *teacherScope, item *models.ArchiveItem) bool {
	if scope == nil {
		return false
	}
	switch item.Scope {
	case models.ArchiveScopeGlobal:
		return true
	case models.ArchiveScopeTerm:
		if item.RefTermID == nil {
			return false
		}
		_, ok := scope.TermIDs[*item.RefTermID]
		return ok
	case models.ArchiveScopeClass:
		if item.RefClassID == nil {
			return false
		}
		_, ok := scope.ClassIDs[*item.RefClassID]
		return ok
	case models.ArchiveScopeStudent:
		if item.RefClassID != nil {
			if _, ok := scope.ClassIDs[*item.RefClassID]; ok {
				return true
			}
		}
		if item.RefStudentID == nil {
			return false
		}
		enrollments, err := s.resolveStudentEnrollments(ctx, *item.RefStudentID, item.RefTermID)
		if err != nil {
			s.logger.Warn("failed to resolve student enrollments", zap.Error(err), zap.String("student_id", *item.RefStudentID))
			return false
		}
		for _, enrollment := range enrollments {
			if _, ok := scope.ClassIDs[enrollment.ClassID]; ok {
				return true
			}
		}
		return false
	default:
		return false
	}
}

func (s *ArchiveService) validateUploadMeta(meta dto.CreateArchiveRequest) error {
	if strings.TrimSpace(meta.Title) == "" {
		return appErrors.Clone(appErrors.ErrValidation, "title is required")
	}
	if strings.TrimSpace(meta.Category) == "" {
		return appErrors.Clone(appErrors.ErrValidation, "category is required")
	}
	scope := strings.ToUpper(string(meta.Scope))
	switch models.ArchiveScope(scope) {
	case models.ArchiveScopeGlobal:
	case models.ArchiveScopeTerm:
		if meta.RefTermID == nil || *meta.RefTermID == "" {
			return appErrors.Clone(appErrors.ErrValidation, "refTermId required for TERM scope")
		}
	case models.ArchiveScopeClass:
		if meta.RefClassID == nil || *meta.RefClassID == "" {
			return appErrors.Clone(appErrors.ErrValidation, "refClassId required for CLASS scope")
		}
	case models.ArchiveScopeStudent:
		if meta.RefStudentID == nil || *meta.RefStudentID == "" || meta.RefClassID == nil || *meta.RefClassID == "" {
			return appErrors.Clone(appErrors.ErrValidation, "refStudentId and refClassId required for STUDENT scope")
		}
	default:
		return appErrors.Clone(appErrors.ErrValidation, "invalid scope")
	}
	return nil
}

func (s *ArchiveService) detectMime(upload ArchiveUpload) (string, error) {
	if upload.Content == nil {
		return "", appErrors.Clone(appErrors.ErrValidation, "file reader missing")
	}
	if upload.MimeType != "" {
		return upload.MimeType, nil
	}
	header := make([]byte, 512)
	n, err := upload.Content.Read(header)
	if err != nil && err != io.EOF {
		return "", appErrors.Wrap(err, appErrors.ErrInternal.Code, appErrors.ErrInternal.Status, "failed to inspect file")
	}
	if _, err := upload.Content.Seek(0, io.SeekStart); err != nil {
		return "", appErrors.Wrap(err, appErrors.ErrInternal.Code, appErrors.ErrInternal.Status, "failed to reset upload stream")
	}
	if n == 0 {
		return "", appErrors.Clone(appErrors.ErrValidation, "empty file")
	}
	return http.DetectContentType(header[:n]), nil
}

func (s *ArchiveService) resolveStudentEnrollments(ctx context.Context, studentID string, termID *string) ([]models.Enrollment, error) {
	if s.enrollments == nil {
		return nil, nil
	}
	if termID != nil && strings.TrimSpace(*termID) != "" {
		return s.enrollments.FindActiveByStudentAndTerm(ctx, studentID, *termID)
	}
	return s.enrollments.ListActiveByStudent(ctx, studentID)
}

func (s *ArchiveService) generateFilename(category, original, mimeType string) string {
	category = sanitize(category)
	ext := strings.ToLower(filepath.Ext(original))
	if ext == "" {
		ext = mimeExtension(mimeType)
	}
	if ext == "" {
		ext = ".bin"
	}
	return fmt.Sprintf("archive_%s_%d_%s%s", category, time.Now().Unix(), randomSuffix(), ext)
}

func sanitize(raw string) string {
	raw = strings.ToLower(raw)
	var b strings.Builder
	for _, r := range raw {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
		} else {
			b.WriteRune('_')
		}
	}
	return strings.Trim(b.String(), "_")
}

func mimeExtension(mime string) string {
	switch strings.ToLower(mime) {
	case "application/pdf":
		return ".pdf"
	case "application/vnd.openxmlformats-officedocument.wordprocessingml.document":
		return ".docx"
	case "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet":
		return ".xlsx"
	case "application/zip":
		return ".zip"
	default:
		return ""
	}
}

func randomSuffix() string {
	buf := make([]byte, 4)
	if _, err := rand.Read(buf); err != nil {
		return fmt.Sprintf("%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(buf)
}

func normalizeRef(value *string) *string {
	if value == nil {
		return nil
	}
	trimmed := strings.TrimSpace(*value)
	if trimmed == "" {
		return nil
	}
	result := trimmed
	return &result
}

func (s *ArchiveService) emitAudit(ctx context.Context, log *models.AuditLog) {
	if s.audit == nil || log == nil {
		return
	}
	log.IPAddress = "system"
	log.UserAgent = "archive-service"
	if err := s.audit.CreateAuditLog(ctx, log); err != nil {
		s.logger.Warn("failed to create archive audit", zap.Error(err))
	}
}
