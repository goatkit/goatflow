package api

import "github.com/goatkit/goatflow/internal/platform/routing"

func init() {
	// Agent template API endpoints
	routing.GlobalHandlerMap["handleGetAgentTemplates"] = handleGetAgentTemplates
	routing.GlobalHandlerMap["handleGetAgentTemplate"] = handleGetAgentTemplate
}
