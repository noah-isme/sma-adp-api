package export

import (
	"bytes"
	"encoding/csv"
	"fmt"
)

// Dataset defines tabular export content.
type Dataset struct {
	Headers []string
	Rows    []map[string]string
}

// CSVExporter renders Dataset records into CSV bytes.
type CSVExporter struct{}

// NewCSVExporter builds a CSV exporter.
func NewCSVExporter() *CSVExporter {
	return &CSVExporter{}
}

// Render produces CSV encoded bytes for the dataset.
func (e *CSVExporter) Render(data Dataset) ([]byte, error) {
	if len(data.Headers) == 0 {
		return nil, fmt.Errorf("csv requires at least one header")
	}
	buf := &bytes.Buffer{}
	writer := csv.NewWriter(buf)
	if err := writer.Write(data.Headers); err != nil {
		return nil, fmt.Errorf("write csv headers: %w", err)
	}
	for _, row := range data.Rows {
		record := make([]string, len(data.Headers))
		for i, header := range data.Headers {
			record[i] = row[header]
		}
		if err := writer.Write(record); err != nil {
			return nil, fmt.Errorf("write csv row: %w", err)
		}
	}
	writer.Flush()
	if err := writer.Error(); err != nil {
		return nil, fmt.Errorf("flush csv: %w", err)
	}
	return buf.Bytes(), nil
}
