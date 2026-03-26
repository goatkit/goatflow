package mcp

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/goatkit/goatflow/internal/customfields"
)

func (s *Server) toolCustomFieldsGet(_ context.Context, args map[string]any) (*ToolCallResult, error) {
	entityType, _ := args["entity_type"].(string)
	objectID := int64(getInt(args, "object_id", 0))
	if entityType == "" || objectID == 0 {
		return nil, fmt.Errorf("entity_type and object_id are required")
	}

	if !customfields.IsValidEntityType(entityType) {
		return nil, fmt.Errorf("invalid entity_type: %s", entityType)
	}

	var fieldNames []string
	if f, ok := args["fields"].([]any); ok {
		for _, v := range f {
			if s, ok := v.(string); ok {
				fieldNames = append(fieldNames, s)
			}
		}
	}

	repo, err := customfields.NewRepository()
	if err != nil {
		return nil, fmt.Errorf("database error: %w", err)
	}

	values, err := repo.GetValues(entityType, objectID, fieldNames)
	if err != nil {
		return nil, fmt.Errorf("get values: %w", err)
	}

	data, _ := json.MarshalIndent(values, "", "  ")
	return &ToolCallResult{Content: []ContentBlock{{Type: "text", Text: string(data)}}}, nil
}

func (s *Server) toolCustomFieldsSet(_ context.Context, args map[string]any) (*ToolCallResult, error) {
	entityType, _ := args["entity_type"].(string)
	objectID := int64(getInt(args, "object_id", 0))
	values, _ := args["values"].(map[string]any)

	if entityType == "" || objectID == 0 || len(values) == 0 {
		return nil, fmt.Errorf("entity_type, object_id, and values are required")
	}

	if !customfields.IsValidEntityType(entityType) {
		return nil, fmt.Errorf("invalid entity_type: %s", entityType)
	}

	repo, err := customfields.NewRepository()
	if err != nil {
		return nil, fmt.Errorf("database error: %w", err)
	}

	// Load defs for validation.
	defs, err := repo.ListDefs(entityType, "", "", true)
	if err != nil {
		return nil, fmt.Errorf("load defs: %w", err)
	}
	defMap := make(map[string]*customfields.FieldDef, len(defs))
	for i := range defs {
		defMap[defs[i].Name] = &defs[i]
	}

	if errs := customfields.ValidateValues(defMap, values); errs != nil {
		data, _ := json.MarshalIndent(errs, "", "  ")
		return &ToolCallResult{Content: []ContentBlock{{Type: "text", Text: "Validation failed:\n" + string(data)}}}, nil
	}

	if err := repo.SetValues(entityType, objectID, values); err != nil {
		return nil, fmt.Errorf("set values: %w", err)
	}

	return &ToolCallResult{Content: []ContentBlock{{Type: "text", Text: fmt.Sprintf("Updated %d custom field(s) on %s %d", len(values), entityType, objectID)}}}, nil
}

func (s *Server) toolCustomFieldsQuery(_ context.Context, args map[string]any) (*ToolCallResult, error) {
	entityType, _ := args["entity_type"].(string)
	if entityType == "" {
		return nil, fmt.Errorf("entity_type is required")
	}

	if !customfields.IsValidEntityType(entityType) {
		return nil, fmt.Errorf("invalid entity_type: %s", entityType)
	}

	rawFilters, ok := args["filters"].([]any)
	if !ok || len(rawFilters) == 0 {
		return nil, fmt.Errorf("at least one filter is required")
	}

	var filters []customfields.FieldFilter
	for _, rf := range rawFilters {
		fm, ok := rf.(map[string]any)
		if !ok {
			continue
		}
		f := customfields.FieldFilter{
			Field:    fmt.Sprintf("%v", fm["field"]),
			Operator: fmt.Sprintf("%v", fm["operator"]),
			Value:    fm["value"],
			Value2:   fm["value2"],
		}
		filters = append(filters, f)
	}

	repo, err := customfields.NewRepository()
	if err != nil {
		return nil, fmt.Errorf("database error: %w", err)
	}

	ids, err := repo.QueryByFields(entityType, filters)
	if err != nil {
		return nil, fmt.Errorf("query: %w", err)
	}

	data, _ := json.MarshalIndent(map[string]any{"entity_type": entityType, "matching_ids": ids, "count": len(ids)}, "", "  ")
	return &ToolCallResult{Content: []ContentBlock{{Type: "text", Text: string(data)}}}, nil
}

func (s *Server) toolCustomFieldsList(_ context.Context, args map[string]any) (*ToolCallResult, error) {
	entityType, _ := args["entity_type"].(string)

	repo, err := customfields.NewRepository()
	if err != nil {
		return nil, fmt.Errorf("database error: %w", err)
	}

	defs, err := repo.ListDefs(entityType, "", "", true)
	if err != nil {
		return nil, fmt.Errorf("list defs: %w", err)
	}

	// Return a summary.
	type defSummary struct {
		ID         int64  `json:"id"`
		Name       string `json:"name"`
		Label      string `json:"label"`
		EntityType string `json:"entity_type"`
		FieldType  string `json:"field_type"`
		OwnerType  string `json:"owner_type"`
		Section    string `json:"section"`
		Required   bool   `json:"required"`
	}
	summaries := make([]defSummary, len(defs))
	for i, d := range defs {
		summaries[i] = defSummary{
			ID:         d.ID,
			Name:       d.Name,
			Label:      d.Label,
			EntityType: d.EntityType,
			FieldType:  d.FieldType,
			OwnerType:  d.OwnerType,
			Section:    d.Section,
			Required:   d.Required,
		}
	}

	data, _ := json.MarshalIndent(summaries, "", "  ")
	return &ToolCallResult{Content: []ContentBlock{{Type: "text", Text: string(data)}}}, nil
}
