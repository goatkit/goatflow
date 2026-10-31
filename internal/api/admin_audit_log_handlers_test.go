package api

import (
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

func TestCSVSafeCellNeutralisesFormulas(t *testing.T) {
	cases := map[string]string{
		"=HYPERLINK(\"x\")": "'=HYPERLINK(\"x\")",
		"+1":                "'+1",
		"-2+3":              "'-2+3",
		"@SUM(A1)":          "'@SUM(A1)",
		"\tcmd":             "'\tcmd",
		"admin.user.update": "admin.user.update",
		"a=b":               "a=b",
		"":                  "",
	}
	for in, want := range cases {
		if got := csvSafeCell(in); got != want {
			t.Errorf("csvSafeCell(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestParseAuditLogFilterRejectsBadInput(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for query, wantErr := range map[string]bool{
		"":                                      false,
		"category=auth&outcome=denied":          false,
		"from=2026-10-01&to=2026-10-31":         false,
		"category=bogus":                        true,
		"outcome=maybe":                         true,
		"from=31/10/2026":                       true,
		"from=2026-10-31&to=2026-10-01":         true,
		"action=login&actor=al%25&target_id=42": false,
	} {
		c, _ := gin.CreateTestContext(httptest.NewRecorder())
		c.Request = httptest.NewRequest("GET", "/admin/api/audit-log?"+query, nil)
		_, err := parseAuditLogFilter(c, time.UTC)
		if (err != nil) != wantErr {
			t.Errorf("%q: err = %v, wantErr %v", query, err, wantErr)
		}
	}
}

func TestAuditLogFilterToIsInclusiveAndLikeEscaped(t *testing.T) {
	loc := time.FixedZone("X", 2*3600)
	from := time.Date(2026, 10, 1, 0, 0, 0, 0, loc)
	f := auditLogFilter{From: from, To: from, Actor: "50%_off"}
	where, args := f.where()
	if where != " WHERE create_time >= ? AND create_time < ? AND LOWER(actor_login) LIKE ?" {
		t.Fatalf("where = %q", where)
	}
	if got := args[1].(time.Time); !got.Equal(from.AddDate(0, 0, 1)) || got.Location() != time.UTC {
		t.Errorf("to bound = %v", got)
	}
	if args[2] != `%50\%\_off%` {
		t.Errorf("like arg = %v", args[2])
	}
}
