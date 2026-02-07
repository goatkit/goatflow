// Package main provides lookup testing utilities.
package main

import (
	"encoding/json"
	"fmt"
	"log"

	"github.com/goatkit/goatflow/internal/service"
)

func main() {
	fmt.Println("Testing Lookup Service...")

	lookupService := service.NewLookupService()

	// Test English
	fmt.Println("\n=== English (en) ===")
	dataEn := lookupService.GetTicketFormDataWithLang("en")

	fmt.Printf("Statuses (%d):\n", len(dataEn.Statuses))
	for _, s := range dataEn.Statuses {
		fmt.Printf("  - Value: %q, Label: %q, Active: %v\n", s.Value, s.Label, s.Active)
	}

	fmt.Printf("\nPriorities (%d):\n", len(dataEn.Priorities))
	for _, p := range dataEn.Priorities {
		fmt.Printf("  - Value: %q, Label: %q, Active: %v\n", p.Value, p.Label, p.Active)
	}

	fmt.Printf("\nQueues (%d):\n", len(dataEn.Queues))
	for _, q := range dataEn.Queues {
		fmt.Printf("  - ID: %d, Name: %q, Active: %v\n", q.ID, q.Name, q.Active)
	}

	// Test German
	fmt.Println("\n=== German (de) ===")
	dataDe := lookupService.GetTicketFormDataWithLang("de")

	fmt.Printf("Priorities (%d):\n", len(dataDe.Priorities))
	for _, p := range dataDe.Priorities {
		fmt.Printf("  - Value: %q, Label: %q\n", p.Value, p.Label)
	}

	fmt.Printf("\nQueues (%d):\n", len(dataDe.Queues))
	for _, q := range dataDe.Queues {
		fmt.Printf("  - ID: %d, Name: %q\n", q.ID, q.Name)
	}

	// Output as JSON to see the full structure
	fmt.Println("\n=== JSON Output (German) ===")
	jsonBytes, err := json.MarshalIndent(dataDe, "", "  ")
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(string(jsonBytes))
}
