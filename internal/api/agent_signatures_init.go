package api

import "github.com/gotrs-io/gotrs-ce/internal/routing"

func init() {
	// Agent signature API endpoints
	routing.GlobalHandlerMap["handleGetQueueSignature"] = handleGetQueueSignature
}
