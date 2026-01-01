package contracts

import (
	"net/http"
)

// SearchContracts defines the API contracts for search endpoints.
var SearchContracts = []Contract{
	{
		Name:        "SearchAll",
		Description: "Search across all entity types",
		Method:      "POST",
		Path:        "/api/v1/search",
		Headers: map[string]string{
			"Authorization": "Bearer {{token}}",
			"Content-Type":  "application/json",
		},
		Body: map[string]interface{}{
			"query": "network issue",
			"types": []string{"ticket", "article", "customer"},
			"limit": 10,
		},
		Expected: Response{
			Status: http.StatusOK,
			BodySchema: ObjectSchema{Required: true, Properties: map[string]Schema{
				"query":      StringSchema{},
				"total_hits": NumberSchema{},
				"took_ms":    NumberSchema{},
				"hits":       ArraySchema{},
			}},
		},
	},
	{
		Name:        "SearchTickets",
		Description: "Search only tickets",
		Method:      "POST",
		Path:        "/api/v1/search",
		Headers: map[string]string{
			"Authorization": "Bearer {{token}}",
			"Content-Type":  "application/json",
		},
		Body: map[string]interface{}{
			"query": "email",
			"types": []string{"ticket"},
			"filters": map[string]string{
				"queue_id": "1",
				"state_id": "1",
			},
			"limit":  20,
			"offset": 0,
		},
		Expected: Response{
			Status: http.StatusOK,
			BodySchema: ObjectSchema{Required: true, Properties: map[string]Schema{
				"hits": ArraySchema{},
			}},
		},
	},
	{
		Name:        "SearchWithHighlighting",
		Description: "Search with result highlighting",
		Method:      "POST",
		Path:        "/api/v1/search",
		Headers: map[string]string{
			"Authorization": "Bearer {{token}}",
			"Content-Type":  "application/json",
		},
		Body: map[string]interface{}{
			"query":     "password reset",
			"types":     []string{"ticket", "article"},
			"highlight": true,
			"limit":     5,
		},
		Expected: Response{
			Status: http.StatusOK,
			BodySchema: ObjectSchema{Required: true, Properties: map[string]Schema{
				"hits": ArraySchema{},
			}},
		},
	},
	{
		Name:        "SearchWithPagination",
		Description: "Search with pagination",
		Method:      "POST",
		Path:        "/api/v1/search",
		Headers: map[string]string{
			"Authorization": "Bearer {{token}}",
			"Content-Type":  "application/json",
		},
		Body: map[string]interface{}{
			"query":  "customer",
			"types":  []string{"customer"},
			"offset": 10,
			"limit":  10,
		},
		Expected: Response{
			Status: http.StatusOK,
			BodySchema: ObjectSchema{Required: true, Properties: map[string]Schema{
				"hits": ArraySchema{},
			}},
		},
	},
	{
		Name:        "SearchSuggestions",
		Description: "Get search suggestions",
		Method:      "GET",
		Path:        "/api/v1/search/suggestions?q=net&type=ticket",
		Headers: map[string]string{
			"Authorization": "Bearer {{token}}",
		},
		Expected: Response{
			Status: http.StatusOK,
			BodySchema: ObjectSchema{Required: true, Properties: map[string]Schema{
				"query":       StringSchema{},
				"type":        StringSchema{},
				"suggestions": ArraySchema{},
			}},
		},
	},
	{
		Name:        "SearchHealth",
		Description: "Check search backend health",
		Method:      "GET",
		Path:        "/api/v1/search/health",
		Headers: map[string]string{
			"Authorization": "Bearer {{token}}",
		},
		Expected: Response{
			Status: http.StatusOK,
			BodySchema: ObjectSchema{Required: true, Properties: map[string]Schema{
				"status":  StringSchema{},
				"backend": StringSchema{},
			}},
		},
	},
	{
		Name:        "Reindex",
		Description: "Trigger search index rebuild",
		Method:      "POST",
		Path:        "/api/v1/search/reindex",
		Headers: map[string]string{
			"Authorization": "Bearer {{token}}",
			"Content-Type":  "application/json",
		},
		Body: map[string]interface{}{
			"types": []string{"ticket", "article"},
			"force": false,
		},
		Expected: Response{
			Status: http.StatusOK,
			BodySchema: ObjectSchema{Required: true, Properties: map[string]Schema{
				"message": StringSchema{},
				"backend": StringSchema{},
			}},
		},
	},
	{
		Name:        "EmptySearchQuery",
		Description: "Search with empty query should fail",
		Method:      "POST",
		Path:        "/api/v1/search",
		Headers: map[string]string{
			"Authorization": "Bearer {{token}}",
			"Content-Type":  "application/json",
		},
		Body: map[string]interface{}{
			"query": "",
			"types": []string{"ticket"},
		},
		Expected: Response{
			Status: http.StatusBadRequest,
			BodySchema: ObjectSchema{Required: true, Properties: map[string]Schema{
				"error": StringSchema{},
			}},
		},
	},
}

// RegisterSearchContracts registers search contracts for testing.
func RegisterSearchContracts() {
	// No-op registrar kept for backward compatibility
}
