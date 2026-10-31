package api

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/flosch/pongo2/v6"
	"github.com/gin-gonic/gin"

	"github.com/goatkit/goatflow/internal/platform/audit"
	"github.com/goatkit/goatflow/internal/platform/database"
	"github.com/goatkit/goatflow/internal/platform/sysconfig"
)

const (
	auditLogPageSize     = 50
	auditLogExportMaxRow = 100000
	auditLogDateLayout   = "2006-01-02"
	auditLogFlushEvery   = 500
)

var (
	auditLogCategories = []string{audit.CategoryAuth, audit.CategoryAdmin, audit.CategoryAccess, audit.CategorySystem}
	auditLogOutcomes   = []string{audit.OutcomeSuccess, audit.OutcomeFailure, audit.OutcomeDenied}
)

// auditLogFilter is the viewer and export filter. From and To are whole
// days in the agent's time zone; To is inclusive.
type auditLogFilter struct {
	From, To   time.Time
	Category   string
	Action     string
	Outcome    string
	Actor      string
	TargetType string
	TargetID   string
}

// auditLogRow is one event as the viewer shows it.
type auditLogRow struct {
	ID          int64  `json:"id"`
	Time        string `json:"time"`
	TimeUTC     string `json:"time_utc"`
	Category    string `json:"category"`
	Action      string `json:"action"`
	Outcome     string `json:"outcome"`
	ActorType   string `json:"actor_type"`
	ActorID     int64  `json:"actor_id,omitempty"`
	ActorLogin  string `json:"actor_login"`
	IP          string `json:"ip"`
	UserAgent   string `json:"user_agent"`
	TargetType  string `json:"target_type"`
	TargetID    string `json:"target_id"`
	TargetLabel string `json:"target_label"`
	Details     string `json:"details"`
}

// HandleAdminAuditLog renders the audit log page.
func HandleAdminAuditLog(c *gin.Context) {
	db, err := database.GetDB()
	if err != nil || db == nil {
		sendErrorResponse(c, http.StatusInternalServerError, "Database unavailable")
		return
	}
	if getPongo2Renderer() == nil {
		sendErrorResponse(c, http.StatusInternalServerError, "Template renderer unavailable")
		return
	}
	settings, loadErr := sysconfig.LoadAuditSettings(db)
	if loadErr != nil {
		log.Printf("HandleAdminAuditLog: load settings: %v", loadErr)
	}
	getPongo2Renderer().HTML(c, http.StatusOK, "pages/admin/audit_log.pongo2", pongo2.Context{
		"Title":            "Audit Log",
		"ActivePage":       "admin",
		"User":             getUserMapForTemplate(c),
		"Settings":         settings,
		"Categories":       auditLogCategories,
		"Outcomes":         auditLogOutcomes,
		"MaxRetentionDays": sysconfig.AuditMaxRetentionDays,
		"PageSize":         auditLogPageSize,
		"TimeZone":         auditLogLocation(c).String(),
	})
}

// HandleAdminAuditLogList returns one page of filtered events, newest first.
func HandleAdminAuditLogList(c *gin.Context) {
	loc := auditLogLocation(c)
	f, err := parseAuditLogFilter(c, loc)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": err.Error()})
		return
	}
	page, err := strconv.Atoi(c.DefaultQuery("page", "1"))
	if err != nil || page < 1 {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "page must be a positive number"})
		return
	}
	db, err := database.GetDB()
	if err != nil || db == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "Database unavailable"})
		return
	}
	ctx := c.Request.Context()
	where, args := f.where()
	var total int
	if err := db.QueryRowContext(ctx, database.ConvertPlaceholders(
		"SELECT COUNT(*) FROM gk_audit_event"+where), args...).Scan(&total); err != nil {
		log.Printf("HandleAdminAuditLogList: count: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "Failed to load audit log"})
		return
	}
	events := make([]auditLogRow, 0, auditLogPageSize)
	err = queryAuditLog(ctx, db, f, auditLogPageSize, (page-1)*auditLogPageSize, func(r auditLogRow) error {
		events = append(events, r)
		return nil
	}, loc)
	if err != nil {
		log.Printf("HandleAdminAuditLogList: query: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "Failed to load audit log"})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success":   true,
		"events":    events,
		"total":     total,
		"page":      page,
		"page_size": auditLogPageSize,
	})
}

// HandleAdminAuditLogExport streams the filtered events as CSV and records
// the export.
func HandleAdminAuditLogExport(c *gin.Context) {
	loc := auditLogLocation(c)
	f, err := parseAuditLogFilter(c, loc)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": err.Error()})
		return
	}
	db, err := database.GetDB()
	if err != nil || db == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "Database unavailable"})
		return
	}

	e := audit.FromRequest(c, audit.CategoryAccess, "access.audit.export", audit.OutcomeSuccess)
	e.TargetType = "audit_log"
	e.Details = f.details()
	audit.RecordRequest(c, e)

	c.Header("Content-Type", "text/csv; charset=utf-8")
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=audit_log_%s.csv", time.Now().In(loc).Format("20060102_150405")))
	c.Header("Cache-Control", "no-store")
	c.Status(http.StatusOK)

	w := csv.NewWriter(c.Writer)
	_ = w.Write([]string{"time_utc", "category", "action", "outcome", "actor_type", "actor_id", "actor_login",
		"ip_address", "user_agent", "target_type", "target_id", "target_label", "details"})
	n := 0
	err = queryAuditLog(c.Request.Context(), db, f, auditLogExportMaxRow, 0, func(r auditLogRow) error {
		actorID := ""
		if r.ActorID != 0 {
			actorID = strconv.FormatInt(r.ActorID, 10)
		}
		if err := w.Write(csvSafeRow(r.TimeUTC, r.Category, r.Action, r.Outcome, r.ActorType, actorID, r.ActorLogin,
			r.IP, r.UserAgent, r.TargetType, r.TargetID, r.TargetLabel, compactJSON(r.Details))); err != nil {
			return err
		}
		n++
		if n%auditLogFlushEvery == 0 {
			w.Flush()
			c.Writer.Flush()
		}
		return nil
	}, time.UTC)
	w.Flush()
	if err != nil {
		log.Printf("HandleAdminAuditLogExport: %v", err)
	}
}

// HandleSaveAdminAuditSettings stores retention and read-access settings.
func HandleSaveAdminAuditSettings(c *gin.Context) {
	var cfg sysconfig.AuditSettings
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
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "Database unavailable"})
		return
	}
	before, _ := sysconfig.LoadAuditSettings(db)
	if err := sysconfig.SaveAuditSettings(db, cfg, userID); err != nil {
		var invalid *sysconfig.AuditSettingsValidationError
		if errors.As(err, &invalid) {
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": invalid.Error()})
			return
		}
		log.Printf("HandleSaveAdminAuditSettings: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "Failed to save audit settings"})
		return
	}
	e := audit.FromRequest(c, audit.CategoryAdmin, "admin.audit_settings.update", audit.OutcomeSuccess)
	e.TargetType, e.TargetID = "sysconfig", sysconfig.AuditSettingsKey
	if changes := audit.Changes(auditSettingsFields(before), auditSettingsFields(cfg)); changes != nil {
		e.Details = map[string]any{"changes": changes}
	}
	audit.RecordRequest(c, e)
	c.JSON(http.StatusOK, gin.H{"success": true})
}

func auditSettingsFields(s sysconfig.AuditSettings) map[string]any {
	return map[string]any{"retention_days": s.RetentionDays, "record_read_access": s.RecordReadAccess}
}

// auditLogLocation is the agent's time zone, else the server's.
func auditLogLocation(c *gin.Context) *time.Location {
	if name := agentTimezone(c); name != "" {
		if loc, err := time.LoadLocation(name); err == nil {
			return loc
		}
	}
	return time.Local
}

func parseAuditLogFilter(c *gin.Context, loc *time.Location) (auditLogFilter, error) {
	f := auditLogFilter{
		Category:   strings.TrimSpace(c.Query("category")),
		Action:     strings.TrimSpace(c.Query("action")),
		Outcome:    strings.TrimSpace(c.Query("outcome")),
		Actor:      strings.TrimSpace(c.Query("actor")),
		TargetType: strings.TrimSpace(c.Query("target_type")),
		TargetID:   strings.TrimSpace(c.Query("target_id")),
	}
	if f.Category != "" && !containsString(auditLogCategories, f.Category) {
		return f, fmt.Errorf("unknown category")
	}
	if f.Outcome != "" && !containsString(auditLogOutcomes, f.Outcome) {
		return f, fmt.Errorf("unknown outcome")
	}
	for _, d := range []struct {
		name string
		dst  *time.Time
	}{{"from", &f.From}, {"to", &f.To}} {
		v := strings.TrimSpace(c.Query(d.name))
		if v == "" {
			continue
		}
		t, err := time.ParseInLocation(auditLogDateLayout, v, loc)
		if err != nil {
			return f, fmt.Errorf("%s must be a date (YYYY-MM-DD)", d.name)
		}
		*d.dst = t
	}
	if !f.From.IsZero() && !f.To.IsZero() && f.To.Before(f.From) {
		return f, fmt.Errorf("to must not be before from")
	}
	return f, nil
}

func containsString(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}

// where builds the SQL condition; text filters match case-insensitively.
func (f auditLogFilter) where() (string, []any) {
	conds := make([]string, 0, 8)
	args := make([]any, 0, 8)
	if !f.From.IsZero() {
		conds = append(conds, "create_time >= ?")
		args = append(args, f.From.UTC())
	}
	if !f.To.IsZero() {
		conds = append(conds, "create_time < ?")
		args = append(args, f.To.AddDate(0, 0, 1).UTC())
	}
	if f.Category != "" {
		conds = append(conds, "category = ?")
		args = append(args, f.Category)
	}
	if f.Outcome != "" {
		conds = append(conds, "outcome = ?")
		args = append(args, f.Outcome)
	}
	if f.Action != "" {
		conds = append(conds, "LOWER(action) LIKE ?")
		args = append(args, likeContains(f.Action))
	}
	if f.Actor != "" {
		conds = append(conds, "LOWER(actor_login) LIKE ?")
		args = append(args, likeContains(f.Actor))
	}
	if f.TargetType != "" {
		conds = append(conds, "target_type = ?")
		args = append(args, f.TargetType)
	}
	if f.TargetID != "" {
		conds = append(conds, "target_id = ?")
		args = append(args, f.TargetID)
	}
	if len(conds) == 0 {
		return "", args
	}
	return " WHERE " + strings.Join(conds, " AND "), args
}

func (f auditLogFilter) details() map[string]any {
	d := map[string]any{}
	if !f.From.IsZero() {
		d["from"] = f.From.Format(auditLogDateLayout)
	}
	if !f.To.IsZero() {
		d["to"] = f.To.Format(auditLogDateLayout)
	}
	for k, v := range map[string]string{"category": f.Category, "action": f.Action, "outcome": f.Outcome,
		"actor": f.Actor, "target_type": f.TargetType, "target_id": f.TargetID} {
		if v != "" {
			d[k] = v
		}
	}
	if len(d) == 0 {
		return nil
	}
	return map[string]any{"filters": d}
}

var likeEscaper = strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`)

func likeContains(s string) string {
	return "%" + likeEscaper.Replace(strings.ToLower(s)) + "%"
}

// queryAuditLog calls fn for each matching event, newest first; times are
// formatted in loc.
func queryAuditLog(ctx context.Context, db *sql.DB, f auditLogFilter, limit, offset int, fn func(auditLogRow) error, loc *time.Location) error {
	where, args := f.where()
	args = append(args, limit, offset)
	rows, err := db.QueryContext(ctx, database.ConvertPlaceholders(`
		SELECT id, create_time, category, action, outcome, actor_type, actor_id, actor_login,
			ip_address, user_agent, target_type, target_id, target_label, details
		FROM gk_audit_event`+where+`
		ORDER BY create_time DESC, id DESC
		LIMIT ? OFFSET ?`), args...)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var (
			r                                                  auditLogRow
			created                                            time.Time
			actorID                                            sql.NullInt64
			login, ip, ua, targetType, targetID, label, detail sql.NullString
		)
		if err := rows.Scan(&r.ID, &created, &r.Category, &r.Action, &r.Outcome, &r.ActorType, &actorID, &login,
			&ip, &ua, &targetType, &targetID, &label, &detail); err != nil {
			return err
		}
		// Stored as UTC wall time; drivers may label it with another zone.
		utc := time.Date(created.Year(), created.Month(), created.Day(), created.Hour(), created.Minute(),
			created.Second(), created.Nanosecond(), time.UTC)
		r.TimeUTC = utc.Format(time.RFC3339)
		r.Time = utc.In(loc).Format("2006-01-02 15:04:05 MST")
		r.ActorID = actorID.Int64
		r.ActorLogin, r.IP, r.UserAgent = login.String, ip.String, ua.String
		r.TargetType, r.TargetID, r.TargetLabel = targetType.String, targetID.String, label.String
		r.Details = prettyJSON(detail.String)
		if err := fn(r); err != nil {
			return err
		}
	}
	return rows.Err()
}

func prettyJSON(s string) string {
	if s == "" {
		return ""
	}
	var buf bytes.Buffer
	if err := json.Indent(&buf, []byte(s), "", "  "); err != nil {
		return s
	}
	return buf.String()
}

func compactJSON(s string) string {
	if s == "" {
		return ""
	}
	var buf bytes.Buffer
	if err := json.Compact(&buf, []byte(s)); err != nil {
		return s
	}
	return buf.String()
}

// csvSafeCell stops spreadsheets reading a cell as a formula.
func csvSafeCell(s string) string {
	if s != "" && strings.ContainsRune("=+-@\t\r", rune(s[0])) {
		return "'" + s
	}
	return s
}

func csvSafeRow(cells ...string) []string {
	for i, s := range cells {
		cells[i] = csvSafeCell(s)
	}
	return cells
}
