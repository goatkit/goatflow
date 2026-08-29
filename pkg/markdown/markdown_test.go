package markdown

import (
	"strings"
	"testing"
)

func TestRenderEmpty(t *testing.T) {
	if got := Render(""); got != "" {
		t.Fatalf("Render(\"\") = %q, want empty", got)
	}
}

func TestRenderBasic(t *testing.T) {
	html := Render("# Title\n\nThis is **bold** and *italic* text.")
	if !strings.Contains(html, "<h1") || !strings.Contains(html, "Title") {
		t.Fatalf("expected h1 heading, got: %s", html)
	}
	if !strings.Contains(html, "<strong>") || !strings.Contains(html, "bold") {
		t.Fatalf("expected <strong>, got: %s", html)
	}
	if !strings.Contains(html, "<em>") || !strings.Contains(html, "italic") {
		t.Fatalf("expected <em>, got: %s", html)
	}
}

func TestRenderGFMTable(t *testing.T) {
	src := "| A | B |\n|---|---|\n| 1 | 2 |"
	html := Render(src)
	if !strings.Contains(html, "<table>") || !strings.Contains(html, "<th>") || !strings.Contains(html, "<td>") {
		t.Fatalf("expected GFM table, got: %s", html)
	}
}

func TestRenderStrikethrough(t *testing.T) {
	html := Render("~~gone~~")
	if !strings.Contains(html, "<del>") {
		t.Fatalf("expected <del> for strikethrough, got: %s", html)
	}
}

func TestRenderStripsUnsafeHTML(t *testing.T) {
	html := Render("Hello\n\n<script>alert(1)</script>\n\n<img src=x onerror=alert(1)>")
	if strings.Contains(html, "<script") {
		t.Fatalf("script tag not stripped: %s", html)
	}
	if strings.Contains(html, "onerror") {
		t.Fatalf("event handler attr not stripped: %s", html)
	}
	if !strings.Contains(html, "Hello") {
		t.Fatalf("expected text to remain, got: %s", html)
	}
}

func TestRenderAllowsClassAttr(t *testing.T) {
	html := Render("<div class=\"foo bar\">x</div>")
	if !strings.Contains(html, `class="foo bar"`) {
		t.Fatalf("class attr not preserved: %s", html)
	}
}
