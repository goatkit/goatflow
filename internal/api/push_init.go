package api

import "github.com/goatkit/goatflow/internal/routing"

func init() {
	routing.RegisterHandler("handleGetVAPIDKey", handleGetVAPIDKey)
	routing.RegisterHandler("handlePushSubscribe", handlePushSubscribe)
	routing.RegisterHandler("handlePushUnsubscribe", handlePushUnsubscribe)
}
