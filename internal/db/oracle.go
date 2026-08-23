package db

import (
	"context"
	"database/sql"
	"fmt"
	"strconv"
	"strings"

	go_ora "github.com/sijms/go-ora/v2"

	"dbterm/internal/config"
)

type OracleDriver struct {
	db             *sql.DB
	profile        *config.ConnectionProfile
	activeDatabase string
}

func NewOracleDriver() *OracleDriver {
	return &OracleDriver{}
}

func (d *OracleDriver) Connect(ctx context.Context, profile *config.ConnectionProfile) error {
	d.profile = profile
	d.activeDatabase = profile.Database
	if d.activeDatabase == "" {
		d.activeDatabase = "ORCLPDB1"
	}

	password, err := profile.ResolvePassword(ctx)
	if err != nil {
		return fmt.Errorf("authentication error: %w", err)
	}

	urlOptions := map[string]string{
		"CONNECTION TIMEOUT": "15",
	}

	connURL := go_ora.BuildUrl(profile.Host, profile.Port, d.activeDatabase, profile.User, password, urlOptions)

	db, err := sql.Open("oracle", connURL)
	if err != nil {
		return fmt.Errorf("failed to open oracle driver: %w", err)
	}

	if err := db.PingContext(ctx); err != nil {
		db.Close()
		return fmt.Errorf("failed to connect to Oracle at %s:%d (service: %s): %w", profile.Host, profile.Port, d.activeDatabase, err)
	}

	d.db = db
	return nil
}

func (d *OracleDriver) Close() error {
	if d.db != nil {
		return d.db.Close()
	}
	return nil
}

func (d *OracleDriver) Ping(ctx context.Context) error {
	if d.db == nil {
		return fmt.Errorf("not connected")
	}
	return d.db.PingContext(ctx)
}

func (d *OracleDriver) ExecuteQuery(ctx context.Context, query string) (*QueryResult, error) {
	if d.db == nil {
		return nil, fmt.Errorf("not connected to database")
	}
	return executeSQL(ctx, d.db, query)
}

func (d *OracleDriver) GetConnectionInfo() string {
	if d.profile == nil {
		return "Disconnected"
	}
	return fmt.Sprintf("Oracle: %s@%s:%d [%s]", d.profile.User, d.profile.Host, d.profile.Port, d.activeDatabase)
}

func (d *OracleDriver) GetActiveDatabase() string {
	return d.activeDatabase
}

func (d *OracleDriver) SwitchDatabase(ctx context.Context, database string) error {
	if d.profile == nil {
		return fmt.Errorf("no active profile")
	}
	d.Close()
	newProfile := *d.profile
	newProfile.Database = database
	return d.Connect(ctx, &newProfile)
}

func (d *OracleDriver) FetchDatabases(ctx context.Context) ([]string, error) {
	if d.db == nil {
		return nil, fmt.Errorf("not connected")
	}
	// In Oracle, schemas/users act as database containers
	query := "SELECT USERNAME FROM ALL_USERS WHERE ORACLE_MAINTAINED = 'N' ORDER BY USERNAME"
	rows, err := d.db.QueryContext(ctx, query)
	if err != nil {
		// Fallback for older Oracle versions
		rows, err = d.db.QueryContext(ctx, "SELECT USERNAME FROM ALL_USERS ORDER BY USERNAME")
		if err != nil {
			return nil, err
		}
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

func (d *OracleDriver) FetchTables(ctx context.Context, database string) ([]TableInfo, error) {
	if d.db == nil {
		return nil, fmt.Errorf("not connected")
	}

	query := `
		SELECT OWNER, TABLE_NAME 
		FROM ALL_TABLES 
		WHERE OWNER = UPPER(:1)
		ORDER BY TABLE_NAME
	`

	rows, err := d.db.QueryContext(ctx, query, database)
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

func (d *OracleDriver) FetchViews(ctx context.Context, database string) ([]TableInfo, error) {
	if d.db == nil {
		return nil, fmt.Errorf("not connected")
	}

	query := `
		SELECT OWNER, VIEW_NAME 
		FROM ALL_VIEWS 
		WHERE OWNER = UPPER(:1)
		ORDER BY VIEW_NAME
	`

	rows, err := d.db.QueryContext(ctx, query, database)
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

func (d *OracleDriver) FetchProcedures(ctx context.Context, database string) ([]string, error) {
	if d.db == nil {
		return nil, fmt.Errorf("not connected")
	}

	query := `
		SELECT OBJECT_NAME 
		FROM ALL_OBJECTS 
		WHERE OWNER = UPPER(:1) AND OBJECT_TYPE IN ('PROCEDURE', 'FUNCTION', 'PACKAGE')
		ORDER BY OBJECT_NAME
	`

	rows, err := d.db.QueryContext(ctx, query, database)
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

func (d *OracleDriver) FetchColumns(ctx context.Context, database, schema, table string) ([]ColumnInfo, error) {
	if d.db == nil {
		return nil, fmt.Errorf("not connected")
	}

	query := `
		SELECT 
			c.COLUMN_NAME, 
			c.DATA_TYPE || CASE WHEN c.DATA_LENGTH > 0 AND c.DATA_TYPE LIKE '%CHAR%' THEN '(' || c.DATA_LENGTH || ')' ELSE '' END,
			c.NULLABLE,
			CASE WHEN pk.COLUMN_NAME IS NOT NULL THEN 1 ELSE 0 END AS IS_PRIMARY_KEY,
			c.COLUMN_ID
		FROM ALL_TAB_COLUMNS c
		LEFT JOIN (
			SELECT cols.COLUMN_NAME, cols.OWNER, cols.TABLE_NAME
			FROM ALL_CONSTRAINTS cons
			JOIN ALL_CONS_COLUMNS cols ON cons.CONSTRAINT_NAME = cols.CONSTRAINT_NAME AND cons.OWNER = cols.OWNER
			WHERE cons.CONSTRAINT_TYPE = 'P'
		) pk ON c.OWNER = pk.OWNER AND c.TABLE_NAME = pk.TABLE_NAME AND c.COLUMN_NAME = pk.COLUMN_NAME
		WHERE c.OWNER = UPPER(:1) AND c.TABLE_NAME = UPPER(:2)
		ORDER BY c.COLUMN_ID
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
			IsNullable:   isNullableStr == "Y",
			IsPrimaryKey: isPKInt == 1,
			Position:     pos,
		})
	}
	return columns, rows.Err()
}

func (d *OracleDriver) GenerateSelectQuery(schema, table string, limit int) string {
	if limit <= 0 {
		limit = 100
	}
	return fmt.Sprintf("SELECT * FROM %s.%s FETCH FIRST %s ROWS ONLY;", schema, table, strconv.Itoa(limit))
}

func (d *OracleDriver) GenerateInsertQuery(schema, table string, columns []ColumnInfo) string {
	target := fmt.Sprintf("%s.%s", schema, table)
	if len(columns) == 0 {
		return fmt.Sprintf("INSERT INTO %s VALUES ();", target)
	}

	var colNames []string
	var valPlaceholders []string
	for _, col := range columns {
		colNames = append(colNames, col.Name)
		valPlaceholders = append(valPlaceholders, fmt.Sprintf("/* %s */ NULL", col.DataType))
	}

	return fmt.Sprintf("INSERT INTO %s (\n    %s\n)\nVALUES (\n    %s\n);",
		target,
		strings.Join(colNames, ", "),
		strings.Join(valPlaceholders, ", "),
	)
}

func (d *OracleDriver) GenerateDDL(ctx context.Context, database, schema, name string, nodeType NodeType) (string, error) {
	if d.db == nil {
		return "", fmt.Errorf("not connected")
	}

	cols, err := d.FetchColumns(ctx, database, schema, name)
	if err != nil {
		return "", err
	}

	var colDefs []string
	var pkCols []string
	for _, col := range cols {
		nullability := ""
		if !col.IsNullable {
			nullability = "NOT NULL"
		}
		colDefs = append(colDefs, fmt.Sprintf("    %s %s %s", col.Name, col.DataType, nullability))
		if col.IsPrimaryKey {
			pkCols = append(pkCols, col.Name)
		}
	}

	if len(pkCols) > 0 {
		pkConstraint := fmt.Sprintf("    CONSTRAINT PK_%s_%s PRIMARY KEY (%s)", schema, name, strings.Join(pkCols, ", "))
		colDefs = append(colDefs, pkConstraint)
	}

	target := fmt.Sprintf("%s.%s", schema, name)
	return fmt.Sprintf("CREATE TABLE %s (\n%s\n);", target, strings.Join(colDefs, ",\n")), nil
}
