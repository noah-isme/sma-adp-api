# Project Status

Dokumen ini adalah sumber utama untuk membaca progres pekerjaan backend Go. Gunakan dokumen ini sebelum membuka task baru, sebelum mengubah route, dan sebelum menandai pekerjaan selesai.

## Ringkasan Saat Ini

Backend Go sudah memiliki implementasi internal untuk domain utama dan API gateway sudah mengekspos endpoint core. Modul lanjutan tetap dikontrol feature flag sampai kontrak dan smoke test selesai (shadow compare SKIPPED karena legacy backend offline).

Sumber kebenaran teknis:

- Runtime route: `cmd/api-gateway/main.go`
- Kontrak API publik: `api/swagger/docs.go`, `api/swagger/swagger.json`, `api/swagger/swagger.yaml`
- Compatibility contract matrix (compatibility status, FE integration, readiness, smoke date): `docs/COMPATIBILITY_CONTRACT_MATRIX.md`
- Mapping index: `docs/FE_BE_MAPPING.md`
- Cutover/rollback: `docs/operations.md`
- Decommission: `docs/decommission.md`

### Latest compatibility verification (2026-08-06)

- `python3 scripts/validate_swagger_routes.py` passed (101 generated paths cover
  all gateway routes).
- `python3 scripts/compatibility_smoke.py` passed its static check (22 required
  compatibility operations are present in both gateway and Swagger).
- **Extended compatibility matrix** with 13 new rows covering:
  - Attendance analytics server aggregation (`/analytics/attendance`)
  - Reports template selection (`POST /reports/generate` with template field)
  - Schedule generator preview (`/schedules/generator`) with conflicts & stats
  - Schedule generator legacy (`/schedule/generate`) - deprecated
  - Schedule save by proposal (`POST /schedule/save` with proposalId)
  - Auth token refresh (`POST /auth/refresh`) - single refresh mechanism
  - Feature discovery (`GET /features`) - runtime feature flags
  - Mutations list & approve/reject (`GET/PATCH /mutations`)
  - Archives list, upload, download (`GET/POST /archives`, `GET /archives/:id/download`)
- Seeded runtime verification passed with Postgres, Redis, migrations 15–18,
  `scripts/seed.sql`, and all compatibility feature flags enabled. The suite
  covered roster/report reads, CSV exports, grade and component edit/delete,
  student and teacher CSV import replay, attendance POST/PATCH, teacher
  preference POST/PUT, exam-event CRUD, enrollment transfer/restore, dashboard,
  analytics, and the asynchronous report job/status/download flow. Every
  compatibility matrix row now records its actual result or explicitly notes
  that it is browser-only.
- The seeded run also exposed a non-fatal audit persistence warning on login
  (`audit_logs` JSON encoding); authentication and all HTTP smoke assertions
  still returned their expected 2xx responses. Treat audit-log persistence as a
  follow-up before production sign-off.
- Go package tests remain blocked by the environment's module-cache/disk quota;
  no current Go test or vet pass is claimed here. The runtime verification above
  is an HTTP-level result against the seeded gateway.

## Status Per Fase

```mermaid
graph TB
    subgraph "Phase 0: Infrastructure"
        P0[✅ Implemented]
    end
    
    subgraph "Phase 1: Auth & User Mgmt"
        P1[✅ Implemented + Exposed]
    end
    
    subgraph "Phase 2: Academic Mgmt"
        P2[✅ Implemented + Exposed]
    end
    
    subgraph "Phase 3: Student & Assessment"
        P3[✅ Implemented + Exposed]
    end
    
    subgraph "Phase 4: Attendance & Comm"
        P4[✅ Implemented + Exposed]
    end
    
    subgraph "Phase 5: Analytics & Optimization"
        P5[🔶 Implemented (Flagged)]
    end
    
    subgraph "Phase 6: Cutover & Decommission"
        P6[🔶 Support Implemented]
    end
    
    P0 --> P1 --> P2 --> P3 --> P4 --> P5 --> P6
```

| Fase | Area | Status | Catatan |
| --- | --- | --- | --- |
| Phase 0 | Infrastructure, config, DB, middleware, Docker, Swagger | Implemented | Health, readiness, metrics, cutover headers, Docker Postgres/Redis, Makefile tersedia. |
| Phase 1 | Auth dan user management | Implemented + exposed | `/auth` dan `/users` aktif di gateway. |
| Phase 2 | Academic management | Implemented + exposed | Terms, subjects, classes, class-subjects, schedules CRUD aktif. Scheduler/generator tetap feature-flagged. |
| Phase 3 | Student, enrollment, grading, report card | Implemented + exposed | Students, enrollments, grade components/configs/grades, report card JSON aktif. Async reports tetap feature-flagged. |
| Phase 4 | Attendance, communication, calendar | Implemented + exposed | Announcements, behavior notes, calendar events aktif. Attendance CRUD (daily/subject and compatibility routes) are always-on; calendar FE alias remains separately feature-flagged. |
| Phase 5 | Analytics, dashboard, cache, async reports, scheduler | Implemented behind flags | Perlu contract test, smoke test, dan data validation sebelum dianggap production-ready. |
| Phase 6 | Cutover, rollback, decommission | Planned + support implemented | Middleware, runbook, dan checklist ada. Cutover produksi belum selesai. |

## Endpoint Availability

```mermaid
graph TB
    subgraph "Always-On Core Endpoints"
        AUTH[/auth]
        USERS[/users]
        TERMS[/terms]
        SUBJ[/subjects]
        CLASSES[/classes]
        SCHED[/schedules]
        STUD[/students]
        ENROL[/enrollments]
        GC[/grade-components]
        GCFG[/grade-configs]
        GRADES[/grades]
        RPT[/reports/*]
        ANNOUNCE[/announcements]
        BEH[/behavior-notes]
        CAL[/calendar-events]
        TEACH[/teachers]
        CSUBJ[/class-subjects]
        TPREF[/teacher-preferences]
        ATTD[/attendance/daily]
        ATTS[/attendance/subject]
        ATTDB[/attendance/daily/bulk]
        ATTSB[/attendance/subject/bulk]
    end
    
    subgraph "Feature-Flagged Modules"
        ANALYTICS[/analytics/*] -->|ENABLE_ANALYTICS| ANALYTICS
        DASH[/dashboard/*] -->|ENABLE_DASHBOARD| DASH
        SCHEDGEN[/schedule/generate] -->|ENABLE_SCHEDULER| SCHEDGEN
        ASYNCRPT[/reports/generate] -->|ENABLE_REPORTS| ASYNCRPT
        MUT[/mutations] -->|ENABLE_MUTATIONS| MUT
        ARCH[/archives] -->|ENABLE_ARCHIVES| ARCH
        HOMEROOM[/homerooms] -->|ENABLE_HOMEROOMS| HOMEROOM
        CALALIAS[/calendar] -->|ENABLE_CALENDAR_ALIAS| CALALIAS
        CONFIG[/configuration] -->|ENABLE_CONFIGURATION_API| CONFIG
    end
```

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
- `/class-subjects`
- `/teacher-preferences`
- `/attendance` (and sub-routes)

Attendance routes and compatibility aliases are now always-on core endpoints.

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
| Configuration API | `/configuration` | `ENABLE_CONFIGURATION_API` |

## Milestone Terakhir

- Core route gateway dibuka untuk auth, users, academic, student, enrollment, grading, reports JSON, communication, calendar event, dan teachers.
- Handler HTTP ditambahkan untuk announcements, behavior notes, dan calendar events.
- **New endpoints added**: `GET /class-subjects` (standalone), `POST /attendance/daily`, `POST /attendance/daily/bulk`, `POST /attendance/subject`, `POST /attendance/subject/bulk` (attendance CRUD), `GET /teacher-preferences` (standalone).
- **Frontend gap fixes completed (2026-08-06)**:
  - Attendance Analytics: Server-side aggregation via `/analytics/attendance` (eliminates 5000+ client fetches)
  - Reports Generation: PDF template selector (simple/detailed/landscape) via `template` field
  - Scheduler Generator: Canonical `/schedules/generator` endpoint with conflict resolution UI, proposalId save
  - Auth/Feature Flags: Single token refresh mechanism (`POST /auth/refresh`), removed dual storage, removed dev bypasses
  - Mutations/Archives: Audit trail UI, approve/reject actions, document preview, upload, download
- Swagger pindah ke output standar Swaggo: `docs.go`, `swagger.json`, dan `swagger.yaml`.
- `api/swagger/swagger.go` lama tidak dipakai lagi karena konflik dengan generated `docs.go`.
- Contract collection diperluas dari health-only menjadi smoke contract berfolder: `Public Gateway`, `Cutover Readiness Smoke`, `Core Protected Smoke`, `Seeded Core Smoke`, dan `Gated Feature Smoke`.
- Target `make contract-test` default hanya menjalankan `Public Gateway` dan `Core Protected Smoke`, serta membutuhkan `ACCESS_TOKEN` untuk protected endpoint.
- **Contract test pertama kali dijalankan dengan server Go hidup** (12 Jul 2026) menggunakan Docker Postgres, Redis, seed data (`scripts/seed.sql`), dan token SUPERADMIN/ADMIN/TEACHER.
- **4 bug ditemukan dan diperbaiki** saat contract test dan RBAC smoke test:
  - Bug 1: `grade_config_components` tabel tidak punya kolom `created_at` yang diharapkan model dan query `loadComponents`. Fix: migrasi `000013` menambah kolom `created_at` dengan `DEFAULT CURRENT_TIMESTAMP`.
  - Bug 2: `ClassDistribution` query di `grade_final_repository.go` error SQL karena `SELECT gf.subject_id, e.term_id` tanpa `GROUP BY` saat menggunakan aggregate functions (MIN/MAX/AVG). Fix: gunakan input parameter sebagai konstanta di SELECT (`$3::varchar AS subject_id, $2::varchar AS term_id`), sehingga aggregate selalu return 1 row.
  - Bug 3: `Dashboard Academics` 404 — handler mengirim `claims.UserID` sebagai `teacherID`, tapi tabel `teachers` pakai ID berbeda. Fix: migrasi `000014` menambah kolom `user_id` di `teachers`, `FindByUserID` method di repository, dan `TeacherID` field di JWT claims yang di-resolve saat login.
  - Bug 4: `Calendar Alias` 500 untuk role TEACHER — root cause sama dengan Bug 3. Fix sistemik: `TeacherID` di JWT claims digunakan di `calendar_alias_service.go`, `attendance_alias_service.go`, dan `homeroom_service.go` dengan fallback ke `claims.UserID` untuk backward compat.
- **Contract collection diperbaiki**: 6 request tidak mengirim query param yang required (reports, dashboard, attendance butuh `termId`; class report butuh `subjectId` + `termId`; dashboard academics butuh `teacherId`). Query params ditambahkan ke collection.
- **RBAC smoke test selesai** untuk 3 role (SUPERADMIN, ADMIN, TEACHER). Semua RBAC denial yang teramati adalah expected behavior.
- **Rollback drill tabletop selesai** — lihat `docs/decommission.md`.
- **Shadow compare: SKIPPED / DEPRECATED** — legacy NestJS backend tidak tersedia di environment ini.
- Hasil contract test SUPERADMIN/ADMIN: **37 dari 38 request return 200 OK, 75 dari 76 assertions pass** (1 failure: Report Job Status 404 — expected).
- Hasil contract test TEACHER: **29 dari 38 request return 200 OK** (9 failures: semua expected RBAC denials atau validation errors).
- Last successful verification — 12 Jul 2026: `go test ./...`, `go vet ./...`, dan `go build` semua pass.

## Hasil Contract Test (12 Jul 2026)

### SUPERADMIN / ADMIN Role

| Folder | Request | Status | Catatan |
| --- | --- | --- | --- |
| Public Gateway | Health, Ready, Cutover Ping Go | 3/3 OK | Semua 200. |
| Core Protected Smoke | Auth Me, Users, Terms, Subjects, Classes, Schedules, Students, Enrollments, Grade Components, Grade Configs, Grades, Announcements, Behavior Notes, Calendar Events, Teachers | 15/15 OK | Semua 200. |
| Seeded Core Smoke | Class Subjects, Class Schedules, Student Detail, Student Behavior Summary, Grade Config Detail, Student Report, Class Report, Teacher Assignments, Teacher Preferences, Teacher Schedules | 10/10 OK | Semua 200. |
| Gated Feature Smoke | Dashboard Admin, Dashboard Academics, Calendar Alias, Attendance Summary, Attendance Daily, Configuration List, Homerooms List, Mutations List, Archives List | 9/9 OK | Semua 200. |
| Gated Feature Smoke | Report Job Status | 404 | Expected — non-existent job ID. |

### TEACHER Role

| Endpoint | Status | Catatan |
| --- | --- | --- |
| Public Gateway (3) | 200 | Health, Ready, Ping Go |
| Auth Me | 200 | |
| Users List | 403 | Expected RBAC — teacher can't manage users |
| Terms, Subjects, Classes, Schedules, Students, Enrollments | 200 | Read access for teachers |
| Grade Components, Grade Configs, Grades | 200 | Read access |
| Announcements, Behavior Notes, Calendar Events | 200 | Read access |
| Teachers List | 403 | Expected RBAC — admin-only |
| Class Subjects, Class Schedules | 200 | |
| Student Detail, Student Behavior Summary | 200 | |
| Grade Config Detail | 200 | |
| Student Report, Class Report | 200 | With termId/subjectId params |
| Teacher Assignments, Preferences, Schedules | 403 | Expected RBAC — admin-only |
| Dashboard Admin | 403 | Expected RBAC — admin/superadmin only |
| **Dashboard Academics** | **200** | **Auto-resolved via JWT TeacherID** |
| **Calendar Alias** | **200** | **Fixed — was 500 before TeacherID fix** |
| Attendance Summary | 200 | With classId for teachers |
| Attendance Daily | 400 | Expected — classId required for teachers |
| Configuration List | 403 | Expected RBAC — admin-only |
| Homerooms List | 200 | |
| Mutations List | 200 | |
| Archives List | 200 | |

## Pekerjaan Berikutnya

1. ~~Jalankan contract test dengan API Go yang hidup~~ — **SELESAI** (12 Jul 2026).
2. ~~Jalankan folder `Seeded Core Smoke` setelah seed ID tersedia~~ — **SELESAI**.
3. ~~Jalankan folder `Gated Feature Smoke` setelah feature flag aktif~~ — **SELESAI**.
4. ~~Perbaiki design issue Dashboard Academics~~ — **SELESAI** (migrasi 000014 + JWT TeacherID).
5. ~~Validasi smoke test RBAC untuk role `SUPERADMIN`, `ADMIN`, dan `TEACHER`~~ — **SELESAI**.
6. ~~Rollback drill tabletop~~ — **SELESAI** (lihat `docs/decommission.md`).
7. ~~**Shadow compare** — BLOCKED: legacy NestJS backend tidak tersedia. Perlu provisi legacy backend di dev/staging.~~ (SKIPPED/DEPRECATED)
8. Lanjutkan checklist cutover di `docs/operations.md` dan `docs/decommission.md`.

## Saran Pengembangan Selanjutnya

### Prioritas Tinggi — Unblock Cutover

1. ~~**Provisi legacy NestJS backend untuk shadow compare**~~ (SKIPPED)

2. **Automasi rollback dengan `make rollback` target**
   - Gabungkan `make toggle-go value=false` + cache purge (`auth`, `grades`, `attendance` keys) dalam satu command.
   - Tambahkan konfirmasi interaktif untuk mencegah eksekusi tidak sengaja.
   - Target eksekusi: < 2 menit dari alert trigger ke traffic kembali ke legacy.

3. **Tambahkan Prometheus alerting rules**
   - `HighErrorRate`: error rate > 1% selama 15 menit.
   - `LatencySLOViolation`: p99 > 600ms selama 15 menit.
   - `CacheMissSpike`: cache hit ratio drop > 30% dalam 5 menit.
   - `DBSlowQuery`: query duration p95 > 500ms.
   - Wire alerts ke notification channel (Slack/PagerDuty).

4. **Rollback drill produksi**
   - Jalankan rollback drill di staging/produksi (bukan tabletop).
   - Catat hasil di tabel `docs/decommission.md` Rollback Drill Log.

### Prioritas Sedang — Hardening

5. **Contract test sebagai CI gate**
   - Integrasi contract test ke GitHub Actions workflow dengan service container Postgres + Redis.
   - Auto-apply migrations + seed sebelum test.
   - Gate PR merge pada contract test pass untuk 3 role (SUPERADMIN, ADMIN, TEACHER).

6. **Test coverage untuk handler layer**
   - `DashboardHandler`, `GradeConfigHandler`, `ReportHandler` saat ini tidak punya covering tests (per codegraph analysis).
   - Tambah unit test untuk edge cases: missing param, invalid ID, unauthorized role, cache miss.

7. **Fix contract collection assertion untuk Report Job Status**
   - Endpoint `GET /reports/status/{id}` dengan non-existent ID correctly return 404.
   - Update test assertion untuk accept 404 sebagai valid response (bukan hanya 200..299).

8. **Seeder CLI tool**
   - Buat `make seed` target yang auto-apply `scripts/seed.sql` setelah migrate.
   - Tambah opsi `make seed-reset` untuk clean + reseed (berguna untuk dev dan CI).

### Prioritas Rendah — Technical Debt

9. **Konsolidasi teacher ID resolution**
   - Saat ini `claims.TeacherID` dipakai dengan fallback `claims.UserID` di 4 service.
   - Setelah semua token lama expire, hapus fallback path untuk mengurangi kompleksitas.
   - Tambahkan metrics untuk track fallback usage.

10. **Dokumentasi API endpoint yang membutuhkan query param**
    - Swagger annotation untuk `reports/students/{id}`, `reports/classes/{id}`, `dashboard`, `attendance` belum mendokumentasikan required query params (`termId`, `subjectId`, `classId`, `teacherId`).
    - Update `@Param` annotation dan regenerate swagger.

11. **Homeroom service: teacher-scoped query optimization**
    - `homeroom_service.go` melakukan 2x teacher ID resolution per request (filter + list).
    - Cache `teacherID` di context atau pass sebagai parameter untuk avoid repeated resolution.

12. **Database connection pool tuning**
    - Validasi `DB_MAX_OPEN_CONNS=25` dan `DB_MAX_IDLE_CONNS=5` under load test.
    - Tambahkan `ConnMaxLifetime` dan `ConnMaxIdleTime` ke config untuk prevent stale connections.

## Blocker Aktif

- **Shadow compare SKIPPED / DEPRECATED**: Legacy NestJS backend tidak tersedia di environment ini.
- Cutover produksi belum boleh ditandai selesai sampai rollback drill produksi terdokumentasi.
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

Contract test manual dengan seed data (semua folder kecuali Cutover Readiness):

```bash
# 1. Start Docker Postgres + Redis
make docker-up

# 2. Apply migrations dan seed
migrate -path migrations -database "postgresql://postgres:postgres@localhost:5432/admin_panel_sma?sslmode=disable" up
psql "postgresql://postgres:postgres@localhost:5432/admin_panel_sma?sslmode=disable" -f scripts/seed.sql

# 3. Start Go API server
make dev

# 4. Login sebagai superadmin (password: admin123) untuk mendapatkan token
curl -X POST http://localhost:8080/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{"email":"superadmin@sma.test","password":"admin123"}'

# 5. Jalankan Newman dengan seed IDs dan feature flags aktif
docker run --rm --network host \
  -v "$(pwd)/tests/contract:/etc/newman" \
  postman/newman:alpine \
  run contract.postman_collection.json \
  --env-var "baseUrl=http://localhost:8080/api/v1" \
  --env-var "gatewayUrl=http://localhost:8080" \
  --env-var "accessToken=<token-dari-step-4>" \
  --env-var "termId=term-001" \
  --env-var "classId=cls-001" \
  --env-var "studentId=std-001" \
  --env-var "teacherId=tch-001" \
  --env-var "subjectId=subj-001" \
  --env-var "scheduleId=sched-001" \
  --env-var "gradeConfigId=gcfg-001" \
  --env-var "reportJobId=job-nonexistent-001" \
  --folder "Public Gateway" \
  --folder "Core Protected Smoke" \
  --folder "Seeded Core Smoke" \
  --folder "Gated Feature Smoke"
```

Manual folder seeded/gated dapat dijalankan langsung dengan Newman setelah variable ID dan feature flag siap. Jangan memasukkan token ke file collection atau dokumentasi repo.
