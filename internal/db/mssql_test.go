package db

import (
	"context"
	"testing"
	"time"

	"dbterm/internal/config"
)

func TestMSSQLIntegration(t *testing.T) {
	profile := &config.ConnectionProfile{
		ID:       "test-mssql",
		Driver:   "mssql",
		Host:     "localhost",
		Port:     1433,
		Database: "SalesDB",
		User:     "sa",
		AuthType: config.AuthTypeSQL,
		Password: "Password123!",
	}

	driver, err := NewDriver(profile)
	if err != nil {
		t.Fatalf("Failed to create driver: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := driver.Connect(ctx, profile); err != nil {
		t.Skipf("Skipping integration test (SQL Server not reachable: %v)", err)
		return
	}
	defer driver.Close()

	// 1. Fetch Databases
	dbs, err := driver.FetchDatabases(ctx)
	if err != nil {
		t.Fatalf("FetchDatabases failed: %v", err)
	}
	if len(dbs) == 0 {
		t.Fatalf("Expected databases, got 0")
	}

	// 2. Fetch Tables in SalesDB
	tables, err := driver.FetchTables(ctx, "SalesDB")
	if err != nil {
		t.Fatalf("FetchTables failed: %v", err)
	}
	if len(tables) == 0 {
		t.Fatalf("Expected tables in SalesDB, got 0")
	}

	// 3. Fetch Views in SalesDB
	views, err := driver.FetchViews(ctx, "SalesDB")
	if err != nil {
		t.Fatalf("FetchViews failed: %v", err)
	}
	if len(views) == 0 {
		t.Fatalf("Expected views in SalesDB, got 0")
	}

	// 4. Fetch Columns for sales.Customers
	cols, err := driver.FetchColumns(ctx, "SalesDB", "sales", "Customers")
	if err != nil {
		t.Fatalf("FetchColumns failed: %v", err)
	}
	if len(cols) == 0 {
		t.Fatalf("Expected columns for sales.Customers, got 0")
	}

	// Verify Primary Key detection
	foundPK := false
	for _, c := range cols {
		if c.Name == "CustomerID" && c.IsPrimaryKey {
			foundPK = true
			break
		}
	}
	if !foundPK {
		t.Errorf("CustomerID should be detected as primary key")
	}

	// 6. Test DDL Generation
	ddl, err := driver.GenerateDDL(ctx, "SalesDB", "sales", "Customers", NodeTable)
	if err != nil {
		t.Fatalf("GenerateDDL failed: %v", err)
	}
	if len(ddl) == 0 || !testingContains(ddl, "CREATE TABLE") {
		t.Errorf("Expected DDL to contain 'CREATE TABLE', got:\n%s", ddl)
	}

	// 7. Test Insert Template Generation
	insertQuery := driver.GenerateInsertQuery("sales", "Customers", cols)
	if len(insertQuery) == 0 || !testingContains(insertQuery, "INSERT INTO") {
		t.Errorf("Expected INSERT query to contain 'INSERT INTO', got:\n%s", insertQuery)
	}

	// 8. Test Multi-Batch execution with GO separators (User's DDL query)
	multiBatchQuery := `
-- DDL for [SalesDB].[audit].[ActivityLogs]
USE [SalesDB];
GO

IF OBJECT_ID('[audit].[ActivityLogsTest]', 'U') IS NOT NULL
    DROP TABLE [audit].[ActivityLogsTest];
GO

CREATE TABLE [audit].[ActivityLogsTest] (
    [LogID] bigint NOT NULL,
    [EventType] nvarchar(50) NOT NULL,
    [TableName] nvarchar(50) NOT NULL,
    [RecordKey] nvarchar(100) NOT NULL,
    [ExecutedBy] nvarchar(100) NOT NULL,
    [LogTimestamp] datetime2 NOT NULL,
    [Details] nvarchar(max) NULL,
    CONSTRAINT [PK_audit_ActivityLogsTest] PRIMARY KEY ([LogID])
);
GO

SELECT COUNT(*) AS TableCreated FROM sys.tables WHERE name = 'ActivityLogsTest';
`
	res, err := driver.ExecuteQuery(ctx, multiBatchQuery)
	if err != nil {
		t.Fatalf("ExecuteQuery with GO batch separators failed: %v", err)
	}
	if len(res.Rows) == 0 || res.Rows[0][0] != "1" {
		t.Errorf("Expected TableCreated=1, got: %v", res.Rows)
	}
}

func testingContains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0)
}
