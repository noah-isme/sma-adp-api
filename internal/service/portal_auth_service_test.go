package service

import (
	"context"
	"database/sql"
	"net/url"
	"testing"
	"time"

	"github.com/go-playground/validator/v10"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"golang.org/x/crypto/bcrypt"

	"github.com/noah-isme/sma-adp-api/internal/models"
	appErrors "github.com/noah-isme/sma-adp-api/pkg/errors"
)

type portalPasswordResetDeliveryStub struct {
	recipient string
	resetURL  string
	expiresAt time.Time
	calls     int
}

func (s *portalPasswordResetDeliveryStub) SendPasswordReset(_ context.Context, recipient, resetURL string, expiresAt time.Time) error {
	s.recipient = recipient
	s.resetURL = resetURL
	s.expiresAt = expiresAt
	s.calls++
	return nil
}

func newPortalPasswordResetService(repo *mockAuthRepo, delivery PasswordResetEmailDelivery) *PortalAuthService {
	return NewPortalAuthServiceWithEmailDelivery(repo, nil, validator.New(), zap.NewNop(), PortalAuthConfig{
		AccessTokenSecret:     "secret",
		AccessTokenExpiry:     time.Hour,
		RefreshTokenExpiry:    24 * time.Hour,
		PasswordResetTokenTTL: 30 * time.Minute,
		PasswordResetURL:      "https://portal.example.test/reset-password?source=email",
	}, delivery)
}

func TestPortalAuthServiceForgotPasswordCreatesAndDeliversPortalToken(t *testing.T) {
	repo := &mockAuthRepo{userByEmail: &models.User{
		ID:     "student-user",
		Email:  "student@example.test",
		Active: true,
		Role:   models.RoleSiswa,
	}}
	delivery := &portalPasswordResetDeliveryStub{}
	svc := newPortalPasswordResetService(repo, delivery)

	require.NoError(t, svc.PortalForgotPassword(context.Background(), models.PortalForgotPasswordRequest{Email: "student@example.test"}))
	require.NotNil(t, repo.passwordResetToken)
	assert.Equal(t, "student-user", repo.passwordResetToken.UserID)
	assert.NotEmpty(t, repo.passwordResetToken.TokenHash)
	assert.Equal(t, 1, delivery.calls)
	assert.Equal(t, "student@example.test", delivery.recipient)

	resetURL, err := url.Parse(delivery.resetURL)
	require.NoError(t, err)
	assert.Equal(t, "email", resetURL.Query().Get("source"))
	rawToken := resetURL.Query().Get("token")
	require.NotEmpty(t, rawToken)
	assert.Equal(t, hashPasswordResetToken(rawToken), repo.passwordResetToken.TokenHash)
	assert.WithinDuration(t, repo.passwordResetToken.ExpiresAt, delivery.expiresAt, time.Second)
}

func TestPortalAuthServiceForgotPasswordKeepsUnknownEmailGeneric(t *testing.T) {
	repo := &mockAuthRepo{findByEmailErr: sql.ErrNoRows}
	delivery := &portalPasswordResetDeliveryStub{}
	svc := newPortalPasswordResetService(repo, delivery)

	err := svc.PortalForgotPassword(context.Background(), models.PortalForgotPasswordRequest{Email: "missing@example.test"})
	require.NoError(t, err)
	assert.Nil(t, repo.passwordResetToken)
	assert.Zero(t, delivery.calls)
}

func TestPortalAuthServiceResetPasswordUpdatesPortalAccount(t *testing.T) {
	user := &models.User{ID: "parent-user", Email: "parent@example.test", Active: true, Role: models.RoleOrtu}
	repo := &mockAuthRepo{
		userByEmail: user,
		userByID:    user,
		passwordResetToken: &models.PasswordResetToken{
			ID:        "reset-token",
			UserID:    user.ID,
			TokenHash: hashPasswordResetToken("valid-token"),
			ExpiresAt: time.Now().Add(time.Hour),
		},
	}
	svc := newPortalPasswordResetService(repo, &portalPasswordResetDeliveryStub{})

	require.NoError(t, svc.PortalResetPassword(context.Background(), models.PortalResetPasswordRequest{
		Token:    "valid-token",
		Password: "new-password",
	}))
	assert.NoError(t, bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte("new-password")))
	assert.True(t, repo.refreshTokensRevoked)
	require.Len(t, repo.auditLogs, 1)
	assert.Equal(t, models.AuditActionPasswordReset, repo.auditLogs[0].Action)
	assert.Equal(t, "portal_auth", repo.auditLogs[0].Resource)
}

func TestPortalAuthServiceResetPasswordRejectsInvalidToken(t *testing.T) {
	svc := newPortalPasswordResetService(&mockAuthRepo{consumeResetErr: sql.ErrNoRows}, &portalPasswordResetDeliveryStub{})

	err := svc.PortalResetPassword(context.Background(), models.PortalResetPasswordRequest{Token: "invalid-token", Password: "new-password"})
	require.Error(t, err)
	assert.Equal(t, appErrors.ErrUnauthorized.Code, appErrors.FromError(err).Code)
}
