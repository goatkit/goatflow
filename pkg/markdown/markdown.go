// Package markdown renders Markdown to sanitized HTML (goldmark GFM +
// bluemonday). It is the importable, canonical Markdown renderer for GoatFlow
// and its plugins — the same goldmark-GFM + bluemonday stack callers previously
// had to duplicate in an internal (unimportable) package or plugin-local shim.
//
// GFM extensions (tables, strikethrough, autolinks, task lists) are enabled and
// from-attribute parsing is allowed. Raw/unsafe HTML in the source is stripped
// by bluemonday, so the returned HTML is safe to insert server-side into any
// page or plugin UI. Class attributes are preserved on common block/inline
// elements so an outer layer (e.g. Tailwind class injection) can restyle the
// output.
package markdown

import (
	"bytes"
	"sync"

	"github.com/microcosm-cc/bluemonday"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/renderer/html"
)

var (
	gmOnce sync.Once
	gm     goldmark.Markdown

	policyOnce sync.Once
	policy     *bluemonday.Policy
)

// Render converts Markdown to sanitized HTML. It returns "" for empty input and
// a sanitized fallback (the raw input with unsafe HTML stripped) if a single
// conversion fails, so a malformed document degrades to its literal text rather
// than a partial render.
func Render(markdown string) string {
	if markdown == "" {
		return ""
	}
	gmOnce.Do(func() {
		gm = goldmark.New(
			goldmark.WithExtensions(extension.GFM),
			goldmark.WithParserOptions(parser.WithAttribute()),
			goldmark.WithRendererOptions(html.WithUnsafe()),
		)
	})
	var buf bytes.Buffer
	if err := gm.Convert([]byte(markdown), &buf); err != nil {
		return sanitizePolicy().Sanitize(markdown)
	}
	return sanitizePolicy().Sanitize(buf.String())
}

func sanitizePolicy() *bluemonday.Policy {
	policyOnce.Do(func() {
		p := bluemonday.UGCPolicy()
		p.AllowAttrs("class").Matching(bluemonday.SpaceSeparatedTokens).OnElements(
			"div", "span", "p", "ul", "ol", "li",
			"table", "thead", "tbody", "tr", "td", "th",
			"h1", "h2", "h3", "h4", "h5", "h6",
			"blockquote", "code", "pre", "img", "a",
		)
		policy = p
	})
	return policy
}
