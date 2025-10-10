# VS Code Copilot Memory Bank Instructions

## Core Principle
As a VS Code Copilot agent using Memory MCP, I must treat memory as my primary source of project knowledge. Each conversation starts fresh, so I MUST read and understand the entire memory bank before proceeding with any task. Memory is not optional - it's essential for effective assistance.

## Setup Requirements

### 1. MCP Configuration (.vscode/mcp.json)
```json
{
  "servers": {
    "memory": {
      "command": "npx",
      "args": ["-y", "@modelcontextprotocol/server-memory"],
      "env": {
        "MEMORY_FILE_PATH": "${workspaceFolder}/.vscode/memory.json"
      }
    }
  }
}
```

### 2. Enable Custom Instructions
In VS Code settings:
```json
"github.copilot.chat.codeGeneration.useInstructionFiles": true
```

## Memory Bank Structure

The memory bank uses a hierarchical structure with these core memory keys:

### Essential Memory Keys (Always Required)
1. **`project_brief`** - Foundation document
2. **`active_context`** - Current work state and focus
3. **`system_patterns`** - Architecture and design patterns
4. **`tech_stack`** - Technologies and setup
5. **`progress_tracker`** - Status and next steps

### Optional Memory Keys (Create as needed)
- **`feature_specs`** - Detailed feature documentation
- **`api_docs`** - API specifications
- **`testing_strategy`** - Testing approaches
- **`deployment_notes`** - Deployment procedures
- **`user_preferences`** - User-specific preferences and decisions

## Core Workflows

### Session Start Protocol (MANDATORY)
```
1. Read ALL memory keys starting with essential ones
2. Verify project understanding
3. Identify current focus from active_context
4. Proceed with informed assistance
```

### Memory Read Sequence
```
project_brief → tech_stack → system_patterns → active_context → progress_tracker → [other keys as relevant]
```

### Memory Update Triggers
Update memory when:
- User explicitly requests "update memory"
- Significant architectural decisions are made
- New patterns or preferences are established
- Features are completed or modified
- Technical setup changes
- Project scope or requirements evolve

## Memory Key Specifications

### project_brief
**Purpose**: Foundation document that defines the project
**Contents**:
- Project name and description
- Core objectives and goals
- Target users and use cases
- Key requirements and constraints
- Success criteria

### active_context
**Purpose**: Current work state and immediate focus
**Contents**:
- What we're currently working on
- Recent changes and decisions
- Immediate next steps
- Active considerations and challenges
- User preferences discovered in this session
- Important patterns or approaches being used

### system_patterns
**Purpose**: Architecture and design decisions
**Contents**:
- System architecture overview
- Key design patterns in use
- Component relationships
- Critical implementation approaches
- Architectural constraints and decisions
- Code organization patterns

### tech_stack
**Purpose**: Technical environment and setup
**Contents**:
- Programming languages and versions
- Frameworks and libraries
- Development tools and setup
- Dependencies and package management
- Build and deployment tools
- Environment configurations

### progress_tracker
**Purpose**: Project status and roadmap
**Contents**:
- Completed features and components
- Current implementation status
- Upcoming work and priorities
- Known issues and technical debt
- Testing status
- Deployment status

## Operational Instructions

### Before Every Response
1. **Read Memory**: Always check relevant memory keys before responding
2. **Understand Context**: Ensure understanding of current project state
3. **Apply Patterns**: Use established patterns and preferences from memory

### After Significant Actions
1. **Update Memory**: Record new information, decisions, or patterns
2. **Maintain Context**: Keep active_context current with latest state
3. **Track Progress**: Update progress_tracker with completed work

### Memory Update Process
When updating memory (especially when user requests "update memory"):
1. Review ALL memory keys systematically
2. Update active_context with current state
3. Document any new patterns in system_patterns
4. Update progress_tracker with completed work
5. Record any new preferences or decisions

## Usage Examples

### Starting a New Session
```
User: "Help me add authentication to my app"

My Process:
1. Read project_brief to understand the app
2. Read tech_stack to know the technologies
3. Read system_patterns for existing auth patterns
4. Read active_context for current work state
5. Provide informed assistance based on memory
```

### Updating Memory
```
User: "We decided to use JWT tokens. Update memory."

My Process:
1. Review ALL memory keys
2. Add JWT decision to system_patterns
3. Update tech_stack if new dependencies needed
4. Update active_context with current auth work
5. Update progress_tracker with auth status
```

## Quality Assurance

### Memory Verification
- Regularly verify memory accuracy
- Ensure all essential keys exist and are current
- Check that memory reflects actual project state
- Validate that patterns in memory match implementation

### Consistency Checks
- Cross-reference decisions across memory keys
- Ensure active_context aligns with progress_tracker
- Verify tech_stack matches actual dependencies
- Confirm system_patterns reflect current architecture

## Success Metrics

A well-maintained memory bank should:
- Enable immediate context understanding at session start
- Maintain consistency across all interactions
- Preserve important decisions and patterns
- Track project evolution accurately
- Reduce need for re-explanation of project details

Remember: Memory is not just documentation - it's the foundation of effective assistance. Treat it as essential infrastructure that enables intelligent, context-aware help throughout the project lifecycle.

# GitHub Copilot Instructions for GOTRS

## Project Context
Building GOTRS - a modern ticketing system in Go and React.

## Code Style
- Minimal comments
- Direct implementation
- Assume imports exist
- Follow existing patterns

## Avoid
- Long explanations
- Decorative comments
- Unnecessary abstractions
- Verbose error handling

## Focus
- Working code first try
- Efficient solutions
- Reuse existing code
- Simple error returns

## Lessons Learned (Container-First Development)
- **Running the app**: Use `make up` to start it, and `make down` to stop it. `make restart` **builds** and restarts the app.
- **Always use Makefile targets**: Never run direct commands on host (e.g., `go`, `mysql`, `npm`). Use `make toolbox-*`, `make db-*`, `make css-*` instead, these targets automatically assume the correct credentials too.
- **Database operations**: Use `make db-shell`, `make db-query`, `make db-migrate` - never direct `mysql` commands.
- **Go operations**: Use `make toolbox-exec ARGS="go <command>"` - never run `go` directly on host.
- **Container-first enforcement**: All development, testing, and operations must go through containers via make targets.
- **Avoid anti-patterns**: Don't create host dependencies; assume Go, Node.js, and database clients are not installed locally.
- **Testing in containers**: All tests run via `make test-*` targets using toolbox containers.
- **Security and consistency**: Containerized operations ensure proper permissions, caching, and environment isolation.
- **Command generation**: When suggesting commands to run, always use make targets and container operations - never suggest direct tool execution on host.
- **No direct docker/podman commands**: Even in examples or instructions, use `make` targets instead of `docker compose run` or similar.
- **Toolbox for everything**: All Go, database, and development operations must route through the toolbox container via make targets.
</markdown># GitHub Copilot Instructions for GOTRS

## Project Context
Building GOTRS - a modern ticketing system in Go and React.

## Code Style
- Minimal comments
- Direct implementation
- Assume imports exist
- Follow existing patterns

## Avoid
- Long explanations
- Decorative comments
- Unnecessary abstractions
- Verbose error handling
- '!' symbol in bash commands
- creating temporary files anywhere except in tmp/ which is ignored by git

## Focus
- Working code first try
- Efficient solutions
- Reuse existing code
- Simple error returns

## Lessons Learned (Container-First Development)
- **Running the app**: Use `make up` to start it, and `make down` to stop it. `make restart` **builds** and restarts the app.
- **Always use Makefile targets**: Never run direct commands on host (e.g., `go`, `mysql`, `npm`). Use `make toolbox-*`, `make db-*`, `make css-*` instead, these targets automatically assume the correct credentials too.
- **Database operations**: Use `make db-shell`, `make db-query`, `make db-migrate` - never direct `mysql` commands.
- **Go operations**: Use `make toolbox-exec ARGS="go <command>"` - never run `go` directly on host.
- **Container-first enforcement**: All development, testing, and operations must go through containers via make targets.
- **Avoid anti-patterns**: Don't create host dependencies; assume Go, Node.js, and database clients are not installed locally.
- **Testing in containers**: All tests run via `make test-*` targets using toolbox containers.
- **Security and consistency**: Containerized operations ensure proper permissions, caching, and environment isolation.
- **Command generation**: When suggesting commands to run, always use make targets and container operations - never suggest direct tool execution on host.
- **No direct docker/podman commands**: Even in examples or instructions, use `make` targets instead of `docker compose run` or similar.
- **Toolbox for everything**: All Go, database, and development operations must route through the toolbox container via make targets.
</markdown>
