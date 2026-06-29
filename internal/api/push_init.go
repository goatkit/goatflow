package api

import "github.com/goatkit/goatflow/internal/platform/routing"

func init() {
	routing.RegisterHandler("handleGetVAPIDKey", handleGetVAPIDKey)
	routing.RegisterHandler("handlePushSubscribe", handlePushSubscribe)
	routing.RegisterHandler("handlePushUnsubscribe", handlePushUnsubscribe)
}
