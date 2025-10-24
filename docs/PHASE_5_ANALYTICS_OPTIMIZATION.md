# Phase 5: Analytics & Optimization (Week 15-17)

## 🎯 Objectives

- Build comprehensive analytics dashboards
- Implement real-time performance metrics
- Create grade & attendance statistics
- Deploy caching layer (Redis)
- Optimize database queries
- Set up monitoring & alerting
- Performance tuning for production scale

## Prerequisites

- ✅ Phase 0-4: All previous systems operational
- ✅ Redis cluster configured
- ✅ Metrics collection infrastructure
- ✅ Sufficient production data for meaningful analytics

---

## 5.1 Database Models & Views

### Analytics Summary Tables

```sql
-- Materialized view for class statistics
CREATE MATERIALIZED VIEW mv_class_statistics AS
SELECT
    c.id AS class_id,
    c.name AS class_name,
    t.id AS term_id,
    COUNT(DISTINCT e.student_id) AS total_students,
    COUNT(DISTINCT cs.subject_id) AS total_subjects,
    -- Attendance stats
    ROUND(AVG(CASE WHEN da.status = 'H' THEN 100.0 ELSE 0 END), 2) AS avg_attendance_rate,
    -- Grade stats
    ROUND(AVG(g.final_grade), 2) AS avg_grade,
    COUNT(CASE WHEN g.final_grade >= 75 THEN 1 END) AS students_passed,
    COUNT(CASE WHEN g.final_grade < 75 THEN 1 END) AS students_failed
FROM classes c
CROSS JOIN terms t
LEFT JOIN enrollments e ON e.class_id = c.id AND e.term_id = t.id
LEFT JOIN class_subjects cs ON cs.class_id = c.id
LEFT JOIN daily_attendance da ON da.enrollment_id = e.id
LEFT JOIN grades g ON g.enrollment_id = e.id
WHERE t.is_active = true
GROUP BY c.id, c.name, t.id;

CREATE UNIQUE INDEX idx_mv_class_stats ON mv_class_statistics(class_id, term_id);

-- Refresh schedule: Every hour or on-demand
-- REFRESH MATERIALIZED VIEW CONCURRENTLY mv_class_statistics;

-- Student performance summary
CREATE MATERIALIZED VIEW mv_student_performance AS
SELECT
    s.id AS student_id,
    s.nis,
    s.full_name,
    e.class_id,
    e.term_id,
    -- Attendance metrics
    COUNT(DISTINCT da.date) AS total_attendance_days,
    COUNT(CASE WHEN da.status = 'H' THEN 1 END) AS days_present,
    ROUND(100.0 * COUNT(CASE WHEN da.status = 'H' THEN 1 END) / NULLIF(COUNT(DISTINCT da.date), 0), 2) AS attendance_percentage,
    -- Grade metrics
    COUNT(DISTINCT g.id) AS subjects_enrolled,
    ROUND(AVG(g.final_grade), 2) AS gpa,
    MIN(g.final_grade) AS lowest_grade,
    MAX(g.final_grade) AS highest_grade,
    COUNT(CASE WHEN g.final_grade >= 75 THEN 1 END) AS subjects_passed,
    COUNT(CASE WHEN g.final_grade < 75 THEN 1 END) AS subjects_failed,
    -- Behavior metrics
    COALESCE(SUM(bn.points), 0) AS behavior_points,
    COUNT(CASE WHEN bn.category = 'POSITIVE' THEN 1 END) AS positive_notes,
    COUNT(CASE WHEN bn.category = 'NEGATIVE' THEN 1 END) AS negative_notes
FROM students s
JOIN enrollments e ON e.student_id = s.id
LEFT JOIN daily_attendance da ON da.enrollment_id = e.id
LEFT JOIN grades g ON g.enrollment_id = e.id
LEFT JOIN behavior_notes bn ON bn.student_id = s.id
WHERE e.term_id IN (SELECT id FROM terms WHERE is_active = true)
GROUP BY s.id, s.nis, s.full_name, e.class_id, e.term_id;

CREATE UNIQUE INDEX idx_mv_student_perf ON mv_student_performance(student_id, class_id, term_id);

-- Subject performance analytics
CREATE MATERIALIZED VIEW mv_subject_statistics AS
SELECT
    sub.id AS subject_id,
    sub.name AS subject_name,
    cs.class_id,
    t.id AS term_id,
    COUNT(DISTINCT g.enrollment_id) AS total_students,
    ROUND(AVG(g.final_grade), 2) AS avg_grade,
    STDDEV_POP(g.final_grade) AS grade_stddev,
    MIN(g.final_grade) AS min_grade,
    MAX(g.final_grade) AS max_grade,
    COUNT(CASE WHEN g.final_grade >= 75 THEN 1 END) AS passed_count,
    COUNT(CASE WHEN g.final_grade < 75 THEN 1 END) AS failed_count,
    ROUND(100.0 * COUNT(CASE WHEN g.final_grade >= 75 THEN 1 END) / NULLIF(COUNT(*), 0), 2) AS pass_rate
FROM subjects sub
JOIN class_subjects cs ON cs.subject_id = sub.id
CROSS JOIN terms t
LEFT JOIN enrollments e ON e.class_id = cs.class_id AND e.term_id = t.id
LEFT JOIN grades g ON g.enrollment_id = e.id AND g.subject_id = sub.id
WHERE t.is_active = true
GROUP BY sub.id, sub.name, cs.class_id, t.id;

CREATE UNIQUE INDEX idx_mv_subject_stats ON mv_subject_statistics(subject_id, class_id, term_id);
```

### Cache Keys Schema

```
Redis Key Patterns:
- stats:class:{classId}:{termId}           → Class statistics JSON
- stats:student:{studentId}:{termId}        → Student performance JSON
- stats:subject:{subjectId}:{classId}:{termId} → Subject statistics JSON
- dashboard:overview:{termId}               → Dashboard overview JSON
- leaderboard:gpa:{termId}                  → Top students by GPA (sorted set)
- leaderboard:attendance:{termId}           → Top students by attendance (sorted set)
- cache:announcement:active                 → Active announcements list
- cache:calendar:{month}                    → Calendar events for month

TTL Strategy:
- Statistics: 1 hour (3600s)
- Leaderboards: 30 minutes (1800s)
- Dashboard: 15 minutes (900s)
- Announcements: 5 minutes (300s)
- Calendar: 1 day (86400s)
```

---

## 5.2 Go Models & DTOs

### internal/models/analytics.go

```go
package models

import "time"

// Dashboard Overview
type DashboardOverview struct {
    TermID             string              `json:"termId"`
    TermName           string              `json:"termName"`
    TotalStudents      int                 `json:"totalStudents"`
    TotalTeachers      int                 `json:"totalTeachers"`
    TotalClasses       int                 `json:"totalClasses"`
    TotalSubjects      int                 `json:"totalSubjects"`
    OverallAttendance  float64             `json:"overallAttendance"`
    OverallGPA         float64             `json:"overallGpa"`
    RecentAnnouncements []*Announcement    `json:"recentAnnouncements"`
    UpcomingEvents     []*CalendarEvent    `json:"upcomingEvents"`
    Statistics         *DashboardStats     `json:"statistics"`
}

type DashboardStats struct {
    AttendanceStats AttendanceStatistics `json:"attendanceStats"`
    GradeStats      GradeStatistics      `json:"gradeStats"`
    BehaviorStats   BehaviorStatistics   `json:"behaviorStats"`
}

// Class Statistics
type ClassStatistics struct {
    ClassID             string  `db:"class_id" json:"classId"`
    ClassName           string  `db:"class_name" json:"className"`
    TermID              string  `db:"term_id" json:"termId"`
    TotalStudents       int     `db:"total_students" json:"totalStudents"`
    TotalSubjects       int     `db:"total_subjects" json:"totalSubjects"`
    AvgAttendanceRate   float64 `db:"avg_attendance_rate" json:"avgAttendanceRate"`
    AvgGrade            float64 `db:"avg_grade" json:"avgGrade"`
    StudentsPassed      int     `db:"students_passed" json:"studentsPassed"`
    StudentsFailed      int     `db:"students_failed" json:"studentsFailed"`
}

// Student Performance
type StudentPerformance struct {
    StudentID            string  `db:"student_id" json:"studentId"`
    NIS                  string  `db:"nis" json:"nis"`
    FullName             string  `db:"full_name" json:"fullName"`
    ClassID              string  `db:"class_id" json:"classId"`
    TermID               string  `db:"term_id" json:"termId"`
    TotalAttendanceDays  int     `db:"total_attendance_days" json:"totalAttendanceDays"`
    DaysPresent          int     `db:"days_present" json:"daysPresent"`
    AttendancePercentage float64 `db:"attendance_percentage" json:"attendancePercentage"`
    SubjectsEnrolled     int     `db:"subjects_enrolled" json:"subjectsEnrolled"`
    GPA                  float64 `db:"gpa" json:"gpa"`
    LowestGrade          float64 `db:"lowest_grade" json:"lowestGrade"`
    HighestGrade         float64 `db:"highest_grade" json:"highestGrade"`
    SubjectsPassed       int     `db:"subjects_passed" json:"subjectsPassed"`
    SubjectsFailed       int     `db:"subjects_failed" json:"subjectsFailed"`
    BehaviorPoints       int     `db:"behavior_points" json:"behaviorPoints"`
    PositiveNotes        int     `db:"positive_notes" json:"positiveNotes"`
    NegativeNotes        int     `db:"negative_notes" json:"negativeNotes"`
}

// Subject Statistics
type SubjectStatistics struct {
    SubjectID    string  `db:"subject_id" json:"subjectId"`
    SubjectName  string  `db:"subject_name" json:"subjectName"`
    ClassID      string  `db:"class_id" json:"classId"`
    TermID       string  `db:"term_id" json:"termId"`
    TotalStudents int    `db:"total_students" json:"totalStudents"`
    AvgGrade     float64 `db:"avg_grade" json:"avgGrade"`
    GradeStddev  float64 `db:"grade_stddev" json:"gradeStddev"`
    MinGrade     float64 `db:"min_grade" json:"minGrade"`
    MaxGrade     float64 `db:"max_grade" json:"maxGrade"`
    PassedCount  int     `db:"passed_count" json:"passedCount"`
    FailedCount  int     `db:"failed_count" json:"failedCount"`
    PassRate     float64 `db:"pass_rate" json:"passRate"`
}

// Attendance Analytics
type AttendanceStatistics struct {
    TermID           string                 `json:"termId"`
    OverallRate      float64                `json:"overallRate"`
    TotalDays        int                    `json:"totalDays"`
    TotalPresent     int                    `json:"totalPresent"`
    TotalAbsent      int                    `json:"totalAbsent"`
    ByStatus         map[string]int         `json:"byStatus"` // H, S, I, A counts
    DailyTrend       []DailyAttendanceTrend `json:"dailyTrend"`
    TopClasses       []ClassAttendanceRank  `json:"topClasses"`
    BottomClasses    []ClassAttendanceRank  `json:"bottomClasses"`
}

type DailyAttendanceTrend struct {
    Date       string  `json:"date"`
    TotalMarked int    `json:"totalMarked"`
    PresentRate float64 `json:"presentRate"`
}

type ClassAttendanceRank struct {
    ClassID   string  `json:"classId"`
    ClassName string  `json:"className"`
    Rate      float64 `json:"rate"`
}

// Grade Analytics
type GradeStatistics struct {
    TermID          string               `json:"termId"`
    OverallGPA      float64              `json:"overallGpa"`
    TotalGraded     int                  `json:"totalGraded"`
    PassedCount     int                  `json:"passedCount"`
    FailedCount     int                  `json:"failedCount"`
    PassRate        float64              `json:"passRate"`
    GradeDistribution map[string]int     `json:"gradeDistribution"` // A, B, C, D, E counts
    TopSubjects     []SubjectGradeRank   `json:"topSubjects"`
    BottomSubjects  []SubjectGradeRank   `json:"bottomSubjects"`
}

type SubjectGradeRank struct {
    SubjectID   string  `json:"subjectId"`
    SubjectName string  `json:"subjectName"`
    AvgGrade    float64 `json:"avgGrade"`
    PassRate    float64 `json:"passRate"`
}

// Behavior Analytics
type BehaviorStatistics struct {
    TermID           string `json:"termId"`
    TotalNotes       int    `json:"totalNotes"`
    PositiveNotes    int    `json:"positiveNotes"`
    NegativeNotes    int    `json:"negativeNotes"`
    NeutralNotes     int    `json:"neutralNotes"`
    TotalPoints      int    `json:"totalPoints"`
    TopStudents      []StudentBehaviorRank `json:"topStudents"`
}

type StudentBehaviorRank struct {
    StudentID   string `json:"studentId"`
    StudentName string `json:"studentName"`
    Points      int    `json:"points"`
}

// Leaderboard
type LeaderboardEntry struct {
    Rank        int     `json:"rank"`
    StudentID   string  `json:"studentId"`
    StudentName string  `json:"studentName"`
    NIS         string  `json:"nis"`
    ClassName   string  `json:"className"`
    Score       float64 `json:"score"` // GPA or attendance %
}

// Performance Trends
type PerformanceTrend struct {
    Period      string  `json:"period"` // Week, Month, Term
    AvgGPA      float64 `json:"avgGpa"`
    AvgAttendance float64 `json:"avgAttendance"`
    TotalStudents int   `json:"totalStudents"`
}
```

---

## 5.3 API Endpoints Specification

### Base URL

```
Development: http://localhost:8080/api/v1
Production:  https://api.yourdomain.com/api/v1
```

---

## Dashboard Analytics

### 1. GET /analytics/dashboard

**Description**: Get comprehensive dashboard overview

**Headers:**

```
Authorization: Bearer {accessToken}
```

**Query Parameters:**

- `termId` (string, required): Active term ID

**Response (200 OK):**

```json
{
  "termId": "term_2024_1",
  "termName": "Semester 1 2024/2025",
  "totalStudents": 450,
  "totalTeachers": 35,
  "totalClasses": 15,
  "totalSubjects": 12,
  "overallAttendance": 92.5,
  "overallGpa": 78.3,
  "statistics": {
    "attendanceStats": {
      "overallRate": 92.5,
      "totalDays": 60,
      "totalPresent": 25000,
      "totalAbsent": 2000,
      "byStatus": {
        "H": 25000,
        "S": 1200,
        "I": 500,
        "A": 300
      },
      "dailyTrend": [
        {
          "date": "2024-10-20",
          "totalMarked": 450,
          "presentRate": 94.2
        }
      ],
      "topClasses": [
        {
          "classId": "cls_xipa1",
          "className": "X IPA 1",
          "rate": 96.5
        }
      ]
    },
    "gradeStats": {
      "overallGpa": 78.3,
      "totalGraded": 5400,
      "passedCount": 4860,
      "failedCount": 540,
      "passRate": 90.0,
      "gradeDistribution": {
        "A": 1080,
        "B": 2160,
        "C": 1620,
        "D": 432,
        "E": 108
      }
    },
    "behaviorStats": {
      "totalNotes": 320,
      "positiveNotes": 200,
      "negativeNotes": 80,
      "neutralNotes": 40,
      "totalPoints": 1500
    }
  },
  "recentAnnouncements": [...],
  "upcomingEvents": [...]
}
```

**Caching**: Redis, 15 minutes TTL

**Permissions**: All authenticated users (filtered by role)

---

### 2. GET /analytics/class/:classId

**Description**: Get class-specific analytics

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
  "termId": "term_2024_1",
  "totalStudents": 30,
  "totalSubjects": 12,
  "avgAttendanceRate": 94.5,
  "avgGrade": 80.2,
  "studentsPassed": 28,
  "studentsFailed": 2,
  "students": [
    {
      "studentId": "std_abc123",
      "fullName": "Ahmad Fauzi",
      "gpa": 85.5,
      "attendancePercentage": 96.0,
      "rank": 1
    }
  ],
  "subjectPerformance": [
    {
      "subjectId": "sub_math",
      "subjectName": "Matematika",
      "avgGrade": 82.0,
      "passRate": 93.3
    }
  ]
}
```

**Caching**: Redis, 1 hour TTL

**Permissions**: TEACHER (for own class), ADMIN, SUPERADMIN

---

### 3. GET /analytics/student/:studentId

**Description**: Get student comprehensive performance analytics

**Headers:**

```
Authorization: Bearer {accessToken}
```

**Query Parameters:**

- `termId` (string, required): Term ID

**Response (200 OK):**

```json
{
  "studentId": "std_abc123",
  "nis": "2024001",
  "fullName": "Ahmad Fauzi",
  "classId": "cls_xipa1",
  "className": "X IPA 1",
  "termId": "term_2024_1",
  "performance": {
    "gpa": 85.5,
    "rank": 1,
    "totalRank": 30,
    "subjectsEnrolled": 12,
    "subjectsPassed": 12,
    "subjectsFailed": 0,
    "lowestGrade": 78.0,
    "highestGrade": 95.0
  },
  "attendance": {
    "percentage": 96.0,
    "totalDays": 60,
    "present": 58,
    "sick": 1,
    "permission": 1,
    "absent": 0
  },
  "behavior": {
    "totalPoints": 45,
    "positiveNotes": 8,
    "negativeNotes": 1,
    "neutralNotes": 2
  },
  "subjectBreakdown": [
    {
      "subjectId": "sub_math",
      "subjectName": "Matematika",
      "finalGrade": 85.0,
      "attendance": 95.0,
      "rank": 2
    }
  ],
  "trends": [
    {
      "period": "Week 1",
      "avgGrade": 82.0,
      "attendance": 100.0
    }
  ]
}
```

**Caching**: Redis, 1 hour TTL

**Permissions**: TEACHER (for own students), ADMIN, SUPERADMIN, STUDENT (self)

---

### 4. GET /analytics/subject/:subjectId

**Description**: Get subject statistics across classes

**Headers:**

```
Authorization: Bearer {accessToken}
```

**Query Parameters:**

- `termId` (string, required): Term ID
- `classId` (string, optional): Filter by class

**Response (200 OK):**

```json
{
  "subjectId": "sub_math",
  "subjectName": "Matematika",
  "termId": "term_2024_1",
  "overall": {
    "totalStudents": 450,
    "avgGrade": 78.5,
    "gradeStddev": 12.3,
    "minGrade": 45.0,
    "maxGrade": 98.0,
    "passRate": 88.5
  },
  "byClass": [
    {
      "classId": "cls_xipa1",
      "className": "X IPA 1",
      "totalStudents": 30,
      "avgGrade": 82.0,
      "passRate": 93.3
    }
  ],
  "gradeDistribution": {
    "A": 90,
    "B": 180,
    "C": 135,
    "D": 36,
    "E": 9
  },
  "topPerformers": [
    {
      "studentId": "std_abc123",
      "studentName": "Ahmad Fauzi",
      "grade": 98.0,
      "className": "X IPA 1"
    }
  ]
}
```

**Caching**: Redis, 1 hour TTL

**Permissions**: TEACHER (for own subjects), ADMIN, SUPERADMIN

---

### 5. GET /analytics/attendance

**Description**: Get attendance analytics

**Headers:**

```
Authorization: Bearer {accessToken}
```

**Query Parameters:**

- `termId` (string, required): Term ID
- `startDate` (string, optional): Filter start
- `endDate` (string, optional): Filter end
- `classId` (string, optional): Filter by class

**Response (200 OK):**

```json
{
  "termId": "term_2024_1",
  "period": {
    "start": "2024-08-01",
    "end": "2024-10-24"
  },
  "overall": {
    "totalDays": 60,
    "totalRecords": 27000,
    "presentRate": 92.5,
    "sickRate": 4.5,
    "permissionRate": 1.5,
    "absentRate": 1.5
  },
  "dailyTrend": [
    {
      "date": "2024-10-24",
      "totalMarked": 450,
      "presentRate": 94.2
    }
  ],
  "byClass": [
    {
      "classId": "cls_xipa1",
      "className": "X IPA 1",
      "rate": 96.5
    }
  ],
  "topStudents": [
    {
      "studentId": "std_abc123",
      "studentName": "Ahmad Fauzi",
      "percentage": 100.0
    }
  ]
}
```

**Caching**: Redis, 1 hour TTL

**Permissions**: TEACHER, ADMIN, SUPERADMIN

---

### 6. GET /analytics/grades

**Description**: Get grade analytics

**Headers:**

```
Authorization: Bearer {accessToken}
```

**Query Parameters:**

- `termId` (string, required): Term ID
- `subjectId` (string, optional): Filter by subject
- `classId` (string, optional): Filter by class

**Response (200 OK):**

```json
{
  "termId": "term_2024_1",
  "overall": {
    "totalGraded": 5400,
    "avgGpa": 78.3,
    "passRate": 90.0,
    "gradeStddev": 11.5
  },
  "distribution": {
    "A": 1080,
    "B": 2160,
    "C": 1620,
    "D": 432,
    "E": 108
  },
  "bySubject": [
    {
      "subjectId": "sub_math",
      "subjectName": "Matematika",
      "avgGrade": 78.5,
      "passRate": 88.5
    }
  ],
  "byClass": [
    {
      "classId": "cls_xipa1",
      "className": "X IPA 1",
      "avgGrade": 80.2,
      "passRate": 93.3
    }
  ],
  "trends": [
    {
      "period": "Week 8",
      "avgGpa": 77.0
    },
    {
      "period": "Week 12",
      "avgGpa": 78.3
    }
  ]
}
```

**Caching**: Redis, 1 hour TTL

**Permissions**: TEACHER, ADMIN, SUPERADMIN

---

## Leaderboards

### 7. GET /analytics/leaderboard/gpa

**Description**: Get GPA leaderboard

**Headers:**

```
Authorization: Bearer {accessToken}
```

**Query Parameters:**

- `termId` (string, required): Term ID
- `classId` (string, optional): Filter by class
- `limit` (integer, default: 10): Top N students

**Response (200 OK):**

```json
{
  "termId": "term_2024_1",
  "classId": "cls_xipa1",
  "leaderboard": [
    {
      "rank": 1,
      "studentId": "std_abc123",
      "studentName": "Ahmad Fauzi",
      "nis": "2024001",
      "className": "X IPA 1",
      "score": 92.5
    },
    {
      "rank": 2,
      "studentId": "std_def456",
      "studentName": "Siti Aminah",
      "nis": "2024002",
      "className": "X IPA 1",
      "score": 90.8
    }
  ]
}
```

**Caching**: Redis sorted set, 30 minutes TTL

**Permissions**: TEACHER, ADMIN, SUPERADMIN

---

### 8. GET /analytics/leaderboard/attendance

**Description**: Get attendance leaderboard

**Headers:**

```
Authorization: Bearer {accessToken}
```

**Query Parameters:**

- `termId` (string, required): Term ID
- `classId` (string, optional): Filter by class
- `limit` (integer, default: 10): Top N students

**Response (200 OK):**

```json
{
  "termId": "term_2024_1",
  "leaderboard": [
    {
      "rank": 1,
      "studentId": "std_abc123",
      "studentName": "Ahmad Fauzi",
      "nis": "2024001",
      "className": "X IPA 1",
      "score": 100.0
    }
  ]
}
```

**Caching**: Redis sorted set, 30 minutes TTL

**Permissions**: TEACHER, ADMIN, SUPERADMIN

---

### 9. GET /analytics/leaderboard/behavior

**Description**: Get behavior points leaderboard

**Headers:**

```
Authorization: Bearer {accessToken}
```

**Query Parameters:**

- `termId` (string, required): Term ID
- `limit` (integer, default: 10): Top N students

**Response (200 OK):**

```json
{
  "termId": "term_2024_1",
  "leaderboard": [
    {
      "rank": 1,
      "studentId": "std_abc123",
      "studentName": "Ahmad Fauzi",
      "points": 150
    }
  ]
}
```

**Caching**: Redis sorted set, 30 minutes TTL

**Permissions**: TEACHER, ADMIN, SUPERADMIN

---

## Performance Monitoring

### 10. GET /analytics/performance/trends

**Description**: Get performance trends over time

**Headers:**

```
Authorization: Bearer {accessToken}
```

**Query Parameters:**

- `termId` (string, required): Term ID
- `granularity` (string, optional): "week" | "month", default: "week"

**Response (200 OK):**

```json
{
  "termId": "term_2024_1",
  "granularity": "week",
  "trends": [
    {
      "period": "Week 1",
      "avgGpa": 75.0,
      "avgAttendance": 90.0,
      "totalStudents": 450
    },
    {
      "period": "Week 12",
      "avgGpa": 78.3,
      "avgAttendance": 92.5,
      "totalStudents": 450
    }
  ]
}
```

**Caching**: Redis, 2 hours TTL

**Permissions**: ADMIN, SUPERADMIN

---

### 11. POST /analytics/refresh

**Description**: Force refresh materialized views and cache

**Headers:**

```
Authorization: Bearer {accessToken}
```

**Request:**

```json
{
  "views": ["class_statistics", "student_performance", "subject_statistics"],
  "clearCache": true
}
```

**Response (200 OK):**

```json
{
  "message": "Analytics refreshed successfully",
  "viewsRefreshed": 3,
  "cacheCleared": true,
  "timestamp": "2024-10-24T10:00:00Z"
}
```

**Permissions**: ADMIN, SUPERADMIN

---

## 5.4 Caching Implementation

### Redis Service Pattern

```go
// internal/infrastructure/cache/redis_service.go
package cache

import (
    "context"
    "encoding/json"
    "fmt"
    "time"

    "github.com/redis/go-redis/v9"
)

type CacheService struct {
    client *redis.Client
}

func NewCacheService(client *redis.Client) *CacheService {
    return &CacheService{client: client}
}

// Get from cache with automatic JSON unmarshaling
func (s *CacheService) Get(ctx context.Context, key string, dest interface{}) error {
    val, err := s.client.Get(ctx, key).Result()
    if err != nil {
        return err
    }
    return json.Unmarshal([]byte(val), dest)
}

// Set to cache with automatic JSON marshaling
func (s *CacheService) Set(ctx context.Context, key string, value interface{}, ttl time.Duration) error {
    data, err := json.Marshal(value)
    if err != nil {
        return err
    }
    return s.client.Set(ctx, key, data, ttl).Err()
}

// Delete from cache
func (s *CacheService) Delete(ctx context.Context, keys ...string) error {
    return s.client.Del(ctx, keys...).Err()
}

// Check if key exists
func (s *CacheService) Exists(ctx context.Context, key string) (bool, error) {
    n, err := s.client.Exists(ctx, key).Result()
    return n > 0, err
}

// Leaderboard operations (sorted sets)
func (s *CacheService) AddToLeaderboard(ctx context.Context, key string, score float64, member string) error {
    return s.client.ZAdd(ctx, key, redis.Z{Score: score, Member: member}).Err()
}

func (s *CacheService) GetLeaderboard(ctx context.Context, key string, limit int) ([]redis.Z, error) {
    return s.client.ZRevRangeWithScores(ctx, key, 0, int64(limit-1)).Result()
}

// Cache-aside pattern helper
func (s *CacheService) GetOrSet(ctx context.Context, key string, dest interface{}, ttl time.Duration, fetchFn func() (interface{}, error)) error {
    // Try cache first
    err := s.Get(ctx, key, dest)
    if err == nil {
        return nil // Cache hit
    }

    if err != redis.Nil {
        return err // Redis error
    }

    // Cache miss - fetch from source
    data, err := fetchFn()
    if err != nil {
        return err
    }

    // Store in cache
    if err := s.Set(ctx, key, data, ttl); err != nil {
        // Log error but don't fail
        fmt.Printf("Failed to cache key %s: %v\n", key, err)
    }

    // Copy to destination
    bytes, _ := json.Marshal(data)
    return json.Unmarshal(bytes, dest)
}
```

### Analytics Service with Caching

```go
// internal/service/analytics_service.go
package service

import (
    "context"
    "fmt"
    "time"

    "yourapp/internal/models"
    "yourapp/internal/repository"
    "yourapp/internal/infrastructure/cache"
)

type AnalyticsService struct {
    repo  repository.AnalyticsRepository
    cache cache.CacheService
}

func NewAnalyticsService(repo repository.AnalyticsRepository, cache cache.CacheService) *AnalyticsService {
    return &AnalyticsService{
        repo:  repo,
        cache: cache,
    }
}

func (s *AnalyticsService) GetDashboardOverview(ctx context.Context, termID string) (*models.DashboardOverview, error) {
    cacheKey := fmt.Sprintf("dashboard:overview:%s", termID)

    var overview models.DashboardOverview
    err := s.cache.GetOrSet(ctx, cacheKey, &overview, 15*time.Minute, func() (interface{}, error) {
        return s.repo.GetDashboardOverview(ctx, termID)
    })

    return &overview, err
}

func (s *AnalyticsService) GetClassStatistics(ctx context.Context, classID, termID string) (*models.ClassStatistics, error) {
    cacheKey := fmt.Sprintf("stats:class:%s:%s", classID, termID)

    var stats models.ClassStatistics
    err := s.cache.GetOrSet(ctx, cacheKey, &stats, 1*time.Hour, func() (interface{}, error) {
        return s.repo.GetClassStatistics(ctx, classID, termID)
    })

    return &stats, err
}

func (s *AnalyticsService) RefreshMaterializedViews(ctx context.Context) error {
    views := []string{
        "mv_class_statistics",
        "mv_student_performance",
        "mv_subject_statistics",
    }

    for _, view := range views {
        if err := s.repo.RefreshMaterializedView(ctx, view); err != nil {
            return fmt.Errorf("failed to refresh %s: %w", view, err)
        }
    }

    // Clear related caches
    s.cache.Delete(ctx, "stats:*", "dashboard:*", "leaderboard:*")

    return nil
}
```

---

## 5.5 Database Optimization

### Query Optimization Strategies

```sql
-- 1. Add covering indexes for common queries
CREATE INDEX idx_grades_enrollment_subject_cover
  ON grades(enrollment_id, subject_id)
  INCLUDE (final_grade);

CREATE INDEX idx_daily_attendance_enrollment_date_cover
  ON daily_attendance(enrollment_id, date)
  INCLUDE (status);

-- 2. Partition large tables by term
CREATE TABLE grades_partitioned (
    LIKE grades INCLUDING ALL
) PARTITION BY LIST (term_id);

CREATE TABLE grades_2024_1 PARTITION OF grades_partitioned
    FOR VALUES IN ('term_2024_1');

-- 3. Add partial indexes for active records
CREATE INDEX idx_enrollments_active
  ON enrollments(term_id, class_id)
  WHERE status = 'ACTIVE';

-- 4. Optimize slow queries with CTEs
WITH attendance_summary AS (
    SELECT
        enrollment_id,
        COUNT(*) FILTER (WHERE status = 'H') * 100.0 / NULLIF(COUNT(*), 0) AS percentage
    FROM daily_attendance
    WHERE date >= CURRENT_DATE - INTERVAL '30 days'
    GROUP BY enrollment_id
)
SELECT
    s.full_name,
    a.percentage
FROM students s
JOIN enrollments e ON e.student_id = s.id
JOIN attendance_summary a ON a.enrollment_id = e.id
WHERE a.percentage < 80.0;
```

### Connection Pooling

```go
// internal/infrastructure/database/pool.go
import (
    "github.com/jmoiron/sqlx"
    _ "github.com/lib/pq"
)

func NewDatabasePool(dsn string) (*sqlx.DB, error) {
    db, err := sqlx.Connect("postgres", dsn)
    if err != nil {
        return nil, err
    }

    // Optimize pool settings for production
    db.SetMaxOpenConns(25)           // Max connections
    db.SetMaxIdleConns(10)           // Idle connections
    db.SetConnMaxLifetime(5 * time.Minute) // Connection lifetime
    db.SetConnMaxIdleTime(2 * time.Minute) // Idle timeout

    return db, nil
}
```

---

## 5.6 Week 15-17 Task Breakdown

### Week 15: Analytics Foundation

- [ ] Create materialized views for statistics
- [ ] Implement Redis caching service
- [ ] Create AnalyticsRepository with optimized queries
- [ ] Create AnalyticsService with caching layer
- [ ] Implement dashboard overview endpoint
- [ ] Implement class/student/subject analytics endpoints
- [ ] Write unit tests for analytics calculations
- [ ] Set up automated view refresh (cron job)

### Week 16: Leaderboards & Reporting

- [ ] Implement leaderboard endpoints (GPA, attendance, behavior)
- [ ] Create attendance analytics endpoint
- [ ] Create grade analytics endpoint
- [ ] Implement performance trends endpoint
- [ ] Add cache invalidation on data changes
- [ ] Write integration tests for analytics
- [ ] Performance testing with realistic data volume
- [ ] Documentation update

### Week 17: Optimization & Monitoring

- [ ] Database query optimization (EXPLAIN ANALYZE)
- [ ] Add database indexes based on slow query log
- [ ] Implement connection pooling optimization
- [ ] Set up APM monitoring (Prometheus/Grafana)
- [ ] Add slow query logging
- [ ] Load testing (Apache JMeter/k6)
- [ ] Memory profiling and optimization
- [ ] Final performance tuning
- [ ] Production deployment

---

## 5.7 Monitoring & Observability

### Prometheus Metrics

```go
// internal/infrastructure/monitoring/metrics.go
package monitoring

import (
    "github.com/prometheus/client_golang/prometheus"
    "github.com/prometheus/client_golang/prometheus/promauto"
)

var (
    // HTTP metrics
    HTTPRequestDuration = promauto.NewHistogramVec(
        prometheus.HistogramOpts{
            Name: "http_request_duration_seconds",
            Help: "HTTP request latency",
            Buckets: prometheus.DefBuckets,
        },
        []string{"method", "endpoint", "status"},
    )

    // Cache metrics
    CacheHits = promauto.NewCounterVec(
        prometheus.CounterOpts{
            Name: "cache_hits_total",
            Help: "Total cache hits",
        },
        []string{"key_pattern"},
    )

    CacheMisses = promauto.NewCounterVec(
        prometheus.CounterOpts{
            Name: "cache_misses_total",
            Help: "Total cache misses",
        },
        []string{"key_pattern"},
    )

    // Database metrics
    DBQueryDuration = promauto.NewHistogramVec(
        prometheus.HistogramOpts{
            Name: "db_query_duration_seconds",
            Help: "Database query latency",
            Buckets: []float64{0.001, 0.005, 0.01, 0.05, 0.1, 0.5, 1.0},
        },
        []string{"query_type"},
    )
)
```

### Grafana Dashboard Configuration

```json
{
  "dashboard": {
    "title": "SIS Analytics Dashboard",
    "panels": [
      {
        "title": "API Response Time (p95)",
        "targets": [
          {
            "expr": "histogram_quantile(0.95, http_request_duration_seconds)"
          }
        ]
      },
      {
        "title": "Cache Hit Rate",
        "targets": [
          {
            "expr": "rate(cache_hits_total[5m]) / (rate(cache_hits_total[5m]) + rate(cache_misses_total[5m]))"
          }
        ]
      },
      {
        "title": "Database Query Latency",
        "targets": [
          {
            "expr": "histogram_quantile(0.95, db_query_duration_seconds)"
          }
        ]
      }
    ]
  }
}
```

---

## 5.8 Success Criteria

- [ ] Dashboard overview loads in < 200ms (with cache)
- [ ] Analytics queries return in < 500ms (without cache)
- [ ] Cache hit rate > 80% for analytics endpoints
- [ ] Materialized views refresh in < 30 seconds
- [ ] Support 1000+ concurrent users
- [ ] Database query p95 latency < 100ms
- [ ] API p95 latency < 300ms
- [ ] 90%+ test coverage for analytics logic
- [ ] Prometheus metrics collecting successfully
- [ ] Grafana dashboards operational

---

## 5.9 Future Enhancements (Post-Phase 5)

- **Real-time Analytics**: WebSocket updates for live dashboards
- **Predictive Analytics**: ML models for student performance prediction
- **Advanced Reporting**: Custom report builder with exports (PDF, Excel)
- **Data Warehouse**: Separate OLAP database for historical analytics
- **A/B Testing**: Feature flags with performance tracking
- **Automated Alerts**: Email/SMS alerts for performance thresholds
- **Mobile Analytics**: Optimized API responses for mobile apps
- **Export Scheduler**: Automated weekly/monthly reports

---

**Next Phase**: [Phase 6: Legacy Decommission](./PHASE_6_LEGACY_DECOMMISSION.md)

**Previous Phases**:

- [Phase 1: Authentication & User Management](./PHASE_1_AUTH_USER_MANAGEMENT.md)
- [Phase 2: Academic Management](./PHASE_2_ACADEMIC_MANAGEMENT.md)
- [Phase 3: Student Management & Assessment](./PHASE_3_STUDENT_ASSESSMENT.md)
- [Phase 4: Attendance & Communication](./PHASE_4_ATTENDANCE_COMMUNICATION.md)
