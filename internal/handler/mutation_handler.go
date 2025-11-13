package handler

import (
	"context"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/noah-isme/sma-adp-api/internal/dto"
	"github.com/noah-isme/sma-adp-api/internal/models"
	appErrors "github.com/noah-isme/sma-adp-api/pkg/errors"
	"github.com/noah-isme/sma-adp-api/pkg/response"
)

type mutationService interface {
	RequestChange(ctx context.Context, req dto.CreateMutationRequest, userID string) (*models.Mutation, error)
	List(ctx context.Context, query dto.MutationQuery, actor *models.JWTClaims) ([]models.Mutation, error)
	Get(ctx context.Context, id string, actor *models.JWTClaims) (*models.Mutation, error)
	Review(ctx context.Context, id string, req dto.ReviewMutationRequest, reviewerID string) (*models.Mutation, error)
}

// MutationHandler exposes REST endpoints for mutation workflows.
type MutationHandler struct {
	service mutationService
}

// NewMutationHandler constructs the handler.
func NewMutationHandler(service mutationService) *MutationHandler {
	return &MutationHandler{service: service}
}

// Create godoc
// @Summary Submit a mutation request
// @Tags Mutations
// @Accept json
// @Produce json
// @Param payload body dto.CreateMutationRequest true "Mutation payload"
// @Success 201 {object} response.Envelope
// @Router /mutations [post]
func (h *MutationHandler) Create(c *gin.Context) {
	if h.service == nil {
		response.Error(c, appErrors.Clone(appErrors.ErrInternal, "mutation service not configured"))
		return
	}
	var req dto.CreateMutationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, appErrors.Clone(appErrors.ErrValidation, "invalid mutation payload"))
		return
	}
	claims := claimsFromContext(c)
	if claims == nil {
		response.Error(c, appErrors.ErrUnauthorized)
		return
	}
	mutation, err := h.service.RequestChange(c.Request.Context(), req, claims.UserID)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.JSON(c, http.StatusCreated, mutation, nil)
}

// List godoc
// @Summary List mutation requests
// @Tags Mutations
// @Produce json
// @Param status query string false "Comma separated statuses"
// @Param entity query string false "Entity name"
// @Param type query string false "Mutation type"
// @Success 200 {object} response.Envelope
// @Router /mutations [get]
func (h *MutationHandler) List(c *gin.Context) {
	if h.service == nil {
		response.Error(c, appErrors.Clone(appErrors.ErrInternal, "mutation service not configured"))
		return
	}
	claims := claimsFromContext(c)
	if claims == nil {
		response.Error(c, appErrors.ErrUnauthorized)
		return
	}
	query := dto.MutationQuery{
		Entity: strings.TrimSpace(c.Query("entity")),
	}
	if rawType := c.Query("type"); rawType != "" {
		query.Type = models.MutationType(strings.ToUpper(rawType))
	}
	if rawStatus := c.Query("status"); rawStatus != "" {
		parts := strings.Split(rawStatus, ",")
		statuses := make([]models.MutationStatus, 0, len(parts))
		for _, part := range parts {
			part = strings.ToUpper(strings.TrimSpace(part))
			if part == "" {
				continue
			}
			statuses = append(statuses, models.MutationStatus(part))
		}
		query.Status = statuses
	}
	mutations, err := h.service.List(c.Request.Context(), query, claims)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.JSON(c, http.StatusOK, mutations, nil)
}

// Get godoc
// @Summary Get mutation detail
// @Tags Mutations
// @Produce json
// @Param id path string true "Mutation ID"
// @Success 200 {object} response.Envelope
// @Router /mutations/{id} [get]
func (h *MutationHandler) Get(c *gin.Context) {
	if h.service == nil {
		response.Error(c, appErrors.Clone(appErrors.ErrInternal, "mutation service not configured"))
		return
	}
	claims := claimsFromContext(c)
	if claims == nil {
		response.Error(c, appErrors.ErrUnauthorized)
		return
	}
	mutation, err := h.service.Get(c.Request.Context(), c.Param("id"), claims)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.JSON(c, http.StatusOK, mutation, nil)
}

// Review godoc
// @Summary Review a mutation request
// @Tags Mutations
// @Accept json
// @Produce json
// @Param id path string true "Mutation ID"
// @Param payload body dto.ReviewMutationRequest true "Review decision"
// @Success 200 {object} response.Envelope
// @Router /mutations/{id}/review [post]
func (h *MutationHandler) Review(c *gin.Context) {
	if h.service == nil {
		response.Error(c, appErrors.Clone(appErrors.ErrInternal, "mutation service not configured"))
		return
	}
	claims := claimsFromContext(c)
	if claims == nil {
		response.Error(c, appErrors.ErrUnauthorized)
		return
	}
	var req dto.ReviewMutationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, appErrors.Clone(appErrors.ErrValidation, "invalid review payload"))
		return
	}
	mutation, err := h.service.Review(c.Request.Context(), c.Param("id"), req, claims.UserID)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.JSON(c, http.StatusOK, mutation, nil)
}
