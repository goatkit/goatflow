package api

import "github.com/goatkit/goatflow/internal/routing"

func init() {
	// Register postmaster filter handlers into the global routing registry
	routing.RegisterHandler("handleAdminPostmasterFilters", HandleAdminPostmasterFilters)
	routing.RegisterHandler("handleAdminPostmasterFilterNew", HandleAdminPostmasterFilterNew)
	routing.RegisterHandler("handleAdminPostmasterFilterEdit", HandleAdminPostmasterFilterEdit)
	routing.RegisterHandler("handleAdminPostmasterFilterGet", HandleAdminPostmasterFilterGet)
	routing.RegisterHandler("handleCreatePostmasterFilter", HandleCreatePostmasterFilter)
	routing.RegisterHandler("handleUpdatePostmasterFilter", HandleUpdatePostmasterFilter)
	routing.RegisterHandler("handleDeletePostmasterFilter", HandleDeletePostmasterFilter)
}
