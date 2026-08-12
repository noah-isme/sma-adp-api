package service

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/go-playground/validator/v10"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/jmoiron/sqlx/types"
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
	CreatePasswordResetToken(ctx context.Context, token *models.PasswordResetToken) error
	ConsumePasswordResetToken(ctx context.Context, tokenHash string, usedAt time.Time) (*models.PasswordResetToken, error)
	CreateAuditLog(ctx context.Context, log *models.AuditLog) error
}

type authTeacherLookup interface {
	FindByUserID(ctx context.Context, userID string) (*models.Teacher, error)
}

// AuthConfig defines configuration for authentication flows.
type AuthConfig struct {
	AccessTokenSecret     string
	AccessTokenExpiry     time.Duration
	RefreshTokenExpiry    time.Duration
	PasswordResetTokenTTL time.Duration
	PasswordResetURL      string
	Issuer                string
	Audience              []string
	SingleSession         bool
}

// AuthService provides authentication use cases.
type AuthService struct {
	repo          authUserRepository
	teacherLookup authTeacherLookup
	validator     *validator.Validate
	logger        *zap.Logger
	config        AuthConfig
	emailDelivery PasswordResetEmailDelivery
}

// NewAuthService constructs an AuthService instance.
func NewAuthService(repo authUserRepository, teacherLookup authTeacherLookup, validate *validator.Validate, logger *zap.Logger, config AuthConfig) *AuthService {
	return NewAuthServiceWithEmailDelivery(repo, teacherLookup, validate, logger, config, NoopPasswordResetEmailDelivery{})
}

// NewAuthServiceWithEmailDelivery constructs an AuthService with an injected
// password-reset delivery implementation. The default constructor remains
// network-free for tests and callers that do not configure email delivery.
func NewAuthServiceWithEmailDelivery(repo authUserRepository, teacherLookup authTeacherLookup, validate *validator.Validate, logger *zap.Logger, config AuthConfig, emailDelivery PasswordResetEmailDelivery) *AuthService {
	if logger == nil {
		logger = zap.NewNop()
	}
	if validate == nil {
		validate = validator.New()
	}
	if emailDelivery == nil {
		emailDelivery = NoopPasswordResetEmailDelivery{}
	}
	if config.PasswordResetTokenTTL <= 0 {
		config.PasswordResetTokenTTL = time.Hour
	}
	if strings.TrimSpace(config.PasswordResetURL) == "" {
		config.PasswordResetURL = "http://localhost:5173/admin/reset-password"
	}
	return &AuthService{
		repo:          repo,
		teacherLookup: teacherLookup,
		validator:     validate,
		logger:        logger,
		config:        config,
		emailDelivery: emailDelivery,
	}
}

// RefreshTokenExpiry exposes the configured refresh-token lifetime to the HTTP
// handler so the browser cookie expires no later than the persisted session.
// Token issuance and validation remain owned by AuthService.
func (s *AuthService) RefreshTokenExpiry() time.Duration {
	return s.config.RefreshTokenExpiry
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

	teacherID := s.resolveTeacherID(ctx, user)

	accessToken, _, err := s.generateAccessToken(user, teacherID)
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
		NewValues:  types.JSONText(`{"status":"success"}`),
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

	teacherID := s.resolveTeacherID(ctx, user)

	accessToken, _, err := s.generateAccessToken(user, teacherID)
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
		NewValues:  types.JSONText(`{"refresh":"rotated"}`),
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

// Logout revokes the provided refresh token after confirming that it belongs
// to the authenticated access-token subject.
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

	return s.revokeLogoutToken(ctx, storedToken, userID, meta)
}

// LogoutByRefreshToken revokes a browser session using only its HttpOnly
// refresh cookie. Logout is deliberately idempotent for this path: an absent,
// expired, or already-revoked cookie is treated as a completed logout. The
// cookie is not accepted as an authentication credential anywhere else.
func (s *AuthService) LogoutByRefreshToken(ctx context.Context, refreshToken string, meta models.LoginRequest) error {
	if refreshToken == "" {
		return nil
	}

	storedToken, err := s.repo.FindRefreshToken(ctx, refreshToken)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil
		}
		return appErrors.Wrap(err, appErrors.ErrInternal.Code, appErrors.ErrInternal.Status, "failed to load refresh token")
	}

	return s.revokeLogoutToken(ctx, storedToken, storedToken.UserID, meta)
}

func (s *AuthService) revokeLogoutToken(ctx context.Context, storedToken *models.RefreshToken, userID string, meta models.LoginRequest) error {
	if storedToken == nil {
		return nil
	}

	if err := s.repo.RevokeRefreshToken(ctx, storedToken.ID, time.Now().UTC()); err != nil {
		return appErrors.Wrap(err, appErrors.ErrInternal.Code, appErrors.ErrInternal.Status, "failed to revoke refresh token")
	}

	if err := s.repo.CreateAuditLog(ctx, &models.AuditLog{
		UserID:     &userID,
		Action:     models.AuditActionLogout,
		Resource:   "auth",
		ResourceID: &userID,
		NewValues:  types.JSONText(`{"status":"logout"}`),
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
		NewValues:  types.JSONText(`{"status":"changed"}`),
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

// ForgotPassword initiates the admin reset flow. User lookup failures for a
// missing email intentionally result in success so the endpoint cannot be
// used to enumerate accounts.
func (s *AuthService) ForgotPassword(ctx context.Context, req models.ResetPasswordRequest) error {
	if err := s.validator.Struct(req); err != nil {
		return appErrors.Wrap(err, appErrors.ErrValidation.Code, appErrors.ErrValidation.Status, "invalid forgot password payload")
	}

	email := strings.ToLower(strings.TrimSpace(req.Email))
	user, err := s.repo.FindByEmail(ctx, email)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil
		}
		return appErrors.Wrap(err, appErrors.ErrInternal.Code, appErrors.ErrInternal.Status, "failed to look up password reset account")
	}
	if user == nil || !user.Active {
		return nil
	}

	rawToken, err := generatePasswordResetToken()
	if err != nil {
		return appErrors.Wrap(err, appErrors.ErrInternal.Code, appErrors.ErrInternal.Status, "failed to generate password reset token")
	}
	now := time.Now().UTC()
	expiresAt := now.Add(s.config.PasswordResetTokenTTL)
	resetToken := &models.PasswordResetToken{
		ID:        uuid.NewString(),
		UserID:    user.ID,
		TokenHash: hashPasswordResetToken(rawToken),
		ExpiresAt: expiresAt,
		CreatedAt: now,
	}
	if err := s.repo.CreatePasswordResetToken(ctx, resetToken); err != nil {
		return appErrors.Wrap(err, appErrors.ErrInternal.Code, appErrors.ErrInternal.Status, "failed to persist password reset token")
	}

	resetURL := addPasswordResetToken(s.config.PasswordResetURL, rawToken)
	if err := s.emailDelivery.SendPasswordReset(ctx, user.Email, resetURL, expiresAt); err != nil {
		// Keep the public response generic even if a configured provider fails;
		// otherwise delivery errors would reveal that the account exists.
		s.logger.Error("failed to deliver password reset email", zap.Error(err), zap.String("user_id", user.ID))
	}

	s.logger.Info("password reset requested", zap.String("user_id", user.ID))
	return nil
}

// ResetPassword atomically consumes a valid reset token before updating the
// bcrypt password hash. The repository's conditional UPDATE makes a token
// single-use even when two reset requests race.
func (s *AuthService) ResetPassword(ctx context.Context, req models.ConfirmResetPasswordRequest) error {
	if err := s.validator.Struct(req); err != nil {
		return appErrors.Wrap(err, appErrors.ErrValidation.Code, appErrors.ErrValidation.Status, "invalid reset password payload")
	}

	resetToken, err := s.repo.ConsumePasswordResetToken(ctx, hashPasswordResetToken(req.Token), time.Now().UTC())
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return appErrors.Clone(appErrors.ErrUnauthorized, "invalid or expired password reset token")
		}
		return appErrors.Wrap(err, appErrors.ErrInternal.Code, appErrors.ErrInternal.Status, "failed to consume password reset token")
	}

	newHash, err := bcrypt.GenerateFromPassword([]byte(req.NewPassword), bcrypt.DefaultCost)
	if err != nil {
		return appErrors.Wrap(err, appErrors.ErrInternal.Code, appErrors.ErrInternal.Status, "failed to hash password")
	}
	if err := s.repo.UpdatePassword(ctx, resetToken.UserID, string(newHash), time.Now().UTC()); err != nil {
		return appErrors.Wrap(err, appErrors.ErrInternal.Code, appErrors.ErrInternal.Status, "failed to update password")
	}
	if err := s.repo.RevokeUserRefreshTokens(ctx, resetToken.UserID); err != nil {
		s.logger.Warn("failed to revoke refresh tokens after password reset", zap.Error(err))
	}

	if err := s.repo.CreateAuditLog(ctx, &models.AuditLog{
		UserID:     &resetToken.UserID,
		Action:     models.AuditActionPasswordReset,
		Resource:   "auth",
		ResourceID: &resetToken.UserID,
		NewValues:  types.JSONText(`{"status":"reset"}`),
	}); err != nil {
		s.logger.Warn("failed to record password reset audit log", zap.Error(err))
	}

	return nil
}

const passwordResetTokenBytes = 32

func generatePasswordResetToken() (string, error) {
	buf := make([]byte, passwordResetTokenBytes)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}

func hashPasswordResetToken(token string) string {
	digest := sha256.Sum256([]byte(token))
	return hex.EncodeToString(digest[:])
}

func addPasswordResetToken(baseURL, rawToken string) string {
	separator := "?"
	if strings.Contains(baseURL, "?") {
		separator = "&"
	}
	return baseURL + separator + url.Values{"token": []string{rawToken}}.Encode()
}

func (s *AuthService) resolveTeacherID(ctx context.Context, user *models.User) string {
	// resolveTeacherID finds the teacher ID linked to a user.
	// Returns empty string if user is not a teacher or lookup fails.
	// This is called at login/refresh to populate the TeacherID claim in JWT.
	// Services should use claims.TeacherID directly; fallback to UserID is temporary
	// for backward compatibility with tokens issued before migration 000014.
	if user.Role != models.RoleTeacher || s.teacherLookup == nil {
		return ""
	}
	teacher, err := s.teacherLookup.FindByUserID(ctx, user.ID)
	if err != nil || teacher == nil {
		return ""
	}
	return teacher.ID
}

func (s *AuthService) generateAccessToken(user *models.User, teacherID string) (string, time.Time, error) {
	issuedAt := time.Now().UTC()
	expiresAt := issuedAt.Add(s.config.AccessTokenExpiry)
	claims := &models.JWTClaims{
		UserID:    user.ID,
		TeacherID: teacherID,
		Role:      user.Role,
		Email:     user.Email,
		FullName:  user.FullName,
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
