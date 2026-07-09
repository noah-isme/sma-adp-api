package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/noah-isme/sma-adp-api/internal/service"
	appErrors "github.com/noah-isme/sma-adp-api/pkg/errors"
	"github.com/noah-isme/sma-adp-api/pkg/response"
)

// BehaviorHandler exposes behavior note endpoints.
type BehaviorHandler struct {
	behavior *service.BehaviorService
}

// NewBehaviorHandler constructs BehaviorHandler.
func NewBehaviorHandler(behavior *service.BehaviorService) *BehaviorHandler {
	return &BehaviorHandler{behavior: behavior}
}

// List godoc
// @Summary List behavior notes
// @Tags Behavior
// @Produce json
// @Param studentId query string false "Student ID"
// @Param dateFrom query string false "From date (YYYY-MM-DD)"
// @Param dateTo query string false "To date (YYYY-MM-DD)"
// @Param noteTypes query string false "Comma-separated note types (+,-,0)"
// @Param page query int false "Page"
// @Param limit query int false "Page size"
// @Success 200 {object} response.Envelope
// @Router /behavior-notes [get]
func (h *BehaviorHandler) List(c *gin.Context) {
	from, err := parseDateParam(c.Query("dateFrom"))
	if err != nil {
		response.Error(c, err)
		return
	}
	to, err := parseDateParam(c.Query("dateTo"))
	if err != nil {
		response.Error(c, err)
		return
	}
	req := service.BehaviorListRequest{
		StudentID: c.Query("studentId"),
		DateFrom:  from,
		DateTo:    to,
		NoteTypes: splitCSV(c.Query("noteTypes")),
		Page:      parseQueryInt(c, "page", 1),
		PageSize:  parseQueryInt(c, "limit", 50),
	}
	notes, pagination, err := h.behavior.List(c.Request.Context(), req)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.JSON(c, http.StatusOK, notes, pagination)
}

// Create godoc
// @Summary Create behavior note
// @Tags Behavior
// @Accept json
// @Produce json
// @Param payload body service.CreateBehaviorRequest true "Behavior payload"
// @Success 201 {object} response.Envelope
// @Router /behavior-notes [post]
func (h *BehaviorHandler) Create(c *gin.Context) {
	claims := claimsFromContext(c)
	if claims == nil {
		response.Error(c, appErrors.ErrUnauthorized)
		return
	}
	var req service.CreateBehaviorRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, appErrors.Wrap(err, appErrors.ErrValidation.Code, http.StatusBadRequest, "invalid behavior payload"))
		return
	}
	req.CreatedBy = claims.UserID

	note, err := h.behavior.Create(c.Request.Context(), req)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Created(c, note)
}

// Update godoc
// @Summary Update behavior note
// @Tags Behavior
// @Accept json
// @Produce json
// @Param id path string true "Behavior note ID"
// @Param payload body service.UpdateBehaviorRequest true "Behavior payload"
// @Success 200 {object} response.Envelope
// @Router /behavior-notes/{id} [put]
func (h *BehaviorHandler) Update(c *gin.Context) {
	var req service.UpdateBehaviorRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, appErrors.Wrap(err, appErrors.ErrValidation.Code, http.StatusBadRequest, "invalid behavior payload"))
		return
	}
	note, err := h.behavior.Update(c.Request.Context(), c.Param("id"), req)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.JSON(c, http.StatusOK, note, nil)
}

// Delete godoc
// @Summary Delete behavior note
// @Tags Behavior
// @Produce json
// @Param id path string true "Behavior note ID"
// @Success 204
// @Router /behavior-notes/{id} [delete]
func (h *BehaviorHandler) Delete(c *gin.Context) {
	if err := h.behavior.Delete(c.Request.Context(), c.Param("id")); err != nil {
		response.Error(c, err)
		return
	}
	response.NoContent(c)
}

// Summary godoc
// @Summary Get student behavior summary
// @Tags Behavior
// @Produce json
// @Param id path string true "Student ID"
// @Success 200 {object} response.Envelope
// @Router /students/{id}/behavior-summary [get]
func (h *BehaviorHandler) Summary(c *gin.Context) {
	studentID := c.Param("studentId")
	if studentID == "" {
		studentID = c.Param("id")
	}
	summary, err := h.behavior.Summary(c.Request.Context(), studentID)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.JSON(c, http.StatusOK, summary, nil)
}
