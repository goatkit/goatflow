package models

import (
	"time"
)

// TicketAttributeRelation represents a relationship between two ticket attributes.
// When a value is selected for Attribute1, it restricts the possible values for Attribute2.
// Data is imported from CSV or Excel files with two columns mapping attr1 values to attr2 values.
type TicketAttributeRelation struct {
	ID         int64     `json:"id" db:"id"`
	Filename   string    `json:"filename" db:"filename"`
	Attribute1 string    `json:"attribute_1" db:"attribute_1"` // e.g., "Queue", "DynamicField_Category"
	Attribute2 string    `json:"attribute_2" db:"attribute_2"` // e.g., "DynamicField_Priority"
	ACLData    string    `json:"acl_data" db:"acl_data"`       // Raw CSV content or base64-encoded Excel
	Priority   int64     `json:"priority" db:"priority"`
	CreateTime time.Time `json:"create_time" db:"create_time"`
	CreateBy   int64     `json:"create_by" db:"create_by"`
	ChangeTime time.Time `json:"change_time" db:"change_time"`
	ChangeBy   int64     `json:"change_by" db:"change_by"`

	// Parsed data (not stored in DB, computed from ACLData)
	Data []AttributeRelationPair `json:"data,omitempty"`
}

// AttributeRelationPair represents a single mapping between attribute values.
type AttributeRelationPair struct {
	Attribute1Value string `json:"attr1_value"`
	Attribute2Value string `json:"attr2_value"`
}

// GetAllowedValues returns all Attribute2 values that are allowed when Attribute1 has the given value.
func (r *TicketAttributeRelation) GetAllowedValues(attr1Value string) []string {
	var allowed []string
	seen := make(map[string]bool)

	for _, pair := range r.Data {
		if pair.Attribute1Value == attr1Value {
			if !seen[pair.Attribute2Value] {
				allowed = append(allowed, pair.Attribute2Value)
				seen[pair.Attribute2Value] = true
			}
		}
	}

	return allowed
}

// GetUniqueAttribute1Values returns all unique values for Attribute1 in the relation data.
func (r *TicketAttributeRelation) GetUniqueAttribute1Values() []string {
	var values []string
	seen := make(map[string]bool)

	for _, pair := range r.Data {
		if !seen[pair.Attribute1Value] {
			values = append(values, pair.Attribute1Value)
			seen[pair.Attribute1Value] = true
		}
	}

	return values
}

// GetUniqueAttribute2Values returns all unique values for Attribute2 in the relation data.
func (r *TicketAttributeRelation) GetUniqueAttribute2Values() []string {
	var values []string
	seen := make(map[string]bool)

	for _, pair := range r.Data {
		if !seen[pair.Attribute2Value] {
			values = append(values, pair.Attribute2Value)
			seen[pair.Attribute2Value] = true
		}
	}

	return values
}

// ValidTicketAttributes lists the standard ticket attributes that can be used in relations.
var ValidTicketAttributes = []string{
	"Queue",
	"State",
	"Priority",
	"Type",
	"Service",
	"SLA",
	"Owner",
	"Responsible",
}

// IsValidAttribute checks if an attribute name is valid for ticket attribute relations.
// Valid attributes are either standard ticket attributes or dynamic fields (prefixed with "DynamicField_").
func IsValidAttribute(attr string) bool {
	// Check standard attributes
	for _, valid := range ValidTicketAttributes {
		if attr == valid {
			return true
		}
	}

	// Check dynamic field format
	if len(attr) > 13 && attr[:13] == "DynamicField_" {
		return true
	}

	return false
}

// IsDynamicFieldAttribute returns true if the attribute is a dynamic field reference.
func IsDynamicFieldAttribute(attr string) bool {
	return len(attr) > 13 && attr[:13] == "DynamicField_"
}

// GetDynamicFieldName extracts the field name from a "DynamicField_Name" attribute.
func GetDynamicFieldName(attr string) string {
	if IsDynamicFieldAttribute(attr) {
		return attr[13:]
	}
	return ""
}
