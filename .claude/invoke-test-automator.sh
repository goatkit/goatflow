#!/bin/bash
# Agent Invocation: test-automator
# Usage: ./invoke-test-automator.sh "Create comprehensive tests for AdminGroups CRUD operations"

set -e

TASK="${1:-Please provide a test creation task}"
AGENT_FILE=".claude/agents/test-automator.md"

if [ ! -f "$AGENT_FILE" ]; then
    echo "Error: test-automator agent not found at $AGENT_FILE"
    exit 1
fi

echo "🧪 Consulting test-automator for task: $TASK"
echo "📋 Agent Requirements: $(grep -A5 "When invoked:" $AGENT_FILE | tail -4)"
echo ""
echo "🔍 Pre-test system verification:"
echo "1. Backend compilation check..."
if go build ./cmd/gotrs 2>/dev/null; then
    echo "   ✅ Backend compiles successfully"
else
    echo "   ❌ Backend compilation failed - fix before testing"
    exit 1
fi

echo ""
echo "2. Container health check..."
if curl -s http://localhost:8080/health >/dev/null 2>&1; then
    echo "   ✅ Backend service responding"
else
    echo "   ⚠️  Backend not responding - may need restart"
fi

echo ""
echo "📝 Test automator context query:"
echo '{
  "requesting_agent": "test-automator",
  "request_type": "get_automation_context",
  "payload": {
    "query": "Automation context needed: application type, tech stack, current coverage, manual tests, CI/CD setup, and team skills.",
    "task": "'"$TASK"'",
    "project": "GOTRS - Go backend with Gin framework",
    "tech_stack": {
      "backend": "Go 1.22+, Gin, PostgreSQL",
      "frontend": "HTMX, Alpine.js, Tailwind CSS",
      "containers": "Docker/Podman",
      "database": "PostgreSQL with OTRS schema"
    },
    "test_requirements": [
      "Container-based test execution",
      "Database integration testing",
      "HTTP endpoint testing",
      "Template rendering verification",
      "CRUD operation validation"
    ],
    "execution_commands": {
      "unit_tests": "./scripts/container-wrapper.sh exec gotrs-backend go test ./...",
      "health_check": "curl http://localhost:8080/health",
      "service_restart": "make restart"
    }
  }
}'
echo ""
echo "⚠️  ENFORCEMENT: No implementation code may be written until tests exist AND fail appropriately"
echo "📖 Next step: Copy this context to Claude and request test-automator consultation"
