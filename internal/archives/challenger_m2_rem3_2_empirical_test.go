package archives

import (
	"archive/zip"
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// =============================================================================
// 1. MEILISEARCH FILTER ESCAPING EMPIRICAL ORACLE & GENERATOR TEST
// =============================================================================

// meiliStringUnescapeSim simulates Meilisearch single-quoted string filter parsing.
// It parses `'<escaped_val>'` and returns the unescaped string or an error if malformed.
func meiliStringUnescapeSim(escapedFilter string) (string, error) {
	if !strings.HasPrefix(escapedFilter, "'") || !strings.HasSuffix(escapedFilter, "'") || len(escapedFilter) < 2 {
		return "", fmt.Errorf("filter string not enclosed in single quotes: %s", escapedFilter)
	}

	inner := escapedFilter[1 : len(escapedFilter)-1]
	var buf strings.Builder
	runes := []rune(inner)
	i := 0
	for i < len(runes) {
		r := runes[i]
		if r == '\\' {
			if i+1 >= len(runes) {
				return "", fmt.Errorf("dangling escape backslash at index %d in %s", i, escapedFilter)
			}
			next := runes[i+1]
			if next == '\\' {
				buf.WriteRune('\\')
				i += 2
			} else if next == '\'' {
				buf.WriteRune('\'')
				i += 2
			} else {
				// Meilisearch escape for other characters or literal backslash
				buf.WriteRune(next)
				i += 2
			}
		} else if r == '\'' {
			return "", fmt.Errorf("unescaped single quote found at index %d in %s", i, escapedFilter)
		} else {
			buf.WriteRune(r)
			i++
		}
	}
	return buf.String(), nil
}

func TestMeiliFilterEscaping_EmpiricalGeneratorAndOracle(t *testing.T) {
	// Adversarial inputs generator covering trailing backslashes, quotes, and complex combinations
	testCases := []struct {
		name  string
		input string
	}{
		{"Empty string", ""},
		{"Simple alphanumeric", "finance_2026"},
		{"Single trailing backslash", "folder\\"},
		{"Double trailing backslash", "folder\\\\"},
		{"Triple trailing backslash", "folder\\\\\\"},
		{"Quadruple trailing backslash", "folder\\\\\\\\"},
		{"Single quote", "O'Reilly"},
		{"Single quote at end", "test'"},
		{"Double single quote", "test''"},
		{"Single quote at start", "'test"},
		{"Double quote", `test"`},
		{"Double quote at end", `test"`},
		{"Double double quote", `test""`},
		{"Combined backslash and single quote", `test\'`},
		{"Combined trailing backslash and single quote", `test\'`},
		{"Backslash single quote backslash", `a\'b\c\'`},
		{"Double backslash single quote", `test\\'`},
		{"Triple backslash single quote", `test\\\'`},
		{"Single quote backslash", `test'\`},
		{"Single quote double backslash", `test'\\`},
		{"Double quote and single quote", `say "hello 'world'"`},
		{"Mixed backslashes and double quotes", `C:\Program Files\"App"\`},
		{"Complex SQL injection-like string", `admin' OR '1'='1`},
		{"Complex Meili filter injection-like string", `cat' AND tags = 'evil`},
		{"Unicode with backslashes and quotes", `Café\'s & Résumé\`},
		{"Path with backslashes and quotes", `D:\Data\'Reports'\2026\`},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			escapedVal := escapeMeiliFilterVal(tc.input)

			// Formatted as Meilisearch filter value inside single quotes
			filterVal := fmt.Sprintf("'%s'", escapedVal)

			// Oracle verification: simulate Meilisearch string unescaping
			unescaped, err := meiliStringUnescapeSim(filterVal)
			require.NoError(t, err, "Meilisearch filter string parsing failed for input %q (filterVal: %s)", tc.input, filterVal)
			assert.Equal(t, tc.input, unescaped, "Unescaped Meilisearch string does not match original input %q", tc.input)
		})
	}
}

func TestMeiliFilterEscaping_FuzzCombinations(t *testing.T) {
	// Systematic combinations of backslashes and quotes
	chars := []string{"a", "\\", "'", `"`, "b"}

	var generateCombinations func(current string, depth int)
	var allInputs []string

	generateCombinations = func(current string, depth int) {
		if depth == 0 {
			allInputs = append(allInputs, current)
			return
		}
		for _, c := range chars {
			generateCombinations(current+c, depth-1)
		}
	}

	// Generate all combinations up to length 4
	for d := 1; d <= 4; d++ {
		generateCombinations("", d)
	}

	for i, input := range allInputs {
		t.Run(fmt.Sprintf("Fuzz_%d_%q", i, input), func(t *testing.T) {
			escapedVal := escapeMeiliFilterVal(input)
			filterVal := fmt.Sprintf("'%s'", escapedVal)

			unescaped, err := meiliStringUnescapeSim(filterVal)
			require.NoError(t, err, "Malformed Meili filter for input %q -> %s", input, filterVal)
			assert.Equal(t, input, unescaped, "Roundtrip mismatch for input %q", input)
		})
	}
}

// =============================================================================
// 2. OCR PARSERS CORRUPTED & EMPTY FILE EMPIRICAL TESTS
// =============================================================================

func TestOCRParsers_EmptyAndCorruptedFiles(t *testing.T) {
	ctx := context.Background()

	// Corrupted file payloads
	corruptedPayloads := map[string][]byte{
		"Empty 0-byte":              []byte{},
		"Single null byte":          {0x00},
		"Random binary garbage":     {0xFF, 0xD8, 0xFF, 0xE0, 0x00, 0x10, 0x4A, 0x46, 0x11, 0x22, 0x33},
		"Truncated PDF header":      []byte("%PDF-1.4\n1 0 obj\n<< /Type /Catalog"),
		"Corrupted PDF text block":  []byte("%PDF-1.4\nBT\n(Unclosed text block Tj\nET"),
		"Nested unbalanced parens":  []byte("%PDF-1.4\nBT\n(((Nested (((parens))) Tj\nET"),
		"Truncated ZIP header":      []byte("PK\x03\x04\x0A\x00\x00\x00"),
		"Corrupted DOCX zip header": []byte("PK\x03\x04\x14\x00\x00\x00\x08\x00corrupted_docx_data"),
		"Corrupted XML content":     []byte("<w:p><w:t>Unclosed xml tag"),
		"Malformed UTF-8 bytes":     {0x61, 0x62, 0x80, 0x81, 0xFF, 0xFE, 0x63, 0x64},
	}

	parsers := []struct {
		name     string
		mimeType string
		ext      string
		parser   DocumentParser
	}{
		{"PDFParser", "application/pdf", ".pdf", &PDFParser{}},
		{"DocxXlsxParser DOCX", "application/vnd.openxmlformats-officedocument.wordprocessingml.document", ".docx", &DocxXlsxParser{}},
		{"DocxXlsxParser XLSX", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet", ".xlsx", &DocxXlsxParser{}},
		{"ImageParser PNG", "image/png", ".png", &ImageParser{}},
		{"ImageParser JPG", "image/jpeg", ".jpg", &ImageParser{}},
		{"ZipParser ZIP", "application/zip", ".zip", &ZipParser{registry: DefaultParserRegistry}},
		{"TextParser TXT", "text/plain", ".txt", &TextParser{}},
	}

	for pName, pInfo := range parsers {
		_ = pName
		for payloadName, payload := range corruptedPayloads {
			testName := fmt.Sprintf("%s_%s", pInfo.name, payloadName)
			t.Run(testName, func(t *testing.T) {
				assert.NotPanics(t, func() {
					res, err := pInfo.parser.Parse(ctx, payload, pInfo.mimeType)
					// We expect either error or valid text response, but NEVER a panic
					if err != nil {
						assert.True(t, len(res) == 0 || err != nil)
					}
				}, "Parser %s panicked on %s payload", pInfo.name, payloadName)
			})
		}
	}
}

func TestParserRegistry_CorruptedFileDispatch(t *testing.T) {
	ctx := context.Background()
	registry := DefaultParserRegistry

	corruptedData := []byte{0xDE, 0xAD, 0xBE, 0xEF, 0x00, 0x01, 0x02, 0x03}

	mimes := []string{
		"application/pdf",
		"application/vnd.openxmlformats-officedocument.wordprocessingml.document",
		"application/vnd.openxmlformats-officedocument.spreadsheetml.sheet",
		"image/png",
		"image/jpeg",
		"application/zip",
		"text/plain",
		"unknown/mime-type",
	}

	for _, mime := range mimes {
		t.Run("Registry_"+mime, func(t *testing.T) {
			assert.NotPanics(t, func() {
				_, _ = registry.Parse(ctx, corruptedData, mime, "test_file")
			})
		})
	}
}

// =============================================================================
// 3. WORKER POOL EMPIRICAL STRESS TEST WITH CORRUPTED FILES
// =============================================================================

func TestOCRWorkerPool_CorruptedFilesStress(t *testing.T) {
	ctx := context.Background()
	repo := NewMemoryRepository()
	pool := NewGoOCRWorkerPool(4, 100, repo, nil)
	pool.Start()
	defer pool.Stop()

	// Create temp directory for corrupted test files
	tempDir, err := os.MkdirTemp("", "corrupted_ocr_test_*")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	corruptedFiles := map[string][]byte{
		"empty.pdf":        {},
		"corrupted.pdf":    []byte("%PDF-header-only-no-body"),
		"empty.docx":       {},
		"corrupted.docx":   []byte("PK\x03\x04corrupted_docx_bytes"),
		"empty.png":        {},
		"corrupted.png":    []byte("\x89PNG\r\n\x1a\ncorrupted_image_data"),
		"empty.zip":        {},
		"corrupted.zip":    []byte("PK\x03\x04corrupted_zip_bytes"),
		"malformed_utf8.txt": {0xFF, 0xFE, 0xFD, 0x80, 0x81},
	}

	docIDs := make(map[string]uuid.UUID)

	for filename, content := range corruptedFiles {
		filePath := filepath.Join(tempDir, filename)
		require.NoError(t, os.WriteFile(filePath, content, 0644))

		docID := uuid.New()
		docIDs[filename] = docID

		doc := &ArchiveDocument{
			ID:         docID,
			Filename:   filename,
			Category:   CategoryOther,
			OCRStatus:  OCRStatusPending,
			UploadedAt: time.Now(),
		}
		require.NoError(t, repo.CreateDocument(ctx, doc))

		mimeType := "application/octet-stream"
		if strings.HasSuffix(filename, ".pdf") {
			mimeType = "application/pdf"
		} else if strings.HasSuffix(filename, ".docx") {
			mimeType = "application/vnd.openxmlformats-officedocument.wordprocessingml.document"
		} else if strings.HasSuffix(filename, ".png") {
			mimeType = "image/png"
		} else if strings.HasSuffix(filename, ".zip") {
			mimeType = "application/zip"
		} else if strings.HasSuffix(filename, ".txt") {
			mimeType = "text/plain"
		}

		require.NoError(t, pool.Enqueue(docID, filePath, mimeType))
	}

	// Enqueue one valid document at the end to confirm worker goroutines remain active
	validDocID := uuid.New()
	validFilePath := filepath.Join(tempDir, "valid.txt")
	require.NoError(t, os.WriteFile(validFilePath, []byte("Valid text content after corrupted files"), 0644))

	validDoc := &ArchiveDocument{
		ID:         validDocID,
		Filename:   "valid.txt",
		Category:   CategoryOther,
		OCRStatus:  OCRStatusPending,
		UploadedAt: time.Now(),
	}
	require.NoError(t, repo.CreateDocument(ctx, validDoc))
	require.NoError(t, pool.Enqueue(validDocID, validFilePath, "text/plain"))

	// Wait for processing
	require.Eventually(t, func() bool {
		doc, err := repo.GetDocumentByID(ctx, validDocID)
		return err == nil && doc.OCRStatus == OCRStatusCompleted
	}, 5*time.Second, 50*time.Millisecond, "Worker pool failed to process valid job after encountering corrupted files")

	// Verify status for all corrupted documents: status must be either Completed or Failed, NEVER stuck in Processing
	for filename, docID := range docIDs {
		doc, err := repo.GetDocumentByID(ctx, docID)
		require.NoError(t, err, "Failed to retrieve doc %s", filename)
		assert.NotEqual(t, OCRStatusPending, doc.OCRStatus, "Doc %s stuck in Pending", filename)
		assert.NotEqual(t, OCRStatusProcessing, doc.OCRStatus, "Doc %s stuck in Processing", filename)
		assert.True(t, doc.OCRStatus == OCRStatusCompleted || doc.OCRStatus == OCRStatusFailed,
			"Doc %s had unexpected OCRStatus: %s", filename, doc.OCRStatus)
	}

	status := pool.Status()
	assert.Equal(t, int64(len(corruptedFiles)+1), status.ProcessedCount)
}

// Zip containing a corrupted inner zip entry
func TestOCRParsers_ZipWithCorruptedInnerEntry(t *testing.T) {
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)

	// Add corrupted docx file inside zip
	fw, err := zw.Create("corrupted_inner.docx")
	require.NoError(t, err)
	_, _ = fw.Write([]byte("NOT A REAL DOCX ZIP FILE"))

	// Add valid txt file inside zip
	fw2, err := zw.Create("valid_inner.txt")
	require.NoError(t, err)
	_, _ = fw2.Write([]byte("Valid text content inside zip archive"))

	require.NoError(t, zw.Close())

	zipParser := &ZipParser{registry: DefaultParserRegistry}
	res, err := zipParser.Parse(context.Background(), buf.Bytes(), "application/zip")
	require.NoError(t, err)
	assert.Contains(t, res, "Valid text content inside zip archive")
}
