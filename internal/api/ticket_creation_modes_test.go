package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

// helper to perform requests.
func performRequest(r http.Handler, method, path string, body *strings.Reader, headers map[string]string) *httptest.ResponseRecorder {
	if body == nil {
		empty := strings.NewReader("")
		body = empty
	}
	req := httptest.NewRequest(method, path, body)
	if headers != nil {
		for k, v := range headers {
			req.Header.Set(k, v)
		}
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func setupTicketCreationTestRouter(t *testing.T) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	r := gin.Default()
	SetupHTMXRoutes(r)
	return r
}

func TestTicketCreationModes_NewTicketForm(t *testing.T) {
	t.Setenv("APP_ENV", "test")
	WithCleanDB(t)
	r := setupTicketCreationTestRouter(t)

	w := performRequest(r, http.MethodGet, "/ticket/new", nil, nil)
	// With DB available, /ticket/new renders the full form directly (200 OK)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", w.Code, w.Body.String())
	}
	// Should contain form elements for ticket creation
	body := w.Body.String()
	if !strings.Contains(body, "queue") && !strings.Contains(body, "Queue") {
		t.Fatalf("expected queue selection in form; body=%s", body[:min(500, len(body))])
	}
}

func TestTicketCreationModes_GetEmailForm(t *testing.T) {
	t.Setenv("APP_ENV", "test")
	WithCleanDB(t)
	r := setupTicketCreationTestRouter(t)

	w := performRequest(r, http.MethodGet, "/ticket/new/email", nil, nil)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 got %d body=%s", w.Code, w.Body.String())
	}
	body := w.Body.String()
	// Email form should have email-specific elements
	if !strings.Contains(body, "email") && !strings.Contains(body, "Email") {
		t.Fatalf("expected email references in form; body=%s", body[:min(500, len(body))])
	}
}

func TestTicketCreationModes_GetPhoneForm(t *testing.T) {
	t.Setenv("APP_ENV", "test")
	WithCleanDB(t)
	r := setupTicketCreationTestRouter(t)

	w := performRequest(r, http.MethodGet, "/ticket/new/phone", nil, nil)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 got %d body=%s", w.Code, w.Body.String())
	}
	body := w.Body.String()
	// Phone form should have phone-specific elements
	if !strings.Contains(body, "phone") && !strings.Contains(body, "Phone") {
		t.Fatalf("expected phone references in form; body=%s", body[:min(500, len(body))])
	}
}

func TestTicketCreationModes_CreateEmailTicket(t *testing.T) {
	t.Setenv("APP_ENV", "test")
	WithCleanDB(t)
	r := setupTicketCreationTestRouter(t)

	form := url.Values{}
	form.Set("subject", "Email subject")
	form.Set("body", "Email body")
	form.Set("customer_email", "user@example.com")
	form.Set("queue_id", "1")

	w := performRequest(r, http.MethodPost, "/api/tickets", strings.NewReader(form.Encode()), map[string]string{
		"Content-Type": "application/x-www-form-urlencoded",
	})
	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201 got %d body=%s", w.Code, w.Body.String())
	}
	var resp map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	if resp["success"] != true {
		t.Fatalf("expected success=true resp=%v", resp)
	}
}

func TestTicketCreationModes_RequiresCustomerEmail(t *testing.T) {
	t.Setenv("APP_ENV", "test")
	WithCleanDB(t)
	r := setupTicketCreationTestRouter(t)

	// Missing customer_email should fail
	form := url.Values{}
	form.Set("subject", "Test subject")
	form.Set("body", "Test body")
	form.Set("queue_id", "1")

	w := performRequest(r, http.MethodPost, "/api/tickets", strings.NewReader(form.Encode()), map[string]string{
		"Content-Type": "application/x-www-form-urlencoded",
	})
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 got %d body=%s", w.Code, w.Body.String())
	}
	// Verify the error mentions customer_email
	if !strings.Contains(w.Body.String(), "customer_email") {
		t.Fatalf("expected customer_email error; body=%s", w.Body.String())
	}
}
