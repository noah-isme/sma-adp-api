package service

import (
	"context"
	"database/sql"
	"time"

	"github.com/go-playground/validator/v10"
	"go.uber.org/zap"

	"github.com/noah-isme/sma-adp-api/internal/models"
	appErrors "github.com/noah-isme/sma-adp-api/pkg/errors"
)

type studentRepository interface {
	List(ctx context.Context, filter models.StudentFilter) ([]models.StudentDetail, int, error)
	FindByID(ctx context.Context, id string) (*models.StudentDetail, error)
	ExistsByNIS(ctx context.Context, nis string, excludeID string) (bool, error)
	Create(ctx context.Context, student *models.Student) error
	Update(ctx context.Context, student *models.Student) error
	Deactivate(ctx context.Context, id string) error
}

// CreateStudentRequest holds payload for creating students.
type CreateStudentRequest struct {
	NIS       string    `json:"nis" validate:"required"`
	FullName  string    `json:"full_name" validate:"required"`
	Gender    string    `json:"gender" validate:"required"`
	BirthDate time.Time `json:"birth_date" validate:"required"`
	Address   string    `json:"address"`
	Phone     string    `json:"phone"`
}

// UpdateStudentRequest holds payload for updating students.
type UpdateStudentRequest struct {
	NIS       string    `json:"nis" validate:"required"`
	FullName  string    `json:"full_name" validate:"required"`
	Gender    string    `json:"gender" validate:"required"`
	BirthDate time.Time `json:"birth_date" validate:"required"`
	Address   string    `json:"address"`
	Phone     string    `json:"phone"`
	Active    bool      `json:"active"`
}

// StudentService handles student use-cases.
type StudentService struct {
	repo      studentRepository
	validator *validator.Validate
	logger    *zap.Logger
}

// NewStudentService constructs the student service.
func NewStudentService(repo studentRepository, validate *validator.Validate, logger *zap.Logger) *StudentService {
	if validate == nil {
		validate = validator.New()
	}
	if logger == nil {
		logger = zap.NewNop()
	}
	return &StudentService{repo: repo, validator: validate, logger: logger}
}

// List returns students and pagination metadata.
func (s *StudentService) List(ctx context.Context, filter models.StudentFilter) ([]models.StudentDetail, *models.Pagination, error) {
	students, total, err := s.repo.List(ctx, filter)
	if err != nil {
		return nil, nil, appErrors.Wrap(err, appErrors.ErrInternal.Code, appErrors.ErrInternal.Status, "failed to list students")
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
	return students, pagination, nil
}

// Get returns detailed student information.
func (s *StudentService) Get(ctx context.Context, id string) (*models.StudentDetail, error) {
	student, err := s.repo.FindByID(ctx, id)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, appErrors.Clone(appErrors.ErrNotFound, "student not found")
		}
		return nil, appErrors.Wrap(err, appErrors.ErrInternal.Code, appErrors.ErrInternal.Status, "failed to load student")
	}
	return student, nil
}

// Create registers a new student.
func (s *StudentService) Create(ctx context.Context, req CreateStudentRequest) (*models.Student, error) {
	if err := s.validator.Struct(req); err != nil {
		return nil, appErrors.Wrap(err, appErrors.ErrValidation.Code, appErrors.ErrValidation.Status, "invalid student payload")
	}
	exists, err := s.repo.ExistsByNIS(ctx, req.NIS, "")
	if err != nil {
		return nil, appErrors.Wrap(err, appErrors.ErrInternal.Code, appErrors.ErrInternal.Status, "failed to validate nis")
	}
	if exists {
		return nil, appErrors.Clone(appErrors.ErrConflict, "nis already used")
	}
	student := &models.Student{
		NIS:       req.NIS,
		FullName:  req.FullName,
		Gender:    req.Gender,
		BirthDate: req.BirthDate,
		Address:   req.Address,
		Phone:     req.Phone,
		Active:    true,
	}
	if err := s.repo.Create(ctx, student); err != nil {
		return nil, appErrors.Wrap(err, appErrors.ErrInternal.Code, appErrors.ErrInternal.Status, "failed to create student")
	}
	return &models.Student{
		ID:        student.ID,
		NIS:       student.NIS,
		FullName:  student.FullName,
		Gender:    student.Gender,
		BirthDate: student.BirthDate,
		Address:   student.Address,
		Phone:     student.Phone,
		Active:    student.Active,
		CreatedAt: student.CreatedAt,
		UpdatedAt: student.UpdatedAt,
	}, nil
}

// Update modifies an existing student record.
func (s *StudentService) Update(ctx context.Context, id string, req UpdateStudentRequest) (*models.Student, error) {
	if err := s.validator.Struct(req); err != nil {
		return nil, appErrors.Wrap(err, appErrors.ErrValidation.Code, appErrors.ErrValidation.Status, "invalid student payload")
	}
	detail, err := s.repo.FindByID(ctx, id)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, appErrors.Clone(appErrors.ErrNotFound, "student not found")
		}
		return nil, appErrors.Wrap(err, appErrors.ErrInternal.Code, appErrors.ErrInternal.Status, "failed to load student")
	}
	exists, err := s.repo.ExistsByNIS(ctx, req.NIS, id)
	if err != nil {
		return nil, appErrors.Wrap(err, appErrors.ErrInternal.Code, appErrors.ErrInternal.Status, "failed to validate nis")
	}
	if exists {
		return nil, appErrors.Clone(appErrors.ErrConflict, "nis already used")
	}
	student := detail.Student
	student.NIS = req.NIS
	student.FullName = req.FullName
	student.Gender = req.Gender
	student.BirthDate = req.BirthDate
	student.Address = req.Address
	student.Phone = req.Phone
	student.Active = req.Active
	if err := s.repo.Update(ctx, &student); err != nil {
		return nil, appErrors.Wrap(err, appErrors.ErrInternal.Code, appErrors.ErrInternal.Status, "failed to update student")
	}
	return &student, nil
}

// Deactivate marks student inactive.
func (s *StudentService) Deactivate(ctx context.Context, id string) error {
	if _, err := s.repo.FindByID(ctx, id); err != nil {
		if err == sql.ErrNoRows {
			return appErrors.Clone(appErrors.ErrNotFound, "student not found")
		}
		return appErrors.Wrap(err, appErrors.ErrInternal.Code, appErrors.ErrInternal.Status, "failed to load student")
	}
	if err := s.repo.Deactivate(ctx, id); err != nil {
		return appErrors.Wrap(err, appErrors.ErrInternal.Code, appErrors.ErrInternal.Status, "failed to deactivate student")
	}
	return nil
}
