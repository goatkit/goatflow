package template

import (
	"fmt"
	"io"
	"strings"

	"golang.org/x/net/html"
)

// voidElements are HTML elements that don't require closing tags
var voidElements = map[string]bool{
	"area":     true,
	"base":     true,
	"br":       true,
	"col":      true,
	"embed":    true,
	"hr":       true,
	"img":      true,
	"input":    true,
	"link":     true,
	"meta":     true,
	"param":    true,
	"source":   true,
	"track":    true,
	"wbr":      true,
	"!doctype": true,
}

// ValidateTagBalance validates that all HTML tags are properly balanced.
// It uses a stack-based approach to ensure every opening tag has a matching
// closing tag and that they are properly nested.
//
// Returns nil if the HTML is valid, otherwise returns an error describing
// the first structural issue found.
func ValidateTagBalance(htmlContent string) error {
	tagStack := []string{}
	tokenizer := html.NewTokenizer(strings.NewReader(htmlContent))

	for {
		tt := tokenizer.Next()
		switch tt {
		case html.ErrorToken:
			err := tokenizer.Err()
			if err == io.EOF {
				// End of document - check if stack is empty
				if len(tagStack) > 0 {
					return fmt.Errorf("unclosed tags at end of document: %v", tagStack)
				}
				return nil
			}
			return fmt.Errorf("HTML tokenizer error: %v", err)

		case html.StartTagToken:
			tn, _ := tokenizer.TagName()
			tagName := strings.ToLower(string(tn))
			if !voidElements[tagName] {
				tagStack = append(tagStack, tagName)
			}

		case html.EndTagToken:
			tn, _ := tokenizer.TagName()
			tagName := strings.ToLower(string(tn))

			// Skip void elements - they don't need closing tags
			if voidElements[tagName] {
				continue
			}

			if len(tagStack) == 0 {
				return fmt.Errorf("unexpected closing tag </%s> with no matching open tag", tagName)
			}

			// Pop from stack and verify it matches
			last := tagStack[len(tagStack)-1]
			if last != tagName {
				return fmt.Errorf("mismatched tags: expected </%s> but got </%s>", last, tagName)
			}
			tagStack = tagStack[:len(tagStack)-1]

		case html.SelfClosingTagToken:
			// Self-closing tags (like <br/>) are fine, no action needed
			continue
		}
	}
}

// ValidateHTML performs comprehensive HTML validation including tag balance.
// This is the main entry point for template HTML validation.
func ValidateHTML(htmlContent string) error {
	return ValidateTagBalance(htmlContent)
}
