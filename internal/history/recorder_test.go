package history

import (
	"strings"
	"testing"
	"unicode/utf8"
)

func TestExcerptTruncatesByRune(t *testing.T) {
	tests := []struct {
		name   string
		in     string
		maxLen int
		want   string
	}{
		{name: "short ascii unchanged", in: "hello", maxLen: 10, want: "hello"},
		{name: "ascii truncated", in: "hello world", maxLen: 8, want: "hello..."},
		{name: "multibyte at limit unchanged", in: "héllo wörld", maxLen: 11, want: "héllo wörld"},
		{name: "multibyte truncated on rune boundary", in: "ééééééééééé", maxLen: 8, want: "ééééé..."},
		{name: "cjk truncated", in: "日本語のテキストです", maxLen: 6, want: "日本語..."},
		{name: "emoji truncated", in: "🙂🙂🙂🙂🙂🙂", maxLen: 5, want: "🙂🙂..."},
		{name: "default limit", in: strings.Repeat("ж", 60), maxLen: 0, want: strings.Repeat("ж", 47) + "..."},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Excerpt(tt.in, tt.maxLen)
			if got != tt.want {
				t.Fatalf("Excerpt(%q, %d) = %q, want %q", tt.in, tt.maxLen, got, tt.want)
			}
			if !utf8.ValidString(got) {
				t.Fatalf("Excerpt(%q, %d) produced invalid UTF-8: %q", tt.in, tt.maxLen, got)
			}
		})
	}
}

func TestExcerptTinyLimitDoesNotPanic(t *testing.T) {
	if got := Excerpt("абвгд", 2); got != "аб" {
		t.Fatalf("Excerpt with limit 2 = %q, want %q", got, "аб")
	}
}
