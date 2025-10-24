package response

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/noah-isme/sma-adp-api/internal/models"
	appErrors "github.com/noah-isme/sma-adp-api/pkg/errors"
)

// Envelope represents the common response contract.
type Envelope struct {
	Data       interface{}        `json:"data,omitempty"`
	Error      *appErrors.Error   `json:"error,omitempty"`
	Pagination *models.Pagination `json:"pagination,omitempty"`
}

// JSON sends a success response with optional pagination metadata.
func JSON(c *gin.Context, status int, data interface{}, pagination *models.Pagination) {
	c.Header("Cache-Control", "no-store")
	c.Header("Pragma", "no-cache")
	c.JSON(status, Envelope{Data: data, Pagination: pagination})
}

// Created responds with HTTP 201 Created.
func Created(c *gin.Context, data interface{}) {
	JSON(c, http.StatusCreated, data, nil)
}

// Error sends an error response converting the error to the common structure.
func Error(c *gin.Context, err error) {
	appErr := appErrors.FromError(err)
	c.Header("Cache-Control", "no-store")
	c.Header("Pragma", "no-cache")
	c.JSON(appErr.Status, Envelope{Error: appErr})
}

// NoContent sends a 204 response.
func NoContent(c *gin.Context) {
	c.Status(http.StatusNoContent)
}
