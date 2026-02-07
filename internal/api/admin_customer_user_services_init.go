package api

import "github.com/goatkit/goatflow/internal/routing"

func init() {
	routing.RegisterHandler("handleAdminDefaultServices", HandleAdminDefaultServices)
	routing.RegisterHandler("handleAdminDefaultServicesUpdate", HandleAdminDefaultServicesUpdate)
}
