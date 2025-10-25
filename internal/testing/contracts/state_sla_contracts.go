package contracts

import (
	"net/http"
)

// TicketStateContracts defines the API contracts for ticket state endpoints
var TicketStateContracts = []Contract{
	{
		Name:        "ListTicketStates",
		Description: "List all ticket states",
		Method:      "GET",
		Path:        "/api/v1/ticket-states",
		Headers: map[string]string{
			"Authorization": "Bearer {{token}}",
		},
		Expected: Response{
			Status: http.StatusOK,
			BodySchema: ObjectSchema{Required: true, Properties: map[string]Schema{
				"states": ArraySchema{ItemsSchema: ObjectSchema{Properties: map[string]Schema{
					"id":       NumberSchema{Required: true},
					"name":     StringSchema{Required: true},
					"type_id":  NumberSchema{},
					"valid_id": NumberSchema{},
				}}, Required: true},
				"total": NumberSchema{},
			}},
		},
	},
	{
		Name:        "GetTicketState",
		Description: "Get single ticket state by ID",
		Method:      "GET",
		Path:        "/api/v1/ticket-states/1",
		Headers: map[string]string{
			"Authorization": "Bearer {{token}}",
		},
		Expected: Response{
			Status: http.StatusOK,
			BodySchema: ObjectSchema{Required: true, Properties: map[string]Schema{
				"id":       NumberSchema{Required: true},
				"name":     StringSchema{Required: true},
				"type_id":  NumberSchema{},
				"valid_id": NumberSchema{},
			}},
		},
	},
	{
		Name:        "CreateTicketState",
		Description: "Create new ticket state",
		Method:      "POST",
		Path:        "/api/v1/ticket-states",
		Headers: map[string]string{
			"Authorization": "Bearer {{token}}",
			"Content-Type":  "application/json",
		},
		Body: map[string]interface{}{
			"name":    "Test State",
			"type_id": 1, // open type
		},
		Expected: Response{
			Status: http.StatusCreated,
			BodySchema: ObjectSchema{Required: true, Properties: map[string]Schema{
				"id":       NumberSchema{Required: true},
				"name":     StringSchema{Required: true},
				"type_id":  NumberSchema{},
				"valid_id": NumberSchema{},
			}},
		},
	},
	{
		Name:        "TicketStateStatistics",
		Description: "Get ticket state statistics",
		Method:      "GET",
		Path:        "/api/v1/ticket-states/statistics",
		Headers: map[string]string{
			"Authorization": "Bearer {{token}}",
		},
		Expected: Response{
			Status: http.StatusOK,
			BodySchema: ObjectSchema{Required: true, Properties: map[string]Schema{
				"statistics": ArraySchema{ItemsSchema: ObjectSchema{Properties: map[string]Schema{
					"state_id":     NumberSchema{Required: true},
					"state_name":   StringSchema{Required: true},
					"type_id":      NumberSchema{},
					"ticket_count": NumberSchema{},
				}}, Required: true},
				"total_tickets": NumberSchema{},
			}},
		},
	},
}

// SLAContracts defines the API contracts for SLA endpoints
var SLAContracts = []Contract{
	{
		Name:        "ListSLAs",
		Description: "List all SLAs",
		Method:      "GET",
		Path:        "/api/v1/slas",
		Headers: map[string]string{
			"Authorization": "Bearer {{token}}",
		},
		Expected: Response{
			Status: http.StatusOK,
			BodySchema: ObjectSchema{Required: true, Properties: map[string]Schema{
				"slas": ArraySchema{ItemsSchema: ObjectSchema{Properties: map[string]Schema{
					"id":                  NumberSchema{Required: true},
					"name":                StringSchema{Required: true},
					"calendar_name":       StringSchema{},
					"first_response_time": NumberSchema{},
					"solution_time":       NumberSchema{},
					"valid_id":            NumberSchema{},
				}}, Required: true},
				"total": NumberSchema{},
			}},
		},
	},
	{
		Name:        "GetSLA",
		Description: "Get single SLA by ID",
		Method:      "GET",
		Path:        "/api/v1/slas/1",
		Headers: map[string]string{
			"Authorization": "Bearer {{token}}",
		},
		Expected: Response{
			Status: http.StatusOK,
			BodySchema: ObjectSchema{Required: true, Properties: map[string]Schema{
				"id":                  NumberSchema{Required: true},
				"name":                StringSchema{Required: true},
				"calendar_name":       StringSchema{},
				"first_response_time": NumberSchema{},
				"solution_time":       NumberSchema{},
				"valid_id":            NumberSchema{},
			}},
		},
	},
	{
		Name:        "CreateSLA",
		Description: "Create new SLA",
		Method:      "POST",
		Path:        "/api/v1/slas",
		Headers: map[string]string{
			"Authorization": "Bearer {{token}}",
			"Content-Type":  "application/json",
		},
		Body: map[string]interface{}{
			"name":                  "Test SLA",
			"calendar_name":         "Default",
			"first_response_time":   60,
			"first_response_notify": 50,
			"update_time":           120,
			"update_notify":         100,
			"solution_time":         480,
			"solution_notify":       400,
		},
		Expected: Response{
			Status: http.StatusCreated,
			BodySchema: ObjectSchema{Required: true, Properties: map[string]Schema{
				"id":                  NumberSchema{Required: true},
				"name":                StringSchema{Required: true},
				"calendar_name":       StringSchema{},
				"first_response_time": NumberSchema{},
				"solution_time":       NumberSchema{},
				"valid_id":            NumberSchema{},
			}},
		},
	},
	{
		Name:        "UpdateSLA",
		Description: "Update existing SLA",
		Method:      "PUT",
		Path:        "/api/v1/slas/1",
		Headers: map[string]string{
			"Authorization": "Bearer {{token}}",
			"Content-Type":  "application/json",
		},
		Body: map[string]interface{}{
			"name":                "Updated SLA",
			"first_response_time": 45,
			"solution_time":       360,
		},
		Expected: Response{
			Status: http.StatusOK,
			BodySchema: ObjectSchema{Required: true, Properties: map[string]Schema{
				"id":                  NumberSchema{Required: true},
				"name":                StringSchema{Required: true},
				"calendar_name":       StringSchema{},
				"first_response_time": NumberSchema{},
				"solution_time":       NumberSchema{},
			}},
		},
	},
	{
		Name:        "DeleteSLA",
		Description: "Soft delete SLA",
		Method:      "DELETE",
		Path:        "/api/v1/slas/10",
		Headers: map[string]string{
			"Authorization": "Bearer {{token}}",
		},
		Expected: Response{
			Status: http.StatusOK,
			BodySchema: ObjectSchema{Required: true, Properties: map[string]Schema{
				"message": StringSchema{},
				"id":      NumberSchema{},
			}},
		},
	},
	{
		Name:        "SLAMetrics",
		Description: "Get SLA performance metrics",
		Method:      "GET",
		Path:        "/api/v1/slas/1/metrics",
		Headers: map[string]string{
			"Authorization": "Bearer {{token}}",
		},
		Expected: Response{
			Status: http.StatusOK,
			BodySchema: ObjectSchema{Required: true, Properties: map[string]Schema{
				"sla_id":   NumberSchema{Required: true},
				"sla_name": StringSchema{},
				"metrics":  ObjectSchema{},
			}},
		},
	},
}

// RegisterStateSLAContracts registers ticket state and SLA contracts for testing
func RegisterStateSLAContracts() {
	// No-op registrar kept for backward compatibility
}
