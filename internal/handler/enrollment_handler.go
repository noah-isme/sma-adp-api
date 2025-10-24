package handler

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/noah-isme/sma-adp-api/internal/models"
	"github.com/noah-isme/sma-adp-api/internal/service"
	appErrors "github.com/noah-isme/sma-adp-api/pkg/errors"
	"github.com/noah-isme/sma-adp-api/pkg/response"
)

// EnrollmentHandler exposes enrollment endpoints.
type EnrollmentHandler struct {
	enrollments *service.EnrollmentService
}

// NewEnrollmentHandler constructs EnrollmentHandler.
func NewEnrollmentHandler(enrollments *service.EnrollmentService) *EnrollmentHandler {
	return &EnrollmentHandler{enrollments: enrollments}
}

// List godoc
// @Summary List enrollments
// @Tags Enrollments
// @Produce json
// @Param studentId query string false "Filter by student"
// @Param classId query string false "Filter by class"
// @Param termId query string false "Filter by term"
// @Param status query string false "Filter by status"
// @Param page query int false "Page"
// @Param limit query int false "Page size"
// @Success 200 {object} response.Envelope
// @Router /enrollments [get]
func (h *EnrollmentHandler) List(c *gin.Context) {
	var filter models.EnrollmentFilter
	filter.StudentID = c.Query("studentId")
	filter.ClassID = c.Query("classId")
	filter.TermID = c.Query("termId")
	filter.Status = models.EnrollmentStatus(strings.ToUpper(c.Query("status")))
	if page, err := strconv.Atoi(c.DefaultQuery("page", "1")); err == nil {
		filter.Page = page
	}
	if size, err := strconv.Atoi(c.DefaultQuery("limit", "20")); err == nil {
		filter.PageSize = size
	}
	filter.SortBy = c.Query("sort")
	filter.SortOrder = c.Query("order")

	enrollments, pagination, err := h.enrollments.List(c.Request.Context(), filter)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.JSON(c, http.StatusOK, enrollments, pagination)
}

// Create godoc
// @Summary Enroll student
// @Tags Enrollments
// @Accept json
// @Produce json
// @Param payload body service.EnrollStudentRequest true "Enrollment payload"
// @Success 201 {object} response.Envelope
// @Router /enrollments [post]
func (h *EnrollmentHandler) Create(c *gin.Context) {
	var req service.EnrollStudentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, appErrors.Wrap(err, appErrors.ErrValidation.Code, http.StatusBadRequest, "invalid payload"))
		return
	}
	enrollment, err := h.enrollments.Enroll(c.Request.Context(), req)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Created(c, enrollment)
}

// Transfer godoc
// @Summary Transfer enrollment to another class
// @Tags Enrollments
// @Accept json
// @Produce json
// @Param id path string true "Enrollment ID"
// @Param payload body service.TransferEnrollmentRequest true "Transfer payload"
// @Success 200 {object} response.Envelope
// @Router /enrollments/{id}/transfer [put]
func (h *EnrollmentHandler) Transfer(c *gin.Context) {
	var req service.TransferEnrollmentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, appErrors.Wrap(err, appErrors.ErrValidation.Code, http.StatusBadRequest, "invalid payload"))
		return
	}
	enrollment, err := h.enrollments.Transfer(c.Request.Context(), c.Param("id"), req)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.JSON(c, http.StatusOK, enrollment, nil)
}

// Delete godoc
// @Summary Unenroll student
// @Tags Enrollments
// @Produce json
// @Param id path string true "Enrollment ID"
// @Success 200 {object} response.Envelope
// @Router /enrollments/{id} [delete]
func (h *EnrollmentHandler) Delete(c *gin.Context) {
	enrollment, err := h.enrollments.Unenroll(c.Request.Context(), c.Param("id"))
	if err != nil {
		response.Error(c, err)
		return
	}
	response.JSON(c, http.StatusOK, enrollment, nil)
}
