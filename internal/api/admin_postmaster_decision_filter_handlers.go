package api

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/flosch/pongo2/v6"
	"github.com/gin-gonic/gin"

	"github.com/goatkit/goatflow/internal/platform/audit"
	"github.com/goatkit/goatflow/internal/platform/database"
	"github.com/goatkit/goatflow/internal/platform/sysconfig"
)

const (
	// decisionFilterLLMPlugin answers the decision filter's questions and
	// lists the providers an admin can pick.
	decisionFilterLLMPlugin = "goatkit-llm"
	// decisionFilterProviderPurpose is the goatkit-llm provider purpose that
	// answers typed decisions.
	decisionFilterProviderPurpose = "decide"
	// decisionFilterProviderListTimeout bounds the provider lookup so a slow
	// plugin cannot hold the settings page.
	decisionFilterProviderListTimeout = 5 * time.Second
)

// decisionFilterProvider is a goatkit-llm decide provider offered on the page.
type decisionFilterProvider struct {
	ID       int64
	Name     string
	Model    string
	Active   bool
	Selected bool
}

// decisionFilterRow is one candidate queue, priority or type. Included rows
// are the saved options; Missing marks a saved option whose name no longer
// names a valid row, so the admin can see and drop it.
type decisionFilterRow struct {
	Name        string
	Description string
	Included    bool
	Missing     bool
}

// decisionFilterSection is the page view of one classified field.
type decisionFilterSection struct {
	Key          string
	Enabled      bool
	Instructions string
	Rows         []decisionFilterRow
}

// HandleAdminPostmasterDecisionFilter renders the inbound-mail decision
// filter settings page.
func HandleAdminPostmasterDecisionFilter(c *gin.Context) {
	db, err := database.GetDB()
	if err != nil || db == nil {
		log.Printf("HandleAdminPostmasterDecisionFilter: database unavailable: %v", err)
		sendErrorResponse(c, http.StatusInternalServerError, "Database unavailable")
		return
	}
	if getPongo2Renderer() == nil {
		sendErrorResponse(c, http.StatusInternalServerError, "Template renderer unavailable")
		return
	}

	// An unreadable stored value leaves the filter off; the page shows the
	// defaults and says so, and saving replaces the value.
	cfg, loadErr := sysconfig.LoadDecisionFilterConfig(db)
	if loadErr != nil {
		log.Printf("HandleAdminPostmasterDecisionFilter: load config: %v", loadErr)
	}

	sections, err := loadDecisionFilterSections(c.Request.Context(), db, cfg)
	if err != nil {
		log.Printf("HandleAdminPostmasterDecisionFilter: %v", err)
		sendErrorResponse(c, http.StatusInternalServerError, "Failed to load form options")
		return
	}

	providers, providersAvailable := listDecideProviders(c)
	providerMissing := cfg.ProviderID != 0
	for i := range providers {
		if providers[i].ID == cfg.ProviderID {
			providers[i].Selected = true
			providerMissing = false
		}
	}

	getPongo2Renderer().HTML(c, http.StatusOK, "pages/admin/postmaster_decision_filter.pongo2", pongo2.Context{
		"Title":              "Mail Decision Filter",
		"ActivePage":         "admin",
		"User":               getUserMapForTemplate(c),
		"Settings":           cfg,
		"Threshold":          strconv.FormatFloat(cfg.Threshold, 'f', -1, 64),
		"Sections":           sections,
		"Providers":          providers,
		"ProvidersAvailable": providersAvailable,
		"ProviderMissing":    providerMissing,
		"ConfigUnreadable":   loadErr != nil,
		"MaxOptions":         sysconfig.DecisionFilterMaxOptions,
		"MaxTimeout":         sysconfig.DecisionFilterMaxTimeout,
	})
}

// HandleSaveAdminPostmasterDecisionFilter stores the decision filter
// settings posted as a JSON DecisionFilterConfig.
func HandleSaveAdminPostmasterDecisionFilter(c *gin.Context) {
	var cfg sysconfig.DecisionFilterConfig
	if err := c.ShouldBindJSON(&cfg); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "Invalid request body"})
		return
	}
	userID, ok := auditUserID(c)
	if !ok {
		return
	}
	db, err := database.GetDB()
	if err != nil || db == nil {
		log.Printf("HandleSaveAdminPostmasterDecisionFilter: database unavailable: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "Database unavailable"})
		return
	}

	// Normalize here so the audit record shows the values that are stored.
	cfg.Normalize()
	before, _ := sysconfig.LoadDecisionFilterConfig(db)
	if err := sysconfig.SaveDecisionFilterConfig(db, cfg, userID); err != nil {
		var invalid *sysconfig.DecisionFilterValidationError
		if errors.As(err, &invalid) {
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": invalid.Error()})
			return
		}
		log.Printf("HandleSaveAdminPostmasterDecisionFilter: save config: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "Failed to save decision filter settings"})
		return
	}
	audit.RecordRequest(c, audit.Event{
		Category:    audit.CategoryAdmin,
		Action:      "admin.postmaster_decision_filter.update",
		Outcome:     audit.OutcomeSuccess,
		TargetType:  "postmaster_decision_filter",
		TargetID:    sysconfig.DecisionFilterKey,
		TargetLabel: "Mail Decision Filter",
		Details: map[string]any{"changes": audit.Changes(
			decisionFilterAuditFields(before), decisionFilterAuditFields(cfg))},
	})
	c.JSON(http.StatusOK, gin.H{"success": true})
}

func decisionFilterAuditFields(cfg sysconfig.DecisionFilterConfig) map[string]any {
	return map[string]any{
		"enabled":         cfg.Enabled,
		"provider_id":     cfg.ProviderID,
		"threshold":       cfg.Threshold,
		"timeout_seconds": cfg.TimeoutSeconds,
		"queue":           cfg.Queue,
		"priority":        cfg.Priority,
		"type":            cfg.Type,
	}
}

// loadDecisionFilterSections pairs each classified field with every valid
// row of its lookup table.
func loadDecisionFilterSections(ctx context.Context, db *sql.DB, cfg sysconfig.DecisionFilterConfig) ([]decisionFilterSection, error) {
	fields := []struct {
		key    string
		lookup string
		field  sysconfig.DecisionFilterField
	}{
		{"queue", "Queues", cfg.Queue},
		{"priority", "Priorities", cfg.Priority},
		{"type", "Types", cfg.Type},
	}
	sections := make([]decisionFilterSection, 0, len(fields))
	for _, f := range fields {
		items, err := queryLookupItems(ctx, db, formLookupQueries[f.lookup])
		if err != nil {
			return nil, fmt.Errorf("load %s: %w", f.lookup, err)
		}
		sections = append(sections, buildDecisionFilterSection(f.key, f.field, items))
	}
	return sections, nil
}

// buildDecisionFilterSection lists the saved options first, in their saved
// order, then the remaining lookup rows unselected.
func buildDecisionFilterSection(key string, field sysconfig.DecisionFilterField, items []LookupItem) decisionFilterSection {
	valid := make(map[string]bool, len(items))
	for _, item := range items {
		valid[item.Name] = true
	}
	saved := make(map[string]bool, len(field.Options))
	rows := make([]decisionFilterRow, 0, len(items)+len(field.Options))
	for _, opt := range field.Options {
		saved[opt.Name] = true
		rows = append(rows, decisionFilterRow{
			Name:        opt.Name,
			Description: opt.Description,
			Included:    true,
			Missing:     !valid[opt.Name],
		})
	}
	for _, item := range items {
		if !saved[item.Name] {
			rows = append(rows, decisionFilterRow{Name: item.Name})
		}
	}
	return decisionFilterSection{
		Key:          key,
		Enabled:      field.Enabled,
		Instructions: field.Instructions,
		Rows:         rows,
	}
}

// listDecideProviders asks goatkit-llm for its decide providers on behalf of
// the signed-in admin. available is false when the plugin is missing,
// disabled or failing; the page then offers only the active provider.
func listDecideProviders(c *gin.Context) (providers []decisionFilterProvider, available bool) {
	mgr := GetPluginManager()
	if mgr == nil {
		return nil, false
	}
	args := map[string]any{}
	addPluginEnvelope(c, args, decisionFilterLLMPlugin)
	rawArgs, err := json.Marshal(args)
	if err != nil {
		log.Printf("listDecideProviders: encode args: %v", err)
		return nil, false
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), decisionFilterProviderListTimeout)
	defer cancel()
	out, err := mgr.Call(ctx, decisionFilterLLMPlugin, "llm_list_providers", rawArgs)
	if err != nil {
		log.Printf("listDecideProviders: %v", err)
		return nil, false
	}

	var resp struct {
		Error     string `json:"error"`
		Providers []struct {
			ID       json.Number `json:"id"`
			Name     string      `json:"name"`
			Purpose  string      `json:"purpose"`
			Model    string      `json:"model"`
			IsActive any         `json:"is_active"`
		} `json:"providers"`
	}
	if err := json.Unmarshal(out, &resp); err != nil {
		log.Printf("listDecideProviders: decode response: %v", err)
		return nil, false
	}
	if resp.Error != "" {
		log.Printf("listDecideProviders: plugin error: %s", resp.Error)
		return nil, false
	}

	providers = []decisionFilterProvider{}
	for _, p := range resp.Providers {
		if p.Purpose != decisionFilterProviderPurpose {
			continue
		}
		id, err := p.ID.Int64()
		if err != nil || id <= 0 {
			continue
		}
		providers = append(providers, decisionFilterProvider{
			ID:     id,
			Name:   p.Name,
			Model:  p.Model,
			Active: isTruthyFlag(p.IsActive),
		})
	}
	return providers, true
}

// isTruthyFlag reads a boolean column a plugin returned as a JSON bool,
// number or string (drivers differ).
func isTruthyFlag(v any) bool {
	switch t := v.(type) {
	case bool:
		return t
	case float64:
		return t != 0
	case string:
		b, err := strconv.ParseBool(t)
		return err == nil && b
	}
	return false
}
