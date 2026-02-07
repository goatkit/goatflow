#!/bin/bash

# GoatKit Demo - YAML Configuration Management for GoatFlow
set -e

echo "   ___________  ___  _________ __ __ ______ "
echo "  / ____/ __ \\/   |/_  __/ __ \\/ //_//  _/ /_"
echo " / / __/ / / / /| | / / / /_/ / ,<   / // __/"
echo "/ /_/ / /_/ / ___ |/ / / _, _/ /| |_/ // /_  "
echo "\\____/\\____/_/  |_/_/ /_/ |_/_/ |_/___/\\__/  "
echo ""
echo "🐐 GoatKit Demo - YAML Configuration Management"
echo "=============================================="
echo ""
echo "GoatKit brings enterprise-grade configuration management"
echo "to GoatFlow with version control, validation, and hot reload."
echo ""

# Build GoatKit
echo "📦 Building GoatKit container..."
docker build -f Dockerfile.goatkit -t goatkit . > /dev/null 2>&1
echo "✅ GoatKit ready!"
echo ""

# Helper to run GoatKit
gk() {
    docker run --rm -v /home/nigel/git/goatkit/goatflow:/workspace:ro goatkit "$@"
}

# 1. Show help
echo "1️⃣ GoatKit Commands"
echo "==================="
gk help | grep -A 20 "Commands:" | head -22
echo ""
read -p "Press Enter to continue..."
echo ""

# 2. Validate a route file
echo "2️⃣ Validating YAML Files"
echo "======================="
echo "Validating a route configuration..."
echo ""
gk validate /workspace/routes/core/health.yaml
echo ""
read -p "Press Enter to continue..."
echo ""

# 3. Lint route files
echo "3️⃣ Linting for Best Practices"
echo "============================"
echo "Checking routes for issues..."
echo ""
gk lint /workspace/routes | head -30
echo ""
read -p "Press Enter to continue..."
echo ""

# 4. Show about
echo "4️⃣ About GoatKit"
echo "==============="
gk about | tail -15
echo ""

# Summary
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo ""
echo "🎉 GoatKit is ready for use!"
echo ""
echo "Quick usage:"
echo "  docker run --rm goatkit help              # Show help"
echo "  docker run --rm goatkit about             # About GoatKit"
echo "  docker run --rm -v \$(pwd):/app goatkit ls  # List configs"
echo ""
echo "Or use the short alias:"
echo "  docker run --rm goatkit                   # Interactive help"
echo ""
echo "GoatKit provides:"
echo "  🐐 Version control for all YAML configs"
echo "  🐐 Validation and linting"
echo "  🐐 Hot reload capabilities"
echo "  🐐 GitOps-ready workflows"
echo "  🐐 100% container-based"
echo ""
echo "Part of the GoatFlow suite - The GOAT of configuration management!"
echo ""