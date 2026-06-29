package connector

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net"
	"strings"
	"time"

	"github.com/emersion/go-imap/v2"
	"github.com/emersion/go-imap/v2/imapclient"
)

type imapClient interface {
	Login(username, password string) commandWaiter
	Logout() commandWaiter
	Close() error
	Select(mailbox string, options *imap.SelectOptions) selectWaiter
	UIDSearch(criteria *imap.SearchCriteria, options *imap.SearchOptions) searchWaiter
	Fetch(numSet imap.NumSet, options *imap.FetchOptions) fetchWaiter
	Store(numSet imap.NumSet, store *imap.StoreFlags, options *imap.StoreOptions) fetchWaiter
	UIDExpunge(uids imap.UIDSet) expungeWaiter
}

type commandWaiter interface{ Wait() error }
type selectWaiter interface {
	Wait() (*imap.SelectData, error)
}
type searchWaiter interface {
	Wait() (*imap.SearchData, error)
}
type fetchWaiter interface {
	Collect() ([]*imapclient.FetchMessageBuffer, error)
	Close() error
}
type expungeWaiter interface{ Close() error }

// IMAPFetcher streams IMAP/IMAPS mailboxes into the inbound pipeline.
type IMAPFetcher struct {
	deleteAfterFetch bool
	dialTimeout      time.Duration
	now              func() time.Time
	logger           *log.Logger
	newClient        func(Account) (imapClient, error)
}

// IMAPFetcherOption customizes fetcher behavior.
type IMAPFetcherOption func(*IMAPFetcher)

// NewIMAPFetcher returns an IMAP connector ready for dispatch polling.
func NewIMAPFetcher(opts ...IMAPFetcherOption) *IMAPFetcher {
	f := &IMAPFetcher{
		deleteAfterFetch: true,
		dialTimeout:      5 * time.Second,
		now:              func() time.Time { return time.Now().UTC() },
		logger:           log.Default(),
	}
	f.newClient = f.defaultClientFactory
	for _, opt := range opts {
		opt(f)
	}
	if f.newClient == nil {
		f.newClient = f.defaultClientFactory
	}
	return f
}

// WithIMAPDeleteAfterFetch toggles destructive IMAP behavior.
func WithIMAPDeleteAfterFetch(delete bool) IMAPFetcherOption {
	return func(f *IMAPFetcher) {
		f.deleteAfterFetch = delete
	}
}

// WithIMAPLogger overrides the logger used for connector diagnostics.
func WithIMAPLogger(logger *log.Logger) IMAPFetcherOption {
	return func(f *IMAPFetcher) {
		if logger != nil {
			f.logger = logger
		}
	}
}

// WithIMAPDialTimeout overrides the socket dial timeout.
func WithIMAPDialTimeout(timeout time.Duration) IMAPFetcherOption {
	return func(f *IMAPFetcher) {
		if timeout > 0 {
			f.dialTimeout = timeout
		}
	}
}

func withIMAPClientFactory(factory func(Account) (imapClient, error)) IMAPFetcherOption {
	return func(f *IMAPFetcher) {
		f.newClient = factory
	}
}

// WithIMAPClock overrides the wall clock, primarily for tests.
func WithIMAPClock(now func() time.Time) IMAPFetcherOption {
	return func(f *IMAPFetcher) {
		if now != nil {
			f.now = now
		}
	}
}

// Name returns the connector identifier.
func (f *IMAPFetcher) Name() string {
	return "imap"
}

// Fetch drains an IMAP mailbox and hands each message to the provided handler.
func (f *IMAPFetcher) Fetch(ctx context.Context, account Account, handler Handler) error {
	if handler == nil {
		return errors.New("imap fetcher requires a handler")
	}
	if err := validateIMAPAccount(account); err != nil {
		return err
	}

	client, err := f.newClient(account)
	if err != nil {
		return fmt.Errorf("imap connect: %w", err)
	}
	defer f.safeClose(client)

	if err := client.Login(account.Username, string(account.Password)).Wait(); err != nil {
		return fmt.Errorf("imap auth: %w", err)
	}

	mailbox := account.IMAPFolder
	if mailbox == "" {
		mailbox = "INBOX"
	}
	if _, err := client.Select(mailbox, nil).Wait(); err != nil {
		return fmt.Errorf("imap select %s: %w", mailbox, err)
	}

	searchData, err := client.UIDSearch(&imap.SearchCriteria{}, nil).Wait()
	if err != nil {
		return fmt.Errorf("imap search: %w", err)
	}
	uids := searchData.AllUIDs()
	if len(uids) == 0 {
		return nil
	}

	uidSet := imap.UIDSetNum(uids...)
	fetchOpts := &imap.FetchOptions{
		UID:          true,
		InternalDate: true,
		BodySection:  []*imap.FetchItemBodySection{{}},
	}
	fetchBuffers, err := client.Fetch(uidSet, fetchOpts).Collect()
	if err != nil {
		return fmt.Errorf("imap fetch: %w", err)
	}

	for _, buf := range fetchBuffers {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		body := buf.FindBodySection(&imap.FetchItemBodySection{})
		if body == nil {
			continue
		}
		received := buf.InternalDate
		if received.IsZero() {
			received = f.now()
		}
		uidStr := fmt.Sprintf("%d", buf.UID)
		msg := &FetchedMessage{
			Connector:  f.Name(),
			UID:        uidStr,
			RemoteID:   buildRemoteID(account, uidStr),
			ReceivedAt: received,
			SizeBytes:  int64(len(body)),
			Raw:        append([]byte(nil), body...),
			Metadata: map[string]string{
				"imap_uid":    uidStr,
				"imap_folder": mailbox,
			},
		}
		msg.WithAccount(account)
		if err := handler.Handle(ctx, msg); err != nil {
			return fmt.Errorf("postmaster handler failed for %s: %w", uidStr, err)
		}
	}

	if f.deleteAfterFetch {
		store := &imap.StoreFlags{Op: imap.StoreFlagsAdd, Flags: []imap.Flag{imap.FlagDeleted}}
		if err := client.Store(uidSet, store, nil).Close(); err != nil {
			return fmt.Errorf("imap store delete: %w", err)
		}
		if err := client.UIDExpunge(uidSet).Close(); err != nil {
			return fmt.Errorf("imap expunge: %w", err)
		}
	}

	if err := client.Logout().Wait(); err != nil {
		return fmt.Errorf("imap logout: %w", err)
	}

	return nil
}

func (f *IMAPFetcher) safeClose(client imapClient) {
	if client == nil {
		return
	}
	if err := client.Close(); err != nil && f.logger != nil {
		f.logger.Printf("imap close error: %v", err)
	}
}

func (f *IMAPFetcher) defaultClientFactory(account Account) (imapClient, error) {
	if account.Host == "" {
		return nil, errors.New("imap account missing host")
	}
	port := account.Port
	if port == 0 {
		if useIMAPTLS(account.Type) {
			port = 993
		} else {
			port = 143
		}
	}
	opts := &imapclient.Options{Dialer: &net.Dialer{Timeout: f.dialTimeout}}
	addr := fmt.Sprintf("%s:%d", account.Host, port)
	var client *imapclient.Client
	var err error
	if useIMAPTLS(account.Type) {
		client, err = imapclient.DialTLS(addr, opts)
	} else {
		client, err = imapclient.DialInsecure(addr, opts)
	}
	if err != nil {
		return nil, err
	}
	return &imapClientWrapper{Client: client}, nil
}

type imapClientWrapper struct{ *imapclient.Client }

func (w *imapClientWrapper) Login(username, password string) commandWaiter {
	return w.Client.Login(username, password)
}
func (w *imapClientWrapper) Logout() commandWaiter { return w.Client.Logout() }
func (w *imapClientWrapper) Select(mailbox string, options *imap.SelectOptions) selectWaiter {
	return w.Client.Select(mailbox, options)
}
func (w *imapClientWrapper) UIDSearch(criteria *imap.SearchCriteria, options *imap.SearchOptions) searchWaiter {
	return w.Client.UIDSearch(criteria, options)
}
func (w *imapClientWrapper) Fetch(numSet imap.NumSet, options *imap.FetchOptions) fetchWaiter {
	return w.Client.Fetch(numSet, options)
}
func (w *imapClientWrapper) Store(numSet imap.NumSet, store *imap.StoreFlags, options *imap.StoreOptions) fetchWaiter {
	return w.Client.Store(numSet, store, options)
}
func (w *imapClientWrapper) UIDExpunge(uids imap.UIDSet) expungeWaiter {
	return w.Client.UIDExpunge(uids)
}

func validateIMAPAccount(account Account) error {
	if account.Username == "" {
		return errors.New("imap account missing username")
	}
	if len(account.Password) == 0 {
		return errors.New("imap account missing password")
	}
	if !supportsIMAP(account.Type) {
		return fmt.Errorf("account type %s not supported by IMAP connector", account.Type)
	}
	return nil
}

func supportsIMAP(t string) bool {
	switch strings.ToLower(t) {
	case "imap", "imaps", "imap_tls", "imaps_tls", "imaptls":
		return true
	default:
		return false
	}
}

func useIMAPTLS(t string) bool {
	switch strings.ToLower(t) {
	case "imaps", "imap_tls", "imaps_tls", "imaptls":
		return true
	default:
		return false
	}
}
