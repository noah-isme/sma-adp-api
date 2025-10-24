package handler

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/noah-isme/sma-adp-api/internal/middleware"
	"github.com/noah-isme/sma-adp-api/internal/models"
	"github.com/noah-isme/sma-adp-api/internal/service"
	appErrors "github.com/noah-isme/sma-adp-api/pkg/errors"
	"github.com/noah-isme/sma-adp-api/pkg/response"
)

// UserHandler handles user CRUD endpoints.
type UserHandler struct {
	service *service.UserService
}

// NewUserHandler creates a new user handler.
func NewUserHandler(svc *service.UserService) *UserHandler {
	return &UserHandler{service: svc}
}

// List godoc
// @Summary List users
// @Description List users with pagination and filtering
// @Tags Users
// @Produce json
// @Param page query int false "Page number"
// @Param page_size query int false "Page size"
// @Param role query string false "Role filter"
// @Param active query bool false "Active filter"
// @Param search query string false "Search term"
// @Param sort_by query string false "Sort by"
// @Param sort_order query string false "Sort order"
// @Success 200 {object} response.Envelope
// @Failure 403 {object} response.Envelope
// @Router /users [get]
func (h *UserHandler) List(c *gin.Context) {
	var filter models.UserFilter

	if page, err := strconv.Atoi(c.DefaultQuery("page", "1")); err == nil {
		filter.Page = page
	}
	if size, err := strconv.Atoi(c.DefaultQuery("page_size", "20")); err == nil {
		filter.PageSize = size
	}

	if role := c.Query("role"); role != "" {
		r := models.UserRole(role)
		filter.Role = &r
	}

	if active := c.Query("active"); active != "" {
		if val, err := strconv.ParseBool(active); err == nil {
			filter.Active = &val
		}
	}

	filter.Search = c.Query("search")
	filter.SortBy = c.Query("sort_by")
	filter.SortOrder = c.Query("sort_order")

	users, pagination, err := h.service.List(c.Request.Context(), filter)
	if err != nil {
		response.Error(c, err)
		return
	}

	response.JSON(c, http.StatusOK, users, pagination)
}

// Get godoc
// @Summary Get user
// @Description Get user detail
// @Tags Users
// @Produce json
// @Param id path string true "User ID"
// @Success 200 {object} response.Envelope
// @Failure 404 {object} response.Envelope
// @Router /users/{id} [get]
func (h *UserHandler) Get(c *gin.Context) {
	id := c.Param("id")

	user, err := h.service.Get(c.Request.Context(), id)
	if err != nil {
		response.Error(c, err)
		return
	}

	response.JSON(c, http.StatusOK, user, nil)
}

// Create godoc
// @Summary Create user
// @Description Create a new user
// @Tags Users
// @Accept json
// @Produce json
// @Param payload body service.CreateUserRequest true "Create user payload"
// @Success 201 {object} response.Envelope
// @Failure 400 {object} response.Envelope
// @Router /users [post]
func (h *UserHandler) Create(c *gin.Context) {
	claims, ok := c.Get(middleware.ContextUserKey)
	if !ok {
		response.Error(c, appErrors.ErrUnauthorized)
		return
	}
	jwtClaims := claims.(*models.JWTClaims)

	var req service.CreateUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, appErrors.Wrap(err, appErrors.ErrValidation.Code, http.StatusBadRequest, "invalid payload"))
		return
	}

	meta := models.LoginRequest{IP: c.ClientIP(), UserAgent: c.GetHeader("User-Agent")}
	user, err := h.service.Create(c.Request.Context(), req, jwtClaims.UserID, meta)
	if err != nil {
		response.Error(c, err)
		return
	}

	response.Created(c, user)
}

// Update godoc
// @Summary Update user
// @Description Update user details
// @Tags Users
// @Accept json
// @Produce json
// @Param id path string true "User ID"
// @Param payload body service.UpdateUserRequest true "Update payload"
// @Success 200 {object} response.Envelope
// @Failure 400 {object} response.Envelope
// @Router /users/{id} [put]
func (h *UserHandler) Update(c *gin.Context) {
	claims, ok := c.Get(middleware.ContextUserKey)
	if !ok {
		response.Error(c, appErrors.ErrUnauthorized)
		return
	}
	jwtClaims := claims.(*models.JWTClaims)

	var req service.UpdateUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, appErrors.Wrap(err, appErrors.ErrValidation.Code, http.StatusBadRequest, "invalid payload"))
		return
	}

	meta := models.LoginRequest{IP: c.ClientIP(), UserAgent: c.GetHeader("User-Agent")}
	user, err := h.service.Update(c.Request.Context(), c.Param("id"), req, jwtClaims.UserID, meta)
	if err != nil {
		response.Error(c, err)
		return
	}

	response.JSON(c, http.StatusOK, user, nil)
}

// Delete godoc
// @Summary Delete user
// @Description Soft delete user by marking inactive
// @Tags Users
// @Produce json
// @Param id path string true "User ID"
// @Success 204 {object} response.Envelope
// @Failure 404 {object} response.Envelope
// @Router /users/{id} [delete]
func (h *UserHandler) Delete(c *gin.Context) {
	claims, ok := c.Get(middleware.ContextUserKey)
	if !ok {
		response.Error(c, appErrors.ErrUnauthorized)
		return
	}
	jwtClaims := claims.(*models.JWTClaims)

	meta := models.LoginRequest{IP: c.ClientIP(), UserAgent: c.GetHeader("User-Agent")}
	if err := h.service.Delete(c.Request.Context(), c.Param("id"), jwtClaims.UserID, meta); err != nil {
		response.Error(c, err)
		return
	}

	response.NoContent(c)
}
