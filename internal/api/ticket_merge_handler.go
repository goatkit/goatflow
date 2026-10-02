package api

import (
	"fmt"
	"log"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/goatkit/goatflow/internal/history"
	"github.com/goatkit/goatflow/internal/models"
	"github.com/goatkit/goatflow/internal/platform/database"
	"github.com/goatkit/goatflow/internal/repository"
)

// recordMergeHistory writes "Merged" ticket_history rows for the target ticket and
// each merged source ticket. It runs after the merge transaction has committed, so a
// failure here is logged rather than rolled back.
func recordMergeHistory(c *gin.Context, primaryID int, mergedIDs []int, reason string) {
	if primaryID <= 0 || len(mergedIDs) == 0 {
		return
	}

	db, err := database.GetDB()
	if err != nil || db == nil {
		log.Printf("history merge: database unavailable, merge of %v into %d not recorded: %v", mergedIDs, primaryID, err)
		return
	}

	repo := repository.NewTicketRepository(db)
	primaryTicket, err := repo.GetByID(uint(primaryID))
	if err != nil {
		log.Printf("history merge load primary %d failed: %v", primaryID, err)
		return
	}

	recorder := history.NewRecorder(repo)
	ctx := c.Request.Context()
	actorID := resolveActorID(c)
	reason = strings.TrimSpace(reason)

	mergedTickets := make([]*models.Ticket, 0, len(mergedIDs))
	labels := make([]string, 0, len(mergedIDs))
	for _, id := range mergedIDs {
		if id <= 0 {
			continue
		}
		ticket, terr := repo.GetByID(uint(id))
		if terr != nil {
			log.Printf("history merge load ticket %d failed: %v", id, terr)
			continue
		}
		mergedTickets = append(mergedTickets, ticket)
		labels = append(labels, ticketLabel(ticket))
	}

	if len(mergedTickets) == 0 {
		return
	}

	targetMsg := mergeSummaryMessage(labels, reason)
	if err := recorder.Record(ctx, nil, primaryTicket, nil, history.TypeMerged, targetMsg, actorID); err != nil {
		log.Printf("history merge record primary %d failed: %v", primaryID, err)
	}

	childMsg := mergeChildMessage(ticketLabel(primaryTicket), reason)
	for _, ticket := range mergedTickets {
		if err := recorder.Record(ctx, nil, ticket, nil, history.TypeMerged, childMsg, actorID); err != nil {
			log.Printf("history merge record ticket %d failed: %v", ticket.ID, err)
		}
	}
}

func resolveActorID(c *gin.Context) int {
	if c == nil {
		return 1
	}
	if userID := GetUserIDFromCtx(c, 0); userID > 0 {
		return userID
	}
	if userVal, ok := c.Get("user"); ok {
		if user, ok := userVal.(*models.User); ok && user.ID > 0 {
			return int(user.ID)
		}
	}
	return 1
}

func ticketLabel(ticket *models.Ticket) string {
	if ticket == nil {
		return ""
	}
	tn := strings.TrimSpace(ticket.TicketNumber)
	if tn != "" {
		return fmt.Sprintf("#%s", tn)
	}
	if ticket.ID > 0 {
		return fmt.Sprintf("#%d", ticket.ID)
	}
	return "ticket"
}

func mergeSummaryMessage(labels []string, reason string) string {
	if len(labels) == 0 {
		return appendReason("Tickets merged", reason)
	}
	base := ""
	if len(labels) == 1 {
		base = fmt.Sprintf("Merged ticket %s into this ticket", labels[0])
	} else {
		base = fmt.Sprintf("Merged tickets %s into this ticket", strings.Join(labels, ", "))
	}
	return appendReason(base, reason)
}

func mergeChildMessage(targetLabel, reason string) string {
	if targetLabel == "" {
		targetLabel = "target ticket"
	}
	return appendReason(fmt.Sprintf("Merged into ticket %s", targetLabel), reason)
}

func appendReason(message, reason string) string {
	reason = strings.TrimSpace(reason)
	if reason == "" {
		return message
	}
	return fmt.Sprintf("%s — %s", message, reason)
}
