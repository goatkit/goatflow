#!/bin/bash

# Script to help debug the sysconfig module loading issue

HANDLER_FILE="internal/components/dynamic/handler.go"
BACKUP_FILE="${HANDLER_FILE}.sysconfig_debug_backup"

show_help() {
    echo "Sysconfig Debug Helper Script"
    echo ""
    echo "Usage: $0 {prepare|apply|test|revert|status}"
    echo ""
    echo "Commands:"
    echo "  prepare  - Create backup and show manual steps"
    echo "  apply    - Apply debug modifications (requires manual editing)"
    echo "  test     - Run basic checks and start debugging session"
    echo "  revert   - Restore original file from backup"
    echo "  status   - Show current debug status"
    echo ""
    echo "Purpose: Debug why sysconfig module loads but doesn't appear in GetAvailableModules()"
}

case "$1" in
    "prepare")
        echo "=== PREPARING SYSCONFIG DEBUG ==="

        # Create backup
        if [ ! -f "$BACKUP_FILE" ]; then
            cp "$HANDLER_FILE" "$BACKUP_FILE"
            echo "✓ Created backup: $BACKUP_FILE"
        else
            echo "! Backup already exists: $BACKUP_FILE"
        fi

        # Run basic check
        echo ""
        echo "=== BASIC MODULE CHECK ==="
        go run debug_sysconfig.go

        echo ""
        echo "=== READY FOR DEBUG MODIFICATIONS ==="
        echo "Next steps:"
        echo "1. Edit $HANDLER_FILE"
        echo "2. Replace 3 functions with debug versions from debug_comprehensive.go:"
        echo "   - loadAllConfigs (around line 194)"
        echo "   - loadConfig (around line 214)"
        echo "   - GetAvailableModules (around line 920)"
        echo "3. Run: $0 test"
        ;;

    "apply")
        echo "=== APPLYING SYSCONFIG DEBUG ==="
        echo "This requires manual editing of $HANDLER_FILE"
        echo ""
        echo "Replace these 3 functions with debug versions from debug_comprehensive.go:"
        echo ""
        echo "1. loadAllConfigs function (around line 194)"
        echo "2. loadConfig function (around line 214)"
        echo "3. GetAvailableModules function (around line 920)"
        echo ""
        echo "After editing, run: $0 test"
        ;;

    "test")
        echo "=== TESTING SYSCONFIG DEBUG ==="

        # Build to check for syntax errors
        echo "Building server..."
        if go build ./cmd/server; then
            echo "✓ Server builds successfully"
        else
            echo "✗ Build failed - check debug modifications"
            exit 1
        fi

        echo ""
        echo "Starting debug session..."
        echo "Watch for debug output about sysconfig loading..."
        echo ""
        echo "Starting server with debug output:"
        make restart
        sleep 3

        echo ""
        echo "Checking server health:"
        curl -s http://localhost:8080/health

        echo ""
        echo ""
        echo "Check the logs for DEBUG output:"
        echo "make logs | grep -E '(DEBUG:|sysconfig)'"

        echo ""
        echo "Look for these patterns in the logs:"
        echo "1. 'DEBUG: Processing file: sysconfig.yaml'"
        echo "2. 'DEBUG: Successfully parsed YAML. Module name: sysconfig'"
        echo "3. 'DEBUG: Stored module sysconfig'"
        echo "4. 'DEBUG: configs map contents:' (should include sysconfig)"
        echo "5. 'DEBUG: Adding module to list: sysconfig'"
        ;;

    "revert")
        echo "=== REVERTING SYSCONFIG DEBUG ==="

        if [ -f "$BACKUP_FILE" ]; then
            mv "$BACKUP_FILE" "$HANDLER_FILE"
            echo "✓ Reverted $HANDLER_FILE from backup"
            echo "✓ Restart the server to remove debug output:"
            echo "  make restart"
        else
            echo "✗ No backup file found: $BACKUP_FILE"
            exit 1
        fi
        ;;

    "status")
        echo "=== SYSCONFIG DEBUG STATUS ==="

        if [ -f "$BACKUP_FILE" ]; then
            echo "✓ Backup exists: $BACKUP_FILE"
        else
            echo "✗ No backup found - debug not prepared"
        fi

        echo ""
        echo "Module file status:"
        if [ -f "modules/sysconfig.yaml" ]; then
            echo "✓ sysconfig.yaml exists"
            echo "  Module name: $(grep 'name:' modules/sysconfig.yaml | head -1 | cut -d: -f2 | tr -d ' ')"
        else
            echo "✗ sysconfig.yaml missing"
        fi

        echo ""
        echo "Current server status:"
        if curl -s http://localhost:8080/health > /dev/null; then
            echo "✓ Server is running"
        else
            echo "✗ Server not responding"
        fi

        echo ""
        echo "Available debug files:"
        ls -la debug*.go 2>/dev/null || echo "No debug files found"
        ;;

    *)
        show_help
        exit 1
        ;;
esac
