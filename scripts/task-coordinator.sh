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
