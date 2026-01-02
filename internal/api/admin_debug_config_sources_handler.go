package api

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/gotrs-io/gotrs-ce/internal/yamlmgmt"
)

// HandleDebugConfigSources lists configuration settings with their source.
func HandleDebugConfigSources(c *gin.Context) {
	vm := yamlmgmt.GetVersionManager()
	if vm == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"success": false, "error": "config version manager not initialized"})
		return
	}
	adapter := yamlmgmt.NewConfigAdapter(vm)
	settings, err := adapter.GetConfigSettings()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
		return
	}
	result := make([]gin.H, 0, len(settings))
	for _, s := range settings {
		name, _ := s["name"].(string) //nolint:errcheck // Type assertion defaults to empty
		defVal := s["default"]
		val, hasVal := s["value"]
		effective := defVal
		source := "default"
		if hasVal && val != nil { // empty string is intentional override
			effective = val
			source = "value"
		}
		result = append(result, gin.H{
			"name":      name,
			"default":   defVal,
			"value":     val,
			"effective": effective,
			"source":    source,
		})
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": result})
}
