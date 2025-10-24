# Phase 2: Academic Management APIs (Week 6-8)

## 🎯 Objectives

- Implement Terms (Semester) management
- Subjects CRUD with track-based filtering
- Classes management with homeroom assignments
- Schedule management with conflict detection
- Class-subject mapping APIs
- Build upon authentication patterns from Phase 1

## Prerequisites

- ✅ Phase 0: Infrastructure setup complete
- ✅ Phase 1: Authentication & User Management operational
- ✅ JWT middleware and RBAC working

---

## 2.1 Database Models

### Terms Table (Existing Schema)

```sql
-- Academic terms/semesters
CREATE TABLE terms (
    id VARCHAR(255) PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    type VARCHAR(50) NOT NULL, -- 'SEMESTER_1', 'SEMESTER_2'
    academic_year VARCHAR(20) NOT NULL, -- '2024/2025'
    start_date DATE NOT NULL,
    end_date DATE NOT NULL,
    is_active BOOLEAN DEFAULT false,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(academic_year, type)
);

CREATE INDEX idx_terms_academic_year ON terms(academic_year);
CREATE INDEX idx_terms_is_active ON terms(is_active);
```

### Subjects Table (Existing Schema)

```sql
-- Subjects/courses
CREATE TABLE subjects (
    id VARCHAR(255) PRIMARY KEY,
    code VARCHAR(50) UNIQUE NOT NULL,
    name VARCHAR(255) NOT NULL,
    track VARCHAR(50), -- NULL, 'IPA', 'IPS'
    subject_group VARCHAR(50) NOT NULL, -- 'CORE', 'DIFFERENTIATED', 'ELECTIVE'
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_subjects_code ON subjects(code);
CREATE INDEX idx_subjects_track ON subjects(track);
CREATE INDEX idx_subjects_group ON subjects(subject_group);
```

### Classes Table (Existing Schema)

```sql
-- Class groups (e.g., X IPA 1)
CREATE TABLE classes (
    id VARCHAR(255) PRIMARY KEY,
    name VARCHAR(255) UNIQUE NOT NULL,
    grade VARCHAR(10) NOT NULL, -- 'X', 'XI', 'XII'
    track VARCHAR(50), -- NULL, 'IPA', 'IPS'
    homeroom_teacher_id VARCHAR(255) REFERENCES users(id) ON DELETE SET NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_classes_grade ON classes(grade);
CREATE INDEX idx_classes_track ON classes(track);
CREATE INDEX idx_classes_homeroom ON classes(homeroom_teacher_id);
```

### Class Subjects Table (Existing Schema)

```sql
-- Mapping between classes and subjects
CREATE TABLE class_subjects (
    id VARCHAR(255) PRIMARY KEY,
    class_id VARCHAR(255) NOT NULL REFERENCES classes(id) ON DELETE CASCADE,
    subject_id VARCHAR(255) NOT NULL REFERENCES subjects(id) ON DELETE CASCADE,
    teacher_id VARCHAR(255) REFERENCES users(id) ON DELETE SET NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(class_id, subject_id)
);

CREATE INDEX idx_class_subjects_class ON class_subjects(class_id);
CREATE INDEX idx_class_subjects_subject ON class_subjects(subject_id);
CREATE INDEX idx_class_subjects_teacher ON class_subjects(teacher_id);
```

### Schedules Table (Existing Schema)

```sql
-- Weekly class schedules
CREATE TABLE schedules (
    id VARCHAR(255) PRIMARY KEY,
    term_id VARCHAR(255) NOT NULL REFERENCES terms(id) ON DELETE CASCADE,
    class_id VARCHAR(255) NOT NULL REFERENCES classes(id) ON DELETE CASCADE,
    subject_id VARCHAR(255) NOT NULL REFERENCES subjects(id) ON DELETE CASCADE,
    teacher_id VARCHAR(255) REFERENCES users(id) ON DELETE SET NULL,
    day_of_week INTEGER NOT NULL CHECK (day_of_week BETWEEN 1 AND 6), -- 1=Monday, 6=Saturday
    time_slot INTEGER NOT NULL CHECK (time_slot BETWEEN 1 AND 10), -- Slot number
    room VARCHAR(100),
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(term_id, class_id, day_of_week, time_slot),
    UNIQUE(term_id, teacher_id, day_of_week, time_slot),
    UNIQUE(term_id, room, day_of_week, time_slot)
);

CREATE INDEX idx_schedules_term ON schedules(term_id);
CREATE INDEX idx_schedules_class ON schedules(class_id);
CREATE INDEX idx_schedules_teacher ON schedules(teacher_id);
CREATE INDEX idx_schedules_day_slot ON schedules(day_of_week, time_slot);
```

---

## 2.2 Go Models & Structs

### internal/models/term.go

```go
package models

import "time"

type TermType string

const (
    TermTypeSemester1 TermType = "SEMESTER_1"
    TermTypeSemester2 TermType = "SEMESTER_2"
)

type Term struct {
    ID           string    `db:"id" json:"id"`
    Name         string    `db:"name" json:"name"`
    Type         TermType  `db:"type" json:"type"`
    AcademicYear string    `db:"academic_year" json:"academicYear"`
    StartDate    time.Time `db:"start_date" json:"startDate"`
    EndDate      time.Time `db:"end_date" json:"endDate"`
    IsActive     bool      `db:"is_active" json:"isActive"`
    CreatedAt    time.Time `db:"created_at" json:"createdAt"`
    UpdatedAt    time.Time `db:"updated_at" json:"updatedAt"`
}

// Request DTOs
type CreateTermRequest struct {
    Name         string   `json:"name" binding:"required"`
    Type         TermType `json:"type" binding:"required,oneof=SEMESTER_1 SEMESTER_2"`
    AcademicYear string   `json:"academicYear" binding:"required"`
    StartDate    string   `json:"startDate" binding:"required"` // YYYY-MM-DD
    EndDate      string   `json:"endDate" binding:"required"`
}

type UpdateTermRequest struct {
    Name      *string   `json:"name"`
    StartDate *string   `json:"startDate"`
    EndDate   *string   `json:"endDate"`
    IsActive  *bool     `json:"isActive"`
}

type SetActiveTermRequest struct {
    TermID string `json:"termId" binding:"required"`
}
```

### internal/models/subject.go

```go
package models

import "time"

type SubjectTrack string
type SubjectGroup string

const (
    TrackNone SubjectTrack = ""
    TrackIPA  SubjectTrack = "IPA"
    TrackIPS  SubjectTrack = "IPS"

    GroupCore           SubjectGroup = "CORE"
    GroupDifferentiated SubjectGroup = "DIFFERENTIATED"
    GroupElective       SubjectGroup = "ELECTIVE"
)

type Subject struct {
    ID           string       `db:"id" json:"id"`
    Code         string       `db:"code" json:"code"`
    Name         string       `db:"name" json:"name"`
    Track        *SubjectTrack `db:"track" json:"track,omitempty"`
    SubjectGroup SubjectGroup `db:"subject_group" json:"subjectGroup"`
    CreatedAt    time.Time    `db:"created_at" json:"createdAt"`
    UpdatedAt    time.Time    `db:"updated_at" json:"updatedAt"`
}

// Request DTOs
type CreateSubjectRequest struct {
    Code         string       `json:"code" binding:"required"`
    Name         string       `json:"name" binding:"required"`
    Track        *string      `json:"track"` // null, "IPA", "IPS"
    SubjectGroup SubjectGroup `json:"subjectGroup" binding:"required,oneof=CORE DIFFERENTIATED ELECTIVE"`
}

type UpdateSubjectRequest struct {
    Code         *string       `json:"code"`
    Name         *string       `json:"name"`
    Track        *string       `json:"track"`
    SubjectGroup *SubjectGroup `json:"subjectGroup"`
}
```

### internal/models/class.go

```go
package models

import "time"

type ClassGrade string
type ClassTrack string

const (
    GradeX   ClassGrade = "X"
    GradeXI  ClassGrade = "XI"
    GradeXII ClassGrade = "XII"

    ClassTrackNone ClassTrack = ""
    ClassTrackIPA  ClassTrack = "IPA"
    ClassTrackIPS  ClassTrack = "IPS"
)

type Class struct {
    ID                 string      `db:"id" json:"id"`
    Name               string      `db:"name" json:"name"`
    Grade              ClassGrade  `db:"grade" json:"grade"`
    Track              *ClassTrack `db:"track" json:"track,omitempty"`
    HomeroomTeacherID  *string     `db:"homeroom_teacher_id" json:"homeroomTeacherId,omitempty"`
    HomeroomTeacher    *UserInfo   `json:"homeroomTeacher,omitempty"` // Joined data
    CreatedAt          time.Time   `db:"created_at" json:"createdAt"`
    UpdatedAt          time.Time   `db:"updated_at" json:"updatedAt"`
}

type ClassSubject struct {
    ID        string    `db:"id" json:"id"`
    ClassID   string    `db:"class_id" json:"classId"`
    SubjectID string    `db:"subject_id" json:"subjectId"`
    TeacherID *string   `db:"teacher_id" json:"teacherId,omitempty"`
    Subject   *Subject  `json:"subject,omitempty"`   // Joined data
    Teacher   *UserInfo `json:"teacher,omitempty"`   // Joined data
    CreatedAt time.Time `db:"created_at" json:"createdAt"`
}

// Request DTOs
type CreateClassRequest struct {
    Name              string     `json:"name" binding:"required"`
    Grade             ClassGrade `json:"grade" binding:"required,oneof=X XI XII"`
    Track             *string    `json:"track"` // null, "IPA", "IPS"
    HomeroomTeacherID *string    `json:"homeroomTeacherId"`
}

type UpdateClassRequest struct {
    Name              *string `json:"name"`
    HomeroomTeacherID *string `json:"homeroomTeacherId"`
}

type AssignSubjectsRequest struct {
    Subjects []SubjectAssignment `json:"subjects" binding:"required,min=1"`
}

type SubjectAssignment struct {
    SubjectID string  `json:"subjectId" binding:"required"`
    TeacherID *string `json:"teacherId"`
}
```

### internal/models/schedule.go

```go
package models

import "time"

type DayOfWeek int

const (
    Monday    DayOfWeek = 1
    Tuesday   DayOfWeek = 2
    Wednesday DayOfWeek = 3
    Thursday  DayOfWeek = 4
    Friday    DayOfWeek = 5
    Saturday  DayOfWeek = 6
)

type Schedule struct {
    ID        string    `db:"id" json:"id"`
    TermID    string    `db:"term_id" json:"termId"`
    ClassID   string    `db:"class_id" json:"classId"`
    SubjectID string    `db:"subject_id" json:"subjectId"`
    TeacherID *string   `db:"teacher_id" json:"teacherId,omitempty"`
    DayOfWeek DayOfWeek `db:"day_of_week" json:"dayOfWeek"`
    TimeSlot  int       `db:"time_slot" json:"timeSlot"`
    Room      *string   `db:"room" json:"room,omitempty"`
    Class     *Class    `json:"class,omitempty"`    // Joined data
    Subject   *Subject  `json:"subject,omitempty"`  // Joined data
    Teacher   *UserInfo `json:"teacher,omitempty"`  // Joined data
    CreatedAt time.Time `db:"created_at" json:"createdAt"`
    UpdatedAt time.Time `db:"updated_at" json:"updatedAt"`
}

// Request DTOs
type CreateScheduleRequest struct {
    TermID    string `json:"termId" binding:"required"`
    ClassID   string `json:"classId" binding:"required"`
    SubjectID string `json:"subjectId" binding:"required"`
    TeacherID string `json:"teacherId" binding:"required"`
    DayOfWeek int    `json:"dayOfWeek" binding:"required,min=1,max=6"`
    TimeSlot  int    `json:"timeSlot" binding:"required,min=1,max=10"`
    Room      string `json:"room"`
}

type UpdateScheduleRequest struct {
    TeacherID *string `json:"teacherId"`
    Room      *string `json:"room"`
}

type BulkCreateSchedulesRequest struct {
    Schedules []CreateScheduleRequest `json:"schedules" binding:"required,min=1"`
}

type ScheduleConflictError struct {
    Type      string `json:"type"` // "class", "teacher", "room"
    Message   string `json:"message"`
    Conflict  *Schedule `json:"conflict,omitempty"`
}
```

---

## 2.3 API Endpoints Specification

### Base URL

```
Development: http://localhost:8080/api/v1
Production:  https://api.yourdomain.com/api/v1
```

---

## Terms Management

### 1. GET /terms

**Description**: List all academic terms

**Headers:**

```
Authorization: Bearer {accessToken}
```

**Query Parameters:**

- `page` (integer, default: 1)
- `limit` (integer, default: 10)
- `academicYear` (string, optional): Filter by academic year
- `type` (string, optional): Filter by type (SEMESTER_1/SEMESTER_2)
- `isActive` (boolean, optional): Filter by active status

**Response (200 OK):**

```json
{
  "data": [
    {
      "id": "term_abc123",
      "name": "Semester 1 2024/2025",
      "type": "SEMESTER_1",
      "academicYear": "2024/2025",
      "startDate": "2024-07-15T00:00:00Z",
      "endDate": "2024-12-20T00:00:00Z",
      "isActive": true,
      "createdAt": "2024-06-01T00:00:00Z",
      "updatedAt": "2024-07-01T00:00:00Z"
    }
  ],
  "pagination": {
    "page": 1,
    "limit": 10,
    "total": 5,
    "totalPages": 1
  }
}
```

**Permissions**: All authenticated users

---

### 2. GET /terms/active

**Description**: Get currently active term

**Headers:**

```
Authorization: Bearer {accessToken}
```

**Response (200 OK):**

```json
{
  "id": "term_abc123",
  "name": "Semester 1 2024/2025",
  "type": "SEMESTER_1",
  "academicYear": "2024/2025",
  "startDate": "2024-07-15T00:00:00Z",
  "endDate": "2024-12-20T00:00:00Z",
  "isActive": true,
  "createdAt": "2024-06-01T00:00:00Z",
  "updatedAt": "2024-07-01T00:00:00Z"
}
```

**Error Responses:**

- `404 Not Found`: No active term

**Permissions**: All authenticated users

---

### 3. POST /terms

**Description**: Create new academic term

**Headers:**

```
Authorization: Bearer {accessToken}
```

**Request:**

```json
{
  "name": "Semester 1 2025/2026",
  "type": "SEMESTER_1",
  "academicYear": "2025/2026",
  "startDate": "2025-07-15",
  "endDate": "2025-12-20"
}
```

**Response (201 Created):**

```json
{
  "id": "term_xyz789",
  "name": "Semester 1 2025/2026",
  "type": "SEMESTER_1",
  "academicYear": "2025/2026",
  "startDate": "2025-07-15T00:00:00Z",
  "endDate": "2025-12-20T00:00:00Z",
  "isActive": false,
  "createdAt": "2025-06-01T10:00:00Z",
  "updatedAt": "2025-06-01T10:00:00Z"
}
```

**Error Responses:**

- `409 Conflict`: Term for this academic year and type already exists

**Permissions**: ADMIN, SUPERADMIN

---

### 4. PUT /terms/:id

**Description**: Update term

**Headers:**

```
Authorization: Bearer {accessToken}
```

**Request:**

```json
{
  "name": "Semester 1 2025/2026 (Updated)",
  "startDate": "2025-07-16",
  "endDate": "2025-12-21"
}
```

**Response (200 OK):**

```json
{
  "id": "term_xyz789",
  "name": "Semester 1 2025/2026 (Updated)",
  "type": "SEMESTER_1",
  "academicYear": "2025/2026",
  "startDate": "2025-07-16T00:00:00Z",
  "endDate": "2025-12-21T00:00:00Z",
  "isActive": false,
  "updatedAt": "2025-06-02T10:00:00Z"
}
```

**Permissions**: ADMIN, SUPERADMIN

---

### 5. POST /terms/set-active

**Description**: Set a term as active (deactivates others)

**Headers:**

```
Authorization: Bearer {accessToken}
```

**Request:**

```json
{
  "termId": "term_xyz789"
}
```

**Response (200 OK):**

```json
{
  "message": "Term activated successfully",
  "activeTerm": {
    "id": "term_xyz789",
    "name": "Semester 1 2025/2026",
    "isActive": true
  }
}
```

**Permissions**: ADMIN, SUPERADMIN

---

### 6. DELETE /terms/:id

**Description**: Delete term (soft delete)

**Headers:**

```
Authorization: Bearer {accessToken}
```

**Response (200 OK):**

```json
{
  "message": "Term deleted successfully"
}
```

**Error Responses:**

- `400 Bad Request`: Cannot delete active term
- `409 Conflict`: Term has associated data (schedules, grades)

**Permissions**: SUPERADMIN

---

## Subjects Management

### 7. GET /subjects

**Description**: List all subjects

**Headers:**

```
Authorization: Bearer {accessToken}
```

**Query Parameters:**

- `page` (integer, default: 1)
- `limit` (integer, default: 50)
- `track` (string, optional): Filter by track (IPA/IPS/NONE)
- `group` (string, optional): Filter by group (CORE/DIFFERENTIATED/ELECTIVE)
- `search` (string, optional): Search by code or name

**Response (200 OK):**

```json
{
  "data": [
    {
      "id": "subj_math001",
      "code": "MAT",
      "name": "Matematika",
      "track": null,
      "subjectGroup": "CORE",
      "createdAt": "2024-01-01T00:00:00Z",
      "updatedAt": "2024-01-01T00:00:00Z"
    },
    {
      "id": "subj_bio001",
      "code": "BIO",
      "name": "Biologi",
      "track": "IPA",
      "subjectGroup": "DIFFERENTIATED",
      "createdAt": "2024-01-01T00:00:00Z",
      "updatedAt": "2024-01-01T00:00:00Z"
    }
  ],
  "pagination": {
    "page": 1,
    "limit": 50,
    "total": 25,
    "totalPages": 1
  }
}
```

**Permissions**: All authenticated users

---

### 8. GET /subjects/:id

**Description**: Get subject by ID

**Headers:**

```
Authorization: Bearer {accessToken}
```

**Response (200 OK):**

```json
{
  "id": "subj_math001",
  "code": "MAT",
  "name": "Matematika",
  "track": null,
  "subjectGroup": "CORE",
  "createdAt": "2024-01-01T00:00:00Z",
  "updatedAt": "2024-01-01T00:00:00Z"
}
```

**Permissions**: All authenticated users

---

### 9. POST /subjects

**Description**: Create new subject

**Headers:**

```
Authorization: Bearer {accessToken}
```

**Request:**

```json
{
  "code": "FIS",
  "name": "Fisika",
  "track": "IPA",
  "subjectGroup": "DIFFERENTIATED"
}
```

**Response (201 Created):**

```json
{
  "id": "subj_fis001",
  "code": "FIS",
  "name": "Fisika",
  "track": "IPA",
  "subjectGroup": "DIFFERENTIATED",
  "createdAt": "2025-10-24T10:00:00Z",
  "updatedAt": "2025-10-24T10:00:00Z"
}
```

**Error Responses:**

- `409 Conflict`: Subject code already exists

**Permissions**: ADMIN, SUPERADMIN

---

### 10. PUT /subjects/:id

**Description**: Update subject

**Headers:**

```
Authorization: Bearer {accessToken}
```

**Request:**

```json
{
  "name": "Fisika (Updated)",
  "track": "IPA"
}
```

**Response (200 OK):**

```json
{
  "id": "subj_fis001",
  "code": "FIS",
  "name": "Fisika (Updated)",
  "track": "IPA",
  "subjectGroup": "DIFFERENTIATED",
  "updatedAt": "2025-10-24T11:00:00Z"
}
```

**Permissions**: ADMIN, SUPERADMIN

---

### 11. DELETE /subjects/:id

**Description**: Delete subject

**Headers:**

```
Authorization: Bearer {accessToken}
```

**Response (200 OK):**

```json
{
  "message": "Subject deleted successfully"
}
```

**Error Responses:**

- `409 Conflict`: Subject is assigned to classes

**Permissions**: SUPERADMIN

---

## Classes Management

### 12. GET /classes

**Description**: List all classes

**Headers:**

```
Authorization: Bearer {accessToken}
```

**Query Parameters:**

- `page` (integer, default: 1)
- `limit` (integer, default: 50)
- `grade` (string, optional): Filter by grade (X/XI/XII)
- `track` (string, optional): Filter by track (IPA/IPS)
- `search` (string, optional): Search by name

**Response (200 OK):**

```json
{
  "data": [
    {
      "id": "cls_xipa1",
      "name": "X IPA 1",
      "grade": "X",
      "track": "IPA",
      "homeroomTeacherId": "usr_teacher01",
      "homeroomTeacher": {
        "id": "usr_teacher01",
        "fullName": "Budi Santoso",
        "email": "budi@school.com"
      },
      "createdAt": "2024-01-01T00:00:00Z",
      "updatedAt": "2024-01-01T00:00:00Z"
    }
  ],
  "pagination": {
    "page": 1,
    "limit": 50,
    "total": 12,
    "totalPages": 1
  }
}
```

**Permissions**: All authenticated users

---

### 13. GET /classes/:id

**Description**: Get class by ID with full details

**Headers:**

```
Authorization: Bearer {accessToken}
```

**Response (200 OK):**

```json
{
  "id": "cls_xipa1",
  "name": "X IPA 1",
  "grade": "X",
  "track": "IPA",
  "homeroomTeacherId": "usr_teacher01",
  "homeroomTeacher": {
    "id": "usr_teacher01",
    "fullName": "Budi Santoso",
    "email": "budi@school.com",
    "role": "TEACHER"
  },
  "createdAt": "2024-01-01T00:00:00Z",
  "updatedAt": "2024-01-01T00:00:00Z"
}
```

**Permissions**: All authenticated users

---

### 14. POST /classes

**Description**: Create new class

**Headers:**

```
Authorization: Bearer {accessToken}
```

**Request:**

```json
{
  "name": "X IPA 2",
  "grade": "X",
  "track": "IPA",
  "homeroomTeacherId": "usr_teacher02"
}
```

**Response (201 Created):**

```json
{
  "id": "cls_xipa2",
  "name": "X IPA 2",
  "grade": "X",
  "track": "IPA",
  "homeroomTeacherId": "usr_teacher02",
  "createdAt": "2025-10-24T10:00:00Z",
  "updatedAt": "2025-10-24T10:00:00Z"
}
```

**Error Responses:**

- `409 Conflict`: Class name already exists

**Permissions**: ADMIN, SUPERADMIN

---

### 15. PUT /classes/:id

**Description**: Update class

**Headers:**

```
Authorization: Bearer {accessToken}
```

**Request:**

```json
{
  "name": "X IPA 2 (Updated)",
  "homeroomTeacherId": "usr_teacher03"
}
```

**Response (200 OK):**

```json
{
  "id": "cls_xipa2",
  "name": "X IPA 2 (Updated)",
  "grade": "X",
  "track": "IPA",
  "homeroomTeacherId": "usr_teacher03",
  "updatedAt": "2025-10-24T11:00:00Z"
}
```

**Permissions**: ADMIN, SUPERADMIN

---

### 16. DELETE /classes/:id

**Description**: Delete class

**Headers:**

```
Authorization: Bearer {accessToken}
```

**Response (200 OK):**

```json
{
  "message": "Class deleted successfully"
}
```

**Error Responses:**

- `409 Conflict`: Class has enrolled students

**Permissions**: SUPERADMIN

---

### 17. GET /classes/:id/subjects

**Description**: Get all subjects assigned to a class

**Headers:**

```
Authorization: Bearer {accessToken}
```

**Response (200 OK):**

```json
{
  "data": [
    {
      "id": "clssubj_001",
      "classId": "cls_xipa1",
      "subjectId": "subj_math001",
      "teacherId": "usr_teacher04",
      "subject": {
        "id": "subj_math001",
        "code": "MAT",
        "name": "Matematika",
        "track": null,
        "subjectGroup": "CORE"
      },
      "teacher": {
        "id": "usr_teacher04",
        "fullName": "Siti Aminah",
        "email": "siti@school.com"
      },
      "createdAt": "2024-07-01T00:00:00Z"
    }
  ]
}
```

**Permissions**: All authenticated users

---

### 18. POST /classes/:id/subjects

**Description**: Assign subjects to a class (bulk)

**Headers:**

```
Authorization: Bearer {accessToken}
```

**Request:**

```json
{
  "subjects": [
    {
      "subjectId": "subj_math001",
      "teacherId": "usr_teacher04"
    },
    {
      "subjectId": "subj_bio001",
      "teacherId": "usr_teacher05"
    }
  ]
}
```

**Response (200 OK):**

```json
{
  "message": "Subjects assigned successfully",
  "assigned": 2
}
```

**Error Responses:**

- `400 Bad Request`: Invalid subject or teacher ID
- `409 Conflict`: Subject already assigned to class

**Permissions**: ADMIN, SUPERADMIN

---

### 19. DELETE /classes/:classId/subjects/:subjectId

**Description**: Remove subject from class

**Headers:**

```
Authorization: Bearer {accessToken}
```

**Response (200 OK):**

```json
{
  "message": "Subject removed from class successfully"
}
```

**Permissions**: ADMIN, SUPERADMIN

---

## Schedules Management

### 20. GET /schedules

**Description**: List schedules with filters

**Headers:**

```
Authorization: Bearer {accessToken}
```

**Query Parameters:**

- `termId` (string, required): Filter by term
- `classId` (string, optional): Filter by class
- `teacherId` (string, optional): Filter by teacher
- `dayOfWeek` (integer, optional): Filter by day (1-6)

**Response (200 OK):**

```json
{
  "data": [
    {
      "id": "sch_001",
      "termId": "term_abc123",
      "classId": "cls_xipa1",
      "subjectId": "subj_math001",
      "teacherId": "usr_teacher04",
      "dayOfWeek": 1,
      "timeSlot": 1,
      "room": "R101",
      "class": {
        "id": "cls_xipa1",
        "name": "X IPA 1"
      },
      "subject": {
        "id": "subj_math001",
        "code": "MAT",
        "name": "Matematika"
      },
      "teacher": {
        "id": "usr_teacher04",
        "fullName": "Siti Aminah"
      },
      "createdAt": "2024-07-01T00:00:00Z"
    }
  ]
}
```

**Permissions**: All authenticated users

---

### 21. GET /schedules/class/:classId

**Description**: Get full weekly schedule for a class

**Headers:**

```
Authorization: Bearer {accessToken}
```

**Query Parameters:**

- `termId` (string, required): Term ID

**Response (200 OK):**

```json
{
  "classId": "cls_xipa1",
  "className": "X IPA 1",
  "termId": "term_abc123",
  "schedule": {
    "1": { // Monday
      "1": {
        "id": "sch_001",
        "subject": "Matematika",
        "teacher": "Siti Aminah",
        "room": "R101"
      },
      "2": {
        "id": "sch_002",
        "subject": "Biologi",
        "teacher": "Ahmad Yani",
        "room": "R102"
      }
    },
    "2": { // Tuesday
      "1": { ... }
    }
  }
}
```

**Permissions**: All authenticated users

---

### 22. GET /schedules/teacher/:teacherId

**Description**: Get full weekly schedule for a teacher

**Headers:**

```
Authorization: Bearer {accessToken}
```

**Query Parameters:**

- `termId` (string, required): Term ID

**Response (200 OK):**

```json
{
  "teacherId": "usr_teacher04",
  "teacherName": "Siti Aminah",
  "termId": "term_abc123",
  "schedule": {
    "1": {
      // Monday
      "1": {
        "id": "sch_001",
        "class": "X IPA 1",
        "subject": "Matematika",
        "room": "R101"
      }
    }
  }
}
```

**Permissions**:

- ADMIN/SUPERADMIN: Can view any teacher
- TEACHER: Can only view own schedule

---

### 23. POST /schedules

**Description**: Create single schedule entry

**Headers:**

```
Authorization: Bearer {accessToken}
```

**Request:**

```json
{
  "termId": "term_abc123",
  "classId": "cls_xipa1",
  "subjectId": "subj_math001",
  "teacherId": "usr_teacher04",
  "dayOfWeek": 1,
  "timeSlot": 1,
  "room": "R101"
}
```

**Response (201 Created):**

```json
{
  "id": "sch_new001",
  "termId": "term_abc123",
  "classId": "cls_xipa1",
  "subjectId": "subj_math001",
  "teacherId": "usr_teacher04",
  "dayOfWeek": 1,
  "timeSlot": 1,
  "room": "R101",
  "createdAt": "2025-10-24T10:00:00Z"
}
```

**Error Responses:**

- `409 Conflict`: Schedule conflict detected
  ```json
  {
    "error": "Schedule conflict",
    "conflicts": [
      {
        "type": "class",
        "message": "Class X IPA 1 already has a schedule at Monday slot 1",
        "conflict": { ...existing schedule... }
      }
    ]
  }
  ```

**Permissions**: ADMIN, SUPERADMIN

---

### 24. POST /schedules/bulk

**Description**: Create multiple schedule entries at once

**Headers:**

```
Authorization: Bearer {accessToken}
```

**Request:**

```json
{
  "schedules": [
    {
      "termId": "term_abc123",
      "classId": "cls_xipa1",
      "subjectId": "subj_math001",
      "teacherId": "usr_teacher04",
      "dayOfWeek": 1,
      "timeSlot": 1,
      "room": "R101"
    },
    {
      "termId": "term_abc123",
      "classId": "cls_xipa1",
      "subjectId": "subj_bio001",
      "teacherId": "usr_teacher05",
      "dayOfWeek": 1,
      "timeSlot": 2,
      "room": "R102"
    }
  ]
}
```

**Response (201 Created):**

```json
{
  "message": "Schedules created successfully",
  "created": 2,
  "failed": 0
}
```

**Permissions**: ADMIN, SUPERADMIN

---

### 25. PUT /schedules/:id

**Description**: Update schedule entry

**Headers:**

```
Authorization: Bearer {accessToken}
```

**Request:**

```json
{
  "teacherId": "usr_teacher06",
  "room": "R103"
}
```

**Response (200 OK):**

```json
{
  "id": "sch_001",
  "teacherId": "usr_teacher06",
  "room": "R103",
  "updatedAt": "2025-10-24T11:00:00Z"
}
```

**Permissions**: ADMIN, SUPERADMIN

---

### 26. DELETE /schedules/:id

**Description**: Delete schedule entry

**Headers:**

```
Authorization: Bearer {accessToken}
```

**Response (200 OK):**

```json
{
  "message": "Schedule deleted successfully"
}
```

**Permissions**: ADMIN, SUPERADMIN

---

### 27. POST /schedules/check-conflicts

**Description**: Check for schedule conflicts before creating

**Headers:**

```
Authorization: Bearer {accessToken}
```

**Request:**

```json
{
  "termId": "term_abc123",
  "classId": "cls_xipa1",
  "teacherId": "usr_teacher04",
  "dayOfWeek": 1,
  "timeSlot": 1,
  "room": "R101"
}
```

**Response (200 OK - No Conflicts):**

```json
{
  "hasConflicts": false,
  "conflicts": []
}
```

**Response (200 OK - Has Conflicts):**

```json
{
  "hasConflicts": true,
  "conflicts": [
    {
      "type": "teacher",
      "message": "Teacher already has a schedule at Monday slot 1",
      "conflict": {
        "id": "sch_existing",
        "class": "XI IPA 1",
        "subject": "Fisika"
      }
    }
  ]
}
```

**Permissions**: ADMIN, SUPERADMIN

---

## 2.4 Implementation Guidelines

### Repository Layer Pattern

```go
// Example: internal/repository/term_repository.go
type TermRepository interface {
    List(ctx context.Context, filters TermFilters) ([]*models.Term, int, error)
    FindByID(ctx context.Context, id string) (*models.Term, error)
    FindActive(ctx context.Context) (*models.Term, error)
    Create(ctx context.Context, term *models.Term) error
    Update(ctx context.Context, term *models.Term) error
    SetActive(ctx context.Context, termID string) error
    Delete(ctx context.Context, id string) error
}
```

### Service Layer Pattern

```go
// Example: internal/service/schedule_service.go
type ScheduleService interface {
    Create(ctx context.Context, req *models.CreateScheduleRequest) (*models.Schedule, error)
    BulkCreate(ctx context.Context, req *models.BulkCreateSchedulesRequest) (int, error)
    CheckConflicts(ctx context.Context, schedule *models.CreateScheduleRequest) ([]models.ScheduleConflictError, error)
    GetClassSchedule(ctx context.Context, classID, termID string) (map[int]map[int]*models.Schedule, error)
    GetTeacherSchedule(ctx context.Context, teacherID, termID string) (map[int]map[int]*models.Schedule, error)
}
```

### Handler Layer Pattern

```go
// Example: internal/handler/class_handler.go
type ClassHandler struct {
    classService service.ClassService
}

func (h *ClassHandler) AssignSubjects(c *gin.Context) {
    classID := c.Param("id")

    var req models.AssignSubjectsRequest
    if err := c.ShouldBindJSON(&req); err != nil {
        c.JSON(400, gin.H{"error": err.Error()})
        return
    }

    err := h.classService.AssignSubjects(c.Request.Context(), classID, req.Subjects)
    if err != nil {
        c.JSON(500, gin.H{"error": err.Error()})
        return
    }

    c.JSON(200, gin.H{"message": "Subjects assigned successfully"})
}
```

---

## 2.5 Week 6-8 Task Breakdown

### Week 6: Terms & Subjects

- [ ] Create Terms repository, service, and handlers
- [ ] Implement active term management logic
- [ ] Create Subjects repository, service, and handlers
- [ ] Implement track/group filtering
- [ ] Write unit tests for Terms and Subjects
- [ ] Update Swagger documentation
- [ ] Frontend integration testing

### Week 7: Classes & Assignments

- [ ] Create Classes repository, service, and handlers
- [ ] Implement homeroom teacher assignment
- [ ] Create Class-Subject mapping logic
- [ ] Implement bulk subject assignment
- [ ] Write unit tests for Classes
- [ ] Integration tests for class-subject mapping
- [ ] Update Swagger docs

### Week 8: Schedules & Conflict Detection

- [ ] Create Schedules repository, service, and handlers
- [ ] Implement conflict detection algorithm (class/teacher/room)
- [ ] Create bulk schedule creation logic
- [ ] Implement schedule view APIs (class/teacher perspectives)
- [ ] Write comprehensive unit tests
- [ ] Integration tests for conflict scenarios
- [ ] Performance optimization (indexed queries)
- [ ] Full frontend integration

---

## 2.6 Migration Strategy

### Database Migration

```sql
-- migrations/000003_add_academic_indexes.up.sql
-- Add performance indexes for academic queries

CREATE INDEX idx_schedules_term_class ON schedules(term_id, class_id);
CREATE INDEX idx_schedules_term_teacher ON schedules(term_id, teacher_id);
CREATE INDEX idx_class_subjects_class_subject ON class_subjects(class_id, subject_id);

-- Add constraint for unique term activation
CREATE UNIQUE INDEX idx_terms_active ON terms(is_active) WHERE is_active = true;
```

### Frontend Migration

1. Update API endpoints in `apps/admin/src/config/api.ts`
2. Feature flag: `USE_GO_ACADEMIC=true`
3. Gradual rollout per module:
   - Week 6: Terms & Subjects (20% users)
   - Week 7: Classes (50% users)
   - Week 8: Schedules (100% users)

---

## 2.7 Success Criteria

- [ ] All academic endpoints return < 150ms response time
- [ ] Schedule conflict detection has 100% accuracy
- [ ] No duplicate schedules in production data
- [ ] Bulk operations handle 100+ items efficiently
- [ ] 90%+ test coverage for critical paths
- [ ] Zero data loss during migration
- [ ] API documentation complete with examples
- [ ] Frontend fully integrated and tested

---

**Next Phase**: [Phase 3: Student Management & Assessment](./PHASE_3_STUDENT_ASSESSMENT.md)

**Previous Phase**: [Phase 1: Authentication & User Management](./PHASE_1_AUTH_USER_MANAGEMENT.md)
