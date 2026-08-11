# Consolidated Frontend ↔ Backend Gap Analysis Report

**Generated:** 2026-08-06 (Consolidated: 2026-08-12)  
**Original Analysis Dates:** 2026-08-06 (Aug 6 Report) and 2026-08-09 (Aug 9 Report)  
**Scope:** `admin-panel-sma` (React/Refine) frontend vs `sma-adp-api` (Go) backend contract and implementation reconciliation.

---

## Executive Summary

This consolidated report merges findings from the August 6, 2026 gap analysis and the August 9, 2026 documentation gap review. All primary identified integration and contract gaps (G-01 through G-09) along with G-10 (Audit log persistence) and G-11 (Stale gap report supersedence) have been resolved across the monorepo.

| Item ID | Category / Description | Status | Resolution Details |
|---|---|---|---|
| **G-01** | Auth Contract Alignment (Refresh & Logout) | [x] Resolved | Aligned request/response payload contracts: admin client (`apps/admin/src/main.tsx`, `authProvider.ts`, `dataProvider.ts`) now sends `refresh_token` in body for refresh and logout, unwraps API response envelope, handles snake_case (`access_token`/`refresh_token`) keys, and Go backend (`auth_handler.go`) revokes refresh tokens in DB upon logout. |
| **G-02** | Feature Flag Drift & Discovery | [x] Resolved | Runtime `/api/v1/features` endpoint made authoritative for frontend resource selection (`selectResources()` in `main.tsx`) with per-feature `VITE_ENABLE_*` build-time fallback precedence and explicit false overrides documented in README. |
| **G-03** | Student & Teacher Roster Filter Alignment | [x] Resolved | Added `status`, `sortField`, `sortOrder` query parameter aliases in `student_handler.go` and `teacher_handler.go`, and implemented documented roster filters (gender, track, subject, homeroom, availability). |
| **G-04** | Grade Report Filter & Aggregation Completeness | [x] Resolved | Enriched query joins in `grade_handler.go` & `grade_repository.go`, implemented status (PASS/REMEDIAL/FAIL), scoreMin/scoreMax, search, teacher filters, frontend sort aliases, shared grade thresholds, and database-backed `COUNT(*)` totals for pagination. |
| **G-05** | Grade CSV Status Filter | [x] Resolved | Replaced `export_compatibility_handler.go` status filter `AND 1=1` no-op with shared PASS/REMEDIAL/FAIL predicate logic matching the grade report. |
| **G-06** | Security Claims Reconciliation | [x] Resolved | Removed unsupported rate-limit, lockout, and Argon2 claims from application docs; documented bcrypt hashing and external WAF/ingress gateway responsibilities for rate limiting. |
| **G-07** | Stale API Versioning Documentation | [x] Resolved | Replaced outdated NestJS-to-Go migration/v2 references in `API_VERSIONING_GUIDE.md` with current dated v1/Go contract guides. |
| **G-08** | Monorepo Checklist Reconciliation | [x] Resolved | Archived outdated implementation checklist (`checklist.md`); replaced with current cross-repository status linked to `PROJECT_STATUS.md`. |
| **G-09** | Swagger Annotation & Public Route Drift | [x] Resolved | Corrected public route security annotations in Swagger, cleaned up duplicate swagger route annotations in `teacher_handler.go`, updated `@Param` annotations, regenerated Swagger, and validated 135 paths / 22 compatibility routes via `make validate-swagger-routes`. |
| **G-10** | Audit Log Persistence | [x] Resolved | Created database migration (`000002_refresh_tokens_and_audit_logs.up.sql`), Go models (`models.AuditLog`), database repository (`UserRepository.CreateAuditLog`), and updated audit middleware to persist log entries to PostgreSQL DB via parameterized SQL `INSERT INTO audit_logs...`. |
| **G-11** | Stale Gap Report Supersedence & Consolidation | [x] Resolved | Merged findings from Aug 6 and Aug 9 reports into consolidated `/home/noah/project/sma/GAP_ANALYSIS_REPORT.md` at root, marking G-01 through G-10 complete with resolution details, preserving secondary backlog items, and archiving the full Aug 9 report content in a dedicated section at the end. |

---

## Detailed Gap Matrix & Resolution Summary

### 1. Authentication & Session Lifecycle (G-01)
- **Status:** [x] Resolved
- **Details:** Frontend auth provider (`apps/admin/src/providers/authProvider.ts`), data provider (`dataProvider.ts`), and initialization (`apps/admin/src/main.tsx`) updated to send `refresh_token` JSON body on token refresh and logout (`POST /api/v1/auth/logout`). The backend handler `auth_handler.go` revokes tokens in PostgreSQL DB upon logout. Response envelopes (`{ data: { access_token, refresh_token } }`) are properly unwrapped, with support for both camelCase and snake_case token keys.

### 2. Feature Gating & Discovery (G-02)
- **Status:** [x] Resolved
- **Details:** Frontend resource initialization (`selectResources()` in `main.tsx`) prioritizes runtime feature discovery (`GET /api/v1/features`) as source of truth. Build-time `VITE_ENABLE_*` flags function as local fallback overrides. Audited README documentation to match runtime flag behavior.

### 3. Student & Teacher Roster Queries (G-03)
- **Status:** [x] Resolved
- **Details:** `student_handler.go` and `teacher_handler.go` support query parameter aliases (`status`, `sortField`, `sortOrder`). Filters for gender, track, homeroom, subject, active status, and teacher availability are fully wired into repository queries.

### 4. Grade Reporting & Filtering (G-04, G-05)
- **Status:** [x] Resolved
- **Details:** `GradeHandler.Report`, `GradeRepository`, and `ExportCompatibilityHandler` support full filter predicates (termId, classId, teacherId, status [PASS/REMEDIAL/FAIL], scoreMin, scoreMax, search, sort). Pagination returns exact database count. Replaced grade CSV export `AND 1=1` no-op with shared status calculation predicates.

### 5. Documentation & Swagger Alignment (G-06, G-07, G-08, G-09)
- **Status:** [x] Resolved
- **Details:** All unauthenticated routes (`/health`, `/ready`, `/features`, `/auth/login`, `/auth/refresh`) properly annotated in Swagger without `@Security BearerAuth`. Cleaned up duplicate route annotations in `teacher_handler.go`. Swagger validation script confirms 135 paths and 22 compatibility operations. Outdated NestJS/v2 migration guides and checklists archived and updated with links to `PROJECT_STATUS.md`.

### 6. Audit Logging (G-10)
- **Status:** [x] Resolved
- **Details:** Database migration `000002_refresh_tokens_and_audit_logs.up.sql` creates `audit_logs` schema with `id`, `user_id`, `action`, `resource`, `resource_id`, `old_values`, `new_values`, `ip_address`, `user_agent`, `created_at`. `UserRepository.CreateAuditLog` and `middleware.Audit` persist all request audit records to PostgreSQL using parameterized SQL queries.

### 7. Gap Analysis Consolidation (G-11)
- **Status:** [x] Resolved
- **Details:** Root `GAP_ANALYSIS_REPORT.md` updated to consolidate all August 6 and August 9 gap analysis findings, marking G-01 through G-10 resolved, retaining open secondary backlog items, and incorporating the complete text of `SMA_ADMIN_API_DOCUMENTATION_GAP_REPORT.md` in a dedicated archived section.

---

## Retained Open / Secondary Backlog Items

The following secondary non-blocking items remain tracked for future releases:

1. **Portal Workflows Secondary Features:**
   - Email delivery integration for portal password reset flows (`service/portal_auth_service.go`).
   - Parent student data access fine-grained permission scope enhancements.
   - Portal announcement pagination parameters parsing.

2. **Operations & Production Infrastructure:**
   - Canary deployment automated metric thresholds in production environment WAF.
   - Production external rate-limiting rule definitions at Nginx/Cloudflare ingress tier.

---

## Archived Report: SMA Admin API Documentation Gap Report (2026-08-09)

# SMA Admin Panel ↔ SMA ADP API

## Documentation and Feature Gap Report

**Review date:** 2026-08-09  
**Scope:** `admin-panel-sma` and `sma-adp-api`  
**Review type:** Cross-repository documentation, contract, and implementation comparison

## Executive summary

The two repositories have a useful documentation base, and the API route inventory is
healthy: the current static checks validate 135 Swagger paths and 22 compatibility
operations. However, route presence is masking several semantic gaps between the admin
client, the published API contract, and the Go implementation.

The highest-risk issue is the authentication session lifecycle. The admin refresh path
sends no JSON body and does not unwrap the API envelope, while the Go API requires
`{"refresh_token":"..."}`. Logout sends `refreshToken`, while the API binds
`refresh_token`. This is inconsistent with both the compatibility matrix's “ready” claim
and the existing gap report's “no auth gap” conclusion.

Other material gaps are:

- individual frontend feature flags are documented as working but the build-time fallback
  currently reads only `VITE_ENABLE_ALL_FEATURES`;
- roster filters and sorting use different query names on the two sides, and roster
  summary/metadata fields are still placeholders;
- the grade report documentation claims full filter support while status filtering,
  several sort names, joins, and total pagination are incomplete;
- the grade CSV `status` filter is explicitly a no-op;
- rate limiting, login lockout, and Argon2 controls are documented but not present in the
  current Go auth configuration/code; and
- versioning, checklist, and generated gap documents contain older migration assumptions
  that are not clearly marked historical.

## Review method and evidence standard

Graphify was used first to scope the cross-repository graph around documentation,
frontend providers/pages, feature flags, API handlers, and existing gap reports. The
focused source review then compared:

- admin client request construction and feature discovery;
- API gateway routes, handlers, repositories, models, and response envelopes;
- the canonical Go API specification, generated Swagger, compatibility matrix, and
  project status; and
- admin-panel setup, versioning, security, checklist, and runbook documentation.

Static verification performed during this review:

```text
python3 scripts/validate_swagger_routes.py
  Validated 135 Swagger paths against API gateway routes.

python3 scripts/compatibility_smoke.py
  Validated 22 compatibility operations in gateway and Swagger.
  Seeded compatibility smoke skipped (RUN_COMPATIBILITY_SMOKE was not set).
```

The findings below therefore distinguish static contract evidence from runtime behavior.
No production or seeded runtime test is claimed for this review.

## Resolution status — 2026-08-09

The implementation and documentation work from this report is now applied as follows:

| Finding | Resolution |
| --- | --- |
| G-01 auth refresh/logout | Fixed the snake_case request bodies and envelope unwrapping in the admin client; added frontend contract tests, Go logout-revocation coverage, and protected Swagger annotations. Seeded login → refresh → logout → refresh rejection is still pending. |
| G-02 feature flags | Fixed per-feature Vite fallback precedence and explicit false overrides; added tests and updated the admin README. |
| G-03 rosters | Added status/sort aliases and applied the documented roster filters. Aggregate catalogs, distributions, teacher assignment counts, and some teacher metadata remain partial by design and are marked that way in the matrix/spec. |
| G-04 grade report | Added enriched joins, status/score/search/teacher filters, frontend sort aliases, shared thresholds, and database-backed totals. Filter catalogs and score distribution remain a follow-up. |
| G-05 grade CSV | Replaced the status no-op with PASS/REMEDIAL/FAIL predicates shared with the grade report. |
| G-06 security claims | Removed unsupported rate-limit, lockout, and Argon2 claims; documented bcrypt and the external gateway/WAF ownership requirement. The controls themselves remain a production gate. |
| G-07/G-08/G-11 stale documents | Replaced the admin versioning guide/checklist with current dated guides and marked the older backend gap report historical. |
| G-09 canonical docs/Swagger | Corrected public-route/auth/rate-limit documentation, regenerated Swagger, and refreshed validation counts to 135 paths / 22 compatibility operations. |
| G-10 readiness | Fixed the audit JSON write path and added a regression test. Rollback, alerting, seeded end-to-end verification, and CI/full-suite gates remain open. |
| Portal secondary gaps | Marked portal reset/email delivery, parent data scoping, and announcement pagination as partial/not ready in the portal specification. |

The current release position is therefore **contract-reconciled but not production-ready** until the remaining runtime, security-owner, rollback, and portal gates are evidenced.

## Prioritized findings

### G-01 — Authentication refresh and logout contracts disagree — P0

**Classification:** Confirmed implementation/integration gap.

**Evidence:**

- The admin refresh helper posts to `/auth/refresh` with only an `Authorization` header
  and no JSON body, then reads token fields from the top-level response:
  `admin-panel-sma/apps/admin/src/providers/dataProvider.ts:91-119`.
- The Go handler calls `ShouldBindJSON` and requires `refresh_token`:
  `sma-adp-api/internal/handler/auth_handler.go:65-80` and
  `sma-adp-api/internal/models/auth.go:26-30`.
- Go success responses are wrapped in `data` by the common response envelope:
  `sma-adp-api/pkg/response/response.go:12-26`.
- Admin logout sends `{refreshToken}`:
  `admin-panel-sma/apps/admin/src/providers/authProvider.ts:275-289`.
  Go logout binds `{refresh_token}`:
  `sma-adp-api/internal/handler/auth_handler.go:101-115`.
- The compatibility matrix labels refresh “Ready” and “single refresh mechanism”:
  `sma-adp-api/docs/COMPATIBILITY_CONTRACT_MATRIX.md:48`.

**Impact:** A 401 recovery request can fail validation before token rotation, and logout
can clear browser storage without revoking the server-side refresh token. Static route
checks will not detect either defect.

**Required action:** Define one shared token contract. Send the required JSON body, unwrap
the envelope in the refresh path, use `refresh_token` for logout, and add an integration
test covering login → refresh → logout → refresh rejection. Update the matrix only after
that test passes.

### G-02 — Feature-flag documentation promises controls the client does not implement — P1

**Classification:** Documentation/implementation drift.

**Evidence:**

- Runtime discovery is intentionally the primary frontend mechanism:
  `admin-panel-sma/apps/admin/src/providers/features.ts:1-13`.
- The build-time fallback ignores its `feature` argument and reads only
  `VITE_ENABLE_ALL_FEATURES`:
  `admin-panel-sma/apps/admin/src/providers/features.ts:46-57`.
- The admin README documents individual `VITE_ENABLE_*` flags and says they can override
  all-on mode:
  `admin-panel-sma/README.md:138-161`.
- The API implements unauthenticated `/features` discovery, which is the reliable path
  when the frontend can reach the API:
  `sma-adp-api/cmd/api-gateway/main.go:88-94`.

**Impact:** An offline/MSW/build-time deployment that sets an individual Vite flag can
render fewer or different modules than the README promises. The runtime path can work
while the documented fallback remains misleading.

**Required action:** Choose one contract and test it. The smaller change is to document
individual Vite flags as unsupported/deprecated and make `/features` authoritative. If
individual fallback flags are required, implement the per-feature mapping and add one
test per flag plus an explicit false-override test.

### G-03 — Student and teacher roster query contracts are not aligned — P1

**Classification:** Confirmed implementation/integration gap; partially documented.

**Evidence:**

- Student and teacher pages send `status`, `sortField`, and `sortOrder`, alongside the
  rich roster filters:
  `admin-panel-sma/apps/admin/src/pages/students.tsx:140-158` and
  `admin-panel-sma/apps/admin/src/pages/teachers.tsx:116-133`.
- The Go student handler reads `active`, `sort`, and `order`, not `status`,
  `sortField`, or `sortOrder`:
  `sma-adp-api/internal/handler/student_handler.go:144-185`.
- The Go teacher handler has the same sort-name mismatch and does not bind the page's
  `status` filter:
  `sma-adp-api/internal/handler/teacher_handler.go:146-175`.
- Roster response summaries and filter metadata are still empty or compatibility
  placeholders, including teacher tracks, assignment counts, availability, and
  distribution arrays:
  `sma-adp-api/internal/handler/student_handler.go:139-141` and
  `sma-adp-api/internal/handler/teacher_handler.go:141-143`.
- The compatibility matrix correctly calls these endpoints partial:
  `sma-adp-api/docs/COMPATIBILITY_CONTRACT_MATRIX.md:26-27`.
  The canonical API specification contradicts that by saying the roster routes have
  “full filter support”:
  `sma-adp-api/docs/GO_BACKEND_API_SPECIFICATION.md:328`.

**Impact:** User-selected status and sorting can be silently ignored, and UI summary
cards/filter selectors cannot be trusted. The API may return 2xx while producing a
different result from the selected admin filters.

**Required action:** Publish one typed query contract, either support both camelCase
  aliases (`status`, `sortField`, `sortOrder`) or change the frontend to the canonical
  names, then implement the missing relationships and metadata. Add seeded assertions
  that each UI filter changes the result set and that the returned summary matches it.

### G-04 — Grade report is advertised as complete but remains a compatibility view — P1

**Classification:** Confirmed implementation gap and specification drift.

**Evidence:**

- The API specification says `/grades/report` supports all frontend filters, including
  status, score ranges, search, and sorting:
  `sma-adp-api/docs/GO_BACKEND_API_SPECIFICATION.md:855-875`.
- The handler parses those fields, but the repository does not apply `filter.Status`;
  its allowed sort keys are backend names such as `grade_value` and `student_name`,
  not the documented frontend names such as `score` and `studentName`:
  `sma-adp-api/internal/repository/grade_repository.go:26-107`.
- The response populates important display fields with IDs, empty strings, or fixed
  values (`studentName`, `className`, `teacherName`, `termName`, `componentWeight`, and
  `kkm`): `sma-adp-api/internal/handler/grade_handler.go:182-205`.
- Summary filter arrays and distributions are empty, and pagination reports the number
  of rows in the current page with `totalPages: 1` instead of a database count:
  `sma-adp-api/internal/handler/grade_handler.go:223-264`.
- The compatibility matrix gives the more accurate “Partial / Not ready” status:
  `sma-adp-api/docs/COMPATIBILITY_CONTRACT_MATRIX.md:28`.

**Impact:** Filters can appear in `appliedFilters` without changing the data, sorting
can fall back to `updated_at`, and pagination/grade context can be wrong. This is a
functional reporting gap, not only a documentation issue.

**Required action:** Either narrow the published contract to the currently supported
subset or complete the query joins, status predicate, frontend sort aliases, count
query, metadata, and real display fields. Update the API spec and matrix from the same
seeded contract test.

### G-05 — Grade CSV status filtering is a documented no-op — P1

**Classification:** Confirmed missing feature and contradictory documentation.

**Evidence:**

- The API specification advertises filter support for browser CSV exports, including
  grade `status`: `sma-adp-api/docs/GO_BACKEND_API_SPECIFICATION.md:329-331` and
  `1489-1501`.
- The grade export handler accepts `status` but appends `AND 1=1` with an explicit
  placeholder comment:
  `sma-adp-api/internal/handler/export_compatibility_handler.go:55-91`.
- The compatibility matrix instead says browser exports are unfiltered and not ready:
  `sma-adp-api/docs/COMPATIBILITY_CONTRACT_MATRIX.md:34`.
- The existing gap report also describes all browser CSV exports as unfiltered:
  `sma-adp-api/GAP_ANALYSIS_REPORT.md:153-167`, although the current student/attendance
  handler code supports some basic filters.

**Impact:** A user requesting a filtered grade export receives all grades, which can
produce an incorrect operational report while returning a successful download.

**Required action:** Implement status using the same KKM/status calculation as the grade
report, or remove `status` from Swagger/spec examples. Decide whether the admin should
use the compatibility CSV routes or the async report job, then document that choice and
test the filtered output.

### G-06 — Security controls are claimed but not evidenced in the Go service — P1

**Classification:** Security implementation/documentation gap.

**Evidence:**

- The admin README claims strict rate limiting, login lockout, and configurable Argon2,
  in addition to password policy:
  `admin-panel-sma/README.md:181-188`.
- The API specification claims anonymous/authenticated/admin request quotas:
  `sma-adp-api/docs/GO_BACKEND_API_SPECIFICATION.md:1937-1941`.
- Current Go auth configuration wires token secrets and expiries but no lockout or rate
  limiter settings: `sma-adp-api/cmd/api-gateway/main.go:98-104`.
- The auth service uses bcrypt (`sma-adp-api/internal/service/auth_service.go:17,
  69-89, 282-288`), and the current API `.env.example`/config does not define the
  documented `AUTH_MAX_LOGIN_ATTEMPTS`, `AUTH_LOCKOUT_DURATION`, or `ARGON2_*` controls.
  A repository-wide static search found no application rate-limiter or lockout
  implementation.

**Impact:** Operators cannot tell whether these controls are supplied by the application,
an ingress layer, or not at all. The documentation may create false production-security
confidence.

**Required action:** Decide the ownership of each control. Implement and test the missing
controls with explicit configuration, or remove the claims and document the external
gateway dependency. Do not describe Argon2 as configurable while the service hashes with
bcrypt unless the migration is intentional and tracked.

### G-07 — API versioning guide is a stale migration plan, not an executable current guide — P2

**Classification:** Documentation drift.

**Evidence:**

- The guide describes NestJS-to-Go migration, a v2 rollout, shadow traffic, deprecation
  headers, and a 2025 sunset timeline:
  `admin-panel-sma/docs/API_VERSIONING_GUIDE.md:1-3` and `:46-70`.
- Its quick reference still instructs operators to use `/api/v2`, `Accept-Version`,
  `ENABLE_V2_*`, `make openapi-v1 openapi-v2`, `tools/shadow_diff.py`, and a
  `/api/deprecation-status` endpoint:
  `admin-panel-sma/docs/API_VERSIONING_GUIDE.md:583-625`.
- The current API default is `/api/v1`:
  `sma-adp-api/pkg/config/config.go:282` and `sma-adp-api/.env.example:4`.
  The current gateway has no v2 route, v2 flag, or version-header implementation in the
  two repositories reviewed.
- The guide labels v2 “Planned” and says it was last updated 2025-01-15:
  `admin-panel-sma/docs/API_VERSIONING_GUIDE.md:629-643`.

**Impact:** Engineers can follow commands and endpoints that do not exist, or assume a
cutover/deprecation state that the current project status explicitly says is incomplete.

**Required action:** Replace this with a current v1/Go contract guide, or clearly move it
to an archived historical migration folder. Remove live-looking example URLs and
commands unless each has a repository owner and executable verification.

### G-08 — Admin implementation checklist is undated and conflicts with current status — P2

**Classification:** Documentation maintenance gap.

**Evidence:**

- The checklist starts with an unfinished monorepo/API initialization and marks Swagger,
  health checks, audit logging, and several implemented domains as pending:
  `admin-panel-sma/docs/checklist.md:1-45`.
- The current API status document identifies the Go gateway, generated Swagger, the
  compatibility matrix, and operations/decommission documents as the sources of truth:
  `sma-adp-api/docs/PROJECT_STATUS.md:1-16`.
- The checklist has no date, historical banner, owner, or link explaining that its
  unchecked items are intentionally retained.

**Impact:** A new contributor cannot distinguish backlog from completed migration work.
This can lead to duplicate implementation or incorrect release status.

**Required action:** Archive the old checklist with a historical label, or regenerate it
as a current cross-repository checklist linked to `PROJECT_STATUS.md`. Every item should
have an owner, status date, and evidence link.

### G-09 — Canonical API documentation has authorization and verification drift — P2

**Classification:** Documentation and release-process gap.

**Evidence:**

- The API specification says all endpoints except `/auth/login` require JWT:
  `sma-adp-api/docs/GO_BACKEND_API_SPECIFICATION.md:1927-1933`.
- The gateway intentionally exposes unauthenticated `/health`, `/ready`, `/features`,
  auth refresh/password routes, and public portal auth routes:
  `sma-adp-api/cmd/api-gateway/main.go:72-94` and `:129-145`.
- The same specification publishes rate limits that have no corresponding application
  implementation/configuration in the current codebase:
  `sma-adp-api/docs/GO_BACKEND_API_SPECIFICATION.md:1937-1941`.
- `PROJECT_STATUS.md` already records missing required query parameters in Swagger
  annotations for report, dashboard, and attendance routes:
  `sma-adp-api/docs/PROJECT_STATUS.md:302-304`.
- That status document says the latest validation covered 101 generated paths, while the
  current validator run in this review validated 135 paths:
  `sma-adp-api/docs/PROJECT_STATUS.md:18-23`.

**Impact:** Consumers cannot reliably infer authentication requirements or the scope of
the last verification from the canonical docs. Missing Swagger parameters also weaken
client generation and contract review.

**Required action:** Make generated Swagger plus the compatibility matrix authoritative;
correct the public-route security annotations, remove unsupported quota claims or assign
them to an external gateway, add the missing `@Param` annotations, regenerate Swagger,
and refresh verification counts/dates.

### G-10 — Production readiness gaps are documented but still unresolved — P1/P2

**Classification:** Operational feature and verification gap.

**Evidence:**

- The latest status records a non-fatal audit-log persistence warning on login and says
  no current Go test/vet pass is claimed:
  `sma-adp-api/docs/PROJECT_STATUS.md:42-48`.
- The same document lists production alert rules, a rollback drill, a CI contract-test
  gate, and handler coverage as remaining work:
  `sma-adp-api/docs/PROJECT_STATUS.md:270-289`.
- Production cutover remains blocked pending the rollback drill and the legacy shadow
  compare is skipped:
  `sma-adp-api/docs/PROJECT_STATUS.md:314-318`.

**Impact:** The documentation is candid, but admin-panel runbooks and readiness language
must not imply production readiness while audit persistence, rollback evidence, alerts,
and repeatable CI contract tests remain open.

**Required action:** Promote these items into a release gate with named owners and dates.
Fix the audit JSON encoding warning, add the contract test workflow, execute and record a
real rollback drill, and add alert rules before declaring cutover complete.

### G-11 — The existing gap report is already stale relative to current source — P2

**Classification:** Report lifecycle/documentation gap.

**Evidence:**

- `sma-adp-api/GAP_ANALYSIS_REPORT.md` is marked generated 2026-08-06 and says auth has
  no gap, browser exports are unfiltered, and announcements/behavior have no frontend
  pages: `:3-5`, `:29-40`, `:153-167`, and `:182-189`.
- The current admin tree contains announcements and behavior-note pages, and the current
  export code supports basic student/attendance filters while grade status remains a
  placeholder.
- The report records the old mutation endpoint mismatch as struck-through/fixed, but
  leaves the historical mismatch in the main detailed section:
  `sma-adp-api/GAP_ANALYSIS_REPORT.md:193-206`.

**Impact:** The report is useful historical context but unsafe as the current planning
source. It can direct work toward already-fixed gaps and miss the auth/semantic issues
identified here.

**Required action:** Mark it as a dated snapshot, link this report and
`docs/PROJECT_STATUS.md` as the current sources, and regenerate gap findings after each
contract or frontend-provider change.

## Secondary API-only gap: portal workflows

This area is not exercised by the current admin-panel pages, but it is documented in the
ADP API and therefore should be tracked separately:

- The portal API documents forgot-password and reset-password endpoints, while the
  service still has TODOs for email delivery and token validation/password reset:
  `sma-adp-api/docs/PORTAL_API_SPEC.md:85-105` and
  `sma-adp-api/internal/service/portal_auth_service.go:283-297`.
- Parent student data access has a TODO permission check, and portal announcement
  pagination parameters are not parsed:
  `sma-adp-api/internal/handler/portal_data_handler.go:68-72` and `:280-290`.

These should be labeled partial/not ready in the portal contract rather than presented as
implemented endpoints.

## Recommended order of work

1. **Immediately:** fix refresh/logout payload and envelope handling; add an end-to-end
   auth contract test.
2. **Before broader admin rollout:** reconcile feature-flag fallback behavior, roster
   query names, grade-report status/sort/count behavior, and grade export status filtering.
3. **Before production sign-off:** resolve or explicitly externalize rate limiting,
   login lockout, and password-hashing claims; fix the audit persistence warning; complete
   rollback and alert verification.
4. **Documentation cleanup:** make Swagger annotations and generated artifacts current;
   reconcile the API specification with the matrix; archive the old versioning guide and
   checklist; mark `GAP_ANALYSIS_REPORT.md` as a historical snapshot.
5. **Portal backlog:** implement reset/password and data-scoping behavior or mark those
   routes not ready in `PORTAL_API_SPEC.md`.

## Bottom line

The repositories have strong route-level documentation and a useful compatibility
workflow, but the current documents overstate readiness at the semantic-contract level.
The auth lifecycle, roster/grade filter behavior, security controls, and document
authority chain should be treated as the primary gaps. The static route checks should
remain in CI, but they need payload/response behavior tests and a single dated source of
truth to prevent these gaps from recurring.
