package notificationevents

import (
	"context"
	"database/sql"
	"fmt"
	"net/mail"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/goatkit/goatflow/internal/platform/database"
)

// batch evaluates the events of one transaction. Rows read for rendering
// come from the service's pool; everything written goes through tx.
type batch struct {
	s  *Service
	tx *sql.Tx
	// metaSent remembers the escalation meta-events already notified per
	// rule and ticket in this batch.
	metaSent map[string]bool
	agents   map[int]*person
}

func newBatch(s *Service, tx *sql.Tx) *batch {
	return &batch{s: s, tx: tx, metaSent: map[string]bool{}, agents: map[int]*person{}}
}

// evaluate queues the notifications of every rule subscribed to ev.
func (b *batch) evaluate(ctx context.Context, ev event, rules []*rule) (int, error) {
	if len(ev.names) == 0 || len(rules) == 0 {
		return 0, nil
	}
	var matched []*rule
	matchedEvent := map[int]string{}
	for _, r := range rules {
		name, ok := r.subscribed(ev.names)
		if !ok {
			continue
		}
		if name == metaEscalation || name == metaEscalationNotifyBefore {
			key := fmt.Sprintf("%d/%d/%s", r.id, ev.ticketID, name)
			if b.metaSent[key] {
				continue
			}
			b.metaSent[key] = true
		}
		matched = append(matched, r)
		matchedEvent[r.id] = name
	}
	if len(matched) == 0 {
		return 0, nil
	}

	db := b.s.db
	ticket, err := loadTicket(ctx, db, ev.ticketID)
	if err != nil || ticket == nil {
		// A ticket deleted before its events were evaluated has no one to
		// notify about.
		return 0, err
	}
	var article *articleData
	if ev.articleID > 0 {
		if article, err = loadArticle(ctx, db, ev.articleID); err != nil {
			return 0, err
		}
	}

	var td *templateData
	queued := 0
	for _, r := range matched {
		if keys := r.unsupportedKeys(); len(keys) > 0 {
			b.s.logger.Printf("notification-events: rule %d %q not evaluated: unsupported filter %s",
				r.id, r.name, strings.Join(keys, ", "))
			continue
		}
		if !r.emailTransport() || !r.matchesTicket(ticket) {
			continue
		}
		if r.hasArticleFilter() && (article == nil || !r.matchesArticle(article)) {
			continue
		}
		if len(r.messages) == 0 {
			b.s.logger.Printf("notification-events: rule %d %q has no message; nothing sent", r.id, r.name)
			continue
		}
		if td == nil {
			if td, err = loadTemplateData(ctx, db, ticket, article, ev.userID, b.agent); err != nil {
				return queued, err
			}
		}
		recipients, err := b.recipients(ctx, r, ticket, ev.userID)
		if err != nil {
			return queued, fmt.Errorf("rule %d recipients: %w", r.id, err)
		}
		for _, rc := range recipients {
			sent, err := b.notify(ctx, r, matchedEvent[r.id], ticket, td, rc)
			if err != nil {
				return queued, fmt.Errorf("rule %d to %s: %w", r.id, rc.email, err)
			}
			if sent {
				queued++
			}
		}
	}
	return queued, nil
}

// agent returns the agent with id, cached for the batch; nil when the user
// does not exist.
func (b *batch) agent(ctx context.Context, id int) (*person, error) {
	if p, ok := b.agents[id]; ok {
		return p, nil
	}
	p, err := loadAgent(ctx, b.s.db, id)
	if err != nil {
		return nil, err
	}
	b.agents[id] = p
	return p, nil
}

// recipient is one address a rule notifies.
type recipient struct {
	*person
	customer bool // the ticket's customer user (SendCustomerNotification)
}

// recipients resolves the rule's recipient items for ticket. Agents must be
// valid, have an email address and at least read permission on the
// ticket's queue; like OTRS (AgentSelfNotifyOnAction off) the agent who
// caused the event and agents listed in SkipRecipients are left out.
// Addresses are deduplicated case-insensitively, agents first.
func (b *batch) recipients(ctx context.Context, r *rule, ticket *ticketData, actorID int) ([]recipient, error) {
	agentIDs := map[int]bool{}
	addAgents := func(ids []int) {
		for _, id := range ids {
			if id > 0 {
				agentIDs[id] = true
			}
		}
	}
	wantCustomer := false
	for _, v := range r.items["Recipients"] {
		switch v {
		case "AgentOwner":
			addAgents([]int{ticket.ownerID})
		case "AgentResponsible":
			addAgents([]int{ticket.responsibleID})
		case "AgentCreateBy":
			addAgents([]int{ticket.createBy})
		case "Customer":
			wantCustomer = true
		case "AgentWatcher":
			ids, err := b.queryIDs(ctx, `SELECT user_id FROM ticket_watcher WHERE ticket_id = ?`, ticket.id)
			if err != nil {
				return nil, err
			}
			addAgents(ids)
		case "AgentMyQueues":
			ids, err := b.queryIDs(ctx, `SELECT user_id FROM personal_queues WHERE queue_id = ?`, ticket.queueID)
			if err != nil {
				return nil, err
			}
			addAgents(ids)
		case "AgentMyServices":
			if ticket.serviceID > 0 {
				ids, err := b.queryIDs(ctx, `SELECT user_id FROM personal_services WHERE service_id = ?`, ticket.serviceID)
				if err != nil {
					return nil, err
				}
				addAgents(ids)
			}
		case "AgentWritePermissions", "AgentReadPermissions":
			perm := "ro"
			if v == "AgentWritePermissions" {
				perm = "rw"
			}
			ids, err := b.groupMembers(ctx, ticket.groupID, perm)
			if err != nil {
				return nil, err
			}
			addAgents(ids)
		default:
			b.s.logger.Printf("notification-events: rule %d %q: unknown recipient %q ignored", r.id, r.name, v)
		}
	}
	addAgents(atoiAll(r.items["RecipientAgents"]))
	for _, g := range atoiAll(r.items["RecipientGroups"]) {
		ids, err := b.groupMembers(ctx, g, "ro")
		if err != nil {
			return nil, err
		}
		addAgents(ids)
	}
	for _, role := range atoiAll(r.items["RecipientRoles"]) {
		ids, err := b.queryIDs(ctx, `
			SELECT ru.user_id FROM role_user ru JOIN roles r ON r.id = ru.role_id
			WHERE ru.role_id = ? AND r.valid_id = 1`, role)
		if err != nil {
			return nil, err
		}
		addAgents(ids)
	}
	for _, skip := range atoiAll(r.items["SkipRecipients"]) {
		delete(agentIDs, skip)
	}
	delete(agentIDs, actorID)

	var out []recipient
	seen := map[string]bool{}
	add := func(rc recipient) {
		addr := strings.ToLower(rc.email)
		if addr == "" || seen[addr] {
			return
		}
		langs := r.items["LanguageID"]
		if len(langs) > 0 && !containsString(langs, rc.language) {
			return
		}
		seen[addr] = true
		out = append(out, rc)
	}

	if len(agentIDs) > 0 {
		readers, err := b.groupMembers(ctx, ticket.groupID, "ro")
		if err != nil {
			return nil, err
		}
		canRead := map[int]bool{}
		for _, id := range readers {
			canRead[id] = true
		}
		ids := make([]int, 0, len(agentIDs))
		for id := range agentIDs {
			ids = append(ids, id)
		}
		sort.Ints(ids)
		for _, id := range ids {
			if !canRead[id] {
				continue
			}
			p, err := b.agent(ctx, id)
			if err != nil {
				return nil, err
			}
			if p != nil && p.valid {
				add(recipient{person: p})
			}
		}
	}
	if wantCustomer {
		c, err := loadCustomer(ctx, b.s.db, ticket.customerUserID)
		if err != nil {
			return nil, err
		}
		if c != nil && c.valid {
			add(recipient{person: c, customer: true})
		}
	}
	for _, value := range r.items["RecipientEmail"] {
		for _, addr := range splitRecipientEmail(value) {
			add(recipient{person: &person{email: addr, valid: true}})
		}
	}
	return out, nil
}

// groupMembers returns the agents with perm ("ro" or "rw") on group, directly
// or through a valid role; rw implies ro, as in OTRS.
func (b *batch) groupMembers(ctx context.Context, groupID int, perm string) ([]int, error) {
	return b.queryIDs(ctx, `
		SELECT gu.user_id FROM group_user gu
		JOIN `+"`groups`"+` g ON g.id = gu.group_id
		WHERE gu.group_id = ? AND g.valid_id = 1 AND (gu.permission_key = ? OR gu.permission_key = 'rw')
		UNION
		SELECT ru.user_id FROM role_user ru
		JOIN roles r ON r.id = ru.role_id
		JOIN group_role gr ON gr.role_id = ru.role_id
		JOIN `+"`groups`"+` g ON g.id = gr.group_id
		WHERE gr.group_id = ? AND r.valid_id = 1 AND g.valid_id = 1
			AND (gr.permission_key = ? OR gr.permission_key = 'rw') AND gr.permission_value = 1`,
		groupID, perm, groupID, perm)
}

func (b *batch) queryIDs(ctx context.Context, query string, args ...any) ([]int, error) {
	rows, err := b.s.db.QueryContext(ctx, database.ConvertPlaceholders(query), args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var ids []int
	for rows.Next() {
		var id int
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

// historyName is the ticket_history name OTRS writes for a sent
// notification: %%<notification>%%<recipient>%%Email.
func historyName(r *rule, rc recipient) string {
	who := rc.login
	if who == "" {
		who = rc.email
	}
	return truncateRunes("%%"+r.name+"%%"+who+"%%Email", 200)
}

// notify renders the rule's message for rc and queues it, with the OTRS
// SendAgentNotification/SendCustomerNotification history row. It returns
// false when OncePerDay suppressed the notification.
func (b *batch) notify(ctx context.Context, r *rule, eventName string, ticket *ticketData, td *templateData, rc recipient) (bool, error) {
	historyType := "SendAgentNotification"
	if rc.customer {
		historyType = "SendCustomerNotification"
	}
	name := historyName(r, rc)
	if r.oncePerDay() {
		var one int
		err := b.tx.QueryRowContext(ctx, database.ConvertPlaceholders(`
			SELECT 1 FROM ticket_history th
			JOIN ticket_history_type tht ON tht.id = th.history_type_id
			WHERE th.ticket_id = ? AND tht.name = ? AND th.name = ? AND th.create_time > ?
			LIMIT 1`), ticket.id, historyType, name, time.Now().Add(-24*time.Hour)).Scan(&one)
		if err == nil {
			return false, nil
		}
		if err != sql.ErrNoRows {
			return false, err
		}
	}

	msg, _ := r.messageFor(rc.language, defaultLanguage())
	isHTML := strings.Contains(strings.ToLower(msg.contentType), "html")
	subject := render(msg.subject, false, td, rc.person, eventName)
	body := render(msg.text, isHTML, td, rc.person, eventName)
	if err := queueMail(ctx, b.tx, rc, subject, body, isHTML); err != nil {
		return false, err
	}
	if err := addHistory(ctx, b.s.db, b.tx, ticket, historyType, name); err != nil {
		return false, err
	}
	return true, nil
}

func recipientEmailFields(value string) []string {
	return strings.FieldsFunc(value, func(r rune) bool {
		return r == ',' || r == ';' || r == ' ' || r == '\t' || r == '\n' || r == '\r'
	})
}

// splitRecipientEmail splits a RecipientEmail item value (addresses
// separated by commas, semicolons or whitespace) into addresses.
func splitRecipientEmail(value string) []string {
	fields := recipientEmailFields(value)
	out := make([]string, 0, len(fields))
	for _, f := range fields {
		if a, err := mail.ParseAddress(f); err == nil {
			out = append(out, a.Address)
		}
	}
	return out
}

// ValidateRecipientEmail checks a RecipientEmail value as the admin form
// stores it: bare addresses separated by commas, semicolons or whitespace,
// at most 200 characters (notification_event_item.event_value).
func ValidateRecipientEmail(value string) error {
	if len(value) > 200 {
		return fmt.Errorf("additional recipient addresses are longer than 200 characters")
	}
	for _, f := range recipientEmailFields(value) {
		if a, err := mail.ParseAddress(f); err != nil || a.Address != f {
			return fmt.Errorf("invalid recipient email address %q", f)
		}
	}
	return nil
}

func atoiAll(values []string) []int {
	out := make([]int, 0, len(values))
	for _, v := range values {
		if n, err := strconv.Atoi(strings.TrimSpace(v)); err == nil {
			out = append(out, n)
		}
	}
	return out
}

func truncateRunes(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n])
}
