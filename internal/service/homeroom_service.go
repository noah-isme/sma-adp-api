package service

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"

	"github.com/go-playground/validator/v10"
	"go.uber.org/zap"

	"github.com/noah-isme/sma-adp-api/internal/dto"
	"github.com/noah-isme/sma-adp-api/internal/models"
	"github.com/noah-isme/sma-adp-api/internal/repository"
	appErrors "github.com/noah-isme/sma-adp-api/pkg/errors"
)

const (
	homeroomSubjectCode = "HOMEROOM"
	homeroomResource    = "homeroom"
)

type homeroomStore interface {
	List(ctx context.Context, filter dto.HomeroomFilter) ([]dto.HomeroomItem, error)
	ListForTeacher(ctx context.Context, teacherID string, filter dto.HomeroomFilter) ([]dto.HomeroomItem, error)
	Get(ctx context.Context, classID, termID string) (*dto.HomeroomItem, error)
	Upsert(ctx context.Context, params repository.HomeroomAssignmentParams) (*string, error)
}

type homeroomTermReader interface {
	FindByID(ctx context.Context, id string) (*models.Term, error)
	FindActive(ctx context.Context) (*models.Term, error)
}

type homeroomSubjectFinder interface {
	FindByCode(ctx context.Context, code string) (*models.Subject, error)
}

type homeroomClassAccessChecker interface {
	HasClassAccess(ctx context.Context, teacherID, classID, termID string) (bool, error)
}

// HomeroomService orchestrates homeroom assignment workflows.
type HomeroomService struct {
	repo        homeroomStore
	classes     classReader
	terms       homeroomTermReader
	teachers    teacherRepository
	subjects    homeroomSubjectFinder
	assignments homeroomClassAccessChecker
	audit       auditLogger
	validator   *validator.Validate
	logger      *zap.Logger
}

// NewHomeroomService builds a HomeroomService with sane defaults.
func NewHomeroomService(
	repo homeroomStore,
	classes classReader,
	terms homeroomTermReader,
	teachers teacherRepository,
	subjects homeroomSubjectFinder,
	assignments homeroomClassAccessChecker,
	audit auditLogger,
	validate *validator.Validate,
	logger *zap.Logger,
) *HomeroomService {
	if validate == nil {
		validate = validator.New()
	}
	if logger == nil {
		logger = zap.NewNop()
	}
	return &HomeroomService{
		repo:        repo,
		classes:     classes,
		terms:       terms,
		teachers:    teachers,
		subjects:    subjects,
		assignments: assignments,
		audit:       audit,
		validator:   validate,
		logger:      logger,
	}
}

// List returns homeroom entries respecting RBAC filter.
func (s *HomeroomService) List(ctx context.Context, filter dto.HomeroomFilter, claims *models.JWTClaims) ([]dto.HomeroomItem, error) {
	if claims == nil {
		return nil, appErrors.ErrUnauthorized
	}

	termID, err := s.resolveTerm(ctx, filter.TermID)
	if err != nil {
		return nil, err
	}
	filter.TermID = termID

	if filter.ClassID != "" {
		if err := s.ensureClass(ctx, filter.ClassID); err != nil {
			return nil, err
		}
		if claims.Role == models.RoleTeacher {
			allowed, err := s.assignments.HasClassAccess(ctx, claims.UserID, filter.ClassID, termID)
			if err != nil {
				return nil, appErrors.Wrap(err, appErrors.ErrInternal.Code, appErrors.ErrInternal.Status, "failed to verify class access")
			}
			if !allowed {
				return nil, appErrors.ErrForbidden
			}
		}
	}

	switch claims.Role {
	case models.RoleAdmin, models.RoleSuperAdmin:
		items, err := s.repo.List(ctx, filter)
		if err != nil {
			return nil, appErrors.Wrap(err, appErrors.ErrInternal.Code, appErrors.ErrInternal.Status, "failed to list homerooms")
		}
		return items, nil
	case models.RoleTeacher:
		items, err := s.repo.ListForTeacher(ctx, claims.UserID, filter)
		if err != nil {
			return nil, appErrors.Wrap(err, appErrors.ErrInternal.Code, appErrors.ErrInternal.Status, "failed to list homerooms")
		}
		return items, nil
	default:
		return nil, appErrors.ErrForbidden
	}
}

// Get returns a single homeroom entry.
func (s *HomeroomService) Get(ctx context.Context, classID, termID string, claims *models.JWTClaims) (*dto.HomeroomItem, error) {
	if claims == nil {
		return nil, appErrors.ErrUnauthorized
	}
	if classID == "" {
		return nil, appErrors.Clone(appErrors.ErrValidation, "classId is required")
	}
	if err := s.ensureClass(ctx, classID); err != nil {
		return nil, err
	}
	resolvedTermID, err := s.resolveTerm(ctx, termID)
	if err != nil {
		return nil, err
	}

	if claims.Role == models.RoleTeacher {
		allowed, err := s.assignments.HasClassAccess(ctx, claims.UserID, classID, resolvedTermID)
		if err != nil {
			return nil, appErrors.Wrap(err, appErrors.ErrInternal.Code, appErrors.ErrInternal.Status, "failed to verify class access")
		}
		if !allowed {
			return nil, appErrors.ErrForbidden
		}
	}

	item, err := s.repo.Get(ctx, classID, resolvedTermID)
	if err != nil {
		return nil, appErrors.Wrap(err, appErrors.ErrInternal.Code, appErrors.ErrInternal.Status, "failed to load homeroom")
	}
	return item, nil
}

// Set assigns or reassigns a homeroom teacher for the provided class and term.
func (s *HomeroomService) Set(ctx context.Context, req dto.SetHomeroomRequest, actor *models.JWTClaims) (*dto.HomeroomItem, error) {
	if actor == nil {
		return nil, appErrors.ErrUnauthorized
	}
	if err := s.validator.Struct(req); err != nil {
		return nil, appErrors.Wrap(err, appErrors.ErrValidation.Code, appErrors.ErrValidation.Status, "invalid homeroom payload")
	}
	if err := s.ensureClass(ctx, req.ClassID); err != nil {
		return nil, err
	}
	if err := s.ensureTerm(ctx, req.TermID); err != nil {
		return nil, err
	}

	teacher, err := s.teachers.FindByID(ctx, req.TeacherID)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, appErrors.Clone(appErrors.ErrNotFound, "teacher not found")
		}
		return nil, appErrors.Wrap(err, appErrors.ErrInternal.Code, appErrors.ErrInternal.Status, "failed to load teacher")
	}
	if !teacher.Active {
		return nil, appErrors.Clone(appErrors.ErrPreconditionFailed, "teacher inactive")
	}

	subject, err := s.subjects.FindByCode(ctx, homeroomSubjectCode)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, appErrors.Clone(appErrors.ErrPreconditionFailed, fmt.Sprintf("%s subject not configured", homeroomSubjectCode))
		}
		return nil, appErrors.Wrap(err, appErrors.ErrInternal.Code, appErrors.ErrInternal.Status, "failed to resolve homeroom subject")
	}

	prevTeacherID, err := s.repo.Upsert(ctx, repository.HomeroomAssignmentParams{
		ClassID:   req.ClassID,
		TermID:    req.TermID,
		TeacherID: req.TeacherID,
		SubjectID: subject.ID,
	})
	if err != nil {
		return nil, appErrors.Wrap(err, appErrors.ErrInternal.Code, appErrors.ErrInternal.Status, "failed to update homeroom")
	}

	item, err := s.repo.Get(ctx, req.ClassID, req.TermID)
	if err != nil {
		return nil, appErrors.Wrap(err, appErrors.ErrInternal.Code, appErrors.ErrInternal.Status, "failed to load homeroom")
	}

	s.emitAudit(ctx, actor, req, prevTeacherID)
	return item, nil
}

func (s *HomeroomService) ensureClass(ctx context.Context, classID string) error {
	if classID == "" {
		return appErrors.Clone(appErrors.ErrValidation, "classId is required")
	}
	if _, err := s.classes.FindByID(ctx, classID); err != nil {
		if err == sql.ErrNoRows {
			return appErrors.Clone(appErrors.ErrNotFound, "class not found")
		}
		return appErrors.Wrap(err, appErrors.ErrInternal.Code, appErrors.ErrInternal.Status, "failed to load class")
	}
	return nil
}

func (s *HomeroomService) ensureTerm(ctx context.Context, termID string) error {
	if _, err := s.terms.FindByID(ctx, termID); err != nil {
		if err == sql.ErrNoRows {
			return appErrors.Clone(appErrors.ErrNotFound, "term not found")
		}
		return appErrors.Wrap(err, appErrors.ErrInternal.Code, appErrors.ErrInternal.Status, "failed to load term")
	}
	return nil
}

func (s *HomeroomService) resolveTerm(ctx context.Context, termID string) (string, error) {
	if termID != "" {
		if err := s.ensureTerm(ctx, termID); err != nil {
			return "", err
		}
		return termID, nil
	}
	term, err := s.terms.FindActive(ctx)
	if err != nil {
		if err == sql.ErrNoRows {
			return "", appErrors.Clone(appErrors.ErrNotFound, "active term not found")
		}
		return "", appErrors.Wrap(err, appErrors.ErrInternal.Code, appErrors.ErrInternal.Status, "failed to load active term")
	}
	return term.ID, nil
}

func (s *HomeroomService) emitAudit(ctx context.Context, actor *models.JWTClaims, req dto.SetHomeroomRequest, oldTeacherID *string) {
	if s.audit == nil {
		return
	}
	payload := map[string]interface{}{
		"classId":           req.ClassID,
		"termId":            req.TermID,
		"homeroomTeacherId": req.TeacherID,
	}
	newValues, _ := json.Marshal(payload)
	var oldValues []byte
	if oldTeacherID != nil {
		oldPayload := map[string]interface{}{
			"classId":           req.ClassID,
			"termId":            req.TermID,
			"homeroomTeacherId": *oldTeacherID,
		}
		oldValues, _ = json.Marshal(oldPayload)
	}
	var userID *string
	if actor != nil {
		userID = &actor.UserID
	}
	log := &models.AuditLog{
		UserID:     userID,
		Action:     models.AuditActionHomeroomUpdate,
		Resource:   homeroomResource,
		ResourceID: &req.ClassID,
		OldValues:  oldValues,
		NewValues:  newValues,
		IPAddress:  "system",
		UserAgent:  "homeroom-service",
	}
	if err := s.audit.CreateAuditLog(ctx, log); err != nil && s.logger != nil {
		s.logger.Warn("failed to record homeroom audit", zap.Error(err))
	}
}
