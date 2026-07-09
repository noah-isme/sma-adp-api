package handler

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/noah-isme/sma-adp-api/internal/service"
	appErrors "github.com/noah-isme/sma-adp-api/pkg/errors"
	"github.com/noah-isme/sma-adp-api/pkg/response"
)

// CalendarHandler exposes calendar event CRUD endpoints.
type CalendarHandler struct {
	calendar *service.CalendarService
}

// NewCalendarHandler constructs CalendarHandler.
func NewCalendarHandler(calendar *service.CalendarService) *CalendarHandler {
	return &CalendarHandler{calendar: calendar}
}

// List godoc
// @Summary List calendar events
// @Tags Calendar
// @Produce json
// @Param startDate query string false "Start date (YYYY-MM-DD)"
// @Param endDate query string false "End date (YYYY-MM-DD)"
// @Param audience query string false "Comma-separated audience filters"
// @Param classIds query string false "Comma-separated class IDs"
// @Param page query int false "Page"
// @Param limit query int false "Page size"
// @Success 200 {object} response.Envelope
// @Router /calendar-events [get]
func (h *CalendarHandler) List(c *gin.Context) {
	start, err := parseDateParam(c.Query("startDate"))
	if err != nil {
		response.Error(c, err)
		return
	}
	end, err := parseDateParam(c.Query("endDate"))
	if err != nil {
		response.Error(c, err)
		return
	}
	req := service.CalendarListRequest{
		StartDate: start,
		EndDate:   end,
		Audience:  splitUpperCSV(c.Query("audience")),
		ClassIDs:  splitCSV(c.Query("classIds")),
		Page:      parseQueryInt(c, "page", 1),
		PageSize:  parseQueryInt(c, "limit", 50),
	}
	events, pagination, err := h.calendar.List(c.Request.Context(), req)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.JSON(c, http.StatusOK, events, pagination)
}

// Get godoc
// @Summary Get calendar event
// @Tags Calendar
// @Produce json
// @Param id path string true "Calendar event ID"
// @Success 200 {object} response.Envelope
// @Router /calendar-events/{id} [get]
func (h *CalendarHandler) Get(c *gin.Context) {
	event, err := h.calendar.Get(c.Request.Context(), c.Param("id"))
	if err != nil {
		response.Error(c, err)
		return
	}
	response.JSON(c, http.StatusOK, event, nil)
}

// Create godoc
// @Summary Create calendar event
// @Tags Calendar
// @Accept json
// @Produce json
// @Param payload body service.CreateCalendarEventRequest true "Calendar event payload"
// @Success 201 {object} response.Envelope
// @Router /calendar-events [post]
func (h *CalendarHandler) Create(c *gin.Context) {
	claims := claimsFromContext(c)
	if claims == nil {
		response.Error(c, appErrors.ErrUnauthorized)
		return
	}
	var req service.CreateCalendarEventRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, appErrors.Wrap(err, appErrors.ErrValidation.Code, http.StatusBadRequest, "invalid calendar event payload"))
		return
	}
	req.CreatedBy = claims.UserID

	event, err := h.calendar.Create(c.Request.Context(), req)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Created(c, event)
}

// Update godoc
// @Summary Update calendar event
// @Tags Calendar
// @Accept json
// @Produce json
// @Param id path string true "Calendar event ID"
// @Param payload body service.UpdateCalendarEventRequest true "Calendar event payload"
// @Success 200 {object} response.Envelope
// @Router /calendar-events/{id} [put]
func (h *CalendarHandler) Update(c *gin.Context) {
	var req service.UpdateCalendarEventRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, appErrors.Wrap(err, appErrors.ErrValidation.Code, http.StatusBadRequest, "invalid calendar event payload"))
		return
	}
	event, err := h.calendar.Update(c.Request.Context(), c.Param("id"), req)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.JSON(c, http.StatusOK, event, nil)
}

// Delete godoc
// @Summary Delete calendar event
// @Tags Calendar
// @Produce json
// @Param id path string true "Calendar event ID"
// @Success 204
// @Router /calendar-events/{id} [delete]
func (h *CalendarHandler) Delete(c *gin.Context) {
	if err := h.calendar.Delete(c.Request.Context(), c.Param("id")); err != nil {
		response.Error(c, err)
		return
	}
	response.NoContent(c)
}

func splitUpperCSV(raw string) []string {
	parts := splitCSV(raw)
	for i := range parts {
		parts[i] = strings.ToUpper(parts[i])
	}
	return parts
}
