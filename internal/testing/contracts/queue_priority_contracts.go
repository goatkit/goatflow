package contracts

import (
	"net/http"
)

// QueueContracts defines the API contracts for queue endpoints
var QueueContracts = []Contract{
	{
		Name:        "ListQueues",
		Description: "List all queues",
		Method:      "GET",
		Path:        "/api/v1/queues",
		Headers: map[string]string{
			"Authorization": "Bearer {{token}}",
		},
        Expected: Response{
            Status: http.StatusOK,
            BodySchema: ObjectSchema{Required: true, Properties: map[string]Schema{
                "queues": ArraySchema{ItemsSchema: ObjectSchema{Properties: map[string]Schema{
                    "id":       NumberSchema{Required: true},
                    "name":     StringSchema{Required: true},
                    "description": StringSchema{},
                    "valid_id": NumberSchema{},
                }}, Required: true},
                "total": NumberSchema{},
            }},
        },
	},
	{
		Name:        "GetQueue",
		Description: "Get single queue by ID",
		Method:      "GET",
		Path:        "/api/v1/queues/1",
		Headers: map[string]string{
			"Authorization": "Bearer {{token}}",
		},
        Expected: Response{
            Status: http.StatusOK,
            BodySchema: ObjectSchema{Required: true, Properties: map[string]Schema{
                "id":       NumberSchema{Required: true},
                "name":     StringSchema{Required: true},
                "description": StringSchema{},
                "valid_id": NumberSchema{},
            }},
        },
	},
	{
		Name:        "CreateQueue",
		Description: "Create new queue",
		Method:      "POST",
		Path:        "/api/v1/queues",
		Headers: map[string]string{
			"Authorization": "Bearer {{token}}",
			"Content-Type":  "application/json",
		},
		Body: map[string]interface{}{
			"name":        "Test Queue",
			"description": "Test queue description",
			"group_access": []int{1, 2},
		},
        Expected: Response{
            Status: http.StatusCreated,
            BodySchema: ObjectSchema{Required: true, Properties: map[string]Schema{
                "id":       NumberSchema{Required: true},
                "name":     StringSchema{Required: true},
                "description": StringSchema{},
                "valid_id": NumberSchema{},
            }},
        },
	},
	{
		Name:        "UpdateQueue",
		Description: "Update existing queue",
		Method:      "PUT",
		Path:        "/api/v1/queues/1",
		Headers: map[string]string{
			"Authorization": "Bearer {{token}}",
			"Content-Type":  "application/json",
		},
		Body: map[string]interface{}{
			"name":        "Updated Queue",
			"description": "Updated description",
		},
        Expected: Response{
            Status: http.StatusOK,
            BodySchema: ObjectSchema{Required: true, Properties: map[string]Schema{
                "id":       NumberSchema{Required: true},
                "name":     StringSchema{Required: true},
                "description": StringSchema{},
                "valid_id": NumberSchema{},
            }},
        },
	},
	{
		Name:        "DeleteQueue",
		Description: "Soft delete queue",
		Method:      "DELETE",
		Path:        "/api/v1/queues/10",
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
}

// PriorityContracts defines the API contracts for priority endpoints
var PriorityContracts = []Contract{
	{
		Name:        "ListPriorities",
		Description: "List all priorities",
		Method:      "GET",
		Path:        "/api/v1/priorities",
		Headers: map[string]string{
			"Authorization": "Bearer {{token}}",
		},
        Expected: Response{
            Status: http.StatusOK,
            BodySchema: ObjectSchema{Required: true, Properties: map[string]Schema{
                "priorities": ArraySchema{ItemsSchema: ObjectSchema{Properties: map[string]Schema{
                    "id":      NumberSchema{Required: true},
                    "name":    StringSchema{Required: true},
                    "valid_id": NumberSchema{},
                }}, Required: true},
                "total": NumberSchema{},
            }},
        },
	},
	{
		Name:        "GetPriority",
		Description: "Get single priority by ID",
		Method:      "GET",
		Path:        "/api/v1/priorities/1",
		Headers: map[string]string{
			"Authorization": "Bearer {{token}}",
		},
        Expected: Response{
            Status: http.StatusOK,
            BodySchema: ObjectSchema{Required: true, Properties: map[string]Schema{
                "id":      NumberSchema{Required: true},
                "name":    StringSchema{Required: true},
                "valid_id": NumberSchema{},
            }},
        },
	},
	{
		Name:        "CreatePriority",
		Description: "Create new priority",
		Method:      "POST",
		Path:        "/api/v1/priorities",
		Headers: map[string]string{
			"Authorization": "Bearer {{token}}",
			"Content-Type":  "application/json",
		},
		Body: map[string]interface{}{
			"name": "Test Priority",
		},
        Expected: Response{
            Status: http.StatusCreated,
            BodySchema: ObjectSchema{Required: true, Properties: map[string]Schema{
                "id":      NumberSchema{Required: true},
                "name":    StringSchema{Required: true},
                "valid_id": NumberSchema{},
            }},
        },
	},
	{
		Name:        "UpdatePriority",
		Description: "Update existing priority",
		Method:      "PUT",
		Path:        "/api/v1/priorities/6",
		Headers: map[string]string{
			"Authorization": "Bearer {{token}}",
			"Content-Type":  "application/json",
		},
		Body: map[string]interface{}{
			"name": "Updated Priority",
		},
        Expected: Response{
            Status: http.StatusOK,
            BodySchema: ObjectSchema{Required: true, Properties: map[string]Schema{
                "id":      NumberSchema{Required: true},
                "name":    StringSchema{Required: true},
                "valid_id": NumberSchema{},
            }},
        },
	},
	{
		Name:        "DeletePriority",
		Description: "Soft delete priority",
		Method:      "DELETE",
		Path:        "/api/v1/priorities/10",
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
}

// RegisterQueuePriorityContracts registers queue and priority contracts for testing
func RegisterQueuePriorityContracts() {
    // No-op registrar kept for backward compatibility
}