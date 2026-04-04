package plugin

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestNewSSEBroker(t *testing.T) {
	b := NewSSEBroker()
	if b == nil {
		t.Fatal("expected non-nil broker")
	}
	if b.ClientCount() != 0 {
		t.Errorf("expected 0 clients, got %d", b.ClientCount())
	}
}

func TestSSEBrokerSubscribeUnsubscribe(t *testing.T) {
	b := NewSSEBroker()

	ch := b.Subscribe("myplugin", "status")
	if b.ClientCount() != 1 {
		t.Errorf("expected 1 client, got %d", b.ClientCount())
	}

	b.Unsubscribe(ch)
	if b.ClientCount() != 0 {
		t.Errorf("expected 0 clients after unsubscribe, got %d", b.ClientCount())
	}
}

func TestSSEBrokerPublishMatchesPluginAndChannel(t *testing.T) {
	b := NewSSEBroker()

	// Subscribe to specific plugin + channel
	ch := b.Subscribe("myplugin", "progress")
	defer b.Unsubscribe(ch)

	// Matching event
	b.Publish(SSEEvent{Plugin: "myplugin", Channel: "progress", Type: "update", Data: "50%"})

	select {
	case ev := <-ch:
		if ev.Type != "update" || ev.Data != "50%" {
			t.Errorf("unexpected event: %+v", ev)
		}
	case <-time.After(100 * time.Millisecond):
		t.Fatal("timed out waiting for matching event")
	}

	// Non-matching plugin — should not deliver
	b.Publish(SSEEvent{Plugin: "other", Channel: "progress", Type: "x", Data: "y"})

	select {
	case ev := <-ch:
		t.Fatalf("should not have received event from wrong plugin: %+v", ev)
	case <-time.After(50 * time.Millisecond):
		// expected
	}

	// Non-matching channel — should not deliver
	b.Publish(SSEEvent{Plugin: "myplugin", Channel: "status", Type: "x", Data: "y"})

	select {
	case ev := <-ch:
		t.Fatalf("should not have received event from wrong channel: %+v", ev)
	case <-time.After(50 * time.Millisecond):
		// expected
	}
}

func TestSSEBrokerPublishEmptyFilterReceivesAll(t *testing.T) {
	b := NewSSEBroker()

	// Empty filters = receive all events (legacy /api/v1/sse behaviour)
	ch := b.Subscribe("", "")
	defer b.Unsubscribe(ch)

	b.Publish(SSEEvent{Plugin: "a", Channel: "x", Type: "ev1", Data: "d1"})
	b.Publish(SSEEvent{Plugin: "b", Channel: "y", Type: "ev2", Data: "d2"})

	for i := 0; i < 2; i++ {
		select {
		case <-ch:
		case <-time.After(100 * time.Millisecond):
			t.Fatalf("timed out on event %d", i)
		}
	}
}

func TestSSEBrokerPublishPluginOnlyFilter(t *testing.T) {
	b := NewSSEBroker()

	// Subscribe to plugin only, all channels
	ch := b.Subscribe("myplugin", "")
	defer b.Unsubscribe(ch)

	b.Publish(SSEEvent{Plugin: "myplugin", Channel: "any", Type: "ev", Data: "d"})

	select {
	case ev := <-ch:
		if ev.Channel != "any" {
			t.Errorf("unexpected channel: %s", ev.Channel)
		}
	case <-time.After(100 * time.Millisecond):
		t.Fatal("timed out")
	}

	// Wrong plugin still filtered
	b.Publish(SSEEvent{Plugin: "other", Channel: "any", Type: "ev", Data: "d"})
	select {
	case <-ch:
		t.Fatal("should not receive event from different plugin")
	case <-time.After(50 * time.Millisecond):
	}
}

func TestSSEBrokerDropsSlowClient(t *testing.T) {
	b := NewSSEBroker()
	ch := b.Subscribe("p", "c")
	defer b.Unsubscribe(ch)

	// Fill the buffer (16) and one more — should not block
	for i := 0; i < 20; i++ {
		b.Publish(SSEEvent{Plugin: "p", Channel: "c", Type: "ev", Data: "d"})
	}

	// Drain what we can — should be exactly 16 (buffer size)
	count := 0
	for {
		select {
		case <-ch:
			count++
		default:
			goto done
		}
	}
done:
	if count != 16 {
		t.Errorf("expected 16 buffered events, got %d", count)
	}
}

func TestSSEBrokerServeHTTP(t *testing.T) {
	b := NewSSEBroker()

	// Publish an event shortly after connection
	go func() {
		time.Sleep(20 * time.Millisecond)
		b.Publish(SSEEvent{Plugin: "p1", Channel: "ch1", Type: "hello", Data: `{"msg":"hi"}`})
	}()

	req := httptest.NewRequest("GET", "/api/v1/sse?plugin=p1", nil)
	ctx, cancel := context.WithTimeout(req.Context(), 100*time.Millisecond)
	defer cancel()
	req = req.WithContext(ctx)

	w := httptest.NewRecorder()
	b.ServeHTTP(w, req)

	body := w.Body.String()

	if !strings.Contains(body, "event: connected") {
		t.Error("missing connected event")
	}
	if !strings.Contains(body, "event: hello") {
		t.Error("missing hello event")
	}
	if !strings.Contains(body, `{"msg":"hi"}`) {
		t.Error("missing event data")
	}

	ct := w.Header().Get("Content-Type")
	if ct != "text/event-stream" {
		t.Errorf("expected text/event-stream, got %s", ct)
	}
}

func TestSSEBrokerServeChannel(t *testing.T) {
	b := NewSSEBroker()

	// Publish matching and non-matching events
	go func() {
		time.Sleep(20 * time.Millisecond)
		b.Publish(SSEEvent{Plugin: "myplugin", Channel: "status", Type: "ok", Data: "1"})
		b.Publish(SSEEvent{Plugin: "myplugin", Channel: "other", Type: "no", Data: "2"})
		b.Publish(SSEEvent{Plugin: "wrong", Channel: "status", Type: "no", Data: "3"})
	}()

	req := httptest.NewRequest("GET", "/api/v1/plugins/myplugin/events/status", nil)
	ctx, cancel := context.WithTimeout(req.Context(), 100*time.Millisecond)
	defer cancel()
	req = req.WithContext(ctx)

	w := httptest.NewRecorder()
	b.ServeChannel(w, req, "myplugin", "status")

	body := w.Body.String()

	if !strings.Contains(body, "event: connected") {
		t.Error("missing connected event")
	}
	if !strings.Contains(body, "event: ok") {
		t.Error("missing matching event")
	}
	if strings.Contains(body, "event: no") {
		t.Error("should not contain non-matching events")
	}
}

func TestSSEBrokerServeHTTPNoFlusher(t *testing.T) {
	b := NewSSEBroker()

	req := httptest.NewRequest("GET", "/api/v1/sse", nil)
	w := &noFlushResponseWriter{header: make(http.Header)}

	b.ServeHTTP(w, req)

	if w.code != http.StatusInternalServerError {
		t.Errorf("expected 500, got %d", w.code)
	}
}

func TestSSEEventStruct(t *testing.T) {
	ev := SSEEvent{
		Plugin:  "test",
		Channel: "progress",
		Type:    "update",
		Data:    "50%",
	}
	if ev.Plugin != "test" || ev.Channel != "progress" || ev.Type != "update" || ev.Data != "50%" {
		t.Errorf("unexpected event fields: %+v", ev)
	}
}

func TestSSEBrokerMultipleClients(t *testing.T) {
	b := NewSSEBroker()

	ch1 := b.Subscribe("p", "c")
	ch2 := b.Subscribe("p", "c")
	ch3 := b.Subscribe("p", "other")
	defer b.Unsubscribe(ch1)
	defer b.Unsubscribe(ch2)
	defer b.Unsubscribe(ch3)

	if b.ClientCount() != 3 {
		t.Errorf("expected 3 clients, got %d", b.ClientCount())
	}

	b.Publish(SSEEvent{Plugin: "p", Channel: "c", Type: "ev", Data: "d"})

	// ch1 and ch2 should receive, ch3 should not
	for _, ch := range []chan SSEEvent{ch1, ch2} {
		select {
		case <-ch:
		case <-time.After(100 * time.Millisecond):
			t.Fatal("matching client did not receive event")
		}
	}

	select {
	case <-ch3:
		t.Fatal("non-matching client should not receive event")
	case <-time.After(50 * time.Millisecond):
	}
}

// noFlushResponseWriter is an http.ResponseWriter that does NOT implement http.Flusher.
type noFlushResponseWriter struct {
	header http.Header
	code   int
	body   []byte
}

func (w *noFlushResponseWriter) Header() http.Header         { return w.header }
func (w *noFlushResponseWriter) Write(b []byte) (int, error) { w.body = append(w.body, b...); return len(b), nil }
func (w *noFlushResponseWriter) WriteHeader(code int)         { w.code = code }
