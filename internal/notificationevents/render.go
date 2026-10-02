package notificationevents

import (
	"context"
	"database/sql"
	"html"
	"regexp"
	"strconv"
	"strings"
)

// templateData is what the OTRS tags of one event resolve to.
type templateData struct {
	ticket      *ticketData
	owner       *person
	responsible *person
	current     *person // agent who caused the event
	customer    *person // the ticket's customer user
	// Newest customer and agent articles: the event's article when it was
	// sent by that party, else the ticket's latest one.
	customerArticle *articleData
	agentArticle    *articleData
}

func loadTemplateData(ctx context.Context, db *sql.DB, t *ticketData, article *articleData, actorID int,
	agent func(context.Context, int) (*person, error)) (*templateData, error) {
	td := &templateData{ticket: t}
	var err error
	if td.owner, err = agent(ctx, t.ownerID); err != nil {
		return nil, err
	}
	if td.responsible, err = agent(ctx, t.responsibleID); err != nil {
		return nil, err
	}
	if td.current, err = agent(ctx, actorID); err != nil {
		return nil, err
	}
	if td.customer, err = loadCustomer(ctx, db, t.customerUserID); err != nil {
		return nil, err
	}
	for _, side := range []struct {
		sender string
		dst    **articleData
	}{{"customer", &td.customerArticle}, {"agent", &td.agentArticle}} {
		if article != nil && article.senderType == side.sender {
			*side.dst = article
			continue
		}
		if *side.dst, err = latestArticle(ctx, db, t.id, side.sender); err != nil {
			return nil, err
		}
	}
	return td, nil
}

// tagPattern matches <OTRS_...> and <GOATFLOW_...> tags, raw or HTML-escaped
// (rich-text editors store &lt;OTRS_...&gt;), with an optional [n] line limit.
var tagPattern = regexp.MustCompile(`(?:<|&lt;)(?:OTRS|GOATFLOW)_([A-Za-z0-9_]+?)(?:\[(\d+)\])?(?:>|&gt;)`)

// render replaces the OTRS tags in text. Values are HTML-escaped when the
// message is HTML; tags that do not resolve become "-", as in OTRS.
func render(text string, isHTML bool, td *templateData, rc *person, eventName string) string {
	return tagPattern.ReplaceAllStringFunc(text, func(tag string) string {
		m := tagPattern.FindStringSubmatch(tag)
		name := m[1]
		lines := 0
		if m[2] != "" {
			lines, _ = strconv.Atoi(m[2])
		}
		value, multiline, ok := td.lookup(name, lines, rc, eventName)
		if !ok {
			value = "-"
		}
		if !isHTML {
			if !multiline {
				value = strings.NewReplacer("\r", " ", "\n", " ").Replace(value)
			}
			return value
		}
		value = html.EscapeString(value)
		if multiline {
			value = strings.ReplaceAll(strings.ReplaceAll(value, "\r\n", "\n"), "\n", "<br/>\n")
		}
		return value
	})
}

// lookup resolves one tag name (without the OTRS_ prefix). multiline is true
// for article bodies, whose line breaks survive.
func (td *templateData) lookup(name string, lines int, rc *person, eventName string) (value string, multiline, ok bool) {
	t := td.ticket
	switch {
	case name == "TICKET_Owner" || name == "TICKET_Responsible":
		p := td.owner
		if name == "TICKET_Responsible" {
			p = td.responsible
		}
		if p == nil {
			return "", false, false
		}
		return p.login, false, true
	case strings.HasPrefix(name, "TICKET_"):
		v, ok := ticketField(t, strings.TrimPrefix(name, "TICKET_"))
		return v, false, ok
	case strings.HasPrefix(name, "OWNER_"):
		v, ok := personField(td.owner, strings.TrimPrefix(name, "OWNER_"))
		return v, false, ok
	case strings.HasPrefix(name, "RESPONSIBLE_"):
		v, ok := personField(td.responsible, strings.TrimPrefix(name, "RESPONSIBLE_"))
		return v, false, ok
	case strings.HasPrefix(name, "CURRENT_"):
		v, ok := personField(td.current, strings.TrimPrefix(name, "CURRENT_"))
		return v, false, ok
	case strings.HasPrefix(name, "NOTIFICATION_RECIPIENT_"):
		v, ok := personField(rc, strings.TrimPrefix(name, "NOTIFICATION_RECIPIENT_"))
		return v, false, ok
	case strings.HasPrefix(name, "CUSTOMER_DATA_"):
		v, ok := personField(td.customer, strings.TrimPrefix(name, "CUSTOMER_DATA_"))
		return v, false, ok
	case strings.HasPrefix(name, "CUSTOMER_"):
		return articleField(td.customerArticle, td.customer, strings.TrimPrefix(name, "CUSTOMER_"), lines)
	case strings.HasPrefix(name, "AGENT_"):
		return articleField(td.agentArticle, nil, strings.TrimPrefix(name, "AGENT_"), lines)
	case name == "EVENT":
		return eventName, false, true
	}
	return "", false, false
}

func ticketField(t *ticketData, field string) (string, bool) {
	itoa := strconv.Itoa
	switch field {
	case "TicketID":
		return strconv.FormatInt(t.id, 10), true
	case "TicketNumber":
		return t.tn, true
	case "Title":
		return t.title, true
	case "Queue":
		return t.queue, true
	case "QueueID":
		return itoa(t.queueID), true
	case "State":
		return t.state, true
	case "StateID":
		return itoa(t.stateID), true
	case "StateType":
		return t.stateType, true
	case "Priority":
		return t.priority, true
	case "PriorityID":
		return itoa(t.priorityID), true
	case "Type":
		return t.typeName, true
	case "TypeID":
		return itoa(t.typeID), true
	case "Lock":
		return t.lock, true
	case "LockID":
		return itoa(t.lockID), true
	case "Service":
		return t.service, true
	case "SLA":
		return t.sla, true
	case "OwnerID":
		return itoa(t.ownerID), true
	case "ResponsibleID":
		return itoa(t.responsibleID), true
	case "CustomerID":
		return t.customerID, true
	case "CustomerUserID":
		return t.customerUserID, true
	case "Created":
		return t.created.Format("2006-01-02 15:04:05"), true
	case "Changed":
		return t.changed.Format("2006-01-02 15:04:05"), true
	}
	return "", false
}

// personField resolves the OTRS user attributes (UserFirstname, ...).
func personField(p *person, field string) (string, bool) {
	if p == nil {
		return "", false
	}
	switch field {
	case "UserFirstname":
		return p.firstName, true
	case "UserLastname":
		return p.lastName, true
	case "UserFullname":
		return p.fullName(), true
	case "UserLogin":
		return p.login, true
	case "UserEmail":
		return p.email, true
	case "UserTitle":
		return p.title, true
	case "UserCustomerID":
		return p.customerID, p.customerID != ""
	case "UserPhone":
		return p.phone, p.phone != ""
	case "UserLanguage":
		return p.language, true
	}
	return "", false
}

// articleField resolves <OTRS_CUSTOMER_...> / <OTRS_AGENT_...> tags: the
// article's SUBJECT, BODY (first n lines with [n]), EMAIL (the body quoted
// with "> "), From, To, Cc and Subject; for CUSTOMER_ also REALNAME and the
// customer user's User* attributes (as GoatFlow's response templates use them).
func articleField(a *articleData, customer *person, field string, lines int) (string, bool, bool) {
	if strings.HasPrefix(field, "User") {
		v, ok := personField(customer, field)
		return v, false, ok
	}
	if field == "REALNAME" && customer != nil {
		return customer.fullName(), false, true
	}
	if a == nil {
		return "", false, false
	}
	switch field {
	case "SUBJECT", "Subject":
		return a.subject, false, true
	case "From":
		return a.from, false, true
	case "To":
		return a.to, false, true
	case "Cc":
		return a.cc, false, true
	case "BODY", "Body":
		return firstLines(a.body, lines), true, true
	case "EMAIL":
		body := firstLines(a.body, lines)
		quoted := strings.Split(body, "\n")
		for i, l := range quoted {
			quoted[i] = "> " + l
		}
		return strings.Join(quoted, "\n"), true, true
	}
	return "", false, false
}

func firstLines(s string, n int) string {
	s = strings.TrimRight(strings.ReplaceAll(s, "\r\n", "\n"), "\n")
	if n <= 0 {
		return s
	}
	parts := strings.SplitN(s, "\n", n+1)
	if len(parts) > n {
		parts = parts[:n]
	}
	return strings.Join(parts, "\n")
}
