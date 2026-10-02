package grpc

import (
	"context"
	"encoding/json"
	"net"
	"net/rpc"
	"reflect"
	"strings"
	"sync"
	"testing"

	"github.com/goatkit/goatflow/internal/platform/plugin"
	"github.com/goatkit/goatflow/pkg/plugin/grpcutil"
)

// recordingHost answers every HostAPI method with fixed values and records
// the method name, arguments and the plugin/acting user in its context.
type recordingHost struct {
	mu     sync.Mutex
	calls  []string
	args   map[string][]any
	caller map[string]string
	actor  map[string]int64
}

func newRecordingHost() *recordingHost {
	return &recordingHost{args: map[string][]any{}, caller: map[string]string{}, actor: map[string]int64{}}
}

func (h *recordingHost) rec(ctx context.Context, method string, args ...any) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.calls = append(h.calls, method)
	h.args[method] = args
	h.caller[method], _ = ctx.Value(plugin.PluginCallerKey).(string)
	h.actor[method], _ = plugin.ActingUserID(ctx)
}

func (h *recordingHost) DBQuery(ctx context.Context, q string, a ...any) ([]map[string]any, error) {
	h.rec(ctx, "DBQuery", q, a)
	return []map[string]any{{"id": float64(7)}}, nil
}
func (h *recordingHost) DBExec(ctx context.Context, q string, a ...any) (int64, error) {
	h.rec(ctx, "DBExec", q, a)
	return 3, nil
}
func (h *recordingHost) CacheGet(ctx context.Context, k string) ([]byte, bool, error) {
	h.rec(ctx, "CacheGet", k)
	return []byte("cached"), true, nil
}
func (h *recordingHost) CacheSet(ctx context.Context, k string, v []byte, ttl int) error {
	h.rec(ctx, "CacheSet", k, string(v), ttl)
	return nil
}
func (h *recordingHost) CacheDelete(ctx context.Context, k string) error {
	h.rec(ctx, "CacheDelete", k)
	return nil
}
func (h *recordingHost) HTTPRequest(ctx context.Context, m, u string, hd map[string]string, b []byte) (int, []byte, error) {
	h.rec(ctx, "HTTPRequest", m, u, hd["X-A"], string(b))
	return 201, []byte("resp"), nil
}
func (h *recordingHost) SendEmail(ctx context.Context, to, subj, body string, html bool) error {
	h.rec(ctx, "SendEmail", to, subj, body, html)
	return nil
}
func (h *recordingHost) Log(ctx context.Context, level, msg string, f map[string]any) {
	h.rec(ctx, "Log", level, msg, f["k"])
}
func (h *recordingHost) ConfigGet(ctx context.Context, k string) (string, error) {
	h.rec(ctx, "ConfigGet", k)
	return "Europe/London", nil
}
func (h *recordingHost) Translate(ctx context.Context, k string, a ...any) string {
	h.rec(ctx, "Translate", k)
	return "Hallo"
}
func (h *recordingHost) CallPlugin(ctx context.Context, p, fn string, a json.RawMessage) (json.RawMessage, error) {
	h.rec(ctx, "CallPlugin", p, fn, string(a))
	return json.RawMessage(`{"r":1}`), nil
}
func (h *recordingHost) PublishEvent(ctx context.Context, ch, ev, d string) error {
	h.rec(ctx, "PublishEvent", ch, ev, d)
	return nil
}
func (h *recordingHost) EntitySoftDelete(ctx context.Context, t string, id int64, r string) error {
	h.rec(ctx, "EntitySoftDelete", t, id, r)
	return nil
}
func (h *recordingHost) EntityRestore(ctx context.Context, t string, id int64) error {
	h.rec(ctx, "EntityRestore", t, id)
	return nil
}
func (h *recordingHost) EntityHardDelete(ctx context.Context, t string, id int64, r string) error {
	h.rec(ctx, "EntityHardDelete", t, id, r)
	return nil
}
func (h *recordingHost) RecycleBinList(ctx context.Context, t string) (json.RawMessage, error) {
	h.rec(ctx, "RecycleBinList", t)
	return json.RawMessage(`[{"id":5}]`), nil
}
func (h *recordingHost) SecureConfigGet(ctx context.Context, k string) (string, error) {
	h.rec(ctx, "SecureConfigGet", k)
	return "s3cret", nil
}
func (h *recordingHost) SecureConfigSet(ctx context.Context, k, v string) error {
	h.rec(ctx, "SecureConfigSet", k, v)
	return nil
}
func (h *recordingHost) OrgID(ctx context.Context) int64 {
	h.rec(ctx, "OrgID")
	return 9
}
func (h *recordingHost) CustomFieldsGet(ctx context.Context, t string, id int64, f []string) (map[string]any, error) {
	h.rec(ctx, "CustomFieldsGet", t, id, f)
	return map[string]any{"score": float64(4)}, nil
}
func (h *recordingHost) CustomFieldsSet(ctx context.Context, t string, id int64, v map[string]any) error {
	h.rec(ctx, "CustomFieldsSet", t, id, v["score"])
	return nil
}
func (h *recordingHost) CustomFieldsQuery(ctx context.Context, t string, f []plugin.CustomFieldFilter) ([]int64, error) {
	h.rec(ctx, "CustomFieldsQuery", t, f[0].Field)
	return []int64{1, 2}, nil
}
func (h *recordingHost) StoreFile(ctx context.Context, k string, d []byte, m map[string]string) error {
	h.rec(ctx, "StoreFile", k, string(d), m["ct"])
	return nil
}
func (h *recordingHost) GetFile(ctx context.Context, k string) ([]byte, map[string]string, error) {
	h.rec(ctx, "GetFile", k)
	return []byte("file"), map[string]string{"ct": "text/plain"}, nil
}
func (h *recordingHost) DeleteFile(ctx context.Context, k string) error {
	h.rec(ctx, "DeleteFile", k)
	return nil
}
func (h *recordingHost) ListFiles(ctx context.Context, p string) ([]plugin.FileInfo, error) {
	h.rec(ctx, "ListFiles", p)
	return []plugin.FileInfo{{Key: "a/b", Size: 4}}, nil
}
func (h *recordingHost) GenerateThumbnail(ctx context.Context, d []byte, ct string, w, ht int) ([]byte, string, error) {
	h.rec(ctx, "GenerateThumbnail", string(d), ct, w, ht)
	return []byte("thumb"), "image/png", nil
}
func (h *recordingHost) CreateArticleAttachment(ctx context.Context, aid, by int64, fn, ct string, c []byte) (int64, error) {
	h.rec(ctx, "CreateArticleAttachment", aid, by, fn, ct, string(c))
	return 11, nil
}
func (h *recordingHost) ListArticleAttachments(ctx context.Context, aid int64) ([]plugin.ArticleAttachment, error) {
	h.rec(ctx, "ListArticleAttachments", aid)
	return []plugin.ArticleAttachment{{ID: 1, ArticleID: aid, Filename: "x.pdf"}}, nil
}
func (h *recordingHost) DeleteArticleAttachment(ctx context.Context, aid, id int64) error {
	h.rec(ctx, "DeleteArticleAttachment", aid, id)
	return nil
}
func (h *recordingHost) CreateArticle(ctx context.Context, tid, by int64, s, b string, v bool) (int64, error) {
	h.rec(ctx, "CreateArticle", tid, by, s, b, v)
	return 12, nil
}
func (h *recordingHost) ChangeTicketStatus(ctx context.Context, tid, sid, uid, until int64) error {
	h.rec(ctx, "ChangeTicketStatus", tid, sid, uid, until)
	return nil
}
func (h *recordingHost) ListTicketStates(ctx context.Context) ([]plugin.TicketStateInfo, error) {
	h.rec(ctx, "ListTicketStates")
	return []plugin.TicketStateInfo{{ID: 4, Name: "open"}}, nil
}
func (h *recordingHost) ListTicketViews(ctx context.Context) ([]plugin.TicketViewInfo, error) {
	h.rec(ctx, "ListTicketViews")
	return []plugin.TicketViewInfo{{PluginName: "p", URL: "/x/{ticket_id}"}}, nil
}
func (h *recordingHost) RenderMarkdownToPdf(ctx context.Context, md string, o plugin.PdfRenderOptions) ([]byte, error) {
	h.rec(ctx, "RenderMarkdownToPdf", md, o.Title)
	return []byte("%PDF"), nil
}

var _ plugin.HostAPI = (*recordingHost)(nil)

// pipeClient connects a plugin-side grpcutil.HostAPIClient to a host-side
// HostAPIRPCServer over an in-memory net/rpc connection, the same wire path
// a plugin process uses.
func pipeClient(t *testing.T, binding *hostBinding, callerName string) *grpcutil.HostAPIClient {
	t.Helper()
	server := rpc.NewServer()
	if err := server.RegisterName("Plugin", &HostAPIRPCServer{binding: binding, CallerName: callerName}); err != nil {
		t.Fatal(err)
	}
	hostSide, pluginSide := net.Pipe()
	go server.ServeConn(hostSide)
	client := rpc.NewClient(pluginSide)
	t.Cleanup(func() { client.Close() })
	return grpcutil.NewHostAPIClient(client, "claimed-name")
}

// TestHostAPIClientContract drives every grpcutil.HostAPIClient method through
// the real RPC server and dispatcher and checks the host received the call
// and the plugin decoded the host's answer.
func TestHostAPIClientContract(t *testing.T) {
	host := newRecordingHost()
	binding := newHostBinding()
	binding.setHost(host)
	c := pipeClient(t, binding, "contract-plugin")
	ctx := context.Background()

	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	eq := func(name string, got, want any) {
		t.Helper()
		if !reflect.DeepEqual(got, want) {
			t.Errorf("%s: got %#v, want %#v", name, got, want)
		}
	}

	rows, err := c.DBQuery(ctx, "SELECT id FROM gk_x WHERE a = ?", "v")
	must(err)
	eq("DBQuery", rows, []map[string]any{{"id": float64(7)}})
	n, err := c.DBExec(ctx, "DELETE FROM gk_x")
	must(err)
	eq("DBExec", n, int64(3))

	val, found, err := c.CacheGet(ctx, "k1")
	must(err)
	eq("CacheGet", []any{string(val), found}, []any{"cached", true})
	must(c.CacheSet(ctx, "k2", []byte("v"), 60))
	eq("CacheSet args", host.args["CacheSet"], []any{"k2", "v", 60})
	must(c.CacheDelete(ctx, "k3"))
	eq("CacheDelete args", host.args["CacheDelete"], []any{"k3"})

	status, body, err := c.HTTPRequest(ctx, "POST", "https://a.example/x", map[string]string{"X-A": "1"}, []byte("b"))
	must(err)
	eq("HTTPRequest", []any{status, string(body)}, []any{201, "resp"})
	eq("HTTPRequest args", host.args["HTTPRequest"], []any{"POST", "https://a.example/x", "1", "b"})
	must(c.SendEmail(ctx, "a@b.c", "s", "b", true))
	eq("SendEmail args", host.args["SendEmail"], []any{"a@b.c", "s", "b", true})
	c.Log(ctx, "warn", "m", map[string]any{"k": "v"})
	eq("Log args", host.args["Log"], []any{"warn", "m", "v"})

	cfg, err := c.ConfigGet(ctx, "app.timezone")
	must(err)
	eq("ConfigGet", cfg, "Europe/London")
	eq("Translate", c.Translate(ctx, "hello"), "Hallo")

	res, err := c.CallPlugin(ctx, "other", "fn", json.RawMessage(`{"a":1}`))
	must(err)
	eq("CallPlugin", string(res), `{"r":1}`)
	must(c.PublishEvent(ctx, "ch", "ev", "<p>"))
	eq("PublishEvent args", host.args["PublishEvent"], []any{"ch", "ev", "<p>"})

	must(c.EntitySoftDelete(ctx, "ticket", 7, "why"))
	eq("EntitySoftDelete args", host.args["EntitySoftDelete"], []any{"ticket", int64(7), "why"})
	must(c.EntityRestore(ctx, "ticket", 7))
	eq("EntityRestore args", host.args["EntityRestore"], []any{"ticket", int64(7)})
	must(c.EntityHardDelete(ctx, "agent", 8, "gone"))
	eq("EntityHardDelete args", host.args["EntityHardDelete"], []any{"agent", int64(8), "gone"})
	bin, err := c.RecycleBinList(ctx, "ticket")
	must(err)
	eq("RecycleBinList", string(bin), `[{"id":5}]`)

	sec, err := c.SecureConfigGet(ctx, "token")
	must(err)
	eq("SecureConfigGet", sec, "s3cret")
	must(c.SecureConfigSet(ctx, "token", "v2"))
	eq("SecureConfigSet args", host.args["SecureConfigSet"], []any{"token", "v2"})
	eq("OrgID", c.OrgID(ctx), int64(9))

	cf, err := c.CustomFieldsGet(ctx, "ticket", 3, []string{"score"})
	must(err)
	eq("CustomFieldsGet", cf, map[string]any{"score": float64(4)})
	must(c.CustomFieldsSet(ctx, "ticket", 3, map[string]any{"score": 5}))
	eq("CustomFieldsSet args", host.args["CustomFieldsSet"], []any{"ticket", int64(3), float64(5)})
	ids, err := c.CustomFieldsQuery(ctx, "ticket", []plugin.CustomFieldFilter{{Field: "score", Operator: "gt", Value: 1}})
	must(err)
	eq("CustomFieldsQuery", ids, []int64{1, 2})

	must(c.StoreFile(ctx, "a/b", []byte("data"), map[string]string{"ct": "text/plain"}))
	eq("StoreFile args", host.args["StoreFile"], []any{"a/b", "data", "text/plain"})
	data, meta, err := c.GetFile(ctx, "a/b")
	must(err)
	eq("GetFile", []any{string(data), meta["ct"]}, []any{"file", "text/plain"})
	must(c.DeleteFile(ctx, "a/b"))
	eq("DeleteFile args", host.args["DeleteFile"], []any{"a/b"})
	files, err := c.ListFiles(ctx, "a/")
	must(err)
	eq("ListFiles", files, []plugin.FileInfo{{Key: "a/b", Size: 4}})
	thumb, thumbCT, err := c.GenerateThumbnail(ctx, []byte("img"), "image/jpeg", 0, 50)
	must(err)
	eq("GenerateThumbnail", []any{string(thumb), thumbCT}, []any{"thumb", "image/png"})
	eq("GenerateThumbnail args", host.args["GenerateThumbnail"], []any{"img", "image/jpeg", 200, 50})

	attID, err := c.CreateArticleAttachment(ctx, 21, 2, "r.pdf", "application/pdf", []byte("%PDF"))
	must(err)
	eq("CreateArticleAttachment", attID, int64(11))
	eq("CreateArticleAttachment args", host.args["CreateArticleAttachment"], []any{int64(21), int64(2), "r.pdf", "application/pdf", "%PDF"})
	atts, err := c.ListArticleAttachments(ctx, 21)
	must(err)
	eq("ListArticleAttachments", atts, []plugin.ArticleAttachment{{ID: 1, ArticleID: 21, Filename: "x.pdf"}})
	must(c.DeleteArticleAttachment(ctx, 21, 1))
	eq("DeleteArticleAttachment args", host.args["DeleteArticleAttachment"], []any{int64(21), int64(1)})
	artID, err := c.CreateArticle(ctx, 31, 2, "subj", "body", true)
	must(err)
	eq("CreateArticle", artID, int64(12))
	eq("CreateArticle args", host.args["CreateArticle"], []any{int64(31), int64(2), "subj", "body", true})
	must(c.ChangeTicketStatus(ctx, 31, 6, 2, 1700000000))
	eq("ChangeTicketStatus args", host.args["ChangeTicketStatus"], []any{int64(31), int64(6), int64(2), int64(1700000000)})
	states, err := c.ListTicketStates(ctx)
	must(err)
	eq("ListTicketStates", states, []plugin.TicketStateInfo{{ID: 4, Name: "open"}})
	views, err := c.ListTicketViews(ctx)
	must(err)
	eq("ListTicketViews", views, []plugin.TicketViewInfo{{PluginName: "p", URL: "/x/{ticket_id}"}})
	pdf, err := c.RenderMarkdownToPdf(ctx, "# T", plugin.PdfRenderOptions{Title: "T"})
	must(err)
	eq("RenderMarkdownToPdf", string(pdf), "%PDF")

	// Every HostAPI method reached the host once, as the authenticated plugin
	// (never the name the plugin claims).
	want := reflect.TypeOf((*plugin.HostAPI)(nil)).Elem().NumMethod()
	if len(host.calls) != want {
		t.Errorf("host saw %d calls, want one per HostAPI method (%d): %v", len(host.calls), want, host.calls)
	}
	for method, caller := range host.caller {
		if caller != "contract-plugin" {
			t.Errorf("%s ran as caller %q, want contract-plugin", method, caller)
		}
	}
}

// TestHostCallbacksBeforeInitRefused: until Init binds the plugin's sandbox,
// the plugin process has no host to call.
func TestHostCallbacksBeforeInitRefused(t *testing.T) {
	c := pipeClient(t, newHostBinding(), "p")
	if _, err := c.DBQuery(context.Background(), "SELECT 1"); err == nil || !strings.Contains(err.Error(), "before plugin Init") {
		t.Fatalf("DBQuery before Init: err = %v, want refusal", err)
	}
}

// TestCallTokenCarriesCallContext: a callback that carries the token of an
// in-flight host call runs with that call's context (acting user); a
// callback without a token runs as a system action; a finished call's token
// is refused.
func TestCallTokenCarriesCallContext(t *testing.T) {
	host := newRecordingHost()
	binding := newHostBinding()
	binding.setHost(host)
	server := &HostAPIRPCServer{binding: binding, CallerName: "p"}
	args, _ := json.Marshal(map[string]any{"entity_type": "ticket", "entity_id": 1, "reason": "r"})

	token, end := binding.begin(plugin.WithActingUser(context.Background(), 42))
	var resp HostAPIResponse
	if err := server.Call(HostAPIRequest{Method: "entity_soft_delete", Args: args, CallToken: token}, &resp); err != nil || resp.Error != "" {
		t.Fatalf("call with token: %v %s", err, resp.Error)
	}
	if got := host.actor["EntitySoftDelete"]; got != 42 {
		t.Errorf("acting user with token = %d, want 42", got)
	}

	resp = HostAPIResponse{}
	_ = server.Call(HostAPIRequest{Method: "entity_restore", Args: args}, &resp)
	if resp.Error != "" {
		t.Fatalf("call without token: %s", resp.Error)
	}
	if got := host.actor["EntityRestore"]; got != 0 {
		t.Errorf("acting user without token = %d, want none", got)
	}

	end()
	resp = HostAPIResponse{}
	_ = server.Call(HostAPIRequest{Method: "entity_hard_delete", Args: args, CallToken: token}, &resp)
	if !strings.Contains(resp.Error, "unknown or finished call context") {
		t.Errorf("finished token: error = %q, want refusal", resp.Error)
	}
	if _, called := host.args["EntityHardDelete"]; called {
		t.Error("host ran a callback for a finished call")
	}
}
