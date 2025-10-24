package service

import (
	"context"
	"errors"
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

type mockAuthRepo struct {
	userByEmail         *models.User
	userByID            *models.User
	findByEmailErr      error
	findByIDErr         error
	refreshTokens       map[string]*models.RefreshToken
	refreshTokenErr     error
	createRefreshErr    error
	revokeRefreshErr    error
	revokeUserTokensErr error
	updatePasswordErr   error
	auditLogs           []*models.AuditLog
	lastLoginUpdated    bool
}

func (m *mockAuthRepo) FindByEmail(ctx context.Context, email string) (*models.User, error) {
	if m.findByEmailErr != nil {
		return nil, m.findByEmailErr
	}
	return m.userByEmail, nil
}

func (m *mockAuthRepo) FindByID(ctx context.Context, id string) (*models.User, error) {
	if m.findByIDErr != nil {
		return nil, m.findByIDErr
	}
	if m.userByID != nil {
		return m.userByID, nil
	}
	return m.userByEmail, nil
}

func (m *mockAuthRepo) UpdateLastLogin(ctx context.Context, id string, ts time.Time) error {
	m.lastLoginUpdated = true
	return nil
}

func (m *mockAuthRepo) UpdatePassword(ctx context.Context, id, passwordHash string, updatedAt time.Time) error {
	if m.updatePasswordErr != nil {
		return m.updatePasswordErr
	}
	if m.userByEmail != nil && m.userByEmail.ID == id {
		m.userByEmail.PasswordHash = passwordHash
	}
	return nil
}

func (m *mockAuthRepo) RevokeUserRefreshTokens(ctx context.Context, userID string) error {
	return m.revokeUserTokensErr
}

func (m *mockAuthRepo) CreateRefreshToken(ctx context.Context, token *models.RefreshToken) error {
	if m.createRefreshErr != nil {
		return m.createRefreshErr
	}
	if m.refreshTokens == nil {
		m.refreshTokens = make(map[string]*models.RefreshToken)
	}
	m.refreshTokens[token.Token] = token
	return nil
}

func (m *mockAuthRepo) FindRefreshToken(ctx context.Context, token string) (*models.RefreshToken, error) {
	if m.refreshTokenErr != nil {
		return nil, m.refreshTokenErr
	}
	rt, ok := m.refreshTokens[token]
	if !ok {
		return nil, errors.New("not found")
	}
	return rt, nil
}

func (m *mockAuthRepo) RevokeRefreshToken(ctx context.Context, id string, revokedAt time.Time) error {
	if m.revokeRefreshErr != nil {
		return m.revokeRefreshErr
	}
	for _, token := range m.refreshTokens {
		if token.ID == id {
			token.Revoked = true
			token.RevokedAt = &revokedAt
		}
	}
	return nil
}

func (m *mockAuthRepo) CreateAuditLog(ctx context.Context, log *models.AuditLog) error {
	m.auditLogs = append(m.auditLogs, log)
	return nil
}

func TestAuthServiceLoginSuccess(t *testing.T) {
	password, _ := bcrypt.GenerateFromPassword([]byte("password"), bcrypt.DefaultCost)
	repo := &mockAuthRepo{userByEmail: &models.User{ID: "123", Email: "user@example.com", PasswordHash: string(password), Active: true, Role: models.RoleAdmin}}
	svc := NewAuthService(repo, validator.New(), zap.NewNop(), AuthConfig{
		AccessTokenSecret:  "secret",
		AccessTokenExpiry:  time.Hour,
		RefreshTokenExpiry: time.Hour * 24,
	})

	res, err := svc.Login(context.Background(), models.LoginRequest{Email: "user@example.com", Password: "password"})
	require.NoError(t, err)
	assert.NotEmpty(t, res.AccessToken)
	assert.NotEmpty(t, res.RefreshToken)
	assert.True(t, repo.lastLoginUpdated)
	assert.NotEmpty(t, repo.refreshTokens)
}

func TestAuthServiceLoginInactive(t *testing.T) {
	password, _ := bcrypt.GenerateFromPassword([]byte("password"), bcrypt.DefaultCost)
	repo := &mockAuthRepo{userByEmail: &models.User{ID: "123", Email: "user@example.com", PasswordHash: string(password), Active: false}}
	svc := NewAuthService(repo, validator.New(), zap.NewNop(), AuthConfig{AccessTokenSecret: "secret", AccessTokenExpiry: time.Hour, RefreshTokenExpiry: time.Hour})

	_, err := svc.Login(context.Background(), models.LoginRequest{Email: "user@example.com", Password: "password"})
	require.Error(t, err)
	appErr := appErrors.FromError(err)
	assert.Equal(t, appErrors.ErrInactiveAccount.Code, appErr.Code)
}

func TestAuthServiceRefreshToken(t *testing.T) {
	repo := &mockAuthRepo{refreshTokens: make(map[string]*models.RefreshToken)}
	user := &models.User{ID: "u1", Email: "user@example.com", PasswordHash: "hash", Active: true, Role: models.RoleAdmin}
	repo.userByEmail = user
	repo.userByID = user
	token := &models.RefreshToken{ID: "rt1", UserID: user.ID, Token: "token", ExpiresAt: time.Now().Add(time.Hour)}
	repo.refreshTokens[token.Token] = token

	svc := NewAuthService(repo, validator.New(), zap.NewNop(), AuthConfig{AccessTokenSecret: "secret", AccessTokenExpiry: time.Hour, RefreshTokenExpiry: time.Hour})

	res, err := svc.RefreshToken(context.Background(), models.RefreshTokenRequest{RefreshToken: "token"})
	require.NoError(t, err)
	assert.NotEmpty(t, res.AccessToken)
	assert.NotEqual(t, "token", res.RefreshToken)
	assert.True(t, repo.refreshTokens["token"].Revoked)
}

func TestAuthServiceChangePassword(t *testing.T) {
	oldHash, _ := bcrypt.GenerateFromPassword([]byte("old"), bcrypt.DefaultCost)
	repo := &mockAuthRepo{userByEmail: &models.User{ID: "u1", PasswordHash: string(oldHash), Active: true}}
	svc := NewAuthService(repo, validator.New(), zap.NewNop(), AuthConfig{AccessTokenSecret: "secret", AccessTokenExpiry: time.Hour, RefreshTokenExpiry: time.Hour})

	err := svc.ChangePassword(context.Background(), "u1", models.ChangePasswordRequest{OldPassword: "old", NewPassword: "newpassword"})
	require.NoError(t, err)
	assert.NotEqual(t, string(oldHash), repo.userByEmail.PasswordHash)
}

func TestValidateToken(t *testing.T) {
	repo := &mockAuthRepo{}
	svc := NewAuthService(repo, validator.New(), zap.NewNop(), AuthConfig{AccessTokenSecret: "secret", AccessTokenExpiry: time.Hour, RefreshTokenExpiry: time.Hour})
	user := &models.User{ID: "u1", Email: "user@example.com", Role: models.RoleAdmin}
	token, _, err := svc.generateAccessToken(user)
	require.NoError(t, err)

	claims, err := svc.ValidateToken(token)
	require.NoError(t, err)
	assert.Equal(t, user.ID, claims.UserID)
}
