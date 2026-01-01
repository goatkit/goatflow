package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/gotrs-io/gotrs-ce/sdk/go/types"
)

func main() {
	// Initialize client with API key
	client := gotrs.NewClientWithAPIKey("https://your-gotrs-instance.com", "your-api-key")

	ctx := context.Background()

	// Test connection
	if err := client.Ping(ctx); err != nil {
		log.Fatalf("Failed to connect to GOTRS: %v", err)
	}
	fmt.Println("✅ Connected to GOTRS successfully")

	// List tickets
	fmt.Println("\n📋 Listing tickets...")
	tickets, err := client.Tickets.List(ctx, &types.TicketListOptions{
		PageSize: 10,
		Status:   []string{"open", "pending"},
	})
	if err != nil {
		log.Fatalf("Failed to list tickets: %v", err)
	}
	fmt.Printf("Found %d tickets\n", tickets.TotalCount)

	for _, ticket := range tickets.Tickets {
		fmt.Printf("- #%s: %s (Status: %s, Priority: %s)\n",
			ticket.TicketNumber, ticket.Title, ticket.Status, ticket.Priority)
	}

	// Create a new ticket
	fmt.Println("\n🎫 Creating a new ticket...")
	newTicket, err := client.Tickets.Create(ctx, &types.TicketCreateRequest{
		Title:       "SDK Test Ticket",
		Description: "This ticket was created using the Go SDK",
		Priority:    "normal",
		Type:        "incident",
		QueueID:     1,
		CustomerID:  1,
		Tags:        []string{"sdk", "test"},
	})
	if err != nil {
		log.Fatalf("Failed to create ticket: %v", err)
	}
	fmt.Printf("✅ Created ticket #%s with ID %d\n", newTicket.TicketNumber, newTicket.ID)

	// Add a message to the ticket
	fmt.Println("\n💬 Adding a message...")
	message, err := client.Tickets.AddMessage(ctx, newTicket.ID, &types.MessageCreateRequest{
		Content:     "This is a test message added via the SDK",
		MessageType: "note",
		IsInternal:  false,
	})
	if err != nil {
		log.Fatalf("Failed to add message: %v", err)
	}
	fmt.Printf("✅ Added message with ID %d\n", message.ID)

	// Update ticket priority
	fmt.Println("\n🔄 Updating ticket priority...")
	priority := "high"
	updatedTicket, err := client.Tickets.Update(ctx, newTicket.ID, &types.TicketUpdateRequest{
		Priority: &priority,
	})
	if err != nil {
		log.Fatalf("Failed to update ticket: %v", err)
	}
	fmt.Printf("✅ Updated ticket priority to %s\n", updatedTicket.Priority)

	// Assign ticket to user
	fmt.Println("\n👤 Assigning ticket...")
	assignedTicket, err := client.Tickets.Assign(ctx, newTicket.ID, 1) // Assign to user ID 1
	if err != nil {
		log.Fatalf("Failed to assign ticket: %v", err)
	}
	fmt.Printf("✅ Assigned ticket to user ID %d\n", *assignedTicket.AssignedTo)

	// Search tickets
	fmt.Println("\n🔍 Searching tickets...")
	searchResults, err := client.Tickets.Search(ctx, "SDK", &types.TicketListOptions{
		PageSize: 5,
	})
	if err != nil {
		log.Fatalf("Failed to search tickets: %v", err)
	}
	fmt.Printf("Found %d tickets matching 'SDK'\n", searchResults.TotalCount)

	// Get dashboard stats
	fmt.Println("\n📊 Getting dashboard statistics...")
	stats, err := client.Dashboard.GetStats(ctx)
	if err != nil {
		log.Fatalf("Failed to get dashboard stats: %v", err)
	}
	fmt.Printf("📈 Dashboard Stats:\n")
	fmt.Printf("  Total Tickets: %d\n", stats.TotalTickets)
	fmt.Printf("  Open Tickets: %d\n", stats.OpenTickets)
	fmt.Printf("  Closed Tickets: %d\n", stats.ClosedTickets)
	fmt.Printf("  My Tickets: %d\n", stats.MyTickets)

	// Get user profile
	fmt.Println("\n👤 Getting user profile...")
	profile, err := client.Auth.GetProfile(ctx)
	if err != nil {
		log.Fatalf("Failed to get profile: %v", err)
	}
	fmt.Printf("Logged in as: %s %s (%s)\n", profile.FirstName, profile.LastName, profile.Email)

	// Close the test ticket
	fmt.Println("\n🔒 Closing test ticket...")
	closedTicket, err := client.Tickets.Close(ctx, newTicket.ID, "Test completed")
	if err != nil {
		log.Fatalf("Failed to close ticket: %v", err)
	}
	fmt.Printf("✅ Closed ticket #%s\n", closedTicket.TicketNumber)

	fmt.Println("\n✨ SDK example completed successfully!")
}

// Example of error handling
func handleTicketOperations(client *gotrs.Client) {
	ctx := context.Background()

	// Example with proper error handling
	ticket, err := client.Tickets.Get(ctx, 12345)
	if err != nil {
		if gotrs.IsNotFound(err) {
			fmt.Println("Ticket not found")
			return
		}
		if gotrs.IsUnauthorized(err) {
			fmt.Println("Authentication failed")
			return
		}
		if gotrs.IsRateLimited(err) {
			fmt.Println("Rate limit exceeded, retrying later...")
			time.Sleep(time.Minute)
			return
		}
		log.Fatalf("Unexpected error: %v", err)
	}

	fmt.Printf("Found ticket: %s\n", ticket.Title)
}

// Example of using different authentication methods
func authenticationExamples() {
	// API Key authentication
	client1 := gotrs.NewClientWithAPIKey("https://gotrs.example.com", "your-api-key")

	// JWT authentication
	expiresAt := time.Now().Add(24 * time.Hour)
	client2 := gotrs.NewClientWithJWT("https://gotrs.example.com", "jwt-token", "refresh-token", expiresAt)

	// OAuth2 authentication
	oauth2Auth := gotrs.NewOAuth2Auth("access-token", "refresh-token", "Bearer", expiresAt)
	client3 := gotrs.NewClient(&gotrs.Config{
		BaseURL: "https://gotrs.example.com",
		Auth:    oauth2Auth,
		Timeout: 30 * time.Second,
		Debug:   true,
	})

	// Use the clients...
	_ = client1
	_ = client2
	_ = client3
}

// Example of concurrent operations
func concurrentOperations(client *gotrs.Client) {
	ctx := context.Background()

	// Create multiple tickets concurrently
	type ticketResult struct {
		ticket *types.Ticket
		err    error
	}

	results := make(chan ticketResult, 5)

	for i := 0; i < 5; i++ {
		go func(index int) {
			ticket, err := client.Tickets.Create(ctx, &types.TicketCreateRequest{
				Title:       fmt.Sprintf("Concurrent Ticket %d", index),
				Description: fmt.Sprintf("Created concurrently #%d", index),
				Priority:    "normal",
				QueueID:     1,
				CustomerID:  1,
			})
			results <- ticketResult{ticket: ticket, err: err}
		}(i)
	}

	// Collect results
	for i := 0; i < 5; i++ {
		result := <-results
		if result.err != nil {
			fmt.Printf("Failed to create ticket: %v\n", result.err)
		} else {
			fmt.Printf("Created ticket #%s\n", result.ticket.TicketNumber)
		}
	}
}
