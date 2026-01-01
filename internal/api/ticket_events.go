package api

import (
	"encoding/json"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

// Ticket event management.
var ticketEventClients = struct {
	sync.RWMutex
	clients map[chan string]bool
}{
	clients: make(map[chan string]bool),
}

// handleTicketEvents provides Server-Sent Events for real-time ticket updates.
//
//nolint:unused
func handleTicketEvents(c *gin.Context) {
	// Set headers for SSE
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("X-Accel-Buffering", "no")

	// Create a channel for this client
	clientChan := make(chan string, 10)

	// Register the client
	ticketEventClients.Lock()
	ticketEventClients.clients[clientChan] = true
	ticketEventClients.Unlock()

	// Remove client on disconnect
	defer func() {
		ticketEventClients.Lock()
		delete(ticketEventClients.clients, clientChan)
		ticketEventClients.Unlock()
		close(clientChan)
	}()

	// Send initial connection message
	c.SSEvent("connected", "Connected to ticket event stream")
	c.Writer.Flush()

	// Send heartbeat every 30 seconds to keep connection alive
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	// Keep connection alive and send events
	for {
		select {
		case msg := <-clientChan:
			c.SSEvent("ticket-update", msg)
			c.Writer.Flush()
		case <-ticker.C:
			c.SSEvent("heartbeat", "ping")
			c.Writer.Flush()
		case <-c.Request.Context().Done():
			return
		}
	}
}

// BroadcastTicketUpdate sends an update to all connected clients.
func BroadcastTicketUpdate(eventType string, ticketData interface{}) {
	ticketEventClients.RLock()
	defer ticketEventClients.RUnlock()

	// Create JSON message
	data, _ := json.Marshal(map[string]interface{}{
		"type":      eventType,
		"data":      ticketData,
		"timestamp": time.Now().Unix(),
	})

	message := string(data)

	// Send to all connected clients
	for client := range ticketEventClients.clients {
		select {
		case client <- message:
		default:
			// Client's channel is full, skip
		}
	}
}
