package service

import (
	"context"
	"database/sql"
	"strings"

	"github.com/go-playground/validator/v10"
	"go.uber.org/zap"

	"github.com/noah-isme/sma-adp-api/internal/models"
	appErrors "github.com/noah-isme/sma-adp-api/pkg/errors"
)

type teacherRepository interface {
	List(ctx context.Context, filter models.TeacherFilter) ([]models.Teacher, int, error)
	FindByID(ctx context.Context, id string) (*models.Teacher, error)
	ExistsByEmail(ctx context.Context, email, excludeID string) (bool, error)
	ExistsByNIP(ctx context.Context, nip, excludeID string) (bool, error)
	Create(ctx context.Context, teacher *models.Teacher) error
	Update(ctx context.Context, teacher *models.Teacher) error
	Deactivate(ctx context.Context, id string) error
}

// CreateTeacherRequest represents payload for creating teachers.
type CreateTeacherRequest struct {
	Email     string  `json:"email" validate:"required,email"`
	FullName  string  `json:"full_name" validate:"required"`
	NIP       *string `json:"nip" validate:"omitempty,max=50"`
	Phone     *string `json:"phone" validate:"omitempty,max=50"`
	Expertise *string `json:"expertise" validate:"omitempty,max=500"`
}

// UpdateTeacherRequest represents payload for updating teachers.
type UpdateTeacherRequest struct {
	Email     string  `json:"email" validate:"required,email"`
	FullName  string  `json:"full_name" validate:"required"`
	NIP       *string `json:"nip" validate:"omitempty,max=50"`
	Phone     *string `json:"phone" validate:"omitempty,max=50"`
	Expertise *string `json:"expertise" validate:"omitempty,max=500"`
	Active    *bool   `json:"active"`
}

// TeacherService orchestrates teacher operations.
type TeacherService struct {
	repo      teacherRepository
	validator *validator.Validate
	logger    *zap.Logger
}

// NewTeacherService constructs a TeacherService.
func NewTeacherService(repo teacherRepository, validate *validator.Validate, logger *zap.Logger) *TeacherService {
	if validate == nil {
		validate = validator.New()
	}
	if logger == nil {
		logger = zap.NewNop()
	}
	return &TeacherService{repo: repo, validator: validate, logger: logger}
}

// List returns teachers plus pagination data.
func (s *TeacherService) List(ctx context.Context, filter models.TeacherFilter) ([]models.Teacher, *models.Pagination, error) {
	teachers, total, err := s.repo.List(ctx, filter)
	if err != nil {
		return nil, nil, appErrors.Wrap(err, appErrors.ErrInternal.Code, appErrors.ErrInternal.Status, "failed to list teachers")
	}
	page := filter.Page
	if page < 1 {
		page = 1
	}
	size := filter.PageSize
	if size <= 0 {
		size = 20
	}
	pagination := &models.Pagination{Page: page, PageSize: size, TotalCount: total}
	return teachers, pagination, nil
}

// Get returns a teacher by id.
func (s *TeacherService) Get(ctx context.Context, id string) (*models.Teacher, error) {
	teacher, err := s.repo.FindByID(ctx, id)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, appErrors.Clone(appErrors.ErrNotFound, "teacher not found")
		}
		return nil, appErrors.Wrap(err, appErrors.ErrInternal.Code, appErrors.ErrInternal.Status, "failed to load teacher")
	}
	return teacher, nil
}

// Create registers a new teacher record.
func (s *TeacherService) Create(ctx context.Context, req CreateTeacherRequest) (*models.Teacher, error) {
	if err := s.validator.Struct(req); err != nil {
		return nil, appErrors.Wrap(err, appErrors.ErrValidation.Code, appErrors.ErrValidation.Status, "invalid teacher payload")
	}
	if err := s.ensureUniqueFields(ctx, req.Email, req.NIP, ""); err != nil {
		return nil, err
	}

	teacher := &models.Teacher{
		Email:    strings.TrimSpace(req.Email),
		FullName: strings.TrimSpace(req.FullName),
		Active:   true,
	}
	teacher.NIP = normalizeOptional(req.NIP)
	teacher.Phone = normalizeOptional(req.Phone)
	teacher.Expertise = normalizeOptional(req.Expertise)

	if err := s.repo.Create(ctx, teacher); err != nil {
		return nil, appErrors.Wrap(err, appErrors.ErrInternal.Code, appErrors.ErrInternal.Status, "failed to create teacher")
	}
	return teacher, nil
}

// Update modifies an existing teacher.
func (s *TeacherService) Update(ctx context.Context, id string, req UpdateTeacherRequest) (*models.Teacher, error) {
	if err := s.validator.Struct(req); err != nil {
		return nil, appErrors.Wrap(err, appErrors.ErrValidation.Code, appErrors.ErrValidation.Status, "invalid teacher payload")
	}

	teacher, err := s.repo.FindByID(ctx, id)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, appErrors.Clone(appErrors.ErrNotFound, "teacher not found")
		}
		return nil, appErrors.Wrap(err, appErrors.ErrInternal.Code, appErrors.ErrInternal.Status, "failed to load teacher")
	}

	if err := s.ensureUniqueFields(ctx, req.Email, req.NIP, id); err != nil {
		return nil, err
	}

	teacher.Email = strings.TrimSpace(req.Email)
	teacher.FullName = strings.TrimSpace(req.FullName)
	teacher.NIP = normalizeOptional(req.NIP)
	teacher.Phone = normalizeOptional(req.Phone)
	teacher.Expertise = normalizeOptional(req.Expertise)
	if req.Active != nil {
		teacher.Active = *req.Active
	}

	if err := s.repo.Update(ctx, teacher); err != nil {
		return nil, appErrors.Wrap(err, appErrors.ErrInternal.Code, appErrors.ErrInternal.Status, "failed to update teacher")
	}
	return teacher, nil
}

// Deactivate marks a teacher inactive.
func (s *TeacherService) Deactivate(ctx context.Context, id string) error {
	if _, err := s.repo.FindByID(ctx, id); err != nil {
		if err == sql.ErrNoRows {
			return appErrors.Clone(appErrors.ErrNotFound, "teacher not found")
		}
		return appErrors.Wrap(err, appErrors.ErrInternal.Code, appErrors.ErrInternal.Status, "failed to load teacher")
	}
	if err := s.repo.Deactivate(ctx, id); err != nil {
		return appErrors.Wrap(err, appErrors.ErrInternal.Code, appErrors.ErrInternal.Status, "failed to deactivate teacher")
	}
	return nil
}

func (s *TeacherService) ensureUniqueFields(ctx context.Context, email string, nip *string, excludeID string) error {
	exists, err := s.repo.ExistsByEmail(ctx, email, excludeID)
	if err != nil {
		return appErrors.Wrap(err, appErrors.ErrInternal.Code, appErrors.ErrInternal.Status, "failed to check email uniqueness")
	}
	if exists {
		return appErrors.Clone(appErrors.ErrConflict, "email already used")
	}
	if nip != nil {
		trimmed := strings.TrimSpace(*nip)
		if trimmed != "" {
			exists, err = s.repo.ExistsByNIP(ctx, trimmed, excludeID)
			if err != nil {
				return appErrors.Wrap(err, appErrors.ErrInternal.Code, appErrors.ErrInternal.Status, "failed to check NIP uniqueness")
			}
			if exists {
				return appErrors.Clone(appErrors.ErrConflict, "nip already used")
			}
		}
	}
	return nil
}

func normalizeOptional(value *string) *string {
	if value == nil {
		return nil
	}
	trimmed := strings.TrimSpace(*value)
	if trimmed == "" {
		return nil
	}
	return &trimmed
}
