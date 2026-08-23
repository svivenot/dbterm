package main

import (
	"context"
	"database/sql"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "github.com/microsoft/go-mssqldb"
)

func main() {
	connStr := "sqlserver://sa:Password123!@localhost:1433?database=master&connection+timeout=30"
	log.Println("Connecting to MS SQL Server on localhost:1433...")

	var db *sql.DB
	var err error

	// Retry connecting for up to 30 seconds while the engine warms up
	for i := 0; i < 15; i++ {
		db, err = sql.Open("sqlserver", connStr)
		if err == nil {
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			err = db.PingContext(ctx)
			cancel()
			if err == nil {
				log.Println("Successfully connected to MS SQL Server!")
				break
			}
		}
		log.Printf("Waiting for SQL Server to be ready... (%v)\n", err)
		time.Sleep(2 * time.Second)
	}

	if err != nil {
		log.Fatalf("Could not connect to SQL Server: %v", err)
	}
	defer db.Close()

	// 1. Create databases on master
	masterDb, err := sql.Open("sqlserver", "sqlserver://sa:Password123!@localhost:1433?database=master")
	if err != nil {
		log.Fatalf("Failed to open master DB: %v", err)
	}
	defer masterDb.Close()

	log.Println("Executing: 01-create-database.sql on master")
	content01, err := os.ReadFile("testenv/mssql/init/01-create-database.sql")
	if err != nil {
		log.Fatalf("Failed to read 01-create-database.sql: %v", err)
	}
	for _, batch := range splitSQLBatches(string(content01)) {
		batch = strings.TrimSpace(batch)
		if batch != "" {
			if _, err := masterDb.Exec(batch); err != nil {
				log.Fatalf("Error on master: %v", err)
			}
		}
	}

	// 2. Connect directly to SalesDB for schema and seed scripts
	salesDb, err := sql.Open("sqlserver", "sqlserver://sa:Password123!@localhost:1433?database=SalesDB")
	if err != nil {
		log.Fatalf("Failed to open SalesDB: %v", err)
	}
	defer salesDb.Close()

	salesScripts := []string{
		"testenv/mssql/init/02-create-tables.sql",
		"testenv/mssql/init/03-seed-data.sql",
		"testenv/mssql/init/04-views-procedures.sql",
	}

	for _, file := range salesScripts {
		log.Printf("Executing SQL script on SalesDB: %s", file)
		content, err := os.ReadFile(file)
		if err != nil {
			log.Fatalf("Failed to read %s: %v", file, err)
		}

		for idx, batch := range splitSQLBatches(string(content)) {
			batch = strings.TrimSpace(batch)
			if batch == "" {
				continue
			}
			_, err := salesDb.Exec(batch)
			if err != nil {
				log.Fatalf("Error executing batch %d in %s:\n%s\nError: %v", idx+1, filepath.Base(file), batch, err)
			}
		}
		log.Printf("Successfully executed: %s", filepath.Base(file))
	}

	log.Println("All SQL scripts executed successfully! Testing data queries...")

	// Verify data
	row := salesDb.QueryRow("SELECT COUNT(*) FROM sales.Customers")
	var count int
	if err := row.Scan(&count); err != nil {
		log.Fatalf("Failed to query Customers count: %v", err)
	}
	log.Printf("Verification: sales.Customers has %d records.", count)

	row = salesDb.QueryRow("SELECT COUNT(*) FROM inventory.Products")
	if err := row.Scan(&count); err != nil {
		log.Fatalf("Failed to query Products count: %v", err)
	}
	log.Printf("Verification: inventory.Products has %d records.", count)

	row = salesDb.QueryRow("SELECT COUNT(*) FROM sales.Orders")
	if err := row.Scan(&count); err != nil {
		log.Fatalf("Failed to query Orders count: %v", err)
	}
	log.Printf("Verification: sales.Orders has %d records.", count)

	row = salesDb.QueryRow("SELECT COUNT(*) FROM sales.v_OrderSummary")
	if err := row.Scan(&count); err != nil {
		log.Fatalf("Failed to query v_OrderSummary count: %v", err)
	}
	log.Printf("Verification: sales.v_OrderSummary (VIEW) has %d records.", count)

	log.Println("Database setup and seeding COMPLETE!")
}

func splitSQLBatches(sqlContent string) []string {
	var batches []string
	lines := strings.Split(sqlContent, "\n")
	var currentBatch strings.Builder

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.EqualFold(trimmed, "GO") {
			batches = append(batches, currentBatch.String())
			currentBatch.Reset()
		} else {
			currentBatch.WriteString(line)
			currentBatch.WriteString("\n")
		}
	}
	if currentBatch.Len() > 0 {
		batches = append(batches, currentBatch.String())
	}
	return batches
}
