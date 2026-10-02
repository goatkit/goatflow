package core

import (
	"strings"

	"github.com/goatkit/goatflow/internal/platform/constants"
)

// The OTRS 6+ schema has no article type: an article is stored as
// communication_channel_id + is_visible_for_customer. GoatFlow's API still
// speaks in article types (constants.ArticleType*), so this file is the one
// place that translates between the two representations.

// MapCommunicationChannel derives communication_channel_id from an article type.
// Fax, SMS and web requests have no dedicated channel in the seeded data and
// are treated as email. Unknown types map to the internal channel so content
// is never misclassified as customer-facing transport.
func MapCommunicationChannel(articleTypeID int) int {
	switch articleTypeID {
	case constants.ArticleTypeEmailExternal, constants.ArticleTypeEmailInternal,
		constants.ArticleTypeEmailNotificationExt, constants.ArticleTypeEmailNotificationInt,
		constants.ArticleTypeFax, constants.ArticleTypeSMS, constants.ArticleTypeWebRequest:
		return constants.CommunicationChannelEmail
	case constants.ArticleTypePhone:
		return constants.CommunicationChannelPhone
	case constants.ArticleTypeChatExternal, constants.ArticleTypeChatInternal:
		return constants.CommunicationChannelChat
	default:
		return constants.CommunicationChannelInternal
	}
}

// ArticleTypeFromStorage derives the article type of a stored article from its
// communication channel and customer visibility. Types that differ only in
// detail the schema does not keep (notifications, fax, SMS, web request,
// report notes) collapse onto the base type of their channel.
func ArticleTypeFromStorage(channelID int, visibleForCustomer bool) int {
	switch channelID {
	case constants.CommunicationChannelEmail:
		if visibleForCustomer {
			return constants.ArticleTypeEmailExternal
		}
		return constants.ArticleTypeEmailInternal
	case constants.CommunicationChannelPhone:
		return constants.ArticleTypePhone
	case constants.CommunicationChannelChat:
		if visibleForCustomer {
			return constants.ArticleTypeChatExternal
		}
		return constants.ArticleTypeChatInternal
	default:
		if visibleForCustomer {
			return constants.ArticleTypeNoteExternal
		}
		return constants.ArticleTypeNoteInternal
	}
}

// ArticleTypeName returns the API name of an article type ("" if unknown).
func ArticleTypeName(articleTypeID int) string {
	return constants.ArticleTypesMetadata[articleTypeID].Name
}

// ArticleTypeByName resolves an API article type name. "note" and "email" are
// accepted as aliases for "note-internal" and "email-external".
func ArticleTypeByName(name string) (int, bool) {
	name = strings.ToLower(strings.TrimSpace(name))
	switch name {
	case "note":
		name = "note-internal"
	case "email":
		name = "email-external"
	}
	for id, meta := range constants.ArticleTypesMetadata {
		if meta.Name == name {
			return id, true
		}
	}
	return 0, false
}
