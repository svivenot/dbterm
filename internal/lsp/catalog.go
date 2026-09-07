package lsp

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"dbterm/internal/db"
)

// ColumnMetadata describes a database column
type ColumnMetadata struct {
	Name         string `json:"name"`
	DataType     string `json:"data_type"`
	IsNullable   bool   `json:"is_nullable"`
	IsPrimaryKey bool   `json:"is_primary_key"`
	Position     int    `json:"position"`
	Comment      string `json:"comment,omitempty"`
}

// TableMetadata describes a database table or view
type TableMetadata struct {
	Database    string           `json:"database"`
	Schema      string           `json:"schema"`
	Name        string           `json:"name"`
	IsView      bool             `json:"is_view"`
	Columns     []ColumnMetadata `json:"columns"`
	PrimaryKeys []string         `json:"primary_keys"`
	Comment     string           `json:"comment,omitempty"`
}

// FullName returns schema.table or just table if schema is empty/default
func (t *TableMetadata) FullName() string {
	if t.Schema != "" {
		return fmt.Sprintf("%s.%s", t.Schema, t.Name)
	}
	return t.Name
}

// ColumnLocation links a column back to its table
type ColumnLocation struct {
	Database  string         `json:"database"`
	Schema    string         `json:"schema"`
	Table     string         `json:"table"`
	IsView    bool           `json:"is_view"`
	Column    ColumnMetadata `json:"column"`
}

// Catalog holds the schema index for fast autocomplete
type Catalog struct {
	mu              sync.RWMutex
	Dialect         string
	CurrentDatabase string
	Databases       []string
	Schemas         []string
	Tables          map[string]*TableMetadata       // keyed by "schema.table" and "table" (lowercased)
	ColumnsByTable  map[string][]ColumnMetadata     // keyed by "schema.table" and "table" (lowercased)
	AllColumnMap    map[string][]ColumnLocation     // keyed by lowercased column name
	AllTables       []TableMetadata
}

// NewCatalog creates an empty Catalog
func NewCatalog() *Catalog {
	return &Catalog{
		Dialect:        "SQL",
		Databases:      []string{},
		Schemas:        []string{},
		Tables:         make(map[string]*TableMetadata),
		ColumnsByTable: make(map[string][]ColumnMetadata),
		AllColumnMap:   make(map[string][]ColumnLocation),
		AllTables:      []TableMetadata{},
	}
}

// UpdateFromDriver queries the live database driver to populate the catalog
func (c *Catalog) UpdateFromDriver(ctx context.Context, driver db.Driver) error {
	if driver == nil {
		return nil
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	activeDB := driver.GetActiveDatabase()
	if activeDB == "" {
		activeDB = "master"
	}
	c.CurrentDatabase = activeDB

	// Detect Dialect
	info := strings.ToLower(driver.GetConnectionInfo())
	if strings.Contains(info, "mssql") || strings.Contains(info, "sqlserver") {
		c.Dialect = "mssql"
	} else if strings.Contains(info, "postgres") || strings.Contains(info, "pg") {
		c.Dialect = "postgres"
	} else if strings.Contains(info, "oracle") || strings.Contains(info, "ora") {
		c.Dialect = "oracle"
	} else {
		c.Dialect = "sql"
	}

	// Fetch databases
	dbs, _ := driver.FetchDatabases(ctx)
	if len(dbs) > 0 {
		c.Databases = dbs
	}

	// Fetch tables and views
	tables, err := driver.FetchTables(ctx, activeDB)
	if err != nil {
		return fmt.Errorf("failed to fetch tables: %w", err)
	}

	views, _ := driver.FetchViews(ctx, activeDB)

	// Reset indices
	c.Tables = make(map[string]*TableMetadata)
	c.ColumnsByTable = make(map[string][]ColumnMetadata)
	c.AllColumnMap = make(map[string][]ColumnLocation)
	c.AllTables = make([]TableMetadata, 0, len(tables)+len(views))
	schemaSet := make(map[string]bool)

	var allItems []TableMetadata

	// 1. Process base tables
	for _, t := range tables {
		tm := TableMetadata{
			Database: activeDB,
			Schema:   t.Schema,
			Name:     t.Name,
			IsView:   false,
		}
		if t.Schema != "" {
			schemaSet[t.Schema] = true
		}
		allItems = append(allItems, tm)
	}

	// 2. Process views
	for _, v := range views {
		tm := TableMetadata{
			Database: activeDB,
			Schema:   v.Schema,
			Name:     v.Name,
			IsView:   true,
		}
		if v.Schema != "" {
			schemaSet[v.Schema] = true
		}
		allItems = append(allItems, tm)
	}

	// Populate schemas
	c.Schemas = make([]string, 0, len(schemaSet))
	for s := range schemaSet {
		c.Schemas = append(c.Schemas, s)
	}

	// 3. Fetch columns for each table/view
	for i := range allItems {
		item := &allItems[i]
		rawCols, err := driver.FetchColumns(ctx, activeDB, item.Schema, item.Name)
		if err == nil {
			var cols []ColumnMetadata
			var pks []string
			for _, rc := range rawCols {
				cm := ColumnMetadata{
					Name:         rc.Name,
					DataType:     rc.DataType,
					IsNullable:   rc.IsNullable,
					IsPrimaryKey: rc.IsPrimaryKey,
					Position:     rc.Position,
				}
				cols = append(cols, cm)
				if rc.IsPrimaryKey {
					pks = append(pks, rc.Name)
				}

				// Index column globally
				colLower := strings.ToLower(rc.Name)
				c.AllColumnMap[colLower] = append(c.AllColumnMap[colLower], ColumnLocation{
					Database: activeDB,
					Schema:   item.Schema,
					Table:    item.Name,
					IsView:   item.IsView,
					Column:   cm,
				})
			}
			item.Columns = cols
			item.PrimaryKeys = pks
		}

		c.AllTables = append(c.AllTables, *item)

		// Keys for lookup
		lowerTable := strings.ToLower(item.Name)
		c.Tables[lowerTable] = item
		c.ColumnsByTable[lowerTable] = item.Columns

		if item.Schema != "" {
			fullKey := strings.ToLower(fmt.Sprintf("%s.%s", item.Schema, item.Name))
			c.Tables[fullKey] = item
			c.ColumnsByTable[fullKey] = item.Columns
		}
	}

	return nil
}

// FindTable looks up a table by name or schema.name (case-insensitive)
func (c *Catalog) FindTable(schema, table string) *TableMetadata {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if schema != "" {
		key := strings.ToLower(fmt.Sprintf("%s.%s", schema, table))
		if t, ok := c.Tables[key]; ok {
			return t
		}
	}

	key := strings.ToLower(table)
	if t, ok := c.Tables[key]; ok {
		return t
	}

	return nil
}

// GetColumnsForTable retrieves columns for a given table name or alias (case-insensitive)
func (c *Catalog) GetColumnsForTable(schema, table string) []ColumnMetadata {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if schema != "" {
		key := strings.ToLower(fmt.Sprintf("%s.%s", schema, table))
		if cols, ok := c.ColumnsByTable[key]; ok {
			return cols
		}
	}

	key := strings.ToLower(table)
	if cols, ok := c.ColumnsByTable[key]; ok {
		return cols
	}

	return nil
}

// GetTablesForSchema retrieves all tables and views belonging to a schema
func (c *Catalog) GetTablesForSchema(schema string) []TableMetadata {
	c.mu.RLock()
	defer c.mu.RUnlock()

	schemaLower := strings.ToLower(schema)
	var result []TableMetadata
	for _, t := range c.AllTables {
		if strings.ToLower(t.Schema) == schemaLower {
			result = append(result, t)
		}
	}
	return result
}

// GetAllTables returns a copy of all tables and views
func (c *Catalog) GetAllTables() []TableMetadata {
	c.mu.RLock()
	defer c.mu.RUnlock()

	result := make([]TableMetadata, len(c.AllTables))
	copy(result, c.AllTables)
	return result
}

// GetAllSchemas returns all distinct schema names
func (c *Catalog) GetAllSchemas() []string {
	c.mu.RLock()
	defer c.mu.RUnlock()

	result := make([]string, len(c.Schemas))
	copy(result, c.Schemas)
	return result
}

// FindMatchingColumns returns all columns matching a name or substring across tables
func (c *Catalog) FindMatchingColumns(prefix string) []ColumnLocation {
	c.mu.RLock()
	defer c.mu.RUnlock()

	prefixLower := strings.ToLower(prefix)
	var results []ColumnLocation
	for colName, locations := range c.AllColumnMap {
		if strings.HasPrefix(colName, prefixLower) || strings.Contains(colName, prefixLower) {
			results = append(results, locations...)
		}
	}
	return results
}
