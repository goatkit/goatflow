package postmaster

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"io"
	"log"
	"mime/multipart"
	"strings"
	"testing"
	"time"

	sqlmock "github.com/DATA-DOG/go-sqlmock"

	"github.com/gotrs-io/gotrs-ce/internal/constants"
	"github.com/gotrs-io/gotrs-ce/internal/core"
	"github.com/gotrs-io/gotrs-ce/internal/email/inbound/connector"
	"github.com/gotrs-io/gotrs-ce/internal/email/inbound/filters"
	"github.com/gotrs-io/gotrs-ce/internal/models"
	"github.com/gotrs-io/gotrs-ce/internal/service"
)

func TestTicketProcessorCreatesTicket(t *testing.T) {
	fake := &recordingTicketService{}
	processor := NewTicketProcessor(fake, WithTicketProcessorSystemUser(5))
	msg := &connector.FetchedMessage{
		Raw: []byte("Subject: Hello World\r\nFrom: Jane <jane@example.com>\r\n\r\nBody line"),
	}
	msg.WithAccount(connector.Account{QueueID: 4})
	res, err := processor.Process(context.Background(), msg, nil)
	if err != nil {
		t.Fatalf("Process returned error: %v", err)
	}
	if res.TicketID != 99 {
		t.Fatalf("expected ticket id 99, got %d", res.TicketID)
	}
	if fake.input.QueueID != 4 {
		t.Fatalf("expected queue id 4, got %d", fake.input.QueueID)
	}
	if fake.input.UserID != 5 {
		t.Fatalf("expected system user 5, got %d", fake.input.UserID)
	}
	if fake.input.Title != "Hello World" {
		t.Fatalf("expected subject preserved, got %q", fake.input.Title)
	}
	if fake.input.CustomerUserID != "jane@example.com" {
		t.Fatalf("expected customer email, got %q", fake.input.CustomerUserID)
	}
	if fake.input.CustomerID != "example.com" {
		t.Fatalf("expected customer id example.com, got %q", fake.input.CustomerID)
	}
	if fake.input.ArticleSubject != "Hello World" {
		t.Fatalf("expected article subject to match, got %q", fake.input.ArticleSubject)
	}
	if fake.input.ArticleSenderTypeID != constants.ArticleSenderCustomer {
		t.Fatalf("expected customer sender type, got %d", fake.input.ArticleSenderTypeID)
	}
	if fake.input.ArticleTypeID != constants.ArticleTypeEmailExternal {
		t.Fatalf("expected email external article type, got %d", fake.input.ArticleTypeID)
	}
	if fake.input.ArticleIsVisibleForCustomer == nil || !*fake.input.ArticleIsVisibleForCustomer {
		t.Fatalf("expected customer-visible article flag")
	}
	if fake.input.Body != "Body line" {
		t.Fatalf("expected body propagated, got %q", fake.input.Body)
	}
	if fake.input.ArticleMimeType != "text/plain" {
		t.Fatalf("expected default mime type, got %q", fake.input.ArticleMimeType)
	}
	if fake.input.ArticleCharset != "utf-8" {
		t.Fatalf("expected default charset, got %q", fake.input.ArticleCharset)
	}
	if fake.input.ArticleCommunicationChannelID != core.MapCommunicationChannel(constants.ArticleTypeEmailExternal) {
		t.Fatalf("expected email communication channel, got %d", fake.input.ArticleCommunicationChannelID)
	}
}

func TestTicketProcessorCapturesContentType(t *testing.T) {
	fake := &recordingTicketService{}
	processor := NewTicketProcessor(fake)
	msg := &connector.FetchedMessage{Raw: []byte("Subject: HTML\r\nFrom: Jane <jane@example.com>\r\nContent-Type: text/html; charset=ISO-8859-1\r\n\r\n<body>Hi</body>")}
	msg.WithAccount(connector.Account{QueueID: 4})
	if _, err := processor.Process(context.Background(), msg, nil); err != nil {
		t.Fatalf("Process returned error: %v", err)
	}
	if fake.input.ArticleMimeType != "text/html" {
		t.Fatalf("expected html mime type, got %q", fake.input.ArticleMimeType)
	}
	if fake.input.ArticleCharset != "iso-8859-1" {
		t.Fatalf("expected charset from header, got %q", fake.input.ArticleCharset)
	}
}

func TestTicketProcessorPrefersPlainTextInlinePart(t *testing.T) {
	fake := &recordingTicketService{}
	processor := NewTicketProcessor(fake)
	raw := strings.Join([]string{
		"Subject: Alt",
		"From: Jane <jane@example.com>",
		"MIME-Version: 1.0",
		"Content-Type: multipart/alternative; boundary=\"XYZ\"",
		"",
		"--XYZ",
		"Content-Type: text/html; charset=UTF-8",
		"",
		"<p>HTML</p>",
		"--XYZ",
		"Content-Type: text/plain; charset=ISO-8859-1",
		"",
		"Plain text body",
		"--XYZ--",
	}, "\r\n")
	msg := &connector.FetchedMessage{Raw: []byte(raw)}
	msg.WithAccount(connector.Account{QueueID: 5})
	if _, err := processor.Process(context.Background(), msg, nil); err != nil {
		t.Fatalf("Process returned error: %v", err)
	}
	if fake.input.ArticleMimeType != "text/plain" {
		t.Fatalf("expected plain mime type, got %q", fake.input.ArticleMimeType)
	}
	if fake.input.ArticleCharset != "iso-8859-1" {
		t.Fatalf("expected charset from plain part, got %q", fake.input.ArticleCharset)
	}
	if fake.input.Body != "Plain text body" {
		t.Fatalf("expected plain text body, got %q", fake.input.Body)
	}
}

func TestTicketProcessorFallsBackToHTMLPart(t *testing.T) {
	fake := &recordingTicketService{}
	processor := NewTicketProcessor(fake)
	raw := strings.Join([]string{
		"Subject: HTML Only",
		"From: Jane <jane@example.com>",
		"MIME-Version: 1.0",
		"Content-Type: multipart/alternative; boundary=\"XYZ\"",
		"",
		"--XYZ",
		"Content-Type: text/html; charset=UTF-8",
		"",
		"<p>Hello</p>",
		"--XYZ--",
	}, "\r\n")
	msg := &connector.FetchedMessage{Raw: []byte(raw)}
	msg.WithAccount(connector.Account{QueueID: 5})
	if _, err := processor.Process(context.Background(), msg, nil); err != nil {
		t.Fatalf("Process returned error: %v", err)
	}
	if fake.input.ArticleMimeType != "text/html" {
		t.Fatalf("expected html mime type, got %q", fake.input.ArticleMimeType)
	}
	if fake.input.ArticleCharset != "utf-8" {
		t.Fatalf("expected utf-8 charset, got %q", fake.input.ArticleCharset)
	}
	if fake.input.Body != "<p>Hello</p>" {
		t.Fatalf("expected html body, got %q", fake.input.Body)
	}
}

func TestTicketProcessorStoresAttachments(t *testing.T) {
	fake := &recordingTicketService{}
	storage := &recordingStorage{}
	articles := &stubArticleRepo{articleID: 555}
	processor := NewTicketProcessor(
		fake,
		WithTicketProcessorStorage(storage),
		WithTicketProcessorArticleLookup(articles),
	)
	payload := base64.StdEncoding.EncodeToString([]byte("Attachment body"))
	raw := strings.Join([]string{
		"Subject: Files",
		"From: Jane <jane@example.com>",
		"MIME-Version: 1.0",
		"Content-Type: multipart/mixed; boundary=XYZ",
		"",
		"--XYZ",
		"Content-Type: text/plain; charset=UTF-8",
		"",
		"Message",
		"--XYZ",
		"Content-Type: text/plain",
		"Content-Disposition: attachment; filename=note.txt",
		"Content-Transfer-Encoding: base64",
		"",
		payload,
		"--XYZ--",
	}, "\r\n")
	msg := &connector.FetchedMessage{Raw: []byte(raw)}
	msg.WithAccount(connector.Account{QueueID: 4})
	if _, err := processor.Process(context.Background(), msg, nil); err != nil {
		t.Fatalf("Process returned error: %v", err)
	}
	if len(storage.files) != 1 {
		t.Fatalf("expected 1 stored attachment, got %d", len(storage.files))
	}
	if storage.files[0].filename != "note.txt" {
		t.Fatalf("expected filename note.txt, got %s", storage.files[0].filename)
	}
	if got := string(storage.files[0].data); got != "Attachment body" {
		t.Fatalf("unexpected attachment body: %q", got)
	}
}

func TestTicketProcessorFallbackQueue(t *testing.T) {
	fake := &recordingTicketService{}
	processor := NewTicketProcessor(fake, WithTicketProcessorFallbackQueue(9))
	msg := &connector.FetchedMessage{Raw: []byte("Subject: Hi\r\n\r\nbody")}
	msg.WithAccount(connector.Account{})
	if _, err := processor.Process(context.Background(), msg, nil); err != nil {
		t.Fatalf("Process returned error: %v", err)
	}
	if fake.input.QueueID != 9 {
		t.Fatalf("expected fallback queue 9, got %d", fake.input.QueueID)
	}
}

func TestTicketProcessorDefaultSubject(t *testing.T) {
	fake := &recordingTicketService{}
	processor := NewTicketProcessor(fake)
	msg := &connector.FetchedMessage{Raw: []byte("From: foo@example.com\r\n\r\nBody"), RemoteID: "abc"}
	msg.WithAccount(connector.Account{QueueID: 2})
	if _, err := processor.Process(context.Background(), msg, nil); err != nil {
		t.Fatalf("Process returned error: %v", err)
	}
	if fake.input.Title != "Inbound email abc" {
		t.Fatalf("expected fallback subject, got %q", fake.input.Title)
	}
}

func TestTicketProcessorPropagatesError(t *testing.T) {
	fake := &recordingTicketService{err: errors.New("boom")}
	processor := NewTicketProcessor(fake)
	msg := &connector.FetchedMessage{Raw: []byte("Subject: Hi\r\n\r\nBody")}
	msg.WithAccount(connector.Account{QueueID: 1})
	if _, err := processor.Process(context.Background(), msg, nil); err == nil {
		t.Fatalf("expected error when ticket service fails")
	}
}

func TestTicketProcessorQueueOverrideViaAnnotations(t *testing.T) {
	fake := &recordingTicketService{}
	processor := NewTicketProcessor(fake)
	msg := &connector.FetchedMessage{Raw: []byte("Subject: Hi\r\n\r\nBody")}
	msg.WithAccount(connector.Account{QueueID: 2})
	meta := &filters.MessageContext{Annotations: map[string]any{filters.AnnotationQueueIDOverride: 8}}
	if _, err := processor.Process(context.Background(), msg, meta); err != nil {
		t.Fatalf("Process returned error: %v", err)
	}
	if fake.input.QueueID != 8 {
		t.Fatalf("expected queue override 8, got %d", fake.input.QueueID)
	}
}

func TestTicketProcessorQueueLookupByName(t *testing.T) {
	fake := &recordingTicketService{}
	processor := NewTicketProcessor(fake, WithTicketProcessorQueueLookup(func(ctx context.Context, name string) (int, error) {
		if name == "VIP" {
			return 42, nil
		}
		return 0, errors.New("unknown queue")
	}))
	msg := &connector.FetchedMessage{Raw: []byte("Subject: Hi\r\n\r\nBody")}
	msg.WithAccount(connector.Account{QueueID: 0})
	meta := &filters.MessageContext{Annotations: map[string]any{filters.AnnotationQueueNameOverride: "VIP"}}
	if _, err := processor.Process(context.Background(), msg, meta); err != nil {
		t.Fatalf("Process returned error: %v", err)
	}
	if fake.input.QueueID != 42 {
		t.Fatalf("expected queue lookup id 42, got %d", fake.input.QueueID)
	}
}

func TestTicketProcessorPriorityAndTitleOverride(t *testing.T) {
	fake := &recordingTicketService{}
	processor := NewTicketProcessor(fake)
	msg := &connector.FetchedMessage{Raw: []byte("Subject: Old\r\n\r\nBody")}
	msg.WithAccount(connector.Account{QueueID: 7})
	meta := &filters.MessageContext{Annotations: map[string]any{
		filters.AnnotationPriorityIDOverride: 5,
		filters.AnnotationTitleOverride:      "NewTitle",
		filters.AnnotationCustomerIDOverride: "cust-99",
	}}
	if _, err := processor.Process(context.Background(), msg, meta); err != nil {
		t.Fatalf("Process returned error: %v", err)
	}
	if fake.input.PriorityID != 5 {
		t.Fatalf("expected priority 5, got %d", fake.input.PriorityID)
	}
	if fake.input.Title != "NewTitle" {
		t.Fatalf("expected title override, got %q", fake.input.Title)
	}
	if fake.input.CustomerID != "cust-99" {
		t.Fatalf("expected customer override, got %q", fake.input.CustomerID)
	}
}

func TestTicketProcessorSkipsIgnoredMessages(t *testing.T) {
	fake := &recordingTicketService{}
	processor := NewTicketProcessor(fake)
	msg := &connector.FetchedMessage{Raw: []byte("Subject: Ignore\r\n\r\nbody"), UID: "uid-9"}
	msg.WithAccount(connector.Account{QueueID: 3})
	meta := &filters.MessageContext{Annotations: map[string]any{filters.AnnotationIgnoreMessage: true}}
	res, err := processor.Process(context.Background(), msg, meta)
	if err != nil {
		t.Fatalf("Process returned error: %v", err)
	}
	if res.Action != "ignored" {
		t.Fatalf("expected ignored action, got %q", res.Action)
	}
	if fake.calls != 0 {
		t.Fatalf("expected ticket service not invoked, got %d calls", fake.calls)
	}
}

func TestTicketProcessorFollowUpAppendsArticle(t *testing.T) {
	service := &recordingTicketService{}
	ticketLookup := &stubTicketFinder{ticket: &models.Ticket{ID: 77, QueueID: 5}}
	queueLookup := &stubQueueFinder{queue: &models.Queue{ID: 5, FollowUpID: 1}}
	articleStore := &recordingArticleStore{}
	processor := NewTicketProcessor(
		service,
		WithTicketProcessorTicketFinder(ticketLookup),
		WithTicketProcessorQueueFinder(queueLookup),
		WithTicketProcessorArticleStore(articleStore),
	)
	msg := &connector.FetchedMessage{Raw: []byte("Subject: Re: [Ticket#77]\r\n\r\nBody")}
	meta := &filters.MessageContext{Annotations: map[string]any{filters.AnnotationFollowUpTicketNumber: "202500077"}}
	res, err := processor.Process(context.Background(), msg, meta)
	if err != nil {
		t.Fatalf("Process returned error: %v", err)
	}
	if res.Action != "follow_up" {
		t.Fatalf("expected follow_up action, got %s", res.Action)
	}
	if service.calls != 0 {
		t.Fatalf("expected ticket service skipped, got %d calls", service.calls)
	}
	if articleStore.calls != 1 {
		t.Fatalf("expected 1 article store call, got %d", articleStore.calls)
	}
	if articleStore.last == nil || articleStore.last.TicketID != 77 {
		t.Fatalf("expected article bound to ticket 77, got %+v", articleStore.last)
	}
	if res.TicketID != 77 {
		t.Fatalf("expected returned ticket id 77, got %d", res.TicketID)
	}
	if res.ArticleID == 0 {
		t.Fatalf("expected article id in result")
	}
}

func TestTicketProcessorFollowUpHonorsQueuePolicy(t *testing.T) {
	service := &recordingTicketService{}
	ticketLookup := &stubTicketFinder{ticket: &models.Ticket{ID: 11, QueueID: 9}}
	queueLookup := &stubQueueFinder{queue: &models.Queue{ID: 9, FollowUpID: 2}}
	articleStore := &recordingArticleStore{}
	processor := NewTicketProcessor(
		service,
		WithTicketProcessorTicketFinder(ticketLookup),
		WithTicketProcessorQueueFinder(queueLookup),
		WithTicketProcessorArticleStore(articleStore),
	)
	msg := &connector.FetchedMessage{Raw: []byte("Subject: Rejected\r\n\r\nBody")}
	msg.WithAccount(connector.Account{QueueID: 3})
	meta := &filters.MessageContext{Annotations: map[string]any{filters.AnnotationFollowUpTicketNumber: "2001"}}
	res, err := processor.Process(context.Background(), msg, meta)
	if err != nil {
		t.Fatalf("Process returned error: %v", err)
	}
	if res.Action != "new_ticket" {
		t.Fatalf("expected new_ticket action when policy rejects follow-up, got %s", res.Action)
	}
	if service.calls != 1 {
		t.Fatalf("expected ticket service called once, got %d", service.calls)
	}
	if articleStore.calls != 0 {
		t.Fatalf("expected article store unused, got %d calls", articleStore.calls)
	}
}

func TestTicketProcessorFollowUpViaReferences(t *testing.T) {
	service := &recordingTicketService{}
	queueLookup := &stubQueueFinder{queue: &models.Queue{ID: 4, FollowUpID: 1}}
	articleStore := &recordingArticleStore{}
	resolver := &stubMessageResolver{responses: map[string]*models.Ticket{
		"msg-2@example.com": {ID: 91, QueueID: 4},
	}}
	processor := NewTicketProcessor(
		service,
		WithTicketProcessorQueueFinder(queueLookup),
		WithTicketProcessorArticleStore(articleStore),
		WithTicketProcessorMessageLookup(resolver),
	)
	raw := strings.Join([]string{
		"Subject: Re: Update",
		"References: <msg-1@example.com> <msg-2@example.com>",
		"In-Reply-To: <reply@example.com>",
		"",
		"Body",
	}, "\r\n")
	msg := &connector.FetchedMessage{Raw: []byte(raw)}
	msg.WithAccount(connector.Account{QueueID: 4})
	res, err := processor.Process(context.Background(), msg, nil)
	if err != nil {
		t.Fatalf("Process returned error: %v", err)
	}
	if res.Action != "follow_up" {
		t.Fatalf("expected follow_up action, got %s", res.Action)
	}
	if res.TicketID != 91 {
		t.Fatalf("expected ticket 91, got %d", res.TicketID)
	}
	if articleStore.calls != 1 {
		t.Fatalf("expected article store called once, got %d", articleStore.calls)
	}
	if len(resolver.calls) < 2 {
		t.Fatalf("expected resolver to be tried twice, got %d", len(resolver.calls))
	}
	if resolver.calls[0] != "reply@example.com" {
		t.Fatalf("expected In-Reply-To checked first, got %s", resolver.calls[0])
	}
	if resolver.calls[1] != "msg-2@example.com" {
		t.Fatalf("expected references checked next, got %s", resolver.calls[1])
	}
}

func TestTicketProcessorRequiresMessage(t *testing.T) {
	processor := NewTicketProcessor(&recordingTicketService{})
	if _, err := processor.Process(context.Background(), nil, nil); err == nil {
		t.Fatalf("expected error for missing message")
	}
}

func TestTicketProcessorMissingQueueRouting(t *testing.T) {
	svc := &recordingTicketService{}
	processor := NewTicketProcessor(svc)
	processor.fallbackQueueID = 0
	msg := &connector.FetchedMessage{Raw: []byte("Subject: Hi\r\n\r\nBody")}
	msg.WithAccount(connector.Account{})
	if _, err := processor.Process(context.Background(), msg, nil); err == nil {
		t.Fatalf("expected error when routing unavailable")
	}
	if svc.calls != 0 {
		t.Fatalf("expected ticket service not invoked")
	}
}

func TestTicketProcessorQueueLookupErrorLogged(t *testing.T) {
	var buf bytes.Buffer
	svc := &recordingTicketService{}
	processor := NewTicketProcessor(
		svc,
		WithTicketProcessorLogger(log.New(&buf, "", 0)),
		WithTicketProcessorQueueLookup(func(ctx context.Context, name string) (int, error) {
			return 0, errors.New("missing")
		}),
	)
	msg := &connector.FetchedMessage{Raw: []byte("Subject: Hi\r\n\r\nBody")}
	msg.WithAccount(connector.Account{QueueID: 3})
	meta := &filters.MessageContext{Annotations: map[string]any{filters.AnnotationQueueNameOverride: "VIP"}}
	if _, err := processor.Process(context.Background(), msg, meta); err != nil {
		t.Fatalf("Process returned error: %v", err)
	}
	if !strings.Contains(buf.String(), "queue lookup failed") {
		t.Fatalf("expected queue lookup failure logged, got %q", buf.String())
	}
	if svc.input.QueueID != 3 {
		t.Fatalf("expected account queue fallback, got %d", svc.input.QueueID)
	}
}

func TestTicketProcessorFollowUpLookupErrorFallsBackToNew(t *testing.T) {
	svc := &recordingTicketService{}
	articleStore := &recordingArticleStore{}
	processor := NewTicketProcessor(
		svc,
		WithTicketProcessorTicketFinder(&stubTicketFinder{err: errors.New("boom")}),
		WithTicketProcessorArticleStore(articleStore),
		WithTicketProcessorLogger(log.New(io.Discard, "", 0)),
	)
	msg := &connector.FetchedMessage{Raw: []byte("Subject: Re: X\r\n\r\nBody")}
	msg.WithAccount(connector.Account{QueueID: 2})
	meta := &filters.MessageContext{Annotations: map[string]any{filters.AnnotationFollowUpTicketNumber: "X"}}
	res, err := processor.Process(context.Background(), msg, meta)
	if err != nil {
		t.Fatalf("Process returned error: %v", err)
	}
	if res.Action != "new_ticket" {
		t.Fatalf("expected new_ticket action, got %s", res.Action)
	}
	if svc.calls != 1 {
		t.Fatalf("expected ticket creation attempted, got %d", svc.calls)
	}
	if articleStore.calls != 0 {
		t.Fatalf("expected no follow-up article stored")
	}
}

func TestTicketProcessorReferencesLookupErrorFallsBackToNew(t *testing.T) {
	svc := &recordingTicketService{}
	articleStore := &recordingArticleStore{}
	processor := NewTicketProcessor(
		svc,
		WithTicketProcessorMessageLookup(&stubMessageResolver{err: errors.New("fail")}),
		WithTicketProcessorArticleStore(articleStore),
		WithTicketProcessorLogger(log.New(io.Discard, "", 0)),
	)
	raw := "Subject: Re: Update\r\nIn-Reply-To: <msg@example.com>\r\n\r\nBody"
	msg := &connector.FetchedMessage{Raw: []byte(raw)}
	msg.WithAccount(connector.Account{QueueID: 2})
	res, err := processor.Process(context.Background(), msg, nil)
	if err != nil {
		t.Fatalf("Process returned error: %v", err)
	}
	if res.Action != "new_ticket" {
		t.Fatalf("expected new_ticket action, got %s", res.Action)
	}
	if svc.calls != 1 {
		t.Fatalf("expected ticket creation attempted, got %d", svc.calls)
	}
	if articleStore.calls != 0 {
		t.Fatalf("expected no follow-up article stored")
	}
}

func TestTicketProcessorFollowUpQueueLookupErrorStillAllows(t *testing.T) {
	svc := &recordingTicketService{}
	queueLookup := &stubQueueFinder{err: errors.New("db down")}
	articleStore := &recordingArticleStore{}
	processor := NewTicketProcessor(
		svc,
		WithTicketProcessorTicketFinder(&stubTicketFinder{ticket: &models.Ticket{ID: 50, QueueID: 7}}),
		WithTicketProcessorQueueFinder(queueLookup),
		WithTicketProcessorArticleStore(articleStore),
	)
	msg := &connector.FetchedMessage{Raw: []byte("Subject: Follow\r\n\r\nBody")}
	msg.WithAccount(connector.Account{QueueID: 7})
	meta := &filters.MessageContext{Annotations: map[string]any{filters.AnnotationFollowUpTicketNumber: "50"}}
	res, err := processor.Process(context.Background(), msg, meta)
	if err != nil {
		t.Fatalf("Process returned error: %v", err)
	}
	if res.Action != "follow_up" {
		t.Fatalf("expected follow_up action, got %s", res.Action)
	}
	if articleStore.calls != 1 {
		t.Fatalf("expected follow-up article stored, got %d", articleStore.calls)
	}
	if svc.calls != 0 {
		t.Fatalf("expected ticket creation skipped")
	}
}

func TestStoreAttachmentsSkipsWhenStorageMissing(t *testing.T) {
	processor := NewTicketProcessor(&recordingTicketService{})
	attachments := []attachmentPart{{filename: "a.txt", data: []byte("x")}}
	processor.storeAttachments(context.Background(), 1, 1, attachments)
}

func TestStoreAttachmentSkipsEmptyData(t *testing.T) {
	storage := &recordingStorage{}
	processor := NewTicketProcessor(&recordingTicketService{}, WithTicketProcessorStorage(storage))
	processor.storeAttachment(context.Background(), 1, 1, attachmentPart{filename: "file.txt"})
	if len(storage.files) != 0 {
		t.Fatalf("expected no stored files, got %d", len(storage.files))
	}
}

func TestStorageIsDBDetection(t *testing.T) {
	processor := NewTicketProcessor(&recordingTicketService{})
	processor.storage = &service.DatabaseStorageService{}
	if !processor.storageIsDB() {
		t.Fatalf("expected database storage detection")
	}
}

func TestInsertAttachmentRecordUsesProvidedDB(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock setup failed: %v", err)
	}
	defer db.Close()
	processor := NewTicketProcessor(&recordingTicketService{})
	processor.db = db
	att := attachmentPart{filename: "file.bin", data: []byte("abc")}
	mock.ExpectExec("INSERT INTO article_data_mime_attachment").
		WithArgs(100, "file.bin", "application/octet-stream", int64(len(att.data)), att.data, "attachment", sqlmock.AnyArg(), processor.systemUserID, sqlmock.AnyArg(), processor.systemUserID).
		WillReturnResult(sqlmock.NewResult(1, 1))
	if err := processor.insertAttachmentRecord(100, att); err != nil {
		t.Fatalf("insertAttachmentRecord returned error: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("sqlmock expectations not met: %v", err)
	}
}

type recordingTicketService struct {
	input service.CreateTicketInput
	err   error
	calls int
}

func (r *recordingTicketService) Create(ctx context.Context, in service.CreateTicketInput) (*models.Ticket, error) {
	r.calls++
	r.input = in
	if r.err != nil {
		return nil, r.err
	}
	return &models.Ticket{ID: 99, TicketNumber: "2025010100001"}, nil
}

type stubArticleRepo struct {
	articleID int
}

func (s *stubArticleRepo) GetLatestCustomerArticleForTicket(ticketID uint) (*models.Article, error) {
	if s.articleID <= 0 {
		return nil, nil
	}
	return &models.Article{ID: s.articleID, TicketID: int(ticketID)}, nil
}

type recordingStorage struct {
	files []storedFile
}

type storedFile struct {
	filename    string
	contentType string
	data        []byte
	path        string
}

func (r *recordingStorage) Store(ctx context.Context, file multipart.File, header *multipart.FileHeader, path string) (*service.FileMetadata, error) {
	body, err := io.ReadAll(file)
	if err != nil {
		return nil, err
	}
	record := storedFile{
		filename:    header.Filename,
		contentType: header.Header.Get("Content-Type"),
		data:        body,
		path:        path,
	}
	r.files = append(r.files, record)
	return &service.FileMetadata{OriginalName: header.Filename, StoragePath: path, Size: int64(len(body))}, nil
}

func (r *recordingStorage) Retrieve(ctx context.Context, path string) (io.ReadCloser, error) {
	return nil, errors.New("not implemented")
}

func (r *recordingStorage) Delete(ctx context.Context, path string) error {
	return nil
}

func (r *recordingStorage) Exists(ctx context.Context, path string) (bool, error) {
	return false, nil
}

func (r *recordingStorage) GetURL(ctx context.Context, path string, expiry time.Duration) (string, error) {
	return "", nil
}

func (r *recordingStorage) GetMetadata(ctx context.Context, path string) (*service.FileMetadata, error) {
	return nil, nil
}

type stubTicketFinder struct {
	ticket *models.Ticket
	err    error
}

func (s *stubTicketFinder) GetByTicketNumber(ticketNumber string) (*models.Ticket, error) {
	if s.err != nil {
		return nil, s.err
	}
	return s.ticket, nil
}

type stubQueueFinder struct {
	queue *models.Queue
	err   error
}

func (s *stubQueueFinder) GetByID(id uint) (*models.Queue, error) {
	if s.err != nil {
		return nil, s.err
	}
	return s.queue, nil
}

type recordingArticleStore struct {
	last  *models.Article
	calls int
}

func (r *recordingArticleStore) Create(article *models.Article) error {
	r.calls++
	copy := *article
	if copy.ID == 0 {
		copy.ID = 100 + r.calls
	}
	r.last = &copy
	article.ID = copy.ID
	return nil
}

type stubMessageResolver struct {
	responses map[string]*models.Ticket
	calls     []string
	err       error
}

func (s *stubMessageResolver) FindTicketByMessageID(ctx context.Context, messageID string) (*models.Ticket, error) {
	s.calls = append(s.calls, messageID)
	if s.err != nil {
		return nil, s.err
	}
	if s.responses == nil {
		return nil, nil
	}
	if ticket, ok := s.responses[messageID]; ok {
		return ticket, nil
	}
	return nil, nil
}
