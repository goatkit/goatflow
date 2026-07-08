#!/bin/bash

# GoatFlow Unified YAML-as-a-Service Platform Demo
# Demonstrates comprehensive configuration management with version control

set -e

echo "🔧 GoatFlow Unified YAML-as-a-Service Platform Demo"
echo "================================================"
echo ""
echo "This demo showcases the unified configuration management system"
echo "with version control, validation, hot reload, and containerized tooling."
echo ""

# Build the config manager container
echo "📦 Building Config Manager Container..."
docker build -f Dockerfile.config-manager -t goatflow-config-manager . > /dev/null 2>&1
echo "✅ Container built successfully"
echo ""

# Create a persistent volume for demonstration
echo "📁 Creating persistent storage for demo..."
docker volume create goatflow-config-demo > /dev/null 2>&1
echo ""

# Helper function to run config manager with persistent storage
run_config() {
    docker run --rm \
        -v goatflow-config-demo:/app/.versions \
        -v "$PWD:/workspace:ro" \
        -v "$PWD/routes:/app/routes:ro" \
        -v "$PWD/config:/app/config:ro" \
        goatflow-config-manager "$@"
}

# 1. Import existing configurations
echo "1️⃣ Importing Existing Configurations"
echo "====================================="
echo "Importing routes, configs, and dashboards into version management..."
echo ""

# Import routes
run_config import /app/routes 2>&1 | grep -E "✅|❌|Import complete"

# Import configs
run_config import /app/config 2>&1 | grep -E "✅|❌|Import complete"

echo ""
read -p "Press Enter to continue..."
echo ""

# 2. List all configurations
echo "2️⃣ Listing All Managed Configurations"
echo "===================================="
run_config list | head -30
echo ""
read -p "Press Enter to continue..."
echo ""

# 3. Validate configurations
echo "3️⃣ Validating Configuration Files"
echo "================================="
echo "Checking schema compliance and best practices..."
echo ""
run_config validate /app/routes/core/health.yaml
echo ""
read -p "Press Enter to continue..."
echo ""

# 4. Lint configurations
echo "4️⃣ Linting for Best Practices"
echo "=============================="
echo "Analyzing configurations for issues..."
echo ""
run_config lint /app/routes | head -40
echo ""
read -p "Press Enter to continue..."
echo ""

# 5. Version management
echo "5️⃣ Version Management Demo"
echo "========================="
echo "Showing version history for a configuration..."
echo ""

# Create a test configuration change
cat > /tmp/test-config.yaml << 'EOF'
apiVersion: goatflow.io/v1
kind: Config
metadata:
  name: test-settings
  description: Test configuration for demo
  version: "1.0"
spec:
  settings:
    - name: DemoMode
      type: boolean
      default: true
      description: Enable demo mode
    - name: MaxConnections
      type: integer
      default: 100
      description: Maximum database connections
EOF

echo "Applying test configuration..."
run_config apply /tmp/test-config.yaml
echo ""

echo "Modifying configuration..."
cat > /tmp/test-config-v2.yaml << 'EOF'
apiVersion: goatflow.io/v1
kind: Config
metadata:
  name: test-settings
  description: Test configuration for demo (updated)
  version: "1.1"
spec:
  settings:
    - name: DemoMode
      type: boolean
      default: false
      description: Enable demo mode
    - name: MaxConnections
      type: integer
      default: 200
      description: Maximum database connections
    - name: CacheEnabled
      type: boolean
      default: true
      description: Enable caching layer
EOF

run_config apply /tmp/test-config-v2.yaml
echo ""

echo "Viewing version history..."
run_config version list config test-settings
echo ""
read -p "Press Enter to continue..."
echo ""

# 6. Diff between versions
echo "6️⃣ Comparing Configuration Versions"
echo "==================================="
echo "Showing changes between versions..."
echo ""
run_config diff config test-settings 2>&1 | head -20
echo ""
read -p "Press Enter to continue..."
echo ""

# 7. Rollback demonstration
echo "7️⃣ Rollback Capability"
echo "====================="
echo "Rolling back to previous version..."
echo ""
echo "Current version before rollback:"
run_config show config test-settings | grep -E "version|default" | head -5
echo ""
echo "Performing rollback..."
echo "yes" | run_config rollback config test-settings v1 2>&1 | grep -E "✅|❌|Rollback"
echo ""
echo "Version after rollback:"
run_config show config test-settings | grep -E "version|default" | head -5
echo ""
read -p "Press Enter to continue..."
echo ""

# 8. Export configurations
echo "8️⃣ Exporting Configurations"
echo "=========================="
echo "Exporting all configs to files..."
echo ""
run_config export config /tmp/export
ls -la /tmp/export/ 2>/dev/null | head -10 || echo "Export directory: /tmp/export/"
echo ""
read -p "Press Enter to continue..."
echo ""

# 9. Hot reload simulation
echo "9️⃣ Hot Reload Capability"
echo "======================="
echo "The system supports hot reload for all configuration types:"
echo ""
cat << 'EOF'
🔄 Hot Reload Features:
- File watching with fsnotify
- Automatic version creation on changes
- Validation before applying changes
- Event notifications for all changes
- Zero-downtime configuration updates

Example output when file changes:
[14:32:15] 📝 config/system-config (v3a2f1b9c)
[14:32:18] ✨ route/new-endpoint (v8d4e2a1f)
[14:32:21] 🗑️ dashboard/old-dashboard
[14:32:24] ❌ config/invalid-config - Error: Validation failed

To enable hot reload in production:
  goatflow-config watch &
EOF
echo ""
read -p "Press Enter to continue..."
echo ""

# 10. Platform benefits summary
echo "🎯 Platform Benefits Summary"
echo "==========================="
echo ""
cat << 'EOF'
✅ Unified Management
   - Single tool for all YAML configurations
   - Consistent interface across config types
   - Centralized version control

✅ Safety & Reliability
   - Version control with rollback
   - Schema validation before apply
   - Linting for best practices
   - Atomic configuration updates

✅ Developer Experience
   - Hot reload without restarts
   - GitOps-ready workflows
   - Comprehensive CLI tools
   - Container-first architecture

✅ Production Ready
   - Complete audit trail
   - Performance impact analysis
   - Security scanning
   - Zero-downtime updates

✅ Extensibility
   - Plugin new YAML types easily
   - Custom validation rules
   - Webhook notifications
   - Integration with CI/CD

📊 Configuration Types Supported:
   - Routes (API endpoints)
   - Config (System settings)
   - Dashboards (UI layouts)
   - Docker Compose (Services)
   - Easily extensible for more
EOF
echo ""

# Cleanup
echo "🧹 Cleaning up demo resources..."
docker volume rm goatflow-config-demo > /dev/null 2>&1 || true
rm -f /tmp/test-config*.yaml
rm -rf /tmp/export

echo ""
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo ""
echo "🎉 Demo Complete!"
echo ""
echo "The GoatFlow Unified YAML-as-a-Service Platform provides:"
echo ""
echo "• Version control for ALL configurations"
echo "• Hot reload without service restarts"
echo "• Schema validation and linting"
echo "• GitOps-ready workflows"
echo "• 100% containerized management"
echo ""
echo "To use in your environment:"
echo "  docker run --rm goatflow-config-manager <command>"
echo ""
echo "This platform dramatically improves configuration safety,"
echo "developer productivity, and operational reliability!"
echo ""