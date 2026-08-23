package exportview

import (
	"strings"
	"testing"
)

func TestExportModal(t *testing.T) {
	modal := New()
	modal.SetSize(80, 24)

	columns := []string{"ID", "Name"}
	rows := [][]string{{"1", "Test"}}

	modal.Open(columns, rows)
	if !modal.Active {
		t.Fatalf("Expected modal to be active")
	}

	view := modal.View()
	if !strings.Contains(view, "EXPORT") {
		t.Errorf("Expected modal title in view")
	}
	if !strings.Contains(view, "xlsx") || !strings.Contains(view, "txt") {
		t.Errorf("Expected format options in view")
	}

	// Change format to XLSX
	modal.SelectedFormat = 1 // FormatXLSX
	modal.updateFilenameExtension()
	if !strings.HasSuffix(modal.FilenameInput.Value(), ".xlsx") {
		t.Errorf("Expected filename to have .xlsx extension, got '%s'", modal.FilenameInput.Value())
	}

	// Change format to TXT
	modal.SelectedFormat = 2 // FormatFixed
	modal.updateFilenameExtension()
	if !strings.HasSuffix(modal.FilenameInput.Value(), ".txt") {
		t.Errorf("Expected filename to have .txt extension, got '%s'", modal.FilenameInput.Value())
	}
}
