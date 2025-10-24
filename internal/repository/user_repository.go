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

// UserRepository provides database access for user management.
type UserRepository struct {
	db *sqlx.DB
}

// NewUserRepository creates a new instance of UserRepository.
func NewUserRepository(db *sqlx.DB) *UserRepository {
	return &UserRepository{db: db}
}

// FindByEmail returns a user by email address.
func (r *UserRepository) FindByEmail(ctx context.Context, email string) (*models.User, error) {
	const query = `SELECT id, email, password_hash, full_name, role, active, last_login, created_at, updated_at FROM users WHERE email = $1 LIMIT 1`
	var user models.User
	if err := r.db.GetContext(ctx, &user, query, email); err != nil {
		if err == sql.ErrNoRows {
			return nil, err
		}
		return nil, fmt.Errorf("find user by email: %w", err)
	}
	return &user, nil
}

// FindByID returns a user by identifier.
func (r *UserRepository) FindByID(ctx context.Context, id string) (*models.User, error) {
	const query = `SELECT id, email, password_hash, full_name, role, active, last_login, created_at, updated_at FROM users WHERE id = $1 LIMIT 1`
	var user models.User
	if err := r.db.GetContext(ctx, &user, query, id); err != nil {
		if err == sql.ErrNoRows {
			return nil, err
		}
		return nil, fmt.Errorf("find user by id: %w", err)
	}
	return &user, nil
}

// UpdateLastLogin updates the last_login timestamp for a user.
func (r *UserRepository) UpdateLastLogin(ctx context.Context, id string, ts time.Time) error {
	const query = `UPDATE users SET last_login = $2, updated_at = $3 WHERE id = $1`
	if _, err := r.db.ExecContext(ctx, query, id, ts, ts); err != nil {
		return fmt.Errorf("update last login: %w", err)
	}
	return nil
}

// UpdatePassword updates the stored password hash.
func (r *UserRepository) UpdatePassword(ctx context.Context, id, passwordHash string, updatedAt time.Time) error {
	const query = `UPDATE users SET password_hash = $2, updated_at = $3 WHERE id = $1`
	if _, err := r.db.ExecContext(ctx, query, id, passwordHash, updatedAt); err != nil {
		return fmt.Errorf("update password: %w", err)
	}
	return nil
}

// List returns users based on filters with total count.
func (r *UserRepository) List(ctx context.Context, filter models.UserFilter) ([]models.User, int, error) {
	baseQuery := `FROM users WHERE 1=1`
	var conditions []string
	var args []interface{}

	if filter.Role != nil {
		conditions = append(conditions, fmt.Sprintf("role = $%d", len(args)+1))
		args = append(args, *filter.Role)
	}
	if filter.Active != nil {
		conditions = append(conditions, fmt.Sprintf("active = $%d", len(args)+1))
		args = append(args, *filter.Active)
	}
	if filter.Search != "" {
		conditions = append(conditions, fmt.Sprintf("(LOWER(email) LIKE $%d OR LOWER(full_name) LIKE $%d)", len(args)+1, len(args)+1))
		args = append(args, "%"+strings.ToLower(filter.Search)+"%")
	}

	if len(conditions) > 0 {
		baseQuery += " AND " + strings.Join(conditions, " AND ")
	}

	sortBy := filter.SortBy
	if sortBy == "" {
		sortBy = "created_at"
	}
	allowedSorts := map[string]bool{
		"email":      true,
		"created_at": true,
		"updated_at": true,
		"full_name":  true,
	}
	if !allowedSorts[sortBy] {
		sortBy = "created_at"
	}

	sortOrder := strings.ToUpper(filter.SortOrder)
	if sortOrder != "ASC" && sortOrder != "DESC" {
		sortOrder = "DESC"
	}

	page := filter.Page
	if page < 1 {
		page = 1
	}
	pageSize := filter.PageSize
	if pageSize <= 0 || pageSize > 100 {
		pageSize = 20
	}
	offset := (page - 1) * pageSize

	listQuery := fmt.Sprintf("SELECT id, email, password_hash, full_name, role, active, last_login, created_at, updated_at %s ORDER BY %s %s LIMIT %d OFFSET %d", baseQuery, sortBy, sortOrder, pageSize, offset)

	var users []models.User
	if err := r.db.SelectContext(ctx, &users, listQuery, args...); err != nil {
		return nil, 0, fmt.Errorf("list users: %w", err)
	}

	countQuery := fmt.Sprintf("SELECT COUNT(*) %s", baseQuery)
	var total int
	if err := r.db.GetContext(ctx, &total, countQuery, args...); err != nil {
		return nil, 0, fmt.Errorf("count users: %w", err)
	}

	return users, total, nil
}

// Create inserts a new user and returns the stored record.
func (r *UserRepository) Create(ctx context.Context, user *models.User) error {
	if user.ID == "" {
		user.ID = uuid.NewString()
	}
	now := time.Now().UTC()
	if user.CreatedAt.IsZero() {
		user.CreatedAt = now
	}
	user.UpdatedAt = now

	const query = `INSERT INTO users (id, email, password_hash, full_name, role, active, created_at, updated_at) VALUES (:id, :email, :password_hash, :full_name, :role, :active, :created_at, :updated_at)`
	if _, err := r.db.NamedExecContext(ctx, query, user); err != nil {
		return fmt.Errorf("create user: %w", err)
	}
	return nil
}

// Update updates mutable fields of a user.
func (r *UserRepository) Update(ctx context.Context, user *models.User) error {
	user.UpdatedAt = time.Now().UTC()
	const query = `UPDATE users SET full_name = :full_name, role = :role, active = :active, updated_at = :updated_at WHERE id = :id`
	if _, err := r.db.NamedExecContext(ctx, query, user); err != nil {
		return fmt.Errorf("update user: %w", err)
	}
	return nil
}

// Delete performs a soft delete by marking the user inactive.
func (r *UserRepository) Delete(ctx context.Context, id string) error {
	const query = `UPDATE users SET active = FALSE, updated_at = $2 WHERE id = $1`
	if _, err := r.db.ExecContext(ctx, query, id, time.Now().UTC()); err != nil {
		return fmt.Errorf("delete user: %w", err)
	}
	return nil
}

// CreateRefreshToken persists a refresh token entry.
func (r *UserRepository) CreateRefreshToken(ctx context.Context, token *models.RefreshToken) error {
	if token.ID == "" {
		token.ID = uuid.NewString()
	}
	const query = `INSERT INTO refresh_tokens (id, user_id, token, expires_at, created_at, revoked, revoked_at, ip_address, user_agent) VALUES (:id, :user_id, :token, :expires_at, :created_at, :revoked, :revoked_at, :ip_address, :user_agent)`
	if token.CreatedAt.IsZero() {
		token.CreatedAt = time.Now().UTC()
	}
	if _, err := r.db.NamedExecContext(ctx, query, token); err != nil {
		return fmt.Errorf("create refresh token: %w", err)
	}
	return nil
}

// FindRefreshToken returns a refresh token by token string.
func (r *UserRepository) FindRefreshToken(ctx context.Context, token string) (*models.RefreshToken, error) {
	const query = `SELECT id, user_id, token, expires_at, created_at, revoked, revoked_at, ip_address, user_agent FROM refresh_tokens WHERE token = $1 LIMIT 1`
	var rt models.RefreshToken
	if err := r.db.GetContext(ctx, &rt, query, token); err != nil {
		if err == sql.ErrNoRows {
			return nil, err
		}
		return nil, fmt.Errorf("find refresh token: %w", err)
	}
	return &rt, nil
}

// RevokeRefreshToken marks a token as revoked.
func (r *UserRepository) RevokeRefreshToken(ctx context.Context, id string, revokedAt time.Time) error {
	const query = `UPDATE refresh_tokens SET revoked = TRUE, revoked_at = $2 WHERE id = $1`
	if _, err := r.db.ExecContext(ctx, query, id, revokedAt); err != nil {
		return fmt.Errorf("revoke refresh token: %w", err)
	}
	return nil
}

// RevokeUserRefreshTokens revokes all refresh tokens for a user.
func (r *UserRepository) RevokeUserRefreshTokens(ctx context.Context, userID string) error {
	const query = `UPDATE refresh_tokens SET revoked = TRUE, revoked_at = $2 WHERE user_id = $1 AND revoked = FALSE`
	if _, err := r.db.ExecContext(ctx, query, userID, time.Now().UTC()); err != nil {
		return fmt.Errorf("revoke user refresh tokens: %w", err)
	}
	return nil
}

// CreateAuditLog stores an audit log entry.
func (r *UserRepository) CreateAuditLog(ctx context.Context, log *models.AuditLog) error {
	if log.ID == "" {
		log.ID = uuid.NewString()
	}
	if log.CreatedAt.IsZero() {
		log.CreatedAt = time.Now().UTC()
	}
	const query = `INSERT INTO audit_logs (id, user_id, action, resource, resource_id, old_values, new_values, ip_address, user_agent, created_at) VALUES (:id, :user_id, :action, :resource, :resource_id, :old_values, :new_values, :ip_address, :user_agent, :created_at)`
	if _, err := r.db.NamedExecContext(ctx, query, log); err != nil {
		return fmt.Errorf("create audit log: %w", err)
	}
	return nil
}
