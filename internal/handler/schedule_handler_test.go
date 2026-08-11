package handler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/noah-isme/sma-adp-api/internal/models"
	"github.com/noah-isme/sma-adp-api/internal/service"
)

type mockScheduleRepo struct {
	schedules []models.Schedule
}

func (m *mockScheduleRepo) List(ctx context.Context, filter models.ScheduleFilter) ([]models.Schedule, int, error) {
	var result []models.Schedule
	for _, s := range m.schedules {
		if filter.ClassID != "" && s.ClassID != filter.ClassID {
			continue
		}
		if filter.TermID != "" && s.TermID != filter.TermID {
			continue
		}
		result = append(result, s)
	}
	return result, len(result), nil
}

func (m *mockScheduleRepo) ListByClass(ctx context.Context, classID string) ([]models.Schedule, error) {
	return m.schedules, nil
}

func (m *mockScheduleRepo) ListByTeacher(ctx context.Context, teacherID string) ([]models.Schedule, error) {
	return m.schedules, nil
}

func (m *mockScheduleRepo) FindByID(ctx context.Context, id string) (*models.Schedule, error) {
	return nil, nil
}

func (m *mockScheduleRepo) FindConflicts(ctx context.Context, termID, dayOfWeek, timeSlot string) ([]models.Schedule, error) {
	return nil, nil
}

func (m *mockScheduleRepo) Create(ctx context.Context, schedule *models.Schedule) error {
	return nil
}

func (m *mockScheduleRepo) BulkCreate(ctx context.Context, schedules []models.Schedule) error {
	return nil
}

func (m *mockScheduleRepo) Update(ctx context.Context, schedule *models.Schedule) error {
	return nil
}

func (m *mockScheduleRepo) Delete(ctx context.Context, id string) error {
	return nil
}

func TestExportPDF_Success(t *testing.T) {
	gin.SetMode(gin.TestMode)

	mockRepo := &mockScheduleRepo{
		schedules: []models.Schedule{
			{
				ID:        "sched-1",
				TermID:    "term-2025",
				ClassID:   "class-10a",
				SubjectID: "Matematika",
				TeacherID: "Budi Santoso",
				DayOfWeek: "SENIN",
				TimeSlot:  "1",
				Room:      "R-101",
			},
			{
				ID:        "sched-2",
				TermID:    "term-2025",
				ClassID:   "class-10a",
				SubjectID: "Fisika",
				TeacherID: "Siti Rahma",
				DayOfWeek: "SELASA",
				TimeSlot:  "2",
				Room:      "Lab Fisika",
			},
		},
	}

	svc := service.NewScheduleService(mockRepo, nil, nil)
	h := NewScheduleHandler(svc)

	r := gin.New()
	r.GET("/api/v1/schedules/export/pdf", h.ExportPDF)

	w := httptest.NewRecorder()
	req, err := http.NewRequest(http.MethodGet, "/api/v1/schedules/export/pdf?class_id=class-10a&term_id=term-2025", nil)
	require.NoError(t, err)

	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "application/pdf", w.Header().Get("Content-Type"))
	assert.Equal(t, "attachment; filename=\"Jadwal_Kelas_class-10a_term-2025.pdf\"", w.Header().Get("Content-Disposition"))
	assert.True(t, len(w.Body.Bytes()) > 0)
	assert.True(t, bytesHasPrefix(w.Body.Bytes(), []byte("%PDF-")))
}

func TestExportPDF_CamelCaseParams(t *testing.T) {
	gin.SetMode(gin.TestMode)

	mockRepo := &mockScheduleRepo{}
	svc := service.NewScheduleService(mockRepo, nil, nil)
	h := NewScheduleHandler(svc)

	r := gin.New()
	r.GET("/api/v1/schedules/export/pdf", h.ExportPDF)

	w := httptest.NewRecorder()
	req, err := http.NewRequest(http.MethodGet, "/api/v1/schedules/export/pdf?classId=class-10a&termId=term-2025", nil)
	require.NoError(t, err)

	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "application/pdf", w.Header().Get("Content-Type"))
	assert.Equal(t, "attachment; filename=\"Jadwal_Kelas_class-10a_term-2025.pdf\"", w.Header().Get("Content-Disposition"))
}

func TestExportPDF_MissingQueryParams(t *testing.T) {
	gin.SetMode(gin.TestMode)

	mockRepo := &mockScheduleRepo{}
	svc := service.NewScheduleService(mockRepo, nil, nil)
	h := NewScheduleHandler(svc)

	r := gin.New()
	r.GET("/api/v1/schedules/export/pdf", h.ExportPDF)

	// Missing term_id
	w1 := httptest.NewRecorder()
	req1, _ := http.NewRequest(http.MethodGet, "/api/v1/schedules/export/pdf?class_id=class-10a", nil)
	r.ServeHTTP(w1, req1)
	assert.Equal(t, http.StatusBadRequest, w1.Code)

	// Missing class_id
	w2 := httptest.NewRecorder()
	req2, _ := http.NewRequest(http.MethodGet, "/api/v1/schedules/export/pdf?term_id=term-2025", nil)
	r.ServeHTTP(w2, req2)
	assert.Equal(t, http.StatusBadRequest, w2.Code)
}

func bytesHasPrefix(data, prefix []byte) bool {
	if len(data) < len(prefix) {
		return false
	}
	for i := range prefix {
		if data[i] != prefix[i] {
			return false
		}
	}
	return true
}
