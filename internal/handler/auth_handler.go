package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/noah-isme/sma-adp-api/internal/middleware"
	"github.com/noah-isme/sma-adp-api/internal/models"
	"github.com/noah-isme/sma-adp-api/internal/service"
	appErrors "github.com/noah-isme/sma-adp-api/pkg/errors"
	"github.com/noah-isme/sma-adp-api/pkg/response"
)

// AuthHandler wires HTTP endpoints to the auth service.
type AuthHandler struct {
	service *service.AuthService
}

// NewAuthHandler creates a new handler.
func NewAuthHandler(svc *service.AuthService) *AuthHandler {
	return &AuthHandler{service: svc}
}

// Login godoc
// @Summary Authenticate user
// @Description Authenticate user by email and password
// @Tags Authentication
// @Accept json
// @Produce json
// @Param payload body models.LoginRequest true "Login payload"
// @Success 200 {object} response.Envelope
// @Failure 400 {object} response.Envelope
// @Failure 401 {object} response.Envelope
// @Router /auth/login [post]
func (h *AuthHandler) Login(c *gin.Context) {
	var req models.LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, appErrors.Wrap(err, appErrors.ErrValidation.Code, http.StatusBadRequest, "invalid login payload"))
		return
	}
	req.IP = c.ClientIP()
	req.UserAgent = c.GetHeader("User-Agent")

	res, err := h.service.Login(c.Request.Context(), req)
	if err != nil {
		response.Error(c, err)
		return
	}

	response.JSON(c, http.StatusOK, res, nil)
}

// Refresh godoc
// @Summary Refresh access token
// @Description Exchange refresh token for new access token
// @Tags Authentication
// @Accept json
// @Produce json
// @Param payload body models.RefreshTokenRequest true "Refresh payload"
// @Success 200 {object} response.Envelope
// @Failure 400 {object} response.Envelope
// @Failure 401 {object} response.Envelope
// @Router /auth/refresh [post]
func (h *AuthHandler) Refresh(c *gin.Context) {
	var req models.RefreshTokenRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, appErrors.Wrap(err, appErrors.ErrValidation.Code, http.StatusBadRequest, "invalid refresh payload"))
		return
	}
	req.IP = c.ClientIP()
	req.UserAgent = c.GetHeader("User-Agent")

	res, err := h.service.RefreshToken(c.Request.Context(), req)
	if err != nil {
		response.Error(c, err)
		return
	}

	response.JSON(c, http.StatusOK, res, nil)
}

// Logout godoc
// @Summary Logout current session
// @Description Revoke refresh token
// @Tags Authentication
// @Accept json
// @Produce json
// @Param payload body map[string]string true "Refresh token"
// @Success 204 {object} response.Envelope
// @Failure 401 {object} response.Envelope
// @Router /auth/logout [post]
func (h *AuthHandler) Logout(c *gin.Context) {
	claims, ok := c.Get(middleware.ContextUserKey)
	if !ok {
		response.Error(c, appErrors.ErrUnauthorized)
		return
	}
	jwtClaims := claims.(*models.JWTClaims)

	var payload struct {
		RefreshToken string `json:"refresh_token" binding:"required"`
	}
	if err := c.ShouldBindJSON(&payload); err != nil {
		response.Error(c, appErrors.Wrap(err, appErrors.ErrValidation.Code, http.StatusBadRequest, "refresh token required"))
		return
	}

	meta := models.LoginRequest{IP: c.ClientIP(), UserAgent: c.GetHeader("User-Agent")}
	if err := h.service.Logout(c.Request.Context(), payload.RefreshToken, jwtClaims.UserID, meta); err != nil {
		response.Error(c, err)
		return
	}

	response.NoContent(c)
}

// ChangePassword godoc
// @Summary Change password
// @Description Change password for current user
// @Tags Authentication
// @Accept json
// @Produce json
// @Param payload body models.ChangePasswordRequest true "Change password"
// @Success 204 {object} response.Envelope
// @Failure 400 {object} response.Envelope
// @Failure 401 {object} response.Envelope
// @Router /auth/change-password [post]
func (h *AuthHandler) ChangePassword(c *gin.Context) {
	claims, ok := c.Get(middleware.ContextUserKey)
	if !ok {
		response.Error(c, appErrors.ErrUnauthorized)
		return
	}
	jwtClaims := claims.(*models.JWTClaims)

	var req models.ChangePasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, appErrors.Wrap(err, appErrors.ErrValidation.Code, http.StatusBadRequest, "invalid payload"))
		return
	}

	if err := h.service.ChangePassword(c.Request.Context(), jwtClaims.UserID, req); err != nil {
		response.Error(c, err)
		return
	}

	response.NoContent(c)
}

// ForgotPassword godoc
// @Summary Forgot password
// @Description Initiate forgot password flow
// @Tags Authentication
// @Accept json
// @Produce json
// @Param payload body models.ResetPasswordRequest true "Forgot password"
// @Success 202 {object} response.Envelope
// @Failure 400 {object} response.Envelope
// @Router /auth/forgot-password [post]
func (h *AuthHandler) ForgotPassword(c *gin.Context) {
	var req models.ResetPasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, appErrors.Wrap(err, appErrors.ErrValidation.Code, http.StatusBadRequest, "invalid payload"))
		return
	}

	if err := h.service.ForgotPassword(c.Request.Context(), req); err != nil {
		response.Error(c, err)
		return
	}

	response.JSON(c, http.StatusAccepted, gin.H{"message": "if the email exists, a reset link will be sent"}, nil)
}

// ResetPassword godoc
// @Summary Reset password
// @Description Reset password with token
// @Tags Authentication
// @Accept json
// @Produce json
// @Param payload body models.ConfirmResetPasswordRequest true "Reset password"
// @Success 204 {object} response.Envelope
// @Failure 400 {object} response.Envelope
// @Router /auth/reset-password [post]
func (h *AuthHandler) ResetPassword(c *gin.Context) {
	var req models.ConfirmResetPasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, appErrors.Wrap(err, appErrors.ErrValidation.Code, http.StatusBadRequest, "invalid payload"))
		return
	}

	if err := h.service.ResetPassword(c.Request.Context(), req); err != nil {
		response.Error(c, err)
		return
	}

	response.NoContent(c)
}

// Me godoc
// @Summary Get current user
// @Description Returns the authenticated user's info
// @Tags Authentication
// @Produce json
// @Success 200 {object} response.Envelope
// @Failure 401 {object} response.Envelope
// @Router /auth/me [get]
func (h *AuthHandler) Me(c *gin.Context) {
	claims, ok := c.Get(middleware.ContextUserKey)
	if !ok {
		response.Error(c, appErrors.ErrUnauthorized)
		return
	}

	jwtClaims := claims.(*models.JWTClaims)
	info := models.UserInfo{
		ID:       jwtClaims.UserID,
		Email:    jwtClaims.Email,
		FullName: jwtClaims.FullName,
		Role:     jwtClaims.Role,
	}

	response.JSON(c, http.StatusOK, info, nil)
}
