package api

import (
	"database/sql"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/goatkit/goatflow/internal/platform/database"
	"github.com/goatkit/goatflow/internal/platform/lookups"
	"github.com/goatkit/goatflow/internal/platform/routing"
)

func init() {
	routing.RegisterHandler("HandleTicketSearchAPI", HandleTicketSearchAPI)
	routing.RegisterHandler("HandleListSavedSearchesAPI", HandleListSavedSearchesAPI)
	routing.RegisterHandler("HandleCreateSavedSearchAPI", HandleCreateSavedSearchAPI)
	routing.RegisterHandler("HandleGetSavedSearchAPI", HandleGetSavedSearchAPI)
	routing.RegisterHandler("HandleUpdateSavedSearchAPI", HandleUpdateSavedSearchAPI)
	routing.RegisterHandler("HandleDeleteSavedSearchAPI", HandleDeleteSavedSearchAPI)
	routing.RegisterHandler("HandleExecuteSavedSearchAPI", HandleExecuteSavedSearchAPI)
}

// Agent ticket search (GET /api/v1/search/tickets) and saved searches
// (/api/v1/search/saved...). Saved searches use the OTRS search_profile table
// the way Kernel::System::SearchProfile does: login is "TicketSearch::<agent
// login>", one row per value, profile_type SCALAR or ARRAY, profile_key the
// OTRS ticket search attribute name.

const (
	ticketSearchProfileBase = "TicketSearch"
	searchProfileFieldMax   = 200 // search_profile login/profile_name/profile_key/profile_value are varchar(200)
	ticketSearchDefaultSize = 25
	ticketSearchMaxSize     = 100
)

type searchParamKind int

const (
	searchParamText searchParamKind = iota // substring match, '*' is a wildcard
	searchParamIDs                         // positive integers, any of
	searchParamList                        // strings, any of
	searchParamDate                        // YYYY-MM-DD, "YYYY-MM-DD HH:MM:SS" or RFC 3339
)

// ticketSearchParam maps an API parameter to its OTRS search profile key.
type ticketSearchParam struct {
	name string
	key  string
	kind searchParamKind
}

var ticketSearchParams = []ticketSearchParam{
	{"q", "Fulltext", searchParamText},
	{"tn", "TicketNumber", searchParamText},
	{"title", "Title", searchParamText},
	{"customer_id", "CustomerID", searchParamText},
	{"customer_user_login", "CustomerUserLogin", searchParamText},
	{"from", "From", searchParamText},
	{"to", "To", searchParamText},
	{"subject", "Subject", searchParamText},
	{"body", "Body", searchParamText},
	{"state_type", "StateType", searchParamList},
	{"state_id", "StateIDs", searchParamIDs},
	{"queue_id", "QueueIDs", searchParamIDs},
	{"priority_id", "PriorityIDs", searchParamIDs},
	{"type_id", "TypeIDs", searchParamIDs},
	{"owner_id", "OwnerIDs", searchParamIDs},
	{"responsible_id", "ResponsibleIDs", searchParamIDs},
	{"lock_id", "LockIDs", searchParamIDs},
	{"created_after", "TicketCreateTimeNewerDate", searchParamDate},
	{"created_before", "TicketCreateTimeOlderDate", searchParamDate},
	{"changed_after", "TicketChangeTimeNewerDate", searchParamDate},
	{"changed_before", "TicketChangeTimeOlderDate", searchParamDate},
}

func ticketSearchParamByName(name string) (ticketSearchParam, bool) {
	for _, p := range ticketSearchParams {
		if p.name == name {
			return p, true
		}
	}
	return ticketSearchParam{}, false
}

func ticketSearchParamByKey(key string) (ticketSearchParam, bool) {
	for _, p := range ticketSearchParams {
		if p.key == key {
			return p, true
		}
	}
	return ticketSearchParam{}, false
}

// ticketSearchPagingParams are query parameters that control the result page,
// not the criteria; they are never stored in a saved search.
var ticketSearchPagingParams = map[string]bool{"page": true, "limit": true, "sort": true, "order": true}

// ticketSearchCriteria is a parsed, validated set of search parameters keyed by
// API parameter name. Values keep their input form so saved searches store
// exactly what was given.
type ticketSearchCriteria map[string][]string

// parseTicketSearchCriteria validates raw values (API parameter name -> values).
// Comma-separated values are split for ID and list parameters; empty values are
// dropped. Unknown names are an error.
func parseTicketSearchCriteria(raw map[string][]string) (ticketSearchCriteria, error) {
	out := ticketSearchCriteria{}
	for name, values := range raw {
		p, ok := ticketSearchParamByName(name)
		if !ok {
			return nil, fmt.Errorf("unknown search parameter %q", name)
		}
		var cleaned []string
		for _, v := range values {
			parts := []string{v}
			if p.kind == searchParamIDs || p.kind == searchParamList {
				parts = strings.Split(v, ",")
			}
			for _, part := range parts {
				part = strings.TrimSpace(part)
				if part == "" {
					continue
				}
				if len(part) > searchProfileFieldMax {
					return nil, fmt.Errorf("%s: value longer than %d characters", name, searchProfileFieldMax)
				}
				switch p.kind {
				case searchParamIDs:
					if id, err := strconv.ParseInt(part, 10, 64); err != nil || id <= 0 {
						return nil, fmt.Errorf("%s: %q is not a positive integer", name, part)
					}
				case searchParamDate:
					if _, _, err := parseSearchTime(part); err != nil {
						return nil, fmt.Errorf("%s: %q is not a date (YYYY-MM-DD) or timestamp", name, part)
					}
				}
				cleaned = append(cleaned, part)
			}
		}
		if len(cleaned) == 0 {
			continue
		}
		if (p.kind == searchParamText || p.kind == searchParamDate) && len(cleaned) > 1 {
			return nil, fmt.Errorf("%s: only one value allowed", name)
		}
		out[name] = cleaned
	}
	return out, nil
}

// parseSearchTime parses a date or timestamp; dateOnly reports a bare date.
func parseSearchTime(v string) (t time.Time, dateOnly bool, err error) {
	if t, err = time.Parse("2006-01-02", v); err == nil {
		return t, true, nil
	}
	for _, layout := range []string{time.RFC3339, "2006-01-02 15:04:05"} {
		if t, err = time.Parse(layout, v); err == nil {
			return t.UTC(), false, nil
		}
	}
	return time.Time{}, false, err
}

// likePattern turns user text into a lower-case LIKE pattern: '*' is a
// wildcard, '%', '_' and '\' match literally, and text without '*' matches
// anywhere in the column.
func likePattern(v string) string {
	r := strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`)
	v = strings.ToLower(r.Replace(v))
	if strings.Contains(v, "*") {
		return strings.ReplaceAll(v, "*", "%")
	}
	return "%" + v + "%"
}

// articleMatchSQL matches tickets with at least one article whose MIME column
// matches the following LIKE argument(s).
func articleMatchSQL(columns ...string) string {
	conds := make([]string, len(columns))
	for i, col := range columns {
		conds[i] = "LOWER(m." + col + ") LIKE ?"
	}
	return "EXISTS (SELECT 1 FROM article a JOIN article_data_mime m ON m.article_id = a.id WHERE a.ticket_id = t.id AND (" +
		strings.Join(conds, " OR ") + "))"
}

func placeholders(n int) string {
	return strings.TrimSuffix(strings.Repeat("?, ", n), ", ")
}

// where builds the WHERE clause (without the keyword) over ticket alias t.
func (cr ticketSearchCriteria) where(scope statsScope) (string, []interface{}) {
	scopeSQL, scopeArgs := scope.filter("t.queue_id")
	conds := []string{scopeSQL}
	args := append([]interface{}{}, scopeArgs...)

	text := func(sqlText string, n int, v string) {
		conds = append(conds, sqlText)
		pattern := likePattern(v)
		for range n {
			args = append(args, pattern)
		}
	}
	for _, p := range ticketSearchParams {
		values, ok := cr[p.name]
		if !ok {
			continue
		}
		switch p.name {
		case "q":
			for _, term := range strings.Fields(values[0]) {
				text("(LOWER(t.tn) LIKE ? OR LOWER(t.title) LIKE ? OR LOWER(t.customer_id) LIKE ? OR LOWER(t.customer_user_id) LIKE ?"+
					" OR t.customer_user_id IN (SELECT cu.login FROM customer_user cu WHERE LOWER(cu.email) LIKE ? OR LOWER(cu.first_name) LIKE ? OR LOWER(cu.last_name) LIKE ?)"+
					" OR t.customer_id IN (SELECT cc.customer_id FROM customer_company cc WHERE LOWER(cc.name) LIKE ?)"+
					" OR "+articleMatchSQL("a_subject", "a_body", "a_from", "a_to")+")", 12, term)
			}
		case "tn":
			text("LOWER(t.tn) LIKE ?", 1, values[0])
		case "title":
			text("LOWER(t.title) LIKE ?", 1, values[0])
		case "customer_id":
			text("LOWER(t.customer_id) LIKE ?", 1, values[0])
		case "customer_user_login":
			text("LOWER(t.customer_user_id) LIKE ?", 1, values[0])
		case "from":
			text(articleMatchSQL("a_from"), 1, values[0])
		case "to":
			text(articleMatchSQL("a_to", "a_cc"), 2, values[0])
		case "subject":
			text(articleMatchSQL("a_subject"), 1, values[0])
		case "body":
			text(articleMatchSQL("a_body"), 1, values[0])
		case "state_type":
			var names []interface{}
			for _, v := range values {
				if strings.EqualFold(v, "pending") {
					names = append(names, lookups.StateTypePendingReminder, lookups.StateTypePendingAuto)
					continue
				}
				names = append(names, strings.ToLower(v))
			}
			conds = append(conds, "t.ticket_state_id IN (SELECT s.id FROM ticket_state s JOIN ticket_state_type st ON st.id = s.type_id WHERE st.name IN ("+placeholders(len(names))+"))")
			args = append(args, names...)
		case "created_after", "created_before", "changed_after", "changed_before":
			col := "t.create_time"
			if strings.HasPrefix(p.name, "changed") {
				col = "t.change_time"
			}
			ts, dateOnly, _ := parseSearchTime(values[0]) // validated by parseTicketSearchCriteria
			switch {
			case strings.HasSuffix(p.name, "_after"):
				conds = append(conds, col+" >= ?")
			case dateOnly: // the whole day is included
				conds = append(conds, col+" < ?")
				ts = ts.AddDate(0, 0, 1)
			default:
				conds = append(conds, col+" <= ?")
			}
			args = append(args, ts)
		default: // ID lists
			col := map[string]string{
				"state_id": "t.ticket_state_id", "queue_id": "t.queue_id", "priority_id": "t.ticket_priority_id",
				"type_id": "t.type_id", "owner_id": "t.user_id", "responsible_id": "t.responsible_user_id",
				"lock_id": "t.ticket_lock_id",
			}[p.name]
			conds = append(conds, col+" IN ("+placeholders(len(values))+")")
			for _, v := range values {
				id, _ := strconv.ParseInt(v, 10, 64) // validated by parseTicketSearchCriteria
				args = append(args, id)
			}
		}
	}
	return strings.Join(conds, " AND "), args
}

// ticketSearchPage holds paging and ordering for one search request.
type ticketSearchPage struct {
	page, limit int
	orderBy     string
}

func parseTicketSearchPage(c *gin.Context) (ticketSearchPage, error) {
	pg := ticketSearchPage{page: 1, limit: ticketSearchDefaultSize}
	if v := c.Query("page"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 1 {
			return pg, errors.New("page must be a positive integer")
		}
		pg.page = n
	}
	if v := c.Query("limit"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 1 || n > ticketSearchMaxSize {
			return pg, fmt.Errorf("limit must be an integer between 1 and %d", ticketSearchMaxSize)
		}
		pg.limit = n
	}
	column, ok := map[string]string{
		"created": "t.create_time", "changed": "t.change_time", "priority": "t.ticket_priority_id",
		"tn": "t.tn", "title": "t.title",
	}[c.DefaultQuery("sort", "created")]
	if !ok {
		return pg, errors.New("sort must be one of created, changed, priority, tn, title")
	}
	order := strings.ToUpper(c.DefaultQuery("order", "desc"))
	if order != "ASC" && order != "DESC" {
		return pg, errors.New("order must be asc or desc")
	}
	pg.orderBy = column + " " + order + ", t.id " + order
	return pg, nil
}

// runTicketSearch executes a search and returns the result page and the total.
func runTicketSearch(db *sql.DB, scope statsScope, cr ticketSearchCriteria, pg ticketSearchPage) ([]gin.H, int, error) {
	where, args := cr.where(scope)

	var total int
	if err := db.QueryRow(database.ConvertPlaceholders("SELECT COUNT(*) FROM ticket t WHERE "+where), args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count: %w", err)
	}

	query := `SELECT t.id, t.tn, t.title, t.queue_id, q.name, t.ticket_state_id, s.name, st.name,
			t.ticket_priority_id, p.name, t.type_id, t.user_id, t.responsible_user_id, t.ticket_lock_id,
			t.customer_id, t.customer_user_id, t.create_time, t.change_time
		FROM ticket t
		JOIN queue q ON q.id = t.queue_id
		JOIN ticket_state s ON s.id = t.ticket_state_id
		JOIN ticket_state_type st ON st.id = s.type_id
		JOIN ticket_priority p ON p.id = t.ticket_priority_id
		WHERE ` + where + `
		ORDER BY ` + pg.orderBy + `
		LIMIT ? OFFSET ?`
	rows, err := db.Query(database.ConvertPlaceholders(query), append(args, pg.limit, (pg.page-1)*pg.limit)...)
	if err != nil {
		return nil, 0, fmt.Errorf("query: %w", err)
	}
	defer rows.Close()

	tickets := []gin.H{}
	for rows.Next() {
		var (
			id                                                        int64
			tn, title, queue, state, stateType, priority              string
			queueID, stateID, priorityID, ownerID, responsibleID, lck int64
			typeID                                                    sql.NullInt64
			customerID, customerUser                                  sql.NullString
			created, changed                                          time.Time
		)
		if err := rows.Scan(&id, &tn, &title, &queueID, &queue, &stateID, &state, &stateType, &priorityID, &priority,
			&typeID, &ownerID, &responsibleID, &lck, &customerID, &customerUser, &created, &changed); err != nil {
			return nil, 0, fmt.Errorf("scan: %w", err)
		}
		ticket := gin.H{
			"id": id, "tn": tn, "title": title,
			"queue_id": queueID, "queue": queue,
			"state_id": stateID, "state": state, "state_type": stateType,
			"priority_id": priorityID, "priority": priority,
			"type_id": nil, "owner_id": ownerID, "responsible_id": responsibleID, "lock_id": lck,
			"customer_id": customerID.String, "customer_user_login": customerUser.String,
			"created_at": created, "updated_at": changed,
		}
		if typeID.Valid {
			ticket["type_id"] = typeID.Int64
		}
		tickets = append(tickets, ticket)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("iterate: %w", err)
	}
	return tickets, total, nil
}

func ticketSearchResponse(tickets []gin.H, total int, pg ticketSearchPage) gin.H {
	return gin.H{
		"tickets":    tickets,
		"pagination": gin.H{"page": pg.page, "limit": pg.limit, "total": total},
	}
}

// ticketSearchAgent authenticates a search request: agents only (customer
// tokens get 403), results limited to the queues the agent may read.
func ticketSearchAgent(c *gin.Context) (*sql.DB, statsScope, int, bool) {
	userID := extractUserIDForRBAC(c)
	if userID == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"success": false, "error": "Unauthorized"})
		return nil, statsScope{}, 0, false
	}
	if isCustomer, _ := c.Get("is_customer"); isCustomer == true || c.GetString("user_role") == "Customer" {
		c.JSON(http.StatusForbidden, gin.H{"success": false, "error": "Ticket search is only available to agents"})
		return nil, statsScope{}, 0, false
	}
	db, err := database.GetDB()
	if err != nil || db == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "Database connection failed"})
		return nil, statsScope{}, 0, false
	}
	scope, err := resolveStatsScope(c, db, userID)
	if err != nil {
		log.Printf("ticket search: resolving queue access for user %d: %v", userID, err)
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "Failed to check permissions"})
		return nil, statsScope{}, 0, false
	}
	return db, scope, userID, true
}

func searchBadRequest(c *gin.Context, err error) {
	c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": err.Error()})
}

func searchServerError(c *gin.Context, what string, err error) {
	log.Printf("ticket search: %s: %v", what, err)
	c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "Ticket search failed"})
}

// HandleTicketSearchAPI searches tickets in the caller's readable queues.
//
//	@Summary		Search tickets
//	@Description	Agent ticket search over ticket, article and customer data. Text parameters match case-insensitively anywhere ('*' is a wildcard); ID and state_type parameters take comma-separated or repeated values; created_/changed_ take YYYY-MM-DD (whole day) or timestamps.
//	@Tags			Search
//	@Produce		json
//	@Param			q					query	string	false	"Words that must each match ticket number, title, customer, customer user (login, name, email), company name, or an article's subject, body, from or to"
//	@Param			tn					query	string	false	"Ticket number"
//	@Param			title				query	string	false	"Ticket title"
//	@Param			customer_id			query	string	false	"Customer (company) ID"
//	@Param			customer_user_login	query	string	false	"Customer user login"
//	@Param			from				query	string	false	"Article From"
//	@Param			to					query	string	false	"Article To or Cc"
//	@Param			subject				query	string	false	"Article subject"
//	@Param			body				query	string	false	"Article body"
//	@Param			state_type			query	string	false	"State types (new, open, pending, pending reminder, pending auto, closed, merged, removed)"
//	@Param			state_id			query	string	false	"State IDs"
//	@Param			queue_id			query	string	false	"Queue IDs"
//	@Param			priority_id			query	string	false	"Priority IDs"
//	@Param			type_id				query	string	false	"Ticket type IDs"
//	@Param			owner_id			query	string	false	"Owner agent IDs"
//	@Param			responsible_id		query	string	false	"Responsible agent IDs"
//	@Param			lock_id				query	string	false	"Lock IDs"
//	@Param			created_after		query	string	false	"Created at or after"
//	@Param			created_before		query	string	false	"Created before (date: up to the end of that day)"
//	@Param			changed_after		query	string	false	"Changed at or after"
//	@Param			changed_before		query	string	false	"Changed before (date: up to the end of that day)"
//	@Param			sort				query	string	false	"created, changed, priority, tn, title"	default(created)
//	@Param			order				query	string	false	"asc or desc"							default(desc)
//	@Param			page				query	int		false	"Page"									default(1)
//	@Param			limit				query	int		false	"Page size (max 100)"					default(25)
//	@Success		200	{object}	map[string]interface{}	"{success, data: {tickets, pagination: {page, limit, total}}}"
//	@Failure		400	{object}	map[string]interface{}	"Invalid parameter"
//	@Failure		401	{object}	map[string]interface{}	"Unauthorized"
//	@Failure		403	{object}	map[string]interface{}	"Customer, or agent without queue access"
//	@Security		BearerAuth
//	@Router			/search/tickets [get]
func HandleTicketSearchAPI(c *gin.Context) {
	db, scope, _, ok := ticketSearchAgent(c)
	if !ok {
		return
	}
	raw := map[string][]string{}
	for name, values := range c.Request.URL.Query() {
		if !ticketSearchPagingParams[name] {
			raw[name] = values
		}
	}
	criteria, err := parseTicketSearchCriteria(raw)
	if err != nil {
		searchBadRequest(c, err)
		return
	}
	pg, err := parseTicketSearchPage(c)
	if err != nil {
		searchBadRequest(c, err)
		return
	}
	tickets, total, err := runTicketSearch(db, scope, criteria, pg)
	if err != nil {
		searchServerError(c, "search", err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": ticketSearchResponse(tickets, total, pg)})
}

// searchProfileLogin returns the search_profile.login of the calling agent.
func searchProfileLogin(c *gin.Context, db *sql.DB, userID int) (string, bool) {
	var login string
	err := db.QueryRow(database.ConvertPlaceholders("SELECT login FROM users WHERE id = ?"), userID).Scan(&login)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			c.JSON(http.StatusUnauthorized, gin.H{"success": false, "error": "Unauthorized"})
		} else {
			searchServerError(c, "load agent login", err)
		}
		return "", false
	}
	return ticketSearchProfileBase + "::" + login, true
}

// savedSearch is a search_profile entry: known parameters plus the keys of any
// stored attribute this API does not search on (e.g. OTRS result-form settings).
type savedSearch struct {
	criteria ticketSearchCriteria
	ignored  []string
}

// loadSavedSearch reads a profile; found is false when it has no rows.
func loadSavedSearch(db *sql.DB, login, name string) (savedSearch, bool, error) {
	rows, err := db.Query(database.ConvertPlaceholders(
		"SELECT profile_key, profile_value FROM search_profile WHERE login = ? AND profile_name = ?"), login, name)
	if err != nil {
		return savedSearch{}, false, err
	}
	defer rows.Close()
	raw := map[string][]string{}
	ignored := map[string]bool{}
	found := false
	for rows.Next() {
		var key string
		var value sql.NullString
		if err := rows.Scan(&key, &value); err != nil {
			return savedSearch{}, false, err
		}
		found = true
		p, ok := ticketSearchParamByKey(key)
		if !ok {
			ignored[key] = true
			continue
		}
		if value.Valid && value.String != "" {
			raw[p.name] = append(raw[p.name], value.String)
		}
	}
	if err := rows.Err(); err != nil {
		return savedSearch{}, false, err
	}
	ss := savedSearch{criteria: ticketSearchCriteria{}, ignored: []string{}}
	for key := range ignored {
		ss.ignored = append(ss.ignored, key)
	}
	// Stored values were validated on save; values written by other tools
	// that do not parse are reported instead of silently widening the search.
	for name, values := range raw {
		cr, err := parseTicketSearchCriteria(map[string][]string{name: values})
		if err != nil {
			p, _ := ticketSearchParamByName(name)
			ss.ignored = append(ss.ignored, p.key)
			continue
		}
		for k, v := range cr {
			ss.criteria[k] = v
		}
	}
	return ss, found, nil
}

// storeSavedSearch replaces the rows of a profile inside tx.
func storeSavedSearch(tx *sql.Tx, login, name string, cr ticketSearchCriteria) error {
	if _, err := tx.Exec(database.ConvertPlaceholders(
		"DELETE FROM search_profile WHERE login = ? AND profile_name = ?"), login, name); err != nil {
		return err
	}
	insert := database.ConvertPlaceholders(
		"INSERT INTO search_profile (login, profile_name, profile_type, profile_key, profile_value) VALUES (?, ?, ?, ?, ?)")
	for _, p := range ticketSearchParams {
		values, ok := cr[p.name]
		if !ok {
			continue
		}
		profileType := "SCALAR"
		if p.kind == searchParamIDs || p.kind == searchParamList {
			profileType = "ARRAY"
		}
		for _, v := range values {
			if _, err := tx.Exec(insert, login, name, profileType, p.key, v); err != nil {
				return err
			}
		}
	}
	return nil
}

// parametersJSON renders criteria for responses: list parameters as arrays,
// the others as strings.
func (cr ticketSearchCriteria) parametersJSON() gin.H {
	out := gin.H{}
	for name, values := range cr {
		p, _ := ticketSearchParamByName(name)
		if p.kind == searchParamIDs || p.kind == searchParamList {
			out[name] = values
		} else {
			out[name] = values[0]
		}
	}
	return out
}

// savedSearchBody is the request body of create and update. Parameter values
// are a string or an array of strings.
type savedSearchBody struct {
	Name       string                 `json:"name"`
	Type       string                 `json:"type"`
	Parameters map[string]interface{} `json:"parameters"`
}

func (b savedSearchBody) criteria() (ticketSearchCriteria, error) {
	raw := map[string][]string{}
	for name, v := range b.Parameters {
		switch val := v.(type) {
		case string:
			raw[name] = []string{val}
		case float64:
			raw[name] = []string{strconv.FormatFloat(val, 'f', -1, 64)}
		case []interface{}:
			for _, item := range val {
				switch s := item.(type) {
				case string:
					raw[name] = append(raw[name], s)
				case float64:
					raw[name] = append(raw[name], strconv.FormatFloat(s, 'f', -1, 64))
				default:
					return nil, fmt.Errorf("%s: values must be strings or numbers", name)
				}
			}
		default:
			return nil, fmt.Errorf("%s: value must be a string, number or array", name)
		}
	}
	cr, err := parseTicketSearchCriteria(raw)
	if err != nil {
		return nil, err
	}
	if len(cr) == 0 {
		return nil, errors.New("parameters must contain at least one search parameter")
	}
	return cr, nil
}

func validSavedSearchName(name string) error {
	if strings.TrimSpace(name) == "" || len(name) > searchProfileFieldMax {
		return fmt.Errorf("name must be 1 to %d characters", searchProfileFieldMax)
	}
	return nil
}

// HandleListSavedSearchesAPI lists the caller's saved ticket searches.
//
//	@Summary		List saved searches
//	@Tags			Search
//	@Produce		json
//	@Success		200	{object}	map[string]interface{}	"{success, data: {searches: [{name, type}], total}}"
//	@Security		BearerAuth
//	@Router			/search/saved [get]
func HandleListSavedSearchesAPI(c *gin.Context) {
	db, _, userID, ok := ticketSearchAgent(c)
	if !ok {
		return
	}
	login, ok := searchProfileLogin(c, db, userID)
	if !ok {
		return
	}
	rows, err := db.Query(database.ConvertPlaceholders(
		"SELECT DISTINCT profile_name FROM search_profile WHERE login = ? ORDER BY profile_name"), login)
	if err != nil {
		searchServerError(c, "list saved searches", err)
		return
	}
	defer rows.Close()
	searches := []gin.H{}
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			searchServerError(c, "list saved searches", err)
			return
		}
		searches = append(searches, gin.H{"name": name, "type": ticketSearchProfileBase})
	}
	if err := rows.Err(); err != nil {
		searchServerError(c, "list saved searches", err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": gin.H{"searches": searches, "total": len(searches)}})
}

// HandleCreateSavedSearchAPI saves a ticket search under a new name.
//
//	@Summary		Create saved search
//	@Tags			Search
//	@Accept			json
//	@Produce		json
//	@Param			search	body	object	true	"{name, type: TicketSearch (optional), parameters: {<search parameter>: string | [string]}}"
//	@Success		201	{object}	map[string]interface{}	"{success, data: {name, type, parameters}}"
//	@Failure		400	{object}	map[string]interface{}	"Invalid name or parameters"
//	@Failure		409	{object}	map[string]interface{}	"Name already used"
//	@Security		BearerAuth
//	@Router			/search/saved [post]
func HandleCreateSavedSearchAPI(c *gin.Context) {
	db, _, userID, ok := ticketSearchAgent(c)
	if !ok {
		return
	}
	var body savedSearchBody
	if err := c.ShouldBindJSON(&body); err != nil {
		searchBadRequest(c, fmt.Errorf("invalid JSON body: %w", err))
		return
	}
	if err := validSavedSearchName(body.Name); err != nil {
		searchBadRequest(c, err)
		return
	}
	if body.Type != "" && body.Type != ticketSearchProfileBase {
		searchBadRequest(c, fmt.Errorf("type must be %s", ticketSearchProfileBase))
		return
	}
	cr, err := body.criteria()
	if err != nil {
		searchBadRequest(c, err)
		return
	}
	login, ok := searchProfileLogin(c, db, userID)
	if !ok {
		return
	}

	tx, err := db.Begin()
	if err != nil {
		searchServerError(c, "begin", err)
		return
	}
	defer func() { _ = tx.Rollback() }() //nolint:errcheck // no-op after commit
	var exists int
	if err := tx.QueryRow(database.ConvertPlaceholders(
		"SELECT COUNT(*) FROM search_profile WHERE login = ? AND profile_name = ?"), login, body.Name).Scan(&exists); err != nil {
		searchServerError(c, "check saved search", err)
		return
	}
	if exists > 0 {
		c.JSON(http.StatusConflict, gin.H{"success": false, "error": "A saved search with this name already exists"})
		return
	}
	if err := storeSavedSearch(tx, login, body.Name, cr); err != nil {
		searchServerError(c, "store saved search", err)
		return
	}
	if err := tx.Commit(); err != nil {
		searchServerError(c, "commit", err)
		return
	}
	c.JSON(http.StatusCreated, gin.H{"success": true, "data": gin.H{
		"name": body.Name, "type": ticketSearchProfileBase, "parameters": cr.parametersJSON(),
	}})
}

// savedSearchForRequest loads the saved search named in the path for the
// caller; on failure the response is written.
func savedSearchForRequest(c *gin.Context) (*sql.DB, statsScope, string, string, savedSearch, bool) {
	db, scope, userID, ok := ticketSearchAgent(c)
	if !ok {
		return nil, statsScope{}, "", "", savedSearch{}, false
	}
	login, ok := searchProfileLogin(c, db, userID)
	if !ok {
		return nil, statsScope{}, "", "", savedSearch{}, false
	}
	name := c.Param("name")
	ss, found, err := loadSavedSearch(db, login, name)
	if err != nil {
		searchServerError(c, "load saved search", err)
		return nil, statsScope{}, "", "", savedSearch{}, false
	}
	if !found {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "error": "Saved search not found"})
		return nil, statsScope{}, "", "", savedSearch{}, false
	}
	return db, scope, login, name, ss, true
}

// HandleGetSavedSearchAPI returns one saved search.
//
//	@Summary		Get saved search
//	@Tags			Search
//	@Produce		json
//	@Param			name	path	string	true	"Saved search name"
//	@Success		200	{object}	map[string]interface{}	"{success, data: {name, type, parameters, ignored_parameters}}"
//	@Failure		404	{object}	map[string]interface{}	"Not found"
//	@Security		BearerAuth
//	@Router			/search/saved/{name} [get]
func HandleGetSavedSearchAPI(c *gin.Context) {
	_, _, _, name, ss, ok := savedSearchForRequest(c)
	if !ok {
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": gin.H{
		"name": name, "type": ticketSearchProfileBase, "parameters": ss.criteria.parametersJSON(),
		"ignored_parameters": ss.ignored,
	}})
}

// HandleUpdateSavedSearchAPI replaces the parameters of a saved search.
//
//	@Summary		Update saved search
//	@Tags			Search
//	@Accept			json
//	@Produce		json
//	@Param			name	path	string	true	"Saved search name"
//	@Param			search	body	object	true	"{parameters: {<search parameter>: string | [string]}}"
//	@Success		200	{object}	map[string]interface{}	"{success, data: {name, type, parameters}}"
//	@Failure		400	{object}	map[string]interface{}	"Invalid parameters"
//	@Failure		404	{object}	map[string]interface{}	"Not found"
//	@Security		BearerAuth
//	@Router			/search/saved/{name} [put]
func HandleUpdateSavedSearchAPI(c *gin.Context) {
	var body savedSearchBody
	if err := c.ShouldBindJSON(&body); err != nil {
		searchBadRequest(c, fmt.Errorf("invalid JSON body: %w", err))
		return
	}
	db, _, login, name, _, ok := savedSearchForRequest(c)
	if !ok {
		return
	}
	cr, err := body.criteria()
	if err != nil {
		searchBadRequest(c, err)
		return
	}
	tx, err := db.Begin()
	if err != nil {
		searchServerError(c, "begin", err)
		return
	}
	defer func() { _ = tx.Rollback() }() //nolint:errcheck // no-op after commit
	if err := storeSavedSearch(tx, login, name, cr); err != nil {
		searchServerError(c, "store saved search", err)
		return
	}
	if err := tx.Commit(); err != nil {
		searchServerError(c, "commit", err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": gin.H{
		"name": name, "type": ticketSearchProfileBase, "parameters": cr.parametersJSON(),
	}})
}

// HandleDeleteSavedSearchAPI deletes a saved search.
//
//	@Summary		Delete saved search
//	@Tags			Search
//	@Param			name	path	string	true	"Saved search name"
//	@Success		204
//	@Failure		404	{object}	map[string]interface{}	"Not found"
//	@Security		BearerAuth
//	@Router			/search/saved/{name} [delete]
func HandleDeleteSavedSearchAPI(c *gin.Context) {
	db, _, login, name, _, ok := savedSearchForRequest(c)
	if !ok {
		return
	}
	if _, err := db.Exec(database.ConvertPlaceholders(
		"DELETE FROM search_profile WHERE login = ? AND profile_name = ?"), login, name); err != nil {
		searchServerError(c, "delete saved search", err)
		return
	}
	c.Status(http.StatusNoContent)
}

// HandleExecuteSavedSearchAPI runs a saved search. Paging and ordering come
// from the query string (page, limit, sort, order).
//
//	@Summary		Execute saved search
//	@Tags			Search
//	@Produce		json
//	@Param			name	path	string	true	"Saved search name"
//	@Success		200	{object}	map[string]interface{}	"{success, data: {name, parameters, ignored_parameters, tickets, pagination}}"
//	@Failure		404	{object}	map[string]interface{}	"Not found"
//	@Security		BearerAuth
//	@Router			/search/saved/{name}/execute [post]
func HandleExecuteSavedSearchAPI(c *gin.Context) {
	pg, err := parseTicketSearchPage(c)
	if err != nil {
		searchBadRequest(c, err)
		return
	}
	db, scope, _, name, ss, ok := savedSearchForRequest(c)
	if !ok {
		return
	}
	tickets, total, err := runTicketSearch(db, scope, ss.criteria, pg)
	if err != nil {
		searchServerError(c, "execute saved search", err)
		return
	}
	data := ticketSearchResponse(tickets, total, pg)
	data["name"] = name
	data["parameters"] = ss.criteria.parametersJSON()
	data["ignored_parameters"] = ss.ignored
	c.JSON(http.StatusOK, gin.H{"success": true, "data": data})
}
