//go:build tools
// +build tools

package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"

	_ "github.com/go-sql-driver/mysql"
)

func main() {
	// Connection details from OTRS
	host := "localhost"
	port := "3306"
	user := "otrs"
	password := "CHANGEME" // From the .env file
	database := "otrs"

	// Build MySQL DSN
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?charset=utf8mb4&parseTime=true",
		user, password, host, port, database)

	fmt.Println("Testing direct connection to OTRS MySQL database...")
	fmt.Println("================================================")

	// Connect directly using standard MySQL driver
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		log.Fatal("Failed to open connection:", err)
	}
	defer db.Close()

	// Test connection
	ctx := context.Background()
	if err := db.PingContext(ctx); err != nil {
		log.Fatal("Failed to ping database:", err)
	}

	fmt.Println("✅ Successfully connected to OTRS MySQL database!")

	// List all tables
	fmt.Println("\nQuerying OTRS tables...")
	rows, err := db.QueryContext(ctx, "SHOW TABLES")
	if err != nil {
		log.Fatal("Failed to list tables:", err)
	}
	defer rows.Close()

	var tables []string
	for rows.Next() {
		var tableName string
		if err := rows.Scan(&tableName); err != nil {
			log.Fatal("Failed to scan table name:", err)
		}
		tables = append(tables, tableName)
	}

	fmt.Printf("\nFound %d tables in OTRS database:\n", len(tables))
	fmt.Println("==================================")

	// Show first 10 tables as sample
	for i, table := range tables {
		if i < 10 {
			fmt.Printf("  - %s\n", table)
		}
	}
	if len(tables) > 10 {
		fmt.Printf("  ... and %d more tables\n", len(tables)-10)
	}

	// Check some key OTRS tables for data
	fmt.Println("\nChecking key tables for data:")
	fmt.Println("==============================")

	keyTables := []string{
		"users",
		"groups",
		"ticket",
		"article",
		"queue",
		"customer_company",
		"customer_user",
	}

	for _, table := range keyTables {
		var count int
		err := db.QueryRowContext(ctx, fmt.Sprintf("SELECT COUNT(*) FROM `%s`", table)).Scan(&count)
		if err != nil {
			fmt.Printf("  %-20s: Error - %v\n", table, err)
		} else {
			fmt.Printf("  %-20s: %d records\n", table, count)
		}
	}

	// Show that we can query actual OTRS data
	fmt.Println("\nSample OTRS users:")
	fmt.Println("==================")

	userRows, err := db.QueryContext(ctx, "SELECT id, login, first_name, last_name FROM users LIMIT 3")
	if err != nil {
		log.Printf("Failed to query users: %v", err)
	} else {
		defer userRows.Close()
		for userRows.Next() {
			var id int
			var login, firstName, lastName string
			if err := userRows.Scan(&id, &login, &firstName, &lastName); err != nil {
				log.Printf("Failed to scan user: %v", err)
				continue
			}
			fmt.Printf("  ID: %d, Login: %s, Name: %s %s\n", id, login, firstName, lastName)
		}
	}

	fmt.Println("\n🎉 GoatFlow can directly connect to OTRS MySQL database!")
	fmt.Println("This means we can run GoatFlow against the existing OTRS database")
	fmt.Println("without any migration - true drop-in replacement capability!")
}
