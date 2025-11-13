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

// ArchiveRepository handles archive metadata persistence.
type ArchiveRepository struct {
	db *sqlx.DB
}

// NewArchiveRepository constructs the repository.
func NewArchiveRepository(db *sqlx.DB) *ArchiveRepository {
	return &ArchiveRepository{db: db}
}

// Create stores metadata for an uploaded archive file.
func (r *ArchiveRepository) Create(ctx context.Context, item *models.ArchiveItem) error {
	if item.ID == "" {
		item.ID = uuid.NewString()
	}
	if item.UploadedAt.IsZero() {
		item.UploadedAt = time.Now().UTC()
	}
	const query = `INSERT INTO archives
	(id, title, category, scope, ref_term_id, ref_class_id, ref_student_id, file_path, mime_type, size_bytes, uploaded_by, uploaded_at, deleted_at)
	VALUES (:id, :title, :category, :scope, :ref_term_id, :ref_class_id, :ref_student_id, :file_path, :mime_type, :size_bytes, :uploaded_by, :uploaded_at, :deleted_at)`
	if _, err := r.db.NamedExecContext(ctx, query, item); err != nil {
		return fmt.Errorf("create archive item: %w", err)
	}
	return nil
}

// GetByID retrieves one archive row.
func (r *ArchiveRepository) GetByID(ctx context.Context, id string) (*models.ArchiveItem, error) {
	const query = `SELECT id, title, category, scope, ref_term_id, ref_class_id, ref_student_id,
       file_path, mime_type, size_bytes, uploaded_by, uploaded_at, deleted_at
	FROM archives WHERE id = $1`
	var item models.ArchiveItem
	if err := r.db.GetContext(ctx, &item, query, id); err != nil {
		return nil, err
	}
	return &item, nil
}

// List returns archives applying filters and excluding deleted rows by default.
func (r *ArchiveRepository) List(ctx context.Context, filter models.ArchiveFilter) ([]models.ArchiveItem, error) {
	builder := strings.Builder{}
	builder.WriteString(`SELECT id, title, category, scope, ref_term_id, ref_class_id, ref_student_id,
       file_path, mime_type, size_bytes, uploaded_by, uploaded_at, deleted_at FROM archives`)
	args := make([]interface{}, 0, 5)
	conditions := make([]string, 0, 5)

	if !filter.IncludeDeleted {
		conditions = append(conditions, "deleted_at IS NULL")
	}
	if filter.Scope != "" {
		args = append(args, filter.Scope)
		conditions = append(conditions, fmt.Sprintf("scope = $%d", len(args)))
	}
	if filter.Category != "" {
		args = append(args, filter.Category)
		conditions = append(conditions, fmt.Sprintf("category = $%d", len(args)))
	}
	if filter.TermID != "" {
		args = append(args, filter.TermID)
		conditions = append(conditions, fmt.Sprintf("ref_term_id = $%d", len(args)))
	}
	if filter.ClassID != "" {
		args = append(args, filter.ClassID)
		conditions = append(conditions, fmt.Sprintf("ref_class_id = $%d", len(args)))
	}

	if len(conditions) > 0 {
		builder.WriteString(" WHERE ")
		builder.WriteString(strings.Join(conditions, " AND "))
	}
	builder.WriteString(" ORDER BY uploaded_at DESC")

	limit := filter.Limit
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	offset := filter.Offset
	if offset < 0 {
		offset = 0
	}
	builder.WriteString(fmt.Sprintf(" LIMIT %d OFFSET %d", limit, offset))

	var records []models.ArchiveItem
	if err := r.db.SelectContext(ctx, &records, builder.String(), args...); err != nil {
		return nil, fmt.Errorf("list archives: %w", err)
	}
	return records, nil
}

// SoftDelete marks an archive as deleted.
func (r *ArchiveRepository) SoftDelete(ctx context.Context, id string, deletedAt time.Time) error {
	const query = `UPDATE archives SET deleted_at = $2 WHERE id = $1 AND deleted_at IS NULL`
	res, err := r.db.ExecContext(ctx, query, id, deletedAt)
	if err != nil {
		return fmt.Errorf("soft delete archive: %w", err)
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("check archive delete rows: %w", err)
	}
	if affected == 0 {
		return sql.ErrNoRows
	}
	return nil
}
