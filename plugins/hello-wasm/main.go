//go:build tinygo.wasm

// Package main implements a simple "hello" WASM plugin for GoatKit.
// Build with: tinygo build -o hello.wasm -target wasi -no-debug main.go
package main

import (
	"encoding/json"
	"unsafe"
)

// Manifest returned by gk_register - includes i18n translations
var manifestJSON = `{
  "name":"hello-wasm",
  "version":"1.0.0",
  "description":"A simple hello world WASM plugin",
  "author":"GOTRS Team",
  "license":"Apache-2.0",
  "routes":[{"method":"GET","path":"/api/plugins/hello-wasm","handler":"hello","description":"Returns a hello message"}],
  "widgets":[{"id":"hello-wasm-widget","title":"Hello WASM","handler":"widget","location":"dashboard","size":"small","refreshable":true}],
  "i18n":{
    "namespace":"hello_wasm",
    "languages":["en","de","es","fr"],
    "translations":{
      "en":{"greeting":"Hello from WASM","widget_title":"WASM Widget","widget_desc":"This widget runs in a sandboxed WASM module."},
      "de":{"greeting":"Hallo aus WASM","widget_title":"WASM-Widget","widget_desc":"Dieses Widget läuft in einem isolierten WASM-Modul."},
      "es":{"greeting":"Hola desde WASM","widget_title":"Widget WASM","widget_desc":"Este widget se ejecuta en un módulo WASM aislado."},
      "fr":{"greeting":"Bonjour depuis WASM","widget_title":"Widget WASM","widget_desc":"Ce widget s'exécute dans un module WASM isolé."}
    }
  }
}`

//export gk_malloc
func gk_malloc(size uint32) uint32 {
	// Use TinyGo's built-in malloc which allocates in linear memory
	buf := make([]byte, size)
	return uint32(uintptr(unsafe.Pointer(&buf[0])))
}

//export gk_free
func gk_free(ptr uint32) {
	// TinyGo GC will handle this
}

//export gk_register
func gk_register() uint64 {
	// Allocate in linear memory and copy the manifest
	ptr := gk_malloc(uint32(len(manifestJSON)))
	dst := unsafe.Slice((*byte)(unsafe.Pointer(uintptr(ptr))), len(manifestJSON))
	copy(dst, manifestJSON)
	
	// Return packed ptr|len (ptr in high 32 bits, len in low 32 bits)
	return (uint64(ptr) << 32) | uint64(len(manifestJSON))
}

//export gk_call
func gk_call(fnPtr, fnLen, argsPtr, argsLen uint32) uint64 {
	// Read function name from linear memory
	fn := readString(fnPtr, fnLen)
	args := readString(argsPtr, argsLen)

	var result string

	switch fn {
	case "hello":
		result = handleHello(args)
	case "widget":
		result = handleWidget()
	default:
		result = `{"error":"unknown function: ` + fn + `"}`
	}

	// Write result to linear memory
	ptr := gk_malloc(uint32(len(result)))
	dst := unsafe.Slice((*byte)(unsafe.Pointer(uintptr(ptr))), len(result))
	copy(dst, result)

	return (uint64(ptr) << 32) | uint64(len(result))
}

func readString(ptr, length uint32) string {
	if ptr == 0 || length == 0 {
		return ""
	}
	return unsafe.String((*byte)(unsafe.Pointer(uintptr(ptr))), length)
}

func handleHello(argsJSON string) string {
	name := "World"
	
	if argsJSON != "" {
		var args map[string]any
		if err := json.Unmarshal([]byte(argsJSON), &args); err == nil {
			if n, ok := args["name"].(string); ok && n != "" {
				name = n
			}
		}
	}

	result := map[string]any{
		"message": "Hello from WASM, " + name + "!",
		"runtime": "tinygo-wasm",
	}
	data, _ := json.Marshal(result)
	return string(data)
}

func handleWidget() string {
	html := `<div class="hello-wasm-widget"><p class="text-lg font-semibold">🦀 Hello from WASM!</p><p class="text-sm text-gray-500">This widget runs in a sandboxed WASM module.</p></div>`
	result := map[string]string{"html": html}
	data, _ := json.Marshal(result)
	return string(data)
}

func main() {}
