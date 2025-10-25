package middleware

import (
	"github.com/gin-gonic/gin"

	"github.com/noah-isme/sma-adp-api/internal/models"
	"github.com/noah-isme/sma-adp-api/internal/service"
)

const (
	cutoverStageContextKey   = "cutover_stage"
	cutoverSegmentContextKey = "cutover_segment"
)

// CutoverStage annotates responses with rollout metadata headers.
func CutoverStage(cutoverSvc *service.CutoverService) gin.HandlerFunc {
	return func(c *gin.Context) {
		if cutoverSvc != nil {
			headers := cutoverSvc.HeadersForRequest(c.Request)
			applyHeader(c, headers.StageHeader, string(headers.Stage))
			applyHeader(c, headers.SegmentHeader, headers.Segment)
			c.Set(cutoverStageContextKey, headers.Stage)
			c.Set(cutoverSegmentContextKey, headers.Segment)
		}
		c.Next()
	}
}

// CutoverMetadata extracts the metadata from context for downstream handlers/tests.
func CutoverMetadata(c *gin.Context) (models.CutoverStage, string) {
	var stage models.CutoverStage
	if value, exists := c.Get(cutoverStageContextKey); exists {
		if typed, ok := value.(models.CutoverStage); ok {
			stage = typed
		}
	}
	var segment string
	if value, exists := c.Get(cutoverSegmentContextKey); exists {
		if typed, ok := value.(string); ok {
			segment = typed
		}
	}
	return stage, segment
}

func applyHeader(c *gin.Context, key, value string) {
	if c == nil || key == "" || value == "" {
		return
	}
	c.Writer.Header().Set(key, value)
	if c.Request != nil {
		c.Request.Header.Set(key, value)
	}
}
