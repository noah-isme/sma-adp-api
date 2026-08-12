package handler

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"regexp"
	"testing"
	"time"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"
	"github.com/jmoiron/sqlx"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"golang.org/x/crypto/bcrypt"

	"github.com/noah-isme/sma-adp-api/internal/middleware"
	"github.com/noah-isme/sma-adp-api/internal/models"
	"github.com/noah-isme/sma-adp-api/internal/repository"
	"github.com/noah-isme/sma-adp-api/internal/service"
)

func TestAuthHandlerForgotPasswordUsesGenericResponseForUnknownEmail(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	require.NoError(t, err)
	defer db.Close()
	repo := repository.NewUserRepository(sqlx.NewDb(db, "sqlmock"))

	userSelectList := "SELECT id, email, password_hash, full_name, role, teacher_id, student_id, class_id, active, last_login, created_at, updated_at"
	mock.ExpectQuery(regexp.QuoteMeta(userSelectList + " FROM users WHERE email = $1 LIMIT 1")).
		WithArgs("missing@example.test").
		WillReturnError(sql.ErrNoRows)

	svc := service.NewAuthService(repo, nil, validator.New(), zap.NewNop(), service.AuthConfig{})
	h := NewAuthHandler(svc)
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.POST("/auth/forgot-password", h.ForgotPassword)

	req := httptest.NewRequest(http.MethodPost, "/auth/forgot-password", bytes.NewBufferString(`{"email":"missing@example.test"}`))
	req.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	assert.Equal(t, http.StatusAccepted, recorder.Code)
	var envelope struct {
		Data struct {
			Message string `json:"message"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &envelope))
	assert.Equal(t, "Jika email terdaftar, tautan reset akan dikirim", envelope.Data.Message)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestAuthHandlerForgotPasswordRejectsMalformedPayload(t *testing.T) {
	// A nil repository is safe here because binding fails before the service is called.
	svc := service.NewAuthService(nil, nil, validator.New(), zap.NewNop(), service.AuthConfig{})
	h := NewAuthHandler(svc)
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.POST("/auth/forgot-password", h.ForgotPassword)

	req := httptest.NewRequest(http.MethodPost, "/auth/forgot-password", bytes.NewBufferString(`{"email":`))
	req.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	assert.Equal(t, http.StatusBadRequest, recorder.Code)
	assert.NotEmpty(t, recorder.Body.String())
}

func TestAuthHandlerLoginSetsSecureRefreshCookieAndRedactsResponse(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	require.NoError(t, err)
	defer db.Close()
	repo := repository.NewUserRepository(sqlx.NewDb(db, "sqlmock"))

	passwordHash, err := bcrypt.GenerateFromPassword([]byte("Password123!"), bcrypt.MinCost)
	require.NoError(t, err)
	now := time.Now().UTC()
	userColumns := []string{"id", "email", "password_hash", "full_name", "role", "teacher_id", "student_id", "class_id", "active", "last_login", "created_at", "updated_at"}
	mock.ExpectQuery(regexp.QuoteMeta("SELECT id, email, password_hash, full_name, role, teacher_id, student_id, class_id, active, last_login, created_at, updated_at FROM users WHERE email = $1 LIMIT 1")).
		WithArgs("admin@example.test").
		WillReturnRows(sqlmock.NewRows(userColumns).AddRow("u1", "admin@example.test", string(passwordHash), "Admin", string(models.RoleAdmin), nil, nil, nil, true, nil, now, now))
	mock.ExpectExec("INSERT INTO refresh_tokens").WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec("UPDATE users SET last_login").WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec("INSERT INTO audit_logs").WillReturnResult(sqlmock.NewResult(1, 1))

	svc := service.NewAuthService(repo, nil, validator.New(), zap.NewNop(), service.AuthConfig{
		AccessTokenSecret:  "test-secret",
		AccessTokenExpiry:  time.Hour,
		RefreshTokenExpiry: time.Hour,
	})
	h := NewAuthHandler(svc)
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.POST("/api/v1/auth/login", h.Login)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewBufferString(`{"email":"admin@example.test","password":"Password123!"}`))
	req.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	assert.Equal(t, http.StatusOK, recorder.Code)
	var envelope struct {
		Data map[string]json.RawMessage `json:"data"`
	}
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &envelope))
	_, hasRefreshToken := envelope.Data["refresh_token"]
	assert.False(t, hasRefreshToken)

	cookies := recorder.Result().Cookies()
	require.Len(t, cookies, 1)
	assert.Equal(t, "refresh_token", cookies[0].Name)
	assert.Equal(t, "/api/v1/auth", cookies[0].Path)
	assert.True(t, cookies[0].HttpOnly)
	assert.True(t, cookies[0].Secure)
	assert.Equal(t, http.SameSiteStrictMode, cookies[0].SameSite)
	assert.Empty(t, cookies[0].Domain)
	assert.Equal(t, int((time.Hour)/time.Second), cookies[0].MaxAge)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestAuthHandlerRefreshReadsCookieWithoutJSONBodyAndRedactsResponse(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	require.NoError(t, err)
	defer db.Close()
	repo := repository.NewUserRepository(sqlx.NewDb(db, "sqlmock"))
	now := time.Now().UTC()

	mock.ExpectQuery(regexp.QuoteMeta("SELECT id, user_id, token, expires_at, created_at, revoked, revoked_at, ip_address, user_agent FROM refresh_tokens WHERE token = $1 LIMIT 1")).
		WithArgs("old-refresh").
		WillReturnRows(sqlmock.NewRows([]string{"id", "user_id", "token", "expires_at", "created_at", "revoked", "revoked_at", "ip_address", "user_agent"}).AddRow("rt1", "u1", "old-refresh", now.Add(time.Hour), now, false, nil, "127.0.0.1", "test"))
	mock.ExpectQuery(regexp.QuoteMeta("SELECT id, email, password_hash, full_name, role, teacher_id, student_id, class_id, active, last_login, created_at, updated_at FROM users WHERE id = $1 LIMIT 1")).
		WithArgs("u1").
		WillReturnRows(sqlmock.NewRows([]string{"id", "email", "password_hash", "full_name", "role", "teacher_id", "student_id", "class_id", "active", "last_login", "created_at", "updated_at"}).AddRow("u1", "admin@example.test", "unused", "Admin", string(models.RoleAdmin), nil, nil, nil, true, nil, now, now))
	mock.ExpectExec("UPDATE refresh_tokens").WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec("INSERT INTO refresh_tokens").WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec("INSERT INTO audit_logs").WillReturnResult(sqlmock.NewResult(1, 1))

	svc := service.NewAuthService(repo, nil, validator.New(), zap.NewNop(), service.AuthConfig{
		AccessTokenSecret:  "test-secret",
		AccessTokenExpiry:  time.Hour,
		RefreshTokenExpiry: time.Hour,
	})
	h := NewAuthHandler(svc)
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.POST("/api/v1/auth/refresh", h.Refresh)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/refresh", bytes.NewBufferString("{malformed"))
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(&http.Cookie{Name: "refresh_token", Value: "old-refresh"})
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	assert.Equal(t, http.StatusOK, recorder.Code)
	var envelope struct {
		Data map[string]json.RawMessage `json:"data"`
	}
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &envelope))
	_, hasRefreshToken := envelope.Data["refresh_token"]
	assert.False(t, hasRefreshToken)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestAuthHandlerLogoutClearsCookieWhenNoRefreshSessionRemains(t *testing.T) {
	svc := service.NewAuthService(nil, nil, validator.New(), zap.NewNop(), service.AuthConfig{})
	h := NewAuthHandler(svc)
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.POST("/api/v1/auth/logout", func(c *gin.Context) {
		c.Set(middleware.ContextUserKey, &models.JWTClaims{UserID: "u1"})
		h.Logout(c)
	})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/logout", nil)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	assert.Equal(t, http.StatusNoContent, recorder.Code)
	cookies := recorder.Result().Cookies()
	require.Len(t, cookies, 1)
	assert.Equal(t, "refresh_token", cookies[0].Name)
	assert.Equal(t, -1, cookies[0].MaxAge)
	assert.Equal(t, "/api/v1/auth", cookies[0].Path)
	assert.True(t, cookies[0].HttpOnly)
	assert.True(t, cookies[0].Secure)
	assert.Equal(t, http.SameSiteStrictMode, cookies[0].SameSite)
}

func TestAuthHandlerLogoutRevokesCookieWithoutAccessToken(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	require.NoError(t, err)
	defer db.Close()
	repo := repository.NewUserRepository(sqlx.NewDb(db, "sqlmock"))
	now := time.Now().UTC()
	mock.ExpectQuery(regexp.QuoteMeta("SELECT id, user_id, token, expires_at, created_at, revoked, revoked_at, ip_address, user_agent FROM refresh_tokens WHERE token = $1 LIMIT 1")).
		WithArgs("cookie-refresh").
		WillReturnRows(sqlmock.NewRows([]string{"id", "user_id", "token", "expires_at", "created_at", "revoked", "revoked_at", "ip_address", "user_agent"}).
			AddRow("rt1", "u1", "cookie-refresh", now.Add(time.Hour), now, false, nil, "127.0.0.1", "test"))
	mock.ExpectExec("UPDATE refresh_tokens").WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec("INSERT INTO audit_logs").WillReturnResult(sqlmock.NewResult(1, 1))

	svc := service.NewAuthService(repo, nil, validator.New(), zap.NewNop(), service.AuthConfig{})
	h := NewAuthHandler(svc)
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.POST("/api/v1/auth/logout", h.Logout)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/logout", nil)
	req.AddCookie(&http.Cookie{Name: "refresh_token", Value: "cookie-refresh"})
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	assert.Equal(t, http.StatusNoContent, recorder.Code)
	assert.Equal(t, -1, recorder.Result().Cookies()[0].MaxAge)
	assert.NoError(t, mock.ExpectationsWereMet())
}
