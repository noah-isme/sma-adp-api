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

// TeacherHandler wires teacher services to HTTP routes.
type TeacherHandler struct {
	teachers    *service.TeacherService
	assignments *service.TeacherAssignmentService
	prefs       *service.TeacherPreferenceService
	imports     importRunStore
}

// ImportCSV imports email, full_name, and optional nip, phone, expertise.
//
// @Summary Import teachers from CSV
// @Tags Teachers
// @Accept text/csv
// @Produce json
// @Param csv body string true "CSV document"
// @Param Idempotency-Key header string false "Stable key for safe retries"
// @Success 200 {object} response.Envelope
// @Router /teachers/import [post]
func (h *TeacherHandler) ImportCSV(c *gin.Context) {
	body, ok := readCSVImportBody(c)
	if !ok {
		return
	}
	columns, rows, err := parseCSVImport(body, []string{"email", "full_name"})
	if err != nil {
		response.Error(c, err)
		return
	}
	run, proceed := beginCSVImport(c, h.imports, "teachers", body)
	if !proceed {
		return
	}
	created, failures := 0, []gin.H{}
	for index, values := range rows {
		row := index + 2
		email, fullName := importOptionalValue(values, columns, "email"), importOptionalValue(values, columns, "full_name")
		if email == nil || fullName == nil {
			failures = append(failures, gin.H{"row": row, "error": "email and full_name required"})
			continue
		}
		if _, err = h.teachers.Create(c.Request.Context(), service.CreateTeacherRequest{Email: *email, FullName: *fullName, NIP: importOptionalValue(values, columns, "nip"), Phone: importOptionalValue(values, columns, "phone"), Expertise: importOptionalValue(values, columns, "expertise")}); err != nil {
			failures = append(failures, gin.H{"row": row, "error": err.Error()})
			continue
		}
		created++
	}
	completeCSVImport(c, h.imports, run, "teachers", created, failures)
}

// NewTeacherHandler constructs a new TeacherHandler.
func NewTeacherHandler(teachers *service.TeacherService, assignments *service.TeacherAssignmentService, prefs *service.TeacherPreferenceService, stores ...importRunStore) *TeacherHandler {
	var imports importRunStore
	if len(stores) > 0 {
		imports = stores[0]
	}
	return &TeacherHandler{
		teachers:    teachers,
		assignments: assignments,
		prefs:       prefs,
		imports:     imports,
	}
}

// List godoc
// @Summary List teachers
// @Tags Teachers
// @Produce json
// @Param search query string false "Search by name/email/NIP"
// @Param active query bool false "Filter by active status"
// @Param page query int false "Page number"
// @Param limit query int false "Page size"
// @Param sort query string false "Sort field (full_name,email,created_at)"
// @Param order query string false "Sort order (asc/desc)"
// @Success 200 {object} response.Envelope
// @Router /teachers [get]
func (h *TeacherHandler) List(c *gin.Context) {
	filter := teacherFilter(c)

	teachers, pagination, err := h.teachers.List(c.Request.Context(), filter)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.JSON(c, http.StatusOK, teachers, pagination)
}

// Roster preserves the existing admin response shape.
//
// @Summary List teacher roster
// @Tags Teachers
// @Produce json
// @Param search query string false "Search by name, email, or NIP"
// @Param active query bool false "Filter by active state"
// @Param subjectId query string false "Filter by subject"
// @Param track query string false "Filter by track"
// @Param availability query string false "Filter by availability"
// @Param homeroomClassId query string false "Filter by homeroom class"
// @Param page query int false "Page"
// @Param perPage query int false "Page size"
// @Param sort query string false "Sort field"
// @Param order query string false "Sort order"
// @Success 200 {object} response.Envelope
// @Router /teachers/roster [get]
func (h *TeacherHandler) Roster(c *gin.Context) {
	filter := teacherFilter(c)
	teachers, pagination, err := h.teachers.List(c.Request.Context(), filter)
	if err != nil {
		response.Error(c, err)
		return
	}
	rows := make([]gin.H, 0, len(teachers))
	activeCount := 0
	for _, teacher := range teachers {
		status := "inactive"
		if teacher.Active {
			status = "active"
			activeCount++
		}
		nip, phone := "", ""
		if teacher.NIP != nil {
			nip = *teacher.NIP
		}
		if teacher.Phone != nil {
			phone = *teacher.Phone
		}
		rows = append(rows, gin.H{"id": teacher.ID, "fullName": teacher.FullName, "nip": nip, "email": teacher.Email, "phone": phone, "status": status, "tracks": []string{}, "assignmentCount": 0, "availability": "MEDIUM", "lastUpdated": teacher.UpdatedAt, "createdAt": teacher.CreatedAt})
	}
	response.JSON(c, http.StatusOK, gin.H{"summary": gin.H{"totalTeachers": pagination.TotalCount, "activeTeachers": activeCount, "inactiveTeachers": len(teachers) - activeCount, "homeroomTeachers": 0, "activeRate": rosterPercent(activeCount, len(teachers)), "subjectDistribution": []gin.H{}, "trackDistribution": []gin.H{}, "availabilityBreakdown": []gin.H{}}, "filters": gin.H{"subjects": []gin.H{}, "statuses": []gin.H{}, "tracks": []gin.H{}, "availabilities": []gin.H{}, "homerooms": []gin.H{}}, "rows": rows, "pagination": gin.H{"page": pagination.Page, "perPage": pagination.PageSize, "total": pagination.TotalCount, "totalPages": teacherPageCount(pagination.TotalCount, pagination.PageSize)}, "appliedFilters": gin.H{"page": pagination.Page, "perPage": pagination.PageSize, "search": filter.Search, "active": filter.Active, "subjectId": filter.SubjectID, "track": filter.Track, "availability": filter.Availability, "homeroomClassId": filter.HomeroomClassID, "sort": filter.SortBy, "order": filter.SortOrder}}, nil)
}

func teacherFilter(c *gin.Context) models.TeacherFilter {
	filter := models.TeacherFilter{
		Search:         strings.TrimSpace(c.Query("search")),
		SubjectID:      c.Query("subjectId"),
		Track:          c.Query("track"),
		Availability:   c.Query("availability"),
		HomeroomClassID: c.Query("homeroomClassId"),
		SortBy:         c.Query("sort"),
		SortOrder:      c.Query("order"),
	}
	if active := c.Query("active"); active != "" {
		v := strings.EqualFold(active, "true")
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
func rosterPercent(n, total int) float64 {
	if total == 0 {
		return 0
	}
	return float64(n) * 100 / float64(total)
}

func teacherPageCount(total, size int) int {
	if size <= 0 {
		return 0
	}
	return (total + size - 1) / size
}

// Get godoc
// @Summary Get teacher detail
// @Tags Teachers
// @Produce json
// @Param id path string true "Teacher ID"
// @Success 200 {object} response.Envelope
// @Router /teachers/{id} [get]
func (h *TeacherHandler) Get(c *gin.Context) {
	teacher, err := h.teachers.Get(c.Request.Context(), c.Param("id"))
	if err != nil {
		response.Error(c, err)
		return
	}
	response.JSON(c, http.StatusOK, teacher, nil)
}

// Create godoc
// @Summary Create teacher
// @Tags Teachers
// @Accept json
// @Produce json
// @Param payload body service.CreateTeacherRequest true "Teacher payload"
// @Success 201 {object} response.Envelope
// @Router /teachers [post]
func (h *TeacherHandler) Create(c *gin.Context) {
	var req service.CreateTeacherRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, appErrors.Wrap(err, appErrors.ErrValidation.Code, http.StatusBadRequest, "invalid teacher payload"))
		return
	}
	teacher, err := h.teachers.Create(c.Request.Context(), req)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Created(c, teacher)
}

// Update godoc
// @Summary Update teacher
// @Tags Teachers
// @Accept json
// @Produce json
// @Param id path string true "Teacher ID"
// @Param payload body service.UpdateTeacherRequest true "Teacher payload"
// @Success 200 {object} response.Envelope
// @Router /teachers/{id} [put]
func (h *TeacherHandler) Update(c *gin.Context) {
	var req service.UpdateTeacherRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, appErrors.Wrap(err, appErrors.ErrValidation.Code, http.StatusBadRequest, "invalid teacher payload"))
		return
	}
	teacher, err := h.teachers.Update(c.Request.Context(), c.Param("id"), req)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.JSON(c, http.StatusOK, teacher, nil)
}

// UpdateStatus toggles a teacher's active state without requiring the full
// teacher update payload used by PUT /teachers/:id.
// @Summary Update teacher status
// @Tags Teachers
// @Accept json
// @Produce json
// @Param id path string true "Teacher ID"
// @Param payload body map[string]interface{} true "Status payload"
// @Success 200 {object} response.Envelope
// @Router /teachers/{id}/status [patch]
func (h *TeacherHandler) UpdateStatus(c *gin.Context) {
	var payload struct {
		Status string `json:"status"`
		Active *bool  `json:"active"`
	}
	if err := c.ShouldBindJSON(&payload); err != nil {
		response.Error(c, appErrors.Wrap(err, appErrors.ErrValidation.Code, http.StatusBadRequest, "invalid teacher status payload"))
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
	teacher, err := h.teachers.SetActive(c.Request.Context(), c.Param("id"), *active)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.JSON(c, http.StatusOK, teacher, nil)
}

// Delete godoc
// @Summary Deactivate teacher
// @Tags Teachers
// @Param id path string true "Teacher ID"
// @Success 204
// @Router /teachers/{id} [delete]
func (h *TeacherHandler) Delete(c *gin.Context) {
	if err := h.teachers.Deactivate(c.Request.Context(), c.Param("id")); err != nil {
		response.Error(c, err)
		return
	}
	response.NoContent(c)
}

// ListAssignments godoc
// @Summary List teacher assignments
// @Tags Teacher Assignments
// @Param id path string true "Teacher ID"
// @Produce json
// @Success 200 {object} response.Envelope
// @Router /teachers/{id}/assignments [get]
func (h *TeacherHandler) ListAssignments(c *gin.Context) {
	assignments, err := h.assignments.ListByTeacher(c.Request.Context(), c.Param("id"))
	if err != nil {
		response.Error(c, err)
		return
	}
	response.JSON(c, http.StatusOK, assignments, nil)
}

// CreateAssignment godoc
// @Summary Create teacher assignment
// @Tags Teacher Assignments
// @Accept json
// @Produce json
// @Param id path string true "Teacher ID"
// @Param payload body service.CreateTeacherAssignmentRequest true "Assignment payload"
// @Success 201 {object} response.Envelope
// @Router /teachers/{id}/assignments [post]
func (h *TeacherHandler) CreateAssignment(c *gin.Context) {
	var req service.CreateTeacherAssignmentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, appErrors.Wrap(err, appErrors.ErrValidation.Code, http.StatusBadRequest, "invalid assignment payload"))
		return
	}
	assignment, err := h.assignments.Assign(c.Request.Context(), c.Param("id"), req)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Created(c, assignment)
}

// DeleteAssignment godoc
// @Summary Delete teacher assignment
// @Tags Teacher Assignments
// @Param id path string true "Teacher ID"
// @Param aid path string true "Assignment ID"
// @Success 204
// @Router /teachers/{id}/assignments/{aid} [delete]
func (h *TeacherHandler) DeleteAssignment(c *gin.Context) {
	if err := h.assignments.Remove(c.Request.Context(), c.Param("id"), c.Param("aid")); err != nil {
		response.Error(c, err)
		return
	}
	response.NoContent(c)
}

// GetPreferences godoc
// @Summary Get teacher preferences
// @Tags Teacher Preferences
// @Param id path string true "Teacher ID"
// @Produce json
// @Success 200 {object} response.Envelope
// @Router /teachers/{id}/preferences [get]
func (h *TeacherHandler) GetPreferences(c *gin.Context) {
	pref, err := h.prefs.Get(c.Request.Context(), c.Param("id"))
	if err != nil {
		response.Error(c, err)
		return
	}
	response.JSON(c, http.StatusOK, pref, nil)
}

// UpsertPreferences godoc
// @Summary Upsert teacher preferences
// @Tags Teacher Preferences
// @Accept json
// @Produce json
// @Param id path string true "Teacher ID"
// @Param payload body service.UpsertTeacherPreferenceRequest true "Preference payload"
// @Success 200 {object} response.Envelope
// @Router /teachers/{id}/preferences [put]
func (h *TeacherHandler) UpsertPreferences(c *gin.Context) {
	var req service.UpsertTeacherPreferenceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, appErrors.Wrap(err, appErrors.ErrValidation.Code, http.StatusBadRequest, "invalid preference payload"))
		return
	}
	pref, err := h.prefs.Upsert(c.Request.Context(), c.Param("id"), req)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.JSON(c, http.StatusOK, pref, nil)
}
