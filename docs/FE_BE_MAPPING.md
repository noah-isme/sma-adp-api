# Frontend ↔ Backend Mapping

Referensi cepat jalur menu frontend ke endpoint backend canonical.

Sumber kebenaran:

- Route runtime: `cmd/api-gateway/main.go`
- Kontrak publik: `api/swagger/swagger.json`
- Status progres: `docs/PROJECT_STATUS.md`

| Menu FE | Backend Endpoint | Status | Catatan |
| --- | --- | --- | --- |
| Auth → Login | `POST /auth/login` | Always-on | Public. |
| Auth → Session | `GET /auth/me`, `POST /auth/refresh`, `POST /auth/logout` | Always-on | `me` dan `logout` butuh JWT. |
| Auth → Password | `POST /auth/change-password`, `POST /auth/forgot-password`, `POST /auth/reset-password` | Always-on | `change-password` butuh JWT. |
| Admin → Users | `GET/POST /users`, `GET/PUT/DELETE /users/{id}` | Always-on | Admin/Superadmin; self access untuk get/update user sendiri. |
| Dashboard / Admin Overview | `GET /dashboard` | Feature-flagged | `ENABLE_DASHBOARD=true`. |
| Dashboard / Academic Snapshot | `GET /dashboard/academics` | Feature-flagged | `ENABLE_DASHBOARD=true`. |
| Akademik → Semester | `GET/POST /terms`, `GET /terms/active`, `POST /terms/set-active`, `PUT/DELETE /terms/{id}` | Always-on | CRUD term canonical. |
| Akademik → Mata Pelajaran | `GET/POST /subjects`, `GET/PUT/DELETE /subjects/{id}` | Always-on | CRUD subject canonical. |
| Akademik → Kelas | `GET/POST /classes`, `GET/PUT/DELETE /classes/{id}` | Always-on | CRUD class canonical. |
| Akademik → Mapel Kelas | `GET/POST /classes/{id}/subjects` | Always-on | Mapping class-subject. |
| Akademik → Jadwal CRUD | `GET/POST /schedules`, `POST /schedules/bulk`, `PUT/DELETE /schedules/{id}` | Always-on | CRUD schedule manual. |
| Akademik → Jadwal → Generator | `POST /schedules/generator` | Feature-flagged | `ENABLE_SCHEDULER=true`; alias FE. |
| Akademik → Jadwal → Preferences | `GET /schedules/preferences`, `POST /schedules/preferences` | Always-on | Admin/Superadmin. |
| Akademik → Jadwal → Simpan Proposal | `POST /schedule/save` | Feature-flagged | `ENABLE_SCHEDULER=true`; legacy low-level. |
| Akademik → Semester Schedule | `GET /semester-schedule`, `GET /semester-schedule/{id}/slots`, `DELETE /semester-schedule/{id}` | Feature-flagged | `ENABLE_SCHEDULER=true`. |
| Akademik → Kalender CRUD | `GET/POST /calendar-events`, `GET/PUT/DELETE /calendar-events/{id}` | Always-on | CRUD canonical. |
| Akademik → Kalender FE Alias | `GET /calendar` | Legacy alias | `ENABLE_CALENDAR_ALIAS=true`. |
| Siswa → Data Siswa | `GET/POST /students`, `GET/PUT/DELETE /students/{id}` | Always-on | CRUD student canonical. |
| Siswa → Enrollment | `GET/POST /enrollments`, `PUT /enrollments/{id}/transfer`, `DELETE /enrollments/{id}` | Always-on | Enrollment and transfer. |
| Penilaian → Komponen | `GET/POST /grade-components` | Always-on | Component config. |
| Penilaian → Konfigurasi | `GET/POST /grade-configs`, `GET/PUT /grade-configs/{id}`, `POST /grade-configs/{id}/finalize` | Always-on | Grade config scope. |
| Penilaian → Input Nilai | `GET/POST /grades`, `POST /grades/bulk`, `POST /grades/recalculate`, `POST /grades/finalize` | Always-on | Teacher/Admin/Superadmin per route. |
| Penilaian → Rapor | `GET /reports/students/{id}`, `GET /reports/classes/{id}` | Always-on | JSON report card/class report. |
| Penilaian → Async Export | `POST /reports/generate`, `GET /reports/status/{id}`, `GET /export/{token}` | Feature-flagged | `ENABLE_REPORTS=true`. |
| Kehadiran → Ringkasan | `GET /attendance` | Feature-flagged | `ENABLE_ATTENDANCE_ALIAS=true`; FE alias. |
| Kehadiran → Harian | `GET /attendance/daily` | Feature-flagged | `ENABLE_ATTENDANCE_ALIAS=true`; FE alias. |
| Komunikasi → Pengumuman | `GET/POST /announcements`, `GET/PUT/DELETE /announcements/{id}` | Always-on | CRUD announcement. |
| Komunikasi → Catatan Perilaku | `GET/POST /behavior-notes`, `PUT/DELETE /behavior-notes/{id}`, `GET /students/{id}/behavior-summary` | Always-on | Behavior notes and summary. |
| Guru → Data Guru | `GET/POST /teachers`, `GET/PUT/DELETE /teachers/{id}` | Always-on | CRUD teacher. |
| Guru → Assignment | `GET/POST /teachers/{id}/assignments`, `DELETE /teachers/{id}/assignments/{aid}` | Always-on | Assignment teacher-class-subject-term. |
| Guru → Preferences/Jadwal | `GET/PUT /teachers/{id}/preferences`, `GET /teachers/{id}/schedules` | Always-on | Teacher preference and schedules. |
| Homeroom | `GET/POST /homerooms`, `GET /homerooms/{classId}` | Feature-flagged | `ENABLE_HOMEROOMS=true`. |
| Analytics | `GET /analytics/attendance`, `/grades`, `/behavior`, `/system` | Feature-flagged | `ENABLE_ANALYTICS=true`. |
| Mutasi → Workflow | `GET/POST /mutations`, `GET /mutations/{id}`, `POST /mutations/{id}/review` | Feature-flagged | `ENABLE_MUTATIONS=true`. |
| Arsip → Manajemen Arsip | `GET/POST /archives`, `GET/DELETE /archives/{id}` | Feature-flagged | `ENABLE_ARCHIVES=true`. |
| Arsip → Download Arsip | `GET /archives/{id}/download` | Feature-flagged | Signed URL/token flow. |
| Konfigurasi Sistem | `GET /configuration`, `GET/PUT /configuration/{key}`, `PUT /configuration/bulk` | Feature-flagged | `ENABLE_CONFIGURATION_API=true`. |

Status `Pending verification` harus dipakai di dokumen ini jika endpoint sudah ditambahkan tetapi belum lulus `go test`, `go vet`, build gateway, dan minimal smoke test terkait.
