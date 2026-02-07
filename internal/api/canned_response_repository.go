package api

import (
	"database/sql"
	"encoding/json"
	"strconv"
	"strings"
	"time"

	"github.com/goatkit/goatflow/internal/database"
)

// CannedResponseDB represents a canned response in the database.
type CannedResponseDB struct {
	ID           int
	Name         string
	Category     sql.NullString
	Content      string
	ContentType  string
	Tags         sql.NullString
	Scope        string
	OwnerID      int
	TeamID       sql.NullInt64
	Placeholders sql.NullString
	UsageCount   int
	LastUsed     sql.NullTime
	ValidID      int
	CreateTime   time.Time
	CreateBy     int
	ChangeTime   time.Time
	ChangeBy     int
}

// ToCannedResponse converts database model to API model.
func (r *CannedResponseDB) ToCannedResponse() *CannedResponse {
	cr := &CannedResponse{
		ID:          r.ID,
		Name:        r.Name,
		Content:     r.Content,
		ContentType: r.ContentType,
		Scope:       r.Scope,
		OwnerID:     r.OwnerID,
		UsageCount:  r.UsageCount,
		CreatedAt:   r.CreateTime,
		UpdatedAt:   r.ChangeTime,
	}

	if r.Category.Valid {
		cr.Category = r.Category.String
	}
	if r.TeamID.Valid {
		cr.TeamID = int(r.TeamID.Int64)
	}
	if r.LastUsed.Valid {
		cr.LastUsed = &r.LastUsed.Time
	}
	if r.Tags.Valid && r.Tags.String != "" {
		if err := json.Unmarshal([]byte(r.Tags.String), &cr.Tags); err != nil {
			cr.Tags = nil // Reset on parse error
		}
	}
	if r.Placeholders.Valid && r.Placeholders.String != "" {
		if err := json.Unmarshal([]byte(r.Placeholders.String), &cr.Placeholders); err != nil {
			cr.Placeholders = nil // Reset on parse error
		}
	}

	return cr
}

// CannedResponseRepository handles database operations for canned responses.
type CannedResponseRepository struct {
	db *sql.DB
}

// NewCannedResponseRepository creates a new repository.
func NewCannedResponseRepository() (*CannedResponseRepository, error) {
	db, err := database.GetDB()
	if err != nil {
		return nil, err
	}
	return &CannedResponseRepository{db: db}, nil
}

// Create inserts a new canned response.
func (r *CannedResponseRepository) Create(cr *CannedResponse, userID int) (int, error) {
	tagsJSON, _ := json.Marshal(cr.Tags)                 //nolint:errcheck // []string marshal cannot fail
	placeholdersJSON, _ := json.Marshal(cr.Placeholders) //nolint:errcheck // []string marshal cannot fail

	var teamID sql.NullInt64
	if cr.TeamID > 0 {
		teamID = sql.NullInt64{Int64: int64(cr.TeamID), Valid: true}
	}

	var category sql.NullString
	if cr.Category != "" {
		category = sql.NullString{String: cr.Category, Valid: true}
	}

	query := database.ConvertPlaceholders(`
		INSERT INTO canned_response 
		(name, category, content, content_type, tags, scope, owner_id, team_id, placeholders, 
		 usage_count, valid_id, create_time, create_by, change_time, change_by)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, 0, 1, NOW(), ?, NOW(), ?)
	`)

	result, err := r.db.Exec(query,
		cr.Name, category, cr.Content, cr.ContentType, string(tagsJSON),
		cr.Scope, cr.OwnerID, teamID, string(placeholdersJSON), userID)
	if err != nil {
		return 0, err
	}

	id, err := result.LastInsertId()
	return int(id), err
}

// GetByID retrieves a canned response by ID.
func (r *CannedResponseRepository) GetByID(id int) (*CannedResponse, error) {
	query := database.ConvertPlaceholders(`
		SELECT id, name, category, content, content_type, tags, scope, owner_id, team_id,
		       placeholders, usage_count, last_used, valid_id, create_time, create_by, change_time, change_by
		FROM canned_response WHERE id = ?
	`)

	var dbr CannedResponseDB
	err := r.db.QueryRow(query, id).Scan(
		&dbr.ID, &dbr.Name, &dbr.Category, &dbr.Content, &dbr.ContentType, &dbr.Tags,
		&dbr.Scope, &dbr.OwnerID, &dbr.TeamID, &dbr.Placeholders, &dbr.UsageCount,
		&dbr.LastUsed, &dbr.ValidID, &dbr.CreateTime, &dbr.CreateBy, &dbr.ChangeTime, &dbr.ChangeBy)
	if err == sql.ErrNoRows {
		return nil, nil //nolint:nilnil
	}
	if err != nil {
		return nil, err
	}

	return dbr.ToCannedResponse(), nil
}

// ListAccessible returns canned responses accessible to a user.
func (r *CannedResponseRepository) ListAccessible(userID, teamID int, filters CannedResponseFilters) ([]*CannedResponse, error) {
	var args []interface{}
	argIdx := 1

	query := `
		SELECT id, name, category, content, content_type, tags, scope, owner_id, team_id,
		       placeholders, usage_count, last_used, valid_id, create_time, create_by, change_time, change_by
		FROM canned_response WHERE valid_id = 1 AND (`

	// Build scope conditions
	conditions := []string{}
	conditions = append(conditions, "scope = 'global'")

	conditions = append(conditions, "(scope = 'personal' AND owner_id = $"+strconv.Itoa(argIdx)+")")
	args = append(args, userID)
	argIdx++

	if teamID > 0 {
		conditions = append(conditions, "(scope = 'team' AND team_id = $"+strconv.Itoa(argIdx)+")")
		args = append(args, teamID)
		argIdx++
	}

	query += strings.Join(conditions, " OR ") + ")"

	// Apply filters
	if filters.Category != "" {
		query += " AND category = $" + strconv.Itoa(argIdx)
		args = append(args, filters.Category)
		argIdx++
	}
	if filters.Scope != "" {
		query += " AND scope = $" + strconv.Itoa(argIdx)
		args = append(args, filters.Scope)
		argIdx++
	}
	if filters.Search != "" {
		query += " AND (name LIKE $" + strconv.Itoa(argIdx) + " OR content LIKE $" + strconv.Itoa(argIdx) + ")"
		args = append(args, "%"+filters.Search+"%")
		argIdx++
	}
	if len(filters.Tags) > 0 {
		for _, tag := range filters.Tags {
			query += " AND tags LIKE $" + strconv.Itoa(argIdx)
			args = append(args, "%\""+tag+"\"%")
			argIdx++
		}
	}

	// Sorting
	switch filters.SortBy {
	case "name":
		query += " ORDER BY name"
	case "usage":
		query += " ORDER BY usage_count DESC"
	case "recent":
		query += " ORDER BY last_used DESC NULLS LAST"
	default:
		query += " ORDER BY name"
	}

	if filters.SortOrder == "desc" {
		query += " DESC"
	}

	rows, err := r.db.Query(database.ConvertPlaceholders(query), args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []*CannedResponse
	for rows.Next() {
		var dbr CannedResponseDB
		if err := rows.Scan(
			&dbr.ID, &dbr.Name, &dbr.Category, &dbr.Content, &dbr.ContentType, &dbr.Tags,
			&dbr.Scope, &dbr.OwnerID, &dbr.TeamID, &dbr.Placeholders, &dbr.UsageCount,
			&dbr.LastUsed, &dbr.ValidID, &dbr.CreateTime, &dbr.CreateBy, &dbr.ChangeTime, &dbr.ChangeBy); err != nil {
			return nil, err
		}
		results = append(results, dbr.ToCannedResponse())
	}

	return results, rows.Err()
}

// Update updates a canned response.
func (r *CannedResponseRepository) Update(id int, cr *CannedResponse, userID int) error {
	tagsJSON, _ := json.Marshal(cr.Tags)                 //nolint:errcheck // []string marshal cannot fail
	placeholdersJSON, _ := json.Marshal(cr.Placeholders) //nolint:errcheck // []string marshal cannot fail

	var teamID sql.NullInt64
	if cr.TeamID > 0 {
		teamID = sql.NullInt64{Int64: int64(cr.TeamID), Valid: true}
	}

	var category sql.NullString
	if cr.Category != "" {
		category = sql.NullString{String: cr.Category, Valid: true}
	}

	query := database.ConvertPlaceholders(`
		UPDATE canned_response 
		SET name = ?, category = ?, content = ?, content_type = ?, tags = ?, 
		    scope = ?, team_id = ?, placeholders = ?, change_time = NOW(), change_by = ?
		WHERE id = ?
	`)

	_, err := r.db.Exec(query,
		cr.Name, category, cr.Content, cr.ContentType, string(tagsJSON),
		cr.Scope, teamID, string(placeholdersJSON), userID, id)
	return err
}

// Delete soft-deletes a canned response by setting valid_id = 2.
func (r *CannedResponseRepository) Delete(id, userID int) error {
	query := database.ConvertPlaceholders(`
		UPDATE canned_response SET valid_id = 2, change_time = NOW(), change_by = ? WHERE id = ?
	`)
	_, err := r.db.Exec(query, userID, id)
	return err
}

// IncrementUsage increments usage count and updates last_used.
func (r *CannedResponseRepository) IncrementUsage(id int) error {
	query := database.ConvertPlaceholders(`
		UPDATE canned_response SET usage_count = usage_count + 1, last_used = NOW() WHERE id = ?
	`)
	_, err := r.db.Exec(query, id)
	return err
}

// CheckDuplicate checks if a response with the same name exists in the same scope.
func (r *CannedResponseRepository) CheckDuplicate(name, scope string, ownerID, teamID int) (bool, error) {
	var query string
	var args []interface{}

	switch scope {
	case "personal":
		query = database.ConvertPlaceholders(`
			SELECT COUNT(*) FROM canned_response 
			WHERE name = ? AND scope = 'personal' AND owner_id = ? AND valid_id = 1
		`)
		args = []interface{}{name, ownerID}
	case "team":
		query = database.ConvertPlaceholders(`
			SELECT COUNT(*) FROM canned_response 
			WHERE name = ? AND scope = 'team' AND team_id = ? AND valid_id = 1
		`)
		args = []interface{}{name, teamID}
	case "global":
		query = database.ConvertPlaceholders(`
			SELECT COUNT(*) FROM canned_response WHERE name = ? AND scope = 'global' AND valid_id = 1
		`)
		args = []interface{}{name}
	default:
		return false, nil
	}

	var count int
	err := r.db.QueryRow(query, args...).Scan(&count)
	return count > 0, err
}

// ListCategories returns all canned response categories.
func (r *CannedResponseRepository) ListCategories() ([]string, error) {
	query := `SELECT DISTINCT name FROM canned_response_category WHERE valid_id = 1 ORDER BY name`
	rows, err := r.db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var categories []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		categories = append(categories, name)
	}

	return categories, rows.Err()
}

// CannedResponseFilters holds filter options for listing responses.
type CannedResponseFilters struct {
	Category  string
	Scope     string
	Search    string
	Tags      []string
	SortBy    string
	SortOrder string
}
