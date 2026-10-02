package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/goatkit/goatflow/internal/platform/constants"
	"github.com/goatkit/goatflow/internal/platform/database"
)

type internalNoteResp struct {
	Message string         `json:"message"`
	NoteID  int            `json:"note_id"`
	Note    InternalNote   `json:"note"`
	Notes   []InternalNote `json:"notes"`
	Total   int            `json:"total"`
	Error   string         `json:"error"`
}

func decodeNoteResp(t *testing.T, body []byte) internalNoteResp {
	t.Helper()
	var r internalNoteResp
	require.NoError(t, json.Unmarshal(body, &r), string(body))
	return r
}

func TestInternalNotesAPI_StoredAsInternalArticles(t *testing.T) {
	db := getTestDB(t)
	ticketID := createWriteTestTicket(t, db, nil)
	author := map[string]any{"user_id": 1, "user_role": "agent"}
	base := fmt.Sprintf("/api/v1/tickets/%d/internal-notes", ticketID)

	// Create persists an internal, customer-invisible article with the body.
	w := serveWriteTest(http.MethodPost, "/api/v1/tickets/:id/internal-notes", base, "application/json",
		`{"content":"VIP customer, ping @team-billing"}`, author, HandleCreateInternalNote)
	require.Equal(t, http.StatusCreated, w.Code, w.Body.String())
	created := decodeNoteResp(t, w.Body.Bytes())
	noteID := created.NoteID
	require.Positive(t, noteID)
	assert.Equal(t, "VIP customer, ping @team-billing", created.Note.Content)
	assert.Equal(t, []string{"team-billing"}, created.Note.Mentions)
	assert.True(t, created.Note.HasTeamMention)
	assert.Equal(t, 1, countRows(t, db, `SELECT COUNT(*) FROM article WHERE id = ? AND ticket_id = ?
		AND communication_channel_id = ? AND is_visible_for_customer = 0`, noteID, ticketID, constants.CommunicationChannelInternal))
	var body string
	require.NoError(t, db.QueryRow(database.ConvertPlaceholders(`SELECT a_body FROM article_data_mime WHERE article_id = ?`), noteID).Scan(&body))
	assert.Equal(t, "VIP customer, ping @team-billing", body)

	// A customer-visible article on the same ticket is not an internal note.
	createdCustomerArticle := serveWriteTest(http.MethodPost, "/api/tickets/:id/reply", fmt.Sprintf("/api/tickets/%d/reply", ticketID),
		"application/x-www-form-urlencoded", "reply=public+answer", author, handleTicketReply)
	require.Equal(t, http.StatusOK, createdCustomerArticle.Code, createdCustomerArticle.Body.String())

	// List returns only the internal note; search filters on content.
	w = serveWriteTest(http.MethodGet, "/api/v1/tickets/:id/internal-notes", base, "", "", author, HandleGetInternalNotes)
	require.Equal(t, http.StatusOK, w.Code)
	listed := decodeNoteResp(t, w.Body.Bytes())
	require.Equal(t, 1, listed.Total)
	assert.Equal(t, noteID, listed.Notes[0].ID)
	w = serveWriteTest(http.MethodGet, "/api/v1/tickets/:id/internal-notes", base+"?search=nomatch", "", "", author, HandleGetInternalNotes)
	assert.Equal(t, 0, decodeNoteResp(t, w.Body.Bytes()).Total)

	// Another agent cannot edit or delete it.
	other := map[string]any{"user_id": 987654, "user_role": "agent"}
	noteURL := fmt.Sprintf("%s/%d", base, noteID)
	w = serveWriteTest(http.MethodPut, "/api/v1/tickets/:id/internal-notes/:note_id", noteURL, "application/json",
		`{"content":"hijack"}`, other, HandleUpdateInternalNote)
	assert.Equal(t, http.StatusForbidden, w.Code)
	w = serveWriteTest(http.MethodDelete, "/api/v1/tickets/:id/internal-notes/:note_id", noteURL, "", "", other, HandleDeleteInternalNote)
	assert.Equal(t, http.StatusForbidden, w.Code)

	// The author edits it: stored body changes.
	w = serveWriteTest(http.MethodPut, "/api/v1/tickets/:id/internal-notes/:note_id", noteURL, "application/json",
		`{"content":"edited"}`, author, HandleUpdateInternalNote)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.Equal(t, "edited", decodeNoteResp(t, w.Body.Bytes()).Note.Content)
	require.NoError(t, db.QueryRow(database.ConvertPlaceholders(`SELECT a_body FROM article_data_mime WHERE article_id = ?`), noteID).Scan(&body))
	assert.Equal(t, "edited", body)

	// The author deletes it: article gone, then 404.
	w = serveWriteTest(http.MethodDelete, "/api/v1/tickets/:id/internal-notes/:note_id", noteURL, "", "", author, HandleDeleteInternalNote)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.Equal(t, 0, countRows(t, db, `SELECT COUNT(*) FROM article WHERE id = ?`, noteID))
	w = serveWriteTest(http.MethodDelete, "/api/v1/tickets/:id/internal-notes/:note_id", noteURL, "", "", author, HandleDeleteInternalNote)
	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestInternalNotesAPI_Validation(t *testing.T) {
	db := getTestDB(t)
	ticketID := createWriteTestTicket(t, db, nil)
	agent := map[string]any{"user_id": 1, "user_role": "agent"}

	w := serveWriteTest(http.MethodPost, "/t/:id", "/t/2147483000", "application/json", `{"content":"x"}`, agent, HandleCreateInternalNote)
	assert.Equal(t, http.StatusNotFound, w.Code)

	w = serveWriteTest(http.MethodPost, "/t/:id", fmt.Sprintf("/t/%d", ticketID), "application/json", `{"content":"  "}`, agent, HandleCreateInternalNote)
	assert.Equal(t, http.StatusBadRequest, w.Code)

	w = serveWriteTest(http.MethodPost, "/t/:id", fmt.Sprintf("/t/%d", ticketID), "application/json", `{"content":"x"}`,
		map[string]any{"user_id": 1, "user_role": "customer"}, HandleCreateInternalNote)
	assert.Equal(t, http.StatusForbidden, w.Code)
	assert.Equal(t, 0, countRows(t, db, `SELECT COUNT(*) FROM article WHERE ticket_id = ?`, ticketID))
}
