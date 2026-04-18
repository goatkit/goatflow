package api

import "github.com/goatkit/goatflow/internal/routing"

func init() {
	routing.RegisterHandler("HandleAdminExecuteSQL", HandleAdminExecuteSQL)
}
