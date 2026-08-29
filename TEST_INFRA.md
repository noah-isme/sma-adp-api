# Archives Module - Integration Test Infrastructure & Specification (`TEST_INFRA.md`)

## 1. Overview & Test Philosophy

This document outlines the testing architecture, methodology, and test tier mapping for the Archives backend module in `sma-adp-api`.

### Key Testing Principles:
1. **End-to-End Integrity**: No facade tests, dummy skips, or hardcoded passes. All test cases exercise genuine domain logic, async concurrency, crypto operations, search indexing, and retention policy evaluation.
2. **Self-Contained Isolation**: Every test creates its own independent test environment, models, and mock/in-memory state, ensuring zero side-effects across test runs.
3. **Progressive Testability**: Verification relies strictly on contracts and components defined in `PROJECT.md` and `ARCHIVES_RETENTION_SEARCH_GUIDE.md`.
4. **Deterministic Handling of Volatile Data**: Dynamic values such as generated UUIDs, cryptographic signatures, and timestamps are validated using structural assertions, regex patterns, and range checks rather than brittle equality matchers.

---

## 2. Test Execution Commands

To execute the Archives module integration test suite:

```bash
# Run all tests in the Archives module with verbose output
go test -v ./internal/archives/...

# Run specifically the E2E Archive Lifecycle integration test
go test -v -run TestArchiveLifecycle_Integration ./internal/archives/...

# Run tests with race detection and code coverage analysis
go test -v -race -cover ./internal/archives/...
```

---

## 3. 4-Tier Test Case Mapping

```
+-----------------------------------------------------------------------------------+
|                              4-TIER TEST ARCHITECTURE                             |
+-----------------------------------------------------------------------------------+
| Tier 1: Unit & Component Isolation                                                |
|   - Checksum calculation, MIME validation, HMAC signature calculation, DTO models |
+-----------------------------------------------------------------------------------+
| Tier 2: Subsystem & Repository Integration                                        |
|   - Repository SQL / In-memory CRUD, Meilisearch client wrapper, Retention Engine |
+-----------------------------------------------------------------------------------+
| Tier 3: Service & Pipeline E2E Integration (TestArchiveLifecycle_Integration)     |
|   - Upload -> Async OCR Pool -> Search Indexing -> Signed URL -> Legal Hold       |
|     -> Retention Evaluator Execution (Legal Hold Skip Verification)               |
+-----------------------------------------------------------------------------------+
| Tier 4: Security & Compliance Edge Cases                                          |
|   - IP binding mismatch on Signed URL, Expired TTL, GDPR Erasure legal hold block |
+-----------------------------------------------------------------------------------+
```

### Tier 1: Unit & Component Isolation Tests
Focus: Isolated domain rules, state machines, and mathematical cryptographic properties.
- **Upload Validation**: SHA-256 checksum generation, MIME type filtering (`application/pdf`, images), max file size limits (10MB).
- **OCR State Machine**: State transitions (`PENDING` -> `PROCESSING` -> `COMPLETED` / `FAILED`).
- **HMAC Signer Unit**: Token signing function, URL query parameter generation, TTL expiry evaluation.
- **DTO Mapping**: Domain model `ArchiveDocument` to `ArchiveResponseDTO` mapping verification.

### Tier 2: Subsystem & Repository Integration Tests
Focus: Data layer, repository queries, search index operations, and background worker queues.
- **Repository Operations**: Store document, retrieve by ID, list with filters (category, tags, legal_hold), soft delete, audit log creation.
- **Meilisearch Search Engine**: Indexing `ArchiveDocument` fields (`filename`, `ocr_text`, `category`), executing BM25 full-text queries, extracting highlighted snippets.
- **Async Worker Pool**: Queue channel enqueuing, worker goroutine pool processing, status updating, concurrent job handling.
- **Retention Evaluator Subsystem**: Scanning `retain_until` dates, identifying expired documents.

### Tier 3: Service & Pipeline E2E Integration (`TestArchiveLifecycle_Integration`)
Focus: Full lifecycle execution of document processing from upload through retention evaluation.
- **Step 1: Document Upload**: Upload document `transcript_stu_001.pdf` (category `STUDENT_RECORD`, tags `["transcript", "grade-10"]`). Verify `OCRStatus == PENDING`, `StorageTier == HOT`, `LegalHold == false`.
- **Step 2: Async OCR Processing**: Background worker receives job from pool, extracts full text (`"Official Academic Transcript for Student ID stu_001. Final Grade: A in Mathematics and Science."`), and updates status to `COMPLETED`. Test polls until status transitions.
- **Step 3: Meilisearch Indexing & Full-Text Search**: Document is automatically indexed upon OCR completion. Execute search for `"Mathematics"`. Verify document ID matches and snippet highlights term. Execute search for invalid term `"NonExistentTerm12345"` to verify zero false positives.
- **Step 4: Signed URL Download Token Generation & Validation**: Generate signed download URL token with 30m TTL and client IP binding (`192.168.1.50`). Validate token with matching IP (succeeds). Validate token with mismatched IP `10.0.0.1` (fails with `ErrIPMismatch`). Validate token with tampered signature (fails).
- **Step 5: Legal Hold Mechanics**: Apply legal hold (`legal_hold = true`, reason: `"Pending litigation inquiry"`). Verify document reflects legal hold status and audit log record `LEGAL_HOLD` is appended.
- **Step 6: Retention Engine Execution & Legal Hold Protection**: Set `retain_until` to 1 year in the past. Trigger `RetentionEngine.EvaluatePolicies()`. Verify document under legal hold is **NOT** deleted (`deleted_at` remains nil) and audit log `SKIPPED_LEGAL_HOLD` is generated. Release legal hold, re-evaluate retention policies, and verify document is auto-deleted with audit log `RETENTION_EXPIRED`.

### Tier 4: Security & Compliance Edge Case Verification
Focus: Security boundaries, access restrictions, and legal compliance constraints.
- **IP Binding Validation**: Strict verification that signed download URLs reject requests from unauthorized client IPs.
- **Signature Security**: HMAC verification rejects altered payload attributes or tampered secret keys.
- **GDPR / PDPA Legal Hold Supremacy**: Ensures erasure requests (GDPR Right to be Forgotten) are rejected when document has an active legal hold (`ErrLegalHoldActive`).
- **Non-Deterministic Fields**: Verifies timestamps (`uploaded_at`, `updated_at`), dynamic UUIDs, and HMAC tokens without brittle exact string matching.

---

## 4. Requirement Traceability Matrix

| Requirement | Description | Test Tier | Test Case / Method |
|---|---|---|---|
| R1.1 | Async OCR Worker Processing (PENDING -> COMPLETED) | Tier 3 | `TestArchiveLifecycle_Integration` (Step 2) |
| R1.2 | Meilisearch Indexing & Full-Text Search | Tier 2 / 3 | `TestArchiveLifecycle_Integration` (Step 3) |
| R2.1 | Retention Policy Expiry Evaluation | Tier 2 / 3 | `TestArchiveLifecycle_Integration` (Step 6) |
| R2.2 | Legal Hold Enforcement (Skip Deletion) | Tier 3 | `TestArchiveLifecycle_Integration` (Step 6) |
| R2.3 | Audit Log Generation (`SKIPPED_LEGAL_HOLD`) | Tier 3 | `TestArchiveLifecycle_Integration` (Step 6) |
| R3.1 | Enriched Signed URL Generation & Validation | Tier 1 / 3 | `TestArchiveLifecycle_Integration` (Step 4) |
| R3.2 | Client IP Binding Verification | Tier 4 | `TestArchiveLifecycle_Integration` (Step 4) |
| R3.3 | GDPR Erasure Legal Hold Protection | Tier 4 | `TestArchiveLifecycle_Integration` (Step 5, 6) |
