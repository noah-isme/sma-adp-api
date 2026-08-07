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

// PortalAuthConfig defines configuration for portal authentication flows.
type PortalAuthConfig struct {
	AccessTokenSecret  string
	AccessTokenExpiry  time.Duration
	RefreshTokenExpiry time.Duration
	Issuer             string
	Audience           []string
}

// PortalUserLookup defines the interface for looking up portal-specific user data.
type PortalUserLookup interface {
	FindStudentByUserID(ctx context.Context, userID string) (*models.StudentDetail, error)
	FindStudentByID(ctx context.Context, studentID string) (*models.StudentDetail, error)
	FindParentLinksByParentID(ctx context.Context, parentID string) ([]*models.ParentStudentLink, error)
	FindPortalPreferences(ctx context.Context, userID string) (*models.PortalPreferences, error)
	FindDeviceTokensByUserID(ctx context.Context, userID string) ([]*models.DeviceToken, error)
	// Parent-student link management
	FindParentStudentLinkByID(ctx context.Context, id string) (*models.ParentStudentLink, error)
	CreateParentStudentLink(ctx context.Context, link *models.ParentStudentLink) error
	UpdateParentStudentLink(ctx context.Context, link *models.ParentStudentLink) error
	DeleteParentStudentLink(ctx context.Context, id string) error
}

// PortalAuthService provides authentication use cases for parent/student portal.
type PortalAuthService struct {
	userRepo        authUserRepository
	portalLookup    PortalUserLookup
	validator       *validator.Validate
	logger          *zap.Logger
	config          PortalAuthConfig
}

// NewPortalAuthService constructs a PortalAuthService instance.
func NewPortalAuthService(
	userRepo authUserRepository,
	portalLookup PortalUserLookup,
	validate *validator.Validate,
	logger *zap.Logger,
	config PortalAuthConfig,
) *PortalAuthService {
	if logger == nil {
		logger = zap.NewNop()
	}
	if validate == nil {
		validate = validator.New()
	}
	return &PortalAuthService{
		userRepo:     userRepo,
		portalLookup: portalLookup,
		validator:    validate,
		logger:       logger,
		config:       config,
	}
}

// PortalLogin authenticates a parent or student user and returns issued tokens with portal info.
func (s *PortalAuthService) PortalLogin(ctx context.Context, req models.PortalLoginRequest) (*models.PortalLoginResponse, error) {
	if err := s.validator.Struct(req); err != nil {
		return nil, appErrors.Wrap(err, appErrors.ErrValidation.Code, appErrors.ErrValidation.Status, "invalid login payload")
	}

	user, err := s.userRepo.FindByEmail(ctx, req.Email)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, appErrors.Clone(appErrors.ErrInvalidCredentials, "invalid email or password")
		}
		return nil, appErrors.Wrap(err, appErrors.ErrInternal.Code, appErrors.ErrInternal.Status, "failed to fetch user")
	}

	// Only allow ORTU (PARENT) and SISWA (STUDENT) roles
	if user.Role != models.RoleOrtu && user.Role != models.RoleSiswa {
		return nil, appErrors.Clone(appErrors.ErrForbidden, "portal access restricted to parents and students")
	}

	if !user.Active {
		return nil, appErrors.Clone(appErrors.ErrInactiveAccount, "account is inactive")
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password)); err != nil {
		return nil, appErrors.Clone(appErrors.ErrInvalidCredentials, "invalid email or password")
	}

	// Build portal user info
	portalUser, err := s.buildPortalUserInfo(ctx, user)
	if err != nil {
		s.logger.Warn("failed to build portal user info", zap.Error(err))
	}

	accessToken, _, err := s.generatePortalAccessToken(user, portalUser.StudentID)
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

	if err := s.userRepo.CreateRefreshToken(ctx, refreshToken); err != nil {
		return nil, appErrors.Wrap(err, appErrors.ErrInternal.Code, appErrors.ErrInternal.Status, "failed to persist refresh token")
	}

	if err := s.userRepo.UpdateLastLogin(ctx, user.ID, time.Now().UTC()); err != nil {
		s.logger.Warn("failed to update last login", zap.Error(err))
	}

	if err := s.userRepo.CreateAuditLog(ctx, &models.AuditLog{
		UserID:     &user.ID,
		Action:     models.AuditActionLogin,
		Resource:   "portal_auth",
		ResourceID: &user.ID,
		NewValues:  []byte(`{"status":"success"}`),
		IPAddress:  req.IP,
		UserAgent:  req.UserAgent,
	}); err != nil {
		s.logger.Warn("failed to record login audit log", zap.Error(err))
	}

	return &models.PortalLoginResponse{
		AccessToken:  accessToken,
		RefreshToken: refreshToken.Token,
		User:         portalUser,
	}, nil
}

// PortalRefreshToken exchanges a refresh token for a new access token pair.
func (s *PortalAuthService) PortalRefreshToken(ctx context.Context, req models.RefreshTokenRequest) (*models.PortalLoginResponse, error) {
	if err := s.validator.Struct(req); err != nil {
		return nil, appErrors.Wrap(err, appErrors.ErrValidation.Code, appErrors.ErrValidation.Status, "invalid refresh payload")
	}

	storedToken, err := s.userRepo.FindRefreshToken(ctx, req.RefreshToken)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, appErrors.Clone(appErrors.ErrUnauthorized, "refresh token not found")
		}
		return nil, appErrors.Wrap(err, appErrors.ErrInternal.Code, appErrors.ErrInternal.Status, "failed to fetch refresh token")
	}

	if storedToken.Revoked || time.Now().UTC().After(storedToken.ExpiresAt) {
		return nil, appErrors.Clone(appErrors.ErrUnauthorized, "refresh token is expired or revoked")
	}

	user, err := s.userRepo.FindByID(ctx, storedToken.UserID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, appErrors.Clone(appErrors.ErrUnauthorized, "associated user no longer exists")
		}
		return nil, appErrors.Wrap(err, appErrors.ErrInternal.Code, appErrors.ErrInternal.Status, "failed to load user")
	}

	if !user.Active {
		return nil, appErrors.Clone(appErrors.ErrInactiveAccount, "account is inactive")
	}

	// Only allow ORTU and SISWA roles
	if user.Role != models.RoleOrtu && user.Role != models.RoleSiswa {
		return nil, appErrors.Clone(appErrors.ErrForbidden, "portal access restricted to parents and students")
	}

	if err := s.userRepo.RevokeRefreshToken(ctx, storedToken.ID, time.Now().UTC()); err != nil {
		s.logger.Warn("failed to revoke used refresh token", zap.Error(err))
	}

	portalUser, err := s.buildPortalUserInfo(ctx, user)
	if err != nil {
		s.logger.Warn("failed to build portal user info", zap.Error(err))
	}

	accessToken, _, err := s.generatePortalAccessToken(user, portalUser.StudentID)
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

	if err := s.userRepo.CreateRefreshToken(ctx, newRefresh); err != nil {
		return nil, appErrors.Wrap(err, appErrors.ErrInternal.Code, appErrors.ErrInternal.Status, "failed to persist refresh token")
	}

	if err := s.userRepo.CreateAuditLog(ctx, &models.AuditLog{
		UserID:     &user.ID,
		Action:     models.AuditActionLogin,
		Resource:   "portal_auth",
		ResourceID: &user.ID,
		NewValues:  []byte(`{"refresh":"rotated"}`),
		IPAddress:  req.IP,
		UserAgent:  req.UserAgent,
	}); err != nil {
		s.logger.Warn("failed to record refresh audit log", zap.Error(err))
	}

	return &models.PortalLoginResponse{
		AccessToken:  accessToken,
		RefreshToken: newRefresh.Token,
		User:         portalUser,
	}, nil
}

// PortalLogout revokes the provided refresh token.
func (s *PortalAuthService) PortalLogout(ctx context.Context, refreshToken string, userID string, meta models.PortalLoginRequest) error {
	storedToken, err := s.userRepo.FindRefreshToken(ctx, refreshToken)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return appErrors.Clone(appErrors.ErrUnauthorized, "refresh token not found")
		}
		return appErrors.Wrap(err, appErrors.ErrInternal.Code, appErrors.ErrInternal.Status, "failed to load refresh token")
	}

	if storedToken.UserID != userID {
		return appErrors.Clone(appErrors.ErrForbidden, "token does not belong to user")
	}

	if err := s.userRepo.RevokeRefreshToken(ctx, storedToken.ID, time.Now().UTC()); err != nil {
		return appErrors.Wrap(err, appErrors.ErrInternal.Code, appErrors.ErrInternal.Status, "failed to revoke refresh token")
	}

	if err := s.userRepo.CreateAuditLog(ctx, &models.AuditLog{
		UserID:     &userID,
		Action:     models.AuditActionLogout,
		Resource:   "portal_auth",
		ResourceID: &userID,
		NewValues:  []byte(`{"status":"logout"}`),
		IPAddress:  meta.IP,
		UserAgent:  meta.UserAgent,
	}); err != nil {
		s.logger.Warn("failed to record logout audit log", zap.Error(err))
	}

	return nil
}

// PortalForgotPassword initiates the reset flow for portal users.
func (s *PortalAuthService) PortalForgotPassword(ctx context.Context, req models.PortalForgotPasswordRequest) error {
	if err := s.validator.Struct(req); err != nil {
		return appErrors.Wrap(err, appErrors.ErrValidation.Code, appErrors.ErrValidation.Status, "invalid forgot password payload")
	}
	s.logger.Info("portal password reset requested", zap.String("email", req.Email))
	// TODO: Implement email sending with reset token
	return nil
}

// PortalResetPassword completes the reset flow for portal users.
func (s *PortalAuthService) PortalResetPassword(ctx context.Context, req models.PortalResetPasswordRequest) error {
	if err := s.validator.Struct(req); err != nil {
		return appErrors.Wrap(err, appErrors.ErrValidation.Code, appErrors.ErrValidation.Status, "invalid reset password payload")
	}
	s.logger.Info("portal reset password token consumed", zap.String("token", req.Token))
	// TODO: Implement token validation and password reset
	return nil
}

// GetPortalProfile returns the full portal profile for a user.
func (s *PortalAuthService) GetPortalProfile(ctx context.Context, userID string) (*models.PortalProfile, error) {
	user, err := s.userRepo.FindByID(ctx, userID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, appErrors.Clone(appErrors.ErrNotFound, "user not found")
		}
		return nil, appErrors.Wrap(err, appErrors.ErrInternal.Code, appErrors.ErrInternal.Status, "failed to load user")
	}

	if user.Role != models.RoleOrtu && user.Role != models.RoleSiswa {
		return nil, appErrors.Clone(appErrors.ErrForbidden, "portal access restricted to parents and students")
	}

	portalUser, err := s.buildPortalUserInfo(ctx, user)
	if err != nil {
		return nil, err
	}

	prefs, _ := s.portalLookup.FindPortalPreferences(ctx, userID)
	deviceTokens, _ := s.portalLookup.FindDeviceTokensByUserID(ctx, userID)

	return &models.PortalProfile{
		User:         portalUser,
		Preferences:  prefs,
		DeviceTokens: deviceTokens,
	}, nil
}

// UpdatePortalPreferences updates the portal preferences for a user.
func (s *PortalAuthService) UpdatePortalPreferences(ctx context.Context, userID string, req models.UpdatePortalPreferencesRequest) (*models.PortalPreferences, error) {
	user, err := s.userRepo.FindByID(ctx, userID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, appErrors.Clone(appErrors.ErrNotFound, "user not found")
		}
		return nil, appErrors.Wrap(err, appErrors.ErrInternal.Code, appErrors.ErrInternal.Status, "failed to load user")
	}

	if user.Role != models.RoleOrtu && user.Role != models.RoleSiswa {
		return nil, appErrors.Clone(appErrors.ErrForbidden, "portal access restricted to parents and students")
	}

	existing, err := s.portalLookup.FindPortalPreferences(ctx, userID)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, appErrors.Wrap(err, appErrors.ErrInternal.Code, appErrors.ErrInternal.Status, "failed to load preferences")
	}

	now := time.Now().UTC()
	var prefs *models.PortalPreferences
	if existing == nil {
		prefs = &models.PortalPreferences{
			UserID:           userID,
			Language:         "id",
			Timezone:         "Asia/Jakarta",
			EmailNotifications: true,
			PushNotifications:  true,
			SMSNotifications:   false,
			GradeAlerts:        true,
			AttendanceAlerts:   true,
			BehaviorAlerts:     true,
			AnnouncementAlerts: true,
			CreatedAt:          now,
			UpdatedAt:          now,
		}
	} else {
		prefs = existing
		prefs.UpdatedAt = now
	}

	// Apply updates
	if req.Language != nil {
		prefs.Language = *req.Language
	}
	if req.Timezone != nil {
		prefs.Timezone = *req.Timezone
	}
	if req.EmailNotifications != nil {
		prefs.EmailNotifications = *req.EmailNotifications
	}
	if req.PushNotifications != nil {
		prefs.PushNotifications = *req.PushNotifications
	}
	if req.SMSNotifications != nil {
		prefs.SMSNotifications = *req.SMSNotifications
	}
	if req.GradeAlerts != nil {
		prefs.GradeAlerts = *req.GradeAlerts
	}
	if req.AttendanceAlerts != nil {
		prefs.AttendanceAlerts = *req.AttendanceAlerts
	}
	if req.BehaviorAlerts != nil {
		prefs.BehaviorAlerts = *req.BehaviorAlerts
	}
	if req.AnnouncementAlerts != nil {
		prefs.AnnouncementAlerts = *req.AnnouncementAlerts
	}

	// Upsert to repository
	// Note: Repository upsert logic will be called from handler/service composition
	return prefs, nil
}

// RegisterDeviceToken registers a new device token for push notifications.
func (s *PortalAuthService) RegisterDeviceToken(ctx context.Context, userID string, req models.RegisterDeviceTokenRequest) (*models.DeviceToken, error) {
	if err := s.validator.Struct(req); err != nil {
		return nil, appErrors.Wrap(err, appErrors.ErrValidation.Code, appErrors.ErrValidation.Status, "invalid device token payload")
	}

	user, err := s.userRepo.FindByID(ctx, userID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, appErrors.Clone(appErrors.ErrNotFound, "user not found")
		}
		return nil, appErrors.Wrap(err, appErrors.ErrInternal.Code, appErrors.ErrInternal.Status, "failed to load user")
	}

	if user.Role != models.RoleOrtu && user.Role != models.RoleSiswa {
		return nil, appErrors.Clone(appErrors.ErrForbidden, "portal access restricted to parents and students")
	}

	// Check if token already exists
	existing, err := s.portalLookup.FindDeviceTokensByUserID(ctx, userID)
	if err == nil {
		for _, t := range existing {
			if t.Token == req.Token {
				// Update last used
				now := time.Now().UTC()
				t.LastUsedAt = now
				return t, nil
			}
		}
	}

	now := time.Now().UTC()
	token := &models.DeviceToken{
		ID:         uuid.NewString(),
		UserID:     userID,
		Token:      req.Token,
		Platform:   req.Platform,
		DeviceID:   req.DeviceID,
		AppVersion: req.AppVersion,
		LastUsedAt: now,
		CreatedAt:  now,
	}

	return token, nil
}

// UnregisterDeviceToken removes a device token.
func (s *PortalAuthService) UnregisterDeviceToken(ctx context.Context, userID string, tokenID string) error {
	user, err := s.userRepo.FindByID(ctx, userID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return appErrors.Clone(appErrors.ErrNotFound, "user not found")
		}
		return appErrors.Wrap(err, appErrors.ErrInternal.Code, appErrors.ErrInternal.Status, "failed to load user")
	}

	if user.Role != models.RoleOrtu && user.Role != models.RoleSiswa {
		return appErrors.Clone(appErrors.ErrForbidden, "portal access restricted to parents and students")
	}

	// Note: Actual deletion is done via repository in handler
	return nil
}

// buildPortalUserInfo builds the portal user info with linked students for parents.
func (s *PortalAuthService) buildPortalUserInfo(ctx context.Context, user *models.User) (*models.PortalUserInfo, error) {
	info := &models.PortalUserInfo{
		ID:         user.ID,
		Email:      user.Email,
		FullName:   user.FullName,
		Role:       user.Role,
		PortalRole: user.Role,
	}

	if user.Role == models.RoleSiswa {
		// For students, get their student record
		student, err := s.portalLookup.FindStudentByUserID(ctx, user.ID)
		if err == nil && student != nil {
			info.StudentID = &student.ID
		}
	} else if user.Role == models.RoleOrtu {
		// For parents, get linked students
		links, err := s.portalLookup.FindParentLinksByParentID(ctx, user.ID)
		if err == nil && len(links) > 0 {
			students := make([]models.StudentSummary, 0, len(links))
			for _, link := range links {
				// TODO: Fetch student details for each link
				// This will be populated when the student service is integrated
				students = append(students, models.StudentSummary{
					ID: link.StudentID,
				})
			}
			info.LinkedStudents = students
		}
	}

	return info, nil
}

// generatePortalAccessToken generates a JWT access token for portal users.
func (s *PortalAuthService) generatePortalAccessToken(user *models.User, studentID *string) (string, time.Time, error) {
	issuedAt := time.Now().UTC()
	expiresAt := issuedAt.Add(s.config.AccessTokenExpiry)
	
	claims := &models.JWTClaims{
		UserID:    user.ID,
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

	// Add student_id to claims if available (for parent access control)
	if studentID != nil {
		// We'll use TeacherID field for student_id in portal context
		claims.TeacherID = *studentID
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString([]byte(s.config.AccessTokenSecret))
	if err != nil {
		return "", time.Time{}, err
	}
	return signed, expiresAt, nil
}

func (s *PortalAuthService) generateRefreshTokenString() (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}

// ValidateToken parses and validates a portal access token returning the claims.
func (s *PortalAuthService) ValidateToken(tokenString string) (*models.JWTClaims, error) {
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

// GetLinkedStudents returns all students linked to a parent.
func (s *PortalAuthService) GetLinkedStudents(ctx context.Context, parentID string) ([]*models.ParentStudentLink, error) {
	user, err := s.userRepo.FindByID(ctx, parentID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, appErrors.Clone(appErrors.ErrNotFound, "user not found")
		}
		return nil, appErrors.Wrap(err, appErrors.ErrInternal.Code, appErrors.ErrInternal.Status, "failed to load user")
	}

	if user.Role != models.RoleOrtu {
		return nil, appErrors.Clone(appErrors.ErrForbidden, "only parents can have linked students")
	}

	links, err := s.portalLookup.FindParentLinksByParentID(ctx, parentID)
	if err != nil {
		return nil, appErrors.Wrap(err, appErrors.ErrInternal.Code, appErrors.ErrInternal.Status, "failed to fetch linked students")
	}

	return links, nil
}

// CreateParentStudentLink creates a new parent-student link.
func (s *PortalAuthService) CreateParentStudentLink(ctx context.Context, parentID string, req models.CreateParentStudentLinkRequest) (*models.ParentStudentLink, error) {
	if err := s.validator.Struct(req); err != nil {
		return nil, appErrors.Wrap(err, appErrors.ErrValidation.Code, appErrors.ErrValidation.Status, "invalid link payload")
	}

	// Verify parent exists and is a parent
	parent, err := s.userRepo.FindByID(ctx, parentID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, appErrors.Clone(appErrors.ErrNotFound, "parent not found")
		}
		return nil, appErrors.Wrap(err, appErrors.ErrInternal.Code, appErrors.ErrInternal.Status, "failed to load parent")
	}

	if parent.Role != models.RoleOrtu {
		return nil, appErrors.Clone(appErrors.ErrForbidden, "user is not a parent")
	}

	// Verify student exists
	student, err := s.portalLookup.FindStudentByID(ctx, req.StudentID)
	if err != nil || student == nil {
		return nil, appErrors.Clone(appErrors.ErrNotFound, "student not found")
	}

	// Check if link already exists
	existing, err := s.portalLookup.FindParentLinksByParentID(ctx, parentID)
	if err == nil {
		for _, link := range existing {
			if link.StudentID == req.StudentID {
				return nil, appErrors.Clone(appErrors.ErrConflict, "parent-student link already exists")
			}
		}
	}

	// Set defaults
	relationship := req.Relationship
	if relationship == "" {
		relationship = models.RelationshipParent
	}

	link := &models.ParentStudentLink{
		ParentID:                 parentID,
		StudentID:                req.StudentID,
		Relationship:             relationship,
		CanViewGrades:            req.CanViewGrades,
		CanViewAttendance:        req.CanViewAttendance,
		CanViewBehavior:          req.CanViewBehavior,
		CanViewAnnouncements:     req.CanViewAnnouncements,
		CanReceiveNotifications:  req.CanReceiveNotifications,
	}

	if err := s.portalLookup.CreateParentStudentLink(ctx, link); err != nil {
		return nil, appErrors.Wrap(err, appErrors.ErrInternal.Code, appErrors.ErrInternal.Status, "failed to create parent-student link")
	}

	return link, nil
}

// UpdateParentStudentLink updates a parent-student link.
func (s *PortalAuthService) UpdateParentStudentLink(ctx context.Context, parentID, linkID string, req models.UpdateParentStudentLinkRequest) (*models.ParentStudentLink, error) {
	if err := s.validator.Struct(req); err != nil {
		return nil, appErrors.Wrap(err, appErrors.ErrValidation.Code, appErrors.ErrValidation.Status, "invalid update payload")
	}

	// Verify parent
	parent, err := s.userRepo.FindByID(ctx, parentID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, appErrors.Clone(appErrors.ErrNotFound, "parent not found")
		}
		return nil, appErrors.Wrap(err, appErrors.ErrInternal.Code, appErrors.ErrInternal.Status, "failed to load parent")
	}

	if parent.Role != models.RoleOrtu {
		return nil, appErrors.Clone(appErrors.ErrForbidden, "user is not a parent")
	}

	// Fetch existing link
	link, err := s.portalLookup.FindParentStudentLinkByID(ctx, linkID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, appErrors.Clone(appErrors.ErrNotFound, "parent-student link not found")
		}
		return nil, appErrors.Wrap(err, appErrors.ErrInternal.Code, appErrors.ErrInternal.Status, "failed to load link")
	}

	// Verify ownership
	if link.ParentID != parentID {
		return nil, appErrors.Clone(appErrors.ErrForbidden, "link does not belong to parent")
	}

	// Apply updates
	if req.Relationship != nil {
		link.Relationship = *req.Relationship
	}
	if req.CanViewGrades != nil {
		link.CanViewGrades = *req.CanViewGrades
	}
	if req.CanViewAttendance != nil {
		link.CanViewAttendance = *req.CanViewAttendance
	}
	if req.CanViewBehavior != nil {
		link.CanViewBehavior = *req.CanViewBehavior
	}
	if req.CanViewAnnouncements != nil {
		link.CanViewAnnouncements = *req.CanViewAnnouncements
	}
	if req.CanReceiveNotifications != nil {
		link.CanReceiveNotifications = *req.CanReceiveNotifications
	}

	if err := s.portalLookup.UpdateParentStudentLink(ctx, link); err != nil {
		return nil, appErrors.Wrap(err, appErrors.ErrInternal.Code, appErrors.ErrInternal.Status, "failed to update parent-student link")
	}

	return link, nil
}

// DeleteParentStudentLink removes a parent-student link.
func (s *PortalAuthService) DeleteParentStudentLink(ctx context.Context, parentID, linkID string) error {
	// Verify parent
	parent, err := s.userRepo.FindByID(ctx, parentID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return appErrors.Clone(appErrors.ErrNotFound, "parent not found")
		}
		return appErrors.Wrap(err, appErrors.ErrInternal.Code, appErrors.ErrInternal.Status, "failed to load parent")
	}

	if parent.Role != models.RoleOrtu {
		return appErrors.Clone(appErrors.ErrForbidden, "user is not a parent")
	}

	// Fetch existing link to verify ownership
	link, err := s.portalLookup.FindParentStudentLinkByID(ctx, linkID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return appErrors.Clone(appErrors.ErrNotFound, "parent-student link not found")
		}
		return appErrors.Wrap(err, appErrors.ErrInternal.Code, appErrors.ErrInternal.Status, "failed to load link")
	}

	// Verify ownership
	if link.ParentID != parentID {
		return appErrors.Clone(appErrors.ErrForbidden, "link does not belong to parent")
	}

	if err := s.portalLookup.DeleteParentStudentLink(ctx, linkID); err != nil {
		return appErrors.Wrap(err, appErrors.ErrInternal.Code, appErrors.ErrInternal.Status, "failed to delete parent-student link")
	}

	return nil
}