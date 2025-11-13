package handler

import (
	"github.com/gin-gonic/gin"

	"github.com/noah-isme/sma-adp-api/internal/middleware"
	"github.com/noah-isme/sma-adp-api/internal/models"
)

func claimsFromContext(c *gin.Context) *models.JWTClaims {
	value, exists := c.Get(middleware.ContextUserKey)
	if !exists {
		return nil
	}
	claims, ok := value.(*models.JWTClaims)
	if !ok {
		return nil
	}
	return claims
}
