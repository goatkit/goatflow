// Package main provides fixed import utilities for migration.
package main

import (
	"bufio"
	"database/sql"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/goatkit/goatflow/internal/database"
)

// IDMapping tracks the mapping between old IDs and new IDs during import.
type IDMapping struct {
	TicketIDMap  map[int]int // oldID -> newID
	ArticleIDMap map[int]int // oldID -> newID
}

func NewIDMapping() *IDMapping {
	return &IDMapping{
		TicketIDMap:  make(map[int]int),
		ArticleIDMap: make(map[int]int),
	}
}

func importSQLDumpFixed(sqlFile, dbURL string, verbose, dryRun, force bool) error {
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

		if err := db.Ping(); err != nil {
			return fmt.Errorf("failed to ping database: %w", err)
		}
		fmt.Printf("✅ Connected to database\n")

		// Validate database state before import
		if err := validateDatabaseState(db, force); err != nil {
			return err
		}
	}

	// Create ID mapping
	idMap := NewIDMapping()

	// Read SQL file
	file, err := os.Open(sqlFile) //nolint:gosec // G304 CLI tool
	if err != nil {
		return fmt.Errorf("failed to open SQL file: %w", err)
	}
	defer func() { _ = file.Close() }()

	scanner := bufio.NewScanner(file)
	buf := make([]byte, 0, 1024*1024)
	scanner.Buffer(buf, 10*1024*1024)

	// First pass: Import tickets and build ID mapping
	fmt.Printf("\n📊 Phase 1: Importing tickets and building ID mapping...\n")
	if err := importTickets(scanner, db, idMap, verbose, dryRun); err != nil {
		return fmt.Errorf("failed to import tickets: %w", err)
	}

	// Reset scanner for second pass
	file.Seek(0, 0)
	scanner = bufio.NewScanner(file)
	scanner.Buffer(buf, 10*1024*1024)

	// Second pass: Import articles with corrected ticket IDs
	fmt.Printf("\n📊 Phase 2: Importing articles with correct ticket mappings...\n")
	if err := importArticles(scanner, db, idMap, verbose, dryRun); err != nil {
		return fmt.Errorf("failed to import articles: %w", err)
	}

	// Reset scanner for third pass
	file.Seek(0, 0)
	scanner = bufio.NewScanner(file)
	scanner.Buffer(buf, 10*1024*1024)

	// Third pass: Import other tables
	fmt.Printf("\n📊 Phase 3: Importing other tables...\n")
	if err := importOtherTables(scanner, db, idMap, verbose, dryRun); err != nil {
		return fmt.Errorf("failed to import other tables: %w", err)
	}

	// Fix sequences
	if !dryRun {
		fmt.Printf("\n🔧 Fixing sequences...\n")
		fixSequences(db)
	}

	fmt.Printf("\n✅ Import completed successfully\n")
	fmt.Printf("📊 Summary:\n")
	fmt.Printf("  Tickets imported: %d\n", len(idMap.TicketIDMap))
	fmt.Printf("  Articles imported: %d\n", len(idMap.ArticleIDMap))

	return nil
}

func importTickets(scanner *bufio.Scanner, db *sql.DB, idMap *IDMapping, verbose, dryRun bool) error {
	ticketCount := 0

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		if !strings.Contains(line, "INSERT INTO `ticket`") {
			continue
		}

		// Extract everything after VALUES
		valuesIdx := strings.Index(line, "VALUES ")
		if valuesIdx == -1 {
			continue
		}

		valuesStr := line[valuesIdx+7:] // Skip "VALUES "
		valuesStr = strings.TrimSuffix(valuesStr, ";")

		// Parse multiple value sets: (1,2,3),(4,5,6)...
		valueSets := parseMultipleValueSets(valuesStr)

		for _, valueSet := range valueSets {
			values := parseValues(valueSet)
			if len(values) < 25 {
				continue
			}

			// Extract old ID
			oldID, err := strconv.Atoi(values[0])
			if err != nil {
				continue
			}

			// Build INSERT without ID (let it auto-generate)
			insertSQL := `INSERT INTO ticket (
			tn, title, queue_id, ticket_lock_id, type_id, service_id, sla_id,
			user_id, responsible_user_id, ticket_priority_id, ticket_state_id,
			customer_id, customer_user_id, timeout, until_time,
			escalation_time, escalation_update_time, escalation_response_time, escalation_solution_time,
			archive_flag, create_time, create_by, change_time, change_by
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19, $20, $21, $22, $23, $24) RETURNING id`

			if !dryRun {
				var newID int
				err := db.QueryRow(insertSQL,
					values[1], values[2], parseIntOrNull(values[3]), parseIntOrNull(values[4]),
					parseIntOrNull(values[5]), parseIntOrNull(values[6]), parseIntOrNull(values[7]),
					parseIntOrNull(values[8]), parseIntOrNull(values[9]), parseIntOrNull(values[10]),
					parseIntOrNull(values[11]), values[12], values[13], parseIntOrNull(values[14]),
					parseIntOrNull(values[15]), parseIntOrNull(values[16]), parseIntOrNull(values[17]),
					parseIntOrNull(values[18]), parseIntOrNull(values[19]), parseIntOrNull(values[20]),
					values[21], parseIntOrNull(values[22]), values[23], parseIntOrNull(values[24]),
				).Scan(&newID)

				if err != nil {
					if verbose {
						log.Printf("Warning: Failed to insert ticket %d: %v", oldID, err)
					}
					continue
				}

				// Store the mapping
				idMap.TicketIDMap[oldID] = newID
				ticketCount++

				if verbose {
					fmt.Printf("  Ticket %d -> %d (TN: %s)\n", oldID, newID, values[1])
				}
			} else {
				// In dry run, simulate mapping
				idMap.TicketIDMap[oldID] = oldID
				ticketCount++
			}
		}
	}

	fmt.Printf("  ✅ Imported %d tickets\n", ticketCount)
	return scanner.Err()
}

func importArticles(scanner *bufio.Scanner, db *sql.DB, idMap *IDMapping, verbose, dryRun bool) error {
	articleCount := 0

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		if !strings.Contains(line, "INSERT INTO `article`") {
			continue
		}

		// Extract everything after VALUES
		valuesIdx := strings.Index(line, "VALUES ")
		if valuesIdx == -1 {
			continue
		}

		valuesStr := line[valuesIdx+7:] // Skip "VALUES "
		valuesStr = strings.TrimSuffix(valuesStr, ";")

		// Parse multiple value sets
		valueSets := parseMultipleValueSets(valuesStr)

		for _, valueSet := range valueSets {
			values := parseValues(valueSet)
			if len(values) < 10 {
				continue
			}

			// Extract old article ID and ticket ID
			oldArticleID, _ := strconv.Atoi(values[0])
			oldTicketID, _ := strconv.Atoi(values[1])

			// Get new ticket ID from mapping
			newTicketID, exists := idMap.TicketIDMap[oldTicketID]
			if !exists {
				if verbose {
					log.Printf("Warning: No mapping for ticket ID %d, skipping article %d", oldTicketID, oldArticleID)
				}
				continue
			}

			// Build INSERT without ID but with mapped ticket_id
			insertSQL := `INSERT INTO article (
			ticket_id, article_sender_type_id, communication_channel_id, 
			is_visible_for_customer, search_index_needs_rebuild,
			insert_fingerprint, create_time, create_by, change_time, change_by
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10) RETURNING id`

			if !dryRun {
				var newArticleID int
				err := db.QueryRow(insertSQL,
					newTicketID, // Use mapped ticket ID
					parseIntOrNull(values[2]), parseIntOrNull(values[3]),
					parseIntOrNull(values[4]), parseIntOrNull(values[5]),
					values[6], values[7], parseIntOrNull(values[8]),
					values[9], parseIntOrNull(values[10]),
				).Scan(&newArticleID)

				if err != nil {
					if verbose {
						log.Printf("Warning: Failed to insert article %d: %v", oldArticleID, err)
					}
					continue
				}

				// Store the mapping
				idMap.ArticleIDMap[oldArticleID] = newArticleID
				articleCount++

				if verbose {
					fmt.Printf("  Article %d -> %d (for ticket %d -> %d)\n",
						oldArticleID, newArticleID, oldTicketID, newTicketID)
				}
			} else {
				idMap.ArticleIDMap[oldArticleID] = oldArticleID
				articleCount++
			}
		}
	}

	// Now import article_data_mime with corrected article IDs
	// Reset scanner
	if err := importArticleDataMime(db, idMap, verbose, dryRun); err != nil {
		return err
	}

	fmt.Printf("  ✅ Imported %d articles\n", articleCount)
	return scanner.Err()
}

func importArticleDataMime(db *sql.DB, idMap *IDMapping, verbose, dryRun bool) error {
	// This would need the SQL file to be re-read
	// For now, we'll handle it in the third pass
	return nil
}

func importOtherTables(scanner *bufio.Scanner, db *sql.DB, idMap *IDMapping, verbose, dryRun bool) error {
	mimeCount := 0
	historyCount := 0
	customerCount := 0
	queueCount := 0
	groupCount := 0
	userCount := 0
	priorityCount := 0
	stateCount := 0

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		// Handle article_data_mime with corrected article IDs
		if strings.Contains(line, "INSERT INTO `article_data_mime`") {
			// Extract everything after VALUES
			valuesIdx := strings.Index(line, "VALUES ")
			if valuesIdx == -1 {
				continue
			}

			valuesStr := line[valuesIdx+7:]
			valuesStr = strings.TrimSuffix(valuesStr, ";")
			valueSets := parseMultipleValueSets(valuesStr)

			for _, valueSet := range valueSets {
				values := parseValues(valueSet)
				if len(values) < 20 {
					continue
				}

				oldArticleID, _ := strconv.Atoi(values[0])
				newArticleID, exists := idMap.ArticleIDMap[oldArticleID]
				if !exists {
					if verbose {
						log.Printf("Warning: No mapping for article ID %d in article_data_mime", oldArticleID)
					}
					continue
				}

				if !dryRun {
					_, err := db.Exec(database.ConvertPlaceholders(`
						INSERT INTO article_data_mime (
							article_id, a_from, a_reply_to, a_to, a_cc, a_bcc,
							a_subject, a_message_id, a_message_id_md5,
							a_in_reply_to, a_references, a_content_type, a_body,
							incoming_time, content_path, create_time, create_by,
							change_time, change_by
						) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19)
					`), newArticleID, values[2], parseNull(values[3]), parseNull(values[4]),
						parseNull(values[5]), parseNull(values[6]), values[7], parseNull(values[8]),
						parseNull(values[9]), parseNull(values[10]), parseNull(values[11]),
						parseNull(values[12]), values[13], parseIntOrNull(values[14]),
						parseNull(values[15]), values[16], parseIntOrNull(values[17]),
						values[18], parseIntOrNull(values[19]))

					if err != nil {
						if verbose {
							log.Printf("Warning: Failed to insert article_data_mime for article %d: %v", newArticleID, err)
						}
					} else {
						mimeCount++
					}
				} else {
					mimeCount++
				}
			}
		}

		// Handle customer_user
		if strings.Contains(line, "INSERT INTO `customer_user`") {
			valuesIdx := strings.Index(line, "VALUES ")
			if valuesIdx >= 0 {
				valuesStr := line[valuesIdx+7:]
				valuesStr = strings.TrimSuffix(valuesStr, ";")
				valueSets := parseMultipleValueSets(valuesStr)

				for _, valueSet := range valueSets {
					values := parseValues(valueSet)
					if len(values) < 19 {
						continue
					}

					if !dryRun {
						_, err := db.Exec(database.ConvertPlaceholders(`
							INSERT INTO customer_user (
								login, email, customer_id, pw, title, first_name, last_name,
								phone, fax, mobile, street, zip, city, country, comments,
								valid_id, create_time, create_by, change_time, change_by
							) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19, $20)
							ON CONFLICT (login) DO NOTHING
						`), values[1], values[2], values[3], parseNull(values[4]), parseNull(values[5]),
							parseNull(values[6]), parseNull(values[7]), parseNull(values[8]),
							parseNull(values[9]), parseNull(values[10]), parseNull(values[11]),
							parseNull(values[12]), parseNull(values[13]), parseNull(values[14]),
							parseNull(values[15]), parseIntOrNull(values[16]), values[17],
							parseIntOrNull(values[18]), values[19], parseIntOrNull(values[20]))

						if err != nil {
							if verbose {
								log.Printf("Warning: Failed to insert customer_user %s: %v", values[1], err)
							}
						} else {
							customerCount++
						}
					} else {
						customerCount++
					}
				}
			}
		}

		// Handle queue table
		if strings.Contains(line, "INSERT INTO `queue`") {
			valuesIdx := strings.Index(line, "VALUES ")
			if valuesIdx >= 0 {
				valuesStr := line[valuesIdx+7:]
				valuesStr = strings.TrimSuffix(valuesStr, ";")
				valueSets := parseMultipleValueSets(valuesStr)

				for _, valueSet := range valueSets {
					values := parseValues(valueSet)
					if len(values) < 17 {
						continue
					}

					if !dryRun {
						// queue structure: id, name, group_id, system_address_id, salutation_id, signature_id,
						// follow_up_id, follow_up_lock, unlock_timeout, calendar_name, default_sign_key,
						// comments, valid_id, create_time, create_by, change_time, change_by

						// Handle NULLs that violate NOT NULL constraints
						salutationID := parseIntOrNull(values[4])
						if salutationID == nil {
							salutationID = 1 // Default to 1
						}
						validID := parseIntOrNull(values[12])
						if validID == nil {
							validID = 1 // Default to valid
						}

						_, err := db.Exec(database.ConvertPlaceholders(`
							INSERT INTO queue (
								id, name, group_id, system_address_id, salutation_id, signature_id,
								follow_up_id, follow_up_lock, unlock_timeout, calendar_name, default_sign_key,
								comments, valid_id, create_time, create_by, change_time, change_by
							) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17)
							ON CONFLICT (id) DO UPDATE SET
								name = EXCLUDED.name,
								group_id = EXCLUDED.group_id,
								valid_id = EXCLUDED.valid_id
						`), parseIntOrNull(values[0]), values[1], parseIntOrNull(values[2]),
							parseIntOrNull(values[3]), salutationID, parseIntOrNull(values[5]),
							parseIntOrNull(values[6]), parseIntOrNull(values[7]), parseIntOrNull(values[8]),
							parseNull(values[9]), parseNull(values[10]), parseNull(values[11]),
							validID, parseTimestamp(values[13]),
							parseIntOrNull(values[14]), parseTimestamp(values[15]), parseIntOrNull(values[16]))

						if err != nil {
							if verbose {
								log.Printf("Warning: Failed to insert queue: %v", err)
							}
						} else {
							queueCount++
						}
					} else {
						queueCount++
					}
				}
			}
		}

		// Handle groups table
		if strings.Contains(line, "INSERT INTO `groups`") {
			valuesIdx := strings.Index(line, "VALUES ")
			if valuesIdx >= 0 {
				valuesStr := line[valuesIdx+7:]
				valuesStr = strings.TrimSuffix(valuesStr, ";")
				valueSets := parseMultipleValueSets(valuesStr)

				for _, valueSet := range valueSets {
					values := parseValues(valueSet)
					if len(values) < 7 {
						continue
					}

					if !dryRun {
						// groups structure: id, name, comments, valid_id, create_time, create_by, change_time, change_by
						_, err := db.Exec(database.ConvertPlaceholders(`
							INSERT INTO groups (
								id, name, comments, valid_id, create_time, create_by, change_time, change_by
							) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
							ON CONFLICT (id) DO UPDATE SET
								name = EXCLUDED.name,
								comments = EXCLUDED.comments,
								valid_id = EXCLUDED.valid_id
						`), parseIntOrNull(values[0]), values[1], parseNull(values[2]),
							parseIntOrNull(values[3]), parseTimestamp(values[4]),
							parseIntOrNull(values[5]), parseTimestamp(values[6]), parseIntOrNull(values[7]))

						if err != nil {
							if verbose {
								log.Printf("Warning: Failed to insert group: %v", err)
							}
						} else {
							groupCount++
						}
					} else {
						groupCount++
					}
				}
			}
		}

		// Handle users table
		if strings.Contains(line, "INSERT INTO `users`") {
			valuesIdx := strings.Index(line, "VALUES ")
			if valuesIdx >= 0 {
				valuesStr := line[valuesIdx+7:]
				valuesStr = strings.TrimSuffix(valuesStr, ";")
				valueSets := parseMultipleValueSets(valuesStr)

				for _, valueSet := range valueSets {
					values := parseValues(valueSet)
					if len(values) < 10 {
						continue
					}

					if !dryRun {
						// users structure: id, login, pw, title, first_name, last_name,
						// valid_id, create_time, create_by, change_time, change_by
						_, err := db.Exec(database.ConvertPlaceholders(`
							INSERT INTO users (
								id, login, pw, title, first_name, last_name,
								valid_id, create_time, create_by, change_time, change_by
							) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
							ON CONFLICT (id) DO UPDATE SET
								login = EXCLUDED.login,
								first_name = EXCLUDED.first_name,
								last_name = EXCLUDED.last_name,
								valid_id = EXCLUDED.valid_id
						`), parseIntOrNull(values[0]), values[1], values[2],
							parseNull(values[3]), parseNull(values[4]), parseNull(values[5]),
							parseIntOrNull(values[6]), parseTimestamp(values[7]),
							parseIntOrNull(values[8]), parseTimestamp(values[9]), parseIntOrNull(values[10]))

						if err != nil {
							if verbose {
								log.Printf("Warning: Failed to insert user: %v", err)
							}
						} else {
							userCount++
						}
					} else {
						userCount++
					}
				}
			}
		}

		// Handle ticket_priority table
		if strings.Contains(line, "INSERT INTO `ticket_priority`") {
			valuesIdx := strings.Index(line, "VALUES ")
			if valuesIdx >= 0 {
				valuesStr := line[valuesIdx+7:]
				valuesStr = strings.TrimSuffix(valuesStr, ";")
				valueSets := parseMultipleValueSets(valuesStr)

				for _, valueSet := range valueSets {
					values := parseValues(valueSet)
					if len(values) < 8 {
						continue
					}

					if !dryRun {
						// ticket_priority structure: id, name, valid_id, color, create_time, create_by, change_time, change_by
						_, err := db.Exec(database.ConvertPlaceholders(`
							INSERT INTO ticket_priority (
								id, name, valid_id, color, create_time, create_by, change_time, change_by
							) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
							ON CONFLICT (id) DO UPDATE SET
								name = EXCLUDED.name,
								valid_id = EXCLUDED.valid_id,
								color = EXCLUDED.color
						`), parseIntOrNull(values[0]), values[1], parseIntOrNull(values[2]), values[3],
							parseTimestamp(values[4]), parseIntOrNull(values[5]),
							parseTimestamp(values[6]), parseIntOrNull(values[7]))

						if err != nil {
							if verbose {
								log.Printf("Warning: Failed to insert ticket_priority: %v", err)
							}
						} else {
							priorityCount++
						}
					} else {
						priorityCount++
					}
				}
			}
		}

		// Handle ticket_state table
		if strings.Contains(line, "INSERT INTO `ticket_state`") {
			valuesIdx := strings.Index(line, "VALUES ")
			if valuesIdx >= 0 {
				valuesStr := line[valuesIdx+7:]
				valuesStr = strings.TrimSuffix(valuesStr, ";")
				valueSets := parseMultipleValueSets(valuesStr)

				for _, valueSet := range valueSets {
					values := parseValues(valueSet)
					if len(values) < 8 {
						continue
					}

					if !dryRun {
						// ticket_state structure: id, name, comments, type_id, valid_id, create_time, create_by, change_time, change_by
						_, err := db.Exec(database.ConvertPlaceholders(`
							INSERT INTO ticket_state (
								id, name, comments, type_id, valid_id, create_time, create_by, change_time, change_by
							) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
							ON CONFLICT (id) DO UPDATE SET
								name = EXCLUDED.name,
								comments = EXCLUDED.comments,
								type_id = EXCLUDED.type_id,
								valid_id = EXCLUDED.valid_id
						`), parseIntOrNull(values[0]), values[1], parseNull(values[2]),
							parseIntOrNull(values[3]), parseIntOrNull(values[4]),
							parseTimestamp(values[5]), parseIntOrNull(values[6]),
							parseTimestamp(values[7]), parseIntOrNull(values[8]))

						if err != nil {
							if verbose {
								log.Printf("Warning: Failed to insert ticket_state: %v", err)
							}
						} else {
							stateCount++
						}
					} else {
						stateCount++
					}
				}
			}
		}

		// Handle ticket_history with corrected ticket IDs
		if strings.Contains(line, "INSERT INTO `ticket_history`") {
			valuesIdx := strings.Index(line, "VALUES ")
			if valuesIdx >= 0 {
				valuesStr := line[valuesIdx+7:]
				valuesStr = strings.TrimSuffix(valuesStr, ";")
				valueSets := parseMultipleValueSets(valuesStr)

				for _, valueSet := range valueSets {
					values := parseValues(valueSet)
					if len(values) < 14 {
						continue
					}

					// ticket_history structure (14 columns total):
					// 0:id, 1:name, 2:history_type_id, 3:ticket_id, 4:article_id, 5:type_id, 6:queue_id,
					// 7:owner_id, 8:priority_id, 9:state_id, 10:create_time, 11:create_by,
					// 12:change_time, 13:change_by
					oldTicketID, _ := strconv.Atoi(values[3])
					newTicketID, exists := idMap.TicketIDMap[oldTicketID]
					if !exists {
						continue
					}

					oldArticleID := 0
					newArticleID := 0
					if values[4] != "NULL" && values[4] != "" {
						oldArticleID, _ = strconv.Atoi(values[4])
						if aid, ok := idMap.ArticleIDMap[oldArticleID]; ok {
							newArticleID = aid
						}
					}

					if !dryRun {
						var articleIDVal interface{} = nil
						if newArticleID > 0 {
							articleIDVal = newArticleID
						}

						// history_type_id is at index 2
						historyTypeID := parseIntOrNull(values[2])
						if historyTypeID == nil {
							historyTypeID = 1 // Default to 1 if missing
						}

						// Parse timestamps properly
						createTime := parseTimestamp(values[10])
						changeTime := parseTimestamp(values[12])

						_, err := db.Exec(database.ConvertPlaceholders(`
							INSERT INTO ticket_history (
								name, history_type_id, ticket_id, article_id,
								type_id, queue_id, owner_id, priority_id,
								state_id, create_time, create_by, change_time, change_by
							) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)
						`), values[1], historyTypeID, newTicketID,
							articleIDVal, parseIntOrNull(values[5]),
							parseIntOrNull(values[6]), parseIntOrNull(values[7]),
							parseIntOrNull(values[8]), parseIntOrNull(values[9]),
							createTime, parseIntOrNull(values[11]), changeTime, parseIntOrNull(values[13]))

						if err != nil {
							if verbose {
								log.Printf("Warning: Failed to insert ticket_history: %v", err)
							}
						} else {
							historyCount++
						}
					} else {
						historyCount++
					}
				}
			}
		}
	}

	if verbose || !dryRun {
		fmt.Printf("  ✅ Imported %d article_data_mime records\n", mimeCount)
		fmt.Printf("  ✅ Imported %d customer_user records\n", customerCount)
		fmt.Printf("  ✅ Imported %d ticket_history records\n", historyCount)
		fmt.Printf("  ✅ Imported %d queue records\n", queueCount)
		fmt.Printf("  ✅ Imported %d group records\n", groupCount)
		fmt.Printf("  ✅ Imported %d user records\n", userCount)
		fmt.Printf("  ✅ Imported %d ticket_priority records\n", priorityCount)
		fmt.Printf("  ✅ Imported %d ticket_state records\n", stateCount)
	}

	return scanner.Err()
}

func parseNull(s string) interface{} {
	if s == "NULL" || s == "" {
		return nil
	}
	return s
}

func parseValues(valueString string) []string {
	// Simple CSV parser for SQL values
	var values []string
	var current strings.Builder
	inQuote := false
	escaped := false

	for _, r := range valueString {
		if escaped {
			current.WriteRune(r)
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

		if r == ',' && !inQuote {
			values = append(values, strings.TrimSpace(current.String()))
			current.Reset()
			continue
		}

		current.WriteRune(r)
	}

	values = append(values, strings.TrimSpace(current.String()))
	return values
}

func parseIntOrNull(s string) interface{} {
	s = strings.TrimSpace(s)
	if s == "NULL" || s == "" {
		return nil
	}
	i, err := strconv.Atoi(s)
	if err != nil {
		return nil
	}
	return i
}

func parseTimestamp(s string) interface{} {
	s = strings.TrimSpace(s)
	if s == "NULL" || s == "" {
		return nil
	}
	// MySQL timestamps come in format '2025-08-26 10:24:57'
	t, err := time.Parse("2006-01-02 15:04:05", s)
	if err != nil {
		// Try Unix timestamp as fallback
		if ts, err := strconv.ParseInt(s, 10, 64); err == nil {
			return time.Unix(ts, 0)
		}
		return nil
	}
	return t
}

func fixSequences(db *sql.DB) {
	sequences := []struct {
		table string
		seq   string
	}{
		{"ticket", "ticket_id_seq"},
		{"article", "article_id_seq"},
		{"ticket_history", "ticket_history_id_seq"},
		{"users", "users_id_seq"},
		{"groups", "groups_id_seq"},
		{"queue", "queue_id_seq"},
		{"ticket_priority", "ticket_priority_id_seq"},
		{"ticket_state", "ticket_state_id_seq"},
		{"customer_user", "customer_user_id_seq"},
		{"customer_company", "customer_company_id_seq"},
	}

	for _, s := range sequences {
		query := fmt.Sprintf("SELECT setval('%s', COALESCE((SELECT MAX(id) FROM %s), 0) + 1, false)", s.seq, s.table) //nolint:gosec // table/seq names from hardcoded list
		if _, err := db.Exec(query); err != nil {
			// Some sequences might not exist, that's okay
			log.Printf("Note: Could not fix sequence %s: %v", s.seq, err)
		}
	}
}

// e.g., "(1,2,3),(4,5,6)" -> ["1,2,3", "4,5,6"].
func parseMultipleValueSets(valuesStr string) []string {
	var result []string
	var current strings.Builder
	depth := 0
	inQuote := false
	escaped := false

	for i, r := range valuesStr {
		if escaped {
			current.WriteRune(r)
			escaped = false
			continue
		}

		if r == '\\' {
			escaped = true
			current.WriteRune(r)
			continue
		}

		if r == '\'' {
			inQuote = !inQuote
			current.WriteRune(r)
			continue
		}

		if !inQuote {
			if r == '(' {
				depth++
				if depth > 1 {
					current.WriteRune(r)
				}
			} else if r == ')' {
				depth--
				if depth > 0 {
					current.WriteRune(r)
				} else if depth == 0 {
					// End of a value set
					result = append(result, current.String())
					current.Reset()
					// Skip comma and whitespace after )
					for i+1 < len(valuesStr) && (valuesStr[i+1] == ',' || valuesStr[i+1] == ' ') {
						i++
					}
				}
			} else if depth > 0 {
				current.WriteRune(r)
			}
		} else {
			current.WriteRune(r)
		}
	}

	// Add any remaining content
	if current.Len() > 0 {
		result = append(result, current.String())
	}

	return result
}

// validateDatabaseState checks if the database is ready for import.
func validateDatabaseState(db *sql.DB, force bool) error {
	// Tables to check for existing data
	tables := []string{
		"ticket", "article", "article_data_mime", "ticket_history",
		"queue", "groups", "users", "ticket_priority", "ticket_state",
		"customer_user", "customer_company",
	}

	nonEmptyTables := []string{}
	for _, table := range tables {
		var count int
		err := db.QueryRow(fmt.Sprintf("SELECT COUNT(*) FROM %s", table)).Scan(&count)
		if err != nil {
			// Table might not exist, that's okay
			continue
		}
		if count > 0 {
			nonEmptyTables = append(nonEmptyTables, fmt.Sprintf("%s (%d rows)", table, count))
		}
	}

	if len(nonEmptyTables) > 0 {
		if !force {
			fmt.Printf("\n❌ ERROR: The following tables contain data:\n")
			for _, table := range nonEmptyTables {
				fmt.Printf("   - %s\n", table)
			}
			fmt.Printf("\n⚠️  Import requires empty tables to avoid duplicate/conflict issues.\n")
			fmt.Printf("\n📘 Options:\n")
			fmt.Printf("   1. Use --force flag to clear existing data before import (DESTRUCTIVE!)\n")
			fmt.Printf("   2. Use 'make db-reset' to reset the database\n")
			fmt.Printf("   3. Manually clear the tables with SQL\n\n")
			return fmt.Errorf("database contains existing data, use --force to clear it")
		}

		// Force mode - clear the data
		fmt.Printf("\n⚠️  WARNING: Force mode enabled - clearing existing data...\n")
		fmt.Printf("   Tables to clear:\n")
		for _, table := range nonEmptyTables {
			fmt.Printf("   - %s\n", table)
		}
		fmt.Printf("\n")

		// Clear in reverse dependency order
		clearOrder := []string{
			"ticket_history",
			"article_data_mime",
			"article",
			"ticket",
			"customer_user",
			"customer_company",
			"ticket_state",
			"ticket_priority",
			"queue",
			"groups",
			"users",
		}

		for _, table := range clearOrder {
			fmt.Printf("   🗑️  Clearing %s...\n", table)
			if _, err := db.Exec(fmt.Sprintf("TRUNCATE %s CASCADE", table)); err != nil {
				// Some tables might not exist or have dependencies
				if _, err := db.Exec(fmt.Sprintf("DELETE FROM %s", table)); err != nil {
					log.Printf("   Warning: Could not clear %s: %v", table, err)
				}
			}
		}
		fmt.Printf("\n✅ Database cleared and ready for import\n\n")
	} else {
		fmt.Printf("✅ Database is clean and ready for import\n")
	}

	return nil
}
