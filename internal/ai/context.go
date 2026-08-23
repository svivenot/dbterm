package ai

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"dbterm/internal/db"
)

// SchemaContextCache holds cached schema DDLs per database to avoid redundant queries
type SchemaContextCache struct {
	mu      sync.RWMutex
	cache   map[string]cacheEntry
	ttl     time.Duration
}

type cacheEntry struct {
	ddl       string
	tableCount int
	colCount   int
	expiresAt time.Time
}

var globalSchemaCache = &SchemaContextCache{
	cache: make(map[string]cacheEntry),
	ttl:   10 * time.Minute,
}

// InvalidateSchemaCache clears cached schema for a database or all databases
func InvalidateSchemaCache(database string) {
	globalSchemaCache.mu.Lock()
	defer globalSchemaCache.mu.Unlock()
	if database == "" {
		globalSchemaCache.cache = make(map[string]cacheEntry)
	} else {
		delete(globalSchemaCache.cache, database)
	}
}

// SchemaSummary holds summary metrics about the active schema
type SchemaSummary struct {
	Database   string
	Dialect    string
	TableCount int
	ColCount   int
	DDLContext string
}

// BuildSchemaContext extracts table & column definitions from the active database driver
func BuildSchemaContext(ctx context.Context, driver db.Driver, maxTables int) (*SchemaSummary, error) {
	if driver == nil {
		return &SchemaSummary{
			Database:   "None",
			Dialect:    "SQL",
			DDLContext: "-- No active database connection.\n",
		}, nil
	}

	activeDB := driver.GetActiveDatabase()
	if activeDB == "" {
		activeDB = "master"
	}

	// Check in-memory cache
	globalSchemaCache.mu.RLock()
	if entry, ok := globalSchemaCache.cache[activeDB]; ok && time.Now().Before(entry.expiresAt) {
		globalSchemaCache.mu.RUnlock()
		return &SchemaSummary{
			Database:   activeDB,
			Dialect:    detectDialect(driver),
			TableCount: entry.tableCount,
			ColCount:   entry.colCount,
			DDLContext: entry.ddl,
		}, nil
	}
	globalSchemaCache.mu.RUnlock()

	tables, err := driver.FetchTables(ctx, activeDB)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch database tables: %w", err)
	}

	if maxTables <= 0 {
		maxTables = 30
	}
	if len(tables) > maxTables {
		tables = tables[:maxTables]
	}

	dialect := detectDialect(driver)
	var b strings.Builder
	b.WriteString(fmt.Sprintf("-- Database: %s (Dialect: %s)\n", activeDB, dialect))

	totalCols := 0

	for _, tbl := range tables {
		cols, err := driver.FetchColumns(ctx, activeDB, tbl.Schema, tbl.Name)
		if err != nil || len(cols) == 0 {
			continue
		}

		totalCols += len(cols)
		fullTableName := tbl.Name
		if tbl.Schema != "" && !strings.Contains(tbl.Name, ".") {
			fullTableName = fmt.Sprintf("%s.%s", tbl.Schema, tbl.Name)
		}

		b.WriteString(fmt.Sprintf("CREATE TABLE %s (\n", fullTableName))
		for ci, col := range cols {
			colDef := fmt.Sprintf("  %s %s", col.Name, col.DataType)
			if col.IsPrimaryKey {
				colDef += " PRIMARY KEY"
			} else if !col.IsNullable {
				colDef += " NOT NULL"
			}
			if ci < len(cols)-1 {
				colDef += ","
			}
			b.WriteString(colDef + "\n")
		}
		b.WriteString(");\n\n")
	}

	ddl := b.String()

	// Store in cache
	globalSchemaCache.mu.Lock()
	globalSchemaCache.cache[activeDB] = cacheEntry{
		ddl:        ddl,
		tableCount: len(tables),
		colCount:   totalCols,
		expiresAt:  time.Now().Add(globalSchemaCache.ttl),
	}
	globalSchemaCache.mu.Unlock()

	return &SchemaSummary{
		Database:   activeDB,
		Dialect:    dialect,
		TableCount: len(tables),
		ColCount:   totalCols,
		DDLContext: ddl,
	}, nil
}

func detectDialect(driver db.Driver) string {
	info := strings.ToLower(driver.GetConnectionInfo())
	if strings.Contains(info, "mssql") || strings.Contains(info, "sql server") {
		return "T-SQL (MS SQL Server)"
	}
	if strings.Contains(info, "postgres") || strings.Contains(info, "pg") {
		return "PostgreSQL"
	}
	if strings.Contains(info, "oracle") || strings.Contains(info, "ora") {
		return "Oracle PL/SQL"
	}
	return "Standard SQL"
}
