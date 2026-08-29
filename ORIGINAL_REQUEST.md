# Original User Request

## 2026-08-09T04:40:05Z

Implement the missing backend infrastructure for the Archives module in the Go API according to `ARCHIVES_RETENTION_SEARCH_GUIDE.md`. This includes Meilisearch integration, an async OCR pipeline using an in-memory Go worker pool, a Retention Engine, S3 Storage Tiering, Signed URLs, and GDPR compliance logic.

Working directory: /home/noah/project/sma/sma-adp-api
Integrity mode: benchmark

## Requirements

### R1. Search & OCR Pipeline
Implement full-text search using Meilisearch. Build an async OCR pipeline using an in-memory Go worker pool that extracts text and updates the document search index.

### R2. Retention & Storage Engine
Build a retention engine that evaluates policies, auto-deletes expired documents, handles legal holds, and simulates storage tier migrations (HOT/WARM/COLD).

### R3. Security & Compliance
Implement secure Signed URLs with TTL/IP binding, bulk API endpoints, and GDPR/PDPA data subject request handlers (Access, Rectification, Erasure, Portability).

## Acceptance Criteria

### Verification
- [ ] Integration test `TestArchiveLifecycle_Integration` is implemented and passes, validating upload, OCR processing, search, signed URLs, and legal hold.
- [ ] `go test ./internal/archives/...` or equivalent tests run successfully with no errors.
- [ ] The Go codebase compiles successfully without errors (`go build ./cmd/api-gateway`).
