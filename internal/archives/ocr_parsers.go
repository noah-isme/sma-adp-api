package archives

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/xml"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"
	"unicode/utf8"
)

const (
	maxZipFiles      = 50
	maxZipTotalBytes = 20 * 1024 * 1024 // 20 MB
)

// DocumentParser interface defines multi-format document extraction contract.
type DocumentParser interface {
	Parse(ctx context.Context, data []byte, mimeType string) (string, error)
}

// ParserRegistry maps MIME types and file extensions to specialized DocumentParser implementations.
type ParserRegistry struct {
	mu            sync.RWMutex
	mimeMap       map[string]DocumentParser
	extMap        map[string]DocumentParser
	defaultParser DocumentParser
}

// NewParserRegistry initializes and registers standard parsers.
func NewParserRegistry() *ParserRegistry {
	r := &ParserRegistry{
		mimeMap:       make(map[string]DocumentParser),
		extMap:        make(map[string]DocumentParser),
		defaultParser: &TextParser{},
	}

	pdfParser := &PDFParser{}
	r.Register(pdfParser, []string{"application/pdf"}, []string{".pdf"})

	docxXlsxParser := &DocxXlsxParser{}
	r.Register(docxXlsxParser, []string{
		"application/vnd.openxmlformats-officedocument.wordprocessingml.document",
		"application/vnd.openxmlformats-officedocument.spreadsheetml.sheet",
	}, []string{".docx", ".xlsx"})

	imgParser := &ImageParser{}
	r.Register(imgParser, []string{
		"image/png", "image/jpeg", "image/jpg", "image/tiff",
	}, []string{".png", ".jpg", ".jpeg", ".tiff"})

	zipParser := &ZipParser{registry: r}
	r.Register(zipParser, []string{"application/zip"}, []string{".zip"})

	textParser := &TextParser{}
	r.Register(textParser, []string{
		"text/plain", "text/csv", "text/html", "application/json", "application/xml",
	}, []string{".txt", ".csv", ".html", ".json", ".xml", ".md", ".log"})

	return r
}

var DefaultParserRegistry = NewParserRegistry()

// Register assigns a parser to specified MIME types and file extensions.
func (r *ParserRegistry) Register(parser DocumentParser, mimeTypes []string, extensions []string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	for _, m := range mimeTypes {
		r.mimeMap[strings.ToLower(m)] = parser
	}
	for _, ext := range extensions {
		r.extMap[strings.ToLower(ext)] = parser
	}
}

// GetParser resolves the best matching parser for a MIME type or filename.
func (r *ParserRegistry) GetParser(mimeType string, filename string) DocumentParser {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if mimeType != "" {
		if parser, ok := r.mimeMap[strings.ToLower(mimeType)]; ok {
			return parser
		}
		if strings.HasPrefix(strings.ToLower(mimeType), "image/") {
			if imgP, ok := r.mimeMap["image/png"]; ok {
				return imgP
			}
		}
	}

	ext := strings.ToLower(filepath.Ext(filename))
	if ext != "" {
		if parser, ok := r.extMap[ext]; ok {
			return parser
		}
	}

	return r.defaultParser
}

// Parse extracts text from document bytes using the matching registered parser.
func (r *ParserRegistry) Parse(ctx context.Context, data []byte, mimeType string, filename string) (string, error) {
	if len(data) == 0 {
		return "", nil
	}
	parser := r.GetParser(mimeType, filename)
	return parser.Parse(ctx, data, mimeType)
}

// ExtractTextFromFile reads a file from disk and performs format-specific text extraction.
func ExtractTextFromFile(filePath string, mimeType string, filename string) (string, error) {
	if filePath == "" {
		return "", nil
	}
	data, err := os.ReadFile(filePath)
	if err != nil {
		return "", fmt.Errorf("read file for OCR: %w", err)
	}
	return ExtractTextFromBytes(data, mimeType, filename)
}

// ExtractTextFromBytes inspects MIME type/extension and dispatches data to the appropriate parser via DefaultParserRegistry.
func ExtractTextFromBytes(data []byte, mimeType string, filename string) (string, error) {
	return DefaultParserRegistry.Parse(context.Background(), data, mimeType, filename)
}

// extractTextFromBytes provides backward compatibility for (data, mimeType) calls.
func extractTextFromBytes(data []byte, mimeType string) string {
	text, _ := ExtractTextFromBytes(data, mimeType, "")
	return text
}

// PDFParser extracts text streams bounded by BT and ET tags and Tj/TJ text operators.
type PDFParser struct{}

func (p *PDFParser) Parse(ctx context.Context, data []byte, mimeType string) (string, error) {
	if len(data) == 0 {
		return "", nil
	}
	var builder strings.Builder
	content := string(data)

	inTextBlock := false
	for _, line := range strings.Split(content, "\n") {
		lineTrim := strings.TrimSpace(line)
		if lineTrim == "BT" {
			inTextBlock = true
			continue
		}
		if lineTrim == "ET" {
			inTextBlock = false
			continue
		}
		if inTextBlock {
			start := 0
			for {
				openIdx := strings.Index(lineTrim[start:], "(")
				if openIdx == -1 {
					break
				}
				closeIdx := strings.Index(lineTrim[start+openIdx:], ")")
				if closeIdx == -1 {
					break
				}
				extracted := lineTrim[start+openIdx+1 : start+openIdx+closeIdx]
				if len(extracted) > 0 {
					builder.WriteString(extracted)
					builder.WriteString(" ")
				}
				start += openIdx + closeIdx + 1
			}
		}
	}

	res := strings.TrimSpace(builder.String())
	if len(res) > 0 {
		return res, nil
	}

	textParser := &TextParser{}
	return textParser.Parse(ctx, data, mimeType)
}

// DocxXlsxParser unpacks OpenXML containers (DOCX/XLSX) and extracts <w:t> and <t> XML tags.
type DocxXlsxParser struct{}

func (p *DocxXlsxParser) Parse(ctx context.Context, data []byte, mimeType string) (string, error) {
	if len(data) == 0 {
		return "", nil
	}

	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return "", fmt.Errorf("read OpenXML zip: %w", err)
	}

	var builder strings.Builder
	isWord := strings.Contains(mimeType, "wordprocessingml")
	isExcel := strings.Contains(mimeType, "spreadsheetml")

	for _, f := range zr.File {
		if (isWord || mimeType == "") && f.Name == "word/document.xml" {
			rc, err := f.Open()
			if err != nil {
				continue
			}
			xmlData, err := io.ReadAll(rc)
			_ = rc.Close()
			if err != nil {
				continue
			}
			text, _ := parseXMLTags(xmlData, "t")
			if text != "" {
				builder.WriteString(text)
				builder.WriteString(" ")
			}
		} else if isExcel || mimeType == "" {
			if f.Name == "xl/sharedStrings.xml" || strings.HasPrefix(f.Name, "xl/worksheets/") {
				rc, err := f.Open()
				if err != nil {
					continue
				}
				xmlData, err := io.ReadAll(rc)
				_ = rc.Close()
				if err != nil {
					continue
				}
				text, _ := parseXMLTags(xmlData, "t")
				if text != "" {
					builder.WriteString(text)
					builder.WriteString(" ")
				}
			}
		}
	}

	if builder.Len() == 0 {
		// Fallback check if it's docx/xlsx without explicit mime match
		for _, f := range zr.File {
			if f.Name == "word/document.xml" || f.Name == "xl/sharedStrings.xml" {
				rc, err := f.Open()
				if err != nil {
					continue
				}
				xmlData, err := io.ReadAll(rc)
				_ = rc.Close()
				if err == nil {
					text, _ := parseXMLTags(xmlData, "t")
					if text != "" {
						builder.WriteString(text)
						builder.WriteString(" ")
					}
				}
			}
		}
	}

	return strings.TrimSpace(builder.String()), nil
}

func parseXMLTags(xmlData []byte, tagName string) (string, error) {
	decoder := xml.NewDecoder(bytes.NewReader(xmlData))
	var builder strings.Builder

	for {
		tok, err := decoder.Token()
		if err != nil {
			break
		}
		switch elem := tok.(type) {
		case xml.StartElement:
			if elem.Name.Local == tagName {
				var charData string
				if err := decoder.DecodeElement(&charData, &elem); err == nil {
					builder.WriteString(charData)
					builder.WriteString(" ")
				}
			}
		}
	}
	return strings.TrimSpace(builder.String()), nil
}

// ImageParser extracts metadata and printable text tokens for PNG/JPG/TIFF files.
type ImageParser struct{}

func (p *ImageParser) Parse(ctx context.Context, data []byte, mimeType string) (string, error) {
	if len(data) == 0 {
		return "", nil
	}

	tesseractPath, err := exec.LookPath("tesseract")
	if err == nil && tesseractPath != "" {
		tmpFile, err := os.CreateTemp("", "ocr_*_image")
		if err == nil {
			tmpPath := tmpFile.Name()
			_, writeErr := tmpFile.Write(data)
			_ = tmpFile.Close()
			defer os.Remove(tmpPath)

			if writeErr == nil {
				execCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
				defer cancel()

				cmd := exec.CommandContext(execCtx, tesseractPath, tmpPath, "stdout", "--dpi", "300", "-l", "eng+ind")
				out, cmdErr := cmd.Output()
				if cmdErr == nil && len(out) > 0 {
					return strings.TrimSpace(string(out)), nil
				}
			}
		}
	}

	textParser := &TextParser{}
	plain, _ := textParser.Parse(ctx, data, mimeType)
	if len(plain) > 20 {
		return plain, nil
	}
	return fmt.Sprintf("Image Document (%s)", mimeType), nil
}

// ZipParser recursively unpacks ZIP archives and aggregates inner contents.
type ZipParser struct {
	registry *ParserRegistry
}

func (p *ZipParser) Parse(ctx context.Context, data []byte, mimeType string) (string, error) {
	if len(data) == 0 {
		return "", nil
	}

	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return "", fmt.Errorf("read zip: %w", err)
	}

	registry := p.registry
	if registry == nil {
		registry = DefaultParserRegistry
	}

	var builder strings.Builder
	fileCount := 0
	var totalBytes int64

	for _, f := range zr.File {
		if f.FileInfo().IsDir() {
			continue
		}
		if fileCount >= maxZipFiles || totalBytes >= maxZipTotalBytes {
			builder.WriteString("\n--- Zip contents truncated (max limit reached) ---")
			break
		}

		ext := strings.ToLower(filepath.Ext(f.Name))
		if ext == ".exe" || ext == ".dll" || ext == ".so" || ext == ".bin" || ext == ".iso" || ext == ".zip" {
			continue
		}

		remainingBytes := maxZipTotalBytes - totalBytes
		if remainingBytes <= 0 {
			builder.WriteString("\n--- Zip contents truncated (max limit reached) ---")
			break
		}

		rc, err := f.Open()
		if err != nil {
			continue
		}
		limitReader := io.LimitReader(rc, remainingBytes+1)
		fileData, err := io.ReadAll(limitReader)
		_ = rc.Close()
		if err != nil {
			continue
		}

		truncated := false
		if int64(len(fileData)) > remainingBytes {
			fileData = fileData[:remainingBytes]
			truncated = true
		}

		fileCount++
		totalBytes += int64(len(fileData))

		subText, _ := registry.Parse(ctx, fileData, "", f.Name)
		if strings.TrimSpace(subText) != "" {
			builder.WriteString(fmt.Sprintf("\n--- File: %s ---\n%s\n", f.Name, subText))
		}

		if truncated {
			builder.WriteString("\n--- Zip contents truncated (max limit reached) ---")
			break
		}
	}

	return strings.TrimSpace(builder.String()), nil
}

// TextParser is the default text extraction parser supporting UTF-8 rune preservation.
type TextParser struct{}

func (p *TextParser) Parse(ctx context.Context, data []byte, mimeType string) (string, error) {
	if len(data) == 0 {
		return "", nil
	}
	if utf8.Valid(data) {
		var builder strings.Builder
		for _, r := range string(data) {
			if (r >= 32 && r != 127) || r == '\n' || r == '\r' || r == '\t' {
				builder.WriteRune(r)
			}
		}
		return strings.TrimSpace(builder.String()), nil
	}

	var builder strings.Builder
	for _, b := range data {
		if (b >= 32 && b <= 126) || b == '\n' || b == '\r' || b == '\t' {
			builder.WriteByte(b)
		}
	}
	return strings.TrimSpace(builder.String()), nil
}
