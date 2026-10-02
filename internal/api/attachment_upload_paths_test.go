package api

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/textproto"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/goatkit/goatflow/internal/platform/database"
	"github.com/goatkit/goatflow/internal/platform/routing"
	"github.com/goatkit/goatflow/internal/repository"
	"github.com/goatkit/goatflow/internal/storage"
	"github.com/goatkit/goatflow/internal/ticketnumber"
)

// Every path that accepts uploaded files must store them through the
// configured article storage: DB rows, or the OTRS ArticleStorageFS tree
// (and then no attachment rows at all).

var uploadPDF = []byte("%PDF-1.4\n%\xe2\xe3\xcf\xd3\n1 0 obj << /Type /Catalog >> endobj\ntrailer\n%%EOF\n")

type uploadFile struct {
	field, name, contentType string
	content                  []byte
}

func multipartForm(t *testing.T, fields map[string]string, files ...uploadFile) (*bytes.Buffer, string) {
	t.Helper()
	body := &bytes.Buffer{}
	w := multipart.NewWriter(body)
	for k, v := range fields {
		require.NoError(t, w.WriteField(k, v))
	}
	for _, f := range files {
		h := make(textproto.MIMEHeader)
		h.Set("Content-Disposition", fmt.Sprintf(`form-data; name=%q; filename=%q`, f.field, f.name))
		h.Set("Content-Type", f.contentType)
		part, err := w.CreatePart(h)
		require.NoError(t, err)
		_, err = part.Write(f.content)
		require.NoError(t, err)
	}
	require.NoError(t, w.Close())
	return body, w.FormDataContentType()
}

// forEachStorageBackend runs fn once with the DB backend and once with the
// FS backend rooted in a fresh directory.
func forEachStorageBackend(t *testing.T, fn func(t *testing.T, backend, articleDir string)) {
	for _, backend := range []string{storage.BackendDB, storage.BackendFS} {
		t.Run(backend, func(t *testing.T) {
			dir := t.TempDir()
			require.NoError(t, storage.Configure(storage.Config{Backend: backend, ArticleDir: dir}))
			t.Cleanup(func() { _ = storage.Configure(storage.Config{Backend: storage.BackendDB}) })
			fn(t, backend, dir)
		})
	}
}

// deleteTicketRows removes a ticket created by a test with all its articles.
func deleteTicketRows(t *testing.T, db *sql.DB, ticketID int64) {
	t.Helper()
	t.Cleanup(func() {
		for _, q := range []string{
			"DELETE FROM article_data_mime_attachment WHERE article_id IN (SELECT id FROM article WHERE ticket_id = ?)",
			"DELETE FROM article_data_mime_plain WHERE article_id IN (SELECT id FROM article WHERE ticket_id = ?)",
			"DELETE FROM article_data_mime WHERE article_id IN (SELECT id FROM article WHERE ticket_id = ?)",
			"DELETE FROM time_accounting WHERE ticket_id = ?",
			"DELETE FROM ticket_history WHERE ticket_id = ?",
			"DELETE FROM article WHERE ticket_id = ?",
			"DELETE FROM ticket WHERE id = ?",
		} {
			_, _ = db.Exec(database.ConvertPlaceholders(q), ticketID)
		}
	})
}

// requireStoredUpload asserts that the article holds exactly one user
// attachment named name with the given bytes, read back through storage, and
// that it physically lives where the backend keeps it.
func requireStoredUpload(t *testing.T, db *sql.DB, backend, articleDir string, articleID int64, name, contentType string, content []byte) {
	t.Helper()
	ctx := context.Background()
	all, err := storage.ForDB(db).ListAttachments(ctx, articleID)
	require.NoError(t, err)
	var atts []storage.Attachment
	for _, a := range all {
		if !storage.IsHTMLBody(a) {
			atts = append(atts, a)
		}
	}
	require.Len(t, atts, 1, "attachments of article %d: %+v", articleID, all)
	a := atts[0]
	assert.Equal(t, name, a.Filename)
	assert.Equal(t, contentType, a.ContentType)
	assert.Equal(t, "attachment", a.Disposition)
	assert.Equal(t, int64(len(content)), a.Size)
	_, got, err := storage.ForDB(db).GetAttachment(ctx, articleID, a.FileID)
	require.NoError(t, err)
	assert.Equal(t, content, got)

	var rows int
	require.NoError(t, db.QueryRow(database.ConvertPlaceholders(
		"SELECT COUNT(*) FROM article_data_mime_attachment WHERE article_id = ? AND filename = ?"),
		articleID, name).Scan(&rows))
	if backend == storage.BackendDB {
		assert.Equal(t, 1, rows, "exactly one DB row (no double write)")
		return
	}
	assert.Zero(t, rows, "FS backend must not also write a DB row")
	matches, err := filepath.Glob(filepath.Join(articleDir, "*", "*", "*", strconv.FormatInt(articleID, 10), name))
	require.NoError(t, err)
	require.Len(t, matches, 1, "%s under <dir>/YYYY/MM/DD/<article id>/", name)
	onDisk, err := os.ReadFile(matches[0])
	require.NoError(t, err)
	assert.Equal(t, content, onDisk)
	ct, err := os.ReadFile(matches[0] + ".content_type")
	require.NoError(t, err)
	assert.Equal(t, contentType, string(ct))
}

func latestArticleID(t *testing.T, db *sql.DB, ticketID int64) int64 {
	t.Helper()
	var id int64
	require.NoError(t, db.QueryRow(database.ConvertPlaceholders(
		"SELECT MAX(id) FROM article WHERE ticket_id = ?"), ticketID).Scan(&id))
	return id
}

func yamlRouter(t *testing.T) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	r := gin.New()
	require.NoError(t, routing.LoadYAMLRoutesForTesting(r))
	return r
}

func serve(r http.Handler, req *http.Request) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func TestAgentNoteStoresAttachment(t *testing.T) {
	db := getTestDB(t)
	router := yamlRouter(t)
	token := GetTestAuthToken(t)

	forEachStorageBackend(t, func(t *testing.T, backend, dir string) {
		ticketID, _ := createAttachmentTestArticle(t, db, "Note upload", "base")
		deleteTicketRows(t, db, ticketID)

		body, ctype := multipartForm(t, map[string]string{"body": "see attached", "subject": "Note"},
			uploadFile{"attachments", "report.pdf", "application/pdf", uploadPDF})
		req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/agent/tickets/%d/note", ticketID), body)
		req.Header.Set("Content-Type", ctype)
		AddTestAuthCookie(req, token)
		w := serve(router, req)
		require.Equal(t, http.StatusOK, w.Code, w.Body.String())

		var resp struct {
			ArticleID int64 `json:"article_id"`
		}
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
		require.Positive(t, resp.ArticleID)
		requireStoredUpload(t, db, backend, dir, resp.ArticleID, "report.pdf", "application/pdf", uploadPDF)
	})
}

func TestAgentReplyStoresAttachmentInTransaction(t *testing.T) {
	db := getTestDB(t)
	router := yamlRouter(t)
	token := GetTestAuthToken(t)

	forEachStorageBackend(t, func(t *testing.T, backend, dir string) {
		ticketID, _ := createAttachmentTestArticle(t, db, "Reply upload", "base")
		deleteTicketRows(t, db, ticketID)

		body, ctype := multipartForm(t,
			map[string]string{"to": "customer@example.com", "subject": "Re: upload", "body": "answer"},
			uploadFile{"attachments", "answer.pdf", "application/pdf", uploadPDF},
			uploadFile{"attachments", "payload.exe", "application/octet-stream", []byte("MZ")})
		req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/agent/tickets/%d/reply", ticketID), body)
		req.Header.Set("Content-Type", ctype)
		AddTestAuthCookie(req, token)
		w := serve(router, req)
		require.Equal(t, http.StatusOK, w.Code, w.Body.String())

		var resp struct {
			ArticleID int64 `json:"article_id"`
		}
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
		require.Positive(t, resp.ArticleID)
		// The blocked .exe is refused; only the PDF is stored.
		requireStoredUpload(t, db, backend, dir, resp.ArticleID, "answer.pdf", "application/pdf", uploadPDF)
	})
}

func TestCreateTicketWithAttachmentsStoresUpload(t *testing.T) {
	db := getTestDB(t)
	gen, err := ticketnumber.Resolve("DateChecksum", "10", nil)
	require.NoError(t, err)
	repository.SetTicketNumberGenerator(gen, ticketnumber.NewDBStore(db, "10"))
	t.Cleanup(func() { require.NoError(t, initTestTicketNumberGenerator()) })
	router := yamlRouter(t)
	token := GetTestAuthToken(t)

	forEachStorageBackend(t, func(t *testing.T, backend, dir string) {
		body, ctype := multipartForm(t, map[string]string{
			"title": "Upload on create", "customer_email": "upload@example.com", "body": "with file", "queue_id": "1",
		}, uploadFile{"attachments", "spec.pdf", "application/pdf", uploadPDF})
		req := httptest.NewRequest(http.MethodPost, "/tickets", body)
		req.Header.Set("Content-Type", ctype)
		AddTestAuthCookie(req, token)
		w := serve(router, req)
		require.Equal(t, http.StatusCreated, w.Code, w.Body.String())

		var resp struct {
			ID          int64 `json:"id"`
			Attachments []struct {
				Filename string `json:"filename"`
				Size     int64  `json:"size"`
			} `json:"attachments"`
		}
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
		require.Positive(t, resp.ID)
		deleteTicketRows(t, db, resp.ID)
		require.Len(t, resp.Attachments, 1)
		assert.Equal(t, "spec.pdf", resp.Attachments[0].Filename)
		assert.Equal(t, int64(len(uploadPDF)), resp.Attachments[0].Size)

		requireStoredUpload(t, db, backend, dir, latestArticleID(t, db, resp.ID), "spec.pdf", "application/pdf", uploadPDF)
	})

	t.Run("blocked extension is refused", func(t *testing.T) {
		body, ctype := multipartForm(t, map[string]string{
			"title": "Blocked upload", "customer_email": "upload@example.com", "body": "with exe", "queue_id": "1",
		}, uploadFile{"attachments", "tool.exe", "application/octet-stream", []byte("MZ")})
		req := httptest.NewRequest(http.MethodPost, "/tickets", body)
		req.Header.Set("Content-Type", ctype)
		AddTestAuthCookie(req, token)
		w := serve(router, req)
		require.Equal(t, http.StatusBadRequest, w.Code, w.Body.String())
		assert.Contains(t, w.Body.String(), "file type not allowed: .exe")
	})
}

func TestCustomerCreateTicketStoresAttachment(t *testing.T) {
	db := getTestDB(t)
	login := fmt.Sprintf("upload-%d", time.Now().UnixNano())
	_, err := db.Exec(database.ConvertPlaceholders(`
		INSERT INTO customer_user (login, email, customer_id, first_name, last_name, valid_id,
			create_time, create_by, change_time, change_by)
		VALUES (?, ?, 'upload-co', 'Up', 'Loader', 1, NOW(), 1, NOW(), 1)`), login, login+"@example.com")
	require.NoError(t, err)
	t.Cleanup(func() {
		_, _ = db.Exec(database.ConvertPlaceholders("DELETE FROM customer_user WHERE login = ?"), login)
	})

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.POST("/customer/tickets/create", func(c *gin.Context) {
		c.Set("username", login)
		c.Set("user_role", "Customer")
	}, handleCustomerCreateTicket(db))

	forEachStorageBackend(t, func(t *testing.T, backend, dir string) {
		body, ctype := multipartForm(t, map[string]string{"title": "Customer upload", "message": "file attached"},
			uploadFile{"attachments", "photo.pdf", "application/pdf", uploadPDF})
		req := httptest.NewRequest(http.MethodPost, "/customer/tickets/create", body)
		req.Header.Set("Content-Type", ctype)
		w := serve(router, req)
		require.Equal(t, http.StatusSeeOther, w.Code, w.Body.String())

		loc := w.Header().Get("Location")
		require.True(t, strings.HasPrefix(loc, "/customer/tickets/"), loc)
		ticketID, err := strconv.ParseInt(strings.TrimPrefix(loc, "/customer/tickets/"), 10, 64)
		require.NoError(t, err)
		deleteTicketRows(t, db, ticketID)

		requireStoredUpload(t, db, backend, dir, latestArticleID(t, db, ticketID), "photo.pdf", "application/pdf", uploadPDF)
	})
}

// TestTicketMessagesAttachmentURLsResolve checks that the messages API lists
// an article's attachments (not its HTML body) with article-scoped URLs that
// the attachment routes actually serve.
func TestTicketMessagesAttachmentURLsResolve(t *testing.T) {
	db := getTestDB(t)
	router := yamlRouter(t)
	token := GetTestAuthToken(t)

	forEachStorageBackend(t, func(t *testing.T, backend, dir string) {
		ticketID, articleID := createAttachmentTestArticle(t, db, "Messages", "body")
		deleteTicketRows(t, db, ticketID)
		store := storage.ForDB(db)
		_, err := store.WriteAttachment(context.Background(), articleID, storage.NewAttachment{
			Filename: storage.HTMLBodyFilename, ContentType: "text/html; charset=utf-8", Disposition: "inline",
			Content: []byte("<p>body</p>"),
		})
		require.NoError(t, err)
		att, err := store.WriteAttachment(context.Background(), articleID, storage.NewAttachment{
			Filename: "manual.pdf", ContentType: "application/pdf", Disposition: "attachment", Content: uploadPDF,
		})
		require.NoError(t, err)

		req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/tickets/%d/messages", ticketID), nil)
		AddTestAuthCookie(req, token)
		w := serve(router, req)
		require.Equal(t, http.StatusOK, w.Code, w.Body.String())
		var resp struct {
			Messages []struct {
				ID          int64 `json:"id"`
				Attachments []struct {
					ID       int64  `json:"id"`
					Filename string `json:"filename"`
					URL      string `json:"url"`
				} `json:"attachments"`
			} `json:"messages"`
		}
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
		require.Len(t, resp.Messages, 1)
		require.Len(t, resp.Messages[0].Attachments, 1, "HTML body part is not an attachment")
		got := resp.Messages[0].Attachments[0]
		wantURL := fmt.Sprintf("/api/tickets/%d/articles/%d/attachments/%d", ticketID, articleID, att.FileID)
		assert.Equal(t, "manual.pdf", got.Filename)
		assert.Equal(t, att.FileID, got.ID)
		require.Equal(t, wantURL, got.URL)

		req = httptest.NewRequest(http.MethodGet, got.URL, nil)
		AddTestAuthCookie(req, token)
		w = serve(router, req)
		require.Equal(t, http.StatusOK, w.Code, w.Body.String())
		assert.Equal(t, uploadPDF, w.Body.Bytes())

		req = httptest.NewRequest(http.MethodGet, got.URL+"/thumbnail", nil)
		AddTestAuthCookie(req, token)
		w = serve(router, req)
		require.Equal(t, http.StatusOK, w.Code)
		assert.True(t, strings.HasPrefix(w.Header().Get("Content-Type"), "image/"), w.Header().Get("Content-Type"))

		req = httptest.NewRequest(http.MethodGet, got.URL+"/view", nil)
		AddTestAuthCookie(req, token)
		w = serve(router, req)
		require.Equal(t, http.StatusOK, w.Code)

		// The HTMX fragment links the same URLs, escaped.
		req = httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/tickets/%d/messages", ticketID), nil)
		req.Header.Set("HX-Request", "true")
		AddTestAuthCookie(req, token)
		w = serve(router, req)
		require.Equal(t, http.StatusOK, w.Code)
		assert.Contains(t, w.Body.String(), `href="`+wantURL+`/view"`)
		assert.Contains(t, w.Body.String(), `href="`+wantURL+`" download`)
		assert.NotContains(t, w.Body.String(), storage.HTMLBodyFilename)
	})
}
