package repository

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/jmoiron/sqlx"

	"github.com/noah-isme/sma-adp-api/internal/models"
)

// AuditRepository provides read access to the audit trail. Writes stay on
// UserRepository.CreateAuditLog so existing emit paths keep working unchanged.
type AuditRepository struct {
	db *sqlx.DB
}

// NewAuditRepository constructs the repository.
func NewAuditRepository(db *sqlx.DB) *AuditRepository {
	return &AuditRepository{db: db}
}

const auditSelectColumns = `al.id, al.user_id, al.action, al.resource, al.resource_id,
	al.old_values, al.new_values, al.ip_address, al.user_agent, al.created_at,
	u.email AS user_email, u.full_name AS user_full_name, u.role AS user_role`

const auditFromClause = `FROM audit_logs al LEFT JOIN users u ON u.id = al.user_id`

// auditSortColumns whitelists ORDER BY targets so the sort parameter can never
// be interpolated into arbitrary SQL.
var auditSortColumns = map[string]string{
	"created_at": "al.created_at",
	"action":     "al.action",
	"resource":   "al.resource",
	"user_id":    "al.user_id",
}

// buildAuditWhere renders the shared WHERE clause and positional args used by
// both the list and count queries.
func buildAuditWhere(filter models.AuditLogFilter) (string, []interface{}) {
	conditions := []string{"1=1"}
	args := []interface{}{}

	if filter.UserID != "" {
		conditions = append(conditions, fmt.Sprintf("al.user_id = $%d", len(args)+1))
		args = append(args, filter.UserID)
	}
	if filter.Action != "" {
		conditions = append(conditions, fmt.Sprintf("al.action = $%d", len(args)+1))
		args = append(args, strings.ToUpper(filter.Action))
	}
	if filter.Resource != "" {
		conditions = append(conditions, fmt.Sprintf("al.resource = $%d", len(args)+1))
		args = append(args, filter.Resource)
	}
	if filter.ResourceID != "" {
		conditions = append(conditions, fmt.Sprintf("al.resource_id = $%d", len(args)+1))
		args = append(args, filter.ResourceID)
	}
	if filter.DateFrom != nil {
		conditions = append(conditions, fmt.Sprintf("al.created_at >= $%d", len(args)+1))
		args = append(args, *filter.DateFrom)
	}
	if filter.DateTo != nil {
		conditions = append(conditions, fmt.Sprintf("al.created_at <= $%d", len(args)+1))
		args = append(args, *filter.DateTo)
	}
	if filter.Search != "" {
		placeholder := len(args) + 1
		conditions = append(conditions, fmt.Sprintf(
			"(LOWER(al.action) LIKE $%d OR LOWER(al.resource) LIKE $%d OR LOWER(COALESCE(u.email, '')) LIKE $%d OR LOWER(COALESCE(u.full_name, '')) LIKE $%d)",
			placeholder, placeholder, placeholder, placeholder))
		args = append(args, "%"+strings.ToLower(filter.Search)+"%")
	}

	return strings.Join(conditions, " AND "), args
}

// List returns a page of audit entries plus the total match count.
func (r *AuditRepository) List(ctx context.Context, filter models.AuditLogFilter) ([]models.AuditLogEntry, int, error) {
	whereClause, args := buildAuditWhere(filter)

	sortColumn, ok := auditSortColumns[strings.ToLower(filter.SortBy)]
	if !ok {
		sortColumn = "al.created_at"
	}
	sortOrder := strings.ToUpper(filter.SortOrder)
	if sortOrder != "ASC" && sortOrder != "DESC" {
		sortOrder = "DESC"
	}

	page := filter.Page
	if page < 1 {
		page = 1
	}
	size := filter.PageSize
	if size <= 0 || size > 200 {
		size = 50
	}
	offset := (page - 1) * size

	listQuery := fmt.Sprintf(
		"SELECT %s %s WHERE %s ORDER BY %s %s LIMIT $%d OFFSET $%d",
		auditSelectColumns, auditFromClause, whereClause, sortColumn, sortOrder, len(args)+1, len(args)+2)

	listArgs := append(append([]interface{}{}, args...), size, offset)

	var entries []models.AuditLogEntry
	if err := r.db.SelectContext(ctx, &entries, listQuery, listArgs...); err != nil {
		return nil, 0, fmt.Errorf("list audit logs: %w", err)
	}

	countQuery := fmt.Sprintf("SELECT COUNT(*) %s WHERE %s", auditFromClause, whereClause)
	var total int
	if err := r.db.GetContext(ctx, &total, countQuery, args...); err != nil {
		return nil, 0, fmt.Errorf("count audit logs: %w", err)
	}

	return entries, total, nil
}

// FindByID loads a single audit entry, returning sql.ErrNoRows when absent so
// callers can map it to a 404 the same way the other repositories do.
func (r *AuditRepository) FindByID(ctx context.Context, id string) (*models.AuditLogEntry, error) {
	query := fmt.Sprintf("SELECT %s %s WHERE al.id = $1", auditSelectColumns, auditFromClause)
	var entry models.AuditLogEntry
	if err := r.db.GetContext(ctx, &entry, query, id); err != nil {
		if err == sql.ErrNoRows {
			return nil, err
		}
		return nil, fmt.Errorf("find audit log: %w", err)
	}
	return &entry, nil
}

// Facets returns the distinct action and resource values with counts so the UI
// can populate filter dropdowns from real data instead of a hardcoded list.
func (r *AuditRepository) Facets(ctx context.Context) (*models.AuditLogFacets, error) {
	actions, err := r.facet(ctx, "action")
	if err != nil {
		return nil, err
	}
	resources, err := r.facet(ctx, "resource")
	if err != nil {
		return nil, err
	}
	return &models.AuditLogFacets{Actions: actions, Resources: resources}, nil
}

// facet aggregates one whitelisted column. The column name is never taken from
// user input; callers pass a literal.
func (r *AuditRepository) facet(ctx context.Context, column string) ([]models.AuditLogFacetCount, error) {
	if column != "action" && column != "resource" {
		return nil, fmt.Errorf("unsupported audit facet column %q", column)
	}
	query := fmt.Sprintf(
		"SELECT %s AS value, COUNT(*) AS count FROM audit_logs GROUP BY %s ORDER BY count DESC, value ASC",
		column, column)
	var rows []models.AuditLogFacetCount
	if err := r.db.SelectContext(ctx, &rows, query); err != nil {
		return nil, fmt.Errorf("audit facet %s: %w", column, err)
	}
	return rows, nil
}
