package webhook

import (
	"context"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBlockedAddr(t *testing.T) {
	blocked := []string{
		"127.0.0.1", "127.255.255.254", "::1", "::ffff:127.0.0.1",
		"10.0.0.0", "10.255.255.255", "172.16.0.0", "172.31.255.255", "192.168.0.1",
		"fc00::1", "fd00:ec2::254", "fdff:ffff::1",
		"169.254.0.1", "169.254.169.254", "fe80::1",
		"0.0.0.0", "0.1.2.3", "::",
		"224.0.0.1", "239.255.255.250", "ff02::1",
		"100.64.0.1", "100.100.100.200", "100.127.255.255",
		"240.0.0.1", "255.255.255.255",
	}
	allowed := []string{
		"8.8.8.8", "1.1.1.1", "172.15.255.255", "172.32.0.0", "192.169.0.1", "169.255.0.1",
		"100.63.255.255", "100.128.0.0", "11.0.0.1", "2606:4700:4700::1111", "2001:4860:4860::8888",
		"::ffff:8.8.8.8",
	}
	for _, s := range blocked {
		assert.True(t, blockedAddr(netip.MustParseAddr(s)), "%s must be blocked", s)
	}
	for _, s := range allowed {
		assert.False(t, blockedAddr(netip.MustParseAddr(s)), "%s must be allowed", s)
	}
}

type fakeResolver map[string][]netip.Addr

func (f fakeResolver) LookupNetIP(_ context.Context, _, host string) ([]netip.Addr, error) {
	if addrs, ok := f[host]; ok {
		return addrs, nil
	}
	return nil, &net.DNSError{Err: "no such host", Name: host, IsNotFound: true}
}

// A host is refused when any of its addresses is internal, even if another is
// public, and nothing is dialed: the dialer would otherwise fall back to the
// internal address when the public one is unreachable.
func TestTargetDialerChecksEveryResolvedAddress(t *testing.T) {
	t.Setenv(AllowPrivateTargetsEnv, "")
	d := &targetDialer{
		resolver: fakeResolver{
			"mixed.example": {netip.MustParseAddr("203.0.113.10"), netip.MustParseAddr("10.0.0.7")},
			"meta.example":  {netip.MustParseAddr("::ffff:169.254.169.254")},
		},
		dialer: &net.Dialer{},
	}
	for host, want := range map[string]string{"mixed.example": "10.0.0.7", "meta.example": "169.254.169.254"} {
		conn, err := d.DialContext(context.Background(), "tcp", net.JoinHostPort(host, "443"))
		require.Nil(t, conn)
		var blocked *BlockedTargetError
		require.True(t, errors.As(err, &blocked), "%s: %v", host, err)
		assert.Equal(t, want, blocked.Addr.String())
		assert.Contains(t, err.Error(), "webhook host "+host+" resolves to "+want)
	}
}

// The dialer connects to the address it checked, not to a fresh resolution of
// the name, so a DNS answer that changes after the check cannot redirect it.
func TestTargetDialerDialsTheCheckedAddress(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer ln.Close()
	go func() {
		if c, err := ln.Accept(); err == nil {
			c.Close()
		}
	}()
	_, port, _ := net.SplitHostPort(ln.Addr().String())

	t.Setenv(AllowPrivateTargetsEnv, "true")
	d := &targetDialer{
		resolver: fakeResolver{"pinned.example": {netip.MustParseAddr("127.0.0.1")}},
		dialer:   &net.Dialer{},
	}
	conn, err := d.DialContext(context.Background(), "tcp", net.JoinHostPort("pinned.example", port))
	require.NoError(t, err)
	assert.Equal(t, ln.Addr().String(), conn.RemoteAddr().String())
	conn.Close()
}

// NewHTTPClient carries the guard to other admin-supplied URLs: an internal
// target fails with BlockedTargetError and is not contacted; redirects are
// not followed.
func TestNewHTTPClientGuardsTargets(t *testing.T) {
	var hits int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&hits, 1)
		if r.URL.Path == "/redirect" {
			http.Redirect(w, r, "/elsewhere", http.StatusFound)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()
	client := NewHTTPClient(5 * time.Second)

	t.Setenv(AllowPrivateTargetsEnv, "")
	_, err := client.Get(srv.URL + "/")
	require.Error(t, err)
	var blocked *BlockedTargetError
	assert.True(t, errors.As(err, &blocked), "got %v", err)
	assert.Equal(t, int32(0), atomic.LoadInt32(&hits))

	t.Setenv(AllowPrivateTargetsEnv, "true")
	resp, err := client.Get(srv.URL + "/redirect")
	require.NoError(t, err)
	resp.Body.Close()
	assert.Equal(t, http.StatusFound, resp.StatusCode, "redirects are not followed")
	assert.Equal(t, int32(1), atomic.LoadInt32(&hits))
}
