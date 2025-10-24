package middleware

import (
	"github.com/gin-gonic/gin"

	"github.com/noah-isme/sma-adp-api/internal/models"
	appErrors "github.com/noah-isme/sma-adp-api/pkg/errors"
	"github.com/noah-isme/sma-adp-api/pkg/response"
)

// RBAC enforces role-based access control for routes.
func RBAC(allowed ...string) gin.HandlerFunc {
	return func(c *gin.Context) {
		claimsValue, exists := c.Get(ContextUserKey)
		if !exists {
			response.Error(c, appErrors.ErrUnauthorized)
			c.Abort()
			return
		}
		claims := claimsValue.(*models.JWTClaims)

		allowSelf := false
		allowedRoles := make(map[models.UserRole]struct{})

		for _, a := range allowed {
			if a == "SELF" {
				allowSelf = true
				continue
			}
			allowedRoles[models.UserRole(a)] = struct{}{}
		}

		if _, ok := allowedRoles[claims.Role]; ok {
			c.Next()
			return
		}

		if allowSelf {
			if targetID := c.Param("id"); targetID != "" && targetID == claims.UserID {
				c.Next()
				return
			}
		}

		response.Error(c, appErrors.ErrForbidden)
		c.Abort()
	}
}

// RequireRoles is a helper that accepts a list of roles.
func RequireRoles(roles ...models.UserRole) gin.HandlerFunc {
	allowed := make([]string, len(roles))
	for i, r := range roles {
		allowed[i] = string(r)
	}
	return RBAC(allowed...)
}
