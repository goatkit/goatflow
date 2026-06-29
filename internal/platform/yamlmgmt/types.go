package yamlmgmt

import (
	"time"
)

// YAMLKind represents the type of YAML configuration.
type YAMLKind string

const (
	KindRoute     YAMLKind = "Route"
	KindConfig    YAMLKind = "Config"
	KindDashboard YAMLKind = "Dashboard"
	KindCompose   YAMLKind = "Compose"
	KindService   YAMLKind = "Service"
)

// YAMLDocument represents any YAML configuration document.
type YAMLDocument struct {
	APIVersion string                 `yaml:"apiVersion" json:"apiVersion"`
	Kind       string                 `yaml:"kind" json:"kind"`
	Metadata   Metadata               `yaml:"metadata" json:"metadata"`
	Spec       interface{}            `yaml:"spec" json:"spec"`
	Data       map[string]interface{} `yaml:"data,omitempty" json:"data,omitempty"`
}

// Metadata contains common metadata for all YAML types.
type Metadata struct {
	Name        string            `yaml:"name" json:"name"`
	Namespace   string            `yaml:"namespace,omitempty" json:"namespace,omitempty"`
	Version     string            `yaml:"version,omitempty" json:"version,omitempty"`
	Description string            `yaml:"description,omitempty" json:"description,omitempty"`
	Labels      map[string]string `yaml:"labels,omitempty" json:"labels,omitempty"`
	Annotations map[string]string `yaml:"annotations,omitempty" json:"annotations,omitempty"`
	Created     time.Time         `yaml:"created,omitempty" json:"created,omitempty"`
	Modified    time.Time         `yaml:"modified,omitempty" json:"modified,omitempty"`
	Author      string            `yaml:"author,omitempty" json:"author,omitempty"`
}

// Version represents a version of any YAML configuration.
type Version struct {
	ID         string        `json:"id"`
	Number     string        `json:"number"`
	Kind       YAMLKind      `json:"kind"`
	Name       string        `json:"name"`
	Hash       string        `json:"hash"`
	ParentHash string        `json:"parent_hash,omitempty"`
	Timestamp  time.Time     `json:"timestamp"`
	Author     string        `json:"author"`
	Message    string        `json:"message"`
	Document   *YAMLDocument `json:"document"`
	Changes    []Change      `json:"changes,omitempty"`
	Stats      *VersionStats `json:"stats,omitempty"`
}

// Change represents a change between versions.
type Change struct {
	Type        ChangeType  `json:"type"`
	Path        string      `json:"path"`
	OldValue    interface{} `json:"old_value,omitempty"`
	NewValue    interface{} `json:"new_value,omitempty"`
	Description string      `json:"description,omitempty"`
}

// ChangeType represents the type of change.
type ChangeType string

const (
	ChangeTypeAdd    ChangeType = "add"
	ChangeTypeModify ChangeType = "modify"
	ChangeTypeDelete ChangeType = "delete"
)

// VersionStats contains statistics about a version.
type VersionStats struct {
	TotalFields   int            `json:"total_fields"`
	ChangedFields int            `json:"changed_fields"`
	AddedFields   int            `json:"added_fields"`
	DeletedFields int            `json:"deleted_fields"`
	CustomStats   map[string]int `json:"custom_stats,omitempty"`
}

// ValidationResult represents the result of validation.
type ValidationResult struct {
	Valid    bool                `json:"valid"`
	Errors   []ValidationError   `json:"errors,omitempty"`
	Warnings []ValidationWarning `json:"warnings,omitempty"`
}

// ValidationError represents a validation error.
type ValidationError struct {
	Path    string `json:"path"`
	Message string `json:"message"`
	Code    string `json:"code"`
}

// ValidationWarning represents a validation warning.
type ValidationWarning struct {
	Path    string `json:"path"`
	Message string `json:"message"`
	Code    string `json:"code"`
}

// Schema represents a JSON schema for validation.
type Schema struct {
	ID          string                 `json:"$id,omitempty"`
	Schema      string                 `json:"$schema,omitempty"`
	Title       string                 `json:"title,omitempty"`
	Description string                 `json:"description,omitempty"`
	Type        string                 `json:"type"`
	Properties  map[string]interface{} `json:"properties,omitempty"`
	Required    []string               `json:"required,omitempty"`
	Additional  interface{}            `json:"additionalProperties,omitempty"`
}

// ConfigEvent represents a configuration change event.
type ConfigEvent struct {
	Type      EventType `json:"type"`
	Kind      YAMLKind  `json:"kind"`
	Name      string    `json:"name"`
	Version   *Version  `json:"version,omitempty"`
	Timestamp time.Time `json:"timestamp"`
	Error     string    `json:"error,omitempty"`
}

// EventType represents the type of configuration event.
type EventType string

const (
	EventTypeCreated  EventType = "created"
	EventTypeModified EventType = "modified"
	EventTypeDeleted  EventType = "deleted"
	EventTypeReloaded EventType = "reloaded"
	EventTypeRollback EventType = "rollback"
	EventTypeError    EventType = "error"
)

// ReloadHandler is a function that handles configuration reloads.
type ReloadHandler func(document *YAMLDocument) error

// ConfigStore interface for storing and retrieving configurations.
type ConfigStore interface {
	// Version management
	CreateVersion(kind YAMLKind, name string, doc *YAMLDocument, message string) (*Version, error)
	GetVersion(kind YAMLKind, name string, versionID string) (*Version, error)
	ListVersions(kind YAMLKind, name string) ([]*Version, error)
	Rollback(kind YAMLKind, name string, versionID string) error

	// Current configuration
	GetCurrent(kind YAMLKind, name string) (*YAMLDocument, error)
	SetCurrent(kind YAMLKind, name string, doc *YAMLDocument) error
	List(kind YAMLKind) ([]*YAMLDocument, error)
	Delete(kind YAMLKind, name string) error

	// Watching
	Watch(kind YAMLKind) (<-chan ConfigEvent, error)
	StopWatch(kind YAMLKind)
}

// Validator interface for validating YAML documents.
type Validator interface {
	Validate(doc *YAMLDocument) (*ValidationResult, error)
	GetSchema(kind YAMLKind) (*Schema, error)
}

// Linter interface for linting YAML documents.
type Linter interface {
	Lint(doc *YAMLDocument) ([]LintIssue, error)
	GetRules(kind YAMLKind) []LintRule
}

// LintIssue represents a linting issue.
type LintIssue struct {
	Severity string `json:"severity"`
	Rule     string `json:"rule"`
	Message  string `json:"message"`
	Path     string `json:"path"`
	Line     int    `json:"line,omitempty"`
}

// LintRule represents a linting rule.
type LintRule struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Severity    string `json:"severity"`
	Enabled     bool   `json:"enabled"`
}
