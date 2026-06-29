package connector

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"

	"github.com/knadh/go-pop3"
)

type pop3Connection interface {
	Auth(user, password string) error
	Quit() error
	Uidl(msgID int) ([]pop3.MessageID, error)
	List(msgID int) ([]pop3.MessageID, error)
	RetrRaw(msgID int) (*bytes.Buffer, error)
	Dele(msgID ...int) error
}

// authNoopSkipper wraps a POP3 connection and ignores servers that don't support NOOP.
type authNoopSkipper struct{ *pop3.Conn }

func (c *authNoopSkipper) Auth(user, password string) error {
	if err := c.Conn.User(user); err != nil {
		return err
	}
	if err := c.Conn.Pass(password); err != nil {
		return err
	}
	if err := c.Conn.Noop(); err != nil {
		msg := strings.ToLower(err.Error())
		if strings.Contains(msg, "unknown command") {
			return nil
		}
		return err
	}
	return nil
}

type pop3ConnFactory func(Account) (pop3Connection, error)

// POP3Fetcher streams POP3/POP3S mailboxes into the inbound pipeline.
type POP3Fetcher struct {
	deleteAfterFetch bool
	dialTimeout      time.Duration
	now              func() time.Time
	logger           *log.Logger
	newConn          pop3ConnFactory
}

// POP3FetcherOption customizes fetcher behavior.
type POP3FetcherOption func(*POP3Fetcher)

// NewPOP3Fetcher returns a POP3 connector ready for dispatch polling.
func NewPOP3Fetcher(opts ...POP3FetcherOption) *POP3Fetcher {
	f := &POP3Fetcher{
		deleteAfterFetch: true,
		dialTimeout:      5 * time.Second,
		now:              func() time.Time { return time.Now().UTC() },
		logger:           log.Default(),
	}
	f.newConn = f.defaultConnFactory
	for _, opt := range opts {
		opt(f)
	}
	if f.newConn == nil {
		f.newConn = f.defaultConnFactory
	}
	return f
}

// WithPOP3DeleteAfterFetch toggles destructive POP3 behavior.
func WithPOP3DeleteAfterFetch(delete bool) POP3FetcherOption {
	return func(f *POP3Fetcher) {
		f.deleteAfterFetch = delete
	}
}

// WithPOP3Logger overrides the logger used for connector diagnostics.
func WithPOP3Logger(logger *log.Logger) POP3FetcherOption {
	return func(f *POP3Fetcher) {
		if logger != nil {
			f.logger = logger
		}
	}
}

// WithPOP3DialTimeout overrides the socket dial timeout.
func WithPOP3DialTimeout(timeout time.Duration) POP3FetcherOption {
	return func(f *POP3Fetcher) {
		if timeout > 0 {
			f.dialTimeout = timeout
		}
	}
}

func withPOP3ConnFactory(factory pop3ConnFactory) POP3FetcherOption {
	return func(f *POP3Fetcher) {
		f.newConn = factory
	}
}

// WithPOP3Clock overrides the wall clock, primarily for tests.
func WithPOP3Clock(now func() time.Time) POP3FetcherOption {
	return func(f *POP3Fetcher) {
		if now != nil {
			f.now = now
		}
	}
}

// Name returns the connector identifier.
func (f *POP3Fetcher) Name() string {
	return "pop3"
}

// Fetch drains a POP3 mailbox and hands each message to the provided handler.
func (f *POP3Fetcher) Fetch(ctx context.Context, account Account, handler Handler) error {
	if handler == nil {
		return errors.New("pop3 fetcher requires a handler")
	}
	if err := validateAccount(account); err != nil {
		return err
	}

	conn, err := f.newConn(account)
	if err != nil {
		return fmt.Errorf("pop3 connect: %w", err)
	}
	defer f.safeQuit(conn)

	if err := conn.Auth(account.Username, string(account.Password)); err != nil {
		return fmt.Errorf("pop3 auth: %w", err)
	}

	msgs, err := conn.Uidl(0)
	if err != nil || len(msgs) == 0 {
		fallback, listErr := conn.List(0)
		if listErr != nil {
			return fmt.Errorf("pop3 list: %w", listErr)
		}
		msgs = fallback
	}
	if len(msgs) == 0 {
		return nil
	}

	for _, meta := range msgs {
		if ctx.Err() != nil {
			return ctx.Err()
		}

		payload, err := conn.RetrRaw(meta.ID)
		if err != nil {
			if isMissingPOP3Message(err) {
				if f.logger != nil {
					f.logger.Printf("pop3 retr skipped %d: %v", meta.ID, err)
				}
				continue
			}
			return fmt.Errorf("pop3 retr %d: %w", meta.ID, err)
		}

		uid := meta.UID
		if uid == "" {
			uid = strconv.Itoa(meta.ID)
		}
		raw := append([]byte(nil), payload.Bytes()...)
		msg := &FetchedMessage{
			Connector:  f.Name(),
			UID:        uid,
			RemoteID:   buildRemoteID(account, uid),
			ReceivedAt: f.now(),
			SizeBytes:  int64(len(raw)),
			Raw:        raw,
			Metadata: map[string]string{
				"uidl":    uid,
				"pop3_id": strconv.Itoa(meta.ID),
			},
		}
		if meta.Size > 0 {
			msg.Metadata["reported_size"] = strconv.Itoa(meta.Size)
		}
		msg.WithAccount(account)

		if err := handler.Handle(ctx, msg); err != nil {
			return fmt.Errorf("postmaster handler failed for %s: %w", uid, err)
		}
		if f.deleteAfterFetch {
			if err := conn.Dele(meta.ID); err != nil {
				return fmt.Errorf("pop3 delete %d: %w", meta.ID, err)
			}
		}
	}
	return nil
}

func (f *POP3Fetcher) safeQuit(conn pop3Connection) {
	if conn == nil {
		return
	}
	if err := conn.Quit(); err != nil && f.logger != nil {
		f.logger.Printf("pop3 quit error: %v", err)
	}
}

func (f *POP3Fetcher) defaultConnFactory(account Account) (pop3Connection, error) {
	if account.Host == "" {
		return nil, errors.New("pop3 account missing host")
	}
	port := account.Port
	if port == 0 {
		if usePOP3TLS(account.Type) {
			port = 995
		} else {
			port = 110
		}
	}
	client := pop3.New(pop3.Opt{
		Host:        account.Host,
		Port:        port,
		DialTimeout: f.dialTimeout,
		TLSEnabled:  usePOP3TLS(account.Type),
	})
	conn, err := client.NewConn()
	if err != nil {
		return nil, err
	}
	return &authNoopSkipper{Conn: conn}, nil
}

func validateAccount(account Account) error {
	if account.Username == "" {
		return errors.New("pop3 account missing username")
	}
	if len(account.Password) == 0 {
		return errors.New("pop3 account missing password")
	}
	if !supportsPOP3(account.Type) {
		return fmt.Errorf("account type %s not supported by POP3 connector", account.Type)
	}
	return nil
}

func supportsPOP3(t string) bool {
	switch strings.ToLower(t) {
	case "pop3", "pop3s", "pop3_tls", "pop3s_tls":
		return true
	default:
		return false
	}
}

func usePOP3TLS(t string) bool {
	switch strings.ToLower(t) {
	case "pop3s", "pop3_tls", "pop3s_tls":
		return true
	default:
		return false
	}
}

func isMissingPOP3Message(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "no such message") || strings.Contains(msg, "message not found")
}

func buildRemoteID(account Account, uid string) string {
	if account.Username == "" {
		return fmt.Sprintf("%s:%s", account.Host, uid)
	}
	return fmt.Sprintf("%s@%s:%s", account.Username, account.Host, uid)
}
