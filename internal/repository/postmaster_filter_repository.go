package repository

import (
	"context"
	"database/sql"
	"fmt"
	"sort"

	"github.com/goatkit/goatflow/internal/database"
)

// FilterMatch represents a match condition in a postmaster filter.
type FilterMatch struct {
	Key   string // Header name (From, To, Subject, X-Custom-Header)
	Value string // Regex pattern
	Not   bool   // Negate match
}

// FilterSet represents a set action in a postmaster filter.
type FilterSet struct {
	Key   string // X-GoatFlow-Queue, X-GoatFlow-Priority, etc.
	Value string // Value to set
}

// PostmasterFilter represents a grouped postmaster filter with its match and set rules.
type PostmasterFilter struct {
	Name    string
	Stop    bool
	Matches []FilterMatch // f_type='Match' rows
	Sets    []FilterSet   // f_type='Set' rows
}

// PostmasterFilterRepository handles database operations for postmaster filters.
type PostmasterFilterRepository struct {
	db *sql.DB
}

// NewPostmasterFilterRepository creates a new repository instance.
func NewPostmasterFilterRepository(db *sql.DB) *PostmasterFilterRepository {
	return &PostmasterFilterRepository{db: db}
}

// List returns all postmaster filters grouped by name.
func (r *PostmasterFilterRepository) List(ctx context.Context) ([]PostmasterFilter, error) {
	query := database.ConvertPlaceholders(`
		SELECT f_name, f_stop, f_type, f_key, f_value, f_not
		FROM postmaster_filter
		ORDER BY f_name, f_type DESC, f_key`)

	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to query postmaster filters: %w", err)
	}
	defer rows.Close()

	return scanFilters(rows)
}

// Get returns a single postmaster filter by name.
func (r *PostmasterFilterRepository) Get(ctx context.Context, name string) (*PostmasterFilter, error) {
	query := database.ConvertPlaceholders(`
		SELECT f_name, f_stop, f_type, f_key, f_value, f_not
		FROM postmaster_filter
		WHERE f_name = ?
		ORDER BY f_type DESC, f_key`)

	rows, err := r.db.QueryContext(ctx, query, name)
	if err != nil {
		return nil, fmt.Errorf("failed to query postmaster filter: %w", err)
	}
	defer rows.Close()

	filters, err := scanFilters(rows)
	if err != nil {
		return nil, err
	}

	if len(filters) == 0 {
		return nil, sql.ErrNoRows
	}

	return &filters[0], nil
}

// Create creates a new postmaster filter with its match and set rules.
func (r *PostmasterFilterRepository) Create(ctx context.Context, filter *PostmasterFilter) error {
	if filter == nil {
		return fmt.Errorf("filter is nil")
	}
	if filter.Name == "" {
		return fmt.Errorf("filter name is required")
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Insert match rules
	for _, match := range filter.Matches {
		if err := insertFilterRow(ctx, tx, filter.Name, filter.Stop, "Match", match.Key, match.Value, match.Not); err != nil {
			return err
		}
	}

	// Insert set rules
	for _, set := range filter.Sets {
		if err := insertFilterRow(ctx, tx, filter.Name, filter.Stop, "Set", set.Key, set.Value, false); err != nil {
			return err
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

// Update updates an existing postmaster filter by replacing all its rules.
func (r *PostmasterFilterRepository) Update(ctx context.Context, name string, filter *PostmasterFilter) error {
	if filter == nil {
		return fmt.Errorf("filter is nil")
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Delete existing rules for this filter
	deleteQuery := database.ConvertPlaceholders(`DELETE FROM postmaster_filter WHERE f_name = ?`)
	if _, err := tx.ExecContext(ctx, deleteQuery, name); err != nil {
		return fmt.Errorf("failed to delete existing filter rules: %w", err)
	}

	// Insert new match rules
	newName := filter.Name
	if newName == "" {
		newName = name
	}

	for _, match := range filter.Matches {
		if err := insertFilterRow(ctx, tx, newName, filter.Stop, "Match", match.Key, match.Value, match.Not); err != nil {
			return err
		}
	}

	// Insert new set rules
	for _, set := range filter.Sets {
		if err := insertFilterRow(ctx, tx, newName, filter.Stop, "Set", set.Key, set.Value, false); err != nil {
			return err
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

// Delete removes a postmaster filter and all its rules.
func (r *PostmasterFilterRepository) Delete(ctx context.Context, name string) error {
	query := database.ConvertPlaceholders(`DELETE FROM postmaster_filter WHERE f_name = ?`)
	result, err := r.db.ExecContext(ctx, query, name)
	if err != nil {
		return fmt.Errorf("failed to delete postmaster filter: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rows == 0 {
		return sql.ErrNoRows
	}

	return nil
}

// insertFilterRow inserts a single row into the postmaster_filter table.
func insertFilterRow(ctx context.Context, tx *sql.Tx, name string, stop bool, fType, key, value string, not bool) error {
	query := database.ConvertPlaceholders(`
		INSERT INTO postmaster_filter (f_name, f_stop, f_type, f_key, f_value, f_not)
		VALUES (?, ?, ?, ?, ?, ?)`)

	stopVal := int16(0)
	if stop {
		stopVal = 1
	}

	notVal := int16(0)
	if not {
		notVal = 1
	}

	_, err := tx.ExecContext(ctx, query, name, stopVal, fType, key, value, notVal)
	if err != nil {
		return fmt.Errorf("failed to insert filter row: %w", err)
	}

	return nil
}

// scanFilters scans rows into grouped PostmasterFilter structs.
func scanFilters(rows *sql.Rows) ([]PostmasterFilter, error) {
	filterMap := make(map[string]*PostmasterFilter)
	var filterOrder []string

	for rows.Next() {
		var (
			name    string
			stopVal sql.NullInt16
			fType   string
			key     string
			value   string
			notVal  sql.NullInt16
		)

		if err := rows.Scan(&name, &stopVal, &fType, &key, &value, &notVal); err != nil {
			return nil, fmt.Errorf("failed to scan filter row: %w", err)
		}

		filter, exists := filterMap[name]
		if !exists {
			filter = &PostmasterFilter{
				Name:    name,
				Stop:    stopVal.Valid && stopVal.Int16 == 1,
				Matches: []FilterMatch{},
				Sets:    []FilterSet{},
			}
			filterMap[name] = filter
			filterOrder = append(filterOrder, name)
		}

		switch fType {
		case "Match":
			filter.Matches = append(filter.Matches, FilterMatch{
				Key:   key,
				Value: value,
				Not:   notVal.Valid && notVal.Int16 == 1,
			})
		case "Set":
			filter.Sets = append(filter.Sets, FilterSet{
				Key:   key,
				Value: value,
			})
		}
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating filter rows: %w", err)
	}

	// Return filters in original order
	result := make([]PostmasterFilter, 0, len(filterOrder))
	for _, name := range filterOrder {
		result = append(result, *filterMap[name])
	}

	// Sort by name for consistent ordering
	sort.Slice(result, func(i, j int) bool {
		return result[i].Name < result[j].Name
	})

	return result, nil
}
