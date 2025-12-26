package api

import "github.com/gotrs-io/gotrs-ce/internal/routing"

func init() {
	routing.SetDynamicFieldLoader(GetDynamicFieldsForScreenGeneric)
}
