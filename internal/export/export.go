package export

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/xuri/excelize/v2"
)

type Format string

const (
	FormatCSV   Format = "csv"
	FormatXLSX  Format = "xlsx"
	FormatFixed Format = "txt"
	FormatJSON  Format = "json"
)

// ExportOptions holds options for exporting query results
type ExportOptions struct {
	Format         Format
	FilePath       string
	IncludeHeaders bool
	Columns        []string
	Rows           [][]string
}

// Export executes the export according to the specified options
func Export(opts ExportOptions) (string, error) {
	if len(opts.Columns) == 0 && len(opts.Rows) == 0 {
		return "", fmt.Errorf("no data to export")
	}

	targetPath := opts.FilePath
	if targetPath == "" {
		targetPath = fmt.Sprintf("export.%s", opts.Format)
	}

	// Ensure file has correct extension
	ext := fmt.Sprintf(".%s", opts.Format)
	if !strings.HasSuffix(strings.ToLower(targetPath), ext) {
		targetPath += ext
	}

	switch opts.Format {
	case FormatCSV:
		return ExportCSV(targetPath, opts.Columns, opts.Rows, opts.IncludeHeaders)
	case FormatXLSX:
		return ExportXLSX(targetPath, opts.Columns, opts.Rows, opts.IncludeHeaders)
	case FormatFixed:
		return ExportFixedText(targetPath, opts.Columns, opts.Rows, opts.IncludeHeaders)
	case FormatJSON:
		return ExportJSON(targetPath, opts.Columns, opts.Rows)
	default:
		return "", fmt.Errorf("unsupported export format: %s", opts.Format)
	}
}

// ExportCSV exports data to a CSV file
func ExportCSV(targetPath string, columns []string, rows [][]string, includeHeaders bool) (string, error) {
	file, err := os.Create(targetPath)
	if err != nil {
		return "", fmt.Errorf("failed to create CSV file: %w", err)
	}
	defer file.Close()

	writer := csv.NewWriter(file)
	defer writer.Flush()

	if includeHeaders && len(columns) > 0 {
		if err := writer.Write(columns); err != nil {
			return "", err
		}
	}

	for _, row := range rows {
		if err := writer.Write(row); err != nil {
			return "", err
		}
	}

	return filepath.Abs(targetPath)
}

// ExportXLSX exports data to a formatted Microsoft Excel spreadsheet
func ExportXLSX(targetPath string, columns []string, rows [][]string, includeHeaders bool) (string, error) {
	f := excelize.NewFile()
	defer f.Close()

	sheetName := "QueryResults"
	index, err := f.NewSheet(sheetName)
	if err != nil {
		return "", fmt.Errorf("failed to create worksheet: %w", err)
	}
	f.SetActiveSheet(index)
	_ = f.DeleteSheet("Sheet1")

	// Header Style: Dark Blue background, White bold text
	headerStyle, err := f.NewStyle(&excelize.Style{
		Font: &excelize.Font{
			Bold:  true,
			Color: "#FFFFFF",
			Size:  11,
		},
		Fill: excelize.Fill{
			Type:    "pattern",
			Color:   []string{"#007ACC"},
			Pattern: 1,
		},
		Alignment: &excelize.Alignment{
			Horizontal: "center",
			Vertical:   "center",
		},
	})

	startRow := 1
	if includeHeaders && len(columns) > 0 {
		for c, col := range columns {
			cellName, _ := excelize.CoordinatesToCellName(c+1, startRow)
			_ = f.SetCellValue(sheetName, cellName, col)
		}

		firstCell, _ := excelize.CoordinatesToCellName(1, startRow)
		lastCell, _ := excelize.CoordinatesToCellName(len(columns), startRow)
		if headerStyle != 0 {
			_ = f.SetCellStyle(sheetName, firstCell, lastCell, headerStyle)
		}
		_ = f.SetRowHeight(sheetName, startRow, 24)
		startRow++
	}

	// Write data rows
	for r, row := range rows {
		currentRow := startRow + r
		for c, val := range row {
			cellName, _ := excelize.CoordinatesToCellName(c+1, currentRow)
			_ = f.SetCellValue(sheetName, cellName, val)
		}
	}

	// Auto-fit column widths
	for c, col := range columns {
		maxLen := len(col)
		for _, row := range rows {
			if c < len(row) && len(row[c]) > maxLen {
				maxLen = len(row[c])
			}
		}
		if maxLen > 50 {
			maxLen = 50
		}
		if maxLen < 10 {
			maxLen = 10
		}
		colLetter, _ := excelize.ColumnNumberToName(c + 1)
		_ = f.SetColWidth(sheetName, colLetter, colLetter, float64(maxLen+3))
	}

	if err := f.SaveAs(targetPath); err != nil {
		return "", fmt.Errorf("failed to save Excel file: %w", err)
	}

	return filepath.Abs(targetPath)
}

// ExportFixedText exports data as formatted fixed-width columns
func ExportFixedText(targetPath string, columns []string, rows [][]string, includeHeaders bool) (string, error) {
	file, err := os.Create(targetPath)
	if err != nil {
		return "", fmt.Errorf("failed to create TXT file: %w", err)
	}
	defer file.Close()

	// Calculate maximum width for each column
	numCols := len(columns)
	colWidths := make([]int, numCols)
	for i, col := range columns {
		colWidths[i] = len(col)
	}

	for _, row := range rows {
		for c, val := range row {
			if c < numCols && len(val) > colWidths[c] {
				colWidths[c] = len(val)
			}
		}
	}

	// Format line builder
	var sb strings.Builder

	// Header
	if includeHeaders && len(columns) > 0 {
		for i, col := range columns {
			sb.WriteString(padRight(col, colWidths[i]))
			if i < numCols-1 {
				sb.WriteString(" | ")
			}
		}
		sb.WriteString("\n")

		// Separator line
		for i := range columns {
			sb.WriteString(strings.Repeat("-", colWidths[i]))
			if i < numCols-1 {
				sb.WriteString("-+-")
			}
		}
		sb.WriteString("\n")
	}

	// Data rows
	for _, row := range rows {
		for c := 0; c < numCols; c++ {
			val := ""
			if c < len(row) {
				val = row[c]
			}
			sb.WriteString(padRight(val, colWidths[c]))
			if c < numCols-1 {
				sb.WriteString(" | ")
			}
		}
		sb.WriteString("\n")
	}

	// Summary footer
	sb.WriteString(fmt.Sprintf("\n(%d row(s) exported)\n", len(rows)))

	if _, err := file.WriteString(sb.String()); err != nil {
		return "", fmt.Errorf("failed to write to TXT file: %w", err)
	}

	return filepath.Abs(targetPath)
}

// ExportJSON exports data as structured JSON records
func ExportJSON(targetPath string, columns []string, rows [][]string) (string, error) {
	var records []map[string]any
	for _, row := range rows {
		rec := make(map[string]any)
		for c, colName := range columns {
			if c < len(row) {
				rec[colName] = row[c]
			}
		}
		records = append(records, rec)
	}

	data, err := json.MarshalIndent(records, "", "  ")
	if err != nil {
		return "", fmt.Errorf("failed to marshal JSON: %w", err)
	}

	if err := os.WriteFile(targetPath, data, 0644); err != nil {
		return "", fmt.Errorf("failed to write JSON file: %w", err)
	}

	return filepath.Abs(targetPath)
}

func padRight(str string, length int) string {
	if len(str) >= length {
		return str
	}
	return str + strings.Repeat(" ", length-len(str))
}
