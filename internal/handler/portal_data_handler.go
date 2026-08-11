package handler

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/noah-isme/sma-adp-api/internal/middleware"
	"github.com/noah-isme/sma-adp-api/internal/models"
	"github.com/noah-isme/sma-adp-api/internal/service"
	appErrors "github.com/noah-isme/sma-adp-api/pkg/errors"
	"github.com/noah-isme/sma-adp-api/pkg/response"
)

// PortalDataHandler wires HTTP endpoints to portal data services.
type PortalDataHandler struct {
	gradesService        *service.PortalGradesService
	attendanceService    *service.PortalAttendanceService
	announcementsService *service.PortalAnnouncementsService
	behaviorService      *service.PortalBehaviorService
	calendarService      *service.PortalCalendarService
	homeroomService      *service.PortalHomeroomService
	accessReader         portalStudentAccessReader
}

type portalStudentAccessReader interface {
	FindParentStudentLinkByParentAndStudent(ctx context.Context, parentID, studentID string) (*models.ParentStudentLink, error)
}

type portalCapability string

const (
	portalStudentLinkContextKey                    = "portalStudentLink"
	portalCapabilityGrades        portalCapability = "grades"
	portalCapabilityAttendance    portalCapability = "attendance"
	portalCapabilityBehavior      portalCapability = "behavior"
	portalCapabilityAnnouncements portalCapability = "announcements"
)

// NewPortalDataHandler creates a new handler.
func NewPortalDataHandler(
	gradesService *service.PortalGradesService,
	attendanceService *service.PortalAttendanceService,
	announcementsService *service.PortalAnnouncementsService,
	behaviorService *service.PortalBehaviorService,
	calendarService *service.PortalCalendarService,
	homeroomService *service.PortalHomeroomService,
	accessReaders ...portalStudentAccessReader,
) *PortalDataHandler {
	var accessReader portalStudentAccessReader
	if len(accessReaders) > 0 {
		accessReader = accessReaders[0]
	}
	return &PortalDataHandler{
		gradesService:        gradesService,
		attendanceService:    attendanceService,
		announcementsService: announcementsService,
		behaviorService:      behaviorService,
		calendarService:      calendarService,
		homeroomService:      homeroomService,
		accessReader:         accessReader,
	}
}

// getPortalContext extracts user info from portal JWT context.
func (h *PortalDataHandler) getPortalContext(c *gin.Context) (*models.JWTClaims, error) {
	claims, ok := c.Get(middleware.PortalContextUserKey)
	if !ok {
		return nil, appErrors.ErrUnauthorized
	}
	jwtClaims, ok := claims.(*models.JWTClaims)
	if !ok || jwtClaims == nil {
		return nil, appErrors.ErrUnauthorized
	}
	return jwtClaims, nil
}

// getStudentIDForRequest determines which student to fetch data for.
// For students: uses their own student ID from claims.
// For parents: uses the studentId query param (required).
func (h *PortalDataHandler) getStudentIDForRequest(c *gin.Context, jwtClaims *models.JWTClaims) (string, error) {
	if jwtClaims == nil {
		return "", appErrors.ErrUnauthorized
	}

	switch jwtClaims.Role {
	case models.RoleStudent:
		if jwtClaims.StudentID == "" {
			return "", appErrors.Clone(appErrors.ErrForbidden, "student ID not found in claims")
		}
		if requested := c.Query("studentId"); requested != "" && requested != jwtClaims.StudentID {
			return "", appErrors.Clone(appErrors.ErrForbidden, "students can only access their own data")
		}
		return jwtClaims.StudentID, nil

	case models.RoleOrtu:
		studentID := c.Query("studentId")
		if studentID == "" {
			return "", appErrors.Clone(appErrors.ErrValidation, "studentId query parameter required for parents")
		}
		if h.accessReader == nil {
			return "", appErrors.Wrap(errors.New("portal student access repository is not configured"), appErrors.ErrInternal.Code, appErrors.ErrInternal.Status, "portal access control unavailable")
		}

		link, err := h.accessReader.FindParentStudentLinkByParentAndStudent(c.Request.Context(), jwtClaims.UserID, studentID)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return "", appErrors.Clone(appErrors.ErrForbidden, "student is not linked to parent")
			}
			return "", appErrors.Wrap(err, appErrors.ErrInternal.Code, appErrors.ErrInternal.Status, "failed to verify parent-student link")
		}
		if link == nil || link.ParentID != jwtClaims.UserID || link.StudentID != studentID {
			return "", appErrors.Clone(appErrors.ErrForbidden, "student is not linked to parent")
		}
		c.Set(portalStudentLinkContextKey, link)
		return studentID, nil

	default:
		return "", appErrors.Clone(appErrors.ErrForbidden, "portal access restricted to parents and students")
	}
}

func (h *PortalDataHandler) requireCapability(c *gin.Context, jwtClaims *models.JWTClaims, capability portalCapability) error {
	if jwtClaims == nil {
		return appErrors.ErrUnauthorized
	}
	if jwtClaims.Role == models.RoleStudent {
		return nil
	}
	if jwtClaims.Role != models.RoleOrtu {
		return appErrors.Clone(appErrors.ErrForbidden, "portal access restricted to parents and students")
	}

	value, ok := c.Get(portalStudentLinkContextKey)
	if !ok {
		return appErrors.Clone(appErrors.ErrForbidden, "student is not linked to parent")
	}
	link, ok := value.(*models.ParentStudentLink)
	if !ok || link == nil {
		return appErrors.Clone(appErrors.ErrForbidden, "student link is invalid")
	}

	allowed := false
	switch capability {
	case portalCapabilityGrades:
		allowed = link.CanViewGrades
	case portalCapabilityAttendance:
		allowed = link.CanViewAttendance
	case portalCapabilityBehavior:
		allowed = link.CanViewBehavior
	case portalCapabilityAnnouncements:
		allowed = link.CanViewAnnouncements
	}
	if !allowed {
		return appErrors.Clone(appErrors.ErrForbidden, "parent capability does not allow this data")
	}
	return nil
}

func parsePortalQueryInt(c *gin.Context, name string, defaultValue, min, max int) (int, error) {
	raw := c.Query(name)
	if raw == "" {
		return defaultValue, nil
	}
	value, err := strconv.Atoi(raw)
	if err != nil || value < min || (max > 0 && value > max) {
		return 0, appErrors.Clone(appErrors.ErrValidation, name+" must be within the allowed range")
	}
	return value, nil
}

func parsePortalQueryBool(c *gin.Context, name string, defaultValue bool) (bool, error) {
	raw := c.Query(name)
	if raw == "" {
		return defaultValue, nil
	}
	value, err := strconv.ParseBool(raw)
	if err != nil {
		return false, appErrors.Clone(appErrors.ErrValidation, name+" must be true or false")
	}
	return value, nil
}

// GetGrades godoc
// @Summary Get grades for student
// @Description Get grades for a student in a term
// @Tags Portal Grades
// @Produce json
// @Security BearerAuth
// @Param termId query string false "Term ID"
// @Param subjectId query string false "Subject ID"
// @Param classId query string false "Class ID"
// @Param studentId query string false "Student ID (parent only)"
// @Success 200 {object} response.Envelope{data=models.PortalGradesResponse}
// @Failure 400 {object} response.Envelope
// @Failure 401 {object} response.Envelope
// @Failure 403 {object} response.Envelope
// @Router /portal/grades [get]
func (h *PortalDataHandler) GetGrades(c *gin.Context) {
	jwtClaims, err := h.getPortalContext(c)
	if err != nil {
		response.Error(c, err)
		return
	}

	studentID, err := h.getStudentIDForRequest(c, jwtClaims)
	if err != nil {
		response.Error(c, err)
		return
	}
	if err := h.requireCapability(c, jwtClaims, portalCapabilityGrades); err != nil {
		response.Error(c, err)
		return
	}

	req := models.PortalGradesRequest{
		UserID:     jwtClaims.UserID,
		PortalRole: jwtClaims.Role,
		StudentID:  studentID,
		TermID:     c.DefaultQuery("termId", ""),
		SubjectID:  c.Query("subjectId"),
		ClassID:    c.Query("classId"),
	}

	res, err := h.gradesService.GetGrades(c.Request.Context(), req)
	if err != nil {
		response.Error(c, err)
		return
	}

	response.JSON(c, http.StatusOK, res, nil)
}

// GetReportCard godoc
// @Summary Get report card for student
// @Description Get full report card for current/selected term
// @Tags Portal Grades
// @Produce json
// @Security BearerAuth
// @Param termId query string false "Term ID"
// @Param studentId query string false "Student ID (parent only)"
// @Success 200 {object} response.Envelope{data=models.PortalReportCardResponse}
// @Failure 400 {object} response.Envelope
// @Failure 401 {object} response.Envelope
// @Failure 403 {object} response.Envelope
// @Router /portal/grades/report-card [get]
func (h *PortalDataHandler) GetReportCard(c *gin.Context) {
	jwtClaims, err := h.getPortalContext(c)
	if err != nil {
		response.Error(c, err)
		return
	}

	studentID, err := h.getStudentIDForRequest(c, jwtClaims)
	if err != nil {
		response.Error(c, err)
		return
	}
	if err := h.requireCapability(c, jwtClaims, portalCapabilityGrades); err != nil {
		response.Error(c, err)
		return
	}

	termID := c.DefaultQuery("termId", "")

	res, err := h.gradesService.GetReportCard(c.Request.Context(), studentID, termID)
	if err != nil {
		response.Error(c, err)
		return
	}

	response.JSON(c, http.StatusOK, res, nil)
}

// GetAttendance godoc
// @Summary Get attendance for student
// @Description Get attendance records (daily and/or subject) for student
// @Tags Portal Attendance
// @Produce json
// @Security BearerAuth
// @Param termId query string false "Term ID"
// @Param startDate query string false "Start date (YYYY-MM-DD)"
// @Param endDate query string false "End date (YYYY-MM-DD)"
// @Param type query string false "daily or subject (default: both)"
// @Param studentId query string false "Student ID (parent only)"
// @Success 200 {object} response.Envelope{data=models.PortalAttendanceResponse}
// @Failure 400 {object} response.Envelope
// @Failure 401 {object} response.Envelope
// @Failure 403 {object} response.Envelope
// @Router /portal/attendance [get]
func (h *PortalDataHandler) GetAttendance(c *gin.Context) {
	jwtClaims, err := h.getPortalContext(c)
	if err != nil {
		response.Error(c, err)
		return
	}

	studentID, err := h.getStudentIDForRequest(c, jwtClaims)
	if err != nil {
		response.Error(c, err)
		return
	}
	if err := h.requireCapability(c, jwtClaims, portalCapabilityAttendance); err != nil {
		response.Error(c, err)
		return
	}

	req := models.PortalAttendanceRequest{
		UserID:     jwtClaims.UserID,
		PortalRole: jwtClaims.Role,
		StudentID:  studentID,
		TermID:     c.DefaultQuery("termId", ""),
		StartDate:  c.Query("startDate"),
		EndDate:    c.Query("endDate"),
		Type:       c.DefaultQuery("type", "both"),
	}

	res, err := h.attendanceService.GetAttendance(c.Request.Context(), req)
	if err != nil {
		response.Error(c, err)
		return
	}

	response.JSON(c, http.StatusOK, res, nil)
}

// GetAttendanceStats godoc
// @Summary Get attendance percentage
// @Description Get overall attendance percentage (lightweight endpoint for dashboard)
// @Tags Portal Attendance
// @Produce json
// @Security BearerAuth
// @Param termId query string false "Term ID"
// @Param studentId query string false "Student ID (parent only)"
// @Success 200 {object} response.Envelope{data=models.PortalAttendanceSummary}
// @Failure 400 {object} response.Envelope
// @Failure 401 {object} response.Envelope
// @Failure 403 {object} response.Envelope
// @Router /portal/attendance/percentage [get]
func (h *PortalDataHandler) GetAttendanceStats(c *gin.Context) {
	jwtClaims, err := h.getPortalContext(c)
	if err != nil {
		response.Error(c, err)
		return
	}

	studentID, err := h.getStudentIDForRequest(c, jwtClaims)
	if err != nil {
		response.Error(c, err)
		return
	}
	if err := h.requireCapability(c, jwtClaims, portalCapabilityAttendance); err != nil {
		response.Error(c, err)
		return
	}

	termID := c.DefaultQuery("termId", "")

	res, err := h.attendanceService.GetAttendanceStats(c.Request.Context(), studentID, termID)
	if err != nil {
		response.Error(c, err)
		return
	}

	response.JSON(c, http.StatusOK, res, nil)
}

// GetAnnouncements godoc
// @Summary Get announcements for student
// @Description Get announcements filtered by audience and student's class
// @Tags Portal Announcements
// @Produce json
// @Security BearerAuth
// @Param page query int false "Page number"
// @Param limit query int false "Page size"
// @Param active query bool false "Only active announcements"
// @Param studentId query string false "Student ID (parent only)"
// @Success 200 {object} response.Envelope{data=models.PortalAnnouncementsResponse}
// @Failure 400 {object} response.Envelope
// @Failure 401 {object} response.Envelope
// @Failure 403 {object} response.Envelope
// @Router /portal/announcements [get]
func (h *PortalDataHandler) GetAnnouncements(c *gin.Context) {
	jwtClaims, err := h.getPortalContext(c)
	if err != nil {
		response.Error(c, err)
		return
	}

	studentID, err := h.getStudentIDForRequest(c, jwtClaims)
	if err != nil {
		response.Error(c, err)
		return
	}
	if err := h.requireCapability(c, jwtClaims, portalCapabilityAnnouncements); err != nil {
		response.Error(c, err)
		return
	}

	termID := c.DefaultQuery("termId", "")
	page, err := parsePortalQueryInt(c, "page", 1, 1, 0)
	if err != nil {
		response.Error(c, err)
		return
	}
	limit, err := parsePortalQueryInt(c, "limit", 20, 1, 100)
	if err != nil {
		response.Error(c, err)
		return
	}
	activeOnly, err := parsePortalQueryBool(c, "active", true)
	if err != nil {
		response.Error(c, err)
		return
	}

	req := models.PortalAnnouncementsRequest{
		UserID:     jwtClaims.UserID,
		PortalRole: jwtClaims.Role,
		StudentID:  studentID,
		TermID:     termID,
		Page:       page,
		Limit:      limit,
		ActiveOnly: activeOnly,
	}

	res, err := h.announcementsService.GetAnnouncements(c.Request.Context(), req)
	if err != nil {
		response.Error(c, err)
		return
	}

	response.JSON(c, http.StatusOK, res, nil)
}

// GetAnnouncementByID godoc
// @Summary Get announcement detail
// @Description Get single announcement detail
// @Tags Portal Announcements
// @Produce json
// @Security BearerAuth
// @Param id path string true "Announcement ID"
// @Success 200 {object} response.Envelope{data=models.PortalAnnouncement}
// @Failure 400 {object} response.Envelope
// @Failure 401 {object} response.Envelope
// @Failure 403 {object} response.Envelope
// @Failure 404 {object} response.Envelope
// @Router /portal/announcements/{id} [get]
func (h *PortalDataHandler) GetAnnouncementByID(c *gin.Context) {
	jwtClaims, err := h.getPortalContext(c)
	if err != nil {
		response.Error(c, err)
		return
	}
	studentID, err := h.getStudentIDForRequest(c, jwtClaims)
	if err != nil {
		response.Error(c, err)
		return
	}
	if err := h.requireCapability(c, jwtClaims, portalCapabilityAnnouncements); err != nil {
		response.Error(c, err)
		return
	}

	id := c.Param("id")
	if id == "" {
		response.Error(c, appErrors.Clone(appErrors.ErrValidation, "announcement ID required"))
		return
	}

	res, err := h.announcementsService.GetAnnouncementByIDForStudent(c.Request.Context(), id, studentID)
	if err != nil {
		response.Error(c, err)
		return
	}

	response.JSON(c, http.StatusOK, res, nil)
}

// GetBehaviorNotes godoc
// @Summary Get behavior notes for student
// @Description Get behavior notes for student in a term
// @Tags Portal Behavior
// @Produce json
// @Security BearerAuth
// @Param termId query string false "Term ID"
// @Param category query string false "POSITIVE, NEGATIVE, NEUTRAL"
// @Param studentId query string false "Student ID (parent only)"
// @Success 200 {object} response.Envelope{data=models.PortalBehaviorResponse}
// @Failure 400 {object} response.Envelope
// @Failure 401 {object} response.Envelope
// @Failure 403 {object} response.Envelope
// @Router /portal/behavior-notes [get]
func (h *PortalDataHandler) GetBehaviorNotes(c *gin.Context) {
	jwtClaims, err := h.getPortalContext(c)
	if err != nil {
		response.Error(c, err)
		return
	}

	studentID, err := h.getStudentIDForRequest(c, jwtClaims)
	if err != nil {
		response.Error(c, err)
		return
	}
	if err := h.requireCapability(c, jwtClaims, portalCapabilityBehavior); err != nil {
		response.Error(c, err)
		return
	}

	req := models.PortalBehaviorRequest{
		UserID:     jwtClaims.UserID,
		PortalRole: jwtClaims.Role,
		StudentID:  studentID,
		TermID:     c.DefaultQuery("termId", ""),
		Category:   c.Query("category"),
	}

	res, err := h.behaviorService.GetBehaviorNotes(c.Request.Context(), req)
	if err != nil {
		response.Error(c, err)
		return
	}

	response.JSON(c, http.StatusOK, res, nil)
}

// GetBehaviorSummary godoc
// @Summary Get behavior summary for student
// @Description Get behavior summary for student
// @Tags Portal Behavior
// @Produce json
// @Security BearerAuth
// @Param studentId query string false "Student ID (parent only)"
// @Success 200 {object} response.Envelope{data=models.PortalBehaviorSummary}
// @Failure 400 {object} response.Envelope
// @Failure 401 {object} response.Envelope
// @Failure 403 {object} response.Envelope
// @Router /portal/behavior-notes/summary [get]
func (h *PortalDataHandler) GetBehaviorSummary(c *gin.Context) {
	jwtClaims, err := h.getPortalContext(c)
	if err != nil {
		response.Error(c, err)
		return
	}

	studentID, err := h.getStudentIDForRequest(c, jwtClaims)
	if err != nil {
		response.Error(c, err)
		return
	}
	if err := h.requireCapability(c, jwtClaims, portalCapabilityBehavior); err != nil {
		response.Error(c, err)
		return
	}

	res, err := h.behaviorService.GetBehaviorSummary(c.Request.Context(), studentID)
	if err != nil {
		response.Error(c, err)
		return
	}

	response.JSON(c, http.StatusOK, res, nil)
}

// GetCalendarEvents godoc
// @Summary Get calendar events for student
// @Description Get calendar events relevant to student filtered by audience and class
// @Tags Portal Calendar
// @Produce json
// @Security BearerAuth
// @Param startDate query string false "Start date (YYYY-MM-DD)"
// @Param endDate query string false "End date (YYYY-MM-DD)"
// @Param month query string false "Month filter (YYYY-MM)"
// @Param studentId query string false "Student ID (parent only)"
// @Success 200 {object} response.Envelope{data=models.PortalCalendarResponse}
// @Failure 400 {object} response.Envelope
// @Failure 401 {object} response.Envelope
// @Failure 403 {object} response.Envelope
// @Router /portal/calendar [get]
func (h *PortalDataHandler) GetCalendarEvents(c *gin.Context) {
	jwtClaims, err := h.getPortalContext(c)
	if err != nil {
		response.Error(c, err)
		return
	}

	studentID, err := h.getStudentIDForRequest(c, jwtClaims)
	if err != nil {
		response.Error(c, err)
		return
	}

	req := models.PortalCalendarRequest{
		UserID:     jwtClaims.UserID,
		PortalRole: jwtClaims.Role,
		StudentID:  studentID,
		TermID:     c.DefaultQuery("termId", ""),
		StartDate:  c.Query("startDate"),
		EndDate:    c.Query("endDate"),
		Month:      c.Query("month"),
	}

	res, err := h.calendarService.GetCalendarEvents(c.Request.Context(), req)
	if err != nil {
		response.Error(c, err)
		return
	}

	response.JSON(c, http.StatusOK, res, nil)
}

// GetUpcomingEvents godoc
// @Summary Get upcoming calendar events
// @Description Get upcoming events (next 7 days) for quick dashboard display
// @Tags Portal Calendar
// @Produce json
// @Security BearerAuth
// @Param studentId query string false "Student ID (parent only)"
// @Success 200 {object} response.Envelope{data=models.PortalCalendarResponse}
// @Failure 400 {object} response.Envelope
// @Failure 401 {object} response.Envelope
// @Failure 403 {object} response.Envelope
// @Router /portal/calendar/upcoming [get]
func (h *PortalDataHandler) GetUpcomingEvents(c *gin.Context) {
	jwtClaims, err := h.getPortalContext(c)
	if err != nil {
		response.Error(c, err)
		return
	}

	studentID, err := h.getStudentIDForRequest(c, jwtClaims)
	if err != nil {
		response.Error(c, err)
		return
	}

	res, err := h.calendarService.GetUpcomingEvents(c.Request.Context(), studentID)
	if err != nil {
		response.Error(c, err)
		return
	}

	response.JSON(c, http.StatusOK, res, nil)
}

// GetHomeroom godoc
// @Summary Get homeroom information for student
// @Description Get homeroom teacher and class information for a student in a term
// @Tags Portal Homeroom
// @Produce json
// @Security BearerAuth
// @Param termId query string false "Term ID"
// @Param studentId query string false "Student ID (parent only)"
// @Success 200 {object} response.Envelope{data=service.PortalHomeroomResponse}
// @Failure 400 {object} response.Envelope
// @Failure 401 {object} response.Envelope
// @Failure 403 {object} response.Envelope
// @Failure 404 {object} response.Envelope
// @Router /portal/homeroom [get]
func (h *PortalDataHandler) GetHomeroom(c *gin.Context) {
	jwtClaims, err := h.getPortalContext(c)
	if err != nil {
		response.Error(c, err)
		return
	}

	studentID, err := h.getStudentIDForRequest(c, jwtClaims)
	if err != nil {
		response.Error(c, err)
		return
	}

	termID := c.DefaultQuery("termId", "")

	req := service.PortalHomeroomRequest{
		StudentID: studentID,
		TermID:    termID,
	}

	res, err := h.homeroomService.GetHomeroom(c.Request.Context(), req)
	if err != nil {
		response.Error(c, err)
		return
	}

	response.JSON(c, http.StatusOK, res, nil)
}
