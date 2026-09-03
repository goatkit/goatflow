package api

import (
	"context"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/goatkit/goatflow/internal/platform/database"
	"github.com/goatkit/goatflow/internal/service"
)

// icsEvent holds the display fields of one VEVENT.
type icsEvent struct {
	Summary          string
	Location         string
	Description      string
	Organizer        string // raw mailto value
	OrganizerName    string
	URL              string
	Status           string
	Categories       []string
	Start, End       time.Time
	HasStart, HasEnd bool
	AllDay           bool
	TZID             string
}

// icsLogicalLine is one unfolded property: NAME;P1=v1;P2=v2:VALUE
type icsLogicalLine struct {
	Name   string
	Params map[string]string
	Value  string
}

// parseICS extracts VEVENTs from RFC 5545 calendar data.
func parseICS(data []byte) []icsEvent {
	lines := unfoldICS(string(data))
	var events []icsEvent
	var cur *icsEvent
	inEvent := false

	for _, ll := range lines {
		switch ll.Name {
		case "BEGIN":
			if strings.EqualFold(ll.Value, "VEVENT") {
				cur = &icsEvent{}
				inEvent = true
			}
			continue
		case "END":
			if strings.EqualFold(ll.Value, "VEVENT") && cur != nil {
				if strings.TrimSpace(cur.Summary) != "" || cur.HasStart {
					events = append(events, *cur)
				}
				inEvent = false
				cur = nil
			}
			continue
		}
		if inEvent && cur != nil {
			cur.set(ll)
		}
	}
	return events
}

// unfoldICS splits raw calendar text into logical lines (RFC 5545 §3.1:
// CRLF followed by a single space/tab folds the previous line).
func unfoldICS(text string) []icsLogicalLine {
	text = strings.ReplaceAll(text, "\r\n", "\n")
	var raws []string
	for _, ln := range strings.Split(text, "\n") {
		if ln == "" {
			continue
		}
		if len(ln) > 0 && (ln[0] == ' ' || ln[0] == '\t') && len(raws) > 0 {
			raws[len(raws)-1] += ln[1:]
			continue
		}
		raws = append(raws, ln)
	}
	out := make([]icsLogicalLine, 0, len(raws))
	for _, raw := range raws {
		colon := strings.IndexByte(raw, ':')
		if colon < 0 {
			continue
		}
		head := raw[:colon]
		params := map[string]string{}
		name := head
		if semi := strings.IndexByte(head, ';'); semi >= 0 {
			name = head[:semi]
			for _, kv := range strings.Split(head[semi+1:], ";") {
				if eq := strings.IndexByte(kv, '='); eq > 0 {
					params[strings.ToUpper(strings.TrimSpace(kv[:eq]))] = strings.TrimSpace(kv[eq+1:])
				}
			}
		}
		out = append(out, icsLogicalLine{
			Name:   strings.ToUpper(strings.TrimSpace(name)),
			Params: params,
			Value:  unescapeICS(raw[colon+1:]),
		})
	}
	return out
}

func (e *icsEvent) set(ll icsLogicalLine) {
	switch ll.Name {
	case "SUMMARY":
		e.Summary = strings.TrimSpace(ll.Value)
	case "LOCATION":
		e.Location = strings.TrimSpace(ll.Value)
	case "DESCRIPTION":
		e.Description = ll.Value
	case "ORGANIZER":
		e.Organizer = strings.TrimSpace(ll.Value)
		if name, ok := ll.Params["NAME"]; ok {
			e.OrganizerName = name
		}
	case "URL":
		e.URL = strings.TrimSpace(ll.Value)
	case "STATUS":
		e.Status = strings.ToUpper(strings.TrimSpace(ll.Value))
	case "CATEGORIES":
		for _, c := range strings.Split(ll.Value, ",") {
			if c = strings.TrimSpace(c); c != "" {
				e.Categories = append(e.Categories, c)
			}
		}
	case "DTSTART":
		e.AllDay = true
		if t, ok := parseICSTime(ll.Value, ll.Params["TZID"]); ok {
			e.Start = t
			e.HasStart = true
			e.AllDay = !hasICSTimeComponent(ll.Value)
			if ll.Params["TZID"] != "" {
				e.TZID = ll.Params["TZID"]
			}
		}
	case "DTEND":
		if t, ok := parseICSTime(ll.Value, ll.Params["TZID"]); ok {
			e.End = t
			e.HasEnd = true
			e.AllDay = !hasICSTimeComponent(ll.Value)
		}
	}
}

func hasICSTimeComponent(v string) bool {
	return strings.Contains(v, "T")
}

// parseICSTime parses the three ICS date forms: YYYYMMDD, YYYYMMDDTHHMMSS,
// YYYYMMDDTHHMMSSZ (UTC).
func parseICSTime(v, tzid string) (time.Time, bool) {
	v = strings.TrimSpace(v)
	if strings.HasSuffix(v, "Z") {
		t, err := time.Parse("20060102T150405", v[:len(v)-1])
		if err != nil {
			return time.Time{}, false
		}
		return t.UTC(), true
	}
	layouts := []string{"20060102T150405", "20060102"}
	for _, layout := range layouts {
		if len(v) == len(layout) {
			if t, err := time.Parse(layout, v); err == nil {
				return t, true
			}
		}
	}
	// Fall back to strict parsing of whatever shape is present.
	for _, layout := range []string{"20060102T150405Z", "20060102T150405", "20060102"} {
		if t, err := time.Parse(layout, v); err == nil {
			return t, true
		}
	}
	return time.Time{}, false
}

// unescapeICS expands RFC 5545 TEXT escapes (\n, \,, \;, \\).
func unescapeICS(s string) string {
	var b strings.Builder
	// Classic loop: escape handling mutates i to skip the escaped byte, which
	// range-over-int sequences do not honor.
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c != '\\' || i+1 >= len(s) {
			b.WriteByte(c)
			continue
		}
		i++
		switch s[i] {
		case 'n', 'N':
			b.WriteByte('\n')
		case ',', ';', '\\', ' ':
			if s[i] == ' ' {
				b.WriteByte(' ')
			} else {
				b.WriteByte(s[i])
			}
		default:
			b.WriteByte(s[i])
		}
	}
	return b.String()
}

// loadAttachmentContent returns the stored bytes for an attachment, mirroring
// the raw-serve fallback chain: DB column, storage service, local file.
func loadAttachmentContent(ctx context.Context, ticketID, attID int) []byte {
	db := attachmentsDB()
	if db == nil {
		return nil
	}
	var contentType string
	var content []byte
	var articleID int
	row := db.QueryRow(database.ConvertPlaceholders(`
		SELECT COALESCE(att.content_type,''), att.content, att.article_id
		FROM article_data_mime_attachment att
		JOIN article a ON a.id = att.article_id
		WHERE att.id = ? AND a.ticket_id = ? LIMIT 1`), attID, ticketID)
	if err := row.Scan(&contentType, &content, &articleID); err != nil {
		return nil
	}
	if len(content) == 0 {
		if ss := GetStorageService(); ss != nil {
			var filename string
			if err := db.QueryRow(database.ConvertPlaceholders(
				`SELECT att.filename FROM article_data_mime_attachment att WHERE att.id = ? AND a.ticket_id = ?`), attID, ticketID).Scan(&filename); err == nil {
				sp := service.GenerateOTRSStoragePath(ticketID, articleID, filename)
				if rc, rerr := ss.Retrieve(ctx, sp); rerr == nil {
					defer rc.Close()
					if buf, berr := io.ReadAll(rc); berr == nil {
						content = buf
					}
				}
			}
		}
		if len(content) == 0 {
			var filename string
			if err := db.QueryRow(database.ConvertPlaceholders(
				`SELECT att.filename FROM article_data_mime_attachment att WHERE att.id = ?`), attID).Scan(&filename); err == nil {
				if buf, ok := findLocalStoredAttachmentBytes(ticketID, filename); ok {
					content = buf
				}
			}
		}
	}
	return content
}

// isICSSignalled reports whether the attachment should be treated as a
// calendar file by content type or extension.
func isICSSignalled(contentType, filename string) bool {
	ct := strings.ToLower(contentType)
	if ct == "text/calendar" || ct == "application/ics" || ct == "application/x-vnd.microsoft.outlook" {
		return true
	}
	lf := strings.ToLower(filename)
	return strings.HasSuffix(lf, ".ics") || strings.HasSuffix(lf, ".ical")
}

// icsEventHTML renders event cards for the attachment viewer popup.
func icsEventHTML(events []icsEvent, rawURL string) string {
	var b strings.Builder
	b.WriteString(`<div style="max-width:720px;width:90%;max-height:86%;overflow:auto;padding:12px 0;">`)
	for i := range len(events) {
		ev := events[i]
		b.WriteString(icsCardHTML(ev))
	}
	b.WriteString(fmt.Sprintf(
		`<div style="text-align:center;margin:18px 0 8px;"><a href="%s" target="_blank" style="font-size:12px;color:#888;text-decoration:underline;">View raw file</a></div>`,
		rawURL))
	b.WriteString(`</div>`)
	return b.String()
}

func icsCardHTML(ev icsEvent) string {
	var b strings.Builder
	b.WriteString(`<div style="background:rgba(255,255,255,.04);border:1px solid rgba(255,255,255,.1);border-radius:14px;padding:20px 24px;margin:14px 0;">`)

	// Icon + summary
	b.WriteString(`<div style="display:flex;align-items:center;gap:12px;margin-bottom:12px;">`)
	b.WriteString(icsCalendarSVG)
	title := htmlEscape(ev.Summary)
	if title == "" {
		title = "Untitled event"
	}
	b.WriteString(`<span style="font-size:18px;font-weight:600;color:#fff;">` + title + `</span>`)
	if ev.Status != "" {
		b.WriteString(fmt.Sprintf(`<span style="font-size:11px;color:#999;border:1px solid rgba(255,255,255,.2);border-radius:99px;padding:2px 8px;">%s</span>`, htmlEscape(ev.Status)))
	}
	b.WriteString(`</div>`)

	// When
	if when := ev.whenText(); when != "" {
		b.WriteString(fmt.Sprintf(`<div style="display:flex;align-items:center;gap:8px;font-size:14px;color:#ddd;margin-bottom:8px;"><svg width="14" height="14" fill="none" stroke="#3b82f6" stroke-width="2" viewBox="0 0 24 24"><circle cx="12" cy="12" r="9"/><path stroke-linecap="round" d="M12 7v5l3 3"/></svg>%s</div>`, htmlEscape(when)))
	}

	// Where
	if ev.Location != "" {
		b.WriteString(fmt.Sprintf(`<div style="display:flex;align-items:center;gap:8px;font-size:13px;color:#ccc;margin-bottom:8px;"><svg width="14" height="14" fill="none" stroke="#3b82f6" stroke-width="2" viewBox="0 0 24 24"><path d="M12 21s-7-5.5-7-11a7 7 0 1114 0c0 5.5-7 11-7 11z"/><circle cx="12" cy="10" r="2.5"/></svg>%s</div>`, htmlEscape(ev.Location)))
	}

	// Description
	if ev.Description != "" {
		desc := htmlEscape(strings.TrimSpace(ev.Description))
		desc = strings.ReplaceAll(desc, "\n", "<br>")
		b.WriteString(fmt.Sprintf(`<div style="font-size:13px;color:#bbb;line-height:1.55;margin-bottom:8px;">%s</div>`, desc))
	}

	// Categories + organizer
	if len(ev.Categories) > 0 || ev.Organizer != "" {
		b.WriteString(`<div style="display:flex;flex-wrap:wrap;align-items:center;gap:6px;margin-bottom:4px;">`)
		for i := range len(ev.Categories) {
			c := ev.Categories[i]
			b.WriteString(fmt.Sprintf(`<span style="font-size:11px;color:#93c5fd;background:rgba(59,130,246,.12);border-radius:99px;padding:2px 8px;">%s</span>`, htmlEscape(c)))
		}
		if ev.Organizer != "" {
			orgName := ev.OrganizerName
			if orgName == "" {
				orgName = ev.Organizer
			}
			b.WriteString(fmt.Sprintf(`<a href="mailto:%s" style="font-size:11px;color:#888;text-decoration:none;">Organizer: %s</a>`, htmlEscape(ev.Organizer), htmlEscape(orgName)))
		}
		b.WriteString(`</div>`)
	}

	// URL
	if ev.URL != "" {
		if isSafeURL(ev.URL) {
			b.WriteString(fmt.Sprintf(`<div style="margin-top:6px;"><a href="%s" target="_blank" style="font-size:12px;color:#3b82f6;text-decoration:underline;">%s</a></div>`, htmlEscape(ev.URL), htmlEscape(ev.URL)))
		}
	}

	b.WriteString(`</div>`)
	return b.String()
}

// whenText formats start/end for display.
func (e icsEvent) whenText() string {
	if !e.HasStart {
		return ""
	}
	if e.AllDay {
		d := e.Start.Format("Mon 2 Jan 2006")
		if e.HasEnd && !e.End.Equal(e.Start) {
			return d + " – " + e.End.Format("2 Jan 2006") + " (all day)"
		}
		return d + " (all day)"
	}
	start := e.Start.Format("Mon 2 Jan 2006, 15:04")
	if !e.HasEnd {
		return start
	}
	end := e.End
	if e.Start.Format("2006-01-02") == end.Format("2006-01-02") {
		return start + " – " + end.Format("15:04")
	}
	return start + " → " + end.Format("Mon 2 Jan 2006, 15:04")
}

// icsCalendarSVG is the calendar glyph used on each event card.
const icsCalendarSVG = `<svg width="26" height="26" viewBox="0 0 24 24" fill="none" stroke="#3b82f6" stroke-width="1.8" style="flex-shrink:0;"><rect x="3" y="5" width="18" height="16" rx="2"/><path stroke-linecap="round" d="M3 10h18M8 3v4M16 3v4"/><circle cx="8.5" cy="14.5" r="1.4" fill="#3b82f6" stroke="none"/><circle cx="12.5" cy="14.5" r="1.4" fill="#3b82f6" stroke="none"/><circle cx="16.5" cy="14.5" r="1.4" fill="#3b82f6" stroke="none"/></svg>`

// isSafeURL allows http/https links only (no javascript:/mailto: in <a href>).
func isSafeURL(u string) bool {
	l := strings.ToLower(strings.TrimSpace(u))
	return strings.HasPrefix(l, "http://") || strings.HasPrefix(l, "https://")
}

// icsFallbackHTML is the plain-text iframe used when an ICS file cannot be
// parsed into events.
func icsFallbackHTML(rawURL string) string {
	return fmt.Sprintf(`<iframe src="%s" style="width:100%%; height:100%%; border:0; background:#111;"></iframe>`, rawURL)
}
