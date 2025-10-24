# Phase 4: Attendance & Communication (Week 12-14)

## 🎯 Objectives

- Implement daily attendance tracking
- Subject-specific attendance per session
- Announcements system with audience targeting
- Behavior notes management
- Calendar events scheduling
- Build upon previous phases (auth, academic, student)

## Prerequisites

- ✅ Phase 0: Infrastructure setup complete
- ✅ Phase 1: Authentication & User Management operational
- ✅ Phase 2: Academic Management complete
- ✅ Phase 3: Student Management & Assessment complete
- ✅ Enrollment data available

---

## 4.1 Database Models

### Daily Attendance Table (Existing Schema)

```sql
-- Daily attendance tracking (per student per day)
CREATE TABLE daily_attendance (
    id VARCHAR(255) PRIMARY KEY,
    enrollment_id VARCHAR(255) NOT NULL REFERENCES enrollments(id) ON DELETE CASCADE,
    date DATE NOT NULL,
    status VARCHAR(10) NOT NULL, -- 'H' (Hadir), 'S' (Sakit), 'I' (Izin), 'A' (Alpha)
    notes TEXT,
    marked_by VARCHAR(255) REFERENCES users(id) ON DELETE SET NULL,
    marked_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(enrollment_id, date)
);

CREATE INDEX idx_daily_attendance_enrollment ON daily_attendance(enrollment_id);
CREATE INDEX idx_daily_attendance_date ON daily_attendance(date);
CREATE INDEX idx_daily_attendance_status ON daily_attendance(status);
CREATE INDEX idx_daily_attendance_enrollment_date ON daily_attendance(enrollment_id, date);
```

### Subject Attendance Table (Existing Schema)

```sql
-- Attendance per subject session
CREATE TABLE subject_attendance (
    id VARCHAR(255) PRIMARY KEY,
    enrollment_id VARCHAR(255) NOT NULL REFERENCES enrollments(id) ON DELETE CASCADE,
    schedule_id VARCHAR(255) NOT NULL REFERENCES schedules(id) ON DELETE CASCADE,
    date DATE NOT NULL,
    status VARCHAR(10) NOT NULL, -- 'H', 'S', 'I', 'A'
    notes TEXT,
    marked_by VARCHAR(255) REFERENCES users(id) ON DELETE SET NULL,
    marked_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(enrollment_id, schedule_id, date)
);

CREATE INDEX idx_subject_attendance_enrollment ON subject_attendance(enrollment_id);
CREATE INDEX idx_subject_attendance_schedule ON subject_attendance(schedule_id);
CREATE INDEX idx_subject_attendance_date ON subject_attendance(date);
CREATE INDEX idx_subject_attendance_status ON subject_attendance(status);
```

### Announcements Table (Existing Schema)

```sql
-- System announcements
CREATE TABLE announcements (
    id VARCHAR(255) PRIMARY KEY,
    title VARCHAR(255) NOT NULL,
    content TEXT NOT NULL,
    audience VARCHAR(50) NOT NULL, -- 'ALL', 'GURU', 'SISWA', 'CLASS'
    target_class_id VARCHAR(255) REFERENCES classes(id) ON DELETE SET NULL,
    priority VARCHAR(20) DEFAULT 'NORMAL', -- 'LOW', 'NORMAL', 'HIGH', 'URGENT'
    is_pinned BOOLEAN DEFAULT false,
    published_by VARCHAR(255) REFERENCES users(id) ON DELETE SET NULL,
    published_at TIMESTAMP,
    expires_at TIMESTAMP,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_announcements_audience ON announcements(audience);
CREATE INDEX idx_announcements_target_class ON announcements(target_class_id);
CREATE INDEX idx_announcements_published_at ON announcements(published_at DESC);
CREATE INDEX idx_announcements_pinned ON announcements(is_pinned);
```

### Behavior Notes Table (Existing Schema)

```sql
-- Student behavior tracking
CREATE TABLE behavior_notes (
    id VARCHAR(255) PRIMARY KEY,
    student_id VARCHAR(255) NOT NULL REFERENCES students(id) ON DELETE CASCADE,
    category VARCHAR(50) NOT NULL, -- 'POSITIVE', 'NEGATIVE', 'NEUTRAL'
    title VARCHAR(255) NOT NULL,
    description TEXT NOT NULL,
    date DATE DEFAULT CURRENT_DATE,
    points INTEGER DEFAULT 0, -- Positive or negative points
    reported_by VARCHAR(255) REFERENCES users(id) ON DELETE SET NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_behavior_notes_student ON behavior_notes(student_id);
CREATE INDEX idx_behavior_notes_category ON behavior_notes(category);
CREATE INDEX idx_behavior_notes_date ON behavior_notes(date DESC);
CREATE INDEX idx_behavior_notes_reported_by ON behavior_notes(reported_by);
```

### Calendar Events Table (Existing Schema)

```sql
-- School calendar events
CREATE TABLE calendar_events (
    id VARCHAR(255) PRIMARY KEY,
    title VARCHAR(255) NOT NULL,
    description TEXT,
    event_type VARCHAR(50) NOT NULL, -- 'EXAM', 'HOLIDAY', 'MEETING', 'ACTIVITY', 'OTHER'
    start_date DATE NOT NULL,
    end_date DATE NOT NULL,
    start_time TIME,
    end_time TIME,
    location VARCHAR(255),
    audience VARCHAR(50) DEFAULT 'ALL', -- 'ALL', 'GURU', 'SISWA', 'CLASS'
    target_class_id VARCHAR(255) REFERENCES classes(id) ON DELETE SET NULL,
    created_by VARCHAR(255) REFERENCES users(id) ON DELETE SET NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_calendar_events_type ON calendar_events(event_type);
CREATE INDEX idx_calendar_events_dates ON calendar_events(start_date, end_date);
CREATE INDEX idx_calendar_events_audience ON calendar_events(audience);
CREATE INDEX idx_calendar_events_target_class ON calendar_events(target_class_id);
```

---

## 4.2 Go Models & Structs

### internal/models/attendance.go

```go
package models

import "time"

type AttendanceStatus string

const (
    AttendancePresent     AttendanceStatus = "H" // Hadir
    AttendanceSick        AttendanceStatus = "S" // Sakit
    AttendancePermission  AttendanceStatus = "I" // Izin
    AttendanceAbsent      AttendanceStatus = "A" // Alpha
)

type DailyAttendance struct {
    ID           string           `db:"id" json:"id"`
    EnrollmentID string           `db:"enrollment_id" json:"enrollmentId"`
    Date         time.Time        `db:"date" json:"date"`
    Status       AttendanceStatus `db:"status" json:"status"`
    Notes        *string          `db:"notes" json:"notes,omitempty"`
    MarkedBy     *string          `db:"marked_by" json:"markedBy,omitempty"`
    MarkedAt     *time.Time       `db:"marked_at" json:"markedAt,omitempty"`
    Student      *Student         `json:"student,omitempty"` // Joined data
    CreatedAt    time.Time        `db:"created_at" json:"createdAt"`
    UpdatedAt    time.Time        `db:"updated_at" json:"updatedAt"`
}

type SubjectAttendance struct {
    ID           string           `db:"id" json:"id"`
    EnrollmentID string           `db:"enrollment_id" json:"enrollmentId"`
    ScheduleID   string           `db:"schedule_id" json:"scheduleId"`
    Date         time.Time        `db:"date" json:"date"`
    Status       AttendanceStatus `db:"status" json:"status"`
    Notes        *string          `db:"notes" json:"notes,omitempty"`
    MarkedBy     *string          `db:"marked_by" json:"markedBy,omitempty"`
    MarkedAt     *time.Time       `db:"marked_at" json:"markedAt,omitempty"`
    Student      *Student         `json:"student,omitempty"`  // Joined data
    Schedule     *Schedule        `json:"schedule,omitempty"` // Joined data
    CreatedAt    time.Time        `db:"created_at" json:"createdAt"`
    UpdatedAt    time.Time        `db:"updated_at" json:"updatedAt"`
}

// Request DTOs
type MarkDailyAttendanceRequest struct {
    EnrollmentID string           `json:"enrollmentId" binding:"required"`
    Date         string           `json:"date" binding:"required"` // YYYY-MM-DD
    Status       AttendanceStatus `json:"status" binding:"required,oneof=H S I A"`
    Notes        *string          `json:"notes"`
}

type BulkMarkDailyAttendanceRequest struct {
    Date        string                      `json:"date" binding:"required"`
    Attendances []BulkAttendanceEntry       `json:"attendances" binding:"required,min=1"`
}

type BulkAttendanceEntry struct {
    EnrollmentID string           `json:"enrollmentId" binding:"required"`
    Status       AttendanceStatus `json:"status" binding:"required,oneof=H S I A"`
    Notes        *string          `json:"notes"`
}

type MarkSubjectAttendanceRequest struct {
    EnrollmentID string           `json:"enrollmentId" binding:"required"`
    ScheduleID   string           `json:"scheduleId" binding:"required"`
    Date         string           `json:"date" binding:"required"`
    Status       AttendanceStatus `json:"status" binding:"required,oneof=H S I A"`
    Notes        *string          `json:"notes"`
}

// Response DTOs
type AttendanceSummary struct {
    StudentID   string  `json:"studentId"`
    StudentName string  `json:"studentName"`
    NIS         string  `json:"nis"`
    TotalDays   int     `json:"totalDays"`
    Present     int     `json:"present"`
    Sick        int     `json:"sick"`
    Permission  int     `json:"permission"`
    Absent      int     `json:"absent"`
    Percentage  float64 `json:"percentage"`
}

type ClassAttendanceReport struct {
    ClassID   string              `json:"classId"`
    ClassName string              `json:"className"`
    Date      string              `json:"date"`
    Students  []StudentAttendance `json:"students"`
    Summary   AttendanceStats     `json:"summary"`
}

type StudentAttendance struct {
    StudentID   string           `json:"studentId"`
    StudentName string           `json:"studentName"`
    NIS         string           `json:"nis"`
    Status      AttendanceStatus `json:"status"`
    Notes       *string          `json:"notes,omitempty"`
}

type AttendanceStats struct {
    Total      int     `json:"total"`
    Present    int     `json:"present"`
    Sick       int     `json:"sick"`
    Permission int     `json:"permission"`
    Absent     int     `json:"absent"`
    Percentage float64 `json:"percentage"`
}
```

### internal/models/announcement.go

```go
package models

import "time"

type AudienceType string
type AnnouncementPriority string

const (
    AudienceAll     AudienceType = "ALL"
    AudienceTeacher AudienceType = "GURU"
    AudienceStudent AudienceType = "SISWA"
    AudienceClass   AudienceType = "CLASS"

    PriorityLow    AnnouncementPriority = "LOW"
    PriorityNormal AnnouncementPriority = "NORMAL"
    PriorityHigh   AnnouncementPriority = "HIGH"
    PriorityUrgent AnnouncementPriority = "URGENT"
)

type Announcement struct {
    ID            string               `db:"id" json:"id"`
    Title         string               `db:"title" json:"title"`
    Content       string               `db:"content" json:"content"`
    Audience      AudienceType         `db:"audience" json:"audience"`
    TargetClassID *string              `db:"target_class_id" json:"targetClassId,omitempty"`
    Priority      AnnouncementPriority `db:"priority" json:"priority"`
    IsPinned      bool                 `db:"is_pinned" json:"isPinned"`
    PublishedBy   *string              `db:"published_by" json:"publishedBy,omitempty"`
    PublishedAt   *time.Time           `db:"published_at" json:"publishedAt,omitempty"`
    ExpiresAt     *time.Time           `db:"expires_at" json:"expiresAt,omitempty"`
    Publisher     *UserInfo            `json:"publisher,omitempty"` // Joined data
    TargetClass   *Class               `json:"targetClass,omitempty"` // Joined data
    CreatedAt     time.Time            `db:"created_at" json:"createdAt"`
    UpdatedAt     time.Time            `db:"updated_at" json:"updatedAt"`
}

// Request DTOs
type CreateAnnouncementRequest struct {
    Title         string               `json:"title" binding:"required"`
    Content       string               `json:"content" binding:"required"`
    Audience      AudienceType         `json:"audience" binding:"required,oneof=ALL GURU SISWA CLASS"`
    TargetClassID *string              `json:"targetClassId"`
    Priority      AnnouncementPriority `json:"priority" binding:"omitempty,oneof=LOW NORMAL HIGH URGENT"`
    IsPinned      bool                 `json:"isPinned"`
    ExpiresAt     *string              `json:"expiresAt"` // ISO 8601
}

type UpdateAnnouncementRequest struct {
    Title     *string               `json:"title"`
    Content   *string               `json:"content"`
    Priority  *AnnouncementPriority `json:"priority"`
    IsPinned  *bool                 `json:"isPinned"`
    ExpiresAt *string               `json:"expiresAt"`
}
```

### internal/models/behavior.go

```go
package models

import "time"

type BehaviorCategory string

const (
    BehaviorPositive BehaviorCategory = "POSITIVE"
    BehaviorNegative BehaviorCategory = "NEGATIVE"
    BehaviorNeutral  BehaviorCategory = "NEUTRAL"
)

type BehaviorNote struct {
    ID          string           `db:"id" json:"id"`
    StudentID   string           `db:"student_id" json:"studentId"`
    Category    BehaviorCategory `db:"category" json:"category"`
    Title       string           `db:"title" json:"title"`
    Description string           `db:"description" json:"description"`
    Date        time.Time        `db:"date" json:"date"`
    Points      int              `db:"points" json:"points"`
    ReportedBy  *string          `db:"reported_by" json:"reportedBy,omitempty"`
    Student     *Student         `json:"student,omitempty"` // Joined data
    Reporter    *UserInfo        `json:"reporter,omitempty"` // Joined data
    CreatedAt   time.Time        `db:"created_at" json:"createdAt"`
    UpdatedAt   time.Time        `db:"updated_at" json:"updatedAt"`
}

// Request DTOs
type CreateBehaviorNoteRequest struct {
    StudentID   string           `json:"studentId" binding:"required"`
    Category    BehaviorCategory `json:"category" binding:"required,oneof=POSITIVE NEGATIVE NEUTRAL"`
    Title       string           `json:"title" binding:"required"`
    Description string           `json:"description" binding:"required"`
    Date        *string          `json:"date"` // YYYY-MM-DD, defaults to today
    Points      int              `json:"points"`
}

type UpdateBehaviorNoteRequest struct {
    Category    *BehaviorCategory `json:"category"`
    Title       *string           `json:"title"`
    Description *string           `json:"description"`
    Points      *int              `json:"points"`
}

// Response DTOs
type StudentBehaviorSummary struct {
    StudentID      string `json:"studentId"`
    StudentName    string `json:"studentName"`
    NIS            string `json:"nis"`
    TotalNotes     int    `json:"totalNotes"`
    PositiveNotes  int    `json:"positiveNotes"`
    NegativeNotes  int    `json:"negativeNotes"`
    NeutralNotes   int    `json:"neutralNotes"`
    TotalPoints    int    `json:"totalPoints"`
}
```

### internal/models/calendar.go

```go
package models

import "time"

type EventType string

const (
    EventExam     EventType = "EXAM"
    EventHoliday  EventType = "HOLIDAY"
    EventMeeting  EventType = "MEETING"
    EventActivity EventType = "ACTIVITY"
    EventOther    EventType = "OTHER"
)

type CalendarEvent struct {
    ID            string       `db:"id" json:"id"`
    Title         string       `db:"title" json:"title"`
    Description   *string      `db:"description" json:"description,omitempty"`
    EventType     EventType    `db:"event_type" json:"eventType"`
    StartDate     time.Time    `db:"start_date" json:"startDate"`
    EndDate       time.Time    `db:"end_date" json:"endDate"`
    StartTime     *string      `db:"start_time" json:"startTime,omitempty"` // HH:MM
    EndTime       *string      `db:"end_time" json:"endTime,omitempty"`
    Location      *string      `db:"location" json:"location,omitempty"`
    Audience      AudienceType `db:"audience" json:"audience"`
    TargetClassID *string      `db:"target_class_id" json:"targetClassId,omitempty"`
    CreatedBy     *string      `db:"created_by" json:"createdBy,omitempty"`
    Creator       *UserInfo    `json:"creator,omitempty"` // Joined data
    TargetClass   *Class       `json:"targetClass,omitempty"` // Joined data
    CreatedAt     time.Time    `db:"created_at" json:"createdAt"`
    UpdatedAt     time.Time    `db:"updated_at" json:"updatedAt"`
}

// Request DTOs
type CreateCalendarEventRequest struct {
    Title         string       `json:"title" binding:"required"`
    Description   *string      `json:"description"`
    EventType     EventType    `json:"eventType" binding:"required,oneof=EXAM HOLIDAY MEETING ACTIVITY OTHER"`
    StartDate     string       `json:"startDate" binding:"required"` // YYYY-MM-DD
    EndDate       string       `json:"endDate" binding:"required"`
    StartTime     *string      `json:"startTime"` // HH:MM
    EndTime       *string      `json:"endTime"`
    Location      *string      `json:"location"`
    Audience      AudienceType `json:"audience" binding:"required,oneof=ALL GURU SISWA CLASS"`
    TargetClassID *string      `json:"targetClassId"`
}

type UpdateCalendarEventRequest struct {
    Title       *string   `json:"title"`
    Description *string   `json:"description"`
    EventType   *EventType `json:"eventType"`
    StartDate   *string   `json:"startDate"`
    EndDate     *string   `json:"endDate"`
    StartTime   *string   `json:"startTime"`
    EndTime     *string   `json:"endTime"`
    Location    *string   `json:"location"`
}
```

---

## 4.3 API Endpoints Specification

### Base URL

```
Development: http://localhost:8080/api/v1
Production:  https://api.yourdomain.com/api/v1
```

---

## Daily Attendance Management

### 1. GET /attendance/daily

**Description**: List daily attendance records with filters

**Headers:**

```
Authorization: Bearer {accessToken}
```

**Query Parameters:**

- `classId` (string, required): Filter by class
- `termId` (string, required): Filter by term
- `startDate` (string, optional): Start date (YYYY-MM-DD)
- `endDate` (string, optional): End date (YYYY-MM-DD)
- `status` (string, optional): Filter by status (H/S/I/A)

**Response (200 OK):**

```json
{
  "data": [
    {
      "id": "att_daily_001",
      "enrollmentId": "enr_abc123",
      "date": "2024-10-24T00:00:00Z",
      "status": "H",
      "notes": null,
      "markedBy": "usr_teacher01",
      "markedAt": "2024-10-24T07:30:00Z",
      "student": {
        "id": "std_abc123",
        "nis": "2024001",
        "fullName": "Ahmad Fauzi"
      },
      "createdAt": "2024-10-24T07:30:00Z"
    }
  ]
}
```

**Permissions**: TEACHER (homeroom), ADMIN, SUPERADMIN

---

### 2. POST /attendance/daily

**Description**: Mark single student's daily attendance

**Headers:**

```
Authorization: Bearer {accessToken}
```

**Request:**

```json
{
  "enrollmentId": "enr_abc123",
  "date": "2024-10-24",
  "status": "H",
  "notes": null
}
```

**Response (201 Created):**

```json
{
  "id": "att_daily_001",
  "enrollmentId": "enr_abc123",
  "date": "2024-10-24T00:00:00Z",
  "status": "H",
  "markedBy": "usr_teacher01",
  "markedAt": "2024-10-24T07:30:00Z"
}
```

**Permissions**: TEACHER (homeroom), ADMIN, SUPERADMIN

---

### 3. POST /attendance/daily/bulk

**Description**: Mark attendance for multiple students at once

**Headers:**

```
Authorization: Bearer {accessToken}
```

**Request:**

```json
{
  "date": "2024-10-24",
  "attendances": [
    {
      "enrollmentId": "enr_abc123",
      "status": "H"
    },
    {
      "enrollmentId": "enr_def456",
      "status": "S",
      "notes": "Flu"
    },
    {
      "enrollmentId": "enr_ghi789",
      "status": "A"
    }
  ]
}
```

**Response (200 OK):**

```json
{
  "message": "Bulk attendance marked successfully",
  "marked": 3,
  "failed": 0,
  "errors": []
}
```

**Permissions**: TEACHER (homeroom), ADMIN, SUPERADMIN

---

### 4. PUT /attendance/daily/:id

**Description**: Update daily attendance record

**Headers:**

```
Authorization: Bearer {accessToken}
```

**Request:**

```json
{
  "status": "S",
  "notes": "Sakit demam"
}
```

**Response (200 OK):**

```json
{
  "id": "att_daily_001",
  "status": "S",
  "notes": "Sakit demam",
  "updatedAt": "2024-10-24T08:00:00Z"
}
```

**Permissions**: TEACHER (homeroom), ADMIN, SUPERADMIN

---

### 5. GET /attendance/daily/class/:classId

**Description**: Get daily attendance report for a class

**Headers:**

```
Authorization: Bearer {accessToken}
```

**Query Parameters:**

- `date` (string, required): Date (YYYY-MM-DD)
- `termId` (string, required): Term ID

**Response (200 OK):**

```json
{
  "classId": "cls_xipa1",
  "className": "X IPA 1",
  "date": "2024-10-24",
  "students": [
    {
      "studentId": "std_abc123",
      "studentName": "Ahmad Fauzi",
      "nis": "2024001",
      "status": "H",
      "notes": null
    },
    {
      "studentId": "std_def456",
      "studentName": "Siti Aminah",
      "nis": "2024002",
      "status": "S",
      "notes": "Flu"
    }
  ],
  "summary": {
    "total": 30,
    "present": 28,
    "sick": 1,
    "permission": 0,
    "absent": 1,
    "percentage": 93.33
  }
}
```

**Permissions**: TEACHER, ADMIN, SUPERADMIN

---

### 6. GET /attendance/daily/student/:studentId

**Description**: Get attendance history for a student

**Headers:**

```
Authorization: Bearer {accessToken}
```

**Query Parameters:**

- `termId` (string, required): Term ID
- `startDate` (string, optional): Start date
- `endDate` (string, optional): End date

**Response (200 OK):**

```json
{
  "studentId": "std_abc123",
  "studentName": "Ahmad Fauzi",
  "nis": "2024001",
  "termId": "term_2024_1",
  "summary": {
    "totalDays": 60,
    "present": 55,
    "sick": 3,
    "permission": 1,
    "absent": 1,
    "percentage": 91.67
  },
  "records": [
    {
      "date": "2024-10-24",
      "status": "H"
    },
    {
      "date": "2024-10-23",
      "status": "S",
      "notes": "Flu"
    }
  ]
}
```

**Permissions**: TEACHER, ADMIN, SUPERADMIN

---

## Subject Attendance Management

### 7. POST /attendance/subject

**Description**: Mark subject attendance for a session

**Headers:**

```
Authorization: Bearer {accessToken}
```

**Request:**

```json
{
  "enrollmentId": "enr_abc123",
  "scheduleId": "sch_001",
  "date": "2024-10-24",
  "status": "H"
}
```

**Response (201 Created):**

```json
{
  "id": "att_subj_001",
  "enrollmentId": "enr_abc123",
  "scheduleId": "sch_001",
  "date": "2024-10-24T00:00:00Z",
  "status": "H",
  "markedBy": "usr_teacher04",
  "markedAt": "2024-10-24T08:00:00Z"
}
```

**Permissions**: TEACHER (for own subjects), ADMIN, SUPERADMIN

---

### 8. POST /attendance/subject/bulk

**Description**: Mark subject attendance for multiple students in a session

**Headers:**

```
Authorization: Bearer {accessToken}
```

**Request:**

```json
{
  "scheduleId": "sch_001",
  "date": "2024-10-24",
  "attendances": [
    {
      "enrollmentId": "enr_abc123",
      "status": "H"
    },
    {
      "enrollmentId": "enr_def456",
      "status": "H"
    }
  ]
}
```

**Response (200 OK):**

```json
{
  "message": "Subject attendance marked successfully",
  "marked": 2,
  "failed": 0
}
```

**Permissions**: TEACHER (for own subjects), ADMIN, SUPERADMIN

---

### 9. GET /attendance/subject/schedule/:scheduleId

**Description**: Get subject attendance for a specific session

**Headers:**

```
Authorization: Bearer {accessToken}
```

**Query Parameters:**

- `date` (string, required): Date (YYYY-MM-DD)

**Response (200 OK):**

```json
{
  "scheduleId": "sch_001",
  "subject": "Matematika",
  "class": "X IPA 1",
  "date": "2024-10-24",
  "students": [
    {
      "studentId": "std_abc123",
      "studentName": "Ahmad Fauzi",
      "status": "H"
    }
  ],
  "summary": {
    "total": 30,
    "present": 29,
    "absent": 1,
    "percentage": 96.67
  }
}
```

**Permissions**: TEACHER (for own subjects), ADMIN, SUPERADMIN

---

## Announcements Management

### 10. GET /announcements

**Description**: List announcements with filters

**Headers:**

```
Authorization: Bearer {accessToken}
```

**Query Parameters:**

- `page` (integer, default: 1)
- `limit` (integer, default: 20)
- `audience` (string, optional): Filter by audience
- `priority` (string, optional): Filter by priority
- `isPinned` (boolean, optional): Filter pinned
- `active` (boolean, optional): Filter active (not expired)

**Response (200 OK):**

```json
{
  "data": [
    {
      "id": "ann_001",
      "title": "Libur Hari Raya",
      "content": "Sekolah libur tanggal 1-7 Mei 2024",
      "audience": "ALL",
      "priority": "HIGH",
      "isPinned": true,
      "publishedBy": "usr_admin01",
      "publishedAt": "2024-10-20T10:00:00Z",
      "expiresAt": "2024-05-08T00:00:00Z",
      "publisher": {
        "id": "usr_admin01",
        "fullName": "Admin Sekolah"
      },
      "createdAt": "2024-10-20T10:00:00Z"
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

**Permissions**: All authenticated users

---

### 11. GET /announcements/:id

**Description**: Get announcement by ID

**Headers:**

```
Authorization: Bearer {accessToken}
```

**Response (200 OK):**

```json
{
  "id": "ann_001",
  "title": "Libur Hari Raya",
  "content": "Sekolah libur tanggal 1-7 Mei 2024",
  "audience": "ALL",
  "priority": "HIGH",
  "isPinned": true,
  "publishedBy": "usr_admin01",
  "publishedAt": "2024-10-20T10:00:00Z",
  "expiresAt": "2024-05-08T00:00:00Z",
  "publisher": {
    "fullName": "Admin Sekolah"
  }
}
```

**Permissions**: All authenticated users

---

### 12. POST /announcements

**Description**: Create announcement

**Headers:**

```
Authorization: Bearer {accessToken}
```

**Request:**

```json
{
  "title": "Pengumuman Ujian",
  "content": "Ujian tengah semester akan dilaksanakan...",
  "audience": "SISWA",
  "priority": "HIGH",
  "isPinned": false,
  "expiresAt": "2024-11-30T23:59:59Z"
}
```

**Response (201 Created):**

```json
{
  "id": "ann_new001",
  "title": "Pengumuman Ujian",
  "publishedBy": "usr_admin01",
  "publishedAt": "2024-10-24T10:00:00Z"
}
```

**Permissions**: ADMIN, SUPERADMIN

---

### 13. PUT /announcements/:id

**Description**: Update announcement

**Headers:**

```
Authorization: Bearer {accessToken}
```

**Request:**

```json
{
  "title": "Pengumuman Ujian (Updated)",
  "isPinned": true
}
```

**Response (200 OK):**

```json
{
  "id": "ann_001",
  "title": "Pengumuman Ujian (Updated)",
  "updatedAt": "2024-10-24T11:00:00Z"
}
```

**Permissions**: ADMIN, SUPERADMIN

---

### 14. DELETE /announcements/:id

**Description**: Delete announcement

**Headers:**

```
Authorization: Bearer {accessToken}
```

**Response (200 OK):**

```json
{
  "message": "Announcement deleted successfully"
}
```

**Permissions**: ADMIN, SUPERADMIN

---

## Behavior Notes Management

### 15. GET /behavior-notes

**Description**: List behavior notes with filters

**Headers:**

```
Authorization: Bearer {accessToken}
```

**Query Parameters:**

- `studentId` (string, optional): Filter by student
- `category` (string, optional): Filter by category
- `startDate` (string, optional): Start date
- `endDate` (string, optional): End date
- `page` (integer, default: 1)
- `limit` (integer, default: 20)

**Response (200 OK):**

```json
{
  "data": [
    {
      "id": "beh_001",
      "studentId": "std_abc123",
      "category": "POSITIVE",
      "title": "Juara Lomba Matematika",
      "description": "Mendapat juara 1 olimpiade matematika tingkat kota",
      "date": "2024-10-20T00:00:00Z",
      "points": 10,
      "reportedBy": "usr_teacher01",
      "student": {
        "id": "std_abc123",
        "fullName": "Ahmad Fauzi",
        "nis": "2024001"
      },
      "reporter": {
        "fullName": "Siti Aminah"
      },
      "createdAt": "2024-10-20T14:00:00Z"
    }
  ],
  "pagination": {
    "page": 1,
    "limit": 20,
    "total": 5,
    "totalPages": 1
  }
}
```

**Permissions**: TEACHER, ADMIN, SUPERADMIN

---

### 16. GET /behavior-notes/student/:studentId

**Description**: Get behavior notes for a specific student

**Headers:**

```
Authorization: Bearer {accessToken}
```

**Query Parameters:**

- `termId` (string, optional): Filter by term

**Response (200 OK):**

```json
{
  "studentId": "std_abc123",
  "studentName": "Ahmad Fauzi",
  "nis": "2024001",
  "summary": {
    "totalNotes": 8,
    "positiveNotes": 5,
    "negativeNotes": 2,
    "neutralNotes": 1,
    "totalPoints": 35
  },
  "notes": [
    {
      "id": "beh_001",
      "category": "POSITIVE",
      "title": "Juara Lomba",
      "date": "2024-10-20",
      "points": 10
    }
  ]
}
```

**Permissions**: TEACHER, ADMIN, SUPERADMIN

---

### 17. POST /behavior-notes

**Description**: Create behavior note

**Headers:**

```
Authorization: Bearer {accessToken}
```

**Request:**

```json
{
  "studentId": "std_abc123",
  "category": "POSITIVE",
  "title": "Membantu Teman",
  "description": "Membantu teman yang kesulitan memahami pelajaran",
  "date": "2024-10-24",
  "points": 5
}
```

**Response (201 Created):**

```json
{
  "id": "beh_new001",
  "studentId": "std_abc123",
  "category": "POSITIVE",
  "title": "Membantu Teman",
  "points": 5,
  "reportedBy": "usr_teacher01",
  "createdAt": "2024-10-24T10:00:00Z"
}
```

**Permissions**: TEACHER, ADMIN, SUPERADMIN

---

### 18. PUT /behavior-notes/:id

**Description**: Update behavior note

**Headers:**

```
Authorization: Bearer {accessToken}
```

**Request:**

```json
{
  "title": "Membantu Teman (Updated)",
  "points": 7
}
```

**Response (200 OK):**

```json
{
  "id": "beh_001",
  "title": "Membantu Teman (Updated)",
  "points": 7,
  "updatedAt": "2024-10-24T11:00:00Z"
}
```

**Permissions**: TEACHER, ADMIN, SUPERADMIN

---

### 19. DELETE /behavior-notes/:id

**Description**: Delete behavior note

**Headers:**

```
Authorization: Bearer {accessToken}
```

**Response (200 OK):**

```json
{
  "message": "Behavior note deleted successfully"
}
```

**Permissions**: ADMIN, SUPERADMIN

---

## Calendar Events Management

### 20. GET /calendar-events

**Description**: List calendar events

**Headers:**

```
Authorization: Bearer {accessToken}
```

**Query Parameters:**

- `startDate` (string, optional): Filter start date
- `endDate` (string, optional): Filter end date
- `eventType` (string, optional): Filter by type
- `audience` (string, optional): Filter by audience
- `month` (string, optional): Filter by month (YYYY-MM)

**Response (200 OK):**

```json
{
  "data": [
    {
      "id": "evt_001",
      "title": "Ujian Tengah Semester",
      "description": "UTS untuk semua kelas",
      "eventType": "EXAM",
      "startDate": "2024-11-01T00:00:00Z",
      "endDate": "2024-11-07T00:00:00Z",
      "startTime": "08:00",
      "endTime": "12:00",
      "location": "Ruang Kelas",
      "audience": "SISWA",
      "createdBy": "usr_admin01",
      "creator": {
        "fullName": "Admin Sekolah"
      },
      "createdAt": "2024-10-15T10:00:00Z"
    }
  ]
}
```

**Permissions**: All authenticated users

---

### 21. GET /calendar-events/:id

**Description**: Get event by ID

**Headers:**

```
Authorization: Bearer {accessToken}
```

**Response (200 OK):**

```json
{
  "id": "evt_001",
  "title": "Ujian Tengah Semester",
  "description": "UTS untuk semua kelas",
  "eventType": "EXAM",
  "startDate": "2024-11-01T00:00:00Z",
  "endDate": "2024-11-07T00:00:00Z",
  "startTime": "08:00",
  "endTime": "12:00",
  "location": "Ruang Kelas",
  "audience": "SISWA"
}
```

**Permissions**: All authenticated users

---

### 22. POST /calendar-events

**Description**: Create calendar event

**Headers:**

```
Authorization: Bearer {accessToken}
```

**Request:**

```json
{
  "title": "Rapat Guru",
  "description": "Rapat evaluasi pembelajaran",
  "eventType": "MEETING",
  "startDate": "2024-10-30",
  "endDate": "2024-10-30",
  "startTime": "14:00",
  "endTime": "16:00",
  "location": "Ruang Guru",
  "audience": "GURU"
}
```

**Response (201 Created):**

```json
{
  "id": "evt_new001",
  "title": "Rapat Guru",
  "eventType": "MEETING",
  "createdBy": "usr_admin01",
  "createdAt": "2024-10-24T10:00:00Z"
}
```

**Permissions**: ADMIN, SUPERADMIN

---

### 23. PUT /calendar-events/:id

**Description**: Update calendar event

**Headers:**

```
Authorization: Bearer {accessToken}
```

**Request:**

```json
{
  "title": "Rapat Guru (Updated)",
  "location": "Aula"
}
```

**Response (200 OK):**

```json
{
  "id": "evt_001",
  "title": "Rapat Guru (Updated)",
  "location": "Aula",
  "updatedAt": "2024-10-24T11:00:00Z"
}
```

**Permissions**: ADMIN, SUPERADMIN

---

### 24. DELETE /calendar-events/:id

**Description**: Delete calendar event

**Headers:**

```
Authorization: Bearer {accessToken}
```

**Response (200 OK):**

```json
{
  "message": "Calendar event deleted successfully"
}
```

**Permissions**: ADMIN, SUPERADMIN

---

## 4.4 Implementation Guidelines

### Attendance Service Pattern

```go
// Example: internal/service/attendance_service.go
type AttendanceService interface {
    // Daily attendance
    MarkDailyAttendance(ctx context.Context, req *models.MarkDailyAttendanceRequest, userID string) (*models.DailyAttendance, error)
    BulkMarkDailyAttendance(ctx context.Context, req *models.BulkMarkDailyAttendanceRequest, userID string) (int, error)
    GetClassAttendanceReport(ctx context.Context, classID, termID, date string) (*models.ClassAttendanceReport, error)
    GetStudentAttendanceSummary(ctx context.Context, studentID, termID string) (*models.AttendanceSummary, error)

    // Subject attendance
    MarkSubjectAttendance(ctx context.Context, req *models.MarkSubjectAttendanceRequest, userID string) (*models.SubjectAttendance, error)

    // Statistics
    CalculateAttendancePercentage(ctx context.Context, studentID, termID string) (float64, error)
}
```

### Announcement Service Pattern

```go
type AnnouncementService interface {
    List(ctx context.Context, filters AnnouncementFilters, userRole models.UserRole, userID string) ([]*models.Announcement, int, error)
    GetByID(ctx context.Context, id string) (*models.Announcement, error)
    Create(ctx context.Context, req *models.CreateAnnouncementRequest, userID string) (*models.Announcement, error)
    Update(ctx context.Context, id string, req *models.UpdateAnnouncementRequest) (*models.Announcement, error)
    Delete(ctx context.Context, id string) error
    GetActiveForUser(ctx context.Context, userRole models.UserRole, classID *string) ([]*models.Announcement, error)
}
```

---

## 4.5 Week 12-14 Task Breakdown

### Week 12: Attendance System

- [ ] Create DailyAttendance repository, service, handlers
- [ ] Create SubjectAttendance repository, service, handlers
- [ ] Implement bulk attendance marking
- [ ] Create attendance report generation
- [ ] Implement attendance percentage calculation
- [ ] Write unit tests for attendance logic
- [ ] Integration tests for bulk operations
- [ ] Update Swagger documentation

### Week 13: Communication Features

- [ ] Create Announcements repository, service, handlers
- [ ] Implement audience-based filtering
- [ ] Create BehaviorNotes repository, service, handlers
- [ ] Implement behavior summary calculations
- [ ] Write unit tests for communication features
- [ ] Integration tests for announcements
- [ ] Update Swagger docs

### Week 14: Calendar & Integration

- [ ] Create CalendarEvents repository, service, handlers
- [ ] Implement event filtering by date range
- [ ] Add notification hooks (future: email/push)
- [ ] Write comprehensive unit tests
- [ ] Integration tests for calendar
- [ ] Performance optimization (caching)
- [ ] Full frontend integration
- [ ] End-to-end testing

---

## 4.6 Migration Strategy

### Database Migration

```sql
-- migrations/000005_add_attendance_indexes.up.sql

-- Optimize attendance queries
CREATE INDEX idx_daily_attendance_class_date ON daily_attendance(enrollment_id, date);
CREATE INDEX idx_subject_attendance_schedule_date ON subject_attendance(schedule_id, date);

-- Optimize announcement queries
CREATE INDEX idx_announcements_active ON announcements(published_at, expires_at)
  WHERE expires_at IS NULL OR expires_at > NOW();

-- Optimize behavior queries
CREATE INDEX idx_behavior_notes_student_date ON behavior_notes(student_id, date DESC);

-- Optimize calendar queries
CREATE INDEX idx_calendar_events_date_range ON calendar_events(start_date, end_date);
```

### Data Validation Rules

1. **Daily Attendance**: One record per student per day
2. **Subject Attendance**: One record per student per schedule per date
3. **Announcements**: Expire date must be after published date
4. **Calendar Events**: End date must be >= start date
5. **Behavior Points**: Can be positive or negative

### Frontend Migration

1. Update API endpoints for attendance/communication
2. Feature flag: `USE_GO_ATTENDANCE=true`
3. Gradual rollout:
   - Week 12: Attendance (30% users)
   - Week 13: Announcements & Behavior (60% users)
   - Week 14: Calendar & Full rollout (100% users)

---

## 4.7 Success Criteria

- [ ] All attendance endpoints return < 150ms response time
- [ ] Bulk attendance marking handles 100+ students in < 2 seconds
- [ ] Attendance calculation accuracy: 100%
- [ ] Announcements delivered to correct audience
- [ ] Calendar events sync correctly across timezones
- [ ] 85%+ test coverage
- [ ] Zero data loss during migration
- [ ] API documentation complete
- [ ] Frontend fully integrated

---

## 4.8 Future Enhancements (Post-Phase 4)

- **Attendance Notifications**: Auto-notify parents of absences
- **Attendance Analytics**: Identify patterns, at-risk students
- **Announcement Push Notifications**: Real-time alerts
- **Behavior Leaderboard**: Gamification for positive behavior
- **Calendar Integration**: Export to Google Calendar, iCal
- **SMS Notifications**: SMS alerts for critical announcements
- **Attendance QR Code**: Students scan QR to mark attendance
- **Mobile App**: Native mobile app for parents/students

---

**Next Phase**: [Phase 5: Analytics & Optimization](./PHASE_5_ANALYTICS_OPTIMIZATION.md)

**Previous Phases**:

- [Phase 1: Authentication & User Management](./PHASE_1_AUTH_USER_MANAGEMENT.md)
- [Phase 2: Academic Management](./PHASE_2_ACADEMIC_MANAGEMENT.md)
- [Phase 3: Student Management & Assessment](./PHASE_3_STUDENT_ASSESSMENT.md)
