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

// AnnouncementHandler exposes announcement CRUD endpoints.
type AnnouncementHandler struct {
	announcements *service.AnnouncementService
}

// NewAnnouncementHandler constructs AnnouncementHandler.
func NewAnnouncementHandler(announcements *service.AnnouncementService) *AnnouncementHandler {
	return &AnnouncementHandler{announcements: announcements}
}

// List godoc
// @Summary List announcements
// @Tags Announcements
// @Produce json
// @Param audience query string false "Comma-separated audience roles"
// @Param classIds query string false "Comma-separated class IDs"
// @Param includePinned query bool false "Include pinned announcements"
// @Param page query int false "Page"
// @Param limit query int false "Page size"
// @Success 200 {object} response.Envelope
// @Security BearerAuth
// @Router /announcements [get]
func (h *AnnouncementHandler) List(c *gin.Context) {
	req := service.AnnouncementListRequest{
		ClassIDs:      splitCSV(c.Query("classIds")),
		IncludePinned: parseBoolDefault(c.Query("includePinned"), false),
		Page:          parseQueryInt(c, "page", 1),
		PageSize:      parseQueryInt(c, "limit", 20),
	}
	for _, role := range splitCSV(c.Query("audience")) {
		req.AudienceRoles = append(req.AudienceRoles, models.UserRole(strings.ToUpper(role)))
	}

	rows, pagination, err := h.announcements.List(c.Request.Context(), req)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.JSON(c, http.StatusOK, rows, pagination)
}

// Get godoc
// @Summary Get announcement
// @Tags Announcements
// @Produce json
// @Param id path string true "Announcement ID"
// @Success 200 {object} response.Envelope
// @Security BearerAuth
// @Router /announcements/{id} [get]
func (h *AnnouncementHandler) Get(c *gin.Context) {
	announcement, err := h.announcements.Get(c.Request.Context(), c.Param("id"))
	if err != nil {
		response.Error(c, err)
		return
	}
	response.JSON(c, http.StatusOK, announcement, nil)
}

// Create godoc
// @Summary Create announcement
// @Tags Announcements
// @Accept json
// @Produce json
// @Param payload body service.CreateAnnouncementRequest true "Announcement payload"
// @Success 201 {object} response.Envelope
// @Security BearerAuth
// @Router /announcements [post]
func (h *AnnouncementHandler) Create(c *gin.Context) {
	claims := claimsFromContext(c)
	if claims == nil {
		response.Error(c, appErrors.ErrUnauthorized)
		return
	}
	var req service.CreateAnnouncementRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, appErrors.Wrap(err, appErrors.ErrValidation.Code, http.StatusBadRequest, "invalid announcement payload"))
		return
	}
	req.CreatedBy = claims.UserID

	announcement, err := h.announcements.Create(c.Request.Context(), req)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Created(c, announcement)
}

// Update godoc
// @Summary Update announcement
// @Tags Announcements
// @Accept json
// @Produce json
// @Param id path string true "Announcement ID"
// @Param payload body service.UpdateAnnouncementRequest true "Announcement payload"
// @Success 200 {object} response.Envelope
// @Security BearerAuth
// @Router /announcements/{id} [put]
func (h *AnnouncementHandler) Update(c *gin.Context) {
	var req service.UpdateAnnouncementRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, appErrors.Wrap(err, appErrors.ErrValidation.Code, http.StatusBadRequest, "invalid announcement payload"))
		return
	}
	announcement, err := h.announcements.Update(c.Request.Context(), c.Param("id"), req)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.JSON(c, http.StatusOK, announcement, nil)
}

// Delete godoc
// @Summary Delete announcement
// @Tags Announcements
// @Produce json
// @Param id path string true "Announcement ID"
// @Success 204
// @Security BearerAuth
// @Router /announcements/{id} [delete]
func (h *AnnouncementHandler) Delete(c *gin.Context) {
	if err := h.announcements.Delete(c.Request.Context(), c.Param("id")); err != nil {
		response.Error(c, err)
		return
	}
	response.NoContent(c)
}

func splitCSV(raw string) []string {
	if raw == "" {
		return nil
	}
	parts := strings.Split(raw, ",")
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}

func parseBoolDefault(raw string, fallback bool) bool {
	if raw == "" {
		return fallback
	}
	value, err := strconv.ParseBool(raw)
	if err != nil {
		return fallback
	}
	return value
}
