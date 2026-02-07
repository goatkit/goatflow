package core

import (
	"testing"

	"github.com/goatkit/goatflow/internal/constants"
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
