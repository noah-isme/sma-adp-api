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

// PortalAuthHandler wires HTTP endpoints to the portal auth service.
type PortalAuthHandler struct {
	service *service.PortalAuthService
}

// NewPortalAuthHandler creates a new handler.
func NewPortalAuthHandler(svc *service.PortalAuthService) *PortalAuthHandler {
	return &PortalAuthHandler{service: svc}
}

// PortalLogin godoc
// @Summary Authenticate parent or student
// @Description Authenticate parent or student by email and password
// @Tags Portal Authentication
// @Accept json
// @Produce json
// @Param payload body models.PortalLoginRequest true "Login payload"
// @Success 200 {object} response.Envelope{data=models.PortalLoginResponse}
// @Failure 400 {object} response.Envelope
// @Failure 401 {object} response.Envelope
// @Router /portal/auth/login [post]
func (h *PortalAuthHandler) PortalLogin(c *gin.Context) {
	var req models.PortalLoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, appErrors.Wrap(err, appErrors.ErrValidation.Code, http.StatusBadRequest, "invalid login payload"))
		return
	}
	req.IP = c.ClientIP()
	req.UserAgent = c.GetHeader("User-Agent")

	res, err := h.service.PortalLogin(c.Request.Context(), req)
	if err != nil {
		response.Error(c, err)
		return
	}

	response.JSON(c, http.StatusOK, res, nil)
}

// PortalRefresh godoc
// @Summary Refresh portal access token
// @Description Exchange refresh token for new access token
// @Tags Portal Authentication
// @Accept json
// @Produce json
// @Param payload body models.RefreshTokenRequest true "Refresh payload"
// @Success 200 {object} response.Envelope{data=models.PortalLoginResponse}
// @Failure 400 {object} response.Envelope
// @Failure 401 {object} response.Envelope
// @Router /portal/auth/refresh [post]
func (h *PortalAuthHandler) PortalRefresh(c *gin.Context) {
	var req models.RefreshTokenRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, appErrors.Wrap(err, appErrors.ErrValidation.Code, http.StatusBadRequest, "invalid refresh payload"))
		return
	}
	req.IP = c.ClientIP()
	req.UserAgent = c.GetHeader("User-Agent")

	res, err := h.service.PortalRefreshToken(c.Request.Context(), req)
	if err != nil {
		response.Error(c, err)
		return
	}

	response.JSON(c, http.StatusOK, res, nil)
}

// PortalLogout godoc
// @Summary Logout portal session
// @Description Revoke refresh token
// @Tags Portal Authentication
// @Accept json
// @Produce json
// @Param payload body models.PortalLogoutRequest true "Refresh token"
// @Security BearerAuth
// @Success 204 {object} response.Envelope
// @Failure 401 {object} response.Envelope
// @Router /portal/auth/logout [post]
func (h *PortalAuthHandler) PortalLogout(c *gin.Context) {
	claims, ok := c.Get(middleware.PortalContextUserKey)
	if !ok {
		response.Error(c, appErrors.ErrUnauthorized)
		return
	}
	jwtClaims := claims.(*models.JWTClaims)

	var payload models.PortalLogoutRequest
	if err := c.ShouldBindJSON(&payload); err != nil {
		response.Error(c, appErrors.Wrap(err, appErrors.ErrValidation.Code, http.StatusBadRequest, "refresh token required"))
		return
	}

	meta := models.PortalLoginRequest{IP: c.ClientIP(), UserAgent: c.GetHeader("User-Agent")}
	if err := h.service.PortalLogout(c.Request.Context(), payload.RefreshToken, jwtClaims.UserID, meta); err != nil {
		response.Error(c, err)
		return
	}

	response.NoContent(c)
}

// PortalForgotPassword godoc
// @Summary Forgot password for portal
// @Description Initiate forgot password flow for parent/student
// @Tags Portal Authentication
// @Accept json
// @Produce json
// @Param payload body models.PortalForgotPasswordRequest true "Forgot password"
// @Success 202 {object} response.Envelope
// @Failure 400 {object} response.Envelope
// @Router /portal/auth/forgot-password [post]
func (h *PortalAuthHandler) PortalForgotPassword(c *gin.Context) {
	var req models.PortalForgotPasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, appErrors.Wrap(err, appErrors.ErrValidation.Code, http.StatusBadRequest, "invalid payload"))
		return
	}

	if err := h.service.PortalForgotPassword(c.Request.Context(), req); err != nil {
		response.Error(c, err)
		return
	}

	response.JSON(c, http.StatusAccepted, gin.H{"message": "if the email exists, a reset link will be sent"}, nil)
}

// PortalResetPassword godoc
// @Summary Reset password for portal
// @Description Reset password with token for parent/student
// @Tags Portal Authentication
// @Accept json
// @Produce json
// @Param payload body models.PortalResetPasswordRequest true "Reset password"
// @Success 204 {object} response.Envelope
// @Failure 400 {object} response.Envelope
// @Router /portal/auth/reset-password [post]
func (h *PortalAuthHandler) PortalResetPassword(c *gin.Context) {
	var req models.PortalResetPasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, appErrors.Wrap(err, appErrors.ErrValidation.Code, http.StatusBadRequest, "invalid payload"))
		return
	}

	if err := h.service.PortalResetPassword(c.Request.Context(), req); err != nil {
		response.Error(c, err)
		return
	}

	response.NoContent(c)
}

// PortalMe godoc
// @Summary Get current portal user
// @Description Returns the authenticated portal user's info
// @Tags Portal Authentication
// @Produce json
// @Security BearerAuth
// @Success 200 {object} response.Envelope{data=models.PortalUserInfo}
// @Failure 401 {object} response.Envelope
// @Router /portal/auth/me [get]
func (h *PortalAuthHandler) PortalMe(c *gin.Context) {
	claims, ok := c.Get(middleware.PortalContextUserKey)
	if !ok {
		response.Error(c, appErrors.ErrUnauthorized)
		return
	}

	jwtClaims := claims.(*models.JWTClaims)

	// Get full profile from service
	profile, err := h.service.GetPortalProfile(c.Request.Context(), jwtClaims.UserID)
	if err != nil {
		response.Error(c, err)
		return
	}

	response.JSON(c, http.StatusOK, profile.User, nil)
}

// PortalProfile godoc
// @Summary Get full portal profile
// @Description Returns the full portal profile including preferences and device tokens
// @Tags Portal Profile
// @Produce json
// @Success 200 {object} response.Envelope{data=models.PortalProfile}
// @Failure 401 {object} response.Envelope
// @Router /portal/profile [get]
func (h *PortalAuthHandler) PortalProfile(c *gin.Context) {
	claims, ok := c.Get(middleware.PortalContextUserKey)
	if !ok {
		response.Error(c, appErrors.ErrUnauthorized)
		return
	}

	jwtClaims := claims.(*models.JWTClaims)

	profile, err := h.service.GetPortalProfile(c.Request.Context(), jwtClaims.UserID)
	if err != nil {
		response.Error(c, err)
		return
	}

	response.JSON(c, http.StatusOK, profile, nil)
}

// UpdatePortalPreferences godoc
// @Summary Update portal preferences
// @Description Update notification and display preferences
// @Tags Portal Profile
// @Accept json
// @Produce json
// @Param payload body models.UpdatePortalPreferencesRequest true "Preferences update"
// @Success 200 {object} response.Envelope{data=models.PortalPreferences}
// @Failure 400 {object} response.Envelope
// @Failure 401 {object} response.Envelope
// @Router /portal/profile/preferences [put]
func (h *PortalAuthHandler) UpdatePortalPreferences(c *gin.Context) {
	claims, ok := c.Get(middleware.PortalContextUserKey)
	if !ok {
		response.Error(c, appErrors.ErrUnauthorized)
		return
	}

	jwtClaims := claims.(*models.JWTClaims)

	var req models.UpdatePortalPreferencesRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, appErrors.Wrap(err, appErrors.ErrValidation.Code, http.StatusBadRequest, "invalid payload"))
		return
	}

	prefs, err := h.service.UpdatePortalPreferences(c.Request.Context(), jwtClaims.UserID, req)
	if err != nil {
		response.Error(c, err)
		return
	}

	response.JSON(c, http.StatusOK, prefs, nil)
}

// RegisterDeviceToken godoc
// @Summary Register device token
// @Description Register a device token for push notifications
// @Tags Portal Profile
// @Accept json
// @Produce json
// @Param payload body models.RegisterDeviceTokenRequest true "Device token"
// @Success 200 {object} response.Envelope{data=models.DeviceToken}
// @Failure 400 {object} response.Envelope
// @Failure 401 {object} response.Envelope
// @Router /portal/profile/device-tokens [post]
func (h *PortalAuthHandler) RegisterDeviceToken(c *gin.Context) {
	claims, ok := c.Get(middleware.PortalContextUserKey)
	if !ok {
		response.Error(c, appErrors.ErrUnauthorized)
		return
	}

	jwtClaims := claims.(*models.JWTClaims)

	var req models.RegisterDeviceTokenRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, appErrors.Wrap(err, appErrors.ErrValidation.Code, http.StatusBadRequest, "invalid payload"))
		return
	}

	token, err := h.service.RegisterDeviceToken(c.Request.Context(), jwtClaims.UserID, req)
	if err != nil {
		response.Error(c, err)
		return
	}

	response.JSON(c, http.StatusOK, token, nil)
}

// UnregisterDeviceToken godoc
// @Summary Unregister device token
// @Description Remove a device token
// @Tags Portal Profile
// @Accept json
// @Produce json
// @Param tokenId path string true "Device token ID"
// @Success 204 {object} response.Envelope
// @Failure 401 {object} response.Envelope
// @Router /portal/profile/device-tokens/{tokenId} [delete]
func (h *PortalAuthHandler) UnregisterDeviceToken(c *gin.Context) {
	claims, ok := c.Get(middleware.PortalContextUserKey)
	if !ok {
		response.Error(c, appErrors.ErrUnauthorized)
		return
	}

	jwtClaims := claims.(*models.JWTClaims)
	tokenID := c.Param("tokenId")

	if err := h.service.UnregisterDeviceToken(c.Request.Context(), jwtClaims.UserID, tokenID); err != nil {
		response.Error(c, err)
		return
	}

	response.NoContent(c)
}

// GetLinkedStudents godoc
// @Summary Get linked students for a parent
// @Description Returns all students linked to the authenticated parent
// @Tags Portal Parent-Student Links
// @Produce json
// @Success 200 {object} response.Envelope{data=[]models.ParentStudentLink}
// @Failure 401 {object} response.Envelope
// @Failure 403 {object} response.Envelope
// @Router /portal/parent/students [get]
func (h *PortalAuthHandler) GetLinkedStudents(c *gin.Context) {
	claims, ok := c.Get(middleware.PortalContextUserKey)
	if !ok {
		response.Error(c, appErrors.ErrUnauthorized)
		return
	}

	jwtClaims := claims.(*models.JWTClaims)

	links, err := h.service.GetLinkedStudents(c.Request.Context(), jwtClaims.UserID)
	if err != nil {
		response.Error(c, err)
		return
	}

	response.JSON(c, http.StatusOK, links, nil)
}

// CreateParentStudentLink godoc
// @Summary Create a parent-student link
// @Description Link a student to the authenticated parent
// @Tags Portal Parent-Student Links
// @Accept json
// @Produce json
// @Param payload body models.CreateParentStudentLinkRequest true "Link payload"
// @Success 201 {object} response.Envelope{data=models.ParentStudentLink}
// @Failure 400 {object} response.Envelope
// @Failure 401 {object} response.Envelope
// @Failure 403 {object} response.Envelope
// @Failure 409 {object} response.Envelope
// @Router /portal/parent/students [post]
func (h *PortalAuthHandler) CreateParentStudentLink(c *gin.Context) {
	claims, ok := c.Get(middleware.PortalContextUserKey)
	if !ok {
		response.Error(c, appErrors.ErrUnauthorized)
		return
	}

	jwtClaims := claims.(*models.JWTClaims)

	var req models.CreateParentStudentLinkRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, appErrors.Wrap(err, appErrors.ErrValidation.Code, http.StatusBadRequest, "invalid payload"))
		return
	}

	link, err := h.service.CreateParentStudentLink(c.Request.Context(), jwtClaims.UserID, req)
	if err != nil {
		response.Error(c, err)
		return
	}

	response.JSON(c, http.StatusCreated, link, nil)
}

// UpdateParentStudentLink godoc
// @Summary Update a parent-student link
// @Description Update permissions for a parent-student link
// @Tags Portal Parent-Student Links
// @Accept json
// @Produce json
// @Param linkId path string true "Link ID"
// @Param payload body models.UpdateParentStudentLinkRequest true "Update payload"
// @Success 200 {object} response.Envelope{data=models.ParentStudentLink}
// @Failure 400 {object} response.Envelope
// @Failure 401 {object} response.Envelope
// @Failure 403 {object} response.Envelope
// @Failure 404 {object} response.Envelope
// @Router /portal/parent/students/{linkId} [put]
func (h *PortalAuthHandler) UpdateParentStudentLink(c *gin.Context) {
	claims, ok := c.Get(middleware.PortalContextUserKey)
	if !ok {
		response.Error(c, appErrors.ErrUnauthorized)
		return
	}

	jwtClaims := claims.(*models.JWTClaims)
	linkID := c.Param("linkId")

	var req models.UpdateParentStudentLinkRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, appErrors.Wrap(err, appErrors.ErrValidation.Code, http.StatusBadRequest, "invalid payload"))
		return
	}

	link, err := h.service.UpdateParentStudentLink(c.Request.Context(), jwtClaims.UserID, linkID, req)
	if err != nil {
		response.Error(c, err)
		return
	}

	response.JSON(c, http.StatusOK, link, nil)
}

// DeleteParentStudentLink godoc
// @Summary Delete a parent-student link
// @Description Remove a parent-student link
// @Tags Portal Parent-Student Links
// @Produce json
// @Param linkId path string true "Link ID"
// @Success 204 {object} response.Envelope
// @Failure 401 {object} response.Envelope
// @Failure 403 {object} response.Envelope
// @Failure 404 {object} response.Envelope
// @Router /portal/parent/students/{linkId} [delete]
func (h *PortalAuthHandler) DeleteParentStudentLink(c *gin.Context) {
	claims, ok := c.Get(middleware.PortalContextUserKey)
	if !ok {
		response.Error(c, appErrors.ErrUnauthorized)
		return
	}

	jwtClaims := claims.(*models.JWTClaims)
	linkID := c.Param("linkId")

	if err := h.service.DeleteParentStudentLink(c.Request.Context(), jwtClaims.UserID, linkID); err != nil {
		response.Error(c, err)
		return
	}

	response.NoContent(c)
}
