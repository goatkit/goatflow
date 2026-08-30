package plugin

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/microcosm-cc/bluemonday"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/renderer/html"
)

// browserlessPdfRenderer renders markdown -> HTML -> PDF via a Browserless
// (headless Chromium) sidecar. The base URL and token come from environment
// variables so the renderer works in any deployment without extra config
// plumbing; NewProdHostAPI wires this as the default renderer.
type browserlessPdfRenderer struct {
	baseURL string
	token   string
	client  *http.Client
	md      goldmark.Markdown
	san     *bluemonday.Policy
}

// newDefaultPdfRenderer builds the renderer from BROWSERLESS_URL /
// BROWSERLESS_TOKEN. Defaults to the localhost sidecar from the dev compose.
func newDefaultPdfRenderer() *browserlessPdfRenderer {
	baseURL := strings.TrimRight(os.Getenv("BROWSERLESS_URL"), "/")
	if baseURL == "" {
		baseURL = "http://127.0.0.1:3000"
	}
	return &browserlessPdfRenderer{
		baseURL: baseURL,
		token:   os.Getenv("BROWSERLESS_TOKEN"),
		client:  &http.Client{Timeout: 90 * time.Second},
		md: goldmark.New(
			// GFM so action-item tables, strikethrough and autolinks render
			// like the on-screen article view (pkg/markdown uses the same
			// stack); html.WithUnsafe is safe because bluemonday sanitizes
			// the converted HTML before it is ever printed.
			goldmark.WithExtensions(extension.GFM),
			goldmark.WithParserOptions(parser.WithAttribute()),
			goldmark.WithRendererOptions(html.WithUnsafe()),
		),
		san: bluemonday.UGCPolicy(),
	}
}

// RenderMarkdownToPdf converts markdown to a styled HTML document and prints it
// to a PDF via the Browserless headless-Chromium /pdf endpoint. Output is
// sanitized with bluemonday before printing, so arbitrary plugin-generated
// markdown cannot inject scripts into the rendered page.
func (r *browserlessPdfRenderer) RenderMarkdownToPdf(ctx context.Context, markdown string, options PdfRenderOptions) ([]byte, error) {
	var html bytes.Buffer
	if err := r.md.Convert([]byte(markdown), &html); err != nil {
		return nil, fmt.Errorf("render markdown to html: %w", err)
	}
	body := r.san.SanitizeBytes(html.Bytes())
	doc := wrapPDFDocument(body, options)

	payload, err := json.Marshal(map[string]any{
		"html":    string(doc),
		"options": pdfPageOptions(options),
	})
	if err != nil {
		return nil, err
	}

	url := r.baseURL + "/pdf"
	if r.token != "" {
		url += "?token=" + r.token
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := r.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("browserless pdf request: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return nil, fmt.Errorf("browserless pdf: %s: %s", resp.Status, string(b))
	}
	pdf, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if len(pdf) == 0 {
		return nil, fmt.Errorf("browserless pdf returned empty body")
	}
	return pdf, nil
}

// wrapPDFDocument wraps sanitized markdown-rendered HTML in a minimal
// print-friendly document skeleton.
func wrapPDFDocument(body []byte, options PdfRenderOptions) []byte {
	head := `<style>` +
		`body{font-family:-apple-system,"Segoe UI",Roboto,Helvetica,Arial,sans-serif;font-size:12px;line-height:1.5;color:#1a1a1a;}` +
		`h1{font-size:22px;}h2{font-size:18px;}h3{font-size:15px;}` +
		`h1,h2,h3{margin:0.8em 0 0.4em;}` +
		`p,ul,ol,blockquote{margin:0.4em 0;}` +
		`table{border-collapse:collapse;width:100%;margin:0.6em 0;}` +
		`th,td{border:1px solid #ccc;padding:5px 8px;text-align:left;font-size:11px;}` +
		`th{background:#f2f2f2;}` +
		`code,pre{font-family:ui-monospace,SFMono-Regular,Menlo,Consolas,monospace;font-size:11px;background:#f5f5f5;border-radius:4px;}` +
		`code{padding:1px 4px;}pre{padding:8px 10px;white-space:pre-wrap;overflow-wrap:break-word;}` +
		`blockquote{border-left:4px solid #ddd;margin:0.4em 0;padding-left:12px;color:#555;}` +
		`img{max-width:100%;}` +
		`hr{border:none;border-top:1px solid #ddd;margin:1em 0;}` +
		brandingCSS(options) +
		`</style>`
	titleTag := ""
	if options.Title != "" {
		titleTag = "<title>" + escapeHTML(options.Title) + "</title>"
	}
	return []byte("<!doctype html><html><head><meta charset=\"utf-8\">" + titleTag + head +
		"</head><body>" + string(body) + "</body></html>")
}

// pdfPageOptions maps PdfRenderOptions to puppeteer page.pdf options that
// Browserless forwards to headless Chromium.
func pdfPageOptions(options PdfRenderOptions) map[string]any {
	pageSize := options.PageSize
	if pageSize == "" {
		pageSize = "A4"
	}
	margin := options.MarginMM
	if margin <= 0 {
		margin = 15
	}
	mm := fmt.Sprintf("%dmm", int(margin))
	pageOpts := map[string]any{
		"format":          pageSize,
		"printBackground": true,
		"margin": map[string]any{
			"top": mm, "right": mm, "bottom": mm, "left": mm,
		},
	}
	brand := brandHeader(options)
	if options.Title != "" || brand != "" {
		sep := ""
		if brand != "" && options.Title != "" {
			sep = ` — `
		}
		pageOpts["displayHeaderFooter"] = true
		pageOpts["headerTemplate"] = `<div style="font-size:10px;width:100%;padding:0 15mm;color:#666;">` +
			brand + sep + escapeHTML(options.Title) + `</div>`
		pageOpts["footerTemplate"] = `<div style="font-size:10px;width:100%;text-align:center;color:#666;">` +
			`<span class="pageNumber"></span> / <span class="totalPages"></span></div>`
	}
	return pageOpts
}

// brandColorRe accepts only 6-digit hex colours; anything else is dropped so
// a plugin can never smuggle arbitrary CSS into the printed page.
var brandColorRe = regexp.MustCompile(`^#[0-9a-fA-F]{6}$`)

// brandingCSS returns the accent styling for a valid BrandColor ("" otherwise):
// headings and links take the brand colour; table headers get a 12%-alpha wash.
func brandingCSS(options PdfRenderOptions) string {
	if !brandColorRe.MatchString(options.BrandColor) {
		return ""
	}
	r, _ := strconv.ParseInt(options.BrandColor[1:3], 16, 32)
	g, _ := strconv.ParseInt(options.BrandColor[3:5], 16, 32)
	b, _ := strconv.ParseInt(options.BrandColor[5:7], 16, 32)
	return fmt.Sprintf(`h1,h2,h3,a{color:%s;}th{background:rgba(%d,%d,%d,0.12);}`,
		options.BrandColor, r, g, b)
}

// brandHeader builds the running header's leading identity fragment
// (https logo + bold name). Non-https or attribute-unsafe logo URLs are
// dropped; the name is HTML-escaped.
func brandHeader(options PdfRenderOptions) string {
	out := ""
	if strings.HasPrefix(options.BrandLogoURL, "https://") && !strings.ContainsAny(options.BrandLogoURL, "\"'<> \t\n") {
		out += `<img src="` + escapeHTML(options.BrandLogoURL) + `" style="height:10px;vertical-align:middle;margin-right:4px;">`
	}
	if options.BrandName != "" {
		out += `<span style="font-weight:600;vertical-align:middle;">` + escapeHTML(options.BrandName) + `</span>`
	}
	return out
}

// escapeHTML escapes the four HTML-sensitive characters.
func escapeHTML(s string) string {
	r := strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;", `"`, "&quot;", "'", "&#39;")
	return r.Replace(s)
}

// RenderMarkdownToPdf renders a markdown document to PDF bytes using the
// platform's configured renderer (headless-Chromium sidecar by default).
func (h *ProdHostAPI) RenderMarkdownToPdf(ctx context.Context, markdown string, options PdfRenderOptions) ([]byte, error) {
	if h.pdfRenderer == nil {
		return nil, fmt.Errorf("pdf renderer not configured")
	}
	return h.pdfRenderer.RenderMarkdownToPdf(ctx, markdown, options)
}
