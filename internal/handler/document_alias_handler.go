package handler

import (
	"github.com/gin-gonic/gin"

	appErrors "github.com/noah-isme/sma-adp-api/pkg/errors"
	"github.com/noah-isme/sma-adp-api/pkg/response"
)

// DocumentAliasHandler exposes /documents as a thin alias over /archives.
//
// The admin panel's RBAC matrix grants ADMIN_TU CRUD on a `documents` resource,
// but the document store in this system is the archive module: same metadata,
// same signed-URL download, same scope rules. Rather than introducing a second
// parallel store, this alias points `documents` at the archive handler so the
// permission the frontend already grants resolves to a real endpoint.
type DocumentAliasHandler struct {
	archives *ArchiveHandler
}

// NewDocumentAliasHandler wires the alias to the archive handler.
func NewDocumentAliasHandler(archives *ArchiveHandler) *DocumentAliasHandler {
	return &DocumentAliasHandler{archives: archives}
}

// ready reports whether the backing archive handler is wired, answering 500
// rather than panicking if the alias was mounted without it.
func (h *DocumentAliasHandler) ready(c *gin.Context) bool {
	if h == nil || h.archives == nil {
		response.Error(c, appErrors.Clone(appErrors.ErrInternal, "archive service not configured"))
		return false
	}
	return true
}

// Upload godoc
// @Summary Upload a document (alias of /archives)
// @Tags Documents
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
// @Router /documents [post]
func (h *DocumentAliasHandler) Upload(c *gin.Context) {
	if !h.ready(c) {
		return
	}
	h.archives.Upload(c)
}

// List godoc
// @Summary List documents (alias of /archives)
// @Tags Documents
// @Produce json
// @Param scope query string false "Scope filter"
// @Param category query string false "Category filter"
// @Param termId query string false "Term reference"
// @Param classId query string false "Class reference"
// @Success 200 {object} response.Envelope
// @Router /documents [get]
func (h *DocumentAliasHandler) List(c *gin.Context) {
	if !h.ready(c) {
		return
	}
	h.archives.List(c)
}

// Get godoc
// @Summary Get document metadata with signed download URL (alias of /archives)
// @Tags Documents
// @Produce json
// @Param id path string true "Document ID"
// @Success 200 {object} response.Envelope
// @Router /documents/{id} [get]
func (h *DocumentAliasHandler) Get(c *gin.Context) {
	if !h.ready(c) {
		return
	}
	h.archives.Get(c)
}

// Download godoc
// @Summary Download a document via signed token (alias of /archives)
// @Tags Documents
// @Produce octet-stream
// @Param id path string true "Document ID"
// @Param token query string true "Signed token"
// @Success 200 {file} binary
// @Router /documents/{id}/download [get]
func (h *DocumentAliasHandler) Download(c *gin.Context) {
	if !h.ready(c) {
		return
	}
	h.archives.Download(c)
}

// Delete godoc
// @Summary Soft delete a document (alias of /archives)
// @Tags Documents
// @Produce json
// @Param id path string true "Document ID"
// @Success 204
// @Router /documents/{id} [delete]
func (h *DocumentAliasHandler) Delete(c *gin.Context) {
	if !h.ready(c) {
		return
	}
	h.archives.Delete(c)
}
