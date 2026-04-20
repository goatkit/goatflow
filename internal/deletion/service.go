package deletion

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/goatkit/goatflow/internal/database"
	"github.com/goatkit/goatflow/internal/organisation"
)

// CascadeHandler is a function that a plugin registers to handle entity deletion.
type CascadeHandler func(ctx context.Context, entityType string, entityID int64) error

// pluginCascadeEntry holds a plugin's soft and/or hard cascade handlers for
// a given (entityType, pluginName) pair. Either handler may be nil.
type pluginCascadeEntry struct {
	soft CascadeHandler
	hard CascadeHandler
}

// Package-level cascade registry. Shared across all Service instances
// because deletion.NewService() is called per-HostAPI-request rather
// than held as a singleton, so instance-local registrations would be
// lost between the plugin-load call that registers them and the
// EntitySoftDelete call that needs to fire them.
var (
	pluginCascadeMu sync.RWMutex
	pluginCascades  = make(map[string]map[string]pluginCascadeEntry) // entityType → pluginName → entry

	// preDispatchHook is invoked immediately before runCascades iterates
	// the registry. The plugin manager sets it at boot to eager-load any
	// discovered-but-unloaded plugin, so a plugin that declares Cascades
	// but has never been called still has its handlers registered by the
	// time the dispatch loop runs. Nil when the platform runs without a
	// plugin manager (tests).
	preDispatchHook func(ctx context.Context, entityType string)
)

// SetPreDispatchHook registers a hook that the deletion service calls
// before dispatching cascades for an entityType. Intended for the
// plugin manager to lazy-load plugins whose cascade closures aren't in
// the registry yet. Safe to call once at boot.
func SetPreDispatchHook(fn func(ctx context.Context, entityType string)) {
	pluginCascadeMu.Lock()
	defer pluginCascadeMu.Unlock()
	preDispatchHook = fn
}

// RegisterPluginCascade records a plugin's cascade handlers for an
// entity type. Either soft or hard may be nil — only the non-nil
// handler fires for its corresponding deletion mode. Re-registering
// the same (entityType, pluginName) pair replaces the prior entry,
// which is what plugin reloads want.
func RegisterPluginCascade(entityType, pluginName string, soft, hard CascadeHandler) {
	pluginCascadeMu.Lock()
	defer pluginCascadeMu.Unlock()
	if _, ok := pluginCascades[entityType]; !ok {
		pluginCascades[entityType] = make(map[string]pluginCascadeEntry)
	}
	pluginCascades[entityType][pluginName] = pluginCascadeEntry{soft: soft, hard: hard}
}

// UnregisterPluginCascades clears every cascade handler registered by
// pluginName. Called when a plugin is unloaded or replaced so stale
// closures don't keep dispatching to a gone plugin.
func UnregisterPluginCascades(pluginName string) {
	pluginCascadeMu.Lock()
	defer pluginCascadeMu.Unlock()
	for entityType, m := range pluginCascades {
		delete(m, pluginName)
		if len(m) == 0 {
			delete(pluginCascades, entityType)
		}
	}
}

// pluginCascadesFor returns all handlers to fire for (entityType, mode),
// in the order plugins were registered (map iteration so actually
// unordered — callers must not depend on ordering).
func pluginCascadesFor(entityType, mode string) []CascadeHandler {
	pluginCascadeMu.RLock()
	defer pluginCascadeMu.RUnlock()
	entries := pluginCascades[entityType]
	handlers := make([]CascadeHandler, 0, len(entries))
	for _, e := range entries {
		switch mode {
		case "soft":
			if e.soft != nil {
				handlers = append(handlers, e.soft)
			}
		case "hard":
			if e.hard != nil {
				handlers = append(handlers, e.hard)
			}
		}
	}
	return handlers
}

// Service orchestrates entity deletion: soft delete, restore, hard delete,
// anonymisation, cascade, and tombstone logging.
type Service struct {
	repo             *Repository
	cascadeHandlers  map[string][]CascadeHandler // entityType → handlers
	retentionDays    map[string]int              // entityType → days
	defaultRetention int
}

// NewService creates a new deletion service.
func NewService() (*Service, error) {
	repo, err := NewRepository()
	if err != nil {
		return nil, err
	}
	return &Service{
		repo:             repo,
		cascadeHandlers:  make(map[string][]CascadeHandler),
		retentionDays:    make(map[string]int),
		defaultRetention: 60,
	}, nil
}

// NewServiceWithDB creates a deletion service with an explicit DB.
func NewServiceWithDB(db *sql.DB) *Service {
	return &Service{
		repo:             NewRepositoryWithDB(db),
		cascadeHandlers:  make(map[string][]CascadeHandler),
		retentionDays:    make(map[string]int),
		defaultRetention: 60,
	}
}

// RegisterCascade registers a cascade handler for an entity type.
func (s *Service) RegisterCascade(entityType string, handler CascadeHandler) {
	s.cascadeHandlers[entityType] = append(s.cascadeHandlers[entityType], handler)
}

// SetRetention sets the retention period for an entity type.
func (s *Service) SetRetention(entityType string, days int) {
	s.retentionDays[entityType] = days
}

// SoftDelete soft-deletes an entity: marks as deleted, anonymises PII, adds to recycle bin, logs.
func (s *Service) SoftDelete(ctx context.Context, entityType string, entityID int64, userID int, reason string) error {
	// Soft-delete using the entity's native mechanism.
	entityName, err := s.softDeleteEntity(entityType, entityID, userID)
	if err != nil {
		return fmt.Errorf("soft delete %s/%d: %w", entityType, entityID, err)
	}

	// Anonymise PII.
	if err := s.repo.AnonymiseEntity(entityType, entityID); err != nil {
		slog.Warn("anonymisation failed", "entity", entityType, "id", entityID, "error", err)
	}

	// Calculate expiry.
	var expiresAt *time.Time
	days := s.retentionForType(entityType)
	if days > 0 {
		t := time.Now().AddDate(0, 0, days)
		expiresAt = &t
	}

	// Add to recycle bin.
	orgID := organisation.OrgIDFromContext(ctx)
	var orgIDPtr *int64
	if orgID > 0 {
		orgIDPtr = &orgID
	}

	entry := &RecycleBinEntry{
		EntityType: entityType,
		EntityID:   entityID,
		EntityName: &entityName,
		DeletedBy:  userID,
		DeletedAt:  time.Now(),
		ExpiresAt:  expiresAt,
		OrgID:      orgIDPtr,
	}
	if _, err := s.repo.AddToRecycleBin(entry); err != nil {
		return fmt.Errorf("add to recycle bin: %w", err)
	}

	// Tombstone log.
	var reasonPtr *string
	if reason != "" {
		reasonPtr = &reason
	}
	s.repo.LogDeletion(&DeletionLog{
		EntityType: entityType, EntityID: entityID, Action: ActionSoftDelete,
		DeletedBy: userID, DeletedAt: time.Now(), OrgID: orgIDPtr, Reason: reasonPtr,
	})

	// Plugin cascades (soft delete).
	s.runCascades(ctx, entityType, entityID, "soft")

	return nil
}

// Restore restores a soft-deleted entity from the recycle bin.
func (s *Service) Restore(ctx context.Context, entityType string, entityID int64, userID int) error {
	// Verify it's in the recycle bin.
	entry, err := s.repo.GetRecycleBinEntry(entityType, entityID)
	if err != nil {
		return err
	}
	if entry == nil {
		return fmt.Errorf("entity %s/%d not found in recycle bin", entityType, entityID)
	}

	// Restore using entity's native mechanism.
	if err := s.restoreEntity(entityType, entityID, userID); err != nil {
		return fmt.Errorf("restore %s/%d: %w", entityType, entityID, err)
	}

	// Remove from recycle bin.
	if err := s.repo.RemoveFromRecycleBin(entityType, entityID); err != nil {
		return err
	}

	// Tombstone log.
	orgID := organisation.OrgIDFromContext(ctx)
	var orgIDPtr *int64
	if orgID > 0 {
		orgIDPtr = &orgID
	}
	s.repo.LogDeletion(&DeletionLog{
		EntityType: entityType, EntityID: entityID, Action: ActionRestore,
		DeletedBy: userID, DeletedAt: time.Now(), OrgID: orgIDPtr,
	})

	return nil
}

// HardDelete permanently removes an entity and all linked data.
func (s *Service) HardDelete(ctx context.Context, entityType string, entityID int64, userID int, reason string) error {
	// Plugin cascades (hard delete) — before we delete the entity.
	s.runCascades(ctx, entityType, entityID, "hard")

	// Hard delete the entity.
	if err := s.hardDeleteEntity(entityType, entityID); err != nil {
		return fmt.Errorf("hard delete %s/%d: %w", entityType, entityID, err)
	}

	// Remove from recycle bin if present.
	s.repo.RemoveFromRecycleBin(entityType, entityID)

	// Tombstone log.
	orgID := organisation.OrgIDFromContext(ctx)
	var orgIDPtr *int64
	if orgID > 0 {
		orgIDPtr = &orgID
	}
	var reasonPtr *string
	if reason != "" {
		reasonPtr = &reason
	}
	s.repo.LogDeletion(&DeletionLog{
		EntityType: entityType, EntityID: entityID, Action: ActionHardDelete,
		DeletedBy: userID, DeletedAt: time.Now(), OrgID: orgIDPtr, Reason: reasonPtr,
	})

	return nil
}

// RecycleBinList returns entries from the recycle bin.
func (s *Service) RecycleBinList(ctx context.Context, entityType string) ([]RecycleBinEntry, error) {
	orgID := organisation.OrgIDFromContext(ctx)
	return s.repo.ListRecycleBin(entityType, orgID)
}

// ScopeSoftDelete soft-deletes all entities of a given type within an org scope.
// Used for project/exercise teardown.
func (s *Service) ScopeSoftDelete(ctx context.Context, entityType string, entityIDs []int64, userID int, reason string) (int, error) {
	deleted := 0
	for _, id := range entityIDs {
		if err := s.SoftDelete(ctx, entityType, id, userID, reason); err != nil {
			slog.Warn("scope soft delete failed", "entity", entityType, "id", id, "error", err)
			continue
		}
		deleted++
	}
	return deleted, nil
}

// ScopeHardDelete permanently removes all entities of a given type by IDs.
// Requires entity.hard_delete permission (checked by caller).
func (s *Service) ScopeHardDelete(ctx context.Context, entityType string, entityIDs []int64, userID int, reason string) (int, error) {
	deleted := 0
	for _, id := range entityIDs {
		if err := s.HardDelete(ctx, entityType, id, userID, reason); err != nil {
			slog.Warn("scope hard delete failed", "entity", entityType, "id", id, "error", err)
			continue
		}
		deleted++
	}
	return deleted, nil
}

// PurgeExpired hard-deletes all expired recycle bin entries. Called by scheduled job.
func (s *Service) PurgeExpired(ctx context.Context) (int, error) {
	expired, err := s.repo.ListExpired()
	if err != nil {
		return 0, err
	}

	purged := 0
	for _, entry := range expired {
		if err := s.HardDelete(ctx, entry.EntityType, entry.EntityID, entry.DeletedBy, "auto-purge: retention expired"); err != nil {
			slog.Warn("auto-purge failed", "entity", entry.EntityType, "id", entry.EntityID, "error", err)
			continue
		}
		purged++
	}
	return purged, nil
}

// --- Internal entity-type dispatch ---

func (s *Service) softDeleteEntity(entityType string, entityID int64, userID int) (string, error) {
	db, err := database.GetDB()
	if err != nil {
		return "", err
	}

	now := time.Now()
	switch entityType {
	case EntityTicket:
		var title string
		database.ConvertPlaceholders("SELECT title FROM ticket WHERE id = ?")
		db.QueryRow(database.ConvertPlaceholders("SELECT title FROM ticket WHERE id = ?"), entityID).Scan(&title)
		_, err := db.Exec(database.ConvertPlaceholders(
			"UPDATE ticket SET archive_flag = 1, ticket_state_id = 2, change_time = ?, change_by = ? WHERE id = ?"),
			now, userID, entityID)
		return title, err

	case EntityContact:
		var name string
		db.QueryRow(database.ConvertPlaceholders("SELECT CONCAT(first_name, ' ', last_name) FROM customer_user WHERE id = ?"), entityID).Scan(&name)
		_, err := db.Exec(database.ConvertPlaceholders(
			"UPDATE customer_user SET valid_id = 2, change_time = ?, change_by = ? WHERE id = ?"),
			now, userID, entityID)
		return name, err

	case EntityAgent:
		var login string
		db.QueryRow(database.ConvertPlaceholders("SELECT login FROM users WHERE id = ?"), entityID).Scan(&login)
		_, err := db.Exec(database.ConvertPlaceholders(
			"UPDATE users SET valid_id = 2, change_time = ?, change_by = ? WHERE id = ?"),
			now, userID, entityID)
		return login, err

	case EntityQueue:
		var name string
		db.QueryRow(database.ConvertPlaceholders("SELECT name FROM queue WHERE id = ?"), entityID).Scan(&name)
		_, err := db.Exec(database.ConvertPlaceholders(
			"UPDATE queue SET valid_id = 2, change_time = ?, change_by = ? WHERE id = ?"),
			now, userID, entityID)
		return name, err

	case EntityOrganisation:
		var name string
		db.QueryRow(database.ConvertPlaceholders("SELECT name FROM gk_organisation WHERE id = ?"), entityID).Scan(&name)
		_, err := db.Exec(database.ConvertPlaceholders(
			"UPDATE gk_organisation SET status = 'archived', change_time = ?, change_by = ? WHERE id = ?"),
			now, userID, entityID)
		return name, err

	case EntityCustomerGroup:
		var name string
		db.QueryRow(database.ConvertPlaceholders("SELECT name FROM customer_company WHERE customer_id = ?"), entityID).Scan(&name)
		_, err := db.Exec(database.ConvertPlaceholders(
			"UPDATE customer_company SET valid_id = 2, change_time = ?, change_by = ? WHERE customer_id = ?"),
			now, userID, entityID)
		return name, err

	default:
		return "", fmt.Errorf("unsupported entity type for soft delete: %s", entityType)
	}
}

func (s *Service) restoreEntity(entityType string, entityID int64, userID int) error {
	db, err := database.GetDB()
	if err != nil {
		return err
	}

	now := time.Now()
	switch entityType {
	case EntityTicket:
		_, err := db.Exec(database.ConvertPlaceholders(
			"UPDATE ticket SET archive_flag = 0, ticket_state_id = 1, change_time = ?, change_by = ? WHERE id = ?"),
			now, userID, entityID)
		return err

	case EntityContact:
		_, err := db.Exec(database.ConvertPlaceholders(
			"UPDATE customer_user SET valid_id = 1, change_time = ?, change_by = ? WHERE id = ?"),
			now, userID, entityID)
		return err

	case EntityAgent:
		_, err := db.Exec(database.ConvertPlaceholders(
			"UPDATE users SET valid_id = 1, change_time = ?, change_by = ? WHERE id = ?"),
			now, userID, entityID)
		return err

	case EntityQueue:
		_, err := db.Exec(database.ConvertPlaceholders(
			"UPDATE queue SET valid_id = 1, change_time = ?, change_by = ? WHERE id = ?"),
			now, userID, entityID)
		return err

	case EntityOrganisation:
		_, err := db.Exec(database.ConvertPlaceholders(
			"UPDATE gk_organisation SET status = 'active', change_time = ?, change_by = ? WHERE id = ?"),
			now, userID, entityID)
		return err

	default:
		return fmt.Errorf("unsupported entity type for restore: %s", entityType)
	}
}

func (s *Service) hardDeleteEntity(entityType string, entityID int64) error {
	db, err := database.GetDB()
	if err != nil {
		return err
	}

	switch entityType {
	case EntityTicket:
		db.Exec(database.ConvertPlaceholders("DELETE FROM ticket_history WHERE ticket_id = ?"), entityID)
		db.Exec(database.ConvertPlaceholders("DELETE FROM article_data_mime WHERE article_id IN (SELECT id FROM article WHERE ticket_id = ?)"), entityID)
		db.Exec(database.ConvertPlaceholders("DELETE FROM article WHERE ticket_id = ?"), entityID)
		_, err := db.Exec(database.ConvertPlaceholders("DELETE FROM ticket WHERE id = ?"), entityID)
		return err

	case EntityContact:
		_, err := db.Exec(database.ConvertPlaceholders("DELETE FROM customer_user WHERE id = ?"), entityID)
		return err

	case EntityAgent:
		db.Exec(database.ConvertPlaceholders("DELETE FROM group_user WHERE user_id = ?"), entityID)
		_, err := db.Exec(database.ConvertPlaceholders("DELETE FROM users WHERE id = ?"), entityID)
		return err

	case EntityQueue:
		_, err := db.Exec(database.ConvertPlaceholders("DELETE FROM queue WHERE id = ?"), entityID)
		return err

	case EntityOrganisation:
		_, err := db.Exec(database.ConvertPlaceholders("DELETE FROM gk_organisation WHERE id = ?"), entityID)
		return err

	default:
		return fmt.Errorf("unsupported entity type for hard delete: %s", entityType)
	}
}

func (s *Service) runCascades(ctx context.Context, entityType string, entityID int64, mode string) {
	// Let the plugin manager ensure any discovered-but-unloaded plugin
	// gets a chance to register its cascade closures before we dispatch.
	// Without this, a plugin that was uploaded mid-session but never
	// called would silently skip cascade cleanup.
	pluginCascadeMu.RLock()
	hook := preDispatchHook
	pluginCascadeMu.RUnlock()
	if hook != nil {
		hook(ctx, entityType)
	}

	// Instance-local handlers fire on every mode (existing behaviour,
	// preserved for tests and code that uses RegisterCascade directly).
	for _, h := range s.cascadeHandlers[entityType] {
		if err := h(ctx, entityType, entityID); err != nil {
			slog.Warn("cascade handler failed", "entity", entityType, "id", entityID, "mode", mode, "error", err)
		}
	}
	// Plugin-registered handlers are mode-aware (CascadeSpec splits into
	// OnSoftDelete / OnHardDelete; only the one matching `mode` fires).
	for _, h := range pluginCascadesFor(entityType, mode) {
		if err := h(ctx, entityType, entityID); err != nil {
			slog.Warn("plugin cascade handler failed", "entity", entityType, "id", entityID, "mode", mode, "error", err)
		}
	}
}

func (s *Service) retentionForType(entityType string) int {
	if days, ok := s.retentionDays[entityType]; ok {
		return days
	}
	return s.defaultRetention
}

// --- HostAPI types (for RecycleBinList return) ---

// ToJSON converts a RecycleBinEntry to a JSON-friendly map.
func (e *RecycleBinEntry) ToJSON() map[string]any {
	m := map[string]any{
		"entity_type": e.EntityType,
		"entity_id":   e.EntityID,
		"deleted_by":  e.DeletedBy,
		"deleted_at":  e.DeletedAt.Format(time.RFC3339),
	}
	if e.EntityName != nil {
		m["entity_name"] = *e.EntityName
	}
	if e.ExpiresAt != nil {
		m["expires_at"] = e.ExpiresAt.Format(time.RFC3339)
	}
	return m
}

// RecycleBinToJSON converts a slice of entries to JSON bytes.
func RecycleBinToJSON(entries []RecycleBinEntry) (json.RawMessage, error) {
	items := make([]map[string]any, len(entries))
	for i, e := range entries {
		items[i] = e.ToJSON()
	}
	return json.Marshal(items)
}
