package core

import (
	"testing"

	"github.com/goatkit/goatflow/internal/platform/constants"
)

func TestMapCommunicationChannel(t *testing.T) {
	cases := []struct {
		name string
		id   int
		want int
	}{
		{"email external", constants.ArticleTypeEmailExternal, 1},
		{"email internal", constants.ArticleTypeEmailInternal, 1},
		{"email notif ext", constants.ArticleTypeEmailNotificationExt, 1},
		{"email notif int", constants.ArticleTypeEmailNotificationInt, 1},
		{"phone", constants.ArticleTypePhone, 2},
		{"fax->email", constants.ArticleTypeFax, 1},
		{"sms->email", constants.ArticleTypeSMS, 1},
		{"webrequest->email", constants.ArticleTypeWebRequest, 1},
		{"note internal", constants.ArticleTypeNoteInternal, 3},
		{"note external", constants.ArticleTypeNoteExternal, 3},
		{"note report", constants.ArticleTypeNoteReport, 3},
		{"chat external", constants.ArticleTypeChatExternal, 4},
		{"chat internal", constants.ArticleTypeChatInternal, 4},
		{"unknown fallback", 9999, 3},
	}
	for _, c := range cases {
		if got := MapCommunicationChannel(c.id); got != c.want {
			// Use t.Fatalf for immediate clarity per case
			if got != c.want {
				// fallback safety: unknown should be 3 internal
				t.Errorf("%s: got %d want %d", c.name, got, c.want)
			}
		}
	}
}

// Every type the API accepts and that the schema can represent must read back
// as itself after being stored as channel + default visibility.
func TestArticleTypeStorageRoundTrip(t *testing.T) {
	for _, name := range []string{"email-external", "email-internal", "phone", "note-internal", "note-external", "chat-external", "chat-internal"} {
		id, ok := ArticleTypeByName(name)
		if !ok {
			t.Fatalf("%s: not resolvable", name)
		}
		meta := constants.ArticleTypesMetadata[id]
		got := ArticleTypeFromStorage(MapCommunicationChannel(id), meta.CustomerVisible)
		if ArticleTypeName(got) != name {
			t.Errorf("%s: stored then read back as %q", name, ArticleTypeName(got))
		}
	}
}

func TestArticleTypeByName(t *testing.T) {
	cases := map[string]int{
		"note":           constants.ArticleTypeNoteInternal,
		"Email":          constants.ArticleTypeEmailExternal,
		" note-report ":  constants.ArticleTypeNoteReport,
		"webrequest":     constants.ArticleTypeWebRequest,
		"email-internal": constants.ArticleTypeEmailInternal,
	}
	for name, want := range cases {
		if got, ok := ArticleTypeByName(name); !ok || got != want {
			t.Errorf("%q: got %d,%v want %d", name, got, ok, want)
		}
	}
	if _, ok := ArticleTypeByName("letter"); ok {
		t.Error("unknown name must not resolve")
	}
}
