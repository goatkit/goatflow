package api

import "github.com/goatkit/goatflow/internal/platform/routing"

func init() {
	routing.GlobalHandlerMap["handleAdminQueueTemplates"] = handleAdminQueueTemplates
	routing.GlobalHandlerMap["handleAdminQueueTemplatesEdit"] = handleAdminQueueTemplatesEdit
	routing.GlobalHandlerMap["handleUpdateQueueTemplates"] = handleUpdateQueueTemplates
}
