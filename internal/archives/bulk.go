package archives

import (
	"archive/zip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/google/uuid"
)

// BulkProcessor manages batch operations on archive documents.
type BulkProcessor struct {
	repo         Repository
	searchEngine SearchEngine
	signer       SignedURLSigner
}

// NewBulkProcessor constructs a new BulkProcessor instance.
func NewBulkProcessor(repo Repository, searchEngine SearchEngine, signer SignedURLSigner) *BulkProcessor {
	return &BulkProcessor{
		repo:         repo,
		searchEngine: searchEngine,
		signer:       signer,
	}
}

// ProcessBulkAction executes requested batch actions (DELETE, CHANGE_CATEGORY, APPLY_RETENTION, DOWNLOAD).
func (b *BulkProcessor) ProcessBulkAction(ctx context.Context, req BulkActionRequest, userID uuid.UUID) (*BulkActionResponse, error) {
	resp := &BulkActionResponse{
		ProcessedCount: 0,
		SkippedCount:   0,
		FailedCount:    0,
		Errors:         make([]string, 0),
		SkippedIDs:     make([]uuid.UUID, 0),
	}

	if len(req.IDs) == 0 {
		return resp, nil
	}

	switch req.Action {
	case "DELETE":
		return b.BulkDeleteDocuments(ctx, req.IDs, userID)
	case "CHANGE_CATEGORY":
		category := DocumentCategory(req.Parameters["category"])
		if category == "" {
			return nil, fmt.Errorf("category parameter is required for CHANGE_CATEGORY action")
		}
		return b.BulkUpdateCategory(ctx, req.IDs, category, userID)
	case "APPLY_RETENTION":
		return b.BulkUpdateRetention(ctx, req.IDs, req.Parameters, userID)
	case "DOWNLOAD":
		return b.executeBulkDownload(ctx, req.IDs, userID, resp)
	default:
		return nil, fmt.Errorf("unsupported bulk action: %s", req.Action)
	}
}

// CreateBulkZipStream streams a ZIP archive containing requested documents to an io.Writer with manifest.json.
func (b *BulkProcessor) CreateBulkZipStream(ctx context.Context, ids []uuid.UUID, w io.Writer) error {
	return b.StreamBulkZip(ctx, ids, w)
}

// StreamBulkZip streams a ZIP archive containing requested documents to an io.Writer with manifest.json.
func (b *BulkProcessor) StreamBulkZip(ctx context.Context, ids []uuid.UUID, w io.Writer) error {
	zw := zip.NewWriter(w)
	defer zw.Close()

	type manifestItem struct {
		ID        string `json:"id"`
		Filename  string `json:"filename"`
		Category  string `json:"category"`
		SizeBytes int64  `json:"size_bytes"`
	}
	var manifest []manifestItem

	for _, id := range ids {
		doc, err := b.repo.GetDocumentByID(ctx, id)
		if err != nil || doc.DeletedAt != nil {
			continue
		}

		manifest = append(manifest, manifestItem{
			ID:        doc.ID.String(),
			Filename:  doc.Filename,
			Category:  string(doc.Category),
			SizeBytes: doc.SizeBytes,
		})

		f, err := os.Open(doc.StoragePath)
		if err != nil {
			// If physical file missing, create a placeholder entry in the zip
			header := &zip.FileHeader{
				Name:     doc.Filename,
				Method:   zip.Deflate,
				Modified: doc.UpdatedAt,
			}
			writer, createErr := zw.CreateHeader(header)
			if createErr == nil {
				if len(doc.OCRText) > 0 {
					_, _ = writer.Write([]byte(doc.OCRText))
				} else {
					_, _ = writer.Write([]byte("Document content unavailable"))
				}
			}
			continue
		}

		header := &zip.FileHeader{
			Name:     doc.Filename,
			Method:   zip.Deflate,
			Modified: doc.UpdatedAt,
		}

		writer, err := zw.CreateHeader(header)
		if err != nil {
			f.Close()
			return err
		}

		_, err = io.Copy(writer, f)
		f.Close()
		if err != nil {
			return err
		}
	}

	// Write embedded manifest.json
	manifestBytes, err := json.MarshalIndent(manifest, "", "  ")
	if err == nil {
		h := &zip.FileHeader{
			Name:     "manifest.json",
			Method:   zip.Deflate,
			Modified: time.Now().UTC(),
		}
		if mw, err := zw.CreateHeader(h); err == nil {
			_, _ = mw.Write(manifestBytes)
		}
	}

	return nil
}

// BulkDeleteDocuments soft deletes documents, skipping items under legal hold and logging audit trails.
func (b *BulkProcessor) BulkDeleteDocuments(ctx context.Context, ids []uuid.UUID, userID uuid.UUID) (*BulkActionResponse, error) {
	resp := &BulkActionResponse{
		ProcessedCount: 0,
		SkippedCount:   0,
		FailedCount:    0,
		Errors:         make([]string, 0),
		SkippedIDs:     make([]uuid.UUID, 0),
	}

	for _, id := range ids {
		doc, err := b.repo.GetDocumentByID(ctx, id)
		if err != nil || doc.DeletedAt != nil {
			resp.SkippedCount++
			resp.SkippedIDs = append(resp.SkippedIDs, id)
			continue
		}

		// Legal Hold Protection check
		if doc.LegalHold {
			resp.SkippedCount++
			resp.SkippedIDs = append(resp.SkippedIDs, id)

			// Log SKIPPED_LEGAL_HOLD audit entry
			_ = b.repo.CreateAuditLog(ctx, &AuditLog{
				ID:         uuid.New(),
				DocumentID: &id,
				Action:     AuditActionSkippedLegalHold,
				UserID:     userID,
				IPAddress:  "127.0.0.1",
				UserAgent:  "BulkProcessor/1.0",
				Details: map[string]any{
					"reason": "bulk delete skipped due to active legal hold",
				},
				CreatedAt: time.Now().UTC(),
			})
			continue
		}

		// Soft delete document in DB
		if err := b.repo.SoftDeleteDocument(ctx, id); err != nil {
			resp.FailedCount++
			resp.Errors = append(resp.Errors, fmt.Sprintf("failed to soft delete document %s: %v", id, err))
			continue
		}

		// Delete physical file if exists
		if doc.StoragePath != "" {
			_ = os.Remove(doc.StoragePath)
		}

		// Delete search index
		if b.searchEngine != nil {
			_ = b.searchEngine.DeleteDocumentIndex(ctx, id)
		}

		// Record audit log for DELETE
		_ = b.repo.CreateAuditLog(ctx, &AuditLog{
			ID:         uuid.New(),
			DocumentID: &id,
			Action:     AuditActionDelete,
			UserID:     userID,
			IPAddress:  "127.0.0.1",
			UserAgent:  "BulkProcessor/1.0",
			Details: map[string]any{
				"operation": "bulk_delete",
			},
			CreatedAt: time.Now().UTC(),
		})

		resp.ProcessedCount++
	}

	return resp, nil
}

// BulkUpdateCategory updates category and recalculates default retention policy for document list.
func (b *BulkProcessor) BulkUpdateCategory(ctx context.Context, ids []uuid.UUID, newCategory DocumentCategory, userID uuid.UUID) (*BulkActionResponse, error) {
	resp := &BulkActionResponse{
		ProcessedCount: 0,
		SkippedCount:   0,
		FailedCount:    0,
		Errors:         make([]string, 0),
		SkippedIDs:     make([]uuid.UUID, 0),
	}

	var policy *RetentionPolicy
	if pol, err := b.repo.GetDefaultPolicyByCategory(ctx, newCategory); err == nil && pol != nil {
		policy = pol
	}

	for _, id := range ids {
		doc, err := b.repo.GetDocumentByID(ctx, id)
		if err != nil || doc.DeletedAt != nil {
			resp.SkippedCount++
			resp.SkippedIDs = append(resp.SkippedIDs, id)
			continue
		}

		doc.Category = newCategory
		if policy != nil {
			doc.RetentionPolicyID = &policy.ID
			retainUntil := time.Now().UTC().AddDate(policy.RetentionYears, 0, 0)
			doc.RetainUntil = time.Date(retainUntil.Year(), retainUntil.Month(), retainUntil.Day(), 0, 0, 0, 0, time.UTC)
		}

		if err := b.repo.UpdateDocument(ctx, doc); err != nil {
			resp.FailedCount++
			resp.Errors = append(resp.Errors, fmt.Sprintf("failed to update document %s category: %v", id, err))
			continue
		}

		if b.searchEngine != nil {
			_ = b.searchEngine.IndexDocument(ctx, doc)
		}

		_ = b.repo.CreateAuditLog(ctx, &AuditLog{
			ID:         uuid.New(),
			DocumentID: &id,
			Action:     AuditActionRetentionChange,
			UserID:     userID,
			IPAddress:  "127.0.0.1",
			UserAgent:  "BulkProcessor/1.0",
			Details: map[string]any{
				"new_category": newCategory,
			},
			CreatedAt: time.Now().UTC(),
		})

		resp.ProcessedCount++
	}

	return resp, nil
}

// BulkUpdateRetention updates retention policy or retain_until date across multiple documents.
func (b *BulkProcessor) BulkUpdateRetention(ctx context.Context, ids []uuid.UUID, params map[string]string, userID uuid.UUID) (*BulkActionResponse, error) {
	resp := &BulkActionResponse{
		ProcessedCount: 0,
		SkippedCount:   0,
		FailedCount:    0,
		Errors:         make([]string, 0),
		SkippedIDs:     make([]uuid.UUID, 0),
	}

	policyIDStr := params["retention_policy_id"]
	var policy *RetentionPolicy
	if policyIDStr != "" {
		policyID, err := uuid.Parse(policyIDStr)
		if err != nil {
			return nil, fmt.Errorf("invalid retention_policy_id: %w", ErrPolicyNotFound)
		}
		pol, err := b.repo.GetRetentionPolicyByID(ctx, policyID)
		if err != nil {
			return nil, ErrPolicyNotFound
		}
		policy = pol
	}

	for _, id := range ids {
		doc, err := b.repo.GetDocumentByID(ctx, id)
		if err != nil || doc.DeletedAt != nil {
			resp.SkippedCount++
			resp.SkippedIDs = append(resp.SkippedIDs, id)
			continue
		}

		if policy != nil {
			doc.RetentionPolicyID = &policy.ID
			retainUntil := time.Now().UTC().AddDate(policy.RetentionYears, 0, 0)
			doc.RetainUntil = time.Date(retainUntil.Year(), retainUntil.Month(), retainUntil.Day(), 0, 0, 0, 0, time.UTC)
		}

		if err := b.repo.UpdateDocument(ctx, doc); err != nil {
			resp.FailedCount++
			resp.Errors = append(resp.Errors, fmt.Sprintf("failed to update document %s retention: %v", id, err))
			continue
		}

		_ = b.repo.CreateAuditLog(ctx, &AuditLog{
			ID:         uuid.New(),
			DocumentID: &id,
			Action:     AuditActionRetentionChange,
			UserID:     userID,
			IPAddress:  "127.0.0.1",
			UserAgent:  "BulkProcessor/1.0",
			Details: map[string]any{
				"operation": "bulk_apply_retention",
			},
			CreatedAt: time.Now().UTC(),
		})

		resp.ProcessedCount++
	}

	return resp, nil
}

func (b *BulkProcessor) executeBulkDownload(ctx context.Context, ids []uuid.UUID, userID uuid.UUID, resp *BulkActionResponse) (*BulkActionResponse, error) {
	for _, id := range ids {
		doc, err := b.repo.GetDocumentByID(ctx, id)
		if err != nil || doc.DeletedAt != nil {
			resp.SkippedCount++
			resp.SkippedIDs = append(resp.SkippedIDs, id)
			continue
		}

		_ = b.repo.CreateAuditLog(ctx, &AuditLog{
			ID:         uuid.New(),
			DocumentID: &id,
			Action:     AuditActionDownload,
			UserID:     userID,
			IPAddress:  "127.0.0.1",
			UserAgent:  "BulkProcessor/1.0",
			Details: map[string]any{
				"operation": "bulk_download",
			},
			CreatedAt: time.Now().UTC(),
		})

		resp.ProcessedCount++
	}

	return resp, nil
}
