package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/noah-isme/sma-adp-api/internal/dto"
	"github.com/noah-isme/sma-adp-api/internal/service"
	appErrors "github.com/noah-isme/sma-adp-api/pkg/errors"
	"github.com/noah-isme/sma-adp-api/pkg/response"
)

// ScheduleGeneratorHandler exposes scheduler endpoints.
type ScheduleGeneratorHandler struct {
	service *service.ScheduleGeneratorService
}

// NewScheduleGeneratorHandler constructs the handler.
func NewScheduleGeneratorHandler(svc *service.ScheduleGeneratorService) *ScheduleGeneratorHandler {
	return &ScheduleGeneratorHandler{service: svc}
}

// Generate godoc
// @Summary Generate conflict-free schedule proposal
// @Tags Scheduler
// @Accept json
// @Produce json
// @Param payload body dto.GenerateScheduleRequest true "Generate schedule payload"
// @Success 200 {object} response.Envelope
// @Router /schedule/generate [post]
func (h *ScheduleGeneratorHandler) Generate(c *gin.Context) {
	var req dto.GenerateScheduleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, appErrors.Wrap(err, appErrors.ErrValidation.Code, http.StatusBadRequest, "invalid generate payload"))
		return
	}
	result, err := h.service.Generate(c.Request.Context(), req)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.JSON(c, http.StatusOK, result, nil)
}

// Save godoc
// @Summary Save schedule proposal to semester schedules
// @Tags Scheduler
// @Accept json
// @Produce json
// @Param payload body dto.SaveScheduleRequest true "Save schedule payload"
// @Success 201 {object} response.Envelope
// @Router /schedule/save [post]
func (h *ScheduleGeneratorHandler) Save(c *gin.Context) {
	var req dto.SaveScheduleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, appErrors.Wrap(err, appErrors.ErrValidation.Code, http.StatusBadRequest, "invalid save payload"))
		return
	}
	id, err := h.service.Save(c.Request.Context(), req)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Created(c, gin.H{"scheduleId": id})
}

// List godoc
// @Summary List semester schedules for class-term
// @Tags Scheduler
// @Produce json
// @Param termId query string true "Term ID"
// @Param classId query string true "Class ID"
// @Success 200 {object} response.Envelope
// @Router /semester-schedule [get]
func (h *ScheduleGeneratorHandler) List(c *gin.Context) {
	query := dto.SemesterScheduleQuery{
		TermID:  c.Query("termId"),
		ClassID: c.Query("classId"),
	}
	result, err := h.service.List(c.Request.Context(), query)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.JSON(c, http.StatusOK, result, nil)
}

// Slots godoc
// @Summary Get slots for a semester schedule
// @Tags Scheduler
// @Produce json
// @Param id path string true "Semester schedule ID"
// @Success 200 {object} response.Envelope
// @Router /semester-schedule/{id}/slots [get]
func (h *ScheduleGeneratorHandler) Slots(c *gin.Context) {
	slots, err := h.service.GetSlots(c.Request.Context(), c.Param("id"))
	if err != nil {
		response.Error(c, err)
		return
	}
	response.JSON(c, http.StatusOK, slots, nil)
}

// Delete godoc
// @Summary Delete draft semester schedule
// @Tags Scheduler
// @Param id path string true "Semester schedule ID"
// @Success 204
// @Router /semester-schedule/{id} [delete]
func (h *ScheduleGeneratorHandler) Delete(c *gin.Context) {
	if err := h.service.Delete(c.Request.Context(), c.Param("id")); err != nil {
		response.Error(c, err)
		return
	}
	response.NoContent(c)
}
