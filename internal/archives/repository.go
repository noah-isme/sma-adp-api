package archives

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/lib/pq"
)

// Repository interface defines the storage contract for the Archives domain.
type Repository interface {
	CreateDocument(ctx context.Context, doc *ArchiveDocument) error
	GetDocumentByID(ctx context.Context, id uuid.UUID) (*ArchiveDocument, error)
	UpdateDocument(ctx context.Context, doc *ArchiveDocument) error
	SoftDeleteDocument(ctx context.Context, id uuid.UUID) error
	ListDocuments(ctx context.Context, filter ArchiveFilter) ([]*ArchiveDocument, int64, error)
	GetDocumentsBySubject(ctx context.Context, subjectID string) ([]*ArchiveDocument, error)

	GetRetentionPolicyByID(ctx context.Context, id uuid.UUID) (*RetentionPolicy, error)
	GetDefaultPolicyByCategory(ctx context.Context, category DocumentCategory) (*RetentionPolicy, error)
	ListRetentionPolicies(ctx context.Context) ([]*RetentionPolicy, error)
	CreateRetentionPolicy(ctx context.Context, policy *RetentionPolicy) error

	GetExpiredDocuments(ctx context.Context, limit int) ([]*ArchiveDocument, error)
	GetDocumentsForTierMigration(ctx context.Context, currentTier StorageTier, olderThanDays int) ([]*ArchiveDocument, error)

	CreateAuditLog(ctx context.Context, log *AuditLog) error
	GetAuditLogsByDocument(ctx context.Context, docID uuid.UUID) ([]*AuditLog, error)
	ListAuditLogs(ctx context.Context, filter AuditLogFilter) ([]*AuditLog, int64, error)
}

// -----------------------------------------------------------------------------
// PostgresRepository Implementation
// -----------------------------------------------------------------------------

// PostgresRepository implements Repository interface using sqlx.DB.
type PostgresRepository struct {
	db *sqlx.DB
}

// NewPostgresRepository constructs a new PostgresRepository instance.
func NewPostgresRepository(db *sqlx.DB) *PostgresRepository {
	return &PostgresRepository{db: db}
}

// CreateDocument inserts a new archive document record into PostgreSQL.
func (r *PostgresRepository) CreateDocument(ctx context.Context, doc *ArchiveDocument) error {
	if doc.ID == uuid.Nil {
		doc.ID = uuid.New()
	}
	now := time.Now().UTC()
	if doc.UploadedAt.IsZero() {
		doc.UploadedAt = now
	}
	doc.UpdatedAt = now

	const query = `
		INSERT INTO archive_documents (
			id, filename, original_filename, mime_type, size_bytes, checksum,
			storage_path, storage_tier, category, tags, metadata, ocr_text,
			ocr_status, retention_policy_id, retain_until, legal_hold,
			legal_hold_reason, uploaded_by, uploaded_at, updated_at
		) VALUES (
			:id, :filename, :original_filename, :mime_type, :size_bytes, :checksum,
			:storage_path, :storage_tier, :category, :tags, :metadata, :ocr_text,
			:ocr_status, :retention_policy_id, :retain_until, :legal_hold,
			:legal_hold_reason, :uploaded_by, :uploaded_at, :updated_at
		)`

	_, err := r.db.NamedExecContext(ctx, query, doc)
	if err != nil {
		return fmt.Errorf("create document: %w", err)
	}
	return nil
}

// GetDocumentByID retrieves a single document by ID from PostgreSQL.
func (r *PostgresRepository) GetDocumentByID(ctx context.Context, id uuid.UUID) (*ArchiveDocument, error) {
	const query = `
		SELECT id, filename, original_filename, mime_type, size_bytes, checksum,
		       storage_path, storage_tier, category, tags, metadata, ocr_text,
		       ocr_status, retention_policy_id, retain_until, legal_hold,
		       legal_hold_reason, uploaded_by, uploaded_at, updated_at, deleted_at
		FROM archive_documents
		WHERE id = $1`

	var doc ArchiveDocument
	if err := r.db.GetContext(ctx, &doc, query, id); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrDocumentNotFound
		}
		return nil, fmt.Errorf("get document by id: %w", err)
	}
	return &doc, nil
}

// UpdateDocument updates an existing document record in PostgreSQL.
func (r *PostgresRepository) UpdateDocument(ctx context.Context, doc *ArchiveDocument) error {
	doc.UpdatedAt = time.Now().UTC()

	const query = `
		UPDATE archive_documents SET
			filename = :filename,
			original_filename = :original_filename,
			mime_type = :mime_type,
			size_bytes = :size_bytes,
			checksum = :checksum,
			storage_path = :storage_path,
			storage_tier = :storage_tier,
			category = :category,
			tags = :tags,
			metadata = :metadata,
			ocr_text = :ocr_text,
			ocr_status = :ocr_status,
			retention_policy_id = :retention_policy_id,
			retain_until = :retain_until,
			legal_hold = :legal_hold,
			legal_hold_reason = :legal_hold_reason,
			updated_at = :updated_at
		WHERE id = :id AND deleted_at IS NULL`

	res, err := r.db.NamedExecContext(ctx, query, doc)
	if err != nil {
		return fmt.Errorf("update document: %w", err)
	}
	rows, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("check update rows: %w", err)
	}
	if rows == 0 {
		return ErrDocumentNotFound
	}
	return nil
}

// SoftDeleteDocument sets deleted_at timestamp for a document in PostgreSQL.
func (r *PostgresRepository) SoftDeleteDocument(ctx context.Context, id uuid.UUID) error {
	const query = `
		UPDATE archive_documents
		SET deleted_at = NOW(), updated_at = NOW()
		WHERE id = $1 AND deleted_at IS NULL`

	res, err := r.db.ExecContext(ctx, query, id)
	if err != nil {
		return fmt.Errorf("soft delete document: %w", err)
	}
	rows, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("check soft delete rows: %w", err)
	}
	if rows == 0 {
		return ErrDocumentNotFound
	}
	return nil
}

// ListDocuments lists documents matching filter criteria in PostgreSQL.
func (r *PostgresRepository) ListDocuments(ctx context.Context, filter ArchiveFilter) ([]*ArchiveDocument, int64, error) {
	where := []string{}
	args := []interface{}{}
	argIdx := 1

	if !filter.IncludeDeleted {
		where = append(where, "deleted_at IS NULL")
	}
	if filter.Category != "" {
		where = append(where, fmt.Sprintf("category = $%d", argIdx))
		args = append(args, filter.Category)
		argIdx++
	}
	if filter.StorageTier != "" {
		where = append(where, fmt.Sprintf("storage_tier = $%d", argIdx))
		args = append(args, filter.StorageTier)
		argIdx++
	}
	if filter.LegalHoldOnly {
		where = append(where, "legal_hold = TRUE")
	}
	if filter.OCRCompleted {
		where = append(where, "ocr_status = 'COMPLETED'")
	}
	if filter.Query != "" {
		where = append(where, fmt.Sprintf("(LOWER(filename) LIKE $%d OR LOWER(ocr_text) LIKE $%d)", argIdx, argIdx))
		args = append(args, "%"+strings.ToLower(filter.Query)+"%")
		argIdx++
	}
	if filter.DateFrom != nil {
		where = append(where, fmt.Sprintf("uploaded_at >= $%d", argIdx))
		args = append(args, *filter.DateFrom)
		argIdx++
	}
	if filter.DateTo != nil {
		where = append(where, fmt.Sprintf("uploaded_at <= $%d", argIdx))
		args = append(args, *filter.DateTo)
		argIdx++
	}
	if len(filter.Tags) > 0 {
		where = append(where, fmt.Sprintf("tags @> $%d", argIdx))
		args = append(args, pq.Array(filter.Tags))
		argIdx++
	}

	whereClause := ""
	if len(where) > 0 {
		whereClause = "WHERE " + strings.Join(where, " AND ")
	}

	countQuery := fmt.Sprintf("SELECT COUNT(*) FROM archive_documents %s", whereClause)
	var total int64
	if err := r.db.GetContext(ctx, &total, countQuery, args...); err != nil {
		return nil, 0, fmt.Errorf("count documents: %w", err)
	}

	limit := filter.Limit
	if limit <= 0 || limit > 500 {
		limit = 50
	}
	offset := filter.Offset
	if offset < 0 {
		offset = 0
	}

	selectQuery := fmt.Sprintf(`
		SELECT id, filename, original_filename, mime_type, size_bytes, checksum,
		       storage_path, storage_tier, category, tags, metadata, ocr_text,
		       ocr_status, retention_policy_id, retain_until, legal_hold,
		       legal_hold_reason, uploaded_by, uploaded_at, updated_at, deleted_at
		FROM archive_documents
		%s
		ORDER BY uploaded_at DESC
		LIMIT %d OFFSET %d`, whereClause, limit, offset)

	var docs []*ArchiveDocument
	if err := r.db.SelectContext(ctx, &docs, selectQuery, args...); err != nil {
		return nil, 0, fmt.Errorf("list documents: %w", err)
	}

	return docs, total, nil
}

// GetRetentionPolicyByID retrieves a retention policy by ID from PostgreSQL.
func (r *PostgresRepository) GetRetentionPolicyByID(ctx context.Context, id uuid.UUID) (*RetentionPolicy, error) {
	const query = `
		SELECT id, name, category, retention_years, auto_delete,
		       legal_hold_override, description, created_at, updated_at
		FROM retention_policies
		WHERE id = $1`

	var pol RetentionPolicy
	if err := r.db.GetContext(ctx, &pol, query, id); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrPolicyNotFound
		}
		return nil, fmt.Errorf("get retention policy by id: %w", err)
	}
	return &pol, nil
}

// GetDefaultPolicyByCategory retrieves the retention policy for a specific category.
func (r *PostgresRepository) GetDefaultPolicyByCategory(ctx context.Context, category DocumentCategory) (*RetentionPolicy, error) {
	const query = `
		SELECT id, name, category, retention_years, auto_delete,
		       legal_hold_override, description, created_at, updated_at
		FROM retention_policies
		WHERE category = $1
		ORDER BY created_at ASC
		LIMIT 1`

	var pol RetentionPolicy
	if err := r.db.GetContext(ctx, &pol, query, category); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrPolicyNotFound
		}
		return nil, fmt.Errorf("get default policy by category: %w", err)
	}
	return &pol, nil
}

// ListRetentionPolicies retrieves all configured retention policies from PostgreSQL.
func (r *PostgresRepository) ListRetentionPolicies(ctx context.Context) ([]*RetentionPolicy, error) {
	const query = `
		SELECT id, name, category, retention_years, auto_delete,
		       legal_hold_override, description, created_at, updated_at
		FROM retention_policies
		ORDER BY category ASC, name ASC`

	var policies []*RetentionPolicy
	if err := r.db.SelectContext(ctx, &policies, query); err != nil {
		return nil, fmt.Errorf("list retention policies: %w", err)
	}
	return policies, nil
}

// CreateRetentionPolicy inserts a new retention policy record into PostgreSQL.
func (r *PostgresRepository) CreateRetentionPolicy(ctx context.Context, policy *RetentionPolicy) error {
	if policy.ID == uuid.Nil {
		policy.ID = uuid.New()
	}
	now := time.Now().UTC()
	if policy.CreatedAt.IsZero() {
		policy.CreatedAt = now
	}
	policy.UpdatedAt = now

	const query = `
		INSERT INTO retention_policies (
			id, name, category, retention_years, auto_delete,
			legal_hold_override, description, created_at, updated_at
		) VALUES (
			:id, :name, :category, :retention_years, :auto_delete,
			:legal_hold_override, :description, :created_at, :updated_at
		)`

	_, err := r.db.NamedExecContext(ctx, query, policy)
	if err != nil {
		return fmt.Errorf("create retention policy: %w", err)
	}
	return nil
}

// GetExpiredDocuments returns documents past retain_until date for retention engine processing.
func (r *PostgresRepository) GetExpiredDocuments(ctx context.Context, limit int) ([]*ArchiveDocument, error) {
	if limit <= 0 || limit > 1000 {
		limit = 100
	}

	const query = `
		SELECT id, filename, original_filename, mime_type, size_bytes, checksum,
		       storage_path, storage_tier, category, tags, metadata, ocr_text,
		       ocr_status, retention_policy_id, retain_until, legal_hold,
		       legal_hold_reason, uploaded_by, uploaded_at, updated_at, deleted_at
		FROM archive_documents
		WHERE retain_until <= CURRENT_DATE AND deleted_at IS NULL
		ORDER BY retain_until ASC
		LIMIT $1`

	var docs []*ArchiveDocument
	if err := r.db.SelectContext(ctx, &docs, query, limit); err != nil {
		return nil, fmt.Errorf("get expired documents: %w", err)
	}
	return docs, nil
}

// GetDocumentsForTierMigration retrieves documents in currentTier uploaded older than olderThanDays.
func (r *PostgresRepository) GetDocumentsForTierMigration(ctx context.Context, currentTier StorageTier, olderThanDays int) ([]*ArchiveDocument, error) {
	const query = `
		SELECT id, filename, original_filename, mime_type, size_bytes, checksum,
		       storage_path, storage_tier, category, tags, metadata, ocr_text,
		       ocr_status, retention_policy_id, retain_until, legal_hold,
		       legal_hold_reason, uploaded_by, uploaded_at, updated_at, deleted_at
		FROM archive_documents
		WHERE storage_tier = $1 
		  AND uploaded_at <= NOW() - ($2 || ' days')::INTERVAL 
		  AND deleted_at IS NULL
		ORDER BY uploaded_at ASC
		LIMIT 200`

	var docs []*ArchiveDocument
	if err := r.db.SelectContext(ctx, &docs, query, currentTier, olderThanDays); err != nil {
		return nil, fmt.Errorf("get documents for tier migration: %w", err)
	}
	return docs, nil
}

// CreateAuditLog inserts a new audit log entry into PostgreSQL.
func (r *PostgresRepository) CreateAuditLog(ctx context.Context, log *AuditLog) error {
	if log.ID == uuid.Nil {
		log.ID = uuid.New()
	}
	if log.CreatedAt.IsZero() {
		log.CreatedAt = time.Now().UTC()
	}

	const query = `
		INSERT INTO archive_audit_log (
			id, document_id, action, user_id, ip_address, user_agent, details, created_at
		) VALUES (
			:id, :document_id, :action, :user_id, :ip_address, :user_agent, :details, :created_at
		)`

	_, err := r.db.NamedExecContext(ctx, query, log)
	if err != nil {
		return fmt.Errorf("create audit log: %w", err)
	}
	return nil
}

// GetAuditLogsByDocument retrieves all audit log entries for a given document.
func (r *PostgresRepository) GetAuditLogsByDocument(ctx context.Context, docID uuid.UUID) ([]*AuditLog, error) {
	const query = `
		SELECT id, document_id, action, user_id, ip_address, user_agent, details, created_at
		FROM archive_audit_log
		WHERE document_id = $1
		ORDER BY created_at ASC`

	var logs []*AuditLog
	if err := r.db.SelectContext(ctx, &logs, query, docID); err != nil {
		return nil, fmt.Errorf("get audit logs by document: %w", err)
	}
	return logs, nil
}

// ListAuditLogs queries audit log entries with filters in PostgreSQL.
func (r *PostgresRepository) ListAuditLogs(ctx context.Context, filter AuditLogFilter) ([]*AuditLog, int64, error) {
	where := []string{}
	args := []interface{}{}
	argIdx := 1

	if filter.DocumentID != nil {
		where = append(where, fmt.Sprintf("document_id = $%d", argIdx))
		args = append(args, *filter.DocumentID)
		argIdx++
	}
	if filter.Action != "" {
		where = append(where, fmt.Sprintf("action = $%d", argIdx))
		args = append(args, filter.Action)
		argIdx++
	}
	if filter.UserID != nil {
		where = append(where, fmt.Sprintf("user_id = $%d", argIdx))
		args = append(args, *filter.UserID)
		argIdx++
	}
	if filter.DateFrom != nil {
		where = append(where, fmt.Sprintf("created_at >= $%d", argIdx))
		args = append(args, *filter.DateFrom)
		argIdx++
	}
	if filter.DateTo != nil {
		where = append(where, fmt.Sprintf("created_at <= $%d", argIdx))
		args = append(args, *filter.DateTo)
		argIdx++
	}

	whereClause := ""
	if len(where) > 0 {
		whereClause = "WHERE " + strings.Join(where, " AND ")
	}

	countQuery := fmt.Sprintf("SELECT COUNT(*) FROM archive_audit_log %s", whereClause)
	var total int64
	if err := r.db.GetContext(ctx, &total, countQuery, args...); err != nil {
		return nil, 0, fmt.Errorf("count audit logs: %w", err)
	}

	limit := filter.Limit
	if limit <= 0 || limit > 500 {
		limit = 50
	}
	offset := filter.Offset
	if offset < 0 {
		offset = 0
	}

	selectQuery := fmt.Sprintf(`
		SELECT id, document_id, action, user_id, ip_address, user_agent, details, created_at
		FROM archive_audit_log
		%s
		ORDER BY created_at DESC
		LIMIT %d OFFSET %d`, whereClause, limit, offset)

	var logs []*AuditLog
	if err := r.db.SelectContext(ctx, &logs, selectQuery, args...); err != nil {
		return nil, 0, fmt.Errorf("list audit logs: %w", err)
	}

	return logs, total, nil
}

// GetDocumentsBySubject retrieves documents associated with a subject ID (e.g. student_id, uploaded_by, or subject_id in metadata).
func (r *PostgresRepository) GetDocumentsBySubject(ctx context.Context, subjectID string) ([]*ArchiveDocument, error) {
	const query = `
		SELECT id, filename, original_filename, mime_type, size_bytes, checksum,
		       storage_path, storage_tier, category, tags, metadata, ocr_text,
		       ocr_status, retention_policy_id, retain_until, legal_hold,
		       legal_hold_reason, uploaded_by, uploaded_at, updated_at, deleted_at
		FROM archive_documents
		WHERE (
			uploaded_by::text = $1 OR
			metadata->>'student_id' = $1 OR
			metadata->>'subject_id' = $1 OR
			metadata->>'subjectId' = $1 OR
			metadata->>'user_id' = $1
		) AND deleted_at IS NULL
		ORDER BY uploaded_at DESC`

	var docs []*ArchiveDocument
	if err := r.db.SelectContext(ctx, &docs, query, subjectID); err != nil {
		return nil, fmt.Errorf("get documents by subject: %w", err)
	}
	return docs, nil
}

// -----------------------------------------------------------------------------
// MemoryRepository Implementation (For Unit Testing & Integration Fallback)
// -----------------------------------------------------------------------------

type MemoryRepository struct {
	mu        sync.RWMutex
	documents map[uuid.UUID]*ArchiveDocument
	policies  map[uuid.UUID]*RetentionPolicy
	auditLogs []*AuditLog
}

// NewMemoryRepository constructs a thread-safe MemoryRepository initialized with default policies.
func NewMemoryRepository() *MemoryRepository {
	repo := &MemoryRepository{
		documents: make(map[uuid.UUID]*ArchiveDocument),
		policies:  make(map[uuid.UUID]*RetentionPolicy),
		auditLogs: make([]*AuditLog, 0),
	}
	repo.seedDefaultPolicies()
	return repo
}

func (r *MemoryRepository) seedDefaultPolicies() {
	defaults := []struct {
		category DocumentCategory
		years    int
		autoDel  bool
		override bool
		name     string
	}{
		{CategoryStudentRecord, 7, true, false, "Student Record Policy"},
		{CategoryGradeReport, 7, true, false, "Grade Report Policy"},
		{CategoryAttendanceRecord, 5, true, false, "Attendance Record Policy"},
		{CategoryBehaviorNote, 3, true, true, "Behavior Note Policy"},
		{CategoryMedicalRecord, 10, false, false, "Medical Record Policy"},
		{CategoryFinancialDoc, 10, true, false, "Financial Document Policy"},
		{CategoryLegalDoc, 99, false, false, "Legal Document Policy"},
		{CategoryCorrespondence, 3, true, true, "Correspondence Policy"},
	}

	for _, d := range defaults {
		id := uuid.New()
		r.policies[id] = &RetentionPolicy{
			ID:                id,
			Name:              d.name,
			Category:          d.category,
			RetentionYears:    d.years,
			AutoDelete:        d.autoDel,
			LegalHoldOverride: d.override,
			Description:       d.name + " pre-seeded rule",
			CreatedAt:         time.Now().UTC(),
			UpdatedAt:         time.Now().UTC(),
		}
	}
}

func (r *MemoryRepository) CreateDocument(ctx context.Context, doc *ArchiveDocument) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if doc.ID == uuid.Nil {
		doc.ID = uuid.New()
	}
	if doc.UploadedAt.IsZero() {
		doc.UploadedAt = time.Now().UTC()
	}
	doc.UpdatedAt = time.Now().UTC()

	cloned := *doc
	r.documents[doc.ID] = &cloned
	return nil
}

func (r *MemoryRepository) GetDocumentByID(ctx context.Context, id uuid.UUID) (*ArchiveDocument, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	doc, exists := r.documents[id]
	if !exists {
		return nil, ErrDocumentNotFound
	}
	cloned := *doc
	return &cloned, nil
}

func (r *MemoryRepository) UpdateDocument(ctx context.Context, doc *ArchiveDocument) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	existing, exists := r.documents[doc.ID]
	if !exists || existing.DeletedAt != nil {
		return ErrDocumentNotFound
	}
	doc.UpdatedAt = time.Now().UTC()
	cloned := *doc
	r.documents[doc.ID] = &cloned
	return nil
}

func (r *MemoryRepository) SoftDeleteDocument(ctx context.Context, id uuid.UUID) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	doc, exists := r.documents[id]
	if !exists {
		return ErrDocumentNotFound
	}
	now := time.Now().UTC()
	doc.DeletedAt = &now
	doc.UpdatedAt = now
	return nil
}

func (r *MemoryRepository) ListDocuments(ctx context.Context, filter ArchiveFilter) ([]*ArchiveDocument, int64, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var results []*ArchiveDocument
	for _, doc := range r.documents {
		if !filter.IncludeDeleted && doc.DeletedAt != nil {
			continue
		}
		if filter.Category != "" && doc.Category != filter.Category {
			continue
		}
		if filter.LegalHoldOnly && !doc.LegalHold {
			continue
		}
		if filter.OCRCompleted && doc.OCRStatus != OCRStatusCompleted {
			continue
		}
		if filter.StorageTier != "" && doc.StorageTier != filter.StorageTier {
			continue
		}
		if filter.DateFrom != nil && doc.UploadedAt.Before(*filter.DateFrom) {
			continue
		}
		if filter.DateTo != nil && doc.UploadedAt.After(*filter.DateTo) {
			continue
		}
		if filter.Query != "" {
			q := strings.ToLower(filter.Query)
			fname := strings.ToLower(doc.Filename)
			ocr := strings.ToLower(doc.OCRText)
			if !strings.Contains(fname, q) && !strings.Contains(ocr, q) {
				continue
			}
		}
		if len(filter.Tags) > 0 {
			match := true
			for _, filterTag := range filter.Tags {
				found := false
				for _, docTag := range doc.Tags {
					if docTag == filterTag {
						found = true
						break
					}
				}
				if !found {
					match = false
					break
				}
			}
			if !match {
				continue
			}
		}

		cloned := *doc
		results = append(results, &cloned)
	}

	total := int64(len(results))

	// Sort documents by UploadedAt DESC
	sort.Slice(results, func(i, j int) bool {
		return results[i].UploadedAt.After(results[j].UploadedAt)
	})

	// Apply Limit & Offset Pagination
	limit := filter.Limit
	if limit <= 0 || limit > 500 {
		limit = 50
	}
	offset := filter.Offset
	if offset < 0 {
		offset = 0
	}

	if offset >= len(results) {
		return []*ArchiveDocument{}, total, nil
	}
	end := offset + limit
	if end > len(results) {
		end = len(results)
	}

	pagedResults := results[offset:end]
	return pagedResults, total, nil
}

func (r *MemoryRepository) GetRetentionPolicyByID(ctx context.Context, id uuid.UUID) (*RetentionPolicy, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	policy, exists := r.policies[id]
	if !exists {
		return nil, ErrPolicyNotFound
	}
	cloned := *policy
	return &cloned, nil
}

func (r *MemoryRepository) GetDefaultPolicyByCategory(ctx context.Context, category DocumentCategory) (*RetentionPolicy, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var matches []*RetentionPolicy
	for _, policy := range r.policies {
		if policy.Category == category {
			matches = append(matches, policy)
		}
	}
	if len(matches) == 0 {
		return nil, ErrPolicyNotFound
	}
	sort.Slice(matches, func(i, j int) bool {
		return matches[i].CreatedAt.Before(matches[j].CreatedAt)
	})
	cloned := *matches[0]
	return &cloned, nil
}

func (r *MemoryRepository) ListRetentionPolicies(ctx context.Context) ([]*RetentionPolicy, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var list []*RetentionPolicy
	for _, policy := range r.policies {
		cloned := *policy
		list = append(list, &cloned)
	}
	return list, nil
}

func (r *MemoryRepository) CreateRetentionPolicy(ctx context.Context, policy *RetentionPolicy) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if policy.ID == uuid.Nil {
		policy.ID = uuid.New()
	}
	policy.CreatedAt = time.Now().UTC()
	policy.UpdatedAt = time.Now().UTC()

	cloned := *policy
	r.policies[policy.ID] = &cloned
	return nil
}

func (r *MemoryRepository) GetExpiredDocuments(ctx context.Context, limit int) ([]*ArchiveDocument, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if limit <= 0 || limit > 1000 {
		limit = 100
	}

	now := time.Now().UTC()
	todayMidnight := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	var expired []*ArchiveDocument
	for _, doc := range r.documents {
		if doc.DeletedAt != nil || doc.RetainUntil.IsZero() {
			continue
		}
		docMidnight := time.Date(doc.RetainUntil.Year(), doc.RetainUntil.Month(), doc.RetainUntil.Day(), 0, 0, 0, 0, time.UTC)
		if docMidnight.Before(todayMidnight) || docMidnight.Equal(todayMidnight) {
			cloned := *doc
			expired = append(expired, &cloned)
			if len(expired) >= limit {
				break
			}
		}
	}
	return expired, nil
}

func (r *MemoryRepository) GetDocumentsForTierMigration(ctx context.Context, currentTier StorageTier, olderThanDays int) ([]*ArchiveDocument, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	cutoff := time.Now().UTC().AddDate(0, 0, -olderThanDays)
	var matches []*ArchiveDocument
	for _, doc := range r.documents {
		if doc.DeletedAt != nil {
			continue
		}
		refTime := doc.UploadedAt
		if refTime.IsZero() {
			refTime = doc.UpdatedAt
		}
		if doc.StorageTier == currentTier && (refTime.Before(cutoff) || refTime.Equal(cutoff)) {
			cloned := *doc
			matches = append(matches, &cloned)
			if len(matches) >= 200 {
				break
			}
		}
	}
	return matches, nil
}

func (r *MemoryRepository) CreateAuditLog(ctx context.Context, log *AuditLog) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if log.ID == uuid.Nil {
		log.ID = uuid.New()
	}
	if log.CreatedAt.IsZero() {
		log.CreatedAt = time.Now().UTC()
	}

	cloned := *log
	r.auditLogs = append(r.auditLogs, &cloned)
	return nil
}

func (r *MemoryRepository) GetAuditLogsByDocument(ctx context.Context, docID uuid.UUID) ([]*AuditLog, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var logs []*AuditLog
	for _, l := range r.auditLogs {
		if l.DocumentID != nil && *l.DocumentID == docID {
			cloned := *l
			logs = append(logs, &cloned)
		}
	}
	return logs, nil
}

func (r *MemoryRepository) ListAuditLogs(ctx context.Context, filter AuditLogFilter) ([]*AuditLog, int64, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var results []*AuditLog
	for _, l := range r.auditLogs {
		if filter.DocumentID != nil && (l.DocumentID == nil || *l.DocumentID != *filter.DocumentID) {
			continue
		}
		if filter.Action != "" && l.Action != filter.Action {
			continue
		}
		if filter.UserID != nil && l.UserID != *filter.UserID {
			continue
		}
		if filter.DateFrom != nil && l.CreatedAt.Before(*filter.DateFrom) {
			continue
		}
		if filter.DateTo != nil && l.CreatedAt.After(*filter.DateTo) {
			continue
		}

		cloned := *l
		results = append(results, &cloned)
	}

	total := int64(len(results))

	sort.Slice(results, func(i, j int) bool {
		return results[i].CreatedAt.After(results[j].CreatedAt)
	})

	limit := filter.Limit
	if limit <= 0 || limit > 500 {
		limit = 50
	}
	offset := filter.Offset
	if offset < 0 {
		offset = 0
	}

	if offset >= len(results) {
		return []*AuditLog{}, total, nil
	}
	end := offset + limit
	if end > len(results) {
		end = len(results)
	}

	pagedResults := results[offset:end]
	return pagedResults, total, nil
}

// GetDocumentsBySubject retrieves documents associated with a subject ID in MemoryRepository.
func (r *MemoryRepository) GetDocumentsBySubject(ctx context.Context, subjectID string) ([]*ArchiveDocument, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var matches []*ArchiveDocument
	for _, doc := range r.documents {
		if doc.DeletedAt != nil {
			continue
		}
		if doc.UploadedBy.String() == subjectID {
			cloned := *doc
			matches = append(matches, &cloned)
			continue
		}
		if doc.Metadata != nil {
			found := false
			for _, key := range []string{"student_id", "subject_id", "subjectId", "user_id"} {
				if val, ok := doc.Metadata[key]; ok && fmt.Sprintf("%v", val) == subjectID {
					cloned := *doc
					matches = append(matches, &cloned)
					found = true
					break
				}
			}
			if found {
				continue
			}
		}
	}

	sort.Slice(matches, func(i, j int) bool {
		return matches[i].UploadedAt.After(matches[j].UploadedAt)
	})

	return matches, nil
}
