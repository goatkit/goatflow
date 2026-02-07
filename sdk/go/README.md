# GoatFlow Go SDK

The official Go SDK for the GoatFlow ticketing system API.

## Installation

```bash
go get github.com/goatkit/goatflow/sdk/go
```

## Quick Start

```go
package main

import (
    "context"
    "fmt"
    "log"

    "github.com/goatkit/goatflow/sdk/go"
    "github.com/goatkit/goatflow/sdk/go/types"
)

func main() {
    // Create client with API key
    client := goatflow.NewClientWithAPIKey("https://your-goatflow-instance.com", "your-api-key")

    ctx := context.Background()

    // List tickets
    tickets, err := client.Tickets.List(ctx, &types.TicketListOptions{
        PageSize: 10,
        Status:   []string{"open"},
    })
    if err != nil {
        log.Fatal(err)
    }

    fmt.Printf("Found %d tickets\n", tickets.TotalCount)
}
```

## Authentication

### API Key (Recommended for server-to-server)

```go
client := goatflow.NewClientWithAPIKey("https://goatflow.example.com", "your-api-key")
```

### JWT Token

```go
expiresAt := time.Now().Add(24 * time.Hour)
client := goatflow.NewClientWithJWT("https://goatflow.example.com", "jwt-token", "refresh-token", expiresAt)
```

### OAuth2

```go
oauth2Auth := goatflow.NewOAuth2Auth("access-token", "refresh-token", "Bearer", expiresAt)
client := goatflow.NewClient(&goatflow.Config{
    BaseURL: "https://goatflow.example.com",
    Auth:    oauth2Auth,
})
```

### Custom Configuration

```go
client := goatflow.NewClient(&goatflow.Config{
    BaseURL:    "https://goatflow.example.com",
    Auth:       goatflow.NewAPIKeyAuth("your-api-key"),
    UserAgent:  "my-app/1.0.0",
    Timeout:    30 * time.Second,
    RetryCount: 3,
    Debug:      true,
})
```

## Features

### Ticket Management

```go
// Create ticket
ticket, err := client.Tickets.Create(ctx, &types.TicketCreateRequest{
    Title:       "New Issue",
    Description: "Something is broken",
    Priority:    "high",
    QueueID:     1,
    CustomerID:  123,
})

// Get ticket
ticket, err := client.Tickets.Get(ctx, ticketID)

// Update ticket
updatedTicket, err := client.Tickets.Update(ctx, ticketID, &types.TicketUpdateRequest{
    Status: &status,
})

// Search tickets
results, err := client.Tickets.Search(ctx, "error", &types.TicketListOptions{
    Priority: []string{"high", "urgent"},
})

// Close ticket
closedTicket, err := client.Tickets.Close(ctx, ticketID, "Issue resolved")
```

### Messages and Attachments

```go
// Add message
message, err := client.Tickets.AddMessage(ctx, ticketID, &types.MessageCreateRequest{
    Content:    "This is a response",
    IsInternal: false,
})

// Get attachments
attachments, err := client.Tickets.GetAttachments(ctx, ticketID)

// Download attachment
data, err := client.Tickets.DownloadAttachment(ctx, ticketID, attachmentID)
```

### User Management

```go
// List users
users, err := client.Users.List(ctx)

// Create user
user, err := client.Users.Create(ctx, &types.UserCreateRequest{
    Email:     "user@example.com",
    FirstName: "John",
    LastName:  "Doe",
    Role:      "agent",
})

// Get current user profile
profile, err := client.Auth.GetProfile(ctx)
```

### Dashboard & Analytics

```go
// Get dashboard statistics
stats, err := client.Dashboard.GetStats(ctx)
fmt.Printf("Open tickets: %d\n", stats.OpenTickets)

// Get my tickets
myTickets, err := client.Dashboard.GetMyTickets(ctx)
```

### LDAP Integration

```go
// Sync users from LDAP
result, err := client.LDAP.SyncUsers(ctx)
fmt.Printf("Synced %d users\n", result.UsersCreated)

// Get LDAP users
ldapUsers, err := client.LDAP.GetUsers(ctx)

// Test LDAP connection
err := client.LDAP.TestConnection(ctx)
```

### Webhooks

```go
// Create webhook
webhook, err := client.Webhooks.Create(ctx, &types.Webhook{
    Name:   "My Webhook",
    URL:    "https://example.com/webhook",
    Events: []string{"ticket.created", "ticket.updated"},
})

// Test webhook
err := client.Webhooks.Test(ctx, webhookID)

// Get webhook deliveries
deliveries, err := client.Webhooks.GetDeliveries(ctx, webhookID)
```

### Internal Notes

```go
// Create note
note, err := client.Notes.CreateNote(ctx, ticketID, &types.InternalNote{
    Content:     "Internal investigation notes",
    Category:    "Investigation",
    IsImportant: true,
})

// Get note templates
templates, err := client.Notes.GetTemplates(ctx)
```

## Error Handling

The SDK provides structured error handling:

```go
ticket, err := client.Tickets.Get(ctx, ticketID)
if err != nil {
    if goatflow.IsNotFound(err) {
        fmt.Println("Ticket not found")
        return
    }
    if goatflow.IsUnauthorized(err) {
        fmt.Println("Authentication failed")
        return
    }
    if goatflow.IsRateLimited(err) {
        fmt.Println("Rate limit exceeded")
        return
    }
    // Handle other errors
    log.Fatal(err)
}
```

### Error Types

- `goatflow.IsNotFound(err)` - 404 Not Found
- `goatflow.IsUnauthorized(err)` - 401 Unauthorized  
- `goatflow.IsForbidden(err)` - 403 Forbidden
- `goatflow.IsRateLimited(err)` - 429 Too Many Requests
- `goatflow.IsAPIError(err)` - Any API error

## Pagination

Most list operations support pagination:

```go
options := &types.TicketListOptions{
    Page:     1,
    PageSize: 50,
    SortBy:   "created_at",
    SortOrder: "desc",
}

tickets, err := client.Tickets.List(ctx, options)
fmt.Printf("Page %d of %d (Total: %d)\n", 
    tickets.Page, tickets.TotalPages, tickets.TotalCount)
```

## Context and Timeouts

All operations accept a context for cancellation and timeouts:

```go
// With timeout
ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
defer cancel()

tickets, err := client.Tickets.List(ctx, nil)

// With cancellation
ctx, cancel := context.WithCancel(context.Background())
go func() {
    time.Sleep(5 * time.Second)
    cancel() // Cancel operation after 5 seconds
}()

ticket, err := client.Tickets.Get(ctx, ticketID)
```

## Concurrent Operations

The SDK is thread-safe and supports concurrent operations:

```go
var wg sync.WaitGroup
results := make(chan *types.Ticket, 10)

// Create multiple tickets concurrently
for i := 0; i < 10; i++ {
    wg.Add(1)
    go func(index int) {
        defer wg.Done()
        ticket, err := client.Tickets.Create(ctx, &types.TicketCreateRequest{
            Title: fmt.Sprintf("Ticket %d", index),
            // ...
        })
        if err == nil {
            results <- ticket
        }
    }(i)
}

wg.Wait()
close(results)

for ticket := range results {
    fmt.Printf("Created ticket #%s\n", ticket.TicketNumber)
}
```

## Rate Limiting

The SDK automatically handles rate limiting with exponential backoff:

```go
// Configure retry behavior
client := goatflow.NewClient(&goatflow.Config{
    BaseURL:    "https://goatflow.example.com",
    Auth:       goatflow.NewAPIKeyAuth("your-api-key"),
    RetryCount: 5, // Retry up to 5 times
})
```

## Testing

The SDK includes comprehensive test coverage. Run tests with:

```bash
go test ./...
```

For integration tests against a live API:

```bash
export GOATFLOW_BASE_URL="https://your-test-instance.com"
export GOATFLOW_API_KEY="your-test-api-key"
go test -tags=integration ./...
```

## Examples

See the `examples/` directory for complete working examples:

- `basic_usage.go` - Basic CRUD operations
- `advanced_features.go` - Advanced features like webhooks and LDAP
- `error_handling.go` - Comprehensive error handling
- `concurrent_operations.go` - Concurrent API calls

## Contributing

1. Fork the repository
2. Create a feature branch
3. Add tests for new functionality
4. Ensure all tests pass
5. Submit a pull request

## License

MIT License - see LICENSE file for details.

## Support

- Documentation: https://docs.goatflow.io/sdk/go
- Issues: https://github.com/goatkit/goatflow/issues
- Discussions: https://github.com/goatkit/goatflow/discussions