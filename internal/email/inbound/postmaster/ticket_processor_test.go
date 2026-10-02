package postmaster

import (
	"context"
	"database/sql"
	"encoding/base64"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/goatkit/goatflow/internal/platform/database"
	"github.com/goatkit/goatflow/internal/platform/email/inbound/connector"
	"github.com/goatkit/goatflow/internal/repository"
	"github.com/goatkit/goatflow/internal/service"
	"github.com/goatkit/goatflow/internal/storage"
	"github.com/goatkit/goatflow/internal/ticketnumber"
)

func postmasterTestDB(t *testing.T) *sql.DB {
	t.Helper()
	if os.Getenv("GOATFLOW_TEST_DB_READY") != "1" {
		t.Skip("needs the test database (GOATFLOW_TEST_DB_READY=1)")
	}
	require.NoError(t, database.InitTestDB())
	db, err := database.GetDB()
	require.NoError(t, err)
	require.NotNil(t, db)
	gen, err := ticketnumber.Resolve("DateChecksum", "10", nil)
	require.NoError(t, err)
	repository.SetTicketNumberGenerator(gen, ticketnumber.NewDBStore(db, "10"))
	t.Cleanup(func() { repository.SetTicketNumberGenerator(nil, nil) })
	return db
}

var (
	fixturePDF       = []byte("%PDF-1.4\n%\xe2\xe3\xcf\xd3\n1 0 obj << /Type /Catalog >> endobj\ntrailer\n%%EOF\n")
	fixtureLogo      = []byte("\x89PNG\r\n\x1a\n\x00\x00\x00\rIHDR logo")
	fixtureSignature = []byte("\x89PNG\r\n\x1a\n\x00\x00\x00\rIHDR signature")
)

// inboundFixture is an HTML mail with an inline logo (Content-Disposition:
// inline + Content-ID), a signature image with only a Content-ID, and a PDF
// attachment.
func inboundFixture() []byte {
	b64 := func(b []byte) string { return base64.StdEncoding.EncodeToString(b) }
	return []byte(strings.ReplaceAll(`From: Alice Example <alice@example.com>
To: support@example.com
Subject: Invoice with logo
Message-ID: <inbound-fixture@example.com>
MIME-Version: 1.0
Content-Type: multipart/mixed; boundary="mixed"

--mixed
Content-Type: multipart/related; boundary="related"

--related
Content-Type: text/html; charset=utf-8

<p>See <img src="cid:logo@example.com"> and <img src="cid:sig@example.com"></p>
--related
Content-Type: image/png; name="logo.png"
Content-Transfer-Encoding: base64
Content-ID: <logo@example.com>
Content-Disposition: inline; filename="logo.png"

`+b64(fixtureLogo)+`
--related
Content-Type: image/png; name="signature.png"
Content-Transfer-Encoding: base64
Content-ID: <sig@example.com>

`+b64(fixtureSignature)+`
--related--
--mixed
Content-Type: application/pdf; name="invoice.pdf"
Content-Transfer-Encoding: base64
Content-Disposition: attachment; filename="invoice.pdf"

`+b64(fixturePDF)+`
--mixed--
`, "\n", "\r\n"))
}

func deleteTicket(db *sql.DB, ticketID int) {
	for _, q := range []string{
		"DELETE FROM article_data_mime_attachment WHERE article_id IN (SELECT id FROM article WHERE ticket_id = ?)",
		"DELETE FROM article_data_mime_plain WHERE article_id IN (SELECT id FROM article WHERE ticket_id = ?)",
		"DELETE FROM article_data_mime WHERE article_id IN (SELECT id FROM article WHERE ticket_id = ?)",
		"DELETE FROM ticket_history WHERE ticket_id = ?",
		"DELETE FROM article WHERE ticket_id = ?",
		"DELETE FROM ticket WHERE id = ?",
	} {
		_, _ = db.Exec(database.ConvertPlaceholders(q), ticketID)
	}
}

// TestProcessStoresMailAttachments processes a real multipart mail and checks
// that every attachment lands in the configured article storage (DB rows or
// the OTRS FS tree) exactly once, with its content type, Content-ID and
// disposition.
func TestProcessStoresMailAttachments(t *testing.T) {
	db := postmasterTestDB(t)

	for _, backend := range []string{storage.BackendDB, storage.BackendFS} {
		t.Run(backend, func(t *testing.T) {
			articleDir := t.TempDir()
			require.NoError(t, storage.Configure(storage.Config{Backend: backend, ArticleDir: articleDir}))
			t.Cleanup(func() { _ = storage.Configure(storage.Config{Backend: storage.BackendDB}) })

			ticketRepo := repository.NewTicketRepository(db)
			articleRepo := repository.NewArticleRepository(db)
			tp := NewTicketProcessor(
				service.NewTicketService(ticketRepo, service.WithArticleRepository(articleRepo)),
				WithTicketProcessorArticleLookup(articleRepo),
				WithTicketProcessorDatabase(db),
			)

			res, err := tp.Process(context.Background(), &connector.FetchedMessage{UID: "fixture-1", Raw: inboundFixture()}, nil)
			require.NoError(t, err)
			require.Equal(t, "new_ticket", res.Action)
			require.Positive(t, res.TicketID)
			t.Cleanup(func() { deleteTicket(db, res.TicketID) })

			article, err := articleRepo.GetLatestCustomerArticleForTicket(uint(res.TicketID))
			require.NoError(t, err)
			articleID := int64(article.ID)

			all, err := storage.ForDB(db).ListAttachments(context.Background(), articleID)
			require.NoError(t, err)
			var atts []storage.Attachment
			for _, a := range all {
				if !storage.IsHTMLBody(a) {
					atts = append(atts, a)
				}
			}
			require.Len(t, atts, 3, "%+v", all)

			want := map[string]struct {
				contentType, contentID, disposition string
				content                             []byte
			}{
				"logo.png":      {"image/png", "<logo@example.com>", "inline", fixtureLogo},
				"signature.png": {"image/png", "<sig@example.com>", "inline", fixtureSignature},
				"invoice.pdf":   {"application/pdf", "", "attachment", fixturePDF},
			}
			for _, a := range atts {
				w, ok := want[a.Filename]
				require.True(t, ok, "unexpected attachment %q", a.Filename)
				assert.Equal(t, w.contentType, a.ContentType, a.Filename)
				assert.Equal(t, w.contentID, a.ContentID, a.Filename)
				assert.Equal(t, w.disposition, a.Disposition, a.Filename)
				assert.Equal(t, int64(len(w.content)), a.Size, a.Filename)
				_, content, err := storage.ForDB(db).GetAttachment(context.Background(), articleID, a.FileID)
				require.NoError(t, err)
				assert.Equal(t, w.content, content, a.Filename)
			}

			var rows int
			require.NoError(t, db.QueryRow(database.ConvertPlaceholders(
				"SELECT COUNT(*) FROM article_data_mime_attachment WHERE article_id = ?"), articleID).Scan(&rows))
			if backend == storage.BackendDB {
				assert.Equal(t, len(all), rows, "one row per stored part, no double write")
				return
			}
			assert.Zero(t, rows, "FS mode must not also write attachment rows")
			matches, err := filepath.Glob(filepath.Join(articleDir, "*", "*", "*", strconv.FormatInt(articleID, 10), "invoice.pdf"))
			require.NoError(t, err)
			require.Len(t, matches, 1, "invoice.pdf in <dir>/YYYY/MM/DD/<article id>/")
			onDisk, err := os.ReadFile(matches[0])
			require.NoError(t, err)
			assert.Equal(t, fixturePDF, onDisk)
			ct, err := os.ReadFile(matches[0] + ".content_type")
			require.NoError(t, err)
			assert.Equal(t, "application/pdf", string(ct))
			cid, err := os.ReadFile(filepath.Join(filepath.Dir(matches[0]), "logo.png.content_id"))
			require.NoError(t, err)
			assert.Equal(t, "<logo@example.com>", string(cid))
		})
	}
}
