package export

import (
	"os"
	"strings"
	"testing"
)

func TestExportFormats(t *testing.T) {
	columns := []string{"ID", "CompanyName", "Balance"}
	rows := [][]string{
		{"1", "Airbus Group SAS", "45000.00"},
		{"2", "Siemens AG", "125000.00"},
		{"3", "TotalEnergies SE", "8900.00"},
	}

	// 1. Test CSV Export
	csvPath, err := Export(ExportOptions{
		Format:         FormatCSV,
		FilePath:       "test_export.csv",
		IncludeHeaders: true,
		Columns:        columns,
		Rows:           rows,
	})
	if err != nil {
		t.Fatalf("CSV export failed: %v", err)
	}
	defer os.Remove(csvPath)
	csvData, _ := os.ReadFile(csvPath)
	if !strings.Contains(string(csvData), "Airbus Group SAS") {
		t.Errorf("CSV missing content")
	}

	// 2. Test XLSX Export
	xlsxPath, err := Export(ExportOptions{
		Format:         FormatXLSX,
		FilePath:       "test_export.xlsx",
		IncludeHeaders: true,
		Columns:        columns,
		Rows:           rows,
	})
	if err != nil {
		t.Fatalf("XLSX export failed: %v", err)
	}
	defer os.Remove(xlsxPath)
	if stat, err := os.Stat(xlsxPath); err != nil || stat.Size() == 0 {
		t.Errorf("XLSX file is empty or not created")
	}

	// 3. Test Fixed-Width TXT Export
	txtPath, err := Export(ExportOptions{
		Format:         FormatFixed,
		FilePath:       "test_export.txt",
		IncludeHeaders: true,
		Columns:        columns,
		Rows:           rows,
	})
	if err != nil {
		t.Fatalf("Fixed TXT export failed: %v", err)
	}
	defer os.Remove(txtPath)
	txtData, _ := os.ReadFile(txtPath)
	if !strings.Contains(string(txtData), "Airbus Group SAS") || !strings.Contains(string(txtData), "-+-") {
		t.Errorf("TXT missing table formatting or content")
	}

	// 4. Test JSON Export
	jsonPath, err := Export(ExportOptions{
		Format:         FormatJSON,
		FilePath:       "test_export.json",
		Columns:        columns,
		Rows:           rows,
	})
	if err != nil {
		t.Fatalf("JSON export failed: %v", err)
	}
	defer os.Remove(jsonPath)
	jsonData, _ := os.ReadFile(jsonPath)
	if !strings.Contains(string(jsonData), "\"CompanyName\": \"Airbus Group SAS\"") {
		t.Errorf("JSON missing structured field content")
	}
}
