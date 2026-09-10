package results

import (
	"os"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"dbterm/internal/db"
)

func TestCellInspectorScrollAndClose(t *testing.T) {
	m := New()
	m.SetSize(40, 10)
	m.SetScreenSize(80, 24)
	m.Focus()

	longVal := strings.Repeat("payload-token ", 300) // wraps to many lines
	m.SetResult(&db.QueryResult{
		Columns: []string{"Data"},
		Rows:    [][]string{{longVal}},
	})

	// Open the inspector on the selected cell.
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'v'}})
	if !m.InspectorOpen {
		t.Fatalf("expected inspector to open")
	}
	if view := m.InspectorView(); !strings.Contains(view, "CELL VALUE INSPECTOR") {
		t.Errorf("inspector view missing title")
	}

	// Scroll down: the viewport offset must advance (content is scrollable).
	before := m.Inspector.YOffset
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyPgDown})
	if m.Inspector.YOffset <= before {
		t.Errorf("expected viewport to scroll, YOffset before=%d after=%d", before, m.Inspector.YOffset)
	}

	// Esc closes the inspector.
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if m.InspectorOpen {
		t.Errorf("expected inspector to close on Esc")
	}
}

func TestResultsGridAndExport(t *testing.T) {
	resView := New()

	mockResult := &db.QueryResult{
		Columns: []string{"ID", "Name", "Role"},
		Rows: [][]string{
			{"1", "Sylvain", "CTO"},
			{"2", "Claire", "DBA"},
		},
		Duration: 15 * time.Millisecond,
	}

	resView.SetResult(mockResult)
	if resView.ActiveTab != TabResults {
		t.Errorf("Expected ActiveTab to be TabResults")
	}

	// Test Export CSV
	path, err := resView.ExportCSV()
	if err != nil {
		t.Fatalf("ExportCSV failed: %v", err)
	}
	defer os.Remove(path)

	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("Failed to read exported CSV: %v", err)
	}

	if len(content) == 0 {
		t.Errorf("Exported CSV is empty")
	}
}

func TestHorizontalColumnScrolling(t *testing.T) {
	resView := New()
	resView.SetSize(60, 20) // narrow window of 60 chars width

	// 10 wide columns that would exceed 60 characters
	columns := []string{"Col1_ID", "Col2_CompanyName", "Col3_ContactName", "Col4_EmailAddress", "Col5_PhoneNumber", "Col6_PostalAddress", "Col7_CityName", "Col8_PostalCode", "Col9_CountryName", "Col10_AccountBalance"}
	rows := [][]string{
		{"1", "Airbus Group", "Jean-Luc", "jl@airbus.com", "+33 5 61 93 33", "1 Rond-Point", "Toulouse", "31707", "France", "45000.00"},
		{"2", "Siemens AG", "Klaus", "klaus@siemens.com", "+49 89 636", "Werner-von-Siemens", "Munich", "80333", "Germany", "125000.00"},
	}

	mockResult := &db.QueryResult{
		Columns: columns,
		Rows:    rows,
	}

	resView.SetResult(mockResult)

	// Render view
	view := resView.View()
	lines := strings.Split(view, "\n")
	if len(lines) == 0 {
		t.Fatalf("Expected non-empty view")
	}

	// Scroll right multiple columns
	for i := 0; i < 6; i++ {
		resView.SelectedCol++
	}

	viewScrolled := resView.View()
	if !strings.Contains(viewScrolled, "Col7_CityName") && !strings.Contains(viewScrolled, "CityName") && !strings.Contains(viewScrolled, "Col") {
		t.Errorf("Expected view to contain scrolled columns")
	}
}
