# GoatFlow Go SDK

Go client for the GoatFlow REST API (`/api/v1`).

## Installation

```bash
go get github.com/goatkit/goatflow/sdk/go
```

## Quick start

```go
package main

import (
    "context"
    "fmt"
    "log"

    "github.com/goatkit/goatflow/sdk/go/client"
    "github.com/goatkit/goatflow/sdk/go/types"
)

func main() {
    gf := client.NewClientWithAPIKey("https://goatflow.example.com", "gf_...")
    ctx := context.Background()

    tickets, err := gf.Tickets.List(ctx, &types.TicketListOptions{Status: "open", PerPage: 20})
    if err != nil {
        log.Fatal(err)
    }
    fmt.Printf("%d open tickets\n", tickets.Pagination.Total)
    for _, t := range tickets.Tickets {
        fmt.Printf("#%s %s (%s)\n", t.TicketNumber, t.Title, t.StateName)
    }
}
```

A runnable walkthrough is in [`examples/basic_usage.go`](examples/basic_usage.go):

```bash
GOATFLOW_URL=https://goatflow.example.com GOATFLOW_TOKEN=gf_... go run ./examples
```

## Authentication

Both kinds of credential are sent as `Authorization: Bearer <token>`.

```go
// API token (gf_...), from the API Tokens settings page or POST /api/v1/tokens
gf := client.NewClientWithAPIKey(baseURL, "gf_...")

// JWT: Login switches the client to the returned access token
gf := client.NewClient(&client.Config{BaseURL: baseURL})
pair, err := gf.Login(ctx, "agent@example.com", "password")

// or resume from a stored pair
gf := client.NewClientWithJWT(baseURL, accessToken, refreshToken, expiresAt)

// explicit refresh: new access token and a rotated refresh token
pair, err = gf.Auth.Refresh(ctx, pair.RefreshToken)
```

A client from `Login` or `NewClientWithJWT` renews its token pair through `POST /api/v1/auth/refresh` when the access token is within a minute of expiry; concurrent requests share one refresh. A rejected refresh token (401) fails the request. For your own renewal logic pass any `auth.RefreshFunc` to `auth.NewJWTAuth` (`gf.Auth.RefreshFunc()` is the built-in one).

## Configuration

```go
gf := client.NewClient(&client.Config{
    BaseURL:    "https://goatflow.example.com",
    Auth:       auth.NewAPIKeyAuth("gf_..."),
    Timeout:    30 * time.Second, // default
    RetryCount: 0,                // retries after transport errors only; default 0
    UserAgent:  "my-app/1.0",
    Debug:      false,
})
```

## Services

### Tickets

```go
list, err := gf.Tickets.List(ctx, &types.TicketListOptions{
    QueueID: 3, Status: "open", Sort: "updated", Order: "desc",
    Include: []string{"article_count", "last_article"},
})
ticket, err := gf.Tickets.Get(ctx, 123)

created, err := gf.Tickets.Create(ctx, &types.TicketCreateRequest{
    Title: "Printer on fire", QueueID: 3, Body: "Smoke everywhere", PriorityID: 4,
})

// closedID: the id of the "closed successful" state from GET /api/v1/states
// (ids differ between installations, so resolve by name)
updated, err := gf.Tickets.Update(ctx, created.ID, &types.TicketUpdateRequest{StateID: &closedID})
reopened, err := gf.Tickets.Reopen(ctx, created.ID, "Customer replied")
err = gf.Tickets.Delete(ctx, created.ID)
```

### Articles

```go
articles, err := gf.Articles.List(ctx, ticketID, true) // with attachment lists
note, err := gf.Articles.Create(ctx, ticketID, &types.ArticleCreateRequest{
    Subject: "Call back", Body: "Customer asked for a call", ArticleType: "note-internal",
})
body := "Corrected text"
_, err = gf.Articles.Update(ctx, ticketID, note.ID, &types.ArticleUpdateRequest{Body: &body})
err = gf.Articles.Delete(ctx, ticketID, note.ID)
```

### Users, queues, statistics, search

```go
me, err := gf.Users.Me(ctx)
agents, err := gf.Users.List(ctx, &types.UserListOptions{Search: "smith", Valid: "1"})
agent, err := gf.Users.Get(ctx, 5)

queues, err := gf.Queues.List(ctx, &types.QueueListOptions{IncludeStats: true})
queue, err := gf.Queues.Get(ctx, 3)

stats, err := gf.Statistics.Dashboard(ctx)

results, err := gf.Search.Query(ctx, &types.SearchQuery{Query: "printer", Types: []string{"ticket"}})
```

### Webhooks

Admin only. Deliveries carry `X-Webhook-Event`, `X-Webhook-Delivery` and, with a
secret, `X-Webhook-Signature: sha256=<hex HMAC-SHA256 of the raw body>`.

```go
// Create webhook (signed)
secret := "a-signing-secret-of-16+-chars"
webhook, err := gf.Webhooks.Create(ctx, &types.Webhook{
    Name:   "My Webhook",
    URL:    "https://example.com/webhook",
    Events: []string{"ticket.created", "ticket.closed", "article.created"},
    Secret: &secret,
})

// Send a webhook.test event now
delivery, err := gf.Webhooks.Test(ctx, webhook.ID)

// Delivery log, one delivery with payload/response, and redelivery
deliveries, err := gf.Webhooks.GetDeliveries(ctx, webhook.ID)
detail, err := gf.Webhooks.GetDelivery(ctx, deliveries[0].ID)
again, err := gf.Webhooks.Redeliver(ctx, detail.ID)
```

### Other endpoints

`Get`, `Post`, `Put` and `Delete` on the client apply the same envelope and error handling to any path:

```go
var priorities []map[string]interface{}
err := gf.Get(ctx, "/api/v1/priorities", &priorities)
```

## Errors

Error responses (HTTP status outside 2xx, or `{"success": false}`) are returned as `*errors.APIError` with `StatusCode`, `Message` and, when the API sent one, `Code` (e.g. `core:invalid_token`). Requests without an HTTP response return `*errors.NetworkError`; a 2xx body that does not match the expected type returns `*errors.DecodeError`.

```go
import sdkerrors "github.com/goatkit/goatflow/sdk/go/errors"

ticket, err := gf.Tickets.Get(ctx, 123)
switch {
case sdkerrors.IsNotFound(err):
    // 404
case sdkerrors.IsUnauthorized(err):
    // 401: missing, invalid or expired token
case err != nil:
    if apiErr, ok := sdkerrors.AsAPIError(err); ok {
        log.Printf("HTTP %d: %s (%s)", apiErr.StatusCode, apiErr.Message, apiErr.Code)
    }
}
```

## Testing

```bash
go vet ./... && go test ./...   # from sdk/go
make test-sdk-go                # same, in the toolbox container (repo root)
```

The tests run the client against `httptest` servers that answer with the response bodies in [`../testdata`](../testdata).
