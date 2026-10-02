package notificationevents

import (
	"context"
	"database/sql"
	"sort"
	"strconv"
	"strings"

	"github.com/goatkit/goatflow/internal/platform/database"
)

// rule is one valid notification_event with its items and messages.
type rule struct {
	id       int
	name     string
	events   map[string]bool
	items    map[string][]string // every notification_event_item except Events
	messages map[string]message  // by language
}

type message struct {
	subject     string
	text        string
	contentType string
}

// Item keys that hold recipients or delivery options rather than filters.
var controlKeys = map[string]bool{
	"Recipients":      true,
	"RecipientAgents": true,
	"RecipientGroups": true,
	"RecipientRoles":  true,
	"RecipientEmail":  true,
	"SkipRecipients":  true,
	"Transports":      true,
	"LanguageID":      true,
	"OncePerDay":      true,
	// OTRS options for the per-agent opt-out screen, out-of-office
	// handling, attachments, the email template and S/MIME/PGP. GoatFlow has
	// none of these features; the options are kept so OTRS imports round-trip.
	"VisibleForAgent":          true,
	"VisibleForAgentTooltip":   true,
	"AgentEnabledByDefault":    true,
	"SendOnOutOfOffice":        true,
	"ArticleAttachmentInclude": true,
	"TransportEmailTemplate":   true,
	"EmailSecuritySettings":    true,
	"EmailSigningCrypting":     true,
	"EmailMissingSigningKeys":  true,
	"EmailMissingCryptingKeys": true,
	"EmailDefaultSigningKeys":  true,
	"IsVisibleForCustomer":     true,
}

// Ticket attribute filters: item key -> ticket field.
var ticketFilterKeys = map[string]func(*ticketData) string{
	"QueueID":        func(t *ticketData) string { return strconv.Itoa(t.queueID) },
	"StateID":        func(t *ticketData) string { return strconv.Itoa(t.stateID) },
	"PriorityID":     func(t *ticketData) string { return strconv.Itoa(t.priorityID) },
	"TypeID":         func(t *ticketData) string { return strconv.Itoa(t.typeID) },
	"LockID":         func(t *ticketData) string { return strconv.Itoa(t.lockID) },
	"ServiceID":      func(t *ticketData) string { return strconv.Itoa(t.serviceID) },
	"SLAID":          func(t *ticketData) string { return strconv.Itoa(t.slaID) },
	"OwnerID":        func(t *ticketData) string { return strconv.Itoa(t.ownerID) },
	"ResponsibleID":  func(t *ticketData) string { return strconv.Itoa(t.responsibleID) },
	"CustomerID":     func(t *ticketData) string { return t.customerID },
	"CustomerUserID": func(t *ticketData) string { return t.customerUserID },
}

// Article attribute filters; a rule with any of them only matches events
// that carry an article.
var articleFilterKeys = map[string]func(*articleData, string) bool{
	"ArticleSenderTypeID": func(a *articleData, v string) bool { return strconv.Itoa(a.senderTypeID) == v },
	"ArticleCommunicationChannelID": func(a *articleData, v string) bool {
		return strconv.Itoa(a.channelID) == v
	},
	"ArticleIsVisibleForCustomer": func(a *articleData, v string) bool {
		return strconv.Itoa(a.visibleForCustomer) == v
	},
	"ArticleSubjectMatch": func(a *articleData, v string) bool {
		return strings.Contains(strings.ToLower(a.subject), strings.ToLower(v))
	},
	"ArticleBodyMatch": func(a *articleData, v string) bool {
		return strings.Contains(strings.ToLower(a.body), strings.ToLower(v))
	},
}

// loadRules returns the valid notification rules, ordered by id.
func loadRules(ctx context.Context, db *sql.DB) ([]*rule, error) {
	rows, err := db.QueryContext(ctx, database.ConvertPlaceholders(
		`SELECT id, name FROM notification_event WHERE valid_id = 1 ORDER BY id`))
	if err != nil {
		return nil, err
	}
	byID := map[int]*rule{}
	var rules []*rule
	for rows.Next() {
		r := &rule{events: map[string]bool{}, items: map[string][]string{}, messages: map[string]message{}}
		if err := rows.Scan(&r.id, &r.name); err != nil {
			_ = rows.Close()
			return nil, err
		}
		byID[r.id] = r
		rules = append(rules, r)
	}
	_ = rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if len(rules) == 0 {
		return nil, nil
	}

	rows, err = db.QueryContext(ctx, database.ConvertPlaceholders(
		`SELECT notification_id, event_key, event_value FROM notification_event_item`))
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var (
			id         int
			key, value string
		)
		if err := rows.Scan(&id, &key, &value); err != nil {
			_ = rows.Close()
			return nil, err
		}
		r := byID[id]
		if r == nil {
			continue
		}
		if key == "Events" {
			r.events[value] = true
		} else {
			r.items[key] = append(r.items[key], value)
		}
	}
	_ = rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}

	rows, err = db.QueryContext(ctx, database.ConvertPlaceholders(
		`SELECT notification_id, language, subject, text, content_type FROM notification_event_message`))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var (
			id   int
			lang string
			m    message
		)
		if err := rows.Scan(&id, &lang, &m.subject, &m.text, &m.contentType); err != nil {
			return nil, err
		}
		if r := byID[id]; r != nil {
			r.messages[lang] = m
		}
	}
	return rules, rows.Err()
}

// subscribed returns the first of names the rule subscribes to.
func (r *rule) subscribed(names []string) (string, bool) {
	for _, n := range names {
		if r.events[n] {
			return n, true
		}
	}
	return "", false
}

// unsupportedKeys returns the item keys the evaluator cannot apply (for
// example OTRS dynamic field filters). A rule with such a filter is not
// evaluated: ignoring the filter would notify for tickets it excludes.
func (r *rule) unsupportedKeys() []string {
	var out []string
	for key := range r.items {
		if controlKeys[key] || ticketFilterKeys[key] != nil || articleFilterKeys[key] != nil {
			continue
		}
		out = append(out, key)
	}
	sort.Strings(out)
	return out
}

func (r *rule) hasArticleFilter() bool {
	for key := range r.items {
		if articleFilterKeys[key] != nil {
			return true
		}
	}
	return false
}

// matchesTicket reports whether every ticket filter of the rule accepts t;
// a filter accepts the ticket when any of its values equals the field.
func (r *rule) matchesTicket(t *ticketData) bool {
	for key, values := range r.items {
		field := ticketFilterKeys[key]
		if field == nil {
			continue
		}
		if !containsString(values, field(t)) {
			return false
		}
	}
	return true
}

// matchesArticle reports whether every article filter of the rule accepts a.
func (r *rule) matchesArticle(a *articleData) bool {
	for key, values := range r.items {
		match := articleFilterKeys[key]
		if match == nil {
			continue
		}
		ok := false
		for _, v := range values {
			if match(a, v) {
				ok = true
				break
			}
		}
		if !ok {
			return false
		}
	}
	return true
}

// emailTransport reports whether the rule sends email. OTRS stores the
// transports a rule uses under Transports; rules without it (GoatFlow's
// admin form) use email, the only transport GoatFlow has.
func (r *rule) emailTransport() bool {
	transports, ok := r.items["Transports"]
	return !ok || containsString(transports, "Email")
}

func (r *rule) oncePerDay() bool {
	return containsString(r.items["OncePerDay"], "1")
}

// messageFor picks the message in lang, else the default language, else
// English, else the first language in alphabetical order.
func (r *rule) messageFor(lang, defaultLang string) (message, bool) {
	for _, l := range []string{lang, defaultLang, "en"} {
		if m, ok := r.messages[l]; ok && l != "" {
			return m, true
		}
	}
	langs := make([]string, 0, len(r.messages))
	for l := range r.messages {
		langs = append(langs, l)
	}
	if len(langs) == 0 {
		return message{}, false
	}
	sort.Strings(langs)
	return r.messages[langs[0]], true
}

func containsString(values []string, s string) bool {
	for _, v := range values {
		if v == s {
			return true
		}
	}
	return false
}
