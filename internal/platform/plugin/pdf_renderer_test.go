package plugin

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
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

// testRenderer builds the production renderer (markdown + sanitizer config)
// pointed at the fake sidecar.
func testRenderer(ts *httptest.Server) *browserlessPdfRenderer {
	r := newDefaultPdfRenderer()
	r.baseURL = ts.URL
	r.token = ""
	r.client = ts.Client()
	return r
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

// TestRenderMarkdownToPdfFetchesNoRemoteImages: the HTML handed to Chromium
// carries no image URL it would fetch (markdown or raw HTML, any scheme or
// relative form); inline data: images still print.
func TestRenderMarkdownToPdfFetchesNoRemoteImages(t *testing.T) {
	ts, captured := newFakeBrowserless(t, "%PDF-1.4 fake")
	h := NewProdHostAPI(WithPdfRenderer(testRenderer(ts)))
	const inline = "data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mNkYPhfDwAChwGA60e6kgAAAABJRU5ErkJggg=="
	md := "# Report\n\n![logo](https://evil.example/track.png)\n\n" +
		`<img src="http://169.254.169.254/latest/meta-data/">` + "\n\n" +
		`<img src="//evil.example/p.png"><img src="/internal.png">` + "\n\n" +
		"![inline](" + inline + ")\n\n[a link](https://example.com/doc)\n"
	if _, err := h.RenderMarkdownToPdf(context.Background(), md, PdfRenderOptions{}); err != nil {
		t.Fatalf("RenderMarkdownToPdf: %v", err)
	}
	for _, leaked := range []string{"evil.example", "169.254.169.254", "internal.png"} {
		if strings.Contains(*captured, leaked) {
			t.Errorf("printed HTML still references %q: %s", leaked, *captured)
		}
	}
	if !strings.Contains(*captured, `src="`+inline+`"`) {
		t.Errorf("inline data: image dropped: %s", *captured)
	}
	if !strings.Contains(*captured, `href="https://example.com/doc"`) {
		t.Errorf("links must survive (Chromium does not fetch them): %s", *captured)
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
	const logo = "data:image/png;base64,iVBORw0KGgo="
	h := brandHeader(PdfRenderOptions{
		BrandName:    `Copperforge <script>x</script>`,
		BrandLogoURL: logo,
	})
	if strings.Contains(h, "<script>") {
		t.Errorf("name not escaped: %s", h)
	}
	if !strings.Contains(h, "Copperforge &lt;script&gt;") || !strings.Contains(h, `src="`+logo+`"`) {
		t.Errorf("brand header missing escaped name or logo: %s", h)
	}
	for _, bad := range []string{
		"https://example.com/logo.png", "http://evil.example/logo.png", "javascript:alert(1)",
		"data:image/svg+xml;base64,PHN2Zz4=", `data:image/png;base64,iVBO" onload="x`, "data:text/html,<b>x</b>",
	} {
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
