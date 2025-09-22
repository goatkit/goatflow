package constants

// Article type IDs seeded via migrations (see legacy default data).
// These mirror OTRS semantics and MUST remain stable for data compatibility.
const (
	ArticleTypeEmailExternal        = 1
	ArticleTypeEmailInternal        = 2
	ArticleTypeEmailNotificationExt = 3
	ArticleTypeEmailNotificationInt = 4
	ArticleTypePhone                = 5
	ArticleTypeFax                  = 6
	ArticleTypeSMS                  = 7
	ArticleTypeWebRequest           = 8
	ArticleTypeNoteInternal         = 9
	ArticleTypeNoteExternal         = 10
	ArticleTypeNoteReport           = 11
	ArticleTypeChatExternal         = 12
	ArticleTypeChatInternal         = 13
)

// Sender (author/origin) type IDs.
const (
	ArticleSenderAgent    = 1
	ArticleSenderSystem   = 2
	ArticleSenderCustomer = 3
)

// InteractionType is a high-level UI-driven abstraction used to map
// user intent to concrete article_type_id + visibility rules.
// This keeps handler logic simple and centralized.
type InteractionType string

const (
	InteractionEmail        InteractionType = "email"
	InteractionPhone        InteractionType = "phone"
	InteractionInternalNote InteractionType = "internal_note"
	InteractionExternalNote InteractionType = "external_note" // may be phased in later
)

// ArticleTypeMeta holds metadata for each article type.
type ArticleTypeMeta struct {
	ID              int
	Name            string
	CustomerVisible bool // default visibility
	UserSelectable  bool // can appear in UI selection list
	InternalOnly    bool // never customer visible regardless of flag
}

// ArticleTypesMetadata provides lookup by ID.
var ArticleTypesMetadata = map[int]ArticleTypeMeta{
	ArticleTypeEmailExternal:        {ID: ArticleTypeEmailExternal, Name: "email-external", CustomerVisible: true, UserSelectable: true},
	ArticleTypeEmailInternal:        {ID: ArticleTypeEmailInternal, Name: "email-internal", CustomerVisible: false, UserSelectable: false, InternalOnly: true},
	ArticleTypeEmailNotificationExt: {ID: ArticleTypeEmailNotificationExt, Name: "email-notification-ext", CustomerVisible: true, UserSelectable: false},
	ArticleTypeEmailNotificationInt: {ID: ArticleTypeEmailNotificationInt, Name: "email-notification-int", CustomerVisible: false, UserSelectable: false, InternalOnly: true},
	ArticleTypePhone:                {ID: ArticleTypePhone, Name: "phone", CustomerVisible: true, UserSelectable: true},
	ArticleTypeFax:                  {ID: ArticleTypeFax, Name: "fax", CustomerVisible: true, UserSelectable: false},
	ArticleTypeSMS:                  {ID: ArticleTypeSMS, Name: "sms", CustomerVisible: true, UserSelectable: false},
	ArticleTypeWebRequest:           {ID: ArticleTypeWebRequest, Name: "webrequest", CustomerVisible: true, UserSelectable: false},
	ArticleTypeNoteInternal:         {ID: ArticleTypeNoteInternal, Name: "note-internal", CustomerVisible: false, UserSelectable: true, InternalOnly: true},
	ArticleTypeNoteExternal:         {ID: ArticleTypeNoteExternal, Name: "note-external", CustomerVisible: true, UserSelectable: true},
	ArticleTypeNoteReport:           {ID: ArticleTypeNoteReport, Name: "note-report", CustomerVisible: false, UserSelectable: false, InternalOnly: true},
	ArticleTypeChatExternal:         {ID: ArticleTypeChatExternal, Name: "chat-external", CustomerVisible: true, UserSelectable: false},
	ArticleTypeChatInternal:         {ID: ArticleTypeChatInternal, Name: "chat-internal", CustomerVisible: false, UserSelectable: false, InternalOnly: true},
}

// InteractionTypeOrdering is the order for UI selection.
var InteractionTypeOrdering = []InteractionType{
	InteractionEmail,
	InteractionPhone,
	InteractionInternalNote,
	// InteractionExternalNote can be appended when enabled
}
