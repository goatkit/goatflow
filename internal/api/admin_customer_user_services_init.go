package api

import "github.com/goatkit/goatflow/internal/platform/routing"

func init() {
	routing.RegisterHandler("handleAdminDefaultServices", HandleAdminDefaultServices)
	routing.RegisterHandler("handleAdminDefaultServicesUpdate", HandleAdminDefaultServicesUpdate)
}
