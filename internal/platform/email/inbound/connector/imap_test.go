package connector

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/emersion/go-imap/v2"
	"github.com/emersion/go-imap/v2/imapclient"
	"github.com/stretchr/testify/require"
)

func TestIMAPFetcherFetchesMessages(t *testing.T) {
	client := &fakeIMAPClient{
		uids: []imap.UID{11, 12},
		bodies: map[imap.UID][]byte{
			11: []byte("first"),
			12: []byte("second"),
		},
		internalDate: map[imap.UID]time.Time{
			11: time.Date(2024, 1, 2, 3, 4, 5, 0, time.UTC),
		},
	}
	now := time.Date(2025, 2, 3, 4, 5, 6, 0, time.UTC)
	h := &recordingHandler{}
	f := NewIMAPFetcher(
		WithIMAPClock(func() time.Time { return now }),
		withIMAPClientFactory(func(Account) (imapClient, error) { return client, nil }),
	)

	acc := Account{ID: 7, Type: "imaps", Host: "mail.example", Username: "agent", Password: []byte("secret"), IMAPFolder: "INBOX"}
	require.NoError(t, f.Fetch(context.Background(), acc, h))

	require.Equal(t, []imap.UID{11, 12}, client.storeUIDs)
	require.Equal(t, 1, client.expungeCalls)
	require.Equal(t, 1, client.logoutCalls)
	require.Equal(t, 2, len(h.messages))
	require.Equal(t, "11", h.messages[0].UID)
	require.Equal(t, time.Date(2024, 1, 2, 3, 4, 5, 0, time.UTC), h.messages[0].ReceivedAt)
	require.Equal(t, now, h.messages[1].ReceivedAt)
}

func TestIMAPFetcherStopsOnHandlerError(t *testing.T) {
	client := &fakeIMAPClient{
		uids:   []imap.UID{11, 12},
		bodies: map[imap.UID][]byte{11: []byte("first"), 12: []byte("second")},
	}
	h := &recordingHandler{failUID: "12"}
	f := NewIMAPFetcher(withIMAPClientFactory(func(Account) (imapClient, error) { return client, nil }))

	acc := Account{ID: 7, Type: "imap", Host: "mail.example", Username: "agent", Password: []byte("secret")}
	err := f.Fetch(context.Background(), acc, h)
	require.Error(t, err)
	require.Empty(t, client.storeUIDs)
	require.Len(t, h.messages, 1)
}

func TestIMAPFetcherEmptyMailboxNoError(t *testing.T) {
	client := &fakeIMAPClient{}
	f := NewIMAPFetcher(withIMAPClientFactory(func(Account) (imapClient, error) { return client, nil }))
	acc := Account{Type: "imap", Username: "u", Password: []byte("p")}
	require.NoError(t, f.Fetch(context.Background(), acc, &recordingHandler{}))
	require.Zero(t, client.storeCalls)
}

func TestIMAPFetcherValidation(t *testing.T) {
	cases := []Account{
		{Type: "imap", Password: []byte("pw")},
		{Type: "imap", Username: "user"},
		{Type: "pop3", Username: "user", Password: []byte("pw")},
	}
	f := NewIMAPFetcher()
	for _, acc := range cases {
		if err := f.Fetch(context.Background(), acc, &recordingHandler{}); err == nil {
			t.Fatalf("expected validation error for account %+v", acc)
		}
	}
}

func TestIMAPFetcherRequiresHandler(t *testing.T) {
	f := NewIMAPFetcher()
	acc := Account{Type: "imap", Username: "u", Password: []byte("p")}
	if err := f.Fetch(context.Background(), acc, nil); err == nil {
		t.Fatalf("expected handler required error")
	}
}

func TestIMAPFetcherSkipsDeletionWhenDisabled(t *testing.T) {
	client := &fakeIMAPClient{
		uids:   []imap.UID{11},
		bodies: map[imap.UID][]byte{11: []byte("body")},
	}
	h := &recordingHandler{}
	f := NewIMAPFetcher(
		WithIMAPDeleteAfterFetch(false),
		withIMAPClientFactory(func(Account) (imapClient, error) { return client, nil }),
	)
	acc := Account{Type: "imap", Username: "u", Password: []byte("p")}
	require.NoError(t, f.Fetch(context.Background(), acc, h))
	require.Zero(t, client.storeCalls)
	require.Zero(t, client.expungeCalls)
}

func TestIMAPFetcherAuthAndSelectErrors(t *testing.T) {
	f := NewIMAPFetcher(withIMAPClientFactory(func(Account) (imapClient, error) {
		return &fakeIMAPClient{loginErr: errors.New("bad creds")}, nil
	}))
	acc := Account{Type: "imap", Username: "u", Password: []byte("p")}
	err := f.Fetch(context.Background(), acc, &recordingHandler{})
	require.ErrorContains(t, err, "imap auth")

	f = NewIMAPFetcher(withIMAPClientFactory(func(Account) (imapClient, error) {
		return &fakeIMAPClient{selectErr: errors.New("no inbox")}, nil
	}))
	err = f.Fetch(context.Background(), acc, &recordingHandler{})
	require.ErrorContains(t, err, "imap select")
}

func TestIMAPFetcherConnectErrorWrapped(t *testing.T) {
	f := NewIMAPFetcher(withIMAPClientFactory(func(Account) (imapClient, error) {
		return nil, errors.New("dial failed")
	}))
	acc := Account{Type: "imap", Username: "u", Password: []byte("p")}
	err := f.Fetch(context.Background(), acc, &recordingHandler{})
	require.ErrorContains(t, err, "imap connect")
}

func TestSupportsIMAPPreds(t *testing.T) {
	require.True(t, supportsIMAP("imap_tls"))
	require.True(t, supportsIMAP("IMAPTLS"))
	require.False(t, supportsIMAP("pop3"))
	require.True(t, useIMAPTLS("imaps"))
	require.True(t, useIMAPTLS("IMAPTLS"))
	require.False(t, useIMAPTLS("imap"))
}

type fakeIMAPClient struct {
	uids         []imap.UID
	bodies       map[imap.UID][]byte
	internalDate map[imap.UID]time.Time

	loginErr   error
	selectErr  error
	searchErr  error
	fetchErr   error
	storeErr   error
	expungeErr error
	logoutErr  error

	storeUIDs    []imap.UID
	storeCalls   int
	expungeCalls int
	logoutCalls  int
	closed       bool
}

func (c *fakeIMAPClient) Login(_, _ string) commandWaiter { return &fakeCommand{err: c.loginErr} }
func (c *fakeIMAPClient) Logout() commandWaiter {
	c.logoutCalls++
	return &fakeCommand{err: c.logoutErr}
}
func (c *fakeIMAPClient) Close() error { c.closed = true; return nil }
func (c *fakeIMAPClient) Select(_ string, _ *imap.SelectOptions) selectWaiter {
	return &fakeSelect{err: c.selectErr}
}
func (c *fakeIMAPClient) UIDSearch(_ *imap.SearchCriteria, _ *imap.SearchOptions) searchWaiter {
	data := &imap.SearchData{All: imap.UIDSetNum(c.uids...)}
	return &fakeSearch{err: c.searchErr, data: data}
}
func (c *fakeIMAPClient) Fetch(_ imap.NumSet, _ *imap.FetchOptions) fetchWaiter {
	var bufs []*imapclient.FetchMessageBuffer
	if c.fetchErr == nil {
		for _, uid := range c.uids {
			bufs = append(bufs, &imapclient.FetchMessageBuffer{
				SeqNum:       uint32(uid),
				UID:          uid,
				InternalDate: c.internalDate[uid],
				BodySection: []imapclient.FetchBodySectionBuffer{{
					Section: &imap.FetchItemBodySection{},
					Bytes:   append([]byte(nil), c.bodies[uid]...),
				}},
			})
		}
	}
	return &fakeFetch{err: c.fetchErr, bufs: bufs}
}
func (c *fakeIMAPClient) Store(_ imap.NumSet, store *imap.StoreFlags, _ *imap.StoreOptions) fetchWaiter {
	c.storeCalls++
	if store != nil {
		c.storeUIDs = append(c.storeUIDs, c.uids...)
	}
	return &fakeFetch{err: c.storeErr}
}
func (c *fakeIMAPClient) UIDExpunge(_ imap.UIDSet) expungeWaiter {
	c.expungeCalls++
	return &fakeExpunge{err: c.expungeErr}
}

type fakeCommand struct{ err error }

func (c *fakeCommand) Wait() error { return c.err }

type fakeSelect struct{ err error }

func (s *fakeSelect) Wait() (*imap.SelectData, error) { return nil, s.err }

type fakeSearch struct {
	err  error
	data *imap.SearchData
}

func (s *fakeSearch) Wait() (*imap.SearchData, error) { return s.data, s.err }

type fakeFetch struct {
	err  error
	bufs []*imapclient.FetchMessageBuffer
}

func (f *fakeFetch) Collect() ([]*imapclient.FetchMessageBuffer, error) { return f.bufs, f.err }
func (f *fakeFetch) Close() error                                       { return f.err }

type fakeExpunge struct{ err error }

func (e *fakeExpunge) Close() error { return e.err }
