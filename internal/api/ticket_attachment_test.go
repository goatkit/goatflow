package api

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/textproto"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/net/html"

	"github.com/goatkit/goatflow/internal/platform/constants"
	"github.com/goatkit/goatflow/internal/platform/database"
	"github.com/goatkit/goatflow/internal/platform/routing"
	"github.com/goatkit/goatflow/internal/storage"
)

// attachmentAPI drives the agent attachment routes through the real YAML
// routes and auth middleware as an authenticated agent (user 1).
type attachmentAPI struct {
	t      *testing.T
	router *gin.Engine
	auth   string
}

func newAttachmentAPI(t *testing.T) *attachmentAPI {
	t.Helper()
	gin.SetMode(gin.TestMode)
	router := gin.New()
	require.NoError(t, routing.LoadYAMLRoutesForTesting(router))
	token := testSessionToken(t, 1, "root@localhost", "root@localhost", "Agent", false, 0)
	return &attachmentAPI{t: t, router: router, auth: "Bearer " + token}
}

func (a *attachmentAPI) do(method, path string, body io.Reader, headers ...string) *httptest.ResponseRecorder {
	a.t.Helper()
	req := httptest.NewRequest(method, path, body)
	req.Header.Set("Authorization", a.auth)
	for i := 0; i+1 < len(headers); i += 2 {
		req.Header.Set(headers[i], headers[i+1])
	}
	w := httptest.NewRecorder()
	a.router.ServeHTTP(w, req)
	return w
}

// uploadResponse is the 201 body of POST /api/tickets/:id/attachments.
type uploadResponse struct {
	ArticleID    int64  `json:"article_id"`
	FileID       int64  `json:"file_id"`
	Filename     string `json:"filename"`
	Size         int64  `json:"size"`
	ContentType  string `json:"content_type"`
	DownloadURL  string `json:"download_url"`
	ThumbnailURL string `json:"thumbnail_url"`
}

// upload posts one file; contentType is the part's browser-provided type.
func (a *attachmentAPI) upload(tn, filename, contentType string, content []byte) *httptest.ResponseRecorder {
	a.t.Helper()
	body := &bytes.Buffer{}
	mw := multipart.NewWriter(body)
	h := make(textproto.MIMEHeader)
	quote := strings.NewReplacer(`\`, `\\`, `"`, `\"`)
	h.Set("Content-Disposition", fmt.Sprintf(`form-data; name="file"; filename="%s"`, quote.Replace(filename)))
	h.Set("Content-Type", contentType)
	part, err := mw.CreatePart(h)
	require.NoError(a.t, err)
	_, err = part.Write(content)
	require.NoError(a.t, err)
	require.NoError(a.t, mw.Close())
	return a.do(http.MethodPost, "/api/tickets/"+tn+"/attachments", body, "Content-Type", mw.FormDataContentType())
}

func (a *attachmentAPI) mustUpload(tn, filename, contentType string, content []byte) uploadResponse {
	a.t.Helper()
	w := a.upload(tn, filename, contentType, content)
	require.Equal(a.t, http.StatusCreated, w.Code, w.Body.String())
	var resp uploadResponse
	require.NoError(a.t, json.Unmarshal(w.Body.Bytes(), &resp))
	return resp
}

type attachmentListResponse struct {
	Attachments []attachmentListItem `json:"attachments"`
	Total       int                  `json:"total"`
}

func (a *attachmentAPI) list(tn string) attachmentListResponse {
	a.t.Helper()
	w := a.do(http.MethodGet, "/api/tickets/"+tn+"/attachments", nil)
	require.Equal(a.t, http.StatusOK, w.Code, w.Body.String())
	var resp attachmentListResponse
	require.NoError(a.t, json.Unmarshal(w.Body.Bytes(), &resp))
	return resp
}

func findListItem(t *testing.T, items []attachmentListItem, filename string) attachmentListItem {
	t.Helper()
	for _, it := range items {
		if it.Filename == filename {
			return it
		}
	}
	require.Failf(t, "attachment not listed", "%q not in %+v", filename, items)
	return attachmentListItem{}
}

// eachArticleStorage runs fn once with the DB backend and once with the FS
// backend (articleDir is "" for DB).
func eachArticleStorage(t *testing.T, fn func(t *testing.T, articleDir string)) {
	t.Run("DB", func(t *testing.T) {
		require.NoError(t, storage.Configure(storage.Config{Backend: storage.BackendDB}))
		fn(t, "")
	})
	t.Run("FS", func(t *testing.T) {
		dir := t.TempDir()
		require.NoError(t, storage.Configure(storage.Config{Backend: storage.BackendFS, ArticleDir: dir}))
		t.Cleanup(func() { _ = storage.Configure(storage.Config{Backend: storage.BackendDB}) })
		fn(t, dir)
	})
}

func attachmentTestDB(t *testing.T) *sql.DB {
	t.Helper()
	require.NoError(t, database.InitTestDB())
	db, err := database.GetDB()
	require.NoError(t, err)
	return db
}

func ticketTN(t *testing.T, db *sql.DB, ticketID int64) string {
	t.Helper()
	var tn string
	require.NoError(t, db.QueryRow(database.ConvertPlaceholders("SELECT tn FROM ticket WHERE id = ?"), ticketID).Scan(&tn))
	return tn
}

// createAttachmentTestTicket inserts a ticket without articles; everything
// attached to it is removed when the test ends.
func createAttachmentTestTicket(t *testing.T, db *sql.DB) (ticketID int64, tn string) {
	t.Helper()
	tn = fmt.Sprintf("TEST-ATTN-%d", time.Now().UnixNano())
	ticketID, err := database.GetAdapter().InsertWithReturning(db, database.ConvertPlaceholders(`
		INSERT INTO ticket (tn, title, queue_id, type_id, ticket_state_id, ticket_priority_id,
		                    ticket_lock_id, user_id, responsible_user_id,
		                    timeout, until_time, escalation_time, escalation_update_time,
		                    escalation_response_time, escalation_solution_time,
		                    create_time, create_by, change_time, change_by)
		VALUES (?, 'Ticket without articles', 1, 1, 1, 1, 1, 1, 1,
		        0, 0, 0, 0, 0, 0, NOW(), 1, NOW(), 1)
		RETURNING id
	`), tn)
	require.NoError(t, err)
	t.Cleanup(func() {
		sub := "(SELECT id FROM article WHERE ticket_id = ?)"
		db.Exec(database.ConvertPlaceholders("DELETE FROM article_data_mime_attachment WHERE article_id IN "+sub), ticketID)
		db.Exec(database.ConvertPlaceholders("DELETE FROM article_data_mime WHERE article_id IN "+sub), ticketID)
		db.Exec(database.ConvertPlaceholders("DELETE FROM article WHERE ticket_id = ?"), ticketID)
		db.Exec(database.ConvertPlaceholders("DELETE FROM ticket WHERE id = ?"), ticketID)
	})
	return ticketID, tn
}

// articleStorageDir is where OTRS ArticleStorageFS keeps an article's files:
// <article dir>/<content_path, or the article's create date>/<article id>.
func articleStorageDir(t *testing.T, db *sql.DB, articleDir string, articleID int64) string {
	t.Helper()
	var contentPath sql.NullString
	err := db.QueryRow(database.ConvertPlaceholders(`
		SELECT content_path FROM article_data_mime
		WHERE article_id = ? AND content_path IS NOT NULL AND content_path <> ''`), articleID).Scan(&contentPath)
	if err == sql.ErrNoRows {
		var created time.Time
		require.NoError(t, db.QueryRow(database.ConvertPlaceholders(
			"SELECT create_time FROM article WHERE id = ?"), articleID).Scan(&created))
		contentPath = sql.NullString{String: created.Format("2006/01/02"), Valid: true}
	} else {
		require.NoError(t, err)
	}
	return filepath.Join(articleDir, filepath.FromSlash(contentPath.String), fmt.Sprint(articleID))
}

func testPNG(t *testing.T, w, h int) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := range h {
		for x := range w {
			img.Set(x, y, color.RGBA{R: uint8(x), G: uint8(y), B: 200, A: 255})
		}
	}
	var buf bytes.Buffer
	require.NoError(t, png.Encode(&buf, img))
	return buf.Bytes()
}

const testPDF = "%PDF-1.4\n1 0 obj<</Type/Catalog/Pages 2 0 R>>endobj\n" +
	"2 0 obj<</Type/Pages/Kids[3 0 R]/Count 1>>endobj\n" +
	"3 0 obj<</Type/Page/MediaBox[0 0 200 200]/Parent 2 0 R>>endobj\n" +
	"trailer<</Root 1 0 R>>\n%%EOF\n"

// TestAgentAttachmentAPI walks upload, list, download, thumbnail, viewer,
// ticket scoping and delete through the real routes, on both storage backends.
func TestAgentAttachmentAPI(t *testing.T) {
	db := attachmentTestDB(t)
	api := newAttachmentAPI(t)

	eachArticleStorage(t, func(t *testing.T, articleDir string) {
		ticketID, articleID := createAttachmentTestArticle(t, db, "Attachments", "body")
		tn := ticketTN(t, db, ticketID)
		base := "/api/tickets/" + tn
		pngBytes := testPNG(t, 640, 480)

		up := api.mustUpload(tn, "screenshot.png", "image/png", pngBytes)
		assert.Equal(t, articleID, up.ArticleID, "attached to the ticket's latest article")
		assert.Equal(t, "screenshot.png", up.Filename)
		assert.Equal(t, int64(len(pngBytes)), up.Size)
		assert.Equal(t, "image/png", up.ContentType)
		assert.Equal(t, fmt.Sprintf("%s/articles/%d/attachments/%d", base, articleID, up.FileID), up.DownloadURL)
		assert.Equal(t, up.DownloadURL+"/thumbnail", up.ThumbnailURL)

		// Browser sent application/octet-stream: the type is detected.
		upPDF := api.mustUpload(tn, "diagram.pdf", "application/octet-stream", []byte(testPDF))
		assert.Equal(t, articleID, upPDF.ArticleID)
		assert.Equal(t, "application/pdf", upPDF.ContentType)
		assert.Equal(t, upPDF.DownloadURL+"/thumbnail", upPDF.ThumbnailURL)

		if articleDir != "" {
			dir := articleStorageDir(t, db, articleDir, articleID)
			onDisk, err := os.ReadFile(filepath.Join(dir, "screenshot.png"))
			require.NoError(t, err, "FS stores the file under <article dir>/<date>/<article id>/<name>")
			assert.Equal(t, pngBytes, onDisk)
			ct, err := os.ReadFile(filepath.Join(dir, "screenshot.png.content_type"))
			require.NoError(t, err)
			assert.Equal(t, "image/png", string(ct))
		} else {
			var stored []byte
			var createBy int
			require.NoError(t, db.QueryRow(database.ConvertPlaceholders(`
				SELECT content, create_by FROM article_data_mime_attachment
				WHERE article_id = ? AND filename = ?`), articleID, "screenshot.png").Scan(&stored, &createBy))
			assert.Equal(t, pngBytes, stored)
			assert.Equal(t, 1, createBy, "create_by is the uploading agent")
		}

		// List (JSON). FS file ids are positions in the sorted name list, so
		// the list (not the upload responses) is the source of current URLs.
		listed := api.list(tn)
		require.Equal(t, 2, listed.Total)
		require.Len(t, listed.Attachments, 2)
		shot := findListItem(t, listed.Attachments, "screenshot.png")
		pdf := findListItem(t, listed.Attachments, "diagram.pdf")
		shotURL := fmt.Sprintf("%s/articles/%d/attachments/%d", base, articleID, shot.FileID)
		assert.Equal(t, articleID, shot.ArticleID)
		assert.Equal(t, shotURL, shot.DownloadURL)
		assert.Equal(t, shotURL+"/view", shot.ViewURL)
		assert.Equal(t, shotURL, shot.DeleteURL)
		assert.Equal(t, shotURL+"/thumbnail", shot.ThumbnailURL)
		assert.Equal(t, int64(len(pngBytes)), shot.Size)
		assert.Equal(t, "image/png", shot.ContentType)
		assert.Equal(t, formatFileSize(int64(len(pngBytes))), shot.SizeFormatted)
		assert.False(t, shot.UploadedAt.IsZero())
		assert.Equal(t, "application/pdf", pdf.ContentType)

		// List (HTMX partial) links the same article-scoped URLs.
		w := api.do(http.MethodGet, base+"/attachments", nil, "HX-Request", "true")
		require.Equal(t, http.StatusOK, w.Code)
		assert.Contains(t, w.Header().Get("Content-Type"), "text/html")
		for _, it := range listed.Attachments {
			assert.Contains(t, w.Body.String(), `href="`+it.ViewURL+`"`)
			assert.Contains(t, w.Body.String(), `href="`+it.DownloadURL+`"`)
		}

		// Download: byte-exact, inline for safe types, safe disposition.
		w = api.do(http.MethodGet, shot.DownloadURL, nil)
		require.Equal(t, http.StatusOK, w.Code)
		assert.Equal(t, pngBytes, w.Body.Bytes())
		assert.Equal(t, "image/png", w.Header().Get("Content-Type"))
		assert.Equal(t, "nosniff", w.Header().Get("X-Content-Type-Options"))
		disp, params, err := mime.ParseMediaType(w.Header().Get("Content-Disposition"))
		require.NoError(t, err)
		assert.Equal(t, "inline", disp)
		assert.Equal(t, "screenshot.png", params["filename"])

		w = api.do(http.MethodGet, pdf.DownloadURL, nil)
		require.Equal(t, http.StatusOK, w.Code)
		assert.Equal(t, testPDF, w.Body.String())
		assert.Equal(t, "application/pdf", w.Header().Get("Content-Type"))

		// Thumbnail: a scaled PNG, revalidated by ETag.
		w = api.do(http.MethodGet, shot.ThumbnailURL, nil)
		require.Equal(t, http.StatusOK, w.Code)
		assert.Equal(t, "image/png", w.Header().Get("Content-Type"))
		cfg, err := png.DecodeConfig(bytes.NewReader(w.Body.Bytes()))
		require.NoError(t, err)
		assert.LessOrEqual(t, cfg.Width, thumbnailMaxW)
		assert.LessOrEqual(t, cfg.Height, thumbnailMaxH)
		etag := w.Header().Get("ETag")
		require.NotEmpty(t, etag)
		w = api.do(http.MethodGet, shot.ThumbnailURL, nil, "If-None-Match", etag)
		assert.Equal(t, http.StatusNotModified, w.Code)
		assert.Empty(t, w.Body.Bytes())

		// Viewer page: navigation walks the ticket's list order.
		order := []attachmentListItem{listed.Attachments[0], listed.Attachments[1]}
		w = api.do(http.MethodGet, order[0].ViewURL, nil)
		require.Equal(t, http.StatusOK, w.Code)
		assert.Contains(t, w.Header().Get("Content-Type"), "text/html")
		assert.Contains(t, w.Body.String(), `class="nav right"><a class="nav-btn" href="`+order[1].ViewURL+`"`)
		assert.NotContains(t, w.Body.String(), `class="nav left"`)
		assert.Contains(t, w.Body.String(), `src="`+order[0].DownloadURL+`/view?raw=1"`)
		w = api.do(http.MethodGet, order[1].ViewURL, nil)
		require.Equal(t, http.StatusOK, w.Code)
		assert.Contains(t, w.Body.String(), `class="nav left"><a class="nav-btn" href="`+order[0].ViewURL+`"`)
		assert.NotContains(t, w.Body.String(), `class="nav right"`)

		// Raw view: the content, framable by the same origin only.
		w = api.do(http.MethodGet, shot.ViewURL+"?raw=1", nil)
		require.Equal(t, http.StatusOK, w.Code)
		assert.Equal(t, pngBytes, w.Body.Bytes())
		assert.Equal(t, "image/png", w.Header().Get("Content-Type"))
		assert.Equal(t, "SAMEORIGIN", w.Header().Get("X-Frame-Options"))
		assert.Contains(t, w.Header().Get("Content-Security-Policy"), "frame-ancestors 'self'")

		// Another ticket's attachment is not reachable through this ticket.
		otherTicketID, otherArticleID := createAttachmentTestArticle(t, db, "Other", "body")
		otherTN := ticketTN(t, db, otherTicketID)
		other := api.mustUpload(otherTN, "secret.txt", "text/plain", []byte("other ticket"))
		require.Equal(t, otherArticleID, other.ArticleID)
		foreign := fmt.Sprintf("%s/articles/%d/attachments/%d", base, other.ArticleID, other.FileID)
		for _, path := range []string{foreign, foreign + "/thumbnail", foreign + "/view", foreign + "/view?raw=1"} {
			assert.Equal(t, http.StatusNotFound, api.do(http.MethodGet, path, nil).Code, path)
		}
		assert.Equal(t, http.StatusNotFound, api.do(http.MethodDelete, foreign, nil).Code)
		w = api.do(http.MethodGet, other.DownloadURL, nil)
		require.Equal(t, http.StatusOK, w.Code, "foreign delete must not remove it")
		assert.Equal(t, "other ticket", w.Body.String())
		missing := fmt.Sprintf("%s/articles/%d/attachments/%d", base, articleID, 999999)
		assert.Equal(t, http.StatusNotFound, api.do(http.MethodGet, missing, nil).Code)
		assert.Equal(t, http.StatusBadRequest, api.do(http.MethodGet, base+"/articles/x/attachments/1", nil).Code)

		// Delete. (FS: screenshot.png sorts after diagram.pdf, so the PDF
		// keeps its index and the deleted index stays unused.)
		assert.Equal(t, http.StatusUnauthorized, httptestDelete(api.router, shot.DeleteURL).Code)
		w = api.do(http.MethodDelete, shot.DeleteURL, nil)
		require.Equal(t, http.StatusOK, w.Code, w.Body.String())
		var del struct {
			Success bool `json:"success"`
		}
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &del))
		assert.True(t, del.Success)
		assert.Equal(t, http.StatusNotFound, api.do(http.MethodGet, shot.DownloadURL, nil).Code)
		assert.Equal(t, http.StatusNotFound, api.do(http.MethodDelete, shot.DeleteURL, nil).Code)
		after := api.list(tn)
		require.Equal(t, 1, after.Total)
		assert.Equal(t, "diagram.pdf", after.Attachments[0].Filename)
		if articleDir != "" {
			dir := articleStorageDir(t, db, articleDir, articleID)
			for _, name := range []string{"screenshot.png", "screenshot.png.content_type", "screenshot.png.disposition"} {
				_, err := os.Stat(filepath.Join(dir, name))
				assert.ErrorIs(t, err, os.ErrNotExist, name)
			}
			_, err := os.Stat(filepath.Join(dir, "diagram.pdf"))
			assert.NoError(t, err)
		} else {
			var n int
			require.NoError(t, db.QueryRow(database.ConvertPlaceholders(
				"SELECT COUNT(*) FROM article_data_mime_attachment WHERE article_id = ?"), articleID).Scan(&n))
			assert.Equal(t, 1, n)
		}
	})
}

// httptestDelete sends an unauthenticated DELETE.
func httptestDelete(router *gin.Engine, path string) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest(http.MethodDelete, path, nil))
	return w
}

// TestAgentAttachmentHostileFilename: a filename with quotes and markup must
// stay inert in the HTMX list (HTML and the onclick JS) and in the
// Content-Disposition header.
func TestAgentAttachmentHostileFilename(t *testing.T) {
	db := attachmentTestDB(t)
	api := newAttachmentAPI(t)
	const name = `q'uote"<img src=x onerror=alert(1)>.png`

	eachArticleStorage(t, func(t *testing.T, _ string) {
		ticketID, _ := createAttachmentTestArticle(t, db, "Hostile", "body")
		tn := ticketTN(t, db, ticketID)
		pngBytes := testPNG(t, 4, 4)
		up := api.mustUpload(tn, name, "image/png", pngBytes)
		require.Equal(t, name, up.Filename)

		w := api.do(http.MethodGet, "/api/tickets/"+tn+"/attachments", nil, "HX-Request", "true")
		require.Equal(t, http.StatusOK, w.Code)
		body := w.Body.String()
		assert.NotContains(t, body, "<img src=x")
		assert.NotContains(t, body, "onerror=alert(1)>")

		doc, err := html.Parse(strings.NewReader(body))
		require.NoError(t, err)
		var linkText, onclick string
		var walk func(n *html.Node)
		walk = func(n *html.Node) {
			if n.Type == html.ElementNode {
				for _, a := range n.Attr {
					if n.Data == "a" && a.Key == "href" && a.Val == up.DownloadURL+"/view" && n.FirstChild != nil && linkText == "" {
						linkText = strings.TrimSpace(n.FirstChild.Data)
					}
					if n.Data == "button" && a.Key == "onclick" {
						onclick = a.Val
					}
				}
				if n.Data == "img" {
					for _, a := range n.Attr {
						assert.NotEqual(t, "onerror", a.Key, "filename markup became an element")
					}
				}
			}
			for c := n.FirstChild; c != nil; c = c.NextSibling {
				walk(c)
			}
		}
		walk(doc)
		assert.Equal(t, name, linkText, "the link shows the filename as text")

		// onclick is deleteAttachment(<url>, <filename>) with JS string literals.
		require.True(t, strings.HasPrefix(onclick, "deleteAttachment(") && strings.HasSuffix(onclick, ")"), onclick)
		var args []string
		require.NoError(t, json.Unmarshal([]byte("["+strings.TrimSuffix(strings.TrimPrefix(onclick, "deleteAttachment("), ")")+"]"), &args), onclick)
		assert.Equal(t, []string{up.DownloadURL, name}, args)

		w = api.do(http.MethodGet, up.DownloadURL, nil)
		require.Equal(t, http.StatusOK, w.Code)
		cd := w.Header().Get("Content-Disposition")
		disp, params, err := mime.ParseMediaType(cd)
		require.NoError(t, err, cd)
		assert.Equal(t, "inline", disp)
		assert.Equal(t, name, params["filename"])

		w = api.do(http.MethodGet, up.DownloadURL+"/view", nil)
		require.Equal(t, http.StatusOK, w.Code)
		assert.NotContains(t, w.Body.String(), "<img src=x")
	})
}

// TestAgentAttachmentUploadCreatesArticle: a ticket without articles gets an
// agent note (internal channel, customer visible) carrying the upload.
func TestAgentAttachmentUploadCreatesArticle(t *testing.T) {
	db := attachmentTestDB(t)
	api := newAttachmentAPI(t)

	eachArticleStorage(t, func(t *testing.T, articleDir string) {
		ticketID, tn := createAttachmentTestTicket(t, db)
		up := api.mustUpload(tn, "notes.txt", "text/plain", []byte("hello"))

		var articleTicket, sender, channel, visible, createBy int
		require.NoError(t, db.QueryRow(database.ConvertPlaceholders(`
			SELECT ticket_id, article_sender_type_id, communication_channel_id, is_visible_for_customer, create_by
			FROM article WHERE id = ?`), up.ArticleID).Scan(&articleTicket, &sender, &channel, &visible, &createBy))
		assert.Equal(t, int(ticketID), articleTicket)
		assert.Equal(t, constants.ArticleSenderAgent, sender)
		assert.Equal(t, constants.CommunicationChannelInternal, channel)
		assert.Equal(t, 1, visible)
		assert.Equal(t, 1, createBy)

		w := api.do(http.MethodGet, up.DownloadURL, nil)
		require.Equal(t, http.StatusOK, w.Code)
		assert.Equal(t, "hello", w.Body.String())
		disp, _, err := mime.ParseMediaType(w.Header().Get("Content-Disposition"))
		require.NoError(t, err)
		assert.Equal(t, "attachment", disp, "text is never served inline")

		if articleDir != "" {
			onDisk, err := os.ReadFile(filepath.Join(articleStorageDir(t, db, articleDir, up.ArticleID), "notes.txt"))
			require.NoError(t, err)
			assert.Equal(t, "hello", string(onDisk))
		}

		// The second upload goes to the same (now latest) article.
		second := api.mustUpload(tn, "more.txt", "text/plain", []byte("again"))
		assert.Equal(t, up.ArticleID, second.ArticleID)
	})
}

// TestAgentAttachmentViewerICS: calendar attachments render as event cards
// from the stored content, on both backends.
func TestAgentAttachmentViewerICS(t *testing.T) {
	db := attachmentTestDB(t)
	api := newAttachmentAPI(t)
	ics := "BEGIN:VCALENDAR\r\nVERSION:2.0\r\nBEGIN:VEVENT\r\nUID:1@test\r\n" +
		"DTSTART:20260301T100000Z\r\nDTEND:20260301T110000Z\r\nSUMMARY:Quarterly <Review>\r\n" +
		"END:VEVENT\r\nEND:VCALENDAR\r\n"

	eachArticleStorage(t, func(t *testing.T, _ string) {
		ticketID, _ := createAttachmentTestArticle(t, db, "Invite", "body")
		tn := ticketTN(t, db, ticketID)
		up := api.mustUpload(tn, "invite.ics", "application/octet-stream", []byte(ics))
		assert.Equal(t, "text/calendar", up.ContentType, "detected from the extension")
		// Browsers send text/calendar for .ics files; it must be accepted.
		assert.Equal(t, "text/calendar", api.mustUpload(tn, "second.ics", "text/calendar", []byte(ics)).ContentType)

		w := api.do(http.MethodGet, up.DownloadURL+"/view", nil)
		require.Equal(t, http.StatusOK, w.Code)
		assert.Contains(t, w.Body.String(), "Quarterly &lt;Review&gt;")

		w = api.do(http.MethodGet, up.DownloadURL+"/view?raw=1", nil)
		require.Equal(t, http.StatusOK, w.Code)
		assert.Equal(t, "text/plain; charset=utf-8", w.Header().Get("Content-Type"))
		assert.Equal(t, ics, w.Body.String())
	})
}

// TestAgentAttachmentUploadValidation: rejected uploads store nothing.
func TestAgentAttachmentUploadValidation(t *testing.T) {
	db := attachmentTestDB(t)
	api := newAttachmentAPI(t)
	ticketID, articleID := createAttachmentTestArticle(t, db, "Validation", "body")
	tn := ticketTN(t, db, ticketID)

	w := api.upload(tn, "malware.exe", "application/octet-stream", []byte("MZ"))
	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "file type not allowed")

	w = api.upload(tn, "huge.bin", "application/octet-stream", bytes.Repeat([]byte("x"), MaxFileSize+1))
	assert.Equal(t, http.StatusRequestEntityTooLarge, w.Code)

	w = api.upload("NO-SUCH-TICKET", "a.txt", "text/plain", []byte("x"))
	assert.Equal(t, http.StatusNotFound, w.Code)

	var n int
	require.NoError(t, db.QueryRow(database.ConvertPlaceholders(
		"SELECT COUNT(*) FROM article_data_mime_attachment WHERE article_id = ?"), articleID).Scan(&n))
	assert.Equal(t, 0, n)
}

func TestAttachmentValidation(t *testing.T) {
	// validateFile checks filename and MIME type, not size
	// Size is checked in the upload handler
	tests := []struct {
		name        string
		filename    string
		contentType string
		wantAllowed bool
		wantError   string
	}{
		{
			name:        "Valid document",
			filename:    "report.pdf",
			contentType: "application/pdf",
			wantAllowed: true,
		},
		{
			name:        "Valid image",
			filename:    "photo.jpg",
			contentType: "image/jpeg",
			wantAllowed: true,
		},
		{
			name:        "Executable blocked",
			filename:    "virus.exe",
			contentType: "application/x-msdownload",
			wantAllowed: false,
			wantError:   "file type not allowed",
		},
		{
			name:        "Script blocked",
			filename:    "hack.bat",
			contentType: "application/x-msdos-program",
			wantAllowed: false,
			wantError:   "file type not allowed",
		},
		{
			name:        "Hidden file blocked",
			filename:    ".htaccess",
			contentType: "text/plain",
			wantAllowed: false,
			wantError:   "not allowed",
		},
		{
			name:        "PowerShell file blocked by extension",
			filename:    "script.ps1",
			contentType: "text/plain",
			wantAllowed: false,
			wantError:   "not allowed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			header := &multipart.FileHeader{
				Filename: tt.filename,
				Header:   make(map[string][]string),
				Size:     1024, // Size is not checked by validateFile
			}
			header.Header.Set("Content-Type", tt.contentType)

			err := validateFile(header)

			if tt.wantAllowed {
				assert.NoError(t, err)
			} else {
				assert.Error(t, err)
				if tt.wantError != "" {
					assert.Contains(t, err.Error(), tt.wantError)
				}
			}
		})
	}
}

func TestAttachmentSecurity(t *testing.T) {
	tests := []struct {
		name        string
		filename    string
		content     []byte
		wantBlocked bool
		reason      string
	}{
		{
			name:        "ZIP file allowed",
			filename:    "archive.zip",
			content:     []byte{0x50, 0x4B, 0x03, 0x04}, // ZIP signature
			wantBlocked: false,
			reason:      "",
		},
		{
			name:        "PowerShell file blocked",
			filename:    "script.ps1",
			content:     []byte("Write-Host 'test'"),
			wantBlocked: true,
			reason:      "file type not allowed",
		},
		{
			name:        "Clean file passes",
			filename:    "document.txt",
			content:     []byte("This is a clean text file"),
			wantBlocked: false,
			reason:      "",
		},
		{
			name:        "Null byte injection blocked",
			filename:    "shell.php\x00.jpg",
			content:     []byte("<?php system($_GET['cmd']); ?>"),
			wantBlocked: true,
			reason:      "illegal characters",
		},
		{
			name:        "Null byte in middle blocked",
			filename:    "test\x00.exe.txt",
			content:     []byte("malicious"),
			wantBlocked: true,
			reason:      "illegal characters",
		},
		{
			name:        "Path traversal sanitised to base",
			filename:    "../../etc/passwd",
			content:     []byte("root:x:0:0"),
			wantBlocked: false, // filepath.Base strips to "passwd" which is a valid filename
			reason:      "",
		},
		{
			name:        "Hidden file after traversal blocked",
			filename:    "../.hidden",
			content:     []byte("secret"),
			wantBlocked: true,
			reason:      "hidden files",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			header := &multipart.FileHeader{
				Filename: tt.filename,
				Header:   make(map[string][]string),
				Size:     int64(len(tt.content)),
			}
			header.Header.Set("Content-Type", "application/octet-stream")

			err := validateFile(header)

			if tt.wantBlocked {
				assert.Error(t, err)
				if tt.reason != "" {
					assert.Contains(t, err.Error(), tt.reason)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
