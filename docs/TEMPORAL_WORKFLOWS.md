# Temporal Workflows Documentation

## Overview

GoatFlow uses Temporal for orchestrating complex business processes, particularly around ticket lifecycle management, SLA enforcement, and automated notifications. Temporal provides durable execution, automatic retries, and visibility into long-running processes.

## Architecture

```
┌─────────────────────────────────────────────┐
│             GoatFlow Application            │
│                                             │
│  ┌──────────────────────────────────────┐   │
│  │         Temporal Client              │   │
│  │  - Start workflows                   │   │
│  │  - Query workflow state              │   │
│  │  - Send signals to workflows         │   │
│  └─────────────┬────────────────────────┘   │
│                │                            │
│  ┌─────────────▼────────────────────────┐   │
│  │         Temporal Worker              │   │
│  │  - Execute workflow definitions      │   │
│  │  - Execute activity functions        │   │
│  │  - Handle retries and failures       │   │
│  └──────────────────────────────────────┘   │
└─────────────────┬───────────────────────────┘
                  │
    ┌─────────────▼────────────────┐
    │    Temporal Server           │
    │  - Workflow state storage    │
    │  - Task queues               │
    │  - History management        │
    └──────────────────────────────┘
```

## Core Workflows

### 1. Ticket Lifecycle Workflow

Manages the complete lifecycle of a ticket from creation to closure.

```go
// TicketLifecycleWorkflow manages ticket state transitions
func TicketLifecycleWorkflow(ctx workflow.Context, ticketID string) error {
    // Set workflow options
    ao := workflow.ActivityOptions{
        StartToCloseTimeout: 10 * time.Minute,
        RetryPolicy: &temporal.RetryPolicy{
            MaximumAttempts: 3,
        },
    }
    ctx = workflow.WithActivityOptions(ctx, ao)

    // Initial ticket creation
    var ticket Ticket
    err := workflow.ExecuteActivity(ctx, CreateTicketActivity, ticketID).Get(ctx, &ticket)
    if err != nil {
        return err
    }

    // Wait for state transitions
    selector := workflow.NewSelector(ctx)
    
    // Handle assignment
    assignChan := workflow.GetSignalChannel(ctx, "assign-ticket")
    selector.AddReceive(assignChan, func(c workflow.ReceiveChannel, more bool) {
        var assignData AssignmentData
        c.Receive(ctx, &assignData)
        workflow.ExecuteActivity(ctx, AssignTicketActivity, ticketID, assignData)
    })

    // Handle status updates
    statusChan := workflow.GetSignalChannel(ctx, "update-status")
    selector.AddReceive(statusChan, func(c workflow.ReceiveChannel, more bool) {
        var status string
        c.Receive(ctx, &status)
        workflow.ExecuteActivity(ctx, UpdateTicketStatusActivity, ticketID, status)
    })

    // SLA timer
    slaTimer := workflow.NewTimer(ctx, ticket.SLADeadline)
    selector.AddFuture(slaTimer, func(f workflow.Future) {
        workflow.ExecuteActivity(ctx, EscalateTicketActivity, ticketID)
    })

    // Run until ticket is closed
    for ticket.Status != "closed" {
        selector.Select(ctx)
    }

    return nil
}
```

### 2. SLA Enforcement Workflow

Monitors and enforces Service Level Agreements for tickets.

```go
// SLAEnforcementWorkflow monitors SLA compliance
func SLAEnforcementWorkflow(ctx workflow.Context, ticketID string, slaConfig SLAConfig) error {
    logger := workflow.GetLogger(ctx)
    
    // Track SLA milestones
    milestones := []SLAMilestone{
        {Name: "FirstResponse", Duration: slaConfig.FirstResponseTime},
        {Name: "Resolution", Duration: slaConfig.ResolutionTime},
    }

    for _, milestone := range milestones {
        // Set timer for milestone
        timer := workflow.NewTimer(ctx, milestone.Duration)
        
        // Wait for either milestone completion or timeout
        selector := workflow.NewSelector(ctx)
        
        // Listen for milestone completion
        completeChan := workflow.GetSignalChannel(ctx, milestone.Name + "-complete")
        selector.AddReceive(completeChan, func(c workflow.ReceiveChannel, more bool) {
            logger.Info("SLA milestone completed", "milestone", milestone.Name)
        })
        
        // Handle timeout
        selector.AddFuture(timer, func(f workflow.Future) {
            // Milestone breached - escalate
            workflow.ExecuteActivity(ctx, SLABreachActivity, ticketID, milestone.Name)
            
            // Send notifications
            workflow.ExecuteActivity(ctx, NotifyManagersActivity, ticketID, milestone.Name)
        })
        
        selector.Select(ctx)
    }

    return nil
}
```

### 3. Notification Workflow

Handles all ticket-related notifications with retry logic and delivery tracking.

```go
// NotificationWorkflow manages notification delivery
func NotificationWorkflow(ctx workflow.Context, notification NotificationRequest) error {
    ao := workflow.ActivityOptions{
        StartToCloseTimeout: 1 * time.Minute,
        RetryPolicy: &temporal.RetryPolicy{
            InitialInterval:    1 * time.Second,
            BackoffCoefficient: 2.0,
            MaximumInterval:    1 * time.Minute,
            MaximumAttempts:    5,
        },
    }
    ctx = workflow.WithActivityOptions(ctx, ao)

    // Try primary notification channel
    err := workflow.ExecuteActivity(ctx, SendEmailActivity, notification).Get(ctx, nil)
    if err != nil {
        // Fallback to secondary channel
        workflow.ExecuteActivity(ctx, SendInAppNotificationActivity, notification)
    }

    // Log notification
    workflow.ExecuteActivity(ctx, LogNotificationActivity, notification)

    return nil
}
```

### 4. Escalation Workflow

Manages ticket escalation based on rules and conditions.

```go
// EscalationWorkflow handles ticket escalation logic
func EscalationWorkflow(ctx workflow.Context, ticketID string, rules []EscalationRule) error {
    for _, rule := range rules {
        // Check escalation condition
        var shouldEscalate bool
        err := workflow.ExecuteActivity(ctx, CheckEscalationConditionActivity, ticketID, rule).
            Get(ctx, &shouldEscalate)
        if err != nil {
            return err
        }

        if shouldEscalate {
            // Perform escalation
            workflow.ExecuteActivity(ctx, EscalateToNextLevelActivity, ticketID, rule.Level)
            
            // Notify relevant parties
            workflow.ExecuteActivity(ctx, NotifyEscalationActivity, ticketID, rule.Level)
            
            // Wait before checking next level
            workflow.Sleep(ctx, rule.WaitDuration)
        }
    }

    return nil
}
```

## Activities

Activities are the building blocks that perform actual work in workflows.

```go
// Activity definitions
type Activities struct {
    db     *sql.DB
    email  EmailService
    search SearchService
}

// CreateTicketActivity creates a new ticket in the database
func (a *Activities) CreateTicketActivity(ctx context.Context, ticketID string) (*Ticket, error) {
    // Implementation
}

// AssignTicketActivity assigns a ticket to an agent
func (a *Activities) AssignTicketActivity(ctx context.Context, ticketID string, agentID string) error {
    // Implementation
}

// SendEmailActivity sends an email notification
func (a *Activities) SendEmailActivity(ctx context.Context, notification NotificationRequest) error {
    // Implementation with retry logic
}
```

## Worker Configuration

```go
// Worker setup in main.go
func startTemporalWorker(temporalClient client.Client) error {
    // Create worker
    w := worker.New(temporalClient, "goatflow-task-queue", worker.Options{
        MaxConcurrentActivityExecutionSize: 10,
        MaxConcurrentWorkflowExecutionSize: 10,
    })

    // Register workflows
    w.RegisterWorkflow(TicketLifecycleWorkflow)
    w.RegisterWorkflow(SLAEnforcementWorkflow)
    w.RegisterWorkflow(NotificationWorkflow)
    w.RegisterWorkflow(EscalationWorkflow)

    // Register activities
    activities := &Activities{
        db:     database,
        email:  emailService,
        search: searchService,
    }
    w.RegisterActivity(activities)

    // Start worker
    return w.Run(worker.InterruptCh())
}
```

## Starting Workflows

```go
// Start a workflow from the application
func handleTicketCreation(c *gin.Context, temporalClient client.Client) {
    ticketID := generateTicketID()
    
    // Start ticket lifecycle workflow
    workflowOptions := client.StartWorkflowOptions{
        ID:        "ticket-" + ticketID,
        TaskQueue: "goatflow-task-queue",
    }
    
    we, err := temporalClient.ExecuteWorkflow(
        context.Background(),
        workflowOptions,
        TicketLifecycleWorkflow,
        ticketID,
    )
    if err != nil {
        // Handle error
    }
    
    // Start SLA workflow
    slaOptions := client.StartWorkflowOptions{
        ID:        "sla-" + ticketID,
        TaskQueue: "goatflow-task-queue",
    }
    
    _, err = temporalClient.ExecuteWorkflow(
        context.Background(),
        slaOptions,
        SLAEnforcementWorkflow,
        ticketID,
        slaConfig,
    )
}
```

## Querying Workflow State

```go
// Query workflow state
func getTicketWorkflowStatus(temporalClient client.Client, ticketID string) (string, error) {
    response, err := temporalClient.QueryWorkflow(
        context.Background(),
        "ticket-"+ticketID,
        "",
        "status",
    )
    if err != nil {
        return "", err
    }
    
    var status string
    err = response.Get(&status)
    return status, err
}
```

## Best Practices

1. **Idempotent Activities**: Always make activities idempotent to handle retries safely
2. **Timeouts**: Set appropriate timeouts for activities
3. **Error Handling**: Use retry policies for transient failures
4. **Workflow ID**: Use meaningful, unique workflow IDs (e.g., "ticket-{ticketID}")
5. **Versioning**: Use workflow versioning for backward compatibility
6. **Monitoring**: Use Temporal UI for workflow visibility
7. **Testing**: Use Temporal's testing framework for unit tests

## Monitoring

Access Temporal UI at http://localhost:8088 to:
- View running workflows
- Inspect workflow history
- Debug failed workflows
- Monitor worker health

## Configuration

```yaml
# config/temporal.yaml
temporal:
  host: temporal:7233
  namespace: default
  task_queue: goatflow-task-queue
  worker:
    max_concurrent_activities: 10
    max_concurrent_workflows: 10
  retry_policy:
    initial_interval: 1s
    backoff_coefficient: 2.0
    maximum_interval: 60s
    maximum_attempts: 5
```

## Testing Workflows

```go
func TestTicketLifecycleWorkflow(t *testing.T) {
    testSuite := &testsuite.WorkflowTestSuite{}
    env := testSuite.NewTestWorkflowEnvironment()
    
    // Mock activities
    env.OnActivity(CreateTicketActivity, mock.Anything, "TICKET-001").
        Return(&Ticket{ID: "TICKET-001", Status: "new"}, nil)
    
    // Execute workflow
    env.ExecuteWorkflow(TicketLifecycleWorkflow, "TICKET-001")
    
    // Verify results
    require.True(t, env.IsWorkflowCompleted())
    require.NoError(t, env.GetWorkflowError())
}
```