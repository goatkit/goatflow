package api

import "github.com/goatkit/goatflow/internal/routing"

func init() {
	// Admin standard template list page
	routing.GlobalHandlerMap["handleAdminStandardTemplates"] = handleAdminStandardTemplates

	// Admin standard template form pages
	routing.GlobalHandlerMap["handleAdminStandardTemplateNew"] = handleAdminStandardTemplateNew
	routing.GlobalHandlerMap["handleAdminStandardTemplateEdit"] = handleAdminStandardTemplateEdit
	routing.GlobalHandlerMap["handleAdminStandardTemplateQueues"] = handleAdminStandardTemplateQueues
	routing.GlobalHandlerMap["handleAdminStandardTemplateAttachments"] = handleAdminStandardTemplateAttachments
	routing.GlobalHandlerMap["handleAdminStandardTemplateImport"] = handleAdminStandardTemplateImport

	// Admin standard template API endpoints
	routing.GlobalHandlerMap["handleCreateStandardTemplate"] = handleCreateStandardTemplate
	routing.GlobalHandlerMap["handleUpdateStandardTemplate"] = handleUpdateStandardTemplate
	routing.GlobalHandlerMap["handleDeleteStandardTemplate"] = handleDeleteStandardTemplate
	routing.GlobalHandlerMap["handleUpdateStandardTemplateQueues"] = handleUpdateStandardTemplateQueues
	routing.GlobalHandlerMap["handleUpdateStandardTemplateAttachments"] = handleUpdateStandardTemplateAttachments
	routing.GlobalHandlerMap["handleExportStandardTemplate"] = handleExportStandardTemplate
	routing.GlobalHandlerMap["handleExportAllStandardTemplates"] = handleExportAllStandardTemplates
	routing.GlobalHandlerMap["handleImportStandardTemplates"] = handleImportStandardTemplates
}
