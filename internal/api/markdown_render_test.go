package api

import (
	"strings"
	"testing"
)

func TestRenderMarkdownBasic(t *testing.T) {
	md := "# Title\n\nThis is **bold** and *italic* text."
	html := RenderMarkdown(md)
	if !strings.Contains(html, "<h1") || !strings.Contains(html, "Title") {
		t.Fatalf("expected header in output, got: %s", html)
	}
	if !strings.Contains(html, "<strong") || !strings.Contains(html, "bold") {
		t.Fatalf("expected bold strong tag in output, got: %s", html)
	}
	if !strings.Contains(html, "<em") || !strings.Contains(html, "italic") {
		t.Fatalf("expected italic em tag in output, got: %s", html)
	}
}

func TestRenderMarkdownList(t *testing.T) {
	md := "- one\n- two\n- three"
	html := RenderMarkdown(md)
	if !strings.Contains(html, "<ul") || !strings.Contains(html, "<li") {
		t.Fatalf("expected list in output, got: %s", html)
	}
}
