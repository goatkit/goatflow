// Package ticketattributerelations provides management of ticket attribute relationships.
// These relationships allow filtering of attribute options based on other attribute values.
package ticketattributerelations

import (
	"context"
	"database/sql"
	"encoding/base64"
	"encoding/csv"
	"fmt"
	"io"
	"log"
	"sort"
	"strings"
	"time"

	"github.com/goatkit/goatflow/internal/database"
	"github.com/goatkit/goatflow/internal/models"
	"github.com/xuri/excelize/v2"
)

// Service manages ticket attribute relations.
type Service struct {
	db     *sql.DB
	logger *log.Logger
	// Cache of all relations (ordered by priority)
	cachedRelations []*models.TicketAttributeRelation
}

// Option configures the service.
type Option func(*Service)

// WithLogger sets a custom logger.
func WithLogger(l *log.Logger) Option {
	return func(s *Service) { s.logger = l }
}

// NewService creates a new ticket attribute relations service.
func NewService(db *sql.DB, opts ...Option) *Service {
	s := &Service{
		db:     db,
		logger: log.Default(),
	}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

// RefreshCache reloads all relations from the database.
func (s *Service) RefreshCache(ctx context.Context) error {
	relations, err := s.GetAll(ctx)
	if err != nil {
		return err
	}
	s.cachedRelations = relations
	return nil
}

// ClearCache clears the cached relations.
func (s *Service) ClearCache() {
	s.cachedRelations = nil
}

// GetAll returns all ticket attribute relations ordered by priority.
func (s *Service) GetAll(ctx context.Context) ([]*models.TicketAttributeRelation, error) {
	query := database.ConvertPlaceholders(`
		SELECT id, filename, attribute_1, attribute_2, acl_data, priority,
		       create_time, create_by, change_time, change_by
		FROM acl_ticket_attribute_relations
		ORDER BY priority ASC
	`)

	rows, err := s.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("query relations: %w", err)
	}
	defer rows.Close()

	var relations []*models.TicketAttributeRelation
	for rows.Next() {
		r := &models.TicketAttributeRelation{}
		err := rows.Scan(
			&r.ID, &r.Filename, &r.Attribute1, &r.Attribute2, &r.ACLData, &r.Priority,
			&r.CreateTime, &r.CreateBy, &r.ChangeTime, &r.ChangeBy,
		)
		if err != nil {
			return nil, fmt.Errorf("scan relation: %w", err)
		}
		// Parse the data
		r.Data, err = s.parseACLData(r.Filename, r.ACLData, r.Attribute1, r.Attribute2)
		if err != nil {
			s.logger.Printf("Warning: failed to parse relation %d data: %v", r.ID, err)
		}
		relations = append(relations, r)
	}

	return relations, rows.Err()
}

// GetByID returns a ticket attribute relation by ID.
func (s *Service) GetByID(ctx context.Context, id int64) (*models.TicketAttributeRelation, error) {
	query := database.ConvertPlaceholders(`
		SELECT id, filename, attribute_1, attribute_2, acl_data, priority,
		       create_time, create_by, change_time, change_by
		FROM acl_ticket_attribute_relations
		WHERE id = ?
	`)

	r := &models.TicketAttributeRelation{}
	err := s.db.QueryRowContext(ctx, query, id).Scan(
		&r.ID, &r.Filename, &r.Attribute1, &r.Attribute2, &r.ACLData, &r.Priority,
		&r.CreateTime, &r.CreateBy, &r.ChangeTime, &r.ChangeBy,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("query relation by id: %w", err)
	}

	// Parse the data
	r.Data, err = s.parseACLData(r.Filename, r.ACLData, r.Attribute1, r.Attribute2)
	if err != nil {
		s.logger.Printf("Warning: failed to parse relation %d data: %v", r.ID, err)
	}

	return r, nil
}

// GetByFilename returns a ticket attribute relation by filename.
func (s *Service) GetByFilename(ctx context.Context, filename string) (*models.TicketAttributeRelation, error) {
	query := database.ConvertPlaceholders(`
		SELECT id, filename, attribute_1, attribute_2, acl_data, priority,
		       create_time, create_by, change_time, change_by
		FROM acl_ticket_attribute_relations
		WHERE filename = ?
	`)

	r := &models.TicketAttributeRelation{}
	err := s.db.QueryRowContext(ctx, query, filename).Scan(
		&r.ID, &r.Filename, &r.Attribute1, &r.Attribute2, &r.ACLData, &r.Priority,
		&r.CreateTime, &r.CreateBy, &r.ChangeTime, &r.ChangeBy,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("query relation by filename: %w", err)
	}

	// Parse the data
	r.Data, err = s.parseACLData(r.Filename, r.ACLData, r.Attribute1, r.Attribute2)
	if err != nil {
		s.logger.Printf("Warning: failed to parse relation %d data: %v", r.ID, err)
	}

	return r, nil
}

// FilenameExists checks if a filename is already in use.
func (s *Service) FilenameExists(ctx context.Context, filename string) (bool, error) {
	query := database.ConvertPlaceholders(`
		SELECT COUNT(*) FROM acl_ticket_attribute_relations WHERE filename = ?
	`)

	var count int
	err := s.db.QueryRowContext(ctx, query, filename).Scan(&count)
	if err != nil {
		return false, fmt.Errorf("check filename exists: %w", err)
	}

	return count > 0, nil
}

// Create creates a new ticket attribute relation.
func (s *Service) Create(ctx context.Context, relation *models.TicketAttributeRelation, userID int64) (int64, error) {
	// Pre-reorder priorities to make room
	if err := s.preReorderPriorities(ctx); err != nil {
		return 0, fmt.Errorf("pre-reorder priorities: %w", err)
	}

	// Calculate temporary priority (target × 10)
	tempPriority := relation.Priority * 10

	now := time.Now()
	query := database.ConvertPlaceholders(`
		INSERT INTO acl_ticket_attribute_relations
		(filename, attribute_1, attribute_2, acl_data, priority, create_time, create_by, change_time, change_by)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
	`)

	// Handle RETURNING for PostgreSQL vs LastInsertId for MySQL
	query, useLastInsert := database.ConvertReturning(query + " RETURNING id")

	var id int64
	if useLastInsert {
		result, err := s.db.ExecContext(ctx, query,
			relation.Filename, relation.Attribute1, relation.Attribute2, relation.ACLData,
			tempPriority, now, userID, now, userID,
		)
		if err != nil {
			return 0, fmt.Errorf("insert relation: %w", err)
		}
		id, err = result.LastInsertId()
		if err != nil {
			return 0, fmt.Errorf("get last insert id: %w", err)
		}
	} else {
		err := s.db.QueryRowContext(ctx, query,
			relation.Filename, relation.Attribute1, relation.Attribute2, relation.ACLData,
			tempPriority, now, userID, now, userID,
		).Scan(&id)
		if err != nil {
			return 0, fmt.Errorf("insert relation: %w", err)
		}
	}

	// Post-reorder priorities to sequential values
	if err := s.postReorderPriorities(ctx); err != nil {
		return 0, fmt.Errorf("post-reorder priorities: %w", err)
	}

	s.ClearCache()
	return id, nil
}

// Update updates an existing ticket attribute relation.
func (s *Service) Update(ctx context.Context, id int64, updates map[string]interface{}, userID int64) error {
	// If priority is being updated, do the reorder dance
	if _, hasPriority := updates["priority"]; hasPriority {
		if err := s.preReorderPriorities(ctx); err != nil {
			return fmt.Errorf("pre-reorder priorities: %w", err)
		}
		// Convert priority to temp value
		if priority, ok := updates["priority"].(int64); ok {
			updates["priority"] = priority * 10
		} else if priority, ok := updates["priority"].(int); ok {
			updates["priority"] = int64(priority) * 10
		}
	}

	// Build dynamic update query
	var setClauses []string
	var args []interface{}
	for field, value := range updates {
		setClauses = append(setClauses, field+" = ?")
		args = append(args, value)
	}
	setClauses = append(setClauses, "change_time = ?", "change_by = ?")
	args = append(args, time.Now(), userID, id)

	query := database.ConvertPlaceholders(fmt.Sprintf(
		"UPDATE acl_ticket_attribute_relations SET %s WHERE id = ?",
		strings.Join(setClauses, ", "),
	))

	_, err := s.db.ExecContext(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("update relation: %w", err)
	}

	// If priority was updated, finalize the reorder
	if _, hasPriority := updates["priority"]; hasPriority {
		if err := s.postReorderPriorities(ctx); err != nil {
			return fmt.Errorf("post-reorder priorities: %w", err)
		}
	}

	s.ClearCache()
	return nil
}

// Delete deletes a ticket attribute relation.
func (s *Service) Delete(ctx context.Context, id int64) error {
	query := database.ConvertPlaceholders(`
		DELETE FROM acl_ticket_attribute_relations WHERE id = ?
	`)

	_, err := s.db.ExecContext(ctx, query, id)
	if err != nil {
		return fmt.Errorf("delete relation: %w", err)
	}

	// Reorder remaining priorities
	if err := s.postReorderPriorities(ctx); err != nil {
		return fmt.Errorf("post-reorder priorities: %w", err)
	}

	s.ClearCache()
	return nil
}

// GetNextPriority returns the next available priority value.
func (s *Service) GetNextPriority(ctx context.Context) (int64, error) {
	query := `SELECT COALESCE(MAX(priority), 0) + 1 FROM acl_ticket_attribute_relations`

	var next int64
	err := s.db.QueryRowContext(ctx, query).Scan(&next)
	if err != nil {
		return 0, fmt.Errorf("get next priority: %w", err)
	}

	return next, nil
}

// preReorderPriorities multiplies all priorities by 10 and adds 1.
// This creates gaps for inserting new priorities.
func (s *Service) preReorderPriorities(ctx context.Context) error {
	query := `UPDATE acl_ticket_attribute_relations SET priority = priority * 10 + 1`
	_, err := s.db.ExecContext(ctx, query)
	return err
}

// postReorderPriorities reassigns sequential priority values starting from 1.
func (s *Service) postReorderPriorities(ctx context.Context) error {
	// Get all relations ordered by current priority
	query := `SELECT id, priority FROM acl_ticket_attribute_relations ORDER BY priority ASC`
	rows, err := s.db.QueryContext(ctx, query)
	if err != nil {
		return err
	}
	defer rows.Close()

	type item struct {
		id       int64
		priority int64
	}
	var items []item
	for rows.Next() {
		var i item
		if err := rows.Scan(&i.id, &i.priority); err != nil {
			return err
		}
		items = append(items, i)
	}
	if err := rows.Err(); err != nil {
		return err
	}

	// Sort by priority and reassign sequential values
	sort.Slice(items, func(i, j int) bool {
		return items[i].priority < items[j].priority
	})

	updateQuery := database.ConvertPlaceholders(`
		UPDATE acl_ticket_attribute_relations SET priority = ? WHERE id = ?
	`)
	for i, item := range items {
		newPriority := int64(i + 1)
		if _, err := s.db.ExecContext(ctx, updateQuery, newPriority, item.id); err != nil {
			return err
		}
	}

	return nil
}

// parseACLData parses the stored CSV or Excel data.
func (s *Service) parseACLData(filename, data, attr1, attr2 string) ([]models.AttributeRelationPair, error) {
	if data == "" {
		return nil, nil
	}

	// Check if it's an Excel file (base64 encoded)
	if isExcelFilename(filename) {
		return s.parseExcelData(data, attr1, attr2)
	}

	// Otherwise parse as CSV
	return s.parseCSVData(data, attr1, attr2)
}

// parseCSVData parses CSV content.
func (s *Service) parseCSVData(data, attr1, attr2 string) ([]models.AttributeRelationPair, error) {
	reader := csv.NewReader(strings.NewReader(data))
	reader.Comma = ';' // OTRS uses semicolon as separator

	// Read header row
	header, err := reader.Read()
	if err != nil {
		return nil, fmt.Errorf("read CSV header: %w", err)
	}

	if len(header) != 2 {
		return nil, fmt.Errorf("CSV must have exactly 2 columns, got %d", len(header))
	}

	// Read data rows
	var pairs []models.AttributeRelationPair
	for {
		record, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("read CSV row: %w", err)
		}

		if len(record) < 2 {
			continue
		}

		val1 := strings.TrimSpace(record[0])
		val2 := strings.TrimSpace(record[1])

		// Handle "-" as empty value
		if val1 == "-" {
			val1 = ""
		}
		if val2 == "-" {
			val2 = ""
		}

		pairs = append(pairs, models.AttributeRelationPair{
			Attribute1Value: val1,
			Attribute2Value: val2,
		})
	}

	return pairs, nil
}

// parseExcelData parses base64-encoded Excel data.
func (s *Service) parseExcelData(data, attr1, attr2 string) ([]models.AttributeRelationPair, error) {
	// Decode base64
	decoded, err := base64.StdEncoding.DecodeString(data)
	if err != nil {
		return nil, fmt.Errorf("decode base64: %w", err)
	}

	// Open Excel file
	f, err := excelize.OpenReader(strings.NewReader(string(decoded)))
	if err != nil {
		return nil, fmt.Errorf("open Excel: %w", err)
	}
	defer f.Close()

	// Get first sheet
	sheets := f.GetSheetList()
	if len(sheets) == 0 {
		return nil, fmt.Errorf("no sheets in Excel file")
	}

	rows, err := f.GetRows(sheets[0])
	if err != nil {
		return nil, fmt.Errorf("get Excel rows: %w", err)
	}

	if len(rows) < 2 {
		return nil, fmt.Errorf("Excel file must have header and at least one data row")
	}

	// Validate header
	header := rows[0]
	if len(header) < 2 {
		return nil, fmt.Errorf("Excel must have at least 2 columns, got %d", len(header))
	}

	// Parse data rows (skip header)
	var pairs []models.AttributeRelationPair
	for _, row := range rows[1:] {
		if len(row) < 2 {
			continue
		}

		val1 := strings.TrimSpace(row[0])
		val2 := strings.TrimSpace(row[1])

		// Handle "-" as empty value
		if val1 == "-" {
			val1 = ""
		}
		if val2 == "-" {
			val2 = ""
		}

		pairs = append(pairs, models.AttributeRelationPair{
			Attribute1Value: val1,
			Attribute2Value: val2,
		})
	}

	return pairs, nil
}

// ParseUploadedFile parses an uploaded CSV or Excel file and returns the relation data.
func (s *Service) ParseUploadedFile(filename string, data []byte) (attr1, attr2 string, pairs []models.AttributeRelationPair, err error) {
	if isExcelFilename(filename) {
		return s.parseExcelUpload(data)
	}
	return s.parseCSVUpload(data)
}

// parseCSVUpload parses an uploaded CSV file.
func (s *Service) parseCSVUpload(data []byte) (attr1, attr2 string, pairs []models.AttributeRelationPair, err error) {
	reader := csv.NewReader(strings.NewReader(string(data)))
	reader.Comma = ';'

	// Read header row (contains attribute names)
	header, err := reader.Read()
	if err != nil {
		return "", "", nil, fmt.Errorf("read CSV header: %w", err)
	}

	if len(header) != 2 {
		return "", "", nil, fmt.Errorf("CSV must have exactly 2 columns, got %d", len(header))
	}

	attr1 = strings.TrimSpace(header[0])
	attr2 = strings.TrimSpace(header[1])

	// Validate attribute names
	if !models.IsValidAttribute(attr1) {
		return "", "", nil, fmt.Errorf("invalid attribute: %s", attr1)
	}
	if !models.IsValidAttribute(attr2) {
		return "", "", nil, fmt.Errorf("invalid attribute: %s", attr2)
	}

	// Read data rows
	for {
		record, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return "", "", nil, fmt.Errorf("read CSV row: %w", err)
		}

		if len(record) < 2 {
			continue
		}

		val1 := strings.TrimSpace(record[0])
		val2 := strings.TrimSpace(record[1])

		if val1 == "-" {
			val1 = ""
		}
		if val2 == "-" {
			val2 = ""
		}

		pairs = append(pairs, models.AttributeRelationPair{
			Attribute1Value: val1,
			Attribute2Value: val2,
		})
	}

	return attr1, attr2, pairs, nil
}

// parseExcelUpload parses an uploaded Excel file.
func (s *Service) parseExcelUpload(data []byte) (attr1, attr2 string, pairs []models.AttributeRelationPair, err error) {
	f, err := excelize.OpenReader(strings.NewReader(string(data)))
	if err != nil {
		return "", "", nil, fmt.Errorf("open Excel: %w", err)
	}
	defer f.Close()

	sheets := f.GetSheetList()
	if len(sheets) == 0 {
		return "", "", nil, fmt.Errorf("no sheets in Excel file")
	}

	rows, err := f.GetRows(sheets[0])
	if err != nil {
		return "", "", nil, fmt.Errorf("get Excel rows: %w", err)
	}

	if len(rows) < 1 {
		return "", "", nil, fmt.Errorf("Excel file is empty")
	}

	// Read header (attribute names)
	header := rows[0]
	if len(header) < 2 {
		return "", "", nil, fmt.Errorf("Excel must have at least 2 columns, got %d", len(header))
	}

	attr1 = strings.TrimSpace(header[0])
	attr2 = strings.TrimSpace(header[1])

	// Validate attribute names
	if !models.IsValidAttribute(attr1) {
		return "", "", nil, fmt.Errorf("invalid attribute: %s", attr1)
	}
	if !models.IsValidAttribute(attr2) {
		return "", "", nil, fmt.Errorf("invalid attribute: %s", attr2)
	}

	// Parse data rows (skip header)
	for _, row := range rows[1:] {
		if len(row) < 2 {
			continue
		}

		val1 := strings.TrimSpace(row[0])
		val2 := strings.TrimSpace(row[1])

		if val1 == "-" {
			val1 = ""
		}
		if val2 == "-" {
			val2 = ""
		}

		pairs = append(pairs, models.AttributeRelationPair{
			Attribute1Value: val1,
			Attribute2Value: val2,
		})
	}

	return attr1, attr2, pairs, nil
}

// PrepareDataForStorage prepares file data for database storage.
// CSV files are stored as-is, Excel files are base64 encoded.
func (s *Service) PrepareDataForStorage(filename string, data []byte) string {
	if isExcelFilename(filename) {
		return base64.StdEncoding.EncodeToString(data)
	}
	return string(data)
}

// GetRawDataForDownload returns the raw file data for download.
func (s *Service) GetRawDataForDownload(relation *models.TicketAttributeRelation) ([]byte, error) {
	if isExcelFilename(relation.Filename) {
		return base64.StdEncoding.DecodeString(relation.ACLData)
	}
	return []byte(relation.ACLData), nil
}

// EvaluateRelations returns allowed values for Attribute2 based on current Attribute1 value.
// This is used for filtering ticket form options.
func (s *Service) EvaluateRelations(ctx context.Context, attr1 string, attr1Value string) (map[string][]string, error) {
	relations, err := s.GetAll(ctx)
	if err != nil {
		return nil, err
	}

	result := make(map[string][]string)

	for _, rel := range relations {
		if rel.Attribute1 != attr1 {
			continue
		}

		// Get allowed values for this relation
		allowed := rel.GetAllowedValues(attr1Value)
		if len(allowed) == 0 {
			continue
		}

		// Merge with existing values for this attribute2
		if existing, ok := result[rel.Attribute2]; ok {
			// Use intersection for restrictive mode
			result[rel.Attribute2] = intersectSlices(existing, allowed)
		} else {
			result[rel.Attribute2] = allowed
		}
	}

	return result, nil
}

// GetPriorityOptions returns priority options for the dropdown.
func (s *Service) GetPriorityOptions(ctx context.Context) ([]int64, error) {
	relations, err := s.GetAll(ctx)
	if err != nil {
		return nil, err
	}

	// Generate options from 1 to count+1
	count := int64(len(relations) + 1)
	options := make([]int64, count)
	for i := int64(1); i <= count; i++ {
		options[i-1] = i
	}

	return options, nil
}

// isExcelFilename checks if a filename has an Excel extension.
func isExcelFilename(filename string) bool {
	lower := strings.ToLower(filename)
	return strings.HasSuffix(lower, ".xlsx") || strings.HasSuffix(lower, ".xls")
}

// intersectSlices returns the intersection of two string slices.
func intersectSlices(a, b []string) []string {
	set := make(map[string]bool)
	for _, v := range a {
		set[v] = true
	}

	var result []string
	for _, v := range b {
		if set[v] {
			result = append(result, v)
		}
	}
	return result
}

// ReorderPriorities updates priorities based on a new ordering of relation IDs.
// The orderedIDs slice contains relation IDs in the desired priority order (first = priority 1).
func (s *Service) ReorderPriorities(ctx context.Context, orderedIDs []int64, userID int64) error {
	if len(orderedIDs) == 0 {
		return nil
	}

	// Update each relation with its new priority (position in slice + 1)
	updateQuery := database.ConvertPlaceholders(`
		UPDATE acl_ticket_attribute_relations
		SET priority = ?, change_time = ?, change_by = ?
		WHERE id = ?
	`)

	now := time.Now()
	for i, id := range orderedIDs {
		newPriority := int64(i + 1)
		_, err := s.db.ExecContext(ctx, updateQuery, newPriority, now, userID, id)
		if err != nil {
			return fmt.Errorf("update priority for relation %d: %w", id, err)
		}
	}

	s.ClearCache()
	return nil
}
