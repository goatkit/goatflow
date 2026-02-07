package api

import (
	"database/sql"
	"net/http"
	"strconv"
	"strings"

	"github.com/flosch/pongo2/v6"
	"github.com/gin-gonic/gin"

	"github.com/goatkit/goatflow/internal/database"
)

type systemAddressDTO struct {
	ID          int    `json:"id"`
	Email       string `json:"email"`
	DisplayName string `json:"display_name"`
	QueueID     int    `json:"queue_id"`
	QueueName   string `json:"queue_name"`
	Comments    string `json:"comments"`
	ValidID     int    `json:"valid_id"`
}

// textEntityDTO is the common structure for salutations and signatures.
type textEntityDTO struct {
	ID          int    `json:"id"`
	Name        string `json:"name"`
	Text        string `json:"text"`
	ContentType string `json:"content_type"`
	Comments    string `json:"comments"`
	ValidID     int    `json:"valid_id"`
}

type salutationDTO = textEntityDTO
type signatureDTO = textEntityDTO

type queueOptionDTO struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

type systemAddressPayload struct {
	Email       string `json:"email"`
	DisplayName string `json:"display_name"`
	QueueID     int    `json:"queue_id"`
	Comments    string `json:"comments"`
	ValidID     int    `json:"valid_id"`
}

// textEntityPayload is the common payload for salutations and signatures.
type textEntityPayload struct {
	Name        string `json:"name"`
	Text        string `json:"text"`
	ContentType string `json:"content_type"`
	Comments    string `json:"comments"`
	ValidID     int    `json:"valid_id"`
}

type salutationPayload = textEntityPayload
type signaturePayload = textEntityPayload

// handleTextEntityUpdate handles common validation and delegation for salutation/signature updates.
func handleTextEntityUpdate(c *gin.Context, tableName, entityName string) {
	userID := resolveContextUserID(c)
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "Invalid " + entityName + " ID"})
		return
	}

	var payload textEntityPayload
	if err := c.ShouldBindJSON(&payload); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "Invalid payload"})
		return
	}

	payload.Name = strings.TrimSpace(payload.Name)
	payload.Text = strings.TrimSpace(payload.Text)
	payload.Comments = strings.TrimSpace(payload.Comments)
	payload.ContentType = normalizeContentType(payload.ContentType)
	payload.ValidID = sanitizeValidID(payload.ValidID)

	if payload.Name == "" {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "Name is required"})
		return
	}
	if payload.Text == "" {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "Text is required"})
		return
	}

	updateTextEntity(c, tableName, entityName, id, userID, payload)
}

func handleAdminEmailIdentities(c *gin.Context) {
	if htmxHandlerSkipDB() || getPongo2Renderer() == nil || getPongo2Renderer().TemplateSet() == nil {
		c.Data(http.StatusOK, "text/html; charset=utf-8", []byte("<main>Email Identities</main>"))
		return
	}

	db, err := database.GetDB()
	if err != nil {
		sendErrorResponse(c, http.StatusInternalServerError, "Database connection failed")
		return
	}

	addresses, err := fetchSystemAddresses(db)
	if err != nil {
		sendErrorResponse(c, http.StatusInternalServerError, "Failed to load system addresses")
		return
	}

	salutations, err := fetchSalutations(db)
	if err != nil {
		sendErrorResponse(c, http.StatusInternalServerError, "Failed to load salutations")
		return
	}

	signatures, err := fetchSignatures(db)
	if err != nil {
		sendErrorResponse(c, http.StatusInternalServerError, "Failed to load signatures")
		return
	}

	queues, err := fetchQueueOptions(db)
	if err != nil {
		sendErrorResponse(c, http.StatusInternalServerError, "Failed to load queues")
		return
	}

	tab := sanitizeEmailIdentityTab(c.Query("tab"))

	getPongo2Renderer().HTML(c, http.StatusOK, "pages/admin/email_identities.pongo2", pongo2.Context{
		"SystemAddresses": addresses,
		"Salutations":     salutations,
		"Signatures":      signatures,
		"Queues":          queues,
		"CurrentTab":      tab,
		"ActivePage":      "admin",
		"User":            getUserMapForTemplate(c),
	})
}

// HandleListSystemAddressesAPI handles GET /api/v1/system-addresses.
//
//	@Summary		List system addresses
//	@Description	List all system email addresses
//	@Tags			Email Identity
//	@Accept			json
//	@Produce		json
//	@Success		200	{object}	map[string]interface{}	"List of system addresses"
//	@Failure		401	{object}	map[string]interface{}	"Unauthorized"
//	@Security		BearerAuth
//	@Router			/system-addresses [get]
func HandleListSystemAddressesAPI(c *gin.Context) {
	db, err := database.GetDB()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "Database connection failed"})
		return
	}

	addresses, err := fetchSystemAddresses(db)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "Failed to load system addresses"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "data": addresses})
}

// HandleCreateSystemAddressAPI handles POST /api/v1/system-addresses.
//
//	@Summary		Create system address
//	@Description	Create a new system email address
//	@Tags			Email Identity
//	@Accept			json
//	@Produce		json
//	@Param			address	body		object	true	"System address data"
//	@Success		201		{object}	map[string]interface{}	"Created address"
//	@Failure		400		{object}	map[string]interface{}	"Invalid request"
//	@Failure		401		{object}	map[string]interface{}	"Unauthorized"
//	@Security		BearerAuth
//	@Router			/system-addresses [post]
func HandleCreateSystemAddressAPI(c *gin.Context) {
	userID := resolveContextUserID(c)

	var payload systemAddressPayload
	if err := c.ShouldBindJSON(&payload); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "Invalid payload"})
		return
	}

	payload.Email = strings.TrimSpace(payload.Email)
	payload.DisplayName = strings.TrimSpace(payload.DisplayName)
	payload.Comments = strings.TrimSpace(payload.Comments)
	payload.ValidID = sanitizeValidID(payload.ValidID)

	if payload.Email == "" {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "Email is required"})
		return
	}
	if payload.DisplayName == "" {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "Display name is required"})
		return
	}
	if payload.QueueID <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "Queue is required"})
		return
	}

	db, err := database.GetDB()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "Database connection failed"})
		return
	}

	adapter := database.GetAdapter()
	insert := database.ConvertPlaceholders(`
        INSERT INTO system_address (value0, value1, queue_id, comments, valid_id, create_time, create_by, change_time, change_by)
        VALUES (?, ?, ?, ?, ?, NOW(), ?, NOW(), ?)
        RETURNING id
    `)

	var comments sql.NullString
	if payload.Comments != "" {
		comments = sql.NullString{String: payload.Comments, Valid: true}
	}

	newID64, err := adapter.InsertWithReturning(
		db, insert, payload.Email, payload.DisplayName, payload.QueueID, comments, payload.ValidID, userID, userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "Failed to create system address"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"success": true,
		"data": systemAddressDTO{
			ID:          int(newID64),
			Email:       payload.Email,
			DisplayName: payload.DisplayName,
			QueueID:     payload.QueueID,
			QueueName:   lookupQueueName(db, payload.QueueID),
			Comments:    payload.Comments,
			ValidID:     payload.ValidID,
		},
	})
}

// HandleUpdateSystemAddressAPI handles PUT /api/v1/system-addresses/:id.
//
//	@Summary		Update system address
//	@Description	Update a system email address
//	@Tags			Email Identity
//	@Accept			json
//	@Produce		json
//	@Param			id		path		int		true	"Address ID"
//	@Param			address	body		object	true	"Address update data"
//	@Success		200		{object}	map[string]interface{}	"Updated address"
//	@Failure		400		{object}	map[string]interface{}	"Invalid request"
//	@Failure		401		{object}	map[string]interface{}	"Unauthorized"
//	@Failure		404		{object}	map[string]interface{}	"Address not found"
//	@Security		BearerAuth
//	@Router			/system-addresses/{id} [put]
func HandleUpdateSystemAddressAPI(c *gin.Context) {
	userID := resolveContextUserID(c)
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "Invalid system address ID"})
		return
	}

	var payload systemAddressPayload
	if err := c.ShouldBindJSON(&payload); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "Invalid payload"})
		return
	}

	payload.Email = strings.TrimSpace(payload.Email)
	payload.DisplayName = strings.TrimSpace(payload.DisplayName)
	payload.Comments = strings.TrimSpace(payload.Comments)
	payload.ValidID = sanitizeValidID(payload.ValidID)

	if payload.Email == "" {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "Email is required"})
		return
	}
	if payload.DisplayName == "" {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "Display name is required"})
		return
	}
	if payload.QueueID <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "Queue is required"})
		return
	}

	db, err := database.GetDB()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "Database connection failed"})
		return
	}

	var comments sql.NullString
	if payload.Comments != "" {
		comments = sql.NullString{String: payload.Comments, Valid: true}
	}

	update := database.ConvertPlaceholders(`
        UPDATE system_address
        SET value0 = ?, value1 = ?, queue_id = ?, comments = ?, valid_id = ?, change_time = NOW(), change_by = ?
        WHERE id = ?
    `)

	if _, err := db.Exec(update, payload.Email, payload.DisplayName, payload.QueueID, comments, payload.ValidID, userID, id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "Failed to update system address"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": systemAddressDTO{
			ID:          id,
			Email:       payload.Email,
			DisplayName: payload.DisplayName,
			QueueID:     payload.QueueID,
			QueueName:   lookupQueueName(db, payload.QueueID),
			Comments:    payload.Comments,
			ValidID:     payload.ValidID,
		},
	})
}

// HandleListSalutationsAPI handles GET /api/v1/salutations.
//
//	@Summary		List salutations
//	@Description	List all email salutations
//	@Tags			Email Identity
//	@Accept			json
//	@Produce		json
//	@Success		200	{object}	map[string]interface{}	"List of salutations"
//	@Failure		401	{object}	map[string]interface{}	"Unauthorized"
//	@Security		BearerAuth
//	@Router			/salutations [get]
func HandleListSalutationsAPI(c *gin.Context) {
	db, err := database.GetDB()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "Database connection failed"})
		return
	}

	items, err := fetchSalutations(db)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "Failed to load salutations"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "data": items})
}

// HandleCreateSalutationAPI handles POST /api/v1/salutations.
//
//	@Summary		Create salutation
//	@Description	Create a new email salutation
//	@Tags			Email Identity
//	@Accept			json
//	@Produce		json
//	@Param			salutation	body		object	true	"Salutation data"
//	@Success		201			{object}	map[string]interface{}	"Created salutation"
//	@Failure		400			{object}	map[string]interface{}	"Invalid request"
//	@Failure		401			{object}	map[string]interface{}	"Unauthorized"
//	@Security		BearerAuth
//	@Router			/salutations [post]
func HandleCreateSalutationAPI(c *gin.Context) {
	userID := resolveContextUserID(c)

	var payload salutationPayload
	if err := c.ShouldBindJSON(&payload); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "Invalid payload"})
		return
	}

	payload.Name = strings.TrimSpace(payload.Name)
	payload.Text = strings.TrimSpace(payload.Text)
	payload.Comments = strings.TrimSpace(payload.Comments)
	payload.ContentType = normalizeContentType(payload.ContentType)
	payload.ValidID = sanitizeValidID(payload.ValidID)

	if payload.Name == "" {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "Name is required"})
		return
	}
	if payload.Text == "" {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "Text is required"})
		return
	}

	db, err := database.GetDB()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "Database connection failed"})
		return
	}

	var comments sql.NullString
	if payload.Comments != "" {
		comments = sql.NullString{String: payload.Comments, Valid: true}
	}

	adapter := database.GetAdapter()
	insert := database.ConvertPlaceholders(`
        INSERT INTO salutation (name, text, content_type, comments, valid_id, create_time, create_by, change_time, change_by)
        VALUES (?, ?, ?, ?, ?, NOW(), ?, NOW(), ?)
        RETURNING id
    `)

	newID64, err := adapter.InsertWithReturning(
		db, insert, payload.Name, payload.Text, payload.ContentType, comments, payload.ValidID, userID, userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "Failed to create salutation"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"success": true,
		"data": salutationDTO{
			ID:          int(newID64),
			Name:        payload.Name,
			Text:        payload.Text,
			ContentType: payload.ContentType,
			Comments:    payload.Comments,
			ValidID:     payload.ValidID,
		},
	})
}

// HandleUpdateSalutationAPI handles PUT /api/v1/salutations/:id.
//
//	@Summary		Update salutation
//	@Description	Update an email salutation
//	@Tags			Email Identity
//	@Accept			json
//	@Produce		json
//	@Param			id			path		int		true	"Salutation ID"
//	@Param			salutation	body		object	true	"Salutation update data"
//	@Success		200			{object}	map[string]interface{}	"Updated salutation"
//	@Failure		400			{object}	map[string]interface{}	"Invalid request"
//	@Failure		401			{object}	map[string]interface{}	"Unauthorized"
//	@Security		BearerAuth
//	@Router			/salutations/{id} [put]
func HandleUpdateSalutationAPI(c *gin.Context) {
	handleTextEntityUpdate(c, "salutation", "salutation")
}

// HandleListSignaturesAPI handles GET /api/v1/signatures.
//
//	@Summary		List signatures
//	@Description	List all email signatures
//	@Tags			Email Identity
//	@Accept			json
//	@Produce		json
//	@Success		200	{object}	map[string]interface{}	"List of signatures"
//	@Failure		401	{object}	map[string]interface{}	"Unauthorized"
//	@Security		BearerAuth
//	@Router			/signatures [get]
func HandleListSignaturesAPI(c *gin.Context) {
	db, err := database.GetDB()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "Database connection failed"})
		return
	}

	items, err := fetchSignatures(db)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "Failed to load signatures"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "data": items})
}

// HandleCreateSignatureAPI handles POST /api/v1/signatures.
//
//	@Summary		Create signature
//	@Description	Create a new email signature
//	@Tags			Email Identity
//	@Accept			json
//	@Produce		json
//	@Param			signature	body		object	true	"Signature data"
//	@Success		201			{object}	map[string]interface{}	"Created signature"
//	@Failure		400			{object}	map[string]interface{}	"Invalid request"
//	@Failure		401			{object}	map[string]interface{}	"Unauthorized"
//	@Security		BearerAuth
//	@Router			/signatures [post]
func HandleCreateSignatureAPI(c *gin.Context) {
	userID := resolveContextUserID(c)

	var payload signaturePayload
	if err := c.ShouldBindJSON(&payload); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "Invalid payload"})
		return
	}

	payload.Name = strings.TrimSpace(payload.Name)
	payload.Text = strings.TrimSpace(payload.Text)
	payload.Comments = strings.TrimSpace(payload.Comments)
	payload.ContentType = normalizeContentType(payload.ContentType)
	payload.ValidID = sanitizeValidID(payload.ValidID)

	if payload.Name == "" {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "Name is required"})
		return
	}
	if payload.Text == "" {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "Text is required"})
		return
	}

	db, err := database.GetDB()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "Database connection failed"})
		return
	}

	var comments sql.NullString
	if payload.Comments != "" {
		comments = sql.NullString{String: payload.Comments, Valid: true}
	}

	insert := database.ConvertPlaceholders(`
        INSERT INTO signature (name, text, content_type, comments, valid_id, create_time, create_by, change_time, change_by)
        VALUES (?, ?, ?, ?, ?, NOW(), ?, NOW(), ?)
        RETURNING id
    `)

	adapter := database.GetAdapter()
	newID64, err := adapter.InsertWithReturning(
		db, insert, payload.Name, payload.Text, payload.ContentType, comments, payload.ValidID, userID, userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "Failed to create signature"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"success": true,
		"data": signatureDTO{
			ID:          int(newID64),
			Name:        payload.Name,
			Text:        payload.Text,
			ContentType: payload.ContentType,
			Comments:    payload.Comments,
			ValidID:     payload.ValidID,
		},
	})
}

// HandleUpdateSignatureAPI handles PUT /api/v1/signatures/:id.
//
//	@Summary		Update signature
//	@Description	Update an email signature
//	@Tags			Email Identity
//	@Accept			json
//	@Produce		json
//	@Param			id			path		int		true	"Signature ID"
//	@Param			signature	body		object	true	"Signature update data"
//	@Success		200			{object}	map[string]interface{}	"Updated signature"
//	@Failure		400			{object}	map[string]interface{}	"Invalid request"
//	@Failure		401			{object}	map[string]interface{}	"Unauthorized"
//	@Security		BearerAuth
//	@Router			/signatures/{id} [put]
func HandleUpdateSignatureAPI(c *gin.Context) {
	handleTextEntityUpdate(c, "signature", "signature")
}

func fetchSystemAddresses(db *sql.DB) ([]systemAddressDTO, error) {
	query := database.ConvertPlaceholders(`
        SELECT sa.id, sa.value0, sa.value1, sa.queue_id, q.name, sa.comments, sa.valid_id
        FROM system_address sa
        LEFT JOIN queue q ON q.id = sa.queue_id
        ORDER BY sa.id
    `)

	rows, err := db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []systemAddressDTO
	for rows.Next() {
		var item systemAddressDTO
		var queueID sql.NullInt64
		var queueName sql.NullString
		var comments sql.NullString

		if err := rows.Scan(&item.ID, &item.Email, &item.DisplayName, &queueID, &queueName, &comments, &item.ValidID); err != nil {
			return nil, err
		}

		if queueID.Valid {
			item.QueueID = int(queueID.Int64)
		}
		if queueName.Valid {
			item.QueueName = queueName.String
		}
		if comments.Valid {
			item.Comments = comments.String
		}

		items = append(items, item)
	}

	return items, rows.Err()
}

// fetchTextEntities retrieves salutations or signatures from the specified table.
func fetchTextEntities(db *sql.DB, tableName string) ([]textEntityDTO, error) {
	query := database.ConvertPlaceholders(`
        SELECT id, name, text, content_type, comments, valid_id
        FROM ` + tableName + `
        ORDER BY name
    `)

	rows, err := db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []textEntityDTO
	for rows.Next() {
		var item textEntityDTO
		var contentType sql.NullString
		var comments sql.NullString

		if err := rows.Scan(&item.ID, &item.Name, &item.Text, &contentType, &comments, &item.ValidID); err != nil {
			return nil, err
		}

		if contentType.Valid {
			item.ContentType = contentType.String
		} else {
			item.ContentType = "text/plain"
		}
		if comments.Valid {
			item.Comments = comments.String
		}

		items = append(items, item)
	}

	return items, rows.Err()
}

// updateTextEntity updates a salutation or signature in the database.
func updateTextEntity(
	c *gin.Context, tableName, entityName string, id, userID int, payload textEntityPayload,
) {
	db, err := database.GetDB()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "Database connection failed"})
		return
	}

	var comments sql.NullString
	if payload.Comments != "" {
		comments = sql.NullString{String: payload.Comments, Valid: true}
	}

	update := database.ConvertPlaceholders(`
        UPDATE ` + tableName + `
        SET name = ?, text = ?, content_type = ?, comments = ?, valid_id = ?, change_time = NOW(), change_by = ?
        WHERE id = ?
    `)

	if _, err := db.Exec(
		update, payload.Name, payload.Text, payload.ContentType, comments, payload.ValidID, userID, id,
	); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false, "error": "Failed to update " + entityName,
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": textEntityDTO{
			ID:          id,
			Name:        payload.Name,
			Text:        payload.Text,
			ContentType: payload.ContentType,
			Comments:    payload.Comments,
			ValidID:     payload.ValidID,
		},
	})
}

func fetchSalutations(db *sql.DB) ([]salutationDTO, error) {
	return fetchTextEntities(db, "salutation")
}

func fetchSignatures(db *sql.DB) ([]signatureDTO, error) {
	return fetchTextEntities(db, "signature")
}

func fetchQueueOptions(db *sql.DB) ([]queueOptionDTO, error) {
	rows, err := db.Query("SELECT id, name FROM queue WHERE valid_id = 1 ORDER BY name")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []queueOptionDTO
	for rows.Next() {
		var item queueOptionDTO
		if err := rows.Scan(&item.ID, &item.Name); err != nil {
			return nil, err
		}
		items = append(items, item)
	}

	return items, rows.Err()
}

func lookupQueueName(db *sql.DB, queueID int) string {
	if queueID <= 0 {
		return ""
	}

	var name string
	if err := db.QueryRow(database.ConvertPlaceholders("SELECT name FROM queue WHERE id = ?"), queueID).Scan(&name); err != nil {
		return ""
	}

	return name
}

func sanitizeValidID(value int) int {
	switch value {
	case 1, 2:
		return value
	default:
		return 1
	}
}

func normalizeContentType(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "text/html":
		return "text/html"
	default:
		return "text/plain"
	}
}

func sanitizeEmailIdentityTab(tab string) string {
	switch strings.ToLower(strings.TrimSpace(tab)) {
	case "salutations":
		return "salutations"
	case "signatures":
		return "signatures"
	default:
		return "system-addresses"
	}
}

func resolveContextUserID(c *gin.Context) int {
	if raw, ok := c.Get("user_id"); ok {
		if id := normalizeUserID(raw); id > 0 {
			return id
		}
	}

	if user := getUserFromContext(c); user != nil && user.ID > 0 {
		return int(user.ID)
	}

	return 1
}
