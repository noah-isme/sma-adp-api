package service

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
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

type recordingPasswordResetDelivery struct {
	recipient string
	resetURL  string
	expiresAt time.Time
	err       error
}

func (d *recordingPasswordResetDelivery) SendPasswordReset(_ context.Context, recipient, resetURL string, expiresAt time.Time) error {
	d.recipient = recipient
	d.resetURL = resetURL
	d.expiresAt = expiresAt
	return d.err
}

func newPasswordResetTestService(repo *mockAuthRepo, delivery PasswordResetEmailDelivery) *AuthService {
	return NewAuthServiceWithEmailDelivery(repo, nil, validator.New(), zap.NewNop(), AuthConfig{
		AccessTokenSecret:     "secret",
		AccessTokenExpiry:     time.Hour,
		RefreshTokenExpiry:    time.Hour,
		PasswordResetTokenTTL: 30 * time.Minute,
		PasswordResetURL:      "https://admin.example.test/reset-password",
	}, delivery)
}

func TestAuthServiceForgotPasswordPersistsHashedExpiringTokenAndSendsEmail(t *testing.T) {
	repo := &mockAuthRepo{userByEmail: &models.User{
		ID:       "user-1",
		Email:    "admin@example.test",
		FullName: "Admin",
		Active:   true,
	}}
	delivery := &recordingPasswordResetDelivery{}
	svc := newPasswordResetTestService(repo, delivery)

	startedAt := time.Now().UTC()
	require.NoError(t, svc.ForgotPassword(context.Background(), models.ResetPasswordRequest{Email: "ADMIN@EXAMPLE.TEST"}))

	require.NotNil(t, repo.passwordResetToken)
	require.NotEmpty(t, delivery.resetURL)
	parsedURL, err := url.Parse(delivery.resetURL)
	require.NoError(t, err)
	rawToken := parsedURL.Query().Get("token")
	require.NotEmpty(t, rawToken)

	digest := sha256.Sum256([]byte(rawToken))
	assert.Equal(t, hex.EncodeToString(digest[:]), repo.passwordResetToken.TokenHash)
	assert.NotEqual(t, rawToken, repo.passwordResetToken.TokenHash)
	assert.Equal(t, "admin@example.test", delivery.recipient)
	assert.WithinDuration(t, startedAt.Add(30*time.Minute), repo.passwordResetToken.ExpiresAt, 2*time.Second)
	assert.Equal(t, repo.passwordResetToken.ExpiresAt, delivery.expiresAt)
}

func TestAuthServiceForgotPasswordDoesNotRevealUnknownEmail(t *testing.T) {
	repo := &mockAuthRepo{findByEmailErr: sql.ErrNoRows}
	delivery := &recordingPasswordResetDelivery{}
	svc := newPasswordResetTestService(repo, delivery)

	require.NoError(t, svc.ForgotPassword(context.Background(), models.ResetPasswordRequest{Email: "missing@example.test"}))
	assert.Nil(t, repo.passwordResetToken)
	assert.Empty(t, delivery.resetURL)
}

func TestAuthServiceResetPasswordConsumesTokenAndStoresBcryptHash(t *testing.T) {
	oldHash, err := bcrypt.GenerateFromPassword([]byte("old-password"), bcrypt.DefaultCost)
	require.NoError(t, err)
	repo := &mockAuthRepo{userByEmail: &models.User{
		ID:           "user-1",
		Email:        "admin@example.test",
		PasswordHash: string(oldHash),
		Active:       true,
	}}
	const rawToken = "reset-token-for-test"
	repo.passwordResetToken = &models.PasswordResetToken{
		ID:        "reset-1",
		UserID:    "user-1",
		TokenHash: hashPasswordResetToken(rawToken),
		ExpiresAt: time.Now().UTC().Add(time.Hour),
		CreatedAt: time.Now().UTC(),
	}
	svc := newPasswordResetTestService(repo, &recordingPasswordResetDelivery{})

	require.NoError(t, svc.ResetPassword(context.Background(), models.ConfirmResetPasswordRequest{
		Token:       rawToken,
		NewPassword: "new-password",
	}))

	require.NoError(t, bcrypt.CompareHashAndPassword([]byte(repo.userByEmail.PasswordHash), []byte("new-password")))
	assert.NotNil(t, repo.passwordResetToken.UsedAt)
	assert.True(t, repo.refreshTokensRevoked)
	require.Len(t, repo.auditLogs, 1)
	assert.Equal(t, models.AuditActionPasswordReset, repo.auditLogs[0].Action)
}

func TestAuthServiceResetPasswordRejectsExpiredToken(t *testing.T) {
	passwordHash, err := bcrypt.GenerateFromPassword([]byte("old-password"), bcrypt.DefaultCost)
	require.NoError(t, err)
	repo := &mockAuthRepo{
		userByEmail: &models.User{ID: "user-1", PasswordHash: string(passwordHash), Active: true},
		passwordResetToken: &models.PasswordResetToken{
			UserID:    "user-1",
			TokenHash: hashPasswordResetToken("expired-token"),
			ExpiresAt: time.Now().UTC().Add(-time.Minute),
		},
	}
	svc := newPasswordResetTestService(repo, &recordingPasswordResetDelivery{})

	err = svc.ResetPassword(context.Background(), models.ConfirmResetPasswordRequest{
		Token:       "expired-token",
		NewPassword: "new-password",
	})

	require.Error(t, err)
	assert.Equal(t, appErrors.ErrUnauthorized.Code, appErrors.FromError(err).Code)
	assert.False(t, repo.refreshTokensRevoked)
	require.NoError(t, bcrypt.CompareHashAndPassword([]byte(repo.userByEmail.PasswordHash), []byte("old-password")))
}
