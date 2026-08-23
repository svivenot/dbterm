package editor

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestEditorTabs(t *testing.T) {
	ed := New("SELECT 1;")
	if len(ed.Tabs) != 1 {
		t.Fatalf("Expected 1 initial tab, got %d", len(ed.Tabs))
	}

	if ed.GetCurrentQuery() != "SELECT 1;" {
		t.Errorf("Expected 'SELECT 1;', got '%s'", ed.GetCurrentQuery())
	}

	// Add new tab
	ed.NewTab("Query-2.sql", "SELECT 2;")
	if len(ed.Tabs) != 2 {
		t.Fatalf("Expected 2 tabs, got %d", len(ed.Tabs))
	}
	if ed.ActiveIndex != 1 {
		t.Errorf("Expected active index 1, got %d", ed.ActiveIndex)
	}
	if ed.GetCurrentQuery() != "SELECT 2;" {
		t.Errorf("Expected 'SELECT 2;', got '%s'", ed.GetCurrentQuery())
	}

	// Close tab
	ed.CloseActiveTab()
	if len(ed.Tabs) != 1 {
		t.Fatalf("Expected 1 tab after closing, got %d", len(ed.Tabs))
	}
	if ed.GetCurrentQuery() != "SELECT 1;" {
		t.Errorf("Expected 'SELECT 1;', got '%s'", ed.GetCurrentQuery())
	}
}

func TestEditorSelectionAndClipboard(t *testing.T) {
	ed := New("SELECT * FROM Customers WHERE City = 'Paris';")

	// Test Select All
	ed.SelectAll()
	if !ed.HasSelection() {
		t.Fatalf("Expected HasSelection to be true after SelectAll")
	}

	selected := ed.GetSelectedText()
	if selected != "SELECT * FROM Customers WHERE City = 'Paris';" {
		t.Errorf("Unexpected selected text: '%s'", selected)
	}

	// Test Partial Query Execution when Selection is active
	if ed.GetCurrentQuery() != "SELECT * FROM Customers WHERE City = 'Paris';" {
		t.Errorf("Expected GetCurrentQuery to return selected text")
	}

	// Clear selection
	ed.ClearSelection()
	if ed.HasSelection() {
		t.Errorf("Expected HasSelection to be false after ClearSelection")
	}

	// Test Shift+Left Arrow selection
	ed.handleShiftArrow("left")
	ed.handleShiftArrow("left")
	ed.handleShiftArrow("left")
	if !ed.HasSelection() {
		t.Fatalf("Expected HasSelection to be true after Shift+Left")
	}

	// Test Visual Selection View
	view := ed.View()
	if !strings.Contains(view, "[SELECTED]") || !strings.Contains(view, "Selection") {
		t.Errorf("Expected selection indicators in View(), got:\n%s", view)
	}
}

func TestShiftDownSelection(t *testing.T) {
	ed := New("SELECT 1;\nSELECT 2;\nSELECT 3;")
	// Reset cursor to (0, 0)
	ed.Tabs[0].Textarea.SetValue("SELECT 1;\nSELECT 2;\nSELECT 3;")
	// Cursor is at (0, 0) initially when SetValue is called or ta.CursorUp/home
	ed.Tabs[0].Textarea.CursorUp()
	ed.Tabs[0].Textarea.CursorUp()

	ed.handleShiftArrow("down")
	if !ed.HasSelection() {
		t.Errorf("Expected HasSelection to be true after Shift+Down, start=(%d, %d), end=(%d, %d)",
			ed.Tabs[0].SelStartLine, ed.Tabs[0].SelStartCol,
			ed.Tabs[0].SelEndLine, ed.Tabs[0].SelEndCol,
		)
	}
	selText := ed.GetSelectedText()
	t.Logf("Selected text after Shift+Down: '%s'", selText)
}

func TestVisualMode(t *testing.T) {
	ed := New("SELECT 1;\nSELECT 2;\nSELECT 3;")
	ed.Tabs[0].Textarea.CursorUp()
	ed.Tabs[0].Textarea.CursorUp()

	// Toggle visual mode with ctrl+space
	ed.Tabs[0].VisualMode = true
	ed.handleShiftArrow("down")
	if !ed.HasSelection() {
		t.Fatalf("Expected selection in Visual Mode")
	}

	view := ed.View()
	if !strings.Contains(view, "[VISUAL]") {
		t.Errorf("Expected [VISUAL] badge in tab header, got:\n%s", view)
	}
}

func TestMouseDragSelection(t *testing.T) {
	ed := New("SELECT * FROM sales.Customers;\nSELECT * FROM inventory.Products;")

	// 1. Mouse Press on line 1, col 0 (relX=6, relY=1)
	pressMsg := tea.MouseMsg{
		X:      6,
		Y:      1,
		Button: tea.MouseButtonLeft,
		Action: tea.MouseActionPress,
	}
	ed, _ = ed.HandleMouse(pressMsg, 6, 1)

	// 2. Mouse Drag to line 2, col 10 (relX=16, relY=2)
	motionMsg := tea.MouseMsg{
		X:      16,
		Y:      2,
		Button: tea.MouseButtonLeft,
		Action: tea.MouseActionMotion,
	}
	ed, _ = ed.HandleMouse(motionMsg, 16, 2)

	// 3. Mouse Release
	releaseMsg := tea.MouseMsg{
		X:      16,
		Y:      2,
		Button: tea.MouseButtonLeft,
		Action: tea.MouseActionRelease,
	}
	ed, _ = ed.HandleMouse(releaseMsg, 16, 2)

	if !ed.HasSelection() {
		t.Fatalf("Expected HasSelection to be true after mouse drag")
	}

	selText := ed.GetSelectedText()
	if len(selText) == 0 || !strings.Contains(selText, "Customers") {
		t.Errorf("Expected selected text to contain 'Customers', got '%s'", selText)
	}
}

func TestOpenFileAndSave(t *testing.T) {
	ed := New("")
	ed.OpenFile("/tmp/test_report.sql", "SELECT * FROM reports;")

	if len(ed.Tabs) != 1 {
		t.Fatalf("Expected 1 tab, got %d", len(ed.Tabs))
	}
	if ed.Tabs[0].Title != "test_report.sql" || ed.Tabs[0].FilePath != "/tmp/test_report.sql" {
		t.Errorf("Unexpected tab state: %+v", ed.Tabs[0])
	}

	// Test SaveModal
	modal := NewSaveModal()
	modal.Open("SELECT 123;", "my_script.sql")

	if !modal.Active || !strings.Contains(modal.InputPath, "my_script.sql") {
		t.Errorf("Expected SaveModal to be active with default path, got: %s", modal.InputPath)
	}

	view := modal.View()
	if !strings.Contains(view, "SAVE SQL QUERY TO FILE") {
		t.Errorf("Expected modal title in view output, got:\n%s", view)
	}
}
