# Development Workflow

Dokumen ini adalah instruksi kerja harian untuk mengurangi kesalahan saat mengubah kode backend Go.

## Before Coding

- Jalankan `git status --short` dan pahami perubahan yang sudah ada. Jangan revert perubahan yang tidak dibuat sendiri.
- Baca `docs/PROJECT_STATUS.md` untuk mengetahui status fase, endpoint aktif, feature flag, dan blocker.
- Cek route aktual di `cmd/api-gateway/main.go` sebelum menambah endpoint.
- Cari handler, service, repository, model, dan DTO yang sudah ada sebelum membuat file baru.
- Cek `docs/FE_BE_MAPPING.md` dan Swagger untuk memastikan endpoint belum tersedia.
- Tentukan apakah fitur harus always-on atau feature-flagged.

## Aturan Implementasi Route

- Semua endpoint publik harus berada di bawah `cfg.APIPrefix`.
- Endpoint protected wajib memakai JWT middleware.
- Endpoint protected wajib memakai RBAC yang sesuai dengan pola domain existing.
- Jangan bypass `response.JSON`, `response.Created`, `response.Error`, atau `response.NoContent` untuk response JSON standar.
- Route runtime di `cmd/api-gateway/main.go` adalah sumber kebenaran. Dokumentasi harus mengikuti file ini, bukan sebaliknya.

## Aturan Feature Flag

Modul berikut tetap gated kecuali ada keputusan eksplisit untuk membuatnya always-on:

- `ENABLE_ANALYTICS`
- `ENABLE_DASHBOARD`
- `ENABLE_SCHEDULER`
- `ENABLE_REPORTS`
- `ENABLE_MUTATIONS`
- `ENABLE_ARCHIVES`
- `ENABLE_HOMEROOMS`
- `ENABLE_CALENDAR_ALIAS`
- `ENABLE_ATTENDANCE_ALIAS`
- `ENABLE_CONFIGURATION_API`

Saat menambah endpoint ke modul gated, daftarkan route hanya di dalam blok config terkait dan update `docs/PROJECT_STATUS.md`.

## Aturan Swagger

- Jangan edit `api/swagger/docs.go`, `api/swagger/swagger.json`, atau `api/swagger/swagger.yaml` secara manual.
- Jangan menghidupkan kembali `api/swagger/swagger.go`; file itu sudah diganti oleh generated `docs.go`.
- Regenerate Swagger setelah route, handler annotation, request DTO, atau response DTO berubah.

Command standar:

```bash
swag init -g cmd/api-gateway/main.go -o api/swagger --parseDependency --parseInternal
```

Fallback jika `swag` belum tersedia:

```bash
GOCACHE=/tmp/go-build GOMODCACHE=/tmp/go-mod go run github.com/swaggo/swag/cmd/swag@v1.16.1 init -g cmd/api-gateway/main.go -o api/swagger --parseDependency --parseInternal
```

## Before Finalizing Work

- Jalankan test:

```bash
GOCACHE=/tmp/go-build GOMODCACHE=/tmp/go-mod go test ./...
```

- Jalankan vet:

```bash
GOCACHE=/tmp/go-build GOMODCACHE=/tmp/go-mod go vet ./...
```

- Jalankan build gateway:

```bash
GOCACHE=/tmp/go-build GOMODCACHE=/tmp/go-mod go build -o /tmp/sma-api-gateway ./cmd/api-gateway
```

- Regenerate Swagger jika route/DTO berubah.
- Update `docs/PROJECT_STATUS.md` jika status pekerjaan berubah.
- Update `docs/FE_BE_MAPPING.md` jika public endpoint atau status endpoint berubah.
- Update Postman collection jika kontrak endpoint berubah.

## Contract Test Workflow

Target default hanya menjalankan folder `Public Gateway` dan `Core Protected Smoke` di `tests/contract/contract.postman_collection.json`. Folder ini berisi request public gateway dan protected read-only endpoint always-on.

Jalankan dengan token admin atau superadmin:

```bash
ACCESS_TOKEN=<admin-or-superadmin-token> make contract-test BASE_URL=http://localhost:8080/api/v1
```

Aturan contract test:

- Jangan simpan token, password, atau credential lain di collection.
- Gunakan variable `accessToken` untuk protected request.
- Gunakan `gatewayUrl` untuk `/health`, `/ready`, dan `/internal/*` karena route tersebut tidak berada di bawah `cfg.APIPrefix`.
- Gunakan `baseUrl` untuk endpoint publik API di bawah `cfg.APIPrefix`, default `http://localhost:8080/api/v1`.
- Folder `Seeded Core Smoke` hanya dijalankan jika seed ID valid tersedia untuk `classId`, `studentId`, `teacherId`, dan `gradeConfigId`.
- Folder `Gated Feature Smoke` hanya dijalankan jika feature flag modul terkait aktif.
- Folder `Cutover Readiness Smoke` hanya dijalankan jika server legacy hidup.
- Jangan menambahkan write/destructive request ke contract default sebelum ada seed data dan cleanup strategy yang eksplisit.

## Smoke Matrix RBAC

Gunakan matrix ini saat validasi manual atau saat memperluas contract test:

| Role | Fokus smoke test | Catatan |
| --- | --- | --- |
| `SUPERADMIN` | Semua list endpoint always-on, delete-only permission, gated admin endpoint saat flag aktif | Role paling cocok untuk contract default dan parity awal. |
| `ADMIN` | CRUD admin tanpa delete superadmin-only, users, academic, students, enrollments, grades, teachers | Cocok untuk smoke operasional harian. |
| `TEACHER` | Read-only academic/student/grade/calendar, teacher self endpoint, reports JSON | Jangan gunakan untuk endpoint admin-only seperti users list atau teachers list. |

## Cutover Readiness

Jangan tandai cutover selesai sebelum semua poin ini terpenuhi:

- `make contract-test BASE_URL=http://localhost:8080/api/v1` pass terhadap API Go yang hidup.
- `make shadow-compare GO_BASE_URL=http://localhost:8080 LEGACY_BASE_URL=http://localhost:3000` pass terhadap backend legacy.
- Feature flags dan rollback command sudah dicatat di `docs/operations.md`.
- Rollback drill tercatat di `docs/decommission.md`.
- Legacy decommission belum dilakukan sebelum audit parity dan monitoring selesai.

## Pola Update Dokumentasi

- `README.md`: hanya index cepat, command penting, dan link dokumen.
- `docs/PROJECT_STATUS.md`: progres harian dan status pekerjaan.
- `docs/FE_BE_MAPPING.md`: mapping menu frontend ke endpoint dan status availability.
- `docs/BACKEND_MIGRATION_PLAN.md`: strategi migrasi tingkat tinggi.
- `docs/operations.md`: runbook cutover/rollback.
- `docs/decommission.md`: checklist pasca cutover.
