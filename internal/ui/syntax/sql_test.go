package syntax

import (
	"strings"
	"testing"
)

func TestSQLHighlighter(t *testing.T) {
	sql := `
-- DDL for [SalesDB].[audit].[ActivityLogs]
USE [SalesDB];
GO

CREATE TABLE [audit].[ActivityLogs] (
    [LogID] bigint NOT NULL,
    [EventType] nvarchar(50) NOT NULL,
    [ExecutedBy] nvarchar(100) NOT NULL,
    [LogTimestamp] datetime2 NOT NULL DEFAULT GETDATE(),
    [Details] nvarchar(max) NULL,
    CONSTRAINT [PK_audit_ActivityLogs] PRIMARY KEY ([LogID])
);
GO

SELECT COUNT(*), UPPER(TableName), 'Active' AS Status
FROM [audit].[ActivityLogs]
WHERE [LogID] > 100 AND [EventType] = 'INSERT';
`
	highlighted := HighlightSQL(sql)
	if len(highlighted) == 0 {
		t.Fatalf("Expected highlighted SQL to not be empty")
	}

	tokens := TokenizeLine("SELECT COUNT(*) AS Total, 'Hello' FROM [SalesDB].[Customers] WHERE ID = 42; -- comment")
	foundKeyword := false
	foundFunc := false
	foundString := false
	foundIdent := false
	foundNumber := false
	foundComment := false

	for _, tok := range tokens {
		switch tok.Type {
		case TokenKeyword:
			if tok.Value == "SELECT" || tok.Value == "FROM" || tok.Value == "WHERE" {
				foundKeyword = true
			}
		case TokenFunction:
			if tok.Value == "COUNT" {
				foundFunc = true
			}
		case TokenString:
			if strings.Contains(tok.Value, "Hello") {
				foundString = true
			}
		case TokenIdentifier:
			if strings.Contains(tok.Value, "SalesDB") || strings.Contains(tok.Value, "Customers") {
				foundIdent = true
			}
		case TokenNumber:
			if tok.Value == "42" {
				foundNumber = true
			}
		case TokenComment:
			if strings.Contains(tok.Value, "comment") {
				foundComment = true
			}
		}
	}

	if !foundKeyword {
		t.Errorf("Expected to find TokenKeyword")
	}
	if !foundFunc {
		t.Errorf("Expected to find TokenFunction")
	}
	if !foundString {
		t.Errorf("Expected to find TokenString")
	}
	if !foundIdent {
		t.Errorf("Expected to find TokenIdentifier")
	}
	if !foundNumber {
		t.Errorf("Expected to find TokenNumber")
	}
	if !foundComment {
		t.Errorf("Expected to find TokenComment")
	}
}
