package models

import "time"

// TimeAccounting represents a time accounting entry (OTRS-compatible)
// Compatible with OTRS table `time_accounting`:
// id, ticket_id, article_id, time_unit, create_time, create_by, change_time, change_by
type TimeAccounting struct {
    ID         int       `json:"id" db:"id"`
    TicketID   int       `json:"ticket_id" db:"ticket_id"`
    ArticleID  *int      `json:"article_id,omitempty" db:"article_id"`
    TimeUnit   int       `json:"time_unit" db:"time_unit"` // minutes
    CreateTime time.Time `json:"create_time" db:"create_time"`
    CreateBy   int       `json:"create_by" db:"create_by"`
    ChangeTime time.Time `json:"change_time" db:"change_time"`
    ChangeBy   int       `json:"change_by" db:"change_by"`
}
