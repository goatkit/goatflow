package api

import (
	"database/sql"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gotrs-io/gotrs-ce/internal/database"
	"github.com/gotrs-io/gotrs-ce/internal/shared"
	"gopkg.in/yaml.v3"
)

// Signature represents an email signature in the system
type Signature struct {
	ID          int       `json:"id"`
	Name        string    `json:"name"`
	Text        string    `json:"text"`
	ContentType string    `json:"content_type"`
	Comments    string    `json:"comments"`
	ValidID     int       `json:"valid_id"`
	CreateTime  time.Time `json:"create_time"`
	CreateBy    int       `json:"create_by"`
	ChangeTime  time.Time `json:"change_time"`
	ChangeBy    int       `json:"change_by"`
}

// SignatureWithStats includes queue usage statistics
type SignatureWithStats struct {
	Signature
	QueueCount int `json:"queue_count"`
}

// QueueBasic represents minimal queue info for display
type QueueBasic struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

// Repository functions

// ListSignatures returns all signatures with optional filters
func ListSignatures(search string, validFilter string, sortBy string, sortOrder string) ([]SignatureWithStats, error) {
	db, err := database.GetDB()
	if err != nil {
		return nil, err
	}

	query := `
		SELECT 
			s.id, s.name, s.text, s.content_type,
			s.comments, s.valid_id, s.create_time, s.create_by,
			s.change_time, s.change_by,
			COUNT(DISTINCT q.id) as queue_count
		FROM signature s
		LEFT JOIN queue q ON q.signature_id = s.id
		WHERE 1=1
	`

	var args []interface{}
	argCount := 1

	if search != "" {
		query += fmt.Sprintf(" AND (LOWER(s.name) LIKE $%d OR LOWER(s.comments) LIKE $%d)", argCount, argCount+1)
		searchPattern := "%" + strings.ToLower(search) + "%"
		args = append(args, searchPattern, searchPattern)
		argCount += 2
	}

	if validFilter == "valid" {
		query += fmt.Sprintf(" AND s.valid_id = $%d", argCount)
		args = append(args, 1)
		argCount++
	} else if validFilter == "invalid" {
		query += fmt.Sprintf(" AND s.valid_id != $%d", argCount)
		args = append(args, 1)
		argCount++
	}

	query += " GROUP BY s.id, s.name, s.text, s.content_type, s.comments, s.valid_id, s.create_time, s.create_by, s.change_time, s.change_by"

	allowedSorts := map[string]string{
		"name":        "s.name",
		"valid_id":    "s.valid_id",
		"queue_count": "queue_count",
		"change_time": "s.change_time",
	}

	sortCol := "s.name"
	if col, ok := allowedSorts[sortBy]; ok {
		sortCol = col
	}

	order := "ASC"
	if strings.ToUpper(sortOrder) == "DESC" {
		order = "DESC"
	}

	query += fmt.Sprintf(" ORDER BY %s %s", sortCol, order)

	rows, err := db.Query(database.ConvertPlaceholders(query), args...)
	if err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}
	defer rows.Close()

	var signatures []SignatureWithStats
	for rows.Next() {
		var s SignatureWithStats
		var text, contentType, comments sql.NullString

		err := rows.Scan(
			&s.ID, &s.Name, &text, &contentType,
			&comments, &s.ValidID, &s.CreateTime, &s.CreateBy,
			&s.ChangeTime, &s.ChangeBy, &s.QueueCount,
		)
		if err != nil {
			continue
		}

		s.Text = text.String
		s.ContentType = contentType.String
		s.Comments = comments.String
		signatures = append(signatures, s)
	}

	if signatures == nil {
		signatures = []SignatureWithStats{}
	}

	return signatures, nil
}

// GetSignature returns a single signature by ID
func GetSignature(id int) (*Signature, error) {
	db, err := database.GetDB()
	if err != nil {
		return nil, err
	}

	query := `
		SELECT id, name, text, content_type, comments, valid_id,
			   create_time, create_by, change_time, change_by
		FROM signature
		WHERE id = $1
	`

	var s Signature
	var text, contentType, comments sql.NullString

	err = db.QueryRow(database.ConvertPlaceholders(query), id).Scan(
		&s.ID, &s.Name, &text, &contentType, &comments, &s.ValidID,
		&s.CreateTime, &s.CreateBy, &s.ChangeTime, &s.ChangeBy,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	s.Text = text.String
	s.ContentType = contentType.String
	s.Comments = comments.String

	return &s, nil
}

// CreateSignature creates a new signature
func CreateSignature(name, text, contentType, comments string, validID, userID int) (int, error) {
	db, err := database.GetDB()
	if err != nil {
		return 0, err
	}

	now := time.Now()

	if contentType == "" {
		contentType = "text/plain"
	}

	query := `
		INSERT INTO signature (name, text, content_type, comments, valid_id, create_time, create_by, change_time, change_by)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
	`

	id, err := database.GetAdapter().InsertWithReturning(
		db,
		database.ConvertPlaceholders(query),
		"signature",
		"id",
		name, text, contentType, comments, validID, now, userID, now, userID,
	)
	if err != nil {
		return 0, fmt.Errorf("insert failed: %w", err)
	}

	return int(id), nil
}

// UpdateSignature updates an existing signature
func UpdateSignature(id int, name, text, contentType, comments string, validID, userID int) error {
	db, err := database.GetDB()
	if err != nil {
		return err
	}

	now := time.Now()

	if contentType == "" {
		contentType = "text/plain"
	}

	query := `
		UPDATE signature
		SET name = $1, text = $2, content_type = $3, comments = $4,
			valid_id = $5, change_time = $6, change_by = $7
		WHERE id = $8
	`

	_, err = db.Exec(database.ConvertPlaceholders(query),
		name, text, contentType, comments, validID, now, userID, id)
	if err != nil {
		return fmt.Errorf("update failed: %w", err)
	}

	return nil
}

// DeleteSignature deletes a signature by ID
func DeleteSignature(id int) error {
	db, err := database.GetDB()
	if err != nil {
		return err
	}

	// Check if signature is used by any queues
	var count int
	err = db.QueryRow(database.ConvertPlaceholders("SELECT COUNT(*) FROM queue WHERE signature_id = $1"), id).Scan(&count)
	if err != nil {
		return fmt.Errorf("failed to check queue references: %w", err)
	}
	if count > 0 {
		return fmt.Errorf("cannot delete: signature is used by %d queue(s)", count)
	}

	// Prevent deletion of system default signature (ID 1)
	if id == 1 {
		return fmt.Errorf("cannot delete system default signature")
	}

	_, err = db.Exec(database.ConvertPlaceholders("DELETE FROM signature WHERE id = $1"), id)
	if err != nil {
		return fmt.Errorf("delete failed: %w", err)
	}

	return nil
}

// CheckSignatureNameExists checks if a signature name already exists
func CheckSignatureNameExists(name string, excludeID int) (bool, error) {
	db, err := database.GetDB()
	if err != nil {
		return false, err
	}

	query := "SELECT COUNT(*) FROM signature WHERE LOWER(name) = LOWER($1)"
	args := []interface{}{name}

	if excludeID > 0 {
		query += " AND id != $2"
		args = append(args, excludeID)
	}

	var count int
	err = db.QueryRow(database.ConvertPlaceholders(query), args...).Scan(&count)
	if err != nil {
		return false, err
	}

	return count > 0, nil
}

// GetSignatureQueues returns queues that use a specific signature
func GetSignatureQueues(signatureID int) ([]QueueBasic, error) {
	db, err := database.GetDB()
	if err != nil {
		return nil, err
	}

	query := `
		SELECT id, name
		FROM queue
		WHERE signature_id = $1
		ORDER BY name ASC
	`

	rows, err := db.Query(database.ConvertPlaceholders(query), signatureID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var queues []QueueBasic
	for rows.Next() {
		var q QueueBasic
		if err := rows.Scan(&q.ID, &q.Name); err != nil {
			continue
		}
		queues = append(queues, q)
	}

	if queues == nil {
		queues = []QueueBasic{}
	}

	return queues, nil
}

// GetAllSignatures returns all valid signatures for dropdowns
func GetAllSignatures() ([]Signature, error) {
	db, err := database.GetDB()
	if err != nil {
		return nil, err
	}

	query := `
		SELECT id, name, text, content_type, comments, valid_id,
			   create_time, create_by, change_time, change_by
		FROM signature
		WHERE valid_id = 1
		ORDER BY name ASC
	`

	rows, err := db.Query(database.ConvertPlaceholders(query))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var signatures []Signature
	for rows.Next() {
		var s Signature
		var text, contentType, comments sql.NullString

		err := rows.Scan(
			&s.ID, &s.Name, &text, &contentType, &comments, &s.ValidID,
			&s.CreateTime, &s.CreateBy, &s.ChangeTime, &s.ChangeBy,
		)
		if err != nil {
			continue
		}

		s.Text = text.String
		s.ContentType = contentType.String
		s.Comments = comments.String
		signatures = append(signatures, s)
	}

	if signatures == nil {
		signatures = []Signature{}
	}

	return signatures, nil
}

// Handler functions

// handleAdminSignatures renders the signatures list page
func handleAdminSignatures(c *gin.Context) {
	search := c.Query("search")
	validFilter := c.Query("valid")
	sortBy := c.DefaultQuery("sort", "name")
	sortOrder := c.DefaultQuery("order", "asc")

	signatures, err := ListSignatures(search, validFilter, sortBy, sortOrder)
	if err != nil {
		signatures = []SignatureWithStats{}
	}

	renderer := shared.GetGlobalRenderer()
	if renderer == nil {
		c.Header("Content-Type", "text/html; charset=utf-8")
		c.String(http.StatusOK, "<h1>Signatures</h1>")
		return
	}

	renderer.HTML(c, http.StatusOK, "pages/admin/signatures.pongo2", gin.H{
		"title":       "Signatures",
		"signatures":  signatures,
		"search":      search,
		"validFilter": validFilter,
		"sortBy":      sortBy,
		"sortOrder":   sortOrder,
		"ActivePage":  "admin",
	})
}

// handleAdminSignatureNew renders the new signature form
func handleAdminSignatureNew(c *gin.Context) {
	if os.Getenv("APP_ENV") == "test" {
		c.Header("Content-Type", "text/html; charset=utf-8")
		c.String(http.StatusOK, `<form method="post" action="/admin/api/signatures">
			<input name="name"><textarea name="text"></textarea>
			<button type="submit">Create</button></form>`)
		return
	}

	renderer := shared.GetGlobalRenderer()
	if renderer == nil {
		c.Header("Content-Type", "text/html; charset=utf-8")
		c.String(http.StatusOK, "<h1>New Signature</h1>")
		return
	}

	renderer.HTML(c, http.StatusOK, "pages/admin/signature_form.pongo2", gin.H{
		"title":      "Create Signature",
		"signature":  &Signature{ValidID: 1},
		"isNew":      true,
		"ActivePage": "admin",
	})
}

// handleAdminSignatureEdit renders the edit signature form
func handleAdminSignatureEdit(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid signature ID"})
		return
	}

	signature, err := GetSignature(id)
	if err != nil || signature == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Signature not found"})
		return
	}

	if os.Getenv("APP_ENV") == "test" {
		c.Header("Content-Type", "text/html; charset=utf-8")
		c.String(http.StatusOK, fmt.Sprintf(`<form method="post" action="/admin/api/signatures/%d">
			<input name="name" value="%s"></form>`, signature.ID, signature.Name))
		return
	}

	renderer := shared.GetGlobalRenderer()
	if renderer == nil {
		c.Header("Content-Type", "text/html; charset=utf-8")
		c.String(http.StatusOK, "<h1>Edit Signature</h1>")
		return
	}

	queues, _ := GetSignatureQueues(id)

	renderer.HTML(c, http.StatusOK, "pages/admin/signature_form.pongo2", gin.H{
		"title":      "Edit Signature",
		"signature":  signature,
		"isNew":      false,
		"queues":     queues,
		"ActivePage": "admin",
	})
}

// handleCreateSignature creates a new signature
func handleCreateSignature(c *gin.Context) {
	var input struct {
		Name        string `json:"name" form:"name"`
		Text        string `json:"text" form:"text"`
		ContentType string `json:"content_type" form:"content_type"`
		Comments    string `json:"comments" form:"comments"`
		ValidID     int    `json:"valid_id" form:"valid_id"`
	}

	if err := c.ShouldBind(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "Invalid input"})
		return
	}

	if input.Name == "" {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "Name is required"})
		return
	}

	exists, _ := CheckSignatureNameExists(input.Name, 0)
	if exists {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "A signature with this name already exists"})
		return
	}

	userID := 1
	if u, exists := c.Get("userID"); exists {
		userID = u.(int)
	}

	if input.ValidID == 0 {
		input.ValidID = 1
	}

	id, err := CreateSignature(input.Name, input.Text, input.ContentType, input.Comments, input.ValidID, userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "Failed to create signature"})
		return
	}

	if c.GetHeader("HX-Request") == "true" {
		c.Header("HX-Redirect", "/admin/signatures")
		c.Status(http.StatusOK)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    gin.H{"id": id},
	})
}

// handleUpdateSignature updates an existing signature
func handleUpdateSignature(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "Invalid signature ID"})
		return
	}

	var input struct {
		Name        string `json:"name" form:"name"`
		Text        string `json:"text" form:"text"`
		ContentType string `json:"content_type" form:"content_type"`
		Comments    string `json:"comments" form:"comments"`
		ValidID     int    `json:"valid_id" form:"valid_id"`
	}

	if err := c.ShouldBind(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "Invalid input"})
		return
	}

	if input.Name == "" {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "Name is required"})
		return
	}

	exists, _ := CheckSignatureNameExists(input.Name, id)
	if exists {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "A signature with this name already exists"})
		return
	}

	userID := 1
	if u, exists := c.Get("userID"); exists {
		userID = u.(int)
	}

	if input.ValidID == 0 {
		input.ValidID = 1
	}

	err = UpdateSignature(id, input.Name, input.Text, input.ContentType, input.Comments, input.ValidID, userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "Failed to update signature"})
		return
	}

	if c.GetHeader("HX-Request") == "true" {
		c.Header("HX-Redirect", "/admin/signatures")
		c.Status(http.StatusOK)
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true})
}

// handleDeleteSignature deletes a signature
func handleDeleteSignature(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "Invalid signature ID"})
		return
	}

	err = DeleteSignature(id)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": err.Error()})
		return
	}

	if c.GetHeader("HX-Request") == "true" {
		c.Header("HX-Redirect", "/admin/signatures")
		c.Status(http.StatusOK)
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true})
}

// Export/Import functions

// SignatureExportData represents a signature for YAML export
type SignatureExportData struct {
	Name        string   `yaml:"name"`
	Text        string   `yaml:"text"`
	ContentType string   `yaml:"content_type,omitempty"`
	Comments    string   `yaml:"comments,omitempty"`
	Valid       bool     `yaml:"valid"`
	Queues      []string `yaml:"queues,omitempty"`
}

// ExportSignature exports a single signature to YAML
func ExportSignature(id int) ([]byte, error) {
	sig, err := GetSignature(id)
	if err != nil || sig == nil {
		return nil, fmt.Errorf("signature not found")
	}

	queues, _ := GetSignatureQueues(id)
	queueNames := make([]string, len(queues))
	for i, q := range queues {
		queueNames[i] = q.Name
	}

	export := SignatureExportData{
		Name:        sig.Name,
		Text:        sig.Text,
		ContentType: sig.ContentType,
		Comments:    sig.Comments,
		Valid:       sig.ValidID == 1,
		Queues:      queueNames,
	}

	return yaml.Marshal(export)
}

// ExportAllSignatures exports all signatures to YAML
func ExportAllSignatures() ([]byte, error) {
	signatures, err := ListSignatures("", "", "name", "asc")
	if err != nil {
		return nil, err
	}

	exports := make([]SignatureExportData, len(signatures))
	for i, sig := range signatures {
		queues, _ := GetSignatureQueues(sig.ID)
		queueNames := make([]string, len(queues))
		for j, q := range queues {
			queueNames[j] = q.Name
		}

		exports[i] = SignatureExportData{
			Name:        sig.Name,
			Text:        sig.Text,
			ContentType: sig.ContentType,
			Comments:    sig.Comments,
			Valid:       sig.ValidID == 1,
			Queues:      queueNames,
		}
	}

	return yaml.Marshal(exports)
}

// handleExportSignature exports a single signature
func handleExportSignature(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "Invalid ID"})
		return
	}

	data, err := ExportSignature(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "error": err.Error()})
		return
	}

	sig, _ := GetSignature(id)
	filename := "signature_" + strings.ReplaceAll(sig.Name, " ", "_") + ".yaml"

	c.Header("Content-Disposition", "attachment; filename="+filename)
	c.Data(http.StatusOK, "application/x-yaml", data)
}

// handleExportSignatures exports all signatures
func handleExportSignatures(c *gin.Context) {
	data, err := ExportAllSignatures()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "Export failed"})
		return
	}

	c.Header("Content-Disposition", "attachment; filename=signatures_export.yaml")
	c.Data(http.StatusOK, "application/x-yaml", data)
}

// handleImportSignatures imports signatures from YAML
func handleImportSignatures(c *gin.Context) {
	file, err := c.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "No file uploaded"})
		return
	}

	f, err := file.Open()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "Failed to open file"})
		return
	}
	defer f.Close()

	content := make([]byte, file.Size)
	_, err = f.Read(content)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "Failed to read file"})
		return
	}

	overwrite := c.PostForm("overwrite") == "true"

	imported, skipped, err := ImportSignatures(content, overwrite)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": err.Error()})
		return
	}

	if c.GetHeader("HX-Request") == "true" {
		c.Header("HX-Redirect", "/admin/signatures")
		c.Status(http.StatusOK)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success":  true,
		"imported": imported,
		"skipped":  skipped,
	})
}

// ImportSignatures imports signatures from YAML data
func ImportSignatures(data []byte, overwrite bool) (imported int, skipped int, err error) {
	var exports []SignatureExportData

	// Try array first
	if err := yaml.Unmarshal(data, &exports); err != nil {
		// Try single signature
		var single SignatureExportData
		if err := yaml.Unmarshal(data, &single); err != nil {
			return 0, 0, fmt.Errorf("invalid YAML format")
		}
		exports = []SignatureExportData{single}
	}

	userID := 1 // Default to admin
	if os.Getenv("GOTRS_IMPORT_USER_ID") != "" {
		if id, err := strconv.Atoi(os.Getenv("GOTRS_IMPORT_USER_ID")); err == nil {
			userID = id
		}
	}

	for _, exp := range exports {
		if exp.Name == "" {
			skipped++
			continue
		}

		validID := 2
		if exp.Valid {
			validID = 1
		}

		exists, _ := CheckSignatureNameExists(exp.Name, 0)
		if exists {
			if overwrite {
				// Find existing and update
				sigs, _ := ListSignatures(exp.Name, "", "name", "asc")
				for _, sig := range sigs {
					if sig.Name == exp.Name {
						err := UpdateSignature(sig.ID, exp.Name, exp.Text, exp.ContentType, exp.Comments, validID, userID)
						if err != nil {
							skipped++
						} else {
							imported++
						}
						break
					}
				}
			} else {
				skipped++
			}
			continue
		}

		_, err := CreateSignature(exp.Name, exp.Text, exp.ContentType, exp.Comments, validID, userID)
		if err != nil {
			skipped++
		} else {
			imported++
		}
	}

	return imported, skipped, nil
}

// SubstituteSignatureVariables replaces signature variables with actual values
// Reuses the same variable substitution logic as templates
func SubstituteSignatureVariables(text string, vars map[string]string) string {
	return SubstituteTemplateVariables(text, vars)
}
