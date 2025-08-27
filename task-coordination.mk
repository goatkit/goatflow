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
