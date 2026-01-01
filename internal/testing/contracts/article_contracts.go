// Package contracts provides API contract definitions for integration testing.
package contracts

import (
	"net/http"
)

// ArticleContracts defines the API contracts for article endpoints.
var ArticleContracts = []Contract{
	{
		Name:        "ListArticles",
		Description: "List all articles for a ticket",
		Method:      "GET",
		Path:        "/api/v1/tickets/1/articles",
		Headers: map[string]string{
			"Authorization": "Bearer {{token}}",
		},
		Expected: Response{
			Status: http.StatusOK,
			BodySchema: ObjectSchema{
				Required: true,
				Properties: map[string]Schema{
					"articles": ArraySchema{ItemsSchema: ObjectSchema{Properties: map[string]Schema{
						"id":        NumberSchema{Required: true},
						"ticket_id": NumberSchema{Required: true},
						"subject":   StringSchema{},
						"body":      StringSchema{},
					}}},
					"total": NumberSchema{},
				},
			},
		},
	},
	{
		Name:        "GetArticle",
		Description: "Get single article by ID",
		Method:      "GET",
		Path:        "/api/v1/tickets/1/articles/1",
		Headers: map[string]string{
			"Authorization": "Bearer {{token}}",
		},
		Expected: Response{
			Status: http.StatusOK,
			BodySchema: ObjectSchema{Required: true, Properties: map[string]Schema{
				"id":        NumberSchema{Required: true},
				"ticket_id": NumberSchema{Required: true},
				"subject":   StringSchema{},
				"body":      StringSchema{},
			}},
		},
	},
	{
		Name:        "GetArticleWithAttachments",
		Description: "Get article with attachment information",
		Method:      "GET",
		Path:        "/api/v1/tickets/1/articles/1?include_attachments=true",
		Headers: map[string]string{
			"Authorization": "Bearer {{token}}",
		},
		Expected: Response{
			Status: http.StatusOK,
			BodySchema: ObjectSchema{Required: true, Properties: map[string]Schema{
				"id":        NumberSchema{Required: true},
				"ticket_id": NumberSchema{Required: true},
				"subject":   StringSchema{},
				"body":      StringSchema{},
				"attachments": ArraySchema{ItemsSchema: ObjectSchema{Properties: map[string]Schema{
					"id":           NumberSchema{Required: true},
					"filename":     StringSchema{Required: true},
					"content_type": StringSchema{},
					"size":         NumberSchema{},
				}}}},
			},
		},
	},
	{
		Name:        "CreateArticle",
		Description: "Create new article for ticket",
		Method:      "POST",
		Path:        "/api/v1/tickets/1/articles",
		Headers: map[string]string{
			"Authorization": "Bearer {{token}}",
			"Content-Type":  "application/json",
		},
		Body: map[string]interface{}{
			"subject":      "Test Article",
			"body":         "This is a test article body",
			"from_email":   "agent@example.com",
			"to_email":     "customer@example.com",
			"article_type": "email-external",
			"sender_type":  "agent",
			"is_visible":   true,
		},
		Expected: Response{
			Status: http.StatusCreated,
			BodySchema: ObjectSchema{Required: true, Properties: map[string]Schema{
				"id":        NumberSchema{Required: true},
				"ticket_id": NumberSchema{Required: true},
				"subject":   StringSchema{},
				"body":      StringSchema{},
			}},
		},
	},
	{
		Name:        "UpdateArticle",
		Description: "Update existing article",
		Method:      "PUT",
		Path:        "/api/v1/tickets/1/articles/1",
		Headers: map[string]string{
			"Authorization": "Bearer {{token}}",
			"Content-Type":  "application/json",
		},
		Body: map[string]interface{}{
			"subject": "Updated Article Subject",
			"body":    "Updated article body content",
		},
		Expected: Response{
			Status: http.StatusOK,
			BodySchema: ObjectSchema{Required: true, Properties: map[string]Schema{
				"id":        NumberSchema{Required: true},
				"ticket_id": NumberSchema{Required: true},
				"subject":   StringSchema{},
				"body":      StringSchema{},
			}},
		},
	},
	{
		Name:        "DeleteArticle",
		Description: "Delete article from ticket",
		Method:      "DELETE",
		Path:        "/api/v1/tickets/1/articles/2",
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

// RegisterArticleContracts registers article contracts for testing.
func RegisterArticleContracts() {
	// No-op registrar kept for backward compatibility
}
