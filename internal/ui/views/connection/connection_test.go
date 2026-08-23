package connection

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"dbterm/internal/config"
)

func TestConnectionTreeHierarchy(t *testing.T) {
	cfg := &config.Config{
		Connections: []config.ConnectionProfile{
			{
				ID:       "local-mssql",
				Name:     "Local MS SQL",
				Group:    "Local / Docker",
				Driver:   "mssql",
				Host:     "localhost",
				Port:     1433,
				Database: "SalesDB",
				User:     "sa",
			},
			{
				ID:       "local-pg",
				Name:     "Local Postgres",
				Group:    "Local / Docker",
				Driver:   "postgres",
				Host:     "localhost",
				Port:     5432,
				Database: "postgres",
				User:     "postgres",
			},
			{
				ID:       "prod-mssql",
				Name:     "Prod MS SQL Enterprise",
				Group:    "Production / Enterprise",
				Driver:   "mssql",
				Host:     "prod.corp.lan",
				Port:     1433,
				Database: "ERP",
				User:     "admin",
			},
		},
	}

	m := New(cfg, "")
	m.Open()

	if len(m.RootNodes) != 2 {
		t.Fatalf("Expected 2 group nodes, got %d", len(m.RootNodes))
	}

	// Total visible items: 2 folders + 3 connections = 5 items
	if len(m.VisibleNodes) != 5 {
		t.Fatalf("Expected 5 visible nodes initially, got %d", len(m.VisibleNodes))
	}

	// 1. Collapse first folder
	m.Cursor = 0
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeySpace})
	if m.RootNodes[0].Expanded {
		t.Errorf("Expected first group to be collapsed")
	}

	// Now visible items should be 1 collapsed folder (1) + 1 expanded folder with 1 child (2) = 3 items
	if len(m.VisibleNodes) != 3 {
		t.Errorf("Expected 3 visible nodes after collapse, got %d", len(m.VisibleNodes))
	}

	// 2. Expand back
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeySpace})
	if !m.RootNodes[0].Expanded {
		t.Errorf("Expected first group to be expanded")
	}

	// 3. Test Filter
	m.Filter = "postgres"
	m.rebuildTree()
	if len(m.VisibleNodes) != 2 { // 1 folder + 1 matching postgres server
		t.Errorf("Expected 2 visible nodes with filter 'postgres', got %d", len(m.VisibleNodes))
	}

	// 4. Test View rendering
	m.Filter = ""
	m.rebuildTree()
	view := m.View()
	if !strings.Contains(view, "REGISTERED DATABASE SERVERS") || !strings.Contains(view, "Local / Docker") {
		t.Errorf("Expected tree headers in view output, got:\n%s", view)
	}
}

func TestAddEditDeleteConnection(t *testing.T) {
	cfg := &config.Config{
		Connections: []config.ConnectionProfile{
			{
				ID:       "test-mssql",
				Name:     "Test MS SQL",
				Group:    "Staging",
				Driver:   "mssql",
				Host:     "localhost",
				Port:     1433,
				Database: "TestDB",
				User:     "sa",
			},
		},
	}

	m := New(cfg, "")
	m.Open()

	// 1. Test 'a' to open Add Connection form
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}})
	if !m.FormModal.Active {
		t.Fatalf("Expected FormModal to be active after pressing 'a'")
	}
	formView := m.View()
	if !strings.Contains(formView, "ADD SQL SERVER CONNECTION") {
		t.Errorf("Expected Add Connection title in form view, got:\n%s", formView)
	}

	// 2. Build profile and save
	m.FormModal.Name = "Oracle Cloud"
	m.FormModal.DriverIdx = 2 // Oracle
	m.FormModal.Port = "1521"
	m.FormModal.Database = "ORCL"
	m.FormModal.User = "system"
	m.FormModal.FocusedField = FieldSaveButton

	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if m.FormModal.Active {
		t.Fatalf("Expected FormModal to close after Enter on Save button")
	}
	if len(cfg.Connections) != 2 {
		t.Fatalf("Expected 2 connections in config after add, got %d", len(cfg.Connections))
	}

	// 3. Test 'e' to open Edit Connection form
	m.Cursor = 1 // Point to first connection
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'e'}})
	if !m.FormModal.Active || !m.FormModal.IsEdit {
		t.Fatalf("Expected FormModal to be active in Edit mode after pressing 'e'")
	}
	m.FormModal.Close()

	// 4. Test 'd' to delete connection
	m.Cursor = 1 // Point to connection node
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'d'}})
	if m.DeleteConfirmID == "" {
		t.Fatalf("Expected DeleteConfirmID to be set after pressing 'd'")
	}

	// Confirm delete with 'y'
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'y'}})
	if len(cfg.Connections) != 1 {
		t.Fatalf("Expected 1 connection remaining after delete, got %d", len(cfg.Connections))
	}
}
