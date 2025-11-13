package service

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"go.uber.org/zap"

	"github.com/noah-isme/sma-adp-api/internal/dto"
	"github.com/noah-isme/sma-adp-api/internal/models"
	"github.com/noah-isme/sma-adp-api/internal/repository"
	appErrors "github.com/noah-isme/sma-adp-api/pkg/errors"
)

type mutationStore interface {
	Create(ctx context.Context, mutation *models.Mutation) error
	GetByID(ctx context.Context, id string) (*models.Mutation, error)
	List(ctx context.Context, filter models.MutationFilter) ([]models.Mutation, error)
	UpdateStatusAndSnapshot(ctx context.Context, params repository.UpdateMutationParams) error
}

type auditLogger interface {
	CreateAuditLog(ctx context.Context, log *models.AuditLog) error
}

// MutationSnapshotProvider resolves the latest entity snapshot for audit trails.
type MutationSnapshotProvider interface {
	Snapshot(ctx context.Context, entity, entityID string) ([]byte, error)
}

// MutationApplier applies changes for a particular entity when approved.
type MutationApplier interface {
	Apply(ctx context.Context, mutation *models.Mutation) ([]byte, error)
}

// MutationApplierFunc allows using plain functions.
type MutationApplierFunc func(ctx context.Context, mutation *models.Mutation) ([]byte, error)

// Apply implements MutationApplier.
func (f MutationApplierFunc) Apply(ctx context.Context, mutation *models.Mutation) ([]byte, error) {
	return f(ctx, mutation)
}

// MutationService orchestrates mutation requests and reviews.
type MutationService struct {
	repo      mutationStore
	audit     auditLogger
	snapshot  MutationSnapshotProvider
	appliers  map[string]MutationApplier
	logger    *zap.Logger
	validator mutationValidator
}

type mutationValidator interface {
	ValidateRequest(req dto.CreateMutationRequest) error
}

// MutationServiceOption configures the service.
type MutationServiceOption func(*MutationService)

// WithMutationAppliers sets the applier map keyed by entity.
func WithMutationAppliers(appliers map[string]MutationApplier) MutationServiceOption {
	return func(s *MutationService) {
		if s.appliers == nil {
			s.appliers = make(map[string]MutationApplier)
		}
		for k, v := range appliers {
			s.appliers[k] = v
		}
	}
}

// WithMutationSnapshotProvider overrides the snapshot provider.
func WithMutationSnapshotProvider(provider MutationSnapshotProvider) MutationServiceOption {
	return func(s *MutationService) {
		if provider != nil {
			s.snapshot = provider
		}
	}
}

// NewMutationService constructs the service with defaults.
func NewMutationService(repo mutationStore, audit auditLogger, logger *zap.Logger, opts ...MutationServiceOption) *MutationService {
	if logger == nil {
		logger = zap.NewNop()
	}
	svc := &MutationService{
		repo:     repo,
		audit:    audit,
		logger:   logger,
		appliers: make(map[string]MutationApplier),
		snapshot: MutationSnapshotProviderFunc(func(context.Context, string, string) ([]byte, error) {
			return []byte("{}"), nil
		}),
		validator: &defaultMutationValidator{},
	}
	for _, opt := range opts {
		if opt != nil {
			opt(svc)
		}
	}
	return svc
}

// RequestChange stores a new mutation request after validating payloads.
func (s *MutationService) RequestChange(ctx context.Context, req dto.CreateMutationRequest, userID string) (*models.Mutation, error) {
	if err := s.validator.ValidateRequest(req); err != nil {
		return nil, err
	}
	entity := strings.ToLower(strings.TrimSpace(req.Entity))
	if entity == "" {
		return nil, appErrors.Clone(appErrors.ErrValidation, "entity is required")
	}
	snapshot, err := s.snapshot.Snapshot(ctx, req.Entity, req.EntityID)
	if err != nil {
		return nil, appErrors.Wrap(err, appErrors.ErrInternal.Code, appErrors.ErrInternal.Status, "failed to capture current snapshot")
	}
	if len(snapshot) == 0 {
		snapshot = []byte("{}")
	}
	mutation := &models.Mutation{
		Type:             models.MutationType(strings.ToUpper(string(req.Type))),
		Entity:           entity,
		EntityID:         req.EntityID,
		Reason:           req.Reason,
		RequestedChanges: append([]byte(nil), req.RequestedChanges...),
		CurrentSnapshot:  append([]byte(nil), snapshot...),
		Status:           models.MutationStatusPending,
		RequestedBy:      userID,
	}
	if err := s.repo.Create(ctx, mutation); err != nil {
		return nil, appErrors.Wrap(err, appErrors.ErrInternal.Code, appErrors.ErrInternal.Status, "failed to create mutation request")
	}
	s.emitAudit(ctx, &models.AuditLog{
		UserID:     &userID,
		Action:     models.AuditActionMutationCreate,
		Resource:   mutation.Entity,
		ResourceID: &mutation.EntityID,
		NewValues:  mutation.RequestedChanges,
	})
	return mutation, nil
}

// List returns accessible mutations respecting actor role.
func (s *MutationService) List(ctx context.Context, query dto.MutationQuery, actor *models.JWTClaims) ([]models.Mutation, error) {
	filter := models.MutationFilter{
		Status: query.Status,
		Entity: strings.ToLower(strings.TrimSpace(query.Entity)),
		Type:   query.Type,
	}
	if actor == nil {
		return nil, appErrors.ErrUnauthorized
	}
	switch actor.Role {
	case models.RoleSuperAdmin, models.RoleAdmin:
		// full access, no extra filters
	case models.RoleTeacher:
		filter.RequestedBy = actor.UserID
	default:
		return nil, appErrors.ErrForbidden
	}
	mutations, err := s.repo.List(ctx, filter)
	if err != nil {
		return nil, appErrors.Wrap(err, appErrors.ErrInternal.Code, appErrors.ErrInternal.Status, "failed to list mutations")
	}
	return mutations, nil
}

// Get returns a mutation enforcing scope constraints.
func (s *MutationService) Get(ctx context.Context, id string, actor *models.JWTClaims) (*models.Mutation, error) {
	if actor == nil {
		return nil, appErrors.ErrUnauthorized
	}
	mutation, err := s.repo.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, appErrors.ErrNotFound
		}
		return nil, appErrors.Wrap(err, appErrors.ErrInternal.Code, appErrors.ErrInternal.Status, "failed to load mutation")
	}
	if actor.Role == models.RoleTeacher && mutation.RequestedBy != actor.UserID {
		return nil, appErrors.ErrForbidden
	}
	return mutation, nil
}

// Review applies reviewer decision and records audit trail.
func (s *MutationService) Review(ctx context.Context, id string, req dto.ReviewMutationRequest, reviewerID string) (*models.Mutation, error) {
	mutation, err := s.repo.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, appErrors.ErrNotFound
		}
		return nil, appErrors.Wrap(err, appErrors.ErrInternal.Code, appErrors.ErrInternal.Status, "failed to load mutation")
	}
	oldSnapshot := append([]byte(nil), mutation.CurrentSnapshot...)
	if mutation.Status != models.MutationStatusPending {
		return nil, appErrors.Clone(appErrors.ErrConflict, "mutation already reviewed")
	}
	if req.Status != models.MutationStatusApproved && req.Status != models.MutationStatusRejected {
		return nil, appErrors.Clone(appErrors.ErrValidation, "status must be APPROVED or REJECTED")
	}

	var newSnapshot []byte
	if req.Status == models.MutationStatusApproved {
		applier := s.appliers[mutation.Entity]
		if applier == nil {
			return nil, appErrors.Clone(appErrors.ErrPreconditionFailed, fmt.Sprintf("unsupported mutation entity: %s", mutation.Entity))
		}
		newSnapshot, err = applier.Apply(ctx, mutation)
		if err != nil {
			return nil, appErrors.Wrap(err, appErrors.ErrInternal.Code, appErrors.ErrInternal.Status, "failed to apply mutation")
		}
	}
	now := time.Now().UTC()
	params := repository.UpdateMutationParams{
		ID:         mutation.ID,
		Status:     req.Status,
		ReviewedBy: reviewerID,
		ReviewedAt: now,
		Note:       optionalString(req.Note),
	}
	if len(newSnapshot) > 0 {
		params.CurrentSnapshot = newSnapshot
	}
	if err := s.repo.UpdateStatusAndSnapshot(ctx, params); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, appErrors.Clone(appErrors.ErrConflict, "mutation already processed")
		}
		return nil, appErrors.Wrap(err, appErrors.ErrInternal.Code, appErrors.ErrInternal.Status, "failed to update mutation")
	}
	mutation.Status = req.Status
	mutation.ReviewedBy = &reviewerID
	mutation.ReviewedAt = &now
	if req.Note != "" {
		mutation.Note = &req.Note
	}
	if len(newSnapshot) > 0 {
		mutation.CurrentSnapshot = newSnapshot
	}
	s.emitAudit(ctx, &models.AuditLog{
		UserID:     &reviewerID,
		Action:     models.AuditActionMutationReview,
		Resource:   mutation.Entity,
		ResourceID: &mutation.EntityID,
		NewValues:  mutation.RequestedChanges,
		OldValues:  oldSnapshot,
	})
	return mutation, nil
}

func (s *MutationService) emitAudit(ctx context.Context, log *models.AuditLog) {
	if s.audit == nil || log == nil {
		return
	}
	log.IPAddress = "system"
	log.UserAgent = "mutation-service"
	if err := s.audit.CreateAuditLog(ctx, log); err != nil {
		s.logger.Warn("failed to persist audit log", zap.Error(err))
	}
}

func optionalString(value string) *string {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	v := strings.TrimSpace(value)
	return &v
}

// defaultMutationValidator enforces basic payload checks.
type defaultMutationValidator struct{}

func (v *defaultMutationValidator) ValidateRequest(req dto.CreateMutationRequest) error {
	if req.Type == "" || req.Entity == "" || req.EntityID == "" {
		return appErrors.Clone(appErrors.ErrValidation, "type, entity, and entityId are required")
	}
	if strings.TrimSpace(req.Reason) == "" {
		return appErrors.Clone(appErrors.ErrValidation, "reason is required")
	}
	if len(req.RequestedChanges) == 0 {
		return appErrors.Clone(appErrors.ErrValidation, "requestedChanges is required")
	}
	if !json.Valid(req.RequestedChanges) {
		return appErrors.Clone(appErrors.ErrValidation, "requestedChanges must be valid JSON")
	}
	switch models.MutationType(strings.ToUpper(string(req.Type))) {
	case models.MutationTypeStudentData,
		models.MutationTypeGradeCorrection,
		models.MutationTypeAttendanceFix,
		models.MutationTypeClassChange,
		models.MutationTypeOther:
	default:
		return appErrors.Clone(appErrors.ErrValidation, "unsupported mutation type")
	}
	return nil
}

// MutationSnapshotProviderFunc helper to use functions as providers.
type MutationSnapshotProviderFunc func(ctx context.Context, entity, entityID string) ([]byte, error)

// Snapshot implements provider interface.
func (f MutationSnapshotProviderFunc) Snapshot(ctx context.Context, entity, entityID string) ([]byte, error) {
	return f(ctx, entity, entityID)
}
