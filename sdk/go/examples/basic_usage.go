// Command basic_usage walks through the GoatFlow Go SDK against a live server.
//
//	GOATFLOW_URL=https://goatflow.example.com GOATFLOW_TOKEN=gf_... go run ./examples
//
// Set GOATFLOW_QUEUE_ID to also create a ticket in that queue, add a note to
// it and close it again.
package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"strconv"

	"github.com/goatkit/goatflow/sdk/go/client"
	sdkerrors "github.com/goatkit/goatflow/sdk/go/errors"
	"github.com/goatkit/goatflow/sdk/go/types"
)

func main() {
	baseURL, token := os.Getenv("GOATFLOW_URL"), os.Getenv("GOATFLOW_TOKEN")
	if baseURL == "" || token == "" {
		log.Fatal("set GOATFLOW_URL and GOATFLOW_TOKEN (an API token, gf_...)")
	}
	gf := client.NewClientWithAPIKey(baseURL, token)
	ctx := context.Background()

	health, err := gf.Health(ctx)
	if err != nil {
		log.Fatalf("health check: %v", err)
	}
	fmt.Printf("GoatFlow %s is %s\n", health.Version, health.Status)

	me, err := gf.Users.Me(ctx)
	if err != nil {
		log.Fatalf("who am I: %v", err)
	}
	fmt.Printf("Authenticated as %s (%s %s)\n", me.Login, me.FirstName, me.LastName)

	tickets, err := gf.Tickets.List(ctx, &types.TicketListOptions{PerPage: 5, Status: "open"})
	if err != nil {
		log.Fatalf("list tickets: %v", err)
	}
	fmt.Printf("%d open tickets, showing %d:\n", tickets.Pagination.Total, len(tickets.Tickets))
	for _, t := range tickets.Tickets {
		fmt.Printf("  #%s %s [%s, %s, %s]\n", t.TicketNumber, t.Title, t.QueueName, t.StateName, t.PriorityName)
	}

	queues, err := gf.Queues.List(ctx, nil)
	if err != nil {
		log.Fatalf("list queues: %v", err)
	}
	fmt.Printf("%d queues readable\n", len(queues))

	if _, err := gf.Tickets.Get(ctx, 999999999); sdkerrors.IsNotFound(err) {
		fmt.Println("Ticket 999999999 does not exist (404 as *errors.APIError)")
	}

	queueID, _ := strconv.ParseUint(os.Getenv("GOATFLOW_QUEUE_ID"), 10, 32) //nolint:errcheck // unset means skip
	if queueID == 0 {
		return
	}

	created, err := gf.Tickets.Create(ctx, &types.TicketCreateRequest{
		Title:   "SDK example ticket",
		QueueID: uint(queueID),
		Body:    "Created by the GoatFlow Go SDK example.",
	})
	if err != nil {
		log.Fatalf("create ticket: %v", err)
	}
	fmt.Printf("Created ticket #%s (id %d)\n", created.TN, created.ID)

	note, err := gf.Articles.Create(ctx, created.ID, &types.ArticleCreateRequest{
		Subject:     "Internal note",
		Body:        "Added by the SDK example.",
		ArticleType: "note-internal",
	})
	if err != nil {
		log.Fatalf("add note: %v", err)
	}
	fmt.Printf("Added article %d (%s)\n", note.ID, note.ArticleType)

	// State ids differ between installations; resolve the state by name.
	var states []struct {
		ID   uint   `json:"id"`
		Name string `json:"name"`
	}
	if err := gf.Get(ctx, "/api/v1/states", &states); err != nil {
		log.Fatalf("list states: %v", err)
	}
	var closedState uint
	for _, s := range states {
		if s.Name == "closed successful" {
			closedState = s.ID
		}
	}
	if closedState == 0 {
		log.Fatal(`no "closed successful" ticket state`)
	}
	updated, err := gf.Tickets.Update(ctx, created.ID, &types.TicketUpdateRequest{StateID: &closedState})
	if err != nil {
		log.Fatalf("close ticket: %v", err)
	}
	fmt.Printf("Ticket %d now in state %d\n", updated.ID, updated.StateID)
}
