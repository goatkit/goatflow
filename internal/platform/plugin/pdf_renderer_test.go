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

func TestBrandingCSS(t *testing.T) {
	got := brandingCSS(PdfRenderOptions{BrandColor: "#7c3aed"})
	if !strings.Contains(got, "color:#7c3aed") || !strings.Contains(got, "rgba(124,58,237,0.12)") {
		t.Errorf("brandingCSS = %q", got)
	}
	for _, bad := range []string{"", "red", "#7c3aed;", "#7c3a", "#gggggg", "expression(alert(1))", "#7C3AED extra", " #7c3aed"} {
		if out := brandingCSS(PdfRenderOptions{BrandColor: bad}); out != "" {
			t.Errorf("brandingCSS(%q) = %q, want empty", bad, out)
		}
	}
}

func TestBrandHeader(t *testing.T) {
	h := brandHeader(PdfRenderOptions{
		BrandName:    `Copperforge <script>x</script>`,
		BrandLogoURL: "https://example.com/logo.png",
	})
	if strings.Contains(h, "<script>") {
		t.Errorf("name not escaped: %s", h)
	}
	if !strings.Contains(h, "Copperforge &lt;script&gt;") || !strings.Contains(h, `src="https://example.com/logo.png"`) {
		t.Errorf("brand header missing escaped name or logo: %s", h)
	}
	for _, bad := range []string{"http://evil.example/logo.png", "javascript:alert(1)", `https://e.example/a" onload="x`, "https://e.example/a b"} {
		if out := brandHeader(PdfRenderOptions{BrandLogoURL: bad}); out != "" {
			t.Errorf("logo %q must be dropped, got %q", bad, out)
		}
	}
}

func TestWrapPDFDocumentBranding(t *testing.T) {
	doc := wrapPDFDocument([]byte("<p>hi</p>"), PdfRenderOptions{BrandColor: "#7c3aed"})
	if !strings.Contains(string(doc), "rgba(124,58,237,0.12)") {
		t.Errorf("branding CSS not applied: %s", doc)
	}
	plain := wrapPDFDocument([]byte("<p>hi</p>"), PdfRenderOptions{})
	if strings.Contains(string(plain), "rgba(124") {
		t.Errorf("zero-value options changed rendering: %s", plain)
	}
}

func TestPdfPageOptionsBrandingHeader(t *testing.T) {
	// branding without title still turns the header on, without separator
	opts := pdfPageOptions(PdfRenderOptions{BrandName: "Acme Coaching"})
	if opts["displayHeaderFooter"] != true {
		t.Fatal("branding-only header not enabled")
	}
	h := opts["headerTemplate"].(string)
	if !strings.Contains(h, "Acme Coaching") || strings.Contains(h, "—") {
		t.Errorf("branding-only header wrong: %s", h)
	}
	// with title: name — title
	opts = pdfPageOptions(PdfRenderOptions{BrandName: "Acme Coaching", Title: "Action Plan"})
	h = opts["headerTemplate"].(string)
	if !strings.Contains(h, "Acme Coaching</span> — Action Plan") {
		t.Errorf("brand+title header wrong: %s", h)
	}
	// zero values: unchanged (title-only header as before, no brand spans)
	opts = pdfPageOptions(PdfRenderOptions{Title: "Action Plan"})
	h = opts["headerTemplate"].(string)
	if !strings.Contains(h, "Action Plan") || strings.Contains(h, "<span") {
		t.Errorf("title-only header changed: %s", h)
	}
	if _, ok := pdfPageOptions(PdfRenderOptions{})["headerTemplate"]; ok {
		t.Error("zero-value options must not emit a header")
	}
}
