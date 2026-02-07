package api

import "github.com/goatkit/goatflow/internal/routing"

func init() {
	// Agent signature API endpoints
	routing.GlobalHandlerMap["handleGetQueueSignature"] = handleGetQueueSignature
}
