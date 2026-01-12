package template

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestValidateTagBalance(t *testing.T) {
	tests := []struct {
		name    string
		html    string
		wantErr bool
		errMsg  string
	}{
		{
			name:    "valid simple HTML",
			html:    "<div><p>Hello</p></div>",
			wantErr: false,
		},
		{
			name:    "valid nested divs",
			html:    "<div><div><div>Content</div></div></div>",
			wantErr: false,
		},
		{
			name:    "valid with void elements",
			html:    "<div><img src='x'><br><input type='text'></div>",
			wantErr: false,
		},
		{
			name:    "valid complete HTML document",
			html:    "<!DOCTYPE html><html><head><title>Test</title></head><body><div>Content</div></body></html>",
			wantErr: false,
		},
		{
			name:    "valid with self-closing void elements",
			html:    "<div><br/><hr/><img src='x'/></div>",
			wantErr: false,
		},
		{
			name:    "valid form with inputs",
			html:    "<form><label>Name</label><input type='text'><button>Submit</button></form>",
			wantErr: false,
		},
		{
			name:    "valid table structure",
			html:    "<table><thead><tr><th>Header</th></tr></thead><tbody><tr><td>Cell</td></tr></tbody></table>",
			wantErr: false,
		},
		{
			name:    "missing closing div - the bug case",
			html:    "<div><p>Hello</p>",
			wantErr: true,
			errMsg:  "unclosed tags",
		},
		{
			name:    "missing closing tag nested",
			html:    "<div><div><p>Content</p></div>",
			wantErr: true,
			errMsg:  "unclosed tags",
		},
		{
			name:    "mismatched tags",
			html:    "<div><p>Hello</div></p>",
			wantErr: true,
			errMsg:  "mismatched tags",
		},
		{
			name:    "extra closing tag",
			html:    "<div></div></div>",
			wantErr: true,
			errMsg:  "unexpected closing tag",
		},
		{
			name:    "nested modals - simulates the roles.pongo2 bug",
			html:    `<div id="roleModal" class="hidden"><div id="roleUsersModal"></div>`,
			wantErr: true,
			errMsg:  "unclosed tags",
		},
		{
			name:    "wrong closing order",
			html:    "<div><span></div></span>",
			wantErr: true,
			errMsg:  "mismatched tags",
		},
		{
			name:    "unclosed form",
			html:    "<form><input type='text'>",
			wantErr: true,
			errMsg:  "unclosed tags",
		},
		{
			name:    "missing multiple closing tags",
			html:    "<html><body><div>",
			wantErr: true,
			errMsg:  "unclosed tags",
		},
		{
			name:    "empty content is valid",
			html:    "",
			wantErr: false,
		},
		{
			name:    "text only is valid",
			html:    "Just some text without tags",
			wantErr: false,
		},
		{
			name:    "valid with comments",
			html:    "<div><!-- comment --><p>Text</p></div>",
			wantErr: false,
		},
		{
			name:    "valid script tag",
			html:    "<div><script>var x = 1;</script></div>",
			wantErr: false,
		},
		{
			name:    "valid style tag",
			html:    "<div><style>.foo { color: red; }</style></div>",
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateTagBalance(tt.html)
			if tt.wantErr {
				assert.Error(t, err, "expected an error for: %s", tt.name)
				if tt.errMsg != "" {
					assert.Contains(t, err.Error(), tt.errMsg,
						"error message should contain %q", tt.errMsg)
				}
			} else {
				assert.NoError(t, err, "unexpected error for: %s", tt.name)
			}
		})
	}
}

func TestValidateHTML(t *testing.T) {
	// ValidateHTML is a wrapper, just verify it calls through correctly
	err := ValidateHTML("<div></div>")
	assert.NoError(t, err)

	err = ValidateHTML("<div>")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unclosed tags")
}

func TestValidateTagBalance_RealWorldTemplates(t *testing.T) {
	// Test patterns commonly found in our templates
	tests := []struct {
		name    string
		html    string
		wantErr bool
	}{
		{
			name: "modal structure",
			html: `
				<div id="modal" class="hidden fixed z-10 inset-0">
					<div class="flex items-center justify-center min-h-screen">
						<div class="fixed inset-0 bg-gray-500"></div>
						<div class="bg-white rounded-lg">
							<div class="px-4 pt-5">
								<h3>Title</h3>
							</div>
							<div class="px-4 py-3">
								<button>Close</button>
							</div>
						</div>
					</div>
				</div>
			`,
			wantErr: false,
		},
		{
			name: "modal with form",
			html: `
				<div id="modal">
					<form id="myForm">
						<div class="form-group">
							<label>Name</label>
							<input type="text" name="name">
						</div>
						<div class="form-group">
							<label>Email</label>
							<input type="email" name="email">
						</div>
						<button type="submit">Submit</button>
					</form>
				</div>
			`,
			wantErr: false,
		},
		{
			name: "table with forms",
			html: `
				<table>
					<thead>
						<tr>
							<th>Name</th>
							<th>Actions</th>
						</tr>
					</thead>
					<tbody>
						<tr>
							<td>Item 1</td>
							<td>
								<form>
									<input type="hidden" name="id" value="1">
									<button>Delete</button>
								</form>
							</td>
						</tr>
					</tbody>
				</table>
			`,
			wantErr: false,
		},
		{
			name: "broken modal - missing closing div",
			html: `
				<div id="modal1" class="hidden">
					<div class="content">
						<form>
							<div class="field">
								<label>Name</label>
								<input type="text">
							<!-- missing </div> here -->
						</form>
					</div>
				</div>
				<div id="modal2" class="hidden">
					<div class="content">Modal 2</div>
				</div>
			`,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateTagBalance(tt.html)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestVoidElements(t *testing.T) {
	// Verify all void elements are handled correctly
	voidTags := []string{
		"area", "base", "br", "col", "embed", "hr", "img",
		"input", "link", "meta", "param", "source", "track", "wbr",
	}

	for _, tag := range voidTags {
		t.Run(tag, func(t *testing.T) {
			// Void element without closing should be valid
			html := "<div><" + tag + "></div>"
			err := ValidateTagBalance(html)
			assert.NoError(t, err, "%s should not require a closing tag", tag)

			// Self-closing syntax should also be valid
			html = "<div><" + tag + "/></div>"
			err = ValidateTagBalance(html)
			assert.NoError(t, err, "self-closing %s should be valid", tag)
		})
	}
}

func TestValidateTagBalance_ErrorMessages(t *testing.T) {
	tests := []struct {
		name        string
		html        string
		errContains []string
	}{
		{
			name:        "unclosed div shows tag name",
			html:        "<div>",
			errContains: []string{"unclosed", "div"},
		},
		{
			name:        "mismatched shows both tags",
			html:        "<div></span>",
			errContains: []string{"mismatched", "div", "span"},
		},
		{
			name:        "unexpected closing shows tag",
			html:        "</div>",
			errContains: []string{"unexpected", "div"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateTagBalance(tt.html)
			assert.Error(t, err)
			errStr := strings.ToLower(err.Error())
			for _, substr := range tt.errContains {
				assert.Contains(t, errStr, strings.ToLower(substr),
					"error should mention %q", substr)
			}
		})
	}
}
