package plugin

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/netip"
	"slices"
	"strings"
	"sync"
	"time"
)

const (
	pluginHTTPTimeout          = 30 * time.Second
	pluginHTTPMaxRedirects     = 5
	pluginHTTPMaxResponseBytes = 32 << 20
	// Distinct exact-host allowlists get their own transport so a pooled
	// connection to an internal host is never reused under a narrower scope.
	maxPluginHTTPTransports = 64
)

// Never reachable from plugins, whatever the scope names: cloud instance
// metadata endpoints hand out the host's credentials.
var metadataAddrs = []netip.Addr{
	netip.MustParseAddr("169.254.169.254"), // AWS, Azure, GCP, OpenStack, Oracle
	netip.MustParseAddr("169.254.170.2"),   // AWS ECS task metadata
	netip.MustParseAddr("100.100.100.200"), // Alibaba Cloud
	netip.MustParseAddr("fd00:ec2::254"),   // AWS IMDS over IPv6
}

var internalPrefixes = []netip.Prefix{
	netip.MustParsePrefix("0.0.0.0/8"),
	netip.MustParsePrefix("100.64.0.0/10"),
	netip.MustParsePrefix("192.0.0.0/24"),
	netip.MustParsePrefix("198.18.0.0/15"),
	netip.MustParsePrefix("240.0.0.0/4"),
	netip.MustParsePrefix("64:ff9b:1::/48"),
	netip.MustParsePrefix("100::/64"),
	netip.MustParsePrefix("fec0::/10"),
}

var (
	nat64Prefix = netip.MustParsePrefix("64:ff9b::/96")
	sixToFour   = netip.MustParsePrefix("2002::/16")
	v4Compat    = netip.MustParsePrefix("::/96")
)

// effectiveAddr unwraps IPv4 addresses carried inside IPv6 forms
// (IPv4-mapped, IPv4-compatible, NAT64 64:ff9b::/96, 6to4) so they are
// judged as the IPv4 address they reach.
func effectiveAddr(a netip.Addr) netip.Addr {
	a = a.Unmap().WithZone("")
	if !a.Is6() {
		return a
	}
	b := a.As16()
	switch {
	case nat64Prefix.Contains(a):
		return netip.AddrFrom4([4]byte(b[12:16]))
	case sixToFour.Contains(a):
		return netip.AddrFrom4([4]byte(b[2:6]))
	case v4Compat.Contains(a) && !a.IsUnspecified() && !a.IsLoopback():
		return netip.AddrFrom4([4]byte(b[12:16]))
	}
	return a
}

func isMetadataAddr(a netip.Addr) bool {
	return slices.Contains(metadataAddrs, effectiveAddr(a))
}

// isInternalAddr reports whether a is loopback, private, link-local,
// CGNAT, unspecified, multicast or otherwise not a public unicast address.
func isInternalAddr(a netip.Addr) bool {
	a = a.Unmap().WithZone("")
	for _, p := range internalPrefixes {
		if p.Contains(a) {
			return true
		}
	}
	if e := effectiveAddr(a); e != a {
		return isInternalAddr(e)
	}
	return a.IsLoopback() || a.IsPrivate() || a.IsLinkLocalUnicast() || a.IsMulticast() || a.IsUnspecified()
}

// normaliseHost lower-cases a host, drops a trailing dot and IPv6
// brackets, and writes IP literals in canonical form.
func normaliseHost(h string) string {
	h = strings.TrimSuffix(strings.ToLower(strings.TrimSpace(h)), ".")
	h = strings.TrimSuffix(strings.TrimPrefix(h, "["), "]")
	if a, err := netip.ParseAddr(h); err == nil {
		return a.Unmap().WithZone("").String()
	}
	return h
}

// httpRefusedError is returned when the host's HTTP client refuses a
// plugin request by policy rather than because the request failed.
type httpRefusedError struct{ msg string }

func (e *httpRefusedError) Error() string { return e.msg }

type httpScopeKeyType struct{}

var httpScopeKey = httpScopeKeyType{}

// httpScope is a sandboxed plugin's http permission scope, carried on the
// request context to the host's HTTP client.
type httpScope struct {
	patterns []string
	all      bool
}

func withHTTPScope(ctx context.Context, s httpScope) context.Context {
	return context.WithValue(ctx, httpScopeKey, s)
}

// allows reports whether the scope's host patterns admit rawURL.
func (s httpScope) allows(rawURL string) bool {
	if s.all {
		return true
	}
	for _, p := range s.patterns {
		if matchURLPattern(p, rawURL) {
			return true
		}
	}
	return false
}

// exactHosts returns the sorted, de-duplicated patterns that name a single
// host. Only these may reach internal addresses.
func (s httpScope) exactHosts() []string {
	hosts := make([]string, 0, len(s.patterns))
	for _, p := range s.patterns {
		if h := normaliseHost(p); h != "" && !strings.Contains(h, "*") {
			hosts = append(hosts, h)
		}
	}
	slices.Sort(hosts)
	return slices.Compact(hosts)
}

// pluginHTTP is the host's outbound HTTP client for plugins. Every dialled
// address is checked after resolution, so DNS answers cannot smuggle a
// request to an internal address.
type pluginHTTP struct {
	lookup  func(ctx context.Context, host string) ([]netip.Addr, error)
	dial    func(ctx context.Context, network, addr string) (net.Conn, error)
	maxBody int64

	mu         sync.Mutex
	transports map[string]*http.Transport
}

func newPluginHTTP() *pluginHTTP {
	dialer := &net.Dialer{Timeout: pluginHTTPTimeout, KeepAlive: 30 * time.Second}
	return &pluginHTTP{
		lookup: func(ctx context.Context, host string) ([]netip.Addr, error) {
			return net.DefaultResolver.LookupNetIP(ctx, "ip", host)
		},
		dial:       dialer.DialContext,
		maxBody:    pluginHTTPMaxResponseBytes,
		transports: make(map[string]*http.Transport),
	}
}

// transport returns the shared transport for an exact-host allowlist.
// No proxy is used: the destination check must see the real target.
func (p *pluginHTTP) transport(allowed []string) *http.Transport {
	key := strings.Join(allowed, " ")
	p.mu.Lock()
	defer p.mu.Unlock()
	if t, ok := p.transports[key]; ok {
		return t
	}
	if len(p.transports) >= maxPluginHTTPTransports {
		for k, t := range p.transports {
			t.CloseIdleConnections()
			delete(p.transports, k)
		}
	}
	t := &http.Transport{
		DialContext:           p.guardedDial(allowed),
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          100,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: time.Second,
	}
	p.transports[key] = t
	return t
}

// guardedDial resolves the host itself, refuses the connection if any
// resolved address is a metadata address, or internal while the host is
// not in allowed, then dials the checked addresses directly.
func (p *pluginHTTP) guardedDial(allowed []string) func(ctx context.Context, network, addr string) (net.Conn, error) {
	return func(ctx context.Context, network, addr string) (net.Conn, error) {
		host, port, err := net.SplitHostPort(addr)
		if err != nil {
			return nil, err
		}
		var ips []netip.Addr
		if ip, perr := netip.ParseAddr(host); perr == nil {
			ips = []netip.Addr{ip}
		} else if ips, err = p.lookup(ctx, host); err != nil {
			return nil, err
		}
		if len(ips) == 0 {
			return nil, fmt.Errorf("lookup %s: no addresses", host)
		}
		named := slices.Contains(allowed, normaliseHost(host))
		for _, ip := range ips {
			if err := checkDestination(host, ip, named); err != nil {
				return nil, err
			}
		}
		var lastErr error
		for _, ip := range ips {
			conn, err := p.dial(ctx, network, net.JoinHostPort(ip.Unmap().String(), port))
			if err == nil {
				return conn, nil
			}
			lastErr = err
		}
		return nil, lastErr
	}
}

func checkDestination(host string, ip netip.Addr, named bool) error {
	target := host
	if normaliseHost(host) != normaliseHost(ip.String()) {
		target = fmt.Sprintf("%s (%s)", host, ip)
	}
	if isMetadataAddr(ip) {
		return &httpRefusedError{fmt.Sprintf("http request to %s refused: cloud metadata address, never reachable from plugins", target)}
	}
	if isInternalAddr(ip) && !named {
		return &httpRefusedError{fmt.Sprintf("http request to %s refused: internal address; name the host in the plugin's http permission scope to allow it", target)}
	}
	return nil
}

// checkRedirect limits redirects, re-checks each hop against the plugin's
// host patterns (when the request came through a sandbox) and drops
// credentials when a hop leaves the original host.
func (s httpScope) checkRedirect(scoped bool) func(*http.Request, []*http.Request) error {
	return func(req *http.Request, via []*http.Request) error {
		if len(via) > pluginHTTPMaxRedirects {
			return &httpRefusedError{fmt.Sprintf("http request stopped after %d redirects", pluginHTTPMaxRedirects)}
		}
		if scoped && !s.allows(req.URL.String()) {
			return &httpRefusedError{fmt.Sprintf("http redirect to %s refused: host not in the plugin's http permission scope", req.URL.Host)}
		}
		if !strings.EqualFold(req.URL.Host, via[0].URL.Host) {
			for _, h := range []string{"Authorization", "Proxy-Authorization", "Cookie", "Cookie2", "Www-Authenticate"} {
				req.Header.Del(h)
			}
		}
		return nil
	}
}

// do sends req under the scope carried on its context; requests without
// one (unsandboxed callers) get the strict default: no internal addresses.
func (p *pluginHTTP) do(req *http.Request) (*http.Response, error) {
	scope, scoped := req.Context().Value(httpScopeKey).(httpScope)
	client := &http.Client{
		Transport:     p.transport(scope.exactHosts()),
		Timeout:       pluginHTTPTimeout,
		CheckRedirect: scope.checkRedirect(scoped),
	}
	resp, err := client.Do(req)
	if err != nil {
		var refused *httpRefusedError
		if errors.As(err, &refused) {
			return nil, refused
		}
		return nil, fmt.Errorf("request failed: %w", err)
	}
	return resp, nil
}

// readBody reads at most maxBody bytes and errors when the body is larger.
func (p *pluginHTTP) readBody(resp *http.Response) ([]byte, error) {
	if resp.ContentLength > p.maxBody {
		return nil, p.tooLarge()
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, p.maxBody+1))
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}
	if int64(len(data)) > p.maxBody {
		return nil, p.tooLarge()
	}
	return data, nil
}

func (p *pluginHTTP) tooLarge() error {
	if p.maxBody >= 1<<20 && p.maxBody%(1<<20) == 0 {
		return fmt.Errorf("response body exceeds the %d MiB limit", p.maxBody>>20)
	}
	return fmt.Errorf("response body exceeds the %d-byte limit", p.maxBody)
}
