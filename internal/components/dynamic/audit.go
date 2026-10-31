package dynamic

import (
	"database/sql"
	"fmt"
	"sort"

	"github.com/gin-gonic/gin"

	"github.com/goatkit/goatflow/internal/platform/audit"
)

// Dynamic modules are served on admin routes only, so their writes are
// recorded as admin changes: admin.<module>.<verb>, with the record read
// before and after the write. Password fields are reduced to markers before
// comparing, so neither the value nor its hash reaches the log.

const (
	auditPasswordSet     = "(set)"
	auditPasswordChanged = "(changed)"
)

// auditState reads the record as the audit log sees it, or nil if it
// cannot be read.
func (h *DynamicModuleHandler) auditState(config *ModuleConfig, id any) map[string]any {
	query := fmt.Sprintf("SELECT %s FROM %s WHERE id = ?", h.getSelectColumns(config), config.Module.Table) //nolint:gk-sql-sprintf // trusted schema identifier from validated field config; values bound via ?
	row := h.scanRow(h.queryRow(query, id), config)
	if row == nil {
		return nil
	}
	state := make(map[string]any, len(config.Fields)+1)
	for _, f := range config.Fields {
		if f.Virtual || auditSkipColumn(f.DBColumn) {
			continue
		}
		v, ok := row[f.Name]
		if f.Type == "password" {
			if ok && fmt.Sprint(v) != "" {
				state[f.Name] = auditPasswordSet
			}
			continue
		}
		if ok {
			state[f.Name] = v
		}
	}
	if config.Module.Name == "users" {
		if groups, err := h.userGroupNames(id); err == nil {
			state["groups"] = groups
		}
	}
	return state
}

func auditSkipColumn(col string) bool {
	switch col {
	case "id", "create_time", "create_by", "change_time", "change_by":
		return true
	}
	return false
}

func (h *DynamicModuleHandler) userGroupNames(id any) ([]string, error) {
	rows, err := h.query(`
		SELECT DISTINCT g.name
		FROM groups g
		INNER JOIN group_user gu ON g.id = gu.group_id
		WHERE gu.user_id = ?`, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	names := []string{}
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		names = append(names, name)
	}
	sort.Strings(names)
	return names, rows.Err()
}

// markPasswordChanges marks the password fields a write replaced, so the
// diff shows that they changed.
func markPasswordChanges(config *ModuleConfig, data, after map[string]any) {
	if after == nil {
		return
	}
	for _, f := range config.Fields {
		if f.Type != "password" {
			continue
		}
		if s, ok := data[f.Name].(string); ok && s != "" {
			after[f.Name] = auditPasswordChanged
		}
	}
}

// recordModuleChange records one admin event for a dynamic module write.
// before is nil for creates and after is nil for deletes.
func recordModuleChange(c *gin.Context, config *ModuleConfig, verb, id string, before, after map[string]any, extra map[string]any) {
	state := after
	if state == nil {
		state = before
	}
	targetID := id
	if config.Module.Name == "sysconfig" {
		if name, ok := state["name"].(string); ok && name != "" {
			targetID = name
		}
	}
	details := map[string]any{}
	switch {
	case before != nil && after != nil:
		changes := audit.Changes(before, after)
		if config.Module.Name == "sysconfig" && audit.Sensitive(targetID) {
			redactChanges(changes)
		}
		if changes != nil {
			details["changes"] = changes
		}
	case state != nil:
		fields := audit.Snapshot(state)
		if config.Module.Name == "sysconfig" && audit.Sensitive(targetID) {
			delete(fields, "effective_value")
		}
		details["fields"] = fields
	}
	for k, v := range extra {
		details[k] = v
	}
	audit.RecordRequest(c, audit.Event{
		Category:    audit.CategoryAdmin,
		Action:      "admin." + config.Module.Name + "." + verb,
		Outcome:     audit.OutcomeSuccess,
		TargetType:  config.Module.Name,
		TargetID:    targetID,
		TargetLabel: auditLabel(state),
		Details:     details,
	})
}

// redactChanges hides every value in a Changes result.
func redactChanges(changes map[string]any) {
	for k := range changes {
		changes[k] = map[string]any{"from": audit.Redacted, "to": audit.Redacted}
	}
}

func auditLabel(state map[string]any) string {
	for _, k := range []string{"name", "login", "title", "email"} {
		if v, ok := state[k]; ok && v != nil {
			if s := fmt.Sprint(v); s != "" {
				return s
			}
		}
	}
	return ""
}

func nullValue(v sql.NullString) any {
	if !v.Valid {
		return nil
	}
	return v.String
}
