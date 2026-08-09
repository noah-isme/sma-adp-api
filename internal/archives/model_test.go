package archives

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestJSONMap_ValueAndScan(t *testing.T) {
	t.Run("nil JSONMap Value", func(t *testing.T) {
		var m JSONMap
		val, err := m.Value()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if val != "{}" {
			t.Errorf("expected '{}', got %v", val)
		}
	})

	t.Run("valid JSONMap Value and Scan", func(t *testing.T) {
		m := JSONMap{
			"key1": "value1",
			"key2": float64(42),
		}
		val, err := m.Value()
		if err != nil {
			t.Fatalf("Value() error: %v", err)
		}

		var scanned JSONMap
		err = scanned.Scan(val)
		if err != nil {
			t.Fatalf("Scan() error: %v", err)
		}

		if scanned["key1"] != "value1" {
			t.Errorf("expected key1='value1', got %v", scanned["key1"])
		}
		if scanned["key2"] != float64(42) {
			t.Errorf("expected key2=42, got %v", scanned["key2"])
		}
	})

	t.Run("Scan bytes and string", func(t *testing.T) {
		jsonStr := `{"author":"john"}`
		var m1, m2 JSONMap

		if err := m1.Scan([]byte(jsonStr)); err != nil {
			t.Fatalf("Scan([]byte) failed: %v", err)
		}
		if m1["author"] != "john" {
			t.Errorf("expected author='john', got %v", m1["author"])
		}

		if err := m2.Scan(jsonStr); err != nil {
			t.Fatalf("Scan(string) failed: %v", err)
		}
		if m2["author"] != "john" {
			t.Errorf("expected author='john', got %v", m2["author"])
		}
	})

	t.Run("Scan nil or empty", func(t *testing.T) {
		var m1, m2, m3 JSONMap
		if err := m1.Scan(nil); err != nil {
			t.Fatalf("Scan(nil) failed: %v", err)
		}
		if m1 == nil {
			t.Errorf("expected non-nil map after scanning nil")
		}

		if err := m2.Scan([]byte("")); err != nil {
			t.Fatalf("Scan(empty) failed: %v", err)
		}
		if m2 == nil {
			t.Errorf("expected non-nil map after scanning empty byte slice")
		}

		if err := m3.Scan([]byte("null")); err != nil {
			t.Fatalf("Scan(null) failed: %v", err)
		}
		if m3 == nil {
			t.Errorf("expected non-nil map after scanning JSON null byte slice")
		}
		m3["test"] = "value"
	})

	t.Run("Scan unsupported type", func(t *testing.T) {
		var m JSONMap
		err := m.Scan(12345)
		if err == nil {
			t.Errorf("expected error scanning int into JSONMap, got nil")
		}
	})
}

func TestConstants(t *testing.T) {
	categories := []DocumentCategory{
		CategoryStudentRecord,
		CategoryGradeReport,
		CategoryAttendanceRecord,
		CategoryBehaviorNote,
		CategoryMedicalRecord,
		CategoryFinancialDoc,
		CategoryLegalDoc,
		CategoryCorrespondence,
		CategoryOther,
	}
	if len(categories) != 9 {
		t.Errorf("expected 9 categories, got %d", len(categories))
	}

	tiers := []StorageTier{StorageTierHot, StorageTierWarm, StorageTierCold}
	if len(tiers) != 3 {
		t.Errorf("expected 3 storage tiers, got %d", len(tiers))
	}

	ocrStatuses := []OCRStatus{OCRStatusPending, OCRStatusProcessing, OCRStatusCompleted, OCRStatusFailed}
	if len(ocrStatuses) != 4 {
		t.Errorf("expected 4 ocr statuses, got %d", len(ocrStatuses))
	}
}

func TestDTOSerialization(t *testing.T) {
	docID := uuid.New()
	userID := uuid.New()
	now := time.Now().UTC()

	resp := DocumentResponse{
		ID:               docID,
		Filename:         "doc1.pdf",
		OriginalFilename: "original.pdf",
		MimeType:         "application/pdf",
		SizeBytes:        1024,
		Checksum:         "abc123hash",
		StorageTier:      StorageTierHot,
		Category:         CategoryStudentRecord,
		Tags:             []string{"student", "record"},
		Metadata:         map[string]interface{}{"dept": "academic"},
		OCRStatus:        OCRStatusPending,
		RetainUntil:      now,
		LegalHold:        false,
		UploadedBy:       userID,
		UploadedAt:       now,
	}

	data, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("failed to marshal DocumentResponse: %v", err)
	}

	var unmarshaled DocumentResponse
	if err := json.Unmarshal(data, &unmarshaled); err != nil {
		t.Fatalf("failed to unmarshal DocumentResponse: %v", err)
	}

	if unmarshaled.ID != docID {
		t.Errorf("expected ID %v, got %v", docID, unmarshaled.ID)
	}
	if unmarshaled.Filename != "doc1.pdf" {
		t.Errorf("expected Filename 'doc1.pdf', got %v", unmarshaled.Filename)
	}
	if len(unmarshaled.Tags) != 2 || unmarshaled.Tags[0] != "student" {
		t.Errorf("unexpected tags: %v", unmarshaled.Tags)
	}
}
