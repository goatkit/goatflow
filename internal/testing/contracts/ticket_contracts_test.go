package contracts

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/gin-gonic/gin"
)

// TestTicketContracts tests all ticket endpoint contracts.
func TestTicketContracts(t *testing.T) {
	// Set up router
	gin.SetMode(gin.TestMode)
	r := gin.New()

	// Use mock handlers for testing
	mocks := &MockHandlers{}

	// Register API v1 routes with mock handlers
	v1 := r.Group("/api/v1")
	{
		// Ticket endpoints
		v1.GET("/tickets", mocks.HandleListTickets)
		v1.GET("/tickets/:id", mocks.HandleGetTicket)
		v1.POST("/tickets", mocks.HandleCreateTicket)
		v1.PUT("/tickets/:id", mocks.HandleUpdateTicket)
		v1.DELETE("/tickets/:id", mocks.HandleDeleteTicket)
	}

	// Create contract tester
	ct := NewContractTest(t, r)

	// Get a valid JWT token for authenticated requests
	validToken := "Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.test"

	// Contract: List Tickets
	ct.AddContract(Contract{
		Name:        "GET /api/v1/tickets - List Tickets",
		Description: "Should return paginated list of tickets",
		Method:      "GET",
		Path:        "/api/v1/tickets",
		Headers: map[string]string{
			"Authorization": validToken,
		},
		Expected: Response{
			Status: 200,
			BodySchema: ObjectSchema{
				Required: true,
				Properties: map[string]Schema{
					"success": BooleanSchema{Required: true},
					"data": ObjectSchema{
						Properties: map[string]Schema{
							"tickets": ArraySchema{
								ItemsSchema: ObjectSchema{
									Properties: map[string]Schema{
										"id":            NumberSchema{},
										"ticket_number": StringSchema{},
										"title":         StringSchema{},
										"state_id":      NumberSchema{},
										"priority_id":   NumberSchema{},
										"queue_id":      NumberSchema{},
									},
								},
							},
							"total":    NumberSchema{},
							"page":     NumberSchema{},
							"per_page": NumberSchema{},
						},
					},
				},
			},
		},
	})

	// Contract: Get Single Ticket
	ct.AddContract(Contract{
		Name:        "GET /api/v1/tickets/:id - Get Ticket",
		Description: "Should return ticket details",
		Method:      "GET",
		Path:        "/api/v1/tickets/1",
		Headers: map[string]string{
			"Authorization": validToken,
		},
		Expected: Response{
			Status: 200,
			BodySchema: ObjectSchema{
				Required: true,
				Properties: map[string]Schema{
					"success": BooleanSchema{Required: true},
					"data": ObjectSchema{
						Properties: map[string]Schema{
							"id":               NumberSchema{Required: true},
							"ticket_number":    StringSchema{Required: true},
							"title":            StringSchema{Required: true},
							"state_id":         NumberSchema{},
							"priority_id":      NumberSchema{},
							"queue_id":         NumberSchema{},
							"customer_id":      StringSchema{},
							"customer_user_id": StringSchema{},
						},
					},
				},
			},
		},
	})

	// Contract: Create Ticket
	ct.AddContract(Contract{
		Name:        "POST /api/v1/tickets - Create Ticket",
		Description: "Should create new ticket",
		Method:      "POST",
		Path:        "/api/v1/tickets",
		Headers: map[string]string{
			"Authorization": validToken,
			"Content-Type":  "application/json",
		},
		Body: map[string]interface{}{
			"title":            "Test Ticket",
			"queue_id":         1,
			"priority_id":      3,
			"state_id":         1,
			"customer_user_id": "customer@example.com",
			"article": map[string]interface{}{
				"subject": "Initial message",
				"body":    "This is the ticket description",
				"type":    "note",
			},
		},
		Expected: Response{
			Status: 201,
			BodySchema: ObjectSchema{
				Required: true,
				Properties: map[string]Schema{
					"success": BooleanSchema{Required: true},
					"data": ObjectSchema{
						Properties: map[string]Schema{
							"id":            NumberSchema{Required: true},
							"ticket_number": StringSchema{Required: true},
						},
					},
				},
			},
			Validations: []Validation{
				IsSuccessResponse(),
			},
		},
	})

	// Contract: Update Ticket
	ct.AddContract(Contract{
		Name:        "PUT /api/v1/tickets/:id - Update Ticket",
		Description: "Should update ticket fields",
		Method:      "PUT",
		Path:        "/api/v1/tickets/1",
		Headers: map[string]string{
			"Authorization": validToken,
			"Content-Type":  "application/json",
		},
		Body: map[string]interface{}{
			"title":       "Updated Title",
			"priority_id": 1,
		},
		Expected: Response{
			Status: 200,
			Validations: []Validation{
				IsSuccessResponse(),
			},
		},
	})

	// Contract: Delete (Archive) Ticket
	ct.AddContract(Contract{
		Name:        "DELETE /api/v1/tickets/:id - Archive Ticket",
		Description: "Should archive ticket (soft delete)",
		Method:      "DELETE",
		Path:        "/api/v1/tickets/1",
		Headers: map[string]string{
			"Authorization": validToken,
		},
		Expected: Response{
			Status: 204,
		},
	})

	// Contract: Invalid Ticket ID
	ct.AddContract(Contract{
		Name:        "GET /api/v1/tickets/:id - Invalid ID",
		Description: "Non-existent ticket should return 404",
		Method:      "GET",
		Path:        "/api/v1/tickets/99999",
		Headers: map[string]string{
			"Authorization": validToken,
		},
		Expected: Response{
			Status: 404,
			Validations: []Validation{
				IsErrorResponse(),
			},
		},
	})

	// Contract: Unauthorized Access
	ct.AddContract(Contract{
		Name:        "GET /api/v1/tickets - No Auth",
		Description: "Missing auth token should return 401",
		Method:      "GET",
		Path:        "/api/v1/tickets",
		Expected: Response{
			Status: 401,
			Validations: []Validation{
				IsErrorResponse(),
			},
		},
	})

	// Contract: Invalid Request Body
	ct.AddContract(Contract{
		Name:        "POST /api/v1/tickets - Invalid Body",
		Description: "Missing required fields should return 400",
		Method:      "POST",
		Path:        "/api/v1/tickets",
		Headers: map[string]string{
			"Authorization": validToken,
			"Content-Type":  "application/json",
		},
		Body: map[string]interface{}{
			"title": "Missing Queue ID",
			// Missing required queue_id
		},
		Expected: Response{
			Status: 400,
			Validations: []Validation{
				IsErrorResponse(),
			},
		},
	})

	// Run all contracts
	ct.Run()
}

// TestTicketActionContracts tests ticket action endpoint contracts.
func TestTicketActionContracts(t *testing.T) {
	// Set up router
	gin.SetMode(gin.TestMode)
	r := gin.New()

	// Use mock handlers for testing
	mocks := &MockHandlers{}

	// Register API v1 routes with mock handlers
	v1 := r.Group("/api/v1")
	{
		// Ticket action endpoints
		v1.POST("/tickets/:id/close", mocks.HandleCloseTicket)
		v1.POST("/tickets/:id/reopen", mocks.HandleReopenTicket)
		v1.POST("/tickets/:id/assign", mocks.HandleAssignTicket)
	}

	// Create contract tester
	ct := NewContractTest(t, r)

	validToken := "Bearer valid_token"

	// Contract: Close Ticket
	ct.AddContract(Contract{
		Name:        "POST /api/v1/tickets/:id/close - Close Ticket",
		Description: "Should close ticket with resolution",
		Method:      "POST",
		Path:        "/api/v1/tickets/1/close",
		Headers: map[string]string{
			"Authorization": validToken,
			"Content-Type":  "application/json",
		},
		Body: map[string]interface{}{
			"resolution": "resolved",
			"comment":    "Issue has been fixed",
		},
		Expected: Response{
			Status: 200,
			BodySchema: ObjectSchema{
				Required: true,
				Properties: map[string]Schema{
					"success":    BooleanSchema{Required: true},
					"id":         NumberSchema{Required: true},
					"state_id":   NumberSchema{Required: true},
					"state":      StringSchema{Required: true},
					"resolution": StringSchema{Required: true},
					"closed_at":  StringSchema{},
				},
			},
			Validations: []Validation{
				IsSuccessResponse(),
				func(body []byte) error {
					// Validate state_id is 2 or 3 (closed states)
					var data map[string]interface{}
					if err := json.Unmarshal(body, &data); err != nil {
						return err
					}
					stateID, ok := data["state_id"].(float64)
					if !ok || (stateID != 2 && stateID != 3) {
						return fmt.Errorf("expected closed state (2 or 3), got %v", stateID)
					}
					return nil
				},
			},
		},
	})

	// Contract: Reopen Ticket
	ct.AddContract(Contract{
		Name:        "POST /api/v1/tickets/:id/reopen - Reopen Ticket",
		Description: "Should reopen closed ticket",
		Method:      "POST",
		Path:        "/api/v1/tickets/1/reopen",
		Headers: map[string]string{
			"Authorization": validToken,
			"Content-Type":  "application/json",
		},
		Body: map[string]interface{}{
			"reason": "Customer reported issue persists",
		},
		Expected: Response{
			Status: 200,
			BodySchema: ObjectSchema{
				Required: true,
				Properties: map[string]Schema{
					"success":     BooleanSchema{Required: true},
					"id":          NumberSchema{Required: true},
					"state_id":    NumberSchema{Required: true},
					"state":       StringSchema{Required: true},
					"reason":      StringSchema{Required: true},
					"reopened_at": StringSchema{},
				},
			},
			Validations: []Validation{
				IsSuccessResponse(),
				func(body []byte) error {
					// Validate state_id is 4 (open)
					var data map[string]interface{}
					if err := json.Unmarshal(body, &data); err != nil {
						return err
					}
					stateID, ok := data["state_id"].(float64)
					if !ok || stateID != 4 {
						return fmt.Errorf("expected open state (4), got %v", stateID)
					}
					return nil
				},
			},
		},
	})

	// Contract: Assign Ticket
	ct.AddContract(Contract{
		Name:        "POST /api/v1/tickets/:id/assign - Assign Ticket",
		Description: "Should assign ticket to user",
		Method:      "POST",
		Path:        "/api/v1/tickets/1/assign",
		Headers: map[string]string{
			"Authorization": validToken,
			"Content-Type":  "application/json",
		},
		Body: map[string]interface{}{
			"assigned_to": 2,
			"comment":     "Assigning to specialist",
		},
		Expected: Response{
			Status: 200,
			BodySchema: ObjectSchema{
				Required: true,
				Properties: map[string]Schema{
					"success":     BooleanSchema{Required: true},
					"id":          NumberSchema{Required: true},
					"assigned_to": NumberSchema{Required: true},
					"assignee":    StringSchema{},
					"assigned_at": StringSchema{},
				},
			},
			Validations: []Validation{
				IsSuccessResponse(),
			},
		},
	})

	// Contract: Close Already Closed Ticket
	ct.AddContract(Contract{
		Name:        "POST /api/v1/tickets/:id/close - Already Closed",
		Description: "Closing closed ticket should return error",
		Method:      "POST",
		Path:        "/api/v1/tickets/2/close", // Assume ticket 2 is closed
		Headers: map[string]string{
			"Authorization": validToken,
			"Content-Type":  "application/json",
		},
		Body: map[string]interface{}{
			"resolution": "resolved",
		},
		Expected: Response{
			Status: 400,
			Validations: []Validation{
				IsErrorResponse(),
			},
		},
	})

	// Contract: Reopen Open Ticket
	ct.AddContract(Contract{
		Name:        "POST /api/v1/tickets/:id/reopen - Already Open",
		Description: "Reopening open ticket should return error",
		Method:      "POST",
		Path:        "/api/v1/tickets/3/reopen", // Assume ticket 3 is open
		Headers: map[string]string{
			"Authorization": validToken,
			"Content-Type":  "application/json",
		},
		Body: map[string]interface{}{
			"reason": "Test",
		},
		Expected: Response{
			Status: 400,
			Validations: []Validation{
				IsErrorResponse(),
			},
		},
	})

	// Contract: Assign to Invalid User
	ct.AddContract(Contract{
		Name:        "POST /api/v1/tickets/:id/assign - Invalid User",
		Description: "Assigning to non-existent user should return error",
		Method:      "POST",
		Path:        "/api/v1/tickets/1/assign",
		Headers: map[string]string{
			"Authorization": validToken,
			"Content-Type":  "application/json",
		},
		Body: map[string]interface{}{
			"assigned_to": 99999,
		},
		Expected: Response{
			Status: 400,
			Validations: []Validation{
				IsErrorResponse(),
			},
		},
	})

	// Run all contracts
	ct.Run()
}
