package core

import "github.com/gotrs-io/gotrs-ce/internal/constants"

// MapCommunicationChannel derives communication_channel_id from article_type_id.
// Channel IDs (OTRS-aligned intent):
// 1 = email, 2 = phone, 3 = internal (notes / system), future: 4 = chat, 5 = sms, etc.
// Fallback returns 3 (internal) for safety so internal-only content is not misclassified.
func MapCommunicationChannel(articleTypeID int) int {
	switch articleTypeID {
	case constants.ArticleTypeEmailExternal, constants.ArticleTypeEmailInternal,
		constants.ArticleTypeEmailNotificationExt, constants.ArticleTypeEmailNotificationInt:
		return 1 // email
	case constants.ArticleTypePhone:
		return 2 // phone
	case constants.ArticleTypeFax, constants.ArticleTypeSMS, constants.ArticleTypeWebRequest:
		return 1 // treat as email-like until dedicated channels added
	case constants.ArticleTypeNoteInternal, constants.ArticleTypeNoteExternal, constants.ArticleTypeNoteReport:
		return 3 // internal/note channel
	case constants.ArticleTypeChatExternal, constants.ArticleTypeChatInternal:
		return 4 // provisional chat channel (not yet surfaced in UI)
	default:
		return 3 // safe default
	}
}
