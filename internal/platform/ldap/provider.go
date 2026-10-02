package ldap

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"log"
	"net"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/go-ldap/ldap/v3"
)

// Authentication outcomes callers map to their own errors.
var (
	ErrUserNotFound       = errors.New("ldap: no directory entry matches the login")
	ErrInvalidCredentials = errors.New("ldap: invalid credentials")
	ErrAmbiguousUser      = errors.New("ldap: user filter matches more than one entry")
	ErrNotAuthorized      = errors.New("ldap: user is not a member of LDAP_AGENT_GROUPS or LDAP_ADMIN_GROUPS")
)

// User is a directory entry whose password has been verified.
type User struct {
	DN          string
	Username    string // GoatFlow login: the UsernameAttribute value, else the typed login
	Email       string
	FirstName   string
	LastName    string
	DisplayName string
	Groups      []string // GroupAttribute values of the groups the user belongs to
}

// Client authenticates users against one LDAP server. It opens a new
// connection per call, so it is safe for concurrent use.
type Client struct {
	cfg       *Config
	tlsConfig *tls.Config
}

// NewClient builds a client; it fails when the configured CA file is unusable.
func NewClient(cfg *Config) (*Client, error) {
	c := &Client{cfg: cfg}
	if cfg.UseSSL || cfg.UseTLS {
		tc := &tls.Config{
			ServerName:         cfg.Host,
			MinVersion:         tls.VersionTLS12,
			InsecureSkipVerify: cfg.SkipTLSVerify, //nolint:gosec // explicit LDAP_SKIP_TLS_VERIFY opt-out
		}
		if cfg.CACertFile != "" {
			pem, err := os.ReadFile(cfg.CACertFile)
			if err != nil {
				return nil, fmt.Errorf("LDAP_TLS_CA_FILE: %w", err)
			}
			pool := x509.NewCertPool()
			if !pool.AppendCertsFromPEM(pem) {
				return nil, fmt.Errorf("LDAP_TLS_CA_FILE=%q contains no PEM certificates", cfg.CACertFile)
			}
			tc.RootCAs = pool
		}
		c.tlsConfig = tc
	}
	return c, nil
}

// Authenticate looks the login up and verifies password by binding as the
// matching entry. An empty password always fails: LDAP treats a simple bind
// with an empty password as an anonymous bind that "succeeds".
func (c *Client) Authenticate(ctx context.Context, login, password string) (*User, error) {
	login = strings.TrimSpace(login)
	if login == "" || password == "" {
		return nil, ErrInvalidCredentials
	}

	conn, release, err := c.connect(ctx)
	if err != nil {
		return nil, err
	}
	defer release()

	entry, err := c.findUser(conn, login)
	if err != nil {
		return nil, err
	}
	user := c.userFromEntry(entry, login)

	// Group lookup runs with the search identity, before the connection is
	// re-bound as the user. A failed lookup fails the login (no fail-open).
	if c.cfg.GroupBaseDN != "" {
		if user.Groups, err = c.findGroups(conn, user); err != nil {
			return nil, err
		}
	}

	if err := conn.Bind(user.DN, password); err != nil {
		if isCredentialRejection(err) {
			return nil, ErrInvalidCredentials
		}
		return nil, fmt.Errorf("ldap: user bind: %w", err)
	}

	if len(c.cfg.AgentGroups) > 0 && !c.IsAdmin(user) && !hasAnyGroup(user.Groups, c.cfg.AgentGroups) {
		return nil, ErrNotAuthorized
	}
	return user, nil
}

// IsAdmin reports whether the user belongs to one of LDAP_ADMIN_GROUPS.
func (c *Client) IsAdmin(u *User) bool { return hasAnyGroup(u.Groups, c.cfg.AdminGroups) }

// connect dials, upgrades with StartTLS when configured and binds with the
// service account. The connection is closed when ctx ends or release is called.
func (c *Client) connect(ctx context.Context) (conn *ldap.Conn, release func(), err error) {
	timeout := c.cfg.Timeout
	if dl, ok := ctx.Deadline(); ok {
		if left := time.Until(dl); left < timeout {
			timeout = left
		}
	}
	if timeout <= 0 || ctx.Err() != nil {
		return nil, nil, fmt.Errorf("ldap: %w", context.DeadlineExceeded)
	}

	scheme := "ldap"
	opts := []ldap.DialOpt{ldap.DialWithDialer(&net.Dialer{Timeout: timeout})}
	if c.cfg.UseSSL {
		scheme = "ldaps"
		opts = append(opts, ldap.DialWithTLSConfig(c.tlsConfig))
	}
	addr := scheme + "://" + net.JoinHostPort(c.cfg.Host, strconv.Itoa(c.cfg.Port))
	conn, err = ldap.DialURL(addr, opts...)
	if err != nil {
		return nil, nil, fmt.Errorf("ldap: connect %s: %w", addr, err)
	}
	conn.SetTimeout(timeout)
	stop := context.AfterFunc(ctx, func() { _ = conn.Close() })
	release = func() {
		stop()
		_ = conn.Close()
	}

	if c.cfg.UseTLS {
		if err := conn.StartTLS(c.tlsConfig); err != nil {
			release()
			return nil, nil, fmt.Errorf("ldap: StartTLS with %s: %w", addr, err)
		}
	}
	if c.cfg.BindDN != "" {
		if err := conn.Bind(c.cfg.BindDN, c.cfg.BindPassword); err != nil {
			release()
			return nil, nil, fmt.Errorf("ldap: service account bind as %q: %w", c.cfg.BindDN, err)
		}
	}
	return conn, release, nil
}

func (c *Client) timeLimit() int {
	s := int(c.cfg.Timeout / time.Second)
	if s < 1 {
		s = 1
	}
	return s
}

// UserFilter returns the search filter for login with every filter
// metacharacter in login escaped (RFC 4515), so input cannot alter the filter.
func (c *Client) UserFilter(login string) string {
	escaped := ldap.EscapeFilter(login)
	filter := fillFilter(c.cfg.UserFilter, escaped)
	if c.cfg.IsActiveDirectory {
		switch {
		case strings.Contains(login, "@"):
			filter = "(|" + filter + "(userPrincipalName=" + escaped + "))"
		case c.cfg.Domain != "":
			filter = "(|" + filter + "(userPrincipalName=" + escaped + "@" + ldap.EscapeFilter(c.cfg.Domain) + "))"
		}
	}
	return filter
}

// GroupFilter returns the group search filter for user, escaped like UserFilter.
func (c *Client) GroupFilter(u *User) string {
	member := u.DN
	if c.cfg.GroupMemberValue == GroupMemberUsername {
		member = u.Username
	}
	return fillFilter(c.cfg.GroupFilter, ldap.EscapeFilter(member))
}

func (c *Client) findUser(conn *ldap.Conn, login string) (*ldap.Entry, error) {
	base := c.cfg.UserBaseDN
	if base == "" {
		base = c.cfg.BaseDN
	}
	var attrs []string
	for _, a := range []string{c.cfg.UsernameAttribute, c.cfg.EmailAttribute, c.cfg.FirstNameAttribute,
		c.cfg.LastNameAttribute, c.cfg.DisplayNameAttribute} {
		if a != "" {
			attrs = append(attrs, a)
		}
	}
	if len(attrs) == 0 {
		attrs = []string{"1.1"}
	}
	// Size limit 2: a filter matching several entries must not log in as an
	// arbitrary one of them.
	res, err := conn.Search(ldap.NewSearchRequest(base, ldap.ScopeWholeSubtree, ldap.NeverDerefAliases,
		2, c.timeLimit(), false, c.UserFilter(login), attrs, nil))
	if err != nil {
		if ldap.IsErrorWithCode(err, ldap.LDAPResultSizeLimitExceeded) {
			return nil, ErrAmbiguousUser
		}
		return nil, fmt.Errorf("ldap: user search: %w", err)
	}
	switch len(res.Entries) {
	case 0:
		return nil, ErrUserNotFound
	case 1:
		return res.Entries[0], nil
	default:
		return nil, ErrAmbiguousUser
	}
}

func (c *Client) userFromEntry(e *ldap.Entry, login string) *User {
	get := func(attr string) string {
		if attr == "" {
			return ""
		}
		return strings.TrimSpace(e.GetAttributeValue(attr))
	}
	u := &User{
		DN:          e.DN,
		Username:    get(c.cfg.UsernameAttribute),
		Email:       get(c.cfg.EmailAttribute),
		FirstName:   get(c.cfg.FirstNameAttribute),
		LastName:    get(c.cfg.LastNameAttribute),
		DisplayName: get(c.cfg.DisplayNameAttribute),
	}
	if u.Username == "" {
		u.Username = login
	}
	return u
}

func (c *Client) findGroups(conn *ldap.Conn, u *User) ([]string, error) {
	res, err := conn.Search(ldap.NewSearchRequest(c.cfg.GroupBaseDN, ldap.ScopeWholeSubtree, ldap.NeverDerefAliases,
		0, c.timeLimit(), false, c.GroupFilter(u), []string{c.cfg.GroupAttribute}, nil))
	if err != nil {
		return nil, fmt.Errorf("ldap: group search: %w", err)
	}
	groups := make([]string, 0, len(res.Entries))
	for _, e := range res.Entries {
		if name := e.GetAttributeValue(c.cfg.GroupAttribute); name != "" {
			groups = append(groups, name)
		}
	}
	return groups, nil
}

// isCredentialRejection reports bind results that mean "wrong password or
// account not allowed to log in" rather than a directory failure.
func isCredentialRejection(err error) bool {
	for _, code := range []uint16{
		ldap.LDAPResultInvalidCredentials,
		ldap.LDAPResultInappropriateAuthentication,
		ldap.LDAPResultUnwillingToPerform,
		ldap.LDAPResultInsufficientAccessRights,
	} {
		if ldap.IsErrorWithCode(err, code) {
			return true
		}
	}
	return false
}

func hasAnyGroup(groups, wanted []string) bool {
	for _, g := range groups {
		for _, w := range wanted {
			if strings.EqualFold(g, w) {
				return true
			}
		}
	}
	return false
}

// WarnInsecure logs settings that weaken transport security.
func (c *Client) WarnInsecure() {
	switch {
	case !c.cfg.UseSSL && !c.cfg.UseTLS:
		log.Printf("WARNING: LDAP to %s uses plain ldap:// without LDAP_USE_TLS or LDAP_USE_SSL; passwords cross the network unencrypted", c.cfg.Host)
	case c.cfg.SkipTLSVerify:
		log.Printf("WARNING: LDAP_SKIP_TLS_VERIFY=true: the certificate of %s is not verified", c.cfg.Host)
	}
}
