package middleware

import "github.com/gin-gonic/gin"

const (
	contentTypeOptions = "nosniff"
	frameOptions       = "DENY"
	referrerPolicy     = "strict-origin-when-cross-origin"
	permissionsPolicy  = "geolocation=(), microphone=(), camera=()"
	hstsPolicy         = "max-age=31536000; includeSubDomains"
	apiContentPolicy   = "default-src 'none'; frame-ancestors 'none'; base-uri 'none'"
)

// SecurityHeaders adds response headers that are safe for API responses.
//
// The optional production argument keeps the middleware convenient for local
// handlers and tests while allowing the gateway to enable HSTS and the
// restrictive API CSP only in production. Swagger is served only outside
// production, so local Swagger assets are not constrained by the API CSP.
func SecurityHeaders(production ...bool) gin.HandlerFunc {
	isProduction := len(production) > 0 && production[0]

	return func(c *gin.Context) {
		c.Header("X-Content-Type-Options", contentTypeOptions)
		c.Header("X-Frame-Options", frameOptions)
		c.Header("Referrer-Policy", referrerPolicy)
		c.Header("Permissions-Policy", permissionsPolicy)

		if isProduction {
			c.Header("Strict-Transport-Security", hstsPolicy)
			c.Header("Content-Security-Policy", apiContentPolicy)
		}

		c.Next()
	}
}
