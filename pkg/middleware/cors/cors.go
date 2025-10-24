package cors

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

// New returns a simple CORS middleware that honors a list of allowed origins.
func New(allowedOrigins []string) gin.HandlerFunc {
	allowAll := len(allowedOrigins) == 0
	originSet := make(map[string]struct{}, len(allowedOrigins))
	for _, origin := range allowedOrigins {
		originSet[strings.TrimRight(origin, "/")] = struct{}{}
	}

	return func(c *gin.Context) {
		origin := c.GetHeader("Origin")
		if origin != "" {
			if allowAll || hasOrigin(originSet, origin) {
				c.Writer.Header().Set("Access-Control-Allow-Origin", origin)
			}
		} else if allowAll {
			c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
		}

		c.Writer.Header().Set("Vary", "Origin")
		c.Writer.Header().Set("Access-Control-Allow-Credentials", "true")
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type, X-Requested-With, X-Request-ID")
		c.Writer.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
		c.Writer.Header().Set("Access-Control-Max-Age", "600")

		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}

		c.Next()
	}
}

func hasOrigin(originSet map[string]struct{}, origin string) bool {
	if len(originSet) == 0 {
		return true
	}

	origin = strings.TrimRight(origin, "/")
	_, ok := originSet[origin]
	return ok
}
