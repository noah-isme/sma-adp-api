package repository

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"

	"github.com/noah-isme/sma-adp-api/internal/models"
)

// MutationRepository persists mutation workflow data.
type MutationRepository struct {
	db *sqlx.DB
}

// NewMutationRepository constructs the repository.
func NewMutationRepository(db *sqlx.DB) *MutationRepository {
	return &MutationRepository{db: db}
}

// Create inserts a new mutation row.
func (r *MutationRepository) Create(ctx context.Context, mutation *models.Mutation) error {
	if mutation.ID == "" {
		mutation.ID = uuid.NewString()
	}
	if mutation.Status == "" {
		mutation.Status = models.MutationStatusPending
	}
	if mutation.RequestedAt.IsZero() {
		mutation.RequestedAt = time.Now().UTC()
	}
	const query = `INSERT INTO mutations
	(id, type, entity, entity_id, current_snapshot, requested_changes, status, reason, requested_by, reviewed_by, requested_at, reviewed_at, note)
	VALUES (:id, :type, :entity, :entity_id, :current_snapshot, :requested_changes, :status, :reason, :requested_by, :reviewed_by, :requested_at, :reviewed_at, :note)`
	if _, err := r.db.NamedExecContext(ctx, query, mutation); err != nil {
		return fmt.Errorf("create mutation: %w", err)
	}
	return nil
}

// GetByID fetches a mutation by identifier.
func (r *MutationRepository) GetByID(ctx context.Context, id string) (*models.Mutation, error) {
	const query = `SELECT id, type, entity, entity_id, current_snapshot, requested_changes, status, reason,
       requested_by, reviewed_by, requested_at, reviewed_at, note
	FROM mutations WHERE id = $1`
	var mutation models.Mutation
	if err := r.db.GetContext(ctx, &mutation, query, id); err != nil {
		return nil, err
	}
	return &mutation, nil
}

// List returns mutations matching the filter (sorted latest first).
func (r *MutationRepository) List(ctx context.Context, filter models.MutationFilter) ([]models.Mutation, error) {
	builder := strings.Builder{}
	args := make([]interface{}, 0, 6)
	builder.WriteString(`SELECT id, type, entity, entity_id, current_snapshot, requested_changes, status, reason,
       requested_by, reviewed_by, requested_at, reviewed_at, note FROM mutations`)

	conditions := make([]string, 0, 4)
	if len(filter.Status) > 0 {
		placeholders := make([]string, len(filter.Status))
		for i, status := range filter.Status {
			args = append(args, status)
			placeholders[i] = fmt.Sprintf("$%d", len(args))
		}
		conditions = append(conditions, fmt.Sprintf("status IN (%s)", strings.Join(placeholders, ",")))
	}
	if filter.Entity != "" {
		args = append(args, filter.Entity)
		conditions = append(conditions, fmt.Sprintf("entity = $%d", len(args)))
	}
	if filter.Type != "" {
		args = append(args, filter.Type)
		conditions = append(conditions, fmt.Sprintf("type = $%d", len(args)))
	}
	if filter.EntityID != "" {
		args = append(args, filter.EntityID)
		conditions = append(conditions, fmt.Sprintf("entity_id = $%d", len(args)))
	}
	if filter.RequestedBy != "" {
		args = append(args, filter.RequestedBy)
		conditions = append(conditions, fmt.Sprintf("requested_by = $%d", len(args)))
	}
	if filter.ReviewerID != "" {
		args = append(args, filter.ReviewerID)
		conditions = append(conditions, fmt.Sprintf("reviewed_by = $%d", len(args)))
	}
	if len(conditions) > 0 {
		builder.WriteString(" WHERE ")
		builder.WriteString(strings.Join(conditions, " AND "))
	}
	builder.WriteString(" ORDER BY requested_at DESC")

	limit := filter.Limit
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	offset := filter.Offset
	if offset < 0 {
		offset = 0
	}
	builder.WriteString(fmt.Sprintf(" LIMIT %d OFFSET %d", limit, offset))

	var mutations []models.Mutation
	if err := r.db.SelectContext(ctx, &mutations, builder.String(), args...); err != nil {
		return nil, fmt.Errorf("list mutations: %w", err)
	}
	return mutations, nil
}

// UpdateMutationParams groups mutable columns for review operations.
type UpdateMutationParams struct {
	ID              string
	Status          models.MutationStatus
	ReviewedBy      string
	ReviewedAt      time.Time
	Note            *string
	CurrentSnapshot []byte
}

// UpdateStatusAndSnapshot persists review outcome.
func (r *MutationRepository) UpdateStatusAndSnapshot(ctx context.Context, params UpdateMutationParams) error {
	setParts := []string{
		"status = :status",
		"reviewed_by = :reviewed_by",
		"reviewed_at = :reviewed_at",
	}
	if params.Note != nil {
		setParts = append(setParts, "note = :note")
	}
	if len(params.CurrentSnapshot) > 0 {
		setParts = append(setParts, "current_snapshot = :current_snapshot")
	}
	query := fmt.Sprintf("UPDATE mutations SET %s WHERE id = :id AND status = '%s'",
		strings.Join(setParts, ", "),
		models.MutationStatusPending,
	)
	result, err := r.db.NamedExecContext(ctx, query, map[string]interface{}{
		"id":               params.ID,
		"status":           params.Status,
		"reviewed_by":      params.ReviewedBy,
		"reviewed_at":      params.ReviewedAt,
		"note":             params.Note,
		"current_snapshot": params.CurrentSnapshot,
	})
	if err != nil {
		return fmt.Errorf("update mutation status: %w", err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("check mutation update rows: %w", err)
	}
	if rows == 0 {
		return sql.ErrNoRows
	}
	return nil
}
