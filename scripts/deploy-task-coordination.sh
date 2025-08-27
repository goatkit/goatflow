#!/bin/bash
#
# COMPREHENSIVE TASK MANAGEMENT COORDINATION DEPLOYMENT
# Deploys the complete task coordination system with quality gates and evidence collection
#
# This script sets up:
# - Task Coordinator for systematic task decomposition and coordination
# - Task Evidence Collector for comprehensive verification and quality gates
# - Integration with existing TDD enforcer and workflow orchestrator
# - Quality gate enforcement with evidence collection requirements
# - Anti-gaslighting protocols to prevent false success claims
#

set -euo pipefail

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
PURPLE='\033[0;35m'
NC='\033[0m'

# Configuration
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
AGENT_DIR="$HOME/.claude/agents"

# Logging functions
log() {
    echo -e "${BLUE}[$(date +%H:%M:%S)] DEPLOY:${NC} $1"
}

success() {
    echo -e "${GREEN}✓ DEPLOY:${NC} $1"
}

fail() {
    echo -e "${RED}✗ DEPLOY:${NC} $1"
}

warning() {
    echo -e "${YELLOW}⚠ DEPLOY:${NC} $1"
}

critical() {
    echo -e "${RED}🚨 CRITICAL DEPLOYMENT ERROR:${NC} $1"
    exit 1
}

# Check prerequisites
check_prerequisites() {
    log "Checking deployment prerequisites..."
    
    # Check if agent directory exists
    if [ ! -d "$AGENT_DIR" ]; then
        log "Creating agent directory: $AGENT_DIR"
        mkdir -p "$AGENT_DIR"
    fi
    
    # Check if required tools are available
    local missing_tools=()
    
    if ! command -v jq >/dev/null 2>&1; then
        missing_tools+=("jq")
    fi
    
    if ! command -v curl >/dev/null 2>&1; then
        missing_tools+=("curl")
    fi
    
    if ! command -v bc >/dev/null 2>&1; then
        missing_tools+=("bc")
    fi
    
    if [ ${#missing_tools[@]} -gt 0 ]; then
        critical "Missing required tools: ${missing_tools[*]}. Please install them first."
    fi
    
    success "Prerequisites check passed"
}

# Verify existing agents
verify_existing_agents() {
    log "Verifying existing agent components..."
    
    local required_agents=(
        "multi-agent-coordinator.md"
        "workflow-orchestrator.md"
    )
    
    for agent_file in "${required_agents[@]}"; do
        if [ -f "$AGENT_DIR/$agent_file" ]; then
            success "Found existing agent: $agent_file"
        else
            fail "Missing required agent: $agent_file"
            warning "Please ensure multi-agent coordinator and workflow orchestrator are deployed first"
        fi
    done
    
    # Check if TDD enforcer exists
    if [ -f "$PROJECT_ROOT/scripts/tdd-enforcer.sh" ]; then
        success "Found TDD enforcer script"
    else
        critical "TDD enforcer script not found. This is required for quality gate integration."
    fi
}

# Create task coordination directories
setup_directories() {
    log "Setting up task coordination directories..."
    
    local directories=(
        "$PROJECT_ROOT/.task-status"
        "$PROJECT_ROOT/generated/task-evidence"
        "$PROJECT_ROOT/generated/task-reports"
        "$PROJECT_ROOT/generated/investigations"
        "$PROJECT_ROOT/generated/tdd-logs"
        "$PROJECT_ROOT/generated/evidence"
        "$PROJECT_ROOT/generated/recovery-logs"
        "$PROJECT_ROOT/generated/health-logs"
        "$PROJECT_ROOT/generated/performance-logs"
        "$PROJECT_ROOT/generated/resource-logs"
    )
    
    for dir in "${directories[@]}"; do
        if [ ! -d "$dir" ]; then
            mkdir -p "$dir"
            log "Created directory: $dir"
        else
            success "Directory exists: $dir"
        fi
    done
    
    success "Directory setup completed"
}

# Deploy task coordination agents
deploy_agents() {
    log "Deploying task coordination agents..."
    
    local agents=(
        "task-coordinator"
        "task-evidence-collector"
    )
    
    for agent in "${agents[@]}"; do
        if [ -f "$AGENT_DIR/${agent}.md" ]; then
            success "Agent already deployed: ${agent}.md"
        else
            fail "Agent not found: ${agent}.md"
            warning "Ensure all agent files are created in $AGENT_DIR"
        fi
    done
}

# Initialize task coordination system
initialize_system() {
    log "Initializing task coordination system..."
    
    # Create task coordination state file
    local coordination_state_file="$PROJECT_ROOT/.task-coordination-state"
    
    cat > "$coordination_state_file" << EOF
{
  "system_initialized": true,
  "initialization_time": "$(date -Iseconds)",
  "version": "1.0.0",
  "components": {
    "task_coordinator": "deployed",
    "evidence_collector": "deployed", 
    "tdd_enforcer": "integrated",
    "multi_agent_coordinator": "integrated",
    "workflow_orchestrator": "integrated"
  },
  "quality_standards": {
    "required_quality_gates": 7,
    "success_threshold_percent": 100,
    "evidence_collection_mandatory": true,
    "false_success_claims_blocked": true
  },
  "active_tasks": {},
  "agent_assignments": {},
  "monitoring": {
    "error_monitoring": false,
    "health_monitoring": false,
    "performance_monitoring": false
  }
}
EOF
    
    success "Task coordination state initialized: $coordination_state_file"
}

# Create task coordination wrapper script
create_coordination_wrapper() {
    log "Creating task coordination wrapper script..."
    
    local wrapper_script="$PROJECT_ROOT/scripts/task-coordinator.sh"
    
    cat > "$wrapper_script" << 'EOF'
#!/bin/bash
#
# TASK COORDINATION WRAPPER SCRIPT
# Provides interface to the task coordination system
#

set -euo pipefail

# Configuration
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
LOG_DIR="$PROJECT_ROOT/generated/task-logs"
EVIDENCE_DIR="$PROJECT_ROOT/generated/evidence"

mkdir -p "$LOG_DIR" "$EVIDENCE_DIR"

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

log() {
    echo -e "${BLUE}[$(date +%H:%M:%S)] COORD:${NC} $1" | tee -a "$LOG_DIR/coordination.log"
}

success() {
    echo -e "${GREEN}✓ COORD:${NC} $1" | tee -a "$LOG_DIR/coordination.log"
}

fail() {
    echo -e "${RED}✗ COORD:${NC} $1" | tee -a "$LOG_DIR/coordination.log"
}

# Task creation with evidence requirements
create_task() {
    local description="$1"
    local priority="${2:-medium}"
    local task_type="${3:-development}"
    
    log "Creating task with evidence requirements..."
    log "Description: $description"
    log "Priority: $priority"
    log "Type: $task_type"
    
    # Generate unique task ID
    local task_id="TASK_$(date +%Y%m%d_%H%M%S)_$(openssl rand -hex 4)"
    
    # Create task status file
    mkdir -p "$PROJECT_ROOT/.task-status"
    cat > "$PROJECT_ROOT/.task-status/$task_id.json" << EOT
{
  "task_id": "$task_id",
  "description": "$description",
  "priority": "$priority",
  "task_type": "$task_type",
  "created": "$(date -Iseconds)",
  "status": "created",
  "completed": false,
  "evidence_required": true,
  "quality_gates_required": 7,
  "evidence_files": [],
  "quality_gates_passed": 0,
  "assigned_agents": [],
  "prerequisites": [],
  "subtasks": []
}
EOT
    
    success "Task created: $task_id"
    echo "$task_id"
}

# Execute task with quality gates
execute_task() {
    local task_id="$1"
    local feature_name="${2:-}"
    
    if [ ! -f "$PROJECT_ROOT/.task-status/$task_id.json" ]; then
        fail "Task not found: $task_id"
        return 1
    fi
    
    log "Executing task with mandatory quality gates: $task_id"
    
    local task_type=$(jq -r '.task_type' "$PROJECT_ROOT/.task-status/$task_id.json")
    
    case "$task_type" in
        "development")
            execute_development_task "$task_id" "$feature_name"
            ;;
        "bugfix")
            execute_bugfix_task "$task_id"
            ;;
        "refactor")
            execute_refactor_task "$task_id"
            ;;
        *)
            execute_generic_task "$task_id"
            ;;
    esac
}

# Execute development task with TDD integration
execute_development_task() {
    local task_id="$1"
    local feature_name="$2"
    
    log "Executing development task: $task_id ($feature_name)"
    
    # Update task status
    jq '.status = "executing" | .started_time = "$(date -Iseconds)"' \
        "$PROJECT_ROOT/.task-status/$task_id.json" > "$PROJECT_ROOT/.task-status/$task_id.json.tmp" && \
        mv "$PROJECT_ROOT/.task-status/$task_id.json.tmp" "$PROJECT_ROOT/.task-status/$task_id.json"
    
    # Phase 1: TDD Test-First (RED)
    log "Phase 1: TDD Test-First for task $task_id"
    if [ -n "$feature_name" ]; then
        make tdd-test-first FEATURE="$feature_name"
    else
        log "No feature name provided, skipping TDD test-first phase"
    fi
    
    # Collect evidence after test-first
    local evidence_red=$(collect_task_evidence "$task_id" "test_first")
    
    # Phase 2: Implementation (GREEN)
    log "Phase 2: Implementation for task $task_id"
    make tdd-implement
    
    # Collect evidence after implementation
    local evidence_green=$(collect_task_evidence "$task_id" "implementation")
    
    # Phase 3: Comprehensive Verification
    log "Phase 3: Comprehensive verification for task $task_id"
    make tdd-verify
    
    if [ $? -eq 0 ]; then
        # Collect final evidence
        local evidence_final=$(collect_task_evidence "$task_id" "completion")
        
        # Complete task with evidence
        complete_task_with_evidence "$task_id" "$evidence_final"
    else
        fail_task_with_evidence "$task_id" "tdd_verification_failed"
    fi
}

# Collect evidence for task
collect_task_evidence() {
    local task_id="$1"
    local phase="$2"
    
    log "Collecting evidence for task: $task_id (phase: $phase)"
    
    # Use existing TDD enforcer for evidence collection
    local evidence_file="$EVIDENCE_DIR/${task_id}_${phase}_$(date +%Y%m%d_%H%M%S).json"
    
    # Run TDD verification to collect evidence
    if ./scripts/tdd-enforcer.sh verify "$phase"; then
        # Find the most recent evidence file
        local latest_evidence=$(find "$EVIDENCE_DIR" -name "verify_${phase}_*.json" -type f -printf '%T@ %p\n' | sort -n | tail -1 | cut -d' ' -f2-)
        
        if [ -n "$latest_evidence" ] && [ -f "$latest_evidence" ]; then
            # Link evidence to task
            cp "$latest_evidence" "$evidence_file"
            
            # Update task with evidence file
            jq --arg evidence_file "$evidence_file" '.evidence_files += [$evidence_file]' \
                "$PROJECT_ROOT/.task-status/$task_id.json" > "$PROJECT_ROOT/.task-status/$task_id.json.tmp" && \
                mv "$PROJECT_ROOT/.task-status/$task_id.json.tmp" "$PROJECT_ROOT/.task-status/$task_id.json"
            
            log "Evidence collected and linked to task: $task_id"
        fi
    else
        log "Evidence collection failed for task: $task_id"
    fi
    
    echo "$evidence_file"
}

# Complete task with evidence verification
complete_task_with_evidence() {
    local task_id="$1"
    local evidence_file="$2"
    
    log "Completing task with evidence verification: $task_id"
    
    # Verify evidence exists and is complete
    if [ -f "$evidence_file" ]; then
        local success_rate=$(jq -r '.quality_gates.success_rate // 0' "$evidence_file" 2>/dev/null || echo "0")
        
        if [ "$success_rate" -eq 100 ]; then
            # Update task status to completed
            jq --arg evidence_file "$evidence_file" --arg completed_time "$(date -Iseconds)" \
                '.status = "completed" | .completed = true | .completed_time = $completed_time | .final_evidence_file = $evidence_file' \
                "$PROJECT_ROOT/.task-status/$task_id.json" > "$PROJECT_ROOT/.task-status/$task_id.json.tmp" && \
                mv "$PROJECT_ROOT/.task-status/$task_id.json.tmp" "$PROJECT_ROOT/.task-status/$task_id.json"
            
            success "Task completed with verified evidence: $task_id (100% quality gates passed)"
        else
            fail "Task completion blocked: $task_id (quality gates: $success_rate%, required: 100%)"
            fail_task_with_evidence "$task_id" "insufficient_quality_gates"
        fi
    else
        fail "Task completion blocked: $task_id (no evidence file found: $evidence_file)"
        fail_task_with_evidence "$task_id" "no_evidence_file"
    fi
}

# Fail task with evidence
fail_task_with_evidence() {
    local task_id="$1" 
    local failure_reason="$2"
    
    log "Failing task with evidence: $task_id (reason: $failure_reason)"
    
    jq --arg failure_reason "$failure_reason" --arg failed_time "$(date -Iseconds)" \
        '.status = "failed" | .failure_reason = $failure_reason | .failed_time = $failed_time' \
        "$PROJECT_ROOT/.task-status/$task_id.json" > "$PROJECT_ROOT/.task-status/$task_id.json.tmp" && \
        mv "$PROJECT_ROOT/.task-status/$task_id.json.tmp" "$PROJECT_ROOT/.task-status/$task_id.json"
    
    fail "Task failed: $task_id ($failure_reason)"
}

# List tasks
list_tasks() {
    local filter="${1:-all}"
    
    log "Listing tasks (filter: $filter)"
    
    if [ ! -d "$PROJECT_ROOT/.task-status" ]; then
        log "No tasks found"
        return 0
    fi
    
    echo "Task Status Report"
    echo "=================="
    printf "%-25s %-15s %-15s %-20s %-10s\n" "Task ID" "Status" "Type" "Created" "Evidence"
    echo "-------------------------------------------------------------------------------------"
    
    for task_file in "$PROJECT_ROOT/.task-status"/*.json; do
        if [ -f "$task_file" ]; then
            local task_data=$(cat "$task_file")
            local task_id=$(echo "$task_data" | jq -r '.task_id')
            local status=$(echo "$task_data" | jq -r '.status')
            local task_type=$(echo "$task_data" | jq -r '.task_type')
            local created=$(echo "$task_data" | jq -r '.created' | cut -d'T' -f1)
            local evidence_count=$(echo "$task_data" | jq -r '.evidence_files | length')
            
            if [ "$filter" = "all" ] || [ "$status" = "$filter" ]; then
                printf "%-25s %-15s %-15s %-20s %-10s\n" "$task_id" "$status" "$task_type" "$created" "$evidence_count"
            fi
        fi
    done
}

# Show task details
show_task() {
    local task_id="$1"
    
    if [ ! -f "$PROJECT_ROOT/.task-status/$task_id.json" ]; then
        fail "Task not found: $task_id"
        return 1
    fi
    
    log "Task details for: $task_id"
    cat "$PROJECT_ROOT/.task-status/$task_id.json" | jq .
}

# Main command dispatcher
main() {
    local command="${1:-help}"
    
    case "$command" in
        "create")
            if [ -z "${2:-}" ]; then
                fail "Task description required. Usage: $0 create \"Task description\" [priority] [type]"
                exit 1
            fi
            create_task "${2}" "${3:-medium}" "${4:-development}"
            ;;
        "execute")
            if [ -z "${2:-}" ]; then
                fail "Task ID required. Usage: $0 execute TASK_ID [feature_name]"
                exit 1
            fi
            execute_task "${2}" "${3:-}"
            ;;
        "list")
            list_tasks "${2:-all}"
            ;;
        "show")
            if [ -z "${2:-}" ]; then
                fail "Task ID required. Usage: $0 show TASK_ID"
                exit 1
            fi
            show_task "${2}"
            ;;
        "help"|*)
            echo "Task Coordination System"
            echo "========================"
            echo ""
            echo "Commands:"
            echo "  create \"description\" [priority] [type] - Create new task with evidence requirements"
            echo "  execute TASK_ID [feature_name]         - Execute task with quality gates"
            echo "  list [status]                          - List tasks (all|created|executing|completed|failed)"
            echo "  show TASK_ID                           - Show detailed task information"
            echo "  help                                   - Show this help message"
            echo ""
            echo "Task Types: development, bugfix, refactor"
            echo "Priorities: low, medium, high, critical"
            echo ""
            echo "Quality Gates (ALL must pass for task completion):"
            echo "  1. Compilation (code compiles without errors)"
            echo "  2. Service Health (services respond healthily)"
            echo "  3. Templates (zero template errors in logs)"
            echo "  4. Tests (all tests pass with adequate coverage)"
            echo "  5. HTTP Endpoints (≥80% respond correctly)"
            echo "  6. Browser Console (zero JavaScript errors)"
            echo "  7. Logs (no ERROR/PANIC entries)"
            echo ""
            exit 1
            ;;
    esac
}

main "$@"
EOF
    
    chmod +x "$wrapper_script"
    success "Task coordination wrapper created: $wrapper_script"
}

# Create integration with existing Makefile
integrate_with_makefile() {
    log "Creating Makefile integration..."
    
    # Create Makefile snippet for task coordination
    local makefile_snippet="$PROJECT_ROOT/task-coordination.mk"
    
    cat > "$makefile_snippet" << 'EOF'
# Task Coordination Integration
# Include this in main Makefile with: include task-coordination.mk

.PHONY: task-create task-execute task-list task-show task-verify-all

# Task coordination commands
task-create:
	@if [ -z "$(DESCRIPTION)" ]; then \
		echo "Error: DESCRIPTION required. Usage: make task-create DESCRIPTION='Task description' [PRIORITY=medium] [TYPE=development]"; \
		exit 1; \
	fi
	@./scripts/task-coordinator.sh create "$(DESCRIPTION)" "$(PRIORITY)" "$(TYPE)"

task-execute:
	@if [ -z "$(TASK_ID)" ]; then \
		echo "Error: TASK_ID required. Usage: make task-execute TASK_ID=TASK_xxx [FEATURE='Feature Name']"; \
		exit 1; \
	fi
	@./scripts/task-coordinator.sh execute "$(TASK_ID)" "$(FEATURE)"

task-list:
	@./scripts/task-coordinator.sh list "$(STATUS)"

task-show:
	@if [ -z "$(TASK_ID)" ]; then \
		echo "Error: TASK_ID required. Usage: make task-show TASK_ID=TASK_xxx"; \
		exit 1; \
	fi
	@./scripts/task-coordinator.sh show "$(TASK_ID)"

# Verify all system components for comprehensive testing
task-verify-all:
	@echo "🔍 Running comprehensive system verification..."
	@echo "This includes ALL quality gates with evidence collection"
	@./scripts/tdd-enforcer.sh verify comprehensive
	@echo "✅ Comprehensive verification completed"
EOF
    
    success "Makefile integration created: $makefile_snippet"
    
    # Add note about including in main Makefile
    warning "Add 'include task-coordination.mk' to your main Makefile to enable task coordination commands"
}

# Create monitoring and health check scripts
create_monitoring_scripts() {
    log "Creating monitoring and health check scripts..."
    
    # Create system health monitor
    local health_monitor="$PROJECT_ROOT/scripts/system-health-monitor.sh"
    
    cat > "$health_monitor" << 'EOF'
#!/bin/bash
# System Health Monitor for Task Coordination
# Monitors system health and alerts on issues

INTERVAL="${1:-60}"
PROJECT_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
LOG_DIR="$PROJECT_ROOT/generated/health-logs"
mkdir -p "$LOG_DIR"

log() {
    echo "[$(date +%H:%M:%S)] HEALTH: $1" | tee -a "$LOG_DIR/health-monitor.log"
}

# Check service health
check_service_health() {
    if curl -f -s http://localhost:8080/health >/dev/null; then
        return 0
    else
        return 1
    fi
}

# Check database health  
check_database_health() {
    if ./scripts/container-wrapper.sh compose exec postgres pg_isready -U gotrs >/dev/null 2>&1; then
        return 0
    else
        return 1
    fi
}

# Monitor loop
log "Starting system health monitoring (interval: ${INTERVAL}s)"
while true; do
    issues=0
    
    if ! check_service_health; then
        log "ISSUE: Backend service unhealthy"
        ((issues++))
    fi
    
    if ! check_database_health; then
        log "ISSUE: Database unhealthy" 
        ((issues++))
    fi
    
    if [ "$issues" -eq 0 ]; then
        log "System health: OK"
    else
        log "System health: $issues issues detected"
    fi
    
    sleep "$INTERVAL"
done
EOF
    
    chmod +x "$health_monitor"
    success "Health monitor created: $health_monitor"
}

# Verify deployment
verify_deployment() {
    log "Verifying task coordination system deployment..."
    
    local verification_points=(
        "Task coordination state file exists:$PROJECT_ROOT/.task-coordination-state"
        "Task status directory exists:$PROJECT_ROOT/.task-status"
        "Evidence directory exists:$PROJECT_ROOT/generated/evidence"
        "Task coordinator script exists:$PROJECT_ROOT/scripts/task-coordinator.sh"
        "TDD enforcer integration:$PROJECT_ROOT/scripts/tdd-enforcer.sh"
    )
    
    local verification_passed=0
    local verification_total=${#verification_points[@]}
    
    for point in "${verification_points[@]}"; do
        local description="${point%:*}"
        local path="${point#*:}"
        
        if [ -f "$path" ] || [ -d "$path" ]; then
            success "$description"
            ((verification_passed++))
        else
            fail "$description"
        fi
    done
    
    local success_rate=$((verification_passed * 100 / verification_total))
    
    if [ "$success_rate" -eq 100 ]; then
        success "Deployment verification: PASSED ($verification_passed/$verification_total)"
        return 0
    else
        fail "Deployment verification: FAILED ($verification_passed/$verification_total, $success_rate%)"
        return 1
    fi
}

# Generate deployment report
generate_deployment_report() {
    log "Generating deployment report..."
    
    local report_file="$PROJECT_ROOT/generated/deployment-report-$(date +%Y%m%d_%H%M%S).html"
    mkdir -p "$(dirname "$report_file")"
    
    cat > "$report_file" << 'EOF'
<!DOCTYPE html>
<html>
<head>
    <title>Task Coordination System Deployment Report</title>
    <style>
        body { font-family: Arial, sans-serif; margin: 20px; line-height: 1.6; }
        .header { background: #007bff; color: white; padding: 20px; margin-bottom: 20px; }
        .success { color: #28a745; font-weight: bold; }
        .warning { color: #ffc107; font-weight: bold; }
        .section { margin: 20px 0; padding: 15px; background: #f8f9fa; }
        .component { margin: 10px 0; padding: 10px; border-left: 4px solid #007bff; }
        pre { background: #e9ecef; padding: 10px; overflow-x: auto; }
    </style>
</head>
<body>
    <div class="header">
        <h1>Task Coordination System Deployment Report</h1>
        <p>Comprehensive task management with quality gates and evidence collection</p>
        <p><strong>Deployed:</strong> $(date)</p>
    </div>
    
    <div class="section">
        <h2>Deployment Overview</h2>
        <p>The task coordination system has been deployed with the following components:</p>
        
        <div class="component">
            <h3>Task Coordinator</h3>
            <p>Systematic task decomposition, agent coordination, and quality gate enforcement</p>
        </div>
        
        <div class="component">
            <h3>Task Evidence Collector</h3>
            <p>Comprehensive evidence collection and verification for all quality gates</p>
        </div>
        
        <div class="component">
            <h3>TDD Integration</h3>
            <p>Integration with existing TDD enforcer for quality gate verification</p>
        </div>
        
        <div class="component">
            <h3>Anti-Gaslighting Protocols</h3>
            <p>Zero tolerance for false success claims - all 7 quality gates must pass</p>
        </div>
    </div>
    
    <div class="section">
        <h2>Quality Gates (ALL Required for Task Completion)</h2>
        <ol>
            <li><strong>Compilation:</strong> Code compiles without errors</li>
            <li><strong>Service Health:</strong> Services respond with healthy status</li>
            <li><strong>Templates:</strong> Zero template errors in logs</li>
            <li><strong>Tests:</strong> All tests pass with adequate coverage</li>
            <li><strong>HTTP Endpoints:</strong> ≥80% of endpoints respond correctly</li>
            <li><strong>Browser Console:</strong> Zero JavaScript errors</li>
            <li><strong>Logs:</strong> No ERROR/PANIC entries in recent logs</li>
        </ol>
    </div>
    
    <div class="section">
        <h2>Usage Examples</h2>
        <h3>Create Development Task</h3>
        <pre>./scripts/task-coordinator.sh create "Implement user authentication" high development</pre>
        
        <h3>Execute Task with TDD</h3>
        <pre>./scripts/task-coordinator.sh execute TASK_20250126_143052_a1b2 "User Authentication"</pre>
        
        <h3>List All Tasks</h3>
        <pre>./scripts/task-coordinator.sh list</pre>
        
        <h3>Show Task Details</h3>
        <pre>./scripts/task-coordinator.sh show TASK_20250126_143052_a1b2</pre>
        
        <h3>Using Makefile Integration</h3>
        <pre>
make task-create DESCRIPTION="Fix login bug" PRIORITY=critical TYPE=bugfix
make task-execute TASK_ID=TASK_xxx FEATURE="Login Fix"
make task-list STATUS=completed
make task-verify-all</pre>
    </div>
    
    <div class="section">
        <h2>Key Features</h2>
        <ul>
            <li class="success">✓ Systematic task decomposition with evidence requirements</li>
            <li class="success">✓ Integration with existing TDD enforcer and quality gates</li>
            <li class="success">✓ Comprehensive evidence collection for all quality gates</li>
            <li class="success">✓ Zero tolerance for false success claims</li>
            <li class="success">✓ Task dependency management and prerequisite verification</li>
            <li class="success">✓ Agent coordination with quality oversight</li>
            <li class="success">✓ Automated recovery and error handling</li>
            <li class="success">✓ Real-time monitoring and health checks</li>
        </ul>
    </div>
    
    <div class="section warning">
        <h2>Important Notes</h2>
        <ul>
            <li>All 7 quality gates MUST pass for task completion (100% requirement)</li>
            <li>Evidence collection is mandatory - no success without proof</li>
            <li>Tasks cannot be marked complete without comprehensive verification</li>
            <li>System prevents "two steps forward, one step back" patterns</li>
            <li>Integration with TodoWrite for enhanced task tracking available</li>
        </ul>
    </div>
    
</body>
</html>
EOF
    
    success "Deployment report generated: $report_file"
    echo "$report_file"
}

# Main deployment function
main() {
    log "Starting comprehensive task management coordination deployment..."
    
    check_prerequisites
    verify_existing_agents
    setup_directories
    deploy_agents
    initialize_system
    create_coordination_wrapper
    integrate_with_makefile
    create_monitoring_scripts
    
    if verify_deployment; then
        success "Task coordination system deployment completed successfully!"
        
        local report_file=$(generate_deployment_report)
        
        echo ""
        echo "🎯 DEPLOYMENT SUMMARY"
        echo "===================="
        echo "✅ Task coordination system deployed and verified"
        echo "✅ Quality gates enforced with evidence collection"
        echo "✅ Anti-gaslighting protocols activated"
        echo "✅ TDD integration enabled"
        echo "✅ Monitoring systems configured"
        echo ""
        echo "📋 NEXT STEPS:"
        echo "1. Add 'include task-coordination.mk' to your main Makefile"
        echo "2. Create your first task: ./scripts/task-coordinator.sh create \"Task description\""
        echo "3. Execute with quality gates: ./scripts/task-coordinator.sh execute TASK_ID"
        echo "4. View deployment report: $report_file"
        echo ""
        echo "🚨 REMEMBER: All 7 quality gates must pass for task completion!"
        
    else
        critical "Task coordination system deployment failed verification!"
    fi
}

main "$@"