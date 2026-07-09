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
- Migration plan: [`docs/BACKEND_MIGRATION_PLAN.md`](docs/BACKEND_MIGRATION_PLAN.md)
- Cutover runbook: [`docs/operations.md`](docs/operations.md)
- Decommission checklist: [`docs/decommission.md`](docs/decommission.md)

## Runtime Docs

- Swagger: `/docs` (dev only)
- Health: `/health`, `/ready`
- Internal health diff: `/internal/ping-legacy`, `/internal/ping-go`

## Endpoint Availability

- Always-on core: `/auth`, `/users`, `/terms`, `/subjects`, `/classes`, `/schedules`, `/students`, `/enrollments`, `/grade-components`, `/grade-configs`, `/grades`, `/reports/students/{id}`, `/reports/classes/{id}`, `/announcements`, `/behavior-notes`, `/calendar-events`, `/teachers`.
- Feature-flagged: `/analytics` (`ENABLE_ANALYTICS`), `/dashboard` (`ENABLE_DASHBOARD`), `/schedule/generate` and semester schedules (`ENABLE_SCHEDULER`), async `/reports/generate` and `/export/{token}` (`ENABLE_REPORTS`), `/mutations` (`ENABLE_MUTATIONS`), `/archives` (`ENABLE_ARCHIVES`), `/homerooms` (`ENABLE_HOMEROOMS`), `/calendar` and `/attendance` aliases (`ENABLE_CALENDAR_ALIAS`, `ENABLE_ATTENDANCE_ALIAS`), `/configuration` (`ENABLE_CONFIGURATION_API`).

Detail status per fase ada di [`docs/PROJECT_STATUS.md`](docs/PROJECT_STATUS.md).

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
