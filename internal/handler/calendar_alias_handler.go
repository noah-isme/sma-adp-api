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
		TermID:  pickQuery(c, "term_id", "termId"),
		ClassID: pickQuery(c, "class_id", "classId"),
	}

	start, err := parseCalendarDate(pickQuery(c, "start_date", "startDate"))
	if err != nil {
		response.Error(c, err)
		return
	}
	end, err := parseCalendarDate(pickQuery(c, "end_date", "endDate"))
	if err != nil {
		response.Error(c, err)
		return
	}
	req.StartDate = start
	req.EndDate = end

	events, err := h.service.List(c.Request.Context(), req, claims)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.JSON(c, http.StatusOK, events, nil)
}

func parseCalendarDate(raw string) (*time.Time, error) {
	if raw == "" {
		return nil, nil
	}
	parsed, err := time.Parse("2006-01-02", raw)
	if err != nil {
		return nil, appErrors.Clone(appErrors.ErrValidation, "invalid date, expected YYYY-MM-DD")
	}
	return &parsed, nil
}

func pickQuery(c *gin.Context, preferred string, fallback string) string {
	if value := c.Query(preferred); value != "" {
		return value
	}
	return c.Query(fallback)
}
