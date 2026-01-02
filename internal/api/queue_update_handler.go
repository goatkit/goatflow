package api

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/gotrs-io/gotrs-ce/internal/database"
)

type queueUpdateRequest struct {
	Name            *string `json:"name"`
	GroupID         *int    `json:"group_id"`
	SystemAddressID *int    `json:"system_address_id"`
	SalutationID    *int    `json:"salutation_id"`
	SignatureID     *int    `json:"signature_id"`
	UnlockTimeout   *int    `json:"unlock_timeout"`
	FollowUpID      *int    `json:"follow_up_id"`
	FollowUpLock    *int    `json:"follow_up_lock"`
	Comments        *string `json:"comments"`
	ValidID         *int    `json:"valid_id"`
	GroupAccess     *[]int  `json:"group_access"`
}

// HandleUpdateQueueAPI handles PUT /api/v1/queues/:id.
func HandleUpdateQueueAPI(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	queueID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid queue ID"})
		return
	}

	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}
	if len(body) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Request body required"})
		return
	}
	c.Request.Body = io.NopCloser(bytes.NewBuffer(body))

	var req queueUpdateRequest
	if err := json.Unmarshal(body, &req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	raw := map[string]json.RawMessage{}
	if err := json.Unmarshal(body, &raw); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid JSON payload"})
		return
	}

	db, err := database.GetDB()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database connection failed"})
		return
	}

	var current struct {
		Name            string
		GroupID         int
		SystemAddressID sql.NullInt64
		SalutationID    sql.NullInt64
		SignatureID     sql.NullInt64
		UnlockTimeout   sql.NullInt64
		FollowUpID      sql.NullInt64
		FollowUpLock    sql.NullInt64
		Comments        sql.NullString
		ValidID         int
	}

	currentQuery := database.ConvertPlaceholders(`
		SELECT name, group_id, system_address_id, salutation_id, signature_id,
		       unlock_timeout, follow_up_id, follow_up_lock, comments, valid_id
		FROM queue
		WHERE id = $1
	`)
	if err := db.QueryRow(currentQuery, queueID).Scan(
		&current.Name,
		&current.GroupID,
		&current.SystemAddressID,
		&current.SalutationID,
		&current.SignatureID,
		&current.UnlockTimeout,
		&current.FollowUpID,
		&current.FollowUpLock,
		&current.Comments,
		&current.ValidID,
	); err != nil {
		if err == sql.ErrNoRows {
			c.JSON(http.StatusNotFound, gin.H{"error": "Queue not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load queue"})
		return
	}

	nameProvided := fieldProvided(raw, "name")
	groupProvided := req.GroupID != nil
	systemAddressProvided := fieldProvided(raw, "system_address_id")
	salutationProvided := fieldProvided(raw, "salutation_id")
	signatureProvided := fieldProvided(raw, "signature_id")
	unlockProvided := req.UnlockTimeout != nil
	followUpProvided := req.FollowUpID != nil
	followLockProvided := req.FollowUpLock != nil
	commentsProvided := fieldProvided(raw, "comments")
	validProvided := req.ValidID != nil
	groupAccessProvided := req.GroupAccess != nil

	name := current.Name
	if nameProvided {
		if req.Name == nil || strings.TrimSpace(*req.Name) == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Queue name cannot be empty"})
			return
		}
		name = strings.TrimSpace(*req.Name)

		var duplicateCount int
		dupQuery := database.ConvertPlaceholders(`
			SELECT COUNT(*) FROM queue
			WHERE LOWER(name) = LOWER($1) AND id <> $2
		`)
		if err := db.QueryRow(dupQuery, name, queueID).Scan(&duplicateCount); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to validate queue name"})
			return
		}
		if duplicateCount > 0 {
			c.JSON(http.StatusConflict, gin.H{"error": "Queue name already exists"})
			return
		}
	}

	groupID := current.GroupID
	if groupProvided {
		if req.GroupID == nil || *req.GroupID <= 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "group_id must be greater than zero"})
			return
		}
		groupID = *req.GroupID
	}

	systemAddress := current.SystemAddressID
	if systemAddressProvided {
		if req.SystemAddressID != nil {
			systemAddress = sql.NullInt64{Int64: int64(*req.SystemAddressID), Valid: true}
		} else {
			systemAddress = sql.NullInt64{}
		}
	}

	salutation := current.SalutationID
	if salutationProvided {
		if req.SalutationID != nil {
			if *req.SalutationID <= 0 {
				c.JSON(http.StatusBadRequest, gin.H{"error": "salutation_id must be positive"})
				return
			}
			salutation = sql.NullInt64{Int64: int64(*req.SalutationID), Valid: true}
		} else {
			salutation = sql.NullInt64{}
		}
	}

	signature := current.SignatureID
	if signatureProvided {
		if req.SignatureID != nil {
			if *req.SignatureID <= 0 {
				c.JSON(http.StatusBadRequest, gin.H{"error": "signature_id must be positive"})
				return
			}
			signature = sql.NullInt64{Int64: int64(*req.SignatureID), Valid: true}
		} else {
			signature = sql.NullInt64{}
		}
	}

	unlockTimeout := nullIntToInt(current.UnlockTimeout, 0)
	if unlockProvided {
		if req.UnlockTimeout == nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "unlock_timeout cannot be null"})
			return
		}
		if *req.UnlockTimeout < 0 {
			*req.UnlockTimeout = 0
		}
		unlockTimeout = *req.UnlockTimeout
	}

	followUpID := nullIntToInt(current.FollowUpID, 1)
	if followUpProvided {
		if req.FollowUpID == nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "follow_up_id cannot be null"})
			return
		}
		followUpID = *req.FollowUpID
	}

	followUpLock := nullIntToInt(current.FollowUpLock, 0)
	if followLockProvided {
		if req.FollowUpLock == nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "follow_up_lock cannot be null"})
			return
		}
		if *req.FollowUpLock != 0 && *req.FollowUpLock != 1 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "follow_up_lock must be 0 or 1"})
			return
		}
		followUpLock = *req.FollowUpLock
	}

	comment := current.Comments
	if commentsProvided {
		if req.Comments != nil {
			comment = sql.NullString{String: *req.Comments, Valid: true}
		} else {
			comment = sql.NullString{}
		}
	}

	validID := current.ValidID
	if validProvided {
		if req.ValidID == nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "valid_id cannot be null"})
			return
		}
		if *req.ValidID != 1 && *req.ValidID != 2 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "valid_id must be 1 or 2"})
			return
		}
		validID = *req.ValidID
	}

	updateRequired := nameProvided || groupProvided || systemAddressProvided ||
		salutationProvided || signatureProvided || unlockProvided ||
		followUpProvided || followLockProvided || commentsProvided || validProvided
	changeBy := normalizeUserID(userID)

	tx, err := db.Begin()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to start transaction"})
		return
	}
	defer func() { _ = tx.Rollback() }()

	if updateRequired {
		updateQuery := database.ConvertPlaceholders(`
			UPDATE queue SET
				name = $1,
				group_id = $2,
				system_address_id = $3,
				salutation_id = $4,
				signature_id = $5,
				unlock_timeout = $6,
				follow_up_id = $7,
				follow_up_lock = $8,
				comments = $9,
				valid_id = $10,
				change_time = NOW(),
				change_by = $11
			WHERE id = $12
		`)
		if _, err := tx.Exec(updateQuery,
			name,
			groupID,
			nullIntArg(systemAddress),
			nullIntArg(salutation),
			nullIntArg(signature),
			unlockTimeout,
			followUpID,
			followUpLock,
			nullStringArg(comment),
			validID,
			changeBy,
			queueID,
		); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update queue"})
			return
		}
	}

	if groupAccessProvided {
		deleteQuery := database.ConvertPlaceholders(`
			DELETE FROM queue_group WHERE queue_id = $1
		`)
		if _, err := tx.Exec(deleteQuery, queueID); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update group access"})
			return
		}

		if req.GroupAccess != nil {
			for _, gid := range *req.GroupAccess {
				insertQuery := database.ConvertPlaceholders(`
					INSERT INTO queue_group (queue_id, group_id)
					VALUES ($1, $2)
				`)
				if _, err := tx.Exec(insertQuery, queueID, gid); err != nil {
					c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update group access"})
					return
				}
			}
		}
	}

	if err := tx.Commit(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to commit transaction"})
		return
	}

	resp := gin.H{
		"id":             queueID,
		"name":           name,
		"group_id":       groupID,
		"unlock_timeout": unlockTimeout,
		"follow_up_id":   followUpID,
		"follow_up_lock": followUpLock,
		"valid_id":       validID,
	}
	if systemAddress.Valid {
		resp["system_address_id"] = int(systemAddress.Int64)
	}
	if salutation.Valid {
		resp["salutation_id"] = int(salutation.Int64)
	}
	if signature.Valid {
		resp["signature_id"] = int(signature.Int64)
	}
	if comment.Valid {
		resp["comments"] = comment.String
	}

	groupQuery := database.ConvertPlaceholders(`
		SELECT group_id FROM queue_group
		WHERE queue_id = $1
	`)
	rows, err := db.Query(groupQuery, queueID)
	if err == nil {
		defer rows.Close()
		var groups []int
		for rows.Next() {
			var gid int
			if err := rows.Scan(&gid); err == nil {
				groups = append(groups, gid)
			}
		}
		_ = rows.Err() //nolint:errcheck // Check for iteration errors
		resp["group_access"] = groups
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "data": resp})
}

func fieldProvided(raw map[string]json.RawMessage, key string) bool {
	_, ok := raw[key]
	return ok
}

func nullIntToInt(n sql.NullInt64, fallback int) int {
	if n.Valid {
		return int(n.Int64)
	}
	return fallback
}

func nullIntArg(n sql.NullInt64) interface{} {
	if n.Valid {
		return n.Int64
	}
	return nil
}

func nullStringArg(s sql.NullString) interface{} {
	if s.Valid {
		return s.String
	}
	return nil
}
