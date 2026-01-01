package models

import (
	"time"
)

// ArticleStatus represents the status of a knowledge article.
type ArticleStatus string

const (
	ArticleStatusDraft     ArticleStatus = "draft"
	ArticleStatusReview    ArticleStatus = "review"
	ArticleStatusApproved  ArticleStatus = "approved"
	ArticleStatusPublished ArticleStatus = "published"
	ArticleStatusArchived  ArticleStatus = "archived"
	ArticleStatusRetired   ArticleStatus = "retired"
)

// ArticleType represents the type of knowledge article.
type ArticleType string

const (
	ArticleTypeFAQ             ArticleType = "faq"
	ArticleTypeHowTo           ArticleType = "how_to"
	ArticleTypeTroubleshooting ArticleType = "troubleshooting"
	ArticleTypeReference       ArticleType = "reference"
	ArticleTypePolicy          ArticleType = "policy"
	ArticleTypeProcedure       ArticleType = "procedure"
	ArticleTypeAnnouncement    ArticleType = "announcement"
	ArticleTypeKnownError      ArticleType = "known_error"
)

// ArticleVisibility represents who can see the article.
type ArticleVisibility string

const (
	VisibilityPublic     ArticleVisibility = "public"     // Everyone including customers
	VisibilityInternal   ArticleVisibility = "internal"   // Internal staff only
	VisibilityRestricted ArticleVisibility = "restricted" // Specific groups only
)

// KnowledgeArticle represents a knowledge base article.
type KnowledgeArticle struct {
	ID            uint              `json:"id" gorm:"primaryKey"`
	ArticleNumber string            `json:"article_number" gorm:"uniqueIndex;not null"`
	Title         string            `json:"title" gorm:"not null;index"`
	Summary       string            `json:"summary"`
	Content       string            `json:"content" gorm:"type:text"`
	Type          ArticleType       `json:"type" gorm:"not null;default:'reference';index"`
	Status        ArticleStatus     `json:"status" gorm:"not null;default:'draft';index"`
	Visibility    ArticleVisibility `json:"visibility" gorm:"not null;default:'internal';index"`

	// Categorization
	CategoryID    uint               `json:"category_id" gorm:"index"`
	Category      *KnowledgeCategory `json:"category,omitempty" gorm:"foreignKey:CategoryID"`
	SubCategoryID *uint              `json:"sub_category_id"`
	SubCategory   *KnowledgeCategory `json:"sub_category,omitempty" gorm:"foreignKey:SubCategoryID"`
	Tags          []string           `json:"tags" gorm:"type:text[]"`
	Keywords      []string           `json:"keywords" gorm:"type:text[]"`

	// Authorship and ownership
	AuthorID uint  `json:"author_id" gorm:"not null"`
	Author   *User `json:"author,omitempty" gorm:"foreignKey:AuthorID"`
	OwnerID  *uint `json:"owner_id"`
	Owner    *User `json:"owner,omitempty" gorm:"foreignKey:OwnerID"`
	ExpertID *uint `json:"expert_id"`
	Expert   *User `json:"expert,omitempty" gorm:"foreignKey:ExpertID"`

	// Review and approval
	ReviewerID    *uint      `json:"reviewer_id"`
	Reviewer      *User      `json:"reviewer,omitempty" gorm:"foreignKey:ReviewerID"`
	ReviewedAt    *time.Time `json:"reviewed_at"`
	ReviewNotes   string     `json:"review_notes"`
	ApproverID    *uint      `json:"approver_id"`
	Approver      *User      `json:"approver,omitempty" gorm:"foreignKey:ApproverID"`
	ApprovedAt    *time.Time `json:"approved_at"`
	ApprovalNotes string     `json:"approval_notes"`

	// Publishing
	PublishedAt   *time.Time `json:"published_at"`
	PublishedByID *uint      `json:"published_by_id"`
	PublishedBy   *User      `json:"published_by,omitempty" gorm:"foreignKey:PublishedByID"`
	ExpiryDate    *time.Time `json:"expiry_date"`
	ReviewDate    *time.Time `json:"review_date"`

	// Versioning
	Version         int               `json:"version" gorm:"default:1"`
	IsCurrent       bool              `json:"is_current" gorm:"default:true"`
	ParentArticleID *uint             `json:"parent_article_id"`
	ParentArticle   *KnowledgeArticle `json:"parent_article,omitempty" gorm:"foreignKey:ParentArticleID"`

	// Related entities
	RelatedIncidents []Incident          `json:"related_incidents,omitempty" gorm:"many2many:article_incidents;"`
	RelatedProblems  []Problem           `json:"related_problems,omitempty" gorm:"many2many:article_problems;"`
	RelatedChanges   []Change            `json:"related_changes,omitempty" gorm:"many2many:article_changes;"`
	RelatedCIs       []ConfigurationItem `json:"related_cis,omitempty" gorm:"many2many:article_cis;"`
	RelatedArticles  []KnowledgeArticle  `json:"related_articles,omitempty" gorm:"many2many:article_relationships;"`

	// Metrics and feedback
	ViewCount       int     `json:"view_count" gorm:"default:0"`
	UseCount        int     `json:"use_count" gorm:"default:0"`
	HelpfulCount    int     `json:"helpful_count" gorm:"default:0"`
	NotHelpfulCount int     `json:"not_helpful_count" gorm:"default:0"`
	Rating          float64 `json:"rating" gorm:"default:0"`
	RatingCount     int     `json:"rating_count" gorm:"default:0"`

	// Search and SEO
	MetaDescription string `json:"meta_description"`
	MetaKeywords    string `json:"meta_keywords"`
	Slug            string `json:"slug" gorm:"uniqueIndex"`
	SearchRank      int    `json:"search_rank" gorm:"default:0"`

	// Attachments and media
	HasAttachments  bool `json:"has_attachments" gorm:"default:false"`
	HasImages       bool `json:"has_images" gorm:"default:false"`
	HasVideos       bool `json:"has_videos" gorm:"default:false"`
	AttachmentCount int  `json:"attachment_count" gorm:"default:0"`

	// Access control
	RestrictedGroups []Group `json:"restricted_groups,omitempty" gorm:"many2many:article_group_access;"`
	RequiresAuth     bool    `json:"requires_auth" gorm:"default:false"`

	// Additional metadata
	Language       string `json:"language" gorm:"default:'en'"`
	ReadingTime    int    `json:"reading_time"` // Minutes
	Complexity     string `json:"complexity"`   // beginner, intermediate, advanced
	TargetAudience string `json:"target_audience"`
	Prerequisites  string `json:"prerequisites"`

	// Audit fields
	CreatedAt        time.Time  `json:"created_at"`
	UpdatedAt        time.Time  `json:"updated_at"`
	LastViewedAt     *time.Time `json:"last_viewed_at"`
	LastModifiedByID *uint      `json:"last_modified_by_id"`
	LastModifiedBy   *User      `json:"last_modified_by,omitempty" gorm:"foreignKey:LastModifiedByID"`
}

// KnowledgeCategory represents a category in the knowledge base.
type KnowledgeCategory struct {
	ID           uint                `json:"id" gorm:"primaryKey"`
	Name         string              `json:"name" gorm:"not null;uniqueIndex"`
	DisplayName  string              `json:"display_name"`
	Description  string              `json:"description"`
	Icon         string              `json:"icon"`
	Color        string              `json:"color"`
	ParentID     *uint               `json:"parent_id"`
	Parent       *KnowledgeCategory  `json:"parent,omitempty" gorm:"foreignKey:ParentID"`
	Children     []KnowledgeCategory `json:"children,omitempty" gorm:"foreignKey:ParentID"`
	Order        int                 `json:"order" gorm:"default:0"`
	IsActive     bool                `json:"is_active" gorm:"default:true"`
	ArticleCount int                 `json:"article_count" gorm:"default:0"`
	CreatedAt    time.Time           `json:"created_at"`
	UpdatedAt    time.Time           `json:"updated_at"`
}

// ArticleComment represents a comment on a knowledge article.
type ArticleComment struct {
	ID        uint              `json:"id" gorm:"primaryKey"`
	ArticleID uint              `json:"article_id" gorm:"not null"`
	Article   *KnowledgeArticle `json:"article,omitempty" gorm:"foreignKey:ArticleID"`
	AuthorID  uint              `json:"author_id" gorm:"not null"`
	Author    *User             `json:"author,omitempty" gorm:"foreignKey:AuthorID"`
	Comment   string            `json:"comment" gorm:"not null"`
	IsPublic  bool              `json:"is_public" gorm:"default:true"`
	IsHelpful *bool             `json:"is_helpful"`
	ParentID  *uint             `json:"parent_id"`
	Parent    *ArticleComment   `json:"parent,omitempty" gorm:"foreignKey:ParentID"`
	Replies   []ArticleComment  `json:"replies,omitempty" gorm:"foreignKey:ParentID"`
	CreatedAt time.Time         `json:"created_at"`
	UpdatedAt time.Time         `json:"updated_at"`
}

// KnowledgeArticleAttachment represents a file attached to an article.
type KnowledgeArticleAttachment struct {
	ID            uint              `json:"id" gorm:"primaryKey"`
	ArticleID     uint              `json:"article_id" gorm:"not null"`
	Article       *KnowledgeArticle `json:"article,omitempty" gorm:"foreignKey:ArticleID"`
	FileName      string            `json:"file_name" gorm:"not null"`
	FilePath      string            `json:"file_path" gorm:"not null"`
	FileSize      int64             `json:"file_size"`
	ContentType   string            `json:"content_type"`
	Description   string            `json:"description"`
	IsInline      bool              `json:"is_inline" gorm:"default:false"`
	DownloadCount int               `json:"download_count" gorm:"default:0"`
	UploadedByID  uint              `json:"uploaded_by_id" gorm:"not null"`
	UploadedBy    *User             `json:"uploaded_by,omitempty" gorm:"foreignKey:UploadedByID"`
	CreatedAt     time.Time         `json:"created_at"`
}

// ArticleFeedback represents user feedback on an article.
type ArticleFeedback struct {
	ID         uint              `json:"id" gorm:"primaryKey"`
	ArticleID  uint              `json:"article_id" gorm:"not null"`
	Article    *KnowledgeArticle `json:"article,omitempty" gorm:"foreignKey:ArticleID"`
	UserID     uint              `json:"user_id" gorm:"not null"`
	User       *User             `json:"user,omitempty" gorm:"foreignKey:UserID"`
	IsHelpful  bool              `json:"is_helpful"`
	Rating     int               `json:"rating"` // 1-5 scale
	Feedback   string            `json:"feedback"`
	Suggestion string            `json:"suggestion"`
	CreatedAt  time.Time         `json:"created_at"`
	UpdatedAt  time.Time         `json:"updated_at"`
}

// ArticleHistory tracks changes to articles.
type ArticleHistory struct {
	ID          uint              `json:"id" gorm:"primaryKey"`
	ArticleID   uint              `json:"article_id" gorm:"not null"`
	Article     *KnowledgeArticle `json:"article,omitempty" gorm:"foreignKey:ArticleID"`
	Version     int               `json:"version"`
	Title       string            `json:"title"`
	Content     string            `json:"content" gorm:"type:text"`
	Summary     string            `json:"summary"`
	ChangedByID uint              `json:"changed_by_id" gorm:"not null"`
	ChangedBy   *User             `json:"changed_by,omitempty" gorm:"foreignKey:ChangedByID"`
	ChangeType  string            `json:"change_type"` // create, update, publish, archive
	ChangeNotes string            `json:"change_notes"`
	CreatedAt   time.Time         `json:"created_at"`
}

// ArticleView tracks article views for analytics.
type ArticleView struct {
	ID           uint              `json:"id" gorm:"primaryKey"`
	ArticleID    uint              `json:"article_id" gorm:"not null;index"`
	Article      *KnowledgeArticle `json:"article,omitempty" gorm:"foreignKey:ArticleID"`
	UserID       *uint             `json:"user_id"`
	User         *User             `json:"user,omitempty" gorm:"foreignKey:UserID"`
	SessionID    string            `json:"session_id"`
	IPAddress    string            `json:"ip_address"`
	UserAgent    string            `json:"user_agent"`
	Referrer     string            `json:"referrer"`
	SearchQuery  string            `json:"search_query"`
	ViewDuration int               `json:"view_duration"` // Seconds
	CreatedAt    time.Time         `json:"created_at"`
}

// KnowledgeSearchRequest represents a search request for articles.
type KnowledgeSearchRequest struct {
	Query      string            `json:"query" form:"query"`
	Type       ArticleType       `json:"type" form:"type"`
	Status     ArticleStatus     `json:"status" form:"status"`
	CategoryID uint              `json:"category_id" form:"category_id"`
	AuthorID   uint              `json:"author_id" form:"author_id"`
	Tags       []string          `json:"tags" form:"tags"`
	Visibility ArticleVisibility `json:"visibility" form:"visibility"`
	Language   string            `json:"language" form:"language"`
	Page       int               `json:"page" form:"page"`
	PerPage    int               `json:"per_page" form:"per_page"`
	SortBy     string            `json:"sort_by" form:"sort_by"`
	SortOrder  string            `json:"sort_order" form:"sort_order"`
}

// KnowledgeSearchResponse represents search results.
type KnowledgeSearchResponse struct {
	Articles   []*KnowledgeArticle `json:"articles"`
	Total      int64               `json:"total"`
	Page       int                 `json:"page"`
	PerPage    int                 `json:"per_page"`
	TotalPages int                 `json:"total_pages"`
	SearchTime float64             `json:"search_time"` // Milliseconds
}
