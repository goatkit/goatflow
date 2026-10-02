package api

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/goatkit/goatflow/internal/platform/constants"
	"github.com/goatkit/goatflow/internal/platform/database"
	"github.com/goatkit/goatflow/internal/service"
)

const missingTicketID = 2147483000

func TestTicketReply_PersistsArticle(t *testing.T) {
	db := getTestDB(t)
	ticketID := createWriteTestTicket(t, db, nil)
	agent := map[string]any{"user_id": 1}
	url := fmt.Sprintf("/api/tickets/%d/reply", ticketID)

	w := serveWriteTest(http.MethodPost, "/api/tickets/:id/reply", url, "application/x-www-form-urlencoded",
		"reply=Hello+there&time_units=7", agent, handleTicketReply)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.Contains(t, w.Body.String(), "Hello there")
	assert.Equal(t, 1, countRows(t, db, `SELECT COUNT(*) FROM article a JOIN article_data_mime m ON m.article_id = a.id
		WHERE a.ticket_id = ? AND a.is_visible_for_customer = 1 AND a.communication_channel_id = ? AND m.a_body = 'Hello there'`,
		ticketID, constants.CommunicationChannelEmail))
	assert.Equal(t, 1, countRows(t, db, `SELECT COUNT(*) FROM time_accounting WHERE ticket_id = ? AND article_id IS NOT NULL`, ticketID))

	w = serveWriteTest(http.MethodPost, "/api/tickets/:id/reply", url, "application/x-www-form-urlencoded",
		"reply=secret&internal=true", agent, handleTicketReply)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.Equal(t, 1, countRows(t, db, `SELECT COUNT(*) FROM article WHERE ticket_id = ? AND is_visible_for_customer = 0
		AND communication_channel_id = ?`, ticketID, constants.CommunicationChannelInternal))

	w = serveWriteTest(http.MethodPost, "/api/tickets/:id/reply", fmt.Sprintf("/api/tickets/%d/reply", missingTicketID),
		"application/x-www-form-urlencoded", "reply=x", agent, handleTicketReply)
	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestAssignTicket_WritesOwnerAndRejectsUnknownTicket(t *testing.T) {
	db := getTestDB(t)
	ticketID := createWriteTestTicket(t, db, nil)

	agent := map[string]any{"user_id": 1}
	w := serveWriteTest(http.MethodPost, "/api/tickets/:id/assign", fmt.Sprintf("/api/tickets/%d/assign", missingTicketID),
		"application/x-www-form-urlencoded", "user_id=1", agent, handleAssignTicket)
	assert.Equal(t, http.StatusNotFound, w.Code, w.Body.String())

	w = serveWriteTest(http.MethodPost, "/api/tickets/:id/assign", fmt.Sprintf("/api/tickets/%d/assign", ticketID),
		"application/x-www-form-urlencoded", "user_id=1", agent, handleAssignTicket)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.Equal(t, 1, countRows(t, db, `SELECT COUNT(*) FROM ticket WHERE id = ? AND user_id = 1 AND responsible_user_id = 1`, ticketID))
}

// Regression: ticket.customer_user_id is nullable; the delete handler scanned it into a
// string, so archiving a ticket without a customer failed with 500.
func TestDeleteTicketAPI_TicketWithoutCustomer(t *testing.T) {
	db := getTestDB(t)
	ticketID := createWriteTestTicket(t, db, nil)

	w := serveWriteTest(http.MethodDelete, "/api/v1/tickets/:id", fmt.Sprintf("/api/v1/tickets/%d", ticketID), "", "",
		map[string]any{"user_id": 1}, HandleDeleteTicketAPI)
	require.Less(t, w.Code, 300, w.Body.String())
	assert.Equal(t, 0, countRows(t, db, `SELECT COUNT(*) FROM ticket t JOIN ticket_state s ON s.id = t.ticket_state_id
		WHERE t.id = ? AND s.name = 'new'`, ticketID), "ticket must have left its open state")
}

func TestUpdateTicketAPI_RealPath(t *testing.T) {
	db := getTestDB(t)
	ticketID := createWriteTestTicket(t, db, nil)
	agent := map[string]any{"user_id": 1}

	w := serveWriteTest(http.MethodPut, "/api/v1/tickets/:id", fmt.Sprintf("/api/v1/tickets/%d", missingTicketID),
		"application/json", `{"title":"x"}`, agent, HandleUpdateTicketAPI)
	assert.Equal(t, http.StatusNotFound, w.Code, w.Body.String())

	// An owner change on a missing ticket must not check owner rights
	// against queue 0; it reports the ticket as missing.
	w = serveWriteTest(http.MethodPut, "/api/v1/tickets/:id", fmt.Sprintf("/api/v1/tickets/%d", missingTicketID),
		"application/json", `{"user_id":1}`, agent, HandleUpdateTicketAPI)
	assert.Equal(t, http.StatusNotFound, w.Code, w.Body.String())

	w = serveWriteTest(http.MethodPut, "/api/v1/tickets/:id", "/api/v1/tickets/-1",
		"application/json", `{"title":"x"}`, agent, HandleUpdateTicketAPI)
	assert.Equal(t, http.StatusBadRequest, w.Code, w.Body.String())

	w = serveWriteTest(http.MethodPut, "/api/v1/tickets/:id", fmt.Sprintf("/api/v1/tickets/%d", ticketID),
		"application/json", `{"title":"Renamed by test"}`, agent, HandleUpdateTicketAPI)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.Equal(t, 1, countRows(t, db, `SELECT COUNT(*) FROM ticket WHERE id = ? AND title = 'Renamed by test'`, ticketID))
}

func TestWriteAPIs_TestModeHeaderDoesNotBypassAuth(t *testing.T) {
	db := getTestDB(t)
	ticketID := createWriteTestTicket(t, db, nil)
	serve := func(method, route, url, body string, h gin.HandlerFunc) int {
		r := gin.New()
		r.Handle(method, route, h)
		req := httptest.NewRequest(method, url, strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Test-Mode", "true")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		return w.Code
	}

	assert.Equal(t, http.StatusUnauthorized, serve(http.MethodPut, "/api/v1/tickets/:id",
		fmt.Sprintf("/api/v1/tickets/%d", ticketID), `{"title":"hacked"}`, HandleUpdateTicketAPI))
	assert.Equal(t, http.StatusUnauthorized, serve(http.MethodPost, "/api/v1/tickets",
		"/api/v1/tickets", `{"title":"t","queue_id":1,"body":"b"}`, HandleCreateTicketAPI))
	assert.Equal(t, http.StatusUnauthorized, serve(http.MethodPost, "/api/v1/tickets/:id/articles",
		fmt.Sprintf("/api/v1/tickets/%d/articles", ticketID), `{"subject":"s","body":"b"}`, HandleCreateArticleAPI))
	assert.Equal(t, 0, countRows(t, db, `SELECT COUNT(*) FROM article WHERE ticket_id = ?`, ticketID))
	assert.Equal(t, 0, countRows(t, db, `SELECT COUNT(*) FROM ticket WHERE id = ? AND title = 'hacked'`, ticketID))
}

func TestSimpleTicketService_AddMessagePersists(t *testing.T) {
	db := getTestDB(t)
	ticketID := createWriteTestTicket(t, db, nil)
	svc := GetTicketService()
	require.NotNil(t, svc)

	msg := &service.SimpleTicketMessage{Subject: "s", Body: "persist me", CreatedBy: 1, AuthorType: "Agent", IsInternal: true}
	require.NoError(t, svc.AddMessage(uint(ticketID), msg))
	require.Positive(t, msg.ID)
	assert.Equal(t, 1, countRows(t, db, `SELECT COUNT(*) FROM article a JOIN article_data_mime m ON m.article_id = a.id
		WHERE a.id = ? AND a.ticket_id = ? AND a.is_visible_for_customer = 0 AND m.a_body = 'persist me'`, msg.ID, ticketID))

	// A fresh service instance (as after a restart) sees the stored message.
	fresh := service.NewSimpleTicketService(GetTicketRepository(), db)
	msgs, err := fresh.GetMessages(uint(ticketID))
	require.NoError(t, err)
	require.Len(t, msgs, 1)
	assert.Equal(t, "persist me", msgs[0].Body)
	assert.True(t, msgs[0].IsInternal)

	assert.Error(t, svc.AddMessage(missingTicketID, &service.SimpleTicketMessage{Body: "x", CreatedBy: 1}))
}

func TestAgentTicketMerge_RecordsHistory(t *testing.T) {
	db := getTestDB(t)
	source := createWriteTestTicket(t, db, nil)
	target := createWriteTestTicket(t, db, nil)
	var targetTN string
	require.NoError(t, db.QueryRow(database.ConvertPlaceholders(`SELECT tn FROM ticket WHERE id = ?`), target).Scan(&targetTN))

	w := serveWriteTest(http.MethodPost, "/agent/tickets/:id/merge", fmt.Sprintf("/agent/tickets/%d/merge", source),
		"application/x-www-form-urlencoded", "target_tn="+targetTN, map[string]any{"user_id": uint(1)}, handleAgentTicketMerge(db))
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())

	for _, id := range []int{source, target} {
		assert.Equal(t, 1, countRows(t, db, `SELECT COUNT(*) FROM ticket_history h JOIN ticket_history_type ht ON ht.id = h.history_type_id
			WHERE h.ticket_id = ? AND ht.name = 'Merged'`, id), "ticket %d", id)
	}
}

func TestUpdateTicketPriorityAndQueue_RealPath(t *testing.T) {
	db := getTestDB(t)
	ticketID := createWriteTestTicket(t, db, nil)
	var priorityID, queueID int
	require.NoError(t, db.QueryRow(database.ConvertPlaceholders(
		`SELECT id FROM ticket_priority WHERE id <> 1 AND valid_id = 1 ORDER BY id`)).Scan(&priorityID))
	require.NoError(t, db.QueryRow(database.ConvertPlaceholders(
		`SELECT id FROM queue WHERE id <> 1 AND valid_id = 1 ORDER BY id`)).Scan(&queueID))
	agent := map[string]any{"user_id": uint(1)}

	w := serveWriteTest(http.MethodPost, "/api/tickets/:id/priority", fmt.Sprintf("/api/tickets/%d/priority", ticketID),
		"application/x-www-form-urlencoded", fmt.Sprintf("priority=%d", priorityID), agent, handleUpdateTicketPriority)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.Equal(t, 1, countRows(t, db, `SELECT COUNT(*) FROM ticket WHERE id = ? AND ticket_priority_id = ?`, ticketID, priorityID))

	w = serveWriteTest(http.MethodPost, "/api/tickets/:id/queue", fmt.Sprintf("/api/tickets/%d/queue", ticketID),
		"application/x-www-form-urlencoded", fmt.Sprintf("queue_id=%d", queueID), agent, handleUpdateTicketQueue)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.Equal(t, 1, countRows(t, db, `SELECT COUNT(*) FROM ticket WHERE id = ? AND queue_id = ?`, ticketID, queueID))

	w = serveWriteTest(http.MethodPost, "/api/tickets/:id/queue", fmt.Sprintf("/api/tickets/%d/queue", ticketID),
		"application/x-www-form-urlencoded", "", agent, handleUpdateTicketQueue)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}
