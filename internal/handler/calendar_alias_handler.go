package handler

import (
	"context"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/noah-isme/sma-adp-api/internal/dto"
	"github.com/noah-isme/sma-adp-api/internal/models"
	appErrors "github.com/noah-isme/sma-adp-api/pkg/errors"
	"github.com/noah-isme/sma-adp-api/pkg/response"
)

type calendarAliasService interface {
	List(ctx context.Context, req dto.CalendarAliasRequest, claims *models.JWTClaims) ([]dto.CalendarAliasEvent, error)
}

// CalendarAliasHandler exposes the /calendar alias endpoint.
type CalendarAliasHandler struct {
	service calendarAliasService
}

// NewCalendarAliasHandler constructs the handler.
func NewCalendarAliasHandler(service calendarAliasService) *CalendarAliasHandler {
	return &CalendarAliasHandler{service: service}
}

// List godoc
// @Summary Calendar alias endpoint
// @Tags Calendar
// @Produce json
// @Param termId query string false "Term ID"
// @Param classId query string false "Class ID"
// @Param startDate query string false "Start date (YYYY-MM-DD)"
// @Param endDate query string false "End date (YYYY-MM-DD)"
// @Success 200 {object} response.Envelope
// @Router /calendar [get]
func (h *CalendarAliasHandler) List(c *gin.Context) {
	claims := claimsFromContext(c)
	if claims == nil {
		response.Error(c, appErrors.ErrUnauthorized)
		return
	}

	req := dto.CalendarAliasRequest{
		TermID:  c.Query("termId"),
		ClassID: c.Query("classId"),
	}
	if raw := c.Query("startDate"); raw != "" {
		parsed, err := time.Parse("2006-01-02", raw)
		if err != nil {
			response.Error(c, appErrors.Clone(appErrors.ErrValidation, "invalid startDate, expected YYYY-MM-DD"))
			return
		}
		req.StartDate = &parsed
	}
	if raw := c.Query("endDate"); raw != "" {
		parsed, err := time.Parse("2006-01-02", raw)
		if err != nil {
			response.Error(c, appErrors.Clone(appErrors.ErrValidation, "invalid endDate, expected YYYY-MM-DD"))
			return
		}
		req.EndDate = &parsed
	}

	events, err := h.service.List(c.Request.Context(), req, claims)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.JSON(c, http.StatusOK, events, nil)
}
