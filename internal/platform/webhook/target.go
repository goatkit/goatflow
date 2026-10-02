package webhook

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/netip"
	"os"
	"strconv"
	"strings"
	"time"
)

// AllowPrivateTargetsEnv names the environment variable that lets webhooks
// reach loopback, private, link-local and other internal addresses. It is an
// operator setting (not editable in the admin UI) because a webhook URL is
// otherwise a way for an admin account to make the server call internal
// services such as cloud metadata endpoints. Default: not allowed.
const AllowPrivateTargetsEnv = "GOATFLOW_WEBHOOK_ALLOW_PRIVATE_TARGETS"

// AllowPrivateTargets reports whether AllowPrivateTargetsEnv is set to a true
// value ("true", "1", ...). It is read on every check, so tests and operators
// do not depend on process start order.
func AllowPrivateTargets() bool {
	v, err := strconv.ParseBool(strings.TrimSpace(os.Getenv(AllowPrivateTargetsEnv)))
	return err == nil && v
}

// extraBlocked lists ranges blocked in addition to the netip classifications
// (loopback, private, link-local, multicast, unspecified).
var extraBlocked = []netip.Prefix{
	netip.MustParsePrefix("0.0.0.0/8"),     // "this network": 0.0.0.x reaches the local host on Linux
	netip.MustParsePrefix("100.64.0.0/10"), // carrier-grade NAT shared space, also used for cloud metadata
	netip.MustParsePrefix("240.0.0.0/4"),   // reserved, including the 255.255.255.255 broadcast address
}

// blockedAddr reports whether deliveries must not connect to ip unless
// private targets are allowed: loopback (127.0.0.0/8, ::1), private
// (10/8, 172.16/12, 192.168/16, fc00::/7), link-local (169.254/16 including
// the 169.254.169.254 metadata address, fe80::/10), multicast, unspecified,
// and the extraBlocked ranges. IPv4-mapped IPv6 addresses are checked as IPv4.
func blockedAddr(ip netip.Addr) bool {
	ip = ip.Unmap()
	if !ip.IsValid() || ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() ||
		ip.IsLinkLocalMulticast() || ip.IsInterfaceLocalMulticast() || ip.IsMulticast() || ip.IsUnspecified() {
		return true
	}
	for _, p := range extraBlocked {
		if p.Contains(ip) {
			return true
		}
	}
	return false
}

// BlockedTargetError is returned when a webhook host is, or resolves to, an
// address deliveries may not reach.
type BlockedTargetError struct {
	Host string
	Addr netip.Addr
}

func (e *BlockedTargetError) Error() string {
	if e.Addr.IsValid() && e.Addr.String() != e.Host {
		return fmt.Sprintf("webhook host %s resolves to %s, a loopback, private, link-local or other internal address (set %s=true to allow internal targets)",
			e.Host, e.Addr, AllowPrivateTargetsEnv)
	}
	return fmt.Sprintf("webhook host %s is a loopback, private, link-local or other internal address (set %s=true to allow internal targets)",
		e.Host, AllowPrivateTargetsEnv)
}

// CheckTargetHost validates the host of an admin-supplied URL (webhook,
// identity-provider metadata) when it is saved. Only IP literals and the
// always-loopback name "localhost" (RFC 6761) can be judged without DNS; other
// names are checked on every connection, after resolution, by targetDialer.
func CheckTargetHost(host string) error {
	if AllowPrivateTargets() {
		return nil
	}
	h := strings.TrimSuffix(strings.ToLower(host), ".")
	if h == "localhost" || strings.HasSuffix(h, ".localhost") {
		return &BlockedTargetError{Host: host}
	}
	if ip, err := netip.ParseAddr(h); err == nil && blockedAddr(ip) {
		return &BlockedTargetError{Host: host, Addr: ip}
	}
	return nil
}

// ipResolver is the part of *net.Resolver the dialer needs.
type ipResolver interface {
	LookupNetIP(ctx context.Context, network, host string) ([]netip.Addr, error)
}

// targetDialer resolves the delivery host itself, refuses the connection when
// any resolved address is blocked, and then connects to one of exactly those
// addresses. Because the checked addresses are the ones dialed, a DNS answer
// that changes between check and connect (DNS rebinding) cannot redirect the
// request to an internal address.
type targetDialer struct {
	resolver ipResolver
	dialer   *net.Dialer
}

func (d *targetDialer) DialContext(ctx context.Context, network, address string) (net.Conn, error) {
	host, port, err := net.SplitHostPort(address)
	if err != nil {
		return nil, err
	}
	var addrs []netip.Addr
	if ip, perr := netip.ParseAddr(host); perr == nil {
		addrs = []netip.Addr{ip}
	} else {
		addrs, err = d.resolver.LookupNetIP(ctx, "ip", host)
		if err != nil {
			return nil, err
		}
		if len(addrs) == 0 {
			return nil, fmt.Errorf("webhook host %s has no IP address", host)
		}
	}
	if !AllowPrivateTargets() {
		for _, ip := range addrs {
			if blockedAddr(ip) {
				return nil, &BlockedTargetError{Host: host, Addr: ip.Unmap()}
			}
		}
	}
	var firstErr error
	for _, ip := range addrs {
		conn, err := d.dialer.DialContext(ctx, network, net.JoinHostPort(ip.Unmap().String(), port))
		if err == nil {
			return conn, nil
		}
		if firstErr == nil {
			firstErr = err
		}
	}
	return nil, firstErr
}

// newHTTPClient returns a client that connects through targetDialer. It does
// not follow redirects (the URL must point at the endpoint itself) and does
// not use HTTP(S)_PROXY: a proxy would resolve the host itself, out of reach
// of the address check.
func newHTTPClient(resolver ipResolver) *http.Client {
	d := &targetDialer{
		resolver: resolver,
		dialer:   &net.Dialer{Timeout: 10 * time.Second, KeepAlive: 30 * time.Second},
	}
	return &http.Client{
		Transport: &http.Transport{
			Proxy:                 nil,
			DialContext:           d.DialContext,
			ForceAttemptHTTP2:     true,
			MaxIdleConns:          100,
			MaxIdleConnsPerHost:   4,
			IdleConnTimeout:       90 * time.Second,
			TLSHandshakeTimeout:   10 * time.Second,
			ExpectContinueTimeout: time.Second,
		},
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}
}

// NewHTTPClient returns a client with the delivery address guard (every
// resolved address checked and pinned, no redirects, no proxy) for other
// admin-supplied URLs the server fetches, such as identity-provider metadata
// and OIDC discovery documents. Requests give up after timeout.
func NewHTTPClient(timeout time.Duration) *http.Client {
	c := newHTTPClient(net.DefaultResolver)
	c.Timeout = timeout
	return c
}

// deliveryClient is shared by every Dispatcher so idle connections are
// pooled per process instead of per request.
var deliveryClient = newHTTPClient(net.DefaultResolver)

// isBlockedTarget reports whether err comes from the address check.
func isBlockedTarget(err error) bool {
	var b *BlockedTargetError
	return errors.As(err, &b)
}
