package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/noah-isme/sma-adp-api/internal/service"
	appErrors "github.com/noah-isme/sma-adp-api/pkg/errors"
	"github.com/noah-isme/sma-adp-api/pkg/response"
)

// ReportHandler exposes reporting endpoints.
type ReportHandler struct {
	grades *service.GradeService
}

// NewReportHandler constructs handler.
func NewReportHandler(grades *service.GradeService) *ReportHandler {
	return &ReportHandler{grades: grades}
}

// StudentReport godoc
// @Summary Student report card
// @Tags Reports
// @Produce json
// @Param id path string true "Student ID"
// @Param termId query string true "Term ID"
// @Success 200 {object} response.Envelope
// @Router /reports/students/{id} [get]
func (h *ReportHandler) StudentReport(c *gin.Context) {
	termID := c.Query("termId")
	if termID == "" {
		response.Error(c, appErrors.Clone(appErrors.ErrValidation, "termId required"))
		return
	}
	report, err := h.grades.ReportCard(c.Request.Context(), c.Param("id"), termID)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.JSON(c, http.StatusOK, report, nil)
}

// ClassReport godoc
// @Summary Class grade report
// @Tags Reports
// @Produce json
// @Param id path string true "Class ID"
// @Param subjectId query string true "Subject ID"
// @Param termId query string true "Term ID"
// @Success 200 {object} response.Envelope
// @Router /reports/classes/{id} [get]
func (h *ReportHandler) ClassReport(c *gin.Context) {
	subjectID := c.Query("subjectId")
	termID := c.Query("termId")
	if subjectID == "" || termID == "" {
		response.Error(c, appErrors.Clone(appErrors.ErrValidation, "subjectId and termId required"))
		return
	}
	report, err := h.grades.ClassReport(c.Request.Context(), c.Param("id"), subjectID, termID)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.JSON(c, http.StatusOK, report, nil)
}
