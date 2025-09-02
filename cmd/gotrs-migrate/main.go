package main

import (
	"bufio"
	"context"
	"database/sql"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	_ "github.com/lib/pq"
)

func main() {
	var (
		command  = flag.String("cmd", "", "Command: analyze, import, validate")
		sqlFile  = flag.String("sql", "", "Path to OTRS SQL dump file")
		dbURL    = flag.String("db", "", "PostgreSQL connection URL")
		verbose  = flag.Bool("v", false, "Verbose output")
		dryRun   = flag.Bool("dry-run", false, "Show what would be imported without executing")
		force    = flag.Bool("force", false, "Force import by clearing existing data (DESTRUCTIVE!)")
	)

	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "GOTRS Migration Tool - Import OTRS data\n\n")
		fmt.Fprintf(os.Stderr, "Usage: %s [options]\n\n", filepath.Base(os.Args[0]))
		fmt.Fprintf(os.Stderr, "Commands:\n")
		fmt.Fprintf(os.Stderr, "  analyze   Analyze OTRS SQL dump file\n")
		fmt.Fprintf(os.Stderr, "  import    Import OTRS data to GOTRS database\n")
		fmt.Fprintf(os.Stderr, "  validate  Validate imported data integrity\n")
		fmt.Fprintf(os.Stderr, "\nOptions:\n")
		flag.PrintDefaults()
		fmt.Fprintf(os.Stderr, "\nExamples:\n")
		fmt.Fprintf(os.Stderr, "  # Analyze OTRS dump\n")
		fmt.Fprintf(os.Stderr, "  %s -cmd=analyze -sql=otrs_dump.sql\n\n", filepath.Base(os.Args[0]))
		fmt.Fprintf(os.Stderr, "  # Import to GOTRS (dry run)\n")
		fmt.Fprintf(os.Stderr, "  %s -cmd=import -sql=otrs_dump.sql -db=postgres://user:pass@localhost/gotrs -dry-run\n\n", filepath.Base(os.Args[0]))
		fmt.Fprintf(os.Stderr, "  # Import to GOTRS\n")
		fmt.Fprintf(os.Stderr, "  %s -cmd=import -sql=otrs_dump.sql -db=postgres://user:pass@localhost/gotrs\n\n", filepath.Base(os.Args[0]))
	}

	flag.Parse()

	if *command == "" {
		flag.Usage()
		log.Fatal("Command is required")
	}

	if *sqlFile == "" && *command != "validate" {
		log.Fatal("SQL file is required")
	}

	switch *command {
	case "analyze":
		err := analyzeSQLDump(*sqlFile, *verbose)
		if err != nil {
			log.Fatalf("Analysis failed: %v", err)
		}
	case "import":
		if *dbURL == "" {
			// Try environment variable
			*dbURL = os.Getenv("DATABASE_URL")
			if *dbURL == "" {
				log.Fatal("Database URL is required (use -db or DATABASE_URL env var)")
			}
		}
		// Use the fixed import that handles ID mapping correctly
		err := importSQLDumpFixed(*sqlFile, *dbURL, *verbose, *dryRun, *force)
		if err != nil {
			log.Fatalf("Import failed: %v", err)
		}
	case "validate":
		if *dbURL == "" {
			*dbURL = os.Getenv("DATABASE_URL")
			if *dbURL == "" {
				log.Fatal("Database URL is required for validation")
			}
		}
		err := validateImportedData(*dbURL, *verbose)
		if err != nil {
			log.Fatalf("Validation failed: %v", err)
		}
	default:
		log.Fatalf("Unknown command: %s", *command)
	}
}

type TableInfo struct {
	Name       string
	RowCount   int
	HasData    bool
	Columns    []string
	SampleData map[string]string
}

func analyzeSQLDump(sqlFile string, verbose bool) error {
	fmt.Printf("🔍 Analyzing OTRS SQL dump: %s\n", sqlFile)

	file, err := os.Open(sqlFile)
	if err != nil {
		return fmt.Errorf("failed to open SQL file: %w", err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	// Increase buffer size to handle very long SQL lines
	buf := make([]byte, 0, 1024*1024) // 1MB buffer
	scanner.Buffer(buf, 10*1024*1024) // 10MB max token size
	
	tables := make(map[string]*TableInfo)
	totalLines := 0
	insertCount := 0

	// Patterns to match SQL statements
	createTablePattern := regexp.MustCompile(`CREATE TABLE (?:IF NOT EXISTS )?` + "`" + `([^` + "`" + `]+)` + "`")
	insertPattern := regexp.MustCompile(`INSERT INTO ` + "`" + `([^` + "`" + `]+)` + "`")
	valuesPattern := regexp.MustCompile(`VALUES\s*\(([^)]+)\)`)

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		totalLines++

		// Skip comments and empty lines
		if strings.HasPrefix(line, "--") || strings.HasPrefix(line, "/*") || line == "" {
			continue
		}

		// Match CREATE TABLE statements
		if match := createTablePattern.FindStringSubmatch(line); match != nil {
			tableName := match[1]
			tables[tableName] = &TableInfo{
				Name:       tableName,
				RowCount:   0,
				HasData:    false,
				SampleData: make(map[string]string),
			}
			if verbose {
				fmt.Printf("  Found table: %s\n", tableName)
			}
		}

		// Match INSERT statements
		if match := insertPattern.FindStringSubmatch(line); match != nil {
			tableName := match[1]
			insertCount++

			// Ensure table exists in our map
			if tables[tableName] == nil {
				tables[tableName] = &TableInfo{
					Name:       tableName,
					RowCount:   0,
					HasData:    false,
					SampleData: make(map[string]string),
				}
			}

			tables[tableName].HasData = true
			
			// Count the number of value sets (rows) in this INSERT
			// Look for pattern: VALUES (row1),(row2),(row3)...
			valuesIdx := strings.Index(line, "VALUES ")
			if valuesIdx >= 0 {
				valuesStr := line[valuesIdx+7:]
				// Count opening parentheses at depth 1
				depth := 0
				rowCount := 0
				inQuote := false
				escaped := false
				
				for _, r := range valuesStr {
					if escaped {
						escaped = false
						continue
					}
					if r == '\\' {
						escaped = true
						continue
					}
					if r == '\'' {
						inQuote = !inQuote
						continue
					}
					if !inQuote {
						if r == '(' {
							depth++
							if depth == 1 {
								rowCount++
							}
						} else if r == ')' {
							depth--
						}
					}
				}
				
				tables[tableName].RowCount += rowCount
				
				// Extract sample data from first value set
				if len(tables[tableName].SampleData) == 0 {
					if valuesMatch := valuesPattern.FindStringSubmatch(line); valuesMatch != nil {
						tables[tableName].SampleData["sample"] = valuesMatch[1]
					}
				}
			} else {
				// Fallback for single row INSERT
				tables[tableName].RowCount++
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("error reading file: %w", err)
	}

	// Display analysis results
	fmt.Printf("\n📊 Analysis Results:\n")
	fmt.Printf("  Total lines: %d\n", totalLines)
	fmt.Printf("  Total tables: %d\n", len(tables))
	fmt.Printf("  Tables with data: %d\n", countTablesWithData(tables))
	fmt.Printf("  Total INSERT statements: %d\n", insertCount)

	fmt.Printf("\n📋 Table Summary:\n")
	fmt.Printf("%-30s %-8s %-10s %s\n", "Table", "Rows", "Has Data", "Sample Data")
	fmt.Printf("%s\n", strings.Repeat("-", 80))

	for _, table := range tables {
		hasDataStr := "No"
		if table.HasData {
			hasDataStr = "Yes"
		}

		sampleStr := ""
		if sample, ok := table.SampleData["sample"]; ok {
			if len(sample) > 30 {
				sampleStr = sample[:30] + "..."
			} else {
				sampleStr = sample
			}
		}

		fmt.Printf("%-30s %-8d %-10s %s\n", table.Name, table.RowCount, hasDataStr, sampleStr)
	}

	// Check for important OTRS tables
	fmt.Printf("\n🎯 OTRS Core Tables Check:\n")
	coreOTRSTables := []string{
		"users", "groups", "queue", "ticket", "article", "ticket_history",
		"customer_user", "customer_company", "ticket_state", "ticket_priority",
	}

	for _, coreTable := range coreOTRSTables {
		if table, exists := tables[coreTable]; exists {
			status := "❌ No data"
			if table.HasData {
				status = fmt.Sprintf("✅ %d rows", table.RowCount)
			}
			fmt.Printf("  %-20s %s\n", coreTable, status)
		} else {
			fmt.Printf("  %-20s ❓ Table not found\n", coreTable)
		}
	}

	return nil
}

func countTablesWithData(tables map[string]*TableInfo) int {
	count := 0
	for _, table := range tables {
		if table.HasData {
			count++
		}
	}
	return count
}

func importSQLDump(sqlFile, dbURL string, verbose, dryRun bool) error {
	if dryRun {
		fmt.Printf("🧪 DRY RUN: Analyzing import process for %s\n", sqlFile)
	} else {
		fmt.Printf("📥 Importing OTRS data from %s\n", sqlFile)
	}

	// Connect to database
	var db *sql.DB
	if !dryRun {
		var err error
		db, err = sql.Open("postgres", dbURL)
		if err != nil {
			return fmt.Errorf("failed to connect to database: %w", err)
		}
		defer db.Close()

		// Test connection
		if err := db.Ping(); err != nil {
			return fmt.Errorf("failed to ping database: %w", err)
		}

		fmt.Printf("✅ Connected to database\n")
	}

	// Read and process SQL file
	file, err := os.Open(sqlFile)
	if err != nil {
		return fmt.Errorf("failed to open SQL file: %w", err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	// Increase buffer size to handle very long SQL lines
	buf := make([]byte, 0, 1024*1024) // 1MB buffer
	scanner.Buffer(buf, 10*1024*1024) // 10MB max token size
	
	processedStatements := 0
	skippedStatements := 0

	// Track which tables we're importing
	importedTables := make(map[string]int)

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		// Skip comments and empty lines
		if strings.HasPrefix(line, "--") || strings.HasPrefix(line, "/*") || 
		   strings.HasPrefix(line, "SET") || strings.HasPrefix(line, "DROP") ||
		   strings.HasPrefix(line, "CREATE DATABASE") || strings.HasPrefix(line, "USE") ||
		   line == "" {
			skippedStatements++
			continue
		}

		// Process INSERT statements
		if strings.HasPrefix(line, "INSERT INTO") {
			convertedSQL, tableName, err := convertMySQLToPostgreSQL(line)
			if err != nil {
				if verbose {
					fmt.Printf("⚠️  Warning: Failed to convert statement: %v\n", err)
				}
				skippedStatements++
				continue
			}

			if convertedSQL == "" {
				// Statement was intentionally skipped
				skippedStatements++
				continue
			}

			importedTables[tableName]++
			processedStatements++

			if verbose {
				fmt.Printf("  Processing: %s (%d rows so far)\n", tableName, importedTables[tableName])
			}

			// Execute the statement (unless dry run)
			if !dryRun {
				// Handle multiple statements separated by semicolons
				statements := strings.Split(convertedSQL, ";\n")
				successCount := 0
				
				for _, stmt := range statements {
					stmt = strings.TrimSpace(stmt)
					if stmt == "" {
						continue
					}
					
					_, err := db.ExecContext(context.Background(), stmt)
					if err != nil {
						if verbose {
							fmt.Printf("⚠️  Warning: Failed to execute statement for %s: %v\n", tableName, err)
						}
						// Continue with other statements even if one fails
						continue
					} else {
						successCount++
					}
				}
				
				// Update count based on successful statements
				if successCount > 0 {
					importedTables[tableName] += successCount - 1 // Adjust for double counting
				}
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("error reading SQL file: %w", err)
	}

	fmt.Printf("\n📊 Import Summary:\n")
	fmt.Printf("  Processed statements: %d\n", processedStatements)
	fmt.Printf("  Skipped statements: %d\n", skippedStatements)
	fmt.Printf("  Imported tables: %d\n", len(importedTables))

	fmt.Printf("\n📋 Imported Data:\n")
	for table, count := range importedTables {
		fmt.Printf("  %-20s %d rows\n", table, count)
	}

	if dryRun {
		fmt.Printf("\n🧪 DRY RUN COMPLETE - No data was actually imported\n")
	} else {
		fmt.Printf("\n✅ Import completed successfully\n")
	}

	return nil
}

func convertMySQLToPostgreSQL(mysqlSQL string) (string, string, error) {
	// Extract table name
	insertPattern := regexp.MustCompile(`INSERT INTO ` + "`" + `([^` + "`" + `]+)` + "`")
	match := insertPattern.FindStringSubmatch(mysqlSQL)
	if match == nil {
		return "", "", fmt.Errorf("could not extract table name")
	}
	
	tableName := match[1]

	// Skip tables that don't exist in GOTRS schema or cause conflicts
	skipTables := map[string]bool{
		"schema_migrations": true,
		"sessions":          true,
		"web_upload_cache":  true,
		// We now want to import users and tickets
	}

	if skipTables[tableName] {
		return "", tableName, nil // Return empty string to indicate skip
	}

	// Convert MySQL backticks to PostgreSQL double quotes for identifiers
	converted := strings.ReplaceAll(mysqlSQL, "`", "\"")
	
	// Handle MySQL-specific escape sequences
	converted = strings.ReplaceAll(converted, `\'`, `''`) // Single quote escaping
	converted = strings.ReplaceAll(converted, `\"`, `"`)  // Double quote handling
	
	// Fix the main issue: OTRS includes explicit IDs but we use auto-generated IDs
	// Convert "INSERT INTO table VALUES (id,..." to "INSERT INTO table (columns...) VALUES (..."
	if strings.Contains(strings.ToUpper(converted), "INSERT INTO") && strings.Contains(converted, "VALUES (") {
		converted = convertInsertStatement(converted, tableName)
	}
	
	// Convert MySQL LOCK TABLES statements (skip them)
	if strings.Contains(strings.ToUpper(converted), "LOCK TABLES") {
		return "", tableName, nil
	}
	
	// Convert MySQL UNLOCK TABLES statements (skip them) 
	if strings.Contains(strings.ToUpper(converted), "UNLOCK TABLES") {
		return "", tableName, nil
	}
	
	// Handle ON DUPLICATE KEY UPDATE (PostgreSQL uses ON CONFLICT)
	if strings.Contains(strings.ToUpper(converted), "ON DUPLICATE KEY UPDATE") {
		// For now, skip these complex statements
		return "", tableName, nil
	}

	// Add ON CONFLICT DO NOTHING for tables that might have existing data
	conflictTables := map[string]bool{
		"groups":                true,
		"queue":                 true,
		"ticket_state":          true,
		"ticket_priority":       true,
		"valid":                 true,
		"salutation":            true,
		"signature":             true,
		"auto_response_type":    true,
		"follow_up_possible":    true,
		"communication_channel": true,
		"article_sender_type":   true,
		"ticket_lock_type":      true,
		"ticket_state_type":     true,
		"ticket_history_type":   true,
		"ticket_type":           true,
		"link_state":            true,
		"link_type":             true,
		"link_object":           true,
	}
	
	if conflictTables[tableName] {
		// Add ON CONFLICT clause to handle existing data
		if !strings.Contains(strings.ToUpper(converted), "ON CONFLICT") {
			converted = strings.TrimSuffix(converted, ";")
			converted += " ON CONFLICT DO NOTHING;"
		}
	}
	
	return converted, tableName, nil
}

func convertInsertStatement(sql, tableName string) string {
	// For tables with auto-increment IDs, we need to remove the ID and add explicit column names
	tablesNeedingColumnMapping := map[string][]string{
		"ticket": {
			"tn", "title", "queue_id", "ticket_lock_id", "type_id", "service_id", "sla_id", 
			"user_id", "responsible_user_id", "ticket_priority_id", "ticket_state_id", 
			"customer_id", "customer_user_id", "timeout", "until_time", "escalation_time", 
			"escalation_update_time", "escalation_response_time", "escalation_solution_time", 
			"archive_flag", "create_time", "create_by", "change_time", "change_by",
		},
		"article_data_mime_plain": {
			"article_id", "body", "create_time", "create_by", "change_time", "change_by",
		},
		"article_data_mime_send_error": {
			"article_id", "message_id", "log_message", "create_time",
		},
		"article_search_index": {
			"ticket_id", "article_id", "article_key", "article_value",
		},
		"auto_response_type": {
			"name", "comments", "valid_id", "create_time", "create_by", "change_time", "change_by",
		},
		"auto_response": {
			"name", "text0", "text1", "text2", "type_id", "system_address_id", "charset", "content_type", "comments", "valid_id", "create_time", "create_by", "change_time", "change_by",
		},
		"communication_channel": {
			"name", "module", "package_name", "channel_data", "valid_id", "create_time", "create_by", "change_time", "change_by",
		},
		"communication_log": {
			"transport", "direction", "status", "account_type", "account_id", "object_log_type", "object_log_id", "communication_id", "create_time",
		},
		"communication_log_object": {
			"communication_id", "object_type", "status",
		},
		"communication_log_object_entry": {
			"communication_log_object_id", "log_key", "log_value", "priority",
		},
		"communication_log_obj_lookup": {
			"communication_log_object_id", "object_type", "object_id",
		},
		"dynamic_field": {
			"internal_field", "name", "label", "field_order", "field_type", "object_type", "config", "valid_id", "create_time", "create_by", "change_time", "change_by",
		},
		"notification_event": {
			"name", "subject", "text", "content_type", "charset", "valid_id", "comments", "create_time", "create_by", "change_time", "change_by",
		},
		"notification_event_message": {
			"notification_id", "subject", "text", "content_type", "language",
		},
		"scheduler_recurrent_task": {
			"name", "task_type", "task_data", "attempts", "lock_key", "lock_time", "lock_update_time", "create_time",
		},
		"standard_attachment": {
			"name", "content_type", "content", "filename", "comments", "valid_id", "create_time", "create_by", "change_time", "change_by",
		},
		"standard_template": {
			"name", "text", "content_type", "template_type", "comments", "valid_id", "create_time", "create_by", "change_time", "change_by",
		},
		"sysconfig_default": {
			"name", "description", "navigation", "is_invisible", "is_readonly", "is_required", "is_valid", "has_configlevel", "user_modification_possible", "user_modification_active", "user_preferences_group", "xml_content_raw", "xml_content_parsed", "xml_filename", "effective_value", "is_dirty", "exclusive_lock_guid", "exclusive_lock_user_id", "exclusive_lock_expiry_time", "create_time", "create_by", "change_time", "change_by",
		},
		"sysconfig_default_version": {
			"sysconfig_default_id", "name", "description", "navigation", "is_invisible", "is_readonly", "is_required", "is_valid", "has_configlevel", "user_modification_possible", "user_modification_active", "user_preferences_group", "xml_content_raw", "xml_content_parsed", "xml_filename", "effective_value", "create_time", "create_by",
		},
		"sysconfig_modified": {
			"sysconfig_default_id", "name", "user_id", "is_valid", "user_modification_active", "effective_value", "reset_to_default", "is_dirty", "create_time", "create_by", "change_time", "change_by",
		},
		"sysconfig_modified_version": {
			"sysconfig_version_id", "name", "user_id", "is_valid", "user_modification_active", "effective_value", "reset_to_default", "create_time", "create_by",
		},
		"sysconfig_deployment": {
			"comments", "user_id", "effective_value", "create_time", "create_by",
		},
		"time_accounting": {
			"ticket_id", "article_id", "time_unit", "create_time", "create_by", "change_time", "change_by",
		},
		"ticket_number_counter": {
			"counter", "content_path", "create_time", "create_by", "change_time", "change_by",
		},
		"system_address": {
			"value0", "value1", "comments", "valid_id", "create_time", "create_by", "change_time", "change_by",
		},
	}
	
	columns, exists := tablesNeedingColumnMapping[tableName]
	if !exists {
		return sql // No special handling needed
	}
	
	// For complex multi-value INSERT statements, split them into individual statements
	// This handles: INSERT INTO table VALUES (1,'a',...),(2,'b',...),(3,'c',...)
	
	valuesIndex := strings.Index(strings.ToUpper(sql), "VALUES")
	if valuesIndex == -1 {
		return sql
	}
	
	valuesSection := sql[valuesIndex+6:] // Skip "VALUES"
	valuesSection = strings.TrimSpace(valuesSection)
	
	// For simplicity, split the complex statement into individual INSERTs
	// This is a basic approach - for production we'd want more robust parsing
	columnList := strings.Join(columns, ",")
	
	// Find all value tuples by looking for patterns like (value,value,...)
	result := ""
	depth := 0
	tupleStart := -1
	
	for i, char := range valuesSection {
		if char == '(' && depth == 0 {
			tupleStart = i
		}
		
		if char == '(' {
			depth++
		} else if char == ')' {
			depth--
			if depth == 0 && tupleStart != -1 {
				// Extract this tuple
				tupleContent := valuesSection[tupleStart+1 : i]
				
				// Split by comma and skip the first value (ID)
				values := strings.Split(tupleContent, ",")
				if len(values) >= len(columns)+1 {
					// Remove the ID value (first one)
					adjustedValues := values[1:len(columns)+1]
					valuesList := strings.Join(adjustedValues, ",")
					
					if result != "" {
						result += ";\n"
					}
					result += fmt.Sprintf(`INSERT INTO "%s" (%s) VALUES (%s)`, tableName, columnList, valuesList)
				}
				tupleStart = -1
			}
		}
	}
	
	if result != "" {
		return result
	}
	
	return sql // Fallback to original
}

func validateImportedData(dbURL string, verbose bool) error {
	fmt.Printf("🔍 Validating imported OTRS data\n")

	db, err := sql.Open("postgres", dbURL)
	if err != nil {
		return fmt.Errorf("failed to connect to database: %w", err)
	}
	defer db.Close()

	if err := db.Ping(); err != nil {
		return fmt.Errorf("failed to ping database: %w", err)
	}

	fmt.Printf("✅ Connected to database\n")

	// Check core tables exist and have data
	coreTables := []string{
		"users", "groups", "queue", "ticket", "article", 
		"customer_user", "customer_company",
	}

	fmt.Printf("\n📊 Data Validation:\n")
	ctx := context.Background()
	totalRows := 0

	for _, table := range coreTables {
		var count int
		query := fmt.Sprintf("SELECT COUNT(*) FROM %s", table)
		
		err := db.QueryRowContext(ctx, query).Scan(&count)
		if err != nil {
			fmt.Printf("  %-20s ❌ Error: %v\n", table, err)
			continue
		}

		status := "✅"
		if count == 0 {
			status = "⚠️  Empty"
		}

		fmt.Printf("  %-20s %s %d rows\n", table, status, count)
		totalRows += count
	}

	fmt.Printf("\n📈 Total imported rows: %d\n", totalRows)

	// Check for data integrity
	fmt.Printf("\n🔗 Data Integrity Checks:\n")

	// Check if tickets have corresponding articles
	var ticketsWithoutArticles int
	query := `SELECT COUNT(*) FROM ticket t WHERE NOT EXISTS (SELECT 1 FROM article a WHERE a.ticket_id = t.id)`
	err = db.QueryRowContext(ctx, query).Scan(&ticketsWithoutArticles)
	if err != nil {
		fmt.Printf("  Tickets without articles: ❌ Error checking\n")
	} else {
		status := "✅"
		if ticketsWithoutArticles > 0 {
			status = "⚠️ "
		}
		fmt.Printf("  Tickets without articles: %s %d\n", status, ticketsWithoutArticles)
	}

	// Check if customer users have companies
	var customersWithoutCompany int
	query = `SELECT COUNT(*) FROM customer_user cu WHERE cu.customer_id = '' OR cu.customer_id IS NULL`
	err = db.QueryRowContext(ctx, query).Scan(&customersWithoutCompany)
	if err != nil {
		fmt.Printf("  Customers without company: ❌ Error checking\n")
	} else {
		status := "✅"
		if customersWithoutCompany > 0 {
			status = "⚠️ "
		}
		fmt.Printf("  Customers without company: %s %d\n", status, customersWithoutCompany)
	}

	fmt.Printf("\n✅ Validation completed\n")
	return nil
}