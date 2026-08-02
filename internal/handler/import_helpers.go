package handler

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/csv"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/noah-isme/sma-adp-api/internal/middleware"
	"github.com/noah-isme/sma-adp-api/internal/models"
	appErrors "github.com/noah-isme/sma-adp-api/pkg/errors"
	"github.com/noah-isme/sma-adp-api/pkg/response"
)

const (
	maxCSVImportBytes = 5 * 1024 * 1024
	maxCSVImportRows  = 10000
)

type importRunStore interface {
	Begin(ctx context.Context, importType, key, requestHash string, userID *string) (*models.ImportRun, bool, error)
	Complete(ctx context.Context, runID, importType string, created, failed int, result []byte, userID *string, ipAddress, userAgent string) error
}

func readCSVImportBody(c *gin.Context) ([]byte, bool) {
	if c.Request.ContentLength > maxCSVImportBytes {
		response.Error(c, appErrors.New("IMPORT_TOO_LARGE", http.StatusRequestEntityTooLarge, "CSV file exceeds the 5 MiB limit"))
		return nil, false
	}
	body, err := io.ReadAll(io.LimitReader(c.Request.Body, maxCSVImportBytes+1))
	if err != nil {
		response.Error(c, appErrors.Wrap(err, appErrors.ErrValidation.Code, appErrors.ErrValidation.Status, "failed to read CSV body"))
		return nil, false
	}
	if len(body) > maxCSVImportBytes {
		response.Error(c, appErrors.New("IMPORT_TOO_LARGE", http.StatusRequestEntityTooLarge, "CSV file exceeds the 5 MiB limit"))
		return nil, false
	}
	return body, true
}

func parseCSVImport(body []byte, required []string) (map[string]int, [][]string, error) {
	r := csv.NewReader(bytes.NewReader(body))
	r.FieldsPerRecord = -1
	header, err := r.Read()
	if err != nil {
		return nil, nil, appErrors.Clone(appErrors.ErrValidation, "CSV header required")
	}
	columns := make(map[string]int, len(header))
	for i, name := range header {
		columns[strings.TrimSpace(strings.ToLower(name))] = i
	}
	for _, name := range required {
		if _, ok := columns[name]; !ok {
			return nil, nil, appErrors.Clone(appErrors.ErrValidation, "missing CSV column: "+name)
		}
	}
	rows, err := r.ReadAll()
	if err != nil {
		return nil, nil, appErrors.Wrap(err, appErrors.ErrValidation.Code, appErrors.ErrValidation.Status, "invalid CSV record")
	}
	if len(rows) > maxCSVImportRows {
		return nil, nil, appErrors.New("IMPORT_TOO_MANY_ROWS", http.StatusRequestEntityTooLarge, fmt.Sprintf("CSV file exceeds the %d row limit", maxCSVImportRows))
	}
	return columns, rows, nil
}

func importValue(values []string, columns map[string]int, name string) string {
	i, ok := columns[name]
	if !ok || i >= len(values) {
		return ""
	}
	return strings.TrimSpace(values[i])
}

func importOptionalValue(values []string, columns map[string]int, name string) *string {
	value := importValue(values, columns, name)
	if value == "" {
		return nil
	}
	return &value
}

func beginCSVImport(c *gin.Context, store importRunStore, importType string, body []byte) (*models.ImportRun, bool) {
	if store == nil {
		return nil, true
	}
	hash := sha256.Sum256(body)
	requestHash := hex.EncodeToString(hash[:])
	userID := currentImportUserID(c)
	key := strings.TrimSpace(c.GetHeader("Idempotency-Key"))
	if key == "" {
		if userID != nil {
			scopedHash := sha256.Sum256([]byte(*userID + ":" + requestHash))
			key = importType + ":" + hex.EncodeToString(scopedHash[:])
		} else {
			key = importType + ":" + requestHash
		}
	}
	if len(key) > 255 {
		response.Error(c, appErrors.Clone(appErrors.ErrValidation, "Idempotency-Key must be at most 255 characters"))
		return nil, false
	}
	run, created, err := store.Begin(c.Request.Context(), importType, key, requestHash, userID)
	if err != nil {
		response.Error(c, appErrors.Wrap(err, appErrors.ErrInternal.Code, appErrors.ErrInternal.Status, "failed to claim import idempotency key"))
		return nil, false
	}
	if created {
		return run, true
	}
	if run.RequestHash != requestHash {
		response.Error(c, appErrors.Clone(appErrors.ErrConflict, "Idempotency-Key was already used for a different CSV"))
		return nil, false
	}
	if run.Status == "COMPLETED" && len(run.Result) > 0 {
		response.JSON(c, http.StatusOK, json.RawMessage(run.Result), nil)
		return nil, false
	}
	response.Error(c, appErrors.New("IMPORT_IN_PROGRESS", http.StatusConflict, "an import with this Idempotency-Key is already processing"))
	return nil, false
}

func completeCSVImport(c *gin.Context, store importRunStore, run *models.ImportRun, importType string, created int, failures []gin.H) bool {
	result := gin.H{"created": created, "failed": len(failures), "failures": failures}
	encoded, err := json.Marshal(result)
	if err != nil {
		response.Error(c, appErrors.Wrap(err, appErrors.ErrInternal.Code, appErrors.ErrInternal.Status, "failed to encode import result"))
		return false
	}
	if run != nil && store != nil {
		if err := store.Complete(c.Request.Context(), run.ID, importType, created, len(failures), encoded, currentImportUserID(c), c.ClientIP(), c.GetHeader("User-Agent")); err != nil {
			response.Error(c, appErrors.Wrap(err, appErrors.ErrInternal.Code, appErrors.ErrInternal.Status, "failed to persist import audit"))
			return false
		}
	}
	response.JSON(c, http.StatusOK, result, nil)
	return true
}

func currentImportUserID(c *gin.Context) *string {
	claimsValue, ok := c.Get(middleware.ContextUserKey)
	if !ok {
		return nil
	}
	claims, ok := claimsValue.(*models.JWTClaims)
	if !ok || claims.UserID == "" {
		return nil
	}
	userID := claims.UserID
	return &userID
}
