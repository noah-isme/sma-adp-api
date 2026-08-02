package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/noah-isme/sma-adp-api/internal/models"
	"github.com/noah-isme/sma-adp-api/internal/service"
	appErrors "github.com/noah-isme/sma-adp-api/pkg/errors"
	"github.com/noah-isme/sma-adp-api/pkg/response"
)

type teacherPreferenceService interface {
	ListAll(ctx context.Context, filter models.TeacherPreferenceFilter) ([]models.TeacherPreference, *models.Pagination, error)
	Get(ctx context.Context, teacherID string) (*models.TeacherPreference, error)
	Upsert(ctx context.Context, teacherID string, req service.UpsertTeacherPreferenceRequest) (*models.TeacherPreference, error)
}

// TeacherPreferenceHandler handles teacher preferences CRUD.
type TeacherPreferenceHandler struct {
	service teacherPreferenceService
}

// NewTeacherPreferenceHandler constructs the handler.
func NewTeacherPreferenceHandler(service teacherPreferenceService) *TeacherPreferenceHandler {
	return &TeacherPreferenceHandler{service: service}
}

// ListAll godoc
// @Summary List all teacher preferences
// @Tags Teacher Preferences
// @Produce json
// @Param teacherId query string false "Filter by teacher ID"
// @Param page query int false "Page number"
// @Param limit query int false "Page size"
// @Param sortBy query string false "Sort by field"
// @Param sortOrder query string false "Sort order (asc/desc)"
// @Success 200 {object} response.Envelope
// @Router /teacher-preferences [get]
func (h *TeacherPreferenceHandler) ListAll(c *gin.Context) {
	filter := models.TeacherPreferenceFilter{
		TeacherID: c.Query("teacherId"),
		Page:      parseQueryInt(c, "page", 1),
		PageSize:  parseQueryInt(c, "limit", 20),
		SortBy:    c.Query("sortBy"),
		SortOrder: strings.ToLower(c.Query("sortOrder")),
	}

	prefs, pagination, err := h.service.ListAll(c.Request.Context(), filter)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.JSON(c, http.StatusOK, legacyPreferences(prefs), pagination)
}

// LegacyUpsert accepts the existing admin-panel preference payload. The
// canonical model only persists capacity and unavailable windows; preferred
// slots remain presentation metadata and are returned for client continuity.
func (h *TeacherPreferenceHandler) LegacyUpsert(c *gin.Context) {
	var payload struct {
		TeacherID         string `json:"teacher_id" binding:"required"`
		PreferredDays     []int  `json:"preferred_days"`
		BlockedDays       []int  `json:"blocked_days"`
		PreferredSlots    []int  `json:"preferred_slots"`
		MaxDailySessions  int    `json:"max_daily_sessions"`
		AvailabilityLevel string `json:"availability_level"`
		Notes             string `json:"notes"`
	}
	if err := c.ShouldBindJSON(&payload); err != nil {
		response.Error(c, appErrors.Wrap(err, appErrors.ErrValidation.Code, http.StatusBadRequest, "invalid preference payload"))
		return
	}
	unavailable := make([]models.TeacherUnavailableSlot, 0, len(payload.BlockedDays))
	for _, day := range payload.BlockedDays {
		unavailable = append(unavailable, models.TeacherUnavailableSlot{DayOfWeek: strconv.Itoa(day), TimeRange: "all"})
	}
	pref, err := h.service.Upsert(c.Request.Context(), payload.TeacherID, service.UpsertTeacherPreferenceRequest{
		MaxLoadPerDay: payload.MaxDailySessions, MaxLoadPerWeek: payload.MaxDailySessions * 5, Unavailable: unavailable,
	})
	if err != nil {
		response.Error(c, err)
		return
	}
	response.JSON(c, http.StatusOK, legacyPreference(*pref), nil)
}

func legacyPreferences(prefs []models.TeacherPreference) []gin.H {
	result := make([]gin.H, len(prefs))
	for i, pref := range prefs {
		result[i] = legacyPreference(pref)
	}
	return result
}

func legacyPreference(pref models.TeacherPreference) gin.H {
	blocked := []int{}
	var unavailable []models.TeacherUnavailableSlot
	_ = json.Unmarshal(pref.Unavailable, &unavailable)
	for _, slot := range unavailable {
		if day, err := strconv.Atoi(slot.DayOfWeek); err == nil {
			blocked = append(blocked, day)
		}
	}
	return gin.H{"id": pref.ID, "teacher_id": pref.TeacherID, "preferred_days": []int{}, "blocked_days": blocked, "preferred_slots": []int{}, "max_daily_sessions": pref.MaxLoadPerDay, "availability_level": "MEDIUM", "notes": ""}
}

// Get godoc
// @Summary Get teacher preferences by ID
// @Tags Teacher Preferences
// @Produce json
// @Param id path string true "Teacher ID"
// @Success 200 {object} response.Envelope
// @Router /teachers/{id}/preferences [get]
func (h *TeacherPreferenceHandler) Get(c *gin.Context) {
	pref, err := h.service.Get(c.Request.Context(), c.Param("id"))
	if err != nil {
		response.Error(c, err)
		return
	}
	response.JSON(c, http.StatusOK, pref, nil)
}

// Upsert godoc
// @Summary Upsert teacher preferences
// @Tags Teacher Preferences
// @Accept json
// @Produce json
// @Param id path string true "Teacher ID"
// @Param payload body service.UpsertTeacherPreferenceRequest true "Preference payload"
// @Success 200 {object} response.Envelope
// @Router /teachers/{id}/preferences [put]
func (h *TeacherPreferenceHandler) Upsert(c *gin.Context) {
	var req service.UpsertTeacherPreferenceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, appErrors.Wrap(err, appErrors.ErrValidation.Code, http.StatusBadRequest, "invalid preference payload"))
		return
	}
	pref, err := h.service.Upsert(c.Request.Context(), c.Param("id"), req)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.JSON(c, http.StatusOK, pref, nil)
}
