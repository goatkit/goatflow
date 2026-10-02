package api

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"html"
	"image"
	"image/color"
	"image/png"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/goatkit/goatflow/internal/platform/database"
	"github.com/goatkit/goatflow/internal/platform/middleware"
	"github.com/goatkit/goatflow/internal/platform/routing"
	"github.com/goatkit/goatflow/internal/storage"
)

// custAttFixture is one customer with an own ticket and a foreign ticket.
type custAttFixture struct {
	db          *sql.DB
	router      *gin.Engine
	token       string
	ticketID    int
	ticketTN    string
	foreignID   int
	internalArt int64
	internalAtt storage.Attachment
	foreignArt  int64
	foreignAtt  storage.Attachment
}

// newCustAttRouter is the production stack for /customer: global security
// headers plus the YAML routes with the customer-portal middleware.
func newCustAttRouter(t *testing.T) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(middleware.SecurityHeaders())
	require.NoError(t, routing.LoadYAMLRoutesForTesting(r))
	return r
}

func custAttInsertCustomer(t *testing.T, db *sql.DB, login string) int64 {
	t.Helper()
	id, err := database.GetAdapter().InsertWithReturning(db, database.ConvertPlaceholders(`
		INSERT INTO customer_user (login, email, customer_id, first_name, last_name, valid_id,
			create_time, create_by, change_time, change_by)
		VALUES (?, ?, 'custatt-co', 'Cust', 'Att', 1, NOW(), 1, NOW(), 1) RETURNING id`), login, login)
	require.NoError(t, err)
	t.Cleanup(func() {
		_, _ = db.Exec(database.ConvertPlaceholders(`DELETE FROM customer_user WHERE id = ?`), id)
	})
	return id
}

func custAttInsertTicket(t *testing.T, db *sql.DB, tn, customerLogin string) int {
	t.Helper()
	id, err := database.GetAdapter().InsertWithReturning(db, database.ConvertPlaceholders(`
		INSERT INTO ticket (tn, title, queue_id, ticket_lock_id, type_id, user_id, responsible_user_id,
			ticket_priority_id, ticket_state_id, customer_id, customer_user_id, timeout, until_time,
			escalation_time, escalation_update_time, escalation_response_time, escalation_solution_time,
			archive_flag, create_time, create_by, change_time, change_by)
		VALUES (?, 'Customer attachment test', 1, 1, 1, 1, 1, 3, 1, 'custatt-co', ?, 0, 0, 0, 0, 0, 0, 0,
			NOW(), 1, NOW(), 1) RETURNING id`), tn, customerLogin)
	require.NoError(t, err)
	t.Cleanup(func() {
		ctx := context.Background()
		rows, err := db.QueryContext(ctx, database.ConvertPlaceholders(`SELECT id FROM article WHERE ticket_id = ?`), id)
		if err == nil {
			var ids []int64
			for rows.Next() {
				var a int64
				if rows.Scan(&a) == nil {
					ids = append(ids, a)
				}
			}
			rows.Close()
			for _, a := range ids {
				_ = storage.ForDB(db).DeleteArticle(ctx, a)
				_, _ = db.Exec(database.ConvertPlaceholders(`DELETE FROM article_data_mime WHERE article_id = ?`), a)
			}
		}
		_, _ = db.Exec(database.ConvertPlaceholders(`DELETE FROM article WHERE ticket_id = ?`), id)
		_, _ = db.Exec(database.ConvertPlaceholders(`DELETE FROM ticket WHERE id = ?`), id)
	})
	return int(id)
}

// custAttInsertArticle creates an article (agent sender) with one attachment.
func custAttInsertArticle(
	t *testing.T, db *sql.DB, ticketID int, visible bool, filename string, content []byte,
) (int64, storage.Attachment) {
	t.Helper()
	vis := 0
	if visible {
		vis = 1
	}
	now := time.Now()
	articleID, err := database.GetAdapter().InsertWithReturning(db, database.ConvertPlaceholders(`
		INSERT INTO article (ticket_id, article_sender_type_id, communication_channel_id, is_visible_for_customer,
			search_index_needs_rebuild, create_time, create_by, change_time, change_by)
		VALUES (?, 1, 3, ?, 1, ?, 1, ?, 1) RETURNING id`), ticketID, vis, now, now)
	require.NoError(t, err)
	_, err = db.Exec(database.ConvertPlaceholders(`
		INSERT INTO article_data_mime (article_id, a_from, a_subject, a_body, a_content_type, content_path,
			incoming_time, create_time, create_by, change_time, change_by)
		VALUES (?, 'agent@example.com', 'note', 'body', 'text/plain', ?, ?, ?, 1, ?, 1)`),
		articleID, storage.ContentPath(now), now.Unix(), now, now)
	require.NoError(t, err)
	att, err := storage.ForDB(db).WriteAttachment(context.Background(), articleID, storage.NewAttachment{
		Filename: filename, ContentType: "text/plain", Disposition: "attachment", Content: content, CreateBy: 1,
	})
	require.NoError(t, err)
	return articleID, att
}

func newCustAttFixture(t *testing.T, router *gin.Engine) custAttFixture {
	t.Helper()
	db, err := database.GetDB()
	require.NoError(t, err)
	require.NotNil(t, db)

	suffix := strconv.FormatInt(time.Now().UnixNano(), 36)
	login := "custatt-" + suffix + "@example.com"
	other := "custatt-other-" + suffix + "@example.com"
	custID := custAttInsertCustomer(t, db, login)
	custAttInsertCustomer(t, db, other)

	f := custAttFixture{db: db, router: router}
	f.ticketTN = "CA" + suffix
	f.ticketID = custAttInsertTicket(t, db, f.ticketTN, login)
	f.foreignID = custAttInsertTicket(t, db, "CB"+suffix, other)
	f.internalArt, f.internalAtt = custAttInsertArticle(t, db, f.ticketID, false, "internal-secret.txt", []byte("agents only"))
	f.foreignArt, f.foreignAtt = custAttInsertArticle(t, db, f.foreignID, true, "foreign.txt", []byte("other customer"))

	f.token = testSessionToken(t, uint(custID), login, login, "Customer", false, 0)
	return f
}

func (f custAttFixture) do(t *testing.T, method, url string, body io.Reader, headers ...string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, url, body)
	req.Header.Set("Authorization", "Bearer "+f.token)
	req.Header.Set("Accept", "application/json")
	for i := 0; i+1 < len(headers); i += 2 {
		req.Header.Set(headers[i], headers[i+1])
	}
	w := httptest.NewRecorder()
	f.router.ServeHTTP(w, req)
	return w
}

type custAttUpload struct {
	name, contentType string
	content           []byte
}

func (f custAttFixture) upload(t *testing.T, ticketRef string, files ...custAttUpload) *httptest.ResponseRecorder {
	t.Helper()
	var body bytes.Buffer
	mw := multipart.NewWriter(&body)
	for _, file := range files {
		h := make(map[string][]string)
		h["Content-Disposition"] = []string{fmt.Sprintf(`form-data; name="attachments"; filename="%s"`,
			strings.NewReplacer(`\`, `\\`, `"`, `\"`).Replace(file.name))}
		h["Content-Type"] = []string{file.contentType}
		part, err := mw.CreatePart(h)
		require.NoError(t, err)
		_, err = part.Write(file.content)
		require.NoError(t, err)
	}
	require.NoError(t, mw.Close())
	return f.do(t, http.MethodPost, "/customer/tickets/"+ticketRef+"/attachments", &body,
		"Content-Type", mw.FormDataContentType())
}

func (f custAttFixture) list(t *testing.T, ticketRef string) []customerAttachment {
	t.Helper()
	w := f.do(t, http.MethodGet, "/customer/tickets/"+ticketRef+"/attachments", nil)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	var resp struct {
		Attachments []customerAttachment `json:"attachments"`
		Total       int                  `json:"total"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	require.Len(t, resp.Attachments, resp.Total)
	return resp.Attachments
}

func custAttPNG(t *testing.T, w, h int) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for x := range w {
		for y := range h {
			img.Set(x, y, color.RGBA{R: 30, G: 120, B: 200, A: 255})
		}
	}
	var buf bytes.Buffer
	require.NoError(t, png.Encode(&buf, img))
	return buf.Bytes()
}

// custAttUseBackend switches the article storage backend for one test.
func custAttUseBackend(t *testing.T, backend string) string {
	t.Helper()
	dir := ""
	if backend == storage.BackendFS {
		dir = t.TempDir()
	}
	require.NoError(t, storage.Configure(storage.Config{Backend: backend, ArticleDir: dir}))
	t.Cleanup(func() { _ = storage.Configure(storage.Config{Backend: storage.BackendDB}) })
	return dir
}

func TestCustomerAttachmentRoutes(t *testing.T) {
	router := newCustAttRouter(t)
	for _, backend := range []string{storage.BackendDB, storage.BackendFS} {
		t.Run(backend, func(t *testing.T) {
			dir := custAttUseBackend(t, backend)
			f := newCustAttFixture(t, router)
			base := fmt.Sprintf("/customer/tickets/%d", f.ticketID)
			pngBytes := custAttPNG(t, 640, 480)
			textBytes := []byte("plain text attachment\nline two\n")

			// The ticket only has an internal article: the upload creates a
			// customer-visible one.
			w := f.upload(t, f.ticketTN,
				custAttUpload{"screenshot.png", "image/png", pngBytes},
				custAttUpload{"notes.txt", "text/plain", textBytes})
			require.Equal(t, http.StatusOK, w.Code, w.Body.String())
			assert.Equal(t, "attachments-updated", w.Header().Get("HX-Trigger"))

			var articleID int64
			var visible int
			var contentPath sql.NullString
			require.NoError(t, f.db.QueryRow(database.ConvertPlaceholders(`
				SELECT a.id, a.is_visible_for_customer, adm.content_path
				FROM article a JOIN article_data_mime adm ON adm.article_id = a.id
				WHERE a.ticket_id = ? AND a.id <> ?`), f.ticketID, f.internalArt).
				Scan(&articleID, &visible, &contentPath))
			assert.Equal(t, 1, visible)
			require.True(t, contentPath.Valid)

			atts := f.list(t, strconv.Itoa(f.ticketID))
			require.Len(t, atts, 2, "internal article attachment must not be listed")
			byName := map[string]customerAttachment{}
			for _, a := range atts {
				byName[a.Filename] = a
				assert.Equal(t, articleID, a.ArticleID)
				url := fmt.Sprintf("%s/articles/%d/attachments/%d", base, a.ArticleID, a.FileID)
				assert.Equal(t, url, a.DownloadURL)
				assert.Equal(t, url+"/view", a.ViewURL)
				assert.Equal(t, url+"/thumbnail", a.ThumbnailURL)
			}
			img, txt := byName["screenshot.png"], byName["notes.txt"]
			require.NotZero(t, img.FileID)
			require.NotZero(t, txt.FileID)
			assert.Equal(t, int64(len(pngBytes)), img.Size)
			assert.Equal(t, "image/png", img.ContentType)
			assert.Equal(t, int64(len(textBytes)), txt.Size)

			// The list also resolves the ticket by its number.
			assert.Len(t, f.list(t, f.ticketTN), 2)

			if backend == storage.BackendFS {
				artDir := filepath.Join(dir, filepath.FromSlash(contentPath.String), strconv.FormatInt(articleID, 10))
				onDisk, err := os.ReadFile(filepath.Join(artDir, "screenshot.png"))
				require.NoError(t, err)
				assert.Equal(t, pngBytes, onDisk)
				ct, err := os.ReadFile(filepath.Join(artDir, "screenshot.png.content_type"))
				require.NoError(t, err)
				assert.Equal(t, "image/png", strings.TrimSpace(string(ct)))
				var dbRows int
				require.NoError(t, f.db.QueryRow(database.ConvertPlaceholders(
					`SELECT COUNT(*) FROM article_data_mime_attachment WHERE article_id = ?`), articleID).Scan(&dbRows))
				assert.Zero(t, dbRows, "FS backend must not store attachments in the database")
			}

			// Download: byte-exact, images inline, ?download=1 forces a download.
			w = f.do(t, http.MethodGet, img.DownloadURL, nil)
			require.Equal(t, http.StatusOK, w.Code, w.Body.String())
			assert.Equal(t, pngBytes, w.Body.Bytes())
			assert.Equal(t, "image/png", w.Header().Get("Content-Type"))
			assert.Equal(t, "nosniff", w.Header().Get("X-Content-Type-Options"))
			disp, params, err := mime.ParseMediaType(w.Header().Get("Content-Disposition"))
			require.NoError(t, err)
			assert.Equal(t, "inline", disp)
			assert.Equal(t, "screenshot.png", params["filename"])

			w = f.do(t, http.MethodGet, img.DownloadURL+"?download=1", nil)
			require.Equal(t, http.StatusOK, w.Code)
			disp, _, err = mime.ParseMediaType(w.Header().Get("Content-Disposition"))
			require.NoError(t, err)
			assert.Equal(t, "attachment", disp)

			w = f.do(t, http.MethodGet, txt.DownloadURL, nil)
			require.Equal(t, http.StatusOK, w.Code)
			assert.Equal(t, textBytes, w.Body.Bytes())
			disp, _, err = mime.ParseMediaType(w.Header().Get("Content-Disposition"))
			require.NoError(t, err)
			assert.Equal(t, "attachment", disp, "text is never rendered inline on our origin")

			// Thumbnail: a scaled-down PNG.
			w = f.do(t, http.MethodGet, img.ThumbnailURL, nil)
			require.Equal(t, http.StatusOK, w.Code, w.Body.String())
			assert.Equal(t, "image/png", w.Header().Get("Content-Type"))
			thumb, err := png.DecodeConfig(bytes.NewReader(w.Body.Bytes()))
			require.NoError(t, err)
			assert.LessOrEqual(t, thumb.Width, thumbnailMaxW)
			assert.LessOrEqual(t, thumb.Height, thumbnailMaxH)
			etag := w.Header().Get("ETag")
			require.NotEmpty(t, etag)
			w = f.do(t, http.MethodGet, img.ThumbnailURL, nil, "If-None-Match", etag)
			assert.Equal(t, http.StatusNotModified, w.Code)

			// Viewer page embeds the framable raw variant.
			w = f.do(t, http.MethodGet, img.ViewURL, nil)
			require.Equal(t, http.StatusOK, w.Code)
			assert.Contains(t, w.Header().Get("Content-Type"), "text/html")
			assert.Contains(t, w.Body.String(), `<img src="`+img.ViewURL+`?raw=1"`)
			assert.Contains(t, w.Body.String(), `href="`+img.DownloadURL+`?download=1"`)

			// Every attachment route of an internal article is a 404, even on
			// the customer's own ticket.
			internalURL := fmt.Sprintf("%s/articles/%d/attachments/%d", base, f.internalArt, f.internalAtt.FileID)
			for _, u := range []string{internalURL, internalURL + "/thumbnail", internalURL + "/view", internalURL + "/view?raw=1"} {
				w = f.do(t, http.MethodGet, u, nil)
				assert.Equal(t, http.StatusNotFound, w.Code, u)
				assert.NotContains(t, w.Body.String(), "agents only", u)
			}

			// Another customer's ticket: refused by the ownership check.
			foreignURL := fmt.Sprintf("/customer/tickets/%d/articles/%d/attachments/%d", f.foreignID, f.foreignArt, f.foreignAtt.FileID)
			for _, u := range []string{
				fmt.Sprintf("/customer/tickets/%d/attachments", f.foreignID),
				foreignURL, foreignURL + "/thumbnail", foreignURL + "/view",
			} {
				w = f.do(t, http.MethodGet, u, nil)
				assert.Equal(t, http.StatusForbidden, w.Code, u)
				assert.NotContains(t, w.Body.String(), "other customer", u)
			}
			w = f.upload(t, strconv.Itoa(f.foreignID), custAttUpload{"x.txt", "text/plain", []byte("x")})
			assert.Equal(t, http.StatusForbidden, w.Code)

			// The other customer's article addressed through the own ticket.
			crossURL := fmt.Sprintf("%s/articles/%d/attachments/%d", base, f.foreignArt, f.foreignAtt.FileID)
			w = f.do(t, http.MethodGet, crossURL, nil)
			assert.Equal(t, http.StatusNotFound, w.Code)
			assert.NotContains(t, w.Body.String(), "other customer")

			// Unknown ticket number and unknown file.
			w = f.do(t, http.MethodGet, "/customer/tickets/NO-SUCH-TN/attachments", nil)
			assert.Equal(t, http.StatusNotFound, w.Code)
			w = f.do(t, http.MethodGet, fmt.Sprintf("%s/articles/%d/attachments/%d", base, articleID, 999999), nil)
			assert.Equal(t, http.StatusNotFound, w.Code)
			w = f.do(t, http.MethodGet, fmt.Sprintf("%s/articles/%d/attachments/abc", base, articleID), nil)
			assert.Equal(t, http.StatusBadRequest, w.Code)

			// A second upload goes to the now existing customer article.
			w = f.upload(t, f.ticketTN, custAttUpload{"notes.txt", "text/plain", []byte("second")})
			require.Equal(t, http.StatusOK, w.Code, w.Body.String())
			var articles int
			require.NoError(t, f.db.QueryRow(database.ConvertPlaceholders(
				`SELECT COUNT(*) FROM article WHERE ticket_id = ?`), f.ticketID).Scan(&articles))
			assert.Equal(t, 2, articles)
			atts = f.list(t, f.ticketTN)
			require.Len(t, atts, 3)
			var second *customerAttachment
			for i := range atts {
				if atts[i].Filename == "notes-1.txt" {
					second = &atts[i]
				}
			}
			require.NotNil(t, second, "same name in one article is deduplicated OTRS-style")
			assert.Equal(t, articleID, second.ArticleID)
			w = f.do(t, http.MethodGet, second.DownloadURL, nil)
			require.Equal(t, http.StatusOK, w.Code)
			assert.Equal(t, "second", w.Body.String())

			// Unauthenticated requests never reach the handlers.
			req := httptest.NewRequest(http.MethodGet, img.DownloadURL, nil)
			req.Header.Set("Accept", "application/json")
			rec := httptest.NewRecorder()
			router.ServeHTTP(rec, req)
			assert.NotEqual(t, http.StatusOK, rec.Code)
			assert.NotEqual(t, pngBytes, rec.Body.Bytes())
		})
	}
}

// TestCustomerAttachmentViewerFraming: the viewer's embedded content must be
// framable by the viewer page. The global security headers deny all framing,
// so embedding the plain download URL showed nothing for PDFs and text.
func TestCustomerAttachmentViewerFraming(t *testing.T) {
	router := newCustAttRouter(t)
	for _, backend := range []string{storage.BackendDB, storage.BackendFS} {
		t.Run(backend, func(t *testing.T) {
			custAttUseBackend(t, backend)
			f := newCustAttFixture(t, router)
			textBytes := []byte("<script>alert(1)</script> plain\n")
			w := f.upload(t, f.ticketTN, custAttUpload{"page.html", "text/html", textBytes})
			require.Equal(t, http.StatusOK, w.Code, w.Body.String())
			atts := f.list(t, f.ticketTN)
			require.Len(t, atts, 1)

			w = f.do(t, http.MethodGet, atts[0].ViewURL, nil)
			require.Equal(t, http.StatusOK, w.Code)
			m := regexp.MustCompile(`<iframe src="([^"]+)"`).FindStringSubmatch(w.Body.String())
			require.Len(t, m, 2, "text attachments are previewed in an iframe")
			src := html.UnescapeString(m[1])
			assert.Equal(t, atts[0].ViewURL+"?raw=1", src)

			w = f.do(t, http.MethodGet, src, nil)
			require.Equal(t, http.StatusOK, w.Code)
			assert.Equal(t, textBytes, w.Body.Bytes())
			assert.NotEqual(t, "DENY", w.Header().Get("X-Frame-Options"))
			assert.Contains(t, w.Header().Get("Content-Security-Policy"), "frame-ancestors 'self'")
			assert.True(t, strings.HasPrefix(w.Header().Get("Content-Type"), "text/plain"),
				"HTML is shown as source, never rendered: %s", w.Header().Get("Content-Type"))
		})
	}
}

// TestCustomerAttachmentFilenameEscaping: attachment names are user input and
// must not inject markup into the HTMX list or the viewer page, nor headers.
func TestCustomerAttachmentFilenameEscaping(t *testing.T) {
	router := newCustAttRouter(t)
	for _, backend := range []string{storage.BackendDB, storage.BackendFS} {
		t.Run(backend, func(t *testing.T) {
			custAttUseBackend(t, backend)
			f := newCustAttFixture(t, router)
			name := `x"><img src=x onerror=alert(1)>.txt`
			w := f.upload(t, f.ticketTN, custAttUpload{name, "text/plain", []byte("evil")})
			require.Equal(t, http.StatusOK, w.Code, w.Body.String())
			atts := f.list(t, f.ticketTN)
			require.Len(t, atts, 1)
			require.Equal(t, name, atts[0].Filename)

			w = f.do(t, http.MethodGet, "/customer/tickets/"+f.ticketTN+"/attachments", nil, "HX-Request", "true")
			require.Equal(t, http.StatusOK, w.Code)
			body := w.Body.String()
			assert.NotContains(t, body, "<img src=x")
			assert.Contains(t, body, html.EscapeString(name))
			assert.Contains(t, body, `href="`+atts[0].ViewURL+`"`)

			w = f.do(t, http.MethodGet, atts[0].ViewURL, nil)
			require.Equal(t, http.StatusOK, w.Code)
			assert.NotContains(t, w.Body.String(), "<img src=x")
			assert.Contains(t, w.Body.String(), "<title>"+html.EscapeString(name)+" - GoatFlow</title>")

			w = f.do(t, http.MethodGet, atts[0].DownloadURL, nil)
			require.Equal(t, http.StatusOK, w.Code)
			_, params, err := mime.ParseMediaType(w.Header().Get("Content-Disposition"))
			require.NoError(t, err)
			assert.Equal(t, name, params["filename"])
		})
	}
}
