package api

import "github.com/goatkit/goatflow/internal/routing"

func init() {
	routing.GlobalHandlerMap["handleAdminTemplateAttachments"] = handleAdminTemplateAttachments
	routing.GlobalHandlerMap["handleAdminAttachmentTemplatesEdit"] = handleAdminAttachmentTemplatesEdit
	routing.GlobalHandlerMap["handleUpdateAttachmentTemplates"] = handleUpdateAttachmentTemplates
}
