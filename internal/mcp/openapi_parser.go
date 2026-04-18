package mcp

import (
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

// OpenAPISpec holds the parsed portions of an OpenAPI 3.0 spec needed for MCP tool schema generation.
type OpenAPISpec struct {
	raw map[string]any // full parsed YAML document
}

// OpenAPIOperation holds extracted metadata for a single API operation.
type OpenAPIOperation struct {
	Summary     string
	Description string
	Parameters  []OpenAPIParam
	BodySchema  *OpenAPISchema // nil for GET/DELETE
}

// OpenAPIParam represents a query or path parameter from the spec.
type OpenAPIParam struct {
	Name        string
	In          string // "query", "path", "header"
	Description string
	Required    bool
	Type        string // "string", "integer", "number", "boolean"
}

// OpenAPISchema represents properties of a request body schema.
type OpenAPISchema struct {
	Properties map[string]OpenAPIProp
	Required   []string
}

// OpenAPIProp represents a single property in a schema.
type OpenAPIProp struct {
	Type        string
	Description string
	Enum        []string
	Default     any
	Format      string
}

// ParseOpenAPIFile loads and parses an OpenAPI YAML file.
func ParseOpenAPIFile(path string) (*OpenAPISpec, error) {
	data, err := os.ReadFile(path) //nolint:gosec // G304 - path from config
	if err != nil {
		return nil, fmt.Errorf("read openapi spec: %w", err)
	}
	var raw map[string]any
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("parse openapi spec: %w", err)
	}
	return &OpenAPISpec{raw: raw}, nil
}

// LookupOperation finds the OpenAPI operation for a given method and path.
// The path uses OpenAPI format ({id}) not Gin format (:id).
func (s *OpenAPISpec) LookupOperation(method, path string) *OpenAPIOperation {
	if s == nil || s.raw == nil {
		return nil
	}
	paths, ok := mapGet[map[string]any](s.raw, "paths")
	if !ok {
		return nil
	}
	pathItem, ok := mapGet[map[string]any](paths, path)
	if !ok {
		return nil
	}
	op, ok := mapGet[map[string]any](pathItem, strings.ToLower(method))
	if !ok {
		return nil
	}
	result := &OpenAPIOperation{}
	result.Summary, _ = mapGet[string](op, "summary")
	result.Description, _ = mapGet[string](op, "description")

	// Parse parameters
	if params, ok := mapGet[[]any](op, "parameters"); ok {
		for _, p := range params {
			param := s.resolveParam(p)
			if param != nil {
				result.Parameters = append(result.Parameters, *param)
			}
		}
	}

	// Parse request body
	if reqBody, ok := mapGet[map[string]any](op, "requestBody"); ok {
		result.BodySchema = s.resolveRequestBody(reqBody)
	}

	return result
}

func (s *OpenAPISpec) resolveParam(raw any) *OpenAPIParam {
	m, ok := raw.(map[string]any)
	if !ok {
		return nil
	}
	// Handle $ref
	if ref, ok := mapGet[string](m, "$ref"); ok {
		resolved := s.resolveRef(ref)
		if resolved == nil {
			return nil
		}
		m = resolved
	}
	param := &OpenAPIParam{}
	param.Name, _ = mapGet[string](m, "name")
	param.In, _ = mapGet[string](m, "in")
	param.Description, _ = mapGet[string](m, "description")
	param.Required, _ = mapGet[bool](m, "required")

	if schema, ok := mapGet[map[string]any](m, "schema"); ok {
		param.Type, _ = mapGet[string](schema, "type")
	}
	if param.Name == "" {
		return nil
	}
	return param
}

func (s *OpenAPISpec) resolveRequestBody(reqBody map[string]any) *OpenAPISchema {
	content, ok := mapGet[map[string]any](reqBody, "content")
	if !ok {
		return nil
	}
	jsonContent, ok := mapGet[map[string]any](content, "application/json")
	if !ok {
		return nil
	}
	schema, ok := mapGet[map[string]any](jsonContent, "schema")
	if !ok {
		return nil
	}
	return s.resolveSchema(schema)
}

func (s *OpenAPISpec) resolveSchema(schema map[string]any) *OpenAPISchema {
	// Handle $ref
	if ref, ok := mapGet[string](schema, "$ref"); ok {
		resolved := s.resolveRef(ref)
		if resolved == nil {
			return nil
		}
		schema = resolved
	}

	result := &OpenAPISchema{
		Properties: make(map[string]OpenAPIProp),
	}

	// Extract required fields
	if reqList, ok := mapGet[[]any](schema, "required"); ok {
		for _, r := range reqList {
			if s, ok := r.(string); ok {
				result.Required = append(result.Required, s)
			}
		}
	}

	// Extract properties
	props, ok := mapGet[map[string]any](schema, "properties")
	if !ok {
		return result
	}
	for name, raw := range props {
		propMap, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		// Resolve $ref in property
		if ref, ok := mapGet[string](propMap, "$ref"); ok {
			if resolved := s.resolveRef(ref); resolved != nil {
				propMap = resolved
			}
		}
		prop := OpenAPIProp{}
		prop.Type, _ = mapGet[string](propMap, "type")
		prop.Description, _ = mapGet[string](propMap, "description")
		prop.Format, _ = mapGet[string](propMap, "format")
		if defVal, ok := propMap["default"]; ok {
			prop.Default = defVal
		}
		if enumList, ok := mapGet[[]any](propMap, "enum"); ok {
			for _, e := range enumList {
				if s, ok := e.(string); ok {
					prop.Enum = append(prop.Enum, s)
				}
			}
		}
		result.Properties[name] = prop
	}
	return result
}

// resolveRef resolves a local $ref like "#/components/schemas/TicketCreate"
func (s *OpenAPISpec) resolveRef(ref string) map[string]any {
	if !strings.HasPrefix(ref, "#/") {
		return nil
	}
	parts := strings.Split(strings.TrimPrefix(ref, "#/"), "/")
	var current any = s.raw
	for _, part := range parts {
		m, ok := current.(map[string]any)
		if !ok {
			return nil
		}
		current, ok = m[part]
		if !ok {
			return nil
		}
	}
	if m, ok := current.(map[string]any); ok {
		return m
	}
	return nil
}

// ginPathToOpenAPI converts Gin path format (:id) to OpenAPI format ({id}).
func ginPathToOpenAPI(path string) string {
	result := path
	// Replace :param with {param}
	for {
		idx := strings.Index(result, "/:")
		if idx == -1 {
			break
		}
		end := strings.IndexByte(result[idx+2:], '/')
		if end == -1 {
			// :param is at the end
			paramName := result[idx+2:]
			result = result[:idx] + "/{" + paramName + "}"
		} else {
			paramName := result[idx+2 : idx+2+end]
			result = result[:idx] + "/{" + paramName + "}" + result[idx+2+end:]
		}
	}
	return result
}

// mapGet is a type-safe map accessor.
func mapGet[T any](m map[string]any, key string) (T, bool) {
	v, ok := m[key]
	if !ok {
		var zero T
		return zero, false
	}
	t, ok := v.(T)
	return t, ok
}
