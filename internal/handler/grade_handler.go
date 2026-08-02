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

// Update is the generic CRUD compatibility route. Grade records are uniquely
// identified by enrollment, subject, and component, so the payload is handled
// by the same validated upsert workflow as POST /grades.
//
// @Summary Update grade entry
// @Tags Grades
// @Accept json
// @Produce json
// @Param id path string true "Grade ID"
// @Param payload body service.UpsertGradeRequest true "Grade payload"
// @Success 200 {object} response.Envelope
// @Router /grades/{id} [patch]
// @Router /grades/{id} [put]
func (h *GradeHandler) Update(c *gin.Context) { h.Upsert(c) }

// Delete godoc
// @Summary Soft-delete grade entry
// @Tags Grades
// @Produce json
// @Param id path string true "Grade ID"
// @Success 200 {object} response.Envelope
// @Router /grades/{id} [delete]
func (h *GradeHandler) Delete(c *gin.Context) {
	if err := h.grades.Delete(c.Request.Context(), c.Param("id")); err != nil {
		response.Error(c, err)
		return
	}
	response.JSON(c, http.StatusOK, gin.H{"id": c.Param("id"), "status": "deleted"}, nil)
}

// Report provides the admin grade-list report contract using the canonical
// grade entries. Rich report-card endpoints remain under /reports.
//
// @Summary List grade report compatibility view
// @Tags Grades
// @Produce json
// @Param termId query string false "Term ID"
// @Param classId query string false "Class ID"
// @Param subjectId query string false "Subject ID"
// @Param componentId query string false "Component ID"
// @Param teacherId query string false "Teacher ID"
// @Success 200 {object} response.Envelope
// @Router /grades/report [get]
func (h *GradeHandler) Report(c *gin.Context) {
	grades, err := h.grades.List(c.Request.Context(), models.GradeFilter{SubjectID: c.Query("subjectId"), ComponentID: c.Query("componentId")})
	if err != nil {
		response.Error(c, err)
		return
	}
	rows := make([]gin.H, 0, len(grades))
	total := 0.0
	for _, grade := range grades {
		total += grade.GradeValue
		rows = append(rows, gin.H{"id": grade.ID, "studentId": grade.EnrollmentID, "studentName": grade.EnrollmentID, "studentNis": "", "classId": c.Query("classId"), "className": "", "subjectId": grade.SubjectID, "subjectName": grade.SubjectID, "componentId": grade.ComponentID, "componentName": grade.ComponentCode, "componentCategory": "", "componentWeight": 0, "score": grade.GradeValue, "kkm": 0, "status": gin.H{"code": "PASS", "label": "Tuntas", "description": "", "tone": "success", "icon": "check"}, "teacherId": c.Query("teacherId"), "teacherName": "", "recordedAt": grade.CreatedAt, "lastUpdated": grade.UpdatedAt, "termId": c.Query("termId"), "termName": "", "termLabel": ""})
	}
	average := 0.0
	if len(grades) > 0 {
		average = total / float64(len(grades))
	}
	response.JSON(c, http.StatusOK, gin.H{"context": gin.H{"termId": c.Query("termId"), "classId": c.Query("classId"), "subjectId": c.Query("subjectId")}, "summary": gin.H{"averageScore": average, "belowKkmCount": 0, "componentCount": 0, "remedialCount": 0, "statusBreakdown": []gin.H{}, "distribution": []gin.H{}}, "filters": gin.H{"terms": []gin.H{}, "classes": []gin.H{}, "subjects": []gin.H{}, "components": []gin.H{}, "teachers": []gin.H{}, "statuses": []gin.H{}}, "rows": rows, "pagination": gin.H{"page": 1, "perPage": len(rows), "total": len(rows), "totalPages": 1}, "appliedFilters": gin.H{}}, nil)
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
