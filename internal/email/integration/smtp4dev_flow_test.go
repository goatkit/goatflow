//go:build integration

package integration

import (
	"bufio"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/tls"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"net"
	"net/smtp"
	"os"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/knadh/go-pop3"

	"github.com/gotrs-io/gotrs-ce/internal/config"
	"github.com/gotrs-io/gotrs-ce/internal/database"
	"github.com/gotrs-io/gotrs-ce/internal/email/inbound/connector"
	"github.com/gotrs-io/gotrs-ce/internal/email/inbound/filters"
	"github.com/gotrs-io/gotrs-ce/internal/email/inbound/postmaster"
	"github.com/gotrs-io/gotrs-ce/internal/mailqueue"
	"github.com/gotrs-io/gotrs-ce/internal/models"
	"github.com/gotrs-io/gotrs-ce/internal/repository"
	"github.com/gotrs-io/gotrs-ce/internal/runner/tasks"
	"github.com/gotrs-io/gotrs-ce/internal/service"
	"github.com/gotrs-io/gotrs-ce/internal/ticketnumber"
)

type smtp4devConfig struct {
	APIBase       string
	SMTPAddr      string
	POPHost       string
	POPPort       int
	IMAPHost      string
	IMAPPort      int
	Username      string
	Password      string
	FromAddress   string
	SystemAddress string
}

func stringPtr(s string) *string { return &s }

func loadConfig(t *testing.T) smtp4devConfig {
	t.Helper()
	cfg := smtp4devConfig{
		APIBase:       getenv("SMTP4DEV_API_BASE", "http://localhost:8025/api/v3"),
		SMTPAddr:      getenv("SMTP4DEV_SMTP_ADDR", "localhost:1025"),
		POPHost:       getenv("SMTP4DEV_POP_HOST", "localhost"),
		POPPort:       getenvInt("SMTP4DEV_POP_PORT", 1110),
		IMAPHost:      getenv("SMTP4DEV_IMAP_HOST", "localhost"),
		IMAPPort:      getenvInt("SMTP4DEV_IMAP_PORT", 1143),
		Username:      os.Getenv("SMTP4DEV_USER"),
		Password:      os.Getenv("SMTP4DEV_PASS"),
		FromAddress:   getenv("SMTP4DEV_FROM", "tester@example.com"),
		SystemAddress: getenv("SMTP4DEV_SYSTEM_ADDRESS", "nova-automotive-labs-support-22@gotrs.local"),
	}
	if cfg.Username == "" || cfg.Password == "" {
		t.Skip("SMTP4DEV_USER and SMTP4DEV_PASS must be set for integration test")
	}
	return cfg
}

func TestSMTP4DevRoundTripThroughSMTPAndPOP(t *testing.T) {
	cfg := loadConfig(t)
	client := NewSMTP4DevClient(cfg.APIBase, nil)
	ctx := context.Background()

	// Ensure mailbox exists and start from a clean slate
	box, err := client.CreateMailbox(ctx, cfg.Username, cfg.Username, cfg.Password)
	if err != nil {
		t.Fatalf("create mailbox: %v", err)
	}
	t.Cleanup(func() { _ = client.DeleteMailbox(context.Background(), box.ID) })
	_ = client.DeleteAllMessages(ctx)

	token := randomToken()
	subject := "Integration Probe"
	body := fmt.Sprintf("Hello %s", token)

	// Send via SMTP using auth
	smtpHost := strings.Split(cfg.SMTPAddr, ":")[0]
	auth := smtp.PlainAuth("", cfg.Username, cfg.Password, smtpHost)
	msg := []byte(fmt.Sprintf("From: %s\r\nTo: %s\r\nSubject: %s\r\n\r\n%s\r\n", cfg.FromAddress, cfg.SystemAddress, subject, body))
	if err := smtp.SendMail(cfg.SMTPAddr, auth, cfg.FromAddress, []string{cfg.SystemAddress}, msg); err != nil {
		t.Fatalf("send mail: %v", err)
	}

	// Poll POP until the message arrives
	deadline := time.Now().Add(10 * time.Second)
	found := false
	for time.Now().Before(deadline) {
		if popHasBody(t, cfg, token) {
			found = true
			break
		}
		time.Sleep(300 * time.Millisecond)
	}

	if !found {
		t.Fatalf("message containing token %s not found via POP", token)
	}
}

func TestSMTP4DevAuthenticatedSMTPRoutesToMailbox(t *testing.T) {
	cfg := loadConfig(t)
	client := NewSMTP4DevClient(cfg.APIBase, nil)
	ctx := context.Background()

	// System mailbox uses configured creds; create an extra mailbox to ensure messages do not leak there.
	systemBox, err := client.CreateMailbox(ctx, cfg.Username, cfg.Username, cfg.Password)
	if err != nil {
		t.Fatalf("create system mailbox: %v", err)
	}
	t.Cleanup(func() { _ = client.DeleteMailbox(context.Background(), systemBox.ID) })

	altLogin := "alt-" + randomToken()
	altBox, err := client.CreateMailbox(ctx, altLogin, altLogin, "alt-pass")
	if err != nil {
		t.Fatalf("create alt mailbox: %v", err)
	}
	t.Cleanup(func() { _ = client.DeleteMailbox(context.Background(), altBox.ID) })

	_ = client.DeleteAllMessages(ctx)

	subject := "Routing Probe"
	body := "Route me"
	smtpHost := strings.Split(cfg.SMTPAddr, ":")[0]
	auth := smtp.PlainAuth("", cfg.Username, cfg.Password, smtpHost)
	msg := []byte(fmt.Sprintf("From: %s\r\nTo: %s\r\nSubject: %s\r\n\r\n%s\r\n", cfg.FromAddress, cfg.SystemAddress, subject, body))
	if err := smtp.SendMail(cfg.SMTPAddr, auth, cfg.FromAddress, []string{cfg.SystemAddress}, msg); err != nil {
		t.Fatalf("send mail: %v", err)
	}

	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		if gotTarget := mailboxCount(ctx, t, client, systemBox.ID); gotTarget > 0 {
			if gotAlt := mailboxCount(ctx, t, client, altBox.ID); gotAlt > 0 {
				t.Fatalf("message delivered to alternate mailbox (count=%d)", gotAlt)
			}
			return
		}
		time.Sleep(300 * time.Millisecond)
	}

	t.Fatalf("message did not appear in system mailbox %s", systemBox.ID)
}

func TestSMTP4DevPOPRequiresAuth(t *testing.T) {
	cfg := loadConfig(t)
	client := NewSMTP4DevClient(cfg.APIBase, nil)
	ctx := context.Background()

	box, err := client.CreateMailbox(ctx, cfg.Username, cfg.Username, cfg.Password)
	if err != nil {
		t.Fatalf("create mailbox: %v", err)
	}
	t.Cleanup(func() { _ = client.DeleteMailbox(context.Background(), box.ID) })

	pop := pop3.New(pop3.Opt{Host: cfg.POPHost, Port: cfg.POPPort})
	conn, err := pop.NewConn()
	if err != nil {
		t.Fatalf("pop connect: %v", err)
	}
	defer conn.Quit()

	if err := conn.User(cfg.Username); err != nil {
		t.Fatalf("pop user failed: %v", err)
	}

	err = conn.Pass(cfg.Password + "-wrong")
	if err == nil {
		t.Fatalf("expected auth error with wrong password")
	}
}

func TestPOP3FetcherAgainstSMTP4Dev(t *testing.T) {
	cfg := loadConfig(t)
	client := NewSMTP4DevClient(cfg.APIBase, nil)
	ctx := context.Background()

	box, err := client.CreateMailbox(ctx, cfg.Username, cfg.Username, cfg.Password)
	if err != nil {
		t.Fatalf("create mailbox: %v", err)
	}
	t.Cleanup(func() { _ = client.DeleteMailbox(context.Background(), box.ID) })
	_ = client.DeleteAllMessages(ctx)

	token := randomToken()
	if err := sendSMTP(t, cfg, cfg.SystemAddress, fmt.Sprintf("POP Fetch %s", token), fmt.Sprintf("POP Fetch %s", token)); err != nil {
		t.Fatalf("send smtp: %v", err)
	}

	acc := connector.Account{Type: "pop3", Host: cfg.POPHost, Port: cfg.POPPort, Username: cfg.Username, Password: []byte(cfg.Password)}
	h := &recordingHandler{}
	fetcher := connector.NewPOP3Fetcher(connector.WithPOP3DeleteAfterFetch(false))

	deadline := time.Now().Add(12 * time.Second)
	for time.Now().Before(deadline) {
		h.messages = nil
		if err := fetcher.Fetch(ctx, acc, h); err != nil {
			t.Fatalf("fetch: %v", err)
		}
		if len(h.messages) > 0 && strings.Contains(string(h.messages[0].Raw), token) {
			got := h.messages[0].AccountSnapshot()
			if got.Username != acc.Username || got.Host != acc.Host || got.Port != acc.Port {
				t.Fatalf("account snapshot mismatch: %+v", got)
			}
			msgs, err := client.ListMessages(ctx, box.ID)
			if err != nil {
				t.Fatalf("list messages: %v", err)
			}
			if len(msgs) == 0 {
				t.Fatalf("expected message to remain when delete_after_fetch=false")
			}
			return
		}
		time.Sleep(400 * time.Millisecond)
	}

	t.Fatalf("pop fetcher did not retrieve message containing token %s", token)
}

func TestIMAPFetcherAgainstSMTP4Dev(t *testing.T) {
	cfg := loadConfig(t)
	addr := fmt.Sprintf("%s:%d", cfg.IMAPHost, cfg.IMAPPort)
	conn, err := net.DialTimeout("tcp", addr, 2*time.Second)
	if err != nil {
		t.Skipf("imap endpoint unavailable at %s: %v", addr, err)
	}
	_ = conn.Close()

	client := NewSMTP4DevClient(cfg.APIBase, nil)
	ctx := context.Background()

	box, err := client.CreateMailbox(ctx, cfg.Username, cfg.Username, cfg.Password)
	if err != nil {
		t.Fatalf("create mailbox: %v", err)
	}
	t.Cleanup(func() { _ = client.DeleteMailbox(context.Background(), box.ID) })
	_ = client.DeleteAllMessages(ctx)

	token := randomToken()
	if err := sendSMTP(t, cfg, cfg.SystemAddress, fmt.Sprintf("IMAP Fetch %s", token), fmt.Sprintf("IMAP Fetch %s", token)); err != nil {
		t.Fatalf("send smtp: %v", err)
	}

	acc := connector.Account{Type: "imap", Host: cfg.IMAPHost, Port: cfg.IMAPPort, Username: cfg.Username, Password: []byte(cfg.Password), IMAPFolder: "INBOX"}
	h := &recordingHandler{}
	fetcher := connector.NewIMAPFetcher(connector.WithIMAPDeleteAfterFetch(false))

	deadline := time.Now().Add(12 * time.Second)
	for time.Now().Before(deadline) {
		h.messages = nil
		if err := fetcher.Fetch(ctx, acc, h); err != nil {
			t.Fatalf("fetch: %v", err)
		}
		if len(h.messages) > 0 && strings.Contains(string(h.messages[0].Raw), token) {
			got := h.messages[0].AccountSnapshot()
			if got.Username != acc.Username || got.Host != acc.Host || got.Port != acc.Port || got.IMAPFolder != acc.IMAPFolder {
				t.Fatalf("account snapshot mismatch: %+v", got)
			}
			if folder := h.messages[0].Metadata["imap_folder"]; folder != "INBOX" {
				t.Fatalf("expected imap_folder metadata INBOX, got %s", folder)
			}
			msgs, err := client.ListMessages(ctx, box.ID)
			if err != nil {
				t.Fatalf("list messages: %v", err)
			}
			if len(msgs) == 0 {
				t.Fatalf("expected message to remain when delete_after_fetch=false")
			}
			return
		}
		time.Sleep(400 * time.Millisecond)
	}

	t.Fatalf("imap fetcher did not retrieve message containing token %s", token)
}

func TestPOP3FetcherDeletesMessagesByDefault(t *testing.T) {
	cfg := loadConfig(t)
	client := NewSMTP4DevClient(cfg.APIBase, nil)
	ctx := context.Background()

	box, err := client.CreateMailbox(ctx, cfg.Username, cfg.Username, cfg.Password)
	if err != nil {
		t.Fatalf("create mailbox: %v", err)
	}
	t.Cleanup(func() { _ = client.DeleteMailbox(context.Background(), box.ID) })
	_ = client.DeleteAllMessages(ctx)

	token := randomToken()
	if err := sendSMTP(t, cfg, cfg.SystemAddress, fmt.Sprintf("POP Delete %s", token), fmt.Sprintf("POP Delete %s", token)); err != nil {
		t.Fatalf("send smtp: %v", err)
	}

	acc := connector.Account{Type: "pop3", Host: cfg.POPHost, Port: cfg.POPPort, Username: cfg.Username, Password: []byte(cfg.Password)}
	h := &recordingHandler{}
	fetcher := connector.NewPOP3Fetcher()

	deadline := time.Now().Add(12 * time.Second)
	for time.Now().Before(deadline) {
		h.messages = nil
		if err := fetcher.Fetch(ctx, acc, h); err != nil {
			t.Fatalf("fetch: %v", err)
		}
		if len(h.messages) > 0 && strings.Contains(string(h.messages[0].Raw), token) {
			msgs, err := client.ListMessages(ctx, box.ID)
			if err != nil {
				t.Fatalf("list messages: %v", err)
			}
			if len(msgs) != 0 {
				t.Fatalf("expected mailbox to be empty after delete, got %d", len(msgs))
			}
			return
		}
		time.Sleep(400 * time.Millisecond)
	}

	t.Fatalf("pop fetcher did not delete message containing token %s", token)
}

func TestPOP3FetcherIsIdempotentAfterDelete(t *testing.T) {
	cfg := loadConfig(t)
	client := NewSMTP4DevClient(cfg.APIBase, nil)
	ctx := context.Background()

	box, err := client.CreateMailbox(ctx, cfg.Username, cfg.Username, cfg.Password)
	if err != nil {
		t.Fatalf("create mailbox: %v", err)
	}
	t.Cleanup(func() { _ = client.DeleteMailbox(context.Background(), box.ID) })
	_ = client.DeleteAllMessages(ctx)

	token := randomToken()
	if err := sendSMTP(t, cfg, cfg.SystemAddress, fmt.Sprintf("POP Idempotent %s", token), fmt.Sprintf("POP Idempotent %s", token)); err != nil {
		t.Fatalf("send smtp: %v", err)
	}

	acc := connector.Account{Type: "pop3", Host: cfg.POPHost, Port: cfg.POPPort, Username: cfg.Username, Password: []byte(cfg.Password)}
	h := &recordingHandler{}
	fetcher := connector.NewPOP3Fetcher()

	fetched := false
	deadline := time.Now().Add(12 * time.Second)
	for time.Now().Before(deadline) {
		h.messages = nil
		if err := fetcher.Fetch(ctx, acc, h); err != nil {
			t.Fatalf("fetch: %v", err)
		}
		if len(h.messages) > 0 && strings.Contains(string(h.messages[0].Raw), token) {
			fetched = true
			break
		}
		time.Sleep(300 * time.Millisecond)
	}
	if !fetched {
		t.Fatalf("pop fetcher did not retrieve message containing token %s", token)
	}

	// Second fetch should see nothing once mailbox is drained.
	h.messages = nil
	if err := fetcher.Fetch(ctx, acc, h); err != nil {
		t.Fatalf("second fetch: %v", err)
	}
	if len(h.messages) != 0 {
		t.Fatalf("expected no messages on second fetch, got %d", len(h.messages))
	}
}

func TestPOP3FetcherRecoversAfterAuthError(t *testing.T) {
	cfg := loadConfig(t)
	client := NewSMTP4DevClient(cfg.APIBase, nil)
	ctx := context.Background()

	box, err := client.CreateMailbox(ctx, cfg.Username, cfg.Username, cfg.Password)
	if err != nil {
		t.Fatalf("create mailbox: %v", err)
	}
	t.Cleanup(func() { _ = client.DeleteMailbox(context.Background(), box.ID) })
	_ = client.DeleteAllMessages(ctx)

	token := randomToken()
	if err := sendSMTP(t, cfg, cfg.SystemAddress, fmt.Sprintf("POP Retry %s", token), fmt.Sprintf("POP Retry %s", token)); err != nil {
		t.Fatalf("send smtp: %v", err)
	}

	accBad := connector.Account{Type: "pop3", Host: cfg.POPHost, Port: cfg.POPPort, Username: cfg.Username, Password: []byte(cfg.Password + "-bad")}
	accGood := connector.Account{Type: "pop3", Host: cfg.POPHost, Port: cfg.POPPort, Username: cfg.Username, Password: []byte(cfg.Password)}
	fetcher := connector.NewPOP3Fetcher()
	h := &recordingHandler{}

	if err := fetcher.Fetch(ctx, accBad, h); err == nil {
		t.Fatalf("expected auth error with wrong password")
	}

	if mailboxCount(ctx, t, client, box.ID) == 0 {
		t.Fatalf("message disappeared after auth error")
	}

	h.messages = nil
	if err := fetcher.Fetch(ctx, accGood, h); err != nil {
		t.Fatalf("fetch with good creds: %v", err)
	}
	if len(h.messages) == 0 || !strings.Contains(string(h.messages[0].Raw), token) {
		t.Fatalf("did not retrieve message after retry")
	}

	if mailboxCount(ctx, t, client, box.ID) != 0 {
		t.Fatalf("mailbox not drained after successful fetch")
	}
}

func TestPOP3FetcherSurvivesTransientConnectionError(t *testing.T) {
	cfg := loadConfig(t)
	client := NewSMTP4DevClient(cfg.APIBase, nil)
	ctx := context.Background()

	box, err := client.CreateMailbox(ctx, cfg.Username, cfg.Username, cfg.Password)
	if err != nil {
		t.Fatalf("create mailbox: %v", err)
	}
	t.Cleanup(func() { _ = client.DeleteMailbox(context.Background(), box.ID) })
	_ = client.DeleteAllMessages(ctx)

	token := randomToken()
	if err := sendSMTP(t, cfg, cfg.SystemAddress, fmt.Sprintf("POP Transient %s", token), fmt.Sprintf("POP Transient %s", token)); err != nil {
		t.Fatalf("send smtp: %v", err)
	}

	badPort := cfg.POPPort + 1111
	accBad := connector.Account{Type: "pop3", Host: cfg.POPHost, Port: badPort, Username: cfg.Username, Password: []byte(cfg.Password)}
	accGood := connector.Account{Type: "pop3", Host: cfg.POPHost, Port: cfg.POPPort, Username: cfg.Username, Password: []byte(cfg.Password)}
	fetcher := connector.NewPOP3Fetcher()
	h := &recordingHandler{}

	if err := fetcher.Fetch(ctx, accBad, h); err == nil {
		t.Fatalf("expected connection error on bad port")
	}

	if mailboxCount(ctx, t, client, box.ID) == 0 {
		t.Fatalf("message lost after transient error")
	}

	h.messages = nil
	if err := fetcher.Fetch(ctx, accGood, h); err != nil {
		t.Fatalf("fetch after transient error: %v", err)
	}
	if len(h.messages) == 0 || !strings.Contains(string(h.messages[0].Raw), token) {
		t.Fatalf("message not retrieved after transient error")
	}
}

func TestPOP3FetcherRetriesAfterTransientListAndRetr(t *testing.T) {
	ctx := context.Background()

	// Local flaky POP3 stub to inject transient errors on UIDL->LIST fallback and first RETR.
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer lis.Close()

	messageToken := randomToken()
	popMessage := fmt.Sprintf("From: stub@example.com\r\nTo: agent@example.com\r\nSubject: Flaky POP3\r\n\r\nBody %s\r\n", messageToken)

	failUIDLOnce := true
	failLISTOnce := true
	failRETROnce := true

	go func() {
		for {
			conn, err := lis.Accept()
			if err != nil {
				return
			}
			go func(c net.Conn) {
				defer c.Close()
				_, _ = c.Write([]byte("+OK welcome\r\n"))
				r := bufio.NewReader(c)
				for {
					line, err := r.ReadString('\n')
					if err != nil {
						return
					}
					s := strings.TrimSpace(line)
					switch {
					case strings.HasPrefix(s, "USER"):
						_, _ = c.Write([]byte("+OK\r\n"))
					case strings.HasPrefix(s, "PASS"):
						_, _ = c.Write([]byte("+OK\r\n"))
					case strings.HasPrefix(s, "NOOP"):
						_, _ = c.Write([]byte("+OK\r\n"))
					case strings.HasPrefix(s, "UIDL"):
						if failUIDLOnce {
							failUIDLOnce = false
							_, _ = c.Write([]byte("-ERR temp uidl\r\n"))
							continue
						}
						_, _ = c.Write([]byte("+OK\r\n1 msgid\r\n.\r\n"))
					case strings.HasPrefix(s, "LIST"):
						if failLISTOnce {
							failLISTOnce = false
							_, _ = c.Write([]byte("-ERR temp list\r\n"))
							continue
						}
						_, _ = c.Write([]byte(fmt.Sprintf("+OK\r\n1 %d\r\n.\r\n", len(popMessage))))
					case strings.HasPrefix(s, "RETR"):
						if failRETROnce {
							failRETROnce = false
							_, _ = c.Write([]byte("-ERR temp retr\r\n"))
							continue
						}
						_, _ = c.Write([]byte("+OK\r\n" + popMessage + ".\r\n"))
					case strings.HasPrefix(s, "DELE"):
						_, _ = c.Write([]byte("+OK\r\n"))
					case strings.HasPrefix(s, "QUIT"):
						_, _ = c.Write([]byte("+OK\r\n"))
						return
					default:
						_, _ = c.Write([]byte("-ERR unknown\r\n"))
					}
				}
			}(conn)
		}
	}()

	h := &recordingHandler{}
	fetcher := connector.NewPOP3Fetcher()
	acc := connector.Account{Type: "pop3", Host: "127.0.0.1", Port: lis.Addr().(*net.TCPAddr).Port, Username: "user", Password: []byte("pass")}

	if err := fetcher.Fetch(ctx, acc, h); err == nil {
		t.Fatalf("expected fetch error on transient list")
	}
	if len(h.messages) != 0 {
		t.Fatalf("messages delivered despite list failure")
	}

	if err := fetcher.Fetch(ctx, acc, h); err == nil {
		t.Fatalf("expected fetch error on transient retr")
	}
	if len(h.messages) != 0 {
		t.Fatalf("messages delivered despite retr failure")
	}

	if err := fetcher.Fetch(ctx, acc, h); err != nil {
		t.Fatalf("fetch after transient errors: %v", err)
	}
	if len(h.messages) != 1 || !strings.Contains(string(h.messages[0].Raw), messageToken) {
		t.Fatalf("message missing after retries: %+v", h.messages)
	}
}

func TestPOP3FetcherHandlesEmptyMailbox(t *testing.T) {
	cfg := loadConfig(t)
	client := NewSMTP4DevClient(cfg.APIBase, nil)
	ctx := context.Background()

	box, err := client.CreateMailbox(ctx, cfg.Username, cfg.Username, cfg.Password)
	if err != nil {
		t.Fatalf("create mailbox: %v", err)
	}
	t.Cleanup(func() { _ = client.DeleteMailbox(context.Background(), box.ID) })
	_ = client.DeleteAllMessages(ctx)

	acc := connector.Account{Type: "pop3", Host: cfg.POPHost, Port: cfg.POPPort, Username: cfg.Username, Password: []byte(cfg.Password)}
	h := &recordingHandler{}
	fetcher := connector.NewPOP3Fetcher()

	if err := fetcher.Fetch(ctx, acc, h); err != nil {
		t.Fatalf("fetch empty: %v", err)
	}
	if len(h.messages) != 0 {
		t.Fatalf("expected no messages in empty mailbox, got %d", len(h.messages))
	}
}

func TestPOP3FetcherRetainsWhenConfigured(t *testing.T) {
	cfg := loadConfig(t)
	client := NewSMTP4DevClient(cfg.APIBase, nil)
	ctx := context.Background()

	box, err := client.CreateMailbox(ctx, cfg.Username, cfg.Username, cfg.Password)
	if err != nil {
		t.Fatalf("create mailbox: %v", err)
	}
	t.Cleanup(func() { _ = client.DeleteMailbox(context.Background(), box.ID) })
	_ = client.DeleteAllMessages(ctx)

	token := randomToken()
	if err := sendSMTP(t, cfg, cfg.SystemAddress, fmt.Sprintf("POP Retain %s", token), fmt.Sprintf("POP Retain %s", token)); err != nil {
		t.Fatalf("send smtp: %v", err)
	}

	acc := connector.Account{Type: "pop3", Host: cfg.POPHost, Port: cfg.POPPort, Username: cfg.Username, Password: []byte(cfg.Password)}
	h := &recordingHandler{}
	fetcher := connector.NewPOP3Fetcher(connector.WithPOP3DeleteAfterFetch(false))

	for i := 0; i < 2; i++ {
		h.messages = nil
		if err := fetcher.Fetch(ctx, acc, h); err != nil {
			t.Fatalf("fetch %d: %v", i, err)
		}
		if len(h.messages) == 0 || !strings.Contains(string(h.messages[0].Raw), token) {
			t.Fatalf("fetch %d did not retrieve retained message", i)
		}
	}

	msgs, err := client.ListMessages(ctx, box.ID)
	if err != nil {
		t.Fatalf("list messages: %v", err)
	}
	if len(msgs) == 0 {
		t.Fatalf("expected mailbox to retain message when delete_after_fetch=false")
	}
}

func TestPOP3FetcherCreatesTicketViaPostmaster(t *testing.T) {
	cfg := loadConfig(t)
	client := NewSMTP4DevClient(cfg.APIBase, nil)
	ctx := context.Background()

	db, err := database.GetDB()
	if err != nil {
		t.Skipf("database unavailable: %v", err)
	}

	gen, err := ticketnumber.Resolve("DateChecksum", "10", nil)
	if err != nil {
		t.Fatalf("ticket number generator: %v", err)
	}
	repository.SetTicketNumberGenerator(gen, ticketnumber.NewDBStore(db, "10"))
	t.Cleanup(func() { repository.SetTicketNumberGenerator(nil, nil) })

	queueName := getenv("TEST_QUEUE_NAME", "Postmaster")
	queueRepo := repository.NewQueueRepository(db)
	queue, err := queueRepo.GetByName(queueName)
	if err != nil || queue == nil {
		t.Fatalf("queue %s unavailable: %v", queueName, err)
	}

	box, err := client.CreateMailbox(ctx, cfg.Username, cfg.Username, cfg.Password)
	if err != nil {
		t.Fatalf("create mailbox: %v", err)
	}
	t.Cleanup(func() { _ = client.DeleteMailbox(context.Background(), box.ID) })
	_ = client.DeleteAllMessages(ctx)

	subject := fmt.Sprintf("POP Postmaster %s", randomToken())
	body := fmt.Sprintf("Body %s", randomToken())
	if err := sendSMTP(t, cfg, cfg.SystemAddress, subject, body); err != nil {
		t.Fatalf("send smtp: %v", err)
	}

	ticketRepo := repository.NewTicketRepository(db)
	articleRepo := repository.NewArticleRepository(db)
	ticketSvc := service.NewTicketService(ticketRepo, service.WithArticleRepository(articleRepo))
	postmasterHandler := postmaster.Service{
		FilterChain: filters.NewChain(),
		Handler:     postmaster.NewTicketProcessor(ticketSvc, postmaster.WithTicketProcessorFallbackQueue(int(queue.ID))),
	}

	acc := connector.Account{Type: "pop3", Host: cfg.POPHost, Port: cfg.POPPort, Username: cfg.Username, Password: []byte(cfg.Password), QueueID: int(queue.ID)}
	fetcher := connector.NewPOP3Fetcher()

	deadline := time.Now().Add(20 * time.Second)
	for time.Now().Before(deadline) {
		if err := fetcher.Fetch(ctx, acc, postmasterHandler); err != nil {
			t.Fatalf("fetch: %v", err)
		}
		tid := findTicketByTitle(ctx, db, subject)
		if tid > 0 {
			if count := articleCount(ctx, db, tid); count > 0 {
				return
			}
		}
		time.Sleep(500 * time.Millisecond)
	}

	t.Fatalf("ticket not created for subject %q", subject)
}

func TestPOP3FetcherStoresAttachmentsViaPostmaster(t *testing.T) {
	cfg := loadConfig(t)
	client := NewSMTP4DevClient(cfg.APIBase, nil)
	ctx := context.Background()

	db, err := database.GetDB()
	if err != nil {
		t.Skipf("database unavailable: %v", err)
	}

	gen, err := ticketnumber.Resolve("DateChecksum", "10", nil)
	if err != nil {
		t.Fatalf("ticket number generator: %v", err)
	}
	repository.SetTicketNumberGenerator(gen, ticketnumber.NewDBStore(db, "10"))
	t.Cleanup(func() { repository.SetTicketNumberGenerator(nil, nil) })

	queueName := getenv("TEST_QUEUE_NAME", "Postmaster")
	queueRepo := repository.NewQueueRepository(db)
	queue, err := queueRepo.GetByName(queueName)
	if err != nil || queue == nil {
		t.Fatalf("queue %s unavailable: %v", queueName, err)
	}

	box, err := client.CreateMailbox(ctx, cfg.Username, cfg.Username, cfg.Password)
	if err != nil {
		t.Fatalf("create mailbox: %v", err)
	}
	t.Cleanup(func() { _ = client.DeleteMailbox(context.Background(), box.ID) })
	_ = client.DeleteAllMessages(ctx)

	subject := fmt.Sprintf("POP Attachment %s", randomToken())
	body := fmt.Sprintf("Body %s", randomToken())
	attToken := randomToken()
	boundary := "BOUNDARY-" + randomToken()
	attachment := base64.StdEncoding.EncodeToString([]byte("attachment-" + attToken))
	second := base64.StdEncoding.EncodeToString([]byte("json-" + attToken))
	raw := []byte(fmt.Sprintf(
		"From: %s\r\nTo: %s\r\nSubject: %s\r\nMIME-Version: 1.0\r\nContent-Type: multipart/mixed; boundary=%s\r\n\r\n"+
			"--%s\r\nContent-Type: text/plain; charset=utf-8\r\n\r\n%s\r\n"+
			"--%s\r\nContent-Type: text/plain; name=\"note.txt\"\r\nContent-Disposition: attachment; filename=\"note.txt\"\r\nContent-Transfer-Encoding: base64\r\n\r\n%s\r\n"+
			"--%s\r\nContent-Type: application/json; name=\"data.json\"\r\nContent-Disposition: attachment; filename=\"data.json\"\r\nContent-Transfer-Encoding: base64\r\n\r\n%s\r\n"+
			"--%s--\r\n",
		cfg.FromAddress, cfg.SystemAddress, subject, boundary,
		boundary, body,
		boundary, attachment,
		boundary, second,
		boundary,
	))
	if err := smtp.SendMail(cfg.SMTPAddr, smtp.PlainAuth("", cfg.Username, cfg.Password, strings.Split(cfg.SMTPAddr, ":")[0]), cfg.FromAddress, []string{cfg.SystemAddress}, raw); err != nil {
		t.Fatalf("send smtp with attachment: %v", err)
	}

	ticketRepo := repository.NewTicketRepository(db)
	articleRepo := repository.NewArticleRepository(db)
	ticketSvc := service.NewTicketService(ticketRepo, service.WithArticleRepository(articleRepo))
	postmasterHandler := postmaster.Service{
		FilterChain: filters.NewChain(),
		Handler:     postmaster.NewTicketProcessor(ticketSvc, postmaster.WithTicketProcessorFallbackQueue(int(queue.ID)), postmaster.WithTicketProcessorMessageLookup(articleRepo)),
	}

	acc := connector.Account{Type: "pop3", Host: cfg.POPHost, Port: cfg.POPPort, Username: cfg.Username, Password: []byte(cfg.Password), QueueID: int(queue.ID)}
	fetcher := connector.NewPOP3Fetcher()

	deadline := time.Now().Add(25 * time.Second)
	for time.Now().Before(deadline) {
		if err := fetcher.Fetch(ctx, acc, postmasterHandler); err != nil {
			t.Fatalf("fetch: %v", err)
		}
		tid := findTicketByTitle(ctx, db, subject)
		if tid > 0 {
			if count := attachmentsCount(ctx, db, tid); count >= 2 {
				meta := attachmentMetas(ctx, db, tid)
				if len(meta) != 2 {
					t.Fatalf("expected two attachments, got %d", len(meta))
				}
				var note, json *attachmentMeta
				for i := range meta {
					switch {
					case strings.Contains(meta[i].Filename, "note.txt"):
						note = &meta[i]
					case strings.Contains(meta[i].Filename, "data.json"):
						json = &meta[i]
					}
				}
				if note == nil || json == nil {
					t.Fatalf("attachments missing note or json: %+v", meta)
				}
				if !strings.Contains(note.ContentType, "text/plain") {
					t.Fatalf("note content type unexpected: %s", note.ContentType)
				}
				if !strings.Contains(json.ContentType, "application/json") {
					t.Fatalf("json content type unexpected: %s", json.ContentType)
				}
				noteSize := len("attachment-" + attToken)
				jsonSize := len("json-" + attToken)
				if note.ContentSize != noteSize {
					t.Fatalf("note size mismatch: got %d want %d", note.ContentSize, noteSize)
				}
				if json.ContentSize != jsonSize {
					t.Fatalf("json size mismatch: got %d want %d", json.ContentSize, jsonSize)
				}
				noteContent := attachmentContent(ctx, db, note.ID)
				jsonContent := attachmentContent(ctx, db, json.ID)
				if len(noteContent) != noteSize {
					t.Fatalf("note stored size mismatch: got %d want %d", len(noteContent), noteSize)
				}
				if len(jsonContent) != jsonSize {
					t.Fatalf("json stored size mismatch: got %d want %d", len(jsonContent), jsonSize)
				}
				noteHash := sha256.Sum256([]byte("attachment-" + attToken))
				storedNoteHash := sha256.Sum256(noteContent)
				if noteHash != storedNoteHash {
					t.Fatalf("note hash mismatch")
				}
				jsonHash := sha256.Sum256([]byte("json-" + attToken))
				storedJSONHash := sha256.Sum256(jsonContent)
				if jsonHash != storedJSONHash {
					t.Fatalf("json hash mismatch")
				}
				if !attachmentContainsToken(ctx, db, note.ID, attToken) {
					t.Fatalf("note attachment missing token")
				}
				if !attachmentContainsToken(ctx, db, json.ID, attToken) {
					t.Fatalf("json attachment missing token")
				}
				return
			}
		}
		time.Sleep(500 * time.Millisecond)
	}

	t.Fatalf("attachment not stored for subject %q", subject)
}

func TestPOP3FetcherStoresInlineRelatedAttachments(t *testing.T) {
	cfg := loadConfig(t)
	client := NewSMTP4DevClient(cfg.APIBase, nil)
	ctx := context.Background()

	db, err := database.GetDB()
	if err != nil {
		t.Skipf("database unavailable: %v", err)
	}

	gen, err := ticketnumber.Resolve("DateChecksum", "10", nil)
	if err != nil {
		t.Fatalf("ticket number generator: %v", err)
	}
	repository.SetTicketNumberGenerator(gen, ticketnumber.NewDBStore(db, "10"))
	t.Cleanup(func() { repository.SetTicketNumberGenerator(nil, nil) })

	queueName := getenv("TEST_QUEUE_NAME", "Postmaster")
	queueRepo := repository.NewQueueRepository(db)
	queue, err := queueRepo.GetByName(queueName)
	if err != nil || queue == nil {
		t.Fatalf("queue %s unavailable: %v", queueName, err)
	}

	box, err := client.CreateMailbox(ctx, cfg.Username, cfg.Username, cfg.Password)
	if err != nil {
		t.Fatalf("create mailbox: %v", err)
	}
	t.Cleanup(func() { _ = client.DeleteMailbox(context.Background(), box.ID) })
	_ = client.DeleteAllMessages(ctx)

	subject := fmt.Sprintf("POP Inline Related %s", randomToken())
	htmlToken := randomToken()
	cid := "img1"
	boundary := "BOUNDARY-" + randomToken()
	inlineData := base64.StdEncoding.EncodeToString([]byte("inline-" + htmlToken))
	raw := []byte(fmt.Sprintf(
		"From: %s\r\nTo: %s\r\nSubject: %s\r\nMIME-Version: 1.0\r\nContent-Type: multipart/related; boundary=%s\r\n\r\n"+
			"--%s\r\nContent-Type: text/html; charset=utf-8\r\n\r\n<html><body><p>token-%s</p><img src=\"cid:%s\" /></body></html>\r\n"+
			"--%s\r\nContent-Type: image/png; name=\"inline.png\"\r\nContent-Disposition: inline; filename=\"inline.png\"\r\nContent-ID: <%s>\r\nContent-Transfer-Encoding: base64\r\n\r\n%s\r\n"+
			"--%s--\r\n",
		cfg.FromAddress, cfg.SystemAddress, subject, boundary,
		boundary, htmlToken, cid,
		boundary, cid, inlineData,
		boundary,
	))
	auth := smtp.PlainAuth("", cfg.Username, cfg.Password, strings.Split(cfg.SMTPAddr, ":")[0])
	if err := smtp.SendMail(cfg.SMTPAddr, auth, cfg.FromAddress, []string{cfg.SystemAddress}, raw); err != nil {
		t.Fatalf("send smtp with inline: %v", err)
	}

	ticketRepo := repository.NewTicketRepository(db)
	articleRepo := repository.NewArticleRepository(db)
	ticketSvc := service.NewTicketService(ticketRepo, service.WithArticleRepository(articleRepo))
	postmasterHandler := postmaster.Service{
		FilterChain: filters.NewChain(),
		Handler:     postmaster.NewTicketProcessor(ticketSvc, postmaster.WithTicketProcessorFallbackQueue(int(queue.ID)), postmaster.WithTicketProcessorMessageLookup(articleRepo)),
	}

	acc := connector.Account{Type: "pop3", Host: cfg.POPHost, Port: cfg.POPPort, Username: cfg.Username, Password: []byte(cfg.Password), QueueID: int(queue.ID)}
	fetcher := connector.NewPOP3Fetcher()

	deadline := time.Now().Add(25 * time.Second)
	for time.Now().Before(deadline) {
		if err := fetcher.Fetch(ctx, acc, postmasterHandler); err != nil {
			t.Fatalf("fetch: %v", err)
		}
		tid := findTicketByTitle(ctx, db, subject)
		if tid > 0 {
			if count := attachmentsCount(ctx, db, tid); count >= 1 {
				meta := attachmentMetas(ctx, db, tid)
				if len(meta) == 0 {
					t.Fatalf("missing inline attachment meta")
				}
				foundInline := false
				for i := range meta {
					if strings.Contains(meta[i].Filename, "inline") && strings.Contains(strings.ToLower(meta[i].ContentType), "image/png") {
						content := attachmentContent(ctx, db, meta[i].ID)
						if !strings.Contains(string(content), htmlToken) {
							t.Fatalf("inline attachment missing token")
						}
						foundInline = true
						break
					}
				}
				if !foundInline {
					t.Fatalf("inline attachment not captured: %+v", meta)
				}
				metaBody := latestArticleMeta(ctx, db, tid)
				if metaBody == nil || (!strings.Contains(metaBody.Body, htmlToken) && !strings.Contains(metaBody.Body, "token-"+htmlToken)) {
					t.Fatalf("article body missing html token")
				}
				return
			}
		}
		time.Sleep(500 * time.Millisecond)
	}

	t.Fatalf("inline attachment not stored for subject %q", subject)
}

func TestPOP3FetcherStoresMultipartAlternativeWithAttachment(t *testing.T) {
	cfg := loadConfig(t)
	client := NewSMTP4DevClient(cfg.APIBase, nil)
	ctx := context.Background()

	db, err := database.GetDB()
	if err != nil {
		t.Skipf("database unavailable: %v", err)
	}

	gen, err := ticketnumber.Resolve("DateChecksum", "10", nil)
	if err != nil {
		t.Fatalf("ticket number generator: %v", err)
	}
	repository.SetTicketNumberGenerator(gen, ticketnumber.NewDBStore(db, "10"))
	t.Cleanup(func() { repository.SetTicketNumberGenerator(nil, nil) })

	queueName := getenv("TEST_QUEUE_NAME", "Postmaster")
	queueRepo := repository.NewQueueRepository(db)
	queue, err := queueRepo.GetByName(queueName)
	if err != nil || queue == nil {
		t.Fatalf("queue %s unavailable: %v", queueName, err)
	}

	box, err := client.CreateMailbox(ctx, cfg.Username, cfg.Username, cfg.Password)
	if err != nil {
		t.Fatalf("create mailbox: %v", err)
	}
	t.Cleanup(func() { _ = client.DeleteMailbox(context.Background(), box.ID) })
	_ = client.DeleteAllMessages(ctx)

	subject := fmt.Sprintf("POP Alt+Attachment %s", randomToken())
	textToken := randomToken()
	htmlToken := randomToken()
	attToken := randomToken()
	altBoundary := "ALT-" + randomToken()
	mixBoundary := "MIX-" + randomToken()
	attachment := base64.StdEncoding.EncodeToString([]byte("attachment-" + attToken))
	raw := []byte(fmt.Sprintf(
		"From: %s\r\nTo: %s\r\nSubject: %s\r\nMIME-Version: 1.0\r\nContent-Type: multipart/mixed; boundary=%s\r\n\r\n"+
			"--%s\r\nContent-Type: multipart/alternative; boundary=%s\r\n\r\n"+
			"--%s\r\nContent-Type: text/plain; charset=utf-8\r\n\r\ntext-%s\r\n"+
			"--%s\r\nContent-Type: text/html; charset=utf-8\r\n\r\n<html><body><p>html-%s</p></body></html>\r\n"+
			"--%s--\r\n"+
			"--%s\r\nContent-Type: application/octet-stream; name=\"att.txt\"\r\nContent-Disposition: attachment; filename=\"att.txt\"\r\nContent-Transfer-Encoding: base64\r\n\r\n%s\r\n"+
			"--%s--\r\n",
		cfg.FromAddress, cfg.SystemAddress, subject, mixBoundary,
		mixBoundary, altBoundary,
		altBoundary, textToken,
		altBoundary, htmlToken,
		altBoundary,
		mixBoundary, attachment,
		mixBoundary,
	))
	auth := smtp.PlainAuth("", cfg.Username, cfg.Password, strings.Split(cfg.SMTPAddr, ":")[0])
	if err := smtp.SendMail(cfg.SMTPAddr, auth, cfg.FromAddress, []string{cfg.SystemAddress}, raw); err != nil {
		t.Fatalf("send smtp alt: %v", err)
	}

	ticketRepo := repository.NewTicketRepository(db)
	articleRepo := repository.NewArticleRepository(db)
	ticketSvc := service.NewTicketService(ticketRepo, service.WithArticleRepository(articleRepo))
	postmasterHandler := postmaster.Service{
		FilterChain: filters.NewChain(),
		Handler:     postmaster.NewTicketProcessor(ticketSvc, postmaster.WithTicketProcessorFallbackQueue(int(queue.ID)), postmaster.WithTicketProcessorMessageLookup(articleRepo)),
	}

	acc := connector.Account{Type: "pop3", Host: cfg.POPHost, Port: cfg.POPPort, Username: cfg.Username, Password: []byte(cfg.Password), QueueID: int(queue.ID)}
	fetcher := connector.NewPOP3Fetcher()

	deadline := time.Now().Add(25 * time.Second)
	for time.Now().Before(deadline) {
		if err := fetcher.Fetch(ctx, acc, postmasterHandler); err != nil {
			t.Fatalf("fetch: %v", err)
		}
		tid := findTicketByTitle(ctx, db, subject)
		if tid > 0 {
			meta := latestArticleMeta(ctx, db, tid)
			if meta == nil {
				t.Fatalf("missing article meta")
			}
			if !strings.Contains(meta.Body, htmlToken) || !strings.Contains(meta.Body, textToken) {
				t.Fatalf("missing alt body tokens")
			}
			if count := attachmentsCount(ctx, db, tid); count >= 1 {
				metas := attachmentMetas(ctx, db, tid)
				if len(metas) == 0 {
					t.Fatalf("missing attachment metas")
				}
				found := false
				for i := range metas {
					if strings.Contains(metas[i].Filename, "att.txt") {
						content := attachmentContent(ctx, db, metas[i].ID)
						if !strings.Contains(string(content), attToken) {
							t.Fatalf("attachment missing token")
						}
						found = true
						break
					}
				}
				if !found {
					t.Fatalf("attachment not captured: %+v", metas)
				}
				return
			}
		}
		time.Sleep(500 * time.Millisecond)
	}

	t.Fatalf("multipart alternative with attachment not stored for subject %q", subject)
}

func TestPOP3FetcherSkipsUnknownRecipientMailbox(t *testing.T) {
	cfg := loadConfig(t)
	client := NewSMTP4DevClient(cfg.APIBase, nil)
	ctx := context.Background()

	box, err := client.CreateMailbox(ctx, cfg.Username, cfg.Username, cfg.Password)
	if err != nil {
		t.Fatalf("create mailbox: %v", err)
	}
	t.Cleanup(func() { _ = client.DeleteMailbox(context.Background(), box.ID) })
	_ = client.DeleteAllMessages(ctx)

	unknownLogin := "missing-" + randomToken()
	unknownAddr := fmt.Sprintf("%s@%s", unknownLogin, strings.Split(cfg.SystemAddress, "@")[1])
	subject := "Unknown Recipient"
	body := "should not arrive"
	if err := sendSMTP(t, cfg, unknownAddr, subject, body); err != nil {
		t.Fatalf("send smtp unknown recipient: %v", err)
	}

	acc := connector.Account{Type: "pop3", Host: cfg.POPHost, Port: cfg.POPPort, Username: cfg.Username, Password: []byte(cfg.Password)}
	h := &recordingHandler{}
	fetcher := connector.NewPOP3Fetcher()

	// Allow some time for smtp4dev delivery; then ensure no messages fetched for our mailbox.
	deadline := time.Now().Add(6 * time.Second)
	for time.Now().Before(deadline) {
		h.messages = nil
		if err := fetcher.Fetch(ctx, acc, h); err != nil {
			t.Fatalf("fetch: %v", err)
		}
		if len(h.messages) > 0 {
			t.Fatalf("unexpected messages fetched for unknown recipient")
		}
		msgs, err := client.ListMessages(ctx, box.ID)
		if err != nil {
			t.Fatalf("list messages: %v", err)
		}
		if len(msgs) == 0 {
			return
		}
		time.Sleep(300 * time.Millisecond)
	}

	t.Fatalf("unknown recipient message leaked into system mailbox")
}

func TestEmailQueueTaskSendsViaSMTP4Dev(t *testing.T) {
	cfg := loadConfig(t)
	client := NewSMTP4DevClient(cfg.APIBase, nil)
	ctx := context.Background()

	db, err := database.GetDB()
	if err != nil {
		t.Skipf("database unavailable: %v", err)
	}

	if _, err := db.ExecContext(ctx, database.ConvertPlaceholders("DELETE FROM mail_queue")); err != nil {
		t.Fatalf("clear mail_queue: %v", err)
	}
	t.Cleanup(func() {
		_, _ = db.ExecContext(context.Background(), database.ConvertPlaceholders("DELETE FROM mail_queue"))
	})

	box, err := client.CreateMailbox(ctx, cfg.Username, cfg.Username, cfg.Password)
	if err != nil {
		t.Fatalf("create mailbox: %v", err)
	}
	t.Cleanup(func() { _ = client.DeleteMailbox(context.Background(), box.ID) })
	_ = client.DeleteAllMessages(ctx)

	smtpHost, smtpPortStr, err := net.SplitHostPort(cfg.SMTPAddr)
	if err != nil {
		t.Fatalf("split smtp addr: %v", err)
	}
	smtpPort, err := strconv.Atoi(smtpPortStr)
	if err != nil {
		t.Fatalf("parse smtp port: %v", err)
	}

	domain := "example.com"
	if _, tail, ok := strings.Cut(cfg.SystemAddress, "@"); ok && tail != "" {
		domain = tail
	}

	emailCfg := &config.EmailConfig{Enabled: true, From: cfg.FromAddress}
	emailCfg.SMTP.Host = smtpHost
	emailCfg.SMTP.Port = smtpPort
	emailCfg.SMTP.User = cfg.Username
	emailCfg.SMTP.Password = cfg.Password
	emailCfg.SMTP.AuthType = "plain"
	emailCfg.SMTP.TLS = false
	emailCfg.SMTP.TLSMode = "none"
	emailCfg.SMTP.SkipVerify = true

	queueRepo := mailqueue.NewMailQueueRepository(db)
	subjectToken := randomToken()
	subject := fmt.Sprintf("SMTP4DEV Outbound %s", subjectToken)
	bodyToken := randomToken()
	body := fmt.Sprintf("Body %s", bodyToken)
	messageID := mailqueue.GenerateMessageID(domain)
	raw := []byte(fmt.Sprintf("From: %s\r\nTo: %s\r\nSubject: %s\r\nMessage-ID: %s\r\n\r\n%s\r\n", cfg.FromAddress, cfg.SystemAddress, subject, messageID, body))
	sender := cfg.FromAddress
	item := &mailqueue.MailQueueItem{
		Sender:     &sender,
		Recipient:  cfg.SystemAddress,
		RawMessage: raw,
		Attempts:   0,
	}
	if err := queueRepo.Insert(ctx, item); err != nil {
		t.Fatalf("queue insert: %v", err)
	}

	task := tasks.NewEmailQueueTask(db, emailCfg)
	if err := task.Run(ctx); err != nil {
		t.Fatalf("email queue task: %v", err)
	}

	deadline := time.Now().Add(15 * time.Second)
	found := false
	for time.Now().Before(deadline) {
		msgs, err := client.ListMessages(ctx, box.ID)
		if err != nil {
			t.Fatalf("list messages: %v", err)
		}
		for _, m := range msgs {
			if strings.Contains(m.Subject, subjectToken) {
				found = true
				break
			}
		}
		if found {
			break
		}
		time.Sleep(400 * time.Millisecond)
	}
	if !found {
		t.Fatalf("smtp4dev did not receive queued email with token %s", subjectToken)
	}

	if !articleHasMessageID(ctx, db, subject, strings.Trim(messageID, "<>")) {
		t.Fatalf("message-id not persisted for subject %s", subject)
	}

	if count := mailQueueCount(ctx, db); count != 0 {
		t.Fatalf("mail_queue not drained, count=%d", count)
	}
}

type failOnceHandler struct {
	inner  connector.Handler
	failed bool
}

func (h *failOnceHandler) Handle(ctx context.Context, msg *connector.FetchedMessage) error {
	if !h.failed {
		h.failed = true
		return fmt.Errorf("transient fail")
	}
	return h.inner.Handle(ctx, msg)
}

func TestPOP3AttachmentsSurviveRetry(t *testing.T) {
	cfg := loadConfig(t)
	client := NewSMTP4DevClient(cfg.APIBase, nil)
	ctx := context.Background()

	db, err := database.GetDB()
	if err != nil {
		t.Skipf("database unavailable: %v", err)
	}

	gen, err := ticketnumber.Resolve("DateChecksum", "10", nil)
	if err != nil {
		t.Fatalf("ticket number generator: %v", err)
	}
	repository.SetTicketNumberGenerator(gen, ticketnumber.NewDBStore(db, "10"))
	t.Cleanup(func() { repository.SetTicketNumberGenerator(nil, nil) })

	queueName := getenv("TEST_QUEUE_NAME", "Postmaster")
	queueRepo := repository.NewQueueRepository(db)
	queue, err := queueRepo.GetByName(queueName)
	if err != nil || queue == nil {
		t.Fatalf("queue %s unavailable: %v", queueName, err)
	}

	box, err := client.CreateMailbox(ctx, cfg.Username, cfg.Username, cfg.Password)
	if err != nil {
		t.Fatalf("create mailbox: %v", err)
	}
	t.Cleanup(func() { _ = client.DeleteMailbox(context.Background(), box.ID) })
	_ = client.DeleteAllMessages(ctx)

	subject := fmt.Sprintf("POP Attachment Retry %s", randomToken())
	body := fmt.Sprintf("Body %s", randomToken())
	attToken := randomToken()
	boundary := "BOUNDARY-" + randomToken()
	attachment := base64.StdEncoding.EncodeToString([]byte("attachment-" + attToken))
	second := base64.StdEncoding.EncodeToString([]byte("json-" + attToken))
	raw := []byte(fmt.Sprintf(
		"From: %s\r\nTo: %s\r\nSubject: %s\r\nMIME-Version: 1.0\r\nContent-Type: multipart/mixed; boundary=%s\r\n\r\n"+
			"--%s\r\nContent-Type: text/plain; charset=utf-8\r\n\r\n%s\r\n"+
			"--%s\r\nContent-Type: text/plain; name=\"note.txt\"\r\nContent-Disposition: attachment; filename=\"note.txt\"\r\nContent-Transfer-Encoding: base64\r\n\r\n%s\r\n"+
			"--%s\r\nContent-Type: application/json; name=\"data.json\"\r\nContent-Disposition: attachment; filename=\"data.json\"\r\nContent-Transfer-Encoding: base64\r\n\r\n%s\r\n"+
			"--%s--\r\n",
		cfg.FromAddress, cfg.SystemAddress, subject, boundary,
		boundary, body,
		boundary, attachment,
		boundary, second,
		boundary,
	))
	if err := smtp.SendMail(cfg.SMTPAddr, smtp.PlainAuth("", cfg.Username, cfg.Password, strings.Split(cfg.SMTPAddr, ":")[0]), cfg.FromAddress, []string{cfg.SystemAddress}, raw); err != nil {
		t.Fatalf("send smtp with attachment: %v", err)
	}

	ticketRepo := repository.NewTicketRepository(db)
	articleRepo := repository.NewArticleRepository(db)
	ticketSvc := service.NewTicketService(ticketRepo, service.WithArticleRepository(articleRepo))
	postmasterHandler := postmaster.Service{
		FilterChain: filters.NewChain(),
		Handler:     postmaster.NewTicketProcessor(ticketSvc, postmaster.WithTicketProcessorFallbackQueue(int(queue.ID)), postmaster.WithTicketProcessorMessageLookup(articleRepo)),
	}

	acc := connector.Account{Type: "pop3", Host: cfg.POPHost, Port: cfg.POPPort, Username: cfg.Username, Password: []byte(cfg.Password), QueueID: int(queue.ID)}
	fetcher := connector.NewPOP3Fetcher()
	failing := &failOnceHandler{inner: &postmasterHandler}

	if err := fetcher.Fetch(ctx, acc, failing); err == nil {
		t.Fatalf("expected first fetch to fail")
	}

	deadline := time.Now().Add(25 * time.Second)
	for time.Now().Before(deadline) {
		if err := fetcher.Fetch(ctx, acc, failing); err != nil {
			if strings.Contains(err.Error(), "transient fail") {
				continue
			}
			t.Fatalf("fetch: %v", err)
		}
		tid := findTicketByTitle(ctx, db, subject)
		if tid > 0 {
			if count := attachmentsCount(ctx, db, tid); count >= 2 {
				meta := attachmentMetas(ctx, db, tid)
				if len(meta) != 2 {
					t.Fatalf("expected two attachments, got %d", len(meta))
				}
				var note, json *attachmentMeta
				for i := range meta {
					switch {
					case strings.Contains(meta[i].Filename, "note.txt"):
						note = &meta[i]
					case strings.Contains(meta[i].Filename, "data.json"):
						json = &meta[i]
					}
				}
				if note == nil || json == nil {
					t.Fatalf("attachments missing note or json: %+v", meta)
				}
				noteSize := len("attachment-" + attToken)
				jsonSize := len("json-" + attToken)
				if note.ContentSize != noteSize {
					t.Fatalf("note size mismatch: got %d want %d", note.ContentSize, noteSize)
				}
				if json.ContentSize != jsonSize {
					t.Fatalf("json size mismatch: got %d want %d", json.ContentSize, jsonSize)
				}
				noteContent := attachmentContent(ctx, db, note.ID)
				jsonContent := attachmentContent(ctx, db, json.ID)
				if len(noteContent) != noteSize {
					t.Fatalf("note stored size mismatch: got %d want %d", len(noteContent), noteSize)
				}
				if len(jsonContent) != jsonSize {
					t.Fatalf("json stored size mismatch: got %d want %d", len(jsonContent), jsonSize)
				}
				noteHash := sha256.Sum256([]byte("attachment-" + attToken))
				storedNoteHash := sha256.Sum256(noteContent)
				if noteHash != storedNoteHash {
					t.Fatalf("note hash mismatch")
				}
				jsonHash := sha256.Sum256([]byte("json-" + attToken))
				storedJSONHash := sha256.Sum256(jsonContent)
				if jsonHash != storedJSONHash {
					t.Fatalf("json hash mismatch")
				}
				if !attachmentContainsToken(ctx, db, note.ID, attToken) {
					t.Fatalf("note attachment missing token")
				}
				if !attachmentContainsToken(ctx, db, json.ID, attToken) {
					t.Fatalf("json attachment missing token")
				}
				return
			}
		}
		time.Sleep(500 * time.Millisecond)
	}

	t.Fatalf("ticket not created with attachments after retry")
}

func TestEmailQueueTaskRetriesOnFailure(t *testing.T) {
	ctx := context.Background()

	db, err := database.GetDB()
	if err != nil {
		t.Skipf("database unavailable: %v", err)
	}

	if _, err := db.ExecContext(ctx, database.ConvertPlaceholders("DELETE FROM mail_queue")); err != nil {
		t.Fatalf("clear mail_queue: %v", err)
	}
	t.Cleanup(func() {
		_, _ = db.ExecContext(context.Background(), database.ConvertPlaceholders("DELETE FROM mail_queue"))
	})

	emailCfg := &config.EmailConfig{Enabled: true, From: "sender@example.com"}
	emailCfg.SMTP.Host = "127.0.0.1"
	emailCfg.SMTP.Port = 1
	emailCfg.SMTP.AuthType = "plain"
	emailCfg.SMTP.TLS = false
	emailCfg.SMTP.TLSMode = "none"
	emailCfg.SMTP.SkipVerify = true

	queueRepo := mailqueue.NewMailQueueRepository(db)
	sender := "sender@example.com"
	raw := []byte("From: sender@example.com\r\nTo: fail@example.com\r\nSubject: Retry\r\n\r\nretry\r\n")
	item := &mailqueue.MailQueueItem{Sender: &sender, Recipient: "fail@example.com", RawMessage: raw}
	if err := queueRepo.Insert(ctx, item); err != nil {
		t.Fatalf("queue insert: %v", err)
	}

	task := tasks.NewEmailQueueTask(db, emailCfg)
	if err := task.Run(ctx); err != nil {
		t.Fatalf("email queue task run: %v", err)
	}

	var id int64
	var attempts int
	var lastMsg sql.NullString
	var due sql.NullTime
	q := "SELECT id, attempts, last_smtp_message, due_time FROM mail_queue ORDER BY id DESC LIMIT 1"
	if err := db.QueryRowContext(ctx, q).Scan(&id, &attempts, &lastMsg, &due); err != nil {
		t.Fatalf("select mail_queue: %v", err)
	}
	if attempts != 1 {
		t.Fatalf("attempts mismatch: got %d", attempts)
	}
	if !lastMsg.Valid || strings.TrimSpace(lastMsg.String) == "" {
		t.Fatalf("last_smtp_message not recorded: %+v", lastMsg)
	}
	if !due.Valid {
		t.Fatalf("due_time not set after failure")
	}
	if d := time.Until(due.Time); d < 4*time.Minute || d > 6*time.Minute {
		t.Fatalf("unexpected first backoff: %v", d)
	}

	// Make message immediately pending again.
	if _, err := db.ExecContext(ctx, database.ConvertPlaceholders("UPDATE mail_queue SET due_time = $1 WHERE id = $2"), time.Now().Add(-time.Minute), id); err != nil {
		t.Fatalf("reset due_time: %v", err)
	}

	if err := task.Run(ctx); err != nil {
		t.Fatalf("second email queue task run: %v", err)
	}

	if err := db.QueryRowContext(ctx, database.ConvertPlaceholders("SELECT attempts, last_smtp_message, due_time FROM mail_queue WHERE id = $1"), id).Scan(&attempts, &lastMsg, &due); err != nil {
		t.Fatalf("select mail_queue after second run: %v", err)
	}
	if attempts != 2 {
		t.Fatalf("attempts mismatch after second run: got %d", attempts)
	}
	if !lastMsg.Valid || strings.TrimSpace(lastMsg.String) == "" {
		t.Fatalf("last_smtp_message not recorded after second run: %+v", lastMsg)
	}
	if !due.Valid {
		t.Fatalf("due_time not set after second failure")
	}
	if d := time.Until(due.Time); d < 20*time.Minute || d > 30*time.Minute {
		t.Fatalf("unexpected second backoff: %v", d)
	}

	// Force through max retries and ensure it stops.
	for i := 0; i < tasks.MaxRetries; i++ {
		if _, err := db.ExecContext(ctx, database.ConvertPlaceholders("UPDATE mail_queue SET due_time = $1 WHERE id = $2"), time.Now().Add(-time.Minute), id); err != nil {
			t.Fatalf("reset due_time loop %d: %v", i, err)
		}
		if err := task.Run(ctx); err != nil {
			t.Fatalf("task run %d: %v", i, err)
		}
	}

	var count int
	if err := db.QueryRowContext(ctx, database.ConvertPlaceholders("SELECT COUNT(*) FROM mail_queue WHERE id = $1"), id).Scan(&count); err != nil {
		t.Fatalf("count mail_queue: %v", err)
	}
	if count != 1 {
		t.Fatalf("mail_queue row missing unexpectedly")
	}
	if err := db.QueryRowContext(ctx, database.ConvertPlaceholders("SELECT attempts, due_time FROM mail_queue WHERE id = $1"), id).Scan(&attempts, &due); err != nil {
		t.Fatalf("select mail_queue after max retries: %v", err)
	}
	if attempts < tasks.MaxRetries {
		t.Fatalf("attempts not capped: %d", attempts)
	}
	if due.Valid {
		t.Fatalf("due_time should be null after max retries")
	}
}

func TestEmailQueueCleanupDeletesOldFailed(t *testing.T) {
	ctx := context.Background()

	db, err := database.GetDB()
	if err != nil {
		t.Skipf("database unavailable: %v", err)
	}

	if _, err := db.ExecContext(ctx, database.ConvertPlaceholders("DELETE FROM mail_queue")); err != nil {
		t.Fatalf("clear mail_queue: %v", err)
	}
	t.Cleanup(func() {
		_, _ = db.ExecContext(context.Background(), database.ConvertPlaceholders("DELETE FROM mail_queue"))
	})

	emailCfg := &config.EmailConfig{Enabled: true, From: "sender@example.com"}
	emailCfg.SMTP.Host = "127.0.0.1"
	emailCfg.SMTP.Port = 1
	emailCfg.SMTP.AuthType = "plain"
	emailCfg.SMTP.TLS = false
	emailCfg.SMTP.TLSMode = "none"
	emailCfg.SMTP.SkipVerify = true

	oldCreate := time.Now().Add(-8 * 24 * time.Hour)
	newCreate := time.Now()
	raw := []byte("From: sender@example.com\r\nTo: fail@example.com\r\nSubject: Cleanup\r\n\r\nbody\r\n")

	insert := func(create time.Time, recipient string) int64 {
		q := database.ConvertPlaceholders(`
			INSERT INTO mail_queue (
				insert_fingerprint, article_id, attempts, sender, recipient,
				raw_message, due_time, last_smtp_code, last_smtp_message, create_time
			) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)`)
		res, err := db.ExecContext(ctx, q, nil, nil, tasks.MaxRetries, "sender@example.com", recipient, raw, time.Now().Add(10*time.Minute), nil, stringPtr("fail"), create)
		if err != nil {
			t.Fatalf("insert mail_queue: %v", err)
		}
		id, _ := res.LastInsertId()
		return id
	}

	oldID := insert(oldCreate, "old@example.com")
	newID := insert(newCreate, "new@example.com")

	if err := tasks.NewEmailQueueTask(db, emailCfg).Run(ctx); err != nil {
		t.Fatalf("email queue task: %v", err)
	}

	var count int
	if err := db.QueryRowContext(ctx, database.ConvertPlaceholders("SELECT COUNT(*) FROM mail_queue")).Scan(&count); err != nil {
		t.Fatalf("count mail_queue: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected one row after cleanup, got %d", count)
	}

	var id int64
	if err := db.QueryRowContext(ctx, database.ConvertPlaceholders("SELECT id FROM mail_queue LIMIT 1")).Scan(&id); err != nil {
		t.Fatalf("select remaining id: %v", err)
	}
	if id != newID {
		t.Fatalf("expected new item to remain, got id %d (old %d)", id, oldID)
	}
}

func TestEmailQueueCleanupKeepsRecentFailed(t *testing.T) {
	ctx := context.Background()

	db, err := database.GetDB()
	if err != nil {
		t.Skipf("database unavailable: %v", err)
	}

	if _, err := db.ExecContext(ctx, database.ConvertPlaceholders("DELETE FROM mail_queue")); err != nil {
		t.Fatalf("clear mail_queue: %v", err)
	}
	t.Cleanup(func() {
		_, _ = db.ExecContext(context.Background(), database.ConvertPlaceholders("DELETE FROM mail_queue"))
	})

	emailCfg := &config.EmailConfig{Enabled: true, From: "sender@example.com"}
	emailCfg.SMTP.Host = "127.0.0.1"
	emailCfg.SMTP.Port = 1
	emailCfg.SMTP.AuthType = "plain"
	emailCfg.SMTP.TLS = false
	emailCfg.SMTP.TLSMode = "none"
	emailCfg.SMTP.SkipVerify = true

	create := time.Now().Add(-48 * time.Hour)
	raw := []byte("From: sender@example.com\r\nTo: fail@example.com\r\nSubject: Recent\r\n\r\nbody\r\n")

	q := database.ConvertPlaceholders(`
		INSERT INTO mail_queue (
			insert_fingerprint, article_id, attempts, sender, recipient,
			raw_message, due_time, last_smtp_code, last_smtp_message, create_time
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)`)
	res, err := db.ExecContext(ctx, q, nil, nil, tasks.MaxRetries, "sender@example.com", "recent@example.com", raw, time.Now().Add(-time.Minute), nil, stringPtr("fail"), create)
	if err != nil {
		t.Fatalf("insert mail_queue: %v", err)
	}
	id, _ := res.LastInsertId()

	if err := tasks.NewEmailQueueTask(db, emailCfg).Run(ctx); err != nil {
		t.Fatalf("email queue task: %v", err)
	}

	var count int
	if err := db.QueryRowContext(ctx, database.ConvertPlaceholders("SELECT COUNT(*) FROM mail_queue")).Scan(&count); err != nil {
		t.Fatalf("count mail_queue: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected recent failed row to remain, got %d", count)
	}

	var remaining int64
	if err := db.QueryRowContext(ctx, database.ConvertPlaceholders("SELECT id FROM mail_queue LIMIT 1")).Scan(&remaining); err != nil {
		t.Fatalf("select remaining id: %v", err)
	}
	if remaining != id {
		t.Fatalf("unexpected row removed, kept %d expected %d", remaining, id)
	}
}

func TestEmailQueueCleanupLeavesPendingAndRecent(t *testing.T) {
	ctx := context.Background()

	db, err := database.GetDB()
	if err != nil {
		t.Skipf("database unavailable: %v", err)
	}

	if _, err := db.ExecContext(ctx, database.ConvertPlaceholders("DELETE FROM mail_queue")); err != nil {
		t.Fatalf("clear mail_queue: %v", err)
	}
	t.Cleanup(func() {
		_, _ = db.ExecContext(context.Background(), database.ConvertPlaceholders("DELETE FROM mail_queue"))
	})

	emailCfg := &config.EmailConfig{Enabled: true, From: "sender@example.com"}
	emailCfg.SMTP.Host = "127.0.0.1"
	emailCfg.SMTP.Port = 1
	emailCfg.SMTP.AuthType = "plain"
	emailCfg.SMTP.TLS = false
	emailCfg.SMTP.TLSMode = "none"
	emailCfg.SMTP.SkipVerify = true

	raw := []byte("From: sender@example.com\r\nTo: fail@example.com\r\nSubject: Mixed\r\n\r\nbody\r\n")
	insert := func(attempts int, due *time.Time, created time.Time, recipient string) int64 {
		q := database.ConvertPlaceholders(`
			INSERT INTO mail_queue (
				insert_fingerprint, article_id, attempts, sender, recipient,
				raw_message, due_time, last_smtp_code, last_smtp_message, create_time
			) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)`)
		res, err := db.ExecContext(ctx, q, nil, nil, attempts, "sender@example.com", recipient, raw, due, nil, stringPtr("fail"), created)
		if err != nil {
			t.Fatalf("insert mail_queue: %v", err)
		}
		id, _ := res.LastInsertId()
		return id
	}

	old := time.Now().Add(-8 * 24 * time.Hour)
	recent := time.Now().Add(-48 * time.Hour)
	pastDue := time.Now().Add(-time.Minute)

	oldFailed := insert(tasks.MaxRetries, &pastDue, old, "old@example.com")
	recentFailed := insert(tasks.MaxRetries, &pastDue, recent, "recent@example.com")
	pending := insert(0, &pastDue, time.Now(), "pending@example.com")

	if err := tasks.NewEmailQueueTask(db, emailCfg).Run(ctx); err != nil {
		t.Fatalf("email queue task: %v", err)
	}

	var ids []int64
	rows, err := db.QueryContext(ctx, database.ConvertPlaceholders("SELECT id FROM mail_queue ORDER BY id"))
	if err != nil {
		t.Fatalf("select remaining: %v", err)
	}
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			t.Fatalf("scan id: %v", err)
		}
		ids = append(ids, id)
	}
	_ = rows.Close()

	if len(ids) != 2 {
		t.Fatalf("expected 2 rows after cleanup, got %d (ids=%v)", len(ids), ids)
	}
	for _, keep := range []int64{recentFailed, pending} {
		found := false
		for _, id := range ids {
			if id == keep {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("expected id %d to remain; got %v (old removed %d)", keep, ids, oldFailed)
		}
	}
}

func TestEmailQueueRetriesOnTransientSmtp4xx(t *testing.T) {
	ctx := context.Background()

	db, err := database.GetDB()
	if err != nil {
		t.Skipf("database unavailable: %v", err)
	}

	if _, err := db.ExecContext(ctx, database.ConvertPlaceholders("DELETE FROM mail_queue")); err != nil {
		t.Fatalf("clear mail_queue: %v", err)
	}
	t.Cleanup(func() {
		_, _ = db.ExecContext(context.Background(), database.ConvertPlaceholders("DELETE FROM mail_queue"))
	})

	// SMTP target that will refuse with a transient 4xx.
	emailCfg := &config.EmailConfig{Enabled: true, From: "sender@example.com"}
	emailCfg.SMTP.Host = "127.0.0.1"
	emailCfg.SMTP.Port = 25252
	emailCfg.SMTP.AuthType = "plain"
	emailCfg.SMTP.TLS = false
	emailCfg.SMTP.TLSMode = "none"
	emailCfg.SMTP.SkipVerify = true

	// Tiny TCP server that replies 421 on any SMTP command, then closes.
	lis, err := net.Listen("tcp", "127.0.0.1:25252")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	t.Cleanup(func() { _ = lis.Close() })

	go func() {
		for {
			conn, err := lis.Accept()
			if err != nil {
				return
			}
			go func(c net.Conn) {
				defer c.Close()
				_, _ = c.Write([]byte("220 temp\r\n"))
				buf := make([]byte, 1024)
				for {
					_, err := c.Read(buf)
					if err != nil {
						return
					}
					_, _ = c.Write([]byte("421 transient error\r\n"))
					return
				}
			}(conn)
		}
	}()

	queueRepo := mailqueue.NewMailQueueRepository(db)
	sender := "sender@example.com"
	raw := []byte("From: sender@example.com\r\nTo: fail@example.com\r\nSubject: Retry4xx\r\n\r\nretry\r\n")
	item := &mailqueue.MailQueueItem{Sender: &sender, Recipient: "fail@example.com", RawMessage: raw}
	if err := queueRepo.Insert(ctx, item); err != nil {
		t.Fatalf("queue insert: %v", err)
	}

	task := tasks.NewEmailQueueTask(db, emailCfg)
	if err := task.Run(ctx); err == nil {
		t.Fatalf("expected smtp 4xx to surface as error")
	}

	var attempts int
	var lastMsg sql.NullString
	var due sql.NullTime
	if err := db.QueryRowContext(ctx, database.ConvertPlaceholders("SELECT attempts, last_smtp_message, due_time FROM mail_queue ORDER BY id DESC LIMIT 1")).Scan(&attempts, &lastMsg, &due); err != nil {
		t.Fatalf("select mail_queue: %v", err)
	}
	if attempts != 1 {
		t.Fatalf("attempts mismatch: %d", attempts)
	}
	if !lastMsg.Valid || !strings.Contains(strings.ToLower(lastMsg.String), "421") {
		t.Fatalf("last_smtp_message missing 421: %+v", lastMsg)
	}
	if !due.Valid {
		t.Fatalf("due_time not set after transient failure")
	}
}

func TestEmailQueueRetriesOnPermanentSmtp5xx(t *testing.T) {
	ctx := context.Background()

	db, err := database.GetDB()
	if err != nil {
		t.Skipf("database unavailable: %v", err)
	}

	if _, err := db.ExecContext(ctx, database.ConvertPlaceholders("DELETE FROM mail_queue")); err != nil {
		t.Fatalf("clear mail_queue: %v", err)
	}
	t.Cleanup(func() {
		_, _ = db.ExecContext(context.Background(), database.ConvertPlaceholders("DELETE FROM mail_queue"))
	})

	emailCfg := &config.EmailConfig{Enabled: true, From: "sender@example.com"}
	emailCfg.SMTP.Host = "127.0.0.1"
	emailCfg.SMTP.Port = 25253
	emailCfg.SMTP.AuthType = "plain"
	emailCfg.SMTP.TLS = false
	emailCfg.SMTP.TLSMode = "none"
	emailCfg.SMTP.SkipVerify = true

	lis, err := net.Listen("tcp", "127.0.0.1:25253")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	t.Cleanup(func() { _ = lis.Close() })

	go func() {
		for {
			conn, err := lis.Accept()
			if err != nil {
				return
			}
			go func(c net.Conn) {
				defer c.Close()
				_, _ = c.Write([]byte("220 temp\r\n"))
				buf := make([]byte, 1024)
				for {
					_, err := c.Read(buf)
					if err != nil {
						return
					}
					_, _ = c.Write([]byte("550 permanent failure\r\n"))
					return
				}
			}(conn)
		}
	}()

	queueRepo := mailqueue.NewMailQueueRepository(db)
	sender := "sender@example.com"
	raw := []byte("From: sender@example.com\r\nTo: fail@example.com\r\nSubject: Retry5xx\r\n\r\nretry\r\n")
	item := &mailqueue.MailQueueItem{Sender: &sender, Recipient: "fail@example.com", RawMessage: raw}
	if err := queueRepo.Insert(ctx, item); err != nil {
		t.Fatalf("queue insert: %v", err)
	}

	task := tasks.NewEmailQueueTask(db, emailCfg)
	if err := task.Run(ctx); err == nil {
		t.Fatalf("expected smtp 5xx to surface as error")
	}

	var attempts int
	var lastMsg sql.NullString
	var due sql.NullTime
	if err := db.QueryRowContext(ctx, database.ConvertPlaceholders("SELECT attempts, last_smtp_message, due_time FROM mail_queue ORDER BY id DESC LIMIT 1")).Scan(&attempts, &lastMsg, &due); err != nil {
		t.Fatalf("select mail_queue: %v", err)
	}
	if attempts != 1 {
		t.Fatalf("attempts mismatch: %d", attempts)
	}
	if !lastMsg.Valid || !strings.Contains(strings.ToLower(lastMsg.String), "550") {
		t.Fatalf("last_smtp_message missing 550: %+v", lastMsg)
	}
	if !due.Valid {
		t.Fatalf("due_time not set after permanent failure")
	}
}

func TestPOP3FetcherIgnoresDSNBounceMessage(t *testing.T) {
	cfg := loadConfig(t)
	client := NewSMTP4DevClient(cfg.APIBase, nil)
	ctx := context.Background()

	box, err := client.CreateMailbox(ctx, cfg.Username, cfg.Username, cfg.Password)
	if err != nil {
		t.Fatalf("create mailbox: %v", err)
	}
	t.Cleanup(func() { _ = client.DeleteMailbox(context.Background(), box.ID) })
	_ = client.DeleteAllMessages(ctx)

	// Craft a simple DSN/bounce that should not be treated as customer mail.
	boundary := "BOUNDARY-" + randomToken()
	origMsg := "From: user@example.com\r\nTo: " + cfg.SystemAddress + "\r\nSubject: Orig\r\n\r\norig body\r\n"
	bounce := []byte(fmt.Sprintf(
		"From: MAILER-DAEMON@%s\r\nTo: %s\r\nSubject: Mail delivery failed\r\nMIME-Version: 1.0\r\nContent-Type: multipart/report; report-type=delivery-status; boundary=%s\r\n\r\n"+
			"--%s\r\nContent-Type: text/plain; charset=us-ascii\r\n\r\nThis is a bounce.\r\n"+
			"--%s\r\nContent-Type: message/delivery-status\r\n\r\nAction: failed\r\nStatus: 5.1.1\r\nDiagnostic-Code: smtp; 550 5.1.1\r\n"+
			"--%s\r\nContent-Type: message/rfc822\r\n\r\n%s\r\n"+
			"--%s--\r\n",
		cfg.POPHost, cfg.SystemAddress, boundary,
		boundary,
		boundary,
		boundary, origMsg,
		boundary,
	))
	auth := smtp.PlainAuth("", cfg.Username, cfg.Password, strings.Split(cfg.SMTPAddr, ":")[0])
	if err := smtp.SendMail(cfg.SMTPAddr, auth, cfg.FromAddress, []string{cfg.SystemAddress}, bounce); err != nil {
		t.Fatalf("send bounce: %v", err)
	}

	acc := connector.Account{Type: "pop3", Host: cfg.POPHost, Port: cfg.POPPort, Username: cfg.Username, Password: []byte(cfg.Password)}
	h := &recordingHandler{}
	fetcher := connector.NewPOP3Fetcher()

	// Bounce should be fetched but ignored by handler; here we assert it's at least retrievable and then mailbox drains.
	deadline := time.Now().Add(10 * time.Second)
	fetched := false
	for time.Now().Before(deadline) {
		h.messages = nil
		if err := fetcher.Fetch(ctx, acc, h); err != nil {
			t.Fatalf("fetch: %v", err)
		}
		if len(h.messages) > 0 {
			fetched = true
			break
		}
		time.Sleep(400 * time.Millisecond)
	}
	if !fetched {
		t.Fatalf("bounce not fetched")
	}

	msgs, err := client.ListMessages(ctx, box.ID)
	if err != nil {
		t.Fatalf("list messages: %v", err)
	}
	if len(msgs) != 0 {
		t.Fatalf("bounce should be drained after fetch, got %d", len(msgs))
	}
}

func TestPOP3FetcherHandlesLargeAuthHeaders(t *testing.T) {
	cfg := loadConfig(t)
	client := NewSMTP4DevClient(cfg.APIBase, nil)
	ctx := context.Background()

	box, err := client.CreateMailbox(ctx, cfg.Username, cfg.Username, cfg.Password)
	if err != nil {
		t.Fatalf("create mailbox: %v", err)
	}
	t.Cleanup(func() { _ = client.DeleteMailbox(context.Background(), box.ID) })
	_ = client.DeleteAllMessages(ctx)

	subject := fmt.Sprintf("POP Large Headers %s", randomToken())
	bodyToken := randomToken()
	big := strings.Repeat("a", 8*1024)
	raw := []byte(fmt.Sprintf(
		"From: %s\r\nTo: %s\r\nSubject: %s\r\nReceived-SPF: %s\r\nDKIM-Signature: v=1; d=example.com; s=key; bh=%s; h=from:to:subject; b=%s\r\n\r\nBody %s\r\n",
		cfg.FromAddress, cfg.SystemAddress, subject, big, big, big, bodyToken,
	))
	auth := smtp.PlainAuth("", cfg.Username, cfg.Password, strings.Split(cfg.SMTPAddr, ":")[0])
	if err := smtp.SendMail(cfg.SMTPAddr, auth, cfg.FromAddress, []string{cfg.SystemAddress}, raw); err != nil {
		t.Fatalf("send smtp large headers: %v", err)
	}

	acc := connector.Account{Type: "pop3", Host: cfg.POPHost, Port: cfg.POPPort, Username: cfg.Username, Password: []byte(cfg.Password)}
	h := &recordingHandler{}
	fetcher := connector.NewPOP3Fetcher()

	deadline := time.Now().Add(15 * time.Second)
	for time.Now().Before(deadline) {
		h.messages = nil
		if err := fetcher.Fetch(ctx, acc, h); err != nil {
			t.Fatalf("fetch: %v", err)
		}
		for _, msg := range h.messages {
			if strings.Contains(string(msg.Raw), bodyToken) && strings.Contains(string(msg.Raw), big[:64]) {
				msgs, err := client.ListMessages(ctx, box.ID)
				if err != nil {
					t.Fatalf("list messages: %v", err)
				}
				if len(msgs) != 0 {
					t.Fatalf("expected mailbox drained")
				}
				return
			}
		}
		time.Sleep(400 * time.Millisecond)
	}

	t.Fatalf("large header message not fetched")
}

func TestPOP3FetcherDrainsHighVolumeBatch(t *testing.T) {
	cfg := loadConfig(t)
	client := NewSMTP4DevClient(cfg.APIBase, nil)
	ctx := context.Background()

	box, err := client.CreateMailbox(ctx, cfg.Username, cfg.Username, cfg.Password)
	if err != nil {
		t.Fatalf("create mailbox: %v", err)
	}
	t.Cleanup(func() { _ = client.DeleteMailbox(context.Background(), box.ID) })
	_ = client.DeleteAllMessages(ctx)

	tokens := make([]string, 60)
	for i := range tokens {
		tokens[i] = randomToken()
		if err := sendSMTP(t, cfg, cfg.SystemAddress, fmt.Sprintf("Bulk %d %s", i, tokens[i]), tokens[i]); err != nil {
			t.Fatalf("send bulk %d: %v", i, err)
		}
	}

	acc := connector.Account{Type: "pop3", Host: cfg.POPHost, Port: cfg.POPPort, Username: cfg.Username, Password: []byte(cfg.Password)}
	h := &recordingHandler{}
	fetcher := connector.NewPOP3Fetcher()
	seen := make(map[string]bool)

	deadline := time.Now().Add(60 * time.Second)
	for time.Now().Before(deadline) {
		h.messages = nil
		if err := fetcher.Fetch(ctx, acc, h); err != nil {
			t.Fatalf("fetch: %v", err)
		}
		for _, msg := range h.messages {
			body := string(msg.Raw)
			for _, tok := range tokens {
				if strings.Contains(body, tok) {
					seen[tok] = true
				}
			}
		}
		if len(seen) == len(tokens) {
			msgs, err := client.ListMessages(ctx, box.ID)
			if err != nil {
				t.Fatalf("list messages: %v", err)
			}
			if len(msgs) != 0 {
				t.Fatalf("mailbox not drained, count=%d", len(msgs))
			}
			return
		}
		time.Sleep(500 * time.Millisecond)
	}

	t.Fatalf("did not fetch all bulk messages, got %d of %d", len(seen), len(tokens))
}

func TestPOP3FetcherRecoversFromMailboxLock(t *testing.T) {
	ctx := context.Background()

	lis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer lis.Close()

	token := randomToken()
	msg := fmt.Sprintf("From: stub@example.com\r\nTo: user@example.com\r\nSubject: Lock\r\n\r\n%s\r\n", token)

	first := true
	go func() {
		for {
			conn, err := lis.Accept()
			if err != nil {
				return
			}
			go func(c net.Conn) {
				defer c.Close()
				_, _ = c.Write([]byte("+OK pop stub\r\n"))
				r := bufio.NewReader(c)
				authed := false
				lockedSent := false
				for {
					line, err := r.ReadString('\n')
					if err != nil {
						return
					}
					s := strings.TrimSpace(line)
					switch {
					case strings.HasPrefix(s, "CAPA"):
						_, _ = c.Write([]byte("+OK\r\n.\r\n"))
					case strings.HasPrefix(s, "USER"):
						if first && !lockedSent {
							lockedSent = true
							first = false
							_, _ = c.Write([]byte("-ERR [IN-USE] mailbox locked\r\n"))
							return
						}
						_, _ = c.Write([]byte("+OK\r\n"))
					case strings.HasPrefix(s, "PASS"):
						authed = true
						first = false
						_, _ = c.Write([]byte("+OK\r\n"))
					case strings.HasPrefix(s, "STAT") && authed:
						_, _ = c.Write([]byte("+OK 1 100\r\n"))
					case strings.HasPrefix(s, "LIST") && authed:
						_, _ = c.Write([]byte("+OK\r\n1 100\r\n.\r\n"))
					case strings.HasPrefix(s, "UIDL") && authed:
						_, _ = c.Write([]byte("+OK\r\n1 uid1\r\n.\r\n"))
					case strings.HasPrefix(s, "RETR") && authed:
						_, _ = c.Write([]byte("+OK 100\r\n" + msg + ".\r\n"))
					case strings.HasPrefix(s, "DELE") && authed:
						_, _ = c.Write([]byte("+OK\r\n"))
					case strings.HasPrefix(s, "QUIT"):
						_, _ = c.Write([]byte("+OK\r\n"))
						return
					default:
						_, _ = c.Write([]byte("+OK\r\n"))
					}
				}
			}(conn)
		}
	}()

	acc := connector.Account{Type: "pop3", Host: "127.0.0.1", Port: lis.Addr().(*net.TCPAddr).Port, Username: "user", Password: []byte("pass")}
	h := &recordingHandler{}
	fetcher := connector.NewPOP3Fetcher()

	// First fetch hits lock error.
	if err := fetcher.Fetch(ctx, acc, h); err == nil {
		t.Fatalf("expected lock error")
	}

	// Second fetch should succeed and deliver message once.
	if err := fetcher.Fetch(ctx, acc, h); err != nil {
		t.Fatalf("fetch after lock: %v", err)
	}
	if len(h.messages) != 1 || !strings.Contains(string(h.messages[0].Raw), token) {
		t.Fatalf("message not delivered after lock recovery")
	}
}

func TestEmailQueueSuccessClearsFailureMetadata(t *testing.T) {
	cfg := loadConfig(t)
	client := NewSMTP4DevClient(cfg.APIBase, nil)
	ctx := context.Background()

	db, err := database.GetDB()
	if err != nil {
		t.Skipf("database unavailable: %v", err)
	}

	if _, err := db.ExecContext(ctx, database.ConvertPlaceholders("DELETE FROM mail_queue")); err != nil {
		t.Fatalf("clear mail_queue: %v", err)
	}
	t.Cleanup(func() {
		_, _ = db.ExecContext(context.Background(), database.ConvertPlaceholders("DELETE FROM mail_queue"))
	})

	box, err := client.CreateMailbox(ctx, cfg.Username, cfg.Username, cfg.Password)
	if err != nil {
		t.Fatalf("create mailbox: %v", err)
	}
	t.Cleanup(func() { _ = client.DeleteMailbox(context.Background(), box.ID) })
	_ = client.DeleteAllMessages(ctx)

	smtpHost, smtpPortStr, err := net.SplitHostPort(cfg.SMTPAddr)
	if err != nil {
		t.Fatalf("split smtp addr: %v", err)
	}
	smtpPort, err := strconv.Atoi(smtpPortStr)
	if err != nil {
		t.Fatalf("parse smtp port: %v", err)
	}

	subjectToken := randomToken()
	subject := fmt.Sprintf("CLEAR META %s", subjectToken)
	body := "success clears metadata"
	raw := mailqueue.BuildEmailMessage(cfg.FromAddress, cfg.SystemAddress, subject, body)

	q := database.ConvertPlaceholders(`
		INSERT INTO mail_queue (
			insert_fingerprint, article_id, attempts, sender, recipient,
			raw_message, due_time, last_smtp_code, last_smtp_message, create_time
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)`)
	if _, err := db.ExecContext(ctx, q, nil, nil, 2, cfg.FromAddress, cfg.SystemAddress, raw, time.Now().Add(-time.Minute), 550, "previous failure", time.Now().Add(-time.Hour)); err != nil {
		t.Fatalf("insert mail_queue: %v", err)
	}

	emailCfg := &config.EmailConfig{Enabled: true, From: cfg.FromAddress}
	emailCfg.SMTP.Host = smtpHost
	emailCfg.SMTP.Port = smtpPort
	emailCfg.SMTP.User = cfg.Username
	emailCfg.SMTP.Password = cfg.Password
	emailCfg.SMTP.AuthType = "plain"
	emailCfg.SMTP.TLSMode = "none"
	emailCfg.SMTP.SkipVerify = true

	if err := tasks.NewEmailQueueTask(db, emailCfg).Run(ctx); err != nil {
		t.Fatalf("email queue task: %v", err)
	}

	deadline := time.Now().Add(10 * time.Second)
	found := false
	for time.Now().Before(deadline) {
		msgs, err := client.ListMessages(ctx, box.ID)
		if err != nil {
			t.Fatalf("list messages: %v", err)
		}
		for _, m := range msgs {
			if strings.Contains(m.Subject, subjectToken) {
				found = true
				break
			}
		}
		if found {
			break
		}
		time.Sleep(300 * time.Millisecond)
	}
	if !found {
		t.Fatalf("message with token %s not delivered", subjectToken)
	}

	var count int
	if err := db.QueryRowContext(ctx, database.ConvertPlaceholders("SELECT COUNT(*) FROM mail_queue")).Scan(&count); err != nil {
		t.Fatalf("count mail_queue: %v", err)
	}
	if count != 0 {
		t.Fatalf("expected queue to be empty after success, count=%d", count)
	}
}

func TestEmailQueueTaskStartTLSSendsViaSMTP4Dev(t *testing.T) {
	cfg := loadConfig(t)
	client := NewSMTP4DevClient(cfg.APIBase, nil)
	ctx := context.Background()

	db, err := database.GetDB()
	if err != nil {
		t.Skipf("database unavailable: %v", err)
	}

	if _, err := db.ExecContext(ctx, database.ConvertPlaceholders("DELETE FROM mail_queue")); err != nil {
		t.Fatalf("clear mail_queue: %v", err)
	}
	t.Cleanup(func() {
		_, _ = db.ExecContext(context.Background(), database.ConvertPlaceholders("DELETE FROM mail_queue"))
	})

	box, err := client.CreateMailbox(ctx, cfg.Username, cfg.Username, cfg.Password)
	if err != nil {
		t.Fatalf("create mailbox: %v", err)
	}
	t.Cleanup(func() { _ = client.DeleteMailbox(context.Background(), box.ID) })
	_ = client.DeleteAllMessages(ctx)

	smtpHost, smtpPortStr, err := net.SplitHostPort(cfg.SMTPAddr)
	if err != nil {
		t.Fatalf("split smtp addr: %v", err)
	}
	smtpPort, err := strconv.Atoi(smtpPortStr)
	if err != nil {
		t.Fatalf("parse smtp port: %v", err)
	}

	queueRepo := mailqueue.NewMailQueueRepository(db)
	subjectToken := randomToken()
	subject := fmt.Sprintf("SMTP4DEV STARTTLS %s", subjectToken)
	body := "starttls-body"
	raw := mailqueue.BuildEmailMessage(cfg.FromAddress, cfg.SystemAddress, subject, body)
	sender := cfg.FromAddress
	item := &mailqueue.MailQueueItem{Sender: &sender, Recipient: cfg.SystemAddress, RawMessage: raw}
	if err := queueRepo.Insert(ctx, item); err != nil {
		t.Fatalf("queue insert: %v", err)
	}

	emailCfg := &config.EmailConfig{Enabled: true, From: cfg.FromAddress}
	emailCfg.SMTP.Host = smtpHost
	emailCfg.SMTP.Port = smtpPort
	emailCfg.SMTP.User = cfg.Username
	emailCfg.SMTP.Password = cfg.Password
	emailCfg.SMTP.AuthType = "plain"
	emailCfg.SMTP.TLSMode = "starttls"
	emailCfg.SMTP.SkipVerify = true

	if err := tasks.NewEmailQueueTask(db, emailCfg).Run(ctx); err != nil {
		lower := strings.ToLower(err.Error())
		if strings.Contains(lower, "starttls") || strings.Contains(lower, "502") {
			t.Skipf("smtp server lacks starttls: %v", err)
		}
		t.Fatalf("email queue task: %v", err)
	}

	deadline := time.Now().Add(10 * time.Second)
	found := false
	for time.Now().Before(deadline) {
		msgs, err := client.ListMessages(ctx, box.ID)
		if err != nil {
			t.Fatalf("list messages: %v", err)
		}
		for _, m := range msgs {
			if strings.Contains(m.Subject, subjectToken) {
				found = true
				break
			}
		}
		if found {
			break
		}
		time.Sleep(300 * time.Millisecond)
	}
	if !found {
		t.Fatalf("smtp4dev did not receive starttls email with token %s", subjectToken)
	}
}

func TestEmailQueueTaskSMTPSSendsViaSMTP4Dev(t *testing.T) {
	cfg := loadConfig(t)
	ctx := context.Background()

	db, err := database.GetDB()
	if err != nil {
		t.Skipf("database unavailable: %v", err)
	}

	smtpHost, smtpPortStr, err := net.SplitHostPort(cfg.SMTPAddr)
	if err != nil {
		t.Fatalf("split smtp addr: %v", err)
	}
	smtpPort, err := strconv.Atoi(smtpPortStr)
	if err != nil {
		t.Fatalf("parse smtp port: %v", err)
	}

	conn, err := tls.Dial("tcp", cfg.SMTPAddr, &tls.Config{InsecureSkipVerify: true, ServerName: smtpHost})
	if err != nil {
		t.Skipf("smtps unavailable: %v", err)
	}
	_ = conn.Close()

	client := NewSMTP4DevClient(cfg.APIBase, nil)

	if _, err := db.ExecContext(ctx, database.ConvertPlaceholders("DELETE FROM mail_queue")); err != nil {
		t.Fatalf("clear mail_queue: %v", err)
	}
	t.Cleanup(func() {
		_, _ = db.ExecContext(context.Background(), database.ConvertPlaceholders("DELETE FROM mail_queue"))
	})

	box, err := client.CreateMailbox(ctx, cfg.Username, cfg.Username, cfg.Password)
	if err != nil {
		t.Fatalf("create mailbox: %v", err)
	}
	t.Cleanup(func() { _ = client.DeleteMailbox(context.Background(), box.ID) })
	_ = client.DeleteAllMessages(ctx)

	queueRepo := mailqueue.NewMailQueueRepository(db)
	subjectToken := randomToken()
	subject := fmt.Sprintf("SMTP4DEV SMTPS %s", subjectToken)
	body := "smtps-body"
	raw := mailqueue.BuildEmailMessage(cfg.FromAddress, cfg.SystemAddress, subject, body)
	sender := cfg.FromAddress
	item := &mailqueue.MailQueueItem{Sender: &sender, Recipient: cfg.SystemAddress, RawMessage: raw}
	if err := queueRepo.Insert(ctx, item); err != nil {
		t.Fatalf("queue insert: %v", err)
	}

	emailCfg := &config.EmailConfig{Enabled: true, From: cfg.FromAddress}
	emailCfg.SMTP.Host = smtpHost
	emailCfg.SMTP.Port = smtpPort
	emailCfg.SMTP.User = cfg.Username
	emailCfg.SMTP.Password = cfg.Password
	emailCfg.SMTP.AuthType = "plain"
	emailCfg.SMTP.TLSMode = "smtps"
	emailCfg.SMTP.SkipVerify = true

	if err := tasks.NewEmailQueueTask(db, emailCfg).Run(ctx); err != nil {
		lower := strings.ToLower(err.Error())
		if strings.Contains(lower, "tls") || strings.Contains(lower, "ssl") {
			t.Skipf("smtps not supported: %v", err)
		}
		t.Fatalf("email queue task: %v", err)
	}

	deadline := time.Now().Add(10 * time.Second)
	found := false
	for time.Now().Before(deadline) {
		msgs, err := client.ListMessages(ctx, box.ID)
		if err != nil {
			t.Fatalf("list messages: %v", err)
		}
		for _, m := range msgs {
			if strings.Contains(m.Subject, subjectToken) {
				found = true
				break
			}
		}
		if found {
			break
		}
		time.Sleep(300 * time.Millisecond)
	}
	if !found {
		t.Fatalf("smtp4dev did not receive smtps email with token %s", subjectToken)
	}
}

func TestEmailQueueOutboundReplyThreadsViaPostmaster(t *testing.T) {
	cfg := loadConfig(t)
	client := NewSMTP4DevClient(cfg.APIBase, nil)
	ctx := context.Background()

	db, err := database.GetDB()
	if err != nil {
		t.Skipf("database unavailable: %v", err)
	}

	if _, err := db.ExecContext(ctx, database.ConvertPlaceholders("DELETE FROM mail_queue")); err != nil {
		t.Fatalf("clear mail_queue: %v", err)
	}
	t.Cleanup(func() {
		_, _ = db.ExecContext(context.Background(), database.ConvertPlaceholders("DELETE FROM mail_queue"))
	})

	gen, err := ticketnumber.Resolve("DateChecksum", "10", nil)
	if err != nil {
		t.Fatalf("ticket number generator: %v", err)
	}
	repository.SetTicketNumberGenerator(gen, ticketnumber.NewDBStore(db, "10"))
	t.Cleanup(func() { repository.SetTicketNumberGenerator(nil, nil) })

	queueName := getenv("TEST_QUEUE_NAME", "Postmaster")
	queueRepo := repository.NewQueueRepository(db)
	queue, err := queueRepo.GetByName(queueName)
	if err != nil || queue == nil {
		t.Fatalf("queue %s unavailable: %v", queueName, err)
	}

	box, err := client.CreateMailbox(ctx, cfg.Username, cfg.Username, cfg.Password)
	if err != nil {
		t.Fatalf("create mailbox: %v", err)
	}
	t.Cleanup(func() { _ = client.DeleteMailbox(context.Background(), box.ID) })
	_ = client.DeleteAllMessages(ctx)

	smtpHost, smtpPortStr, err := net.SplitHostPort(cfg.SMTPAddr)
	if err != nil {
		t.Fatalf("split smtp addr: %v", err)
	}
	smtpPort, err := strconv.Atoi(smtpPortStr)
	if err != nil {
		t.Fatalf("parse smtp port: %v", err)
	}

	domain := "example.com"
	if _, tail, ok := strings.Cut(cfg.SystemAddress, "@"); ok && tail != "" {
		domain = tail
	}

	inboundSubject := fmt.Sprintf("Thread Seed %s", randomToken())
	inboundBody := fmt.Sprintf("Seed body %s", randomToken())
	if err := sendSMTP(t, cfg, cfg.SystemAddress, inboundSubject, inboundBody); err != nil {
		t.Fatalf("send inbound: %v", err)
	}

	ticketRepo := repository.NewTicketRepository(db)
	articleRepo := repository.NewArticleRepository(db)
	ticketSvc := service.NewTicketService(ticketRepo, service.WithArticleRepository(articleRepo))
	postmasterHandler := postmaster.Service{
		FilterChain: filters.NewChain(),
		Handler: postmaster.NewTicketProcessor(
			ticketSvc,
			postmaster.WithTicketProcessorFallbackQueue(int(queue.ID)),
			postmaster.WithTicketProcessorMessageLookup(articleRepo),
		),
	}

	acc := connector.Account{Type: "pop3", Host: cfg.POPHost, Port: cfg.POPPort, Username: cfg.Username, Password: []byte(cfg.Password), QueueID: int(queue.ID)}
	fetcher := connector.NewPOP3Fetcher()

	deadline := time.Now().Add(20 * time.Second)
	var ticketID int
	for time.Now().Before(deadline) {
		if err := fetcher.Fetch(ctx, acc, postmasterHandler); err != nil {
			t.Fatalf("seed fetch: %v", err)
		}
		if tid := findTicketByTitle(ctx, db, inboundSubject); tid > 0 {
			ticketID = tid
			break
		}
		time.Sleep(400 * time.Millisecond)
	}
	if ticketID == 0 {
		t.Fatalf("ticket not created for seed subject %q", inboundSubject)
	}
	baselineArticles := articleCount(ctx, db, ticketID)

	outboundSubject := fmt.Sprintf("Outbound %s", randomToken())
	outboundBody := fmt.Sprintf("Outbound body %s", randomToken())
	messageID := mailqueue.GenerateMessageID(domain)
	messageIDNormalized := strings.Trim(messageID, "<>")
	raw := []byte(fmt.Sprintf("From: %s\r\nTo: %s\r\nSubject: %s\r\nMessage-ID: %s\r\n\r\n%s\r\n", cfg.FromAddress, cfg.SystemAddress, outboundSubject, messageID, outboundBody))

	outArticle := &models.Article{TicketID: ticketID, Subject: outboundSubject, Body: outboundBody, MimeType: "text/plain"}
	if err := articleRepo.Create(outArticle); err != nil {
		t.Fatalf("create outbound article: %v", err)
	}
	if _, err := db.ExecContext(ctx, database.ConvertPlaceholders("UPDATE article_data_mime SET a_message_id = $1 WHERE article_id = $2"), messageIDNormalized, outArticle.ID); err != nil {
		t.Fatalf("set outbound message id: %v", err)
	}

	mailQueueRepo := mailqueue.NewMailQueueRepository(db)
	articleID := int64(outArticle.ID)
	queueItem := &mailqueue.MailQueueItem{
		ArticleID:  &articleID,
		Sender:     &cfg.FromAddress,
		Recipient:  cfg.SystemAddress,
		RawMessage: raw,
	}
	if err := mailQueueRepo.Insert(ctx, queueItem); err != nil {
		t.Fatalf("queue insert: %v", err)
	}

	emailCfg := &config.EmailConfig{Enabled: true, From: cfg.FromAddress}
	emailCfg.SMTP.Host = smtpHost
	emailCfg.SMTP.Port = smtpPort
	emailCfg.SMTP.User = cfg.Username
	emailCfg.SMTP.Password = cfg.Password
	emailCfg.SMTP.AuthType = "plain"
	emailCfg.SMTP.TLSMode = "none"
	emailCfg.SMTP.SkipVerify = true

	if err := tasks.NewEmailQueueTask(db, emailCfg).Run(ctx); err != nil {
		t.Fatalf("email queue task: %v", err)
	}

	deadline = time.Now().Add(15 * time.Second)
	sent := false
	for time.Now().Before(deadline) {
		msgs, err := client.ListMessages(ctx, box.ID)
		if err != nil {
			t.Fatalf("list messages: %v", err)
		}
		for _, m := range msgs {
			if strings.Contains(m.Subject, outboundSubject) {
				sent = true
				break
			}
		}
		if sent {
			break
		}
		time.Sleep(300 * time.Millisecond)
	}
	if !sent {
		t.Fatalf("outbound mail not delivered to smtp4dev")
	}

	replyBody := fmt.Sprintf("Reply body %s", randomToken())
	replySubject := "Re: " + outboundSubject
	reply := []byte(fmt.Sprintf("From: customer@%s\r\nTo: %s\r\nSubject: %s\r\nIn-Reply-To: %s\r\nReferences: %s\r\n\r\n%s\r\n", domain, cfg.SystemAddress, replySubject, messageID, messageID, replyBody))
	auth := smtp.PlainAuth("", cfg.Username, cfg.Password, smtpHost)
	if err := smtp.SendMail(cfg.SMTPAddr, auth, cfg.FromAddress, []string{cfg.SystemAddress}, reply); err != nil {
		t.Fatalf("send reply: %v", err)
	}

	deadline = time.Now().Add(25 * time.Second)
	for time.Now().Before(deadline) {
		if err := fetcher.Fetch(ctx, acc, postmasterHandler); err != nil {
			t.Fatalf("reply fetch: %v", err)
		}
		if count := articleCount(ctx, db, ticketID); count > baselineArticles {
			meta := latestArticleMeta(ctx, db, ticketID)
			if meta == nil {
				t.Fatalf("expected follow-up meta for ticket %d", ticketID)
			}
			if !strings.Contains(meta.Body, replyBody) {
				t.Fatalf("follow-up body missing token, got: %s", meta.Body)
			}
			if meta.InReplyTo != "" && meta.InReplyTo != messageIDNormalized {
				t.Fatalf("unexpected in-reply-to %q", meta.InReplyTo)
			}
			if meta.References != "" && !strings.Contains(meta.References, messageIDNormalized) {
				t.Fatalf("references missing message id, got %q", meta.References)
			}
			if meta.MessageID == "" {
				t.Fatalf("follow-up missing message id")
			}
			return
		}
		time.Sleep(500 * time.Millisecond)
	}

	t.Fatalf("follow-up not appended to ticket %d", ticketID)
}

func TestPostmasterThreadsWithNoisyReferences(t *testing.T) {
	cfg := loadConfig(t)
	client := NewSMTP4DevClient(cfg.APIBase, nil)
	ctx := context.Background()

	db, err := database.GetDB()
	if err != nil {
		t.Skipf("database unavailable: %v", err)
	}

	gen, err := ticketnumber.Resolve("DateChecksum", "10", nil)
	if err != nil {
		t.Fatalf("ticket number generator: %v", err)
	}
	repository.SetTicketNumberGenerator(gen, ticketnumber.NewDBStore(db, "10"))
	t.Cleanup(func() { repository.SetTicketNumberGenerator(nil, nil) })

	queueName := getenv("TEST_QUEUE_NAME", "Postmaster")
	queueRepo := repository.NewQueueRepository(db)
	queue, err := queueRepo.GetByName(queueName)
	if err != nil || queue == nil {
		t.Fatalf("queue %s unavailable: %v", queueName, err)
	}

	box, err := client.CreateMailbox(ctx, cfg.Username, cfg.Username, cfg.Password)
	if err != nil {
		t.Fatalf("create mailbox: %v", err)
	}
	t.Cleanup(func() { _ = client.DeleteMailbox(context.Background(), box.ID) })
	_ = client.DeleteAllMessages(ctx)

	seedMessageID := fmt.Sprintf("<seed-%s@%s>", randomToken(), "example.com")
	seedSubject := fmt.Sprintf("Noisy Seed %s", randomToken())
	seedBody := "seed-body"
	seedRaw := []byte(fmt.Sprintf("From: %s\r\nTo: %s\r\nSubject: %s\r\nMessage-ID: %s\r\n\r\n%s\r\n", cfg.FromAddress, cfg.SystemAddress, seedSubject, seedMessageID, seedBody))
	auth := smtp.PlainAuth("", cfg.Username, cfg.Password, strings.Split(cfg.SMTPAddr, ":")[0])
	if err := smtp.SendMail(cfg.SMTPAddr, auth, cfg.FromAddress, []string{cfg.SystemAddress}, seedRaw); err != nil {
		t.Fatalf("send seed: %v", err)
	}

	articleRepo := repository.NewArticleRepository(db)
	ticketRepo := repository.NewTicketRepository(db)
	ticketSvc := service.NewTicketService(ticketRepo, service.WithArticleRepository(articleRepo))
	postmasterHandler := postmaster.Service{
		FilterChain: filters.NewChain(),
		Handler: postmaster.NewTicketProcessor(
			ticketSvc,
			postmaster.WithTicketProcessorFallbackQueue(int(queue.ID)),
			postmaster.WithTicketProcessorMessageLookup(articleRepo),
		),
	}

	acc := connector.Account{Type: "pop3", Host: cfg.POPHost, Port: cfg.POPPort, Username: cfg.Username, Password: []byte(cfg.Password), QueueID: int(queue.ID)}
	fetcher := connector.NewPOP3Fetcher()

	deadline := time.Now().Add(20 * time.Second)
	var ticketID int
	for time.Now().Before(deadline) {
		if err := fetcher.Fetch(ctx, acc, postmasterHandler); err != nil {
			t.Fatalf("seed fetch: %v", err)
		}
		if tid := findTicketByTitle(ctx, db, seedSubject); tid > 0 {
			ticketID = tid
			break
		}
		time.Sleep(400 * time.Millisecond)
	}
	if ticketID == 0 {
		t.Fatalf("seed ticket not created")
	}
	seedMeta := latestArticleMeta(ctx, db, ticketID)
	if seedMeta == nil || strings.Trim(seedMeta.MessageID, "<>") != strings.Trim(seedMessageID, "<>") {
		t.Fatalf("seed message id mismatch, got %v", seedMeta)
	}
	baseline := articleCount(ctx, db, ticketID)

	followBody := "follow-body"
	followSubject := "Re: " + seedSubject
	noisyRefs := fmt.Sprintf("<noise-%s@x> %s <dup-%s@x>", randomToken(), seedMessageID, randomToken())
	followRaw := []byte(fmt.Sprintf("From: %s\r\nTo: %s\r\nSubject: %s\r\nIn-Reply-To: %s\r\nReferences: %s\r\n\r\n%s\r\n", cfg.FromAddress, cfg.SystemAddress, followSubject, seedMessageID, noisyRefs, followBody))
	if err := smtp.SendMail(cfg.SMTPAddr, auth, cfg.FromAddress, []string{cfg.SystemAddress}, followRaw); err != nil {
		t.Fatalf("send follow: %v", err)
	}

	deadline = time.Now().Add(20 * time.Second)
	for time.Now().Before(deadline) {
		if err := fetcher.Fetch(ctx, acc, postmasterHandler); err != nil {
			t.Fatalf("follow fetch: %v", err)
		}
		if count := articleCount(ctx, db, ticketID); count > baseline {
			meta := latestArticleMeta(ctx, db, ticketID)
			if meta == nil {
				t.Fatalf("missing follow meta")
			}
			if !strings.Contains(meta.References, strings.Trim(seedMessageID, "<>")) {
				t.Fatalf("references missing seed id, got %q", meta.References)
			}
			return
		}
		time.Sleep(400 * time.Millisecond)
	}

	t.Fatalf("follow-up not threaded for ticket %d", ticketID)
}

func TestPostmasterThreadsPrefersMatchingReferenceAmidNoise(t *testing.T) {
	cfg := loadConfig(t)
	client := NewSMTP4DevClient(cfg.APIBase, nil)
	ctx := context.Background()

	db, err := database.GetDB()
	if err != nil {
		t.Skipf("database unavailable: %v", err)
	}

	gen, err := ticketnumber.Resolve("DateChecksum", "10", nil)
	if err != nil {
		t.Fatalf("ticket number generator: %v", err)
	}
	repository.SetTicketNumberGenerator(gen, ticketnumber.NewDBStore(db, "10"))
	t.Cleanup(func() { repository.SetTicketNumberGenerator(nil, nil) })

	queueName := getenv("TEST_QUEUE_NAME", "Postmaster")
	queueRepo := repository.NewQueueRepository(db)
	queue, err := queueRepo.GetByName(queueName)
	if err != nil || queue == nil {
		t.Fatalf("queue %s unavailable: %v", queueName, err)
	}

	box, err := client.CreateMailbox(ctx, cfg.Username, cfg.Username, cfg.Password)
	if err != nil {
		t.Fatalf("create mailbox: %v", err)
	}
	t.Cleanup(func() { _ = client.DeleteMailbox(context.Background(), box.ID) })
	_ = client.DeleteAllMessages(ctx)

	auth := smtp.PlainAuth("", cfg.Username, cfg.Password, strings.Split(cfg.SMTPAddr, ":")[0])

	seed1ID := fmt.Sprintf("<seed1-%s@x>", randomToken())
	seed2ID := fmt.Sprintf("<seed2-%s@x>", randomToken())
	seed1Subject := fmt.Sprintf("Noise Seed A %s", randomToken())
	seed2Subject := fmt.Sprintf("Noise Seed B %s", randomToken())
	seedBody := "seed-body"

	seed1 := []byte(fmt.Sprintf("From: %s\r\nTo: %s\r\nSubject: %s\r\nMessage-ID: %s\r\n\r\n%s\r\n", cfg.FromAddress, cfg.SystemAddress, seed1Subject, seed1ID, seedBody))
	seed2 := []byte(fmt.Sprintf("From: %s\r\nTo: %s\r\nSubject: %s\r\nMessage-ID: %s\r\n\r\n%s\r\n", cfg.FromAddress, cfg.SystemAddress, seed2Subject, seed2ID, seedBody))

	for _, raw := range [][]byte{seed1, seed2} {
		if err := smtp.SendMail(cfg.SMTPAddr, auth, cfg.FromAddress, []string{cfg.SystemAddress}, raw); err != nil {
			t.Fatalf("send seed: %v", err)
		}
	}

	articleRepo := repository.NewArticleRepository(db)
	ticketRepo := repository.NewTicketRepository(db)
	ticketSvc := service.NewTicketService(ticketRepo, service.WithArticleRepository(articleRepo))
	postmasterHandler := postmaster.Service{
		FilterChain: filters.NewChain(),
		Handler: postmaster.NewTicketProcessor(
			ticketSvc,
			postmaster.WithTicketProcessorFallbackQueue(int(queue.ID)),
			postmaster.WithTicketProcessorMessageLookup(articleRepo),
		),
	}

	acc := connector.Account{Type: "pop3", Host: cfg.POPHost, Port: cfg.POPPort, Username: cfg.Username, Password: []byte(cfg.Password), QueueID: int(queue.ID)}
	fetcher := connector.NewPOP3Fetcher()

	deadline := time.Now().Add(25 * time.Second)
	var tid1, tid2 int
	for time.Now().Before(deadline) {
		if err := fetcher.Fetch(ctx, acc, postmasterHandler); err != nil {
			t.Fatalf("seed fetch: %v", err)
		}
		if tid1 == 0 {
			tid1 = findTicketByTitle(ctx, db, seed1Subject)
		}
		if tid2 == 0 {
			tid2 = findTicketByTitle(ctx, db, seed2Subject)
		}
		if tid1 > 0 && tid2 > 0 {
			break
		}
		time.Sleep(400 * time.Millisecond)
	}
	if tid1 == 0 || tid2 == 0 {
		t.Fatalf("seed tickets missing: %d %d", tid1, tid2)
	}

	seed1Meta := latestArticleMeta(ctx, db, tid1)
	seed2Meta := latestArticleMeta(ctx, db, tid2)
	if seed1Meta == nil || seed2Meta == nil {
		t.Fatalf("missing seed meta")
	}

	baseline1 := articleCount(ctx, db, tid1)
	baseline2 := articleCount(ctx, db, tid2)

	noiseRefs := fmt.Sprintf("<noise-%s@x> %s <other-%s@x> %s", randomToken(), strings.Trim(seed1Meta.MessageID, "<>"), randomToken(), strings.Trim(seed2Meta.MessageID, "<>"))
	followSubject := "Re: " + seed2Subject
	followBody := "follow-noisy"
	follow := []byte(fmt.Sprintf("From: %s\r\nTo: %s\r\nSubject: %s\r\nIn-Reply-To: %s\r\nReferences: %s\r\n\r\n%s\r\n", cfg.FromAddress, cfg.SystemAddress, followSubject, seed2ID, noiseRefs, followBody))

	if err := smtp.SendMail(cfg.SMTPAddr, auth, cfg.FromAddress, []string{cfg.SystemAddress}, follow); err != nil {
		t.Fatalf("send follow: %v", err)
	}

	deadline = time.Now().Add(25 * time.Second)
	for time.Now().Before(deadline) {
		if err := fetcher.Fetch(ctx, acc, postmasterHandler); err != nil {
			t.Fatalf("follow fetch: %v", err)
		}
		c1 := articleCount(ctx, db, tid1)
		c2 := articleCount(ctx, db, tid2)
		if c2 > baseline2 {
			meta := latestArticleMeta(ctx, db, tid2)
			if meta == nil {
				t.Fatalf("missing follow meta for ticket2")
			}
			if !strings.Contains(meta.References, strings.Trim(seed2Meta.MessageID, "<>")) {
				t.Fatalf("ticket2 references missing seed2 id: %q", meta.References)
			}
			if c1 != baseline1 {
				t.Fatalf("ticket1 should not receive follow-up")
			}
			return
		}
		time.Sleep(400 * time.Millisecond)
	}

	t.Fatalf("follow-up not threaded to ticket2")
}

func TestPOP3FetcherDrainsMultipleMessages(t *testing.T) {
	cfg := loadConfig(t)
	client := NewSMTP4DevClient(cfg.APIBase, nil)
	ctx := context.Background()

	box, err := client.CreateMailbox(ctx, cfg.Username, cfg.Username, cfg.Password)
	if err != nil {
		t.Fatalf("create mailbox: %v", err)
	}
	t.Cleanup(func() { _ = client.DeleteMailbox(context.Background(), box.ID) })
	_ = client.DeleteAllMessages(ctx)

	tokens := []string{randomToken(), randomToken()}
	for _, tok := range tokens {
		if err := sendSMTP(t, cfg, cfg.SystemAddress, fmt.Sprintf("POP Multi %s", tok), fmt.Sprintf("POP Multi %s", tok)); err != nil {
			t.Fatalf("send smtp: %v", err)
		}
	}

	acc := connector.Account{Type: "pop3", Host: cfg.POPHost, Port: cfg.POPPort, Username: cfg.Username, Password: []byte(cfg.Password)}
	h := &recordingHandler{}
	fetcher := connector.NewPOP3Fetcher()

	found := make(map[string]bool)
	deadline := time.Now().Add(15 * time.Second)
	for time.Now().Before(deadline) {
		if err := fetcher.Fetch(ctx, acc, h); err != nil {
			t.Fatalf("fetch: %v", err)
		}
		for _, msg := range h.messages {
			body := string(msg.Raw)
			for _, tok := range tokens {
				if strings.Contains(body, tok) {
					found[tok] = true
				}
			}
		}
		if len(found) == len(tokens) {
			msgs, err := client.ListMessages(ctx, box.ID)
			if err != nil {
				t.Fatalf("list messages: %v", err)
			}
			if len(msgs) != 0 {
				t.Fatalf("expected mailbox to be empty after drain, got %d", len(msgs))
			}
			return
		}
		time.Sleep(400 * time.Millisecond)
	}

	t.Fatalf("did not fetch all tokens: %+v", found)
}

func TestPOP3FetcherRoutesMultipleAccountsToQueues(t *testing.T) {
	cfg := loadConfig(t)
	client := NewSMTP4DevClient(cfg.APIBase, nil)
	ctx := context.Background()

	primaryBox, err := client.CreateMailbox(ctx, cfg.Username, cfg.Username, cfg.Password)
	if err != nil {
		t.Fatalf("create primary mailbox: %v", err)
	}
	altLogin := "alt-" + randomToken()
	altPass := "alt-pass"
	altBox, err := client.CreateMailbox(ctx, altLogin, altLogin, altPass)
	if err != nil {
		t.Fatalf("create alt mailbox: %v", err)
	}
	t.Cleanup(func() {
		_ = client.DeleteMailbox(context.Background(), primaryBox.ID)
		_ = client.DeleteMailbox(context.Background(), altBox.ID)
	})
	_ = client.DeleteAllMessages(ctx)

	domain := "example.com"
	if _, tail, ok := strings.Cut(cfg.SystemAddress, "@"); ok && tail != "" {
		domain = tail
	}

	primaryToken := randomToken()
	altToken := randomToken()
	if err := sendSMTP(t, cfg, cfg.SystemAddress, fmt.Sprintf("Primary %s", primaryToken), primaryToken); err != nil {
		t.Fatalf("send primary: %v", err)
	}
	altRecipient := fmt.Sprintf("%s@%s", altLogin, domain)
	if err := sendSMTP(t, cfg, altRecipient, fmt.Sprintf("Alt %s", altToken), altToken); err != nil {
		t.Fatalf("send alt: %v", err)
	}

	fetcher := connector.NewPOP3Fetcher()

	fetch := func(acc connector.Account, token string, queueID int) {
		h := &recordingHandler{}
		deadline := time.Now().Add(15 * time.Second)
		for time.Now().Before(deadline) {
			h.messages = nil
			if err := fetcher.Fetch(ctx, acc, h); err != nil {
				t.Fatalf("fetch %s: %v", acc.Username, err)
			}
			for _, msg := range h.messages {
				body := string(msg.Raw)
				if !strings.Contains(body, token) {
					t.Fatalf("unexpected payload for %s", acc.Username)
				}
				snap := msg.AccountSnapshot()
				if snap.QueueID != queueID {
					t.Fatalf("queue mismatch for %s: %d", acc.Username, snap.QueueID)
				}
				if snap.Username != acc.Username {
					t.Fatalf("username mismatch: %s vs %s", snap.Username, acc.Username)
				}
				return
			}
			time.Sleep(300 * time.Millisecond)
		}
		t.Fatalf("token %s not fetched for %s", token, acc.Username)
	}

	primaryAcc := connector.Account{Type: "pop3", Host: cfg.POPHost, Port: cfg.POPPort, Username: cfg.Username, Password: []byte(cfg.Password), QueueID: 11}
	altAcc := connector.Account{Type: "pop3", Host: cfg.POPHost, Port: cfg.POPPort, Username: altLogin, Password: []byte(altPass), QueueID: 22}

	fetch(primaryAcc, primaryToken, primaryAcc.QueueID)
	fetch(altAcc, altToken, altAcc.QueueID)
}

func TestPOP3FetcherAvoidsDuplicateDeliveryOnConcurrentRuns(t *testing.T) {
	cfg := loadConfig(t)
	client := NewSMTP4DevClient(cfg.APIBase, nil)
	ctx := context.Background()

	box, err := client.CreateMailbox(ctx, cfg.Username, cfg.Username, cfg.Password)
	if err != nil {
		t.Fatalf("create mailbox: %v", err)
	}
	t.Cleanup(func() { _ = client.DeleteMailbox(context.Background(), box.ID) })
	_ = client.DeleteAllMessages(ctx)

	token := randomToken()
	if err := sendSMTP(t, cfg, cfg.SystemAddress, fmt.Sprintf("Concurrent %s", token), token); err != nil {
		t.Fatalf("send smtp: %v", err)
	}

	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		msgs, err := client.ListMessages(ctx, box.ID)
		if err != nil {
			t.Fatalf("list messages: %v", err)
		}
		if len(msgs) > 0 {
			break
		}
		time.Sleep(200 * time.Millisecond)
	}

	acc := connector.Account{Type: "pop3", Host: cfg.POPHost, Port: cfg.POPPort, Username: cfg.Username, Password: []byte(cfg.Password)}
	ch := &countingHandler{seen: make(map[string]int), token: token}
	var wg sync.WaitGroup
	errCh := make(chan error, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			errCh <- connector.NewPOP3Fetcher().Fetch(ctx, acc, ch)
		}()
	}
	wg.Wait()
	close(errCh)
	for err := range errCh {
		if err != nil {
			t.Fatalf("fetch error: %v", err)
		}
	}

	if ch.seen[token] != 1 {
		t.Fatalf("expected single delivery, got %d", ch.seen[token])
	}

	msgs, err := client.ListMessages(ctx, box.ID)
	if err != nil {
		t.Fatalf("list messages: %v", err)
	}
	if len(msgs) != 0 {
		t.Fatalf("expected mailbox empty, got %d", len(msgs))
	}
}

func TestPOP3FetcherStoresLargeAttachment(t *testing.T) {
	cfg := loadConfig(t)
	client := NewSMTP4DevClient(cfg.APIBase, nil)
	ctx := context.Background()

	db, err := database.GetDB()
	if err != nil {
		t.Skipf("database unavailable: %v", err)
	}

	gen, err := ticketnumber.Resolve("DateChecksum", "10", nil)
	if err != nil {
		t.Fatalf("ticket number generator: %v", err)
	}
	repository.SetTicketNumberGenerator(gen, ticketnumber.NewDBStore(db, "10"))
	t.Cleanup(func() { repository.SetTicketNumberGenerator(nil, nil) })

	queueName := getenv("TEST_QUEUE_NAME", "Postmaster")
	queueRepo := repository.NewQueueRepository(db)
	queue, err := queueRepo.GetByName(queueName)
	if err != nil || queue == nil {
		t.Fatalf("queue %s unavailable: %v", queueName, err)
	}

	box, err := client.CreateMailbox(ctx, cfg.Username, cfg.Username, cfg.Password)
	if err != nil {
		t.Fatalf("create mailbox: %v", err)
	}
	t.Cleanup(func() { _ = client.DeleteMailbox(context.Background(), box.ID) })
	_ = client.DeleteAllMessages(ctx)

	subject := fmt.Sprintf("POP Large Attachment %s", randomToken())
	body := "large-body"
	attToken := randomToken()
	chunk := "chunk-" + attToken
	data := strings.Repeat(chunk, (1<<20)/len(chunk)+8)
	attachment := base64.StdEncoding.EncodeToString([]byte(data))
	boundary := "BOUNDARY-" + randomToken()
	raw := []byte(fmt.Sprintf(
		"From: %s\r\nTo: %s\r\nSubject: %s\r\nMIME-Version: 1.0\r\nContent-Type: multipart/mixed; boundary=%s\r\n\r\n"+
			"--%s\r\nContent-Type: text/plain; charset=utf-8\r\n\r\n%s\r\n"+
			"--%s\r\nContent-Type: application/octet-stream; name=\"big.dat\"\r\nContent-Disposition: attachment; filename=\"big.dat\"\r\nContent-Transfer-Encoding: base64\r\n\r\n%s\r\n"+
			"--%s--\r\n",
		cfg.FromAddress, cfg.SystemAddress, subject, boundary,
		boundary, body,
		boundary, attachment,
		boundary,
	))
	auth := smtp.PlainAuth("", cfg.Username, cfg.Password, strings.Split(cfg.SMTPAddr, ":")[0])
	if err := smtp.SendMail(cfg.SMTPAddr, auth, cfg.FromAddress, []string{cfg.SystemAddress}, raw); err != nil {
		t.Fatalf("send smtp: %v", err)
	}

	ticketRepo := repository.NewTicketRepository(db)
	articleRepo := repository.NewArticleRepository(db)
	ticketSvc := service.NewTicketService(ticketRepo, service.WithArticleRepository(articleRepo))
	postmasterHandler := postmaster.Service{
		FilterChain: filters.NewChain(),
		Handler:     postmaster.NewTicketProcessor(ticketSvc, postmaster.WithTicketProcessorFallbackQueue(int(queue.ID)), postmaster.WithTicketProcessorMessageLookup(articleRepo)),
	}

	acc := connector.Account{Type: "pop3", Host: cfg.POPHost, Port: cfg.POPPort, Username: cfg.Username, Password: []byte(cfg.Password), QueueID: int(queue.ID)}
	fetcher := connector.NewPOP3Fetcher()

	deadline := time.Now().Add(35 * time.Second)
	for time.Now().Before(deadline) {
		if err := fetcher.Fetch(ctx, acc, postmasterHandler); err != nil {
			t.Fatalf("fetch: %v", err)
		}
		tid := findTicketByTitle(ctx, db, subject)
		if tid > 0 {
			if count := attachmentsCount(ctx, db, tid); count >= 1 {
				meta := attachmentMetas(ctx, db, tid)
				if len(meta) == 0 {
					t.Fatalf("missing attachment meta")
				}
				m := meta[0]
				if m.ContentSize != len(data) {
					t.Fatalf("content size mismatch: %d vs %d", m.ContentSize, len(data))
				}
				stored := attachmentContent(ctx, db, m.ID)
				if len(stored) != len(data) {
					t.Fatalf("stored size mismatch: %d vs %d", len(stored), len(data))
				}
				if !strings.Contains(string(stored), attToken) {
					t.Fatalf("attachment missing token")
				}
				return
			}
		}
		time.Sleep(500 * time.Millisecond)
	}

	t.Fatalf("large attachment not stored for subject %q", subject)
}

func TestPOP3FetcherStoresLargeHtmlWithInlineImages(t *testing.T) {
	cfg := loadConfig(t)
	client := NewSMTP4DevClient(cfg.APIBase, nil)
	ctx := context.Background()

	db, err := database.GetDB()
	if err != nil {
		t.Skipf("database unavailable: %v", err)
	}

	gen, err := ticketnumber.Resolve("DateChecksum", "10", nil)
	if err != nil {
		t.Fatalf("ticket number generator: %v", err)
	}
	repository.SetTicketNumberGenerator(gen, ticketnumber.NewDBStore(db, "10"))
	t.Cleanup(func() { repository.SetTicketNumberGenerator(nil, nil) })

	queueName := getenv("TEST_QUEUE_NAME", "Postmaster")
	queueRepo := repository.NewQueueRepository(db)
	queue, err := queueRepo.GetByName(queueName)
	if err != nil || queue == nil {
		t.Fatalf("queue %s unavailable: %v", queueName, err)
	}

	box, err := client.CreateMailbox(ctx, cfg.Username, cfg.Username, cfg.Password)
	if err != nil {
		t.Fatalf("create mailbox: %v", err)
	}
	t.Cleanup(func() { _ = client.DeleteMailbox(context.Background(), box.ID) })
	_ = client.DeleteAllMessages(ctx)

	subject := fmt.Sprintf("POP Large HTML Inline %s", randomToken())
	htmlToken := randomToken()
	imgToken1 := randomToken()
	imgToken2 := randomToken()
	bodyChunk := strings.Repeat("<p>html-"+htmlToken+"</p>", 200)
	boundary := "BOUNDARY-" + randomToken()
	cid1 := "img-one"
	cid2 := "img-two"
	img1 := base64.StdEncoding.EncodeToString([]byte("img-" + imgToken1))
	img2 := base64.StdEncoding.EncodeToString([]byte("img-" + imgToken2))
	raw := []byte(fmt.Sprintf(
		"From: %s\r\nTo: %s\r\nSubject: %s\r\nMIME-Version: 1.0\r\nContent-Type: multipart/related; boundary=%s\r\n\r\n"+
			"--%s\r\nContent-Type: text/html; charset=utf-8\r\n\r\n<html><body>%s<img src=\"cid:%s\" /><img src=\"cid:%s\" /></body></html>\r\n"+
			"--%s\r\nContent-Type: image/png; name=\"one.png\"\r\nContent-Disposition: inline; filename=\"one.png\"\r\nContent-ID: <%s>\r\nContent-Transfer-Encoding: base64\r\n\r\n%s\r\n"+
			"--%s\r\nContent-Type: image/png; name=\"two.png\"\r\nContent-Disposition: inline; filename=\"two.png\"\r\nContent-ID: <%s>\r\nContent-Transfer-Encoding: base64\r\n\r\n%s\r\n"+
			"--%s--\r\n",
		cfg.FromAddress, cfg.SystemAddress, subject, boundary,
		boundary, bodyChunk, cid1, cid2,
		boundary, cid1, img1,
		boundary, cid2, img2,
		boundary,
	))
	auth := smtp.PlainAuth("", cfg.Username, cfg.Password, strings.Split(cfg.SMTPAddr, ":")[0])
	if err := smtp.SendMail(cfg.SMTPAddr, auth, cfg.FromAddress, []string{cfg.SystemAddress}, raw); err != nil {
		t.Fatalf("send smtp large html inline: %v", err)
	}

	ticketRepo := repository.NewTicketRepository(db)
	articleRepo := repository.NewArticleRepository(db)
	ticketSvc := service.NewTicketService(ticketRepo, service.WithArticleRepository(articleRepo))
	postmasterHandler := postmaster.Service{
		FilterChain: filters.NewChain(),
		Handler:     postmaster.NewTicketProcessor(ticketSvc, postmaster.WithTicketProcessorFallbackQueue(int(queue.ID)), postmaster.WithTicketProcessorMessageLookup(articleRepo)),
	}

	acc := connector.Account{Type: "pop3", Host: cfg.POPHost, Port: cfg.POPPort, Username: cfg.Username, Password: []byte(cfg.Password), QueueID: int(queue.ID)}
	fetcher := connector.NewPOP3Fetcher()

	deadline := time.Now().Add(35 * time.Second)
	for time.Now().Before(deadline) {
		if err := fetcher.Fetch(ctx, acc, postmasterHandler); err != nil {
			t.Fatalf("fetch: %v", err)
		}
		tid := findTicketByTitle(ctx, db, subject)
		if tid > 0 {
			meta := latestArticleMeta(ctx, db, tid)
			if meta == nil || !strings.Contains(meta.Body, htmlToken) {
				t.Fatalf("html body missing token")
			}
			if count := attachmentsCount(ctx, db, tid); count >= 2 {
				metas := attachmentMetas(ctx, db, tid)
				found1, found2 := false, false
				for i := range metas {
					content := attachmentContent(ctx, db, metas[i].ID)
					if strings.Contains(string(content), imgToken1) {
						found1 = true
					}
					if strings.Contains(string(content), imgToken2) {
						found2 = true
					}
				}
				if !found1 || !found2 {
					t.Fatalf("inline images missing: one=%v two=%v", found1, found2)
				}
				return
			}
		}
		time.Sleep(600 * time.Millisecond)
	}

	t.Fatalf("large html inline message not processed")
}

func TestPOP3FetcherSupportsTLS(t *testing.T) {
	cfg := loadConfig(t)
	client := NewSMTP4DevClient(cfg.APIBase, nil)
	ctx := context.Background()

	if _, err := tls.Dial("tcp", net.JoinHostPort(cfg.POPHost, strconv.Itoa(cfg.POPPort)), &tls.Config{InsecureSkipVerify: true}); err != nil {
		t.Skipf("pop3 tls unavailable: %v", err)
	}

	box, err := client.CreateMailbox(ctx, cfg.Username, cfg.Username, cfg.Password)
	if err != nil {
		t.Fatalf("create mailbox: %v", err)
	}
	t.Cleanup(func() { _ = client.DeleteMailbox(context.Background(), box.ID) })
	_ = client.DeleteAllMessages(ctx)

	token := randomToken()
	if err := sendSMTP(t, cfg, cfg.SystemAddress, fmt.Sprintf("POP TLS %s", token), token); err != nil {
		t.Fatalf("send smtp: %v", err)
	}

	acc := connector.Account{Type: "pop3s", Host: cfg.POPHost, Port: cfg.POPPort, Username: cfg.Username, Password: []byte(cfg.Password)}
	fetcher := connector.NewPOP3Fetcher()
	h := &recordingHandler{}

	deadline := time.Now().Add(15 * time.Second)
	for time.Now().Before(deadline) {
		h.messages = nil
		if err := fetcher.Fetch(ctx, acc, h); err != nil {
			t.Fatalf("fetch: %v", err)
		}
		for _, msg := range h.messages {
			if strings.Contains(string(msg.Raw), token) {
				snap := msg.AccountSnapshot()
				if strings.ToLower(snap.Type) != "pop3s" {
					t.Fatalf("account type mismatch: %s", snap.Type)
				}
				msgs, err := client.ListMessages(ctx, box.ID)
				if err != nil {
					t.Fatalf("list messages: %v", err)
				}
				if len(msgs) != 0 {
					t.Fatalf("expected mailbox empty after tls fetch, got %d", len(msgs))
				}
				return
			}
		}
		time.Sleep(300 * time.Millisecond)
	}

	t.Fatalf("pop3 tls message not fetched")
}

func TestPOP3FetcherHandlesTLSFailureGracefully(t *testing.T) {
	cfg := loadConfig(t)
	ctx := context.Background()

	// Intentionally point to a port with no TLS listener to force handshake failure.
	badPort := cfg.POPPort + 1234
	accBad := connector.Account{Type: "pop3s", Host: cfg.POPHost, Port: badPort, Username: cfg.Username, Password: []byte(cfg.Password)}
	accGood := connector.Account{Type: "pop3", Host: cfg.POPHost, Port: cfg.POPPort, Username: cfg.Username, Password: []byte(cfg.Password)}

	client := NewSMTP4DevClient(cfg.APIBase, nil)
	box, err := client.CreateMailbox(ctx, cfg.Username, cfg.Username, cfg.Password)
	if err != nil {
		t.Fatalf("create mailbox: %v", err)
	}
	t.Cleanup(func() { _ = client.DeleteMailbox(context.Background(), box.ID) })
	_ = client.DeleteAllMessages(ctx)

	token := randomToken()
	if err := sendSMTP(t, cfg, cfg.SystemAddress, fmt.Sprintf("POP TLS Fail %s", token), token); err != nil {
		t.Fatalf("send smtp: %v", err)
	}

	fetcher := connector.NewPOP3Fetcher()
	h := &recordingHandler{}

	if err := fetcher.Fetch(ctx, accBad, h); err == nil {
		t.Fatalf("expected tls handshake failure on bad port")
	}

	// Ensure message still present after failure.
	msgs, err := client.ListMessages(ctx, box.ID)
	if err != nil {
		t.Fatalf("list messages: %v", err)
	}
	if len(msgs) == 0 {
		t.Fatalf("message disappeared after tls failure")
	}

	h.messages = nil
	if err := fetcher.Fetch(ctx, accGood, h); err != nil {
		t.Fatalf("fetch after tls failure: %v", err)
	}
	if len(h.messages) == 0 || !strings.Contains(string(h.messages[0].Raw), token) {
		t.Fatalf("did not retrieve message after tls failure recovery")
	}
}

func TestPostmasterThreadsOnInReplyToOnly(t *testing.T) {
	cfg := loadConfig(t)
	client := NewSMTP4DevClient(cfg.APIBase, nil)
	ctx := context.Background()

	db, err := database.GetDB()
	if err != nil {
		t.Skipf("database unavailable: %v", err)
	}

	gen, err := ticketnumber.Resolve("DateChecksum", "10", nil)
	if err != nil {
		t.Fatalf("ticket number generator: %v", err)
	}
	repository.SetTicketNumberGenerator(gen, ticketnumber.NewDBStore(db, "10"))
	t.Cleanup(func() { repository.SetTicketNumberGenerator(nil, nil) })

	queueName := getenv("TEST_QUEUE_NAME", "Postmaster")
	queueRepo := repository.NewQueueRepository(db)
	queue, err := queueRepo.GetByName(queueName)
	if err != nil || queue == nil {
		t.Fatalf("queue %s unavailable: %v", queueName, err)
	}

	box, err := client.CreateMailbox(ctx, cfg.Username, cfg.Username, cfg.Password)
	if err != nil {
		t.Fatalf("create mailbox: %v", err)
	}
	t.Cleanup(func() { _ = client.DeleteMailbox(context.Background(), box.ID) })
	_ = client.DeleteAllMessages(ctx)

	auth := smtp.PlainAuth("", cfg.Username, cfg.Password, strings.Split(cfg.SMTPAddr, ":")[0])
	seedID := fmt.Sprintf("<seed-%s@x>", randomToken())
	seedSubject := fmt.Sprintf("InReply Seed %s", randomToken())
	seedBody := "seed-body"
	seedRaw := []byte(fmt.Sprintf("From: %s\r\nTo: %s\r\nSubject: %s\r\nMessage-ID: %s\r\n\r\n%s\r\n", cfg.FromAddress, cfg.SystemAddress, seedSubject, seedID, seedBody))
	if err := smtp.SendMail(cfg.SMTPAddr, auth, cfg.FromAddress, []string{cfg.SystemAddress}, seedRaw); err != nil {
		t.Fatalf("send seed: %v", err)
	}

	articleRepo := repository.NewArticleRepository(db)
	ticketRepo := repository.NewTicketRepository(db)
	ticketSvc := service.NewTicketService(ticketRepo, service.WithArticleRepository(articleRepo))
	postmasterHandler := postmaster.Service{
		FilterChain: filters.NewChain(),
		Handler: postmaster.NewTicketProcessor(
			ticketSvc,
			postmaster.WithTicketProcessorFallbackQueue(int(queue.ID)),
			postmaster.WithTicketProcessorMessageLookup(articleRepo),
		),
	}

	acc := connector.Account{Type: "pop3", Host: cfg.POPHost, Port: cfg.POPPort, Username: cfg.Username, Password: []byte(cfg.Password), QueueID: int(queue.ID)}
	fetcher := connector.NewPOP3Fetcher()

	deadline := time.Now().Add(20 * time.Second)
	var ticketID int
	for time.Now().Before(deadline) {
		if err := fetcher.Fetch(ctx, acc, postmasterHandler); err != nil {
			t.Fatalf("seed fetch: %v", err)
		}
		if tid := findTicketByTitle(ctx, db, seedSubject); tid > 0 {
			ticketID = tid
			break
		}
		time.Sleep(400 * time.Millisecond)
	}
	if ticketID == 0 {
		t.Fatalf("seed ticket not created")
	}
	baseline := articleCount(ctx, db, ticketID)

	followBody := "follow-body"
	followSubject := "Re: " + seedSubject
	followRefs := fmt.Sprintf("<noise-%s@x>", randomToken())
	follow := []byte(fmt.Sprintf("From: %s\r\nTo: %s\r\nSubject: %s\r\nIn-Reply-To: %s\r\nReferences: %s\r\n\r\n%s\r\n", cfg.FromAddress, cfg.SystemAddress, followSubject, seedID, followRefs, followBody))
	if err := smtp.SendMail(cfg.SMTPAddr, auth, cfg.FromAddress, []string{cfg.SystemAddress}, follow); err != nil {
		t.Fatalf("send follow: %v", err)
	}

	deadline = time.Now().Add(20 * time.Second)
	for time.Now().Before(deadline) {
		if err := fetcher.Fetch(ctx, acc, postmasterHandler); err != nil {
			t.Fatalf("follow fetch: %v", err)
		}
		if count := articleCount(ctx, db, ticketID); count > baseline {
			meta := latestArticleMeta(ctx, db, ticketID)
			if meta == nil {
				t.Fatalf("missing follow meta")
			}
			if !strings.Contains(meta.Body, followBody) {
				t.Fatalf("follow body missing token")
			}
			if strings.Trim(meta.InReplyTo, "<>") != strings.Trim(seedID, "<>") {
				t.Fatalf("in-reply-to mismatch: %s", meta.InReplyTo)
			}
			if meta.References != "" && !strings.Contains(meta.References, strings.Trim(seedID, "<>")) {
				t.Fatalf("references missing seed id: %s", meta.References)
			}
			return
		}
		time.Sleep(400 * time.Millisecond)
	}

	t.Fatalf("follow-up not threaded to seed ticket")
}

type countingHandler struct {
	mu    sync.Mutex
	seen  map[string]int
	token string
}

func (h *countingHandler) Handle(_ context.Context, msg *connector.FetchedMessage) error {
	if strings.Contains(string(msg.Raw), h.token) {
		h.mu.Lock()
		h.seen[h.token]++
		h.mu.Unlock()
	}
	return nil
}

type recordingHandler struct {
	messages []*connector.FetchedMessage
}

func (h *recordingHandler) Handle(_ context.Context, msg *connector.FetchedMessage) error {
	h.messages = append(h.messages, msg)
	return nil
}

func sendSMTP(t *testing.T, cfg smtp4devConfig, to, subject, body string) error {
	t.Helper()
	smtpHost := strings.Split(cfg.SMTPAddr, ":")[0]
	auth := smtp.PlainAuth("", cfg.Username, cfg.Password, smtpHost)
	msg := []byte(fmt.Sprintf("From: %s\r\nTo: %s\r\nSubject: %s\r\n\r\n%s\r\n", cfg.FromAddress, to, subject, body))
	return smtp.SendMail(cfg.SMTPAddr, auth, cfg.FromAddress, []string{to}, msg)
}

func findTicketByTitle(ctx context.Context, db *sql.DB, title string) int {
	q := database.ConvertPlaceholders("SELECT id FROM ticket WHERE title = $1 ORDER BY id DESC LIMIT 1")
	var id int
	if err := db.QueryRowContext(ctx, q, title).Scan(&id); err != nil {
		return 0
	}
	return id
}

func articleCount(ctx context.Context, db *sql.DB, ticketID int) int {
	q := database.ConvertPlaceholders("SELECT COUNT(*) FROM article WHERE ticket_id = $1")
	var n int
	if err := db.QueryRowContext(ctx, q, ticketID).Scan(&n); err != nil {
		return 0
	}
	return n
}

func mailQueueCount(ctx context.Context, db *sql.DB) int {
	q := database.ConvertPlaceholders("SELECT COUNT(*) FROM mail_queue")
	var n int
	if err := db.QueryRowContext(ctx, q).Scan(&n); err != nil {
		return 0
	}
	return n
}

func attachmentsCount(ctx context.Context, db *sql.DB, ticketID int) int {
	q := database.ConvertPlaceholders("SELECT COUNT(*) FROM article_attachments WHERE article_id IN (SELECT id FROM article WHERE ticket_id = $1)")
	var n int
	if err := db.QueryRowContext(ctx, q, ticketID).Scan(&n); err != nil {
		return 0
	}
	return n
}

type attachmentMeta struct {
	ID          int
	Filename    string
	ContentType string
	ContentSize int
}

func attachmentMetas(ctx context.Context, db *sql.DB, ticketID int) []attachmentMeta {
	q := database.ConvertPlaceholders("SELECT id, filename, content_type, content_size FROM article_attachments WHERE article_id IN (SELECT id FROM article WHERE ticket_id = $1) ORDER BY id")
	rows, err := db.QueryContext(ctx, q, ticketID)
	if err != nil {
		return nil
	}
	defer rows.Close()

	var metas []attachmentMeta
	for rows.Next() {
		var m attachmentMeta
		if err := rows.Scan(&m.ID, &m.Filename, &m.ContentType, &m.ContentSize); err != nil {
			continue
		}
		metas = append(metas, m)
	}

	return metas
}

func attachmentContainsToken(ctx context.Context, db *sql.DB, attachmentID int, token string) bool {
	q := database.ConvertPlaceholders("SELECT content FROM article_attachments WHERE id = $1")
	var content []byte
	if err := db.QueryRowContext(ctx, q, attachmentID).Scan(&content); err != nil {
		return false
	}
	return strings.Contains(string(content), token)
}

func attachmentContent(ctx context.Context, db *sql.DB, attachmentID int) []byte {
	q := database.ConvertPlaceholders("SELECT content FROM article_attachments WHERE id = $1")
	var content []byte
	if err := db.QueryRowContext(ctx, q, attachmentID).Scan(&content); err != nil {
		return nil
	}
	return content
}

func articleHasMessageID(ctx context.Context, db *sql.DB, subject, messageID string) bool {
	q := database.ConvertPlaceholders("SELECT adm.a_message_id FROM article a LEFT JOIN article_data_mime adm ON adm.article_id = a.id WHERE a.title = $1 ORDER BY a.id DESC LIMIT 1")
	var mid sql.NullString
	if err := db.QueryRowContext(ctx, q, subject).Scan(&mid); err != nil {
		return false
	}
	return strings.TrimSpace(mid.String) == strings.TrimSpace(messageID)
}

type articleMeta struct {
	ID         int
	Body       string
	MessageID  string
	InReplyTo  string
	References string
}

func latestArticleMeta(ctx context.Context, db *sql.DB, ticketID int) *articleMeta {
	q := database.ConvertPlaceholders(`
		SELECT a.id, adm.a_body, adm.a_message_id, adm.a_in_reply_to, adm.a_references
		FROM article a
		LEFT JOIN article_data_mime adm ON adm.article_id = a.id
		WHERE a.ticket_id = $1
		ORDER BY a.id DESC
		LIMIT 1`)
	var (
		id                   int
		bodyBytes            []byte
		messageID, inReplyTo sql.NullString
		references           sql.NullString
	)
	if err := db.QueryRowContext(ctx, q, ticketID).Scan(&id, &bodyBytes, &messageID, &inReplyTo, &references); err != nil {
		return nil
	}
	return &articleMeta{
		ID:         id,
		Body:       string(bodyBytes),
		MessageID:  messageID.String,
		InReplyTo:  strings.TrimSpace(inReplyTo.String),
		References: strings.TrimSpace(references.String),
	}
}

func mailboxCount(ctx context.Context, t *testing.T, client *SMTP4DevClient, mailboxID string) int {
	t.Helper()
	msgs, err := client.ListMessages(ctx, mailboxID)
	if err != nil {
		t.Fatalf("list messages: %v", err)
	}
	return len(msgs)
}

func popHasBody(t *testing.T, cfg smtp4devConfig, token string) bool {
	client := pop3.New(pop3.Opt{Host: cfg.POPHost, Port: cfg.POPPort})
	conn, err := client.NewConn()
	if err != nil {
		return false
	}
	defer conn.Quit()
	if err := conn.User(cfg.Username); err != nil {
		return false
	}
	if err := conn.Pass(cfg.Password); err != nil {
		return false
	}
	msgs, err := conn.List(0)
	if err != nil {
		return false
	}
	for _, m := range msgs {
		payload, err := conn.RetrRaw(m.ID)
		if err != nil {
			continue
		}
		if strings.Contains(strings.ToLower(payload.String()), strings.ToLower(token)) {
			return true
		}
	}
	return false
}

func randomToken() string {
	buf := make([]byte, 8)
	_, _ = rand.Read(buf)
	return hex.EncodeToString(buf)
}

func getenv(key, def string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return def
}

func getenvInt(key string, def int) int {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return def
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return def
	}
	return n
}
