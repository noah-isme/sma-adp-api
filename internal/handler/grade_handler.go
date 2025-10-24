package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/noah-isme/sma-adp-api/internal/models"
	"github.com/noah-isme/sma-adp-api/internal/service"
	appErrors "github.com/noah-isme/sma-adp-api/pkg/errors"
	"github.com/noah-isme/sma-adp-api/pkg/response"
)

// GradeHandler exposes grade endpoints.
type GradeHandler struct {
	grades *service.GradeService
}

// NewGradeHandler constructs handler.
func NewGradeHandler(grades *service.GradeService) *GradeHandler {
	return &GradeHandler{grades: grades}
}

// List godoc
// @Summary List grade entries
// @Tags Grades
// @Produce json
// @Param enrollmentId query string false "Filter by enrollment"
// @Param subjectId query string false "Filter by subject"
// @Param componentId query string false "Filter by component"
// @Success 200 {object} response.Envelope
// @Router /grades [get]
func (h *GradeHandler) List(c *gin.Context) {
	filter := models.GradeFilter{EnrollmentID: c.Query("enrollmentId"), SubjectID: c.Query("subjectId"), ComponentID: c.Query("componentId")}
	grades, err := h.grades.List(c.Request.Context(), filter)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.JSON(c, http.StatusOK, grades, nil)
}

// Upsert godoc
// @Summary Upsert grade entry
// @Tags Grades
// @Accept json
// @Produce json
// @Param payload body service.UpsertGradeRequest true "Grade payload"
// @Success 200 {object} response.Envelope
// @Router /grades [post]
func (h *GradeHandler) Upsert(c *gin.Context) {
	var req service.UpsertGradeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, appErrors.Wrap(err, appErrors.ErrValidation.Code, http.StatusBadRequest, "invalid payload"))
		return
	}
	grade, err := h.grades.Upsert(c.Request.Context(), req)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.JSON(c, http.StatusOK, grade, nil)
}

// Bulk godoc
// @Summary Bulk upsert grades
// @Tags Grades
// @Accept json
// @Produce json
// @Param payload body service.BulkGradesRequest true "Bulk payload"
// @Success 200 {object} response.Envelope
// @Router /grades/bulk [post]
func (h *GradeHandler) Bulk(c *gin.Context) {
	var req service.BulkGradesRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, appErrors.Wrap(err, appErrors.ErrValidation.Code, http.StatusBadRequest, "invalid payload"))
		return
	}
	result, err := h.grades.BulkUpsert(c.Request.Context(), req)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.JSON(c, http.StatusOK, result, nil)
}

// Recalculate godoc
// @Summary Recalculate final grades
// @Tags Grades
// @Accept json
// @Produce json
// @Param payload body models.FinalGradeFilter true "Scope payload"
// @Success 200 {object} response.Envelope
// @Router /grades/recalculate [post]
func (h *GradeHandler) Recalculate(c *gin.Context) {
	var filter models.FinalGradeFilter
	if err := c.ShouldBindJSON(&filter); err != nil {
		response.Error(c, appErrors.Wrap(err, appErrors.ErrValidation.Code, http.StatusBadRequest, "invalid payload"))
		return
	}
	if err := h.grades.Recalculate(c.Request.Context(), filter); err != nil {
		response.Error(c, err)
		return
	}
	response.JSON(c, http.StatusOK, gin.H{"status": "recalculated"}, nil)
}

// Finalize godoc
// @Summary Finalize final grades
// @Tags Grades
// @Accept json
// @Produce json
// @Param payload body service.FinalizeGradesRequest true "Finalize payload"
// @Success 200 {object} response.Envelope
// @Router /grades/finalize [post]
func (h *GradeHandler) Finalize(c *gin.Context) {
	var req service.FinalizeGradesRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, appErrors.Wrap(err, appErrors.ErrValidation.Code, http.StatusBadRequest, "invalid payload"))
		return
	}
	if err := h.grades.Finalize(c.Request.Context(), req); err != nil {
		response.Error(c, err)
		return
	}
	response.JSON(c, http.StatusOK, gin.H{"status": "finalized"}, nil)
}
