package explorer

import (
	"context"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"dbterm/internal/config"
	"dbterm/internal/db"
)

type mockDriver struct {
	db.Driver
}

func (m *mockDriver) FetchDatabases(ctx context.Context) ([]string, error) {
	return []string{"TestDB"}, nil
}

func (m *mockDriver) FetchTables(ctx context.Context, database string) ([]db.TableInfo, error) {
	return []db.TableInfo{
		{Catalog: "TestDB", Schema: "sales", Name: "Customers", Type: db.NodeTable},
	}, nil
}

func (m *mockDriver) FetchViews(ctx context.Context, database string) ([]db.TableInfo, error) {
	return nil, nil
}

func (m *mockDriver) FetchProcedures(ctx context.Context, database string) ([]string, error) {
	return nil, nil
}

func (m *mockDriver) GenerateSelectQuery(schema, table string, limit int) string {
	return "SELECT TOP 100 * FROM [sales].[Customers];"
}

func TestExplorerContextMenu(t *testing.T) {
	expl := Model{
		ActiveDriver: &mockDriver{},
		ActiveDB:     "TestDB",
		Width:        35,
		Height:       20,
		Visible:      true,
		Flattened: []*db.TreeNode{
			{ID: "tbl_1", Name: "sales.Customers", Type: db.NodeTable, Catalog: "TestDB", Schema: "sales"},
		},
		Cursor: 0,
	}

	// 1. Trigger Right Click on item
	mouseMsg := tea.MouseMsg{
		X:      5,
		Y:      1,
		Button: tea.MouseButtonRight,
		Action: tea.MouseActionPress,
	}

	expl, _ = expl.HandleMouse(mouseMsg, 1)
	if !expl.ContextMenuOpen {
		t.Fatalf("Expected Context Menu to be open on right click")
	}

	if len(expl.MenuItems) < 3 {
		t.Errorf("Expected menu items for table node, got %d", len(expl.MenuItems))
	}

	// Verify Context Menu view rendering
	view := expl.View()
	if !strings.Contains(view, "CONTEXT MENU") || !strings.Contains(view, "Script as SELECT") {
		t.Errorf("Expected context menu in rendered view")
	}

	// 2. Execute Action (Enter on SELECT)
	cmd := expl.executeContextMenuAction("select")
	if cmd == nil {
		t.Fatalf("Expected tea.Cmd to be returned for select action")
	}
	msg := cmd()
	scriptMsg, ok := msg.(ScriptTableMsg)
	if !ok || !strings.Contains(scriptMsg.Query, "SELECT") {
		t.Errorf("Expected ScriptTableMsg with SELECT query")
	}
}

func TestExplorerMultiServerTree(t *testing.T) {
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
			},
			{
				ID:       "prod-mssql",
				Name:     "Prod MS SQL",
				Group:    "Production / Enterprise",
				Driver:   "mssql",
				Host:     "prod.lan",
				Port:     1433,
				Database: "ERP",
			},
		},
	}

	activeProf := &cfg.Connections[0]
	expl := New(cfg, activeProf, &mockDriver{}, "TestDB")
	expl.SetSize(40, 30)

	if len(expl.RootNodes) != 2 {
		t.Fatalf("Expected 2 group nodes in explorer, got %d", len(expl.RootNodes))
	}

	view := expl.View()
	if !strings.Contains(view, "Local / Docker") || !strings.Contains(view, "Production / Enterprise") {
		t.Errorf("Expected group names in explorer view, got:\n%s", view)
	}

	if !strings.Contains(view, "Local MS SQL") {
		t.Errorf("Expected server name in explorer view, got:\n%s", view)
	}

	// 2. Switch to Files Tab with '2'
	expl, _ = expl.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'2'}})
	if expl.ActiveTab != TabFiles {
		t.Errorf("Expected active tab to be TabFiles (1), got %d", expl.ActiveTab)
	}

	filesView := expl.View()
	if !strings.Contains(filesView, "Files [2]") {
		t.Errorf("Expected Files tab header in view, got:\n%s", filesView)
	}
}
