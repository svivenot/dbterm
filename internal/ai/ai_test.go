package ai

import (
	"context"
	"strings"
	"testing"

	"dbterm/internal/config"
)

func TestExtractSQLAndExplanation(t *testing.T) {
	// Test 1: Markdown code block with explanation
	raw := "Here is the query to find top customers:\n\n```sql\nSELECT CustomerID, FirstName, LastName\nFROM sales.Customers\nWHERE Active = 1\nORDER BY CreatedAt DESC;\n```\n\nThis query filters active customers and sorts them by creation date."

	sql, exp := ExtractSQLAndExplanation(raw)
	if !strings.Contains(sql, "SELECT CustomerID") || !strings.Contains(sql, "FROM sales.Customers") {
		t.Errorf("Expected extracted SQL to contain query, got: '%s'", sql)
	}
	if !strings.Contains(exp, "Here is the query") && !strings.Contains(exp, "This query filters") {
		t.Errorf("Expected explanation to contain descriptive text, got: '%s'", exp)
	}

	// Test 2: Raw SQL without markdown
	raw2 := "SELECT * FROM sales.Orders WHERE TotalAmount > 100;"
	sql2, _ := ExtractSQLAndExplanation(raw2)
	if sql2 != raw2 {
		t.Errorf("Expected raw SQL to be extracted, got: '%s'", sql2)
	}
}

func TestBuildSystemPrompt(t *testing.T) {
	schema := "CREATE TABLE sales.Customers (\n  CustomerID int PRIMARY KEY,\n  Email nvarchar(100)\n);\n"
	sysPrompt := BuildSystemPrompt("T-SQL (MS SQL Server)", schema)

	if !strings.Contains(sysPrompt, "T-SQL (MS SQL Server)") {
		t.Errorf("Expected dialect in system prompt")
	}
	if !strings.Contains(sysPrompt, "sales.Customers") {
		t.Errorf("Expected schema in system prompt")
	}
}

func TestBuildUserPrompt(t *testing.T) {
	// Generate mode
	p1 := BuildUserPrompt(AIModeGenerate, "Show all users", "", "")
	if !strings.Contains(p1, "Show all users") {
		t.Errorf("Expected user prompt in output: %s", p1)
	}

	// Fix Error mode
	p2 := BuildUserPrompt(AIModeFixError, "", "SELECT * FROM MissingTable", "Invalid object name 'MissingTable'")
	if !strings.Contains(p2, "MissingTable") || !strings.Contains(p2, "Database Error Message") {
		t.Errorf("Expected error details in output: %s", p2)
	}
}

func TestModelInfoAndPaths(t *testing.T) {
	dir, err := GetModelsDir()
	if err != nil {
		t.Fatalf("GetModelsDir failed: %v", err)
	}
	if len(dir) == 0 {
		t.Errorf("Expected non-empty model directory")
	}

	path, err := GetModelFilePath(DefaultModel)
	if err != nil {
		t.Fatalf("GetModelFilePath failed: %v", err)
	}
	if !strings.HasSuffix(path, ".gguf") {
		t.Errorf("Expected path to end in .gguf, got: %s", path)
	}
}

func TestEngineFallback(t *testing.T) {
	cfg := config.AIConfig{
		Enabled:   true,
		ModelName: DefaultModel.ID,
		Endpoint:  "http://127.0.0.1:9999", // Unreachable port to trigger fallback
	}
	engine := NewEngine(cfg)
	if !engine.IsAvailable() {
		t.Errorf("Expected engine to be available")
	}

	req := AIRequest{
		Mode:       AIModeGenerate,
		UserPrompt: "donne moi la liste des clients",
	}

	res, err := engine.Generate(context.Background(), req)
	if err != nil {
		t.Fatalf("Expected fallback generation without crash, got err: %v", err)
	}
	if !strings.Contains(res.GeneratedSQL, "sales.Customers") {
		t.Errorf("Expected fallback SQL to match customers table, got: %s", res.GeneratedSQL)
	}
}

func TestEmbeddedSQLGenerator(t *testing.T) {
	gen := NewEmbeddedSQLGenerator()

	schema := &SchemaSummary{
		Database: "SalesDB",
		Dialect:  "T-SQL (MS SQL Server)",
		DDLContext: `CREATE TABLE sales.Customers (
  CustomerID bigint PRIMARY KEY,
  FirstName nvarchar(50),
  LastName nvarchar(50),
  Email nvarchar(100)
);

CREATE TABLE sales.Orders (
  OrderID bigint PRIMARY KEY,
  CustomerID bigint,
  OrderDate datetime2,
  TotalAmount decimal(18,2)
);
`,
	}

	// 1. Text-to-SQL with Join
	sql1, exp1 := gen.Generate(AIRequest{
		Mode:       AIModeGenerate,
		UserPrompt: "top 5 clients avec leurs commandes",
	}, schema)

	if !strings.Contains(sql1, "sales.Customers") || !strings.Contains(sql1, "sales.Orders") || !strings.Contains(sql1, "JOIN") {
		t.Errorf("Expected multi-table JOIN query, got: %s", sql1)
	}
	if exp1 == "" {
		t.Errorf("Expected explanation for generated query")
	}

	// 2. Fix Error
	sql2, exp2 := gen.Generate(AIRequest{
		Mode:         AIModeFixError,
		CurrentSQL:   "SELECT * FROM sales.Customers;\nGO;\nCREATE TABLE Test (ID INT);",
		ErrorMessage: "Incorrect syntax near 'GO'.",
	}, schema)

	if strings.Contains(sql2, "GO;") {
		t.Errorf("Expected fixed SQL to remove semicolon after GO, got: %s", sql2)
	}
	if !strings.Contains(exp2, "GO") {
		t.Errorf("Expected explanation mentioning GO, got: %s", exp2)
	}

	// 4. Seniority / Tenure calculation (ancienneté moyenne)
	schemaHR := &SchemaSummary{
		Database: "SalesDB",
		Dialect:  "T-SQL (MS SQL Server)",
		DDLContext: `CREATE TABLE hr.Employees (
  EmployeeID bigint PRIMARY KEY,
  FirstName nvarchar(50),
  LastName nvarchar(50),
  DepartmentID int,
  Salary decimal(18,2),
  HireDate date
);

CREATE TABLE hr.Departments (
  DepartmentID int PRIMARY KEY,
  DepartmentName nvarchar(50)
);
`,
	}

	sql4, exp4 := gen.Generate(AIRequest{
		Mode:       AIModeGenerate,
		UserPrompt: "calculer l'ancienneté moyenne des employés",
	}, schemaHR)

	if !strings.Contains(sql4, "AVG(DATEDIFF(") || !strings.Contains(sql4, "HireDate") || !strings.Contains(sql4, "hr.Employees") {
		t.Errorf("Expected DATEDIFF seniority calculation in SQL, got: %s", sql4)
	}
	if !strings.Contains(exp4, "ancienneté") && !strings.Contains(exp4, "HireDate") {
		t.Errorf("Expected explanation to mention ancienneté, got: %s", exp4)
	}

	// 5. Seniority by Department
	sql5, _ := gen.Generate(AIRequest{
		Mode:       AIModeGenerate,
		UserPrompt: "ancienneté moyenne par département",
	}, schemaHR)

	if !strings.Contains(sql5, "GROUP BY") || !strings.Contains(sql5, "DepartmentName") || !strings.Contains(sql5, "AVG(DATEDIFF(") {
		t.Errorf("Expected Group By DepartmentName with DATEDIFF, got: %s", sql5)
	}
}
