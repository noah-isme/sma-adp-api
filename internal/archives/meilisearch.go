package archives

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/google/uuid"
)

// SearchEngine represents full-text search indexer and query provider.
type SearchEngine interface {
	IndexDocument(ctx context.Context, doc *ArchiveDocument) error
	DeleteDocumentIndex(ctx context.Context, id uuid.UUID) error
	Search(ctx context.Context, req SearchRequest) (*SearchResult, error)
}

// MeiliConfig contains connection settings for Meilisearch.
type MeiliConfig struct {
	Host   string
	APIKey string
	Index  string
}

type MeilisearchConfig = MeiliConfig

// MeiliArchiveDoc is the document representation sent to Meilisearch.
type MeiliArchiveDoc struct {
	ID               string                 `json:"id"`
	Filename         string                 `json:"filename"`
	OriginalFilename string                 `json:"original_filename"`
	MimeType         string                 `json:"mime_type"`
	SizeBytes        int64                  `json:"size_bytes"`
	Checksum         string                 `json:"checksum"`
	StorageTier      string                 `json:"storage_tier"`
	Category         string                 `json:"category"`
	Tags             []string               `json:"tags"`
	Metadata         map[string]interface{} `json:"metadata"`
	OCRText          string                 `json:"ocr_text"`
	OCRStatus        string                 `json:"ocr_status"`
	RetainUntil      int64                  `json:"retain_until"`
	LegalHold        bool                   `json:"legal_hold"`
	UploadedBy       string                 `json:"uploaded_by"`
	UploadedAt       int64                  `json:"uploaded_at"`
	DeletedAt        *int64                 `json:"deleted_at,omitempty"`
}

// MeiliSearchEngine implements SearchEngine client wrapper via HTTP REST API with automatic fallback.
type MeiliSearchEngine struct {
	client   *http.Client
	host     string
	apiKey   string
	index    string
	fallback SearchEngine
}

type MeilisearchEngine = MeiliSearchEngine

// NewMeiliSearchEngine initializes a Meilisearch client wrapper.
func NewMeiliSearchEngine(cfg MeiliConfig) *MeiliSearchEngine {
	return NewMeiliSearchEngineWithFallback(cfg, nil)
}

// NewMeiliSearchEngineWithFallback initializes Meilisearch client with a fallback search engine.
func NewMeiliSearchEngineWithFallback(cfg MeiliConfig, fallback SearchEngine) *MeiliSearchEngine {
	host := strings.TrimRight(cfg.Host, "/")
	indexName := cfg.Index
	if indexName == "" {
		indexName = "archives"
	}
	engine := &MeiliSearchEngine{
		client:   &http.Client{Timeout: 5 * time.Second},
		host:     host,
		apiKey:   cfg.APIKey,
		index:    indexName,
		fallback: fallback,
	}
	if host != "" {
		_ = engine.ensureIndexSettings()
	}
	return engine
}

func NewMeilisearchEngine(cfg MeiliConfig) *MeiliSearchEngine {
	return NewMeiliSearchEngine(cfg)
}

func (m *MeiliSearchEngine) SetFallback(fallback SearchEngine) {
	m.fallback = fallback
}

func (m *MeiliSearchEngine) ensureIndexSettings() error {
	if m.host == "" {
		return fmt.Errorf("empty meilisearch host")
	}

	// 1. Create index if not exists
	createPayload := map[string]string{
		"uid":        m.index,
		"primaryKey": "id",
	}
	bodyBytes, _ := json.Marshal(createPayload)
	req, err := http.NewRequest(http.MethodPost, fmt.Sprintf("%s/indexes", m.host), bytes.NewReader(bodyBytes))
	if err == nil {
		req.Header.Set("Content-Type", "application/json")
		if m.apiKey != "" {
			req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", m.apiKey))
		}
		resp, err := m.client.Do(req)
		if err == nil {
			_ = resp.Body.Close()
		}
	}

	// 2. Configure index settings
	settingsPayload := map[string]interface{}{
		"searchableAttributes": []string{"filename", "original_filename", "ocr_text", "tags"},
		"filterableAttributes": []string{"category", "tags", "storage_tier", "legal_hold", "ocr_status", "uploaded_by", "uploaded_at", "retain_until", "deleted_at"},
		"sortableAttributes":   []string{"uploaded_at", "retain_until", "size_bytes"},
	}
	settingsBytes, _ := json.Marshal(settingsPayload)
	settingsReq, err := http.NewRequest(http.MethodPatch, fmt.Sprintf("%s/indexes/%s/settings", m.host, m.index), bytes.NewReader(settingsBytes))
	if err != nil {
		return err
	}
	settingsReq.Header.Set("Content-Type", "application/json")
	if m.apiKey != "" {
		settingsReq.Header.Set("Authorization", fmt.Sprintf("Bearer %s", m.apiKey))
	}
	resp, err := m.client.Do(settingsReq)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return nil
}

func (m *MeiliSearchEngine) IndexDocument(ctx context.Context, doc *ArchiveDocument) error {
	if m.host == "" {
		if m.fallback != nil {
			return m.fallback.IndexDocument(ctx, doc)
		}
		return fmt.Errorf("meilisearch host unconfigured")
	}

	var deletedAtUnix *int64
	if doc.DeletedAt != nil {
		unix := doc.DeletedAt.Unix()
		deletedAtUnix = &unix
	}

	meiliDoc := MeiliArchiveDoc{
		ID:               doc.ID.String(),
		Filename:         doc.Filename,
		OriginalFilename: doc.OriginalFilename,
		MimeType:         doc.MimeType,
		SizeBytes:        doc.SizeBytes,
		Checksum:         doc.Checksum,
		StorageTier:      string(doc.StorageTier),
		Category:         string(doc.Category),
		Tags:             doc.Tags,
		Metadata:         doc.Metadata,
		OCRText:          doc.OCRText,
		OCRStatus:        string(doc.OCRStatus),
		RetainUntil:      doc.RetainUntil.Unix(),
		LegalHold:        doc.LegalHold,
		UploadedBy:       doc.UploadedBy.String(),
		UploadedAt:       doc.UploadedAt.Unix(),
		DeletedAt:        deletedAtUnix,
	}

	bodyBytes, err := json.Marshal([]MeiliArchiveDoc{meiliDoc})
	if err != nil {
		return fmt.Errorf("marshal meilisearch doc: %w", err)
	}

	url := fmt.Sprintf("%s/indexes/%s/documents", m.host, m.index)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(bodyBytes))
	if err != nil {
		if m.fallback != nil {
			return m.fallback.IndexDocument(ctx, doc)
		}
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	if m.apiKey != "" {
		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", m.apiKey))
	}

	resp, err := m.client.Do(req)
	if err != nil {
		if m.fallback != nil {
			return m.fallback.IndexDocument(ctx, doc)
		}
		return fmt.Errorf("meilisearch index request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		if m.fallback != nil {
			return m.fallback.IndexDocument(ctx, doc)
		}
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("meilisearch index failed (%d): %s", resp.StatusCode, string(respBody))
	}

	return nil
}

func (m *MeiliSearchEngine) DeleteDocumentIndex(ctx context.Context, id uuid.UUID) error {
	if m.host == "" {
		if m.fallback != nil {
			return m.fallback.DeleteDocumentIndex(ctx, id)
		}
		return nil
	}

	url := fmt.Sprintf("%s/indexes/%s/documents/%s", m.host, m.index, id.String())
	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, url, nil)
	if err != nil {
		if m.fallback != nil {
			return m.fallback.DeleteDocumentIndex(ctx, id)
		}
		return err
	}
	if m.apiKey != "" {
		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", m.apiKey))
	}

	resp, err := m.client.Do(req)
	if err != nil {
		if m.fallback != nil {
			return m.fallback.DeleteDocumentIndex(ctx, id)
		}
		return fmt.Errorf("meilisearch delete index: %w", err)
	}
	defer resp.Body.Close()

	if m.fallback != nil {
		_ = m.fallback.DeleteDocumentIndex(ctx, id)
	}
	return nil
}

func (m *MeiliSearchEngine) Search(ctx context.Context, req SearchRequest) (*SearchResult, error) {
	if m.host == "" {
		if m.fallback != nil {
			return m.fallback.Search(ctx, req)
		}
		return nil, fmt.Errorf("meilisearch host unconfigured")
	}

	startTime := time.Now()
	page := req.Page
	if page < 1 {
		page = 1
	}
	limit := req.Limit
	if limit < 1 {
		limit = 20
	}
	offset := (page - 1) * limit

	var filters []string
	filters = append(filters, "deleted_at IS NULL")
	if req.Category != "" {
		filters = append(filters, fmt.Sprintf("category = '%s'", escapeMeiliFilterVal(string(req.Category))))
	}
	if req.StorageTier != "" {
		filters = append(filters, fmt.Sprintf("storage_tier = '%s'", escapeMeiliFilterVal(string(req.StorageTier))))
	}
	if req.LegalHoldOnly {
		filters = append(filters, "legal_hold = true")
	}
	if req.DateFrom != nil {
		filters = append(filters, fmt.Sprintf("uploaded_at >= %d", req.DateFrom.Unix()))
	}
	if req.DateTo != nil {
		filters = append(filters, fmt.Sprintf("uploaded_at <= %d", req.DateTo.Unix()))
	}
	for _, tag := range req.Tags {
		filters = append(filters, fmt.Sprintf("tags = '%s'", escapeMeiliFilterVal(tag)))
	}

	searchPayload := map[string]interface{}{
		"q":                     req.Query,
		"offset":                offset,
		"limit":                 limit,
		"attributesToHighlight": []string{"ocr_text", "filename", "original_filename"},
		"highlightPreTag":       "<em>",
		"highlightPostTag":      "</em>",
	}
	if len(filters) > 0 {
		searchPayload["filter"] = strings.Join(filters, " AND ")
	}

	bodyBytes, err := json.Marshal(searchPayload)
	if err != nil {
		if m.fallback != nil {
			return m.fallback.Search(ctx, req)
		}
		return nil, err
	}

	url := fmt.Sprintf("%s/indexes/%s/search", m.host, m.index)
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(bodyBytes))
	if err != nil {
		if m.fallback != nil {
			return m.fallback.Search(ctx, req)
		}
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	if m.apiKey != "" {
		httpReq.Header.Set("Authorization", fmt.Sprintf("Bearer %s", m.apiKey))
	}

	resp, err := m.client.Do(httpReq)
	if err != nil {
		if m.fallback != nil {
			return m.fallback.Search(ctx, req)
		}
		return nil, fmt.Errorf("meilisearch search request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		if m.fallback != nil {
			return m.fallback.Search(ctx, req)
		}
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("meilisearch search status %d: %s", resp.StatusCode, string(respBody))
	}

	var resStruct struct {
		Hits               []map[string]interface{} `json:"hits"`
		EstimatedTotalHits int64                    `json:"estimatedTotalHits"`
		TotalHits          int64                    `json:"totalHits"`
		ProcessingTimeMs   int64                    `json:"processingTimeMs"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&resStruct); err != nil {
		if m.fallback != nil {
			return m.fallback.Search(ctx, req)
		}
		return nil, fmt.Errorf("decode meilisearch search response: %w", err)
	}

	total := resStruct.TotalHits
	if total == 0 {
		total = resStruct.EstimatedTotalHits
	}

	var hits []SearchHit
	for _, hitMap := range resStruct.Hits {
		docID, _ := uuid.Parse(fmt.Sprintf("%v", hitMap["id"]))
		filename, _ := hitMap["filename"].(string)
		origFilename, _ := hitMap["original_filename"].(string)
		mimeType, _ := hitMap["mime_type"].(string)
		ocrText, _ := hitMap["ocr_text"].(string)
		ocrStatusStr, _ := hitMap["ocr_status"].(string)
		tierStr, _ := hitMap["storage_tier"].(string)
		catStr, _ := hitMap["category"].(string)
		legalHold, _ := hitMap["legal_hold"].(bool)

		snippet := extractSnippet(ocrText, req.Query)
		if formatted, ok := hitMap["_formatted"].(map[string]interface{}); ok {
			if fmtSnippet, ok := formatted["ocr_text"].(string); ok && strings.Contains(fmtSnippet, "<em>") {
				snippet = fmtSnippet
			}
		}

		hits = append(hits, SearchHit{
			ID:               docID,
			Filename:         filename,
			OriginalFilename: origFilename,
			MimeType:         mimeType,
			StorageTier:      StorageTier(tierStr),
			Category:         DocumentCategory(catStr),
			OCRStatus:        OCRStatus(ocrStatusStr),
			OCRText:          ocrText,
			Snippet:          snippet,
			LegalHold:        legalHold,
		})
	}

	return &SearchResult{
		Data:        hits,
		Total:       total,
		Page:        page,
		Limit:       limit,
		QueryTimeMs: time.Since(startTime).Milliseconds(),
	}, nil
}

// PostgresSearchEngine implements SearchEngine fallback using PostgreSQL queries.
type PostgresSearchEngine struct {
	repo Repository
}

// NewPostgresSearchEngine constructs a PostgreSQL ILIKE fallback search engine.
func NewPostgresSearchEngine(repo Repository) *PostgresSearchEngine {
	return &PostgresSearchEngine{repo: repo}
}

func (p *PostgresSearchEngine) IndexDocument(ctx context.Context, doc *ArchiveDocument) error {
	return nil
}

func (p *PostgresSearchEngine) DeleteDocumentIndex(ctx context.Context, id uuid.UUID) error {
	return nil
}

func (p *PostgresSearchEngine) Search(ctx context.Context, req SearchRequest) (*SearchResult, error) {
	if p.repo == nil {
		return &SearchResult{Data: []SearchHit{}, Total: 0, Page: req.Page, Limit: req.Limit}, nil
	}

	startTime := time.Now()
	page := req.Page
	if page < 1 {
		page = 1
	}
	limit := req.Limit
	if limit < 1 {
		limit = 20
	}
	offset := (page - 1) * limit

	filter := ArchiveFilter{
		Query:         req.Query,
		Category:      req.Category,
		Tags:          req.Tags,
		StorageTier:   req.StorageTier,
		LegalHoldOnly: req.LegalHoldOnly,
		DateFrom:      req.DateFrom,
		DateTo:        req.DateTo,
		Limit:         limit,
		Offset:        offset,
	}

	docs, total, err := p.repo.ListDocuments(ctx, filter)
	if err != nil {
		return nil, fmt.Errorf("postgres search fallback: %w", err)
	}

	var hits []SearchHit
	for _, doc := range docs {
		snippet := extractSnippet(doc.OCRText, req.Query)
		hits = append(hits, SearchHit{
			ID:               doc.ID,
			Filename:         doc.Filename,
			OriginalFilename: doc.OriginalFilename,
			MimeType:         doc.MimeType,
			SizeBytes:        doc.SizeBytes,
			Checksum:         doc.Checksum,
			StorageTier:      doc.StorageTier,
			Category:         doc.Category,
			Tags:             doc.Tags,
			Metadata:         doc.Metadata,
			OCRStatus:        doc.OCRStatus,
			OCRText:          doc.OCRText,
			Snippet:          snippet,
			RetainUntil:      doc.RetainUntil,
			LegalHold:        doc.LegalHold,
			UploadedBy:       doc.UploadedBy,
			UploadedAt:       doc.UploadedAt,
		})
	}

	return &SearchResult{
		Data:        hits,
		Total:       total,
		Page:        page,
		Limit:       limit,
		QueryTimeMs: time.Since(startTime).Milliseconds(),
	}, nil
}

// HybridSearchEngine encapsulates primary (Meilisearch) and fallback engines.
type HybridSearchEngine struct {
	primary  SearchEngine
	fallback SearchEngine
}

// NewHybridSearchEngine creates a decorator combining primary and fallback engines.
func NewHybridSearchEngine(primary SearchEngine, fallback SearchEngine) *HybridSearchEngine {
	return &HybridSearchEngine{
		primary:  primary,
		fallback: fallback,
	}
}

func (h *HybridSearchEngine) IndexDocument(ctx context.Context, doc *ArchiveDocument) error {
	if h.primary != nil {
		_ = h.primary.IndexDocument(ctx, doc)
	}
	if h.fallback != nil {
		_ = h.fallback.IndexDocument(ctx, doc)
	}
	return nil
}

func (h *HybridSearchEngine) DeleteDocumentIndex(ctx context.Context, id uuid.UUID) error {
	if h.primary != nil {
		_ = h.primary.DeleteDocumentIndex(ctx, id)
	}
	if h.fallback != nil {
		_ = h.fallback.DeleteDocumentIndex(ctx, id)
	}
	return nil
}

func (h *HybridSearchEngine) Search(ctx context.Context, req SearchRequest) (*SearchResult, error) {
	if h.primary != nil {
		res, err := h.primary.Search(ctx, req)
		if err == nil && res != nil {
			return res, nil
		}
	}
	if h.fallback != nil {
		return h.fallback.Search(ctx, req)
	}
	return &SearchResult{
		Data:  []SearchHit{},
		Total: 0,
		Page:  req.Page,
		Limit: req.Limit,
	}, nil
}

// NewSearchEngine constructs the appropriate SearchEngine based on config and repository.
func NewSearchEngine(cfg MeiliConfig, repo Repository) SearchEngine {
	var fallback SearchEngine
	if repo != nil {
		fallback = NewPostgresSearchEngine(repo)
	} else {
		fallback = NewMemorySearchEngine()
	}

	if cfg.Host != "" {
		return NewMeiliSearchEngineWithFallback(cfg, fallback)
	}
	return fallback
}

// MemorySearchEngine provides an in-memory search implementation for testing.
type MemorySearchEngine struct {
	mu          sync.RWMutex
	indexedDocs map[uuid.UUID]*ArchiveDocument
}

// NewMemorySearchEngine initializes a thread-safe MemorySearchEngine.
func NewMemorySearchEngine() *MemorySearchEngine {
	return &MemorySearchEngine{
		indexedDocs: make(map[uuid.UUID]*ArchiveDocument),
	}
}

func (s *MemorySearchEngine) IndexDocument(ctx context.Context, doc *ArchiveDocument) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	cloned := *doc
	s.indexedDocs[doc.ID] = &cloned
	return nil
}

func (s *MemorySearchEngine) DeleteDocumentIndex(ctx context.Context, id uuid.UUID) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	delete(s.indexedDocs, id)
	return nil
}

func (s *MemorySearchEngine) Search(ctx context.Context, req SearchRequest) (*SearchResult, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	startTime := time.Now()
	var hits []SearchHit

	queryLower := strings.ToLower(strings.TrimSpace(req.Query))

	for _, doc := range s.indexedDocs {
		if doc.DeletedAt != nil {
			continue
		}

		if req.Category != "" && doc.Category != req.Category {
			continue
		}

		if req.StorageTier != "" && doc.StorageTier != req.StorageTier {
			continue
		}

		if req.LegalHoldOnly && !doc.LegalHold {
			continue
		}

		if req.DateFrom != nil && doc.UploadedAt.Before(*req.DateFrom) {
			continue
		}

		if req.DateTo != nil && doc.UploadedAt.After(*req.DateTo) {
			continue
		}

		if len(req.Tags) > 0 {
			docTagMap := make(map[string]bool)
			for _, t := range doc.Tags {
				docTagMap[t] = true
			}
			matchedAll := true
			for _, requiredTag := range req.Tags {
				if !docTagMap[requiredTag] {
					matchedAll = false
					break
				}
			}
			if !matchedAll {
				continue
			}
		}

		matched := false
		snippet := ""

		if queryLower == "" {
			matched = true
			snippet = extractSnippet(doc.OCRText, "")
		} else {
			fNameLower := strings.ToLower(doc.Filename)
			origLower := strings.ToLower(doc.OriginalFilename)
			ocrLower := strings.ToLower(doc.OCRText)

			if strings.Contains(fNameLower, queryLower) || strings.Contains(origLower, queryLower) || strings.Contains(ocrLower, queryLower) {
				matched = true
				snippet = extractSnippet(doc.OCRText, queryLower)
			}
		}

		if matched {
			hits = append(hits, SearchHit{
				ID:               doc.ID,
				Filename:         doc.Filename,
				OriginalFilename: doc.OriginalFilename,
				MimeType:         doc.MimeType,
				SizeBytes:        doc.SizeBytes,
				Checksum:         doc.Checksum,
				StorageTier:      doc.StorageTier,
				Category:         doc.Category,
				Tags:             doc.Tags,
				Metadata:         doc.Metadata,
				OCRStatus:        doc.OCRStatus,
				OCRText:          doc.OCRText,
				Snippet:          snippet,
				RetainUntil:      doc.RetainUntil,
				LegalHold:        doc.LegalHold,
				UploadedBy:       doc.UploadedBy,
				UploadedAt:       doc.UploadedAt,
			})
		}
	}

	page := req.Page
	if page < 1 {
		page = 1
	}
	limit := req.Limit
	if limit < 1 {
		limit = 20
	}

	total := int64(len(hits))
	startIdx := (page - 1) * limit
	endIdx := startIdx + limit

	var pageHits []SearchHit
	if startIdx < len(hits) {
		if endIdx > len(hits) {
			endIdx = len(hits)
		}
		pageHits = hits[startIdx:endIdx]
	} else {
		pageHits = []SearchHit{}
	}

	elapsed := time.Since(startTime).Milliseconds()

	return &SearchResult{
		Data:        pageHits,
		Total:       total,
		Page:        page,
		Limit:       limit,
		QueryTimeMs: elapsed,
	}, nil
}

func escapeMeiliFilterVal(val string) string {
	val = strings.ReplaceAll(val, "\\", "\\\\")
	val = strings.ReplaceAll(val, "'", "\\'")
	return val
}

func extractSnippet(fullText string, query string) string {
	if fullText == "" {
		return ""
	}
	trimmedQuery := strings.TrimSpace(query)
	runes := []rune(fullText)
	if len(runes) == 0 {
		return ""
	}
	if trimmedQuery == "" {
		if len(runes) <= 100 {
			return string(runes)
		}
		return string(runes[:100]) + "..."
	}

	lowerText := strings.ToLower(fullText)
	lowerQuery := strings.ToLower(trimmedQuery)

	matchStartRune := -1
	matchEndRune := -1

	// Strategy 1: 1-to-1 rune case-folding search (preserves rune indices accurately across multi-byte Unicode strings)
	lowerRunes := make([]rune, len(runes))
	for i, r := range runes {
		lr := []rune(strings.ToLower(string(r)))
		if len(lr) > 0 {
			lowerRunes[i] = lr[0]
		} else {
			lowerRunes[i] = unicode.ToLower(r)
		}
	}
	qRunes := []rune(trimmedQuery)
	queryRunes := make([]rune, len(qRunes))
	for j, r := range qRunes {
		lr := []rune(strings.ToLower(string(r)))
		if len(lr) > 0 {
			queryRunes[j] = lr[0]
		} else {
			queryRunes[j] = unicode.ToLower(r)
		}
	}

	if len(queryRunes) > 0 && len(queryRunes) <= len(lowerRunes) {
		for i := 0; i <= len(lowerRunes)-len(queryRunes); i++ {
			found := true
			for j := 0; j < len(queryRunes); j++ {
				r1 := lowerRunes[i+j]
				r2 := queryRunes[j]
				if r1 != r2 && unicode.ToLower(r1) != unicode.ToLower(r2) {
					found = false
					break
				}
			}
			if found {
				matchStartRune = i
				matchEndRune = i + len(queryRunes)
				break
			}
		}
	}

	// Strategy 2: Fallback to byte offset calculation on lowerText if 1-to-1 rune search didn't match
	if matchStartRune == -1 || matchEndRune == -1 {
		idx := strings.Index(lowerText, lowerQuery)
		if idx != -1 {
			currByte := 0
			for rIdx, r := range runes {
				rLen := utf8.RuneLen(r)
				if currByte == idx {
					matchStartRune = rIdx
				}
				currByte += rLen
				if matchStartRune != -1 && currByte >= idx+len(lowerQuery) {
					matchEndRune = rIdx + 1
					break
				}
			}
		}
	}

	// Always safely clamp match bounds to prevent any slice bounds panic
	if matchStartRune < 0 {
		matchStartRune = 0
	}
	if matchStartRune > len(runes) {
		matchStartRune = len(runes)
	}
	if matchEndRune < matchStartRune {
		matchEndRune = matchStartRune
	}
	if matchEndRune > len(runes) {
		matchEndRune = len(runes)
	}

	if matchStartRune >= matchEndRune {
		if len(runes) <= 100 {
			return string(runes)
		}
		return string(runes[:100]) + "..."
	}

	startRune := matchStartRune - 30
	if startRune < 0 {
		startRune = 0
	}
	endRune := matchEndRune + 40
	if endRune > len(runes) {
		endRune = len(runes)
	}
	if startRune > matchStartRune {
		startRune = matchStartRune
	}
	if endRune < matchEndRune {
		endRune = matchEndRune
	}

	prefix := ""
	if startRune > 0 {
		prefix = "..."
	}
	suffix := ""
	if endRune < len(runes) {
		suffix = "..."
	}

	matchedSubstring := string(runes[matchStartRune:matchEndRune])
	snippetCore := string(runes[startRune:matchStartRune]) + fmt.Sprintf("<em>%s</em>", matchedSubstring) + string(runes[matchEndRune:endRune])

	return prefix + snippetCore + suffix
}
