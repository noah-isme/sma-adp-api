package requestid

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/gin-gonic/gin"
)

const (
	headerKey  = "X-Request-ID"
	contextKey = "request_id"
)

// Middleware assigns a unique request ID to each incoming HTTP request.
func Middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		reqID := c.GetHeader(headerKey)
		if reqID == "" {
			reqID = generateID()
		}

		c.Set(contextKey, reqID)
		c.Writer.Header().Set(headerKey, reqID)

		c.Next()
	}
}

// Value returns the request ID stored in the Gin context.
func Value(c *gin.Context) string {
	if v, exists := c.Get(contextKey); exists {
		if id, ok := v.(string); ok {
			return id
		}
	}
	return ""
}

func generateID() string {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err == nil {
		return hex.EncodeToString(buf)
	}

	return fmt.Sprintf("fallback-%d", time.Now().UnixNano())
}
