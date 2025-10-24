package service

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"strings"

	"github.com/go-playground/validator/v10"
	"github.com/google/uuid"
	"go.uber.org/zap"
	"golang.org/x/crypto/bcrypt"

	"github.com/noah-isme/sma-adp-api/internal/models"
	appErrors "github.com/noah-isme/sma-adp-api/pkg/errors"
)

type userRepository interface {
	List(ctx context.Context, filter models.UserFilter) ([]models.User, int, error)
	FindByID(ctx context.Context, id string) (*models.User, error)
	FindByEmail(ctx context.Context, email string) (*models.User, error)
	Create(ctx context.Context, user *models.User) error
	Update(ctx context.Context, user *models.User) error
	Delete(ctx context.Context, id string) error
	CreateAuditLog(ctx context.Context, log *models.AuditLog) error
}

// CreateUserRequest represents payload for creating users.
type CreateUserRequest struct {
	Email    string          `json:"email" validate:"required,email"`
	FullName string          `json:"full_name" validate:"required"`
	Role     models.UserRole `json:"role" validate:"required,oneof=SUPERADMIN ADMIN TEACHER STUDENT"`
	Active   bool            `json:"active"`
	Password string          `json:"password" validate:"required,min=6"`
}

// UpdateUserRequest payload for updating users.
type UpdateUserRequest struct {
	FullName string          `json:"full_name" validate:"required"`
	Role     models.UserRole `json:"role" validate:"required,oneof=SUPERADMIN ADMIN TEACHER STUDENT"`
	Active   *bool           `json:"active"`
}

// UserService handles user management workflows.
type UserService struct {
	repo      userRepository
	validator *validator.Validate
	logger    *zap.Logger
}

// NewUserService creates an instance of UserService.
func NewUserService(repo userRepository, validate *validator.Validate, logger *zap.Logger) *UserService {
	if logger == nil {
		logger = zap.NewNop()
	}
	if validate == nil {
		validate = validator.New()
	}
	return &UserService{repo: repo, validator: validate, logger: logger}
}

// List returns paginated users and pagination metadata.
func (s *UserService) List(ctx context.Context, filter models.UserFilter) ([]models.User, *models.Pagination, error) {
	users, total, err := s.repo.List(ctx, filter)
	if err != nil {
		return nil, nil, appErrors.Wrap(err, appErrors.ErrInternal.Code, appErrors.ErrInternal.Status, "failed to list users")
	}

	page := filter.Page
	if page < 1 {
		page = 1
	}
	pageSize := filter.PageSize
	if pageSize <= 0 {
		pageSize = 20
	}

	pagination := &models.Pagination{
		Page:       page,
		PageSize:   pageSize,
		TotalCount: total,
	}

	return users, pagination, nil
}

// Get returns a user by ID.
func (s *UserService) Get(ctx context.Context, id string) (*models.User, error) {
	user, err := s.repo.FindByID(ctx, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, appErrors.Clone(appErrors.ErrNotFound, "user not found")
		}
		return nil, appErrors.Wrap(err, appErrors.ErrInternal.Code, appErrors.ErrInternal.Status, "failed to load user")
	}
	return user, nil
}

// Create adds a new user.
func (s *UserService) Create(ctx context.Context, req CreateUserRequest, actorID string, meta models.LoginRequest) (*models.User, error) {
	if err := s.validator.Struct(req); err != nil {
		return nil, appErrors.Wrap(err, appErrors.ErrValidation.Code, appErrors.ErrValidation.Status, "invalid create user payload")
	}

	if _, err := s.repo.FindByEmail(ctx, req.Email); err == nil {
		return nil, appErrors.Clone(appErrors.ErrConflict, "email already exists")
	} else if !errors.Is(err, sql.ErrNoRows) {
		return nil, appErrors.Wrap(err, appErrors.ErrInternal.Code, appErrors.ErrInternal.Status, "failed to check email uniqueness")
	}

	passwordHash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		return nil, appErrors.Wrap(err, appErrors.ErrInternal.Code, appErrors.ErrInternal.Status, "failed to hash password")
	}

	user := &models.User{
		ID:           uuid.NewString(),
		Email:        strings.ToLower(req.Email),
		FullName:     req.FullName,
		Role:         req.Role,
		Active:       req.Active,
		PasswordHash: string(passwordHash),
	}

	if err := s.repo.Create(ctx, user); err != nil {
		return nil, appErrors.Wrap(err, appErrors.ErrInternal.Code, appErrors.ErrInternal.Status, "failed to create user")
	}

	newPayload, _ := json.Marshal(map[string]interface{}{"id": user.ID, "email": user.Email, "role": user.Role})
	if err := s.repo.CreateAuditLog(ctx, &models.AuditLog{
		UserID:     &actorID,
		Action:     models.AuditActionUserCreate,
		Resource:   "users",
		ResourceID: &user.ID,
		NewValues:  newPayload,
		IPAddress:  meta.IP,
		UserAgent:  meta.UserAgent,
	}); err != nil {
		s.logger.Warn("failed to record user create audit log", zap.Error(err))
	}

	return user, nil
}

// Update modifies the user attributes.
func (s *UserService) Update(ctx context.Context, id string, req UpdateUserRequest, actorID string, meta models.LoginRequest) (*models.User, error) {
	if err := s.validator.Struct(req); err != nil {
		return nil, appErrors.Wrap(err, appErrors.ErrValidation.Code, appErrors.ErrValidation.Status, "invalid update payload")
	}

	user, err := s.repo.FindByID(ctx, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, appErrors.Clone(appErrors.ErrNotFound, "user not found")
		}
		return nil, appErrors.Wrap(err, appErrors.ErrInternal.Code, appErrors.ErrInternal.Status, "failed to load user")
	}

	oldPayload, _ := json.Marshal(map[string]interface{}{"role": user.Role, "active": user.Active})

	user.FullName = req.FullName
	user.Role = req.Role
	if req.Active != nil {
		user.Active = *req.Active
	}

	if err := s.repo.Update(ctx, user); err != nil {
		return nil, appErrors.Wrap(err, appErrors.ErrInternal.Code, appErrors.ErrInternal.Status, "failed to update user")
	}

	newPayload, _ := json.Marshal(map[string]interface{}{"role": user.Role, "active": user.Active})
	if err := s.repo.CreateAuditLog(ctx, &models.AuditLog{
		UserID:     &actorID,
		Action:     models.AuditActionUserUpdate,
		Resource:   "users",
		ResourceID: &user.ID,
		OldValues:  oldPayload,
		NewValues:  newPayload,
		IPAddress:  meta.IP,
		UserAgent:  meta.UserAgent,
	}); err != nil {
		s.logger.Warn("failed to record user update audit log", zap.Error(err))
	}

	return user, nil
}

// Delete performs a soft delete (inactive) on a user.
func (s *UserService) Delete(ctx context.Context, id string, actorID string, meta models.LoginRequest) error {
	user, err := s.repo.FindByID(ctx, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return appErrors.Clone(appErrors.ErrNotFound, "user not found")
		}
		return appErrors.Wrap(err, appErrors.ErrInternal.Code, appErrors.ErrInternal.Status, "failed to load user")
	}

	if err := s.repo.Delete(ctx, id); err != nil {
		return appErrors.Wrap(err, appErrors.ErrInternal.Code, appErrors.ErrInternal.Status, "failed to delete user")
	}

	oldPayload, _ := json.Marshal(map[string]interface{}{"active": user.Active})
	newPayload, _ := json.Marshal(map[string]interface{}{"active": false})

	if err := s.repo.CreateAuditLog(ctx, &models.AuditLog{
		UserID:     &actorID,
		Action:     models.AuditActionUserDelete,
		Resource:   "users",
		ResourceID: &user.ID,
		OldValues:  oldPayload,
		NewValues:  newPayload,
		IPAddress:  meta.IP,
		UserAgent:  meta.UserAgent,
	}); err != nil {
		s.logger.Warn("failed to record user delete audit log", zap.Error(err))
	}

	return nil
}
