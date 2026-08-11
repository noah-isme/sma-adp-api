package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"

	internalmiddleware "github.com/noah-isme/sma-adp-api/internal/middleware"
	"github.com/noah-isme/sma-adp-api/internal/models"
	"github.com/noah-isme/sma-adp-api/internal/service"
	appErrors "github.com/noah-isme/sma-adp-api/pkg/errors"
)

type mockGradeConfigRepo struct {
	configs []models.GradeConfig
	getRes  *models.GradeConfig
	listErr error
}

func (m *mockGradeConfigRepo) List(ctx context.Context, filter models.FinalGradeFilter) ([]models.GradeConfig, error) {
	if m.listErr != nil {
		return nil, m.listErr
	}
	return m.configs, nil
}

func (m *mockGradeConfigRepo) FindByID(ctx context.Context, id string) (*models.GradeConfig, error) {
	if id == "notfound" {
		return nil, appErrors.ErrNotFound
	}
	if m.getRes != nil {
		return m.getRes, nil
	}
	return &models.GradeConfig{ID: id, ClassID: "class-1", SubjectID: "math", TermID: "term-1"}, nil
}

func (m *mockGradeConfigRepo) FindByScope(ctx context.Context, classID, subjectID, termID string) (*models.GradeConfig, error) {
	return &models.GradeConfig{ID: "cfg-1", ClassID: classID, SubjectID: subjectID, TermID: termID}, nil
}

func (m *mockGradeConfigRepo) Exists(ctx context.Context, classID, subjectID, termID, excludeID string) (bool, error) {
	return false, nil
}

func (m *mockGradeConfigRepo) Create(ctx context.Context, config *models.GradeConfig) error {
	config.ID = "cfg-100"
	config.CreatedAt = time.Now()
	return nil
}

func (m *mockGradeConfigRepo) Update(ctx context.Context, config *models.GradeConfig) error {
	return nil
}

func (m *mockGradeConfigRepo) Finalize(ctx context.Context, id string, finalized bool) error {
	if id == "notfound" {
		return appErrors.ErrNotFound
	}
	return nil
}

type mockGradeComponentReader struct{}

func (m *mockGradeComponentReader) FindByID(ctx context.Context, id string) (*models.GradeComponent, error) {
	return &models.GradeComponent{ID: id, Code: "HW", Name: "Homework"}, nil
}

func setupGradeConfigRouter(h *GradeConfigHandler) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		if role := c.GetHeader("X-Test-Role"); role != "" {
			c.Set(internalmiddleware.ContextUserKey, &models.JWTClaims{
				UserID: "user-1",
				Role:   models.UserRole(role),
			})
		}
		c.Next()
	})

	api := r.Group("/api/v1/grade-configs")
	api.Use(internalmiddleware.RBAC(string(models.RoleAdmin), string(models.RoleTeacher)))
	{
		api.GET("", h.List)
		api.GET("/:id", h.Get)
		api.POST("", h.Create)
		api.PUT("/:id", h.Update)
		api.POST("/:id/finalize", h.Finalize)
	}
	return r
}

func TestGradeConfigHandlerListSuccess(t *testing.T) {
	repo := &mockGradeConfigRepo{
		configs: []models.GradeConfig{{ID: "cfg-1", ClassID: "class-1"}},
	}
	svc := service.NewGradeConfigService(repo, &mockGradeComponentReader{}, validator.New(), zap.NewNop())
	h := NewGradeConfigHandler(svc)
	router := setupGradeConfigRouter(h)

	req, _ := http.NewRequest(http.MethodGet, "/api/v1/grade-configs?classId=class-1", nil)
	req.Header.Set("X-Test-Role", string(models.RoleAdmin))

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestGradeConfigHandlerGetSuccess(t *testing.T) {
	repo := &mockGradeConfigRepo{}
	svc := service.NewGradeConfigService(repo, &mockGradeComponentReader{}, validator.New(), zap.NewNop())
	h := NewGradeConfigHandler(svc)
	router := setupGradeConfigRouter(h)

	req, _ := http.NewRequest(http.MethodGet, "/api/v1/grade-configs/cfg-1", nil)
	req.Header.Set("X-Test-Role", string(models.RoleTeacher))

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestGradeConfigHandlerCreateSuccess(t *testing.T) {
	repo := &mockGradeConfigRepo{}
	svc := service.NewGradeConfigService(repo, &mockGradeComponentReader{}, validator.New(), zap.NewNop())
	h := NewGradeConfigHandler(svc)
	router := setupGradeConfigRouter(h)

	body, _ := json.Marshal(map[string]interface{}{
		"class_id":           "class-1",
		"subject_id":         "math",
		"term_id":            "term-1",
		"calculation_scheme": "WEIGHTED",
		"components": []map[string]interface{}{
			{"component_id": "cmp-1", "weight": 100},
		},
	})
	req, _ := http.NewRequest(http.MethodPost, "/api/v1/grade-configs", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Test-Role", string(models.RoleAdmin))

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)
}

func TestGradeConfigHandlerUpdateSuccess(t *testing.T) {
	repo := &mockGradeConfigRepo{}
	svc := service.NewGradeConfigService(repo, &mockGradeComponentReader{}, validator.New(), zap.NewNop())
	h := NewGradeConfigHandler(svc)
	router := setupGradeConfigRouter(h)

	body, _ := json.Marshal(map[string]interface{}{
		"calculation_scheme": "WEIGHTED",
		"components": []map[string]interface{}{
			{"component_id": "cmp-1", "weight": 100},
		},
	})
	req, _ := http.NewRequest(http.MethodPut, "/api/v1/grade-configs/cfg-1", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Test-Role", string(models.RoleTeacher))

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestGradeConfigHandlerFinalizeSuccess(t *testing.T) {
	repo := &mockGradeConfigRepo{}
	svc := service.NewGradeConfigService(repo, &mockGradeComponentReader{}, validator.New(), zap.NewNop())
	h := NewGradeConfigHandler(svc)
	router := setupGradeConfigRouter(h)

	req, _ := http.NewRequest(http.MethodPost, "/api/v1/grade-configs/cfg-1/finalize", nil)
	req.Header.Set("X-Test-Role", string(models.RoleAdmin))

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestGradeConfigHandlerUnauthorized(t *testing.T) {
	repo := &mockGradeConfigRepo{}
	svc := service.NewGradeConfigService(repo, &mockGradeComponentReader{}, validator.New(), zap.NewNop())
	h := NewGradeConfigHandler(svc)
	router := setupGradeConfigRouter(h)

	req, _ := http.NewRequest(http.MethodGet, "/api/v1/grade-configs", nil)
	// No X-Test-Role

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestGradeConfigHandlerForbidden(t *testing.T) {
	repo := &mockGradeConfigRepo{}
	svc := service.NewGradeConfigService(repo, &mockGradeComponentReader{}, validator.New(), zap.NewNop())
	h := NewGradeConfigHandler(svc)
	router := setupGradeConfigRouter(h)

	req, _ := http.NewRequest(http.MethodGet, "/api/v1/grade-configs", nil)
	req.Header.Set("X-Test-Role", string(models.RoleStudent)) // Not authorized

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusForbidden, w.Code)
}

func TestGradeConfigHandlerCreateInvalidJSON(t *testing.T) {
	repo := &mockGradeConfigRepo{}
	svc := service.NewGradeConfigService(repo, &mockGradeComponentReader{}, validator.New(), zap.NewNop())
	h := NewGradeConfigHandler(svc)
	router := setupGradeConfigRouter(h)

	req, _ := http.NewRequest(http.MethodPost, "/api/v1/grade-configs", bytes.NewBufferString(`{invalid}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Test-Role", string(models.RoleAdmin))

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestGradeConfigHandlerInternalError(t *testing.T) {
	repo := &mockGradeConfigRepo{
		listErr: appErrors.ErrInternal,
	}
	svc := service.NewGradeConfigService(repo, &mockGradeComponentReader{}, validator.New(), zap.NewNop())
	h := NewGradeConfigHandler(svc)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	req, _ := http.NewRequest(http.MethodGet, "/grade-configs", nil)
	c.Request = req

	h.List(c)
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}
