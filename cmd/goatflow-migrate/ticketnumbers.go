package main

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/goatkit/goatflow/internal/platform/database"
)

// counterSeed is a GoatFlow ticket number counter set from the imported tickets.
type counterSeed struct {
	generator, systemID string
	counters            map[string]int64 // counter_uid -> last counter OTRS used
}

// otrsSetting returns an OTRS setting's effective value: the global
// override from sysconfig_modified, else the definition's default.
func (im *importer) otrsSetting(name string) (string, bool, error) {
	var value string
	found := false
	scan := func(table string, global func(columns []string, row []dumpValue) bool) error {
		if im.tables[table] == nil {
			return nil
		}
		return im.src.rows(table, func(columns []string, row []dumpValue) error {
			ni, vi := indexOf(columns, "name"), indexOf(columns, "effective_value")
			if ni < 0 || vi < 0 || string(row[ni].data) != name || !global(columns, row) {
				return nil
			}
			value, found = otrsYAMLScalar(string(row[vi].data)), true
			return nil
		})
	}
	err := scan("sysconfig_modified", func(columns []string, row []dumpValue) bool {
		ui, vi := indexOf(columns, "user_id"), indexOf(columns, "is_valid")
		return (ui < 0 || row[ui].null) && (vi < 0 || string(row[vi].data) == "1")
	})
	if err != nil || found {
		return value, found, err
	}
	err = scan("sysconfig_default", func([]string, []dumpValue) bool { return true })
	return value, found, err
}

// otrsYAMLScalar decodes the YAML scalar OTRS stores ("--- 10\n", "--- '10'\n").
func otrsYAMLScalar(s string) string {
	s = strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(s), "---"))
	if len(s) >= 2 && (s[0] == '\'' && s[len(s)-1] == '\'' || s[0] == '"' && s[len(s)-1] == '"') {
		s = s[1 : len(s)-1]
	}
	return s
}

// setTicketNumberCounters continues OTRS's ticket numbering in GoatFlow.
// OTRS's ticket_number_counter rows (one per number, random counter_uid)
// mean nothing to GoatFlow, which keeps one counter per SystemID
// (AutoIncrement) or per SystemID and day (Date, DateChecksum). Without
// them a new GoatFlow ticket restarts at counter 1 and, on the day of the
// migration, gets the number of a ticket OTRS created that day. The counters
// are set to the highest counter in the imported ticket numbers, parsed with
// the generator and SystemID the OTRS source was configured with (OTRS
// defaults: DateChecksum, 10). A target counter that is already higher is
// kept.
func (im *importer) setTicketNumberCounters() (*counterSeed, error) {
	module, ok, err := im.otrsSetting("Ticket::NumberGenerator")
	if err != nil {
		return nil, err
	}
	if !ok {
		module = "Kernel::System::Ticket::Number::DateChecksum"
	}
	systemID, ok, err := im.otrsSetting("SystemID")
	if err != nil {
		return nil, err
	}
	if !ok || systemID == "" {
		systemID = "10"
	}
	seed := &counterSeed{generator: module[strings.LastIndex(module, ":")+1:], systemID: systemID, counters: map[string]int64{}}

	tns, err := im.queryKeys("SELECT tn FROM ticket")
	if err != nil {
		return nil, err
	}
	for _, tn := range tns {
		uid, counter, ok := parseTicketNumber(seed.generator, systemID, tn)
		if ok && counter > seed.counters[uid] {
			seed.counters[uid] = counter
		}
	}

	uids := make([]string, 0, len(seed.counters))
	for uid := range seed.counters {
		uids = append(uids, uid)
	}
	sort.Strings(uids)
	for _, uid := range uids {
		counter := seed.counters[uid]
		existing, err := im.queryKeys("SELECT counter FROM ticket_number_counter WHERE counter_uid = ?", uid)
		if err != nil {
			return nil, err
		}
		if len(existing) == 0 {
			_, err = im.tx.Exec(database.ConvertPlaceholders(
				"INSERT INTO ticket_number_counter (counter, counter_uid, create_time) VALUES (?, ?, ?)"), counter, uid, time.Now())
		} else {
			_, err = im.tx.Exec(database.ConvertPlaceholders(
				"UPDATE ticket_number_counter SET counter = ? WHERE counter_uid = ? AND counter < ?"), counter, uid, counter)
		}
		if err != nil {
			return nil, fmt.Errorf("ticket number counter %s: %w", uid, err)
		}
	}
	return seed, nil
}

// parseTicketNumber returns the GoatFlow counter_uid and the counter of a
// ticket number OTRS's generator built (Kernel::System::Ticket::Number::*):
// AutoIncrement SystemID.Counter, Date Year.Month.Day.SystemID.Counter,
// DateChecksum Year.Month.Day.SystemID.Counter.CheckDigit.
func parseTicketNumber(generator, systemID, tn string) (string, int64, bool) {
	var uid, digits string
	switch generator {
	case "AutoIncrement":
		if !strings.HasPrefix(tn, systemID) {
			return "", 0, false
		}
		uid, digits = systemID, tn[len(systemID):]
	case "Date", "DateChecksum":
		if len(tn) < 8+len(systemID)+1 || tn[8:8+len(systemID)] != systemID {
			return "", 0, false
		}
		if _, err := time.Parse("20060102", tn[:8]); err != nil {
			return "", 0, false
		}
		uid, digits = systemID+"_"+tn[:8], tn[8+len(systemID):]
		if generator == "DateChecksum" {
			digits = digits[:len(digits)-1]
		}
	default:
		return "", 0, false
	}
	counter, err := strconv.ParseInt(digits, 10, 64)
	if err != nil || digits == "" {
		return "", 0, false
	}
	return uid, counter, true
}
