package archives

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// ArchiveHandler implements Gin HTTP REST API handlers for Archives Security & Compliance.
type ArchiveHandler struct {
	service *ArchiveService
}

// NewArchiveHandler creates a new ArchiveHandler.
func NewArchiveHandler(service *ArchiveService) *ArchiveHandler {
	return &ArchiveHandler{service: service}
}

// RegisterRoutes registers all archive endpoints under the provided RouterGroup.
func (h *ArchiveHandler) RegisterRoutes(rg *gin.RouterGroup) {
	rg.POST("/upload", h.Upload)
	rg.POST("", h.Upload)
	// rg.GET("/search", h.Search)
	rg.GET("", h.List)
	// rg.GET("/policies", h.ListPolicies)
	// rg.POST("/policies", h.CreatePolicy)
	rg.GET("/audit-logs", h.ListAuditLogs)
	rg.POST("/bulk", h.Bulk)
	// rg.GET("/bulk/download", h.BulkDownload)
	rg.POST("/gdpr", h.GDPR)
	rg.POST("/gdpr/request", h.GDPR)

	rg.GET("/:id", h.Get)
	rg.GET("/:id/download-url", h.GenerateDownloadURL)
	rg.POST("/:id/signed-url", h.GenerateDownloadURL)
	rg.POST("/:id/download-url", h.GenerateDownloadURL)
	rg.GET("/:id/download", h.Download)
	rg.PUT("/:id/retention", h.UpdateRetention)
	rg.PUT("/:id/legal-hold", h.SetLegalHold)
	rg.POST("/:id/legal-hold", h.SetLegalHold)
	rg.DELETE("/:id", h.Delete)
	rg.GET("/:id/audit-logs", h.GetDocumentAuditLogs)
}

// Upload handles POST /api/v1/archives/upload or /api/v1/archives
func (h *ArchiveHandler) Upload(c *gin.Context) {
	categoryStr := c.PostForm("category")
	if categoryStr == "" {
		categoryStr = string(CategoryOther)
	}
	category := DocumentCategory(categoryStr)

	tagsStr := c.PostForm("tags")
	var tags []string
	if tagsStr != "" {
		for _, t := range strings.Split(tagsStr, ",") {
			if trimmed := strings.TrimSpace(t); trimmed != "" {
				tags = append(tags, trimmed)
			}
		}
	}

	metadataStr := c.PostForm("metadata")
	metadata := make(map[string]interface{})
	if metadataStr != "" {
		_ = json.Unmarshal([]byte(metadataStr), &metadata)
	}

	fileHeader, err := c.FormFile("file")
	var fileContent []byte
	filename := "document.pdf"
	if err == nil && fileHeader != nil {
		filename = fileHeader.Filename
		src, oErr := fileHeader.Open()
		if oErr == nil {
			fileContent, _ = io.ReadAll(src)
			src.Close()
		}
	} else {
		// Fallback to raw request body if not multipart
		body, _ := io.ReadAll(c.Request.Body)
		if len(body) > 0 {
			fileContent = body
		}
	}

	userID := h.extractUserID(c)
	doc, err := h.service.UploadDocument(c.Request.Context(), filename, category, tags, metadata, fileContent, userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"data": doc})
}

// List handles GET /api/v1/archives/search or GET /api/v1/archives
func (h *ArchiveHandler) List(c *gin.Context) {
	q := c.Query("q")
	if q == "" {
		q = c.Query("query")
	}

	if q != "" {
		req := SearchRequest{
			Query:       q,
			Category:    DocumentCategory(c.Query("category")),
			StorageTier: StorageTier(c.Query("storage_tier")),
			Page:        1,
			Limit:       50,
		}
		res, err := h.service.Search(c.Request.Context(), req)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, res)
		return
	}

	limit, _ := strconv.Atoi(c.Query("limit"))
	offset, _ := strconv.Atoi(c.Query("offset"))

	filter := ArchiveFilter{
		Query:         q,
		Category:      DocumentCategory(c.Query("category")),
		StorageTier:   StorageTier(c.Query("storage_tier")),
		LegalHoldOnly: c.Query("legal_hold") == "true",
		Limit:         limit,
		Offset:        offset,
	}

	docs, total, err := h.service.ListDocuments(c.Request.Context(), filter)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"data":  docs,
		"total": total,
	})
}

// Get handles GET /api/v1/archives/:id
func (h *ArchiveHandler) Get(c *gin.Context) {
	idStr := c.Param("id")
	docID, err := uuid.Parse(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid document ID format"})
		return
	}

	doc, err := h.service.GetDocument(c.Request.Context(), docID)
	if err != nil {
		if errors.Is(err, ErrDocumentNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "document not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": doc})
}

// GenerateDownloadURL handles POST/GET /api/v1/archives/:id/signed-url or /:id/download-url
func (h *ArchiveHandler) GenerateDownloadURL(c *gin.Context) {
	idStr := c.Param("id")
	docID, err := uuid.Parse(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid document ID format"})
		return
	}

	clientIP := c.Query("client_ip")
	if clientIP == "" {
		var reqBody struct {
			ClientIP string `json:"client_ip"`
			TTLSec   int    `json:"ttl_seconds"`
		}
		_ = c.ShouldBindJSON(&reqBody)
		if reqBody.ClientIP != "" {
			clientIP = reqBody.ClientIP
		}
	}
	if clientIP == "" {
		clientIP = c.ClientIP()
	}

	ttlStr := c.Query("ttl")
	ttl := 30 * time.Minute
	if ttlSec, err := strconv.Atoi(ttlStr); err == nil && ttlSec > 0 {
		ttl = time.Duration(ttlSec) * time.Second
	}

	downloadURL, err := h.service.GenerateDownloadURL(c.Request.Context(), docID, clientIP, ttl)
	if err != nil {
		if errors.Is(err, ErrDocumentNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "document not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"downloadUrl": downloadURL,
		"url":         downloadURL,
	})
}

// Download handles GET /api/v1/archives/:id/download or ?token=...
func (h *ArchiveHandler) Download(c *gin.Context) {
	token := c.Query("token")
	clientIP := c.ClientIP()

	var docID uuid.UUID
	var err error

	if token != "" {
		docID, err = h.service.ValidateDownloadToken(token, clientIP)
		if err != nil {
			if errors.Is(err, ErrTokenExpired) {
				c.JSON(http.StatusForbidden, gin.H{"error": "signed URL token expired"})
				return
			}
			if errors.Is(err, ErrIPMismatch) {
				c.JSON(http.StatusForbidden, gin.H{"error": "client IP mismatch"})
				return
			}
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid signed URL token"})
			return
		}
	} else {
		idStr := c.Param("id")
		docID, err = uuid.Parse(idStr)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "missing token or invalid document ID"})
			return
		}
	}

	doc, err := h.service.GetDocument(c.Request.Context(), docID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "document not found"})
		return
	}

	if doc.StoragePath != "" {
		if _, fErr := os.Stat(doc.StoragePath); fErr == nil {
			c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", doc.Filename))
			c.File(doc.StoragePath)
			return
		}
	}

	// Fallback response if physical file missing
	c.Header("Content-Type", "application/octet-stream")
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", doc.Filename))
	if len(doc.OCRText) > 0 {
		c.String(http.StatusOK, doc.OCRText)
	} else {
		c.String(http.StatusOK, "Document content unavailable")
	}
}

// Delete handles DELETE /api/v1/archives/:id
func (h *ArchiveHandler) Delete(c *gin.Context) {
	idStr := c.Param("id")
	docID, err := uuid.Parse(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid document ID format"})
		return
	}

	userID := h.extractUserID(c)
	if err := h.service.DeleteDocument(c.Request.Context(), docID, userID); err != nil {
		if errors.Is(err, ErrLegalHoldActive) {
			c.JSON(http.StatusConflict, gin.H{"error": "cannot delete: legal hold is active on document"})
			return
		}
		if errors.Is(err, ErrDocumentNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "document not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "document soft deleted successfully"})
}

// Bulk handles POST /api/v1/archives/bulk
func (h *ArchiveHandler) Bulk(c *gin.Context) {
	var req BulkActionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid bulk action request body"})
		return
	}

	if req.Action == "DOWNLOAD" || c.GetHeader("Accept") == "application/zip" || c.Query("format") == "zip" {
		c.Header("Content-Type", "application/zip")
		c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=\"archives_bulk_export_%d.zip\"", time.Now().Unix()))
		_ = h.service.StreamBulkZip(c.Request.Context(), req.IDs, c.Writer)
		return
	}

	userID := h.extractUserID(c)
	resp, err := h.service.ProcessBulkAction(c.Request.Context(), req, userID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, resp)
}

// GDPR handles POST /api/v1/archives/gdpr
func (h *ArchiveHandler) GDPR(c *gin.Context) {
	var req GDPRRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid GDPR request body"})
		return
	}

	resp, err := h.service.ProcessGDPRRequest(c.Request.Context(), req)
	if err != nil {
		if errors.Is(err, ErrLegalHoldActive) {
			c.JSON(http.StatusUnprocessableEntity, gin.H{"error": "GDPR request blocked: active legal hold on document", "code": "LEGAL_HOLD_ACTIVE"})
			return
		}
		if errors.Is(err, ErrRetentionNotExpired) {
			c.JSON(http.StatusUnprocessableEntity, gin.H{"error": "GDPR request blocked: retention policy not expired", "code": "RETENTION_NOT_EXPIRED"})
			return
		}
		if errors.Is(err, ErrDocumentNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "document not found"})
			return
		}
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, resp)
}

// UpdateRetention handles PUT /api/v1/archives/:id/retention
func (h *ArchiveHandler) UpdateRetention(c *gin.Context) {
	idStr := c.Param("id")
	docID, err := uuid.Parse(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid document ID format"})
		return
	}

	var req UpdateRetentionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid retention request body"})
		return
	}

	userID := h.extractUserID(c)
	doc, err := h.service.UpdateRetention(c.Request.Context(), docID, req, userID)
	if err != nil {
		if errors.Is(err, ErrDocumentNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "document not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": doc})
}

// SetLegalHold handles PUT/POST /api/v1/archives/:id/legal-hold
func (h *ArchiveHandler) SetLegalHold(c *gin.Context) {
	idStr := c.Param("id")
	docID, err := uuid.Parse(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid document ID format"})
		return
	}

	var body struct {
		LegalHold bool   `json:"legalHold"`
		Reason    string `json:"reason"`
	}
	_ = c.ShouldBindJSON(&body)

	userID := h.extractUserID(c)
	doc, err := h.service.SetLegalHold(c.Request.Context(), docID, body.LegalHold, body.Reason, userID)
	if err != nil {
		if errors.Is(err, ErrDocumentNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "document not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": doc})
}

// GetDocumentAuditLogs handles GET /api/v1/archives/:id/audit-logs
func (h *ArchiveHandler) GetDocumentAuditLogs(c *gin.Context) {
	idStr := c.Param("id")
	docID, err := uuid.Parse(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid document ID format"})
		return
	}

	logs, err := h.service.GetAuditLogsByDocument(c.Request.Context(), docID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": logs})
}

// ListAuditLogs handles GET /api/v1/archives/audit-logs
func (h *ArchiveHandler) ListAuditLogs(c *gin.Context) {
	limit, _ := strconv.Atoi(c.Query("limit"))
	offset, _ := strconv.Atoi(c.Query("offset"))

	filter := AuditLogFilter{
		Action: c.Query("action"),
		Limit:  limit,
		Offset: offset,
	}

	if docIDStr := c.Query("document_id"); docIDStr != "" {
		if docID, err := uuid.Parse(docIDStr); err == nil {
			filter.DocumentID = &docID
		}
	}
	if userIDStr := c.Query("user_id"); userIDStr != "" {
		if userID, err := uuid.Parse(userIDStr); err == nil {
			filter.UserID = &userID
		}
	}

	logs, total, err := h.service.ListAuditLogs(c.Request.Context(), filter)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, AuditLogResponse{
		Data:  logs,
		Total: total,
	})
}

func (h *ArchiveHandler) extractUserID(c *gin.Context) uuid.UUID {
	if val, exists := c.Get("userID"); exists {
		if uid, ok := val.(uuid.UUID); ok {
			return uid
		}
		if str, ok := val.(string); ok {
			if parsed, err := uuid.Parse(str); err == nil {
				return parsed
			}
		}
	}
	return uuid.Nil
}
