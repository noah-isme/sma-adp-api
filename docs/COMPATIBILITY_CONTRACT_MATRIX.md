# Frontend ↔ Go API Compatibility Contract Matrix

This matrix covers compatibility routes and recently reconciled frontend contracts;
it is not an exhaustive inventory of every core resource CRUD operation. The API
specification and generated Swagger remain authoritative for the complete API
surface. This matrix records integration and readiness state for compatibility
work. Update it when a caller, route, handler, flag, or smoke result changes.

Status vocabulary:

- **Backend implementation:** `Implemented`, `Partial`, `Feature-flagged`, or `Missing`.
- **Frontend integration:** `Integrated`, `Browser-only`, `Mock-only`, or `Not wired`.
- **Production readiness:** `Ready`, `Flagged/pending smoke`, or `Not ready`.
- Smoke status is deliberately dated. `Static contract smoke passed` means the
  gateway operation and generated Swagger method/path are both present;
  `seeded runtime blocked` means a live seeded request could not run in this
  environment (database/module-cache prerequisites were unavailable).

The static and optional seeded checks are maintained by
`scripts/compatibility_smoke.py`; CI always runs the static check, while a
deployment environment can set `RUN_COMPATIBILITY_SMOKE=1`, `BASE_URL`, and
`ACCESS_TOKEN` for read-only seeded requests.

| Frontend caller | HTTP method/path | Go handler | Feature flag | Payload casing | Response | Backend implementation | Frontend integration | Production readiness | Smoke-test status/date |
| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |
| Student roster page | `GET /students/roster` | `StudentHandler.Roster` | None | Query accepts camelCase aliases; API body preserves admin row names | JSON `{data: {summary, filters, rows, pagination}}` | Partial: search, class, active, pagination, and sorting are supported; rich demographic/guardian/track filters are not | Integrated | Not ready: aggregate filters are compatibility-level | Static contract smoke passed — 2026-08-02; seeded runtime blocked by DB/module cache |
| Teacher roster page | `GET /teachers/roster` | `TeacherHandler.Roster` | None | Query accepts camelCase aliases; API body preserves admin row names | JSON `{data: {summary, filters, rows, pagination}}` | Partial: search, active, pagination, and sorting are supported; subject/track/availability/homeroom filters are not | Integrated | Not ready: aggregate filters are compatibility-level | Static contract smoke passed — 2026-08-02; seeded runtime blocked by DB/module cache |
| Grade report page | `GET /grades/report` | `GradeHandler.Report` | None | Query camelCase; canonical write payloads are snake_case | JSON `{data: {context, summary, filters, rows, pagination}}` | Partial: subject/component filters and average are implemented; rich joins/metadata are not | Integrated | Not ready: term/class/teacher/status/score filters and seeded-data validation pending | Static contract smoke passed — 2026-08-02; seeded runtime blocked by DB/module cache |
| Generic grade edit | `PUT/PATCH /grades/:id` | `GradeHandler.Update` | None | Request is snake_case after data-provider conversion; legacy `score` is normalized to `grade_value` | JSON `{data: grade}` | Implemented compatibility alias | Integrated: generic data provider PUT and legacy PATCH both supported | Not ready: composite grade identity must be smoke-tested | Static contract smoke passed — 2026-08-02; seeded runtime blocked by DB/module cache |
| Grade delete | `DELETE /grades/:id` | `GradeHandler.Delete` | None | Path ID; no request body | JSON `{data: {id, status: "deleted"}}` | Implemented soft delete (`deleted_at`) with final-grade recalculation | Integrated: grades page delete action | Not ready: authorization and seeded-data smoke pending | Static contract smoke passed — 2026-08-02; seeded runtime blocked by DB/module cache |
| Grade component edit/delete | `PUT/DELETE /grade-components/:id` | `GradeComponentHandler.Update/Delete` | None | JSON `code`, `name`, optional `description`; code is preserved when omitted | JSON `{data: gradeComponent}`; delete returns `{data: {id, status: "deleted"}}` | Implemented soft delete (`deleted_at`) | Integrated: grade-component edit/delete resources | Not ready: seeded authorization and referential smoke pending | Static contract smoke passed — 2026-08-02; seeded runtime blocked by DB/module cache |
| Student CSV import | `POST /students/import` | `StudentHandler.ImportCSV` | None | CSV headers: `nis,full_name,gender,birth_date,address,phone`; optional `Idempotency-Key` | JSON `{data: {created, failed, failures}}` | Implemented: 5 MiB/10,000-row limits, deterministic/keyed replay, duplicate row failures, audit record, best-effort row transactions | Integrated | Not ready: rate limiting and seeded-data smoke pending | Static contract smoke passed — 2026-08-02; seeded runtime blocked by DB/module cache |
| Teacher CSV import | `POST /teachers/import` | `TeacherHandler.ImportCSV` | None | CSV headers: `email,full_name,nip,phone,expertise`; optional `Idempotency-Key` | JSON `{data: {created, failed, failures}}` | Implemented: 5 MiB/10,000-row limits, deterministic/keyed replay, duplicate row failures, audit record, best-effort row transactions | Integrated | Not ready: rate limiting and seeded-data smoke pending | Static contract smoke passed — 2026-08-02; seeded runtime blocked by DB/module cache |
| Browser CSV compatibility export | `GET /export/students`, `/export/grades`, `/export/attendance` | `ExportCompatibilityHandler` | None | Unfiltered CSV only; `format`, `classId`, `status`, and other query filters are unsupported; columns are snake_case | Direct unfiltered `text/csv` download, no JSON envelope | Implemented | Not wired: current UI uses local table exports; routes remain compatibility-only | Not ready: filtered/XLSX exports are not implemented; authorization/data-volume smoke pending | Static contract smoke passed — 2026-08-02; seeded runtime blocked by DB/module cache |
| Attendance lesson editor | `POST /attendance`, `PUT/PATCH /attendance/:id` | `AttendanceHandler.LegacyUpsert` | `ENABLE_ATTENDANCE_ALIAS` | Request snake_case after data-provider conversion | JSON `{data: attendance}` | Feature-flagged | Integrated | Flagged/pending smoke | Static contract smoke passed — 2026-08-02; seeded runtime blocked by DB/module cache |
| Teacher preferences editor | `POST /teacher-preferences`, `PUT /teacher-preferences/:id` | `TeacherPreferenceHandler.LegacyUpsert` | None | Request snake_case after data-provider conversion | JSON `{data: preference}` | Implemented | Integrated | Pending contract smoke | Static contract smoke passed — 2026-08-02; seeded runtime blocked by DB/module cache |
| Exam events | `GET/POST/PUT/DELETE /exam-events` | `CalendarHandler` | None | Canonical snake_case | JSON envelope | Implemented compatibility alias | Integrated | Pending contract smoke | Static contract smoke passed — 2026-08-02; seeded runtime blocked by DB/module cache |
| Enrollment edit | `PUT /enrollments/:id` | `EnrollmentHandler.Update` | None | `class_id` or legacy `target_class_id` | JSON `{data: enrollment}` | Implemented compatibility alias | Integrated | Pending contract smoke | Pending — 2026-08-02 |
| Dashboard | `GET /dashboard`, `/dashboard/academics` | `DashboardHandler` | `ENABLE_DASHBOARD` | Query snake_case | JSON envelope | Feature-flagged | Integrated behind `VITE_ENABLE_DASHBOARD` | Flagged/pending smoke | Previously smoke-tested; rerun pending — 2026-08-02 |
| Analytics | `GET /analytics/*` | `AnalyticsHandler` | `ENABLE_ANALYTICS` | Query snake_case | JSON envelope | Feature-flagged | Backend dependency of the dashboard; no standalone `VITE_ENABLE_ANALYTICS` flag (attendance analytics is gated by `VITE_ENABLE_ATTENDANCE_ALIAS`) | Flagged/pending smoke; enable with dashboard when cached analytics are desired | Pending — 2026-08-02 |
| Reports page server jobs | `POST /reports/generate`, `GET /reports/status/:id`, `GET /export/:token` | `ReportHandler` | `ENABLE_REPORTS` | Request snake_case | JSON job status; tokenized CSV/PDF download | Feature-flagged | Integrated: reports page submits jobs and polls status | Flagged/pending smoke | Previously smoke-tested; rerun pending — 2026-08-02 |
| Attendance analytics local CSV | No API request (browser `Blob` from loaded summaries) | None (browser-only) | `VITE_ENABLE_ATTENDANCE_ALIAS` page gate | Derived from loaded camelCase table rows | Local `text/csv` browser download | Not applicable | Browser-only: integrated in attendance analytics page | Not ready: export is limited to rows currently loaded in the browser | Pending — 2026-08-02 |

The canonical API spec is [GO_BACKEND_API_SPECIFICATION.md](GO_BACKEND_API_SPECIFICATION.md). The admin repository contains only a pointer to that document.
