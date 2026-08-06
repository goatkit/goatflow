package api

import (
	"io"
	"net/http"
	"strconv"
	"strings"
	"sync"

	"github.com/flosch/pongo2/v6"
	"github.com/gin-gonic/gin"
	"github.com/goatkit/goatflow/internal/models"
	"github.com/goatkit/goatflow/internal/platform/database"
	"github.com/goatkit/goatflow/internal/platform/lookups"
	"github.com/goatkit/goatflow/internal/platform/routing"
	"github.com/goatkit/goatflow/internal/repository"
	"github.com/goatkit/goatflow/internal/service"
)

// setupAssistantSvc is the lazily-constructed service singleton, built on first
// use from the live DB and plugin manager — same accessor pattern as
// database.GetDB / GetPluginManager.
var (
	setupAssistantSvc     *service.SetupAssistantService
	setupAssistantSvcOnce sync.Once
)

// getSetupAssistantService returns the shared service, constructing it on first
// call. Returns nil only if the DB is not yet available; callers must guard.
func getSetupAssistantService() *service.SetupAssistantService {
	setupAssistantSvcOnce.Do(func() {
		if db, err := database.GetDB(); err == nil && db != nil {
			setupAssistantSvc = service.NewSetupAssistantService(db, GetPluginManager())
		}
	})
	if setupAssistantSvc != nil {
		return setupAssistantSvc
	}
	// Fallback if the first Once call ran before the DB was ready.
	if db, err := database.GetDB(); err == nil && db != nil {
		setupAssistantSvc = service.NewSetupAssistantService(db, GetPluginManager())
	}
	return setupAssistantSvc
}

// currentCreateBy returns the authenticated admin's user id for audit fields,
// falling back to the system user (1) when no identity is present.
func currentCreateBy(c *gin.Context) int {
	if v, ok := c.Get("user_id"); ok {
		switch n := v.(type) {
		case int:
			if n > 0 {
				return n
			}
		case int64:
			return int(n)
		case float64:
			return int(n)
		case uint:
			return int(n)
		}
	}
	if userCtx, ok := c.Get("user"); ok {
		if u, ok := userCtx.(*models.User); ok && u != nil && u.ID > 0 {
			return int(u.ID)
		}
	}
	return 1
}

func init() {
	routing.RegisterHandler("handleAdminSetupWizard", handleAdminSetupWizard)
	routing.RegisterHandler("handleAdminSetupWizardSubmit", handleAdminSetupWizardSubmit)
	routing.RegisterHandler("handleAdminSetupAssistant", handleAdminSetupAssistant)
	routing.RegisterHandler("handleAdminSetupTask", handleAdminSetupTask)
	routing.RegisterHandler("handleAdminSetupCustomerSearch", handleAdminSetupCustomerSearch)
	routing.RegisterHandler("handleAdminSetupCustomerConfig", handleAdminSetupCustomerConfig)
}

// handleAdminSetupWizard renders the first-run setup wizard (Mode 1). If setup
// is already complete, redirect to the re-runnable assistant instead.
func handleAdminSetupWizard(c *gin.Context) {
	svc := getSetupAssistantService()
	if svc == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"success": false, "error": "System unavailable"})
		return
	}

	// If existing_customer_id query param is present, load that customer for review
	existingID := strings.TrimSpace(c.Query("existing_customer_id"))
	if existingID != "" {
		config, err := svc.LoadExistingCustomer(c.Request.Context(), existingID)
		if err != nil {
			c.Redirect(http.StatusSeeOther, "/admin/setup/assistant")
			return
		}
		ctx := pongo2Context(c, nil)
		ctx["ExistingCustomerID"] = existingID
		ctx["ReviewMode"] = true
		ctx["CustomerConfig"] = config
		if getPongo2Renderer() == nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"success": false, "error": "Renderer unavailable"})
			return
		}
		getPongo2Renderer().HTML(c, http.StatusOK, "pages/admin/setup_wizard.pongo2", ctx)
		return
	}

	snap, _ := svc.Recce(c.Request.Context())
	if snap != nil && snap.SetupCompleted {
		c.Redirect(http.StatusSeeOther, "/admin/setup/assistant")
		return
	}
	if getPongo2Renderer() == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"success": false, "error": "Renderer unavailable"})
		return
	}
	getPongo2Renderer().HTML(c, http.StatusOK, "pages/admin/setup_wizard.pongo2", pongo2Context(c, snap))
}

// handleAdminSetupWizardSubmit processes the wizard submission. The wizard
// front-end collects every step into one JSON payload and posts it here; on
// success the service creates all entities, marks setup complete, and the
// handler tells the client to redirect to the dashboard.
func handleAdminSetupWizardSubmit(c *gin.Context) {
	svc := getSetupAssistantService()
	if svc == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"success": false, "error": "System unavailable"})
		return
	}

	var req service.WizardRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "Invalid request: " + err.Error()})
		return
	}
	req.CreateBy = currentCreateBy(c)

	result := svc.ExecuteWizard(c.Request.Context(), req)
	if !result.Success {
		c.JSON(http.StatusOK, gin.H{"success": false, "error": result.Error, "created": result.Created})
		return
	}

	// First-run complete: dismiss future dashboard redirects.
	_ = svc.MarkSetupComplete(c.Request.Context())

	c.JSON(http.StatusOK, gin.H{
		"success":  true,
		"created":  result.Created,
		"redirect": "/admin",
	})
}

// handleAdminSetupAssistant renders the re-runnable task catalog (Mode 2) with
// the system recce and the combined core + plugin task list.
func handleAdminSetupAssistant(c *gin.Context) {
	svc := getSetupAssistantService()
	if svc == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"success": false, "error": "System unavailable"})
		return
	}
	if getPongo2Renderer() == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"success": false, "error": "Renderer unavailable"})
		return
	}

	snap, _ := svc.Recce(c.Request.Context())
	ctx := pongo2Context(c, snap)
	ctx["Tasks"] = svc.GetAllTasks()
	ctx["Title"] = "Setup Assistant"
	ctx["ActiveAdminPage"] = "setup-assistant"
	getPongo2Renderer().HTML(c, http.StatusOK, "pages/admin/setup_assistant.pongo2", ctx)
}

// handleAdminSetupTask dispatches a single catalog task. Core tasks
// (plugin == "setup-assistant") render an inline mini-form and act on POST;
// plugin tasks are delegated to the plugin handler via the plugin manager.

// handleAdminSetupCustomerSearch handles AJAX requests for customer dropdown type-ahead search.
// Supports filtering by name and customer_id for the dropdown with auto-completion.
func handleAdminSetupCustomerSearch(c *gin.Context) {
	query := strings.TrimSpace(c.Param("query"))
	if query == "" {
		query = strings.TrimSpace(c.Query("q"))
	}
	if query == "" {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "Query parameter is required"})
		return
	}
	likeQuery := "%" + query + "%"

	db, err := database.GetDB()
	if err != nil || db == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"success": false, "error": "Database unavailable"})
		return
	}

	// Search customer companies by name or customer_id
	rows, err := db.QueryContext(c.Request.Context(),
		"SELECT customer_id, name FROM customer_company WHERE name LIKE ? OR customer_id LIKE ? LIMIT 50",
		likeQuery, likeQuery)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "Database error: " + err.Error()})
		return
	}
	defer rows.Close()

	type CustomerOption struct {
		Value string `json:"value"`
		Label string `json:"label"`
	}

	var options []CustomerOption
	for rows.Next() {
		var customerID, name string
		if err := rows.Scan(&customerID, &name); err != nil {
			continue
		}
		options = append(options, CustomerOption{
			Value: customerID,
			Label: name + " (" + customerID + ")",
		})
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "data": options})
}

// handleAdminSetupCustomerConfig returns the full configuration for an existing
// customer as JSON, used by the onboarding form to auto-populate all fields.
func handleAdminSetupCustomerConfig(c *gin.Context) {
	svc := getSetupAssistantService()
	if svc == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"success": false, "error": "System unavailable"})
		return
	}
	customerID := strings.TrimSpace(c.Param("customer_id"))
	if customerID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "customer_id is required"})
		return
	}
	config, err := svc.LoadExistingCustomer(c.Request.Context(), customerID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": config})
}

func handleAdminSetupTask(c *gin.Context) {
	svc := getSetupAssistantService()
	if svc == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"success": false, "error": "System unavailable"})
		return
	}

	pluginName := c.Param("plugin")
	taskID := c.Param("task_id")

	if pluginName == "setup-assistant" || pluginName == "" {
		handleCoreSetupTask(c, svc, taskID)
		return
	}

	// Plugin task: forward the request body to the plugin handler.
	if c.Request.Method == http.MethodPost {
		body, err := io.ReadAll(c.Request.Body)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "Could not read request body"})
			return
		}
		out, err := svc.CallPluginTask(c.Request.Context(), pluginName, taskID, body)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
			return
		}
		c.Data(http.StatusOK, "application/json", out)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "plugin": pluginName, "task": taskID})
}

// handleCoreSetupTask renders (GET) or executes (POST) a built-in task. GET
// renders setup_task_form.pongo2 with the task spec and, for tasks that need
// group selection, the existing groups as checkboxes.
func handleCoreSetupTask(c *gin.Context, svc *service.SetupAssistantService, taskID string) {
	// Resolve the core task spec for the template header.
	var spec service.PluginSetupTask
	for _, t := range svc.GetCoreTasks() {
		if t.ID == taskID {
			spec = t
			break
		}
	}

	if c.Request.Method == http.MethodPost {
		handleCoreSetupTaskSubmit(c, svc, taskID)
		return
	}

	if getPongo2Renderer() == nil || spec.ID == "" {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "error": "Unknown setup task"})
		return
	}

	ctx := pongo2Context(c, nil)
	ctx["Task"] = spec
	ctx["Title"] = spec.Title

	// Customer onboarding is a multi-step wizard with its own template.
	if taskID == "create_customer" {
		ctx["Countries"] = lookups.Countries()
		ctx["SLAs"] = listActiveSLAs()
		ctx["Groups"] = listActiveGroups()
		ctx["Queues"] = listActiveQueues()
		ctx["Companies"] = listExistingCompanies()
		getPongo2Renderer().HTML(c, http.StatusOK, "pages/admin/onboard_customer.pongo2", ctx)
		return
	}

	ctx["NeedsGroups"] = taskID == "create_queue" || taskID == "create_agent" || taskID == "assign_queue_group" || taskID == "assign_agent_group"
	ctx["Groups"] = listActiveGroups()
	ctx["Agents"] = listActiveAgents()
	ctx["Queues"] = listActiveQueues()
	if taskID == "mark_complete" {
		ctx["IsMarkComplete"] = true
	}
	getPongo2Renderer().HTML(c, http.StatusOK, "pages/admin/setup_task_form.pongo2", ctx)
}

// handleCoreSetupTaskSubmit executes a built-in task from form data.
func handleCoreSetupTaskSubmit(c *gin.Context, svc *service.SetupAssistantService, taskID string) {
	createBy := currentCreateBy(c)
	ctx := c.Request.Context()

	switch taskID {
	case "create_group":
		name := strings.TrimSpace(c.PostForm("name"))
		id, err := svc.CreateGroup(ctx, name, c.PostForm("comments"), createBy)
		respondCore(c, err, "group", name, id, "/admin/setup/assistant")
		return
	case "create_queue":
		id, err := svc.CreateQueue(ctx, strings.TrimSpace(c.PostForm("name")), postFormInts(c, "group_ids"), c.PostForm("comments"), createBy)
		respondCore(c, err, "queue", c.PostForm("name"), id, "/admin/setup/assistant")
		return
	case "create_agent":
		id, err := svc.CreateAgent(ctx,
			strings.TrimSpace(c.PostForm("login")),
			c.PostForm("first_name"), c.PostForm("last_name"), c.PostForm("email"),
			postFormInts(c, "group_ids"), createBy)
		respondCore(c, err, "agent", c.PostForm("login"), id, "/admin/setup/assistant")
		return
	case "create_customer":
		var req service.OnboardCustomerRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "Invalid request: " + err.Error()})
			return
		}
		result := svc.OnboardCustomer(ctx, req)
		status := http.StatusOK
		if !result.Success {
			status = http.StatusUnprocessableEntity
		}
		c.JSON(status, gin.H{"success": result.Success, "data": result})
		return
	case "create_sla":
		id, err := svc.CreateSLA(ctx,
			strings.TrimSpace(c.PostForm("name")),
			postFormInt(c, "first_response_time", 0),
			postFormInt(c, "solution_time", 0), createBy)
		respondCore(c, err, "sla", c.PostForm("name"), id, "/admin/setup/assistant")
		return
	case "create_response_template":
		var req []service.ResponseTemplateInput
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "Invalid request: " + err.Error()})
			return
		}
		if err := svc.CreateCannedResponses(ctx, req, createBy); err != nil {
			c.JSON(http.StatusOK, gin.H{"success": false, "error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"success": true})
		return

	case "configure_business_hours":
		var req service.BusinessHoursInput
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "Invalid request: " + err.Error()})
			return
		}
		if err := svc.CreateBusinessHours(ctx, req, createBy); err != nil {
			c.JSON(http.StatusOK, gin.H{"success": false, "error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"success": true})
		return

	case "configure_email_transport":
		var req service.EmailTransportInput
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "Invalid request: " + err.Error()})
			return
		}
		if err := svc.CreateEmailTransport(ctx, req, createBy); err != nil {
			c.JSON(http.StatusOK, gin.H{"success": false, "error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"success": true})
		return

	case "assign_agent_group":
		userID := postFormInt(c, "agent_id", 0)
		groupIDs := postFormInts(c, "group_ids")
		if userID <= 0 {
			c.JSON(http.StatusOK, gin.H{"success": false, "error": "Select an agent"})
			return
		}
		if len(groupIDs) == 0 {
			c.JSON(http.StatusOK, gin.H{"success": false, "error": "Select at least one team"})
			return
		}
		if err := svc.AssignAgentToGroups(ctx, userID, groupIDs, createBy); err != nil {
			c.JSON(http.StatusOK, gin.H{"success": false, "error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"success": true, "redirect": "/admin/setup/assistant"})
		return
	case "assign_queue_group":
		queueID := postFormInt(c, "queue_id", 0)
		groupID := postFormInt(c, "group_id", 0)
		if queueID <= 0 {
			c.JSON(http.StatusOK, gin.H{"success": false, "error": "Select a queue"})
			return
		}
		if groupID <= 0 {
			c.JSON(http.StatusOK, gin.H{"success": false, "error": "Select a team"})
			return
		}
		if err := svc.AssignQueueToGroup(ctx, queueID, groupID, createBy); err != nil {
			c.JSON(http.StatusOK, gin.H{"success": false, "error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"success": true, "redirect": "/admin/setup/assistant"})
		return
	case "mark_complete":
		if err := svc.MarkSetupComplete(ctx); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"success": true, "redirect": "/admin"})
		return
	}
	c.JSON(http.StatusNotFound, gin.H{"success": false, "error": "Unknown setup task"})
}

// respondCore is the shared success/error shape for core task submissions.
// Front-end JS follows `redirect` on success; on error it shows `error` inline.
func respondCore(c *gin.Context, err error, kind, name string, id int, redirect string) {
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success":  true,
		"created":  service.CreatedEntity{Kind: kind, Name: name, ID: id},
		"redirect": redirect,
	})
}

// listActiveGroups returns id/name pairs for active groups, for task forms that
// need group selection. Uses the repository (no raw SQL in handlers).
func listActiveGroups() []models.Group {
	db, err := database.GetDB()
	if err != nil || db == nil {
		return nil
	}
	groups, err := repository.NewGroupRepository(db).List()
	if err != nil {
		return nil
	}
	out := make([]models.Group, 0, len(groups))
	for _, g := range groups {
		if g == nil {
			continue
		}
		out = append(out, *g)
	}
	return out
}

// slaOption is a minimal {id, name} view for SLA dropdowns in setup forms.
type slaOption struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

// listActiveSLAs returns id/name pairs for active SLAs, for the onboarding
// wizard's Service/SLA step. Uses a parameterised query (no user input).
func listActiveSLAs() []slaOption {
	db, err := database.GetDB()
	if err != nil || db == nil {
		return nil
	}
	rows, err := db.Query(database.ConvertPlaceholders(
		"SELECT id, name FROM sla WHERE valid_id = 1 ORDER BY name"))
	if err != nil {
		return nil
	}
	defer rows.Close()
	out := make([]slaOption, 0)
	for rows.Next() {
		var o slaOption
		if err := rows.Scan(&o.ID, &o.Name); err == nil {
			out = append(out, o)
		}
	}
	return out
}

// queueOption is a minimal {id, name} view for queue dropdowns in setup forms.
type queueOption struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

// agentOption is a minimal {id, login} view for agent dropdowns in setup forms.
type agentOption struct {
	ID    int    `json:"id"`
	Login string `json:"login"`
}

// listActiveAgents returns id/login pairs for active agents, for the
// assign-agent-to-team setup task form's agent dropdown.
func listActiveAgents() []agentOption {
	db, err := database.GetDB()
	if err != nil || db == nil {
		return nil
	}
	rows, err := db.Query(database.ConvertPlaceholders(
		"SELECT id, login FROM users WHERE valid_id = 1 ORDER BY login"))
	if err != nil {
		return nil
	}
	defer rows.Close()
	out := make([]agentOption, 0)
	for rows.Next() {
		var a agentOption
		if err := rows.Scan(&a.ID, &a.Login); err == nil {
			out = append(out, a)
		}
	}
	return out
}

// listActiveQueues returns id/name pairs for active queues, for the mail-account
// destination dropdown in the onboarding wizard.
func listActiveQueues() []queueOption {
	db, err := database.GetDB()
	if err != nil || db == nil {
		return nil
	}
	rows, err := db.Query(database.ConvertPlaceholders(
		"SELECT id, name FROM queue WHERE valid_id = 1 ORDER BY name"))
	if err != nil {
		return nil
	}
	defer rows.Close()
	out := make([]queueOption, 0)
	for rows.Next() {
		var q queueOption
		if err := rows.Scan(&q.ID, &q.Name); err == nil {
			out = append(out, q)
		}
	}
	return out
}

// companyOption is a minimal customer_company row for the autocomplete seed.
type companyOption struct {
	CustomerID string `json:"customer_id"`
	Name       string `json:"name"`
}

// listExistingCompanies returns all active customer companies for the
// onboarding form's company-name autocomplete (seed data for data-gk-autocomplete).
func listExistingCompanies() []companyOption {
	db, err := database.GetDB()
	if err != nil || db == nil {
		return nil
	}
	rows, err := db.Query(database.ConvertPlaceholders(
		"SELECT customer_id, name FROM customer_company WHERE valid_id = 1 ORDER BY name"))
	if err != nil {
		return nil
	}
	defer rows.Close()
	out := make([]companyOption, 0)
	for rows.Next() {
		var c companyOption
		if err := rows.Scan(&c.CustomerID, &c.Name); err == nil {
			out = append(out, c)
		}
	}
	return out
}

// postFormInts parses a repeated/comma-separated form field into []int.
func postFormInts(c *gin.Context, field string) []int {
	raw := c.PostFormArray(field)
	if len(raw) == 0 {
		if single := strings.TrimSpace(c.PostForm(field)); single != "" {
			raw = strings.Split(single, ",")
		}
	}
	out := make([]int, 0, len(raw))
	for _, r := range raw {
		r = strings.TrimSpace(r)
		if r == "" {
			continue
		}
		if n, err := strconv.Atoi(r); err == nil && n > 0 {
			out = append(out, n)
		}
	}
	return out
}

// postFormInt parses a single form field as int, returning def on miss/error.
func postFormInt(c *gin.Context, field string, def int) int {
	raw := strings.TrimSpace(c.PostForm(field))
	if raw == "" {
		return def
	}
	n, err := strconv.Atoi(raw)
	if err != nil {
		return def
	}
	return n
}

// pongo2Context builds the common template context for setup pages.
func pongo2Context(c *gin.Context, snap *service.SystemSnapshot) pongo2.Context {
	ctx := pongo2.Context{
		"ActivePage": "admin",
		"User":       getUserMapForTemplate(c),
	}
	if snap != nil {
		ctx["Snapshot"] = snap
	}
	return ctx
}
