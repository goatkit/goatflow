package notifications

import (
	"html"
	"strings"

	"github.com/gotrs-io/gotrs-ce/internal/utils"
)

// RenderContext carries values used to interpolate legacy placeholders.
type RenderContext struct {
	CustomerFullName string
	AgentFirstName   string
	AgentLastName    string
}

// ApplyBranding stitches salutation and signature around the base body and expands placeholders.
func ApplyBranding(base string, baseIsHTML bool, identity *QueueIdentity, ctx *RenderContext) string {
	if identity == nil {
		return strings.TrimSpace(applyPlaceholders(base, ctx))
	}
	return composeBody(
		applyPlaceholders(base, ctx),
		baseIsHTML,
		snippetWithContext(identity.SalutationSnippet(), ctx),
		snippetWithContext(identity.SignatureSnippet(), ctx),
	)
}

func composeBody(base string, baseIsHTML bool, salutation, signature *Snippet) string {
	trimmed := strings.TrimSpace(base)
	finalIsHTML := baseIsHTML || snippetIsHTML(salutation) || snippetIsHTML(signature)
	if finalIsHTML {
		var parts []string
		if salutation != nil {
			parts = append(parts, snippetAsHTML(salutation))
		}
		if trimmed != "" {
			parts = append(parts, textAsHTML(trimmed, baseIsHTML))
		}
		if signature != nil {
			parts = append(parts, snippetAsHTML(signature))
		}
		return strings.Join(filterEmpty(parts), "\n")
	}

	var blocks []string
	if salutation != nil {
		blocks = append(blocks, snippetAsText(salutation))
	}
	if trimmed != "" {
		blocks = append(blocks, trimmed)
	}
	if signature != nil {
		blocks = append(blocks, snippetAsText(signature))
	}
	return strings.Join(filterEmpty(blocks), "\n\n")
}

func snippetIsHTML(snippet *Snippet) bool {
	return snippet != nil && strings.Contains(snippet.ContentType, "html")
}

func snippetAsText(snippet *Snippet) string {
	if snippet == nil {
		return ""
	}
	if snippetIsHTML(snippet) {
		return strings.TrimSpace(utils.StripHTML(snippet.Text))
	}
	return strings.TrimSpace(snippet.Text)
}

func snippetAsHTML(snippet *Snippet) string {
	if snippet == nil {
		return ""
	}
	if snippetIsHTML(snippet) {
		return strings.TrimSpace(snippet.Text)
	}
	return wrapPlainText(snippet.Text)
}

func snippetWithContext(snippet *Snippet, ctx *RenderContext) *Snippet {
	if snippet == nil || ctx == nil {
		return snippet
	}
	return &Snippet{Text: applyPlaceholders(snippet.Text, ctx), ContentType: snippet.ContentType}
}

func textAsHTML(content string, alreadyHTML bool) string {
	if alreadyHTML {
		return strings.TrimSpace(content)
	}
	return wrapPlainText(content)
}

func applyPlaceholders(value string, ctx *RenderContext) string {
	if ctx == nil {
		ctx = &RenderContext{}
	}
	replacer := strings.NewReplacer(
		"<OTRS_CUSTOMER_REALNAME>", strings.TrimSpace(ctx.CustomerFullName),
		"<OTRS_Agent_UserFirstname>", strings.TrimSpace(ctx.AgentFirstName),
		"<OTRS_Agent_UserLastname>", strings.TrimSpace(ctx.AgentLastName),
	)
	return strings.TrimSpace(replacer.Replace(value))
}

func wrapPlainText(value string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return ""
	}
	escaped := html.EscapeString(trimmed)
	escaped = strings.ReplaceAll(escaped, "\n", "<br>")
	return "<p>" + escaped + "</p>"
}

func filterEmpty(values []string) []string {
	result := make([]string, 0, len(values))
	for _, v := range values {
		if strings.TrimSpace(v) == "" {
			continue
		}
		result = append(result, v)
	}
	return result
}
