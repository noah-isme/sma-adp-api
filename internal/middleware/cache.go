package middleware

import (
	"time"

	"github.com/gin-gonic/gin"
)

const (
	responseMetaKey = "response_meta"
	cacheHitKey     = "cache_hit"
)

// WithResponseMeta initialises response metadata storage on the request context.
func WithResponseMeta() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Set(responseMetaKey, map[string]interface{}{})
		c.Next()
		duration := time.Since(start)
		meta := ensureMeta(c)
		if _, exists := meta["processing_time_ms"]; !exists {
			meta["processing_time_ms"] = duration.Milliseconds()
		}
	}
}

// SetCacheHit records cache hit information for the current response.
func SetCacheHit(c *gin.Context, hit bool) {
	meta := ensureMeta(c)
	meta[cacheHitKey] = hit
}

// ExtractMeta returns the metadata map stored on the context.
func ExtractMeta(c *gin.Context) map[string]interface{} {
	if c == nil {
		return nil
	}
	if meta, exists := c.Get(responseMetaKey); exists {
		if typed, ok := meta.(map[string]interface{}); ok {
			return typed
		}
	}
	return nil
}

func ensureMeta(c *gin.Context) map[string]interface{} {
	if c == nil {
		return map[string]interface{}{}
	}
	if meta, exists := c.Get(responseMetaKey); exists {
		if typed, ok := meta.(map[string]interface{}); ok {
			return typed
		}
	}
	newMeta := make(map[string]interface{})
	c.Set(responseMetaKey, newMeta)
	return newMeta
}
