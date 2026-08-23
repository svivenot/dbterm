package db

import (
	"context"
	"database/sql"
	"fmt"
	"net/url"
	"strconv"
	"strings"

	_ "github.com/microsoft/go-mssqldb"

	"dbterm/internal/config"
)

type MSSQLDriver struct {
	db             *sql.DB
	profile        *config.ConnectionProfile
	activeDatabase string
}

func NewMSSQLDriver() *MSSQLDriver {
	return &MSSQLDriver{}
}

func (d *MSSQLDriver) Connect(ctx context.Context, profile *config.ConnectionProfile) error {
	d.profile = profile
	d.activeDatabase = profile.Database
	if d.activeDatabase == "" {
		d.activeDatabase = "master"
	}

	password, err := profile.ResolvePassword(ctx)
	if err != nil {
		return fmt.Errorf("authentication error: %w", err)
	}

	connURL := url.URL{
		Scheme: "sqlserver",
		Host:   fmt.Sprintf("%s:%d", profile.Host, profile.Port),
	}

	query := connURL.Query()
	query.Add("database", d.activeDatabase)
	query.Add("connection timeout", "15")

	if profile.AuthType == config.AuthTypeWindows {
		// Windows Authentication / Integrated Security
		query.Add("trusted_connection", "yes")
		query.Add("integrated security", "true")

		username := profile.User
		if profile.Domain != "" && !strings.Contains(username, "\\") {
			username = fmt.Sprintf("%s\\%s", profile.Domain, username)
		}
		if username != "" && password != "" {
			connURL.User = url.UserPassword(username, password)
		}
	} else {
		// Standard SQL Authentication
		if profile.User != "" {
			if password != "" {
				connURL.User = url.UserPassword(profile.User, password)
			} else {
				connURL.User = url.User(profile.User)
			}
		}
	}

	if profile.SSLMode == "disable" || profile.SSLMode == "false" {
		query.Add("encrypt", "disable")
		query.Add("trustservercertificate", "true")
	} else {
		query.Add("trustservercertificate", "true")
	}

	connURL.RawQuery = query.Encode()

	db, err := sql.Open("sqlserver", connURL.String())
	if err != nil {
		return fmt.Errorf("failed to open mssql driver: %w", err)
	}

	// Test connection
	if err := db.PingContext(ctx); err != nil {
		db.Close()
		return fmt.Errorf("failed to connect to MS SQL Server at %s:%d (database: %s): %w", profile.Host, profile.Port, d.activeDatabase, err)
	}

	d.db = db
	return nil
}

func (d *MSSQLDriver) Close() error {
	if d.db != nil {
		return d.db.Close()
	}
	return nil
}

func (d *MSSQLDriver) Ping(ctx context.Context) error {
	if d.db == nil {
		return fmt.Errorf("not connected")
	}
	return d.db.PingContext(ctx)
}

func (d *MSSQLDriver) ExecuteQuery(ctx context.Context, query string) (*QueryResult, error) {
	if d.db == nil {
		return nil, fmt.Errorf("not connected to database")
	}
	return executeSQL(ctx, d.db, query)
}

func (d *MSSQLDriver) GetConnectionInfo() string {
	if d.profile == nil {
		return "Disconnected"
	}
	auth := "SQL"
	if d.profile.AuthType == config.AuthTypeWindows {
		auth = "Windows"
	}
	return fmt.Sprintf("MSSQL: %s@%s:%d [%s] (Auth: %s)", d.profile.User, d.profile.Host, d.profile.Port, d.activeDatabase, auth)
}

func (d *MSSQLDriver) GetActiveDatabase() string {
	return d.activeDatabase
}

func (d *MSSQLDriver) SwitchDatabase(ctx context.Context, database string) error {
	if d.db == nil {
		return fmt.Errorf("not connected")
	}
	_, err := d.db.ExecContext(ctx, fmt.Sprintf("USE [%s];", database))
	if err != nil {
		return fmt.Errorf("failed to switch database to %s: %w", database, err)
	}
	d.activeDatabase = database
	return nil
}

func (d *MSSQLDriver) FetchDatabases(ctx context.Context) ([]string, error) {
	if d.db == nil {
		return nil, fmt.Errorf("not connected")
	}
	query := "SELECT name FROM sys.databases WHERE state = 0 AND name NOT IN ('model') ORDER BY name;"
	rows, err := d.db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var databases []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		databases = append(databases, name)
	}
	return databases, rows.Err()
}

func (d *MSSQLDriver) FetchTables(ctx context.Context, database string) ([]TableInfo, error) {
	if d.db == nil {
		return nil, fmt.Errorf("not connected")
	}

	query := fmt.Sprintf(`
		SELECT TABLE_SCHEMA, TABLE_NAME 
		FROM [%s].INFORMATION_SCHEMA.TABLES 
		WHERE TABLE_TYPE = 'BASE TABLE'
		ORDER BY TABLE_SCHEMA, TABLE_NAME;
	`, database)

	rows, err := d.db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tables []TableInfo
	for rows.Next() {
		var schema, name string
		if err := rows.Scan(&schema, &name); err != nil {
			return nil, err
		}
		tables = append(tables, TableInfo{
			Catalog: database,
			Schema:  schema,
			Name:    name,
			Type:    NodeTable,
		})
	}
	return tables, rows.Err()
}

func (d *MSSQLDriver) FetchViews(ctx context.Context, database string) ([]TableInfo, error) {
	if d.db == nil {
		return nil, fmt.Errorf("not connected")
	}

	query := fmt.Sprintf(`
		SELECT TABLE_SCHEMA, TABLE_NAME 
		FROM [%s].INFORMATION_SCHEMA.TABLES 
		WHERE TABLE_TYPE = 'VIEW'
		ORDER BY TABLE_SCHEMA, TABLE_NAME;
	`, database)

	rows, err := d.db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var views []TableInfo
	for rows.Next() {
		var schema, name string
		if err := rows.Scan(&schema, &name); err != nil {
			return nil, err
		}
		views = append(views, TableInfo{
			Catalog: database,
			Schema:  schema,
			Name:    name,
			Type:    NodeView,
		})
	}
	return views, rows.Err()
}

func (d *MSSQLDriver) FetchProcedures(ctx context.Context, database string) ([]string, error) {
	if d.db == nil {
		return nil, fmt.Errorf("not connected")
	}

	query := fmt.Sprintf(`
		SELECT ROUTINE_SCHEMA + '.' + ROUTINE_NAME
		FROM [%s].INFORMATION_SCHEMA.ROUTINES 
		WHERE ROUTINE_TYPE = 'PROCEDURE'
		ORDER BY ROUTINE_SCHEMA, ROUTINE_NAME;
	`, database)

	rows, err := d.db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var procs []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		procs = append(procs, name)
	}
	return procs, rows.Err()
}

func (d *MSSQLDriver) FetchColumns(ctx context.Context, database, schema, table string) ([]ColumnInfo, error) {
	if d.db == nil {
		return nil, fmt.Errorf("not connected")
	}

	query := fmt.Sprintf(`
		SELECT 
			c.COLUMN_NAME, 
			c.DATA_TYPE + COALESCE('(' + CASE WHEN c.CHARACTER_MAXIMUM_LENGTH = -1 THEN 'max' ELSE CAST(c.CHARACTER_MAXIMUM_LENGTH AS VARCHAR) END + ')', ''),
			c.IS_NULLABLE,
			CASE WHEN pk.COLUMN_NAME IS NOT NULL THEN 1 ELSE 0 END AS IsPrimaryKey,
			c.ORDINAL_POSITION
		FROM [%s].INFORMATION_SCHEMA.COLUMNS c
		LEFT JOIN (
			SELECT kcu.COLUMN_NAME, kcu.TABLE_SCHEMA, kcu.TABLE_NAME
			FROM [%s].INFORMATION_SCHEMA.TABLE_CONSTRAINTS tc
			JOIN [%s].INFORMATION_SCHEMA.KEY_COLUMN_USAGE kcu 
				ON tc.CONSTRAINT_NAME = kcu.CONSTRAINT_NAME AND tc.TABLE_SCHEMA = kcu.TABLE_SCHEMA
			WHERE tc.CONSTRAINT_TYPE = 'PRIMARY KEY'
		) pk ON c.TABLE_SCHEMA = pk.TABLE_SCHEMA AND c.TABLE_NAME = pk.TABLE_NAME AND c.COLUMN_NAME = pk.COLUMN_NAME
		WHERE c.TABLE_SCHEMA = @p1 AND c.TABLE_NAME = @p2
		ORDER BY c.ORDINAL_POSITION;
	`, database, database, database)

	rows, err := d.db.QueryContext(ctx, query, schema, table)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var columns []ColumnInfo
	for rows.Next() {
		var name, dataType, isNullableStr string
		var isPKInt, pos int
		if err := rows.Scan(&name, &dataType, &isNullableStr, &isPKInt, &pos); err != nil {
			return nil, err
		}
		columns = append(columns, ColumnInfo{
			Name:         name,
			DataType:     dataType,
			IsNullable:   isNullableStr == "YES",
			IsPrimaryKey: isPKInt == 1,
			Position:     pos,
		})
	}
	return columns, rows.Err()
}

func (d *MSSQLDriver) GenerateSelectQuery(schema, table string, limit int) string {
	if limit <= 0 {
		limit = 100
	}
	return fmt.Sprintf("SELECT TOP %s * FROM [%s].[%s];", strconv.Itoa(limit), schema, table)
}

func (d *MSSQLDriver) GenerateInsertQuery(schema, table string, columns []ColumnInfo) string {
	if len(columns) == 0 {
		return fmt.Sprintf("INSERT INTO [%s].[%s] DEFAULT VALUES;", schema, table)
	}

	var colNames []string
	var valPlaceholders []string
	for _, col := range columns {
		if col.DataType == "timestamp" || strings.Contains(col.DataType, "identity") {
			continue
		}
		colNames = append(colNames, fmt.Sprintf("[%s]", col.Name))
		valPlaceholders = append(valPlaceholders, fmt.Sprintf("/* %s */ NULL", col.DataType))
	}

	return fmt.Sprintf("INSERT INTO [%s].[%s] (\n    %s\n)\nVALUES (\n    %s\n);",
		schema, table,
		strings.Join(colNames, ", "),
		strings.Join(valPlaceholders, ", "),
	)
}

func (d *MSSQLDriver) GenerateDDL(ctx context.Context, database, schema, name string, nodeType NodeType) (string, error) {
	if d.db == nil {
		return "", fmt.Errorf("not connected")
	}

	if nodeType == NodeView || nodeType == NodeProcedure || nodeType == NodeFunction {
		// Fetch definition from sys.sql_modules or OBJECT_DEFINITION
		fullObj := fmt.Sprintf("[%s].[%s]", schema, name)
		query := fmt.Sprintf("SELECT OBJECT_DEFINITION(OBJECT_ID('%s'));", fullObj)
		var def sql.NullString
		if err := d.db.QueryRowContext(ctx, query).Scan(&def); err == nil && def.Valid && def.String != "" {
			return def.String, nil
		}
	}

	// Generate CREATE TABLE DDL from columns
	cols, err := d.FetchColumns(ctx, database, schema, name)
	if err != nil {
		return "", err
	}

	var colDefs []string
	var pkCols []string
	for _, col := range cols {
		nullability := "NULL"
		if !col.IsNullable {
			nullability = "NOT NULL"
		}
		colDefs = append(colDefs, fmt.Sprintf("    [%s] %s %s", col.Name, col.DataType, nullability))
		if col.IsPrimaryKey {
			pkCols = append(pkCols, fmt.Sprintf("[%s]", col.Name))
		}
	}

	if len(pkCols) > 0 {
		pkConstraint := fmt.Sprintf("    CONSTRAINT [PK_%s_%s] PRIMARY KEY (%s)", schema, name, strings.Join(pkCols, ", "))
		colDefs = append(colDefs, pkConstraint)
	}

	header := fmt.Sprintf("-- DDL for [%s].[%s].[%s]\nUSE [%s];\nGO\n\n", database, schema, name, database)
	body := fmt.Sprintf("CREATE TABLE [%s].[%s] (\n%s\n);\nGO", schema, name, strings.Join(colDefs, ",\n"))
	return header + body, nil
}
