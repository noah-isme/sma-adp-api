package archives

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/google/uuid"
)

type ArchiveService struct {
	repo            Repository
	searchEngine    SearchEngine
	ocrWorkerPool   OCRWorkerPool
	signer          SignedURLSigner
	retentionEngine *RetentionEngine
	bulkProcessor   *BulkProcessor
	gdprProcessor   *GDPRProcessor
}

func NewArchiveService(
	repo Repository,
	searchEngine SearchEngine,
	ocrWorkerPool OCRWorkerPool,
	signer SignedURLSigner,
	retentionEngine *RetentionEngine,
) *ArchiveService {
	if retentionEngine != nil {
		if searchEngine != nil {
			retentionEngine.SetSearchEngine(searchEngine)
		}
		if repo != nil && retentionEngine.tierMigrator == nil {
			retentionEngine.SetStorageTierMigrator(NewStorageTierMigrator(repo, searchEngine))
		}
	}

	bulkProc := NewBulkProcessor(repo, searchEngine, signer)
	gdprProc := NewGDPRProcessor(repo, searchEngine, signer, bulkProc)

	return &ArchiveService{
		repo:            repo,
		searchEngine:    searchEngine,
		ocrWorkerPool:   ocrWorkerPool,
		signer:          signer,
		retentionEngine: retentionEngine,
		bulkProcessor:   bulkProc,
		gdprProcessor:   gdprProc,
	}
}

func (s *ArchiveService) UploadDocument(
	ctx context.Context,
	filename string,
	category DocumentCategory,
	tags []string,
	metadata map[string]any,
	fileContent []byte,
	userID uuid.UUID,
) (*ArchiveDocument, error) {
	policy, err := s.repo.GetDefaultPolicyByCategory(ctx, category)
	var policyID *uuid.UUID
	retentionYears := 7
	if err == nil && policy != nil {
		pID := policy.ID
		policyID = &pID
		retentionYears = policy.RetentionYears
	}

	hasher := sha256.New()
	hasher.Write(fileContent)
	checksum := hex.EncodeToString(hasher.Sum(nil))

	docID := uuid.New()
	retainUntil := time.Now().UTC().AddDate(retentionYears, 0, 0)
	retainUntil = time.Date(retainUntil.Year(), retainUntil.Month(), retainUntil.Day(), 0, 0, 0, 0, time.UTC)

	tempStorageDir := filepath.Join(os.TempDir(), "sma_archives")
	_ = os.MkdirAll(tempStorageDir, 0755)
	fullPath := filepath.Join(tempStorageDir, docID.String()+"_"+filename)
	if len(fileContent) > 0 {
		_ = os.WriteFile(fullPath, fileContent, 0644)
	}

	doc := &ArchiveDocument{
		ID:                docID,
		Filename:          filename,
		OriginalFilename:  filename,
		MimeType:          "application/pdf",
		SizeBytes:         int64(len(fileContent)),
		Checksum:          checksum,
		StoragePath:       fullPath,
		StorageTier:       StorageTierHot,
		Category:          category,
		Tags:              tags,
		Metadata:          metadata,
		OCRStatus:         OCRStatusPending,
		RetentionPolicyID: policyID,
		RetainUntil:       retainUntil,
		LegalHold:         false,
		UploadedBy:        userID,
		UploadedAt:        time.Now(),
		UpdatedAt:         time.Now(),
	}

	if err := s.repo.CreateDocument(ctx, doc); err != nil {
		return nil, err
	}

	// Create audit log for UPLOAD
	_ = s.repo.CreateAuditLog(ctx, &AuditLog{
		ID:         uuid.New(),
		DocumentID: &docID,
		Action:     AuditActionUpload,
		UserID:     userID,
		IPAddress:  "127.0.0.1",
		UserAgent:  "ArchiveService/1.0",
		Details: map[string]any{
			"filename": filename,
			"size":     len(fileContent),
			"category": category,
		},
		CreatedAt: time.Now(),
	})

	// Enqueue to OCR worker pool if available
	if s.ocrWorkerPool != nil {
		_ = s.ocrWorkerPool.Enqueue(docID, doc.StoragePath, doc.MimeType)
	}

	return doc, nil
}

func (s *ArchiveService) GetDocument(ctx context.Context, id uuid.UUID) (*ArchiveDocument, error) {
	return s.repo.GetDocumentByID(ctx, id)
}

func (s *ArchiveService) ListDocuments(ctx context.Context, filter ArchiveFilter) ([]*ArchiveDocument, int64, error) {
	return s.repo.ListDocuments(ctx, filter)
}

func (s *ArchiveService) DeleteDocument(ctx context.Context, id uuid.UUID, userID uuid.UUID) error {
	doc, err := s.repo.GetDocumentByID(ctx, id)
	if err != nil {
		return err
	}
	if doc.LegalHold {
		return ErrLegalHoldActive
	}

	if err := s.repo.SoftDeleteDocument(ctx, id); err != nil {
		return err
	}

	if doc.StoragePath != "" {
		_ = os.Remove(doc.StoragePath)
	}

	if s.searchEngine != nil {
		_ = s.searchEngine.DeleteDocumentIndex(ctx, id)
	}

	_ = s.repo.CreateAuditLog(ctx, &AuditLog{
		ID:         uuid.New(),
		DocumentID: &id,
		Action:     AuditActionDelete,
		UserID:     userID,
		IPAddress:  "127.0.0.1",
		UserAgent:  "ArchiveService/1.0",
		Details: map[string]any{
			"filename": doc.Filename,
		},
		CreatedAt: time.Now().UTC(),
	})

	return nil
}

func (s *ArchiveService) Search(ctx context.Context, req SearchRequest) (*SearchResult, error) {
	if s.searchEngine != nil {
		res, err := s.searchEngine.Search(ctx, req)
		if err == nil {
			_ = s.repo.CreateAuditLog(ctx, &AuditLog{
				ID:        uuid.New(),
				Action:    AuditActionSearch,
				UserID:    uuid.Nil,
				IPAddress: "127.0.0.1",
				UserAgent: "ArchiveService/1.0",
				Details: map[string]any{
					"query": req.Query,
					"total": res.Total,
				},
				CreatedAt: time.Now().UTC(),
			})
		}
		return res, err
	}
	return &SearchResult{Data: []SearchHit{}, Total: 0}, nil
}

func (s *ArchiveService) GenerateDownloadURL(ctx context.Context, docID uuid.UUID, clientIP string, ttl time.Duration) (string, error) {
	doc, err := s.repo.GetDocumentByID(ctx, docID)
	if err != nil {
		return "", err
	}

	url, err := s.signer.GenerateSignedURL(doc.ID, doc.Filename, clientIP, ttl)
	if err != nil {
		return "", err
	}

	// Record audit log for DOWNLOAD request
	_ = s.repo.CreateAuditLog(ctx, &AuditLog{
		ID:         uuid.New(),
		DocumentID: &docID,
		Action:     AuditActionDownload,
		UserID:     doc.UploadedBy,
		IPAddress:  clientIP,
		UserAgent:  "ArchiveService/1.0",
		Details: map[string]any{
			"signed_url": url,
		},
		CreatedAt: time.Now(),
	})

	return url, nil
}

func (s *ArchiveService) ValidateDownloadToken(tokenString string, clientIP string) (uuid.UUID, error) {
	return s.signer.ValidateSignedURLToken(tokenString, clientIP)
}

func (s *ArchiveService) SetLegalHold(ctx context.Context, docID uuid.UUID, legalHold bool, reason string, userID uuid.UUID) (*ArchiveDocument, error) {
	doc, err := s.repo.GetDocumentByID(ctx, docID)
	if err != nil {
		return nil, err
	}

	doc.LegalHold = legalHold
	doc.LegalHoldReason = reason
	if err := s.repo.UpdateDocument(ctx, doc); err != nil {
		return nil, err
	}

	action := AuditActionLegalHold
	if legalHold {
		action = "APPLY_LEGAL_HOLD"
	} else {
		action = "RELEASE_LEGAL_HOLD"
	}

	_ = s.repo.CreateAuditLog(ctx, &AuditLog{
		ID:         uuid.New(),
		DocumentID: &docID,
		Action:     action,
		UserID:     userID,
		IPAddress:  "127.0.0.1",
		UserAgent:  "ArchiveService/1.0",
		Details: map[string]any{
			"legal_hold": legalHold,
			"reason":     reason,
		},
		CreatedAt: time.Now(),
	})

	return doc, nil
}

func (s *ArchiveService) UpdateRetention(ctx context.Context, docID uuid.UUID, req UpdateRetentionRequest, userID uuid.UUID) (*ArchiveDocument, error) {
	doc, err := s.repo.GetDocumentByID(ctx, docID)
	if err != nil {
		return nil, err
	}

	switch req.Action {
	case "LEGAL_HOLD":
		return s.SetLegalHold(ctx, docID, true, req.Reason, userID)
	case "RELEASE_HOLD":
		return s.SetLegalHold(ctx, docID, false, req.Reason, userID)
	case "EXTEND", "REDUCE":
		if req.RetainUntil != nil {
			doc.RetainUntil = *req.RetainUntil
		}
	default:
		if req.RetainUntil != nil {
			doc.RetainUntil = *req.RetainUntil
		}
	}

	if err := s.repo.UpdateDocument(ctx, doc); err != nil {
		return nil, err
	}

	_ = s.repo.CreateAuditLog(ctx, &AuditLog{
		ID:         uuid.New(),
		DocumentID: &docID,
		Action:     AuditActionRetentionChange,
		UserID:     userID,
		IPAddress:  "127.0.0.1",
		UserAgent:  "ArchiveService/1.0",
		Details: map[string]any{
			"action":       req.Action,
			"retain_until": doc.RetainUntil,
			"reason":       req.Reason,
		},
		CreatedAt: time.Now().UTC(),
	})

	return doc, nil
}

func (s *ArchiveService) ProcessBulkAction(ctx context.Context, req BulkActionRequest, userID uuid.UUID) (*BulkActionResponse, error) {
	if s.bulkProcessor != nil {
		return s.bulkProcessor.ProcessBulkAction(ctx, req, userID)
	}
	return nil, nil
}

func (s *ArchiveService) StreamBulkZip(ctx context.Context, ids []uuid.UUID, w io.Writer) error {
	if s.bulkProcessor != nil {
		return s.bulkProcessor.StreamBulkZip(ctx, ids, w)
	}
	return nil
}

func (s *ArchiveService) ProcessGDPRRequest(ctx context.Context, req GDPRRequest) (*GDPRResponse, error) {
	if s.gdprProcessor != nil {
		return s.gdprProcessor.ProcessGDPRRequest(ctx, req)
	}
	return nil, nil
}

func (s *ArchiveService) ListRetentionPolicies(ctx context.Context) ([]*RetentionPolicy, error) {
	return s.repo.ListRetentionPolicies(ctx)
}

func (s *ArchiveService) CreateRetentionPolicy(ctx context.Context, req CreateRetentionPolicyRequest) (*RetentionPolicy, error) {
	policy := &RetentionPolicy{
		ID:                uuid.New(),
		Name:              req.Name,
		Category:          req.Category,
		RetentionYears:    req.RetentionYears,
		AutoDelete:        req.AutoDelete,
		LegalHoldOverride: req.LegalHoldOverride,
		Description:       req.Description,
		CreatedAt:         time.Now().UTC(),
		UpdatedAt:         time.Now().UTC(),
	}
	if err := s.repo.CreateRetentionPolicy(ctx, policy); err != nil {
		return nil, err
	}
	return policy, nil
}

func (s *ArchiveService) ListAuditLogs(ctx context.Context, filter AuditLogFilter) ([]*AuditLog, int64, error) {
	return s.repo.ListAuditLogs(ctx, filter)
}

func (s *ArchiveService) GetAuditLogsByDocument(ctx context.Context, docID uuid.UUID) ([]*AuditLog, error) {
	return s.repo.GetAuditLogsByDocument(ctx, docID)
}

func (s *ArchiveService) RunRetentionEvaluator(ctx context.Context) error {
	if s.retentionEngine != nil {
		return s.retentionEngine.EvaluatePolicies(ctx)
	}
	return nil
}

func (s *ArchiveService) RunStorageTierMigration(ctx context.Context) error {
	if s.retentionEngine != nil {
		return s.retentionEngine.MigrateStorageTiers(ctx)
	}
	return nil
}

func (s *ArchiveService) StartBackgroundTasks(ctx context.Context) error {
	if s.ocrWorkerPool != nil {
		s.ocrWorkerPool.Start()
	}
	if s.retentionEngine != nil {
		if err := s.retentionEngine.Start(ctx); err != nil {
			return err
		}
	}
	return nil
}

func (s *ArchiveService) StopBackgroundTasks() {
	if s.ocrWorkerPool != nil {
		s.ocrWorkerPool.Stop()
	}
	if s.retentionEngine != nil {
		s.retentionEngine.Stop()
	}
}

func (s *ArchiveService) HandleGDPRRequest(ctx context.Context, requestType string, docID uuid.UUID) error {
	req := GDPRRequest{
		Type:       requestType,
		DocumentID: &docID,
	}
	_, err := s.ProcessGDPRRequest(ctx, req)
	return err
}
