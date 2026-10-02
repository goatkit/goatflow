package wasm_test

import (
	"context"
	"encoding/json"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"

	"github.com/goatkit/goatflow/internal/platform/plugin"
	"github.com/goatkit/goatflow/internal/platform/plugin/wasm"
)

// parityHost answers the HostAPI methods WASM plugins used to lack and
// records which ones the guest reached.
type parityHost struct {
	*trackingHostAPI
	mu      sync.Mutex
	reached map[string]bool
}

func (h *parityHost) hit(method string) {
	h.mu.Lock()
	h.reached[method] = true
	h.mu.Unlock()
}

func (h *parityHost) CacheDelete(ctx context.Context, key string) error {
	h.hit("CacheDelete")
	return nil
}
func (h *parityHost) StoreFile(ctx context.Context, key string, data []byte, md map[string]string) error {
	h.hit("StoreFile")
	return nil
}
func (h *parityHost) GetFile(ctx context.Context, key string) ([]byte, map[string]string, error) {
	h.hit("GetFile")
	return []byte("file-bytes"), map[string]string{"ct": "text/plain"}, nil
}
func (h *parityHost) DeleteFile(ctx context.Context, key string) error {
	h.hit("DeleteFile")
	return nil
}
func (h *parityHost) ListFiles(ctx context.Context, prefix string) ([]plugin.FileInfo, error) {
	h.hit("ListFiles")
	return []plugin.FileInfo{{Key: "docs/a.txt", Size: 3}}, nil
}
func (h *parityHost) GenerateThumbnail(ctx context.Context, d []byte, ct string, w, ht int) ([]byte, string, error) {
	h.hit("GenerateThumbnail")
	return []byte("thumb"), "image/png", nil
}
func (h *parityHost) CreateArticleAttachment(ctx context.Context, aid, by int64, fn, ct string, c []byte) (int64, error) {
	h.hit("CreateArticleAttachment")
	return 77, nil
}
func (h *parityHost) ListArticleAttachments(ctx context.Context, aid int64) ([]plugin.ArticleAttachment, error) {
	h.hit("ListArticleAttachments")
	return []plugin.ArticleAttachment{{ID: 1, ArticleID: aid, Filename: "r.pdf"}}, nil
}
func (h *parityHost) DeleteArticleAttachment(ctx context.Context, aid, id int64) error {
	h.hit("DeleteArticleAttachment")
	return nil
}
func (h *parityHost) CreateArticle(ctx context.Context, tid, by int64, s, b string, v bool) (int64, error) {
	h.hit("CreateArticle")
	return 88, nil
}
func (h *parityHost) ChangeTicketStatus(ctx context.Context, tid, sid, uid, until int64) error {
	h.hit("ChangeTicketStatus")
	return nil
}
func (h *parityHost) ListTicketStates(ctx context.Context) ([]plugin.TicketStateInfo, error) {
	h.hit("ListTicketStates")
	return []plugin.TicketStateInfo{{ID: 4, Name: "open"}}, nil
}
func (h *parityHost) ListTicketViews(ctx context.Context) ([]plugin.TicketViewInfo, error) {
	h.hit("ListTicketViews")
	return []plugin.TicketViewInfo{{PluginName: "kanban", URL: "/k/{ticket_id}"}}, nil
}
func (h *parityHost) RenderMarkdownToPdf(ctx context.Context, md string, o plugin.PdfRenderOptions) ([]byte, error) {
	h.hit("RenderMarkdownToPdf")
	return []byte("%PDF-1.7"), nil
}

// TestWASMHostCallParity calls, from a real WASM guest, every host function
// that gRPC plugins had and WASM plugins lacked, and checks the host ran it
// and the guest got the host's answer.
func TestWASMHostCallParity(t *testing.T) {
	_, filename, _, _ := runtime.Caller(0)
	repoRoot := filepath.Join(filepath.Dir(filename), "..", "..", "..", "..")
	wasmPath := filepath.Join(repoRoot, "plugins", "test-hostapi-wasm", "test-hostapi.wasm")
	requireWASMPlugin(t, wasmPath)

	ctx := context.Background()
	p, err := wasm.LoadFromFile(ctx, wasmPath)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	defer p.Shutdown(ctx)
	host := &parityHost{trackingHostAPI: &trackingHostAPI{}, reached: map[string]bool{}}
	if err := p.Init(ctx, host); err != nil {
		t.Fatalf("init: %v", err)
	}

	cases := []struct {
		fn, args, method, want string
	}{
		{"cache_delete", `{"key":"k"}`, "CacheDelete", `"ok":true`},
		{"store_file", `{"key":"docs/a.txt","data":"YWJj","metadata":{"ct":"text/plain"}}`, "StoreFile", `"status":"ok"`},
		{"get_file", `{"key":"docs/a.txt"}`, "GetFile", `"data":"ZmlsZS1ieXRlcw=="`},
		{"delete_file", `{"key":"docs/a.txt"}`, "DeleteFile", `"status":"ok"`},
		{"list_files", `{"prefix":"docs/"}`, "ListFiles", `docs/a.txt`},
		{"generate_thumbnail", `{"data":"aW1n","content_type":"image/jpeg"}`, "GenerateThumbnail", `"thumb_content_type":"image/png"`},
		{"create_article_attachment", `{"article_id":5,"created_by":2,"filename":"r.pdf","content_type":"application/pdf","content":"JVBERg=="}`, "CreateArticleAttachment", `"id":77`},
		{"list_article_attachments", `{"article_id":5}`, "ListArticleAttachments", `r.pdf`},
		{"delete_article_attachment", `{"article_id":5,"attachment_id":1}`, "DeleteArticleAttachment", `"status":"ok"`},
		{"create_article", `{"ticket_id":9,"created_by":2,"subject":"s","body":"b"}`, "CreateArticle", `"id":88`},
		{"change_ticket_status", `{"ticket_id":9,"state_id":4,"user_id":2}`, "ChangeTicketStatus", `"status":"ok"`},
		{"list_ticket_states", `{}`, "ListTicketStates", `open`},
		{"list_ticket_views", `{}`, "ListTicketViews", `{ticket_id}`},
		{"render_markdown_to_pdf", `{"markdown":"# T","options":{}}`, "RenderMarkdownToPdf", `"pdf":"JVBERi0xLjc="`},
	}
	for _, tc := range cases {
		t.Run(tc.fn, func(t *testing.T) {
			args, _ := json.Marshal(map[string]any{"fn": tc.fn, "args": json.RawMessage(tc.args)})
			out, err := p.Call(ctx, "host", args)
			if err != nil {
				t.Fatalf("call: %v", err)
			}
			if !strings.Contains(string(out), tc.want) {
				t.Errorf("guest got %s, want it to contain %s", out, tc.want)
			}
			host.mu.Lock()
			reached := host.reached[tc.method]
			host.mu.Unlock()
			if !reached {
				t.Errorf("host %s was not called", tc.method)
			}
		})
	}
}
