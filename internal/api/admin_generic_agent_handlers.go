package api

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/flosch/pongo2/v6"
	"github.com/gin-gonic/gin"

	"github.com/gotrs-io/gotrs-ce/internal/database"
)

// GenericAgentJob represents a generic agent job configuration.
// Generic agent jobs are stored as key-value pairs in the database.
type GenericAgentJob struct {
	Name   string            `json:"name"`
	Valid  bool              `json:"valid"`
	Config map[string]string `json:"config"`
}

// handleAdminGenericAgent renders the admin generic agent management page.
func handleAdminGenericAgent(c *gin.Context) {
	db, err := database.GetDB()
	if err != nil {
		sendErrorResponse(c, http.StatusInternalServerError, "Database connection failed")
		return
	}

	// Get search parameter
	searchQuery := c.Query("search")
	validFilter := c.DefaultQuery("valid", "all")

	// Get all unique job names
	query := `SELECT DISTINCT job_name FROM generic_agent_jobs ORDER BY job_name`
	rows, err := db.Query(query)
	if err != nil {
		sendErrorResponse(c, http.StatusInternalServerError, "Failed to fetch generic agent jobs")
		return
	}
	defer rows.Close()

	var jobNames []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err == nil {
			// Apply search filter
			if searchQuery != "" && !strings.Contains(strings.ToLower(name), strings.ToLower(searchQuery)) {
				continue
			}
			jobNames = append(jobNames, name)
		}
	}
	_ = rows.Err()

	// Build job details for each job
	var jobs []GenericAgentJob
	for _, jobName := range jobNames {
		job := GenericAgentJob{
			Name:   jobName,
			Valid:  true, // Default to valid
			Config: make(map[string]string),
		}

		// Get all key-value pairs for this job
		kvQuery := database.ConvertPlaceholders(`
			SELECT job_key, COALESCE(job_value, '')
			FROM generic_agent_jobs
			WHERE job_name = ?
		`)
		kvRows, err := db.Query(kvQuery, jobName)
		if err == nil {
			defer kvRows.Close()
			for kvRows.Next() {
				var key, value string
				if err := kvRows.Scan(&key, &value); err == nil {
					job.Config[key] = value
					// Check for Valid key
					if key == "Valid" && value == "0" {
						job.Valid = false
					}
				}
			}
			_ = kvRows.Err()
		}

		// Apply valid filter
		if validFilter == "valid" && !job.Valid {
			continue
		}
		if validFilter == "invalid" && job.Valid {
			continue
		}

		jobs = append(jobs, job)
	}

	// Check if JSON response is requested
	if strings.Contains(c.GetHeader("Accept"), "application/json") {
		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"data":    jobs,
		})
		return
	}

	// Render the template
	getPongo2Renderer().HTML(c, http.StatusOK, "pages/admin/generic_agent.pongo2", pongo2.Context{
		"Title":       "Generic Agent Jobs",
		"Jobs":        jobs,
		"SearchQuery": searchQuery,
		"ValidFilter": validFilter,
		"User":        getUserMapForTemplate(c),
		"ActivePage":  "admin",
	})
}

// handleAdminGenericAgentCreate creates a new generic agent job.
func handleAdminGenericAgentCreate(c *gin.Context) {
	var input struct {
		Name   string            `json:"name" binding:"required"`
		Valid  bool              `json:"valid"`
		Config map[string]string `json:"config"`
	}

	// Default valid to true
	input.Valid = true

	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "Job name is required",
		})
		return
	}

	// Validate name is not empty
	if strings.TrimSpace(input.Name) == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "Job name is required",
		})
		return
	}

	db, err := database.GetDB()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "Database connection failed",
		})
		return
	}

	// Check for duplicate name
	var exists bool
	err = db.QueryRow(database.ConvertPlaceholders("SELECT EXISTS(SELECT 1 FROM generic_agent_jobs WHERE job_name = ? LIMIT 1)"), input.Name).Scan(&exists)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "Failed to check for duplicate",
		})
		return
	}

	if exists {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "A job with this name already exists",
		})
		return
	}

	// Start transaction
	tx, err := db.Begin()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "Failed to start transaction",
		})
		return
	}

	// Add Valid key to config
	if input.Config == nil {
		input.Config = make(map[string]string)
	}
	if input.Valid {
		input.Config["Valid"] = "1"
	} else {
		input.Config["Valid"] = "0"
	}

	// Insert all key-value pairs
	insertQuery := database.ConvertPlaceholders(`
		INSERT INTO generic_agent_jobs (job_name, job_key, job_value)
		VALUES (?, ?, ?)
	`)

	for key, value := range input.Config {
		_, err := tx.Exec(insertQuery, input.Name, key, value)
		if err != nil {
			tx.Rollback()
			c.JSON(http.StatusInternalServerError, gin.H{
				"success": false,
				"error":   "Failed to create job: " + err.Error(),
			})
			return
		}
	}

	if err := tx.Commit(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "Failed to commit transaction",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Generic agent job created successfully",
		"data": gin.H{
			"name": input.Name,
		},
	})
}

// handleAdminGenericAgentUpdate updates an existing generic agent job.
func handleAdminGenericAgentUpdate(c *gin.Context) {
	jobName := c.Param("name")
	if jobName == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "Job name is required",
		})
		return
	}

	var input struct {
		Name   *string           `json:"name"`
		Valid  *bool             `json:"valid"`
		Config map[string]string `json:"config"`
	}

	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "Invalid input",
		})
		return
	}

	db, err := database.GetDB()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "Database connection failed",
		})
		return
	}

	// Check if job exists
	var exists bool
	err = db.QueryRow(database.ConvertPlaceholders("SELECT EXISTS(SELECT 1 FROM generic_agent_jobs WHERE job_name = ? LIMIT 1)"), jobName).Scan(&exists)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "Database query failed",
		})
		return
	}

	if !exists {
		c.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"error":   "Job not found",
		})
		return
	}

	// Start transaction
	tx, err := db.Begin()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "Failed to start transaction",
		})
		return
	}

	// If renaming, update all rows with the new name
	newName := jobName
	if input.Name != nil && *input.Name != "" && *input.Name != jobName {
		// Check if new name already exists
		err = db.QueryRow(database.ConvertPlaceholders("SELECT EXISTS(SELECT 1 FROM generic_agent_jobs WHERE job_name = ? LIMIT 1)"), *input.Name).Scan(&exists)
		if err == nil && exists {
			tx.Rollback()
			c.JSON(http.StatusBadRequest, gin.H{
				"success": false,
				"error":   "A job with this name already exists",
			})
			return
		}

		_, err := tx.Exec(database.ConvertPlaceholders("UPDATE generic_agent_jobs SET job_name = ? WHERE job_name = ?"), *input.Name, jobName)
		if err != nil {
			tx.Rollback()
			c.JSON(http.StatusInternalServerError, gin.H{
				"success": false,
				"error":   "Failed to rename job",
			})
			return
		}
		newName = *input.Name
	}

	// Update Valid status if provided
	if input.Valid != nil {
		validValue := "1"
		if !*input.Valid {
			validValue = "0"
		}

		// Delete existing Valid key and insert new one
		tx.Exec(database.ConvertPlaceholders("DELETE FROM generic_agent_jobs WHERE job_name = ? AND job_key = 'Valid'"), newName)
		_, err := tx.Exec(database.ConvertPlaceholders("INSERT INTO generic_agent_jobs (job_name, job_key, job_value) VALUES (?, 'Valid', ?)"), newName, validValue)
		if err != nil {
			tx.Rollback()
			c.JSON(http.StatusInternalServerError, gin.H{
				"success": false,
				"error":   "Failed to update job status",
			})
			return
		}
	}

	// Update config if provided
	if input.Config != nil {
		for key, value := range input.Config {
			// Delete existing key and insert new one (upsert pattern)
			tx.Exec(database.ConvertPlaceholders("DELETE FROM generic_agent_jobs WHERE job_name = ? AND job_key = ?"), newName, key)
			_, err := tx.Exec(database.ConvertPlaceholders("INSERT INTO generic_agent_jobs (job_name, job_key, job_value) VALUES (?, ?, ?)"), newName, key, value)
			if err != nil {
				tx.Rollback()
				c.JSON(http.StatusInternalServerError, gin.H{
					"success": false,
					"error":   "Failed to update job config",
				})
				return
			}
		}
	}

	if err := tx.Commit(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "Failed to commit transaction",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Generic agent job updated successfully",
	})
}

// handleAdminGenericAgentDelete deletes a generic agent job.
func handleAdminGenericAgentDelete(c *gin.Context) {
	jobName := c.Param("name")
	if jobName == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "Job name is required",
		})
		return
	}

	db, err := database.GetDB()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "Database connection failed",
		})
		return
	}

	// Delete all key-value pairs for this job
	result, err := db.Exec(database.ConvertPlaceholders("DELETE FROM generic_agent_jobs WHERE job_name = ?"), jobName)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "Failed to delete job",
		})
		return
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		rowsAffected = 0
	}
	if rowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"error":   "Job not found",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": fmt.Sprintf("Generic agent job '%s' deleted successfully", jobName),
	})
}

// handleAdminGenericAgentGet returns a single generic agent job by name.
func handleAdminGenericAgentGet(c *gin.Context) {
	jobName := c.Param("name")
	if jobName == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "Job name is required",
		})
		return
	}

	db, err := database.GetDB()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "Database connection failed",
		})
		return
	}

	// Get all key-value pairs for this job
	query := database.ConvertPlaceholders(`
		SELECT job_key, COALESCE(job_value, '')
		FROM generic_agent_jobs
		WHERE job_name = ?
	`)

	rows, err := db.Query(query, jobName)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "Failed to fetch job",
		})
		return
	}
	defer rows.Close()

	job := GenericAgentJob{
		Name:   jobName,
		Valid:  true,
		Config: make(map[string]string),
	}

	found := false
	for rows.Next() {
		found = true
		var key, value string
		if err := rows.Scan(&key, &value); err == nil {
			job.Config[key] = value
			if key == "Valid" && value == "0" {
				job.Valid = false
			}
		}
	}
	_ = rows.Err()

	if !found {
		c.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"error":   "Job not found",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    job,
	})
}
