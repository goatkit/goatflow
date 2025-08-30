#!/bin/bash
# Agent Invocation: code-reviewer
# Usage: ./invoke-code-reviewer.sh "Verify AdminGroups implementation meets quality standards"

set -e

TASK="${1:-Please provide a code review task}"
AGENT_FILE=".claude/agents/code-reviewer.md"

if [ ! -f "$AGENT_FILE" ]; then
    echo "Error: code-reviewer agent not found at $AGENT_FILE"
    exit 1
fi

echo "🔍 Consulting code-reviewer for task: $TASK"
echo "📋 Agent Requirements: $(grep -A5 "When invoked:" $AGENT_FILE | tail -4)"
echo ""

# Mandatory quality verification checklist
echo "📊 MANDATORY QUALITY VERIFICATION CHECKLIST:"
echo ""

echo "1. 🏗️  Build Verification:"
if go build ./cmd/gotrs 2>/dev/null; then
    echo "   ✅ Backend compiles without errors"
else
    echo "   ❌ BLOCKER: Backend compilation failed"
    echo "   🚨 Cannot proceed with review until compilation succeeds"
    exit 1
fi

echo ""
echo "2. 🚀 Service Health:"
echo "   Restarting backend service..."
make restart >/dev/null 2>&1
sleep 3

if curl -s http://localhost:8080/health | grep -q "healthy"; then
    echo "   ✅ Backend service healthy and responding"
else
    echo "   ❌ BLOCKER: Backend service not responding or unhealthy"
    echo "   🚨 Cannot proceed with review until service is healthy"
    exit 1
fi

echo ""
echo "3. 📋 Template Verification:"
TEMPLATE_ERRORS=$(make logs | grep "Template error" | tail -5)
if [ -z "$TEMPLATE_ERRORS" ]; then
    echo "   ✅ No template errors in recent logs"
else
    echo "   ❌ BLOCKER: Template errors found in logs:"
    echo "$TEMPLATE_ERRORS"
    exit 1
fi

echo ""
echo "4. 🌐 HTTP Status Verification:"
echo "   Recent HTTP requests from logs:"
make logs | grep -E "GET|POST|PUT|DELETE" | grep -E "[0-9]{3}" | tail -3

echo ""
echo "📝 Code reviewer context query:"
echo '{
  "requesting_agent": "code-reviewer",
  "request_type": "get_review_context",
  "payload": {
    "query": "Code review context needed: language, coding standards, security requirements, performance criteria, team conventions, and review scope.",
    "task": "'"$TASK"'",
    "project": "GOTRS - OTRS replacement system",
    "quality_standards": {
      "compilation": "Must build without errors or warnings",
      "service_health": "Must start and respond to health checks",
      "template_rendering": "Zero template errors in logs",
      "http_responses": "No 500 or 404 responses for claimed functionality",
      "browser_testing": "Zero JavaScript console errors",
      "crud_operations": "All CRUD operations must be manually verified"
    },
    "verification_commands": {
      "build": "go build ./cmd/server",
      "health": "curl http://localhost:8080/health",
      "logs": "make logs",
      "restart": "make restart"
    },
    "review_scope": [
      "Code quality and maintainability",
      "Security vulnerabilities",
      "Performance bottlenecks",
      "OTRS schema compliance",
      "Internationalization completeness",
      "Container compatibility"
    ]
  }
}'
echo ""
echo "⚠️  ENFORCEMENT: Cannot claim 'done' without code-reviewer verification of ALL quality gates"
echo "🚨 CRITICAL: Must test actual functionality in browser, not just check HTTP status codes"
echo "📖 Next step: Copy this context to Claude and request code-reviewer consultation"
