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

// ParentStudentRepository provides database access for parent-student relationships.
type ParentStudentRepository struct {
	db *sqlx.DB
}

// NewParentStudentRepository creates a new instance of ParentStudentRepository.
func NewParentStudentRepository(db *sqlx.DB) *ParentStudentRepository {
	return &ParentStudentRepository{db: db}
}

// FindByID returns a parent-student link by ID.
func (r *ParentStudentRepository) FindByID(ctx context.Context, id string) (*models.ParentStudentLink, error) {
	const query = `
		SELECT id, parent_id, student_id, relationship, can_view_grades, can_view_attendance, 
		       can_view_behavior, can_view_announcements, can_receive_notifications, created_at, updated_at
		FROM parent_students WHERE id = $1 LIMIT 1
	`
	var link models.ParentStudentLink
	if err := r.db.GetContext(ctx, &link, query, id); err != nil {
		if err == sql.ErrNoRows {
			return nil, err
		}
		return nil, fmt.Errorf("find parent student link by id: %w", err)
	}
	return &link, nil
}

// FindByParentAndStudent returns a link by parent and student IDs.
func (r *ParentStudentRepository) FindByParentAndStudent(ctx context.Context, parentID, studentID string) (*models.ParentStudentLink, error) {
	const query = `
		SELECT id, parent_id, student_id, relationship, can_view_grades, can_view_attendance, 
		       can_view_behavior, can_view_announcements, can_receive_notifications, created_at, updated_at
		FROM parent_students WHERE parent_id = $1 AND student_id = $2 LIMIT 1
	`
	var link models.ParentStudentLink
	if err := r.db.GetContext(ctx, &link, query, parentID, studentID); err != nil {
		if err == sql.ErrNoRows {
			return nil, err
		}
		return nil, fmt.Errorf("find parent student link: %w", err)
	}
	return &link, nil
}

// FindByParentID returns all links for a parent.
func (r *ParentStudentRepository) FindByParentID(ctx context.Context, parentID string) ([]*models.ParentStudentLink, error) {
	const query = `
		SELECT id, parent_id, student_id, relationship, can_view_grades, can_view_attendance, 
		       can_view_behavior, can_view_announcements, can_receive_notifications, created_at, updated_at
		FROM parent_students WHERE parent_id = $1 ORDER BY created_at DESC
	`
	var links []*models.ParentStudentLink
	if err := r.db.SelectContext(ctx, &links, query, parentID); err != nil {
		return nil, fmt.Errorf("find parent student links by parent: %w", err)
	}
	return links, nil
}

// FindByStudentID returns all links for a student.
func (r *ParentStudentRepository) FindByStudentID(ctx context.Context, studentID string) ([]*models.ParentStudentLink, error) {
	const query = `
		SELECT id, parent_id, student_id, relationship, can_view_grades, can_view_attendance, 
		       can_view_behavior, can_view_announcements, can_receive_notifications, created_at, updated_at
		FROM parent_students WHERE student_id = $1 ORDER BY created_at DESC
	`
	var links []*models.ParentStudentLink
	if err := r.db.SelectContext(ctx, &links, query, studentID); err != nil {
		return nil, fmt.Errorf("find parent student links by student: %w", err)
	}
	return links, nil
}

// List returns parent-student links with filtering and pagination.
func (r *ParentStudentRepository) List(ctx context.Context, filter models.ParentStudentLinkFilter) ([]*models.ParentStudentLink, int, error) {
	baseQuery := `
		FROM parent_students WHERE 1=1
	`
	var conditions []string
	var args []interface{}

	if filter.ParentID != nil {
		conditions = append(conditions, fmt.Sprintf("parent_id = $%d", len(args)+1))
		args = append(args, *filter.ParentID)
	}
	if filter.StudentID != nil {
		conditions = append(conditions, fmt.Sprintf("student_id = $%d", len(args)+1))
		args = append(args, *filter.StudentID)
	}
	if filter.Relationship != nil {
		conditions = append(conditions, fmt.Sprintf("relationship = $%d", len(args)+1))
		args = append(args, *filter.Relationship)
	}

	if len(conditions) > 0 {
		baseQuery += " AND " + strings.Join(conditions, " AND ")
	}

	sortBy := filter.SortBy
	if sortBy == "" {
		sortBy = "created_at"
	}
	allowedSorts := map[string]bool{
		"created_at":   true,
		"updated_at":   true,
		"relationship": true,
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

	listQuery := fmt.Sprintf(`
		SELECT id, parent_id, student_id, relationship, can_view_grades, can_view_attendance, 
		       can_view_behavior, can_view_announcements, can_receive_notifications, created_at, updated_at
		%s ORDER BY %s %s LIMIT %d OFFSET %d
	`, baseQuery, sortBy, sortOrder, pageSize, offset)

	var links []*models.ParentStudentLink
	if err := r.db.SelectContext(ctx, &links, listQuery, args...); err != nil {
		return nil, 0, fmt.Errorf("list parent student links: %w", err)
	}

	countQuery := fmt.Sprintf("SELECT COUNT(*) %s", baseQuery)
	var total int
	if err := r.db.GetContext(ctx, &total, countQuery, args...); err != nil {
		return nil, 0, fmt.Errorf("count parent student links: %w", err)
	}

	return links, total, nil
}

// Create inserts a new parent-student link.
func (r *ParentStudentRepository) Create(ctx context.Context, link *models.ParentStudentLink) error {
	if link.ID == "" {
		link.ID = uuid.NewString()
	}
	now := time.Now().UTC()
	if link.CreatedAt.IsZero() {
		link.CreatedAt = now
	}
	link.UpdatedAt = now

	const query = `
		INSERT INTO parent_students (id, parent_id, student_id, relationship, can_view_grades, can_view_attendance, 
			can_view_behavior, can_view_announcements, can_receive_notifications, created_at, updated_at)
		VALUES (:id, :parent_id, :student_id, :relationship, :can_view_grades, :can_view_attendance, 
			:can_view_behavior, :can_view_announcements, :can_receive_notifications, :created_at, :updated_at)
	`
	if _, err := r.db.NamedExecContext(ctx, query, link); err != nil {
		return fmt.Errorf("create parent student link: %w", err)
	}
	return nil
}

// Update updates a parent-student link.
func (r *ParentStudentRepository) Update(ctx context.Context, link *models.ParentStudentLink) error {
	link.UpdatedAt = time.Now().UTC()
	const query = `
		UPDATE parent_students SET 
			relationship = :relationship, can_view_grades = :can_view_grades, 
			can_view_attendance = :can_view_attendance, can_view_behavior = :can_view_behavior,
			can_view_announcements = :can_view_announcements, can_receive_notifications = :can_receive_notifications,
			updated_at = :updated_at
		WHERE id = :id
	`
	if _, err := r.db.NamedExecContext(ctx, query, link); err != nil {
		return fmt.Errorf("update parent student link: %w", err)
	}
	return nil
}

// Delete removes a parent-student link.
func (r *ParentStudentRepository) Delete(ctx context.Context, id string) error {
	const query = `DELETE FROM parent_students WHERE id = $1`
	if _, err := r.db.ExecContext(ctx, query, id); err != nil {
		return fmt.Errorf("delete parent student link: %w", err)
	}
	return nil
}

// PortalPreferencesRepository provides database access for portal preferences.
type PortalPreferencesRepository struct {
	db *sqlx.DB
}

// NewPortalPreferencesRepository creates a new instance of PortalPreferencesRepository.
func NewPortalPreferencesRepository(db *sqlx.DB) *PortalPreferencesRepository {
	return &PortalPreferencesRepository{db: db}
}

// FindByUserID returns preferences for a user.
func (r *PortalPreferencesRepository) FindByUserID(ctx context.Context, userID string) (*models.PortalPreferences, error) {
	const query = `
		SELECT user_id, language, timezone, email_notifications, push_notifications, 
		       sms_notifications, grade_alerts, attendance_alerts, behavior_alerts, 
		       announcement_alerts, created_at, updated_at
		FROM portal_preferences WHERE user_id = $1 LIMIT 1
	`
	var prefs models.PortalPreferences
	if err := r.db.GetContext(ctx, &prefs, query, userID); err != nil {
		if err == sql.ErrNoRows {
			return nil, err
		}
		return nil, fmt.Errorf("find portal preferences: %w", err)
	}
	return &prefs, nil
}

// Create inserts new portal preferences.
func (r *PortalPreferencesRepository) Create(ctx context.Context, prefs *models.PortalPreferences) error {
	now := time.Now().UTC()
	if prefs.CreatedAt.IsZero() {
		prefs.CreatedAt = now
	}
	prefs.UpdatedAt = now

	const query = `
		INSERT INTO portal_preferences (user_id, language, timezone, email_notifications, push_notifications, 
			sms_notifications, grade_alerts, attendance_alerts, behavior_alerts, announcement_alerts, created_at, updated_at)
		VALUES (:user_id, :language, :timezone, :email_notifications, :push_notifications, 
			:sms_notifications, :grade_alerts, :attendance_alerts, :behavior_alerts, :announcement_alerts, :created_at, :updated_at)
	`
	if _, err := r.db.NamedExecContext(ctx, query, prefs); err != nil {
		return fmt.Errorf("create portal preferences: %w", err)
	}
	return nil
}

// Update updates portal preferences.
func (r *PortalPreferencesRepository) Update(ctx context.Context, prefs *models.PortalPreferences) error {
	prefs.UpdatedAt = time.Now().UTC()
	const query = `
		UPDATE portal_preferences SET 
			language = :language, timezone = :timezone, email_notifications = :email_notifications,
			push_notifications = :push_notifications, sms_notifications = :sms_notifications,
			grade_alerts = :grade_alerts, attendance_alerts = :attendance_alerts,
			behavior_alerts = :behavior_alerts, announcement_alerts = :announcement_alerts,
			updated_at = :updated_at
		WHERE user_id = :user_id
	`
	if _, err := r.db.NamedExecContext(ctx, query, prefs); err != nil {
		return fmt.Errorf("update portal preferences: %w", err)
	}
	return nil
}

// Upsert creates or updates portal preferences.
func (r *PortalPreferencesRepository) Upsert(ctx context.Context, prefs *models.PortalPreferences) error {
	existing, err := r.FindByUserID(ctx, prefs.UserID)
	if err != nil && err != sql.ErrNoRows {
		return err
	}
	if existing == nil {
		return r.Create(ctx, prefs)
	}
	return r.Update(ctx, prefs)
}

// DeviceTokenRepository provides database access for device tokens.
type DeviceTokenRepository struct {
	db *sqlx.DB
}

// NewDeviceTokenRepository creates a new instance of DeviceTokenRepository.
func NewDeviceTokenRepository(db *sqlx.DB) *DeviceTokenRepository {
	return &DeviceTokenRepository{db: db}
}

// FindByID returns a device token by ID.
func (r *DeviceTokenRepository) FindByID(ctx context.Context, id string) (*models.DeviceToken, error) {
	const query = `
		SELECT id, user_id, token, platform, device_id, app_version, last_used_at, created_at
		FROM device_tokens WHERE id = $1 LIMIT 1
	`
	var dt models.DeviceToken
	if err := r.db.GetContext(ctx, &dt, query, id); err != nil {
		if err == sql.ErrNoRows {
			return nil, err
		}
		return nil, fmt.Errorf("find device token by id: %w", err)
	}
	return &dt, nil
}

// FindByUserID returns all device tokens for a user.
func (r *DeviceTokenRepository) FindByUserID(ctx context.Context, userID string) ([]*models.DeviceToken, error) {
	const query = `
		SELECT id, user_id, token, platform, device_id, app_version, last_used_at, created_at
		FROM device_tokens WHERE user_id = $1 ORDER BY last_used_at DESC
	`
	var tokens []*models.DeviceToken
	if err := r.db.SelectContext(ctx, &tokens, query, userID); err != nil {
		return nil, fmt.Errorf("find device tokens by user: %w", err)
	}
	return tokens, nil
}

// FindByToken returns a device token by token string.
func (r *DeviceTokenRepository) FindByToken(ctx context.Context, token string) (*models.DeviceToken, error) {
	const query = `
		SELECT id, user_id, token, platform, device_id, app_version, last_used_at, created_at
		FROM device_tokens WHERE token = $1 LIMIT 1
	`
	var dt models.DeviceToken
	if err := r.db.GetContext(ctx, &dt, query, token); err != nil {
		if err == sql.ErrNoRows {
			return nil, err
		}
		return nil, fmt.Errorf("find device token by token: %w", err)
	}
	return &dt, nil
}

// Create inserts a new device token.
func (r *DeviceTokenRepository) Create(ctx context.Context, dt *models.DeviceToken) error {
	if dt.ID == "" {
		dt.ID = uuid.NewString()
	}
	now := time.Now().UTC()
	dt.LastUsedAt = now
	if dt.CreatedAt.IsZero() {
		dt.CreatedAt = now
	}

	const query = `
		INSERT INTO device_tokens (id, user_id, token, platform, device_id, app_version, last_used_at, created_at)
		VALUES (:id, :user_id, :token, :platform, :device_id, :app_version, :last_used_at, :created_at)
	`
	if _, err := r.db.NamedExecContext(ctx, query, dt); err != nil {
		return fmt.Errorf("create device token: %w", err)
	}
	return nil
}

// UpdateLastUsed updates the last_used_at timestamp.
func (r *DeviceTokenRepository) UpdateLastUsed(ctx context.Context, id string, ts time.Time) error {
	const query = `UPDATE device_tokens SET last_used_at = $2 WHERE id = $1`
	if _, err := r.db.ExecContext(ctx, query, id, ts); err != nil {
		return fmt.Errorf("update device token last used: %w", err)
	}
	return nil
}

// Delete removes a device token.
func (r *DeviceTokenRepository) Delete(ctx context.Context, id string) error {
	const query = `DELETE FROM device_tokens WHERE id = $1`
	if _, err := r.db.ExecContext(ctx, query, id); err != nil {
		return fmt.Errorf("delete device token: %w", err)
	}
	return nil
}

// DeleteByUserIDAndToken removes a device token by user and token.
func (r *DeviceTokenRepository) DeleteByUserIDAndToken(ctx context.Context, userID, token string) error {
	const query = `DELETE FROM device_tokens WHERE user_id = $1 AND token = $2`
	if _, err := r.db.ExecContext(ctx, query, userID, token); err != nil {
		return fmt.Errorf("delete device token by user and token: %w", err)
	}
	return nil
}
