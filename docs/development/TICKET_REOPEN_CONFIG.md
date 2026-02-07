# Ticket Reopen Configuration Decision

## Background
In OTRS, the ability to reopen closed tickets is controlled by configuration settings. This allows organizations to enforce their ticket lifecycle policies.

## OTRS Configuration Reference
In OTRS `Kernel/Config.pm`:
```perl
# Ticket::Frontend::AgentTicketClose###StateType
# Defines the state type for closing tickets
$Self->{'Ticket::Frontend::AgentTicketClose'}->{'StateType'} = ['closed'];

# Ticket::Frontend::AgentTicketClose###StateDefault
# Defines the default next state for closing tickets
$Self->{'Ticket::Frontend::AgentTicketClose'}->{'StateDefault'} = 'closed successful';

# Ticket::Frontend::AgentTicketClose###Permission
# Required permission to close tickets
$Self->{'Ticket::Frontend::AgentTicketClose'}->{'Permission'} = 'close';

# Ticket::Frontend::AgentTicketReopen
# Allow/disallow reopening of closed tickets
$Self->{'Ticket::Frontend::AgentTicketReopen'} = 1;  # 1 = allow, 0 = disallow
```

## Proposed GoatFlow Configuration

### Using Viper/YAML (as per project decision)
```yaml
# config/goatflow.yaml
ticket:
  # Ticket state change controls
  state_changes:
    # Allow reopening closed tickets
    allow_reopen: true
    
    # Who can reopen tickets
    reopen_permissions:
      - "admin"
      - "agent"
      # - "customer"  # Uncomment to allow customers to reopen their tickets
    
    # Time limit for reopening (0 = no limit)
    reopen_time_limit_days: 0  # Can only reopen within X days of closing
    
    # States that can be reopened
    reopenable_states:
      - "closed successful"
      - "closed unsuccessful"
      # - "closed with workaround"  # Uncomment if workarounds can be revisited
    
    # Target state when reopening
    reopen_target_state: "open"  # or "new" depending on workflow
    
    # Require reason for reopening
    reopen_requires_reason: true
    
    # Automatically add note when reopening
    auto_add_reopen_note: true
```

## Implementation Plan

### Phase 1: Basic Configuration (Current)
- Hardcoded allow/disallow in template
- Simple reopen to "open" state
- Add note on reopen

### Phase 2: Configuration Integration (After Viper integration)
```go
// internal/config/ticket.go
type TicketConfig struct {
    StateChanges struct {
        AllowReopen           bool     `mapstructure:"allow_reopen"`
        ReopenPermissions     []string `mapstructure:"reopen_permissions"`
        ReopenTimeLimitDays   int      `mapstructure:"reopen_time_limit_days"`
        ReopenableStates      []string `mapstructure:"reopenable_states"`
        ReopenTargetState     string   `mapstructure:"reopen_target_state"`
        ReopenRequiresReason  bool     `mapstructure:"reopen_requires_reason"`
        AutoAddReopenNote     bool     `mapstructure:"auto_add_reopen_note"`
    } `mapstructure:"state_changes"`
}
```

### Phase 3: Template Integration
```django
{# In ticket_detail.pongo2 #}
{% if 'closed' in Ticket.status and Config.ticket.state_changes.allow_reopen %}
    {% if User.role in Config.ticket.state_changes.reopen_permissions %}
        {% if Ticket.status in Config.ticket.state_changes.reopenable_states %}
            <button onclick="reopenTicket()">Reopen Ticket</button>
        {% endif %}
    {% endif %}
{% endif %}
```

### Phase 4: Reopen Dialog (IMPLEMENTED)
Professional reopen dialog implemented with:
- Reason for reopening (required field)
- Target state selection (New/Open)
- Additional notes (optional)
- Customer notification option
- Replaces unprofessional browser alert() with branded dialog

## Security Considerations

1. **Permission Control**: Only authorized users should reopen tickets
2. **Audit Trail**: All reopens must be logged with user and timestamp
3. **Time Limits**: Prevent reopening very old tickets
4. **State Validation**: Only certain closed states should be reopenable

## Business Rules

1. **Customer Reopens**: 
   - May be limited to own tickets only
   - May have stricter time limits
   - May require approval

2. **Agent Reopens**:
   - Can reopen any ticket in their queues
   - Should add mandatory reason

3. **Admin Reopens**:
   - Unrestricted access
   - For audit/compliance purposes

## Migration Notes

- Default to `allow_reopen: true` for compatibility
- Existing OTRS installations can map their settings
- Configuration can be overridden per queue/group

## Examples

### Strict Policy (No Reopens)
```yaml
ticket:
  state_changes:
    allow_reopen: false
```

### Customer Service Policy (Limited Reopens)
```yaml
ticket:
  state_changes:
    allow_reopen: true
    reopen_permissions: ["admin", "agent"]
    reopen_time_limit_days: 30
    reopenable_states: ["closed successful", "closed with workaround"]
    reopen_requires_reason: true
```

### Development/Testing Policy (Flexible)
```yaml
ticket:
  state_changes:
    allow_reopen: true
    reopen_permissions: ["admin", "agent", "customer"]
    reopen_time_limit_days: 0
    reopenable_states: ["closed successful", "closed unsuccessful", "closed with workaround"]
    reopen_requires_reason: false
```

## Note
This configuration will be implemented after Viper configuration system is integrated, as decided in `CONFIG_DECISION.md`. For now, reopen is enabled by default for all closed tickets.