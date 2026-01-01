// Package main provides a tool for adding translations.
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

	// Ensure admin section exists
	if translations["admin"] == nil {
		translations["admin"] = make(map[string]interface{})
	}
	admin := translations["admin"].(map[string]interface{})

	// Add common section under admin
	admin["common"] = map[string]interface{}{
		"actions": map[string]interface{}{
			"new":             "New",
			"edit":            "Edit",
			"delete":          "Delete",
			"save":            "Save",
			"cancel":          "Cancel",
			"create":          "Create",
			"update":          "Update",
			"toggle_status":   "Toggle Status",
			"export_selected": "Export Selected",
			"delete_selected": "Delete Selected",
			"import_csv":      "Import CSV",
			"export_csv":      "Export CSV",
		},
		"search": map[string]interface{}{
			"placeholder": "Search...",
		},
		"filters": map[string]interface{}{
			"active":               "Active",
			"inactive":             "Inactive",
			"open":                 "Open",
			"closed":               "Closed",
			"pending":              "Pending",
			"active_users":         "Active Users",
			"inactive_users":       "Inactive Users",
			"agents":               "Agents",
			"active_queues":        "Active Queues",
			"email_queues":         "Email Queues",
			"clear_all":            "Clear All",
			"saved_sets":           "Saved Filter Sets",
			"set_name_placeholder": "Enter filter set name...",
			"name_required":        "Please enter a name for the filter set",
		},
		"pagination": map[string]interface{}{
			"show":     "Show",
			"per_page": "per page",
			"previous": "Previous",
			"next":     "Next",
		},
		"selected": "selected",
		"status": map[string]interface{}{
			"active":   "Active",
			"inactive": "Inactive",
			"valid":    "Valid",
			"invalid":  "Invalid",
		},
	}

	// Add module-specific translations structure
	if admin["modules"] == nil {
		admin["modules"] = make(map[string]interface{})
	}
	modules := admin["modules"].(map[string]interface{})

	// Add template for dynamic modules
	moduleNames := []string{"queue", "priority", "state", "type", "sla", "service", "salutation"}
	for _, mod := range moduleNames {
		modules[mod] = map[string]interface{}{
			"actions": map[string]interface{}{
				"new": fmt.Sprintf("New %s", capitalize(mod)),
			},
			"create_title":   fmt.Sprintf("New %s", capitalize(mod)),
			"edit_title":     fmt.Sprintf("Edit %s", capitalize(mod)),
			"delete_title":   fmt.Sprintf("Delete %s", capitalize(mod)),
			"delete_confirm": fmt.Sprintf("Are you sure you want to delete this %s? This action cannot be undone.", mod),
		}
	}

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

	fmt.Println("Successfully added common translations to en.json")

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

	// Ensure admin section exists
	if deTranslations["admin"] == nil {
		deTranslations["admin"] = make(map[string]interface{})
	}
	deAdmin := deTranslations["admin"].(map[string]interface{})

	// Add German common section
	deAdmin["common"] = map[string]interface{}{
		"actions": map[string]interface{}{
			"new":             "Neu",
			"edit":            "Bearbeiten",
			"delete":          "Löschen",
			"save":            "Speichern",
			"cancel":          "Abbrechen",
			"create":          "Erstellen",
			"update":          "Aktualisieren",
			"toggle_status":   "Status umschalten",
			"export_selected": "Ausgewählte exportieren",
			"delete_selected": "Ausgewählte löschen",
			"import_csv":      "CSV importieren",
			"export_csv":      "CSV exportieren",
		},
		"search": map[string]interface{}{
			"placeholder": "Suchen...",
		},
		"filters": map[string]interface{}{
			"active":               "Aktiv",
			"inactive":             "Inaktiv",
			"open":                 "Offen",
			"closed":               "Geschlossen",
			"pending":              "Ausstehend",
			"active_users":         "Aktive Benutzer",
			"inactive_users":       "Inaktive Benutzer",
			"agents":               "Agenten",
			"active_queues":        "Aktive Warteschlangen",
			"email_queues":         "E-Mail-Warteschlangen",
			"clear_all":            "Alle löschen",
			"saved_sets":           "Gespeicherte Filtersets",
			"set_name_placeholder": "Filtersetnamen eingeben...",
			"name_required":        "Bitte geben Sie einen Namen für das Filterset ein",
		},
		"pagination": map[string]interface{}{
			"show":     "Zeige",
			"per_page": "pro Seite",
			"previous": "Zurück",
			"next":     "Weiter",
		},
		"selected": "ausgewählt",
		"status": map[string]interface{}{
			"active":   "Aktiv",
			"inactive": "Inaktiv",
			"valid":    "Gültig",
			"invalid":  "Ungültig",
		},
	}

	// Add module-specific German translations
	if deAdmin["modules"] == nil {
		deAdmin["modules"] = make(map[string]interface{})
	}
	deModules := deAdmin["modules"].(map[string]interface{})

	// German module translations
	deModuleNames := map[string]string{
		"queue":      "Warteschlange",
		"priority":   "Priorität",
		"state":      "Status",
		"type":       "Typ",
		"sla":        "SLA",
		"service":    "Service",
		"salutation": "Anrede",
	}

	for mod, deName := range deModuleNames {
		deModules[mod] = map[string]interface{}{
			"actions": map[string]interface{}{
				"new": fmt.Sprintf("Neue %s", deName),
			},
			"create_title":   fmt.Sprintf("Neue %s", deName),
			"edit_title":     fmt.Sprintf("%s bearbeiten", deName),
			"delete_title":   fmt.Sprintf("%s löschen", deName),
			"delete_confirm": fmt.Sprintf("Sind Sie sicher, dass Sie diese %s löschen möchten? Diese Aktion kann nicht rückgängig gemacht werden.", deName),
		}
	}

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

	fmt.Println("Successfully added common translations to de.json")
}

func capitalize(s string) string {
	if len(s) == 0 {
		return s
	}
	return string(s[0]-32) + s[1:]
}
