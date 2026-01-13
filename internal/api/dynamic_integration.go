package api

import (
	"database/sql"
	"log"
	"net/http"
	"strings"

	"github.com/flosch/pongo2/v6"
	"github.com/gin-gonic/gin"

	"github.com/gotrs-io/gotrs-ce/internal/components/dynamic"
)

var dynamicHandler *dynamic.DynamicModuleHandler

type dynamicModuleAlias struct {
	HandlerName  string
	FriendlyPath string
}

var dynamicModuleAliases = map[string]dynamicModuleAlias{
	"mail_account": {
		HandlerName:  "handleAdminModuleMailAccounts",
		FriendlyPath: "/admin/mail-accounts",
	},
	// notification_event uses custom handler, not dynamic module system
	"communication_channel": {
		HandlerName:  "handleAdminModuleCommunication",
		FriendlyPath: "/admin/communication-channels",
	},
	"package_repository": {
		HandlerName:  "handleAdminModulePackageRepos",
		FriendlyPath: "/admin/package-repositories",
	},
	"auto_response": {
		HandlerName:  "handleAdminModuleAutoResponses",
		FriendlyPath: "/admin/auto-responses",
	},
	"auto_response_type": {
		HandlerName:  "handleAdminModuleAutoResponseTypes",
		FriendlyPath: "/admin/auto-response-types",
	},
	"follow_up_possible": {
		HandlerName:  "handleAdminModuleFollowUps",
		FriendlyPath: "/admin/follow-up-options",
	},
	"link_state": {
		HandlerName:  "handleAdminModuleLinkStates",
		FriendlyPath: "/admin/link-states",
	},
	"link_type": {
		HandlerName:  "handleAdminModuleLinkTypes",
		FriendlyPath: "/admin/link-types",
	},
	"queue_auto_response": {
		HandlerName:  "handleAdminModuleQueueAutoResponses",
		FriendlyPath: "/admin/queue-auto-responses",
	},
	"article_color": {
		HandlerName:  "handleAdminModuleArticleColors",
		FriendlyPath: "/admin/article-colors",
	},
	"salutation": {
		FriendlyPath: "/admin/modules/salutation",
	},
	"users": {
		FriendlyPath: "/admin/modules/users",
	},
}

func init() {
	registerDynamicModuleHandlers()
}

func friendlyPathForModule(module string) string {
	if alias, ok := dynamicModuleAliases[module]; ok {
		return alias.FriendlyPath
	}
	return ""
}

// SetupDynamicModules initializes the dynamic module system. Route registration
// is handled via YAML; this ensures endpoints exist even before the handler is ready.
func SetupDynamicModules(db *sql.DB) error {
	if getPongo2Renderer() == nil || getPongo2Renderer().TemplateSet() == nil {
		log.Printf("Dynamic modules disabled: template renderer not initialized")
		return nil
	}

	handler, err := dynamic.NewDynamicModuleHandler(db, getPongo2Renderer().TemplateSet(), "modules")
	if err != nil {
		return err
	}

	dynamicHandler = handler
	modules := handler.GetAvailableModules()
	log.Printf("Dynamic Module System loaded with %d modules:", len(modules))
	for _, module := range modules {
		friendly := friendlyPathForModule(module)
		if friendly != "" {
			log.Printf("  - %s (alias %s)", module, friendly)
			continue
		}
		log.Printf("  - %s", module)
	}

	return nil
}

// GetDynamicHandler returns the initialized dynamic handler.
func GetDynamicHandler() *dynamic.DynamicModuleHandler {
	return dynamicHandler
}

func HandleAdminDynamicIndex(c *gin.Context) {
	handleAdminDynamicIndex(c)
}

func HandleAdminDynamicModule(c *gin.Context) {
	handleAdminDynamicModule(c)
}

// HandleAdminDynamicModuleFor returns a handler that forces the module param to a static value.
func HandleAdminDynamicModuleFor(module string) gin.HandlerFunc {
	return func(c *gin.Context) {
		injectModuleParam(c, module)
		handleAdminDynamicModule(c)
	}
}

func injectModuleParam(c *gin.Context, module string) {
	replaced := false
	for i := range c.Params {
		if c.Params[i].Key == "module" {
			c.Params[i].Value = module
			replaced = true
			break
		}
	}
	if !replaced {
		c.Params = append(c.Params, gin.Param{Key: "module", Value: module})
	}
}

func handleAdminDynamicIndex(c *gin.Context) {
	handler := GetDynamicHandler()
	if handler == nil {
		respondDynamicUnavailable(c)
		return
	}

	modules := handler.GetAvailableModules()
	if strings.EqualFold(c.GetHeader("X-Requested-With"), "XMLHttpRequest") || wantsJSONResponse(c) {
		c.JSON(http.StatusOK, gin.H{"success": true, "modules": modules})
		return
	}

	comparisons := make([]map[string]interface{}, 0, len(modules))
	for _, module := range modules {
		friendly := friendlyPathForModule(module)
		comparison := map[string]interface{}{
			"name":         module,
			"static_url":   "/admin/" + module,
			"has_static":   module == "users" || module == "groups" || module == "queues" || module == "priorities",
			"friendly_url": friendly,
			"has_friendly": friendly != "",
		}
		comparisons = append(comparisons, comparison)
	}

	if getPongo2Renderer() == nil || getPongo2Renderer().TemplateSet() == nil {
		c.JSON(http.StatusOK, gin.H{"success": true, "modules": comparisons})
		return
	}

	getPongo2Renderer().HTML(c, http.StatusOK, "pages/admin/dynamic_test.pongo2", pongo2.Context{
		"Modules":    comparisons,
		"User":       getUserMapForTemplate(c),
		"ActivePage": "admin",
		"Title":      "Dynamic Module Testing",
	})
}

func handleAdminDynamicModule(c *gin.Context) {
	handler := GetDynamicHandler()
	if handler == nil {
		respondDynamicUnavailable(c)
		return
	}
	handler.ServeModule(c)
}

func respondDynamicUnavailable(c *gin.Context) {
	msg := "Dynamic module system not initialized"
	if wantsJSONResponse(c) {
		c.JSON(http.StatusServiceUnavailable, gin.H{"success": false, "error": msg})
		return
	}
	c.String(http.StatusServiceUnavailable, msg)
}
