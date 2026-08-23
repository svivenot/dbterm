package db

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"dbterm/internal/config"
)

// Driver is the common database interface for dbterm
type Driver interface {
	Connect(ctx context.Context, profile *config.ConnectionProfile) error
	Close() error
	Ping(ctx context.Context) error
	ExecuteQuery(ctx context.Context, query string) (*QueryResult, error)
	FetchDatabases(ctx context.Context) ([]string, error)
	FetchTables(ctx context.Context, database string) ([]TableInfo, error)
	FetchViews(ctx context.Context, database string) ([]TableInfo, error)
	FetchProcedures(ctx context.Context, database string) ([]string, error)
	FetchColumns(ctx context.Context, database, schema, table string) ([]ColumnInfo, error)
	GenerateSelectQuery(schema, table string, limit int) string
	GenerateInsertQuery(schema, table string, columns []ColumnInfo) string
	GenerateDDL(ctx context.Context, database, schema, name string, nodeType NodeType) (string, error)
	GetConnectionInfo() string
	GetActiveDatabase() string
	SwitchDatabase(ctx context.Context, database string) error
}

// NewDriver returns the appropriate driver implementation based on the profile
func NewDriver(profile *config.ConnectionProfile) (Driver, error) {
	switch strings.ToLower(profile.Driver) {
	case "mssql", "sqlserver":
		return NewMSSQLDriver(), nil
	case "postgres", "postgresql", "pg":
		return NewPostgresDriver(), nil
	case "oracle", "ora":
		return NewOracleDriver(), nil
	default:
		return nil, fmt.Errorf("unsupported database driver: %s (supported: mssql, postgres, oracle)", profile.Driver)
	}
}

// splitSQLBatches splits a SQL script on GO batch separators (case-insensitive on its own line)
func splitSQLBatches(query string) []string {
	lines := strings.Split(query, "\n")
	var batches []string
	var currentBatch []string

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		// Check for GO batch separator on its own line (e.g. "GO", "go", "GO;", "GO -- comment")
		isGO := false
		if strings.EqualFold(trimmed, "GO") || strings.EqualFold(trimmed, "GO;") {
			isGO = true
		} else if strings.HasPrefix(strings.ToUpper(trimmed), "GO ") || strings.HasPrefix(strings.ToUpper(trimmed), "GO--") || strings.HasPrefix(strings.ToUpper(trimmed), "GO\t") {
			isGO = true
		}

		if isGO {
			if len(currentBatch) > 0 {
				batchStr := strings.TrimSpace(strings.Join(currentBatch, "\n"))
				if batchStr != "" {
					batches = append(batches, batchStr)
				}
				currentBatch = nil
			}
		} else {
			currentBatch = append(currentBatch, line)
		}
	}

	if len(currentBatch) > 0 {
		batchStr := strings.TrimSpace(strings.Join(currentBatch, "\n"))
		if batchStr != "" {
			batches = append(batches, batchStr)
		}
	}

	if len(batches) == 0 {
		return []string{query}
	}
	return batches
}

// Helper to execute a query against *sql.DB and populate *QueryResult
func executeSQL(ctx context.Context, db *sql.DB, query string) (*QueryResult, error) {
	startTime := time.Now()
	res := &QueryResult{
		Query:      query,
		ExecutedAt: startTime,
		Messages:   []string{},
	}

	trimmed := strings.TrimSpace(query)
	if trimmed == "" {
		res.Duration = time.Since(startTime)
		return res, nil
	}

	batches := splitSQLBatches(query)
	var lastRes *QueryResult
	var totalAffected int64

	for batchIdx, batch := range batches {
		batchTrimmed := strings.TrimSpace(batch)
		if batchTrimmed == "" {
			continue
		}

		batchRes, err := executeSingleSQL(ctx, db, batchTrimmed)
		if err != nil {
			res.Duration = time.Since(startTime)
			res.Error = err
			res.Messages = append(res.Messages, fmt.Sprintf("Msg in batch %d: %v", batchIdx+1, err))
			return res, err
		}

		totalAffected += batchRes.AffectedRows
		res.Messages = append(res.Messages, batchRes.Messages...)
		lastRes = batchRes
	}

	res.Duration = time.Since(startTime)
	if lastRes != nil && len(lastRes.Columns) > 0 {
		res.Columns = lastRes.Columns
		res.Rows = lastRes.Rows
		res.AffectedRows = int64(len(lastRes.Rows))
	} else {
		res.AffectedRows = totalAffected
		if len(res.Messages) == 0 {
			res.Messages = append(res.Messages, "Commands completed successfully.")
		}
	}

	return res, nil
}

func executeSingleSQL(ctx context.Context, db *sql.DB, query string) (*QueryResult, error) {
	startTime := time.Now()
	res := &QueryResult{
		Query:      query,
		ExecutedAt: startTime,
		Messages:   []string{},
	}

	trimmed := strings.TrimSpace(query)
	if trimmed == "" {
		res.Duration = time.Since(startTime)
		return res, nil
	}

	// Strip leading comments to identify command verb
	cleanQuery := trimmed
	for strings.HasPrefix(cleanQuery, "--") || strings.HasPrefix(cleanQuery, "/*") {
		if strings.HasPrefix(cleanQuery, "--") {
			idx := strings.Index(cleanQuery, "\n")
			if idx == -1 {
				cleanQuery = ""
				break
			}
			cleanQuery = strings.TrimSpace(cleanQuery[idx+1:])
		} else if strings.HasPrefix(cleanQuery, "/*") {
			idx := strings.Index(cleanQuery, "*/")
			if idx == -1 {
				cleanQuery = ""
				break
			}
			cleanQuery = strings.TrimSpace(cleanQuery[idx+2:])
		}
	}

	if cleanQuery == "" {
		res.Duration = time.Since(startTime)
		return res, nil
	}

	// Check if this is an Exec statement (INSERT, UPDATE, DELETE, CREATE, DROP, ALTER, USE, SET, TRUNCATE, EXEC)
	isExecOnly := false
	upper := strings.ToUpper(cleanQuery)
	execPrefixes := []string{"INSERT ", "UPDATE ", "DELETE ", "CREATE ", "DROP ", "ALTER ", "USE ", "SET ", "TRUNCATE ", "EXEC "}
	for _, prefix := range execPrefixes {
		if strings.HasPrefix(upper, prefix) {
			isExecOnly = true
			break
		}
	}

	if isExecOnly && !strings.Contains(upper, "OUTPUT ") && !strings.Contains(upper, "RETURNING ") {
		execRes, err := db.ExecContext(ctx, query)
		res.Duration = time.Since(startTime)
		if err != nil {
			res.Error = err
			res.Messages = append(res.Messages, fmt.Sprintf("Msg: %v", err))
			return res, err
		}
		affected, _ := execRes.RowsAffected()
		res.AffectedRows = affected
		res.Messages = append(res.Messages, fmt.Sprintf("(%d row(s) affected)", affected))
		return res, nil
	}

	// Otherwise execute as Query
	rows, err := db.QueryContext(ctx, query)
	if err != nil {
		res.Duration = time.Since(startTime)
		res.Error = err
		res.Messages = append(res.Messages, fmt.Sprintf("Error: %v", err))
		return res, err
	}
	defer rows.Close()

	cols, err := rows.Columns()
	if err != nil {
		res.Duration = time.Since(startTime)
		res.Error = err
		return res, err
	}
	res.Columns = cols

	numCols := len(cols)
	for rows.Next() {
		rawValues := make([]any, numCols)
		valuePtrs := make([]any, numCols)
		for i := range rawValues {
			valuePtrs[i] = &rawValues[i]
		}

		if err := rows.Scan(valuePtrs...); err != nil {
			res.Duration = time.Since(startTime)
			res.Error = err
			return res, err
		}

		rowStrings := make([]string, numCols)
		for i, val := range rawValues {
			if val == nil {
				rowStrings[i] = "NULL"
			} else {
				switch v := val.(type) {
				case []byte:
					rowStrings[i] = string(v)
				case time.Time:
					rowStrings[i] = v.Format("2006-01-02 15:04:05")
				default:
					rowStrings[i] = fmt.Sprintf("%v", v)
				}
			}
		}
		res.Rows = append(res.Rows, rowStrings)
	}

	if err := rows.Err(); err != nil {
		res.Duration = time.Since(startTime)
		res.Error = err
		return res, err
	}

	res.Duration = time.Since(startTime)
	res.AffectedRows = int64(len(res.Rows))
	res.Messages = append(res.Messages, fmt.Sprintf("(%d row(s) returned)", len(res.Rows)))
	return res, nil
}
