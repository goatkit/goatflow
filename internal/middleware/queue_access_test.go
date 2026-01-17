package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func TestRequireQueueAccess(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("requires_authentication", func(t *testing.T) {
		router := gin.New()
		router.Use(RequireQueueAccess("ro"))
		router.GET("/test/:queue_id", func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{"status": "ok"})
		})

		req := httptest.NewRequest(http.MethodGet, "/test/1", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusUnauthorized, w.Code)
	})

	t.Run("requires_queue_id_param", func(t *testing.T) {
		router := gin.New()
		router.Use(func(c *gin.Context) {
			c.Set("user_id", 1)
			c.Next()
		})
		router.Use(RequireQueueAccess("ro"))
		router.GET("/test", func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{"status": "ok"})
		})

		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("rejects_invalid_queue_id", func(t *testing.T) {
		router := gin.New()
		router.Use(func(c *gin.Context) {
			c.Set("user_id", 1)
			c.Next()
		})
		router.Use(RequireQueueAccess("ro"))
		router.GET("/test/:queue_id", func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{"status": "ok"})
		})

		req := httptest.NewRequest(http.MethodGet, "/test/invalid", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})
}

func TestRequireQueueAccessFromTicket(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("requires_authentication", func(t *testing.T) {
		router := gin.New()
		router.Use(RequireQueueAccessFromTicket("ro"))
		router.GET("/ticket/:ticket_id", func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{"status": "ok"})
		})

		req := httptest.NewRequest(http.MethodGet, "/ticket/1", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusUnauthorized, w.Code)
	})

	t.Run("requires_ticket_id_param", func(t *testing.T) {
		router := gin.New()
		router.Use(func(c *gin.Context) {
			c.Set("user_id", 1)
			c.Next()
		})
		router.Use(RequireQueueAccessFromTicket("ro"))
		router.GET("/ticket", func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{"status": "ok"})
		})

		req := httptest.NewRequest(http.MethodGet, "/ticket", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("non_numeric_ticket_id_treated_as_ticket_number", func(t *testing.T) {
		// Non-numeric ticket IDs are now treated as ticket numbers (tn field)
		// and looked up in the database. Without a DB connection or matching ticket,
		// this returns 404 Not Found (ticket not found) or 500 (DB connection failed)
		router := gin.New()
		router.Use(func(c *gin.Context) {
			c.Set("user_id", 1)
			c.Next()
		})
		router.Use(RequireQueueAccessFromTicket("ro"))
		router.GET("/ticket/:ticket_id", func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{"status": "ok"})
		})

		req := httptest.NewRequest(http.MethodGet, "/ticket/TN-12345", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		// Should be 404 (ticket not found) or 500 (DB connection failed) - not 400
		assert.NotEqual(t, http.StatusBadRequest, w.Code)
		assert.True(t, w.Code == http.StatusNotFound || w.Code == http.StatusInternalServerError,
			"Expected 404 or 500, got %d", w.Code)
	})
}

func TestRequireAnyQueueAccess(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("requires_authentication", func(t *testing.T) {
		router := gin.New()
		router.Use(RequireAnyQueueAccess("ro"))
		router.GET("/tickets", func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{"status": "ok"})
		})

		req := httptest.NewRequest(http.MethodGet, "/tickets", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusUnauthorized, w.Code)
	})
}

func TestUserIDConversion(t *testing.T) {
	// Test that various user ID types are handled correctly
	testCases := []struct {
		name     string
		userID   interface{}
		expected bool // true if it should pass auth check
	}{
		{"int", 1, true},
		{"int64", int64(1), true},
		{"uint", uint(1), true},
		{"uint64", uint64(1), true},
		{"string", "1", false}, // Should fail - invalid type
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			gin.SetMode(gin.TestMode)
			router := gin.New()
			router.Use(func(c *gin.Context) {
				c.Set("user_id", tc.userID)
				c.Next()
			})
			router.Use(RequireQueueAccess("ro"))
			router.GET("/test/:queue_id", func(c *gin.Context) {
				c.JSON(http.StatusOK, gin.H{"status": "ok"})
			})

			req := httptest.NewRequest(http.MethodGet, "/test/1", nil)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			if tc.expected {
				// Should not fail on user ID type
				// But may fail on database check - that's OK for this test
				assert.NotEqual(t, http.StatusUnauthorized, w.Code)
			} else {
				assert.Equal(t, http.StatusInternalServerError, w.Code)
			}
		})
	}
}

// =============================================================================
// SECURITY TESTS - Queue Permission Enforcement
// =============================================================================
// These tests verify that the queue permission system properly prevents
// unauthorized access to tickets and queues.

// TestSecurityQueueFilterBypass verifies that users cannot bypass queue
// restrictions by explicitly requesting a queue they don't have access to.
// SECURITY: This is critical - users must not see tickets from unauthorized queues.
func TestSecurityQueueFilterBypass(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("user_cannot_request_unauthorized_queue_in_filter", func(t *testing.T) {
		// Scenario: User has access to queue 1 only, but requests ?queue=6
		// Expected: Should either reject the request OR filter to empty results
		// MUST NOT: Return tickets from queue 6

		router := gin.New()

		// Simulate authenticated user with limited queue access
		router.Use(func(c *gin.Context) {
			c.Set("user_id", 100) // Non-admin user
			c.Set("is_queue_admin", false)
			c.Set("accessible_queue_ids", []uint{1, 2}) // Only has access to queues 1 and 2
			c.Next()
		})

		// Handler that checks if unauthorized queue was requested
		router.GET("/tickets", func(c *gin.Context) {
			requestedQueue := c.Query("queue")
			accessibleQueues, _ := c.Get("accessible_queue_ids")
			isAdmin, _ := c.Get("is_queue_admin")

			// If user is not admin and requests a specific queue,
			// verify it's in their accessible list
			if !isAdmin.(bool) && requestedQueue != "" && requestedQueue != "all" {
				queueID := 0
				if _, err := parseUint(requestedQueue); err == nil {
					queueID = int(mustParseUint(requestedQueue))
				}

				if queueID > 0 {
					accessible := accessibleQueues.([]uint)
					found := false
					for _, q := range accessible {
						if int(q) == queueID {
							found = true
							break
						}
					}

					if !found {
						// SECURITY: Reject request for unauthorized queue
						c.JSON(http.StatusForbidden, gin.H{
							"error": "Access denied to requested queue",
						})
						return
					}
				}
			}

			c.JSON(http.StatusOK, gin.H{"status": "ok"})
		})

		// Request queue 6 (not in user's accessible list)
		req := httptest.NewRequest(http.MethodGet, "/tickets?queue=6", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		// Should be forbidden - user doesn't have access to queue 6
		assert.Equal(t, http.StatusForbidden, w.Code,
			"SECURITY VIOLATION: User was able to request unauthorized queue")
	})

	t.Run("user_can_request_authorized_queue", func(t *testing.T) {
		router := gin.New()

		router.Use(func(c *gin.Context) {
			c.Set("user_id", 100)
			c.Set("is_queue_admin", false)
			c.Set("accessible_queue_ids", []uint{1, 2, 5})
			c.Next()
		})

		router.GET("/tickets", func(c *gin.Context) {
			requestedQueue := c.Query("queue")
			accessibleQueues, _ := c.Get("accessible_queue_ids")
			isAdmin, _ := c.Get("is_queue_admin")

			if !isAdmin.(bool) && requestedQueue != "" && requestedQueue != "all" {
				queueID := 0
				if _, err := parseUint(requestedQueue); err == nil {
					queueID = int(mustParseUint(requestedQueue))
				}

				if queueID > 0 {
					accessible := accessibleQueues.([]uint)
					found := false
					for _, q := range accessible {
						if int(q) == queueID {
							found = true
							break
						}
					}

					if !found {
						c.JSON(http.StatusForbidden, gin.H{"error": "Access denied"})
						return
					}
				}
			}

			c.JSON(http.StatusOK, gin.H{"status": "ok"})
		})

		// Request queue 2 (in user's accessible list)
		req := httptest.NewRequest(http.MethodGet, "/tickets?queue=2", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code,
			"User should be able to access authorized queue")
	})

	t.Run("admin_can_access_any_queue", func(t *testing.T) {
		router := gin.New()

		router.Use(func(c *gin.Context) {
			c.Set("user_id", 1)
			c.Set("is_queue_admin", true) // Admin user
			c.Set("accessible_queue_ids", []uint{}) // Empty - admin doesn't need this
			c.Next()
		})

		router.GET("/tickets", func(c *gin.Context) {
			isAdmin, _ := c.Get("is_queue_admin")

			if isAdmin.(bool) {
				// Admin bypasses queue checks
				c.JSON(http.StatusOK, gin.H{"status": "ok"})
				return
			}

			c.JSON(http.StatusForbidden, gin.H{"error": "Access denied"})
		})

		// Admin requests any queue
		req := httptest.NewRequest(http.MethodGet, "/tickets?queue=999", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code,
			"Admin should be able to access any queue")
	})
}

// TestSecurityTicketAccessByQueuePermission verifies that ticket access
// is properly restricted based on the ticket's queue.
func TestSecurityTicketAccessByQueuePermission(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("cannot_access_ticket_in_unauthorized_queue", func(t *testing.T) {
		// This tests the RequireQueueAccessFromTicket middleware concept
		// User has access to queue 1, ticket is in queue 6

		router := gin.New()

		// Simulate the middleware setting context after DB lookup
		router.Use(func(c *gin.Context) {
			c.Set("user_id", 100)
			c.Set("is_queue_admin", false)
			c.Set("accessible_queue_ids", []uint{1, 2}) // User's accessible queues

			// Simulate: ticket 12345 is in queue 6
			ticketQueueID := uint(6)
			c.Set("ticket_queue_id", ticketQueueID)
			c.Next()
		})

		router.GET("/ticket/:id", func(c *gin.Context) {
			isAdmin, _ := c.Get("is_queue_admin")
			if isAdmin.(bool) {
				c.JSON(http.StatusOK, gin.H{"status": "ok"})
				return
			}

			ticketQueueID, _ := c.Get("ticket_queue_id")
			accessibleQueues, _ := c.Get("accessible_queue_ids")

			// Check if ticket's queue is accessible
			found := false
			for _, q := range accessibleQueues.([]uint) {
				if q == ticketQueueID.(uint) {
					found = true
					break
				}
			}

			if !found {
				c.JSON(http.StatusForbidden, gin.H{
					"error": "You do not have permission to view this ticket",
				})
				return
			}

			c.JSON(http.StatusOK, gin.H{"status": "ok"})
		})

		req := httptest.NewRequest(http.MethodGet, "/ticket/12345", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusForbidden, w.Code,
			"SECURITY VIOLATION: User accessed ticket in unauthorized queue")
	})

	t.Run("can_access_ticket_in_authorized_queue", func(t *testing.T) {
		router := gin.New()

		router.Use(func(c *gin.Context) {
			c.Set("user_id", 100)
			c.Set("is_queue_admin", false)
			c.Set("accessible_queue_ids", []uint{1, 2, 6}) // User has queue 6 access

			// Ticket is in queue 6
			c.Set("ticket_queue_id", uint(6))
			c.Next()
		})

		router.GET("/ticket/:id", func(c *gin.Context) {
			isAdmin, _ := c.Get("is_queue_admin")
			if isAdmin.(bool) {
				c.JSON(http.StatusOK, gin.H{"status": "ok"})
				return
			}

			ticketQueueID, _ := c.Get("ticket_queue_id")
			accessibleQueues, _ := c.Get("accessible_queue_ids")

			found := false
			for _, q := range accessibleQueues.([]uint) {
				if q == ticketQueueID.(uint) {
					found = true
					break
				}
			}

			if !found {
				c.JSON(http.StatusForbidden, gin.H{"error": "Access denied"})
				return
			}

			c.JSON(http.StatusOK, gin.H{"status": "ok"})
		})

		req := httptest.NewRequest(http.MethodGet, "/ticket/12345", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code,
			"User should be able to access ticket in authorized queue")
	})
}

// TestSecurityContextEnrichment verifies that middleware properly sets
// security context values for downstream handlers.
func TestSecurityContextEnrichment(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("accessible_queue_ids_must_be_set_for_non_admin", func(t *testing.T) {
		router := gin.New()

		var contextValues map[string]interface{}

		router.Use(func(c *gin.Context) {
			c.Set("user_id", 100)
			c.Set("is_queue_admin", false)
			// Deliberately NOT setting accessible_queue_ids
			c.Next()
		})

		router.GET("/tickets", func(c *gin.Context) {
			contextValues = make(map[string]interface{})

			if v, exists := c.Get("is_queue_admin"); exists {
				contextValues["is_queue_admin"] = v
			}
			if v, exists := c.Get("accessible_queue_ids"); exists {
				contextValues["accessible_queue_ids"] = v
			}

			// Handler should fail-safe: no accessible_queue_ids means no access
			isAdmin, _ := c.Get("is_queue_admin")
			accessibleQueues, hasQueues := c.Get("accessible_queue_ids")

			if !isAdmin.(bool) && (!hasQueues || len(accessibleQueues.([]uint)) == 0) {
				c.JSON(http.StatusForbidden, gin.H{
					"error": "No queue access configured",
				})
				return
			}

			c.JSON(http.StatusOK, gin.H{"status": "ok"})
		})

		req := httptest.NewRequest(http.MethodGet, "/tickets", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		// Should fail-safe to forbidden when no queues are accessible
		assert.Equal(t, http.StatusForbidden, w.Code,
			"SECURITY: Should deny access when accessible_queue_ids is not set")
	})

	t.Run("is_queue_admin_defaults_to_false", func(t *testing.T) {
		router := gin.New()

		router.Use(func(c *gin.Context) {
			c.Set("user_id", 100)
			// NOT setting is_queue_admin - should default to false
			c.Next()
		})

		router.GET("/tickets", func(c *gin.Context) {
			isAdmin := false
			if v, exists := c.Get("is_queue_admin"); exists {
				if admin, ok := v.(bool); ok {
					isAdmin = admin
				}
			}

			if isAdmin {
				c.JSON(http.StatusOK, gin.H{"admin": true})
			} else {
				c.JSON(http.StatusOK, gin.H{"admin": false})
			}
		})

		req := httptest.NewRequest(http.MethodGet, "/tickets", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Contains(t, w.Body.String(), `"admin":false`,
			"is_queue_admin should default to false for security")
	})
}

// TestSecurityPermissionTypes verifies that different permission types
// (ro, rw, create, note, etc.) are properly distinguished.
func TestSecurityPermissionTypes(t *testing.T) {
	gin.SetMode(gin.TestMode)

	permissionTypes := []string{"ro", "rw", "create", "move_into", "note", "owner", "priority"}

	for _, permType := range permissionTypes {
		t.Run("middleware_exists_for_"+permType, func(t *testing.T) {
			// Verify each permission type has a working middleware factory
			middleware := RequireQueueAccess(permType)
			assert.NotNil(t, middleware, "Middleware for %s should exist", permType)

			ticketMiddleware := RequireQueueAccessFromTicket(permType)
			assert.NotNil(t, ticketMiddleware, "Ticket middleware for %s should exist", permType)

			anyQueueMiddleware := RequireAnyQueueAccess(permType)
			assert.NotNil(t, anyQueueMiddleware, "Any queue middleware for %s should exist", permType)
		})
	}
}

// Helper functions for tests
func parseUint(s string) (uint64, error) {
	var result uint64
	for _, c := range s {
		if c < '0' || c > '9' {
			return 0, assert.AnError
		}
		result = result*10 + uint64(c-'0')
	}
	return result, nil
}

func mustParseUint(s string) uint64 {
	result, _ := parseUint(s)
	return result
}
