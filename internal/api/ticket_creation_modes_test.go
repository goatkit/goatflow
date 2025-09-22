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

// helper to perform requests
func performRequest(r http.Handler, method, path string, body *strings.Reader, headers map[string]string) *httptest.ResponseRecorder {
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
	SetupHTMXRoutes(r) // no JWT manager -> test auth injection path
	return r
}

func TestTicketCreationModes_Redirect(t *testing.T) {
	t.Setenv("APP_ENV", "test")
	r := setupTicketCreationTestRouter(t)

	w := performRequest(r, http.MethodGet, "/ticket/new", nil, nil)
	if w.Code != http.StatusFound { // 302
		t.Fatalf("expected 302 redirect, got %d body=%s", w.Code, w.Body.String())
	}
	loc := w.Header().Get("Location")
	if loc != "/ticket/new/email" {
		t.Fatalf("expected redirect to /ticket/new/email got %s", loc)
	}
}

func TestTicketCreationModes_GetEmailForm(t *testing.T) {
	t.Setenv("APP_ENV", "test")
	r := setupTicketCreationTestRouter(t)

	w := performRequest(r, http.MethodGet, "/ticket/new/email", nil, nil)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 got %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), "Email Ticket Creation") {
		t.Fatalf("email form missing marker text; body=%s", w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "name=\"customer_email\"") {
		t.Fatalf("expected email field in email ticket form")
	}
}

func TestTicketCreationModes_GetPhoneForm(t *testing.T) {
	t.Setenv("APP_ENV", "test")
	r := setupTicketCreationTestRouter(t)

	w := performRequest(r, http.MethodGet, "/ticket/new/phone", nil, nil)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 got %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), "Phone Ticket Creation") {
		t.Fatalf("phone form missing marker text; body=%s", w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "name=\"customer_phone\"") {
		t.Fatalf("expected phone field in phone ticket form")
	}
}

func TestTicketCreationModes_CreateEmailTicket(t *testing.T) {
	t.Setenv("APP_ENV", "test")
	r := setupTicketCreationTestRouter(t)

	form := url.Values{}
	form.Set("subject", "Email subject")
	form.Set("body", "Email body")
	form.Set("customer_email", "user@example.com")
	form.Set("channel", "email")

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
	if resp["channel"] != "email" {
		t.Fatalf("expected channel=email got %v", resp["channel"])
	}
}

func TestTicketCreationModes_CreatePhoneTicketRequiresPhone(t *testing.T) {
	t.Setenv("APP_ENV", "test")
	r := setupTicketCreationTestRouter(t)

	// Missing phone should fail
	form := url.Values{}
	form.Set("subject", "Phone subject")
	form.Set("body", "Phone body")
	form.Set("channel", "phone")

	w := performRequest(r, http.MethodPost, "/api/tickets", strings.NewReader(form.Encode()), map[string]string{
		"Content-Type": "application/x-www-form-urlencoded",
	})
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 got %d body=%s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "customerphone is required") {
		t.Fatalf("expected customerphone error; body=%s", w.Body.String())
	}

	// Provide phone (no email) should succeed
	form2 := url.Values{}
	form2.Set("subject", "Phone subject")
	form2.Set("body", "Phone body")
	form2.Set("channel", "phone")
	form2.Set("customer_phone", "+4412345")

	w2 := performRequest(r, http.MethodPost, "/api/tickets", strings.NewReader(form2.Encode()), map[string]string{
		"Content-Type": "application/x-www-form-urlencoded",
	})
	if w2.Code != http.StatusCreated {
		t.Fatalf("expected 201 got %d body=%s", w2.Code, w2.Body.String())
	}
	var resp map[string]any
	_ = json.Unmarshal(w2.Body.Bytes(), &resp)
	if resp["channel"] != "phone" {
		t.Fatalf("expected channel=phone got %v", resp["channel"])
	}
}
