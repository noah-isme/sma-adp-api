package service

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/base64"
	"errors"
	"fmt"
	"time"

	"github.com/go-playground/validator/v10"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"go.uber.org/zap"
	"golang.org/x/crypto/bcrypt"

	"github.com/noah-isme/sma-adp-api/internal/models"
	appErrors "github.com/noah-isme/sma-adp-api/pkg/errors"
)

type authUserRepository interface {
	FindByEmail(ctx context.Context, email string) (*models.User, error)
	FindByID(ctx context.Context, id string) (*models.User, error)
	UpdateLastLogin(ctx context.Context, id string, ts time.Time) error
	UpdatePassword(ctx context.Context, id, passwordHash string, updatedAt time.Time) error
	RevokeUserRefreshTokens(ctx context.Context, userID string) error
	CreateRefreshToken(ctx context.Context, token *models.RefreshToken) error
	FindRefreshToken(ctx context.Context, token string) (*models.RefreshToken, error)
	RevokeRefreshToken(ctx context.Context, id string, revokedAt time.Time) error
	CreateAuditLog(ctx context.Context, log *models.AuditLog) error
}

// AuthConfig defines configuration for authentication flows.
type AuthConfig struct {
	AccessTokenSecret  string
	AccessTokenExpiry  time.Duration
	RefreshTokenExpiry time.Duration
	Issuer             string
	Audience           []string
	SingleSession      bool
}

// AuthService provides authentication use cases.
type AuthService struct {
	repo      authUserRepository
	validator *validator.Validate
	logger    *zap.Logger
	config    AuthConfig
}

// NewAuthService constructs an AuthService instance.
func NewAuthService(repo authUserRepository, validate *validator.Validate, logger *zap.Logger, config AuthConfig) *AuthService {
	if logger == nil {
		logger = zap.NewNop()
	}
	if validate == nil {
		validate = validator.New()
	}
	return &AuthService{repo: repo, validator: validate, logger: logger, config: config}
}

// Login authenticates a user and returns issued tokens.
func (s *AuthService) Login(ctx context.Context, req models.LoginRequest) (*models.LoginResponse, error) {
	if err := s.validator.Struct(req); err != nil {
		return nil, appErrors.Wrap(err, appErrors.ErrValidation.Code, appErrors.ErrValidation.Status, "invalid login payload")
	}

	user, err := s.repo.FindByEmail(ctx, req.Email)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, appErrors.Clone(appErrors.ErrInvalidCredentials, "invalid email or password")
		}
		return nil, appErrors.Wrap(err, appErrors.ErrInternal.Code, appErrors.ErrInternal.Status, "failed to fetch user")
	}

	if !user.Active {
		return nil, appErrors.Clone(appErrors.ErrInactiveAccount, "account is inactive")
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password)); err != nil {
		return nil, appErrors.Clone(appErrors.ErrInvalidCredentials, "invalid email or password")
	}

	if s.config.SingleSession {
		if err := s.repo.RevokeUserRefreshTokens(ctx, user.ID); err != nil {
			s.logger.Warn("failed to revoke previous refresh tokens", zap.Error(err))
		}
	}

	accessToken, _, err := s.generateAccessToken(user)
	if err != nil {
		return nil, appErrors.Wrap(err, appErrors.ErrInternal.Code, appErrors.ErrInternal.Status, "failed to create access token")
	}

	refreshTokenValue, err := s.generateRefreshTokenString()
	if err != nil {
		return nil, appErrors.Wrap(err, appErrors.ErrInternal.Code, appErrors.ErrInternal.Status, "failed to create refresh token")
	}

	refreshToken := &models.RefreshToken{
		ID:        uuid.NewString(),
		UserID:    user.ID,
		Token:     refreshTokenValue,
		ExpiresAt: time.Now().UTC().Add(s.config.RefreshTokenExpiry),
		CreatedAt: time.Now().UTC(),
		Revoked:   false,
		IPAddress: req.IP,
		UserAgent: req.UserAgent,
	}

	if err := s.repo.CreateRefreshToken(ctx, refreshToken); err != nil {
		return nil, appErrors.Wrap(err, appErrors.ErrInternal.Code, appErrors.ErrInternal.Status, "failed to persist refresh token")
	}

	if err := s.repo.UpdateLastLogin(ctx, user.ID, time.Now().UTC()); err != nil {
		s.logger.Warn("failed to update last login", zap.Error(err))
	}

	if err := s.repo.CreateAuditLog(ctx, &models.AuditLog{
		UserID:     &user.ID,
		Action:     models.AuditActionLogin,
		Resource:   "auth",
		ResourceID: &user.ID,
		NewValues:  []byte(`{"status":"success"}`),
		IPAddress:  req.IP,
		UserAgent:  req.UserAgent,
	}); err != nil {
		s.logger.Warn("failed to record login audit log", zap.Error(err))
	}

	return &models.LoginResponse{
		AccessToken:  accessToken,
		RefreshToken: refreshToken.Token,
		ExpiresIn:    int64(s.config.AccessTokenExpiry.Seconds()),
		IssuedAt:     time.Now().UTC(),
		User: models.UserInfo{
			ID:       user.ID,
			Email:    user.Email,
			FullName: user.FullName,
			Role:     user.Role,
		},
	}, nil
}

// RefreshToken exchanges a refresh token for a new access token pair.
func (s *AuthService) RefreshToken(ctx context.Context, req models.RefreshTokenRequest) (*models.RefreshTokenResponse, error) {
	if err := s.validator.Struct(req); err != nil {
		return nil, appErrors.Wrap(err, appErrors.ErrValidation.Code, appErrors.ErrValidation.Status, "invalid refresh payload")
	}

	storedToken, err := s.repo.FindRefreshToken(ctx, req.RefreshToken)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, appErrors.Clone(appErrors.ErrUnauthorized, "refresh token not found")
		}
		return nil, appErrors.Wrap(err, appErrors.ErrInternal.Code, appErrors.ErrInternal.Status, "failed to fetch refresh token")
	}

	if storedToken.Revoked || time.Now().UTC().After(storedToken.ExpiresAt) {
		return nil, appErrors.Clone(appErrors.ErrUnauthorized, "refresh token is expired or revoked")
	}

	user, err := s.repo.FindByID(ctx, storedToken.UserID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, appErrors.Clone(appErrors.ErrUnauthorized, "associated user no longer exists")
		}
		return nil, appErrors.Wrap(err, appErrors.ErrInternal.Code, appErrors.ErrInternal.Status, "failed to load user")
	}

	if !user.Active {
		return nil, appErrors.Clone(appErrors.ErrInactiveAccount, "account is inactive")
	}

	if err := s.repo.RevokeRefreshToken(ctx, storedToken.ID, time.Now().UTC()); err != nil {
		s.logger.Warn("failed to revoke used refresh token", zap.Error(err))
	}

	accessToken, _, err := s.generateAccessToken(user)
	if err != nil {
		return nil, appErrors.Wrap(err, appErrors.ErrInternal.Code, appErrors.ErrInternal.Status, "failed to generate access token")
	}

	refreshTokenValue, err := s.generateRefreshTokenString()
	if err != nil {
		return nil, appErrors.Wrap(err, appErrors.ErrInternal.Code, appErrors.ErrInternal.Status, "failed to create refresh token")
	}

	newRefresh := &models.RefreshToken{
		ID:        uuid.NewString(),
		UserID:    user.ID,
		Token:     refreshTokenValue,
		ExpiresAt: time.Now().UTC().Add(s.config.RefreshTokenExpiry),
		CreatedAt: time.Now().UTC(),
		Revoked:   false,
		IPAddress: req.IP,
		UserAgent: req.UserAgent,
	}

	if err := s.repo.CreateRefreshToken(ctx, newRefresh); err != nil {
		return nil, appErrors.Wrap(err, appErrors.ErrInternal.Code, appErrors.ErrInternal.Status, "failed to persist refresh token")
	}

	if err := s.repo.CreateAuditLog(ctx, &models.AuditLog{
		UserID:     &user.ID,
		Action:     models.AuditActionLogin,
		Resource:   "auth",
		ResourceID: &user.ID,
		NewValues:  []byte(`{"refresh":"rotated"}`),
		IPAddress:  req.IP,
		UserAgent:  req.UserAgent,
	}); err != nil {
		s.logger.Warn("failed to record refresh audit log", zap.Error(err))
	}

	return &models.RefreshTokenResponse{
		AccessToken:  accessToken,
		RefreshToken: newRefresh.Token,
		ExpiresIn:    int64(s.config.AccessTokenExpiry.Seconds()),
		IssuedAt:     time.Now().UTC(),
	}, nil
}

// Logout revokes the provided refresh token.
func (s *AuthService) Logout(ctx context.Context, refreshToken string, userID string, meta models.LoginRequest) error {
	storedToken, err := s.repo.FindRefreshToken(ctx, refreshToken)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return appErrors.Clone(appErrors.ErrUnauthorized, "refresh token not found")
		}
		return appErrors.Wrap(err, appErrors.ErrInternal.Code, appErrors.ErrInternal.Status, "failed to load refresh token")
	}

	if storedToken.UserID != userID {
		return appErrors.Clone(appErrors.ErrForbidden, "token does not belong to user")
	}

	if err := s.repo.RevokeRefreshToken(ctx, storedToken.ID, time.Now().UTC()); err != nil {
		return appErrors.Wrap(err, appErrors.ErrInternal.Code, appErrors.ErrInternal.Status, "failed to revoke refresh token")
	}

	if err := s.repo.CreateAuditLog(ctx, &models.AuditLog{
		UserID:     &userID,
		Action:     models.AuditActionLogout,
		Resource:   "auth",
		ResourceID: &userID,
		NewValues:  []byte(`{"status":"logout"}`),
		IPAddress:  meta.IP,
		UserAgent:  meta.UserAgent,
	}); err != nil {
		s.logger.Warn("failed to record logout audit log", zap.Error(err))
	}

	return nil
}

// ChangePassword changes the password for the given user ID.
func (s *AuthService) ChangePassword(ctx context.Context, userID string, req models.ChangePasswordRequest) error {
	if err := s.validator.Struct(req); err != nil {
		return appErrors.Wrap(err, appErrors.ErrValidation.Code, appErrors.ErrValidation.Status, "invalid change password payload")
	}

	user, err := s.repo.FindByID(ctx, userID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return appErrors.Clone(appErrors.ErrNotFound, "user not found")
		}
		return appErrors.Wrap(err, appErrors.ErrInternal.Code, appErrors.ErrInternal.Status, "failed to load user")
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.OldPassword)); err != nil {
		return appErrors.Clone(appErrors.ErrForbidden, "old password does not match")
	}

	newHash, err := bcrypt.GenerateFromPassword([]byte(req.NewPassword), bcrypt.DefaultCost)
	if err != nil {
		return appErrors.Wrap(err, appErrors.ErrInternal.Code, appErrors.ErrInternal.Status, "failed to hash password")
	}

	if err := s.repo.UpdatePassword(ctx, userID, string(newHash), time.Now().UTC()); err != nil {
		return appErrors.Wrap(err, appErrors.ErrInternal.Code, appErrors.ErrInternal.Status, "failed to update password")
	}

	if err := s.repo.RevokeUserRefreshTokens(ctx, userID); err != nil {
		s.logger.Warn("failed to revoke refresh tokens after password change", zap.Error(err))
	}

	if err := s.repo.CreateAuditLog(ctx, &models.AuditLog{
		UserID:     &userID,
		Action:     models.AuditActionPasswordChange,
		Resource:   "auth",
		ResourceID: &userID,
		NewValues:  []byte(`{"status":"changed"}`),
	}); err != nil {
		s.logger.Warn("failed to record password change audit log", zap.Error(err))
	}

	return nil
}

// ValidateToken parses and validates an access token returning the claims.
func (s *AuthService) ValidateToken(tokenString string) (*models.JWTClaims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &models.JWTClaims{}, func(token *jwt.Token) (interface{}, error) {
		if token.Method != jwt.SigningMethodHS256 {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return []byte(s.config.AccessTokenSecret), nil
	})
	if err != nil {
		return nil, appErrors.Wrap(err, appErrors.ErrUnauthorized.Code, appErrors.ErrUnauthorized.Status, "invalid token")
	}

	claims, ok := token.Claims.(*models.JWTClaims)
	if !ok || !token.Valid {
		return nil, appErrors.Clone(appErrors.ErrUnauthorized, "invalid token claims")
	}

	return claims, nil
}

// ForgotPassword initiates the reset flow. Phase 1 stub.
func (s *AuthService) ForgotPassword(ctx context.Context, req models.ResetPasswordRequest) error {
	if err := s.validator.Struct(req); err != nil {
		return appErrors.Wrap(err, appErrors.ErrValidation.Code, appErrors.ErrValidation.Status, "invalid forgot password payload")
	}
	s.logger.Info("password reset requested", zap.String("email", req.Email))
	return nil
}

// ResetPassword completes the reset flow. Phase 1 stub.
func (s *AuthService) ResetPassword(ctx context.Context, req models.ConfirmResetPasswordRequest) error {
	if err := s.validator.Struct(req); err != nil {
		return appErrors.Wrap(err, appErrors.ErrValidation.Code, appErrors.ErrValidation.Status, "invalid reset password payload")
	}
	s.logger.Info("reset password token consumed", zap.String("token", req.Token))
	return nil
}

func (s *AuthService) generateAccessToken(user *models.User) (string, time.Time, error) {
	issuedAt := time.Now().UTC()
	expiresAt := issuedAt.Add(s.config.AccessTokenExpiry)
	claims := &models.JWTClaims{
		UserID:   user.ID,
		Role:     user.Role,
		Email:    user.Email,
		FullName: user.FullName,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    s.config.Issuer,
			Subject:   user.ID,
			Audience:  s.config.Audience,
			ExpiresAt: jwt.NewNumericDate(expiresAt),
			IssuedAt:  jwt.NewNumericDate(issuedAt),
			NotBefore: jwt.NewNumericDate(issuedAt),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString([]byte(s.config.AccessTokenSecret))
	if err != nil {
		return "", time.Time{}, err
	}
	return signed, expiresAt, nil
}

func (s *AuthService) generateRefreshTokenString() (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}
