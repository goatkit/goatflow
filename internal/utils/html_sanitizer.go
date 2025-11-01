package utils

import (
	"strings"

	"github.com/microcosm-cc/bluemonday"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/renderer/html"
)

// HTMLSanitizer provides HTML sanitization for user input
type HTMLSanitizer struct {
	policy *bluemonday.Policy
}

// NewHTMLSanitizer creates a new HTML sanitizer with GOTRS-specific policy
func NewHTMLSanitizer() *HTMLSanitizer {
	// Create a policy that allows common formatting but prevents XSS
	p := bluemonday.NewPolicy()
	
	// Basic formatting
	p.AllowElements("b", "strong", "i", "em", "u", "s", "strike", "del")
	
	// Headings
	p.AllowElements("h1", "h2", "h3", "h4", "h5", "h6")
	
	// Paragraphs and breaks
	p.AllowElements("p", "br", "hr")
	
	// Lists
	p.AllowElements("ul", "ol", "li")
	
	// Quotes and code
	p.AllowElements("blockquote", "code", "pre")
	
	// Tables
	p.AllowElements("table", "thead", "tbody", "tfoot", "tr", "th", "td")
	p.AllowAttrs("colspan", "rowspan").OnElements("td", "th")
	
	// Images (with safe attributes)
	p.AllowElements("img")
	p.AllowAttrs("src", "alt", "title", "width", "height").OnElements("img")
	p.AllowURLSchemes("http", "https", "data") // Allow data URLs for base64 images
	
	// Links (with safe attributes only)
	p.AllowElements("a")
	p.AllowAttrs("href").OnElements("a")
	p.AllowURLSchemes("http", "https", "mailto")
	p.RequireParseableURLs(true)
	p.RequireNoFollowOnLinks(true) // Add rel="nofollow" to links
	p.RequireNoReferrerOnLinks(true) // Add rel="noreferrer" to links
	p.AddTargetBlankToFullyQualifiedLinks(true) // Open external links in new tab
	
	// Allow class attributes for styling (limited to safe values)
	p.AllowAttrs("class").Matching(bluemonday.SpaceSeparatedTokens).OnElements(
		"div", "span", "p", "ul", "ol", "li", "table", "tr", "td", "th",
		"h1", "h2", "h3", "h4", "h5", "h6", "blockquote", "code", "pre", "img",
	)
	
	// Allow style attributes for color and background-color (TipTap rich text editor)
	p.AllowAttrs("style").OnElements("span", "mark")
	
	return &HTMLSanitizer{
		policy: p,
	}
}

// Sanitize cleans HTML content to prevent XSS attacks
func (s *HTMLSanitizer) Sanitize(html string) string {
	return s.policy.Sanitize(html)
}

// IsHTML checks if the content appears to be HTML
func IsHTML(content string) bool {
	// Check for common HTML tags
	htmlTags := []string{"<p>", "<br>", "<div>", "<span>", "<b>", "<i>", "<strong>", "<em>", "<h1>", "<h2>", "<h3>", "<ul>", "<ol>", "<li>", "<table>", "<a ", "<blockquote>", "<img "}
	
	contentLower := strings.ToLower(content)
	for _, tag := range htmlTags {
		if strings.Contains(contentLower, tag) {
			return true
		}
	}
	
	return false
}

// IsMarkdown checks if the content appears to be markdown (common patterns from Tiptap conversion)
func IsMarkdown(content string) bool {
	// Check for common markdown patterns that indicate rich text formatting
	markdownPatterns := []string{"**", "*", "_", "`", "# ", "## ", "### ", "- ", "* ", "+ ", "1. ", "[", "](", "![", "](", "\n"}
	
	// Count markdown elements
	markdownCount := 0
	for _, pattern := range markdownPatterns {
		if strings.Contains(content, pattern) {
			markdownCount++
		}
	}
	
	// If we have multiple markdown patterns, it's likely rich text
	return markdownCount >= 2
}

// MarkdownToHTML converts markdown content to HTML
func MarkdownToHTML(markdown string) string {
	md := goldmark.New(
		goldmark.WithRendererOptions(
			html.WithUnsafe(), // Allow raw HTML in markdown
		),
	)
	
	var buf strings.Builder
	if err := md.Convert([]byte(markdown), &buf); err != nil {
		// If conversion fails, return original content
		return markdown
	}
	
	return buf.String()
}

// StripHTML removes all HTML tags and returns plain text
func StripHTML(html string) string {
	// Use strict policy that strips all HTML
	p := bluemonday.StrictPolicy()
	return p.Sanitize(html)
}