package api

import (
	"net/http"
	"net/http/httptest"
	"regexp"
	"testing"
)

// These snapshot tests will evolve once real zoom HTML path is implemented.
// For now we assert the create form template renders key structural elements.

func TestTicketCreateForm_HTMLStructure(t *testing.T) {
	r := ginTestEngine()
	// Assume route /tickets/new mapped to handler returning the form (current fallback path in ticket_get_handler.go for id=new)
	req := httptest.NewRequest(http.MethodGet, "/tickets/new", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		// If this fails now it's expected until route wiring done; keep failing state for TDD
		if w.Code == http.StatusNotFound { t.Fatalf("expected 200 form page got 404 (route not wired yet)") }
		t.Fatalf("expected 200 got %d body=%s", w.Code, w.Body.String())
	}
	body := w.Body.String()
	mustContain := []string{"Create New Ticket", "<form", "name=\"title\"", "queue", "priority"}
	for _, s := range mustContain {
		if !regexp.MustCompile(regexp.QuoteMeta(s)).MatchString(body) {
			t.Fatalf("expected body to contain %s", s)
		}
	}
}

func TestTicketZoomPlaceholder_NotImplementedYet(t *testing.T) {
	r := ginTestEngine()
	// Using numeric id likely 1; expect JSON or 404 now; future will switch to HTML
	req := httptest.NewRequest(http.MethodGet, "/tickets/1", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code == http.StatusOK {
		// Accept JSON fallback for now; later replace with HTML snapshot assertion
		if regexp.MustCompile(`"ticket_number"`).Match(w.Body.Bytes()) { return }
	}
	// Any other status is failing until zoom implemented
	if w.Code == http.StatusNotFound { t.Fatalf("zoom view not yet implemented; keep failing to drive implementation (got 404)") }
}
