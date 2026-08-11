package export

import (
	"bytes"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/jung-kurt/gofpdf"
)

// PDFExporter renders datasets into a basic tabular PDF.
type PDFExporter struct{}

// NewPDFExporter constructs a PDF exporter.
func NewPDFExporter() *PDFExporter {
	return &PDFExporter{}
}

// Render creates a PDF document with an optional title and table body.
// template can be used to select different PDF layouts (e.g., "simple", "detailed", "landscape").
func (e *PDFExporter) Render(data Dataset, title string, template *string) ([]byte, error) {
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

	// Apply template-specific formatting
	switch deref(template) {
	case "landscape":
		pdf = gofpdf.New("L", "mm", "A4", "")
		pdf.SetMargins(15, 10, 15)
		pdf.AddPage()
	case "detailed":
		pdf.SetFont("Arial", "B", 12)
		pdf.CellFormat(0, 10, strings.ToUpper(title), "", 1, "L", false, 0, "")
		pdf.Ln(8)
		pdf.SetFont("Arial", "I", 9)
		pdf.CellFormat(0, 6, "Generated: "+time.Now().UTC().Format("2006-01-02 15:04:05 UTC"), "", 1, "L", false, 0, "")
		pdf.Ln(5)
	default: // "simple" or empty
		pdf.SetFont("Arial", "B", 10)
	}

	colWidth := getPageWidth(pdf) / float64(len(data.Headers))
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

// TimetableCell represents a single cell in the timetable grid.
type TimetableCell struct {
	SubjectName string `json:"subject_name"`
	TeacherName string `json:"teacher_name"`
	Room        string `json:"room,omitempty"`
}

// GridCell describes subject, teacher, and room for compatibility.
type GridCell struct {
	Subject string `json:"subject"`
	Teacher string `json:"teacher"`
	Room    string `json:"room,omitempty"`
}

// TimetableGrid describes input for GenerateTimetablePDF.
type TimetableGrid struct {
	Title       string              `json:"title"`
	ClassID     string              `json:"class_id"`
	TermID      string              `json:"term_id"`
	Days        []string            `json:"days"`
	TimeSlots   []string            `json:"time_slots"`
	GridEntries map[string]GridCell `json:"grid_entries"`
}

// GenerateTimetablePDF generates PDF bytes from a TimetableGrid structure.
func GenerateTimetablePDF(grid TimetableGrid) ([]byte, error) {
	exporter := NewPDFExporter()
	gridData := make(map[int]map[int]TimetableCell)
	for k, cell := range grid.GridEntries {
		parts := strings.Split(k, "-")
		if len(parts) == 2 {
			dayIdx := 0
			for i, d := range grid.Days {
				if strings.EqualFold(d, parts[0]) {
					dayIdx = i
					break
				}
			}
			slotIdx := 0
			if num, err := strconv.Atoi(parts[1]); err == nil {
				slotIdx = num
			}
			if gridData[slotIdx] == nil {
				gridData[slotIdx] = make(map[int]TimetableCell)
			}
			gridData[slotIdx][dayIdx] = TimetableCell{
				SubjectName: cell.Subject,
				TeacherName: cell.Teacher,
				Room:        cell.Room,
			}
		}
	}
	subtitle := fmt.Sprintf("Class: %s, Term: %s", grid.ClassID, grid.TermID)
	return exporter.RenderTimetableGrid(grid.Title, subtitle, grid.Days, grid.TimeSlots, gridData)
}

// RenderTimetableGrid renders an A4 Landscape timetable grid layout.
func (e *PDFExporter) RenderTimetableGrid(title, subtitle string, days []string, timeSlots []string, grid map[int]map[int]TimetableCell) ([]byte, error) {
	pdf := gofpdf.New("L", "mm", "A4", "")
	pdf.SetMargins(10, 10, 10)
	pdf.AddPage()

	if title != "" {
		pdf.SetFont("Arial", "B", 14)
		pdf.CellFormat(0, 8, strings.ToUpper(title), "", 1, "C", false, 0, "")
	}
	if subtitle != "" {
		pdf.SetFont("Arial", "", 10)
		pdf.CellFormat(0, 6, subtitle, "", 1, "C", false, 0, "")
	}
	pdf.Ln(3)

	numDays := len(days)
	if numDays == 0 {
		days = []string{"Senin", "Selasa", "Rabu", "Kamis", "Jumat", "Sabtu"}
		numDays = len(days)
	}

	timeColWidth := 37.0
	dayColWidth := (277.0 - timeColWidth) / float64(numDays)

	// Header row
	pdf.SetFont("Arial", "B", 10)
	pdf.SetFillColor(230, 235, 245)
	pdf.CellFormat(timeColWidth, 9, "Jam / Waktu", "1", 0, "C", true, 0, "")
	for _, day := range days {
		pdf.CellFormat(dayColWidth, 9, day, "1", 0, "C", true, 0, "")
	}
	pdf.Ln(-1)

	// Time slot rows
	for slotIdx, slotName := range timeSlots {
		slotMap := grid[slotIdx]
		if slotMap == nil {
			slotMap = grid[slotIdx+1]
		}

		rowHeight := 16.0
		startY := pdf.GetY()

		pdf.SetFont("Arial", "B", 8)
		pdf.SetFillColor(245, 247, 250)
		pdf.CellFormat(timeColWidth, rowHeight, slotName, "1", 0, "C", true, 0, "")

		for dayIdx := range days {
			cellData, exists := slotMap[dayIdx]
			if !exists {
				cellData, exists = slotMap[dayIdx+1]
			}

			x := pdf.GetX()
			y := pdf.GetY()

			pdf.Rect(x, y, dayColWidth, rowHeight, "D")

			if exists && (cellData.SubjectName != "" || cellData.TeacherName != "" || cellData.Room != "") {
				pdf.SetXY(x+1, y+2)
				pdf.SetFont("Arial", "B", 8)
				pdf.CellFormat(dayColWidth-2, 4, truncateString(cellData.SubjectName, 22), "", 1, "C", false, 0, "")

				if cellData.TeacherName != "" {
					pdf.SetX(x + 1)
					pdf.SetFont("Arial", "", 7)
					pdf.CellFormat(dayColWidth-2, 3.5, truncateString(cellData.TeacherName, 24), "", 1, "C", false, 0, "")
				}

				if cellData.Room != "" {
					pdf.SetX(x + 1)
					pdf.SetFont("Arial", "I", 7)
					pdf.CellFormat(dayColWidth-2, 3.5, truncateString(cellData.Room, 20), "", 1, "C", false, 0, "")
				}
			}

			pdf.SetXY(x+dayColWidth, y)
		}
		pdf.SetY(startY + rowHeight)
	}

	pdf.Ln(4)
	pdf.SetFont("Arial", "I", 8)
	pdf.CellFormat(0, 5, fmt.Sprintf("Dicetak pada: %s", time.Now().Format("02 Jan 2006 15:04")), "", 0, "R", false, 0, "")

	buf := &bytes.Buffer{}
	if err := pdf.Output(buf); err != nil {
		return nil, fmt.Errorf("render timetable grid pdf: %w", err)
	}
	return buf.Bytes(), nil
}

func truncateString(s string, maxLen int) string {
	runes := []rune(s)
	if len(runes) > maxLen {
		return string(runes[:maxLen-2]) + ".."
	}
	return s
}

func getPageWidth(pdf *gofpdf.Fpdf) float64 {
	_, pageWidth := pdf.GetPageSize()
	left, _, right, _ := pdf.GetMargins()
	return pageWidth - left - right
}

func deref(ptr *string) string {
	if ptr == nil {
		return ""
	}
	return *ptr
}

