package api

import "github.com/goatkit/goatflow/internal/routing"

func init() {
	// Admin standard attachment list page
	routing.GlobalHandlerMap["handleAdminAttachment"] = handleAdminAttachment

	// Admin standard attachment API endpoints
	routing.GlobalHandlerMap["handleAdminAttachmentCreate"] = handleAdminAttachmentCreate
	routing.GlobalHandlerMap["handleAdminAttachmentUpdate"] = handleAdminAttachmentUpdate
	routing.GlobalHandlerMap["handleAdminAttachmentDelete"] = handleAdminAttachmentDelete
	routing.GlobalHandlerMap["handleAdminAttachmentDownload"] = handleAdminAttachmentDownload
	routing.GlobalHandlerMap["handleAdminAttachmentPreview"] = handleAdminAttachmentPreview
	routing.GlobalHandlerMap["handleAdminAttachmentToggle"] = handleAdminAttachmentToggle
}
