# Frontend ↔ Backend Gap Analysis Report

**Generated:** 2026-08-06  
**Scope:** Comparison of `admin-panel-sma` (React Admin) frontend expectations vs `sma-adp-api` (Go) backend implementation  
**Sources:** GO_BACKEND_API_SPECIFICATION.md, COMPATIBILITY_CONTRACT_MATRIX.md, backend handlers, frontend pages, data provider, features provider, custom hooks

> **Historical snapshot.** This report predates the 2026-08-09 contract
> reconciliation. Use the repository-root
> [SMA_ADMIN_API_DOCUMENTATION_GAP_REPORT.md](../SMA_ADMIN_API_DOCUMENTATION_GAP_REPORT.md)
> for the current findings and resolution status. In particular, the auth,
> roster, grade-report, and filtered-export entries below are no longer an
> accurate description of the current implementation.

## Current status (2026-08-09)

The current contract status is maintained in the repository-root
`SMA_ADMIN_API_DOCUMENTATION_GAP_REPORT.md`, the canonical API specification, and
the compatibility matrix. The focused auth, roster, grade-report, export, and
feature-flag gaps identified in this historical report have been reconciled. The
remaining release gates are seeded runtime verification, full-suite evidence,
production gateway security ownership, rollback/alerting evidence, and incomplete
portal capabilities.

The remainder of this file is retained as an archived 2026-08-06 comparison and
should not be used to infer current route or readiness status.

---

## Archived executive summary

| Category | Frontend Pages | Backend Endpoints | Status |
|----------|---------------|-------------------|--------|
| Core CRUD (always-on) | Students, Teachers, Classes, Subjects, Enrollments, Grades | Implemented & exposed | ✅ Ready |
| Feature-flagged modules | Dashboard, Scheduler, Reports, Mutations, Archives, Calendar, Attendance | Implemented but gated | ⚠️ Flagged |
| Analytics | Attendance Analytics (client + server), Grade Analytics | `/analytics/*` + `/grades/report` + `/dashboard` | ⚠️ Partial |
| Reports | Async report generation (jobs) | `/reports/generate`, `/reports/status/:id`, `/export/:token` | ✅ Ready (flagged) |
| Schedule Generator | Visual generator with drag-drop, proposals, conflicts | `/schedules/generator`, `/schedule/save`, `/semester-schedule` | ✅ Ready (flagged) |

**Key Finding:** The backend has implemented all major domains. The primary gaps are:
1. **Feature flag alignment** - Frontend VITE_ENABLE_* must match backend ENABLE_* flags
2. **Partial compatibility endpoints** - `/grades/report`, `/students/roster`, `/teachers/roster` lack full filter support
3. **Export limitations** - Browser CSV exports are unfiltered; filtered exports require async report jobs
4. **Analytics coverage** - Attendance analytics server aggregation exists but grade/behavior analytics are minimal

---

## Detailed Gap Matrix

### 1. Authentication & User Management

| Frontend Expectation | Backend Endpoint | Status | Notes |
|---------------------|------------------|--------|-------|
| Login | `POST /auth/login` | ✅ Implemented | JWT with access/refresh tokens |
| Get current user | `GET /auth/me` | ✅ Implemented | Returns user + role |
| Token refresh | `POST /auth/refresh` | ✅ Implemented | Axios interceptor handles queueing |
| Logout | `POST /auth/logout` | ✅ Implemented | Clears tokens |
| User CRUD | `GET/POST/PATCH/DELETE /users` | ✅ Implemented | RBAC protected |
| User roles | SUPERADMIN, ADMIN_TU, WALI_KELAS, GURU_MAPEL, KEPALA_SEKOLAH, SISWA, ORTU | ✅ Implemented | 7 roles defined |

**Gap:** None - auth is fully implemented and integrated.

---

### 2. Academic Management (Terms, Subjects, Classes, Class-Subjects)

| Frontend Expectation | Backend Endpoint | Status | Notes |
|---------------------|------------------|--------|-------|
| Terms CRUD | `GET/POST/PATCH /terms` | ✅ Implemented | Active term management |
| Subjects CRUD | `GET/POST /subjects` | ✅ Implemented | Track-based (IPA/IPS), groups (CORE/DIFFERENTIATED/ELECTIVE) |
| Classes CRUD | `GET/POST /classes` | ✅ Implemented | Homeroom assignment, level/track |
| Class-Subject mappings | `GET /class-subjects` | ✅ Implemented | Filter by class/subject/teacher/term |
| Class students | `GET /classes/:id/students` | ✅ Implemented | |
| Class subjects | `GET /classes/:id/subjects` | ✅ Implemented | |

**Gap:** None - academic core is complete.

---

### 3. Student Management

| Frontend Expectation | Backend Endpoint | Status | Notes |
|---------------------|------------------|--------|-------|
| Student roster (compat) | `GET /students/roster` | ⚠️ Partial | Supports: search, classId, active, pagination, sort. **Missing:** gender, track, guardian, birthYear filters |
| Student detail | `GET /students/:id` | ✅ Implemented | |
| Student CRUD | `POST/PATCH/DELETE /students` | ✅ Implemented | Soft delete |
| Student status toggle | `PATCH /students/:id/status` | ✅ Implemented | |
| CSV Import | `POST /students/import` | ✅ Implemented | 5MB/10k rows, idempotency key, audit trail |

**Gap:** `/students/roster` compatibility endpoint lacks documented rich filters (gender, track, guardian, birthYear). Frontend may expect these.

---

### 4. Teacher Management

| Frontend Expectation | Backend Endpoint | Status | Notes |
|---------------------|------------------|--------|-------|
| Teacher roster (compat) | `GET /teachers/roster` | ⚠️ Partial | Supports: search, active, pagination, sort. **Missing:** subject, track, availability, homeroom filters |
| Teacher CRUD | `POST/PATCH /teachers` | ✅ Implemented | |
| Teacher status toggle | `PATCH /teachers/:id/status` | ✅ Implemented | |
| Teacher assignments | `GET /teachers/:id/assignments` | ✅ Implemented | |
| CSV Import | `POST /teachers/import` | ✅ Implemented | 5MB/10k rows, idempotency key |

**Gap:** `/teachers/roster` compatibility endpoint lacks documented rich filters.

---

### 5. Grade Management

| Frontend Expectation | Backend Endpoint | Status | Notes |
|---------------------|------------------|--------|-------|
| Grade list (simple) | `GET /grades` | ⚠️ Partial | Supports: enrollmentId, subjectId, componentId. **Missing:** teacherId, scoreMin, scoreMax |
| Grade report (compat) | `GET /grades/report` | ⚠️ Partial | Supports: subjectId, componentId. **Missing:** term, class, teacher, status, score-range, search, sorting, rich filter metadata |
| Grade CRUD | `POST/PUT/PATCH/DELETE /grades` | ✅ Implemented | Upsert semantics, soft delete with final-grade recalc |
| Grade bulk upsert | `POST /grades/bulk` | ✅ Implemented | |
| Grade recalculation | `POST /grades/recalculate` | ✅ Implemented | |
| Grade finalization | `POST /grades/finalize` | ✅ Implemented | |
| Grade components CRUD | `GET/POST/PUT/DELETE /grade-components` | ✅ Implemented | Soft delete preserves history |
| Grade configs | `GET/POST /grade-configs` | ✅ Implemented | |

**Gap:** The `/grades/report` compatibility endpoint (used by GradesPage) has significantly fewer filters than documented in the spec. Frontend filter UI includes termId, classId, subjectId, componentId, teacherId, status, scoreMin, scoreMax, search, sort - but backend only implements subjectId and componentId.

---

### 6. Attendance Management

| Frontend Expectation | Backend Endpoint | Status | Notes |
|---------------------|------------------|--------|-------|
| Attendance list | `GET /attendance` | ✅ Implemented | classId, subjectId, teacherId, studentId, date, dateFrom, dateTo, status, slot, pagination |
| Attendance create | `POST /attendance` | ✅ Implemented | |
| Attendance bulk | `POST /attendance/bulk` | ✅ Implemented | |
| Attendance summary | `GET /attendance/summary` | ✅ Implemented | |
| Daily attendance (canonical) | `POST/GET /attendance/daily`, `/attendance/daily/bulk` | ✅ Implemented | |
| Subject attendance (canonical) | `POST/GET /attendance/subject`, `/attendance/subject/bulk`, `/attendance/subject/summary` | ✅ Implemented | |
| Legacy upsert (compat) | `POST/PUT/PATCH /attendance`, `/attendance/:id` | ✅ Implemented | Always-on |

**Gap:** None - the legacy compatibility endpoints (`/attendance`, `/attendance/:id`) are now always-on core routes.

---

### 7. Schedule Management

| Frontend Expectation | Backend Endpoint | Status | Notes |
|---------------------|------------------|--------|-------|
| Schedule CRUD | `GET/POST/PUT/DELETE /schedules` | ✅ Implemented | ListByClass, ListByTeacher, BulkCreate |
| Semester schedule slots | `GET /semester-schedule` | ⚠️ Feature-flagged | Requires `ENABLE_SCHEDULER` |
| Schedule generator (legacy) | `POST /schedule/generate` | ⚠️ Feature-flagged (legacy) | Minimal payload, deprecated |
| Schedule generator (canonical) | `POST /schedules/generator` | ⚠️ Feature-flagged | Full payload with conflicts/stats |
| Schedule save | `POST /schedule/save` | ⚠️ Feature-flagged | Saves by proposalId |
| Teacher preferences | `GET/POST /teacher-preferences`, `PUT /teacher-preferences/:id` | ✅ Implemented | Compatibility upsert |

**Gap:** All schedule generator endpoints require `ENABLE_SCHEDULER=true`. Frontend `ScheduleGeneratorPage` uses `/schedules/generator` and `/schedule/save` via `useScheduleGenerator` hook.

---

### 8. Dashboard & Analytics

| Frontend Expectation | Backend Endpoint | Status | Notes |
|---------------------|------------------|--------|-------|
| Principal dashboard | `GET /dashboard` | ⚠️ Feature-flagged | Requires `ENABLE_DASHBOARD` |
| Dashboard academics alias | `GET /dashboard/academics` | ⚠️ Feature-flagged | Alias |
| Analytics (standalone) | `GET /analytics/*` | ⚠️ Feature-flagged | Requires `ENABLE_ANALYTICS` |
| Attendance analytics (server) | `GET /analytics/attendance` | ⚠️ Feature-flagged | Requires `ENABLE_ANALYTICS`; used by `use-attendance-analytics-server` hook |
| Grade analytics | Not implemented | ❌ Missing | Frontend expects distribution/outliers/remedial via `/grades/report` |
| Behavior analytics | Not implemented | ❌ Missing | |

**Gap:** 
- Dashboard requires BOTH `ENABLE_DASHBOARD` (frontend) AND `ENABLE_DASHBOARD` (backend)
- Attendance analytics server aggregation is implemented but requires `ENABLE_ANALYTICS` 
- No dedicated grade/behavior analytics endpoints - frontend derives from `/grades/report` and local data

---

### 9. Reports & Export

| Frontend Expectation | Backend Endpoint | Status | Notes |
|---------------------|------------------|--------|-------|
| Report generation (async) | `POST /reports/generate` | ⚠️ Feature-flagged | Requires `ENABLE_REPORTS`; supports template field |
| Report status polling | `GET /reports/status/:id` | ⚠️ Feature-flagged | |
| Report download | `GET /export/:token` | ⚠️ Feature-flagged | Signed token download |
| Browser CSV exports (compat) | `GET /export/students`, `/export/grades`, `/export/attendance` | ✅ Implemented | **Unfiltered only** - no query params supported |
| Student report card | `GET /reports/students/:id` | ✅ Implemented | Canonical, not feature-flagged |
| Class report | `GET /reports/classes/:id` | ✅ Implemented | Canonical |

**Gap:** 
- All async report endpoints require `ENABLE_REPORTS=true`
- Browser CSV exports are unfiltered - frontend's "Cetak Laporan" tab uses local table data, not server-filtered exports
- Reports page integrates with async job flow (generate → poll status → download)

---

### 10. Calendar & Events

| Frontend Expectation | Backend Endpoint | Status | Notes |
|---------------------|------------------|--------|-------|
| Calendar events CRUD | `GET/POST/PATCH/DELETE /calendar-events` | ✅ Implemented | |
| Exam events (compat) | `GET/POST/PATCH/DELETE /exam-events` | ✅ Implemented | Alias of calendar-events |

**Gap:** None - calendar is fully implemented (not feature-flagged).

---

### 11. Announcements & Behavior Notes

| Frontend Expectation | Backend Endpoint | Status | Notes |
|---------------------|------------------|--------|-------|
| Announcements CRUD | `GET/POST/PATCH /announcements` | ✅ Implemented | Target audience, priority, published/expiry |
| Behavior notes CRUD | `GET/POST /behavior-notes` | ✅ Implemented | Type (POSITIVE/NEGATIVE/NEUTRAL), category |

**Gap:** Neither announcements nor behavior notes have dedicated frontend pages in the current codebase.

---

### 12. Mutations (Student Transfers)

| Frontend Expectation | Backend Endpoint | Status | Notes |
|---------------------|------------------|--------|-------|
| Mutations list | `GET /mutations` | ⚠️ Feature-flagged | Requires `ENABLE_MUTATIONS` |
| Mutation create | `POST /mutations` | ⚠️ Feature-flagged | |
| Mutation detail | `GET /mutations/:id` | ⚠️ Feature-flagged | |
| Mutation approve | `PATCH /mutations/:id/approve` | ⚠️ Feature-flagged | Added approve/reject alias endpoints |
| Mutation reject | `PATCH /mutations/:id/reject` | ⚠️ Feature-flagged | Added approve/reject alias endpoints |

**Gap:** 
- All mutation endpoints require `ENABLE_MUTATIONS=true`
- ~~Frontend `MutationsPage` uses `/mutations` resource and calls `PATCH /mutations/:id/approve` and `/mutations/:id/reject` via dataProvider.custom - but backend has `PATCH /mutations/:id/review` with a decision payload. **Endpoint mismatch!**~~
- **Fixed**: Backend now exposes `PATCH /mutations/:id/approve` and `PATCH /mutations/:id/reject` as aliases that accept `{comment}` payload and internally call the review service.

---

### 13. Archives (Document Management)

| Frontend Expectation | Backend Endpoint | Status | Notes |
|---------------------|------------------|--------|-------|
| Archives list | `GET /archives` | ⚠️ Feature-flagged | Requires `ENABLE_ARCHIVES` |
| Archive upload | `POST /archives/upload` (multipart) | ⚠️ Feature-flagged | |
| Archive detail + download URL | `GET /archives/:id` | ⚠️ Feature-flagged | Returns downloadURL |
| Archive download | `GET /archives/:id/download` | ⚠️ Feature-flagged | Signed token |
| Archive delete | `DELETE /archives/:id` | ⚠️ Feature-flagged | Soft delete |

**Gap:** All archive endpoints require `ENABLE_ARCHIVES=true`. Frontend `ArchivesPage` uses all these endpoints.

---

### 14. Enrollments

| Frontend Expectation | Backend Endpoint | Status | Notes |
|---------------------|------------------|--------|-------|
| Enrollments list | `GET /enrollments` | ✅ Implemented | |
| Enrollment create | `POST /enrollments` | ✅ Implemented | |
| Enrollment update (compat) | `PUT /enrollments/:id` | ✅ Implemented | Accepts `class_id` or legacy `target_class_id` |

**Gap:** None - enrollments are fully implemented.

---

### 15. Feature Discovery

| Frontend Expectation | Backend Endpoint | Status | Notes |
|---------------------|------------------|--------|-------|
| Runtime feature flags | `GET /features` | ✅ Implemented | Unauthenticated, returns `{apiPrefix, env, features: {...}}` |

**Integration:** Frontend `fetchFeatures()` calls this at startup, merges with build-time `VITE_ENABLE_*` flags. This is the primary mechanism for feature gating.

**Gap:** Frontend feature names must map to backend feature keys:
| Frontend | Backend |
|----------|---------|
| dashboard | dashboard |
| calendar | calendarAlias |
| attendance | attendanceAlias |
| homerooms | homerooms |
| settings | configuration |
| schedules | scheduler |
| mutations | mutations |
| archives | archives |
| reports | reports |
| documents | documents |
| audit | audit |

---

## Critical Mismatches Requiring Action

### 1. Mutation Approve/Reject Endpoint Mismatch
- **Frontend:** `PATCH /mutations/:id/approve` and `PATCH /mutations/:id/reject` with `{comment}`
- **Backend:** `PATCH /mutations/:id/review` with `{decision: "APPROVE"|"REJECT", comment}`
- **Fix:** Update frontend to use `/review` endpoint or add approve/reject aliases in backend

### 2. Grades Report Filter Incompleteness
- **Frontend GradesPage** sends: termId, classId, subjectId, componentId, teacherId, status, scoreMin, scoreMax, search, sortField, sortOrder
- **Backend `/grades/report`** only reads: subjectId, componentId
- **Fix:** Implement remaining filters in `GradeHandler.Report` or document as unsupported

### 3. Student/Teacher Roster Missing Filters
- **Documented filters** (gender, track, guardian, birthYear, subject, availability, homeroom) not implemented
- **Fix:** Implement filters or update compatibility contract matrix

### 4. Feature Flag Dependencies
| Frontend Page | Required Backend Flag | Required Frontend Flag |
|---------------|----------------------|------------------------|
| Dashboard | `ENABLE_DASHBOARD` + `ENABLE_ANALYTICS` | `VITE_ENABLE_DASHBOARD` + `VITE_ENABLE_ANALYTICS` |
| Schedule Generator | `ENABLE_SCHEDULER` | `VITE_ENABLE_SCHEDULER` |
| Reports | `ENABLE_REPORTS` | `VITE_ENABLE_REPORTS` |
| Mutations | `ENABLE_MUTATIONS` | `VITE_ENABLE_MUTATIONS` |
| Archives | `ENABLE_ARCHIVES` | `VITE_ENABLE_ARCHIVES` |
| Attendance (legacy) | `ENABLE_ATTENDANCE_ALIAS` | `VITE_ENABLE_ATTENDANCE_ALIAS` |
| Calendar (compat) | `ENABLE_CALENDAR_ALIAS` | `VITE_ENABLE_CALENDAR_ALIAS` |
| Attendance Analytics (server) | `ENABLE_ANALYTICS` | `VITE_ENABLE_ANALYTICS` (page gate) |
| Analytics API | `ENABLE_ANALYTICS` | `VITE_ENABLE_ANALYTICS` |

**Note:** `ENABLE_ANALYTICS` now has a standalone Vite flag: `VITE_ENABLE_ANALYTICS`. Dashboard requires both `ENABLE_DASHBOARD` + `ENABLE_ANALYTICS` / `VITE_ENABLE_DASHBOARD` + `VITE_ENABLE_ANALYTICS`. Attendance analytics page requires `ENABLE_ATTENDANCE_ALIAS` + `ENABLE_ANALYTICS` / `VITE_ENABLE_ATTENDANCE_ALIAS` + `VITE_ENABLE_ANALYTICS`.

### 5. Export Limitations
- Frontend "Cetak Laporan" (export tab) uses local `downloadCsv()` on currently loaded rows
- Backend async reports (`/reports/generate`) support templates (simple, detailed, landscape) and filtered output
- **Gap:** Frontend doesn't use server-side filtered exports for grades/attendance

### 6. Attendance Analytics Dual Path
- **Client-side:** `use-attendance-analytics` hook - loads all data to browser, computes locally
- **Server-side:** `use-attendance-analytics-server` hook - calls `GET /analytics/attendance`
- **Gap:** Server path requires `ENABLE_ANALYTICS`; client path works with just `VITE_ENABLE_ATTENDANCE_ALIAS` but fetches 5000+ records

---

## Recommended Action Plan

### Immediate (Blocking)
1. **Fix mutation approve/reject endpoint mismatch** - Either add aliases in backend or update frontend to use `/review`
2. **Verify feature flag defaults** - Ensure `ENABLE_*` and `VITE_ENABLE_*` defaults match deployment intent

### Short-term (Integration)
3. **Complete `/grades/report` filters** - Add termId, classId, teacherId, status, scoreMin, scoreMax, search, sort support
4. **Implement roster missing filters** - Or explicitly document as unsupported in compatibility matrix
5. **Add filtered export support** - Either extend browser CSV endpoints or integrate Reports page with grade/attendance report types

### Medium-term (Completeness)
6. **Add grade/behavior analytics endpoints** - If dashboard needs richer analytics
7. **Document canonical vs compatibility endpoints** - Clear migration path for frontend
8. **Add announcements/behavior notes frontend pages** - Backend exists but no UI

---

## Feature Flag Deployment Checklist

When deploying with specific modules enabled, verify BOTH sides:

```bash
# Backend (sma-adp-api)
ENABLE_DASHBOARD=true
ENABLE_ANALYTICS=true        # Required for dashboard analytics + attendance analytics server + analytics API
ENABLE_SCHEDULER=true        # Schedule generator + semester-schedule
ENABLE_REPORTS=true          # Async report generation
ENABLE_MUTATIONS=true        # Student mutations
ENABLE_ARCHIVES=true         # Document archives
ENABLE_HOMEROOMS=true        # Homeroom endpoints
ENABLE_CONFIGURATION_API=true # Configuration endpoints
ENABLE_CALENDAR_ALIAS=true   # /calendar compatibility
ENABLE_ATTENDANCE_ALIAS=true # /attendance compatibility + attendance pages

# Frontend (admin-panel-sma)
VITE_ENABLE_DASHBOARD=true
VITE_ENABLE_ANALYTICS=true   # Required for dashboard + attendance analytics + analytics API
VITE_ENABLE_SCHEDULER=true
VITE_ENABLE_REPORTS=true
VITE_ENABLE_MUTATIONS=true
VITE_ENABLE_ARCHIVES=true
VITE_ENABLE_HOMEROOMS=true
VITE_ENABLE_CONFIGURATION_API=true
VITE_ENABLE_CALENDAR_ALIAS=true
VITE_ENABLE_ATTENDANCE_ALIAS=true  # Gates attendance-daily, attendance-lesson
```

**Critical:** `VITE_ENABLE_ATTENDANCE_ALIAS` gates the entire attendance module (3 pages). `ENABLE_ATTENDANCE_ALIAS` gates the legacy compatibility endpoints. They must be toggled together.

---

## Appendix: Frontend Page → Backend Endpoint Mapping

| Frontend Page | Primary Resources | Backend Endpoints | Feature Flags |
|---------------|-------------------|-------------------|---------------|
| students (roster) | students | `GET /students/roster` | Always-on |
| teachers (roster) | teachers | `GET /teachers/roster` | Always-on |
| classes | classes, class-subjects | `GET/POST /classes`, `GET /class-subjects` | Always-on |
| subjects | subjects | `GET/POST /subjects` | Always-on |
| enrollments | enrollments | `GET/POST/PUT /enrollments` | Always-on |
| grades | grades, grade-components, grade-configs | `GET /grades/report`, `GET/POST/PUT/DELETE /grades`, `/grade-components`, `/grade-configs` | Always-on |
| attendance-daily | attendance | `POST/PUT/PATCH /attendance` (compat) | `ENABLE_ATTENDANCE_ALIAS` / `VITE_ENABLE_ATTENDANCE_ALIAS` |
| attendance-lesson | attendance | `POST/PUT/PATCH /attendance` (compat) | `ENABLE_ATTENDANCE_ALIAS` / `VITE_ENABLE_ATTENDANCE_ALIAS` |
| attendance-analytics | attendance + analytics | `GET /attendance` (client), `GET /analytics/attendance` (server) | `ENABLE_ATTENDANCE_ALIAS` + `ENABLE_ANALYTICS` / `VITE_ENABLE_ATTENDANCE_ALIAS` + `VITE_ENABLE_ANALYTICS` |
| schedule-generator | schedules, semester-schedule, teacher-preferences | `POST /schedules/generator`, `POST /schedule/save`, `GET /semester-schedule` | `ENABLE_SCHEDULER` / `VITE_ENABLE_SCHEDULER` |
| reports | reports | `POST /reports/generate`, `GET /reports/status/:id`, `GET /export/:token` | `ENABLE_REPORTS` / `VITE_ENABLE_REPORTS` |
| mutations | mutations | `GET/POST /mutations`, `PATCH /mutations/:id/review` | `ENABLE_MUTATIONS` / `VITE_ENABLE_MUTATIONS` |
| archives | archives | `GET/POST /archives`, `GET /archives/:id/download`, `DELETE /archives/:id` | `ENABLE_ARCHIVES` / `VITE_ENABLE_ARCHIVES` |
| dashboard | dashboard, analytics | `GET /dashboard`, `GET /analytics/*` | `ENABLE_DASHBOARD` + `ENABLE_ANALYTICS` / `VITE_ENABLE_DASHBOARD` + `VITE_ENABLE_ANALYTICS` |
| configuration | settings | `GET/POST /configuration` | `ENABLE_CONFIGURATION_API` / `VITE_ENABLE_CONFIGURATION_API` |
| homerooms | homerooms | `GET/POST /homerooms` | `ENABLE_HOMEROOMS` / `VITE_ENABLE_HOMEROOMS` |

---

**End of Report**
