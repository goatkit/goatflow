// Package api provides HTTP handlers for the API.
package api

import (
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/flosch/pongo2/v6"
	"github.com/gin-gonic/gin"

	"github.com/goatkit/goatflow/internal/database"
	"github.com/goatkit/goatflow/internal/history"
	"github.com/goatkit/goatflow/internal/repository"
)

// HandleTicketHistoryFragment is an HTMX handler for ticket history fragment in tab panel.
func HandleTicketHistoryFragment(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.String(http.StatusBadRequest, "Invalid ticket id")
		return
	}

	db, err := database.GetDB()
	if err != nil {
		c.String(http.StatusInternalServerError, "Database unavailable")
		return
	}

	repo := repository.NewTicketRepository(db)
	entries, err := repo.GetTicketHistoryEntries(uint(id), 25)
	if err != nil {
		c.String(http.StatusInternalServerError, "Failed to load history")
		return
	}

	historyRows := make([]map[string]string, 0, len(entries))
	for _, entry := range entries {
		actor := strings.TrimSpace(entry.CreatorFullName)
		if actor == "" {
			actor = entry.CreatorLogin
		}
		if actor == "" {
			actor = "System"
		}

		when := entry.CreatedAt.In(time.Local).Format("02 Jan 2006 15:04")

		displayName := history.NormalizeHistoryName(entry)
		event := entry.HistoryType
		if displayName != "" && !strings.EqualFold(displayName, event) {
			if event != "" {
				event = fmt.Sprintf("%s — %s", event, displayName)
			} else {
				event = displayName
			}
		}
		if event == "" {
			event = "Ticket update"
		}

		metaParts := make([]string, 0, 4)
		if entry.QueueName != "" {
			metaParts = append(metaParts, entry.QueueName)
		}
		if entry.StateName != "" {
			metaParts = append(metaParts, entry.StateName)
		}
		if entry.PriorityName != "" {
			metaParts = append(metaParts, fmt.Sprintf("Priority %s", entry.PriorityName))
		}
		if entry.ArticleSubject != "" {
			metaParts = append(metaParts, entry.ArticleSubject)
		}
		meta := strings.Join(metaParts, " • ")
		if meta == "" {
			meta = fmt.Sprintf("#%d", entry.ID)
		}

		historyRows = append(historyRows, map[string]string{
			"event": event,
			"by":    actor,
			"when":  when,
			"meta":  meta,
		})
	}

	renderer := getPongo2Renderer()
	if renderer == nil {
		c.String(http.StatusInternalServerError, "Template renderer unavailable")
		return
	}

	renderer.HTML(c, http.StatusOK, "partials/tickets/ticket_history.pongo2", pongo2.Context{
		"history": historyRows,
	})
}

// HandleTicketLinksFragment is an HTMX handler for ticket links fragment in tab panel.
func HandleTicketLinksFragment(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.String(http.StatusBadRequest, "Invalid ticket id")
		return
	}

	db, err := database.GetDB()
	if err != nil {
		c.String(http.StatusInternalServerError, "Database unavailable")
		return
	}

	repo := repository.NewTicketRepository(db)
	entries, err := repo.GetTicketLinks(uint(id), 25)
	if err != nil {
		log.Printf("ticket links load error (ticketID=%d): %v", id, err)
		c.String(http.StatusInternalServerError, "Failed to load links")
		return
	}

	links := make([]map[string]string, 0, len(entries))
	for _, entry := range entries {
		titleParts := make([]string, 0, 2)
		if entry.RelatedTicketTN != "" {
			titleParts = append(titleParts, entry.RelatedTicketTN)
		}
		if entry.RelatedTicketTitle != "" {
			titleParts = append(titleParts, entry.RelatedTicketTitle)
		}
		title := strings.Join(titleParts, " — ")
		if title == "" {
			title = fmt.Sprintf("Ticket #%d", entry.RelatedTicketID)
		}

		typeLabel := entry.LinkType
		if typeLabel == "" {
			typeLabel = "related"
		}
		if entry.Direction != "" {
			dirLabel := entry.Direction
			if len(dirLabel) > 0 {
				dirLabel = strings.ToUpper(dirLabel[:1]) + dirLabel[1:]
			}
			typeLabel = fmt.Sprintf("%s (%s)", typeLabel, dirLabel)
		}

		actor := strings.TrimSpace(entry.CreatorFullName)
		if actor == "" {
			actor = entry.CreatorLogin
		}
		if actor == "" {
			actor = "System"
		}

		noteParts := make([]string, 0, 2)
		if entry.LinkState != "" {
			noteParts = append(noteParts, entry.LinkState)
		}
		noteParts = append(noteParts, fmt.Sprintf("%s on %s", actor, entry.CreatedAt.In(time.Local).Format("02 Jan 2006 15:04")))
		note := strings.Join(noteParts, " • ")

		links = append(links, map[string]string{
			"href":  fmt.Sprintf("/agent/tickets/%d", entry.RelatedTicketID),
			"title": title,
			"type":  typeLabel,
			"note":  note,
		})
	}

	renderer := getPongo2Renderer()
	if renderer == nil {
		c.String(http.StatusInternalServerError, "Template renderer unavailable")
		return
	}

	renderer.HTML(c, http.StatusOK, "partials/tickets/ticket_links.pongo2", pongo2.Context{
		"links": links,
	})
}
