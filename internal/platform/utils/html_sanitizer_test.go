package utils

import (
	"strings"
	"testing"
)

func TestNewHTMLSanitizer(t *testing.T) {
	s := NewHTMLSanitizer()
	if s == nil {
		t.Fatal("NewHTMLSanitizer returned nil")
	}
	if s.policy == nil {
		t.Fatal("policy should not be nil")
	}
}

func TestHTMLSanitizer_Sanitize(t *testing.T) {
	s := NewHTMLSanitizer()

	tests := []struct {
		name     string
		input    string
		contains []string
		excludes []string
	}{
		{
			name:     "allows basic formatting",
			input:    "<b>bold</b> <i>italic</i> <strong>strong</strong> <em>emphasis</em>",
			contains: []string{"<b>bold</b>", "<i>italic</i>", "<strong>strong</strong>", "<em>emphasis</em>"},
		},
		{
			name:     "allows headings",
			input:    "<h1>Title</h1><h2>Subtitle</h2><h3>Section</h3>",
			contains: []string{"<h1>Title</h1>", "<h2>Subtitle</h2>", "<h3>Section</h3>"},
		},
		{
			name:     "allows paragraphs and breaks",
			input:    "<p>Paragraph</p><br><hr>",
			contains: []string{"<p>Paragraph</p>", "<br>"},
		},
		{
			name:     "allows lists",
			input:    "<ul><li>Item 1</li></ul><ol><li>Item 2</li></ol>",
			contains: []string{"<ul>", "<li>Item 1</li>", "</ul>", "<ol>", "<li>Item 2</li>", "</ol>"},
		},
		{
			name:     "allows tables",
			input:    "<table><tr><th>Header</th></tr><tr><td>Cell</td></tr></table>",
			contains: []string{"<table>", "<tr>", "<th>Header</th>", "<td>Cell</td>", "</table>"},
		},
		{
			name:     "allows blockquote and code",
			input:    "<blockquote>Quote</blockquote><code>code</code><pre>preformatted</pre>",
			contains: []string{"<blockquote>Quote</blockquote>", "<code>code</code>", "<pre>preformatted</pre>"},
		},
		{
			name:     "allows safe links",
			input:    `<a href="https://example.com">Link</a>`,
			contains: []string{`href="https://example.com"`, ">Link</a>"},
		},
		{
			name:     "allows mailto links",
			input:    `<a href="mailto:test@example.com">Email</a>`,
			contains: []string{`href="mailto:test@example.com"`, ">Email</a>"},
		},
		{
			name:     "allows images with safe attributes",
			input:    `<img src="https://example.com/img.png" alt="Image" title="Title">`,
			contains: []string{`src="https://example.com/img.png"`, `alt="Image"`},
		},
		{
			name:     "strips script tags",
			input:    `<script>alert('xss')</script>`,
			excludes: []string{"<script>", "alert", "</script>"},
		},
		{
			name:     "strips onclick handlers",
			input:    `<div onclick="alert('xss')">Click me</div>`,
			excludes: []string{"onclick", "alert"},
		},
		{
			name:     "strips javascript URLs",
			input:    `<a href="javascript:alert('xss')">Link</a>`,
			excludes: []string{"javascript:", "alert"},
		},
		{
			name:     "strips iframe tags",
			input:    `<iframe src="https://evil.com"></iframe>`,
			excludes: []string{"<iframe", "</iframe>"},
		},
		{
			name:     "strips form elements",
			input:    `<form action="https://evil.com"><input type="text"></form>`,
			excludes: []string{"<form", "<input", "</form>"},
		},
		{
			name:     "allows class attributes on allowed elements",
			input:    `<div class="container"><span class="highlight">Text</span></div>`,
			contains: []string{`class="container"`, `class="highlight"`},
		},
		{
			name:     "allows colspan and rowspan on table cells",
			input:    `<td colspan="2" rowspan="3">Cell</td>`,
			contains: []string{`colspan="2"`, `rowspan="3"`},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := s.Sanitize(tt.input)

			for _, want := range tt.contains {
				if !strings.Contains(result, want) {
					t.Errorf("Sanitize(%q) = %q, should contain %q", tt.input, result, want)
				}
			}

			for _, exclude := range tt.excludes {
				if strings.Contains(result, exclude) {
					t.Errorf("Sanitize(%q) = %q, should not contain %q", tt.input, result, exclude)
				}
			}
		})
	}
}

func TestIsHTML(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantVal bool
	}{
		{"empty string", "", false},
		{"plain text", "Hello World", false},
		{"text with angle brackets", "5 < 10 and 10 > 5", false},
		{"paragraph tag", "<p>Hello</p>", true},
		{"div tag", "<div>Content</div>", true},
		{"span tag", "<span>Text</span>", true},
		{"bold tag", "<b>Bold</b>", true},
		{"italic tag", "<i>Italic</i>", true},
		{"strong tag", "<strong>Strong</strong>", true},
		{"em tag", "<em>Emphasis</em>", true},
		{"br tag", "Line 1<br>Line 2", true},
		{"heading tag", "<h1>Title</h1>", true},
		{"list tags", "<ul><li>Item</li></ul>", true},
		{"table tag", "<table><tr><td>Cell</td></tr></table>", true},
		{"link tag", `<a href="url">Link</a>`, true},
		{"blockquote tag", "<blockquote>Quote</blockquote>", true},
		{"image tag", `<img src="image.png">`, true},
		{"case insensitive", "<P>Paragraph</P>", true},
		{"mixed case", "<Div>Content</Div>", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsHTML(tt.input)
			if got != tt.wantVal {
				t.Errorf("IsHTML(%q) = %v, want %v", tt.input, got, tt.wantVal)
			}
		})
	}
}

func TestIsMarkdown(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantVal bool
	}{
		{"empty string", "", false},
		{"plain text", "Hello World", false},
		{"single asterisk only", "This has one asterisk", false},
		{"bold markdown with newline", "This is **bold**\ntext", true},
		{"single asterisk with newline", "This is *italic*\ntext", true},
		{"heading and list", "# Title\n- Item", true},
		{"multiple patterns", "**bold** and `code`", true},
		{"code blocks multiple", "Use `code` here\nmore `code`", true},
		{"link syntax with more", "Check [this](url)\nout", true},
		{"numbered list multiline", "1. First\n2. Second", true},
		{"bullet list dash multiline", "- Item 1\n- Item 2", true},
		{"bullet list asterisk multiline", "* Item 1\n* Item 2", true},
		{"image syntax with more", "![alt](image.png)\ntext", true},
		{"heading levels", "## Heading\n### Subheading", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsMarkdown(tt.input)
			if got != tt.wantVal {
				t.Errorf("IsMarkdown(%q) = %v, want %v", tt.input, got, tt.wantVal)
			}
		})
	}
}

func TestMarkdownToHTML(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		contains []string
	}{
		{
			name:     "converts bold",
			input:    "**bold text**",
			contains: []string{"<strong>bold text</strong>"},
		},
		{
			name:     "converts italic",
			input:    "*italic text*",
			contains: []string{"<em>italic text</em>"},
		},
		{
			name:     "converts heading",
			input:    "# Heading 1",
			contains: []string{"<h1>Heading 1</h1>"},
		},
		{
			name:     "converts code",
			input:    "`inline code`",
			contains: []string{"<code>inline code</code>"},
		},
		{
			name:     "converts link",
			input:    "[Link](https://example.com)",
			contains: []string{`href="https://example.com"`, ">Link</a>"},
		},
		{
			name:     "converts unordered list",
			input:    "- Item 1\n- Item 2",
			contains: []string{"<ul>", "<li>Item 1</li>", "<li>Item 2</li>", "</ul>"},
		},
		{
			name:     "converts ordered list",
			input:    "1. First\n2. Second",
			contains: []string{"<ol>", "<li>First</li>", "<li>Second</li>", "</ol>"},
		},
		{
			name:     "converts paragraph",
			input:    "This is a paragraph.",
			contains: []string{"<p>This is a paragraph.</p>"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := MarkdownToHTML(tt.input)

			for _, want := range tt.contains {
				if !strings.Contains(result, want) {
					t.Errorf("MarkdownToHTML(%q) = %q, should contain %q", tt.input, result, want)
				}
			}
		})
	}
}

func TestStripHTML(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantVal string
	}{
		{"empty string", "", ""},
		{"plain text", "Hello World", "Hello World"},
		{"simple tag", "<p>Paragraph</p>", "Paragraph"},
		{"nested tags", "<div><p>Text</p></div>", "Text"},
		{"multiple tags", "<b>Bold</b> and <i>italic</i>", "Bold and italic"},
		{"link tag", `<a href="url">Link</a>`, "Link"},
		{"script tag", `<script>alert('xss')</script>`, ""},
		{"attributes stripped", `<div class="test" id="main">Content</div>`, "Content"},
		{"preserves text between tags", "<p>First</p><p>Second</p>", "FirstSecond"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := StripHTML(tt.input)
			if got != tt.wantVal {
				t.Errorf("StripHTML(%q) = %q, want %q", tt.input, got, tt.wantVal)
			}
		})
	}
}

func BenchmarkSanitize(b *testing.B) {
	s := NewHTMLSanitizer()
	input := `<div class="container"><h1>Title</h1><p>Paragraph with <b>bold</b> and <a href="https://example.com">link</a></p><script>alert('xss')</script></div>`

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		s.Sanitize(input)
	}
}

func BenchmarkStripHTML(b *testing.B) {
	input := `<div class="container"><h1>Title</h1><p>Paragraph with <b>bold</b> and <a href="https://example.com">link</a></p></div>`

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		StripHTML(input)
	}
}
