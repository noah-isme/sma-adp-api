package handler

import (
	"context"
	"net/http"
	"regexp"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/noah-isme/sma-adp-api/internal/models"
	"github.com/noah-isme/sma-adp-api/internal/service"
	appErrors "github.com/noah-isme/sma-adp-api/pkg/errors"
	"github.com/noah-isme/sma-adp-api/pkg/response"
)

type schedulePreferenceService interface {
	Get(ctx context.Context, teacherID string) (*models.TeacherPreference, error)
	Upsert(ctx context.Context, teacherID string, req service.UpsertTeacherPreferenceRequest) (*models.TeacherPreference, error)
	ListAll(ctx context.Context, filter models.TeacherPreferenceFilter) ([]models.TeacherPreference, *models.Pagination, error)
}

// SchedulePreferenceAliasHandler exposes /schedules/preferences alias endpoints.
type SchedulePreferenceAliasHandler struct {
	service schedulePreferenceService
}

// NewSchedulePreferenceHandler constructs the alias handler.
func NewSchedulePreferenceHandler(service schedulePreferenceService) *SchedulePreferenceAliasHandler {
	return &SchedulePreferenceAliasHandler{service: service}
}

var teacherIDPattern = regexp.MustCompile(`^[a-zA-Z0-9_-]{3,64}$`)

// Get godoc
// @Summary Get teacher schedule preferences (alias)
// @Description Returns one teacher's preferences when teacher_id is supplied. Omit teacher_id to page through every teacher's preferences, which is what the schedule generator needs to seed its constraint set.
// @Tags Academics
// @Produce json
// @Param teacher_id query string false "Teacher ID. When omitted, all preferences are listed." example(teacher_123)
// @Param page query int false "Page (list mode only)"
// @Param limit query int false "Page size (list mode only)"
// @Param sortBy query string false "Sort field (list mode only)"
// @Param sortOrder query string false "Sort order (asc/desc, list mode only)"
// @Success 200 {object} response.Envelope
// @Router /schedules/preferences [get]
func (h *SchedulePreferenceAliasHandler) Get(c *gin.Context) {
	// No teacher_id means "give me everything": the generator and the
	// preferences grid both need the full set, not one row at a time.
	if !hasTeacherIDParam(c) {
		h.list(c)
		return
	}
	teacherID := requireTeacherID(c)
	if teacherID == "" {
		return
	}
	pref, err := h.service.Get(c.Request.Context(), teacherID)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.JSON(c, http.StatusOK, pref, nil)
}

// list pages through every teacher preference record.
func (h *SchedulePreferenceAliasHandler) list(c *gin.Context) {
	filter := models.TeacherPreferenceFilter{
		Page:      parseQueryInt(c, "page", 1),
		PageSize:  parseQueryInt(c, "limit", 100),
		SortBy:    c.Query("sortBy"),
		SortOrder: strings.ToLower(c.Query("sortOrder")),
	}
	prefs, pagination, err := h.service.ListAll(c.Request.Context(), filter)
	if err != nil {
		response.Error(c, err)
		return
	}
	if prefs == nil {
		prefs = []models.TeacherPreference{}
	}
	response.JSON(c, http.StatusOK, prefs, pagination)
}

// Upsert godoc
// @Summary Upsert teacher schedule preferences (alias)
// @Tags Academics
// @Accept json
// @Produce json
// @Param teacher_id query string true "Teacher ID" example(teacher_123)
// @Param payload body service.UpsertTeacherPreferenceRequest true "Preference payload"
// @Success 200 {object} response.Envelope
// @Router /schedules/preferences [post]
func (h *SchedulePreferenceAliasHandler) Upsert(c *gin.Context) {
	teacherID := requireTeacherID(c)
	if teacherID == "" {
		return
	}
	var req service.UpsertTeacherPreferenceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, appErrors.Wrap(err, appErrors.ErrValidation.Code, http.StatusBadRequest, "invalid preference payload"))
		return
	}
	pref, err := h.service.Upsert(c.Request.Context(), teacherID, req)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.JSON(c, http.StatusOK, pref, nil)
}

func requireTeacherID(c *gin.Context) string {
	teacherID := strings.TrimSpace(c.Query("teacher_id"))
	if teacherID == "" {
		teacherID = strings.TrimSpace(c.Query("teacherId"))
	}
	if teacherID == "" {
		response.Error(c, appErrors.Clone(appErrors.ErrValidation, "teacher_id is required"))
		return ""
	}
	if _, err := strconv.Atoi(teacherID); err != nil {
		if !teacherIDPattern.MatchString(teacherID) {
			response.Error(c, appErrors.Clone(appErrors.ErrValidation, "teacher_id must be numeric or a slug/uuid"))
			return ""
		}
	}
	return teacherID
}

// hasTeacherIDParam reports whether the caller scoped the request to one teacher.
func hasTeacherIDParam(c *gin.Context) bool {
	return strings.TrimSpace(c.Query("teacher_id")) != "" || strings.TrimSpace(c.Query("teacherId")) != ""
}
