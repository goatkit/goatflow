package yamlmgmt

import (
	"encoding/json"
	"fmt"
	"sync"

	"github.com/xeipuuv/gojsonschema"
)

// SchemaRegistry manages JSON schemas for YAML validation.
type SchemaRegistry struct {
	mu      sync.RWMutex
	schemas map[YAMLKind]*Schema
	loaders map[YAMLKind]gojsonschema.JSONLoader
}

// NewSchemaRegistry creates a new schema registry.
func NewSchemaRegistry() *SchemaRegistry {
	sr := &SchemaRegistry{
		schemas: make(map[YAMLKind]*Schema),
		loaders: make(map[YAMLKind]gojsonschema.JSONLoader),
	}

	// Register default schemas
	sr.registerDefaultSchemas()

	return sr
}

// RegisterSchema registers a schema for a specific kind.
func (sr *SchemaRegistry) RegisterSchema(kind YAMLKind, schema *Schema) error {
	sr.mu.Lock()
	defer sr.mu.Unlock()

	// Convert schema to JSON
	schemaJSON, err := json.Marshal(schema)
	if err != nil {
		return fmt.Errorf("failed to marshal schema: %w", err)
	}

	// Create loader
	loader := gojsonschema.NewBytesLoader(schemaJSON)

	sr.schemas[kind] = schema
	sr.loaders[kind] = loader

	return nil
}

// GetSchema returns the schema for a specific kind.
func (sr *SchemaRegistry) GetSchema(kind YAMLKind) (*Schema, error) {
	sr.mu.RLock()
	defer sr.mu.RUnlock()

	schema, exists := sr.schemas[kind]
	if !exists {
		return nil, fmt.Errorf("no schema registered for kind: %s", kind)
	}

	return schema, nil
}

// Validate validates a document against its schema.
func (sr *SchemaRegistry) Validate(doc *YAMLDocument) (*ValidationResult, error) {
	sr.mu.RLock()
	defer sr.mu.RUnlock()

	kind := YAMLKind(doc.Kind)
	loader, exists := sr.loaders[kind]
	if !exists {
		// No schema registered, consider valid
		return &ValidationResult{Valid: true}, nil
	}

	// Convert document to JSON for validation
	docJSON, err := json.Marshal(doc)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal document: %w", err)
	}

	docLoader := gojsonschema.NewBytesLoader(docJSON)

	// Validate
	result, err := gojsonschema.Validate(loader, docLoader)
	if err != nil {
		return nil, fmt.Errorf("validation error: %w", err)
	}

	// Convert to our result format
	validationResult := &ValidationResult{
		Valid:    result.Valid(),
		Errors:   []ValidationError{},
		Warnings: []ValidationWarning{},
	}

	for _, err := range result.Errors() {
		validationResult.Errors = append(validationResult.Errors, ValidationError{
			Path:    err.Field(),
			Message: err.Description(),
			Code:    err.Type(),
		})
	}

	return validationResult, nil
}

// registerDefaultSchemas registers built-in schemas.
func (sr *SchemaRegistry) registerDefaultSchemas() {
	// Route schema
	sr.RegisterSchema(KindRoute, &Schema{
		Schema: "http://json-schema.org/draft-07/schema#",
		Title:  "Route Configuration",
		Type:   "object",
		Properties: map[string]interface{}{
			"apiVersion": map[string]interface{}{
				"type":    "string",
				"pattern": "^(routes|goatflow\\.io)/v[0-9]+",
			},
			"kind": map[string]interface{}{
				"type": "string",
				"enum": []string{"Route", "RouteGroup"},
			},
			"metadata": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"name": map[string]interface{}{
						"type":      "string",
						"minLength": 1,
					},
					"namespace": map[string]interface{}{
						"type": "string",
					},
					"description": map[string]interface{}{
						"type": "string",
					},
					"enabled": map[string]interface{}{
						"type": "boolean",
					},
					"labels": map[string]interface{}{
						"type": "object",
						"additionalProperties": map[string]interface{}{
							"type": "string",
						},
					},
				},
				"required": []string{"name"},
			},
			"spec": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"prefix": map[string]interface{}{
						"type": "string",
					},
					"routes": map[string]interface{}{
						"type": "array",
						"items": map[string]interface{}{
							"type": "object",
							"properties": map[string]interface{}{
								"path": map[string]interface{}{
									"type": "string",
								},
								"method": map[string]interface{}{
									"oneOf": []interface{}{
										map[string]interface{}{
											"type": "string",
										},
										map[string]interface{}{
											"type": "array",
											"items": map[string]interface{}{
												"type": "string",
											},
										},
									},
								},
								"handler": map[string]interface{}{
									"type": "string",
								},
								"name": map[string]interface{}{
									"type": "string",
								},
								"description": map[string]interface{}{
									"type": "string",
								},
							},
							"required": []string{"path", "method"},
						},
					},
				},
			},
		},
		Required: []string{"apiVersion", "kind", "metadata", "spec"},
	})

	// Config schema
	sr.RegisterSchema(KindConfig, &Schema{
		Schema: "http://json-schema.org/draft-07/schema#",
		Title:  "System Configuration",
		Type:   "object",
		Properties: map[string]interface{}{
			"version": map[string]interface{}{
				"type": "string",
			},
			"metadata": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"description": map[string]interface{}{
						"type": "string",
					},
					"last_updated": map[string]interface{}{
						"type": "string",
					},
				},
			},
			"settings": map[string]interface{}{
				"type": "array",
				"items": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"name": map[string]interface{}{
							"type": "string",
						},
						"navigation": map[string]interface{}{
							"type": "string",
						},
						"description": map[string]interface{}{
							"type": "string",
						},
						"group": map[string]interface{}{
							"type": "string",
						},
						"type": map[string]interface{}{
							"type": "string",
							"enum": []string{"string", "integer", "boolean", "email", "select", "array"},
						},
						"default": map[string]interface{}{},
						"required": map[string]interface{}{
							"type": "boolean",
						},
						"readonly": map[string]interface{}{
							"type": "boolean",
						},
					},
					"required": []string{"name", "type"},
				},
			},
		},
		Required: []string{"version", "settings"},
	})

	// Dashboard schema
	sr.RegisterSchema(KindDashboard, &Schema{
		Schema: "http://json-schema.org/draft-07/schema#",
		Title:  "Dashboard Configuration",
		Type:   "object",
		Properties: map[string]interface{}{
			"apiVersion": map[string]interface{}{
				"type": "string",
			},
			"kind": map[string]interface{}{
				"type": "string",
				"enum": []string{"Dashboard"},
			},
			"metadata": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"name": map[string]interface{}{
						"type": "string",
					},
					"version": map[string]interface{}{
						"type": "string",
					},
					"created": map[string]interface{}{
						"type": "string",
					},
					"description": map[string]interface{}{
						"type": "string",
					},
				},
				"required": []string{"name"},
			},
			"spec": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"dashboard": map[string]interface{}{
						"type": "object",
						"properties": map[string]interface{}{
							"title": map[string]interface{}{
								"type": "string",
							},
							"subtitle": map[string]interface{}{
								"type": "string",
							},
							"theme": map[string]interface{}{
								"type": "string",
							},
							"stats": map[string]interface{}{
								"type": "array",
								"items": map[string]interface{}{
									"type": "object",
									"properties": map[string]interface{}{
										"name": map[string]interface{}{
											"type": "string",
										},
										"icon": map[string]interface{}{
											"type": "string",
										},
										"color": map[string]interface{}{
											"type": "string",
										},
									},
									"required": []string{"name"},
								},
							},
							"tiles": map[string]interface{}{
								"type": "array",
								"items": map[string]interface{}{
									"type": "object",
									"properties": map[string]interface{}{
										"name": map[string]interface{}{
											"type": "string",
										},
										"description": map[string]interface{}{
											"type": "string",
										},
										"url": map[string]interface{}{
											"type": "string",
										},
										"icon": map[string]interface{}{
											"type": "string",
										},
										"color": map[string]interface{}{
											"type": "string",
										},
									},
									"required": []string{"name", "url"},
								},
							},
						},
						"required": []string{"title"},
					},
				},
				"required": []string{"dashboard"},
			},
		},
		Required: []string{"apiVersion", "kind", "metadata", "spec"},
	})
}
