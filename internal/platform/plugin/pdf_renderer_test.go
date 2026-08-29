package plugin

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/microcosm-cc/bluemonday"
	"github.com/yuin/goldmark"
)

// newFakeBrowserless spins up an httptest server that behaves like a
// Browserless /pdf endpoint: it records the HTML it was asked to print and
// returns canned PDF bytes.
func newFakeBrowserless(t *testing.T, wantPdf string) (*httptest.Server, *string) {
	t.Helper()
	var capturedHTML string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/pdf" {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		body, _ := io.ReadAll(r.Body)
		var req struct {
			HTML string `json:"html"`
		}
		if err := json.Unmarshal(body, &req); err != nil {
			http.Error(w, "bad json", http.StatusBadRequest)
			return
		}
		capturedHTML = req.HTML
		w.Header().Set("Content-Type", "application/pdf")
		_, _ = w.Write([]byte(wantPdf))
	}))
	t.Cleanup(ts.Close)
	return ts, &capturedHTML
}

// testRenderer builds a browserlessPdfRenderer pointed at the fake sidecar.
func testRenderer(ts *httptest.Server) *browserlessPdfRenderer {
	return &browserlessPdfRenderer{
		baseURL: ts.URL,
		client:  ts.Client(),
		md:      goldmark.New(),
		san:     bluemonday.UGCPolicy(),
	}
}

func TestRenderMarkdownToPdf(t *testing.T) {
	ts, captured := newFakeBrowserless(t, "%PDF-1.4 fake")
	h := NewProdHostAPI(WithPdfRenderer(testRenderer(ts)))

	pdf, err := h.RenderMarkdownToPdf(context.Background(), "# Hello\n\nSome *text*.", PdfRenderOptions{})
	if err != nil {
		t.Fatalf("RenderMarkdownToPdf: %v", err)
	}
	if string(pdf) != "%PDF-1.4 fake" {
		t.Fatalf("unexpected pdf bytes: %q", pdf)
	}
	if !strings.Contains(*captured, "<h1>Hello</h1>") {
		t.Errorf("rendered HTML missing heading; got: %s", *captured)
	}
	if !strings.Contains(*captured, "<em>text</em>") {
		t.Errorf("rendered HTML missing emphasis; got: %s", *captured)
	}
}

func TestRenderMarkdownToPdfSanitizesScript(t *testing.T) {
	ts, captured := newFakeBrowserless(t, "%PDF-1.4 fake")
	h := NewProdHostAPI(WithPdfRenderer(testRenderer(ts)))

	_, err := h.RenderMarkdownToPdf(context.Background(), "# Safe\n\n<script>alert(1)</script>", PdfRenderOptions{})
	if err != nil {
		t.Fatalf("RenderMarkdownToPdf: %v", err)
	}
	if strings.Contains(*captured, "<script") {
		t.Errorf("script tag not sanitized; got: %s", *captured)
	}
}

func TestRenderMarkdownToPdfUnconfigured(t *testing.T) {
	h := NewProdHostAPI()
	// ProdHostAPI defaults to a renderer, so forcing nil lets us check the guard.
	h.pdfRenderer = nil
	if _, err := h.RenderMarkdownToPdf(context.Background(), "# x", PdfRenderOptions{}); err == nil {
		t.Fatal("expected error when no renderer configured")
	}
}
