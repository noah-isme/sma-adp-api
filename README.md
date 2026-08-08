# SMA ADP API (Golang)

Backend Golang untuk migrasi Admin Panel SMA dari NestJS.

## Quickstart

```bash
cp .env.example .env
make docker-up
make dev
```

## Dokumentasi Utama

- Status progres: [`docs/PROJECT_STATUS.md`](docs/PROJECT_STATUS.md)
- Instruksi kerja: [`docs/DEVELOPMENT_WORKFLOW.md`](docs/DEVELOPMENT_WORKFLOW.md)
- FE ↔ BE mapping: [`docs/FE_BE_MAPPING.md`](docs/FE_BE_MAPPING.md)
- Compatibility contract matrix: [`docs/COMPATIBILITY_CONTRACT_MATRIX.md`](docs/COMPATIBILITY_CONTRACT_MATRIX.md)
- Complete core-resource contract: generated [`api/swagger/swagger.json`](api/swagger/swagger.json) and [`docs/GO_BACKEND_API_SPECIFICATION.md`](docs/GO_BACKEND_API_SPECIFICATION.md)
- Migration plan: [`docs/BACKEND_MIGRATION_PLAN.md`](docs/BACKEND_MIGRATION_PLAN.md)
- Cutover runbook: [`docs/operations.md`](docs/operations.md)
- Decommission checklist: [`docs/decommission.md`](docs/decommission.md)

## Runtime Docs

- Swagger: `/docs` (dev only)
- Health: `/health`, `/ready`
- Internal health diff: `/internal/ping-legacy`, `/internal/ping-go`

## Endpoint Availability

### Runtime discovery

`GET {API_PREFIX}/features` returns the flag set this process actually mounted.
It is unauthenticated on purpose (the admin shell reads it before login) and
returns only booleans plus the API prefix and env, never configuration values:

```json
{
  "data": {
    "apiPrefix": "/api/v1",
    "env": "production",
    "features": {
      "analytics": true,
      "dashboard": true,
      "scheduler": false,
      "reports": true,
      "mutations": false,
      "archives": true,
      "documents": true,
      "homerooms": false,
      "configuration": true,
      "calendarAlias": false,
      "attendanceAlias": true,
      "audit": true,
      "lessonAttendance": true
    }
  }
}
```

Prefer this endpoint over hardcoding availability in the client: a module that is
off answers `404`, and hiding its navigation entry is better than surfacing a
dead link.

### Always-on core

No flag required: `/features`, `/auth`, `/users`, `/terms`, `/subjects`,
`/classes`, `/class-subjects`, `/schedules`, `/students`, `/enrollments`,
`/grade-components`, `/grade-configs`, `/grades`, `/reports/students/{id}`,
`/reports/classes/{id}`, `/export/students`, `/export/grades`,
`/export/attendance`, `/announcements`, `/behavior-notes`, `/calendar-events`,
`/exam-events`, `/teachers`, `/teacher-preferences`, `/schedules/preferences`,
`/audit-logs`.

Note that `/schedules/preferences` is **not** gated by `ENABLE_SCHEDULER` even
though it reads like a scheduler route: teacher preferences exist independently
of the generator, so the endpoint is mounted whenever the preference service is.

### Feature flags

All flags default to **`false`**. A deployment that sets none of them serves only
the always-on core, so the admin panel will show empty states or 404s on every
gated screen. Set the flags for the modules you intend to run.

| Env var | Default | Endpoints mounted | Admin panel pages affected | Extra config required |
| --- | --- | --- | --- | --- |
| `ENABLE_ANALYTICS` | `false` | `GET /analytics/attendance`, `/analytics/grades`, `/analytics/behavior`, `/analytics/system`; also mounts `/debug/pprof` | `attendance-analytics` | `ANALYTICS_CACHE_TTL` (default `10m`); Redis optional, falls back to no cache |
| `ENABLE_DASHBOARD` | `false` | `GET /dashboard` (SUPERADMIN, ADMIN_TU, KEPALA_SEKOLAH), `GET /dashboard/academics` (teachers) | `dashboard` | `DASHBOARD_CACHE_TTL` (default `5m`) |
| `ENABLE_SCHEDULER` | `false` | `POST /schedule/generate`, `POST /schedules/generator`, `POST /schedule/save`, `GET /semester-schedule`, `GET /semester-schedule/{id}/slots`, `DELETE /semester-schedule/{id}` | `schedule-generator` | `SCHEDULER_PROPOSAL_TTL` (default `30m`) |
| `ENABLE_REPORTS` | `false` | `POST /reports/generate`, `GET /reports/status/{id}`, `GET /export/{token}` | `reports` | `REPORTS_STORAGE_DIR`, `REPORTS_SIGNED_URL_SECRET`, `REPORTS_SIGNED_URL_TTL`, `REPORTS_CLEANUP_INTERVAL`, `REPORTS_WORKER_CONCURRENCY`, `REPORTS_WORKER_RETRIES` |
| `ENABLE_MUTATIONS` | `false` | `POST /mutations`, `GET /mutations`, `GET /mutations/{id}`, `POST /mutations/{id}/review` | `mutations` | none |
| `ENABLE_ARCHIVES` | `false` | `POST/GET /archives`, `GET /archives/{id}`, `GET /archives/{id}/download`, `DELETE /archives/{id}`, plus the `/documents` alias over the same store | `archives` | `ARCHIVES_STORAGE_DIR`, `ARCHIVES_SIGNED_URL_SECRET` (**startup fails if empty**), `ARCHIVES_SIGNED_URL_TTL`, `ARCHIVES_MAX_FILE_SIZE`, `ARCHIVES_ALLOWED_MIME_TYPES` |
| `ENABLE_HOMEROOMS` | `false` | `GET /homerooms`, `GET /homerooms/{classId}`, `POST /homerooms` | `homeroom-assignments` | none |
| `ENABLE_CONFIGURATION_API` | `false` | `GET /configuration`, `GET /configuration/{key}`, `PUT /configuration/{key}`, `PUT /configuration/bulk` | `configuration` | `CONFIG_ACTIVE_TERM_ID`, `CONFIG_DEFAULT_DASHBOARD_TERM_ID`, `CONFIG_DEFAULT_CALENDAR_TERM_ID` seed defaults |
| `ENABLE_CALENDAR_ALIAS` | `false` | `GET /calendar` | `calendar` | none |
| `ENABLE_ATTENDANCE_ALIAS` | `false` | `GET /attendance`, `GET /attendance/daily`, `POST /attendance/daily`, `POST /attendance/daily/bulk`, `POST /attendance/subject`, `POST /attendance/subject/bulk`, `GET /attendance/subject`, `GET /attendance/subject/summary`, `GET /attendance/subject/{id}`, `DELETE /attendance/subject/{id}`, and the `POST/PUT/PATCH /attendance` compatibility routes | `attendance-daily`, `attendance-lesson`, `attendance-create`, `attendance-edit` | none |

Notes:

- `/documents` has no flag of its own. It is an alias over the archive store, so
  it appears exactly when `ENABLE_ARCHIVES` is set. The admin RBAC matrix grants
  `ADMIN_TU` a `documents` resource; this is the endpoint that permission resolves to.
- `/audit-logs` has no flag. Audit rows are written unconditionally by the
  middleware and services, so gating the read side would only hide data that
  already exists. Access is restricted to `SUPERADMIN`.
- Lesson (per-subject) attendance rides on `ENABLE_ATTENDANCE_ALIAS` rather than
  a separate flag, since it shares the attendance service and repositories.

### Admin panel counterparts

The admin now has `VITE_ENABLE_ANALYTICS` to match `ENABLE_ANALYTICS`. Dashboard requires both `VITE_ENABLE_DASHBOARD` + `VITE_ENABLE_ANALYTICS`, and attendance analytics requires `VITE_ENABLE_ATTENDANCE_ALIAS` + `VITE_ENABLE_ANALYTICS`. Where possible prefer reading `/features` at runtime over duplicating flags in the frontend build.

Detail status per fase ada di [`docs/PROJECT_STATUS.md`](docs/PROJECT_STATUS.md).
The latest seeded HTTP verification and per-route smoke evidence are recorded in
[`docs/COMPATIBILITY_CONTRACT_MATRIX.md`](docs/COMPATIBILITY_CONTRACT_MATRIX.md).

## Verifikasi

```bash
GOCACHE=/tmp/go-build GOMODCACHE=/tmp/go-mod go test ./...
GOCACHE=/tmp/go-build GOMODCACHE=/tmp/go-mod go vet ./...
GOCACHE=/tmp/go-build GOMODCACHE=/tmp/go-mod go build -o /tmp/sma-api-gateway ./cmd/api-gateway
```

Regenerate Swagger setelah route/DTO berubah:

```bash
make swag
```

Lihat semua target via `make help`.
