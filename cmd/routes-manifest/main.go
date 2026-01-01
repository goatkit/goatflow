// Package main provides route manifest generation.
package main

import (
	"log"
	"os"

	"github.com/gotrs-io/gotrs-ce/internal/api"
)

func main() {
	// Try full generation (register + write)
	if err := api.GenerateRoutesManifest(); err == nil {
		log.Println("routes manifest generated at generated/routes-manifest.json")
		return
	}
	// Fallback: build JSON and print to stdout (CI can redirect)
	if b, err := api.BuildRoutesManifest(); err == nil {
		os.Stdout.Write(b)
		os.Stdout.Write([]byte("\n"))
		log.Println("(stdout fallback) routes manifest emitted")
		return
	} else {
		log.Printf("failed to build routes manifest fallback: %v", err)
		os.Exit(1)
	}
}
