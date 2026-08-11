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

func setupExportRouter(h *ExportCompatibilityHandler) *gin.Engine {
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

	api := r.Group("/api/v1/export")
	api.Use(internalmiddleware.RBAC(string(models.RoleAdmin), string(models.RoleTeacher)))
	{
		api.GET("/students", h.Students)
		api.GET("/grades", h.Grades)
		api.GET("/attendance", h.Attendance)
	}
	return r
}

func TestExportCompatibilityStudentsSuccess(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()
	sqlxDB := sqlx.NewDb(db, "sqlmock")

	rows := sqlmock.NewRows([]string{"id", "nis", "full_name", "gender", "active"}).
		AddRow("std-1", "1001", "John Doe", "L", true)
	mock.ExpectQuery(`SELECT s\.id, s\.nis, s\.full_name, s\.gender, s\.active FROM students`).
		WillReturnRows(rows)

	h := NewExportCompatibilityHandler(sqlxDB)
	router := setupExportRouter(h)

	req, _ := http.NewRequest(http.MethodGet, "/api/v1/export/students?status=active", nil)
	req.Header.Set("X-Test-Role", string(models.RoleAdmin))

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "text/csv; charset=utf-8", w.Header().Get("Content-Type"))
	assert.Contains(t, w.Body.String(), "John Doe")
}

func TestExportCompatibilityGradesSuccess(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()
	sqlxDB := sqlx.NewDb(db, "sqlmock")

	rows := sqlmock.NewRows([]string{"id", "enrollment_id", "subject_id", "component_id", "grade_value", "updated_at"}).
		AddRow("g-1", "enr-1", "subj-1", "cmp-1", 88.5, "2026-01-01")
	mock.ExpectQuery(`SELECT g\.id, g\.enrollment_id`).
		WillReturnRows(rows)

	h := NewExportCompatibilityHandler(sqlxDB)
	router := setupExportRouter(h)

	req, _ := http.NewRequest(http.MethodGet, "/api/v1/export/grades?status=PASS", nil)
	req.Header.Set("X-Test-Role", string(models.RoleTeacher))

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "88.5")
}

func TestExportCompatibilityAttendanceSuccess(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()
	sqlxDB := sqlx.NewDb(db, "sqlmock")

	rows := sqlmock.NewRows([]string{"id", "enrollment_id", "date", "status", "notes", "updated_at"}).
		AddRow("att-1", "enr-1", "2026-01-01", "PRESENT", "On time", "2026-01-01")
	mock.ExpectQuery(`SELECT da\.id, da\.enrollment_id`).
		WillReturnRows(rows)

	h := NewExportCompatibilityHandler(sqlxDB)
	router := setupExportRouter(h)

	req, _ := http.NewRequest(http.MethodGet, "/api/v1/export/attendance", nil)
	req.Header.Set("X-Test-Role", string(models.RoleAdmin))

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "PRESENT")
}

func TestExportCompatibilityUnauthorized(t *testing.T) {
	db, _, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()
	sqlxDB := sqlx.NewDb(db, "sqlmock")

	h := NewExportCompatibilityHandler(sqlxDB)
	router := setupExportRouter(h)

	req, _ := http.NewRequest(http.MethodGet, "/api/v1/export/students", nil)
	// No X-Test-Role

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestExportCompatibilityForbidden(t *testing.T) {
	db, _, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()
	sqlxDB := sqlx.NewDb(db, "sqlmock")

	h := NewExportCompatibilityHandler(sqlxDB)
	router := setupExportRouter(h)

	req, _ := http.NewRequest(http.MethodGet, "/api/v1/export/students", nil)
	req.Header.Set("X-Test-Role", string(models.RoleStudent)) // Not authorized

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusForbidden, w.Code)
}

func TestExportCompatibilityStudentsInvalidStatus(t *testing.T) {
	db, _, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()
	sqlxDB := sqlx.NewDb(db, "sqlmock")

	h := NewExportCompatibilityHandler(sqlxDB)
	router := setupExportRouter(h)

	req, _ := http.NewRequest(http.MethodGet, "/api/v1/export/students?status=invalid_status", nil)
	req.Header.Set("X-Test-Role", string(models.RoleAdmin))

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "status must be active or inactive")
}

func TestExportCompatibilityGradesInvalidStatus(t *testing.T) {
	db, _, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()
	sqlxDB := sqlx.NewDb(db, "sqlmock")

	h := NewExportCompatibilityHandler(sqlxDB)
	router := setupExportRouter(h)

	req, _ := http.NewRequest(http.MethodGet, "/api/v1/export/grades?status=INVALID", nil)
	req.Header.Set("X-Test-Role", string(models.RoleAdmin))

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "status must be PASS, REMEDIAL, or FAIL")
}

func TestExportCompatibilityStudentsDBError(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()
	sqlxDB := sqlx.NewDb(db, "sqlmock")

	mock.ExpectQuery(`SELECT s\.id, s\.nis, s\.full_name, s\.gender, s\.active FROM students`).
		WillReturnError(assert.AnError)

	h := NewExportCompatibilityHandler(sqlxDB)
	router := setupExportRouter(h)

	req, _ := http.NewRequest(http.MethodGet, "/api/v1/export/students", nil)
	req.Header.Set("X-Test-Role", string(models.RoleAdmin))

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

