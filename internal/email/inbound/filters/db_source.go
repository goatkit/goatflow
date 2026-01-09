package filters

import (
	"bytes"
	"context"
	"database/sql"
	"log"
	"net/mail"
	"regexp"
	"strconv"
	"strings"

	"github.com/gotrs-io/gotrs-ce/internal/database"
)

// DBSourceFilter loads postmaster filters from the database and applies them.
// This is equivalent to OTRS's PostMaster::PreFilterModule###000-MatchDBSource.
type DBSourceFilter struct {
	db     *sql.DB
	logger *log.Logger
}

// NewDBSourceFilter creates a new database source filter.
func NewDBSourceFilter(db *sql.DB, logger *log.Logger) *DBSourceFilter {
	return &DBSourceFilter{db: db, logger: logger}
}

// ID returns the filter identifier.
func (f *DBSourceFilter) ID() string { return "db_source" }

// Apply loads filters from the database and applies matching rules to the message.
func (f *DBSourceFilter) Apply(ctx context.Context, m *MessageContext) error {
	if f.db == nil || m == nil || m.Message == nil || len(m.Message.Raw) == 0 {
		return nil
	}

	// Parse the email message to access headers
	reader, err := mail.ReadMessage(bytes.NewReader(m.Message.Raw))
	if err != nil {
		f.logf("db_source: failed to parse message: %v", err)
		return nil // Don't fail the chain, just skip
	}

	// Load all filters from database
	filters, err := f.loadFilters(ctx)
	if err != nil {
		f.logf("db_source: failed to load filters: %v", err)
		return nil // Don't fail the chain, just skip
	}

	// Apply each filter
	for _, filter := range filters {
		matched, err := f.matchesFilter(filter, reader)
		if err != nil {
			f.logf("db_source: filter %q match error: %v", filter.Name, err)
			continue
		}

		if matched {
			f.logf("db_source: filter %q matched", filter.Name)
			f.applySetRules(filter, m)

			if filter.Stop {
				f.logf("db_source: filter %q has stop flag, ending filter processing", filter.Name)
				break
			}
		}
	}

	return nil
}

// dbFilter represents a grouped filter loaded from the database.
type dbFilter struct {
	Name    string
	Stop    bool
	Matches []dbMatch
	Sets    []dbSet
}

type dbMatch struct {
	Key   string
	Value string
	Not   bool
}

type dbSet struct {
	Key   string
	Value string
}

// loadFilters queries the postmaster_filter table and groups rows by filter name.
func (f *DBSourceFilter) loadFilters(ctx context.Context) ([]dbFilter, error) {
	query := database.ConvertPlaceholders(`
		SELECT f_name, f_stop, f_type, f_key, f_value, f_not
		FROM postmaster_filter
		ORDER BY f_name, f_type DESC, f_key`)

	rows, err := f.db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	filterMap := make(map[string]*dbFilter)
	var filterOrder []string

	for rows.Next() {
		var (
			name    string
			stopVal sql.NullInt16
			fType   string
			key     string
			value   string
			notVal  sql.NullInt16
		)

		if err := rows.Scan(&name, &stopVal, &fType, &key, &value, &notVal); err != nil {
			return nil, err
		}

		filter, exists := filterMap[name]
		if !exists {
			filter = &dbFilter{
				Name:    name,
				Stop:    stopVal.Valid && stopVal.Int16 == 1,
				Matches: []dbMatch{},
				Sets:    []dbSet{},
			}
			filterMap[name] = filter
			filterOrder = append(filterOrder, name)
		}

		switch fType {
		case "Match":
			filter.Matches = append(filter.Matches, dbMatch{
				Key:   key,
				Value: value,
				Not:   notVal.Valid && notVal.Int16 == 1,
			})
		case "Set":
			filter.Sets = append(filter.Sets, dbSet{
				Key:   key,
				Value: value,
			})
		}
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	// Return filters in order they were seen
	result := make([]dbFilter, 0, len(filterOrder))
	for _, name := range filterOrder {
		result = append(result, *filterMap[name])
	}

	return result, nil
}

// matchesFilter checks if all Match rules in the filter match the message.
// All Match rules must match (AND logic) for the filter to apply.
func (f *DBSourceFilter) matchesFilter(filter dbFilter, reader *mail.Message) (bool, error) {
	if len(filter.Matches) == 0 {
		// No match rules means always match (like OTRS behavior)
		return true, nil
	}

	for _, match := range filter.Matches {
		matched, err := f.matchRule(match, reader)
		if err != nil {
			return false, err
		}
		if !matched {
			return false, nil
		}
	}

	return true, nil
}

// matchRule checks if a single Match rule matches the message.
func (f *DBSourceFilter) matchRule(match dbMatch, reader *mail.Message) (bool, error) {
	// Get the header value to match against
	headerValue := f.getMatchValue(match.Key, reader)

	// Compile the regex pattern
	re, err := regexp.Compile(match.Value)
	if err != nil {
		return false, err
	}

	// Check if the pattern matches
	matched := re.MatchString(headerValue)

	// Apply negation if needed
	if match.Not {
		matched = !matched
	}

	return matched, nil
}

// getMatchValue gets the value to match against for a given key.
// Supports standard email headers and special keys like Body.
func (f *DBSourceFilter) getMatchValue(key string, reader *mail.Message) string {
	key = strings.TrimSpace(key)

	switch strings.ToLower(key) {
	case "body":
		// Read the body (for Body matching like OTRS)
		buf := new(strings.Builder)
		if reader.Body != nil {
			bodyBytes := make([]byte, 64*1024) // Read up to 64KB
			n, _ := reader.Body.Read(bodyBytes)
			buf.Write(bodyBytes[:n])
		}
		return buf.String()
	default:
		// Standard header lookup
		return reader.Header.Get(key)
	}
}

// applySetRules applies all Set rules from a matched filter.
func (f *DBSourceFilter) applySetRules(filter dbFilter, m *MessageContext) {
	for _, set := range filter.Sets {
		f.applySetRule(set, m)
	}
}

// applySetRule applies a single Set rule by mapping X-GOTRS-* headers to annotations.
func (f *DBSourceFilter) applySetRule(set dbSet, m *MessageContext) {
	if m.Annotations == nil {
		m.Annotations = make(map[string]any)
	}

	key := strings.TrimSpace(set.Key)
	value := strings.TrimSpace(set.Value)

	if key == "" || value == "" {
		return
	}

	// Map X-GOTRS-* headers to annotations
	switch strings.ToLower(key) {
	case "x-gotrs-queue", "x-otrs-queue", "x-gotrs-queuename", "x-otrs-queuename":
		m.Annotations[AnnotationQueueNameOverride] = value

	case "x-gotrs-queueid", "x-otrs-queueid":
		if id, err := strconv.Atoi(value); err == nil && id > 0 {
			m.Annotations[AnnotationQueueIDOverride] = id
		}

	case "x-gotrs-priority", "x-otrs-priority":
		// Priority by name - store as string
		m.Annotations[AnnotationPriorityNameOverride] = value

	case "x-gotrs-priorityid", "x-otrs-priorityid":
		if id, err := strconv.Atoi(value); err == nil && id > 0 {
			m.Annotations[AnnotationPriorityIDOverride] = id
		}

	case "x-gotrs-title", "x-otrs-title":
		m.Annotations[AnnotationTitleOverride] = value

	case "x-gotrs-customerid", "x-otrs-customerid":
		m.Annotations[AnnotationCustomerIDOverride] = value

	case "x-gotrs-customeruser", "x-otrs-customeruser", "x-gotrs-customeruserid", "x-otrs-customeruserid":
		m.Annotations[AnnotationCustomerUserOverride] = value

	case "x-gotrs-ignore", "x-otrs-ignore":
		switch strings.ToLower(value) {
		case "1", "true", "yes", "y":
			m.Annotations[AnnotationIgnoreMessage] = true
		case "0", "false", "no", "n":
			m.Annotations[AnnotationIgnoreMessage] = false
		}

	case "x-gotrs-state", "x-otrs-state":
		m.Annotations[AnnotationStateOverride] = value

	case "x-gotrs-type", "x-otrs-type":
		m.Annotations[AnnotationTypeOverride] = value

	default:
		// Store unknown headers with prefix for potential future use
		m.Annotations[AnnotationTrustedHeaderPrefix+strings.ToLower(key)] = value
	}
}

func (f *DBSourceFilter) logf(format string, args ...any) {
	if f.logger != nil {
		f.logger.Printf(format, args...)
	}
}

// AnnotationPriorityNameOverride is the annotation key for priority name override.
const AnnotationPriorityNameOverride = "postmaster.priority_name_override"

// AnnotationStateOverride is the annotation key for state override.
const AnnotationStateOverride = "postmaster.state_override"

// AnnotationTypeOverride is the annotation key for ticket type override.
const AnnotationTypeOverride = "postmaster.type_override"
