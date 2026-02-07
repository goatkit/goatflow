package postmaster

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"log"
	"mime"
	"mime/multipart"
	stdmail "net/mail"
	"net/textproto"
	"regexp"
	"strconv"
	"strings"
	"time"

	gomessage "github.com/emersion/go-message"
	gomail "github.com/emersion/go-message/mail"
	htmlcharset "golang.org/x/net/html/charset"

	"github.com/goatkit/goatflow/internal/constants"
	"github.com/goatkit/goatflow/internal/core"
	"github.com/goatkit/goatflow/internal/database"
	"github.com/goatkit/goatflow/internal/email/inbound/connector"
	"github.com/goatkit/goatflow/internal/email/inbound/filters"
	"github.com/goatkit/goatflow/internal/models"
	"github.com/goatkit/goatflow/internal/service"
)

type ticketCreator interface {
	Create(context.Context, service.CreateTicketInput) (*models.Ticket, error)
}

type ticketFinder interface {
	GetByTicketNumber(ticketNumber string) (*models.Ticket, error)
}

type queueFinder interface {
	GetByID(id uint) (*models.Queue, error)
}

type articleStore interface {
	Create(article *models.Article) error
}

type messageTicketLookup interface {
	FindTicketByMessageID(ctx context.Context, messageID string) (*models.Ticket, error)
}

// QueueLookupFunc resolves queue names to identifiers.
type QueueLookupFunc func(ctx context.Context, name string) (int, error)

type TicketProcessor struct {
	tickets         ticketCreator
	logger          *log.Logger
	systemUserID    int
	fallbackQueueID int
	maxBodyBytes    int64
	decoder         *mime.WordDecoder
	queueLookup     QueueLookupFunc
	storage         service.StorageService
	articleLookup   articleFinder
	ticketFinder    ticketFinder
	queueFinder     queueFinder
	articleStore    articleStore
	messageLookup   messageTicketLookup
	db              *sql.DB
	attachmentLimit int64
}

const (
	defaultSystemUserID    = 1
	defaultFallbackQueueID = 1
	defaultBodyLimit       = 128 * 1024
	defaultAttachmentLimit = 25 * 1024 * 1024
)

type articleFinder interface {
	GetLatestCustomerArticleForTicket(ticketID uint) (*models.Article, error)
}

func init() {
	gomessage.CharsetReader = func(charset string, input io.Reader) (io.Reader, error) {
		return htmlcharset.NewReaderLabel(charset, input)
	}
}

// TicketProcessorOption customizes TicketProcessor.
type TicketProcessorOption func(*TicketProcessor)

// NewTicketProcessor builds a processor that creates new tickets for inbound messages.
func NewTicketProcessor(tickets ticketCreator, opts ...TicketProcessorOption) *TicketProcessor {
	tp := &TicketProcessor{
		tickets:         tickets,
		logger:          log.Default(),
		systemUserID:    defaultSystemUserID,
		fallbackQueueID: defaultFallbackQueueID,
		maxBodyBytes:    defaultBodyLimit,
		decoder:         &mime.WordDecoder{},
		attachmentLimit: defaultAttachmentLimit,
	}
	for _, opt := range opts {
		if opt != nil {
			opt(tp)
		}
	}
	if tp.decoder == nil {
		tp.decoder = &mime.WordDecoder{}
	}
	return tp
}

// WithTicketProcessorLogger overrides the logger used for diagnostics.
func WithTicketProcessorLogger(logger *log.Logger) TicketProcessorOption {
	return func(tp *TicketProcessor) {
		if logger != nil {
			tp.logger = logger
		}
	}
}

// WithTicketProcessorSystemUser sets the user id used for ticket creation.
func WithTicketProcessorSystemUser(userID int) TicketProcessorOption {
	return func(tp *TicketProcessor) {
		if userID > 0 {
			tp.systemUserID = userID
		}
	}
}

// WithTicketProcessorFallbackQueue overrides the queue used when accounts omit routing.
func WithTicketProcessorFallbackQueue(queueID int) TicketProcessorOption {
	return func(tp *TicketProcessor) {
		if queueID > 0 {
			tp.fallbackQueueID = queueID
		}
	}
}

// WithTicketProcessorBodyLimit constrains how much of the body is stored on create.
func WithTicketProcessorBodyLimit(limit int64) TicketProcessorOption {
	return func(tp *TicketProcessor) {
		if limit > 0 {
			tp.maxBodyBytes = limit
		}
	}
}

// WithTicketProcessorQueueLookup wires a queue resolver for trusted header overrides.
func WithTicketProcessorQueueLookup(fn QueueLookupFunc) TicketProcessorOption {
	return func(tp *TicketProcessor) {
		if fn != nil {
			tp.queueLookup = fn
		}
	}
}

// WithTicketProcessorTicketFinder wires the ticket lookup used for follow-up detection.
func WithTicketProcessorTicketFinder(f ticketFinder) TicketProcessorOption {
	return func(tp *TicketProcessor) {
		if f != nil {
			tp.ticketFinder = f
		}
	}
}

// WithTicketProcessorQueueFinder wires the queue repository used for follow-up policy checks.
func WithTicketProcessorQueueFinder(f queueFinder) TicketProcessorOption {
	return func(tp *TicketProcessor) {
		if f != nil {
			tp.queueFinder = f
		}
	}
}

// WithTicketProcessorArticleStore wires the article repository used for follow-up inserts.
func WithTicketProcessorArticleStore(store articleStore) TicketProcessorOption {
	return func(tp *TicketProcessor) {
		if store != nil {
			tp.articleStore = store
		}
	}
}

// WithTicketProcessorMessageLookup wires the resolver used for References-based follow-ups.
func WithTicketProcessorMessageLookup(lookup messageTicketLookup) TicketProcessorOption {
	return func(tp *TicketProcessor) {
		if lookup != nil {
			tp.messageLookup = lookup
		}
	}
}

// WithTicketProcessorStorage wires the storage backend used for attachments.
func WithTicketProcessorStorage(storage service.StorageService) TicketProcessorOption {
	return func(tp *TicketProcessor) {
		if storage != nil {
			tp.storage = storage
		}
	}
}

// WithTicketProcessorArticleLookup provides access to created articles for attachment binding.
func WithTicketProcessorArticleLookup(lookup articleFinder) TicketProcessorOption {
	return func(tp *TicketProcessor) {
		if lookup != nil {
			tp.articleLookup = lookup
		}
	}
}

// WithTicketProcessorDatabase sets the database connection used for attachment metadata inserts.
func WithTicketProcessorDatabase(db *sql.DB) TicketProcessorOption {
	return func(tp *TicketProcessor) {
		if db != nil {
			tp.db = db
		}
	}
}

// WithTicketProcessorAttachmentLimit overrides the maximum attachment bytes buffered in memory.
func WithTicketProcessorAttachmentLimit(limit int64) TicketProcessorOption {
	return func(tp *TicketProcessor) {
		if limit > 0 {
			tp.attachmentLimit = limit
		}
	}
}

// Process parses the message and creates a ticket via the injected service.
func (tp *TicketProcessor) Process(ctx context.Context, msg *connector.FetchedMessage, meta *filters.MessageContext) (Result, error) {
	if msg == nil {
		return Result{}, errors.New("postmaster: message required")
	}
	if tp == nil || tp.tickets == nil {
		return Result{}, errors.New("postmaster: ticket service unavailable")
	}
	if annotationBool(meta, filters.AnnotationIgnoreMessage) {
		tp.logf("postmaster: ignoring message %s due to annotation", msg.UID)
		return Result{Action: "ignored"}, nil
	}

	queueID := tp.resolveQueueID(ctx, msg, meta)
	if queueID <= 0 {
		queueID = tp.fallbackQueueID
	}
	if queueID <= 0 {
		return Result{}, errors.New("postmaster: queue routing unavailable")
	}

	env := tp.extractEnvelope(msg)
	tp.applyAnnotationOverrides(meta, &env)
	title := strings.TrimSpace(env.Subject)
	if title == "" {
		title = tp.defaultSubject(msg)
	}
	env.Subject = title
	if res, handled, err := tp.tryFollowUp(ctx, msg, meta, &env); handled {
		return res, err
	}

	mimeType := tp.resolveMimeType(env.ContentType)
	charset := tp.resolveCharset(env.Charset)
	visible := true
	input := service.CreateTicketInput{
		Title:                         title,
		QueueID:                       queueID,
		UserID:                        tp.systemUserID,
		Body:                          env.Body,
		ArticleSubject:                title,
		ArticleSenderTypeID:           constants.ArticleSenderCustomer,
		ArticleTypeID:                 constants.ArticleTypeEmailExternal,
		ArticleIsVisibleForCustomer:   &visible,
		ArticleMimeType:               mimeType,
		ArticleCharset:                charset,
		ArticleCommunicationChannelID: core.MapCommunicationChannel(constants.ArticleTypeEmailExternal),
	}
	if env.CustomerID != "" {
		input.CustomerID = env.CustomerID
	}
	if env.CustomerUserID != "" {
		input.CustomerUserID = env.CustomerUserID
	}
	if priority := annotationInt(meta, filters.AnnotationPriorityIDOverride); priority > 0 {
		input.PriorityID = priority
	}

	ticket, err := tp.tickets.Create(ctx, input)
	if err != nil {
		return Result{Action: "error", Err: err}, err
	}
	if articleID := tp.resolveArticleID(ticket.ID); articleID > 0 {
		tp.storeAttachments(ctx, ticket.ID, articleID, env.Attachments)
	}

	return Result{TicketID: ticket.ID, Action: "new_ticket"}, nil
}

type envelope struct {
	Subject        string
	Body           string
	CustomerUserID string
	CustomerID     string
	ContentType    string
	Charset        string
	MessageID      string
	ReferenceIDs   []string
	Attachments    []attachmentPart
}

func (tp *TicketProcessor) applyAnnotationOverrides(meta *filters.MessageContext, env *envelope) {
	if meta == nil || env == nil {
		return
	}
	if title := annotationString(meta, filters.AnnotationTitleOverride); title != "" {
		env.Subject = title
	}
	if cid := annotationString(meta, filters.AnnotationCustomerIDOverride); cid != "" {
		env.CustomerID = cid
	}
	if customer := annotationString(meta, filters.AnnotationCustomerUserOverride); customer != "" {
		env.CustomerUserID = customer
	}
}

func (tp *TicketProcessor) resolveQueueID(ctx context.Context, msg *connector.FetchedMessage, meta *filters.MessageContext) int {
	if id := annotationInt(meta, filters.AnnotationQueueIDOverride); id > 0 {
		return id
	}
	if name := annotationString(meta, filters.AnnotationQueueNameOverride); name != "" && tp.queueLookup != nil {
		if id, err := tp.queueLookup(ctx, name); err == nil && id > 0 {
			return id
		} else if err != nil {
			tp.logf("postmaster: queue lookup failed for %s: %v", name, err)
		}
	}
	if msg != nil {
		if acc := msg.AccountSnapshot(); acc.QueueID > 0 {
			return acc.QueueID
		}
	}
	return 0
}

func (tp *TicketProcessor) extractEnvelope(msg *connector.FetchedMessage) envelope {
	var env envelope
	if msg == nil || len(msg.Raw) == 0 {
		return env
	}
	reader, err := gomail.CreateReader(bytes.NewReader(msg.Raw))
	if err != nil {
		tp.logf("postmaster: structured parse failed: %v", err)
		return tp.legacyEnvelope(msg)
	}
	env.Subject = tp.subjectFromHeader(&reader.Header)
	env.CustomerUserID = tp.addressFromHeader(&reader.Header)
	env.CustomerID = tp.domainFromAddress(env.CustomerUserID)
	env.ContentType, env.Charset = tp.contentTypeFromHeader(&reader.Header)
	env.MessageID = normalizeMessageID(reader.Header.Get("Message-Id"))
	referenceValues := reader.Header.Values("References")
	if inReply := reader.Header.Get("In-Reply-To"); inReply != "" {
		referenceValues = append(referenceValues, inReply)
	}
	env.ReferenceIDs = uniqueMessageIDs(referenceValues...)
	body, mimeType, charset, attachments := tp.readBodyParts(reader)
	if len(attachments) > 0 {
		env.Attachments = attachments
	}
	if body != "" {
		env.Body = body
		if mimeType != "" {
			env.ContentType = mimeType
		}
		if charset != "" {
			env.Charset = charset
		}
		return env
	}
	// Fallback to legacy parser when structured parsing does not yield a body
	legacy := tp.legacyEnvelope(msg)
	if env.Subject == "" {
		env.Subject = legacy.Subject
	}
	if env.CustomerUserID == "" {
		env.CustomerUserID = legacy.CustomerUserID
		env.CustomerID = legacy.CustomerID
	}
	if env.Body == "" {
		env.Body = legacy.Body
	}
	if env.ContentType == "" {
		env.ContentType = legacy.ContentType
	}
	if env.Charset == "" {
		env.Charset = legacy.Charset
	}
	if len(env.Attachments) == 0 {
		env.Attachments = nil
	}
	return env
}

func (tp *TicketProcessor) legacyEnvelope(msg *connector.FetchedMessage) envelope {
	var env envelope
	if msg == nil || len(msg.Raw) == 0 {
		return env
	}
	reader, err := stdmail.ReadMessage(bytes.NewReader(msg.Raw))
	if err != nil {
		tp.logf("postmaster: parse message failed: %v", err)
		env.Body = tp.fallbackBody(msg.Raw)
		return env
	}
	env.Subject = tp.decodeHeader(reader.Header.Get("Subject"))
	env.CustomerUserID = tp.parseAddress(reader.Header.Get("From"))
	env.CustomerID = tp.domainFromAddress(env.CustomerUserID)
	env.ContentType, env.Charset = tp.parseContentType(reader.Header.Get("Content-Type"))
	env.MessageID = normalizeMessageID(reader.Header.Get("Message-Id"))
	env.ReferenceIDs = uniqueMessageIDs(reader.Header.Get("References"), reader.Header.Get("In-Reply-To"))
	body, err := io.ReadAll(io.LimitReader(reader.Body, tp.bodyLimit()))
	if err != nil {
		tp.logf("postmaster: read body failed: %v", err)
		env.Body = tp.fallbackBody(msg.Raw)
	} else {
		env.Body = string(body)
	}
	return env
}

func (tp *TicketProcessor) subjectFromHeader(header *gomail.Header) string {
	if header == nil {
		return ""
	}
	if subject, err := header.Subject(); err == nil {
		return subject
	}
	return tp.decodeHeader(header.Get("Subject"))
}

func (tp *TicketProcessor) addressFromHeader(header *gomail.Header) string {
	if header == nil {
		return ""
	}
	if list, err := header.AddressList("From"); err == nil && len(list) > 0 {
		return strings.TrimSpace(list[0].Address)
	}
	return tp.parseAddress(header.Get("From"))
}

func (tp *TicketProcessor) contentTypeFromHeader(header *gomail.Header) (string, string) {
	if header == nil {
		return "", ""
	}
	if mediaType, params, err := header.ContentType(); err == nil {
		charset := strings.TrimSpace(params["charset"])
		return strings.ToLower(mediaType), strings.ToLower(charset)
	}
	return tp.parseContentType(header.Get("Content-Type"))
}

func (tp *TicketProcessor) readBodyParts(reader *gomail.Reader) (string, string, string, []attachmentPart) {
	if reader == nil {
		return "", "", "", nil
	}
	var plainCandidate, htmlCandidate *bodyCandidate
	var attachments []attachmentPart
	for {
		part, err := reader.NextPart()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			tp.logf("postmaster: read part failed: %v", err)
			break
		}
		switch header := part.Header.(type) {
		case *gomail.InlineHeader:
			body, mimeType, charset := tp.extractInlineBody(part, header)
			if body == "" {
				continue
			}
			candidate := &bodyCandidate{body: body, mimeType: mimeType, charset: charset}
			switch {
			case strings.HasPrefix(mimeType, "text/plain"):
				if plainCandidate == nil {
					plainCandidate = candidate
				}
			case strings.HasPrefix(mimeType, "text/html"):
				if htmlCandidate == nil {
					htmlCandidate = candidate
				}
			default:
				if plainCandidate == nil && htmlCandidate == nil {
					plainCandidate = candidate
				}
			}
		case *gomail.AttachmentHeader:
			if att := tp.extractAttachment(part, header); att != nil {
				attachments = append(attachments, *att)
			}
		default:
			// Ignore other part types
		}
	}
	if plainCandidate != nil {
		return plainCandidate.body, plainCandidate.mimeType, plainCandidate.charset, attachments
	}
	if htmlCandidate != nil {
		return htmlCandidate.body, htmlCandidate.mimeType, htmlCandidate.charset, attachments
	}
	return "", "", "", attachments
}

type bodyCandidate struct {
	body     string
	mimeType string
	charset  string
}

type attachmentPart struct {
	filename    string
	contentType string
	data        []byte
}

func (tp *TicketProcessor) extractInlineBody(part *gomail.Part, header *gomail.InlineHeader) (string, string, string) {
	if part == nil || header == nil {
		return "", "", ""
	}
	mimeType, params, err := header.ContentType()
	charset := ""
	if err != nil {
		mimeType, charset = tp.parseContentType(header.Get("Content-Type"))
	} else {
		charset = params["charset"]
	}
	mimeType = strings.ToLower(strings.TrimSpace(mimeType))
	charset = strings.ToLower(strings.TrimSpace(charset))
	if mimeType == "" {
		mimeType = "text/plain"
	}
	body, readErr := tp.readPartBody(part.Body)
	if readErr != nil {
		tp.logf("postmaster: read part body failed: %v", readErr)
		return "", "", ""
	}
	return body, mimeType, charset
}

func (tp *TicketProcessor) extractAttachment(part *gomail.Part, header *gomail.AttachmentHeader) *attachmentPart {
	if part == nil || header == nil {
		return nil
	}
	filename, err := header.Filename()
	if err != nil || strings.TrimSpace(filename) == "" {
		filename = fmt.Sprintf("attachment-%d.bin", time.Now().UnixNano())
	}
	mimeType, _, ctErr := header.ContentType()
	if ctErr != nil || strings.TrimSpace(mimeType) == "" {
		mimeType, _ = tp.parseContentType(header.Get("Content-Type"))
	}
	mimeType = strings.ToLower(strings.TrimSpace(mimeType))
	if mimeType == "" {
		mimeType = "application/octet-stream"
	}
	body, readErr := tp.readAttachmentBody(part.Body)
	if readErr != nil {
		tp.logf("postmaster: read attachment body failed: %v", readErr)
		return nil
	}
	if len(body) == 0 {
		return nil
	}
	return &attachmentPart{filename: filename, contentType: mimeType, data: body}
}

func (tp *TicketProcessor) readPartBody(src io.Reader) (string, error) {
	if src == nil {
		return "", nil
	}
	limit := tp.bodyLimit()
	data, err := io.ReadAll(io.LimitReader(src, limit))
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func (tp *TicketProcessor) readAttachmentBody(src io.Reader) ([]byte, error) {
	if src == nil {
		return nil, nil
	}
	limit := tp.attachmentLimitBytes()
	return io.ReadAll(io.LimitReader(src, limit))
}

func (tp *TicketProcessor) decodeHeader(value string) string {
	value = strings.TrimSpace(value)
	if value == "" || tp.decoder == nil {
		return value
	}
	decoded, err := tp.decoder.DecodeHeader(value)
	if err != nil {
		return value
	}
	return decoded
}

func (tp *TicketProcessor) parseAddress(value string) string {
	value = tp.decodeHeader(value)
	if value == "" {
		return ""
	}
	if addrs, err := stdmail.ParseAddressList(value); err == nil && len(addrs) > 0 {
		return strings.TrimSpace(addrs[0].Address)
	}
	if addr, err := stdmail.ParseAddress(value); err == nil {
		return strings.TrimSpace(addr.Address)
	}
	return strings.TrimSpace(value)
}

func (tp *TicketProcessor) domainFromAddress(addr string) string {
	addr = strings.TrimSpace(addr)
	if addr == "" {
		return ""
	}
	if at := strings.LastIndex(addr, "@"); at >= 0 && at < len(addr)-1 {
		return strings.TrimSpace(strings.ToLower(addr[at+1:]))
	}
	return ""
}

func (tp *TicketProcessor) defaultSubject(msg *connector.FetchedMessage) string {
	if msg == nil {
		return "Inbound email"
	}
	if msg.RemoteID != "" {
		return fmt.Sprintf("Inbound email %s", msg.RemoteID)
	}
	if msg.UID != "" {
		return fmt.Sprintf("Inbound email %s", msg.UID)
	}
	return "Inbound email"
}

func (tp *TicketProcessor) parseContentType(value string) (string, string) {
	raw := strings.TrimSpace(value)
	if raw == "" {
		return "", ""
	}
	mediaType := raw
	charset := ""
	if parsed, params, err := mime.ParseMediaType(raw); err == nil {
		mediaType = parsed
		if cs, ok := params["charset"]; ok {
			charset = strings.TrimSpace(cs)
		}
	}
	return strings.ToLower(mediaType), strings.ToLower(charset)
}

func (tp *TicketProcessor) resolveMimeType(value string) string {
	v := strings.TrimSpace(strings.ToLower(value))
	if v == "" || strings.HasPrefix(v, "multipart/") {
		return "text/plain"
	}
	return v
}

func (tp *TicketProcessor) resolveCharset(value string) string {
	v := strings.TrimSpace(strings.ToLower(value))
	if v == "" {
		return "utf-8"
	}
	return v
}

func (tp *TicketProcessor) fallbackBody(raw []byte) string {
	if len(raw) == 0 {
		return ""
	}
	limit := tp.bodyLimit()
	if limit > 0 && int64(len(raw)) > limit {
		raw = raw[:limit]
	}
	return string(raw)
}

func (tp *TicketProcessor) bodyLimit() int64 {
	if tp == nil || tp.maxBodyBytes <= 0 {
		return defaultBodyLimit
	}
	return tp.maxBodyBytes
}

func (tp *TicketProcessor) attachmentLimitBytes() int64 {
	if tp == nil || tp.attachmentLimit <= 0 {
		return defaultAttachmentLimit
	}
	return tp.attachmentLimit
}

func (tp *TicketProcessor) resolveArticleID(ticketID int) int {
	if tp == nil || tp.articleLookup == nil || ticketID <= 0 {
		return 0
	}
	article, err := tp.articleLookup.GetLatestCustomerArticleForTicket(uint(ticketID))
	if err != nil {
		tp.logf("postmaster: article lookup failed for ticket %d: %v", ticketID, err)
		return 0
	}
	if article == nil {
		return 0
	}
	return article.ID
}

func (tp *TicketProcessor) storeAttachments(ctx context.Context, ticketID, articleID int, attachments []attachmentPart) {
	if len(attachments) == 0 || ticketID <= 0 || articleID <= 0 || tp == nil || tp.storage == nil {
		return
	}
	for _, att := range attachments {
		tp.storeAttachment(ctx, ticketID, articleID, att)
	}
}

func (tp *TicketProcessor) storeAttachment(ctx context.Context, ticketID, articleID int, att attachmentPart) {
	if len(att.data) == 0 || att.filename == "" {
		return
	}
	if ctx == nil {
		ctx = context.Background()
	}
	ctx = service.WithArticleID(ctx, articleID)
	ctx = service.WithUserID(ctx, tp.systemUserID)
	file := newMemoryFile(att.data)
	header := buildFileHeader(att)
	path := service.GenerateOTRSStoragePath(ticketID, articleID, att.filename)
	if _, err := tp.storage.Store(ctx, file, header, path); err != nil {
		tp.logf("postmaster: attachment store failed for %s: %v", att.filename, err)
		return
	}
	if !tp.storageIsDB() {
		if err := tp.insertAttachmentRecord(articleID, att); err != nil {
			tp.logf("postmaster: attachment metadata insert failed for %s: %v", att.filename, err)
		}
	}
}

func (tp *TicketProcessor) storageIsDB() bool {
	if tp == nil || tp.storage == nil {
		return false
	}
	_, ok := tp.storage.(*service.DatabaseStorageService)
	return ok
}

func (tp *TicketProcessor) tryFollowUp(ctx context.Context, msg *connector.FetchedMessage, meta *filters.MessageContext, env *envelope) (Result, bool, error) {
	if tp == nil || env == nil || tp.articleStore == nil {
		return Result{}, false, nil
	}
	ticket := tp.resolveFollowUpTicket(ctx, meta, env)
	if ticket == nil {
		return Result{}, false, nil
	}
	if !tp.followUpAllowed(ticket.QueueID) {
		tp.logf("postmaster: queue %d rejects follow-up for ticket %d", ticket.QueueID, ticket.ID)
		return Result{}, false, nil
	}
	article := tp.buildFollowUpArticle(ticket.ID, env, msg)
	if article == nil {
		return Result{}, true, errors.New("postmaster: unable to build follow-up article")
	}
	if err := tp.articleStore.Create(article); err != nil {
		return Result{}, true, err
	}
	tp.storeAttachments(ctx, ticket.ID, article.ID, env.Attachments)
	tp.logf("postmaster: appended follow-up to ticket %d", ticket.ID)
	return Result{TicketID: ticket.ID, ArticleID: article.ID, Action: "follow_up"}, true, nil
}

func (tp *TicketProcessor) followUpAllowed(queueID int) bool {
	if queueID <= 0 {
		return false
	}
	if tp.queueFinder == nil {
		return true
	}
	queue, err := tp.queueFinder.GetByID(uint(queueID))
	if err != nil {
		tp.logf("postmaster: queue lookup failed for %d: %v", queueID, err)
		return true
	}
	if queue == nil {
		return false
	}
	if queue.FollowUpID == 0 || queue.FollowUpID == 1 {
		return true
	}
	return false
}

func (tp *TicketProcessor) resolveFollowUpTicket(ctx context.Context, meta *filters.MessageContext, env *envelope) *models.Ticket {
	if ticket := tp.resolveFollowUpTicketFromAnnotation(meta); ticket != nil {
		return ticket
	}
	return tp.resolveFollowUpTicketFromReferences(ctx, env)
}

func (tp *TicketProcessor) resolveFollowUpTicketFromAnnotation(meta *filters.MessageContext) *models.Ticket {
	if tp == nil || tp.ticketFinder == nil {
		return nil
	}
	ticketNumber := annotationString(meta, filters.AnnotationFollowUpTicketNumber)
	if ticketNumber == "" {
		return nil
	}
	ticket, err := tp.ticketFinder.GetByTicketNumber(ticketNumber)
	if err != nil {
		tp.logf("postmaster: follow-up ticket lookup failed for %s: %v", ticketNumber, err)
		return nil
	}
	return ticket
}

func (tp *TicketProcessor) resolveFollowUpTicketFromReferences(ctx context.Context, env *envelope) *models.Ticket {
	if tp == nil || tp.messageLookup == nil || env == nil || len(env.ReferenceIDs) == 0 {
		return nil
	}
	if ctx == nil {
		ctx = context.Background()
	}
	for i := len(env.ReferenceIDs) - 1; i >= 0; i-- {
		id := env.ReferenceIDs[i]
		if id == "" {
			continue
		}
		ticket, err := tp.messageLookup.FindTicketByMessageID(ctx, id)
		if err != nil {
			tp.logf("postmaster: references lookup failed for message-id %s: %v", id, err)
			return nil
		}
		if ticket != nil {
			tp.logf("postmaster: matched follow-up via message-id %s", id)
			return ticket
		}
	}
	return nil
}

func (tp *TicketProcessor) buildFollowUpArticle(ticketID int, env *envelope, msg *connector.FetchedMessage) *models.Article {
	if ticketID <= 0 || env == nil {
		return nil
	}
	subject := strings.TrimSpace(env.Subject)
	if subject == "" {
		subject = tp.defaultSubject(msg)
	}
	body := env.Body
	mimeType := tp.resolveMimeType(env.ContentType)
	charset := tp.resolveCharset(env.Charset)
	return &models.Article{
		TicketID:               ticketID,
		Subject:                subject,
		Body:                   body,
		ArticleTypeID:          constants.ArticleTypeEmailExternal,
		SenderTypeID:           constants.ArticleSenderCustomer,
		CommunicationChannelID: core.MapCommunicationChannel(constants.ArticleTypeEmailExternal),
		IsVisibleForCustomer:   1,
		MimeType:               mimeType,
		Charset:                charset,
		CreateBy:               tp.systemUserID,
		ChangeBy:               tp.systemUserID,
	}
}

func (tp *TicketProcessor) insertAttachmentRecord(articleID int, att attachmentPart) error {
	db := tp.db
	if db == nil {
		var err error
		db, err = database.GetDB()
		if err != nil || db == nil {
			return fmt.Errorf("database unavailable: %w", err)
		}
	}
	contentType := att.contentType
	if strings.TrimSpace(contentType) == "" {
		contentType = "application/octet-stream"
	}
	now := time.Now()
	_, err := db.Exec(database.ConvertPlaceholders(`
		INSERT INTO article_data_mime_attachment (
			article_id, filename, content_type, content_size, content,
			disposition, create_time, create_by, change_time, change_by
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)`),
		articleID,
		att.filename,
		contentType,
		int64(len(att.data)),
		att.data,
		"attachment",
		now, tp.systemUserID, now, tp.systemUserID,
	)
	return err
}

func buildFileHeader(att attachmentPart) *multipart.FileHeader {
	headers := make(textproto.MIMEHeader)
	if ct := strings.TrimSpace(att.contentType); ct != "" {
		headers.Set("Content-Type", ct)
	} else {
		headers.Set("Content-Type", "application/octet-stream")
	}
	return &multipart.FileHeader{
		Filename: att.filename,
		Header:   headers,
		Size:     int64(len(att.data)),
	}
}

type memoryFile struct {
	*bytes.Reader
}

func newMemoryFile(data []byte) *memoryFile {
	if data == nil {
		data = []byte{}
	}
	return &memoryFile{Reader: bytes.NewReader(data)}
}

func (m *memoryFile) Close() error {
	return nil
}

var messageIDPattern = regexp.MustCompile(`<([^<>]+)>`)

func uniqueMessageIDs(values ...string) []string {
	seen := make(map[string]struct{})
	var ids []string
	for _, raw := range values {
		for _, candidate := range parseMessageIDs(raw) {
			if _, ok := seen[candidate]; ok {
				continue
			}
			seen[candidate] = struct{}{}
			ids = append(ids, candidate)
		}
	}
	if len(ids) == 0 {
		return nil
	}
	return ids
}

func parseMessageIDs(raw string) []string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	if messageIDPattern == nil {
		if id := normalizeMessageID(raw); id != "" {
			return []string{id}
		}
		return nil
	}
	matches := messageIDPattern.FindAllStringSubmatch(raw, -1)
	if len(matches) == 0 {
		if id := normalizeMessageID(raw); id != "" {
			return []string{id}
		}
		return nil
	}
	ids := make([]string, 0, len(matches))
	for _, match := range matches {
		if len(match) < 2 {
			continue
		}
		if id := normalizeMessageID(match[1]); id != "" {
			ids = append(ids, id)
		}
	}
	return ids
}

func normalizeMessageID(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	value = strings.Trim(value, "<>")
	value = strings.Trim(value, "\"")
	return strings.TrimSpace(value)
}

func annotationString(meta *filters.MessageContext, key string) string {
	if meta == nil || meta.Annotations == nil {
		return ""
	}
	if raw, ok := meta.Annotations[key]; ok {
		switch v := raw.(type) {
		case string:
			return strings.TrimSpace(v)
		case fmt.Stringer:
			return strings.TrimSpace(v.String())
		case []byte:
			return strings.TrimSpace(string(v))
		default:
			return strings.TrimSpace(fmt.Sprint(v))
		}
	}
	return ""
}

func annotationInt(meta *filters.MessageContext, key string) int {
	if meta == nil || meta.Annotations == nil {
		return 0
	}
	raw, ok := meta.Annotations[key]
	if !ok {
		return 0
	}
	switch v := raw.(type) {
	case int:
		return v
	case int32:
		return int(v)
	case int64:
		return int(v)
	case float64:
		return int(v)
	case string:
		if n, err := strconv.Atoi(strings.TrimSpace(v)); err == nil {
			return n
		}
	case fmt.Stringer:
		if n, err := strconv.Atoi(strings.TrimSpace(v.String())); err == nil {
			return n
		}
	}
	return 0
}

func annotationBool(meta *filters.MessageContext, key string) bool {
	if meta == nil || meta.Annotations == nil {
		return false
	}
	raw, ok := meta.Annotations[key]
	if !ok {
		return false
	}
	switch v := raw.(type) {
	case bool:
		return v
	case int:
		return v != 0
	case int32:
		return v != 0
	case int64:
		return v != 0
	case float64:
		return v != 0
	case string:
		value := strings.TrimSpace(strings.ToLower(v))
		return value == "1" || value == "true" || value == "yes" || value == "y"
	case fmt.Stringer:
		value := strings.TrimSpace(strings.ToLower(v.String()))
		return value == "1" || value == "true" || value == "yes" || value == "y"
	default:
		return false
	}
}

func (tp *TicketProcessor) logf(format string, args ...any) {
	if tp == nil || tp.logger == nil {
		return
	}
	tp.logger.Printf(format, args...)
}
