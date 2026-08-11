package handler

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/gin-gonic/gin"
	"github.com/jmoiron/sqlx"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	internalmiddleware "github.com/noah-isme/sma-adp-api/internal/middleware"
	"github.com/noah-isme/sma-adp-api/internal/models"
)

func setupCompatTestRouter(h *ExportCompatibilityHandler) *gin.Engine {
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

	api := r.Group("/api/v1/compat")
	api.Use(internalmiddleware.RBAC(string(models.RoleAdmin), string(models.RoleTeacher)))
	{
		api.GET("/students", h.Students)
		api.GET("/grades", h.Grades)
		api.GET("/attendance", h.Attendance)
	}
	return r
}

func TestCompatHandlerStudentsSuccess(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()
	sqlxDB := sqlx.NewDb(db, "sqlmock")

	rows := sqlmock.NewRows([]string{"id", "nis", "full_name", "gender", "active"}).
		AddRow("std-101", "1001", "Ahmad", "L", true)
	mock.ExpectQuery(`SELECT s\.id, s\.nis, s\.full_name, s\.gender, s\.active FROM students`).
		WillReturnRows(rows)

	h := NewExportCompatibilityHandler(sqlxDB)
	router := setupCompatTestRouter(h)

	req, _ := http.NewRequest(http.MethodGet, "/api/v1/compat/students?status=active", nil)
	req.Header.Set("X-Test-Role", string(models.RoleAdmin))

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "Ahmad")
}

func TestCompatHandlerUnauthorized(t *testing.T) {
	db, _, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()
	sqlxDB := sqlx.NewDb(db, "sqlmock")

	h := NewExportCompatibilityHandler(sqlxDB)
	router := setupCompatTestRouter(h)

	req, _ := http.NewRequest(http.MethodGet, "/api/v1/compat/students", nil)

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestCompatHandlerForbidden(t *testing.T) {
	db, _, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()
	sqlxDB := sqlx.NewDb(db, "sqlmock")

	h := NewExportCompatibilityHandler(sqlxDB)
	router := setupCompatTestRouter(h)

	req, _ := http.NewRequest(http.MethodGet, "/api/v1/compat/students", nil)
	req.Header.Set("X-Test-Role", string(models.RoleStudent))

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusForbidden, w.Code)
}

func TestCompatHandlerValidation(t *testing.T) {
	db, _, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()
	sqlxDB := sqlx.NewDb(db, "sqlmock")

	h := NewExportCompatibilityHandler(sqlxDB)
	router := setupCompatTestRouter(h)

	req, _ := http.NewRequest(http.MethodGet, "/api/v1/compat/students?status=invalid", nil)
	req.Header.Set("X-Test-Role", string(models.RoleAdmin))

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}
