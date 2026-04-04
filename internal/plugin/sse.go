package plugin

import (
	"fmt"
	"net/http"
	"sync"
	"time"
)

// SSEEvent represents a server-sent event.
type SSEEvent struct {
	Plugin  string // source plugin name
	Channel string // channel name (e.g. "status", "progress")
	Type    string // event type (e.g. "device-table")
	Data    string // event data (typically HTML fragment)
}

// sseSubscription holds the filter criteria for a subscribed client.
type sseSubscription struct {
	plugin  string // required plugin filter
	channel string // required channel filter
}

// SSEBroker manages SSE client connections and event broadcasting.
type SSEBroker struct {
	mu      sync.RWMutex
	clients map[chan SSEEvent]sseSubscription
}

// NewSSEBroker creates a new SSE broker.
func NewSSEBroker() *SSEBroker {
	return &SSEBroker{
		clients: make(map[chan SSEEvent]sseSubscription),
	}
}

// Subscribe adds a client filtered by plugin and channel.
// Both filters are required (non-empty) for per-plugin channel isolation.
func (b *SSEBroker) Subscribe(plugin, channel string) chan SSEEvent {
	ch := make(chan SSEEvent, 16)
	b.mu.Lock()
	b.clients[ch] = sseSubscription{plugin: plugin, channel: channel}
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

	for ch, sub := range b.clients {
		if sub.plugin != "" && sub.plugin != event.Plugin {
			continue
		}
		if sub.channel != "" && sub.channel != event.Channel {
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
//   - plugin: filter events to a specific plugin (optional, for legacy /api/v1/sse)
func (b *SSEBroker) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	b.serveSSE(w, r, r.URL.Query().Get("plugin"), "")
}

// ServeChannel handles SSE connections scoped to a specific plugin and channel.
func (b *SSEBroker) ServeChannel(w http.ResponseWriter, r *http.Request, pluginName, channel string) {
	b.serveSSE(w, r, pluginName, channel)
}

func (b *SSEBroker) serveSSE(w http.ResponseWriter, r *http.Request, pluginFilter, channelFilter string) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming not supported", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no") // disable nginx buffering

	ch := b.Subscribe(pluginFilter, channelFilter)
	defer b.Unsubscribe(ch)

	// Send initial connection event.
	fmt.Fprintf(w, "event: connected\ndata: {\"status\":\"ok\"}\n\n")
	flusher.Flush()

	// Keepalive ticker — send a comment every 30 seconds to prevent
	// proxies and browsers from closing idle connections.
	keepalive := time.NewTicker(30 * time.Second)
	defer keepalive.Stop()

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
		case <-keepalive.C:
			fmt.Fprintf(w, ": keepalive\n\n")
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
