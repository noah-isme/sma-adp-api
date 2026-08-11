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
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"

	internalmiddleware "github.com/noah-isme/sma-adp-api/internal/middleware"
	"github.com/noah-isme/sma-adp-api/internal/models"
	"github.com/noah-isme/sma-adp-api/internal/service"
	appErrors "github.com/noah-isme/sma-adp-api/pkg/errors"
)

type mockAnnouncementRepo struct {
	listRes []models.Announcement
	count   int
	getRes  *models.Announcement
	getErr  error
}

func (m *mockAnnouncementRepo) List(ctx context.Context, filter models.AnnouncementFilter) ([]models.Announcement, int, error) {
	return m.listRes, m.count, nil
}

func (m *mockAnnouncementRepo) GetByID(ctx context.Context, id string) (*models.Announcement, error) {
	if m.getErr != nil {
		return nil, m.getErr
	}
	if m.getRes != nil {
		return m.getRes, nil
	}
	return &models.Announcement{ID: id, Title: "Test Announcement"}, nil
}

func (m *mockAnnouncementRepo) Create(ctx context.Context, announcement *models.Announcement) error {
	announcement.ID = "anc-123"
	announcement.CreatedAt = time.Now()
	return nil
}

func (m *mockAnnouncementRepo) Update(ctx context.Context, announcement *models.Announcement) error {
	return nil
}

func (m *mockAnnouncementRepo) Delete(ctx context.Context, id string) error {
	if id == "notfound" {
		return appErrors.ErrNotFound
	}
	return nil
}

func setupAnnouncementRouter(h *AnnouncementHandler) *gin.Engine {
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

	api := r.Group("/api/v1/announcements")
	{
		api.GET("", h.List)
		api.GET("/:id", h.Get)
		api.POST("", internalmiddleware.RBAC(string(models.RoleAdmin), string(models.RoleTeacher)), h.Create)
		api.PUT("/:id", internalmiddleware.RBAC(string(models.RoleAdmin), string(models.RoleTeacher)), h.Update)
		api.DELETE("/:id", internalmiddleware.RBAC(string(models.RoleAdmin)), h.Delete)
	}
	return r
}

func TestAnnouncementHandlerListSuccess(t *testing.T) {
	repo := &mockAnnouncementRepo{
		listRes: []models.Announcement{{ID: "anc-1", Title: "Notice"}},
		count:   1,
	}
	svc := service.NewAnnouncementService(repo, nil, zap.NewNop())
	h := NewAnnouncementHandler(svc)
	router := setupAnnouncementRouter(h)

	req, _ := http.NewRequest(http.MethodGet, "/api/v1/announcements?audience=ADMIN,TEACHER&includePinned=true&page=1&limit=10", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestAnnouncementHandlerGetSuccess(t *testing.T) {
	repo := &mockAnnouncementRepo{
		getRes: &models.Announcement{ID: "anc-1", Title: "Notice Details"},
	}
	svc := service.NewAnnouncementService(repo, nil, zap.NewNop())
	h := NewAnnouncementHandler(svc)
	router := setupAnnouncementRouter(h)

	req, _ := http.NewRequest(http.MethodGet, "/api/v1/announcements/anc-1", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestAnnouncementHandlerCreateSuccess(t *testing.T) {
	repo := &mockAnnouncementRepo{}
	svc := service.NewAnnouncementService(repo, nil, zap.NewNop())
	h := NewAnnouncementHandler(svc)
	router := setupAnnouncementRouter(h)

	body, _ := json.Marshal(map[string]interface{}{
		"title":        "New School Event",
		"content":      "Details of event",
		"audience":     "ALL",
		"priority":     "NORMAL",
		"published_at": time.Now().Format(time.RFC3339),
	})
	req, _ := http.NewRequest(http.MethodPost, "/api/v1/announcements", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Test-Role", string(models.RoleAdmin))

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)
}

func TestAnnouncementHandlerUpdateSuccess(t *testing.T) {
	repo := &mockAnnouncementRepo{}
	svc := service.NewAnnouncementService(repo, nil, zap.NewNop())
	h := NewAnnouncementHandler(svc)
	router := setupAnnouncementRouter(h)

	body, _ := json.Marshal(map[string]interface{}{
		"title":        "Updated Title",
		"content":      "Updated content",
		"audience":     "ALL",
		"priority":     "NORMAL",
		"published_at": time.Now().Format(time.RFC3339),
	})
	req, _ := http.NewRequest(http.MethodPut, "/api/v1/announcements/anc-1", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Test-Role", string(models.RoleTeacher))

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestAnnouncementHandlerDeleteSuccess(t *testing.T) {
	repo := &mockAnnouncementRepo{}
	svc := service.NewAnnouncementService(repo, nil, zap.NewNop())
	h := NewAnnouncementHandler(svc)
	router := setupAnnouncementRouter(h)

	req, _ := http.NewRequest(http.MethodDelete, "/api/v1/announcements/anc-1", nil)
	req.Header.Set("X-Test-Role", string(models.RoleAdmin))

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNoContent, w.Code)
}

func TestAnnouncementHandlerCreateUnauthorized(t *testing.T) {
	repo := &mockAnnouncementRepo{}
	svc := service.NewAnnouncementService(repo, nil, zap.NewNop())
	h := NewAnnouncementHandler(svc)
	router := setupAnnouncementRouter(h)

	body, _ := json.Marshal(map[string]interface{}{
		"title":   "Title",
		"content": "Content",
	})
	req, _ := http.NewRequest(http.MethodPost, "/api/v1/announcements", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	// No X-Test-Role

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestAnnouncementHandlerCreateForbidden(t *testing.T) {
	repo := &mockAnnouncementRepo{}
	svc := service.NewAnnouncementService(repo, nil, zap.NewNop())
	h := NewAnnouncementHandler(svc)
	router := setupAnnouncementRouter(h)

	body, _ := json.Marshal(map[string]interface{}{
		"title":   "Title",
		"content": "Content",
	})
	req, _ := http.NewRequest(http.MethodPost, "/api/v1/announcements", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Test-Role", string(models.RoleStudent))

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusForbidden, w.Code)
}

func TestAnnouncementHandlerCreateInvalidJSON(t *testing.T) {
	repo := &mockAnnouncementRepo{}
	svc := service.NewAnnouncementService(repo, nil, zap.NewNop())
	h := NewAnnouncementHandler(svc)
	router := setupAnnouncementRouter(h)

	req, _ := http.NewRequest(http.MethodPost, "/api/v1/announcements", bytes.NewBufferString(`{invalid}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Test-Role", string(models.RoleAdmin))

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestAnnouncementHandlerServiceError(t *testing.T) {
	repo := &mockAnnouncementRepo{
		getErr: appErrors.ErrInternal,
	}
	svc := service.NewAnnouncementService(repo, nil, zap.NewNop())
	h := NewAnnouncementHandler(svc)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	req, _ := http.NewRequest(http.MethodGet, "/announcements/anc-1", nil)
	c.Request = req
	c.Params = gin.Params{{Key: "id", Value: "anc-1"}}

	h.Get(c)
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

