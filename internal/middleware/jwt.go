package middleware

import (
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/noah-isme/sma-adp-api/internal/service"
	appErrors "github.com/noah-isme/sma-adp-api/pkg/errors"
	"github.com/noah-isme/sma-adp-api/pkg/response"
)

// ContextUserKey is the gin context key storing JWT claims.
const ContextUserKey = "currentUser"

// JWT protects routes by requiring a valid access token.
func JWT(authService *service.AuthService) gin.HandlerFunc {
	return func(c *gin.Context) {
		header := c.GetHeader("Authorization")
		if header == "" {
			response.Error(c, appErrors.ErrUnauthorized)
			c.Abort()
			return
		}

		parts := strings.SplitN(header, " ", 2)
		if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
			response.Error(c, appErrors.Clone(appErrors.ErrUnauthorized, "invalid authorization header"))
			c.Abort()
			return
		}

		claims, err := authService.ValidateToken(parts[1])
		if err != nil {
			response.Error(c, err)
			c.Abort()
			return
		}

		c.Set(ContextUserKey, claims)
		c.Next()
	}
}

// OptionalJWT attaches claims when present but does not block.
func OptionalJWT(authService *service.AuthService) gin.HandlerFunc {
	return func(c *gin.Context) {
		header := c.GetHeader("Authorization")
		if header == "" {
			c.Next()
			return
		}

		parts := strings.SplitN(header, " ", 2)
		if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
			c.Next()
			return
		}

		claims, err := authService.ValidateToken(parts[1])
		if err != nil {
			c.Next()
			return
		}

		c.Set(ContextUserKey, claims)
		c.Next()
	}
}
