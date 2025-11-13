package handler

import (
	"context"
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/noah-isme/sma-adp-api/internal/dto"
	"github.com/noah-isme/sma-adp-api/internal/middleware"
	"github.com/noah-isme/sma-adp-api/internal/models"
	"github.com/noah-isme/sma-adp-api/internal/service"
	appErrors "github.com/noah-isme/sma-adp-api/pkg/errors"
	"github.com/noah-isme/sma-adp-api/pkg/response"
)

// ReportHandler exposes reporting endpoints.
type reportService interface {
	CreateJob(ctx context.Context, req dto.ReportRequest, actorID string, role models.UserRole) (*dto.ReportJobResponse, error)
	GetStatus(ctx context.Context, id string, actorID string, role models.UserRole) (*dto.ReportStatusResponse, error)
	ResolveDownload(ctx context.Context, token string) (*service.ReportDownload, error)
}

// ReportHandler exposes reporting endpoints.
type ReportHandler struct {
	grades  *service.GradeService
	reports reportService
}

// NewReportHandler constructs handler.
func NewReportHandler(reportSvc reportService, gradeSvc *service.GradeService) *ReportHandler {
	return &ReportHandler{grades: gradeSvc, reports: reportSvc}
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
	if h.grades == nil {
		response.Error(c, appErrors.Clone(appErrors.ErrInternal, "grade service not available"))
		return
	}
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
	if h.grades == nil {
		response.Error(c, appErrors.Clone(appErrors.ErrInternal, "grade service not available"))
		return
	}
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

// GenerateReport godoc
// @Summary Queue a new report job
// @Tags Reports
// @Accept json
// @Produce json
// @Param payload body dto.ReportRequest true "Report request"
// @Success 202 {object} response.Envelope
// @Router /reports/generate [post]
func (h *ReportHandler) GenerateReport(c *gin.Context) {
	if h.reports == nil {
		response.Error(c, appErrors.Clone(appErrors.ErrInternal, "report service not configured"))
		return
	}
	var req dto.ReportRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, appErrors.Clone(appErrors.ErrValidation, "invalid report payload"))
		return
	}
	claimsValue, exists := c.Get(middleware.ContextUserKey)
	if !exists {
		response.Error(c, appErrors.ErrUnauthorized)
		return
	}
	claims, ok := claimsValue.(*models.JWTClaims)
	if !ok {
		response.Error(c, appErrors.ErrUnauthorized)
		return
	}
	job, err := h.reports.CreateJob(c.Request.Context(), req, claims.UserID, claims.Role)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.JSON(c, http.StatusAccepted, job, nil)
}

// ReportStatus godoc
// @Summary Get report job status
// @Tags Reports
// @Produce json
// @Param id path string true "Job ID"
// @Success 200 {object} response.Envelope
// @Router /reports/status/{id} [get]
func (h *ReportHandler) ReportStatus(c *gin.Context) {
	if h.reports == nil {
		response.Error(c, appErrors.Clone(appErrors.ErrInternal, "report service not configured"))
		return
	}
	claimsValue, exists := c.Get(middleware.ContextUserKey)
	if !exists {
		response.Error(c, appErrors.ErrUnauthorized)
		return
	}
	claims, ok := claimsValue.(*models.JWTClaims)
	if !ok {
		response.Error(c, appErrors.ErrUnauthorized)
		return
	}
	status, err := h.reports.GetStatus(c.Request.Context(), c.Param("id"), claims.UserID, claims.Role)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.JSON(c, http.StatusOK, status, nil)
}

// DownloadReport godoc
// @Summary Download generated report via signed token
// @Tags Reports
// @Produce octet-stream
// @Param token path string true "Signed token"
// @Success 200 {file} binary
// @Router /export/{token} [get]
func (h *ReportHandler) DownloadReport(c *gin.Context) {
	if h.reports == nil {
		response.Error(c, appErrors.Clone(appErrors.ErrInternal, "report service not configured"))
		return
	}
	token := c.Param("token")
	if token == "" {
		response.Error(c, appErrors.Clone(appErrors.ErrValidation, "token required"))
		return
	}
	file, err := h.reports.ResolveDownload(c.Request.Context(), token)
	if err != nil {
		response.Error(c, err)
		return
	}
	defer file.File.Close() //nolint:errcheck
	info, err := file.File.Stat()
	if err != nil {
		response.Error(c, appErrors.Wrap(err, appErrors.ErrInternal.Code, appErrors.ErrInternal.Status, "failed to read export metadata"))
		return
	}
	contentType := mimeForFormat(file.Format)
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", file.Filename))
	c.Header("Cache-Control", "no-store")
	c.DataFromReader(http.StatusOK, info.Size(), contentType, file.File, nil)
}

func mimeForFormat(format models.ReportFormat) string {
	switch format {
	case models.ReportFormatPDF:
		return "application/pdf"
	default:
		return "text/csv"
	}
}
