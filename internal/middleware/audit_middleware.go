package middleware

import (
	"github.com/gin-gonic/gin"
	"github.com/noah-isme/sma-adp-api/internal/repository"
)

// AuditMiddleware is an alias wrapper for Audit middleware.
// It handles user authentication claims (JWTClaims / UserID / Anonymous) and records persistent audit logs.
func AuditMiddleware(repo *repository.UserRepository, action, resource string) gin.HandlerFunc {
	return Audit(repo, action, resource)
}
