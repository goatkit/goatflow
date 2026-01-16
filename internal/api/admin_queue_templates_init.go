package api

import "github.com/gotrs-io/gotrs-ce/internal/routing"

func init() {
	routing.GlobalHandlerMap["handleAdminQueueTemplates"] = handleAdminQueueTemplates
	routing.GlobalHandlerMap["handleAdminQueueTemplatesEdit"] = handleAdminQueueTemplatesEdit
	routing.GlobalHandlerMap["handleUpdateQueueTemplates"] = handleUpdateQueueTemplates
}
