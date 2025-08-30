#!/bin/bash

# Script to apply debug modifications for sysconfig investigation

HANDLER_FILE="internal/components/dynamic/handler.go"
BACKUP_FILE="${HANDLER_FILE}.debug_backup"

case "$1" in
    "apply")
        echo "=== APPLYING DEBUG MODIFICATIONS ==="

        # Create backup
        if [ ! -f "$BACKUP_FILE" ]; then
            cp "$HANDLER_FILE" "$BACKUP_FILE"
            echo "✓ Created backup: $BACKUP_FILE"
        else
            echo "! Backup already exists: $BACKUP_FILE"
        fi

        # Apply debug modifications
        echo "✓ Ready to apply debug modifications"
        echo ""
        echo "MANUAL STEPS REQUIRED:"
        echo "1. Edit $HANDLER_FILE"
        echo "2. Find the loadModuleConfig function (around line 215)"
        echo "3. Replace it with the DEBUG VERSION from debug_handler_modification.go"
        echo "4. Find the GetAvailableModules function (around line 920)"
        echo "5. Replace it with the DEBUG VERSION from debug_handler_modification.go"
        echo "6. Save the file"
        echo "7. Run: make restart"
        echo "8. Check logs for detailed debug output"
        echo ""
        echo "Or run: ./apply_debug.sh patch (to apply automatically)"
        ;;

    "revert")
        echo "=== REVERTING DEBUG MODIFICATIONS ==="

        if [ -f "$BACKUP_FILE" ]; then
            mv "$BACKUP_FILE" "$HANDLER_FILE"
            echo "✓ Reverted $HANDLER_FILE from backup"
            echo "✓ Restart the server to remove debug output"
        else
            echo "✗ No backup file found: $BACKUP_FILE"
            exit 1
        fi
        ;;

    "patch")
        echo "=== APPLYING AUTOMATED PATCH ==="

        # Create backup
        if [ ! -f "$BACKUP_FILE" ]; then
            cp "$HANDLER_FILE" "$BACKUP_FILE"
            echo "✓ Created backup: $BACKUP_FILE"
        fi

        # Create the debug patch
        cat > debug_patch.go << 'EOF'
// DEBUG VERSION - loadModuleConfig
func (h *DynamicModuleHandler) loadModuleConfig(configPath string) error {
	fmt.Printf("DEBUG: Loading module config from: %s\n", configPath)

	data, err := os.ReadFile(configPath)
	if err != nil {
		fmt.Printf("DEBUG: Failed to read config file %s: %v\n", configPath, err)
		return err
	}

	var config ModuleConfig
	if err := yaml.Unmarshal(data, &config); err != nil {
		fmt.Printf("DEBUG: Failed to unmarshal config file %s: %v\n", configPath, err)
		return err
	}

	fmt.Printf("DEBUG: Parsed module name: '%s'\n", config.Module.Name)
	fmt.Printf("DEBUG: Module config details: Title=%s, Description=%s\n",
		config.Module.Title, config.Module.Description)

	h.mu.Lock()
	fmt.Printf("DEBUG: About to store module '%s' in configs map\n", config.Module.Name)
	h.configs[config.Module.Name] = &config
	fmt.Printf("DEBUG: Stored module '%s' in configs map. Map now has %d entries\n",
		config.Module.Name, len(h.configs))

	// Debug: Print all keys currently in the map
	fmt.Printf("DEBUG: Current keys in configs map: ")
	for key := range h.configs {
		fmt.Printf("'%s' ", key)
	}
	fmt.Printf("\n")

	h.mu.Unlock()

	fmt.Printf("Loaded module: %s\n", config.Module.Name)
	return nil
}

// DEBUG VERSION - GetAvailableModules
func (h *DynamicModuleHandler) GetAvailableModules() []string {
	fmt.Printf("DEBUG: GetAvailableModules called\n")

	h.mu.RLock()
	defer h.mu.RUnlock()

	fmt.Printf("DEBUG: configs map has %d entries\n", len(h.configs))

	// Debug: Print all keys in the map
	fmt.Printf("DEBUG: All keys in configs map: ")
	for key := range h.configs {
		fmt.Printf("'%s' ", key)
	}
	fmt.Printf("\n")

	modules := []string{}
	for name := range h.configs {
		fmt.Printf("DEBUG: Adding module to list: '%s'\n", name)
		modules = append(modules, name)
	}

	fmt.Printf("DEBUG: Returning %d modules: ", len(modules))
	for _, mod := range modules {
		fmt.Printf("'%s' ", mod)
	}
	fmt.Printf("\n")

	return modules
}
EOF

        echo "✓ Debug patch created"
        echo ""
        echo "MANUAL APPLICATION STILL REQUIRED:"
        echo "Replace the two functions in $HANDLER_FILE with the versions from debug_patch.go"
        echo "Then restart the server and check logs"
        ;;

    *)
        echo "Usage: $0 {apply|revert|patch}"
        echo ""
        echo "  apply  - Prepare to apply debug modifications (manual steps)"
        echo "  patch  - Create debug patch file (still requires manual application)"
        echo "  revert - Restore original file from backup"
        echo ""
        echo "Purpose: Debug why sysconfig module doesn't appear in GetAvailableModules()"
        exit 1
        ;;
esac
