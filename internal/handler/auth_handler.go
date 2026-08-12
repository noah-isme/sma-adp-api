package handler

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/noah-isme/sma-adp-api/internal/middleware"
	"github.com/noah-isme/sma-adp-api/internal/models"
	"github.com/noah-isme/sma-adp-api/internal/service"
	appErrors "github.com/noah-isme/sma-adp-api/pkg/errors"
	"github.com/noah-isme/sma-adp-api/pkg/response"
)

const (
	// refreshCookieName is deliberately not prefixed with __Host- because the
	// cookie is scoped to the authentication endpoints rather than the whole
	// host. Omitting Domain keeps it host-only.
	refreshCookieName = "refresh_token"
	refreshCookiePath = "/api/v1/auth"
)

// loginHTTPResponse and refreshHTTPResponse intentionally do not expose the
// refresh token. The service still returns it so that its use cases remain
// compatible with existing callers and unit tests; only this browser-facing
// HTTP boundary moves the value into an HttpOnly cookie.
type loginHTTPResponse struct {
	AccessToken string          `json:"access_token"`
	ExpiresIn   int64           `json:"expires_in"`
	User        models.UserInfo `json:"user"`
	IssuedAt    time.Time       `json:"issued_at"`
}

type refreshHTTPResponse struct {
	AccessToken string    `json:"access_token"`
	ExpiresIn   int64     `json:"expires_in"`
	IssuedAt    time.Time `json:"issued_at"`
}

func setRefreshCookie(c *gin.Context, token string, expiry time.Duration) {
	cookie := &http.Cookie{
		Name:     refreshCookieName,
		Value:    token,
		Path:     refreshCookiePath,
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteStrictMode,
	}
	if expiry > 0 {
		cookie.MaxAge = int(expiry / time.Second)
		cookie.Expires = time.Now().UTC().Add(expiry)
	}
	http.SetCookie(c.Writer, cookie)
}

func clearRefreshCookie(c *gin.Context) {
	http.SetCookie(c.Writer, &http.Cookie{
		Name:     refreshCookieName,
		Value:    "",
		Path:     refreshCookiePath,
		MaxAge:   -1,
		Expires:  time.Unix(1, 0).UTC(),
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteStrictMode,
	})
}

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

	setRefreshCookie(c, res.RefreshToken, h.service.RefreshTokenExpiry())
	response.JSON(c, http.StatusOK, loginHTTPResponse{
		AccessToken: res.AccessToken,
		ExpiresIn:   res.ExpiresIn,
		User:        res.User,
		IssuedAt:    res.IssuedAt,
	}, nil)
}

// Refresh godoc
// @Summary Refresh access token
// @Description Exchange refresh token for new access token
// @Tags Authentication
// @Produce json
// @Success 200 {object} response.Envelope
// @Failure 400 {object} response.Envelope
// @Failure 401 {object} response.Envelope
// @Router /auth/refresh [post]
func (h *AuthHandler) Refresh(c *gin.Context) {
	refreshToken, err := c.Cookie(refreshCookieName)
	if err != nil || refreshToken == "" {
		response.Error(c, appErrors.Clone(appErrors.ErrUnauthorized, "refresh token cookie required"))
		return
	}

	req := models.RefreshTokenRequest{RefreshToken: refreshToken}
	req.IP = c.ClientIP()
	req.UserAgent = c.GetHeader("User-Agent")

	res, err := h.service.RefreshToken(c.Request.Context(), req)
	if err != nil {
		response.Error(c, err)
		return
	}

	setRefreshCookie(c, res.RefreshToken, h.service.RefreshTokenExpiry())
	response.JSON(c, http.StatusOK, refreshHTTPResponse{
		AccessToken: res.AccessToken,
		ExpiresIn:   res.ExpiresIn,
		IssuedAt:    res.IssuedAt,
	}, nil)
}

// Logout godoc
// @Summary Logout current session
// @Description Revoke refresh token
// @Tags Authentication
// @Produce json
// @Success 204 {object} response.Envelope
// @Router /auth/logout [post]
func (h *AuthHandler) Logout(c *gin.Context) {
	refreshToken, _ := c.Cookie(refreshCookieName)
	// Clear the browser cookie even when the persisted token is already
	// expired/revoked or the request is missing authentication claims.
	clearRefreshCookie(c)

	if refreshToken == "" {
		// Logout is intentionally idempotent from the browser's perspective:
		// there is nothing left to revoke when the cookie is absent.
		response.NoContent(c)
		return
	}

	meta := models.LoginRequest{IP: c.ClientIP(), UserAgent: c.GetHeader("User-Agent")}
	if claims, ok := c.Get(middleware.ContextUserKey); ok {
		jwtClaims, validClaims := claims.(*models.JWTClaims)
		if !validClaims || jwtClaims == nil {
			response.Error(c, appErrors.ErrUnauthorized)
			return
		}
		if err := h.service.Logout(c.Request.Context(), refreshToken, jwtClaims.UserID, meta); err != nil {
			response.Error(c, err)
			return
		}
		response.NoContent(c)
		return
	}

	if err := h.service.LogoutByRefreshToken(c.Request.Context(), refreshToken, meta); err != nil {
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
// @Security BearerAuth
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

	response.JSON(c, http.StatusAccepted, gin.H{"message": "Jika email terdaftar, tautan reset akan dikirim"}, nil)
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
// @Security BearerAuth
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
