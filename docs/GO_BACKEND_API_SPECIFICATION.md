# 🚀 Go Backend API Specification

> **Admin Panel SMA - Complete API Endpoints Documentation**  
> Version: 1.2.0
> Last Updated: 2026-08-02
> Target: Go + Fiber/Gin + PostgreSQL + Redis

---

## 📋 Table of Contents

1. [Authentication & Authorization](#1-authentication--authorization)
2. [User Management](#2-user-management)
3. [Compatibility & Runtime Flags](#3-compatibility--runtime-flags)
4. [Academic Management](#4-academic-management)
5. [Student Management](#5-student-management)
6. [Teacher Management](#6-teacher-management)
7. [Class Management](#7-class-management)
8. [Subject Management](#8-subject-management)
9. [Grade Management](#9-grade-management)
10. [Attendance Management](#10-attendance-management)
11. [Schedule Management](#11-schedule-management)
12. [Dashboard & Analytics](#12-dashboard--analytics)
13. [Reports & Export](#13-reports--export)
14. [Calendar & Events](#14-calendar--events)
15. [Announcements](#15-announcements)
16. [Behavior Notes](#16-behavior-notes)
17. [Mutations & Archives](#17-mutations--archives)

---

## 📦 Response Envelope & Field Naming (IMPORTANT)

The implemented Go backend (`sma-adp-api`) wraps **every** JSON success response in a common envelope and uses **snake_case** field names throughout. The per-endpoint examples below were written before the NestJS → Go migration and show **camelCase** fields at the top level — treat them as illustrative of the payload shape only. The actual contract is:

- **Success:** `{ "data": <resource or array>, "pagination": {...}, "meta": {...} }` (see `pkg/response/response.go`).
- **Error:** `{ "error": { "code": "...", "message": "...", ... } }`.
- Field names are **snake_case**: `access_token`, `refresh_token`, `expires_in`, `full_name`, `created_at`, etc.
- `POST`/`PUT`/`PATCH` create/update return `201`/`200` with the resource inside `data`; `DELETE` and password changes return `204 No Content` (empty body).

Example — `POST /api/v1/auth/login` actual response:

```json
{
  "data": {
    "access_token": "eyJhbGciOiJIUzI1NiIs...",
    "refresh_token": "eyJhbGciOiJIUzI1NiIs...",
    "expires_in": 3600,
    "issued_at": "2026-07-12T00:00:00Z",
    "user": {
      "id": "user_123",
      "email": "admin@harapannusantara.sch.id",
      "full_name": "Admin Tata Usaha",
      "role": "ADMIN_TU"
    }
  }
}
```

Canonical live examples are in the repository root `README.md` ("Contoh curl endpoint utama"), the generated Swagger served at `/docs`, and the [compatibility contract matrix](COMPATIBILITY_CONTRACT_MATRIX.md). Swagger and this specification cover the complete core-resource API surface.

---

## 🔐 1. Authentication & Authorization

### POST /api/v1/auth/login

**Login dengan email dan password**

**Request:**

```json
{
  "email": "admin@harapannusantara.sch.id",
  "password": "Admin123!"
}
```

**Response (200):**

```json
{
  "data": {
    "access_token": "eyJhbGciOiJIUzI1NiIs...",
    "refresh_token": "eyJhbGciOiJIUzI1NiIs...",
    "expires_in": 3600,
    "issued_at": "2026-07-12T00:00:00Z",
    "user": {
      "id": "user_123",
      "email": "admin@harapannusantara.sch.id",
      "full_name": "Admin Tata Usaha",
      "role": "ADMIN_TU"
    }
  }
}
```

**Errors:**

- `401`: Invalid credentials
- `422`: Validation error

---

### GET /api/v1/auth/me

**Get current authenticated user**

**Headers:**

```
Authorization: Bearer {accessToken}
```

**Response (200):**

```json
{
  "data": {
    "id": "user_123",
    "email": "admin@harapannusantara.sch.id",
    "full_name": "Admin Tata Usaha",
    "role": "ADMIN_TU"
  }
}
```

---

### POST /api/v1/auth/refresh

**Refresh access token**

**Request:**

```json
{
  "refresh_token": "eyJhbGciOiJIUzI1NiIs..."
}
```

**Response (200):**

```json
{
  "data": {
    "access_token": "eyJhbGciOiJIUzI1NiIs...",
    "refresh_token": "eyJhbGciOiJIUzI1NiIs...",
    "expires_in": 3600,
    "issued_at": "2026-07-12T00:00:00Z"
  }
}
```

---

### POST /api/v1/auth/logout

**Logout current user**

**Headers:**

```
Authorization: Bearer {accessToken}
```

**Response (200):**

```json
{
  "success": true,
  "message": "Logged out successfully"
}
```

---

## 👥 2. User Management

### GET /api/v1/users

**List all users with pagination and filters**

**Query Parameters:**

- `page` (int): Page number (default: 1)
- `perPage` (int): Items per page (default: 20, max: 100)
- `role` (string): Filter by role (ADMIN_TU, KEPALA_SEKOLAH, WALI_KELAS, GURU_MAPEL, etc.)
- `search` (string): Search by name or email
- `sort` (string): Sort field (default: fullName)
- `order` (string): Sort order (asc, desc)

**Response (200):**

```json
{
  "data": [
    {
      "id": "user_123",
      "email": "admin@harapannusantara.sch.id",
      "fullName": "Admin Tata Usaha",
      "role": "ADMIN_TU",
      "teacherId": null,
      "studentId": null,
      "classId": null,
      "createdAt": "2024-07-15T07:00:00Z",
      "updatedAt": "2024-08-20T10:30:00Z"
    }
  ],
  "total": 150,
  "page": 1,
  "perPage": 20,
  "totalPages": 8
}
```

---

### GET /api/v1/users/:id

**Get user by ID**

**Response (200):**

```json
{
  "id": "user_123",
  "email": "admin@harapannusantara.sch.id",
  "fullName": "Admin Tata Usaha",
  "role": "ADMIN_TU",
  "teacherId": null,
  "studentId": null,
  "classId": null,
  "createdAt": "2024-07-15T07:00:00Z",
  "updatedAt": "2024-08-20T10:30:00Z"
}
```

---

### POST /api/v1/users

**Create new user**

**Request:**

```json
{
  "email": "newuser@harapannusantara.sch.id",
  "password": "SecurePass123!",
  "fullName": "Guru Baru",
  "role": "GURU_MAPEL",
  "teacherId": "teacher_456",
  "studentId": null,
  "classId": null
}
```

**Response (201):**

```json
{
  "id": "user_789",
  "email": "newuser@harapannusantara.sch.id",
  "fullName": "Guru Baru",
  "role": "GURU_MAPEL",
  "teacherId": "teacher_456",
  "createdAt": "2024-11-09T13:00:00Z"
}
```

---

### PATCH /api/v1/users/:id

**Update user**

**Request:**

```json
{
  "fullName": "Guru Baru Updated",
  "role": "WALI_KELAS",
  "classId": "class_x_ipa_1"
}
```

**Response (200):**

```json
{
  "id": "user_789",
  "email": "newuser@harapannusantara.sch.id",
  "fullName": "Guru Baru Updated",
  "role": "WALI_KELAS",
  "classId": "class_x_ipa_1",
  "updatedAt": "2024-11-09T14:00:00Z"
}
```

---

### DELETE /api/v1/users/:id

**Delete user**

**Response (200):**

```json
{
  "success": true,
  "message": "User deleted successfully"
}
```

---

## 🔁 3. Compatibility & Runtime Flags

The Go API keeps the existing admin-panel contracts available while the frontend migrates to the canonical resource routes. These aliases use the same response envelope and authorization rules as their canonical handlers:

| Admin contract | Go route | Notes |
| --- | --- | --- |
| Exam events | `/exam-events` | Alias of `/calendar-events`; use `event_type` to distinguish exams. |
| Enrollment edit | `PUT /enrollments/:id` | Accepts `class_id` (or legacy `target_class_id`) and performs the validated transfer workflow. |
| Attendance write | `POST /attendance`, `PUT/PATCH /attendance/:id` | Compatibility upsert mapped to daily attendance. Canonical bulk and subject routes remain available under `/attendance/daily` and `/attendance/subject`. |
| Teacher preferences | `POST /teacher-preferences`, `PUT /teacher-preferences/:id` | Compatibility upsert; per-teacher canonical route is `PUT /teachers/:id/preferences`. |
| Student/teacher rosters | `GET /students/roster`, `GET /teachers/roster` | Admin-compatible roster responses with full filter support (gender, track, guardian, birthYearStart/End for students; subjectId, track, availability, homeroomClassId for teachers); canonical list resources remain available. |
| Grade report/edit/delete | `GET /grades/report`, `PUT/PATCH /grades/:id`, `DELETE /grades/:id` | Admin-compatible grade-list view with full filter support (termId, classId, subjectId, componentId, teacherId, status, scoreMin, scoreMax, search, sortField, sortOrder); PUT/PATCH uses the validated grade upsert payload. DELETE soft-deletes the entry, recalculates its non-finalized final grade, and is rejected after finalization. |
| Grade component edit/delete | `PUT/DELETE /grade-components/:id` | Updates an active component; DELETE soft-deletes it so historical grade/config references remain intact. |
| Browser CSV exports | `GET /export/students`, `/export/grades`, `/export/attendance` | Direct CSV downloads with filter support (classId, active, gender for students; classId, subjectId, componentId, status for grades; classId, dateFrom, dateTo for attendance); report-job token downloads remain under `/export/{token}`. |
| CSV imports | `POST /students/import`, `POST /teachers/import` | Row-level validation summary; see `FE_BE_MAPPING.md` for required columns. |

The admin data provider unwraps `data` from the response envelope and converts browser camelCase fields to the API's snake_case fields. New integrations should use the canonical snake_case contract directly.

### CSV import reliability contract

`POST /students/import` and `POST /teachers/import` process CSV rows synchronously and
return a created/failed summary. Each request is limited to 5 MiB and 10,000 data
rows. The admin sends an `Idempotency-Key`; clients that omit it receive the same
deterministic behavior from a key derived from the import type, authenticated actor,
and request body hash.

- Repeating the same key and body replays the completed result without creating rows again.
- Reusing a key with a different body returns `409 Conflict`; a concurrent request with
  the same key returns `409 Import in progress`.
- Database uniqueness rules (NIS for students and email/NIP rules for teachers) are
  reported as row failures. A duplicate does not abort valid rows in the same file.
- Processing is best-effort per row: successful rows remain committed when a later row
  fails; this is not a whole-file rollback transaction.
- Every completed request is recorded in `import_runs` and an `audit_logs` row with
  action `CSV_IMPORT`, result counts, and failure details. The idempotency key, body
  hash, actor, timestamps, and status are retained for retry/audit inspection.

These limits and behaviors are part of the compatibility contract and should be
covered by seeded import smoke tests before production readiness is declared.

### User roles and persisted relations

The API accepts exactly these seven role values:

`SUPERADMIN`, `ADMIN_TU`, `WALI_KELAS`, `GURU_MAPEL`, `KEPALA_SEKOLAH`, `SISWA`, `ORTU`.

User records persist optional authoritative links in `teacher_id`, `student_id`, and `class_id`. These fields are returned by user endpoints and are intended to connect an account to its teacher, student, or class record; they are not frontend-only metadata. The relation columns and indexes are added by migration `000015_user_relations.up.sql` (and are included in the initial schema for new installations).

### Feature flags

Optional API capabilities are disabled by default. The frontend must expose a page/resource only when its corresponding Vite flag is enabled, so disabled backend capabilities do not produce navigable 404 pages.

| Go API flag | Admin flag | Capability |
| --- | --- | --- |
| `ENABLE_DASHBOARD` | `VITE_ENABLE_DASHBOARD` | Dashboard endpoints (including dashboard analytics sections) |
| `ENABLE_SCHEDULER` | `VITE_ENABLE_SCHEDULER` | Schedule generator and preferences UI |
| `ENABLE_REPORTS` | `VITE_ENABLE_REPORTS` | Report generation/export endpoints |
| `ENABLE_MUTATIONS` | `VITE_ENABLE_MUTATIONS` | Student mutation workflows |
| `ENABLE_ARCHIVES` | `VITE_ENABLE_ARCHIVES` | Archive storage/download workflows |
| `ENABLE_HOMEROOMS` | `VITE_ENABLE_HOMEROOMS` | Homeroom endpoints |
| `ENABLE_CONFIGURATION_API` | `VITE_ENABLE_CONFIGURATION_API` | Configuration endpoints |
| `ENABLE_CALENDAR_ALIAS` | `VITE_ENABLE_CALENDAR_ALIAS` | `/calendar` compatibility alias |
| `ENABLE_ATTENDANCE_ALIAS` | `VITE_ENABLE_ATTENDANCE_ALIAS` | Attendance routes and compatibility aliases (daily, subject, generic writes, and summary) |

`ENABLE_ANALYTICS` controls the standalone `/analytics/*` API. There is no standalone
`VITE_ENABLE_ANALYTICS` page flag: the admin dashboard is gated by
`VITE_ENABLE_DASHBOARD`, while the attendance analytics screen is gated by
`VITE_ENABLE_ATTENDANCE_ALIAS` and reads the attendance resource. When dashboard
analytics are enabled, set `ENABLE_ANALYTICS` alongside `ENABLE_DASHBOARD` for the
cached analytics service; the dashboard has a repository fallback when only
`ENABLE_DASHBOARD` is enabled.

Set both flags to `true` when enabling a paired capability. `ENABLE_ANALYTICS` is the exception: it has no separate Vite flag and is a backend dependency/optimization for dashboard analytics. Omitting a flag is equivalent to `false`; canonical resources that are always registered (for example `/schedules`) remain available independently.

---

## 📚 4. Academic Management

### GET /api/v1/terms

**List academic terms**

**Response (200):**

```json
{
  "data": [
    {
      "id": "term_2024_1",
      "name": "Tahun Pelajaran 2024/2025 Semester 1",
      "year": "2024/2025",
      "semester": 1,
      "startDate": "2024-07-15",
      "endDate": "2024-12-20",
      "active": true,
      "createdAt": "2024-06-01T00:00:00Z"
    }
  ],
  "total": 2
}
```

---

### POST /api/v1/terms

**Create new term**

**Request:**

```json
{
  "name": "Tahun Pelajaran 2025/2026 Semester 1",
  "year": "2025/2026",
  "semester": 1,
  "startDate": "2025-07-15",
  "endDate": "2025-12-20",
  "active": false
}
```

---

### PATCH /api/v1/terms/:id

**Update term**

**Request:**

```json
{
  "active": true
}
```

---

## 🎓 5. Student Management

### GET /api/v1/students/roster

**Get students roster**

The handler supports all documented filters: `search`, `classId`, `active`, `gender`, `track`, `guardian`, `birthYearStart`, `birthYearEnd`, `page`/`perPage`, `sort`, `order`.

**Query Parameters:**

- `page`, `perPage`
- `classId` (string): Filter by class
- `search` (string): Search by name or NIS
- `active` (bool): Filter by active state
- `gender` (string): Filter by gender (M, F)
- `track` (string): Filter by track/program
- `guardian` (string): Filter by guardian name/phone
- `birthYearStart` (int): Filter by birth year start
- `birthYearEnd` (int): Filter by birth year end
- `sort` (string): Sort field (fullName, nis, birthDate, createdAt, gender)
- `order` (string): Sort order (asc, desc)

**Response (200):**

```json
{
  "summary": {
    "totalStudents": 300,
    "activeStudents": 285,
    "inactiveStudents": 10,
    "alumniStudents": 5,
    "activeRate": 95.0,
    "genderBreakdown": [
      { "gender": "M", "label": "Laki-laki", "count": 150 },
      { "gender": "F", "label": "Perempuan", "count": 150 }
    ],
    "classDistribution": [
      { "classId": "class_x_ipa_1", "className": "Kelas X IPA-1", "count": 30 }
    ],
    "statusBreakdown": [
      { "status": "active", "label": "Aktif", "count": 285 }
    ]
  },
  "filters": {
    "classes": [...],
    "statuses": [...],
    "genders": [...],
    "tracks": [...]
  },
  "rows": [
    {
      "id": "stu_aditya_wijaya",
      "nis": "2024001",
      "fullName": "Aditya Wijaya",
      "preferredName": "Aditya",
      "gender": "M",
      "birthDate": "2008-03-15",
      "birthPlace": "Jakarta",
      "classId": "class_x_ipa_1",
      "className": "Kelas X IPA-1",
      "classLevel": 10,
      "classTrack": "IPA",
      "homeroomId": "teacher_001",
      "homeroomName": "Pak Budi Santoso",
      "status": "active",
      "guardianName": "Ayah Aditya",
      "guardianPhone": "081234567890",
      "guardianEmail": "ayah.aditya@email.com",
      "emergencyPhone": "081234567891",
      "address": "Jl. Merdeka No. 10, Jakarta",
      "lastUpdated": "2024-11-01T08:00:00Z",
      "createdAt": "2024-07-15T08:00:00Z"
    }
  ],
  "pagination": {
    "page": 1,
    "perPage": 20,
    "total": 300,
    "totalPages": 15
  },
  "appliedFilters": {
    "classId": "class_x_ipa_1",
    "status": "active",
    "gender": "M",
    "track": "IPA",
    "guardian": "Ayah",
    "birthYearStart": 2008,
    "birthYearEnd": 2009,
    "page": 1,
    "perPage": 20,
    "sort": "fullName",
    "order": "asc"
  }
}
```

---

### GET /api/v1/students/:id

**Get student details**

**Response (200):**

```json
{
  "id": "stu_aditya_wijaya",
  "nis": "2024001",
  "fullName": "Aditya Wijaya",
  "gender": "M",
  "birthDate": "2008-03-15",
  "birthPlace": "Jakarta",
  "classId": "class_x_ipa_1",
  "status": "active",
  "guardian": "Ayah Aditya",
  "guardianPhone": "081234567890",
  "guardianEmail": "ayah.aditya@email.com",
  "address": "Jl. Merdeka No. 10, Jakarta",
  "enrollments": [...],
  "grades": [...],
  "attendance": {...}
}
```

---

### POST /api/v1/students

**Create new student**

**Request:**

```json
{
  "nis": "2024301",
  "fullName": "Siswa Baru",
  "gender": "M",
  "birthDate": "2008-05-20",
  "birthPlace": "Bandung",
  "classId": "class_x_ipa_1",
  "guardian": "Orang Tua Siswa",
  "guardianPhone": "081234567890",
  "guardianEmail": "guardian@email.com",
  "address": "Jl. Example No. 1"
}
```

---

### PATCH /api/v1/students/:id

**Update student**

### PATCH /api/v1/students/:id/status

**Update only the active status**

```json
{ "status": "active" }
```

---

### DELETE /api/v1/students/:id

**Delete student** (soft delete)

---

## 👨‍🏫 6. Teacher Management

### GET /api/v1/teachers/roster

**Get teachers roster**

The handler supports all documented filters: `search`, `active`, `subjectId`, `track`, `availability`, `homeroomClassId`, `page`/`perPage`, `sort`, `order`.

**Query Parameters:**

- `page`, `perPage`
- `search` (string): Search by name, email, or NIP
- `active` (bool): Filter by active state
- `subjectId` (string): Filter by subject/expertise
- `track` (string): Filter by track/program
- `availability` (string): Filter by availability (HIGH, MEDIUM, LOW)
- `homeroomClassId` (string): Filter by homeroom class
- `sort` (string): Sort field (fullName, email, createdAt, updatedAt)
- `order` (string): Sort order (asc, desc)

**Response (200):**

```json
{
  "summary": {
    "totalTeachers": 25,
    "activeTeachers": 23,
    "inactiveTeachers": 2,
    "homeroomTeachers": 10,
    "activeRate": 92.0,
    "subjectDistribution": [...],
    "trackDistribution": [...],
    "availabilityBreakdown": [...]
  },
  "filters": {
    "subjects": [...],
    "statuses": [...],
    "tracks": [...],
    "availabilities": [...],
    "homerooms": [...]
  },
  "rows": [
    {
      "id": "teacher_001",
      "fullName": "Pak Budi Santoso",
      "nip": "198505012010011001",
      "email": "budi.santoso@harapannusantara.sch.id",
      "phone": "081234567890",
      "status": "active",
      "mainSubjectId": "subject_mat",
      "mainSubjectName": "Matematika",
      "subjectGroup": "CORE",
      "tracks": ["IPA", "IPS"],
      "homeroomClassId": "class_x_ipa_1",
      "homeroomClassName": "Kelas X IPA-1",
      "assignmentCount": 5,
      "availability": "HIGH",
      "lastUpdated": "2024-11-01T08:00:00Z",
      "createdAt": "2024-07-01T00:00:00Z"
    }
  ],
  "pagination": {...},
  "appliedFilters": {
    "search": "budi",
    "active": true,
    "subjectId": "subject_mat",
    "track": "IPA",
    "availability": "HIGH",
    "homeroomClassId": "class_x_ipa_1",
    "page": 1,
    "perPage": 20,
    "sort": "fullName",
    "order": "asc"
  }
}
```

---

### POST /api/v1/teachers

**Create new teacher**

---

### PATCH /api/v1/teachers/:id

**Update teacher**

### PATCH /api/v1/teachers/:id/status

**Update only the active status**

```json
{ "active": false }
```

---

### GET /api/v1/teachers/:id/assignments

**Get teacher's class assignments**

**Response (200):**

```json
{
  "teacherId": "teacher_001",
  "teacherName": "Pak Budi Santoso",
  "assignments": [
    {
      "id": "cs_xipa1_mat",
      "classId": "class_x_ipa_1",
      "className": "Kelas X IPA-1",
      "subjectId": "subject_mat",
      "subjectName": "Matematika",
      "termId": "term_2024_1",
      "studentCount": 30,
      "scheduleCount": 4
    }
  ]
}
```

---

## 🏫 7. Class Management

### GET /api/v1/classes

**List all classes**

**Query Parameters:**

- `termId` (string): Filter by term
- `level` (int): 10, 11, 12
- `track` (string): IPA, IPS
- `homeroomId` (string): Filter by homeroom teacher

**Response (200):**

```json
{
  "data": [
    {
      "id": "class_x_ipa_1",
      "code": "X-IPA-1",
      "name": "Kelas X IPA-1",
      "level": 10,
      "track": "IPA",
      "homeroomId": "teacher_001",
      "homeroomName": "Pak Budi Santoso",
      "termId": "term_2024_1",
      "studentCount": 30,
      "subjectCount": 13
    }
  ],
  "total": 10
}
```

---

### POST /api/v1/classes

**Create new class**

---

### GET /api/v1/classes/:id/students

**Get students in a class**

---

### GET /api/v1/classes/:id/subjects

**Get subjects taught in a class**

---

## 📖 8. Subject Management

### GET /api/v1/subjects

**List all subjects**

**Response (200):**

```json
{
  "data": [
    {
      "id": "subject_mat",
      "code": "MAT",
      "name": "Matematika",
      "group": "CORE",
      "tracks": ["ALL"],
      "description": "Matematika Wajib"
    },
    {
      "id": "subject_mat_p",
      "code": "MAT-P",
      "name": "Matematika Peminatan",
      "group": "DIFFERENTIATED",
      "tracks": ["IPA"],
      "description": "Matematika untuk IPA"
    }
  ],
  "total": 15
}
```

---

### POST /api/v1/subjects

**Create new subject**

---

### GET /api/v1/class-subjects

**Get class-subject mappings**

**Query Parameters:**

- `classId` (string)
- `subjectId` (string)
- `teacherId` (string)
- `termId` (string)

---

## 📝 9. Grade Management

### GET /api/v1/grades/report

**Get grade report compatibility view**

The handler now supports all frontend filters: `termId`, `classId`, `subjectId`, `componentId`, `teacherId`, `status`, `scoreMin`, `scoreMax`, `search`, `sortField`, `sortOrder`, `page`, `perPage`.

**Query Parameters:**

- `termId` (string): Filter by term
- `classId` (string): Filter by class
- `subjectId` (string): Filter by subject
- `componentId` (string): Filter by component
- `teacherId` (string): Filter by teacher
- `status` (string): Filter by status (PASS, REMEDIAL, FAIL)
- `scoreMin` (number): Minimum score
- `scoreMax` (number): Maximum score
- `search` (string): Search by student name/NIS/component
- `sortField` (string): Sort field (score, studentName, subjectName, componentName, lastUpdated)
- `sortOrder` (string): Sort order (ascend, descend)
- `page` (int): Page number (default: 1)
- `perPage` (int): Page size (default: 20)

**Response (200):**

```json
{
  "context": {
    "termId": "term_2024_1",
    "classId": "class_x_ipa_1",
    "subjectId": "subject_mat"
  },
  "summary": {
    "averageScore": 78.5,
    "belowKkmCount": 8,
    "componentCount": 5,
    "remedialCount": 5,
    "statusBreakdown": [
      { "code": "PASS", "label": "✅ Lulus", "count": 22 },
      { "code": "CAUTION", "label": "⚠️ Perlu perhatian", "count": 3 },
      { "code": "REMEDIAL", "label": "❌ Remedial", "count": 5 }
    ],
    "distribution": [
      { "bucket": "90-100", "from": 90, "to": 100, "count": 5 },
      { "bucket": "80-89", "from": 80, "to": 89, "count": 10 },
      { "bucket": "70-79", "from": 70, "to": 79, "count": 10 },
      { "bucket": "60-69", "from": 60, "to": 69, "count": 3 },
      { "bucket": "0-59", "from": 0, "to": 59, "count": 2 }
    ]
  },
  "filters": {
    "terms": [...],
    "classes": [...],
    "subjects": [...],
    "components": [...],
    "teachers": [...],
    "statuses": [...]
  },
  "rows": [
    {
      "id": "grade_001",
      "studentId": "stu_aditya_wijaya",
      "studentName": "Aditya Wijaya",
      "studentNis": "2024001",
      "classId": "class_x_ipa_1",
      "className": "Kelas X IPA-1",
      "subjectId": "subject_mat",
      "subjectName": "Matematika",
      "componentId": "comp_uts_mat_xipa1",
      "componentName": "UTS Matematika",
      "componentCategory": "UTS",
      "componentWeight": 30,
      "componentDescription": "Ujian Tengah Semester",
      "score": 85,
      "kkm": 75,
      "status": {
        "code": "PASS",
        "label": "✅ Lulus",
        "description": "Nilai memenuhi atau melampaui KKM.",
        "tone": "success",
        "icon": "check"
      },
      "teacherId": "teacher_001",
      "teacherName": "Pak Budi Santoso",
      "recordedAt": "2024-10-15T08:00:00Z",
      "lastUpdated": "2024-10-16T10:00:00Z",
      "termId": "term_2024_1",
      "termName": "Tahun Pelajaran 2024/2025 Semester 1",
      "termLabel": "2024/2025 • Semester 1"
    }
  ],
  "pagination": {
    "page": 1,
    "perPage": 25,
    "total": 150,
    "totalPages": 6
  },
  "appliedFilters": {
    "termId": "term_2024_1",
    "classId": "class_x_ipa_1",
    "subjectId": "subject_mat",
    "componentId": "comp_uts_mat_xipa1",
    "teacherId": "teacher_001",
    "status": "PASS",
    "scoreMin": 70,
    "scoreMax": 100,
    "search": "aditya",
    "sortField": "score",
    "sortOrder": "descend",
    "page": 1,
    "perPage": 25
  }
}
```

---

### GET /api/v1/grades

**List grades with simple filtering**

The current handler supports `enrollmentId`, `subjectId`, and `componentId`. `teacherId`, `scoreMin`, and `scoreMax` are not currently implemented.

**Query Parameters:**

- `enrollmentId`, `componentId`, `subjectId`

**Response (200):**

```json
{
  "data": [
    {
      "id": "grade_001",
      "enrollmentId": "enrollment_001",
      "componentId": "comp_001",
      "subjectId": "subject_mat",
      "teacherId": "teacher_001",
      "score": 85,
      "recordedAt": "2024-10-15T08:00:00Z"
    }
  ],
  "total": 500
}
```

---

### POST /api/v1/grades

**Create grade entry**

**Request:**

```json
{
  "enrollmentId": "enrollment_001",
  "componentId": "comp_001",
  "subjectId": "subject_mat",
  "teacherId": "teacher_001",
  "score": 85
}
```

---

### PUT/PATCH /api/v1/grades/:id

**Update grade**

Both methods are supported. `PUT` is the backward-compatible method used by the
generic admin data provider; `PATCH` remains available for existing callers. The
canonical `grade_value` field and the legacy admin `score` field are both
accepted.

---

### DELETE /api/v1/grades/:id

**Soft-delete grade**

The grade row is retained with `deleted_at` set, omitted from normal grade
queries, and the non-finalized final grade is recalculated. Deletion is rejected
when the grade configuration or final grade is already finalized.

---

### GET /api/v1/grade-components

**List grade components**

**Query Parameters:**

- `classSubjectId` (string)
- `termId` (string)

**Response (200):**

```json
{
  "data": [
    {
      "id": "comp_uts_mat_xipa1",
      "name": "UTS Matematika",
      "description": "Ujian Tengah Semester",
      "weight": 30,
      "kkm": 75,
      "classSubjectId": "cs_xipa1_mat",
      "termId": "term_2024_1"
    }
  ],
  "total": 5
}
```

---

### POST /api/v1/grade-components

**Create grade component**

---

### PUT /api/v1/grade-components/:id

**Update grade component**

`name` is required. `code` is optional for the admin edit form; when omitted,
the existing code is preserved. Codes are normalized to uppercase and must remain
unique among active components.

---

### DELETE /api/v1/grade-components/:id

**Soft-delete grade component**

The component is marked with `deleted_at` and omitted from active component lists.
Existing grade and configuration rows remain intact, including their component labels,
for historical reporting.

---

### GET /api/v1/grade-configs

**Get grade configuration for class-subject**

---

### POST /api/v1/grade-configs

**Create/Update grade config**

---

## 📅 10. Attendance Management

### GET /api/v1/attendance

**List attendance records**

**Query Parameters:**

- `classId` (string)
- `subjectId` (string)
- `teacherId` (string)
- `studentId` (string)
- `date` (string): YYYY-MM-DD
- `dateFrom`, `dateTo` (string)
- `status` (string): H (hadir), I (izin), S (sakit), A (alpha)
- `slot` (int)
- `page`, `perPage`

**Response (200):**

```json
{
  "data": [
    {
      "id": "att_001",
      "studentId": "stu_aditya_wijaya",
      "classId": "class_x_ipa_1",
      "subjectId": "subject_mat",
      "teacherId": "teacher_001",
      "date": "2024-11-09",
      "slot": 1,
      "status": "H",
      "notes": null,
      "recordedAt": "2024-11-09T07:30:00Z",
      "recordedBy": "teacher_001",
      "updatedAt": "2024-11-09T07:30:00Z"
    }
  ],
  "total": 1000,
  "pagination": {...}
}
```

---

### POST /api/v1/attendance

**Record attendance**

**Request:**

```json
{
  "studentId": "stu_aditya_wijaya",
  "classId": "class_x_ipa_1",
  "subjectId": "subject_mat",
  "teacherId": "teacher_001",
  "date": "2024-11-09",
  "slot": 1,
  "status": "H",
  "notes": ""
}
```

---

### POST /api/v1/attendance/bulk

**Record attendance for multiple students**

**Request:**

```json
{
  "classId": "class_x_ipa_1",
  "subjectId": "subject_mat",
  "teacherId": "teacher_001",
  "date": "2024-11-09",
  "slot": 1,
  "records": [
    { "studentId": "stu_001", "status": "H" },
    { "studentId": "stu_002", "status": "H" },
    { "studentId": "stu_003", "status": "I", "notes": "Sakit" }
  ]
}
```

---

### GET /api/v1/attendance/summary

**Get attendance summary**

**Query Parameters:**

- `classId` (string)
- `studentId` (string)
- `startDate`, `endDate` (string)

**Response (200):**

```json
{
  "classId": "class_x_ipa_1",
  "studentId": "stu_aditya_wijaya",
  "period": {
    "startDate": "2024-07-15",
    "endDate": "2024-11-09"
  },
  "total": 80,
  "byStatus": {
    "H": 75,
    "I": 2,
    "S": 1,
    "A": 2
  },
  "percentage": 93.75,
  "weeklyTrend": [
    { "week": "2024-W44", "present": 5, "total": 5, "percentage": 100.0 },
    { "week": "2024-W43", "present": 4, "total": 5, "percentage": 80.0 }
  ]
}
```

---

## 🗓️ 11. Schedule Management

### GET /api/v1/schedules

**List schedules**

**Query Parameters:**

- `classId` (string)
- `subjectId` (string)
- `teacherId` (string)
- `dayOfWeek` (int): 1-5 (Senin-Jumat)
- `slot` (int)

**Response (200):**

```json
{
  "data": [
    {
      "id": "schedule_001",
      "classSubjectId": "cs_xipa1_mat",
      "dayOfWeek": 1,
      "dayName": "Senin",
      "slot": 1,
      "startTime": "07:00",
      "endTime": "08:30",
      "room": "Lab IPA 1",
      "className": "Kelas X IPA-1",
      "subjectName": "Matematika",
      "teacherName": "Pak Budi Santoso"
    }
  ],
  "total": 45
}
```

---

### GET /api/v1/semester-schedule

**Get semester schedule slots**

**Query Parameters:**

- `classId` (string)
- `termId` (string)

**Response (200):**

```json
{
  "data": [
    {
      "id": "slot_001",
      "classId": "class_x_ipa_1",
      "dayOfWeek": 1,
      "slot": 1,
      "teacherId": "teacher_001",
      "subjectId": "subject_mat",
      "status": "PREFERENCE",
      "locked": false
    }
  ],
  "total": 30
}
```

---

### POST /api/v1/schedule/generate

**Generate schedule for a class**

**Request:**

```json
{
  "classId": "class_x_ipa_1",
  "termId": "term_2024_1"
}
```

**Response (200):**

```json
{
  "slots": [...],
  "summary": {
    "preferenceMatches": 20,
    "compromise": 8,
    "conflicts": 2,
    "empty": 0,
    "confidence": 66.7
  }
}
```

---

### POST /api/v1/schedule/save

**Save generated schedule**

**Request:**

```json
{
  "classId": "class_x_ipa_1",
  "slots": [...]
}
```

---

### GET /api/v1/teacher-preferences/:teacherId

**Get teacher scheduling preferences**

---

### POST /api/v1/teacher-preferences

**Create/Update teacher preferences**

---

## 📊 12. Dashboard & Analytics

### GET /api/v1/dashboard

**Get principal dashboard data**

**Response (200):**

```json
{
  "termId": "term_2024_1",
  "updatedAt": "2024-11-09T13:00:00Z",
  "distribution": {
    "overallAverage": 79.8,
    "totalStudents": 300,
    "byRange": [
      { "range": "90-100", "count": 45 },
      { "range": "80-89", "count": 105 },
      { "range": "70-79", "count": 105 },
      { "range": "60-69", "count": 36 },
      { "range": "<60", "count": 9 }
    ],
    "byClass": [
      {
        "classId": "class_x_ipa_1",
        "className": "Kelas X IPA-1",
        "average": 81.5,
        "highest": 95.0,
        "lowest": 65.0
      }
    ]
  },
  "outliers": [...],
  "remedial": [
    {
      "studentId": "stu_030",
      "studentName": "Siswa Z",
      "classId": "class_x_ipa_1",
      "className": "Kelas X IPA-1",
      "subjectId": "subject_mat",
      "subjectName": "Matematika",
      "score": 55,
      "kkm": 75,
      "attempts": 1,
      "lastAttempt": "2024-10-20"
    }
  ],
  "attendance": {
    "overall": 89.2,
    "byClass": [
      {
        "classId": "class_x_ipa_1",
        "className": "Kelas X IPA-1",
        "percentage": 93.5
      }
    ],
    "alerts": [
      {
        "classId": "class_x_ips_2",
        "className": "Kelas X IPS-2",
        "indicator": "ABSENCE_SPIKE",
        "percentage": 79.3,
        "week": "2024-W45",
        "trend": [85.0, 83.0, 81.0, 79.5, 78.0, 79.3]
      }
    ]
  }
}
```

---

### GET /api/v1/dashboard/academics

**Alias for /api/v1/dashboard**

---

## 📄 13. Reports & Export

### POST /api/v1/reports/generate

**Generate report (enqueue job)**

**Request:**

```json
{
  "type": "GRADE_REPORT",
  "format": "PDF",
  "filters": {
    "termId": "term_2024_1",
    "classId": "class_x_ipa_1"
  }
}
```

**Response (202):**

```json
{
  "jobId": "job_123",
  "status": "QUEUED",
  "message": "Report generation queued"
}
```

---

### GET /api/v1/reports/status/:jobId

**Check report generation status**

**Response (200):**

```json
{
  "jobId": "job_123",
  "status": "COMPLETED",
  "progress": 100,
  "downloadUrl": "https://storage.example.com/reports/report_123.pdf",
  "expiresAt": "2024-11-16T13:00:00Z"
}
```

---

### GET /api/v1/export/students

**Export students data (CSV with optional filters)**

This browser-compatibility endpoint streams students as `text/csv` with optional query filters. For non-CSV output or complex reports, use the asynchronous report flow under `/reports/generate` and `/export/{token}`.

**Query Parameters:**

- `classId` (string): Filter by class
- `active` (bool): Filter by active state
- `gender` (string): Filter by gender

**Response (200):** Returns a CSV file download.

---

### GET /api/v1/export/grades

**Export grades data (CSV with optional filters)**

The endpoint streams grades as `text/csv` with optional query filters.

**Query Parameters:**

- `classId` (string): Filter by class
- `subjectId` (string): Filter by subject
- `componentId` (string): Filter by component
- `status` (string): Filter by status

**Response (200):** Returns a CSV file download.

---

### GET /api/v1/export/attendance

**Export attendance data (CSV with optional filters)**

The endpoint streams daily attendance as `text/csv` with optional query filters.

**Query Parameters:**

- `classId` (string): Filter by class
- `dateFrom` (string): Filter by date from (RFC3339)
- `dateTo` (string): Filter by date to (RFC3339)

**Response (200):** Returns a CSV file download.

---

## 📆 14. Calendar & Events

### GET /api/v1/calendar-events

**List calendar events**

**Query Parameters:**

- `startDate`, `endDate` (string)
- `type` (string): HOLIDAY, SCHOOL_EVENT, EXAM, MEETING
- `termId` (string)

**Response (200):**

```json
{
  "data": [
    {
      "id": "event_001",
      "title": "Libur Hari Kemerdekaan",
      "description": "Peringatan HUT RI ke-79",
      "type": "HOLIDAY",
      "startDate": "2024-08-17T00:00:00Z",
      "endDate": "2024-08-17T23:59:59Z",
      "allDay": true,
      "location": null,
      "termId": "term_2024_1"
    }
  ],
  "total": 20
}
```

---

### POST /api/v1/calendar-events

**Create calendar event**

---

### GET /api/v1/exam-events

**List exam events**

---

### POST /api/v1/exam-events

**Create exam event**

---

## 📢 15. Announcements

### GET /api/v1/announcements

**List announcements**

**Query Parameters:**

- `targetAudience` (string): ALL, TEACHERS, STUDENTS, PARENTS
- `priority` (string): LOW, NORMAL, HIGH, URGENT
- `publishedOnly` (boolean)
- `page`, `perPage`

**Response (200):**

```json
{
  "data": [
    {
      "id": "announcement_001",
      "title": "Pengumuman Libur Semester",
      "content": "Libur semester akan dimulai...",
      "targetAudience": "ALL",
      "priority": "HIGH",
      "publishedAt": "2024-11-01T08:00:00Z",
      "expiresAt": "2024-12-20T23:59:59Z",
      "authorId": "user_123",
      "authorName": "Admin TU",
      "createdAt": "2024-10-31T10:00:00Z"
    }
  ],
  "total": 15,
  "pagination": {...}
}
```

---

### POST /api/v1/announcements

**Create announcement**

---

### PATCH /api/v1/announcements/:id

**Update announcement**

---

## 📝 16. Behavior Notes

### GET /api/v1/behavior-notes

**List behavior notes**

**Query Parameters:**

- `studentId` (string)
- `teacherId` (string)
- `type` (string): POSITIVE, NEGATIVE, NEUTRAL
- `dateFrom`, `dateTo` (string)

**Response (200):**

```json
{
  "data": [
    {
      "id": "note_001",
      "studentId": "stu_aditya_wijaya",
      "studentName": "Aditya Wijaya",
      "teacherId": "teacher_001",
      "teacherName": "Pak Budi Santoso",
      "date": "2024-11-09",
      "type": "POSITIVE",
      "category": "ACHIEVEMENT",
      "note": "Memenangkan olimpiade matematika tingkat provinsi",
      "createdAt": "2024-11-09T14:00:00Z"
    }
  ],
  "total": 50
}
```

---

### POST /api/v1/behavior-notes

**Create behavior note**

---

## 🔄 17. Mutations & Archives

### GET /api/v1/mutations

**List student mutations (transfers, graduations)**

**Query Parameters:**

- `studentId` (string)
- `type` (string): TRANSFER_IN, TRANSFER_OUT, PROMOTION, GRADUATION, DROPOUT
- `status` (string): PENDING, APPROVED, REJECTED

**Response (200):**

```json
{
  "data": [
    {
      "id": "mutation_001",
      "studentId": "stu_030",
      "studentName": "Siswa Z",
      "type": "TRANSFER_OUT",
      "status": "APPROVED",
      "fromClassId": "class_x_ipa_1",
      "toClassId": null,
      "reason": "Pindah sekolah ke Jakarta",
      "effectiveDate": "2024-12-01",
      "requestedBy": "user_123",
      "approvedBy": "user_principal",
      "approvedAt": "2024-11-05T10:00:00Z",
      "auditTrail": [...],
      "createdAt": "2024-11-01T08:00:00Z"
    }
  ],
  "total": 10
}
```

---

### POST /api/v1/mutations

**Create mutation request**

---

### PATCH /api/v1/mutations/:id/approve

**Approve mutation**

**Path Parameters:**
- `id` (string): Mutation ID

**Request Body:**
```json
{
  "comment": "Mutation approved as student has completed transfer requirements"
}
```

**Response (200):**
```json
{
  "data": {
    "id": "mutation_001",
    "studentId": "stu_030",
    "studentName": "Siswa Z",
    "type": "TRANSFER_OUT",
    "status": "APPROVED",
    "fromClassId": "class_x_ipa_1",
    "toClassId": null,
    "reason": "Pindah sekolah ke Jakarta",
    "effectiveDate": "2024-12-01",
    "requestedBy": "user_123",
    "approvedBy": "user_principal",
    "approvedAt": "2024-11-05T10:00:00Z",
    "auditTrail": [...],
    "createdAt": "2024-11-01T08:00:00Z"
  }
}
```

**Error Responses:**
- `400` - Invalid payload or mutation already reviewed
- `401` - Unauthorized
- `403` - Forbidden (requires SUPERADMIN)
- `404` - Mutation not found
- `409` - Conflict (mutation already processed)

---

### PATCH /api/v1/mutations/:id/reject

**Reject mutation**

**Path Parameters:**
- `id` (string): Mutation ID

**Request Body:**
```json
{
  "comment": "Insufficient documentation for transfer"
}
```

**Response (200):**
```json
{
  "data": {
    "id": "mutation_001",
    "studentId": "stu_030",
    "studentName": "Siswa Z",
    "type": "TRANSFER_OUT",
    "status": "REJECTED",
    "fromClassId": "class_x_ipa_1",
    "toClassId": null,
    "reason": "Pindah sekolah ke Jakarta",
    "effectiveDate": "2024-12-01",
    "requestedBy": "user_123",
    "approvedBy": null,
    "approvedAt": null,
    "auditTrail": [...],
    "createdAt": "2024-11-01T08:00:00Z"
  }
}
```

**Error Responses:**
- `400` - Invalid payload or mutation already reviewed
- `401` - Unauthorized
- `403` - Forbidden (requires SUPERADMIN)
- `404` - Mutation not found
- `409` - Conflict (mutation already processed)

---

### GET /api/v1/archives

**List archived documents**

**Query Parameters:**

- `category` (string): RAPOR, CERTIFICATE, TRANSCRIPT, PHOTO, OTHER
- `studentId` (string)
- `termId` (string)

**Response (200):**

```json
{
  "data": [
    {
      "id": "archive_001",
      "fileName": "rapor_stu_001_sem1.pdf",
      "originalName": "Rapor Semester 1.pdf",
      "category": "RAPOR",
      "fileSize": 2048576,
      "mimeType": "application/pdf",
      "url": "https://storage.example.com/archives/rapor_stu_001_sem1.pdf",
      "studentId": "stu_aditya_wijaya",
      "termId": "term_2024_1",
      "uploadedBy": "user_123",
      "uploadedAt": "2024-12-15T10:00:00Z"
    }
  ],
  "total": 100
}
```

---

### POST /api/v1/archives/upload

**Upload archive document**

**Request (multipart/form-data):**

```
file: [binary]
category: RAPOR
studentId: stu_aditya_wijaya
termId: term_2024_1
```

---

## 🔧 Additional Endpoints

### GET /api/v1/enrollments

**List student enrollments**

---

### POST /api/v1/enrollments

**Enroll student to class**

---

### GET /api/v1/health

**Health check endpoint**

**Response (200):**

```json
{
  "status": "ok",
  "timestamp": "2024-11-09T13:53:00Z",
  "version": "1.0.0",
  "database": "connected",
  "redis": "connected"
}
```

---

### GET /api/v1/version

**Get API version**

---

## 📋 Common Response Codes

| Code | Meaning                    |
| ---- | -------------------------- |
| 200  | Success                    |
| 201  | Created                    |
| 202  | Accepted (async operation) |
| 204  | No Content                 |
| 400  | Bad Request                |
| 401  | Unauthorized               |
| 403  | Forbidden                  |
| 404  | Not Found                  |
| 422  | Validation Error           |
| 429  | Too Many Requests          |
| 500  | Internal Server Error      |
| 503  | Service Unavailable        |

---

## 🔐 Authentication

All endpoints (except `/auth/login`) require JWT token:

```http
Authorization: Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...
```

---

## 🎯 Rate Limiting

- **Anonymous**: 100 requests/hour
- **Authenticated**: 1000 requests/hour
- **Admin**: 5000 requests/hour

---

## 📦 Pagination Standard

**Request:**

```
GET /api/v1/students?page=2&perPage=20
```

**Response:**

```json
{
  "data": [...],
  "total": 300,
  "page": 2,
  "perPage": 20,
  "totalPages": 15
}
```

---

## 🔍 Filtering & Sorting

**Filtering:**

```
GET /api/v1/students?status=active&classId=class_x_ipa_1
```

**Sorting:**

```
GET /api/v1/students?sort=fullName&order=asc
```

**Search:**

```
GET /api/v1/students?search=aditya
```

---

## 📝 Notes for Backend Implementation

1. **Use Database Indexes** on:

   - Foreign keys (classId, studentId, teacherId, etc.)
   - Frequently filtered fields (status, date, termId)
   - Search fields (fullName, email, nis)

2. **Implement Caching** with Redis for:

   - Dashboard data (TTL: 5 minutes)
   - Roster summaries (TTL: 10 minutes)
   - Static data (subjects, terms)

3. **Use Transactions** for:

   - Bulk attendance recording
   - Grade component creation
   - Student mutations

4. **Background Jobs** for:

   - Report generation
   - Email notifications
   - Data aggregation

5. **Validation Rules**:

   - Email format
   - Date ranges
   - Score ranges (0-100)
   - KKM ranges (0-100)
   - Required fields

6. **Audit Logging** for:

   - Grade changes
   - User actions
   - Mutation approvals

7. **File Storage**:
   - Supabase Storage or Cloudflare R2
   - Pre-signed URLs for uploads
   - Automatic file validation

---

## 🚀 Technology Stack Recommendations

**Framework Options:**

- **Fiber** (recommended): Fast, Express-like API
- **Gin**: Mature, widely used
- **Echo**: Good performance, clean API

**Database:**

- **PostgreSQL 14+**: Main database
- **pgx**: Go PostgreSQL driver

**Caching:**

- **Redis 7+**: Cache & sessions
- **go-redis**: Redis client

**ORM:**

- **GORM**: Feature-rich ORM (optional)
- **sqlx**: Lightweight SQL toolkit

**Background Jobs:**

- **Asynq**: Redis-based job queue
- **Temporal**: Complex workflows (if needed)

**File Storage:**

- **Supabase Storage SDK**: For Supabase
- **AWS SDK Go v2**: For R2/S3

**Authentication:**

- **golang-jwt/jwt**: JWT tokens
- **bcrypt**: Password hashing

**Validation:**

- **go-playground/validator**: Struct validation

**Monitoring:**

- **Prometheus**: Metrics
- **Sentry**: Error tracking

---

## 📚 References

- [Go Best Practices](https://github.com/golang-standards/project-layout)
- [REST API Design Guidelines](https://restfulapi.net/)
- [PostgreSQL Performance Tips](https://wiki.postgresql.org/wiki/Performance_Optimization)
- [Redis Caching Strategies](https://redis.io/docs/manual/patterns/)

---

**Total Endpoints: 100+**

**Last Updated:** 2026-08-08
**Document Version:** 1.2.0
**Maintained By:** Development Team
