// Package main provides a tool for adding ticket translations.
package main

import (
	"encoding/json"
	"fmt"
	"os"
)

func main() {
	// Read existing English translations
	enFile := "./internal/i18n/translations/en.json"
	data, err := os.ReadFile(enFile)
	if err != nil {
		fmt.Printf("Error reading file: %v\n", err)
		return
	}

	var translations map[string]interface{}
	if err := json.Unmarshal(data, &translations); err != nil {
		fmt.Printf("Error parsing JSON: %v\n", err)
		return
	}

	// Ensure agent section exists
	if translations["agent"] == nil {
		translations["agent"] = make(map[string]interface{})
	}
	agent := translations["agent"].(map[string]interface{})

	// Add ticket section under agent
	agent["ticket"] = map[string]interface{}{
		"zoom": map[string]interface{}{
			"title": "Ticket Zoom",
		},
	}

	// Update tickets section with all needed keys
	if translations["tickets"] == nil {
		translations["tickets"] = make(map[string]interface{})
	}
	tickets := translations["tickets"].(map[string]interface{})

	// Add all the ticket action keys
	tickets["reply"] = "Reply"
	tickets["reply_all"] = "Reply All"
	tickets["forward"] = "Forward"
	tickets["phone"] = "Phone"
	tickets["note"] = "Note"
	tickets["actions"] = "Actions"
	tickets["owner"] = "Owner"
	tickets["responsible"] = "Responsible"
	tickets["customer"] = "Customer"
	tickets["state"] = "State"
	tickets["type"] = "Type"
	tickets["service"] = "Service"
	tickets["sla"] = "SLA"
	tickets["lock"] = "Lock"
	tickets["unlock"] = "Unlock"
	tickets["locked"] = "Locked"
	tickets["unlocked"] = "Unlocked"
	tickets["history"] = "History"
	tickets["print"] = "Print"
	tickets["link"] = "Link"
	tickets["miscellaneous"] = "Misc"
	tickets["watch"] = "Watch"
	tickets["bulk"] = "Bulk"
	tickets["spam"] = "Spam"
	tickets["articles"] = "Articles"
	tickets["details"] = "Details"
	tickets["to"] = "To"
	tickets["email"] = "Email"
	tickets["internal_note"] = "Internal Note"
	tickets["phone_call"] = "Phone Call"
	tickets["customer_info"] = "Customer Information"
	tickets["linked_tickets"] = "Linked Tickets"

	// Add customer keys
	if translations["customer"] == nil {
		translations["customer"] = make(map[string]interface{})
	}
	customer := translations["customer"].(map[string]interface{})
	customer["name"] = "Name"
	customer["email"] = "Email"
	customer["phone"] = "Phone"
	customer["company"] = "Company"

	// Write back the updated translations
	output, err := json.MarshalIndent(translations, "", "  ")
	if err != nil {
		fmt.Printf("Error marshaling JSON: %v\n", err)
		return
	}

	if err := os.WriteFile(enFile, output, 0644); err != nil {
		fmt.Printf("Error writing file: %v\n", err)
		return
	}

	fmt.Println("Successfully added ticket translations to en.json")

	// Now do the same for German
	deFile := "./internal/i18n/translations/de.json"
	data, err = os.ReadFile(deFile)
	if err != nil {
		fmt.Printf("Error reading German file: %v\n", err)
		return
	}

	var deTranslations map[string]interface{}
	if err := json.Unmarshal(data, &deTranslations); err != nil {
		fmt.Printf("Error parsing German JSON: %v\n", err)
		return
	}

	// Ensure agent section exists
	if deTranslations["agent"] == nil {
		deTranslations["agent"] = make(map[string]interface{})
	}
	deAgent := deTranslations["agent"].(map[string]interface{})

	// Add ticket section under agent
	deAgent["ticket"] = map[string]interface{}{
		"zoom": map[string]interface{}{
			"title": "Ticket Zoom",
		},
	}

	// Update tickets section with German translations
	if deTranslations["tickets"] == nil {
		deTranslations["tickets"] = make(map[string]interface{})
	}
	deTickets := deTranslations["tickets"].(map[string]interface{})

	// Add German translations for ticket actions
	deTickets["reply"] = "Antworten"
	deTickets["reply_all"] = "Allen antworten"
	deTickets["forward"] = "Weiterleiten"
	deTickets["phone"] = "Telefon"
	deTickets["note"] = "Notiz"
	deTickets["actions"] = "Aktionen"
	deTickets["owner"] = "Besitzer"
	deTickets["responsible"] = "Verantwortlicher"
	deTickets["customer"] = "Kunde"
	deTickets["state"] = "Status"
	deTickets["type"] = "Typ"
	deTickets["service"] = "Service"
	deTickets["sla"] = "SLA"
	deTickets["lock"] = "Sperren"
	deTickets["unlock"] = "Entsperren"
	deTickets["locked"] = "Gesperrt"
	deTickets["unlocked"] = "Entsperrt"
	deTickets["history"] = "Historie"
	deTickets["print"] = "Drucken"
	deTickets["link"] = "Verknüpfen"
	deTickets["miscellaneous"] = "Sonstiges"
	deTickets["watch"] = "Beobachten"
	deTickets["bulk"] = "Sammelaktion"
	deTickets["spam"] = "Spam"
	deTickets["articles"] = "Artikel"
	deTickets["details"] = "Details"
	deTickets["to"] = "An"
	deTickets["email"] = "E-Mail"
	deTickets["internal_note"] = "Interne Notiz"
	deTickets["phone_call"] = "Telefonanruf"
	deTickets["customer_info"] = "Kundeninformationen"
	deTickets["linked_tickets"] = "Verknüpfte Tickets"

	// Add German customer keys
	if deTranslations["customer"] == nil {
		deTranslations["customer"] = make(map[string]interface{})
	}
	deCustomer := deTranslations["customer"].(map[string]interface{})
	deCustomer["name"] = "Name"
	deCustomer["email"] = "E-Mail"
	deCustomer["phone"] = "Telefon"
	deCustomer["company"] = "Firma"

	// Write back the updated German translations
	output, err = json.MarshalIndent(deTranslations, "", "  ")
	if err != nil {
		fmt.Printf("Error marshaling German JSON: %v\n", err)
		return
	}

	if err := os.WriteFile(deFile, output, 0644); err != nil {
		fmt.Printf("Error writing German file: %v\n", err)
		return
	}

	fmt.Println("Successfully added ticket translations to de.json")
}
