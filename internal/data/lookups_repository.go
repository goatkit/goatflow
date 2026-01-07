// Package data provides data access repositories for lookup tables.
package data

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"
)

// LookupItem represents a database lookup value.
type LookupItem struct {
	ID       int
	Name     string
	ValidID  int
	IsSystem bool
	TypeID   int // For states
}

// LookupsRepository handles database operations for lookup values.
type LookupsRepository struct {
	db *sql.DB
}

// NewLookupsRepository creates a new lookups repository.
func NewLookupsRepository(db *sql.DB) *LookupsRepository {
	return &LookupsRepository{
		db: db,
	}
}

// GetTicketStates fetches all ticket states from the database.
func (r *LookupsRepository) GetTicketStates(ctx context.Context) ([]LookupItem, error) {
	query := `
		SELECT id, name, valid_id, type_id,
		       CASE WHEN name IN ('new', 'open', 'pending reminder', 'pending auto close+', 
		                          'pending auto close-', 'closed successful', 'closed unsuccessful',
		                          'merged', 'removed') THEN true ELSE false END as is_system
		FROM ticket_state
		ORDER BY 
			CASE type_id
				WHEN 1 THEN 1  -- new
				WHEN 2 THEN 2  -- open
				WHEN 5 THEN 3  -- pending
				WHEN 3 THEN 4  -- closed
				WHEN 4 THEN 5  -- removed
				ELSE 6
			END,
			name
	`

	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to query ticket states: %w", err)
	}
	defer rows.Close()

	var states []LookupItem
	for rows.Next() {
		var state LookupItem
		err := rows.Scan(&state.ID, &state.Name, &state.ValidID, &state.TypeID, &state.IsSystem)
		if err != nil {
			return nil, fmt.Errorf("failed to scan ticket state: %w", err)
		}
		states = append(states, state)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating ticket states: %w", err)
	}

	return states, nil
}

// GetTicketPriorities fetches all ticket priorities from the database.
func (r *LookupsRepository) GetTicketPriorities(ctx context.Context) ([]LookupItem, error) {
	query := `
		SELECT id, name, valid_id,
		       CASE WHEN name IN ('1 very low', '2 low', '3 normal', '4 high', '5 very high') 
		            THEN true ELSE false END as is_system
		FROM ticket_priority
		WHERE valid_id = 1
		ORDER BY 
			CASE 
				WHEN name LIKE '1 %' THEN 1
				WHEN name LIKE '2 %' THEN 2
				WHEN name LIKE '3 %' THEN 3
				WHEN name LIKE '4 %' THEN 4
				WHEN name LIKE '5 %' THEN 5
				ELSE 6
			END,
			name
	`

	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to query ticket priorities: %w", err)
	}
	defer rows.Close()

	var priorities []LookupItem
	for rows.Next() {
		var priority LookupItem
		err := rows.Scan(&priority.ID, &priority.Name, &priority.ValidID, &priority.IsSystem)
		if err != nil {
			return nil, fmt.Errorf("failed to scan ticket priority: %w", err)
		}
		priorities = append(priorities, priority)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating ticket priorities: %w", err)
	}

	return priorities, nil
}

// GetQueues fetches all queues from the database.
func (r *LookupsRepository) GetQueues(ctx context.Context) ([]LookupItem, error) {
	query := `
		SELECT id, name, valid_id,
		       CASE WHEN id <= 10 THEN true ELSE false END as is_system
		FROM queue
		WHERE valid_id = 1
		ORDER BY name
	`

	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to query queues: %w", err)
	}
	defer rows.Close()

	var queues []LookupItem
	for rows.Next() {
		var queue LookupItem
		err := rows.Scan(&queue.ID, &queue.Name, &queue.ValidID, &queue.IsSystem)
		if err != nil {
			return nil, fmt.Errorf("failed to scan queue: %w", err)
		}
		queues = append(queues, queue)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating queues: %w", err)
	}

	return queues, nil
}

// GetTicketTypes fetches all ticket types from the database.
func (r *LookupsRepository) GetTicketTypes(ctx context.Context) ([]LookupItem, error) {
	// Check if ticket_types table exists
	var tableExists bool
	err := r.db.QueryRowContext(ctx, `
		SELECT EXISTS (
			SELECT FROM information_schema.tables 
			WHERE table_schema = 'public' 
			AND table_name = 'ticket_types'
		)
	`).Scan(&tableExists)

	if err != nil || !tableExists {
		// Return default types if table doesn't exist
		return []LookupItem{
			{ID: 1, Name: "Unclassified", ValidID: 1, IsSystem: true},
			{ID: 2, Name: "Incident", ValidID: 1, IsSystem: true},
			{ID: 3, Name: "Problem", ValidID: 1, IsSystem: true},
			{ID: 4, Name: "Change Request", ValidID: 1, IsSystem: true},
		}, nil
	}

	query := `
		SELECT id, name, valid_id,
		       CASE WHEN name IN ('Unclassified', 'Incident', 'Problem', 'Change Request') 
		            THEN true ELSE false END as is_system
		FROM ticket_types
		WHERE valid_id = 1
		ORDER BY name
	`

	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to query ticket types: %w", err)
	}
	defer rows.Close()

	var types []LookupItem
	for rows.Next() {
		var ticketType LookupItem
		err := rows.Scan(&ticketType.ID, &ticketType.Name, &ticketType.ValidID, &ticketType.IsSystem)
		if err != nil {
			return nil, fmt.Errorf("failed to scan ticket type: %w", err)
		}
		types = append(types, ticketType)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating ticket types: %w", err)
	}

	return types, nil
}

// GetTranslation fetches a translation from the lookup_translations table.
func (r *LookupsRepository) GetTranslation(ctx context.Context, tableName, fieldValue, lang string) (string, error) {
	var translation string
	query := `
		SELECT translation 
		FROM lookup_translations
		WHERE table_name = ? AND field_value = ? AND language_code = ?
		LIMIT 1
	`

	err := r.db.QueryRowContext(ctx, query, tableName, fieldValue, lang).Scan(&translation)
	if err == sql.ErrNoRows {
		// No translation found - this is ok, we'll use the original value
		return "", nil
	}
	if err != nil {
		// Check if table doesn't exist - treat same as no translation found
		if strings.Contains(err.Error(), "lookup_translations") && strings.Contains(err.Error(), "does not exist") {
			return "", nil
		}
		return "", fmt.Errorf("failed to query translation: %w", err)
	}

	return translation, nil
}

// AddTranslation adds a new translation to the database.
func (r *LookupsRepository) AddTranslation(ctx context.Context, tableName, fieldValue, lang, translation string, isSystem bool) error {
	query := `
		INSERT INTO lookup_translations (table_name, field_value, language_code, translation, is_system, create_time, change_time)
		VALUES (?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT (table_name, field_value, language_code) 
		DO UPDATE SET translation = ?, change_time = ?
	`

	_, err := r.db.ExecContext(ctx, query, tableName, fieldValue, lang, translation, isSystem, time.Now())
	if err != nil {
		// Check if table doesn't exist - silently ignore since translations are optional
		if strings.Contains(err.Error(), "lookup_translations") && strings.Contains(err.Error(), "does not exist") {
			return nil
		}
		return fmt.Errorf("failed to add translation: %w", err)
	}

	return nil
}

// GetAllTranslations fetches all translations for a specific language.
func (r *LookupsRepository) GetAllTranslations(ctx context.Context, lang string) (map[string]map[string]string, error) {
	query := `
		SELECT table_name, field_value, translation
		FROM lookup_translations
		WHERE language_code = ?
	`

	rows, err := r.db.QueryContext(ctx, query, lang)
	if err != nil {
		// Check if table doesn't exist - return empty map instead of error
		if strings.Contains(err.Error(), "lookup_translations") && strings.Contains(err.Error(), "does not exist") {
			return make(map[string]map[string]string), nil
		}
		return nil, fmt.Errorf("failed to query translations: %w", err)
	}
	defer rows.Close()

	translations := make(map[string]map[string]string)
	for rows.Next() {
		var tableName, fieldValue, translation string
		err := rows.Scan(&tableName, &fieldValue, &translation)
		if err != nil {
			return nil, fmt.Errorf("failed to scan translation: %w", err)
		}

		if translations[tableName] == nil {
			translations[tableName] = make(map[string]string)
		}
		translations[tableName][fieldValue] = translation
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating translations: %w", err)
	}

	return translations, nil
}
