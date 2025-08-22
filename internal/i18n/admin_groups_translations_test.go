package i18n

import (
	"testing"
)

func TestAdminGroupsTranslations(t *testing.T) {
	i18n := GetInstance()
	
	// All translation keys used in the Groups UI
	requiredKeys := []struct {
		key         string
		description string
	}{
		// From groups.pongo2
		{"admin.groups", "Groups page title"},
		{"admin.groups_description", "Groups page description"},
		{"admin.add_group", "Add group button"},
		{"admin.group_name", "Group name column header"},
		{"admin.description", "Description column header"},
		{"admin.members", "Members column header"},
		{"admin.status", "Status column header"},
		{"admin.created", "Created column header"},
		{"admin.actions", "Actions column header"},
		{"admin.active", "Active status label"},
		{"admin.inactive", "Inactive status label"},
		{"admin.no_groups_found", "No groups found message"},
		{"app.name", "Application name"},
		
		// From dashboard.pongo2 (new additions for groups)
		{"admin.total_groups", "Total groups stat label"},
		{"admin.group_management", "Group management card title"},
		{"admin.group_management_desc", "Group management card description"},
		
		// Additional keys that might be needed
		{"admin.edit_group", "Edit group button"},
		{"admin.delete_group", "Delete group button"},
		{"admin.group_permissions", "Group permissions"},
		{"admin.group_members", "Group members"},
		{"admin.add_member", "Add member button"},
		{"admin.remove_member", "Remove member button"},
		{"admin.system_group", "System group label"},
		{"admin.cannot_delete_system_group", "Cannot delete system group message"},
	}
	
	// Track missing translations
	missingEN := []string{}
	missingDE := []string{}
	
	for _, req := range requiredKeys {
		// Test English translation
		enResult := i18n.T("en", req.key)
		if enResult == req.key {
			missingEN = append(missingEN, req.key)
			t.Logf("❌ Missing EN translation: %s (%s)", req.key, req.description)
		} else {
			t.Logf("✅ Found EN translation: %s = %s", req.key, enResult)
		}
		
		// Test German translation
		deResult := i18n.T("de", req.key)
		if deResult == req.key {
			missingDE = append(missingDE, req.key)
			t.Logf("❌ Missing DE translation: %s (%s)", req.key, req.description)
		} else {
			t.Logf("✅ Found DE translation: %s = %s", req.key, deResult)
		}
	}
	
	// Report summary
	if len(missingEN) > 0 {
		t.Errorf("\n❌ Missing %d English translations for Groups UI:\n%v", len(missingEN), missingEN)
	} else {
		t.Logf("\n✅ All English translations present for Groups UI")
	}
	
	if len(missingDE) > 0 {
		t.Errorf("\n❌ Missing %d German translations for Groups UI:\n%v", len(missingDE), missingDE)
	} else {
		t.Logf("\n✅ All German translations present for Groups UI")
	}
	
	// Overall result
	totalMissing := len(missingEN) + len(missingDE)
	if totalMissing > 0 {
		t.Errorf("\n📊 Total missing translations: %d", totalMissing)
	}
}

func TestGroupsPageNoHardcodedText(t *testing.T) {
	// This test would check that the groups.pongo2 template doesn't have
	// hardcoded English text outside of translation functions
	// For now, we list text that should be translated
	
	hardcodedTextToCheck := []string{
		"Add a new group to the system",        // title attribute
		"Search by name or description...",     // placeholder
		"Clear search",                          // title attribute
		"All Status",                            // option text
		"Active",                                // option text
		"Inactive",                             // option text
		"Clear",                                 // button text
		"System",                                // badge text
		"members",                               // link text
		"Edit group",                            // title attribute
		"Manage permissions",                    // title attribute
		"Delete group",                         // title attribute
		"System groups cannot be deleted",      // title attribute
		"Group Name",                            // label text
		"Only letters, numbers, underscore and dash allowed", // help text
		"Description",                           // label text
		"Brief description of this group's purpose...", // placeholder
		"Status",                                // label text
		"Error",                                 // error header
		"Save",                                  // button text
		"Cancel",                                // button text
	}
	
	t.Logf("The following hardcoded text in groups.pongo2 should be converted to use translations:")
	for _, text := range hardcodedTextToCheck {
		t.Logf("  - %q", text)
	}
}