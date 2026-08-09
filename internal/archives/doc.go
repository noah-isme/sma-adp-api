// Package archives implements the core backend infrastructure for document archival,
// full-text search indexing, asynchronous OCR text extraction, retention policy evaluation,
// legal hold management, HMAC-SHA256 signed download URLs with client IP binding,
// and GDPR/PDPA data subject compliance request processors.
//
// Architecture Overview:
//
// 1. Core Data Models (model.go):
//   - ArchiveDocument: Represents an archived document with metadata, OCR status, storage tier,
//     retention policies, legal hold flags, and upload audit metadata.
//   - RetentionPolicy: Rules dictating document retention periods, auto-deletion settings, and
//     legal hold override permissions per category.
//   - AuditLog: Immutable structured audit trail for document lifecycle events (UPLOAD, DOWNLOAD,
//     SEARCH, RETENTION_CHANGE, LEGAL_HOLD, DELETE).
//
// 2. Repositories (repository.go):
//   - Database access layer for ArchiveDocument, RetentionPolicy, and AuditLog CRUD operations,
//     filtered queries, retention scans, and storage tier migration queries.
//
// 3. Search Engine (meilisearch.go):
//   - Provides full-text indexing and BM25 search query execution using Meilisearch, with
//     a fallback PostgreSQL ILIKE search implementation. Returns highlighted search snippets.
//
// 4. Async OCR Pipeline (ocr_worker.go, ocr_parsers.go):
//   - In-memory Go channel worker pool managing asynchronous text extraction for uploaded files
//     (PDF, PNG/JPG/TIFF, DOCX/XLSX, ZIP). Transitions document status from PENDING to COMPLETED,
//     updates DB records, and triggers search indexing.
//
// 5. Retention Engine (retention_engine.go):
//   - Automated background evaluator scanning documents past retain_until dates. Auto-deletes expired
//     documents unless blocked by active Legal Hold. Generates SKIPPED_LEGAL_HOLD audit logs.
//
// 6. Security & Signed URLs (signed_url.go):
//   - Generates and verifies HMAC-SHA256 signed download tokens bound to specific TTLs and requester IP addresses.
//
// 7. Data Compliance & GDPR (gdpr.go):
//   - Processes Data Subject Requests: ACCESS (Zip export), RECTIFICATION (metadata update),
//     ERASURE (deletion with legal hold safety checks), PORTABILITY (JSON export).
//
// Testing:
// Integration testing for package archives is implemented in lifecycle_integration_test.go,
// covering the end-to-end document lifecycle under TestArchiveLifecycle_Integration.
package archives
