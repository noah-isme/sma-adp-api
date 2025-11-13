package export

import (
	"bytes"
	"fmt"
	"strings"

	"github.com/jung-kurt/gofpdf"
)

// PDFExporter renders datasets into a basic tabular PDF.
type PDFExporter struct{}

// NewPDFExporter constructs a PDF exporter.
func NewPDFExporter() *PDFExporter {
	return &PDFExporter{}
}

// Render creates a PDF document with an optional title and table body.
func (e *PDFExporter) Render(data Dataset, title string) ([]byte, error) {
	if len(data.Headers) == 0 {
		return nil, fmt.Errorf("pdf requires at least one header")
	}
	pdf := gofpdf.New("P", "mm", "A4", "")
	pdf.SetMargins(10, 15, 10)
	pdf.AddPage()

	if title != "" {
		pdf.SetFont("Arial", "B", 14)
		pdf.CellFormat(0, 10, strings.ToUpper(title), "", 1, "C", false, 0, "")
		pdf.Ln(5)
	}

	pdf.SetFont("Arial", "B", 10)
	colWidth := 190.0 / float64(len(data.Headers))
	for _, header := range data.Headers {
		pdf.CellFormat(colWidth, 8, header, "1", 0, "C", false, 0, "")
	}
	pdf.Ln(-1)

	pdf.SetFont("Arial", "", 9)
	for _, row := range data.Rows {
		for _, header := range data.Headers {
			value := row[header]
			pdf.CellFormat(colWidth, 7, value, "1", 0, "", false, 0, "")
		}
		pdf.Ln(-1)
	}

	buf := &bytes.Buffer{}
	if err := pdf.Output(buf); err != nil {
		return nil, fmt.Errorf("render pdf: %w", err)
	}
	return buf.Bytes(), nil
}
