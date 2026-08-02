package handler

import (
	"encoding/csv"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/noah-isme/sma-adp-api/internal/models"
	"github.com/noah-isme/sma-adp-api/internal/service"
	appErrors "github.com/noah-isme/sma-adp-api/pkg/errors"
	"github.com/noah-isme/sma-adp-api/pkg/response"
)

// StudentHandler exposes student endpoints.
type StudentHandler struct {
	students *service.StudentService
}

// ImportCSV imports the supported student CSV columns: nis, full_name, gender,
// birth_date, address, phone. Invalid rows are reported without rolling back
// valid rows.
//
// @Summary Import students from CSV
// @Tags Students
// @Accept text/csv
// @Produce json
// @Param csv body string true "CSV document"
// @Success 200 {object} response.Envelope
// @Router /students/import [post]
func (h *StudentHandler) ImportCSV(c *gin.Context) {
	r := csv.NewReader(c.Request.Body)
	header, err := r.Read()
	if err != nil {
		response.Error(c, appErrors.Clone(appErrors.ErrValidation, "CSV header required"))
		return
	}
	columns := map[string]int{}
	for i, name := range header {
		columns[strings.TrimSpace(strings.ToLower(name))] = i
	}
	for _, required := range []string{"nis", "full_name", "gender", "birth_date"} {
		if _, ok := columns[required]; !ok {
			response.Error(c, appErrors.Clone(appErrors.ErrValidation, "missing CSV column: "+required))
			return
		}
	}
	created, failures, row := 0, []gin.H{}, 1
	for {
		row++
		values, err := r.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			failures = append(failures, gin.H{"row": row, "error": err.Error()})
			continue
		}
		value := func(name string) string {
			i, ok := columns[name]
			if !ok || i >= len(values) {
				return ""
			}
			return strings.TrimSpace(values[i])
		}
		birthDate, err := time.Parse("2006-01-02", value("birth_date"))
		if err != nil {
			failures = append(failures, gin.H{"row": row, "error": "invalid birth_date"})
			continue
		}
		if _, err = h.students.Create(c.Request.Context(), service.CreateStudentRequest{NIS: value("nis"), FullName: value("full_name"), Gender: value("gender"), BirthDate: birthDate, Address: value("address"), Phone: value("phone")}); err != nil {
			failures = append(failures, gin.H{"row": row, "error": err.Error()})
			continue
		}
		created++
	}
	response.JSON(c, http.StatusOK, gin.H{"created": created, "failed": len(failures), "failures": failures}, nil)
}

// NewStudentHandler constructs StudentHandler.
func NewStudentHandler(students *service.StudentService) *StudentHandler {
	return &StudentHandler{students: students}
}

// List godoc
// @Summary List students
// @Tags Students
// @Produce json
// @Param search query string false "Search by name or NIS"
// @Param classId query string false "Filter by class"
// @Param active query bool false "Filter by active state"
// @Param page query int false "Page"
// @Param limit query int false "Page size"
// @Success 200 {object} response.Envelope
// @Router /students [get]
func (h *StudentHandler) List(c *gin.Context) {
	filter := studentFilter(c)

	students, pagination, err := h.students.List(c.Request.Context(), filter)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.JSON(c, http.StatusOK, students, pagination)
}

// Roster preserves the admin screen's aggregate response shape while using
// the canonical student service as its data source.
//
// @Summary List student roster
// @Tags Students
// @Produce json
// @Param search query string false "Search by name or NIS"
// @Param classId query string false "Filter by class"
// @Param active query bool false "Filter by active state"
// @Param page query int false "Page"
// @Param perPage query int false "Page size"
// @Param sort query string false "Sort field"
// @Param order query string false "Sort order"
// @Success 200 {object} response.Envelope
// @Router /students/roster [get]
func (h *StudentHandler) Roster(c *gin.Context) {
	filter := studentFilter(c)
	students, pagination, err := h.students.List(c.Request.Context(), filter)
	if err != nil {
		response.Error(c, err)
		return
	}
	rows := make([]gin.H, 0, len(students))
	activeCount := 0
	for _, student := range students {
		status := "inactive"
		if student.Active {
			status = "active"
			activeCount++
		}
		classID, className := "", ""
		if student.CurrentClassID != nil {
			classID = *student.CurrentClassID
		}
		if student.CurrentClassName != nil {
			className = *student.CurrentClassName
		}
		rows = append(rows, gin.H{"id": student.ID, "nis": student.NIS, "fullName": student.FullName, "gender": student.Gender, "birthDate": student.BirthDate, "classId": classID, "className": className, "status": status, "address": student.Address, "lastUpdated": student.UpdatedAt, "createdAt": student.CreatedAt})
	}
	response.JSON(c, http.StatusOK, gin.H{"summary": gin.H{"totalStudents": pagination.TotalCount, "activeStudents": activeCount, "inactiveStudents": len(students) - activeCount, "alumniStudents": 0, "activeRate": percent(activeCount, len(students)), "genderBreakdown": []gin.H{}, "classDistribution": []gin.H{}, "statusBreakdown": []gin.H{}}, "filters": gin.H{"classes": []gin.H{}, "statuses": []gin.H{}, "genders": []gin.H{}, "guardians": []gin.H{}, "birthYears": []gin.H{}, "tracks": []gin.H{}}, "rows": rows, "pagination": gin.H{"page": pagination.Page, "perPage": pagination.PageSize, "total": pagination.TotalCount, "totalPages": pageCount(pagination.TotalCount, pagination.PageSize)}, "appliedFilters": gin.H{"page": pagination.Page, "perPage": pagination.PageSize, "search": filter.Search, "classId": filter.ClassID}}, nil)
}

func studentFilter(c *gin.Context) models.StudentFilter {
	filter := models.StudentFilter{Search: strings.TrimSpace(c.Query("search")), ClassID: c.Query("classId"), SortBy: c.Query("sort"), SortOrder: c.Query("order")}
	if active := c.Query("active"); active != "" {
		v := active == "true"
		filter.Active = &v
	}
	if page, err := strconv.Atoi(c.DefaultQuery("page", "1")); err == nil {
		filter.Page = page
	}
	sizeQuery := c.DefaultQuery("limit", c.DefaultQuery("perPage", "20"))
	if size, err := strconv.Atoi(sizeQuery); err == nil {
		filter.PageSize = size
	}
	return filter
}

func percent(n, total int) float64 {
	if total == 0 {
		return 0
	}
	return float64(n) * 100 / float64(total)
}

func pageCount(total, size int) int {
	if size <= 0 {
		return 0
	}
	return (total + size - 1) / size
}

// Get godoc
// @Summary Get student detail
// @Tags Students
// @Produce json
// @Param id path string true "Student ID"
// @Success 200 {object} response.Envelope
// @Router /students/{id} [get]
func (h *StudentHandler) Get(c *gin.Context) {
	student, err := h.students.Get(c.Request.Context(), c.Param("id"))
	if err != nil {
		response.Error(c, err)
		return
	}
	response.JSON(c, http.StatusOK, student, nil)
}

// Create godoc
// @Summary Create student
// @Tags Students
// @Accept json
// @Produce json
// @Param payload body service.CreateStudentRequest true "Student payload"
// @Success 201 {object} response.Envelope
// @Router /students [post]
func (h *StudentHandler) Create(c *gin.Context) {
	var req service.CreateStudentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, appErrors.Wrap(err, appErrors.ErrValidation.Code, http.StatusBadRequest, "invalid payload"))
		return
	}
	student, err := h.students.Create(c.Request.Context(), req)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Created(c, student)
}

// Update godoc
// @Summary Update student
// @Tags Students
// @Accept json
// @Produce json
// @Param id path string true "Student ID"
// @Param payload body service.UpdateStudentRequest true "Student payload"
// @Success 200 {object} response.Envelope
// @Router /students/{id} [put]
func (h *StudentHandler) Update(c *gin.Context) {
	var req service.UpdateStudentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, appErrors.Wrap(err, appErrors.ErrValidation.Code, http.StatusBadRequest, "invalid payload"))
		return
	}
	student, err := h.students.Update(c.Request.Context(), c.Param("id"), req)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.JSON(c, http.StatusOK, student, nil)
}

// UpdateStatus toggles a student's active state without requiring the full
// student update payload used by PUT /students/:id.
// @Summary Update student status
// @Tags Students
// @Accept json
// @Produce json
// @Param id path string true "Student ID"
// @Param payload body map[string]interface{} true "Status payload"
// @Success 200 {object} response.Envelope
// @Router /students/{id}/status [patch]
func (h *StudentHandler) UpdateStatus(c *gin.Context) {
	var payload struct {
		Status string `json:"status"`
		Active *bool  `json:"active"`
	}
	if err := c.ShouldBindJSON(&payload); err != nil {
		response.Error(c, appErrors.Wrap(err, appErrors.ErrValidation.Code, http.StatusBadRequest, "invalid student status payload"))
		return
	}
	active := payload.Active
	if active == nil {
		value := strings.ToLower(strings.TrimSpace(payload.Status))
		switch value {
		case "active":
			v := true
			active = &v
		case "inactive":
			v := false
			active = &v
		default:
			response.Error(c, appErrors.Clone(appErrors.ErrValidation, "status must be active or inactive"))
			return
		}
	}
	student, err := h.students.SetActive(c.Request.Context(), c.Param("id"), *active)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.JSON(c, http.StatusOK, student, nil)
}

// Delete godoc
// @Summary Deactivate student
// @Tags Students
// @Produce json
// @Param id path string true "Student ID"
// @Success 204
// @Router /students/{id} [delete]
func (h *StudentHandler) Delete(c *gin.Context) {
	if err := h.students.Deactivate(c.Request.Context(), c.Param("id")); err != nil {
		response.Error(c, err)
		return
	}
	response.NoContent(c)
}
