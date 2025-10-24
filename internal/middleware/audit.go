package middleware

import (
	"encoding/json"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/noah-isme/sma-adp-api/internal/models"
	"github.com/noah-isme/sma-adp-api/internal/repository"
)

// Audit creates a middleware that records audit logs after successful requests.
func Audit(repo *repository.UserRepository, action, resource string) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now().UTC()
		c.Next()

		if c.Writer.Status() >= 400 {
			return
		}

		var userID *string
		if claims, ok := c.Get(ContextUserKey); ok {
			user := claims.(*models.JWTClaims)
			userID = &user.UserID
		}

		body, _ := json.Marshal(map[string]interface{}{
			"path":    c.FullPath(),
			"method":  c.Request.Method,
			"status":  c.Writer.Status(),
			"latency": time.Since(start).Milliseconds(),
		})

		_ = repo.CreateAuditLog(c.Request.Context(), &models.AuditLog{
			UserID:     userID,
			Action:     action,
			Resource:   resource,
			ResourceID: nil,
			NewValues:  body,
			IPAddress:  c.ClientIP(),
			UserAgent:  c.GetHeader("User-Agent"),
		})
	}
}
