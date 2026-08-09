package archives

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupTestRouter() (*gin.Engine, *ArchiveService, *ArchiveHandler) {
	gin.SetMode(gin.TestMode)
	repo := NewMemoryRepository()
	searchEngine := NewMemorySearchEngine()
	workerPool := NewGoOCRWorkerPool(1, 10, repo, searchEngine)
	signer := NewHMACSignedURLSigner("test_secret", "/api/v1/archives")
	retentionEngine := NewRetentionEngine(repo)
	service := NewArchiveService(repo, searchEngine, workerPool, signer, retentionEngine)
	handler := NewArchiveHandler(service)

	r := gin.New()
	rg := r.Group("/api/v1/archives")
	handler.RegisterRoutes(rg)

	return r, service, handler
}

func TestArchiveHandler_UploadAndGet(t *testing.T) {
	r, _, _ := setupTestRouter()

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/archives/upload", bytes.NewBufferString("sample file content"))
	req.Header.Set("Content-Type", "application/octet-stream")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)
	var resp map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	data, ok := resp["data"].(map[string]interface{})
	require.True(t, ok)
	docIDStr := data["id"].(string)
	docID, err := uuid.Parse(docIDStr)
	require.NoError(t, err)

	// GET Document
	wGet := httptest.NewRecorder()
	reqGet, _ := http.NewRequest("GET", "/api/v1/archives/"+docID.String(), nil)
	r.ServeHTTP(wGet, reqGet)

	assert.Equal(t, http.StatusOK, wGet.Code)
}

func TestArchiveHandler_DownloadURLAndDownload(t *testing.T) {
	r, service, _ := setupTestRouter()
	ctx := context.Background()

	doc, err := service.UploadDocument(ctx, "download_test.pdf", CategoryStudentRecord, []string{}, nil, []byte("Hello Download"), uuid.New())
	require.NoError(t, err)

	// Generate Download URL via POST /signed-url
	wURL := httptest.NewRecorder()
	reqURL, _ := http.NewRequest("POST", fmt.Sprintf("/api/v1/archives/%s/signed-url", doc.ID), nil)
	r.ServeHTTP(wURL, reqURL)

	assert.Equal(t, http.StatusOK, wURL.Code)
	var urlResp map[string]string
	require.NoError(t, json.Unmarshal(wURL.Body.Bytes(), &urlResp))
	downloadURLStr := urlResp["downloadUrl"]
	require.NotEmpty(t, downloadURLStr)

	parsedURL, err := url.Parse(downloadURLStr)
	require.NoError(t, err)
	token := parsedURL.Query().Get("token")
	require.NotEmpty(t, token)

	// Download file using signed token
	wDL := httptest.NewRecorder()
	reqDL, _ := http.NewRequest("GET", fmt.Sprintf("/api/v1/archives/%s/download?token=%s", doc.ID, url.QueryEscape(token)), nil)
	r.ServeHTTP(wDL, reqDL)

	assert.Equal(t, http.StatusOK, wDL.Code)
	assert.Contains(t, wDL.Body.String(), "Hello Download")
}

func TestArchiveHandler_BulkAndGDPR(t *testing.T) {
	r, service, _ := setupTestRouter()
	ctx := context.Background()
	userID := uuid.New()

	doc1, err := service.UploadDocument(ctx, "doc1.pdf", CategoryStudentRecord, []string{}, map[string]any{"student_id": "stu_88"}, []byte("Content 1"), userID)
	require.NoError(t, err)
	doc2, err := service.UploadDocument(ctx, "doc2.pdf", CategoryStudentRecord, []string{}, map[string]any{"student_id": "stu_88"}, []byte("Content 2"), userID)
	require.NoError(t, err)

	// Test Bulk Change Category
	bulkReq := BulkActionRequest{
		Action: "CHANGE_CATEGORY",
		IDs:    []uuid.UUID{doc1.ID, doc2.ID},
		Parameters: map[string]string{
			"category": string(CategoryFinancialDoc),
		},
	}
	bulkBody, _ := json.Marshal(bulkReq)
	wBulk := httptest.NewRecorder()
	reqBulk, _ := http.NewRequest("POST", "/api/v1/archives/bulk", bytes.NewBuffer(bulkBody))
	reqBulk.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(wBulk, reqBulk)

	assert.Equal(t, http.StatusOK, wBulk.Code)

	// Test GDPR Access Request
	gdprAccessReq := GDPRRequest{
		Type:           "ACCESS",
		SubjectID:      userID.String(),
		RequesterEmail: "test@school.edu",
	}
	gdprAccessBody, _ := json.Marshal(gdprAccessReq)
	wGDPR := httptest.NewRecorder()
	reqGDPR, _ := http.NewRequest("POST", "/api/v1/archives/gdpr", bytes.NewBuffer(gdprAccessBody))
	reqGDPR.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(wGDPR, reqGDPR)

	assert.Equal(t, http.StatusOK, wGDPR.Code)
}

func TestArchiveHandler_RetentionAndLegalHold(t *testing.T) {
	r, service, _ := setupTestRouter()
	ctx := context.Background()

	doc, err := service.UploadDocument(ctx, "retention_test.pdf", CategoryStudentRecord, []string{}, nil, []byte("Retention Content"), uuid.New())
	require.NoError(t, err)

	// Update Retention
	retBody, _ := json.Marshal(map[string]interface{}{
		"legalHold":       true,
		"legalHoldReason": "Subpoena 2026",
	})
	wRet := httptest.NewRecorder()
	reqRet, _ := http.NewRequest("PUT", fmt.Sprintf("/api/v1/archives/%s/retention", doc.ID), bytes.NewBuffer(retBody))
	reqRet.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(wRet, reqRet)

	assert.Equal(t, http.StatusOK, wRet.Code)

	// Get Document Audit Logs
	wLogs := httptest.NewRecorder()
	reqLogs, _ := http.NewRequest("GET", fmt.Sprintf("/api/v1/archives/%s/audit-logs", doc.ID), nil)
	r.ServeHTTP(wLogs, reqLogs)

	assert.Equal(t, http.StatusOK, wLogs.Code)

	// List Global Audit Logs
	wGlobal := httptest.NewRecorder()
	reqGlobal, _ := http.NewRequest("GET", "/api/v1/archives/audit-logs", nil)
	r.ServeHTTP(wGlobal, reqGlobal)

	assert.Equal(t, http.StatusOK, wGlobal.Code)
}

