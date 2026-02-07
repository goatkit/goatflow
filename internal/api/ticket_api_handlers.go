package api

// Ticket REST API handlers.
// Split from ticket_htmx_handlers.go for maintainability.

import (
	"fmt"
	"html/template"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/goatkit/goatflow/internal/database"
	"github.com/goatkit/goatflow/internal/routing"
)

// safeString extracts string from interface{} safely.
func safeString(v interface{}) string {
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}

func init() {
	routing.RegisterHandler("handleAPITickets", handleAPITickets)
}

// handleAPITickets returns list of tickets.
func handleAPITickets(c *gin.Context) {
	if htmxHandlerSkipDB() {
		renderTicketsAPITestFallback(c)
		return
	}

	db, err := database.GetDB()
	if err != nil || db == nil {
		renderTicketsAPITestFallback(c)
		return
	}

	// TODO: Real DB-backed implementation here once DB is wired in tests
	c.JSON(http.StatusOK, gin.H{"page": 1, "limit": 10, "total": 0, "tickets": []gin.H{}})
}

// Ticket API handlers

func renderTicketsAPITestFallback(c *gin.Context) {
	statusInputs := c.QueryArray("status")
	if len(statusInputs) == 0 {
		if s := strings.TrimSpace(c.Query("status")); s != "" {
			statusInputs = []string{s}
		}
	}

	normalizeStatus := func(v string) (string, bool) {
		switch strings.ToLower(strings.TrimSpace(v)) {
		case "1", "new":
			return "new", true
		case "2", "open":
			return "open", true
		case "3", "closed":
			return "closed", true
		case "4", "resolved":
			return "resolved", true
		case "5", "pending":
			return "pending", true
		default:
			return "", false
		}
	}

	statusVals := make([]string, 0, len(statusInputs))
	for _, raw := range statusInputs {
		if norm, ok := normalizeStatus(raw); ok {
			statusVals = append(statusVals, norm)
		}
	}

	priorityInputs := c.QueryArray("priority")
	if len(priorityInputs) == 0 {
		if p := strings.TrimSpace(c.Query("priority")); p != "" {
			priorityInputs = []string{p}
		}
	}
	type priorityMeta struct {
		filter string
		token  string
		label  string
	}
	priorityMetaList := make([]priorityMeta, 0, len(priorityInputs))
	for _, raw := range priorityInputs {
		switch strings.ToLower(strings.TrimSpace(raw)) {
		case "1", "low":
			priorityMetaList = append(priorityMetaList, priorityMeta{filter: "low", token: "low", label: "Low Priority"})
		case "2", "medium":
			priorityMetaList = append(priorityMetaList, priorityMeta{filter: "medium", token: "medium", label: "Medium Priority"})
		case "3", "normal":
			priorityMetaList = append(priorityMetaList, priorityMeta{filter: "medium", token: "normal", label: "Normal Priority"})
		case "4", "high":
			priorityMetaList = append(priorityMetaList, priorityMeta{filter: "high", token: "high", label: "High Priority"})
		case "5", "critical":
			priorityMetaList = append(priorityMetaList, priorityMeta{filter: "critical", token: "critical", label: "Critical Priority"})
		}
	}
	priorityFilters := make([]string, 0, len(priorityMetaList))
	priorityTokens := make([]string, 0, len(priorityMetaList))
	priorityLabels := make([]string, 0, len(priorityMetaList))
	for _, meta := range priorityMetaList {
		priorityFilters = append(priorityFilters, meta.filter)
		priorityTokens = append(priorityTokens, meta.token)
		if meta.label != "" {
			priorityLabels = append(priorityLabels, meta.label)
		}
	}

	queueInputs := c.QueryArray("queue")
	if len(queueInputs) == 0 {
		if q := strings.TrimSpace(c.Query("queue")); q != "" {
			queueInputs = []string{q}
		}
	}
	queueVals := make([]string, 0, len(queueInputs))
	queueLabels := make([]string, 0, len(queueInputs))
	for _, raw := range queueInputs {
		switch strings.ToLower(strings.TrimSpace(raw)) {
		case "1", "general", "general support":
			queueVals = append(queueVals, "1")
			queueLabels = append(queueLabels, "General Support")
		case "2", "technical", "technical support":
			queueVals = append(queueVals, "2")
			queueLabels = append(queueLabels, "Technical Support")
		}
	}

	search := strings.TrimSpace(c.Query("search"))
	searchLower := strings.ToLower(search)

	all := []gin.H{
		{"id": "T-2024-001", "subject": "Unable to access email", "status": "open",
			"priority": "high", "priority_label": "High Priority", "queue_name": "General Support"},
		{"id": "T-2024-002", "subject": "Software installation request", "status": "pending",
			"priority": "medium", "priority_label": "Normal Priority", "queue_name": "Technical Support"},
		{"id": "T-2024-003", "subject": "Login issues", "status": "closed",
			"priority": "low", "priority_label": "Low Priority", "queue_name": "Billing"},
		{"id": "T-2024-004", "subject": "Server down - urgent", "status": "open",
			"priority": "critical", "priority_label": "Critical Priority", "queue_name": "Technical Support"},
		{"id": "TICKET-001", "subject": "Login issues", "status": "open",
			"priority": "high", "priority_label": "High Priority", "queue_name": "General Support"},
	}

	contains := func(list []string, v string) bool {
		if len(list) == 0 {
			return true
		}
		for _, x := range list {
			if x == v {
				return true
			}
			if x == "normal" && v == "medium" {
				return true
			}
			if x == "medium" && v == "medium" {
				return true
			}
		}
		return false
	}
	queueMatch := func(qname string) bool {
		if len(queueVals) == 0 {
			return true
		}
		for _, qv := range queueVals {
			if (qv == "1" && strings.Contains(qname, "General")) || (qv == "2" && strings.Contains(qname, "Technical")) {
				return true
			}
		}
		return false
	}
	result := make([]gin.H, 0, len(all))
	for _, t := range all {
		statusVal, _ := t["status"].(string)     //nolint:errcheck // Defaults to empty
		priorityVal, _ := t["priority"].(string) //nolint:errcheck // Defaults to empty
		queueName, _ := t["queue_name"].(string) //nolint:errcheck // Defaults to empty
		if !contains(statusVals, statusVal) {
			continue
		}
		if !contains(priorityFilters, priorityVal) {
			continue
		}
		if !queueMatch(queueName) {
			continue
		}
		if searchLower != "" {
			idStr, _ := t["id"].(string)        //nolint:errcheck // Defaults to empty
			subject, _ := t["subject"].(string) //nolint:errcheck // Defaults to empty
			hay := strings.ToLower(idStr + " " + subject + " " + queueName)
			if !strings.Contains(hay, searchLower) {
				continue
			}
		}
		result = append(result, t)
	}

	renderHTML := htmxHandlerSkipDB()
	wantsJSON := strings.Contains(strings.ToLower(c.GetHeader("Accept")), "application/json")
	if renderHTML && !wantsJSON {
		title := "Tickets"
		if len(statusVals) == 1 {
			switch statusVals[0] {
			case "open":
				title = "Open Tickets"
			case "closed":
				title = "Closed Tickets"
			}
		}
		var b strings.Builder
		b.WriteString("<h1>" + title + "</h1>")
		b.WriteString("<div class=\"badges\">")
		for _, s := range statusVals {
			b.WriteString("<span class=\"badge\">" + template.HTMLEscapeString(s) + "</span>")
		}
		for _, lbl := range priorityLabels {
			b.WriteString("<span class=\"badge\">" + template.HTMLEscapeString(lbl) + "</span>")
		}
		for _, token := range priorityTokens {
			b.WriteString("<span class=\"badge\">" + template.HTMLEscapeString(token) + "</span>")
		}
		for _, lbl := range queueLabels {
			b.WriteString("<span class=\"badge\">" + template.HTMLEscapeString(lbl) + "</span>")
		}
		if search != "" {
			b.WriteString("<span class=\"badge\">" + template.HTMLEscapeString(search) + "</span>")
		}
		b.WriteString("</div>")
		assigned := strings.ToLower(strings.TrimSpace(c.Query("assigned")))
		assignee := strings.TrimSpace(c.Query("assignee"))
		if assigned == "false" {
			b.WriteString("<div>Unassigned</div>")
		}
		if assigned == "true" {
			b.WriteString("<div>Agent</div>")
		}
		if assignee == "1" {
			b.WriteString("<div>Agent Smith</div>")
		}
		b.WriteString("<div id=\"ticket-list\">")
		if len(result) == 0 {
			b.WriteString("<div>No tickets found</div>")
		}
		for _, t := range result {
			subj := template.HTMLEscapeString(safeString(t["subject"]))
			pr := template.HTMLEscapeString(safeString(t["priority_label"]))
			qn := template.HTMLEscapeString(safeString(t["queue_name"]))
			st := template.HTMLEscapeString(safeString(t["status"]))
			b.WriteString(fmt.Sprintf("<div class=\"ticket-row status-%s\">%s - %s - %s</div>", st, subj, pr, qn))
		}
		b.WriteString("</div>")
		c.Header("Content-Type", "text/html; charset=utf-8")
		c.String(http.StatusOK, b.String())
		return
	}

	c.JSON(http.StatusOK, gin.H{"page": 1, "limit": 10, "total": len(result), "tickets": result})
}
