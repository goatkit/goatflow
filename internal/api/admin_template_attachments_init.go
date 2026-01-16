package api

import "github.com/gotrs-io/gotrs-ce/internal/routing"

func init() {
	routing.GlobalHandlerMap["handleAdminTemplateAttachments"] = handleAdminTemplateAttachments
	routing.GlobalHandlerMap["handleAdminAttachmentTemplatesEdit"] = handleAdminAttachmentTemplatesEdit
	routing.GlobalHandlerMap["handleUpdateAttachmentTemplates"] = handleUpdateAttachmentTemplates
}
