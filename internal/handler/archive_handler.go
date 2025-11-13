package handler

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/noah-isme/sma-adp-api/internal/dto"
	"github.com/noah-isme/sma-adp-api/internal/models"
	"github.com/noah-isme/sma-adp-api/internal/service"
	appErrors "github.com/noah-isme/sma-adp-api/pkg/errors"
	"github.com/noah-isme/sma-adp-api/pkg/response"
)

type archiveService interface {
	Upload(ctx context.Context, meta dto.CreateArchiveRequest, upload service.ArchiveUpload, actor *models.JWTClaims) (*models.ArchiveItem, error)
	List(ctx context.Context, filter dto.ArchiveFilter, actor *models.JWTClaims) ([]models.ArchiveItem, error)
	Get(ctx context.Context, id string, actor *models.JWTClaims) (*models.ArchiveItem, error)
	GetDownloadURL(ctx context.Context, id string, actor *models.JWTClaims) (string, error)
	Download(ctx context.Context, id, token string, actor *models.JWTClaims) (*service.ArchiveDownload, error)
	Delete(ctx context.Context, id string, actor *models.JWTClaims) error
}

// ArchiveHandler manages archive HTTP endpoints.
type ArchiveHandler struct {
	service archiveService
}

// NewArchiveHandler constructs the handler.
func NewArchiveHandler(service archiveService) *ArchiveHandler {
	return &ArchiveHandler{service: service}
}

// Upload godoc
// @Summary Upload archive document
// @Tags Archives
// @Accept multipart/form-data
// @Produce json
// @Param title formData string true "Title"
// @Param category formData string true "Category"
// @Param scope formData string true "Scope"
// @Param refTermId formData string false "Term reference"
// @Param refClassId formData string false "Class reference"
// @Param refStudentId formData string false "Student reference"
// @Param file formData file true "Document"
// @Success 201 {object} response.Envelope
// @Router /archives [post]
func (h *ArchiveHandler) Upload(c *gin.Context) {
	if h.service == nil {
		response.Error(c, appErrors.Clone(appErrors.ErrInternal, "archive service not configured"))
		return
	}
	var req dto.CreateArchiveRequest
	if err := c.ShouldBind(&req); err != nil {
		response.Error(c, appErrors.Clone(appErrors.ErrValidation, "invalid archive payload"))
		return
	}
	fileHeader, err := c.FormFile("file")
	if err != nil {
		response.Error(c, appErrors.Clone(appErrors.ErrValidation, "file is required"))
		return
	}
	src, err := fileHeader.Open()
	if err != nil {
		response.Error(c, appErrors.Wrap(err, appErrors.ErrInternal.Code, appErrors.ErrInternal.Status, "failed to open file"))
		return
	}
	defer src.Close()

	reader, ok := src.(io.ReadSeeker)
	if !ok {
		buf, readErr := io.ReadAll(src)
		if readErr != nil {
			response.Error(c, appErrors.Wrap(readErr, appErrors.ErrInternal.Code, appErrors.ErrInternal.Status, "failed to buffer file"))
			return
		}
		reader = bytes.NewReader(buf)
	}
	claims := claimsFromContext(c)
	if claims == nil {
		response.Error(c, appErrors.ErrUnauthorized)
		return
	}
	upload := service.ArchiveUpload{
		Filename: fileHeader.Filename,
		Size:     fileHeader.Size,
		MimeType: fileHeader.Header.Get("Content-Type"),
		Content:  reader,
	}
	item, err := h.service.Upload(c.Request.Context(), req, upload, claims)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.JSON(c, http.StatusCreated, item, nil)
}

// List godoc
// @Summary List archives
// @Tags Archives
// @Produce json
// @Param scope query string false "Scope filter"
// @Param category query string false "Category filter"
// @Param termId query string false "Term reference"
// @Param classId query string false "Class reference"
// @Success 200 {object} response.Envelope
// @Router /archives [get]
func (h *ArchiveHandler) List(c *gin.Context) {
	if h.service == nil {
		response.Error(c, appErrors.Clone(appErrors.ErrInternal, "archive service not configured"))
		return
	}
	claims := claimsFromContext(c)
	if claims == nil {
		response.Error(c, appErrors.ErrUnauthorized)
		return
	}
	filter := dto.ArchiveFilter{
		Category: strings.TrimSpace(c.Query("category")),
		TermID:   strings.TrimSpace(c.Query("termId")),
		ClassID:  strings.TrimSpace(c.Query("classId")),
	}
	if scope := c.Query("scope"); scope != "" {
		filter.Scope = models.ArchiveScope(strings.ToUpper(scope))
	}
	items, err := h.service.List(c.Request.Context(), filter, claims)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.JSON(c, http.StatusOK, items, nil)
}

// Get godoc
// @Summary Get archive metadata
// @Tags Archives
// @Produce json
// @Param id path string true "Archive ID"
// @Success 200 {object} response.Envelope
// @Router /archives/{id} [get]
func (h *ArchiveHandler) Get(c *gin.Context) {
	if h.service == nil {
		response.Error(c, appErrors.Clone(appErrors.ErrInternal, "archive service not configured"))
		return
	}
	claims := claimsFromContext(c)
	if claims == nil {
		response.Error(c, appErrors.ErrUnauthorized)
		return
	}
	item, err := h.service.Get(c.Request.Context(), c.Param("id"), claims)
	if err != nil {
		response.Error(c, err)
		return
	}
	downloadURL, err := h.service.GetDownloadURL(c.Request.Context(), item.ID, claims)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.JSON(c, http.StatusOK, dto.ArchiveDownloadResponse{
		ArchiveItem: *item,
		DownloadURL: downloadURL,
	}, nil)
}

// Download godoc
// @Summary Download archive document via signed token
// @Tags Archives
// @Produce octet-stream
// @Param id path string true "Archive ID"
// @Param token query string true "Signed token"
// @Success 200 {file} binary
// @Router /archives/{id}/download [get]
func (h *ArchiveHandler) Download(c *gin.Context) {
	if h.service == nil {
		response.Error(c, appErrors.Clone(appErrors.ErrInternal, "archive service not configured"))
		return
	}
	claims := claimsFromContext(c)
	if claims == nil {
		response.Error(c, appErrors.ErrUnauthorized)
		return
	}
	token := c.Query("token")
	if strings.TrimSpace(token) == "" {
		response.Error(c, appErrors.Clone(appErrors.ErrValidation, "token is required"))
		return
	}
	result, err := h.service.Download(c.Request.Context(), c.Param("id"), token, claims)
	if err != nil {
		response.Error(c, err)
		return
	}
	defer result.File.Close() //nolint:errcheck
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", result.Filename))
	c.Header("Cache-Control", "no-store")
	c.DataFromReader(http.StatusOK, result.SizeBytes, result.MimeType, result.File, nil)
}

// Delete godoc
// @Summary Soft delete an archive entry
// @Tags Archives
// @Produce json
// @Param id path string true "Archive ID"
// @Success 204
// @Router /archives/{id} [delete]
func (h *ArchiveHandler) Delete(c *gin.Context) {
	if h.service == nil {
		response.Error(c, appErrors.Clone(appErrors.ErrInternal, "archive service not configured"))
		return
	}
	claims := claimsFromContext(c)
	if claims == nil {
		response.Error(c, appErrors.ErrUnauthorized)
		return
	}
	if err := h.service.Delete(c.Request.Context(), c.Param("id"), claims); err != nil {
		response.Error(c, err)
		return
	}
	response.NoContent(c)
}
