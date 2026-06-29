package models

// LookupItem represents a generic lookup value (priority, type, status, etc.)
type LookupItem struct {
	ID     int    `json:"id"`
	Value  string `json:"value"`
	Label  string `json:"label"`
	Order  int    `json:"order"`
	Active bool   `json:"active"`
}
