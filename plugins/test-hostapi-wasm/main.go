//go:build tinygo.wasm

// Package main implements a test WASM plugin that exercises host API calls.
// This plugin is used by internal/plugin/wasm tests to verify host callbacks work.
package main

import (
	"encoding/json"
	"unsafe"
)

// Host functions imported from the gk module
//
//go:wasmimport gk host_call
func hostCall(fnPtr, fnLen, argsPtr, argsLen uint32) uint64

//go:wasmimport gk log
func hostLog(level uint32, msgPtr, msgLen uint32)

// Log levels
const (
	LogDebug = 0
	LogInfo  = 1
	LogWarn  = 2
	LogError = 3
)

// Manifest
var manifestJSON = `{
  "name":"test-hostapi",
  "version":"1.0.0",
  "description":"Test plugin that exercises host API calls",
  "author":"GOTRS Team",
  "license":"Apache-2.0",
  "routes":[
    {"method":"GET","path":"/api/plugins/test-hostapi/test","handler":"test"}
  ]
}`

//export gk_malloc
func gk_malloc(size uint32) uint32 {
	buf := make([]byte, size)
	return uint32(uintptr(unsafe.Pointer(&buf[0])))
}

//export gk_free
func gk_free(ptr uint32) {}

//export gk_register
func gk_register() uint64 {
	ptr := gk_malloc(uint32(len(manifestJSON)))
	dst := unsafe.Slice((*byte)(unsafe.Pointer(uintptr(ptr))), len(manifestJSON))
	copy(dst, manifestJSON)
	return (uint64(ptr) << 32) | uint64(len(manifestJSON))
}

//export gk_call
func gk_call(fnPtr, fnLen, argsPtr, argsLen uint32) uint64 {
	fn := readString(fnPtr, fnLen)
	var result string
	switch fn {
	case "test":
		result = runTests()
	case "test_log":
		result = testLog()
	default:
		result = `{"error":"unknown"}`
	}
	return writeResult(result)
}

func readString(ptr, length uint32) string {
	if ptr == 0 || length == 0 {
		return ""
	}
	return unsafe.String((*byte)(unsafe.Pointer(uintptr(ptr))), length)
}

func writeResult(s string) uint64 {
	ptr := gk_malloc(uint32(len(s)))
	dst := unsafe.Slice((*byte)(unsafe.Pointer(uintptr(ptr))), len(s))
	copy(dst, s)
	return (uint64(ptr) << 32) | uint64(len(s))
}

func writeString(s string) (uint32, uint32) {
	if len(s) == 0 {
		return 0, 0
	}
	ptr := gk_malloc(uint32(len(s)))
	dst := unsafe.Slice((*byte)(unsafe.Pointer(uintptr(ptr))), len(s))
	copy(dst, s)
	return ptr, uint32(len(s))
}

func log(level uint32, msg string) {
	ptr, length := writeString(msg)
	hostLog(level, ptr, length)
}

func callHost(fn string, args string) string {
	fnPtr, fnLen := writeString(fn)
	argsPtr, argsLen := writeString(args)
	result := hostCall(fnPtr, fnLen, argsPtr, argsLen)
	resultPtr := uint32(result >> 32)
	resultLen := uint32(result & 0xFFFFFFFF)
	if resultPtr == 0 || resultLen == 0 {
		return ""
	}
	return readString(resultPtr, resultLen)
}

func testLog() string {
	log(LogDebug, "Debug from WASM")
	log(LogInfo, "Info from WASM")
	log(LogWarn, "Warn from WASM")
	log(LogError, "Error from WASM")
	data, _ := json.Marshal(map[string]any{"logged": 4})
	return string(data)
}

func runTests() string {
	results := make(map[string]any)

	// Test logging
	log(LogInfo, "Running host API tests")
	results["log"] = true

	// Test db_query
	dbResult := callHost("db_query", `{"query":"SELECT 1","args":[]}`)
	results["db_query"] = dbResult != ""

	// Test db_exec
	dbExecResult := callHost("db_exec", `{"query":"UPDATE test SET x = 1","args":[]}`)
	results["db_exec"] = dbExecResult != ""

	// Test cache_set
	callHost("cache_set", `{"key":"test","value":"dGVzdA==","ttl":60}`)
	results["cache_set"] = true

	// Test cache_get
	cacheResult := callHost("cache_get", `{"key":"test"}`)
	results["cache_get"] = cacheResult != ""

	// Test http_request
	httpResult := callHost("http_request", `{"method":"GET","url":"https://example.com","headers":{}}`)
	results["http_request"] = httpResult != ""

	// Test send_email
	emailResult := callHost("send_email", `{"to":"test@example.com","subject":"Test","body":"Hello","html":false}`)
	results["send_email"] = emailResult != ""

	// Test config_get
	configResult := callHost("config_get", `{"key":"app.name"}`)
	results["config_get"] = configResult != ""

	// Test translate
	translateResult := callHost("translate", `{"key":"hello","args":[]}`)
	results["translate"] = translateResult != ""

	// Test plugin_call
	pluginResult := callHost("plugin_call", `{"plugin":"other","function":"test","args":{}}`)
	results["plugin_call"] = pluginResult != ""

	log(LogInfo, "Tests completed")

	data, _ := json.Marshal(map[string]any{"results": results})
	return string(data)
}

func main() {}
