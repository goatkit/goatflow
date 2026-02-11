package plugin

import (
	"fmt"
	"net/http"
	"sync"
)

// SSEEvent represents a server-sent event.
type SSEEvent struct {
	Plugin string // source plugin name
	Type   string // event type (e.g. "device-table")
	Data   string // event data (typically HTML fragment)
}

// SSEBroker manages SSE client connections and event broadcasting.
type SSEBroker struct {
	mu      sync.RWMutex
	clients map[chan SSEEvent]string // channel -> plugin filter ("" = all)
}

// NewSSEBroker creates a new SSE broker.
func NewSSEBroker() *SSEBroker {
	return &SSEBroker{
		clients: make(map[chan SSEEvent]string),
	}
}

// Subscribe adds a client and returns its event channel. The pluginFilter
// limits events to a specific plugin ("" receives all).
func (b *SSEBroker) Subscribe(pluginFilter string) chan SSEEvent {
	ch := make(chan SSEEvent, 16)
	b.mu.Lock()
	b.clients[ch] = pluginFilter
	b.mu.Unlock()
	return ch
}

// Unsubscribe removes a client channel.
func (b *SSEBroker) Unsubscribe(ch chan SSEEvent) {
	b.mu.Lock()
	delete(b.clients, ch)
	b.mu.Unlock()
	close(ch)
}

// Publish sends an event to all matching clients. Non-blocking: slow clients
// have their events dropped rather than blocking the publisher.
func (b *SSEBroker) Publish(event SSEEvent) {
	b.mu.RLock()
	defer b.mu.RUnlock()

	for ch, filter := range b.clients {
		if filter != "" && filter != event.Plugin {
			continue
		}
		select {
		case ch <- event:
		default:
			// Client too slow, drop event
		}
	}
}

// ServeHTTP handles SSE client connections. Query params:
//   - plugin: filter events to a specific plugin (optional)
func (b *SSEBroker) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming not supported", http.StatusInternalServerError)
		return
	}

	pluginFilter := r.URL.Query().Get("plugin")

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no") // disable nginx buffering

	ch := b.Subscribe(pluginFilter)
	defer b.Unsubscribe(ch)

	// Send initial connection event.
	fmt.Fprintf(w, "event: connected\ndata: {\"status\":\"ok\"}\n\n")
	flusher.Flush()

	for {
		select {
		case <-r.Context().Done():
			return
		case event, ok := <-ch:
			if !ok {
				return
			}
			fmt.Fprintf(w, "event: %s\ndata: %s\n\n", event.Type, event.Data)
			flusher.Flush()
		}
	}
}

// ClientCount returns the number of connected SSE clients.
func (b *SSEBroker) ClientCount() int {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return len(b.clients)
}
