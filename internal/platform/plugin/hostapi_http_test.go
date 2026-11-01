package plugin

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
)

// fakeNet resolves test hostnames and routes made-up public addresses to
// loopback test servers, so no test touches real DNS or the internet.
type fakeNet struct {
	names  map[string][]string
	routes map[string]string

	mu     sync.Mutex
	dialed []string
}

func (f *fakeNet) host() *ProdHostAPI {
	h := NewProdHostAPI()
	h.httpOut.lookup = func(_ context.Context, host string) ([]netip.Addr, error) {
		ips, ok := f.names[host]
		if !ok {
			return nil, fmt.Errorf("lookup %s: no such host", host)
		}
		out := make([]netip.Addr, 0, len(ips))
		for _, ip := range ips {
			out = append(out, netip.MustParseAddr(ip))
		}
		return out, nil
	}
	dial := h.httpOut.dial
	h.httpOut.dial = func(ctx context.Context, network, addr string) (net.Conn, error) {
		f.mu.Lock()
		f.dialed = append(f.dialed, addr)
		f.mu.Unlock()
		host, port, _ := net.SplitHostPort(addr)
		if real, ok := f.routes[host]; ok {
			addr = net.JoinHostPort(real, port)
		}
		return dial(ctx, network, addr)
	}
	return h
}

func (f *fakeNet) dials() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.dialed)
}

func httpPlugin(inner HostAPI, scope ...string) *SandboxedHostAPI {
	return NewSandboxedHostAPI(inner, "net-test", ResourcePolicy{
		PluginName:  "net-test",
		Status:      "approved",
		Permissions: []Permission{{Type: "http", Scope: scope}},
	})
}

func serverPort(t *testing.T, ts *httptest.Server) string {
	t.Helper()
	u, err := url.Parse(ts.URL)
	if err != nil {
		t.Fatal(err)
	}
	return u.Port()
}

func countingServer(body string) (*httptest.Server, *atomic.Int64) {
	var hits atomic.Int64
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		hits.Add(1)
		_, _ = w.Write([]byte(body))
	}))
	return ts, &hits
}

func wantRefused(t *testing.T, err error, want string) {
	t.Helper()
	if err == nil {
		t.Fatalf("expected refusal containing %q, request succeeded", want)
	}
	if !strings.Contains(err.Error(), want) {
		t.Fatalf("error %q does not contain %q", err, want)
	}
}

const internalRefusal = "refused: internal address; name the host in the plugin's http permission scope to allow it"

func TestPluginHTTP_InternalAddressNeedsExactScope(t *testing.T) {
	ts, hits := countingServer("ok")
	defer ts.Close()
	fn := &fakeNet{names: map[string][]string{"svc.lan.test": {"127.0.0.1"}}}
	h := fn.host()
	byName := "http://svc.lan.test:" + serverPort(t, ts) + "/"

	tests := []struct {
		name    string
		scope   []string
		url     string
		refusal string
	}{
		{"exact IP literal", []string{"127.0.0.1"}, ts.URL, ""},
		{"exact hostname", []string{"svc.lan.test"}, byName, ""},
		{"exact hostname, other case and trailing dot", []string{"SVC.lan.test."}, byName, ""},
		{"wildcard scope", []string{"*"}, ts.URL, "http request to 127.0.0.1 " + internalRefusal},
		{"domain wildcard scope", []string{"*.lan.test"}, byName, "http request to svc.lan.test (127.0.0.1) " + internalRefusal},
		{"empty scope", nil, ts.URL, "http request to 127.0.0.1 " + internalRefusal},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			before := hits.Load()
			status, body, err := httpPlugin(h, tt.scope...).HTTPRequest(context.Background(), "GET", tt.url, nil, nil)
			if tt.refusal != "" {
				wantRefused(t, err, tt.refusal)
				if hits.Load() != before {
					t.Fatal("refused request reached the server")
				}
				return
			}
			if err != nil {
				t.Fatalf("HTTPRequest: %v", err)
			}
			if status != http.StatusOK || string(body) != "ok" {
				t.Fatalf("got %d %q, want 200 \"ok\"", status, body)
			}
		})
	}

	t.Run("unsandboxed caller", func(t *testing.T) {
		_, _, err := h.HTTPRequest(context.Background(), "GET", ts.URL, nil, nil)
		wantRefused(t, err, "http request to 127.0.0.1 "+internalRefusal)
	})
}

func TestPluginHTTP_PooledConnectionNotSharedAcrossScopes(t *testing.T) {
	ts, hits := countingServer("ok")
	defer ts.Close()
	h := (&fakeNet{}).host()

	if _, _, err := httpPlugin(h, "127.0.0.1").HTTPRequest(context.Background(), "GET", ts.URL, nil, nil); err != nil {
		t.Fatalf("named plugin: %v", err)
	}
	_, _, err := httpPlugin(h, "*").HTTPRequest(context.Background(), "GET", ts.URL, nil, nil)
	wantRefused(t, err, internalRefusal)
	if hits.Load() != 1 {
		t.Fatalf("server saw %d requests, want 1", hits.Load())
	}
}

func TestPluginHTTP_DNSAnswerWithAnyInternalAddressRefused(t *testing.T) {
	ts, hits := countingServer("ok")
	defer ts.Close()
	fn := &fakeNet{
		names:  map[string][]string{"mixed.example.test": {"203.0.113.10", "127.0.0.1"}},
		routes: map[string]string{"203.0.113.10": "127.0.0.1"},
	}
	_, _, err := httpPlugin(fn.host(), "*.example.test").HTTPRequest(context.Background(), "GET",
		"http://mixed.example.test:"+serverPort(t, ts)+"/", nil, nil)
	wantRefused(t, err, "http request to mixed.example.test (127.0.0.1) "+internalRefusal)
	if hits.Load() != 0 || fn.dials() != 0 {
		t.Fatalf("refused request was dialled (%d dials, %d hits)", fn.dials(), hits.Load())
	}
}

func TestPluginHTTP_MetadataAlwaysRefused(t *testing.T) {
	fn := &fakeNet{names: map[string][]string{"metadata.test": {"169.254.169.254"}}}
	h := fn.host()
	for _, raw := range []string{
		"http://169.254.169.254/latest/meta-data/",
		"http://[::ffff:169.254.169.254]/latest/meta-data/",
		"http://[64:ff9b::a9fe:a9fe]/latest/meta-data/",
		"http://[fd00:ec2::254]/latest/meta-data/",
		"http://169.254.170.2/v2/credentials",
		"http://100.100.100.200/latest/meta-data/",
		"http://metadata.test/computeMetadata/v1/",
	} {
		t.Run(raw, func(t *testing.T) {
			u, _ := url.Parse(raw)
			_, _, err := httpPlugin(h, u.Hostname()).HTTPRequest(context.Background(), "GET", raw, nil, nil)
			wantRefused(t, err, "refused: cloud metadata address, never reachable from plugins")
		})
	}
	if fn.dials() != 0 {
		t.Fatalf("metadata address was dialled %d times", fn.dials())
	}
}

func TestPluginHTTP_RedirectRechecked(t *testing.T) {
	target, targetHits := countingServer("internal")
	defer target.Close()
	redirector := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, r.URL.Query().Get("to"), http.StatusFound)
	}))
	defer redirector.Close()
	fn := &fakeNet{
		names: map[string][]string{
			"public.example.test":   {"203.0.113.10"},
			"internal.example.test": {"127.0.0.1"},
		},
		routes: map[string]string{"203.0.113.10": "127.0.0.1"},
	}
	h := fn.host()
	targetPort := serverPort(t, target)
	start := func(to string) string {
		return "http://public.example.test:" + serverPort(t, redirector) + "/?to=" + url.QueryEscape(to)
	}

	t.Run("to internal address under wildcard scope", func(t *testing.T) {
		_, _, err := httpPlugin(h, "*.example.test").HTTPRequest(context.Background(), "GET",
			start("http://internal.example.test:"+targetPort+"/"), nil, nil)
		wantRefused(t, err, "http request to internal.example.test (127.0.0.1) "+internalRefusal)
	})
	t.Run("to host outside scope", func(t *testing.T) {
		_, _, err := httpPlugin(h, "public.example.test").HTTPRequest(context.Background(), "GET",
			start("http://other.example.org:"+targetPort+"/"), nil, nil)
		wantRefused(t, err, "http redirect to other.example.org:"+targetPort+" refused: host not in the plugin's http permission scope")
	})
	if targetHits.Load() != 0 {
		t.Fatalf("refused redirect reached the target %d times", targetHits.Load())
	}
	t.Run("to internal host named in scope", func(t *testing.T) {
		status, body, err := httpPlugin(h, "public.example.test", "internal.example.test").HTTPRequest(context.Background(), "GET",
			start("http://internal.example.test:"+targetPort+"/"), nil, nil)
		if err != nil {
			t.Fatalf("HTTPRequest: %v", err)
		}
		if status != http.StatusOK || string(body) != "internal" {
			t.Fatalf("got %d %q", status, body)
		}
	})
}

func TestPluginHTTP_RedirectLimit(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n, _ := strconv.Atoi(strings.TrimPrefix(r.URL.Path, "/hop/"))
		if n > 0 {
			http.Redirect(w, r, "/hop/"+strconv.Itoa(n-1), http.StatusFound)
			return
		}
		_, _ = w.Write([]byte("done"))
	}))
	defer ts.Close()
	p := httpPlugin((&fakeNet{}).host(), "127.0.0.1")

	_, body, err := p.HTTPRequest(context.Background(), "GET", ts.URL+"/hop/5", nil, nil)
	if err != nil || string(body) != "done" {
		t.Fatalf("5 redirects: body %q, err %v", body, err)
	}
	_, _, err = p.HTTPRequest(context.Background(), "GET", ts.URL+"/hop/6", nil, nil)
	wantRefused(t, err, "http request stopped after 5 redirects")
}

func TestPluginHTTP_CrossHostRedirectDropsCredentials(t *testing.T) {
	type seen struct{ auth, cookie, trace string }
	var mu sync.Mutex
	landed := map[string]seen{}
	record := func(name string) http.HandlerFunc {
		return func(_ http.ResponseWriter, r *http.Request) {
			mu.Lock()
			landed[name] = seen{r.Header.Get("Authorization"), r.Header.Get("Cookie"), r.Header.Get("X-Trace")}
			mu.Unlock()
		}
	}
	other := httptest.NewServer(record("other host"))
	defer other.Close()
	mux := http.NewServeMux()
	mux.HandleFunc("/same", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/landing", http.StatusFound)
	})
	mux.HandleFunc("/cross", func(w http.ResponseWriter, r *http.Request) {
		// A subdomain on another port: net/http alone would keep the credentials.
		http.Redirect(w, r, "http://files.api.example.test:"+serverPort(t, other)+"/landing", http.StatusFound)
	})
	mux.Handle("/landing", record("same host"))
	origin := httptest.NewServer(mux)
	defer origin.Close()

	fn := &fakeNet{
		names: map[string][]string{
			"api.example.test":       {"203.0.113.10"},
			"files.api.example.test": {"203.0.113.10"},
		},
		routes: map[string]string{"203.0.113.10": "127.0.0.1"},
	}
	p := httpPlugin(fn.host(), "*.example.test")
	headers := map[string]string{"Authorization": "Bearer secret", "Cookie": "s=1", "X-Trace": "t"}
	base := "http://api.example.test:" + serverPort(t, origin)
	for _, path := range []string{"/same", "/cross"} {
		if _, _, err := p.HTTPRequest(context.Background(), "GET", base+path, headers, nil); err != nil {
			t.Fatalf("%s: %v", path, err)
		}
	}

	if got := landed["same host"]; got != (seen{"Bearer secret", "s=1", "t"}) {
		t.Errorf("same-host redirect: got %+v, want credentials kept", got)
	}
	if got := landed["other host"]; got != (seen{"", "", "t"}) {
		t.Errorf("cross-host redirect: got %+v, want credentials dropped", got)
	}
}

func TestPluginHTTP_ResponseSizeCap(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n, _ := strconv.Atoi(r.URL.Query().Get("n"))
		if r.URL.Query().Get("stream") != "" {
			w.(http.Flusher).Flush() // no Content-Length: the cap must hold while reading
		}
		chunk := make([]byte, 1<<20)
		for n > 0 {
			k := min(n, len(chunk))
			if _, err := w.Write(chunk[:k]); err != nil {
				return
			}
			n -= k
		}
	}))
	defer ts.Close()
	ctx := context.Background()

	t.Run("default 32 MiB limit, streamed", func(t *testing.T) {
		p := httpPlugin((&fakeNet{}).host(), "127.0.0.1")
		_, body, err := p.HTTPRequest(ctx, "GET", ts.URL+"/?stream=1&n="+strconv.Itoa(32<<20), nil, nil)
		if err != nil || len(body) != 32<<20 {
			t.Fatalf("exactly 32 MiB: %d bytes, err %v", len(body), err)
		}
		_, body, err = p.HTTPRequest(ctx, "GET", ts.URL+"/?stream=1&n="+strconv.Itoa(32<<20+1), nil, nil)
		wantRefused(t, err, "response body exceeds the 32 MiB limit")
		if body != nil {
			t.Fatal("over-size response returned a truncated body")
		}
	})

	t.Run("declared Content-Length over the limit", func(t *testing.T) {
		h := (&fakeNet{}).host()
		h.httpOut.maxBody = 1024
		p := httpPlugin(h, "127.0.0.1")
		_, body, err := p.HTTPRequest(ctx, "GET", ts.URL+"/?n=1024", nil, nil)
		if err != nil || len(body) != 1024 {
			t.Fatalf("under the limit: %d bytes, err %v", len(body), err)
		}
		_, _, err = p.HTTPRequest(ctx, "GET", ts.URL+"/?n=1025", nil, nil)
		wantRefused(t, err, "response body exceeds the 1024-byte limit")
	})
}

func TestIsInternalAddr(t *testing.T) {
	internal := []string{
		"127.0.0.1", "10.0.0.5", "172.16.0.1", "192.168.1.1", "169.254.1.1", "100.64.0.1",
		"0.0.0.0", "0.1.2.3", "198.18.0.1", "224.0.0.1", "255.255.255.255",
		"::1", "::", "fc00::1", "fd12::1", "fe80::1", "ff02::1",
		"::ffff:10.0.0.1", "64:ff9b::a00:1", "64:ff9b:1::1", "2002:a00:1::", "::7f00:1",
	}
	public := []string{
		"8.8.8.8", "1.1.1.1", "203.0.113.10", "2606:4700:4700::1111",
		"::ffff:8.8.8.8", "64:ff9b::808:808", "2002:808:808::",
	}
	for _, s := range internal {
		if !isInternalAddr(netip.MustParseAddr(s)) {
			t.Errorf("%s should be internal", s)
		}
	}
	for _, s := range public {
		if isInternalAddr(netip.MustParseAddr(s)) {
			t.Errorf("%s should not be internal", s)
		}
	}
}
