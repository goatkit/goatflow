package api

import (
	"io"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/goatkit/goatflow/internal/platform/routing"
	"github.com/goatkit/goatflow/internal/service"
)

func init() {
	routing.RegisterHandler("HandleAPISetupRecce", HandleAPISetupRecce)
	routing.RegisterHandler("HandleAPISetupWizard", HandleAPISetupWizard)
	routing.RegisterHandler("HandleAPISetupTasks", HandleAPISetupTasks)
	routing.RegisterHandler("HandleAPISetupTask", HandleAPISetupTask)
	routing.RegisterHandler("HandleAPISetupOnboardCustomer", HandleAPISetupOnboardCustomer)
}

// HandleAPISetupRecce returns the system snapshot (entity counts + setup
// status) as JSON. Lets an LLM or script understand current state.
// GET /api/v1/admin/setup/recce
func HandleAPISetupRecce(c *gin.Context) {
	svc := getSetupAssistantService()
	if svc == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"success": false, "error": "System unavailable"})
		return
	}
	snap, err := svc.Recce(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": snap})
}

// HandleAPISetupOnboardCustomer provisions a full customer setup (company +
// portal users with generated passwords) from one JSON payload. Lets an LLM
// or script onboard a customer end-to-end without the wizard UI.
// POST /api/v1/admin/setup/onboard-customer
func HandleAPISetupOnboardCustomer(c *gin.Context) {
	svc := getSetupAssistantService()
	if svc == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"success": false, "error": "System unavailable"})
		return
	}
	var req service.OnboardCustomerRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "Invalid request: " + err.Error()})
		return
	}
	result := svc.OnboardCustomer(c.Request.Context(), req)
	status := http.StatusOK
	if !result.Success {
		status = http.StatusUnprocessableEntity
	}
	c.JSON(status, gin.H{"success": result.Success, "data": result})
}

// HandleAPISetupWizard accepts a full WizardRequest JSON payload and creates
// every entity in dependency order. Does NOT auto-mark setup complete — the
// caller (UI or LLM) decides by calling the mark-complete task afterwards.
// POST /api/v1/admin/setup/wizard
func HandleAPISetupWizard(c *gin.Context) {
	svc := getSetupAssistantService()
	if svc == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"success": false, "error": "System unavailable"})
		return
	}
	var req struct {
		service.WizardRequest
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "Invalid request: " + err.Error()})
		return
	}
	req.CreateBy = currentCreateBy(c)
	result := svc.ExecuteWizard(c.Request.Context(), req.WizardRequest)
	status := http.StatusOK
	if !result.Success {
		status = http.StatusUnprocessableEntity
	}
	c.JSON(status, gin.H{"success": result.Success, "data": result})
}

// HandleAPISetupTasks returns the combined core + plugin task catalog.
// GET /api/v1/admin/setup/tasks
func HandleAPISetupTasks(c *gin.Context) {
	svc := getSetupAssistantService()
	if svc == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"success": false, "error": "System unavailable"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": svc.GetAllTasks()})
}

// HandleAPISetupTask dispatches a plugin setup task, forwarding the JSON body
// to the plugin handler. Core tasks use the HTML route (handleAdminSetupTask).
// POST /api/v1/admin/setup/tasks/:plugin/:task_id
func HandleAPISetupTask(c *gin.Context) {
	svc := getSetupAssistantService()
	if svc == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"success": false, "error": "System unavailable"})
		return
	}
	pluginName := c.Param("plugin")
	taskID := c.Param("task_id")

	// Mark-complete is exposed here too for programmatic callers.
	if pluginName == "setup-assistant" && taskID == "mark_complete" {
		if err := svc.MarkSetupComplete(c.Request.Context()); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"success": true})
		return
	}

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
}
