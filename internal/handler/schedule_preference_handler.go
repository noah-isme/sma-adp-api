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
// @Tags Academics
// @Produce json
// @Param teacher_id query string true "Teacher ID" example(teacher_123)
// @Success 200 {object} response.Envelope
// @Router /schedules/preferences [get]
func (h *SchedulePreferenceAliasHandler) Get(c *gin.Context) {
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
