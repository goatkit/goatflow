package api

import "github.com/goatkit/goatflow/internal/routing"

func init() {
	// Register MCP handlers into the global routing registry
	routing.RegisterHandler("HandleMCP", HandleMCP)
	routing.RegisterHandler("HandleMCPInfo", HandleMCPInfo)
}
