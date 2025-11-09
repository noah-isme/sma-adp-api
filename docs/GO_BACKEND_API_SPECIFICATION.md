# 🚀 Go Backend API Specification

> **Admin Panel SMA - Complete API Endpoints Documentation**  
> Version: 1.0.0  
> Last Updated: 2025-11-09  
> Target: Go + Fiber/Gin + PostgreSQL + Redis

---

## 📋 Table of Contents

1. [Authentication & Authorization](#1-authentication--authorization)
2. [User Management](#2-user-management)
3. [Academic Management](#3-academic-management)
4. [Student Management](#4-student-management)
5. [Teacher Management](#5-teacher-management)
6. [Class Management](#6-class-management)
7. [Subject Management](#7-subject-management)
8. [Grade Management](#8-grade-management)
9. [Attendance Management](#9-attendance-management)
10. [Schedule Management](#10-schedule-management)
11. [Dashboard & Analytics](#11-dashboard--analytics)
12. [Reports & Export](#12-reports--export)
13. [Calendar & Events](#13-calendar--events)
14. [Announcements](#14-announcements)
15. [Behavior Notes](#15-behavior-notes)
16. [Mutations & Archives](#16-mutations--archives)

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
  "accessToken": "eyJhbGciOiJIUzI1NiIs...",
  "refreshToken": "eyJhbGciOiJIUzI1NiIs...",
  "expiresIn": 3600,
  "refreshExpiresIn": 86400,
  "tokenType": "Bearer",
  "user": {
    "id": "user_123",
    "email": "admin@harapannusantara.sch.id",
    "fullName": "Admin Tata Usaha",
    "role": "ADMIN_TU",
    "teacherId": null,
    "studentId": null,
    "classId": null
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
  "id": "user_123",
  "email": "admin@harapannusantara.sch.id",
  "fullName": "Admin Tata Usaha",
  "role": "ADMIN_TU",
  "teacherId": null,
  "studentId": null,
  "classId": null
}
```

---

### POST /api/v1/auth/refresh

**Refresh access token**

**Request:**

```json
{
  "refreshToken": "eyJhbGciOiJIUzI1NiIs..."
}
```

**Response (200):**

```json
{
  "accessToken": "eyJhbGciOiJIUzI1NiIs...",
  "refreshToken": "eyJhbGciOiJIUzI1NiIs...",
  "expiresIn": 3600
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

## 📚 3. Academic Management

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

## 🎓 4. Student Management

### GET /api/v1/students/roster

**Get students roster with advanced filtering**

**Query Parameters:**

- `page`, `perPage`
- `classId` (string): Filter by class
- `status` (string): active, inactive, alumni, graduated
- `gender` (string): M, F
- `track` (string): IPA, IPS
- `guardian` (string): Filter by guardian name
- `birthYearStart`, `birthYearEnd` (int)
- `search` (string)
- `sortField` (string): fullName, className, nis, lastUpdated
- `sortOrder` (string): ascend, descend

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
    "page": 1,
    "perPage": 20
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

---

### DELETE /api/v1/students/:id

**Delete student** (soft delete)

---

## 👨‍🏫 5. Teacher Management

### GET /api/v1/teachers/roster

**Get teachers roster with filtering**

**Query Parameters:**

- `page`, `perPage`
- `subjectId` (string): Filter by main subject
- `status` (string): active, inactive, on_leave
- `track` (string): IPA, IPS
- `availability` (string): HIGH, MEDIUM, LOW
- `homeroomClassId` (string): Filter homeroom teachers
- `search` (string)
- `sortField` (string): fullName, mainSubjectName, assignmentCount, availability
- `sortOrder` (string): ascend, descend

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
  "appliedFilters": {...}
}
```

---

### POST /api/v1/teachers

**Create new teacher**

---

### PATCH /api/v1/teachers/:id

**Update teacher**

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

## 🏫 6. Class Management

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

## 📖 7. Subject Management

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

## 📝 8. Grade Management

### GET /api/v1/grades/report

**Get comprehensive grade report with filters**

**Query Parameters:**

- `termId` (string)
- `classId` (string)
- `subjectId` (string)
- `componentId` (string)
- `teacherId` (string)
- `status` (string): ALL, PASS, CAUTION, REMEDIAL
- `scoreMin`, `scoreMax` (number)
- `search` (string): Student name or NIS
- `page`, `perPage`
- `sortField` (string): studentName, subjectName, componentName, score, lastUpdated
- `sortOrder` (string): ascend, descend

**Response (200):**

```json
{
  "context": {
    "termId": "term_2024_1",
    "termName": "Tahun Pelajaran 2024/2025 Semester 1",
    "termLabel": "2024/2025 • Semester 1",
    "classId": "class_x_ipa_1",
    "className": "Kelas X IPA-1",
    "subjectId": "subject_mat",
    "subjectName": "Matematika",
    "teacherId": "teacher_001",
    "teacherName": "Pak Budi Santoso"
  },
  "summary": {
    "averageScore": 78.5,
    "highestScore": {
      "score": 95,
      "studentId": "stu_001",
      "studentName": "Siswa A",
      "componentName": "UTS",
      "componentCategory": "UTS"
    },
    "lowestScore": {
      "score": 55,
      "studentId": "stu_030",
      "studentName": "Siswa Z",
      "componentName": "UH 1"
    },
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
    "page": 1,
    "perPage": 25
  }
}
```

---

### GET /api/v1/grades

**List grades with simple filtering**

**Query Parameters:**

- `enrollmentId`, `componentId`, `subjectId`, `teacherId`
- `scoreMin`, `scoreMax`

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

### PATCH /api/v1/grades/:id

**Update grade**

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

### GET /api/v1/grade-configs

**Get grade configuration for class-subject**

---

### POST /api/v1/grade-configs

**Create/Update grade config**

---

## 📅 9. Attendance Management

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

## 🗓️ 10. Schedule Management

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

## 📊 11. Dashboard & Analytics

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

## 📄 12. Reports & Export

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

**Export students data (CSV/Excel)**

**Query Parameters:**

- `format` (string): csv, xlsx
- `classId` (string)
- `status` (string)

**Response (200):**
Returns file download

---

### GET /api/v1/export/grades

**Export grades data**

---

### GET /api/v1/export/attendance

**Export attendance data**

---

## 📆 13. Calendar & Events

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

## 📢 14. Announcements

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

## 📝 15. Behavior Notes

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

## 🔄 16. Mutations & Archives

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

---

### PATCH /api/v1/mutations/:id/reject

**Reject mutation**

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

**Last Updated:** 2025-11-09  
**Document Version:** 1.0.0  
**Maintained By:** Development Team
