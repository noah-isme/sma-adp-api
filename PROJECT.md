# Project: Archives Module Backend Infrastructure

## Architecture
The Archives module backend infrastructure is implemented cleanly within `internal/archives` in `sma-adp-api`. It provides full-text search indexing via Meilisearch (with PostgreSQL fallback), an async OCR worker pool, a retention policy engine with legal hold support, S3 storage tiering simulation (HOT/WARM/COLD), HMAC-SHA256 signed URLs with IP binding, bulk operations, and GDPR/PDPA data subject request processors.

### Data Flow
1. **Upload Flow**: Client HTTP POST -> `ArchiveHandler` -> `ArchiveService` -> Save file to storage -> Create DB record (`PENDING` OCR) -> Push job to `OCRWorkerPool`.
2. **OCR & Search Flow**: `OCRWorkerPool` worker goroutine extracts text -> Updates DB (`ocr_text`, `ocr_status='COMPLETED'`) -> Indexes document in Meilisearch `archives` index.
3. **Search Flow**: Client HTTP GET -> `ArchiveHandler` -> `SearchService` -> Queries Meilisearch (or Postgres ILIKE fallback) -> Returns highlighted snippets.
4. **Retention Flow**: `RetentionEngine` background ticker -> Scans records past `retain_until` -> Checks legal hold (skips if hold active, unless policy override set) -> Performs auto-delete / audit log -> Checks storage tier migration (HOT <= 90d, WARM 90d-2y, COLD > 2y).
5. **Security & Download**: `SignedURLSigner` -> Generates HMAC token (with TTL & requester IP) -> Download handler validates signature, TTL, and IP binding.
6. **Bulk & GDPR**: Bulk zip stream / mass updates; GDPR processors for Access (ZIP), Rectification (metadata update), Erasure (delete if valid), Portability (JSON).

## Feature Inventory
Every feature identified during survey is cataloged and assigned to a milestone below.

| # | Feature | Description | Milestone | Source |
|---|---|---|---|---|
| F01 | DB Migration & Schema | Migration `000023_archives_retention_search.up.sql` creating `retention_policies`, `archive_audit_log`, and altering `archives` | M1 | Survey / Guide §9 |
| F02 | Pre-seeded Policies | Seed default retention policies for 8 document categories | M1 | Survey / Guide §2.2 |
| F03 | Models & Repository | Data models, DTOs, and SQL repository in `internal/archives` | M1 | Survey / Guide §2.1 |
| F04 | Meilisearch Integration | Meilisearch index `archives` setup, indexing, and Postgres ILIKE fallback search | M2 | Survey / ORIGINAL_REQUEST R1 |
| F05 | Full-text Snippet Search | BM25 full-text search with highlight snippets and search filters | M2 | Survey / Guide §3.2 |
| F06 | Async OCR Worker Pool | Go channel/worker pool for async text extraction from uploaded files | M2 | Survey / ORIGINAL_REQUEST R1 |
| F07 | Multi-format Parsing | Text extraction engine for PDF, PNG/JPG/TIFF, DOCX/XLSX, and ZIP | M2 | Survey / Guide §4.3 |
| F08 | Retention Engine | Policy evaluator checking `retain_until` dates and auto-deletion | M3 | Survey / ORIGINAL_REQUEST R2 |
| F09 | Legal Hold Mechanics | Apply/release legal hold lock; skip deletion on active hold with audit log `SKIPPED_LEGAL_HOLD` | M3 | Survey / Guide §3.4 |
| F10 | Storage Tier Migration | Evaluates document age and manages HOT -> WARM -> COLD storage tiering | M3 | Survey / Guide §6.2 |
| F11 | Enriched Signed URLs | HMAC-SHA256 signed URLs with TTL and client IP address binding validation | M4 | Survey / ORIGINAL_REQUEST R3 |
| F12 | Bulk Operations | Bulk Zip stream download, bulk delete, bulk category change, bulk retention update | M4 | Survey / Guide §3.5 |
| F13 | GDPR/PDPA Handlers | Data subject request processors: ACCESS (Zip), RECTIFICATION (Metadata), ERASURE (Delete), PORTABILITY (JSON) | M4 | Survey / ORIGINAL_REQUEST R3 |
| F14 | Audit Logging | Complete structured audit trail logging in `archive_audit_log` | M4 | Survey / Guide §10.1 |
| F15 | Integration Test Suite | Comprehensive integration test `TestArchiveLifecycle_Integration` verifying end-to-end lifecycle | M5 | Survey / ORIGINAL_REQUEST Acceptance |

## Milestones

| # | Name | Scope | Dependencies | Status |
|---|---|---|---|---|
| M1 | Database & Core Models | Migration `000023`, models, DTOs, repository SQL queries, pre-seeded retention policies | None | DONE |
| M2 | Search & OCR Pipeline | Meilisearch client wrapper with Postgres fallback, OCR worker pool & multi-format parsers | M1 | DONE |
| M3 | Retention & Storage Engine | Scheduled retention policy evaluator, legal hold mechanics, S3 storage tier migrator | M1, M2 | PLANNED |
| M4 | Security & Compliance API | Signed URL IP binding, bulk Zip & endpoints, GDPR request handlers, HTTP handler wiring | M1, M2, M3 | PLANNED |
| M5 | Integration Verification & Hardening | `TestArchiveLifecycle_Integration`, unit tests, build validation, and Forensic Audit | M1, M2, M3, M4 | PLANNED |

## Interface Contracts

### 1. Repository Interface (`internal/archives/repository.go`)
```go
type Repository interface {
    CreateDocument(ctx context.Context, doc *ArchiveDocument) error
    GetDocumentByID(ctx context.Context, id uuid.UUID) (*ArchiveDocument, error)
    UpdateDocument(ctx context.Context, doc *ArchiveDocument) error
    SoftDeleteDocument(ctx context.Context, id uuid.UUID) error
    ListDocuments(ctx context.Context, filter ArchiveFilter) ([]*ArchiveDocument, int64, error)
    
    GetRetentionPolicyByID(ctx context.Context, id uuid.UUID) (*RetentionPolicy, error)
    GetDefaultPolicyByCategory(ctx context.Context, category string) (*RetentionPolicy, error)
    ListRetentionPolicies(ctx context.Context) ([]*RetentionPolicy, error)
    
    GetExpiredDocuments(ctx context.Context, limit int) ([]*ArchiveDocument, error)
    GetDocumentsForTierMigration(ctx context.Context, currentTier string, olderThanDays int) ([]*ArchiveDocument, error)
    
    CreateAuditLog(ctx context.Context, log *AuditLog) error
}
```

### 2. Search Engine Interface (`internal/archives/search.go`)
```go
type SearchEngine interface {
    IndexDocument(ctx context.Context, doc *ArchiveDocument) error
    DeleteDocumentIndex(ctx context.Context, id uuid.UUID) error
    Search(ctx context.Context, req SearchRequest) (*SearchResult, error)
}
```

### 3. OCR Worker Pool Interface (`internal/archives/ocr_worker.go`)
```go
type OCRWorkerPool interface {
    Start()
    Stop()
    Enqueue(docID uuid.UUID, filePath string, mimeType string) error
    Status() WorkerPoolStatus
}
```

## Code Layout
```
sma-adp-api/
├── migrations/
│   └── 000023_archives_retention_search.up.sql
└── internal/
    └── archives/
        ├── doc.go                       # Package documentation
        ├── model.go                     # ArchiveDocument, RetentionPolicy, AuditLog structs
        ├── dto.go                       # Data Transfer Objects
        ├── repository.go                # Postgres repository implementation
        ├── meilisearch.go               # Meilisearch client wrapper & Postgres fallback search
        ├── ocr_worker.go                # Worker pool and async processing pipeline
        ├── ocr_parsers.go               # Extraction engines (PDF, PNG/JPG/TIFF, DOCX/XLSX, ZIP)
        ├── retention_engine.go          # Policy evaluator, auto-delete, legal hold checker
        ├── storage_tier.go              # Storage tier manager (HOT/WARM/COLD)
        ├── signed_url.go                # HMAC signer with TTL & IP binding
        ├── bulk.go                      # Bulk Zip builder & operations handler
        ├── gdpr.go                      # GDPR/PDPA data subject request processors
        ├── handler.go                   # HTTP REST Handlers
        ├── service.go                   # ArchiveService implementation & wiring
        └── lifecycle_integration_test.go# Lifecycle integration test suite
```
