package handler

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"regexp"
	"testing"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"
	"github.com/jmoiron/sqlx"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

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
