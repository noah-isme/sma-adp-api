package archives

import (
	"archive/zip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/google/uuid"
)

// GDPRProcessor manages data subject request compliance workflows.
type GDPRProcessor struct {
	repo         Repository
	searchEngine SearchEngine
	signer       SignedURLSigner
	bulkProc     *BulkProcessor
}

// NewGDPRProcessor constructs a new GDPRProcessor instance.
func NewGDPRProcessor(repo Repository, searchEngine SearchEngine, signer SignedURLSigner, bulkProc *BulkProcessor) *GDPRProcessor {
	return &GDPRProcessor{
		repo:         repo,
		searchEngine: searchEngine,
		signer:       signer,
		bulkProc:     bulkProc,
	}
}

// ProcessGDPRRequest routes and executes ACCESS, RECTIFICATION, ERASURE, or PORTABILITY requests.
func (g *GDPRProcessor) ProcessGDPRRequest(ctx context.Context, req GDPRRequest) (*GDPRResponse, error) {
	reqID := fmt.Sprintf("gdpr_req_%d", time.Now().UnixNano())

	switch req.Type {
	case "ACCESS":
		return g.ProcessAccess(ctx, reqID, req)
	case "RECTIFICATION":
		return g.ProcessRectification(ctx, reqID, req)
	case "ERASURE":
		return g.ProcessErasure(ctx, reqID, req)
	case "PORTABILITY":
		return g.ProcessPortability(ctx, reqID, req)
	default:
		return nil, fmt.Errorf("unsupported GDPR request type: %s", req.Type)
	}
}

// ProcessAccess handles GDPR Data Subject Access Requests (ZIP archive export + signed URL).
func (g *GDPRProcessor) ProcessAccess(ctx context.Context, reqID string, req GDPRRequest) (*GDPRResponse, error) {
	if reqID == "" {
		reqID = fmt.Sprintf("gdpr_req_%d", time.Now().UnixNano())
	}
	docs, err := g.findSubjectDocuments(ctx, req.SubjectID, req.DocumentID)
	if err != nil {
		return nil, err
	}
	if len(docs) == 0 {
		return nil, ErrDocumentNotFound
	}

	tempDir := filepath.Join(os.TempDir(), "gdpr_access_exports")
	_ = os.MkdirAll(tempDir, 0755)
	zipPath := filepath.Join(tempDir, fmt.Sprintf("%s.zip", reqID))

	zipFile, err := os.Create(zipPath)
	if err != nil {
		return nil, fmt.Errorf("create zip export file: %w", err)
	}

	zw := zip.NewWriter(zipFile)
	for _, doc := range docs {
		// Include original or storage file if present
		if doc.StoragePath != "" {
			if f, err := os.Open(doc.StoragePath); err == nil {
				h := &zip.FileHeader{
					Name:     fmt.Sprintf("documents/%s", doc.Filename),
					Method:   zip.Deflate,
					Modified: doc.UpdatedAt,
				}
				if w, err := zw.CreateHeader(h); err == nil {
					_, _ = io.Copy(w, f)
				}
				f.Close()
			}
		}

		// Include metadata JSON file
		metaBytes, _ := json.MarshalIndent(doc, "", "  ")
		h := &zip.FileHeader{
			Name:     fmt.Sprintf("metadata/%s_metadata.json", doc.ID.String()),
			Method:   zip.Deflate,
			Modified: doc.UpdatedAt,
		}
		if w, err := zw.CreateHeader(h); err == nil {
			_, _ = w.Write(metaBytes)
		}
	}
	_ = zw.Close()
	_ = zipFile.Close()

	exportURL := fmt.Sprintf("/api/v1/archives/gdpr/export/%s.zip", reqID)
	if g.signer != nil && len(docs) > 0 {
		if signedURL, err := g.signer.GenerateSignedURL(docs[0].ID, fmt.Sprintf("%s.zip", reqID), "127.0.0.1", 1*time.Hour); err == nil {
			exportURL = signedURL
		}
	}

	// Record audit log for ACCESS
	userID := uuid.Nil
	if parsed, err := uuid.Parse(req.SubjectID); err == nil {
		userID = parsed
	}
	_ = g.repo.CreateAuditLog(ctx, &AuditLog{
		ID:        uuid.New(),
		Action:    AuditActionGDPRRequest,
		UserID:    userID,
		IPAddress: "127.0.0.1",
		UserAgent: "GDPRProcessor/1.0",
		Details: map[string]any{
			"request_id":      reqID,
			"gdpr_type":       "ACCESS",
			"documents_count": len(docs),
			"requester_email": req.RequesterEmail,
		},
		CreatedAt: time.Now().UTC(),
	})

	return &GDPRResponse{
		RequestID: reqID,
		Type:      "ACCESS",
		Status:    "COMPLETED",
		ExportURL: exportURL,
		Message:   fmt.Sprintf("Data subject access request processed; %d document(s) exported", len(docs)),
	}, nil
}

// ProcessRectification handles GDPR Data Subject Rectification Requests (metadata correction).
func (g *GDPRProcessor) ProcessRectification(ctx context.Context, reqID string, req GDPRRequest) (*GDPRResponse, error) {
	if reqID == "" {
		reqID = fmt.Sprintf("gdpr_req_%d", time.Now().UnixNano())
	}
	if req.DocumentID == nil {
		return nil, fmt.Errorf("documentId is required for RECTIFICATION request")
	}

	doc, err := g.repo.GetDocumentByID(ctx, *req.DocumentID)
	if err != nil {
		return nil, err
	}

	if doc.Metadata == nil {
		doc.Metadata = make(JSONMap)
	}

	for k, v := range req.Corrections {
		doc.Metadata[k] = v
	}

	if err := g.repo.UpdateDocument(ctx, doc); err != nil {
		return nil, fmt.Errorf("failed to rectify document metadata: %w", err)
	}

	if g.searchEngine != nil {
		_ = g.searchEngine.IndexDocument(ctx, doc)
	}

	_ = g.repo.CreateAuditLog(ctx, &AuditLog{
		ID:         uuid.New(),
		DocumentID: req.DocumentID,
		Action:     AuditActionGDPRRequest,
		UserID:     doc.UploadedBy,
		IPAddress:  "127.0.0.1",
		UserAgent:  "GDPRProcessor/1.0",
		Details: map[string]any{
			"request_id":  reqID,
			"gdpr_type":   "RECTIFICATION",
			"corrections": req.Corrections,
		},
		CreatedAt: time.Now().UTC(),
	})

	return &GDPRResponse{
		RequestID: reqID,
		Type:      "RECTIFICATION",
		Status:    "COMPLETED",
		Message:   "Document metadata corrected successfully",
	}, nil
}

// ProcessErasure handles GDPR Right to be Forgotten / Erasure Requests with legal hold and retention safeguards.
func (g *GDPRProcessor) ProcessErasure(ctx context.Context, reqID string, req GDPRRequest) (*GDPRResponse, error) {
	if reqID == "" {
		reqID = fmt.Sprintf("gdpr_req_%d", time.Now().UnixNano())
	}
	if req.DocumentID == nil {
		return nil, fmt.Errorf("documentId is required for ERASURE request")
	}

	doc, err := g.repo.GetDocumentByID(ctx, *req.DocumentID)
	if err != nil {
		return nil, err
	}

	// 1. Legal Hold Protection Check
	if doc.LegalHold {
		_ = g.repo.CreateAuditLog(ctx, &AuditLog{
			ID:         uuid.New(),
			DocumentID: req.DocumentID,
			Action:     AuditActionSkippedLegalHold,
			UserID:     doc.UploadedBy,
			IPAddress:  "127.0.0.1",
			UserAgent:  "GDPRProcessor/1.0",
			Details: map[string]any{
				"request_id": reqID,
				"reason":     "GDPR erasure blocked by active legal hold",
			},
			CreatedAt: time.Now().UTC(),
		})
		return nil, ErrLegalHoldActive
	}

	// 2. Retention Policy Expiry Check
	if !doc.RetainUntil.IsZero() && time.Now().Before(doc.RetainUntil) {
		return nil, ErrRetentionNotExpired
	}

	// 3. Soft Delete and Storage Unlink
	if err := g.repo.SoftDeleteDocument(ctx, *req.DocumentID); err != nil {
		return nil, fmt.Errorf("failed to erase document: %w", err)
	}

	if doc.StoragePath != "" {
		_ = os.Remove(doc.StoragePath)
	}

	if g.searchEngine != nil {
		_ = g.searchEngine.DeleteDocumentIndex(ctx, *req.DocumentID)
	}

	_ = g.repo.CreateAuditLog(ctx, &AuditLog{
		ID:         uuid.New(),
		DocumentID: req.DocumentID,
		Action:     AuditActionGDPRRequest,
		UserID:     doc.UploadedBy,
		IPAddress:  "127.0.0.1",
		UserAgent:  "GDPRProcessor/1.0",
		Details: map[string]any{
			"request_id": reqID,
			"gdpr_type":  "ERASURE",
		},
		CreatedAt: time.Now().UTC(),
	})

	return &GDPRResponse{
		RequestID: reqID,
		Type:      "ERASURE",
		Status:    "COMPLETED",
		Message:   "Document erased successfully in compliance with GDPR/PDPA",
	}, nil
}

// ProcessPortability handles GDPR Data Portability Requests (structured JSON export).
func (g *GDPRProcessor) ProcessPortability(ctx context.Context, reqID string, req GDPRRequest) (*GDPRResponse, error) {
	if reqID == "" {
		reqID = fmt.Sprintf("gdpr_req_%d", time.Now().UnixNano())
	}
	docs, err := g.findSubjectDocuments(ctx, req.SubjectID, req.DocumentID)
	if err != nil {
		return nil, err
	}
	if len(docs) == 0 {
		return nil, ErrDocumentNotFound
	}

	exportItems := make([]map[string]any, 0, len(docs))
	for _, doc := range docs {
		item := map[string]any{
			"id":               doc.ID.String(),
			"filename":         doc.Filename,
			"originalFilename": doc.OriginalFilename,
			"mimeType":         doc.MimeType,
			"sizeBytes":        doc.SizeBytes,
			"checksum":         doc.Checksum,
			"category":         doc.Category,
			"tags":             doc.Tags,
			"metadata":         doc.Metadata,
			"ocrText":          doc.OCRText,
			"uploadedAt":       doc.UploadedAt.Format(time.RFC3339),
			"retainUntil":      doc.RetainUntil.Format(time.RFC3339),
		}
		exportItems = append(exportItems, item)
	}

	userID := uuid.Nil
	if parsed, err := uuid.Parse(req.SubjectID); err == nil {
		userID = parsed
	}
	_ = g.repo.CreateAuditLog(ctx, &AuditLog{
		ID:        uuid.New(),
		Action:    AuditActionGDPRRequest,
		UserID:    userID,
		IPAddress: "127.0.0.1",
		UserAgent: "GDPRProcessor/1.0",
		Details: map[string]any{
			"request_id":      reqID,
			"gdpr_type":       "PORTABILITY",
			"documents_count": len(docs),
		},
		CreatedAt: time.Now().UTC(),
	})

	return &GDPRResponse{
		RequestID: reqID,
		Type:      "PORTABILITY",
		Status:    "COMPLETED",
		Message:   "Data portability export generated successfully",
		Data: map[string]any{
			"subjectId": req.SubjectID,
			"total":     len(docs),
			"documents": exportItems,
		},
	}, nil
}

func (g *GDPRProcessor) findSubjectDocuments(ctx context.Context, subjectID string, docID *uuid.UUID) ([]*ArchiveDocument, error) {
	if docID != nil {
		doc, err := g.repo.GetDocumentByID(ctx, *docID)
		if err != nil {
			return nil, err
		}
		if doc.DeletedAt != nil {
			return []*ArchiveDocument{}, nil
		}
		return []*ArchiveDocument{doc}, nil
	}

	allDocs, _, err := g.repo.ListDocuments(ctx, ArchiveFilter{Limit: 500})
	if err != nil {
		return nil, err
	}

	var matches []*ArchiveDocument
	for _, doc := range allDocs {
		if doc.DeletedAt != nil {
			continue
		}
		if subjectID != "" {
			if doc.UploadedBy.String() == subjectID {
				matches = append(matches, doc)
				continue
			}
			if doc.Metadata != nil {
				for _, key := range []string{"student_id", "user_id", "subject_id"} {
					if val, ok := doc.Metadata[key]; ok && fmt.Sprintf("%v", val) == subjectID {
						matches = append(matches, doc)
						break
					}
				}
			}
		} else {
			matches = append(matches, doc)
		}
	}
	return matches, nil
}
