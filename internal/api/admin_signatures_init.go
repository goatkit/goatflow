package api

import "github.com/goatkit/goatflow/internal/routing"

func init() {
	routing.RegisterHandler("handleAdminSignatures", handleAdminSignatures)
	routing.RegisterHandler("handleAdminSignatureNew", handleAdminSignatureNew)
	routing.RegisterHandler("handleAdminSignatureEdit", handleAdminSignatureEdit)
	routing.RegisterHandler("handleCreateSignature", handleCreateSignature)
	routing.RegisterHandler("handleUpdateSignature", handleUpdateSignature)
	routing.RegisterHandler("handleDeleteSignature", handleDeleteSignature)
	routing.RegisterHandler("handleExportSignature", handleExportSignature)
	routing.RegisterHandler("handleExportSignatures", handleExportSignatures)
	routing.RegisterHandler("handleImportSignatures", handleImportSignatures)
}
