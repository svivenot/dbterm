package dialogs

import (
	"fmt"
	"path/filepath"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"

	"dbterm/internal/export"
)

// ShowExportDialog presents the multi-format export dialog
func ShowExportDialog(w fyne.Window, columns []string, rows [][]string, onExportDone func(path string)) {
	if len(columns) == 0 || len(rows) == 0 {
		dialog.ShowInformation("Export", "No query results to export.", w)
		return
	}

	formatSelect := widget.NewSelect([]string{
		"Excel Workbook (.xlsx)",
		"CSV (Comma Separated)",
		"CSV (Semicolon Separated - French Excel)",
		"JSON Array",
		"Markdown Table",
		"HTML Table",
		"TSV (Tab Separated)",
	}, nil)
	formatSelect.SetSelected("Excel Workbook (.xlsx)")

	sheetEntry := widget.NewEntry()
	sheetEntry.SetText("Query Results")

	filenameEntry := widget.NewEntry()
	filenameEntry.SetText("query_export")

	headersCheck := widget.NewCheck("Include column headers", nil)
	headersCheck.SetChecked(true)

	statusLabel := widget.NewLabel(fmt.Sprintf("%d rows and %d columns ready to export", len(rows), len(columns)))
	statusLabel.TextStyle = fyne.TextStyle{Italic: true}

	form := widget.NewForm(
		widget.NewFormItem("Format", formatSelect),
		widget.NewFormItem("File Base Name", filenameEntry),
		widget.NewFormItem("Sheet Name (Excel)", sheetEntry),
		widget.NewFormItem("Headers", headersCheck),
	)

	content := container.NewVBox(
		statusLabel,
		form,
	)

	dialog.ShowCustomConfirm("Export Query Results", "Export Now", "Cancel", content, func(ok bool) {
		if !ok {
			return
		}

		var fmtType export.Format
		switch formatSelect.Selected {
		case "Excel Workbook (.xlsx)":
			fmtType = export.FormatXLSX
		case "CSV (Comma Separated)":
			fmtType = export.FormatCSV
		case "CSV (Semicolon Separated - French Excel)":
			fmtType = export.FormatCSV
		case "JSON Array":
			fmtType = export.FormatJSON
		case "TSV (Tab Separated)":
			fmtType = export.FormatFixed
		default:
			fmtType = export.FormatXLSX
		}

		targetPath := filenameEntry.Text
		if targetPath == "" {
			targetPath = "query_export"
		}

		opts := export.ExportOptions{
			Format:         fmtType,
			FilePath:       targetPath,
			IncludeHeaders: headersCheck.Checked,
			Columns:        columns,
			Rows:           rows,
		}

		outPath, err := export.Export(opts)
		if err != nil {
			dialog.ShowError(fmt.Errorf("Export failed: %w", err), w)
			return
		}

		if onExportDone != nil {
			onExportDone(outPath)
		}
		dialog.ShowInformation("Export Succeeded", fmt.Sprintf("✓ Successfully exported %d rows to:\n%s", len(rows), filepath.Base(outPath)), w)
	}, w)
}
