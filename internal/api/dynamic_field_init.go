package api

import "github.com/goatkit/goatflow/internal/platform/routing"

func init() {
	routing.SetDynamicFieldLoader(GetDynamicFieldsForScreenGeneric)
}
