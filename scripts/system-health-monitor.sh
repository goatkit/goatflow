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
