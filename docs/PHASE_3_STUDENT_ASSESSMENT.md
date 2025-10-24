# Phase 3: Student Management & Assessment (Week 9-11)

## 🎯 Objectives

- Implement student CRUD operations
- Enrollment management (class & term)
- Grade components & configurations
- Grade entry & calculations
- Report generation
- Build upon authentication and academic management from Phase 1 & 2

## Prerequisites

- ✅ Phase 0: Infrastructure setup complete
- ✅ Phase 1: Authentication & User Management operational
- ✅ Phase 2: Academic Management (terms, subjects, classes, schedules) complete
- ✅ JWT middleware and RBAC working

---

## 3.1 Database Models

### Students Table (Existing Schema)

```sql
-- Student records
CREATE TABLE students (
    id VARCHAR(255) PRIMARY KEY,
    nis VARCHAR(50) UNIQUE NOT NULL, -- Nomor Induk Siswa
    nisn VARCHAR(50) UNIQUE, -- Nomor Induk Siswa Nasional
    full_name VARCHAR(255) NOT NULL,
    gender VARCHAR(10) NOT NULL, -- 'M', 'F'
    birth_date DATE NOT NULL,
    birth_place VARCHAR(255),
    address TEXT,
    phone VARCHAR(50),
    email VARCHAR(255),
    status VARCHAR(50) DEFAULT 'ACTIVE', -- 'ACTIVE', 'INACTIVE', 'GRADUATED', 'TRANSFERRED'
    admission_date DATE,
    guardian_name VARCHAR(255),
    guardian_phone VARCHAR(50),
    guardian_relationship VARCHAR(100),
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_students_nis ON students(nis);
CREATE INDEX idx_students_nisn ON students(nisn);
CREATE INDEX idx_students_status ON students(status);
CREATE INDEX idx_students_full_name ON students(full_name);
```

### Enrollments Table (Existing Schema)

```sql
-- Student enrollment in classes per term
CREATE TABLE enrollments (
    id VARCHAR(255) PRIMARY KEY,
    student_id VARCHAR(255) NOT NULL REFERENCES students(id) ON DELETE CASCADE,
    class_id VARCHAR(255) NOT NULL REFERENCES classes(id) ON DELETE CASCADE,
    term_id VARCHAR(255) NOT NULL REFERENCES terms(id) ON DELETE CASCADE,
    enrollment_date DATE DEFAULT CURRENT_DATE,
    status VARCHAR(50) DEFAULT 'ACTIVE', -- 'ACTIVE', 'INACTIVE', 'TRANSFERRED'
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(student_id, term_id)
);

CREATE INDEX idx_enrollments_student ON enrollments(student_id);
CREATE INDEX idx_enrollments_class ON enrollments(class_id);
CREATE INDEX idx_enrollments_term ON enrollments(term_id);
CREATE INDEX idx_enrollments_status ON enrollments(status);
```

### Grade Components Table (Existing Schema)

```sql
-- Grade component definitions (e.g., UH, UTS, UAS, Tugas)
CREATE TABLE grade_components (
    id VARCHAR(255) PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    code VARCHAR(50) NOT NULL,
    weight INTEGER NOT NULL CHECK (weight >= 0 AND weight <= 100),
    description TEXT,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_grade_components_code ON grade_components(code);
```

### Grade Configs Table (Existing Schema)

```sql
-- Grade configuration per class-subject
CREATE TABLE grade_configs (
    id VARCHAR(255) PRIMARY KEY,
    class_subject_id VARCHAR(255) NOT NULL REFERENCES class_subjects(id) ON DELETE CASCADE,
    term_id VARCHAR(255) NOT NULL REFERENCES terms(id) ON DELETE CASCADE,
    scheme VARCHAR(50) DEFAULT 'WEIGHTED', -- 'WEIGHTED', 'AVERAGE'
    kkm INTEGER DEFAULT 75 CHECK (kkm >= 0 AND kkm <= 100), -- Minimum passing grade
    status VARCHAR(50) DEFAULT 'DRAFT', -- 'DRAFT', 'FINALIZED'
    finalized_at TIMESTAMP,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(class_subject_id, term_id)
);

CREATE INDEX idx_grade_configs_class_subject ON grade_configs(class_subject_id);
CREATE INDEX idx_grade_configs_term ON grade_configs(term_id);
CREATE INDEX idx_grade_configs_status ON grade_configs(status);
```

### Grade Config Components Table (Mapping)

```sql
-- Which components are used for a specific class-subject
CREATE TABLE grade_config_components (
    id VARCHAR(255) PRIMARY KEY,
    grade_config_id VARCHAR(255) NOT NULL REFERENCES grade_configs(id) ON DELETE CASCADE,
    component_id VARCHAR(255) NOT NULL REFERENCES grade_components(id) ON DELETE CASCADE,
    weight INTEGER NOT NULL CHECK (weight >= 0 AND weight <= 100),
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(grade_config_id, component_id)
);

CREATE INDEX idx_grade_config_components_config ON grade_config_components(grade_config_id);
CREATE INDEX idx_grade_config_components_component ON grade_config_components(component_id);
```

### Grades Table (Existing Schema)

```sql
-- Individual student grades
CREATE TABLE grades (
    id VARCHAR(255) PRIMARY KEY,
    enrollment_id VARCHAR(255) NOT NULL REFERENCES enrollments(id) ON DELETE CASCADE,
    class_subject_id VARCHAR(255) NOT NULL REFERENCES class_subjects(id) ON DELETE CASCADE,
    component_id VARCHAR(255) NOT NULL REFERENCES grade_components(id) ON DELETE CASCADE,
    score NUMERIC(5,2) CHECK (score >= 0 AND score <= 100),
    notes TEXT,
    entered_by VARCHAR(255) REFERENCES users(id) ON DELETE SET NULL,
    entered_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(enrollment_id, class_subject_id, component_id)
);

CREATE INDEX idx_grades_enrollment ON grades(enrollment_id);
CREATE INDEX idx_grades_class_subject ON grades(class_subject_id);
CREATE INDEX idx_grades_component ON grades(component_id);
CREATE INDEX idx_grades_entered_by ON grades(entered_by);
```

---

## 3.2 Go Models & Structs

### internal/models/student.go

```go
package models

import "time"

type StudentGender string
type StudentStatus string

const (
    GenderMale   StudentGender = "M"
    GenderFemale StudentGender = "F"

    StudentStatusActive      StudentStatus = "ACTIVE"
    StudentStatusInactive    StudentStatus = "INACTIVE"
    StudentStatusGraduated   StudentStatus = "GRADUATED"
    StudentStatusTransferred StudentStatus = "TRANSFERRED"
)

type Student struct {
    ID                   string        `db:"id" json:"id"`
    NIS                  string        `db:"nis" json:"nis"`
    NISN                 *string       `db:"nisn" json:"nisn,omitempty"`
    FullName             string        `db:"full_name" json:"fullName"`
    Gender               StudentGender `db:"gender" json:"gender"`
    BirthDate            time.Time     `db:"birth_date" json:"birthDate"`
    BirthPlace           *string       `db:"birth_place" json:"birthPlace,omitempty"`
    Address              *string       `db:"address" json:"address,omitempty"`
    Phone                *string       `db:"phone" json:"phone,omitempty"`
    Email                *string       `db:"email" json:"email,omitempty"`
    Status               StudentStatus `db:"status" json:"status"`
    AdmissionDate        *time.Time    `db:"admission_date" json:"admissionDate,omitempty"`
    GuardianName         *string       `db:"guardian_name" json:"guardianName,omitempty"`
    GuardianPhone        *string       `db:"guardian_phone" json:"guardianPhone,omitempty"`
    GuardianRelationship *string       `db:"guardian_relationship" json:"guardianRelationship,omitempty"`
    CreatedAt            time.Time     `db:"created_at" json:"createdAt"`
    UpdatedAt            time.Time     `db:"updated_at" json:"updatedAt"`
}

// Request DTOs
type CreateStudentRequest struct {
    NIS                  string        `json:"nis" binding:"required"`
    NISN                 *string       `json:"nisn"`
    FullName             string        `json:"fullName" binding:"required"`
    Gender               StudentGender `json:"gender" binding:"required,oneof=M F"`
    BirthDate            string        `json:"birthDate" binding:"required"` // YYYY-MM-DD
    BirthPlace           *string       `json:"birthPlace"`
    Address              *string       `json:"address"`
    Phone                *string       `json:"phone"`
    Email                *string       `json:"email"`
    AdmissionDate        *string       `json:"admissionDate"`
    GuardianName         *string       `json:"guardianName"`
    GuardianPhone        *string       `json:"guardianPhone"`
    GuardianRelationship *string       `json:"guardianRelationship"`
}

type UpdateStudentRequest struct {
    FullName             *string       `json:"fullName"`
    Gender               *StudentGender `json:"gender"`
    BirthDate            *string       `json:"birthDate"`
    BirthPlace           *string       `json:"birthPlace"`
    Address              *string       `json:"address"`
    Phone                *string       `json:"phone"`
    Email                *string       `json:"email"`
    Status               *StudentStatus `json:"status"`
    GuardianName         *string       `json:"guardianName"`
    GuardianPhone        *string       `json:"guardianPhone"`
    GuardianRelationship *string       `json:"guardianRelationship"`
}

type BulkImportStudentsRequest struct {
    Students []CreateStudentRequest `json:"students" binding:"required,min=1"`
}
```

### internal/models/enrollment.go

```go
package models

import "time"

type EnrollmentStatus string

const (
    EnrollmentStatusActive      EnrollmentStatus = "ACTIVE"
    EnrollmentStatusInactive    EnrollmentStatus = "INACTIVE"
    EnrollmentStatusTransferred EnrollmentStatus = "TRANSFERRED"
)

type Enrollment struct {
    ID             string           `db:"id" json:"id"`
    StudentID      string           `db:"student_id" json:"studentId"`
    ClassID        string           `db:"class_id" json:"classId"`
    TermID         string           `db:"term_id" json:"termId"`
    EnrollmentDate time.Time        `db:"enrollment_date" json:"enrollmentDate"`
    Status         EnrollmentStatus `db:"status" json:"status"`
    Student        *Student         `json:"student,omitempty"`  // Joined data
    Class          *Class           `json:"class,omitempty"`    // Joined data
    Term           *Term            `json:"term,omitempty"`     // Joined data
    CreatedAt      time.Time        `db:"created_at" json:"createdAt"`
    UpdatedAt      time.Time        `db:"updated_at" json:"updatedAt"`
}

// Request DTOs
type CreateEnrollmentRequest struct {
    StudentID string `json:"studentId" binding:"required"`
    ClassID   string `json:"classId" binding:"required"`
    TermID    string `json:"termId" binding:"required"`
}

type BulkEnrollRequest struct {
    ClassID   string   `json:"classId" binding:"required"`
    TermID    string   `json:"termId" binding:"required"`
    StudentIDs []string `json:"studentIds" binding:"required,min=1"`
}

type TransferStudentRequest struct {
    NewClassID string `json:"newClassId" binding:"required"`
    Reason     string `json:"reason"`
}
```

### internal/models/grade.go

```go
package models

import "time"

type GradeScheme string
type GradeConfigStatus string

const (
    SchemeWeighted GradeScheme = "WEIGHTED"
    SchemeAverage  GradeScheme = "AVERAGE"

    GradeConfigDraft     GradeConfigStatus = "DRAFT"
    GradeConfigFinalized GradeConfigStatus = "FINALIZED"
)

type GradeComponent struct {
    ID          string    `db:"id" json:"id"`
    Name        string    `db:"name" json:"name"`
    Code        string    `db:"code" json:"code"`
    Weight      int       `db:"weight" json:"weight"`
    Description *string   `db:"description" json:"description,omitempty"`
    CreatedAt   time.Time `db:"created_at" json:"createdAt"`
    UpdatedAt   time.Time `db:"updated_at" json:"updatedAt"`
}

type GradeConfig struct {
    ID             string            `db:"id" json:"id"`
    ClassSubjectID string            `db:"class_subject_id" json:"classSubjectId"`
    TermID         string            `db:"term_id" json:"termId"`
    Scheme         GradeScheme       `db:"scheme" json:"scheme"`
    KKM            int               `db:"kkm" json:"kkm"`
    Status         GradeConfigStatus `db:"status" json:"status"`
    FinalizedAt    *time.Time        `db:"finalized_at" json:"finalizedAt,omitempty"`
    Components     []GradeConfigComponent `json:"components,omitempty"` // Joined data
    CreatedAt      time.Time         `db:"created_at" json:"createdAt"`
    UpdatedAt      time.Time         `db:"updated_at" json:"updatedAt"`
}

type GradeConfigComponent struct {
    ID            string          `db:"id" json:"id"`
    GradeConfigID string          `db:"grade_config_id" json:"gradeConfigId"`
    ComponentID   string          `db:"component_id" json:"componentId"`
    Weight        int             `db:"weight" json:"weight"`
    Component     *GradeComponent `json:"component,omitempty"` // Joined data
    CreatedAt     time.Time       `db:"created_at" json:"createdAt"`
}

type Grade struct {
    ID             string          `db:"id" json:"id"`
    EnrollmentID   string          `db:"enrollment_id" json:"enrollmentId"`
    ClassSubjectID string          `db:"class_subject_id" json:"classSubjectId"`
    ComponentID    string          `db:"component_id" json:"componentId"`
    Score          *float64        `db:"score" json:"score,omitempty"`
    Notes          *string         `db:"notes" json:"notes,omitempty"`
    EnteredBy      *string         `db:"entered_by" json:"enteredBy,omitempty"`
    EnteredAt      *time.Time      `db:"entered_at" json:"enteredAt,omitempty"`
    Component      *GradeComponent `json:"component,omitempty"` // Joined data
    CreatedAt      time.Time       `db:"created_at" json:"createdAt"`
    UpdatedAt      time.Time       `db:"updated_at" json:"updatedAt"`
}

// Request DTOs
type CreateGradeComponentRequest struct {
    Name        string  `json:"name" binding:"required"`
    Code        string  `json:"code" binding:"required"`
    Weight      int     `json:"weight" binding:"required,min=0,max=100"`
    Description *string `json:"description"`
}

type UpdateGradeComponentRequest struct {
    Name        *string `json:"name"`
    Weight      *int    `json:"weight"`
    Description *string `json:"description"`
}

type CreateGradeConfigRequest struct {
    ClassSubjectID string      `json:"classSubjectId" binding:"required"`
    TermID         string      `json:"termId" binding:"required"`
    Scheme         GradeScheme `json:"scheme" binding:"required,oneof=WEIGHTED AVERAGE"`
    KKM            int         `json:"kkm" binding:"required,min=0,max=100"`
    Components     []ConfigComponentWeight `json:"components" binding:"required,min=1"`
}

type ConfigComponentWeight struct {
    ComponentID string `json:"componentId" binding:"required"`
    Weight      int    `json:"weight" binding:"required,min=0,max=100"`
}

type UpdateGradeConfigRequest struct {
    Scheme     *GradeScheme            `json:"scheme"`
    KKM        *int                    `json:"kkm"`
    Components []ConfigComponentWeight `json:"components"`
}

type EnterGradeRequest struct {
    Score float64 `json:"score" binding:"required,min=0,max=100"`
    Notes *string `json:"notes"`
}

type BulkEnterGradesRequest struct {
    Grades []BulkGradeEntry `json:"grades" binding:"required,min=1"`
}

type BulkGradeEntry struct {
    EnrollmentID   string  `json:"enrollmentId" binding:"required"`
    ClassSubjectID string  `json:"classSubjectId" binding:"required"`
    ComponentID    string  `json:"componentId" binding:"required"`
    Score          float64 `json:"score" binding:"required,min=0,max=100"`
    Notes          *string `json:"notes"`
}

// Response DTOs
type StudentGradeReport struct {
    StudentID   string                 `json:"studentId"`
    StudentName string                 `json:"studentName"`
    NIS         string                 `json:"nis"`
    ClassID     string                 `json:"classId"`
    ClassName   string                 `json:"className"`
    TermID      string                 `json:"termId"`
    TermName    string                 `json:"termName"`
    Subjects    []SubjectGradeDetail   `json:"subjects"`
}

type SubjectGradeDetail struct {
    SubjectID   string               `json:"subjectId"`
    SubjectCode string               `json:"subjectCode"`
    SubjectName string               `json:"subjectName"`
    TeacherName string               `json:"teacherName"`
    KKM         int                  `json:"kkm"`
    Components  []ComponentGrade     `json:"components"`
    FinalScore  *float64             `json:"finalScore,omitempty"`
    Status      string               `json:"status"` // "PASSED", "FAILED", "INCOMPLETE"
}

type ComponentGrade struct {
    ComponentID   string   `json:"componentId"`
    ComponentName string   `json:"componentName"`
    Weight        int      `json:"weight"`
    Score         *float64 `json:"score,omitempty"`
}
```

---

## 3.3 API Endpoints Specification

### Base URL

```
Development: http://localhost:8080/api/v1
Production:  https://api.yourdomain.com/api/v1
```

---

## Students Management

### 1. GET /students

**Description**: List all students with filters and pagination

**Headers:**

```
Authorization: Bearer {accessToken}
```

**Query Parameters:**

- `page` (integer, default: 1)
- `limit` (integer, default: 20, max: 100)
- `status` (string, optional): Filter by status (ACTIVE/INACTIVE/GRADUATED/TRANSFERRED)
- `gender` (string, optional): Filter by gender (M/F)
- `search` (string, optional): Search by NIS, NISN, or name
- `classId` (string, optional): Filter students in specific class
- `termId` (string, optional): Filter enrolled students in specific term
- `sort` (string, default: "full_name"): Sort field
- `order` (string, default: "asc"): Sort order

**Response (200 OK):**

```json
{
  "data": [
    {
      "id": "std_abc123",
      "nis": "2024001",
      "nisn": "0012345678",
      "fullName": "Ahmad Fauzi",
      "gender": "M",
      "birthDate": "2008-05-15T00:00:00Z",
      "birthPlace": "Jakarta",
      "address": "Jl. Merdeka No. 123",
      "phone": "081234567890",
      "email": "ahmad@student.com",
      "status": "ACTIVE",
      "admissionDate": "2024-07-01T00:00:00Z",
      "guardianName": "Budi Santoso",
      "guardianPhone": "081234567891",
      "guardianRelationship": "Ayah",
      "createdAt": "2024-06-01T00:00:00Z",
      "updatedAt": "2024-06-01T00:00:00Z"
    }
  ],
  "pagination": {
    "page": 1,
    "limit": 20,
    "total": 300,
    "totalPages": 15
  }
}
```

**Permissions**: All authenticated users

---

### 2. GET /students/:id

**Description**: Get student by ID

**Headers:**

```
Authorization: Bearer {accessToken}
```

**Response (200 OK):**

```json
{
  "id": "std_abc123",
  "nis": "2024001",
  "nisn": "0012345678",
  "fullName": "Ahmad Fauzi",
  "gender": "M",
  "birthDate": "2008-05-15T00:00:00Z",
  "birthPlace": "Jakarta",
  "address": "Jl. Merdeka No. 123",
  "phone": "081234567890",
  "email": "ahmad@student.com",
  "status": "ACTIVE",
  "admissionDate": "2024-07-01T00:00:00Z",
  "guardianName": "Budi Santoso",
  "guardianPhone": "081234567891",
  "guardianRelationship": "Ayah",
  "createdAt": "2024-06-01T00:00:00Z",
  "updatedAt": "2024-06-01T00:00:00Z"
}
```

**Permissions**: All authenticated users

---

### 3. POST /students

**Description**: Create new student

**Headers:**

```
Authorization: Bearer {accessToken}
```

**Request:**

```json
{
  "nis": "2024001",
  "nisn": "0012345678",
  "fullName": "Ahmad Fauzi",
  "gender": "M",
  "birthDate": "2008-05-15",
  "birthPlace": "Jakarta",
  "address": "Jl. Merdeka No. 123",
  "phone": "081234567890",
  "email": "ahmad@student.com",
  "admissionDate": "2024-07-01",
  "guardianName": "Budi Santoso",
  "guardianPhone": "081234567891",
  "guardianRelationship": "Ayah"
}
```

**Response (201 Created):**

```json
{
  "id": "std_abc123",
  "nis": "2024001",
  "nisn": "0012345678",
  "fullName": "Ahmad Fauzi",
  "gender": "M",
  "status": "ACTIVE",
  "createdAt": "2025-10-24T10:00:00Z"
}
```

**Error Responses:**

- `409 Conflict`: NIS or NISN already exists

**Permissions**: ADMIN, SUPERADMIN

---

### 4. PUT /students/:id

**Description**: Update student

**Headers:**

```
Authorization: Bearer {accessToken}
```

**Request:**

```json
{
  "fullName": "Ahmad Fauzi Updated",
  "phone": "081234567899",
  "address": "Jl. Merdeka No. 124"
}
```

**Response (200 OK):**

```json
{
  "id": "std_abc123",
  "nis": "2024001",
  "fullName": "Ahmad Fauzi Updated",
  "updatedAt": "2025-10-24T11:00:00Z"
}
```

**Permissions**: ADMIN, SUPERADMIN

---

### 5. DELETE /students/:id

**Description**: Delete student (soft delete - set status to INACTIVE)

**Headers:**

```
Authorization: Bearer {accessToken}
```

**Response (200 OK):**

```json
{
  "message": "Student deleted successfully"
}
```

**Permissions**: SUPERADMIN

---

### 6. POST /students/import

**Description**: Bulk import students

**Headers:**

```
Authorization: Bearer {accessToken}
```

**Request:**

```json
{
  "students": [
    {
      "nis": "2024001",
      "fullName": "Ahmad Fauzi",
      "gender": "M",
      "birthDate": "2008-05-15"
    },
    {
      "nis": "2024002",
      "fullName": "Siti Aminah",
      "gender": "F",
      "birthDate": "2008-06-20"
    }
  ]
}
```

**Response (200 OK):**

```json
{
  "message": "Bulk import completed",
  "imported": 2,
  "failed": 0,
  "errors": []
}
```

**Permissions**: ADMIN, SUPERADMIN

---

## Enrollment Management

### 7. GET /enrollments

**Description**: List enrollments with filters

**Headers:**

```
Authorization: Bearer {accessToken}
```

**Query Parameters:**

- `termId` (string, required): Filter by term
- `classId` (string, optional): Filter by class
- `studentId` (string, optional): Filter by student
- `status` (string, optional): Filter by status

**Response (200 OK):**

```json
{
  "data": [
    {
      "id": "enr_abc123",
      "studentId": "std_abc123",
      "classId": "cls_xipa1",
      "termId": "term_2024_1",
      "enrollmentDate": "2024-07-15T00:00:00Z",
      "status": "ACTIVE",
      "student": {
        "id": "std_abc123",
        "nis": "2024001",
        "fullName": "Ahmad Fauzi"
      },
      "class": {
        "id": "cls_xipa1",
        "name": "X IPA 1"
      },
      "createdAt": "2024-07-15T00:00:00Z"
    }
  ]
}
```

**Permissions**: All authenticated users

---

### 8. GET /students/:id/enrollments

**Description**: Get enrollment history for a student

**Headers:**

```
Authorization: Bearer {accessToken}
```

**Response (200 OK):**

```json
{
  "studentId": "std_abc123",
  "studentName": "Ahmad Fauzi",
  "enrollments": [
    {
      "id": "enr_abc123",
      "termId": "term_2024_1",
      "termName": "Semester 1 2024/2025",
      "classId": "cls_xipa1",
      "className": "X IPA 1",
      "status": "ACTIVE",
      "enrollmentDate": "2024-07-15T00:00:00Z"
    }
  ]
}
```

**Permissions**: All authenticated users

---

### 9. POST /enrollments

**Description**: Enroll student in a class

**Headers:**

```
Authorization: Bearer {accessToken}
```

**Request:**

```json
{
  "studentId": "std_abc123",
  "classId": "cls_xipa1",
  "termId": "term_2024_1"
}
```

**Response (201 Created):**

```json
{
  "id": "enr_abc123",
  "studentId": "std_abc123",
  "classId": "cls_xipa1",
  "termId": "term_2024_1",
  "status": "ACTIVE",
  "enrollmentDate": "2025-10-24T00:00:00Z"
}
```

**Error Responses:**

- `409 Conflict`: Student already enrolled in a class for this term

**Permissions**: ADMIN, SUPERADMIN

---

### 10. POST /enrollments/bulk

**Description**: Bulk enroll students in a class

**Headers:**

```
Authorization: Bearer {accessToken}
```

**Request:**

```json
{
  "classId": "cls_xipa1",
  "termId": "term_2024_1",
  "studentIds": ["std_abc123", "std_def456", "std_ghi789"]
}
```

**Response (200 OK):**

```json
{
  "message": "Bulk enrollment completed",
  "enrolled": 3,
  "failed": 0,
  "errors": []
}
```

**Permissions**: ADMIN, SUPERADMIN

---

### 11. POST /enrollments/:id/transfer

**Description**: Transfer student to another class

**Headers:**

```
Authorization: Bearer {accessToken}
```

**Request:**

```json
{
  "newClassId": "cls_xipa2",
  "reason": "Parent request"
}
```

**Response (200 OK):**

```json
{
  "message": "Student transferred successfully",
  "oldClassId": "cls_xipa1",
  "newClassId": "cls_xipa2",
  "newEnrollmentId": "enr_xyz789"
}
```

**Permissions**: ADMIN, SUPERADMIN

---

### 12. DELETE /enrollments/:id

**Description**: Remove enrollment (set status to INACTIVE)

**Headers:**

```
Authorization: Bearer {accessToken}
```

**Response (200 OK):**

```json
{
  "message": "Enrollment removed successfully"
}
```

**Permissions**: ADMIN, SUPERADMIN

---

## Grade Components Management

### 13. GET /grade-components

**Description**: List all grade components

**Headers:**

```
Authorization: Bearer {accessToken}
```

**Response (200 OK):**

```json
{
  "data": [
    {
      "id": "comp_uh",
      "name": "Ulangan Harian",
      "code": "UH",
      "weight": 30,
      "description": "Daily test",
      "createdAt": "2024-01-01T00:00:00Z"
    },
    {
      "id": "comp_uts",
      "name": "Ujian Tengah Semester",
      "code": "UTS",
      "weight": 30,
      "description": "Mid-term exam"
    },
    {
      "id": "comp_uas",
      "name": "Ujian Akhir Semester",
      "code": "UAS",
      "weight": 40,
      "description": "Final exam"
    }
  ]
}
```

**Permissions**: All authenticated users

---

### 14. POST /grade-components

**Description**: Create grade component

**Headers:**

```
Authorization: Bearer {accessToken}
```

**Request:**

```json
{
  "name": "Tugas",
  "code": "TGS",
  "weight": 20,
  "description": "Homework assignments"
}
```

**Response (201 Created):**

```json
{
  "id": "comp_tgs",
  "name": "Tugas",
  "code": "TGS",
  "weight": 20,
  "createdAt": "2025-10-24T10:00:00Z"
}
```

**Permissions**: ADMIN, SUPERADMIN

---

### 15. PUT /grade-components/:id

**Description**: Update grade component

**Headers:**

```
Authorization: Bearer {accessToken}
```

**Request:**

```json
{
  "name": "Tugas (Updated)",
  "weight": 25
}
```

**Response (200 OK):**

```json
{
  "id": "comp_tgs",
  "name": "Tugas (Updated)",
  "weight": 25,
  "updatedAt": "2025-10-24T11:00:00Z"
}
```

**Permissions**: ADMIN, SUPERADMIN

---

### 16. DELETE /grade-components/:id

**Description**: Delete grade component

**Headers:**

```
Authorization: Bearer {accessToken}
```

**Response (200 OK):**

```json
{
  "message": "Grade component deleted successfully"
}
```

**Error Responses:**

- `409 Conflict`: Component is used in grade configs

**Permissions**: SUPERADMIN

---

## Grade Configuration Management

### 17. GET /grade-configs

**Description**: List grade configurations

**Headers:**

```
Authorization: Bearer {accessToken}
```

**Query Parameters:**

- `termId` (string, required): Filter by term
- `classId` (string, optional): Filter by class
- `subjectId` (string, optional): Filter by subject
- `status` (string, optional): Filter by status (DRAFT/FINALIZED)

**Response (200 OK):**

```json
{
  "data": [
    {
      "id": "cfg_abc123",
      "classSubjectId": "clssubj_001",
      "termId": "term_2024_1",
      "scheme": "WEIGHTED",
      "kkm": 75,
      "status": "FINALIZED",
      "finalizedAt": "2024-07-20T00:00:00Z",
      "components": [
        {
          "id": "cfgcomp_001",
          "componentId": "comp_uh",
          "weight": 30,
          "component": {
            "id": "comp_uh",
            "name": "Ulangan Harian",
            "code": "UH"
          }
        },
        {
          "id": "cfgcomp_002",
          "componentId": "comp_uts",
          "weight": 30,
          "component": {
            "name": "UTS"
          }
        },
        {
          "id": "cfgcomp_003",
          "componentId": "comp_uas",
          "weight": 40,
          "component": {
            "name": "UAS"
          }
        }
      ],
      "createdAt": "2024-07-15T00:00:00Z"
    }
  ]
}
```

**Permissions**: All authenticated users

---

### 18. POST /grade-configs

**Description**: Create grade configuration

**Headers:**

```
Authorization: Bearer {accessToken}
```

**Request:**

```json
{
  "classSubjectId": "clssubj_001",
  "termId": "term_2024_1",
  "scheme": "WEIGHTED",
  "kkm": 75,
  "components": [
    {
      "componentId": "comp_uh",
      "weight": 30
    },
    {
      "componentId": "comp_uts",
      "weight": 30
    },
    {
      "componentId": "comp_uas",
      "weight": 40
    }
  ]
}
```

**Response (201 Created):**

```json
{
  "id": "cfg_abc123",
  "classSubjectId": "clssubj_001",
  "termId": "term_2024_1",
  "scheme": "WEIGHTED",
  "kkm": 75,
  "status": "DRAFT",
  "createdAt": "2025-10-24T10:00:00Z"
}
```

**Error Responses:**

- `400 Bad Request`: Total weight must equal 100 for WEIGHTED scheme
- `409 Conflict`: Config already exists for this class-subject-term

**Permissions**: ADMIN, SUPERADMIN, TEACHER (for own subjects)

---

### 19. PUT /grade-configs/:id

**Description**: Update grade configuration (only if DRAFT)

**Headers:**

```
Authorization: Bearer {accessToken}
```

**Request:**

```json
{
  "kkm": 80,
  "components": [
    {
      "componentId": "comp_uh",
      "weight": 25
    },
    {
      "componentId": "comp_uts",
      "weight": 35
    },
    {
      "componentId": "comp_uas",
      "weight": 40
    }
  ]
}
```

**Response (200 OK):**

```json
{
  "id": "cfg_abc123",
  "kkm": 80,
  "updatedAt": "2025-10-24T11:00:00Z"
}
```

**Error Responses:**

- `400 Bad Request`: Cannot modify finalized config

**Permissions**: ADMIN, SUPERADMIN, TEACHER (for own subjects)

---

### 20. POST /grade-configs/:id/finalize

**Description**: Finalize grade configuration (lock for editing)

**Headers:**

```
Authorization: Bearer {accessToken}
```

**Response (200 OK):**

```json
{
  "id": "cfg_abc123",
  "status": "FINALIZED",
  "finalizedAt": "2025-10-24T12:00:00Z"
}
```

**Permissions**: ADMIN, SUPERADMIN

---

### 21. DELETE /grade-configs/:id

**Description**: Delete grade configuration (only if DRAFT and no grades entered)

**Headers:**

```
Authorization: Bearer {accessToken}
```

**Response (200 OK):**

```json
{
  "message": "Grade configuration deleted successfully"
}
```

**Permissions**: ADMIN, SUPERADMIN

---

## Grade Entry & Management

### 22. POST /grades

**Description**: Enter single grade

**Headers:**

```
Authorization: Bearer {accessToken}
```

**Request:**

```json
{
  "enrollmentId": "enr_abc123",
  "classSubjectId": "clssubj_001",
  "componentId": "comp_uh",
  "score": 85.5,
  "notes": "Good performance"
}
```

**Response (201 Created):**

```json
{
  "id": "grd_abc123",
  "enrollmentId": "enr_abc123",
  "classSubjectId": "clssubj_001",
  "componentId": "comp_uh",
  "score": 85.5,
  "enteredBy": "usr_teacher01",
  "enteredAt": "2025-10-24T10:00:00Z"
}
```

**Permissions**: TEACHER (for own subjects), ADMIN, SUPERADMIN

---

### 23. POST /grades/bulk

**Description**: Bulk enter grades

**Headers:**

```
Authorization: Bearer {accessToken}
```

**Request:**

```json
{
  "grades": [
    {
      "enrollmentId": "enr_abc123",
      "classSubjectId": "clssubj_001",
      "componentId": "comp_uh",
      "score": 85.5
    },
    {
      "enrollmentId": "enr_def456",
      "classSubjectId": "clssubj_001",
      "componentId": "comp_uh",
      "score": 90.0
    }
  ]
}
```

**Response (200 OK):**

```json
{
  "message": "Bulk grade entry completed",
  "entered": 2,
  "failed": 0,
  "errors": []
}
```

**Permissions**: TEACHER (for own subjects), ADMIN, SUPERADMIN

---

### 24. PUT /grades/:id

**Description**: Update grade

**Headers:**

```
Authorization: Bearer {accessToken}
```

**Request:**

```json
{
  "score": 87.0,
  "notes": "Score corrected"
}
```

**Response (200 OK):**

```json
{
  "id": "grd_abc123",
  "score": 87.0,
  "updatedAt": "2025-10-24T11:00:00Z"
}
```

**Permissions**: TEACHER (for own subjects), ADMIN, SUPERADMIN

---

### 25. DELETE /grades/:id

**Description**: Delete grade

**Headers:**

```
Authorization: Bearer {accessToken}
```

**Response (200 OK):**

```json
{
  "message": "Grade deleted successfully"
}
```

**Permissions**: TEACHER (for own subjects), ADMIN, SUPERADMIN

---

## Grade Reports & Analytics

### 26. GET /students/:id/grades

**Description**: Get complete grade report for a student

**Headers:**

```
Authorization: Bearer {accessToken}
```

**Query Parameters:**

- `termId` (string, required): Filter by term

**Response (200 OK):**

```json
{
  "studentId": "std_abc123",
  "studentName": "Ahmad Fauzi",
  "nis": "2024001",
  "classId": "cls_xipa1",
  "className": "X IPA 1",
  "termId": "term_2024_1",
  "termName": "Semester 1 2024/2025",
  "subjects": [
    {
      "subjectId": "subj_math001",
      "subjectCode": "MAT",
      "subjectName": "Matematika",
      "teacherName": "Siti Aminah",
      "kkm": 75,
      "components": [
        {
          "componentId": "comp_uh",
          "componentName": "Ulangan Harian",
          "weight": 30,
          "score": 85.5
        },
        {
          "componentId": "comp_uts",
          "componentName": "UTS",
          "weight": 30,
          "score": 80.0
        },
        {
          "componentId": "comp_uas",
          "componentName": "UAS",
          "weight": 40,
          "score": 90.0
        }
      ],
      "finalScore": 85.65,
      "status": "PASSED"
    },
    {
      "subjectId": "subj_bio001",
      "subjectCode": "BIO",
      "subjectName": "Biologi",
      "teacherName": "Ahmad Yani",
      "kkm": 75,
      "components": [
        {
          "componentId": "comp_uh",
          "componentName": "UH",
          "weight": 30,
          "score": 70.0
        },
        {
          "componentId": "comp_uts",
          "componentName": "UTS",
          "weight": 30,
          "score": null
        },
        {
          "componentId": "comp_uas",
          "componentName": "UAS",
          "weight": 40,
          "score": null
        }
      ],
      "finalScore": null,
      "status": "INCOMPLETE"
    }
  ]
}
```

**Permissions**:

- ADMIN/SUPERADMIN: Can view any student
- TEACHER: Can view students in own classes
- STUDENT: Can only view own grades (future feature)

---

### 27. GET /classes/:classId/grades

**Description**: Get grades for all students in a class

**Headers:**

```
Authorization: Bearer {accessToken}
```

**Query Parameters:**

- `termId` (string, required): Filter by term
- `subjectId` (string, optional): Filter by specific subject

**Response (200 OK):**

```json
{
  "classId": "cls_xipa1",
  "className": "X IPA 1",
  "termId": "term_2024_1",
  "termName": "Semester 1 2024/2025",
  "students": [
    {
      "studentId": "std_abc123",
      "studentName": "Ahmad Fauzi",
      "nis": "2024001",
      "subjects": [
        {
          "subjectCode": "MAT",
          "subjectName": "Matematika",
          "finalScore": 85.65,
          "status": "PASSED"
        },
        {
          "subjectCode": "BIO",
          "subjectName": "Biologi",
          "finalScore": null,
          "status": "INCOMPLETE"
        }
      ]
    }
  ]
}
```

**Permissions**: TEACHER (for own classes), ADMIN, SUPERADMIN

---

### 28. GET /grades/statistics

**Description**: Get grade statistics and analytics

**Headers:**

```
Authorization: Bearer {accessToken}
```

**Query Parameters:**

- `termId` (string, required): Filter by term
- `classId` (string, optional): Filter by class
- `subjectId` (string, optional): Filter by subject

**Response (200 OK):**

```json
{
  "termId": "term_2024_1",
  "classId": "cls_xipa1",
  "statistics": {
    "totalStudents": 30,
    "studentsWithCompleteGrades": 28,
    "studentsWithIncompleteGrades": 2,
    "averageScore": 82.5,
    "highestScore": 95.0,
    "lowestScore": 65.0,
    "passRate": 93.33,
    "subjectBreakdown": [
      {
        "subjectId": "subj_math001",
        "subjectName": "Matematika",
        "average": 85.2,
        "passRate": 95.0,
        "studentsAboveKKM": 28,
        "studentsBelowKKM": 2
      }
    ],
    "distribution": {
      "90-100": 8,
      "80-89": 12,
      "70-79": 6,
      "60-69": 3,
      "0-59": 1
    }
  }
}
```

**Permissions**: TEACHER (for own classes), ADMIN, SUPERADMIN

---

## 3.4 Implementation Guidelines

### Service Layer Pattern

```go
// Example: internal/service/grade_service.go
type GradeService interface {
    // Grade entry
    EnterGrade(ctx context.Context, req *models.EnterGradeRequest, userID string) (*models.Grade, error)
    BulkEnterGrades(ctx context.Context, req *models.BulkEnterGradesRequest, userID string) (int, error)

    // Grade calculation
    CalculateFinalScore(ctx context.Context, enrollmentID, classSubjectID string) (*float64, error)
    CalculateClassAverages(ctx context.Context, classID, termID string) (map[string]float64, error)

    // Reports
    GetStudentGradeReport(ctx context.Context, studentID, termID string) (*models.StudentGradeReport, error)
    GetClassGradeReport(ctx context.Context, classID, termID string) ([]models.StudentGradeReport, error)

    // Analytics
    GetGradeStatistics(ctx context.Context, filters GradeStatFilters) (*GradeStatistics, error)
}
```

### Grade Calculation Logic

```go
func (s *gradeService) CalculateFinalScore(ctx context.Context, enrollmentID, classSubjectID string) (*float64, error) {
    // 1. Get grade config
    config, err := s.gradeConfigRepo.FindByClassSubject(ctx, classSubjectID)
    if err != nil {
        return nil, err
    }

    // 2. Get all component grades
    grades, err := s.gradeRepo.FindByEnrollmentAndSubject(ctx, enrollmentID, classSubjectID)
    if err != nil {
        return nil, err
    }

    // 3. Check completeness
    if len(grades) < len(config.Components) {
        return nil, nil // Incomplete grades
    }

    // 4. Calculate based on scheme
    var finalScore float64

    if config.Scheme == models.SchemeWeighted {
        // Weighted average
        for _, grade := range grades {
            component := findComponent(config.Components, grade.ComponentID)
            finalScore += (*grade.Score * float64(component.Weight)) / 100.0
        }
    } else {
        // Simple average
        sum := 0.0
        for _, grade := range grades {
            sum += *grade.Score
        }
        finalScore = sum / float64(len(grades))
    }

    return &finalScore, nil
}
```

---

## 3.5 Week 9-11 Task Breakdown

### Week 9: Students & Enrollment

- [ ] Create Students repository, service, and handlers
- [ ] Implement enrollment repository and service
- [ ] Create student CRUD endpoints
- [ ] Implement enrollment management (create, transfer, bulk)
- [ ] Add student import functionality
- [ ] Write unit tests for student/enrollment services
- [ ] Integration tests for enrollment workflows
- [ ] Update Swagger documentation

### Week 10: Grade Components & Configuration

- [ ] Create GradeComponent repository, service, handlers
- [ ] Create GradeConfig repository, service, handlers
- [ ] Implement grade config validation (weight totals)
- [ ] Create finalize workflow
- [ ] Write unit tests for grade config logic
- [ ] Integration tests for component management
- [ ] Update Swagger docs

### Week 11: Grade Entry & Reporting

- [ ] Create Grades repository, service, handlers
- [ ] Implement grade calculation engine
- [ ] Create bulk grade entry functionality
- [ ] Implement student grade report generation
- [ ] Create class grade report
- [ ] Implement grade statistics and analytics
- [ ] Write comprehensive unit tests
- [ ] Integration tests for grade calculations
- [ ] Performance optimization (caching, indexing)
- [ ] Full frontend integration

---

## 3.6 Migration Strategy

### Database Migration

```sql
-- migrations/000004_add_grade_indexes.up.sql
-- Performance indexes for grade queries

CREATE INDEX idx_grades_enrollment_subject ON grades(enrollment_id, class_subject_id);
CREATE INDEX idx_enrollments_term_class ON enrollments(term_id, class_id);
CREATE INDEX idx_enrollments_student_term ON enrollments(student_id, term_id);

-- Prevent duplicate enrollments
CREATE UNIQUE INDEX idx_unique_student_term ON enrollments(student_id, term_id)
  WHERE status = 'ACTIVE';

-- Efficient grade calculation queries
CREATE INDEX idx_grades_calculation ON grades(enrollment_id, class_subject_id, component_id)
  WHERE score IS NOT NULL;
```

### Data Validation Rules

1. **Student NIS**: Must be unique, cannot be changed after creation
2. **Enrollment**: Student can only be enrolled in one class per term
3. **Grade Config**: Total weight must equal 100 for WEIGHTED scheme
4. **Grade Entry**: Score must be between 0-100
5. **Finalization**: Once finalized, grade config cannot be modified

### Frontend Migration

1. Update API endpoints for student management
2. Feature flag: `USE_GO_GRADES=true`
3. Gradual rollout:
   - Week 9: Students & Enrollment (20% users)
   - Week 10: Grade Configuration (50% users)
   - Week 11: Grade Entry & Reports (100% users)

---

## 3.7 Success Criteria

- [ ] All student/grade endpoints return < 200ms response time
- [ ] Grade calculation accuracy: 100%
- [ ] Bulk operations handle 500+ students efficiently
- [ ] Report generation < 3 seconds for full class report
- [ ] 90%+ test coverage for grade calculation logic
- [ ] Zero data inconsistency during migration
- [ ] API documentation complete with examples
- [ ] Frontend fully integrated and tested
- [ ] Export reports to PDF/Excel (optional stretch goal)

---

## 3.8 Future Enhancements (Post-Phase 3)

- **Student Portal**: Students can view own grades
- **Parent Portal**: Parents can view child's grades
- **Grade Prediction**: ML-based grade prediction for incomplete components
- **Remedial Tracking**: Track remedial exams and scores
- **Grade Distribution Charts**: Visual analytics
- **Export Functionality**: PDF/Excel report exports
- **Notification System**: Alert students/parents when grades are entered

---

**Next Phase**: [Phase 4: Attendance & Communication](./PHASE_4_ATTENDANCE_COMMUNICATION.md)

**Previous Phases**:

- [Phase 1: Authentication & User Management](./PHASE_1_AUTH_USER_MANAGEMENT.md)
- [Phase 2: Academic Management](./PHASE_2_ACADEMIC_MANAGEMENT.md)
