package handler

import (
	"context"
	"net/http"
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

// SchedulePreferenceHandler exposes /schedules/preferences alias endpoints.
type SchedulePreferenceAliasHandler struct {
	service schedulePreferenceService
}

// NewSchedulePreferenceHandler constructs the alias handler.
func NewSchedulePreferenceHandler(service schedulePreferenceService) *SchedulePreferenceAliasHandler {
	return &SchedulePreferenceAliasHandler{service: service}
}

// Get godoc
// @Summary Get teacher schedule preferences (alias)
// @Tags Scheduler
// @Produce json
// @Param teacher_id query string true "Teacher ID"
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
// @Tags Scheduler
// @Accept json
// @Produce json
// @Param teacher_id query string true "Teacher ID"
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
	return teacherID
}
