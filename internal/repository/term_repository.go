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

// TermRepository handles persistence for academic terms.
type TermRepository struct {
	db *sqlx.DB
}

// NewTermRepository instantiates a term repository.
func NewTermRepository(db *sqlx.DB) *TermRepository {
	return &TermRepository{db: db}
}

// List returns terms matching provided filters.
func (r *TermRepository) List(ctx context.Context, filter models.TermFilter) ([]models.Term, int, error) {
	base := "FROM terms WHERE 1=1"
	var conditions []string
	var args []interface{}

	if filter.AcademicYear != "" {
		conditions = append(conditions, fmt.Sprintf("academic_year = $%d", len(args)+1))
		args = append(args, filter.AcademicYear)
	}
	if filter.Type != "" {
		conditions = append(conditions, fmt.Sprintf("type = $%d", len(args)+1))
		args = append(args, filter.Type)
	}
	if filter.IsActive != nil {
		conditions = append(conditions, fmt.Sprintf("is_active = $%d", len(args)+1))
		args = append(args, *filter.IsActive)
	}

	if len(conditions) > 0 {
		base += " AND " + strings.Join(conditions, " AND ")
	}

	sortBy := filter.SortBy
	if sortBy == "" {
		sortBy = "start_date"
	}
	allowedSorts := map[string]bool{
		"name":          true,
		"start_date":    true,
		"end_date":      true,
		"academic_year": true,
		"created_at":    true,
	}
	if !allowedSorts[sortBy] {
		sortBy = "start_date"
	}

	order := strings.ToUpper(filter.SortOrder)
	if order != "ASC" && order != "DESC" {
		order = "DESC"
	}

	page := filter.Page
	if page < 1 {
		page = 1
	}
	size := filter.PageSize
	if size <= 0 || size > 100 {
		size = 20
	}
	offset := (page - 1) * size

	query := fmt.Sprintf("SELECT id, name, type, academic_year, start_date, end_date, is_active, created_at, updated_at %s ORDER BY %s %s LIMIT %d OFFSET %d", base, sortBy, order, size, offset)

	var terms []models.Term
	if err := r.db.SelectContext(ctx, &terms, query, args...); err != nil {
		return nil, 0, fmt.Errorf("list terms: %w", err)
	}

	countQuery := fmt.Sprintf("SELECT COUNT(*) %s", base)
	var total int
	if err := r.db.GetContext(ctx, &total, countQuery, args...); err != nil {
		return nil, 0, fmt.Errorf("count terms: %w", err)
	}

	return terms, total, nil
}

// FindByID loads a term by identifier.
func (r *TermRepository) FindByID(ctx context.Context, id string) (*models.Term, error) {
	const query = `SELECT id, name, type, academic_year, start_date, end_date, is_active, created_at, updated_at FROM terms WHERE id = $1`
	var term models.Term
	if err := r.db.GetContext(ctx, &term, query, id); err != nil {
		return nil, err
	}
	return &term, nil
}

// FindActive returns the currently active term.
func (r *TermRepository) FindActive(ctx context.Context) (*models.Term, error) {
	const query = `SELECT id, name, type, academic_year, start_date, end_date, is_active, created_at, updated_at FROM terms WHERE is_active = TRUE LIMIT 1`
	var term models.Term
	if err := r.db.GetContext(ctx, &term, query); err != nil {
		return nil, err
	}
	return &term, nil
}

// ExistsByYearAndType checks if a term with the same academic year and type exists.
func (r *TermRepository) ExistsByYearAndType(ctx context.Context, academicYear string, termType models.TermType, excludeID string) (bool, error) {
	base := "SELECT 1 FROM terms WHERE academic_year = $1 AND type = $2"
	args := []interface{}{academicYear, termType}
	if excludeID != "" {
		base += " AND id <> $3"
		args = append(args, excludeID)
	}
	var exists int
	if err := r.db.GetContext(ctx, &exists, base+" LIMIT 1", args...); err != nil {
		if err == sql.ErrNoRows {
			return false, nil
		}
		return false, fmt.Errorf("check term uniqueness: %w", err)
	}
	return true, nil
}

// Create inserts a new term record.
func (r *TermRepository) Create(ctx context.Context, term *models.Term) error {
	if term.ID == "" {
		term.ID = uuid.NewString()
	}
	now := time.Now().UTC()
	if term.CreatedAt.IsZero() {
		term.CreatedAt = now
	}
	term.UpdatedAt = now

	const query = `INSERT INTO terms (id, name, type, academic_year, start_date, end_date, is_active, created_at, updated_at) VALUES (:id, :name, :type, :academic_year, :start_date, :end_date, :is_active, :created_at, :updated_at)`
	if _, err := r.db.NamedExecContext(ctx, query, term); err != nil {
		return fmt.Errorf("create term: %w", err)
	}
	return nil
}

// Update modifies an existing term.
func (r *TermRepository) Update(ctx context.Context, term *models.Term) error {
	term.UpdatedAt = time.Now().UTC()
	const query = `UPDATE terms SET name = :name, type = :type, academic_year = :academic_year, start_date = :start_date, end_date = :end_date, is_active = :is_active, updated_at = :updated_at WHERE id = :id`
	if _, err := r.db.NamedExecContext(ctx, query, term); err != nil {
		return fmt.Errorf("update term: %w", err)
	}
	return nil
}

// SetActive marks the provided term as active and deactivates the rest.
func (r *TermRepository) SetActive(ctx context.Context, id string) error {
	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin set active tx: %w", err)
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()

	if _, err = tx.ExecContext(ctx, `UPDATE terms SET is_active = FALSE, updated_at = $1 WHERE is_active = TRUE AND id <> $2`, time.Now().UTC(), id); err != nil {
		return fmt.Errorf("deactivate other terms: %w", err)
	}

	if _, err = tx.ExecContext(ctx, `UPDATE terms SET is_active = TRUE, updated_at = $2 WHERE id = $1`, id, time.Now().UTC()); err != nil {
		return fmt.Errorf("activate term: %w", err)
	}

	if err = tx.Commit(); err != nil {
		return fmt.Errorf("commit set active tx: %w", err)
	}
	return nil
}

// Delete removes a term permanently.
func (r *TermRepository) Delete(ctx context.Context, id string) error {
	if _, err := r.db.ExecContext(ctx, `DELETE FROM terms WHERE id = $1`, id); err != nil {
		return fmt.Errorf("delete term: %w", err)
	}
	return nil
}

// CountSchedules returns the number of schedules referencing the term.
func (r *TermRepository) CountSchedules(ctx context.Context, id string) (int, error) {
	const query = `SELECT COUNT(*) FROM schedules WHERE term_id = $1`
	var count int
	if err := r.db.GetContext(ctx, &count, query, id); err != nil {
		return 0, fmt.Errorf("count term schedules: %w", err)
	}
	return count, nil
}
