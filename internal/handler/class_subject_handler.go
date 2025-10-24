package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/noah-isme/sma-adp-api/internal/service"
	appErrors "github.com/noah-isme/sma-adp-api/pkg/errors"
	"github.com/noah-isme/sma-adp-api/pkg/response"
)

// ClassSubjectHandler handles class subject assignments.
type ClassSubjectHandler struct {
	service *service.ClassService
}

// NewClassSubjectHandler constructs handler.
func NewClassSubjectHandler(service *service.ClassService) *ClassSubjectHandler {
	return &ClassSubjectHandler{service: service}
}

// List godoc
// @Summary List class subjects
// @Tags Class-Subjects
// @Produce json
// @Param id path string true "Class ID"
// @Success 200 {object} response.Envelope
// @Router /classes/{id}/subjects [get]
func (h *ClassSubjectHandler) List(c *gin.Context) {
	assignments, err := h.service.ListSubjects(c.Request.Context(), c.Param("id"))
	if err != nil {
		response.Error(c, err)
		return
	}
	response.JSON(c, http.StatusOK, assignments, nil)
}

// Assign godoc
// @Summary Assign subjects to class
// @Tags Class-Subjects
// @Accept json
// @Produce json
// @Param id path string true "Class ID"
// @Param payload body service.AssignSubjectsRequest true "Assignments payload"
// @Success 200 {object} response.Envelope
// @Router /classes/{id}/subjects [post]
func (h *ClassSubjectHandler) Assign(c *gin.Context) {
	var req service.AssignSubjectsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, appErrors.Wrap(err, appErrors.ErrValidation.Code, http.StatusBadRequest, "invalid payload"))
		return
	}
	if err := h.service.AssignSubjects(c.Request.Context(), c.Param("id"), req); err != nil {
		response.Error(c, err)
		return
	}
	response.JSON(c, http.StatusOK, gin.H{"status": "updated"}, nil)
}
