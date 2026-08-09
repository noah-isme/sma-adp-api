package middleware

import (
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/noah-isme/sma-adp-api/internal/models"
	"github.com/noah-isme/sma-adp-api/internal/service"
	appErrors "github.com/noah-isme/sma-adp-api/pkg/errors"
	"github.com/noah-isme/sma-adp-api/pkg/response"
)

// PortalContextUserKey is the gin context key storing portal JWT claims.
const PortalContextUserKey = "portalUser"

// PortalJWT protects routes by requiring a valid portal access token for PARENT/STUDENT roles.
func PortalJWT(authService *service.PortalAuthService) gin.HandlerFunc {
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

		// Verify portal role (ORTU or SISWA)
		if claims.Role != models.RoleOrtu && claims.Role != models.RoleSiswa {
			response.Error(c, appErrors.Clone(appErrors.ErrForbidden, "portal access restricted to parents and students"))
			c.Abort()
			return
		}

		c.Set(PortalContextUserKey, claims)
		c.Next()
	}
}

// OptionalPortalJWT attaches portal claims when present but does not block.
func OptionalPortalJWT(authService *service.PortalAuthService) gin.HandlerFunc {
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

		// Verify portal role (ORTU or SISWA)
		if claims.Role != models.RoleOrtu && claims.Role != models.RoleSiswa {
			c.Next()
			return
		}

		c.Set(PortalContextUserKey, claims)
		c.Next()
	}
}

// GetPortalUser retrieves the portal user claims from the context.
func GetPortalUser(c *gin.Context) (*models.JWTClaims, bool) {
	val, exists := c.Get(PortalContextUserKey)
	if !exists {
		return nil, false
	}
	claims, ok := val.(*models.JWTClaims)
	return claims, ok
}
