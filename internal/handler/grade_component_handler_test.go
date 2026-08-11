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

type mockGradeComponentRepo struct {
	components []models.GradeComponent
	getRes     *models.GradeComponent
	listErr    error
}

func (m *mockGradeComponentRepo) List(ctx context.Context, search string) ([]models.GradeComponent, error) {
	if m.listErr != nil {
		return nil, m.listErr
	}
	return m.components, nil
}

func (m *mockGradeComponentRepo) ExistsByCode(ctx context.Context, code string, excludeID string) (bool, error) {
	return false, nil
}

func (m *mockGradeComponentRepo) Create(ctx context.Context, component *models.GradeComponent) error {
	component.ID = "cmp-100"
	component.CreatedAt = time.Now()
	return nil
}

func (m *mockGradeComponentRepo) FindByID(ctx context.Context, id string) (*models.GradeComponent, error) {
	if id == "notfound" {
		return nil, appErrors.ErrNotFound
	}
	if m.getRes != nil {
		return m.getRes, nil
	}
	return &models.GradeComponent{ID: id, Code: "HW", Name: "Homework"}, nil
}

func (m *mockGradeComponentRepo) FindByCode(ctx context.Context, code string) (*models.GradeComponent, error) {
	return &models.GradeComponent{ID: "cmp-1", Code: code, Name: "Component " + code}, nil
}

func (m *mockGradeComponentRepo) Update(ctx context.Context, component *models.GradeComponent) error {
	return nil
}

func (m *mockGradeComponentRepo) Delete(ctx context.Context, id string) error {
	if id == "notfound" {
		return appErrors.ErrNotFound
	}
	return nil
}

func setupGradeComponentRouter(h *GradeComponentHandler) *gin.Engine {
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

	api := r.Group("/api/v1/grade-components")
	api.Use(internalmiddleware.RBAC(string(models.RoleAdmin), string(models.RoleTeacher)))
	{
		api.GET("", h.List)
		api.POST("", h.Create)
		api.PUT("/:id", h.Update)
		api.DELETE("/:id", h.Delete)
	}
	return r
}

func TestGradeComponentHandlerListSuccess(t *testing.T) {
	repo := &mockGradeComponentRepo{
		components: []models.GradeComponent{{ID: "cmp-1", Code: "QUIZ", Name: "Quiz"}},
	}
	svc := service.NewGradeComponentService(repo, validator.New(), zap.NewNop())
	h := NewGradeComponentHandler(svc)
	router := setupGradeComponentRouter(h)

	req, _ := http.NewRequest(http.MethodGet, "/api/v1/grade-components?search=quiz", nil)
	req.Header.Set("X-Test-Role", string(models.RoleAdmin))

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestGradeComponentHandlerCreateSuccess(t *testing.T) {
	repo := &mockGradeComponentRepo{}
	svc := service.NewGradeComponentService(repo, validator.New(), zap.NewNop())
	h := NewGradeComponentHandler(svc)
	router := setupGradeComponentRouter(h)

	body, _ := json.Marshal(map[string]interface{}{
		"code": "EXAM",
		"name": "Final Exam",
	})
	req, _ := http.NewRequest(http.MethodPost, "/api/v1/grade-components", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Test-Role", string(models.RoleAdmin))

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)
}

func TestGradeComponentHandlerUpdateSuccess(t *testing.T) {
	repo := &mockGradeComponentRepo{}
	svc := service.NewGradeComponentService(repo, validator.New(), zap.NewNop())
	h := NewGradeComponentHandler(svc)
	router := setupGradeComponentRouter(h)

	body, _ := json.Marshal(map[string]interface{}{
		"code": "EXAM2",
		"name": "Final Exam 2",
	})
	req, _ := http.NewRequest(http.MethodPut, "/api/v1/grade-components/cmp-1", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Test-Role", string(models.RoleTeacher))

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestGradeComponentHandlerDeleteSuccess(t *testing.T) {
	repo := &mockGradeComponentRepo{}
	svc := service.NewGradeComponentService(repo, validator.New(), zap.NewNop())
	h := NewGradeComponentHandler(svc)
	router := setupGradeComponentRouter(h)

	req, _ := http.NewRequest(http.MethodDelete, "/api/v1/grade-components/cmp-1", nil)
	req.Header.Set("X-Test-Role", string(models.RoleAdmin))

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestGradeComponentHandlerUnauthorized(t *testing.T) {
	repo := &mockGradeComponentRepo{}
	svc := service.NewGradeComponentService(repo, validator.New(), zap.NewNop())
	h := NewGradeComponentHandler(svc)
	router := setupGradeComponentRouter(h)

	req, _ := http.NewRequest(http.MethodGet, "/api/v1/grade-components", nil)
	// No X-Test-Role

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestGradeComponentHandlerForbidden(t *testing.T) {
	repo := &mockGradeComponentRepo{}
	svc := service.NewGradeComponentService(repo, validator.New(), zap.NewNop())
	h := NewGradeComponentHandler(svc)
	router := setupGradeComponentRouter(h)

	req, _ := http.NewRequest(http.MethodGet, "/api/v1/grade-components", nil)
	req.Header.Set("X-Test-Role", string(models.RoleStudent)) // Not authorized

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusForbidden, w.Code)
}

func TestGradeComponentHandlerCreateInvalidJSON(t *testing.T) {
	repo := &mockGradeComponentRepo{}
	svc := service.NewGradeComponentService(repo, validator.New(), zap.NewNop())
	h := NewGradeComponentHandler(svc)
	router := setupGradeComponentRouter(h)

	req, _ := http.NewRequest(http.MethodPost, "/api/v1/grade-components", bytes.NewBufferString(`{invalid}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Test-Role", string(models.RoleAdmin))

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestGradeComponentHandlerInternalError(t *testing.T) {
	repo := &mockGradeComponentRepo{
		listErr: appErrors.ErrInternal,
	}
	svc := service.NewGradeComponentService(repo, validator.New(), zap.NewNop())
	h := NewGradeComponentHandler(svc)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	req, _ := http.NewRequest(http.MethodGet, "/grade-components", nil)
	c.Request = req

	h.List(c)
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}
