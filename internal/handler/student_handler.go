package handler

import (
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
	imports  importRunStore
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
// @Param Idempotency-Key header string false "Stable key for safe retries"
// @Success 200 {object} response.Envelope
// @Security BearerAuth
// @Router /students/import [post]
func (h *StudentHandler) ImportCSV(c *gin.Context) {
	body, ok := readCSVImportBody(c)
	if !ok {
		return
	}
	columns, rows, err := parseCSVImport(body, []string{"nis", "full_name", "gender", "birth_date"})
	if err != nil {
		response.Error(c, err)
		return
	}
	run, proceed := beginCSVImport(c, h.imports, "students", body)
	if !proceed {
		return
	}
	created, failures := 0, []gin.H{}
	for index, values := range rows {
		row := index + 2
		birthDate, err := time.Parse("2006-01-02", importValue(values, columns, "birth_date"))
		if err != nil {
			failures = append(failures, gin.H{"row": row, "error": "invalid birth_date"})
			continue
		}
		if _, err = h.students.Create(c.Request.Context(), service.CreateStudentRequest{NIS: importValue(values, columns, "nis"), FullName: importValue(values, columns, "full_name"), Gender: importValue(values, columns, "gender"), BirthDate: birthDate, Address: importValue(values, columns, "address"), Phone: importValue(values, columns, "phone")}); err != nil {
			failures = append(failures, gin.H{"row": row, "error": err.Error()})
			continue
		}
		created++
	}
	completeCSVImport(c, h.imports, run, "students", created, failures)
}

// NewStudentHandler constructs StudentHandler.
func NewStudentHandler(students *service.StudentService, stores ...importRunStore) *StudentHandler {
	var imports importRunStore
	if len(stores) > 0 {
		imports = stores[0]
	}
	return &StudentHandler{students: students, imports: imports}
}

// List godoc
// @Summary List students
// @Tags Students
// @Produce json
// @Param search query string false "Search by name or NIS"
// @Param classId query string false "Filter by class"
// @Param status query string false "Filter by status (active or inactive)"
// @Param active query bool false "Legacy alias for status"
// @Param page query int false "Page"
// @Param limit query int false "Page size"
// @Success 200 {object} response.Envelope
// @Security BearerAuth
// @Router /students [get]
func (h *StudentHandler) List(c *gin.Context) {
	if !validateRosterStatus(c) {
		return
	}
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
// @Param status query string false "Filter by status (active or inactive)"
// @Param active query bool false "Legacy alias for status"
// @Param gender query string false "Filter by gender"
// @Param track query string false "Filter by track"
// @Param guardian query string false "Filter by guardian"
// @Param birthYearStart query int false "Birth year start"
// @Param birthYearEnd query int false "Birth year end"
// @Param page query int false "Page"
// @Param perPage query int false "Page size"
// @Param sortField query string false "Sort field"
// @Param sortOrder query string false "Sort order (ascend or descend)"
// @Param sort query string false "Legacy alias for sortField"
// @Param order query string false "Legacy alias for sortOrder"
// @Success 200 {object} response.Envelope
// @Security BearerAuth
// @Router /students/roster [get]
func (h *StudentHandler) Roster(c *gin.Context) {
	if !validateRosterStatus(c) {
		return
	}
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
	response.JSON(c, http.StatusOK, gin.H{"summary": gin.H{"totalStudents": pagination.TotalCount, "activeStudents": activeCount, "inactiveStudents": len(students) - activeCount, "alumniStudents": 0, "activeRate": percent(activeCount, len(students)), "genderBreakdown": []gin.H{}, "classDistribution": []gin.H{}, "statusBreakdown": []gin.H{}}, "filters": gin.H{"classes": []gin.H{}, "statuses": []gin.H{{"value": "active", "label": "Aktif"}, {"value": "inactive", "label": "Tidak aktif"}}, "genders": []gin.H{}, "guardians": []gin.H{}, "birthYears": []gin.H{}, "tracks": []gin.H{}}, "rows": rows, "pagination": gin.H{"page": pagination.Page, "perPage": pagination.PageSize, "total": pagination.TotalCount, "totalPages": pageCount(pagination.TotalCount, pagination.PageSize)}, "appliedFilters": gin.H{"page": pagination.Page, "perPage": pagination.PageSize, "search": filter.Search, "classId": filter.ClassID, "status": rosterStatusValue(c), "active": filter.Active, "gender": filter.Gender, "track": filter.Track, "guardian": filter.Guardian, "birthYearStart": filter.BirthYearStart, "birthYearEnd": filter.BirthYearEnd, "sortField": filter.SortBy, "sortOrder": filter.SortOrder}}, nil)
}

func studentFilter(c *gin.Context) models.StudentFilter {
	filter := models.StudentFilter{
		Search:    strings.TrimSpace(c.Query("search")),
		ClassID:   c.Query("classId"),
		Gender:    c.Query("gender"),
		Track:     c.Query("track"),
		Guardian:  c.Query("guardian"),
		SortBy:    firstQuery(c, "sortField", "sort"),
		SortOrder: firstQuery(c, "sortOrder", "order"),
	}
	if active := firstQuery(c, "status", "active"); active != "" {
		v := strings.EqualFold(active, "true") || strings.EqualFold(active, "active")
		filter.Active = &v
	}
	if birthYearStart := c.Query("birthYearStart"); birthYearStart != "" {
		if v, err := strconv.Atoi(birthYearStart); err == nil {
			filter.BirthYearStart = &v
		}
	}
	if birthYearEnd := c.Query("birthYearEnd"); birthYearEnd != "" {
		if v, err := strconv.Atoi(birthYearEnd); err == nil {
			filter.BirthYearEnd = &v
		}
	}
	if birthYear := c.Query("birthYear"); birthYear != "" {
		if v, err := strconv.Atoi(birthYear); err == nil {
			if filter.BirthYearStart == nil {
				filter.BirthYearStart = &v
			}
			if filter.BirthYearEnd == nil {
				filter.BirthYearEnd = &v
			}
		}
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

func firstQuery(c *gin.Context, keys ...string) string {
	for _, key := range keys {
		if value := strings.TrimSpace(c.Query(key)); value != "" {
			return value
		}
	}
	return ""
}

func rosterStatusValue(c *gin.Context) string {
	status := strings.ToLower(firstQuery(c, "status", "active"))
	switch status {
	case "true":
		return "active"
	case "false":
		return "inactive"
	default:
		return status
	}
}

func validateRosterStatus(c *gin.Context) bool {
	switch strings.ToLower(firstQuery(c, "status", "active")) {
	case "", "true", "false", "active", "inactive":
		return true
	default:
		response.Error(c, appErrors.Clone(appErrors.ErrValidation, "status must be active or inactive"))
		return false
	}
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
// @Security BearerAuth
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
// @Security BearerAuth
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
// @Security BearerAuth
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
// @Security BearerAuth
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
// @Security BearerAuth
// @Router /students/{id} [delete]
func (h *StudentHandler) Delete(c *gin.Context) {
	if err := h.students.Deactivate(c.Request.Context(), c.Param("id")); err != nil {
		response.Error(c, err)
		return
	}
	response.NoContent(c)
}
