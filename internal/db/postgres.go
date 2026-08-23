package db

import (
	"context"
	"database/sql"
	"fmt"
	"net/url"
	"strconv"
	"strings"

	_ "github.com/jackc/pgx/v5/stdlib"

	"dbterm/internal/config"
)

type PostgresDriver struct {
	db             *sql.DB
	profile        *config.ConnectionProfile
	activeDatabase string
}

func NewPostgresDriver() *PostgresDriver {
	return &PostgresDriver{}
}

func (d *PostgresDriver) Connect(ctx context.Context, profile *config.ConnectionProfile) error {
	d.profile = profile
	d.activeDatabase = profile.Database
	if d.activeDatabase == "" {
		d.activeDatabase = "postgres"
	}

	password, err := profile.ResolvePassword(ctx)
	if err != nil {
		return fmt.Errorf("authentication error: %w", err)
	}

	sslMode := profile.SSLMode
	if sslMode == "" {
		sslMode = "disable"
	}

	userInfo := url.User(profile.User)
	if password != "" {
		userInfo = url.UserPassword(profile.User, password)
	}

	connURL := url.URL{
		Scheme: "postgres",
		User:   userInfo,
		Host:   fmt.Sprintf("%s:%d", profile.Host, profile.Port),
		Path:   d.activeDatabase,
	}

	query := connURL.Query()
	query.Add("sslmode", sslMode)
	query.Add("connect_timeout", "10")
	connURL.RawQuery = query.Encode()

	db, err := sql.Open("pgx", connURL.String())
	if err != nil {
		return fmt.Errorf("failed to open pgx driver: %w", err)
	}

	if err := db.PingContext(ctx); err != nil {
		db.Close()
		return fmt.Errorf("failed to connect to PostgreSQL at %s:%d (database: %s): %w", profile.Host, profile.Port, d.activeDatabase, err)
	}

	d.db = db
	return nil
}

func (d *PostgresDriver) Close() error {
	if d.db != nil {
		return d.db.Close()
	}
	return nil
}

func (d *PostgresDriver) Ping(ctx context.Context) error {
	if d.db == nil {
		return fmt.Errorf("not connected")
	}
	return d.db.PingContext(ctx)
}

func (d *PostgresDriver) ExecuteQuery(ctx context.Context, query string) (*QueryResult, error) {
	if d.db == nil {
		return nil, fmt.Errorf("not connected to database")
	}
	return executeSQL(ctx, d.db, query)
}

func (d *PostgresDriver) GetConnectionInfo() string {
	if d.profile == nil {
		return "Disconnected"
	}
	return fmt.Sprintf("PostgreSQL: %s@%s:%d [%s]", d.profile.User, d.profile.Host, d.profile.Port, d.activeDatabase)
}

func (d *PostgresDriver) GetActiveDatabase() string {
	return d.activeDatabase
}

func (d *PostgresDriver) SwitchDatabase(ctx context.Context, database string) error {
	if d.profile == nil {
		return fmt.Errorf("no active profile")
	}
	d.Close()
	newProfile := *d.profile
	newProfile.Database = database
	return d.Connect(ctx, &newProfile)
}

func (d *PostgresDriver) FetchDatabases(ctx context.Context) ([]string, error) {
	if d.db == nil {
		return nil, fmt.Errorf("not connected")
	}
	query := "SELECT datname FROM pg_database WHERE datistemplate = false ORDER BY datname;"
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

func (d *PostgresDriver) FetchTables(ctx context.Context, database string) ([]TableInfo, error) {
	if d.db == nil {
		return nil, fmt.Errorf("not connected")
	}

	query := `
		SELECT table_schema, table_name 
		FROM information_schema.tables 
		WHERE table_type = 'BASE TABLE' AND table_schema NOT IN ('pg_catalog', 'information_schema')
		ORDER BY table_schema, table_name;
	`

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

func (d *PostgresDriver) FetchViews(ctx context.Context, database string) ([]TableInfo, error) {
	if d.db == nil {
		return nil, fmt.Errorf("not connected")
	}

	query := `
		SELECT table_schema, table_name 
		FROM information_schema.tables 
		WHERE table_type = 'VIEW' AND table_schema NOT IN ('pg_catalog', 'information_schema')
		ORDER BY table_schema, table_name;
	`

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

func (d *PostgresDriver) FetchProcedures(ctx context.Context, database string) ([]string, error) {
	if d.db == nil {
		return nil, fmt.Errorf("not connected")
	}

	query := `
		SELECT routine_schema || '.' || routine_name
		FROM information_schema.routines 
		WHERE routine_schema NOT IN ('pg_catalog', 'information_schema')
		ORDER BY routine_schema, routine_name;
	`

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

func (d *PostgresDriver) FetchColumns(ctx context.Context, database, schema, table string) ([]ColumnInfo, error) {
	if d.db == nil {
		return nil, fmt.Errorf("not connected")
	}

	query := `
		SELECT 
			c.column_name, 
			c.data_type,
			c.is_nullable,
			CASE WHEN pk.column_name IS NOT NULL THEN 1 ELSE 0 END AS is_primary_key,
			c.ordinal_position
		FROM information_schema.columns c
		LEFT JOIN (
			SELECT kcu.column_name, kcu.table_schema, kcu.table_name
			FROM information_schema.table_constraints tc
			JOIN information_schema.key_column_usage kcu 
				ON tc.constraint_name = kcu.constraint_name AND tc.table_schema = kcu.table_schema
			WHERE tc.constraint_type = 'PRIMARY KEY'
		) pk ON c.table_schema = pk.table_schema AND c.table_name = pk.table_name AND c.column_name = pk.column_name
		WHERE c.table_schema = $1 AND c.table_name = $2
		ORDER BY c.ordinal_position;
	`

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

func (d *PostgresDriver) GenerateSelectQuery(schema, table string, limit int) string {
	if limit <= 0 {
		limit = 100
	}
	if schema != "" && schema != "public" {
		return fmt.Sprintf("SELECT * FROM %s.%s LIMIT %s;", schema, table, strconv.Itoa(limit))
	}
	return fmt.Sprintf("SELECT * FROM %s LIMIT %s;", table, strconv.Itoa(limit))
}

func (d *PostgresDriver) GenerateInsertQuery(schema, table string, columns []ColumnInfo) string {
	target := table
	if schema != "" && schema != "public" {
		target = fmt.Sprintf("%s.%s", schema, table)
	}

	if len(columns) == 0 {
		return fmt.Sprintf("INSERT INTO %s DEFAULT VALUES;", target)
	}

	var colNames []string
	var valPlaceholders []string
	for _, col := range columns {
		if strings.Contains(col.DataType, "serial") {
			continue
		}
		colNames = append(colNames, col.Name)
		valPlaceholders = append(valPlaceholders, fmt.Sprintf("/* %s */ NULL", col.DataType))
	}

	return fmt.Sprintf("INSERT INTO %s (\n    %s\n)\nVALUES (\n    %s\n);",
		target,
		strings.Join(colNames, ", "),
		strings.Join(valPlaceholders, ", "),
	)
}

func (d *PostgresDriver) GenerateDDL(ctx context.Context, database, schema, name string, nodeType NodeType) (string, error) {
	if d.db == nil {
		return "", fmt.Errorf("not connected")
	}

	if nodeType == NodeView {
		fullObj := fmt.Sprintf("%s.%s", schema, name)
		query := fmt.Sprintf("SELECT pg_get_viewdef('%s'::regclass, true);", fullObj)
		var def sql.NullString
		if err := d.db.QueryRowContext(ctx, query).Scan(&def); err == nil && def.Valid && def.String != "" {
			return fmt.Sprintf("CREATE OR REPLACE VIEW %s.%s AS\n%s;", schema, name, def.String), nil
		}
	}

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
		colDefs = append(colDefs, fmt.Sprintf("    %s %s %s", col.Name, col.DataType, nullability))
		if col.IsPrimaryKey {
			pkCols = append(pkCols, col.Name)
		}
	}

	if len(pkCols) > 0 {
		pkConstraint := fmt.Sprintf("    PRIMARY KEY (%s)", strings.Join(pkCols, ", "))
		colDefs = append(colDefs, pkConstraint)
	}

	target := fmt.Sprintf("%s.%s", schema, name)
	return fmt.Sprintf("CREATE TABLE %s (\n%s\n);", target, strings.Join(colDefs, ",\n")), nil
}
