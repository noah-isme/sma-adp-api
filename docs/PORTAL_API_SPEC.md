# Parent/Student Portal API Specification

**Base URL:** `/api/v1/portal`

**Version:** 1.0.0

**Authentication:** JWT Bearer Token with `PARENT` or `STUDENT` role

> **Implementation status (2026-08-09): Partial / not production-ready.**
> JSON responses are wrapped in the common Go `data` envelope. The admin API uses
> snake_case, while the current portal auth models still expose camelCase token
> and profile fields; the route-specific examples below record that actual
> implementation detail. Portal forgot password and reset password currently
> validate/log the request but do not send a reset email, validate a reset token,
> or change a password. Parent access checks for portal data routes and
> announcement page/limit parsing are also incomplete.

---

## Overview

The Portal API provides dedicated endpoints for parents and students to access academic information. Unlike the admin/teacher API which uses broad RBAC, the portal API enforces strict data scoping:

- **Students** can only access their own data
- **Parents** can access data for their linked children only
- All endpoints filter by audience (`STUDENT`, `PARENT`, `ALL`) appropriately

---

## Authentication Endpoints

### POST /portal/auth/login

Authenticate a parent or student user.

**Request:**
```json
{
  "email": "parent@example.com",
  "password": "SecurePass123!"
}
```

**Response (200):**
```json
{
  "data": {
    "accessToken": "eyJhbGciOiJIUzI1NiIs...",
    "refreshToken": "eyJhbGciOiJIUzI1NiIs...",
    "user": {
      "id": "usr_parent_001",
      "email": "parent@example.com",
      "fullName": "Budi Santoso",
      "role": "PARENT",
      "portalRole": "PARENT",
      "linkedStudents": [
        {
          "id": "std_001",
          "nis": "2024001",
          "fullName": "Ahmad Fauzi",
          "className": "X IPA 1",
          "currentTerm": "Semester 1 2024/2025"
        }
      ]
    }
  }
}
```

**Error Responses:**
- `400` - Invalid request body
- `401` - Invalid credentials
- `403` - User is not PARENT or STUDENT role

---

### POST /portal/auth/refresh

Refresh access token using refresh token.

**Request:**
```json
{
  "refresh_token": "refresh_token_to_exchange"
}
```

**Response (200):**
```json
{
  "data": {
    "accessToken": "eyJhbGciOiJIUzI1NiIs...",
    "refreshToken": "eyJhbGciOiJIUzI1NiIs...",
    "user": {}
  }
}
```

---

### POST /portal/auth/forgot-password

Request a password reset email. The current service validates and logs the
request but does not send an email.

**Request:**
```json
{
  "email": "parent@example.com"
}
```

**Response (202):**
```json
{
  "data": {
    "message": "if the email exists, a reset link will be sent"
  }
}
```

---

### POST /portal/auth/reset-password

Reset password with token from email. This endpoint currently validates and
logs the request but does not validate the token or change the password.

**Request:**
```json
{
  "token": "reset_token_from_email",
  "password": "NewSecurePass123!"
}
```

**Response (204):** No content

---

### POST /portal/auth/logout

Revoke refresh token.

**Headers:**
```
Authorization: Bearer <access_token>
```

**Request:**
```json
{
  "refreshToken": "refresh_token_to_revoke"
}
```

**Response (204):** No content

---

## Profile & Student Management

### GET /portal/profile

Get current user's profile with preferences and linked students.

**Headers:**
```
Authorization: Bearer <access_token>
```

**Response (200) - Parent:**
```json
{
  "user": {
    "id": "usr_parent_001",
    "email": "parent@example.com",
    "fullName": "Budi Santoso",
    "role": "PARENT",
    "portalRole": "PARENT",
    "linkedStudents": [
      {
        "id": "std_001",
        "nis": "2024001",
        "fullName": "Ahmad Fauzi",
        "birthDate": "2008-05-15",
        "gender": "M",
        "className": "X IPA 1",
        "currentTerm": "Semester 1 2024/2025"
      }
    ]
  },
  "preferences": {
    "userId": "usr_parent_001",
    "language": "id",
    "timezone": "Asia/Jakarta",
    "emailNotifications": true,
    "pushNotifications": true,
    "smsNotifications": false,
    "gradeAlerts": true,
    "attendanceAlerts": true,
    "behaviorAlerts": true,
    "announcementAlerts": true
  },
  "deviceTokens": [
    {
      "id": "dt_001",
      "userId": "usr_parent_001",
      "token": "fcm_token_...",
      "platform": "android",
      "deviceId": "device_123",
      "appVersion": "1.0.0",
      "lastUsedAt": "2024-10-15T08:00:00Z",
      "createdAt": "2024-10-01T10:00:00Z"
    }
  ]
}
```

**Response (200) - Student:**
```json
{
  "user": {
    "id": "usr_student_001",
    "email": "student@example.com",
    "fullName": "Ahmad Fauzi",
    "role": "STUDENT",
    "portalRole": "STUDENT",
    "studentId": "std_001",
    "linkedStudents": [
      {
        "id": "std_001",
        "nis": "2024001",
        "fullName": "Ahmad Fauzi",
        "birthDate": "2008-05-15",
        "gender": "M",
        "className": "X IPA 1",
        "currentTerm": "Semester 1 2024/2025"
      }
    ]
  },
  "preferences": { ... },
  "deviceTokens": [ ... ]
}
```

---

### GET /portal/students

**Parent only** - List all students linked to the authenticated parent.

**Headers:**
```
Authorization: Bearer <access_token>
```

**Response (200):**
```json
[
  {
    "id": "std_001",
    "nis": "2024001",
    "fullName": "Ahmad Fauzi",
    "birthDate": "2008-05-15",
    "gender": "M",
    "className": "X IPA 1",
    "currentTerm": "Semester 1 2024/2025"
  }
]
```

**Error Responses:**
- `403` - Not a parent role

---

### GET /portal/students/:studentId

**Parent only** - Get detailed profile of a specific linked student.

**Headers:**
```
Authorization: Bearer <access_token>
```

**Response (200):**
```json
{
  "id": "std_001",
  "nis": "2024001",
  "fullName": "Ahmad Fauzi",
  "birthDate": "2008-05-15",
  "gender": "M",
  "className": "X IPA 1",
  "currentTerm": "Semester 1 2024/2025",
  "enrollments": [
    {
      "id": "enr_001",
      "classId": "cls_001",
      "className": "X IPA 1",
      "termId": "term_001",
      "termName": "Semester 1 2024/2025",
      "status": "ACTIVE"
    }
  ]
}
```

**Error Responses:**
- `403` - Not a parent or student not linked
- `404` - Student not found

---

## Parent-Student Link Management (Parent Only)

These endpoints allow parents to manage their linked student relationships.

---

### GET /portal/parent/students

Get all students linked to the authenticated parent with relationship details and permissions.

**Headers:**
```
Authorization: Bearer <access_token>
```

**Response (200):**
```json
[
  {
    "id": "link_001",
    "parentId": "usr_parent_001",
    "studentId": "std_001",
    "relationship": "PARENT",
    "canViewGrades": true,
    "canViewAttendance": true,
    "canViewBehavior": true,
    "canViewAnnouncements": true,
    "canReceiveNotifications": true,
    "createdAt": "2024-01-15T08:00:00Z",
    "updatedAt": "2024-01-15T08:00:00Z"
  }
]
```

**Error Responses:**
- `401` - Unauthorized
- `403` - Not a parent role

---

### POST /portal/parent/students

Link a student to the authenticated parent.

**Headers:**
```
Authorization: Bearer <access_token>
```

**Request:**
```json
{
  "studentId": "std_002",
  "relationship": "PARENT",
  "canViewGrades": true,
  "canViewAttendance": true,
  "canViewBehavior": true,
  "canViewAnnouncements": true,
  "canReceiveNotifications": true
}
```

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| studentId | string | Yes | Student ID to link |
| relationship | string | No | `PARENT`, `GUARDIAN`, or `EMERGENCY_CONTACT` (default: `PARENT`) |
| canViewGrades | boolean | No | Allow viewing grades (default: true) |
| canViewAttendance | boolean | No | Allow viewing attendance (default: true) |
| canViewBehavior | boolean | No | Allow viewing behavior notes (default: true) |
| canViewAnnouncements | boolean | No | Allow viewing announcements (default: true) |
| canReceiveNotifications | boolean | No | Receive notifications (default: true) |

**Response (201):**
```json
{
  "id": "link_002",
  "parentId": "usr_parent_001",
  "studentId": "std_002",
  "relationship": "PARENT",
  "canViewGrades": true,
  "canViewAttendance": true,
  "canViewBehavior": true,
  "canViewAnnouncements": true,
  "canReceiveNotifications": true,
  "createdAt": "2024-10-20T10:00:00Z",
  "updatedAt": "2024-10-20T10:00:00Z"
}
```

**Error Responses:**
- `400` - Invalid request payload
- `401` - Unauthorized
- `403` - Not a parent role
- `404` - Student not found
- `409` - Parent-student link already exists

---

### PUT /portal/parent/students/{linkId}

Update permissions for a parent-student link.

**Headers:**
```
Authorization: Bearer <access_token>
```

**Request:**
```json
{
  "relationship": "GUARDIAN",
  "canViewGrades": false,
  "canViewBehavior": true
}
```

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| relationship | string | No | `PARENT`, `GUARDIAN`, or `EMERGENCY_CONTACT` |
| canViewGrades | boolean | No | Allow viewing grades |
| canViewAttendance | boolean | No | Allow viewing attendance |
| canViewBehavior | boolean | No | Allow viewing behavior notes |
| canViewAnnouncements | boolean | No | Allow viewing announcements |
| canReceiveNotifications | boolean | No | Receive notifications |

**Response (200):**
```json
{
  "id": "link_001",
  "parentId": "usr_parent_001",
  "studentId": "std_001",
  "relationship": "GUARDIAN",
  "canViewGrades": false,
  "canViewAttendance": true,
  "canViewBehavior": true,
  "canViewAnnouncements": true,
  "canReceiveNotifications": true,
  "createdAt": "2024-01-15T08:00:00Z",
  "updatedAt": "2024-10-20T10:30:00Z"
}
```

**Error Responses:**
- `400` - Invalid request payload
- `401` - Unauthorized
- `403` - Not a parent or link does not belong to parent
- `404` - Link not found

---

### DELETE /portal/parent/students/{linkId}

Remove a parent-student link.

**Headers:**
```
Authorization: Bearer <access_token>
```

**Response (204):** No content

**Error Responses:**
- `401` - Unauthorized
- `403` - Not a parent or link does not belong to parent
- `404` - Link not found

---

## Grades Endpoints

### GET /portal/grades

Get grades for student(s). Parents can specify `studentId` query param to filter.

**Headers:**
```
Authorization: Bearer <access_token>
```

**Query Parameters:**
| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| termId | string | No | Term ID (defaults to active term) |
| subjectId | string | No | Filter by subject |
| classId | string | No | Filter by class |
| studentId | string | No | **Parent only** - Specific student |

**Response (200):**
```json
{
  "termId": "term_001",
  "grades": [
    {
      "studentId": "std_001",
      "enrollmentId": "enr_001",
      "subjectId": "sub_001",
      "subjectName": "Matematika",
      "subjectCode": "MTK",
      "className": "X IPA 1",
      "componentGrades": {
        "UH1": 85,
        "UH2": 90,
        "UTS": 88,
        "UAS": 92
      },
      "finalGrade": 88.75,
      "letterGrade": "A",
      "isPassed": true,
      "teacherName": "Siti Aminah, S.Pd"
    }
  ],
  "summary": {
    "gpa": 88.75,
    "totalSubjects": 8,
    "passedSubjects": 8,
    "failedSubjects": 0
  }
}
```

---

### GET /portal/grades/report-card

Get full report card for current/selected term.

**Headers:**
```
Authorization: Bearer <access_token>
```

**Query Parameters:**
| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| termId | string | No | Term ID (defaults to active term) |
| studentId | string | No | **Parent only** - Specific student |

**Response (200):**
```json
{
  "studentId": "std_001",
  "studentName": "Ahmad Fauzi",
  "nis": "2024001",
  "className": "X IPA 1",
  "termId": "term_001",
  "termName": "Semester 1 2024/2025",
  "grades": [
    {
      "subjectId": "sub_001",
      "subjectName": "Matematika",
      "subjectCode": "MTK",
      "components": [
        { "name": "UH1", "weight": 20, "score": 85 },
        { "name": "UH2", "weight": 20, "score": 90 },
        { "name": "UTS", "weight": 30, "score": 88 },
        { "name": "UAS", "weight": 30, "score": 92 }
      ],
      "finalGrade": 88.75,
      "letterGrade": "A",
      "isPassed": true,
      "teacherName": "Siti Aminah, S.Pd"
    }
  ],
  "summary": {
    "gpa": 88.75,
    "rank": 3,
    "totalStudents": 30,
    "passedSubjects": 8,
    "failedSubjects": 0
  },
  "attendanceSummary": {
    "percentage": 96.5,
    "totalDays": 60,
    "present": 58,
    "sick": 1,
    "permission": 1,
    "absent": 0
  },
  "behaviorSummary": {
    "totalPoints": 45,
    "positiveNotes": 5,
    "negativeNotes": 0
  }
}
```

---

## Attendance Endpoints

### GET /portal/attendance

Get attendance records (daily and/or subject) for student(s).

**Headers:**
```
Authorization: Bearer <access_token>
```

**Query Parameters:**
| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| termId | string | No | Term ID (defaults to active term) |
| startDate | string | No | Start date (YYYY-MM-DD) |
| endDate | string | No | End date (YYYY-MM-DD) |
| type | string | No | `daily` or `subject` (default: both) |
| studentId | string | No | **Parent only** - Specific student |

**Response (200):**
```json
{
  "studentId": "std_001",
  "termId": "term_001",
  "daily": [
    {
      "id": "att_001",
      "date": "2024-10-15",
      "status": "H",
      "notes": null
    }
  ],
  "subject": [
    {
      "id": "att_sub_001",
      "date": "2024-10-15",
      "subjectId": "sub_001",
      "subjectName": "Matematika",
      "status": "H",
      "notes": null
    }
  ],
  "summary": {
    "totalDays": 60,
    "present": 58,
    "sick": 1,
    "permission": 1,
    "absent": 0,
    "percentage": 96.67
  }
}
```

---

### GET /portal/attendance/percentage

Get overall attendance percentage (lightweight endpoint for dashboard).

**Headers:**
```
Authorization: Bearer <access_token>
```

**Query Parameters:**
| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| termId | string | No | Term ID |
| studentId | string | No | **Parent only** - Specific student |

**Response (200):**
```json
{
  "studentId": "std_001",
  "termId": "term_001",
  "percentage": 96.67,
  "present": 58,
  "totalDays": 60
}
```

---

## Announcements Endpoints

### GET /portal/announcements

Get announcements filtered by audience (STUDENT/PARENT/ALL) and student's class.

**Headers:**
```
Authorization: Bearer <access_token>
```

**Query Parameters:**
| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| page | integer | No | Page number (default: 1) |
| limit | integer | No | Page size (default: 20) |
| active | boolean | No | Only active announcements (default: true) |
| studentId | string | No | **Parent only** - Specific student |

**Response (200):**
```json
{
  "data": [
    {
      "id": "ann_001",
      "title": "Libur Hari Raya Idul Fitri",
      "content": "Sekolah libur tanggal 10-20 April 2024",
      "audience": "ALL",
      "priority": "HIGH",
      "isPinned": true,
      "publishedAt": "2024-04-01T08:00:00Z",
      "expiresAt": "2024-04-21T23:59:59Z",
      "publisherName": "Kepala Sekolah"
    }
  ],
  "pagination": {
    "page": 1,
    "limit": 20,
    "total": 15,
    "totalPages": 1
  }
}
```

---

### GET /portal/announcements/:id

Get single announcement detail.

**Headers:**
```
Authorization: Bearer <access_token>
```

**Response (200):**
```json
{
  "id": "ann_001",
  "title": "Libur Hari Raya Idul Fitri",
  "content": "Sekolah libur tanggal 10-20 April 2024...",
  "audience": "ALL",
  "priority": "HIGH",
  "isPinned": true,
  "publishedAt": "2024-04-01T08:00:00Z",
  "expiresAt": "2024-04-21T23:59:59Z",
  "publisherName": "Kepala Sekolah"
}
```

---

## Behavior Notes Endpoints

### GET /portal/behavior-notes

Get behavior notes for student(s).

**Headers:**
```
Authorization: Bearer <access_token>
```

**Query Parameters:**
| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| termId | string | No | Term ID |
| category | string | No | `POSITIVE`, `NEGATIVE`, `NEUTRAL` |
| studentId | string | No | **Parent only** - Specific student |

**Response (200):**
```json
{
  "studentId": "std_001",
  "termId": "term_001",
  "notes": [
    {
      "id": "beh_001",
      "category": "POSITIVE",
      "title": "Juara Lomba Matematika",
      "description": "Mendapat juara 1 Olimpiade Matematika tingkat kota",
      "date": "2024-10-15",
      "points": 15,
      "reporterName": "Siti Aminah, S.Pd"
    }
  ],
  "summary": {
    "totalNotes": 3,
    "positiveNotes": 2,
    "negativeNotes": 0,
    "neutralNotes": 1,
    "totalPoints": 25
  }
}
```

---

## Calendar Endpoints

### GET /portal/calendar

Get calendar events relevant to student(s) - filtered by audience and class.

**Headers:**
```
Authorization: Bearer <access_token>
```

**Query Parameters:**
| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| startDate | string | No | Start date (YYYY-MM-DD) |
| endDate | string | No | End date (YYYY-MM-DD) |
| month | string | No | Month filter (YYYY-MM) |
| studentId | string | No | **Parent only** - Specific student |

**Response (200):**
```json
{
  "events": [
    {
      "id": "evt_001",
      "title": "Ujian Tengah Semester",
      "description": "UTS untuk semua mata pelajaran",
      "eventType": "EXAM",
      "startDate": "2024-11-01",
      "endDate": "2024-11-07",
      "startTime": "08:00",
      "endTime": "12:00",
      "location": "Ruang Kelas",
      "audience": "SISWA",
      "className": "X IPA 1"
    }
  ]
}
```

---

### GET /portal/calendar/upcoming

Get upcoming events (next 7 days) for quick dashboard display.

**Headers:**
```
Authorization: Bearer <access_token>
```

**Query Parameters:**
| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| studentId | string | No | **Parent only** - Specific student |

**Response (200):**
```json
[
  {
    "id": "evt_001",
    "title": "Ujian Matematika",
    "eventType": "EXAM",
    "startDate": "2024-11-01",
    "startTime": "08:00",
    "location": "Ruang 101",
    "className": "X IPA 1"
  }
]
```

---

## Homeroom Endpoints

### GET /portal/homeroom

Get homeroom teacher and class information for a student in a term.

**Headers:**
```
Authorization: Bearer <access_token>
```

**Query Parameters:**
| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| termId | string | No | Term ID (defaults to active term) |
| studentId | string | No | **Parent only** - Specific student |

**Response (200):**
```json
{
  "studentId": "std_001",
  "studentName": "Ahmad Fauzi",
  "termId": "term_001",
  "termName": "Semester 1 2024/2025",
  "classId": "cls_001",
  "className": "X IPA 1",
  "homeroomTeacher": {
    "id": "tch_001",
    "name": "Siti Aminah, S.Pd"
  }
}
```

**Error Responses:**
- `400` - Invalid request
- `401` - Unauthorized
- `403` - Not a parent/student or student not linked
- `404` - Student/enrollment/homeroom not found

---

## Preferences Endpoints

### GET /portal/preferences

Get notification and display preferences.

**Headers:**
```
Authorization: Bearer <access_token>
```

**Response (200):**
```json
{
  "userId": "usr_parent_001",
  "language": "id",
  "timezone": "Asia/Jakarta",
  "emailNotifications": true,
  "pushNotifications": true,
  "smsNotifications": false,
  "gradeAlerts": true,
  "attendanceAlerts": true,
  "behaviorAlerts": true,
  "announcementAlerts": true
}
```

---

### PUT /portal/preferences

Update preferences.

**Headers:**
```
Authorization: Bearer <access_token>
```

**Request:**
```json
{
  "language": "en",
  "pushNotifications": false,
  "smsNotifications": true,
  "gradeAlerts": true,
  "attendanceAlerts": false
}
```

**Response (200):**
```json
{
  "userId": "usr_parent_001",
  "language": "en",
  "timezone": "Asia/Jakarta",
  "emailNotifications": true,
  "pushNotifications": false,
  "smsNotifications": true,
  "gradeAlerts": true,
  "attendanceAlerts": false,
  "behaviorAlerts": true,
  "announcementAlerts": true
}
```

---

## Device Token Endpoints (Push Notifications)

### POST /portal/device-tokens

Register device token for push notifications.

**Headers:**
```
Authorization: Bearer <access_token>
```

**Request:**
```json
{
  "token": "fcm_token_or_apns_token",
  "platform": "android",
  "deviceId": "device_unique_id",
  "appVersion": "1.0.0"
}
```

**Response (201):**
```json
{
  "status": "registered"
}
```

---

### DELETE /portal/device-tokens/:token

Unregister device token.

**Headers:**
```
Authorization: Bearer <access_token>
```

**Response (204):** No content

---

## Error Response Format

All errors follow this format:

```json
{
  "error": {
    "code": "ERROR_CODE",
    "message": "Human readable message",
    "details": {}
  }
}
```

**Common Error Codes:**
| Code | HTTP Status | Description |
|------|-------------|-------------|
| `UNAUTHORIZED` | 401 | Invalid or missing token |
| `FORBIDDEN` | 403 | Insufficient permissions |
| `NOT_FOUND` | 404 | Resource not found |
| `VALIDATION_ERROR` | 400 | Invalid request payload |
| `INVALID_CREDENTIALS` | 401 | Wrong email/password |
| `TOKEN_EXPIRED` | 401 | Access token expired |
| `RATE_LIMITED` | 429 | Too many requests |

---

## Rate Limiting

- The current Go API does not enforce the limits listed in this historical
  design section. Configure rate limiting at the API gateway/WAF before
  exposing portal endpoints in production.
- The `RATE_LIMITED` error code and `X-RateLimit-*` headers below are planned
  contract elements, not behavior currently guaranteed by the service.

Rate limit headers:
```
X-RateLimit-Limit: 100
X-RateLimit-Remaining: 95
X-RateLimit-Reset: 1730000000
```

---

## Data Scoping Rules

### For Students (STUDENT role):
- Can only access own data (`studentId` from JWT claims)
- A `studentId` query parameter is optional when it matches the claim; a different
  value is rejected with `403 Forbidden`
- Announcements: audience = `STUDENT` or `ALL`
- Calendar: audience = `STUDENT` or `ALL` + class-specific events

### For Parents (PARENT role):
- Can access data for linked children only
- Must specify `studentId` query param for single-student endpoints
- Missing `studentId` is rejected with `400 Bad Request`; this version does not
  expose an aggregate response for all linked children
- Announcements: audience = `SISWA` or `ALL` + class-specific announcements
- Calendar: audience = `SISWA` or `ALL` + class-specific events
- Permissions controlled by `parent_students` link flags (`can_view_grades`, etc.)

---

## Versioning

API version in URL path: `/api/v1/portal`

No `/api/v2/portal` route is implemented or scheduled. Any future breaking
version requires an approved migration and an updated compatibility matrix.

---

## Changelog

| Version | Date | Changes |
|---------|------|---------|
| 1.0.0 | 2024-10-XX | Initial release |
| 1.1.0 | 2024-XX-XX | Added Parent-Student Link Management endpoints (GET/POST/PUT/DELETE /portal/parent/students), Report Card, Announcement Detail, Upcoming Events |
| 1.2.0 | 2024-XX-XX | Added Homeroom endpoint (GET /portal/homeroom) for students and parents |
