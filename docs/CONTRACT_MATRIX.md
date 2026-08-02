# Frontend ↔ Go API Contract Matrix

This is the single route-contract matrix for the admin application. The API specification describes payloads and behavior; this matrix records the integration and readiness state. Update this file when a frontend caller, route, handler, flag, or smoke result changes.

Status vocabulary:

- **Backend implementation:** `Implemented`, `Feature-flagged`, or `Missing`.
- **Frontend integration:** `Integrated`, `Browser-only`, `Mock-only`, or `Not wired`.
- **Production readiness:** `Ready`, `Flagged/pending smoke`, or `Not ready`.
- Smoke status is deliberately dated; `Pending` means the route exists but has not been re-run in the current environment.

| Frontend caller | HTTP method/path | Go handler | Feature flag | Payload casing | Response | Backend implementation | Frontend integration | Production readiness | Smoke-test status/date |
| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |
| Student roster page | `GET /students/roster` | `StudentHandler.Roster` | None | Query accepts camelCase aliases; API body preserves admin row names | JSON `{data: {summary, filters, rows, pagination}}` | Implemented | Integrated | Not ready: aggregate filters are compatibility-level | Pending — 2026-08-02; Go smoke blocked by cache environment |
| Teacher roster page | `GET /teachers/roster` | `TeacherHandler.Roster` | None | Query accepts camelCase aliases; API body preserves admin row names | JSON `{data: {summary, filters, rows, pagination}}` | Implemented | Integrated | Not ready: aggregate filters are compatibility-level | Pending — 2026-08-02; Go smoke blocked by cache environment |
| Grade report page | `GET /grades/report` | `GradeHandler.Report` | None | Query camelCase; canonical write payloads are snake_case | JSON `{data: {context, summary, filters, rows, pagination}}` | Partial: subject/component filters and average are implemented; rich joins/metadata are not | Integrated | Not ready: term/class/teacher/status/score filters and seeded-data validation pending | Pending — 2026-08-02 |
| Generic grade edit | `PATCH /grades/:id` | `GradeHandler.Update` | None | Request is snake_case after data-provider conversion | JSON `{data: grade}` | Implemented | Integrated | Not ready: composite grade identity must be smoke-tested | Pending — 2026-08-02 |
| Student CSV import | `POST /students/import` | `StudentHandler.ImportCSV` | None | CSV headers: `nis,full_name,gender,birth_date,address,phone` | JSON `{data: {created, failed, failures}}` | Implemented | Integrated | Not ready: production file-size/rate limits pending | Pending — 2026-08-02 |
| Teacher CSV import | `POST /teachers/import` | `TeacherHandler.ImportCSV` | None | CSV headers: `email,full_name,nip,phone,expertise` | JSON `{data: {created, failed, failures}}` | Implemented | Integrated | Not ready: production file-size/rate limits pending | Pending — 2026-08-02 |
| Student/teacher browser export | `GET /export/students`, `/export/grades`, `/export/attendance` | `ExportCompatibilityHandler` | None | Query filters are not yet supported; CSV columns are snake_case | Direct `text/csv` download, no JSON envelope | Implemented | Browser-only: current UI exports loaded rows locally | Not ready: authorization/data-volume smoke pending | Pending — 2026-08-02 |
| Attendance lesson editor | `POST /attendance`, `PUT/PATCH /attendance/:id` | `AttendanceHandler.LegacyUpsert` | `ENABLE_ATTENDANCE_ALIAS` | Request snake_case after data-provider conversion | JSON `{data: attendance}` | Feature-flagged | Integrated | Flagged/pending smoke | Pending — 2026-08-02 |
| Teacher preferences editor | `POST /teacher-preferences`, `PUT /teacher-preferences/:id` | `TeacherPreferenceHandler.LegacyUpsert` | None | Request snake_case after data-provider conversion | JSON `{data: preference}` | Implemented | Integrated | Pending contract smoke | Pending — 2026-08-02 |
| Exam events | `GET/POST/PUT/DELETE /exam-events` | `CalendarHandler` | None | Canonical snake_case | JSON envelope | Implemented compatibility alias | Integrated | Pending contract smoke | Pending — 2026-08-02 |
| Enrollment edit | `PUT /enrollments/:id` | `EnrollmentHandler.Update` | None | `class_id` or legacy `target_class_id` | JSON `{data: enrollment}` | Implemented compatibility alias | Integrated | Pending contract smoke | Pending — 2026-08-02 |
| Dashboard | `GET /dashboard`, `/dashboard/academics` | `DashboardHandler` | `ENABLE_DASHBOARD` | Query snake_case | JSON envelope | Feature-flagged | Integrated behind `VITE_ENABLE_DASHBOARD` | Flagged/pending smoke | Previously smoke-tested; rerun pending — 2026-08-02 |
| Analytics | `GET /analytics/*` | `AnalyticsHandler` | `ENABLE_ANALYTICS` | Query snake_case | JSON envelope | Feature-flagged | No standalone admin resource; consumed by dashboard/analytics pages | Flagged/pending frontend pairing | Pending — 2026-08-02 |
| Async reports | `POST /reports/generate`, `GET /reports/status/:id`, `GET /export/:token` | `ReportHandler` | `ENABLE_REPORTS` | Request snake_case | JSON jobs; direct CSV/PDF download | Feature-flagged | Not used by current browser-only export buttons | Flagged/pending smoke | Previously smoke-tested; rerun pending — 2026-08-02 |

The canonical API spec is [GO_BACKEND_API_SPECIFICATION.md](GO_BACKEND_API_SPECIFICATION.md). The admin repository contains only a pointer to that document.
