package models

import (
	"time"
)

// ParentStudentRelationship represents the relationship between parent and student
type ParentStudentRelationship string

const (
	RelationshipParent         ParentStudentRelationship = "PARENT"
	RelationshipGuardian       ParentStudentRelationship = "GUARDIAN"
	RelationshipEmergencyContact ParentStudentRelationship = "EMERGENCY_CONTACT"
)

// ParentStudentLink represents the link between a parent user and a student
type ParentStudentLink struct {
	ID                     string                    `db:"id" json:"id"`
	ParentID               string                    `db:"parent_id" json:"parentId"`
	StudentID              string                    `db:"student_id" json:"studentId"`
	Relationship           ParentStudentRelationship `db:"relationship" json:"relationship"`
	CanViewGrades          bool                      `db:"can_view_grades" json:"canViewGrades"`
	CanViewAttendance      bool                      `db:"can_view_attendance" json:"canViewAttendance"`
	CanViewBehavior        bool                      `db:"can_view_behavior" json:"canViewBehavior"`
	CanViewAnnouncements   bool                      `db:"can_view_announcements" json:"canViewAnnouncements"`
	CanReceiveNotifications bool                     `db:"can_receive_notifications" json:"canReceiveNotifications"`
	CreatedAt              time.Time                 `db:"created_at" json:"createdAt"`
	UpdatedAt              time.Time                 `db:"updated_at" json:"updatedAt"`
}

// CreateParentStudentLinkRequest represents the request to create a parent-student link
type CreateParentStudentLinkRequest struct {
	ParentID               string                    `json:"parentId" binding:"required"`
	StudentID              string                    `json:"studentId" binding:"required"`
	Relationship           ParentStudentRelationship `json:"relationship" binding:"omitempty,oneof=PARENT GUARDIAN EMERGENCY_CONTACT"`
	CanViewGrades          bool                      `json:"canViewGrades"`
	CanViewAttendance      bool                      `json:"canViewAttendance"`
	CanViewBehavior        bool                      `json:"canViewBehavior"`
	CanViewAnnouncements   bool                      `json:"canViewAnnouncements"`
	CanReceiveNotifications bool                     `json:"canReceiveNotifications"`
}

// UpdateParentStudentLinkRequest represents the request to update a parent-student link
type UpdateParentStudentLinkRequest struct {
	Relationship           *ParentStudentRelationship `json:"relationship"`
	CanViewGrades          *bool                      `json:"canViewGrades"`
	CanViewAttendance      *bool                      `json:"canViewAttendance"`
	CanViewBehavior        *bool                      `json:"canViewBehavior"`
	CanViewAnnouncements   *bool                      `json:"canViewAnnouncements"`
	CanReceiveNotifications *bool                     `json:"canReceiveNotifications"`
}

// PortalPreferences represents user preferences for the portal
type PortalPreferences struct {
	UserID               string    `db:"user_id" json:"userId"`
	Language             string    `db:"language" json:"language"`
	Timezone             string    `db:"timezone" json:"timezone"`
	EmailNotifications   bool      `db:"email_notifications" json:"emailNotifications"`
	PushNotifications    bool      `db:"push_notifications" json:"pushNotifications"`
	SMSNotifications     bool      `db:"sms_notifications" json:"smsNotifications"`
	GradeAlerts          bool      `db:"grade_alerts" json:"gradeAlerts"`
	AttendanceAlerts     bool      `db:"attendance_alerts" json:"attendanceAlerts"`
	BehaviorAlerts       bool      `db:"behavior_alerts" json:"behaviorAlerts"`
	AnnouncementAlerts   bool      `db:"announcement_alerts" json:"announcementAlerts"`
	CreatedAt            time.Time `db:"created_at" json:"createdAt"`
	UpdatedAt            time.Time `db:"updated_at" json:"updatedAt"`
}

// UpdatePortalPreferencesRequest represents the request to update portal preferences
type UpdatePortalPreferencesRequest struct {
	Language             *string `json:"language"`
	Timezone             *string `json:"timezone"`
	EmailNotifications   *bool   `json:"emailNotifications"`
	PushNotifications    *bool   `json:"pushNotifications"`
	SMSNotifications     *bool   `json:"smsNotifications"`
	GradeAlerts          *bool   `json:"gradeAlerts"`
	AttendanceAlerts     *bool   `json:"attendanceAlerts"`
	BehaviorAlerts       *bool   `json:"behaviorAlerts"`
	AnnouncementAlerts   *bool   `json:"announcementAlerts"`
}

// DeviceToken represents a device token for push notifications
type DeviceToken struct {
	ID          string    `db:"id" json:"id"`
	UserID      string    `db:"user_id" json:"userId"`
	Token       string    `db:"token" json:"token"`
	Platform    string    `db:"platform" json:"platform"` // ios, android, web
	DeviceID    *string   `db:"device_id" json:"deviceId,omitempty"`
	AppVersion  *string   `db:"app_version" json:"appVersion,omitempty"`
	LastUsedAt  time.Time `db:"last_used_at" json:"lastUsedAt"`
	CreatedAt   time.Time `db:"created_at" json:"createdAt"`
}

// RegisterDeviceTokenRequest represents the request to register a device token
type RegisterDeviceTokenRequest struct {
	Token      string  `json:"token" binding:"required"`
	Platform   string  `json:"platform" binding:"required,oneof=ios android web"`
	DeviceID   *string `json:"deviceId"`
	AppVersion *string `json:"appVersion"`
}

// PortalUserInfo represents user info returned in portal login/profile
type PortalUserInfo struct {
	ID             string          `json:"id"`
	Email          string          `json:"email"`
	FullName       string          `json:"fullName"`
	Role           UserRole        `json:"role"` // ORTU or SISWA
	PortalRole     UserRole        `json:"portalRole"` // Same as role for portal
	StudentID      *string         `json:"studentId,omitempty"`
	LinkedStudents []StudentSummary `json:"linkedStudents,omitempty"`
}

// StudentSummary represents a summary of student info for parent views
type StudentSummary struct {
	ID           string  `json:"id"`
	NIS          string  `json:"nis"`
	FullName     string  `json:"fullName"`
	BirthDate    string  `json:"birthDate"`
	Gender       string  `json:"gender"`
	ClassName    *string `json:"className,omitempty"`
	CurrentTerm  *string `json:"currentTerm,omitempty"`
	CurrentClassID *string `json:"currentClassId,omitempty"`
}

// PortalProfile represents the full portal profile
type PortalProfile struct {
	User          *PortalUserInfo     `json:"user"`
	Preferences   *PortalPreferences  `json:"preferences"`
	DeviceTokens  []*DeviceToken      `json:"deviceTokens"`
}

// PortalGradesRequest represents the request for portal grades
type PortalGradesRequest struct {
	UserID     string
	PortalRole UserRole
	StudentID  string // For parents: specific student; for students: from claims
	TermID     string
	SubjectID  string
	ClassID    string
}

// PortalGrade represents a single grade entry in portal response
type PortalGrade struct {
	StudentID       string                 `json:"studentId"`
	EnrollmentID    string                 `json:"enrollmentId"`
	SubjectID       string                 `json:"subjectId"`
	SubjectName     string                 `json:"subjectName"`
	SubjectCode     string                 `json:"subjectCode"`
	ClassName       string                 `json:"className"`
	ComponentGrades map[string]float64     `json:"componentGrades"`
	FinalGrade      float64                `json:"finalGrade"`
	LetterGrade     string                 `json:"letterGrade"`
	IsPassed        bool                   `json:"isPassed"`
	TeacherName     *string                `json:"teacherName,omitempty"`
}

// PortalGradesResponse represents the portal grades response
type PortalGradesResponse struct {
	TermID  string         `json:"termId"`
	Grades  []*PortalGrade `json:"grades"`
	Summary *GradesSummary `json:"summary,omitempty"`
}

// GradesSummary represents the grades summary
type GradesSummary struct {
	GPA              float64 `json:"gpa"`
	TotalSubjects    int     `json:"totalSubjects"`
	PassedSubjects   int     `json:"passedSubjects"`
	FailedSubjects   int     `json:"failedSubjects"`
}

// PortalReportCardSubject represents a subject in the portal report card
type PortalReportCardSubject struct {
	SubjectID   string   `json:"subjectId"`
	SubjectName string   `json:"subjectName"`
	SubjectCode string   `json:"subjectCode,omitempty"`
	FinalGrade  float64  `json:"finalGrade"`
	LetterGrade string   `json:"letterGrade"`
	IsPassed    bool     `json:"isPassed"`
	TeacherName *string  `json:"teacherName,omitempty"`
}

// PortalReportCardResponse represents the portal report card response
type PortalReportCardResponse struct {
	StudentID string                     `json:"studentId"`
	TermID    string                     `json:"termId"`
	Subjects  []*PortalReportCardSubject `json:"subjects"`
	Summary   *GradesSummary             `json:"summary,omitempty"`
}

// PortalAttendanceRequest represents the request for portal attendance
type PortalAttendanceRequest struct {
	UserID     string
	PortalRole UserRole
	StudentID  string
	TermID     string
	StartDate  string
	EndDate    string
	Type       string // "daily" or "subject"
}

// PortalDailyAttendance represents a daily attendance record
type PortalDailyAttendance struct {
	ID    string `json:"id"`
	Date  string `json:"date"`
	Status string `json:"status"` // H, S, I, A
	Notes *string `json:"notes,omitempty"`
}

// PortalSubjectAttendance represents a subject attendance record
type PortalSubjectAttendance struct {
	ID           string  `json:"id"`
	Date         string  `json:"date"`
	SubjectID    string  `json:"subjectId"`
	SubjectName  string  `json:"subjectName"`
	Status       string  `json:"status"` // H, S, I, A
	Notes        *string `json:"notes,omitempty"`
}

// PortalAttendanceSummary represents attendance summary
type PortalAttendanceSummary struct {
	TotalDays    int     `json:"totalDays"`
	Present      int     `json:"present"`
	Sick         int     `json:"sick"`
	Permission   int     `json:"permission"`
	Absent       int     `json:"absent"`
	Percentage   float64 `json:"percentage"`
}

// PortalAttendanceResponse represents the portal attendance response
type PortalAttendanceResponse struct {
	StudentID string                    `json:"studentId"`
	TermID    string                    `json:"termId"`
	Daily     []*PortalDailyAttendance  `json:"daily"`
	Subject   []*PortalSubjectAttendance `json:"subject"`
	Summary   PortalAttendanceSummary   `json:"summary"`
}

// PortalAnnouncementsRequest represents the request for portal announcements
type PortalAnnouncementsRequest struct {
	UserID      string
	PortalRole  UserRole
	StudentID   string
	TermID      string
	Page        int
	Limit       int
	ActiveOnly  bool
}

// PortalAnnouncement represents an announcement in portal response
type PortalAnnouncement struct {
	ID            string  `json:"id"`
	Title         string  `json:"title"`
	Content       string  `json:"content"`
	Audience      string  `json:"audience"` // ALL, GURU, SISWA, CLASS, PARENT
	Priority      string  `json:"priority"` // LOW, NORMAL, HIGH, URGENT
	IsPinned      bool    `json:"isPinned"`
	PublishedAt   *string `json:"publishedAt,omitempty"`
	ExpiresAt     *string `json:"expiresAt,omitempty"`
	PublisherName *string `json:"publisherName,omitempty"`
}

// PortalAnnouncementsResponse represents the portal announcements response
type PortalAnnouncementsResponse struct {
	Announcements []*PortalAnnouncement `json:"announcements"`
	Pagination    *PaginationMeta       `json:"pagination"`
}

// PortalBehaviorRequest represents the request for portal behavior notes
type PortalBehaviorRequest struct {
	UserID     string
	PortalRole UserRole
	StudentID  string
	TermID     string
	Category   string
}

// PortalBehaviorNote represents a behavior note in portal response
type PortalBehaviorNote struct {
	ID           string  `json:"id"`
	Category     string  `json:"category"` // POSITIVE, NEGATIVE, NEUTRAL
	Title        string  `json:"title"`
	Description  string  `json:"description"`
	Date         string  `json:"date"`
	Points       int     `json:"points"`
	ReporterName *string `json:"reporterName,omitempty"`
}

// PortalBehaviorSummary represents behavior summary
type PortalBehaviorSummary struct {
	TotalNotes     int `json:"totalNotes"`
	PositiveNotes  int `json:"positiveNotes"`
	NegativeNotes  int `json:"negativeNotes"`
	NeutralNotes   int `json:"neutralNotes"`
	TotalPoints    int `json:"totalPoints"`
}

// PortalBehaviorResponse represents the portal behavior response
type PortalBehaviorResponse struct {
	StudentID string                 `json:"studentId"`
	TermID    string                 `json:"termId"`
	Notes     []*PortalBehaviorNote  `json:"notes"`
	Summary   PortalBehaviorSummary  `json:"summary"`
}

// PortalCalendarRequest represents the request for portal calendar
type PortalCalendarRequest struct {
	UserID     string
	PortalRole UserRole
	StudentID  string
	TermID     string
	StartDate  string
	EndDate    string
	Month      string
}

// PortalCalendarEvent represents a calendar event in portal response
type PortalCalendarEvent struct {
	ID          string  `json:"id"`
	Title       string  `json:"title"`
	Description *string `json:"description,omitempty"`
	EventType   string  `json:"eventType"` // EXAM, HOLIDAY, MEETING, ACTIVITY, OTHER
	StartDate   string  `json:"startDate"`
	EndDate     string  `json:"endDate"`
	StartTime   *string `json:"startTime,omitempty"`
	EndTime     *string `json:"endTime,omitempty"`
	Location    *string `json:"location,omitempty"`
	Audience    string  `json:"audience"` // ALL, GURU, SISWA, CLASS, PARENT
	ClassName   *string `json:"className,omitempty"`
}

// PortalCalendarResponse represents the portal calendar response
type PortalCalendarResponse struct {
	Events []*PortalCalendarEvent `json:"events"`
}

// PortalAuthRequest/Response types
type PortalLoginRequest struct {
	Email      string `json:"email" binding:"required,email"`
	Password   string `json:"password" binding:"required,min=8"`
	IP         string `json:"-"`
	UserAgent  string `json:"-"`
}

type PortalLoginResponse struct {
	AccessToken  string           `json:"accessToken"`
	RefreshToken string           `json:"refreshToken"`
	User         *PortalUserInfo  `json:"user"`
}

type PortalForgotPasswordRequest struct {
	Email string `json:"email" binding:"required,email"`
}

type PortalResetPasswordRequest struct {
	Token    string `json:"token" binding:"required"`
	Password string `json:"password" binding:"required,min=8"`
}

type PortalLogoutRequest struct {
	RefreshToken string `json:"refreshToken"`
	IP           string `json:"-"`
	UserAgent    string `json:"-"`
}

// ParentStudentLinkFilter captures filtering criteria for listing parent-student links.
type ParentStudentLinkFilter struct {
	ParentID     *string
	StudentID    *string
	Relationship *ParentStudentRelationship
	Page         int
	PageSize     int
	SortBy       string
	SortOrder    string
}