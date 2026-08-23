package db

import (
	"time"
)

// NodeType represents the type of node in the SSMS Object Explorer tree
type NodeType string

const (
	NodeGroup        NodeType = "group" // Environment / Folder (e.g. "Local / Docker", "Production / Enterprise")
	NodeServer       NodeType = "server"
	NodeDatabases    NodeType = "databases"
	NodeDatabase     NodeType = "database"
	NodeFolderTables NodeType = "folder_tables"
	NodeFolderViews  NodeType = "folder_views"
	NodeFolderProcs  NodeType = "folder_procs"
	NodeFolderFuncs  NodeType = "folder_funcs"
	NodeTable        NodeType = "table"
	NodeView         NodeType = "view"
	NodeProcedure    NodeType = "procedure"
	NodeFunction     NodeType = "function"
	NodeColumn       NodeType = "column"
	NodeFileDir      NodeType = "file_dir"
	NodeFileSQL      NodeType = "file_sql"
	NodeFileOther    NodeType = "file_other"
)

// TreeNode represents a node in the Object Explorer hierarchy
type TreeNode struct {
	ID           string     `json:"id"`
	Name         string     `json:"name"`
	Type         NodeType   `json:"type"`
	GroupPath    string     `json:"group_path,omitempty"`
	ProfileID    string     `json:"profile_id,omitempty"`
	DriverName   string     `json:"driver_name,omitempty"`
	Connected    bool       `json:"connected,omitempty"`
	FilePath     string     `json:"file_path,omitempty"`
	FileSize     int64      `json:"file_size,omitempty"`
	Schema       string     `json:"schema,omitempty"`
	Catalog      string     `json:"catalog,omitempty"` // Database name
	DataType     string     `json:"data_type,omitempty"`
	IsPrimaryKey bool       `json:"is_primary_key,omitempty"`
	IsNullable   bool       `json:"is_nullable,omitempty"`
	Children     []TreeNode `json:"children,omitempty"`
	Expanded     bool       `json:"expanded"`
	Loaded       bool       `json:"loaded"`
}

// ColumnInfo holds details about a table column
type ColumnInfo struct {
	Name         string `json:"name"`
	DataType     string `json:"data_type"`
	IsNullable   bool   `json:"is_nullable"`
	IsPrimaryKey bool   `json:"is_primary_key"`
	Position     int    `json:"position"`
}

// TableInfo holds metadata about a table or view
type TableInfo struct {
	Catalog string       `json:"catalog"`
	Schema  string       `json:"schema"`
	Name    string       `json:"name"`
	Type    NodeType     `json:"type"` // Table or View
	Columns []ColumnInfo `json:"columns"`
}

// QueryResult represents the outcome of executing a SQL statement
type QueryResult struct {
	Query        string        `json:"query"`
	Columns      []string      `json:"columns"`
	Rows         [][]string    `json:"rows"`
	AffectedRows int64         `json:"affected_rows"`
	Duration     time.Duration `json:"duration"`
	Messages     []string      `json:"messages"`
	ExecutedAt   time.Time     `json:"executed_at"`
	Error        error         `json:"error,omitempty"`
}

// ExecutionStats provides summary metrics
type ExecutionStats struct {
	RowCount     int
	AffectedRows int64
	Duration     time.Duration
	Timestamp    time.Time
}
