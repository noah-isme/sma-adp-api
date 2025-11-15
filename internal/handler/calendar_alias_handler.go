package handler

import (
	"context"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"github.com/noah-isme/sma-adp-api/internal/dto"
	"github.com/noah-isme/sma-adp-api/internal/models"
	appErrors "github.com/noah-isme/sma-adp-api/pkg/errors"
	"github.com/noah-isme/sma-adp-api/pkg/response"
)

type calendarAliasService interface {
	List(ctx context.Context, req dto.CalendarAliasRequest, claims *models.JWTClaims) (*dto.CalendarAliasResponse, error)
}

// CalendarAliasHandler exposes the /calendar alias endpoint.
type CalendarAliasHandler struct {
	service calendarAliasService
	logger  *zap.Logger
}

// NewCalendarAliasHandler constructs the handler.
func NewCalendarAliasHandler(service calendarAliasService, logger *zap.Logger) *CalendarAliasHandler {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &CalendarAliasHandler{service: service, logger: logger}
}

// List godoc
// @Summary Calendar alias endpoint (canonical)
// @Description Preferred FE endpoint that returns curated calendar events scoped by term/class.
// @Tags Academics
// @Produce json
// @Param term_id query string false "Term ID" example(2024_1)
// @Param class_id query string false "Class ID" example(10A)
// @Param start_date query string false "Start date (YYYY-MM-DD)" example(2024-01-01)
// @Param end_date query string false "End date (YYYY-MM-DD)" example(2024-01-31)
// @Success 200 {object} response.Envelope{data=dto.CalendarAliasResponse}
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

	if req.StartDate != nil && req.EndDate != nil && req.StartDate.After(*req.EndDate) {
		response.Error(c, appErrors.Clone(appErrors.ErrValidation, "start_date cannot be after end_date"))
		return
	}

	result, err := h.service.List(c.Request.Context(), req, claims)
	if err != nil {
		response.Error(c, err)
		return
	}
	h.logger.Info("calendar_alias",
		zap.String("term_id", pickOrEmpty(result.TermID)),
		zap.String("class_id", pickOrEmpty(result.ClassID)),
		zap.String("start_date", result.Range.StartDate),
		zap.String("end_date", result.Range.EndDate),
		zap.String("user_id", claims.UserID),
		zap.String("role", string(claims.Role)),
	)
	response.JSON(c, http.StatusOK, result, nil)
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

func pickOrEmpty(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}
