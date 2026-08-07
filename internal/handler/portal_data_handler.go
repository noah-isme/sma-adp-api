package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/noah-isme/sma-adp-api/internal/middleware"
	"github.com/noah-isme/sma-adp-api/internal/models"
	"github.com/noah-isme/sma-adp-api/internal/service"
	appErrors "github.com/noah-isme/sma-adp-api/pkg/errors"
	"github.com/noah-isme/sma-adp-api/pkg/response"
)

// PortalDataHandler wires HTTP endpoints to portal data services.
type PortalDataHandler struct {
	gradesService      *service.PortalGradesService
	attendanceService  *service.PortalAttendanceService
	announcementsService *service.PortalAnnouncementsService
	behaviorService    *service.PortalBehaviorService
	calendarService    *service.PortalCalendarService
	homeroomService    *service.PortalHomeroomService
}

// NewPortalDataHandler creates a new handler.
func NewPortalDataHandler(
	gradesService *service.PortalGradesService,
	attendanceService *service.PortalAttendanceService,
	announcementsService *service.PortalAnnouncementsService,
	behaviorService *service.PortalBehaviorService,
	calendarService *service.PortalCalendarService,
	homeroomService *service.PortalHomeroomService,
) *PortalDataHandler {
	return &PortalDataHandler{
		gradesService:       gradesService,
		attendanceService:   attendanceService,
		announcementsService: announcementsService,
		behaviorService:     behaviorService,
		calendarService:     calendarService,
		homeroomService:     homeroomService,
	}
}

// getPortalContext extracts user info from portal JWT context.
func (h *PortalDataHandler) getPortalContext(c *gin.Context) (*models.JWTClaims, error) {
	claims, ok := c.Get(middleware.PortalContextUserKey)
	if !ok {
		return nil, appErrors.ErrUnauthorized
	}
	jwtClaims := claims.(*models.JWTClaims)
	return jwtClaims, nil
}

// getStudentIDForRequest determines which student to fetch data for.
// For students: uses their own student ID from claims.
// For parents: uses the studentId query param (required).
func (h *PortalDataHandler) getStudentIDForRequest(c *gin.Context, jwtClaims *models.JWTClaims) (string, error) {
	if jwtClaims.Role == models.RoleStudent {
		if jwtClaims.TeacherID == "" {
			return "", appErrors.Clone(appErrors.ErrForbidden, "student ID not found in claims")
		}
		return jwtClaims.TeacherID, nil
	}

	// Parent role - requires studentId query param
	studentID := c.Query("studentId")
	if studentID == "" {
		return "", appErrors.Clone(appErrors.ErrValidation, "studentId query parameter required for parents")
	}

	// TODO: Add permission check against parent_students link
	return studentID, nil
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

	termID := c.DefaultQuery("termId", "")

	req := models.PortalAnnouncementsRequest{
		UserID:     jwtClaims.UserID,
		PortalRole: jwtClaims.Role,
		StudentID:  studentID,
		TermID:     termID,
		Page:       1,
		Limit:      20,
		ActiveOnly: true,
	}

	if page := c.Query("page"); page != "" {
		req.Page = 1
		// TODO: parse page
	}
	if limit := c.Query("limit"); limit != "" {
		// TODO: parse limit
	}
	if active := c.Query("active"); active == "false" {
		req.ActiveOnly = false
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
	id := c.Param("id")
	if id == "" {
		response.Error(c, appErrors.Clone(appErrors.ErrValidation, "announcement ID required"))
		return
	}

	res, err := h.announcementsService.GetAnnouncementByID(c.Request.Context(), id)
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