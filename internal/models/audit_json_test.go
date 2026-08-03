package models_test

import (
	"encoding/json"
	"testing"

	"github.com/jmoiron/sqlx/types"

	"github.com/noah-isme/sma-adp-api/internal/models"
)

// TestAuditLogEntryEmitsRawJSON is the reason AuditLogEntry does not embed
// AuditLog: the write model types the JSON columns as []byte, which
// encoding/json base64-encodes. The viewer must receive the stored JSON.
func TestAuditLogEntryEmitsRawJSON(t *testing.T) {
	raw := types.JSONText(`{"created":1}`)
	entry := models.AuditLogEntry{ID: "log-1", NewValues: &raw}

	encoded, err := json.Marshal(entry)
	if err != nil {
		t.Fatalf("marshal audit entry: %v", err)
	}

	var decoded struct {
		NewValues map[string]int `json:"new_values"`
	}
	if err := json.Unmarshal(encoded, &decoded); err != nil {
		t.Fatalf("audit entry did not round-trip as JSON (likely base64): %v\npayload: %s", err, encoded)
	}
	if decoded.NewValues["created"] != 1 {
		t.Fatalf("expected new_values.created == 1, got %v (payload: %s)", decoded.NewValues, encoded)
	}
}

// Absent JSON columns must be omitted rather than serialised as null.
func TestAuditLogEntryOmitsEmptyJSONColumns(t *testing.T) {
	encoded, err := json.Marshal(models.AuditLogEntry{ID: "log-1"})
	if err != nil {
		t.Fatalf("marshal audit entry: %v", err)
	}

	var decoded map[string]any
	if err := json.Unmarshal(encoded, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if _, ok := decoded["old_values"]; ok {
		t.Fatalf("expected old_values to be omitted, got %s", encoded)
	}
	if _, ok := decoded["new_values"]; ok {
		t.Fatalf("expected new_values to be omitted, got %s", encoded)
	}
}
