package utils

import (
	"regexp"
	"strings"
)

// This ensures OTRS compatibility when Unicode support is disabled.
func FilterUnicode(input string) string {
	// Remove emojis and other 4-byte Unicode characters (U+10000 and above)
	// This includes most emojis, some mathematical symbols, and extended Unicode
	emojiRegex := regexp.MustCompile(`[\x{10000}-\x{10FFFF}]`)
	result := emojiRegex.ReplaceAllString(input, "")

	// Also remove other potentially problematic Unicode characters
	// Keep common accented characters (Latin-1 Supplement and Latin Extended-A)
	// but remove rarer Unicode characters that might cause issues

	// Remove characters from Supplementary Multilingual Plane (U+10000-U+1FFFF)
	// except for some common mathematical symbols that are widely supported
	smpRegex := regexp.MustCompile(`[\x{1D000}-\x{1D7FF}]`) // Mathematical symbols block
	result = smpRegex.ReplaceAllString(result, "")

	// Remove characters from other planes that are rarely used in text
	// but keep Latin, Cyrillic, Arabic, etc. which are in the BMP

	return strings.TrimSpace(result)
}
