# Project Status

Dokumen ini adalah sumber utama untuk membaca progres pekerjaan backend Go. Gunakan dokumen ini sebelum membuka task baru, sebelum mengubah route, dan sebelum menandai pekerjaan selesai.

## Ringkasan Saat Ini

Backend Go sudah memiliki implementasi internal untuk domain utama dan API gateway sudah mengekspos endpoint core. Modul lanjutan tetap dikontrol feature flag sampai kontrak, smoke test, dan shadow compare terhadap backend legacy selesai.

Sumber kebenaran teknis:

- Runtime route: `cmd/api-gateway/main.go`
- Kontrak API publik: `api/swagger/docs.go`, `api/swagger/swagger.json`, `api/swagger/swagger.yaml`
- Mapping frontend: `docs/FE_BE_MAPPING.md`
- Cutover/rollback: `docs/operations.md`
- Decommission: `docs/decommission.md`

## Status Per Fase

| Fase | Area | Status | Catatan |
| --- | --- | --- | --- |
| Phase 0 | Infrastructure, config, DB, middleware, Docker, Swagger | Implemented | Health, readiness, metrics, cutover headers, Docker Postgres/Redis, Makefile tersedia. |
| Phase 1 | Auth dan user management | Implemented + exposed | `/auth` dan `/users` aktif di gateway. |
| Phase 2 | Academic management | Implemented + exposed | Terms, subjects, classes, class-subjects, schedules CRUD aktif. Scheduler/generator tetap feature-flagged. |
| Phase 3 | Student, enrollment, grading, report card | Implemented + exposed | Students, enrollments, grade components/configs/grades, report card JSON aktif. Async reports tetap feature-flagged. |
| Phase 4 | Attendance, communication, calendar | Partially exposed | Announcements, behavior notes, calendar events aktif. Attendance dan calendar FE aliases tetap feature-flagged. |
| Phase 5 | Analytics, dashboard, cache, async reports, scheduler | Implemented behind flags | Perlu contract test, smoke test, dan data validation sebelum dianggap production-ready. |
| Phase 6 | Cutover, rollback, decommission | Planned + support implemented | Middleware, runbook, shadow compare script, dan checklist ada. Cutover produksi belum selesai. |

## Endpoint Availability

Always-on core:

- `/auth`
- `/users`
- `/terms`
- `/subjects`
- `/classes`
- `/schedules`
- `/students`
- `/enrollments`
- `/grade-components`
- `/grade-configs`
- `/grades`
- `/reports/students/{id}`
- `/reports/classes/{id}`
- `/announcements`
- `/behavior-notes`
- `/calendar-events`
- `/teachers`

Feature-flagged:

| Modul | Endpoint utama | Env flag |
| --- | --- | --- |
| Analytics | `/analytics/*` | `ENABLE_ANALYTICS` |
| Dashboard | `/dashboard`, `/dashboard/academics` | `ENABLE_DASHBOARD` |
| Scheduler | `/schedule/generate`, `/schedules/generator`, `/semester-schedule` | `ENABLE_SCHEDULER` |
| Async reports/export | `/reports/generate`, `/reports/status/{id}`, `/export/{token}` | `ENABLE_REPORTS` |
| Mutations | `/mutations` | `ENABLE_MUTATIONS` |
| Archives | `/archives` | `ENABLE_ARCHIVES` |
| Homerooms | `/homerooms` | `ENABLE_HOMEROOMS` |
| Calendar alias | `/calendar` | `ENABLE_CALENDAR_ALIAS` |
| Attendance alias | `/attendance`, `/attendance/daily` | `ENABLE_ATTENDANCE_ALIAS` |
| Configuration API | `/configuration` | `ENABLE_CONFIGURATION_API` |

## Milestone Terakhir

- Core route gateway dibuka untuk auth, users, academic, student, enrollment, grading, reports JSON, communication, calendar event, dan teachers.
- Handler HTTP ditambahkan untuk announcements, behavior notes, dan calendar events.
- Swagger pindah ke output standar Swaggo: `docs.go`, `swagger.json`, dan `swagger.yaml`.
- `api/swagger/swagger.go` lama tidak dipakai lagi karena konflik dengan generated `docs.go`.
- Contract collection diperluas dari health-only menjadi smoke contract berfolder: `Public Gateway`, `Cutover Readiness Smoke`, `Core Protected Smoke`, `Seeded Core Smoke`, dan `Gated Feature Smoke`.
- Target `make contract-test` default hanya menjalankan `Public Gateway` dan `Core Protected Smoke`, serta membutuhkan `ACCESS_TOKEN` untuk protected endpoint.
- Verifikasi terakhir setelah pembaruan collection/workflow sudah pass: `go test ./...`, `go vet ./...`, dan `go build -o /tmp/sma-api-gateway ./cmd/api-gateway`.
- Contract runtime belum dijalankan karena membutuhkan server Go hidup dan `ACCESS_TOKEN` valid.

## Pekerjaan Berikutnya

1. Jalankan contract test dengan API Go yang hidup dan token admin/superadmin: `ACCESS_TOKEN=<token> make contract-test BASE_URL=http://localhost:8080/api/v1`.
2. Jalankan folder `Seeded Core Smoke` setelah seed ID tersedia untuk `classId`, `studentId`, `teacherId`, dan `gradeConfigId`.
3. Jalankan folder `Gated Feature Smoke` hanya setelah feature flag terkait aktif.
4. Jalankan folder `Cutover Readiness Smoke` dan shadow compare setelah legacy hidup: `make shadow-compare GO_BASE_URL=http://localhost:8080 LEGACY_BASE_URL=http://localhost:3000`.
5. Validasi smoke test RBAC untuk role `SUPERADMIN`, `ADMIN`, dan `TEACHER`.
6. Baru setelah parity stabil, lanjutkan checklist cutover di `docs/operations.md` dan `docs/decommission.md`.

## Blocker Aktif

- Contract test default membutuhkan server Go hidup, Docker/Newman, dan `ACCESS_TOKEN` valid. Token tidak boleh disimpan di repo.
- Folder seeded contract membutuhkan data uji dan variable ID yang valid sebelum bisa dijadikan gate CI.
- Folder gated contract membutuhkan feature flag aktif sesuai modul yang diuji.
- Shadow compare membutuhkan server Go dan server legacy hidup.
- Cutover produksi belum boleh ditandai selesai sampai contract test, shadow compare, dan rollback drill terdokumentasi.
- Dokumen fase lama masih berisi desain rinci dan estimasi awal; status aktual harian harus dirujuk ke dokumen ini.

## Command Verifikasi

Gunakan cache writable jika environment default read-only:

```bash
GOCACHE=/tmp/go-build GOMODCACHE=/tmp/go-mod go test ./...
GOCACHE=/tmp/go-build GOMODCACHE=/tmp/go-mod go vet ./...
GOCACHE=/tmp/go-build GOMODCACHE=/tmp/go-mod go build -o /tmp/sma-api-gateway ./cmd/api-gateway
```

Regenerate Swagger setelah route/DTO berubah:

```bash
swag init -g cmd/api-gateway/main.go -o api/swagger --parseDependency --parseInternal
```

Jika `swag` belum terinstall:

```bash
GOCACHE=/tmp/go-build GOMODCACHE=/tmp/go-mod go run github.com/swaggo/swag/cmd/swag@v1.16.1 init -g cmd/api-gateway/main.go -o api/swagger --parseDependency --parseInternal
```

Contract test default:

```bash
ACCESS_TOKEN=<admin-or-superadmin-token> make contract-test BASE_URL=http://localhost:8080/api/v1
```

Manual folder seeded/gated dapat dijalankan langsung dengan Newman setelah variable ID dan feature flag siap. Jangan memasukkan token ke file collection atau dokumentasi repo.
