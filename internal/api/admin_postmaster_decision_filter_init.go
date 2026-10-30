package api

import "github.com/goatkit/goatflow/internal/platform/routing"

func init() {
	// Register decision filter settings handlers into the global routing registry
	routing.RegisterHandler("handleAdminPostmasterDecisionFilter", HandleAdminPostmasterDecisionFilter)
	routing.RegisterHandler("handleSaveAdminPostmasterDecisionFilter", HandleSaveAdminPostmasterDecisionFilter)
}
