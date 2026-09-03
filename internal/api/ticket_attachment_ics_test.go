package api

import (
	"strings"
	"testing"
	"time"
)

func TestParseICSSingleEventMinimal(t *testing.T) {
	data := []byte("BEGIN:VCALENDAR\nVERSION:2.0\nPRODID:-//goatcoach//EN\nBEGIN:VEVENT\nDTSTAMP:20260829T000000Z\nSUMMARY:Coaching session\nEND:VEVENT\nEND:VCALENDAR\n")
	events := parseICS(data)
	if len(events) != 1 {
		t.Fatalf("want 1 event, got %d", len(events))
	}
	if events[0].Summary != "Coaching session" {
		t.Errorf("summary = %q", events[0].Summary)
	}
	if events[0].HasStart {
		t.Error("minimal event must not have a start time")
	}
}

func TestParseICSMultiEventUnfolding(t *testing.T) {
	data := []byte(strings.Join([]string{
		"BEGIN:VCALENDAR",
		"BEGIN:VEVENT",
		"UID:gc-1-0",
		"DTSTART:20260903T090000",
		"DTEND:20260903T100000",
		"SUMMARY:Ship the report",
		"DESCRIPTION:From deliverable\\nShip the report\\nOwne",
		" r: Nigel\\nDue: 2026-09-03",
		"CATEGORIES:action-items\\,Nigel",
		"STATUS:CONFIRMED",
		"END:VEVENT",
		"BEGIN:VEVENT",
		"DTSTART:20260409T090000",
		"DTEND:20260409T100000",
		"SUMMARY:Renew hosting",
		"END:VEVENT",
		"END:VCALENDAR",
	}, "\n"))
	events := parseICS(data)
	if len(events) != 2 {
		t.Fatalf("want 2 events, got %d", len(events))
	}
	ev := events[0]
	if ev.Summary != "Ship the report" {
		t.Errorf("summary = %q", ev.Summary)
	}
	// Unfolding must rejoin "Owne" + "r: Nigel" into "Owner: Nigel"
	if !strings.Contains(ev.Description, "Owner: Nigel") {
		t.Errorf("description not unfolded: %q", ev.Description)
	}
	// Escapes: \\n -> newline, \\, -> comma
	if !strings.Contains(ev.Description, "\nDue: 2026-09-03") {
		t.Errorf("description escapes not expanded: %q", ev.Description)
	}
	if len(ev.Categories) != 2 || ev.Categories[0] != "action-items" || ev.Categories[1] != "Nigel" {
		t.Errorf("categories = %v", ev.Categories)
	}
	if ev.Status != "CONFIRMED" {
		t.Errorf("status = %q", ev.Status)
	}
	if !ev.HasStart || !ev.HasEnd {
		t.Fatal("start/end not parsed")
	}
	if got := ev.Start.Format("2006-01-02 15:04"); got != "2026-09-03 09:00" {
		t.Errorf("start = %s", got)
	}
	if got := ev.End.Format("2006-01-02 15:04"); got != "2026-09-03 10:00" {
		t.Errorf("end = %s", got)
	}
	if ev.AllDay {
		t.Error("timed event must not be all-day")
	}
}

func TestParseICSAllDay(t *testing.T) {
	data := []byte("BEGIN:VCALENDAR\nBEGIN:VEVENT\nDTSTART:20260903\nDTEND:20260904\nSUMMARY:Workshop\nEND:VEVENT\nEND:VCALENDAR\n")
	events := parseICS(data)
	if len(events) != 1 {
		t.Fatalf("want 1 event, got %d", len(events))
	}
	ev := events[0]
	if !ev.AllDay {
		t.Error("date-only event must be all-day")
	}
	if got := ev.whenText(); got != "Thu 3 Sep 2026 – 4 Sep 2026 (all day)" {
		t.Errorf("whenText = %q", got)
	}
}

func TestParseICSTimeZone(t *testing.T) {
	data := []byte("BEGIN:VCALENDAR\nBEGIN:VEVENT\nDTSTART;TZID=Europe/Berlin:20260903T090000\nSUMMARY:Local event\nEND:VEVENT\nEND:VCALENDAR\n")
	events := parseICS(data)
	if len(events) != 1 {
		t.Fatalf("want 1 event, got %d", len(events))
	}
	ev := events[0]
	if ev.TZID != "Europe/Berlin" {
		t.Errorf("tzid = %q", ev.TZID)
	}
	if !ev.HasStart || ev.AllDay {
		t.Error("TZID event must be timed")
	}
}

func TestParseICSUTC(t *testing.T) {
	data := []byte("BEGIN:VCALENDAR\nBEGIN:VEVENT\nDTSTART:20260829T174308Z\nSUMMARY:Zulu\nEND:VEVENT\nEND:VCALENDAR\n")
	events := parseICS(data)
	if len(events) != 1 {
		t.Fatalf("want 1 event, got %d", len(events))
	}
	if got := events[0].Start.Format(time.RFC3339); got != "2026-08-29T17:43:08Z" {
		t.Errorf("utc start = %s", got)
	}
}

func TestUnescapeICS(t *testing.T) {
	cases := map[string]string{
		`a\nb`:  "a\nb",
		`a\,b`:  "a,b",
		`a\;b`:  "a;b",
		`a\\b`:  `a\b`,
		"plain": "plain",
	}
	for in, want := range cases {
		if got := unescapeICS(in); got != want {
			t.Errorf("unescapeICS(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestParseICSNoEvents(t *testing.T) {
	if events := parseICS([]byte("BEGIN:VCALENDAR\nEND:VCALENDAR\n")); len(events) != 0 {
		t.Errorf("empty calendar must yield no events, got %d", len(events))
	}
	if events := parseICS([]byte("junk")); len(events) != 0 {
		t.Errorf("junk must yield no events, got %d", len(events))
	}
}
