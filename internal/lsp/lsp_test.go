package lsp

import (
	"encoding/json"
	"strings"
	"testing"
)

func setupTestCatalog() *Catalog {
	cat := NewCatalog()
	cat.CurrentDatabase = "SalesDB"
	cat.Dialect = "mssql"
	cat.Schemas = []string{"sales", "inventory", "hr"}

	// Table 1: sales.Customers
	cat.AllTables = append(cat.AllTables, TableMetadata{
		Database: "SalesDB",
		Schema:   "sales",
		Name:     "Customers",
		IsView:   false,
		Columns: []ColumnMetadata{
			{Name: "CustomerID", DataType: "nvarchar(10)", IsPrimaryKey: true, IsNullable: false},
			{Name: "CompanyName", DataType: "nvarchar(100)", IsPrimaryKey: false, IsNullable: false},
			{Name: "ContactName", DataType: "nvarchar(100)", IsPrimaryKey: false, IsNullable: true},
			{Name: "City", DataType: "nvarchar(50)", IsPrimaryKey: false, IsNullable: false},
			{Name: "Country", DataType: "nvarchar(50)", IsPrimaryKey: false, IsNullable: false},
		},
		PrimaryKeys: []string{"CustomerID"},
	})

	// Table 2: sales.Orders
	cat.AllTables = append(cat.AllTables, TableMetadata{
		Database: "SalesDB",
		Schema:   "sales",
		Name:     "Orders",
		IsView:   false,
		Columns: []ColumnMetadata{
			{Name: "OrderID", DataType: "int", IsPrimaryKey: true, IsNullable: false},
			{Name: "CustomerID", DataType: "nvarchar(10)", IsPrimaryKey: false, IsNullable: false},
			{Name: "OrderDate", DataType: "datetime2", IsPrimaryKey: false, IsNullable: false},
			{Name: "Freight", DataType: "decimal(10,2)", IsPrimaryKey: false, IsNullable: false},
		},
		PrimaryKeys: []string{"OrderID"},
	})

	// Table 3: inventory.Products
	cat.AllTables = append(cat.AllTables, TableMetadata{
		Database: "SalesDB",
		Schema:   "inventory",
		Name:     "Products",
		IsView:   false,
		Columns: []ColumnMetadata{
			{Name: "ProductID", DataType: "int", IsPrimaryKey: true, IsNullable: false},
			{Name: "ProductName", DataType: "nvarchar(100)", IsPrimaryKey: false, IsNullable: false},
			{Name: "UnitPrice", DataType: "decimal(10,2)", IsPrimaryKey: false, IsNullable: false},
		},
		PrimaryKeys: []string{"ProductID"},
	})

	// Populate indices
	for i := range cat.AllTables {
		t := &cat.AllTables[i]
		lowerName := strings.ToLower(t.Name)
		cat.Tables[lowerName] = t
		cat.ColumnsByTable[lowerName] = t.Columns
		if t.Schema != "" {
			fullKey := strings.ToLower(t.Schema + "." + t.Name)
			cat.Tables[fullKey] = t
			cat.ColumnsByTable[fullKey] = t.Columns
		}
		for _, col := range t.Columns {
			colLower := strings.ToLower(col.Name)
			cat.AllColumnMap[colLower] = append(cat.AllColumnMap[colLower], ColumnLocation{
				Database: t.Database,
				Schema:   t.Schema,
				Table:    t.Name,
				IsView:   t.IsView,
				Column:   col,
			})
		}
	}

	return cat
}

func TestCompleter_KeywordsAndSnippets(t *testing.T) {
	cat := setupTestCatalog()
	completer := NewCompleter(cat)
	items := completer.Complete("", 0, 0)
	if len(items) == 0 {
		t.Fatalf("expected keywords/snippets, got 0")
	}
}

func TestCompleter_General(t *testing.T) {
	cat := setupTestCatalog()
	completer := NewCompleter(cat)

	items := completer.Complete("SEL", 0, 3)
	if len(items) == 0 {
		t.Fatalf("expected completions for 'SEL', got 0")
	}

	foundSelect := false
	for _, item := range items {
		if item.Label == "SELECT" {
			foundSelect = true
			break
		}
	}
	if !foundSelect {
		t.Errorf("expected 'SELECT' in completions")
	}
}

func TestCompleter_FromClause(t *testing.T) {
	cat := setupTestCatalog()
	completer := NewCompleter(cat)

	items := completer.Complete("SELECT * FROM ", 0, 14)
	if len(items) == 0 {
		t.Fatalf("expected completions after 'FROM ', got 0")
	}

	foundCustomers := false
	foundSalesSchema := false
	for _, item := range items {
		if item.Label == "sales.Customers" || item.Label == "Customers" {
			foundCustomers = true
		}
		if item.Label == "sales" && item.Kind == CompletionItemKindModule {
			foundSalesSchema = true
		}
	}

	if !foundCustomers {
		t.Errorf("expected 'sales.Customers' in FROM completions")
	}
	if !foundSalesSchema {
		t.Errorf("expected 'sales' schema in FROM completions")
	}
}

func TestCompleter_SchemaDot(t *testing.T) {
	cat := setupTestCatalog()
	completer := NewCompleter(cat)

	// "SELECT * FROM sales."
	items := completer.Complete("SELECT * FROM sales.", 0, 20)
	if len(items) == 0 {
		t.Fatalf("expected completions after 'sales.', got 0")
	}

	foundCustomers := false
	foundOrders := false
	for _, item := range items {
		if item.Label == "Customers" {
			foundCustomers = true
		}
		if item.Label == "Orders" {
			foundOrders = true
		}
	}

	if !foundCustomers || !foundOrders {
		t.Errorf("expected Customers and Orders in 'sales.' dot completions")
	}
}

func TestCompleter_AliasDot(t *testing.T) {
	cat := setupTestCatalog()
	completer := NewCompleter(cat)

	// "SELECT c. FROM sales.Customers c"
	sql := "SELECT c. FROM sales.Customers c"
	items := completer.Complete(sql, 0, 9)
	if len(items) == 0 {
		t.Fatalf("expected column completions for alias 'c.', got 0")
	}

	foundCustomerID := false
	foundCompanyName := false
	for _, item := range items {
		if item.Label == "CustomerID" {
			foundCustomerID = true
			if !strings.Contains(item.Detail, "PK") && !strings.Contains(item.Detail, "🔑") {
				t.Errorf("expected PK indicator for CustomerID, got %s", item.Detail)
			}
		}
		if item.Label == "CompanyName" {
			foundCompanyName = true
		}
	}

	if !foundCustomerID || !foundCompanyName {
		t.Errorf("expected CustomerID and CompanyName in 'c.' dot completions")
	}
}

func TestCompleter_JoinOnPredicate(t *testing.T) {
	cat := setupTestCatalog()
	completer := NewCompleter(cat)

	// "SELECT * FROM sales.Customers c JOIN sales.Orders o ON "
	sql := "SELECT * FROM sales.Customers c JOIN sales.Orders o ON "
	items := completer.Complete(sql, 0, len(sql))
	if len(items) == 0 {
		t.Fatalf("expected completions after 'ON ', got 0")
	}

	foundJoinPredicate := false
	for _, item := range items {
		if strings.Contains(item.InsertText, "c.CustomerID = o.CustomerID") || strings.Contains(item.InsertText, "o.CustomerID = c.CustomerID") {
			foundJoinPredicate = true
			break
		}
	}

	if !foundJoinPredicate {
		t.Errorf("expected auto join predicate 'c.CustomerID = o.CustomerID' in JOIN ON completions")
	}
}

func TestCompleter_Hover(t *testing.T) {
	cat := setupTestCatalog()
	completer := NewCompleter(cat)

	sql := "SELECT CustomerID FROM sales.Customers"
	hover := completer.Hover(sql, 0, 10) // on CustomerID
	if hover == nil {
		t.Fatalf("expected hover for CustomerID, got nil")
	}
	if !strings.Contains(hover.Contents.Value, "CustomerID") {
		t.Errorf("hover should mention CustomerID, got: %s", hover.Contents.Value)
	}

	hoverTbl := completer.Hover(sql, 0, 30) // on Customers
	if hoverTbl == nil {
		t.Fatalf("expected hover for Customers, got nil")
	}
	if !strings.Contains(hoverTbl.Contents.Value, "Customers") || !strings.Contains(hoverTbl.Contents.Value, "nvarchar(10)") {
		t.Errorf("hover should show table columns, got: %s", hoverTbl.Contents.Value)
	}
}

func TestServer_JSONRPC(t *testing.T) {
	server := NewServer()
	server.catalog = setupTestCatalog()
	server.completer = NewCompleter(server.catalog)

	initReq := `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}`
	respBytes, err := server.HandleMessage([]byte(initReq))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var resp ResponseMessage
	if err := json.Unmarshal(respBytes, &resp); err != nil {
		t.Fatalf("failed to unmarshal init response: %v", err)
	}
	if resp.ID != float64(1) {
		t.Errorf("expected id 1, got %v", resp.ID)
	}

	// Open document
	openReq := `{"jsonrpc":"2.0","method":"textDocument/didOpen","params":{"textDocument":{"uri":"file:///query.sql","languageId":"sql","version":1,"text":"SELECT * FROM sales."}}}`
	_, _ = server.HandleMessage([]byte(openReq))

	// Request completions
	compReq := `{"jsonrpc":"2.0","id":2,"method":"textDocument/completion","params":{"textDocument":{"uri":"file:///query.sql"},"position":{"line":0,"character":20}}}`
	compRespBytes, err := server.HandleMessage([]byte(compReq))
	if err != nil {
		t.Fatalf("completion request failed: %v", err)
	}

	var compResp struct {
		JSONRPC string         `json:"jsonrpc"`
		ID      int            `json:"id"`
		Result  CompletionList `json:"result"`
	}
	if err := json.Unmarshal(compRespBytes, &compResp); err != nil {
		t.Fatalf("failed to parse completion response: %v", err)
	}

	if len(compResp.Result.Items) == 0 {
		t.Fatalf("expected items in completion list, got 0")
	}
}
