package notificationevents

import (
	"context"
	"database/sql"
	"errors"
	"html"
	"strings"
	"time"

	"github.com/goatkit/goatflow/internal/platform/database"
	"github.com/goatkit/goatflow/internal/platform/utils"
)

// ticketData is the ticket as the notification sees it.
type ticketData struct {
	id                         int64
	tn, title                  string
	queueID, groupID           int
	queue                      string
	stateID                    int
	state, stateType           string
	priorityID                 int
	priority                   string
	typeID                     int
	typeName                   string
	lockID                     int
	lock                       string
	serviceID, slaID           int
	service, sla               string
	ownerID, responsibleID     int
	customerID, customerUserID string
	created, changed           time.Time
	createBy                   int
}

// loadTicket returns the ticket, or nil when it no longer exists.
func loadTicket(ctx context.Context, db *sql.DB, id int64) (*ticketData, error) {
	t := &ticketData{id: id}
	err := db.QueryRowContext(ctx, database.ConvertPlaceholders(`
		SELECT t.tn, COALESCE(t.title, ''), t.queue_id, q.group_id, q.name,
			t.ticket_state_id, s.name, st.name, t.ticket_priority_id, p.name,
			COALESCE(t.type_id, 0), COALESCE(tt.name, ''), t.ticket_lock_id, COALESCE(tl.name, ''),
			COALESCE(t.service_id, 0), COALESCE(sv.name, ''), COALESCE(t.sla_id, 0), COALESCE(sl.name, ''),
			t.user_id, t.responsible_user_id, COALESCE(t.customer_id, ''), COALESCE(t.customer_user_id, ''),
			t.create_time, t.change_time, t.create_by
		FROM ticket t
		JOIN queue q ON q.id = t.queue_id
		JOIN ticket_state s ON s.id = t.ticket_state_id
		JOIN ticket_state_type st ON st.id = s.type_id
		JOIN ticket_priority p ON p.id = t.ticket_priority_id
		LEFT JOIN ticket_type tt ON tt.id = t.type_id
		LEFT JOIN ticket_lock_type tl ON tl.id = t.ticket_lock_id
		LEFT JOIN service sv ON sv.id = t.service_id
		LEFT JOIN sla sl ON sl.id = t.sla_id
		WHERE t.id = ?`), id).Scan(
		&t.tn, &t.title, &t.queueID, &t.groupID, &t.queue,
		&t.stateID, &t.state, &t.stateType, &t.priorityID, &t.priority,
		&t.typeID, &t.typeName, &t.lockID, &t.lock,
		&t.serviceID, &t.service, &t.slaID, &t.sla,
		&t.ownerID, &t.responsibleID, &t.customerID, &t.customerUserID,
		&t.created, &t.changed, &t.createBy)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	return t, err
}

// articleData is an article as the notification sees it.
type articleData struct {
	id                 int64
	senderTypeID       int
	senderType         string
	channelID          int
	visibleForCustomer int
	from, to, cc       string
	subject, body      string
}

const articleSelect = `
	SELECT a.id, a.article_sender_type_id, COALESCE(ast.name, ''), a.communication_channel_id,
		a.is_visible_for_customer, COALESCE(m.a_from, ''), COALESCE(m.a_to, ''), COALESCE(m.a_cc, ''),
		COALESCE(m.a_subject, ''), m.a_body, COALESCE(m.a_content_type, '')
	FROM article a
	LEFT JOIN article_sender_type ast ON ast.id = a.article_sender_type_id
	LEFT JOIN article_data_mime m ON m.article_id = a.id`

func scanArticle(row *sql.Row) (*articleData, error) {
	a := &articleData{}
	var (
		body        []byte
		contentType string
	)
	err := row.Scan(&a.id, &a.senderTypeID, &a.senderType, &a.channelID, &a.visibleForCustomer,
		&a.from, &a.to, &a.cc, &a.subject, &body, &contentType)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	a.body = string(body)
	if strings.Contains(strings.ToLower(contentType), "text/html") {
		a.body = html.UnescapeString(utils.StripHTML(a.body))
	}
	return a, nil
}

// loadArticle returns the article, or nil when it no longer exists.
func loadArticle(ctx context.Context, db *sql.DB, id int64) (*articleData, error) {
	return scanArticle(db.QueryRowContext(ctx, database.ConvertPlaceholders(articleSelect+` WHERE a.id = ?`), id))
}

// latestArticle returns the ticket's newest article sent by senderType
// ("customer", "agent"), or by anyone when senderType is empty.
func latestArticle(ctx context.Context, db *sql.DB, ticketID int64, senderType string) (*articleData, error) {
	if senderType == "" {
		return scanArticle(db.QueryRowContext(ctx, database.ConvertPlaceholders(
			articleSelect+` WHERE a.ticket_id = ? ORDER BY a.id DESC LIMIT 1`), ticketID))
	}
	return scanArticle(db.QueryRowContext(ctx, database.ConvertPlaceholders(
		articleSelect+` WHERE a.ticket_id = ? AND ast.name = ? ORDER BY a.id DESC LIMIT 1`), ticketID, senderType))
}

// person is an agent or customer user as templates and recipients see them.
type person struct {
	userID     int // agent id; 0 for customers and plain addresses
	login      string
	firstName  string
	lastName   string
	email      string
	language   string
	customerID string
	phone      string
	title      string
	valid      bool
}

func (p *person) fullName() string {
	return strings.TrimSpace(p.firstName + " " + p.lastName)
}

// loadAgent returns the agent with its UserEmail and language preferences,
// or nil when the user does not exist.
func loadAgent(ctx context.Context, db *sql.DB, id int) (*person, error) {
	p := &person{userID: id}
	var validID int
	err := db.QueryRowContext(ctx, database.ConvertPlaceholders(`
		SELECT login, first_name, last_name, COALESCE(title, ''), valid_id FROM users WHERE id = ?`), id).
		Scan(&p.login, &p.firstName, &p.lastName, &p.title, &validID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	p.valid = validID == 1
	rows, err := db.QueryContext(ctx, database.ConvertPlaceholders(`
		SELECT preferences_key, preferences_value FROM user_preferences
		WHERE user_id = ? AND preferences_key IN ('UserEmail', 'Language', 'UserLanguage')`), id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var otrsLanguage string
	for rows.Next() {
		var (
			key   string
			value []byte
		)
		if err := rows.Scan(&key, &value); err != nil {
			return nil, err
		}
		switch key {
		case "UserEmail":
			p.email = strings.TrimSpace(string(value))
		case "Language":
			p.language = strings.TrimSpace(string(value))
		case "UserLanguage":
			otrsLanguage = strings.TrimSpace(string(value))
		}
	}
	if p.language == "" {
		p.language = otrsLanguage
	}
	return p, rows.Err()
}

// loadCustomer returns the ticket's customer user. The ticket's
// customer_user_id is a customer_user login, or an email address for
// customers without an account; a nil result means neither applies.
func loadCustomer(ctx context.Context, db *sql.DB, customerUserID string) (*person, error) {
	customerUserID = strings.TrimSpace(customerUserID)
	if customerUserID == "" {
		return nil, nil
	}
	p := &person{}
	var validID int
	err := db.QueryRowContext(ctx, database.ConvertPlaceholders(`
		SELECT login, first_name, last_name, email, customer_id, COALESCE(phone, ''), COALESCE(title, ''), valid_id
		FROM customer_user WHERE login = ?`), customerUserID).
		Scan(&p.login, &p.firstName, &p.lastName, &p.email, &p.customerID, &p.phone, &p.title, &validID)
	if errors.Is(err, sql.ErrNoRows) {
		if strings.Contains(customerUserID, "@") {
			return &person{login: customerUserID, email: customerUserID, valid: true}, nil
		}
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	p.valid = validID == 1
	rows, err := db.QueryContext(ctx, database.ConvertPlaceholders(`
		SELECT preferences_key, COALESCE(preferences_value, '') FROM customer_preferences
		WHERE user_id = ? AND preferences_key IN ('Language', 'UserLanguage')`), p.login)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var otrsLanguage string
	for rows.Next() {
		var key, value string
		if err := rows.Scan(&key, &value); err != nil {
			return nil, err
		}
		if key == "Language" {
			p.language = strings.TrimSpace(value)
		} else {
			otrsLanguage = strings.TrimSpace(value)
		}
	}
	if p.language == "" {
		p.language = otrsLanguage
	}
	return p, rows.Err()
}
