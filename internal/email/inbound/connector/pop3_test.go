package connector

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"log"
	"testing"
	"time"

	"github.com/knadh/go-pop3"
	"github.com/stretchr/testify/require"
)

func TestPOP3FetcherFetchesMessages(t *testing.T) {
	conn := &fakePOP3Conn{
		uidl: []pop3.MessageID{
			{ID: 1, UID: "uid-1", Size: 123},
			{ID: 2, UID: "uid-2", Size: 456},
		},
		raw: map[int][]byte{
			1: []byte("first"),
			2: []byte("second"),
		},
	}
	now := time.Date(2024, 1, 2, 3, 4, 5, 0, time.UTC)
	h := &recordingHandler{}
	f := NewPOP3Fetcher(
		WithPOP3Clock(func() time.Time { return now }),
		withPOP3ConnFactory(func(Account) (pop3Connection, error) { return conn, nil }),
	)

	acc := Account{ID: 7, Type: "pop3s", Host: "mail.example", Port: 995, Username: "agent", Password: []byte("secret")}
	require.NoError(t, f.Fetch(context.Background(), acc, h))

	require.Equal(t, 2, len(h.messages))
	require.Equal(t, []int{1, 2}, conn.deleted)
	require.Equal(t, 1, conn.quitCalls)
	require.Equal(t, "uid-1", h.messages[0].UID)
	require.Equal(t, now, h.messages[0].ReceivedAt)
	require.Equal(t, []byte("first"), h.messages[0].Raw)
}

func TestPOP3FetcherStopsOnHandlerError(t *testing.T) {
	conn := &fakePOP3Conn{
		uidl: []pop3.MessageID{{ID: 1, UID: "uid-1"}, {ID: 2, UID: "uid-2"}},
		raw:  map[int][]byte{1: []byte("first"), 2: []byte("second")},
	}
	h := &recordingHandler{failUID: "uid-2"}
	f := NewPOP3Fetcher(
		WithPOP3Clock(func() time.Time { return time.Unix(0, 0) }),
		withPOP3ConnFactory(func(Account) (pop3Connection, error) { return conn, nil }),
	)

	acc := Account{ID: 7, Type: "pop3", Host: "mail.example", Username: "agent", Password: []byte("secret")}
	err := f.Fetch(context.Background(), acc, h)
	require.Error(t, err)
	require.Equal(t, []int{1}, conn.deleted)
	require.Equal(t, 1, len(h.messages))
}

func TestPOP3FetcherSkipsMissingMessages(t *testing.T) {
	conn := &fakePOP3Conn{
		uidl:    []pop3.MessageID{{ID: 1, UID: "uid-1"}, {ID: 2, UID: "uid-2"}},
		raw:     map[int][]byte{1: []byte("first"), 2: []byte("second")},
		retrErr: map[int]error{2: errors.New("-ERR No such message")},
	}
	h := &recordingHandler{}
	f := NewPOP3Fetcher(withPOP3ConnFactory(func(Account) (pop3Connection, error) { return conn, nil }))

	acc := Account{ID: 7, Type: "pop3", Host: "mail.example", Username: "agent", Password: []byte("secret")}
	require.NoError(t, f.Fetch(context.Background(), acc, h))
	require.Equal(t, []int{1}, conn.deleted)
	require.Len(t, h.messages, 1)
	require.Equal(t, "uid-1", h.messages[0].UID)
}

func TestPOP3FetcherFallsBackWhenUidlFails(t *testing.T) {
	conn := &fakePOP3Conn{
		uidl:    []pop3.MessageID{{ID: 1, UID: "uid-1"}},
		raw:     map[int][]byte{1: []byte("body")},
		uidlErr: errors.New("uidl broken"),
	}
	h := &recordingHandler{}
	f := NewPOP3Fetcher(withPOP3ConnFactory(func(Account) (pop3Connection, error) { return conn, nil }))

	acc := Account{ID: 7, Type: "pop3", Host: "mail.example", Username: "agent", Password: []byte("secret")}
	require.NoError(t, f.Fetch(context.Background(), acc, h))
	require.Len(t, h.messages, 1)
	require.Equal(t, []int{1}, conn.deleted)
}

func TestPOP3FetcherRetriesAfterRetrError(t *testing.T) {
	failing := true
	h := &recordingHandler{}
	var firstConn, secondConn *fakePOP3Conn
	factory := func(Account) (pop3Connection, error) {
		if failing {
			failing = false
			firstConn = &fakePOP3Conn{
				uidl:    []pop3.MessageID{{ID: 1, UID: "uid-1"}},
				raw:     map[int][]byte{1: []byte("body")},
				retrErr: map[int]error{1: errors.New("temporary retr failure")},
			}
			return firstConn, nil
		}
		secondConn = &fakePOP3Conn{
			uidl: []pop3.MessageID{{ID: 1, UID: "uid-1"}},
			raw:  map[int][]byte{1: []byte("body")},
		}
		return secondConn, nil
	}

	f := NewPOP3Fetcher(withPOP3ConnFactory(factory))
	acc := Account{ID: 7, Type: "pop3", Host: "mail.example", Username: "agent", Password: []byte("secret")}

	err := f.Fetch(context.Background(), acc, h)
	require.Error(t, err)
	require.Empty(t, h.messages)
	require.NotNil(t, firstConn)
	require.Empty(t, firstConn.deleted)

	require.NoError(t, f.Fetch(context.Background(), acc, h))
	require.Len(t, h.messages, 1)
	require.NotNil(t, secondConn)
	require.Equal(t, []int{1}, secondConn.deleted)
}

func TestPOP3FetcherRetriesAfterListError(t *testing.T) {
	failing := true
	h := &recordingHandler{}
	var firstConn, secondConn *fakePOP3Conn
	factory := func(Account) (pop3Connection, error) {
		if failing {
			failing = false
			firstConn = &fakePOP3Conn{
				uidlErr: errors.New("uidl down"),
				listErr: errors.New("temp list failure"),
			}
			return firstConn, nil
		}
		secondConn = &fakePOP3Conn{
			uidlErr: errors.New("uidl still down"),
			uidl:    []pop3.MessageID{{ID: 1, UID: "uid-1"}},
			raw:     map[int][]byte{1: []byte("body")},
		}
		return secondConn, nil
	}

	f := NewPOP3Fetcher(withPOP3ConnFactory(factory))
	acc := Account{ID: 7, Type: "pop3", Host: "mail.example", Username: "agent", Password: []byte("secret")}

	err := f.Fetch(context.Background(), acc, h)
	require.Error(t, err)
	require.Empty(t, h.messages)
	require.NotNil(t, firstConn)
	require.Empty(t, firstConn.deleted)

	require.NoError(t, f.Fetch(context.Background(), acc, h))
	require.Len(t, h.messages, 1)
	require.NotNil(t, secondConn)
	require.Equal(t, []int{1}, secondConn.deleted)
}

func TestPOP3FetcherReturnsAuthError(t *testing.T) {
	conn := &fakePOP3Conn{authErr: errors.New("bad creds")}
	f := NewPOP3Fetcher(withPOP3ConnFactory(func(Account) (pop3Connection, error) { return conn, nil }))
	h := &recordingHandler{}
	acc := Account{ID: 7, Type: "pop3", Host: "mail.example", Username: "agent", Password: []byte("secret")}
	err := f.Fetch(context.Background(), acc, h)
	require.ErrorContains(t, err, "pop3 auth")
	require.Empty(t, h.messages)
}

func TestPOP3FetcherDialTimeout(t *testing.T) {
	ctx := context.Background()
	f := NewPOP3Fetcher(WithPOP3DialTimeout(200 * time.Millisecond))
	acc := Account{ID: 7, Type: "pop3", Host: "10.255.255.1", Port: 65000, Username: "agent", Password: []byte("secret")}

	start := time.Now()
	err := f.Fetch(ctx, acc, &recordingHandler{})
	duration := time.Since(start)

	require.Error(t, err)
	require.Contains(t, err.Error(), "pop3 connect")
	require.Less(t, duration, 2*time.Second)
}

func TestPOP3FetcherValidatesAccount(t *testing.T) {
	cases := []Account{
		{Type: "pop3", Password: []byte("pw")},
		{Type: "pop3", Username: "user"},
		{Type: "imap", Username: "user", Password: []byte("pw")},
	}
	f := NewPOP3Fetcher()
	for _, acc := range cases {
		if err := f.Fetch(context.Background(), acc, &recordingHandler{}); err == nil {
			t.Fatalf("expected validation error for account %+v", acc)
		}
	}
}

func TestPOP3FetcherRequiresHandler(t *testing.T) {
	f := NewPOP3Fetcher()
	acc := Account{Type: "pop3", Username: "u", Password: []byte("p")}
	if err := f.Fetch(context.Background(), acc, nil); err == nil {
		t.Fatalf("expected handler required error")
	}
}

func TestPOP3FetcherEmptyMailboxNoError(t *testing.T) {
	conn := &fakePOP3Conn{}
	f := NewPOP3Fetcher(withPOP3ConnFactory(func(Account) (pop3Connection, error) { return conn, nil }))
	acc := Account{Type: "pop3", Username: "u", Password: []byte("p")}
	require.NoError(t, f.Fetch(context.Background(), acc, &recordingHandler{}))
}

func TestPOP3FetcherSkipsDeletionWhenDisabled(t *testing.T) {
	conn := &fakePOP3Conn{
		uidl: []pop3.MessageID{{ID: 1, UID: "uid-1"}},
		raw:  map[int][]byte{1: []byte("body")},
	}
	h := &recordingHandler{}
	f := NewPOP3Fetcher(
		WithPOP3DeleteAfterFetch(false),
		withPOP3ConnFactory(func(Account) (pop3Connection, error) { return conn, nil }),
	)
	acc := Account{Type: "pop3", Username: "u", Password: []byte("p")}
	require.NoError(t, f.Fetch(context.Background(), acc, h))
	require.Empty(t, conn.deleted)
}

func TestPOP3FetcherConnectErrorWrapped(t *testing.T) {
	f := NewPOP3Fetcher(withPOP3ConnFactory(func(Account) (pop3Connection, error) {
		return nil, errors.New("dial failed")
	}))
	acc := Account{Type: "pop3", Username: "u", Password: []byte("p")}
	err := f.Fetch(context.Background(), acc, &recordingHandler{})
	require.ErrorContains(t, err, "pop3 connect")
}

func TestPOP3FetcherSafeQuitLogs(t *testing.T) {
	var buf bytes.Buffer
	logger := log.New(&buf, "", 0)
	f := NewPOP3Fetcher(WithPOP3Logger(logger))
	f.safeQuit(&fakePOP3Conn{quitErr: errors.New("quit boom")})
	require.Contains(t, buf.String(), "quit boom")
}

func TestBuildRemoteID(t *testing.T) {
	require.Equal(t, "user@mail:uid", buildRemoteID(Account{Username: "user", Host: "mail"}, "uid"))
	require.Equal(t, "mail:uid", buildRemoteID(Account{Host: "mail"}, "uid"))
}

func TestSupportsAndTLSPreds(t *testing.T) {
	require.True(t, supportsPOP3("pop3_tls"))
	require.False(t, supportsPOP3("imap"))
	require.True(t, usePOP3TLS("pop3s"))
	require.False(t, usePOP3TLS("pop3"))
}

type recordingHandler struct {
	messages []*FetchedMessage
	failUID  string
}

func (h *recordingHandler) Handle(_ context.Context, msg *FetchedMessage) error {
	if h.failUID == msg.UID {
		return fmt.Errorf("fail %s", msg.UID)
	}
	h.messages = append(h.messages, msg)
	return nil
}

type fakePOP3Conn struct {
	uidl      []pop3.MessageID
	raw       map[int][]byte
	deleted   []int
	quitCalls int

	authErr error
	uidlErr error
	listErr error
	retrErr map[int]error
	deleErr error
	quitErr error
}

func (f *fakePOP3Conn) Auth(_, _ string) error {
	return f.authErr
}

func (f *fakePOP3Conn) Quit() error {
	f.quitCalls++
	return f.quitErr
}

func (f *fakePOP3Conn) Uidl(_ int) ([]pop3.MessageID, error) {
	if f.uidlErr != nil {
		return nil, f.uidlErr
	}
	out := make([]pop3.MessageID, len(f.uidl))
	copy(out, f.uidl)
	return out, nil
}

func (f *fakePOP3Conn) List(_ int) ([]pop3.MessageID, error) {
	if f.listErr != nil {
		return nil, f.listErr
	}
	out := make([]pop3.MessageID, len(f.uidl))
	copy(out, f.uidl)
	return out, nil
}

func (f *fakePOP3Conn) RetrRaw(id int) (*bytes.Buffer, error) {
	if err, ok := f.retrErr[id]; ok {
		return nil, err
	}
	payload, ok := f.raw[id]
	if !ok {
		return nil, fmt.Errorf("unknown message %d", id)
	}
	return bytes.NewBuffer(payload), nil
}

func (f *fakePOP3Conn) Dele(ids ...int) error {
	if f.deleErr != nil {
		return f.deleErr
	}
	f.deleted = append(f.deleted, ids...)
	return nil
}
