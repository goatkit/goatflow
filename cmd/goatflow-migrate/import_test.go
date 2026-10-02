package main

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/go-sql-driver/mysql"
	"github.com/lib/pq"

	"github.com/goatkit/goatflow/internal/api"
	"github.com/goatkit/goatflow/internal/models"
	"github.com/goatkit/goatflow/internal/platform/auth"
	"github.com/goatkit/goatflow/internal/platform/database"
	"github.com/goatkit/goatflow/internal/platform/lookups"
	"github.com/goatkit/goatflow/internal/platform/middleware"
	"github.com/goatkit/goatflow/internal/platform/shared"
	"github.com/goatkit/goatflow/internal/platform/sysconfig"
	"github.com/goatkit/goatflow/internal/repository"
	"github.com/goatkit/goatflow/internal/service"
	"github.com/goatkit/goatflow/internal/storage"
	"github.com/goatkit/goatflow/internal/ticketnumber"
)

// Values held by testdata/otrs6_dump.sql (an OTRS 6 database dumped with
// mariadb-dump), byte for byte.
const (
	dumpBody7  = "Hello,\n\nthe printer's on fire \\ again.\r\nIt says \"PC LOAD LETTER\" — ünïcödé ✓\tand a \x1a sub."
	dumpBody12 = "Dear Bob,\n\nplease find the invoice attached.\n\n-- \nAlice Smith\nExample Support"
	// Attachment 31 is a --hex-blob literal, 32 a MySQL 8 _binary string, 33 a plain escaped string.
	dumpContent31 = "%PDF-1.4\n\x00'\\\xff\r\x1a\"end\n"
	dumpContent32 = "\x89PNG\r\n\x1a\n\x00\x00''\\\\\xff\xfe\x00IEND"
	dumpContent33 = "notes: it's a \\ backslash\näöü\n"
	dumpPlain7    = "From: \"Jane Customer\" <jane@acme.example>\r\nTo: support@example.com\r\nSubject: Printer on fire\r\n" +
		"Message-ID: <20140305.abc@acme.example>\r\nContent-Type: text/plain; charset=utf-8\r\n\r\n" +
		"Hello,\r\n\r\nthe printer's on fire \\ again.\r\n\x00\xff\r\n"
)

// importTarget is the GoatFlow database an import test writes to.
type importTarget struct {
	db  *sql.DB
	url string // the -db argument an operator would pass for it
}

// importTestDB returns the database the imports of a test write to. An import
// replaces the lookups, agents, settings and tickets of its target with the
// OTRS fixture's, and the packages tested after this one must still find the
// seeded test database. Where the test database user may create databases,
// the imports go into a scratch database migrated like a fresh install and
// dropped when the test ends; elsewhere (the MySQL test user's grants cover
// its own database only) the tables of the test database are put back.
func importTestDB(t *testing.T) importTarget {
	t.Helper()
	if os.Getenv("GOATFLOW_TEST_DB_READY") != "1" {
		t.Skip("needs the test database (GOATFLOW_TEST_DB_READY=1)")
	}
	if err := database.InitTestDB(); err != nil {
		t.Fatalf("init test db: %v", err)
	}
	db, err := database.GetDB()
	if err != nil || db == nil {
		t.Fatalf("get test db: %v", err)
	}
	name := os.Getenv("TEST_DB_NAME") + "_import_" + strconv.FormatInt(time.Now().UnixNano(), 36)
	if _, err := db.Exec(database.ConvertPlaceholders("CREATE DATABASE `" + name + "`")); err != nil {
		if !accessDenied(err) {
			t.Fatalf("create scratch database %s: %v", name, err)
		}
		keepTables(t, db)
		return importTarget{db: db, url: testDBURL(t, os.Getenv("TEST_DB_NAME"))}
	}
	t.Cleanup(func() {
		if _, err := db.Exec(database.ConvertPlaceholders("DROP DATABASE IF EXISTS `" + name + "`")); err != nil {
			t.Errorf("drop scratch database %s: %v", name, err)
		}
	})
	target := importTarget{url: testDBURL(t, name)}
	migrateScratch(t, target.url)
	if target.db, err = openDB(target.url); err != nil {
		t.Fatal(err)
	}
	// The API handlers an import is checked through use the global database.
	database.SetDB(target.db)
	t.Cleanup(func() {
		database.ResetDB()
		_ = target.db.Close()
	})
	return target
}

// accessDenied reports whether the server refused a statement for lack of
// privileges.
func accessDenied(err error) bool {
	var myErr *mysql.MySQLError
	var pqErr *pq.Error
	return (errors.As(err, &myErr) && myErr.Number == 1044) || (errors.As(err, &pqErr) && pqErr.Code == "42501")
}

// migrateScratch applies the GoatFlow migrations of the driver to the empty
// database at dbURL, as a fresh install does.
func migrateScratch(t *testing.T, dbURL string) {
	t.Helper()
	dir := "mysql"
	if database.IsPostgreSQL() {
		dir = "postgres"
	} else {
		dbURL += "&multiStatements=true"
	}
	files, err := filepath.Glob(filepath.Join("..", "..", "migrations", dir, "*.up.sql"))
	if err != nil || len(files) == 0 {
		t.Fatalf("migrations/%s: %v", dir, err)
	}
	db, err := openDB(dbURL)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	for _, f := range files {
		script, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := db.Exec(string(script)); err != nil { // sql-converted: migration file of the active driver
			t.Fatalf("migrate scratch database: %s: %v", filepath.Base(f), err)
		}
	}
}

// keepTables copies every table of db into temporary tables of one
// connection and, when the test ends, puts the copied rows back in place of
// whatever the test left. AUTO_INCREMENT counters move past the restored ids
// by themselves.
func keepTables(t *testing.T, db *sql.DB) {
	t.Helper()
	ctx := context.Background()
	schema, err := loadTargetSchema(db)
	if err != nil {
		t.Fatal(err)
	}
	tables := make([]string, 0, len(schema))
	for name := range schema {
		tables = append(tables, name)
	}
	sort.Strings(tables)
	conn, err := db.Conn(ctx)
	if err != nil {
		t.Fatal(err)
	}
	exec := func(query string) error {
		_, err := conn.ExecContext(ctx, database.ConvertPlaceholders(query))
		return err
	}
	for _, name := range tables {
		if err := exec("CREATE TEMPORARY TABLE `kept_" + name + "` AS SELECT * FROM `" + name + "`"); err != nil {
			_ = conn.Close()
			t.Fatalf("keep %s: %v", name, err)
		}
	}
	t.Cleanup(func() {
		defer conn.Close()
		// Rows go back in name order, not foreign key order.
		if err := exec("SET FOREIGN_KEY_CHECKS = 0"); err != nil {
			t.Errorf("restore test database: %v", err)
			return
		}
		for _, name := range tables {
			for _, stmt := range []string{
				"DELETE FROM `" + name + "`",
				"INSERT INTO `" + name + "` SELECT * FROM `kept_" + name + "`",
				"DROP TABLE `kept_" + name + "`",
			} {
				if err := exec(stmt); err != nil {
					t.Errorf("restore %s: %v", name, err)
				}
			}
		}
		if err := exec("SET FOREIGN_KEY_CHECKS = 1"); err != nil {
			t.Errorf("restore test database: %v", err)
		}
		lookups.Invalidate(db)
	})
}

// testDBURL is the -db argument an operator would pass for database name on
// the test database server.
func testDBURL(t *testing.T, name string) string {
	t.Helper()
	host, port := os.Getenv("TEST_DB_HOST"), os.Getenv("TEST_DB_PORT")
	user, pass := os.Getenv("TEST_DB_USER"), os.Getenv("TEST_DB_PASSWORD")
	if host == "" || port == "" || name == "" || user == "" {
		t.Fatalf("TEST_DB_HOST/PORT/NAME/USER must be set")
	}
	if database.IsPostgreSQL() {
		u := url.URL{Scheme: "postgres", User: url.UserPassword(user, pass), Host: host + ":" + port, Path: "/" + name, RawQuery: "sslmode=disable"}
		return u.String()
	}
	return fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?parseTime=true", user, pass, host, port, name)
}

// writeOTRSArticleTree writes the ArticleStorageFS directory OTRS keeps for
// article 7 (content_path 2014/03/05) and returns its files.
func writeOTRSArticleTree(t *testing.T, root string) map[string]string {
	t.Helper()
	files := map[string]string{
		"Report.pdf":                "%PDF-1.5\n\x00\x01\x02 binary report\xff\n",
		"Report.pdf.content_type":   "application/pdf",
		"Report.pdf.disposition":    "attachment",
		"image001.png":              "\x89PNG\r\n\x1a\n\x00inline image\xff",
		"image001.png.content_type": "image/png",
		"image001.png.content_id":   "<image001.png@01D03F2A.5B6C7D80>",
		"plain.txt":                 "From: jane@acme.example\r\nSubject: Printer on fire\r\n\r\nHello from the FS tree.\r\n",
	}
	dir := filepath.Join(root, "2014", "03", "05", "7")
	if err := os.MkdirAll(dir, 0o750); err != nil {
		t.Fatal(err)
	}
	for name, content := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return files
}

// importSources are the OTRS source formats the import reads, each holding
// the same OTRS database.
var importSources = []struct {
	name string
	open func(t *testing.T, target importTarget) source
}{
	{"mysqldump file", func(t *testing.T, _ importTarget) source {
		src, err := openMySQLDump("testdata/otrs6_dump.sql")
		if err != nil {
			t.Fatal(err)
		}
		return src
	}},
	{"PostgreSQL database", openPostgresFixture},
}

// openPostgresFixture loads testdata/otrs6_postgres.sql (an OTRS database on
// PostgreSQL) into its own schema of the target database and opens it as the
// source through a DSN, as an operator would.
func openPostgresFixture(t *testing.T, target importTarget) source {
	t.Helper()
	db := target.db
	if !database.IsPostgreSQL() {
		t.Skip("the OTRS-on-PostgreSQL source fixture is loaded into the PostgreSQL test server; the MySQL test user cannot create a second database")
	}
	script, err := os.ReadFile("testdata/otrs6_postgres.sql")
	if err != nil {
		t.Fatal(err)
	}
	for _, stmt := range []string{"DROP SCHEMA IF EXISTS otrs_source CASCADE", "CREATE SCHEMA otrs_source"} {
		if _, err := db.Exec(database.ConvertPlaceholders(stmt)); err != nil {
			t.Fatal(err)
		}
	}
	t.Cleanup(func() { _, _ = db.Exec(database.ConvertPlaceholders("DROP SCHEMA IF EXISTS otrs_source CASCADE")) })
	dsn := target.url + "&search_path=otrs_source"
	loader, err := sql.Open("postgres", dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer loader.Close()
	if _, err := loader.Exec(string(script)); err != nil { // sql-converted: PostgreSQL fixture script for the source schema
		t.Fatalf("load %s: %v", "testdata/otrs6_postgres.sql", err)
	}
	src, err := openDBSource(dsn)
	if err != nil {
		t.Fatal(err)
	}
	return src
}

func TestImportOTRS(t *testing.T) {
	target := importTestDB(t)
	tree := t.TempDir()
	files := writeOTRSArticleTree(t, tree)
	for _, s := range importSources {
		t.Run(s.name, func(t *testing.T) {
			src := s.open(t, target)
			defer src.Close()
			if err := runImport(src, target.url, false, false, true); err != nil {
				t.Fatalf("import: %v", err)
			}
			checkImport(t, target, tree, files)
		})
	}
}

func checkImport(t *testing.T, target importTarget, tree string, files map[string]string) {
	db := target.db
	ctx := context.Background()

	t.Run("tickets and articles keep their OTRS ids", func(t *testing.T) {
		for tn, want := range map[string]int64{"2014030510000013": 3, "2014030610000091": 9, "2014030710000024": 15} {
			var id int64
			if err := db.QueryRow(database.ConvertPlaceholders("SELECT id FROM ticket WHERE tn = ?"), tn).Scan(&id); err != nil {
				t.Errorf("ticket %s: %v", tn, err)
			} else if id != want {
				t.Errorf("ticket %s has id %d, want OTRS id %d", tn, id, want)
			}
		}
		rows, err := db.Query(database.ConvertPlaceholders("SELECT id, ticket_id FROM article ORDER BY id"))
		if err != nil {
			t.Fatal(err)
		}
		defer rows.Close()
		var got [][2]int64
		for rows.Next() {
			var a [2]int64
			if err := rows.Scan(&a[0], &a[1]); err != nil {
				t.Fatal(err)
			}
			got = append(got, a)
		}
		if want := [][2]int64{{7, 3}, {12, 9}}; fmt.Sprint(got) != fmt.Sprint(want) {
			t.Errorf("articles (id, ticket_id) = %v, want %v", got, want)
		}
	})

	t.Run("ticket references resolve to the OTRS lookup rows", func(t *testing.T) {
		var queue, state, stateType, owner, ticketType, priority string
		err := db.QueryRow(database.ConvertPlaceholders(`
			SELECT q.name, s.name, st.name, u.login, ty.name, p.name
			FROM ticket t
			JOIN queue q ON q.id = t.queue_id
			JOIN ticket_state s ON s.id = t.ticket_state_id
			JOIN ticket_state_type st ON st.id = s.type_id
			JOIN users u ON u.id = t.user_id
			JOIN ticket_type ty ON ty.id = t.type_id
			JOIN ticket_priority p ON p.id = t.ticket_priority_id
			WHERE t.id = ?`), 9).Scan(&queue, &state, &stateType, &owner, &ticketType, &priority)
		if err != nil {
			t.Fatalf("ticket 9 lookups: %v", err)
		}
		got := []string{queue, state, stateType, owner, ticketType, priority}
		want := []string{"Support", "closed successful", "closed", "asmith", "Problem", "4 high"}
		if fmt.Sprint(got) != fmt.Sprint(want) {
			t.Errorf("ticket 9 queue/state/state type/owner/type/priority = %q, want %q", got, want)
		}
		var changeBy int
		if err := db.QueryRow(database.ConvertPlaceholders("SELECT change_by FROM users WHERE login = ?"), "jdoe").Scan(&changeBy); err != nil {
			t.Errorf("user jdoe: %v", err)
		} else if changeBy != 3 {
			t.Errorf("jdoe change_by = %d, want 3", changeBy)
		}
		var customerIDs []int64
		cu, err := db.Query(database.ConvertPlaceholders("SELECT id FROM customer_user WHERE customer_id = ? ORDER BY id"), "ACME")
		if err != nil {
			t.Fatal(err)
		}
		defer cu.Close()
		for cu.Next() {
			var id int64
			if err := cu.Scan(&id); err != nil {
				t.Fatal(err)
			}
			customerIDs = append(customerIDs, id)
		}
		if fmt.Sprint(customerIDs) != "[5 8]" {
			t.Errorf("customer_user ids = %v, want [5 8]", customerIDs)
		}
		var company string
		if err := db.QueryRow(database.ConvertPlaceholders("SELECT name FROM customer_company WHERE customer_id = ?"), "ACME").Scan(&company); err != nil || company != "ACME Corp." {
			t.Errorf("customer_company ACME = %q, %v", company, err)
		}
	})

	t.Run("article body and content path come from article_data_mime", func(t *testing.T) {
		for _, c := range []struct {
			article     int64
			body, cpath string
		}{{7, dumpBody7, "2014/03/05"}, {12, dumpBody12, "2014/03/06"}} {
			var body, cpath sql.NullString
			err := db.QueryRow(database.ConvertPlaceholders(
				"SELECT a_body, content_path FROM article_data_mime WHERE article_id = ?"), c.article).Scan(&body, &cpath)
			if err != nil {
				t.Errorf("article_data_mime of article %d: %v", c.article, err)
				continue
			}
			if body.String != c.body {
				t.Errorf("article %d body = %q, want %q", c.article, body.String, c.body)
			}
			if cpath.String != c.cpath {
				t.Errorf("article %d content_path = %q, want %q", c.article, cpath.String, c.cpath)
			}
		}
	})

	t.Run("DB storage serves the imported attachments byte for byte", func(t *testing.T) {
		store := storage.NewDatabaseStore(db)
		list, err := store.ListAttachments(ctx, 12)
		if err != nil {
			t.Fatal(err)
		}
		want := []struct {
			id                                       int64
			filename, contentType, contentID, dispos string
			content                                  string
		}{
			{31, "invoice.pdf", "application/pdf", "", "attachment", dumpContent31},
			{32, "logo.png", "image/png", "<logo.png@01D03F>", "inline", dumpContent32},
			{33, "notes.txt", "text/plain; charset=utf-8", "", "attachment", dumpContent33},
		}
		if len(list) != len(want) {
			t.Fatalf("article 12 has %d attachments, want %d: %+v", len(list), len(want), list)
		}
		for i, w := range want {
			a := list[i]
			if a.FileID != w.id || a.Filename != w.filename || a.ContentType != w.contentType ||
				a.ContentID != w.contentID || a.Disposition != w.dispos || a.Size != int64(len(w.content)) {
				t.Errorf("attachment %d = %+v, want id %d %s %s %q %s size %d",
					i, a, w.id, w.filename, w.contentType, w.contentID, w.dispos, len(w.content))
			}
			_, content, err := store.GetAttachment(ctx, 12, w.id)
			if err != nil {
				t.Errorf("get attachment %d: %v", w.id, err)
			} else if string(content) != w.content {
				t.Errorf("attachment %d content = %q, want %q", w.id, content, w.content)
			}
		}
		plain, err := store.ReadPlain(ctx, 7)
		if err != nil {
			t.Errorf("plain email of article 7: %v", err)
		} else if string(plain) != dumpPlain7 {
			t.Errorf("plain email of article 7 = %q, want %q", plain, dumpPlain7)
		}
	})

	t.Run("FS storage finds the OTRS article tree under the imported id and content path", func(t *testing.T) {
		store := storage.NewFilesystemStore(tree, db)
		list, err := store.ListAttachments(ctx, 7)
		if err != nil {
			t.Fatal(err)
		}
		if len(list) != 2 {
			t.Fatalf("article 7 FS attachments = %+v, want Report.pdf and image001.png", list)
		}
		want := []storage.Attachment{
			{ArticleID: 7, FileID: 1, Filename: "Report.pdf", ContentType: "application/pdf", Disposition: "attachment", Size: int64(len(files["Report.pdf"]))},
			{ArticleID: 7, FileID: 2, Filename: "image001.png", ContentType: "image/png", ContentID: files["image001.png.content_id"], Disposition: "inline", Size: int64(len(files["image001.png"]))},
		}
		for i, w := range want {
			a := list[i]
			if a.ArticleID != w.ArticleID || a.FileID != w.FileID || a.Filename != w.Filename || a.ContentType != w.ContentType ||
				a.ContentID != w.ContentID || a.Disposition != w.Disposition || a.Size != w.Size {
				t.Errorf("FS attachment %d = %+v, want %+v", i, a, w)
			}
			_, content, err := store.GetAttachment(ctx, 7, w.FileID)
			if err != nil {
				t.Errorf("get FS attachment %d: %v", w.FileID, err)
			} else if string(content) != files[w.Filename] {
				t.Errorf("FS attachment %s content = %q, want %q", w.Filename, content, files[w.Filename])
			}
		}
		plain, err := store.ReadPlain(ctx, 7)
		if err != nil {
			t.Errorf("FS plain email: %v", err)
		} else if string(plain) != files["plain.txt"] {
			t.Errorf("FS plain email = %q, want %q", plain, files["plain.txt"])
		}
	})

	t.Run("ticket history keeps its ids and article links", func(t *testing.T) {
		rows, err := db.Query(database.ConvertPlaceholders(`
			SELECT h.id, h.ticket_id, h.article_id, ht.name
			FROM ticket_history h JOIN ticket_history_type ht ON ht.id = h.history_type_id
			ORDER BY h.id`))
		if err != nil {
			t.Fatal(err)
		}
		defer rows.Close()
		var got []string
		for rows.Next() {
			var id, ticketID int64
			var articleID sql.NullInt64
			var typ string
			if err := rows.Scan(&id, &ticketID, &articleID, &typ); err != nil {
				t.Fatal(err)
			}
			got = append(got, fmt.Sprintf("%d:%d:%v:%s", id, ticketID, articleID.Int64, typ))
		}
		want := []string{"100:3:7:NewTicket", "101:9:12:SendAnswer", "102:9:0:StateUpdate"}
		if fmt.Sprint(got) != fmt.Sprint(want) {
			t.Errorf("ticket_history (id:ticket:article:type) = %v, want %v", got, want)
		}
	})

	t.Run("new tickets and articles continue after the imported ids", func(t *testing.T) {
		now := time.Now()
		ticketID, err := database.GetAdapter().InsertWithReturning(db, database.ConvertPlaceholders(`
			INSERT INTO ticket (tn, title, queue_id, ticket_lock_id, type_id, user_id, responsible_user_id,
				ticket_priority_id, ticket_state_id, timeout, until_time, escalation_time, escalation_update_time,
				escalation_response_time, escalation_solution_time, archive_flag, create_time, create_by, change_time, change_by)
			VALUES (?, ?, 1, 1, 1, 1, 1, 3, 1, 0, 0, 0, 0, 0, 0, 0, ?, 1, ?, 1) RETURNING id`),
			"2026100110000001", "after import", now, now)
		if err != nil {
			t.Fatalf("insert ticket: %v", err)
		}
		if ticketID != 16 {
			t.Errorf("next ticket id = %d, want 16 (max imported id 15 + 1)", ticketID)
		}
		articleID, err := database.GetAdapter().InsertWithReturning(db, database.ConvertPlaceholders(`
			INSERT INTO article (ticket_id, article_sender_type_id, communication_channel_id, is_visible_for_customer,
				search_index_needs_rebuild, create_time, create_by, change_time, change_by)
			VALUES (?, 1, 3, 0, 1, ?, 1, ?, 1) RETURNING id`), ticketID, now, now)
		if err != nil {
			t.Fatalf("insert article: %v", err)
		}
		if articleID != 13 {
			t.Errorf("next article id = %d, want 13 (max imported id 12 + 1)", articleID)
		}
	})

	t.Run("permissions are exactly the OTRS group, role and customer group grants", func(t *testing.T) {
		expectRows(t, db, "SELECT user_id, group_id, permission_key FROM group_user ORDER BY user_id, group_id, permission_key", []string{
			"1:1:rw", "1:2:rw", "1:3:rw", "1:4:rw", "2:1:rw", "3:4:rw", "4:4:note", "4:4:rw",
		})
		expectRows(t, db, "SELECT r.id, r.name FROM roles r WHERE r.id IN (1, 2) ORDER BY r.id", []string{"1:Support Team", "2:Billing Team"})
		expectRows(t, db, "SELECT user_id, role_id FROM role_user ORDER BY user_id", []string{"3:1", "5:2"})
		expectRows(t, db, "SELECT role_id, group_id, permission_key, permission_value FROM group_role ORDER BY role_id, group_id",
			[]string{"1:4:ro:1", "2:4:rw:0", "2:5:rw:1"})
		expectRows(t, db, "SELECT user_id, group_id, permission_key FROM group_customer_user ORDER BY permission_key",
			[]string{"jane:4:ro", "jane:4:rw"})
	})

	t.Run("agent and customer preferences", func(t *testing.T) {
		var sig []byte
		if err := db.QueryRow(database.ConvertPlaceholders(
			"SELECT preferences_value FROM user_preferences WHERE user_id = ? AND preferences_key = ?"), 4, "UserSignature").Scan(&sig); err != nil {
			t.Fatal(err)
		}
		if string(sig) != "Grüße,\nKarl" {
			t.Errorf("UserSignature = %q", sig)
		}
		expectRows(t, db, "SELECT user_id, preferences_key, preferences_value FROM customer_preferences ORDER BY user_id",
			[]string{"bob:UserShowTickets:25", "jane:UserLanguage:fr"})
		expectRows(t, db, "SELECT user_id, customer_id FROM customer_user_customer", []string{"bob:ACME"})
	})

	t.Run("ticket data: dynamic fields, time accounting, flags, links, watchers", func(t *testing.T) {
		expectRows(t, db, `SELECT f.name, v.object_id, v.value_text FROM dynamic_field_value v
			JOIN dynamic_field f ON f.id = v.field_id ORDER BY v.id`,
			[]string{"CustomerReference:9:PO-4711", "NoteKind:12:internal", "CustomerReference:3:REF-0815"})
		expectRows(t, db, "SELECT ticket_id, article_id, time_unit FROM time_accounting ORDER BY id", []string{"9:12:15.50", "3::0.25"})
		expectRows(t, db, "SELECT ticket_id, ticket_key, create_by FROM ticket_flag ORDER BY ticket_id", []string{"3:Seen:2", "9:Seen:3"})
		expectRows(t, db, "SELECT article_id, article_key, create_by FROM article_flag", []string{"12:Seen:3"})
		expectRows(t, db, `SELECT r.source_key, r.target_key, ty.name, st.name FROM link_relation r
			JOIN link_type ty ON ty.id = r.type_id JOIN link_state st ON st.id = r.state_id`, []string{"3:9:ParentChild:Valid"})
		expectRows(t, db, "SELECT ticket_id, user_id FROM ticket_watcher", []string{"9:2"})
	})

	t.Run("templates with attachments and auto responses stay linked to their queues", func(t *testing.T) {
		expectRows(t, db, `SELECT q.name, st.name FROM queue_standard_template qst
			JOIN queue q ON q.id = qst.queue_id JOIN standard_template st ON st.id = qst.standard_template_id
			ORDER BY q.id, st.id`, []string{"Support:empty answer", "Support:Invoice copy", "Billing:Invoice copy"})
		var filename string
		var content []byte
		if err := db.QueryRow(database.ConvertPlaceholders(`SELECT sa.filename, sa.content FROM standard_template_attachment sta
			JOIN standard_attachment sa ON sa.id = sta.standard_attachment_id
			JOIN standard_template st ON st.id = sta.standard_template_id WHERE st.name = ?`), "Invoice copy").Scan(&filename, &content); err != nil {
			t.Fatal(err)
		}
		if filename != "terms.pdf" || string(content) != "%PDF-1.3\n\x00\xff\nterms\n" {
			t.Errorf("template attachment = %s %q", filename, content)
		}
		expectRows(t, db, `SELECT q.name, ar.name, art.name, sa.value0 FROM queue_auto_response qar
			JOIN queue q ON q.id = qar.queue_id JOIN auto_response ar ON ar.id = qar.auto_response_id
			JOIN auto_response_type art ON art.id = ar.type_id JOIN system_address sa ON sa.id = ar.system_address_id`,
			[]string{"Support:Support auto reply:auto reply:support@example.com"})
	})

	t.Run("notifications, generic agent jobs, ACLs, services and personal queues", func(t *testing.T) {
		expectRows(t, db, `SELECT n.name, m.language, m.subject FROM notification_event_message m
			JOIN notification_event n ON n.id = m.notification_id ORDER BY m.language`, []string{
			"Ticket create notification:de:Ticket erstellt: <OTRS_TICKET_Title>",
			"Ticket create notification:en:Ticket Created: <OTRS_TICKET_Title>",
		})
		expectRows(t, db, "SELECT notification_id, event_key, event_value FROM notification_event_item ORDER BY event_key",
			[]string{"1:Events:NotificationNewTicket", "1:Recipients:AgentMyQueues"})
		expectRows(t, db, "SELECT job_key, job_value FROM generic_agent_jobs WHERE job_name = 'close stale pending' ORDER BY job_key",
			[]string{"NewStateID:2", "ScheduleDays:1", "Valid:1"})
		var match []byte
		if err := db.QueryRow(database.ConvertPlaceholders("SELECT config_match FROM acl WHERE name = ?"), "billing-no-close").Scan(&match); err != nil {
			t.Fatal(err)
		}
		if string(match) != "---\nProperties:\n  Queue:\n    Name:\n    - Billing\n" {
			t.Errorf("acl config_match = %q", match)
		}
		expectRows(t, db, `SELECT s.name, l.name FROM service_sla x JOIN service s ON s.id = x.service_id JOIN sla l ON l.id = x.sla_id`,
			[]string{"Printing:Gold"})
		expectRows(t, db, "SELECT customer_user_login, service_id FROM service_customer_user", []string{"jane:1"})
		expectRows(t, db, "SELECT user_id, queue_id FROM personal_queues", []string{"4:5"})
	})

	t.Run("OTRS setting overrides are read by name", func(t *testing.T) {
		// GoatFlow defines TimeWorkingHours: the override points at its definition.
		if v, ok := sysconfig.Value(db, "TimeWorkingHours"); !ok || v != "---\nMon:\n- '9'\n- '10'\n- '11'\nTue: []\n" {
			t.Errorf("TimeWorkingHours = %q, %v", v, ok)
		}
		// GoatFlow does not define the calendar setting: its OTRS definition comes along.
		if v, ok := sysconfig.Value(db, "TimeWorkingHours::Calendar1"); !ok || v != "---\nSat:\n- '10'\n- '11'\n" {
			t.Errorf("TimeWorkingHours::Calendar1 = %q, %v", v, ok)
		}
		expectRows(t, db, `SELECT d.name FROM sysconfig_default d
			WHERE d.name IN ('TimeWorkingHours', 'TimeWorkingHours::Calendar1', 'Ticket::Hook') ORDER BY d.name`,
			[]string{"TimeWorkingHours", "TimeWorkingHours::Calendar1"})
	})

	t.Run("sessions are not imported", func(t *testing.T) {
		expectRows(t, db, "SELECT COUNT(*) FROM sessions WHERE session_id = 'a1b2c3'", []string{"0"})
	})

	t.Run("imported agents log in and see the tickets of the queues their groups and roles grant", func(t *testing.T) {
		r := ticketAPIRouter()
		for _, c := range []struct {
			login, password string
			status          int
			tickets         []string
		}{
			// group_user: support rw → queue Support.
			{"kgroup", "Imported-Agent-1", http.StatusOK, []string{"2014030610000091"}},
			// role Billing Team: billing rw → queue Billing; its support grant has permission_value 0.
			{"lrole", "Tr0ub4dor&3", http.StatusOK, []string{"2014030710000024"}},
			// no group, no role: no queue.
			{"mnone", "Imported-Agent-3", http.StatusForbidden, nil},
		} {
			t.Run(c.login, func(t *testing.T) {
				code, body := sendJSON(t, r, http.MethodPost, "/api/v1/auth/login", "", map[string]string{"login": c.login, "password": c.password})
				if code != http.StatusOK {
					t.Fatalf("login %s = %d %s", c.login, code, body)
				}
				var login struct {
					AccessToken string `json:"access_token"`
				}
				if err := json.Unmarshal([]byte(body), &login); err != nil || login.AccessToken == "" {
					t.Fatalf("login response %s: %v", body, err)
				}
				code, body = sendJSON(t, r, http.MethodGet, "/api/v1/tickets?per_page=100", login.AccessToken, nil)
				if code != c.status {
					t.Fatalf("GET /api/v1/tickets = %d %s, want %d", code, body, c.status)
				}
				if c.status != http.StatusOK {
					return
				}
				var list struct {
					Data []struct {
						TN string `json:"tn"`
					} `json:"data"`
				}
				if err := json.Unmarshal([]byte(body), &list); err != nil {
					t.Fatal(err)
				}
				var got []string
				for _, tk := range list.Data {
					got = append(got, tk.TN)
				}
				sort.Strings(got)
				if fmt.Sprint(got) != fmt.Sprint(c.tickets) {
					t.Errorf("%s sees tickets %v, want %v", c.login, got, c.tickets)
				}
			})
		}
	})

	t.Run("a second import without force is refused and changes nothing", func(t *testing.T) {
		src, err := openMySQLDump("testdata/otrs6_dump.sql")
		if err != nil {
			t.Fatal(err)
		}
		defer src.Close()
		err = runImport(src, target.url, false, false, false)
		if err == nil || !strings.Contains(err.Error(), "use --force") {
			t.Fatalf("import into a database with tickets: err = %v, want a --force refusal", err)
		}
		expectRows(t, db, "SELECT COUNT(*) FROM ticket", []string{"4"})
	})
}

// ticketAPIRouter wires the agent login and ticket list endpoints the way
// cmd/goats and routes/api-v1-global.yaml do.
func ticketAPIRouter() *gin.Engine {
	gin.SetMode(gin.TestMode)
	auth.SetUserRepoFactory(func(db *sql.DB) auth.UserLookup { return repository.NewUserRepository(db) })
	middleware.SetQueueAccessCheckerFactory(func(db *sql.DB) middleware.QueueAccessChecker {
		return service.NewQueueAccessService(db)
	})
	r := gin.New()
	r.POST("/api/v1/auth/login", api.HandleAPIv1AuthLogin)
	v1 := r.Group("/api/v1", middleware.UnifiedAuthMiddleware(shared.GetJWTManager()))
	v1.GET("/tickets", middleware.RequireAnyQueueAccess("ro"), api.HandleListTicketsAPI)
	return r
}

func sendJSON(t *testing.T, r *gin.Engine, method, path, token string, body interface{}) (int, string) {
	t.Helper()
	var reader *bytes.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			t.Fatal(err)
		}
		reader = bytes.NewReader(b)
	} else {
		reader = bytes.NewReader(nil)
	}
	req := httptest.NewRequest(method, path, reader)
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w.Code, w.Body.String()
}

// expectRows compares the rows of query, each joined with ':' (NULL as
// empty), to want.
func expectRows(t *testing.T, db *sql.DB, query string, want []string) {
	t.Helper()
	rows, err := db.Query(database.ConvertPlaceholders(query))
	if err != nil {
		t.Fatalf("%s: %v", query, err)
	}
	defer rows.Close()
	cols, err := rows.Columns()
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for rows.Next() {
		vals := make([]sql.NullString, len(cols))
		ptrs := make([]interface{}, len(cols))
		for i := range vals {
			ptrs[i] = &vals[i]
		}
		if err := rows.Scan(ptrs...); err != nil {
			t.Fatal(err)
		}
		parts := make([]string, len(vals))
		for i, v := range vals {
			parts[i] = v.String
		}
		got = append(got, strings.Join(parts, ":"))
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	if fmt.Sprint(got) != fmt.Sprint(want) {
		t.Errorf("%s\n got  %q\n want %q", strings.Join(strings.Fields(query), " "), got, want)
	}
}

// The database source reads both drivers byte for byte: read back the
// attachments the import wrote, through a DSN.
func TestDatabaseSourceReadsRowsByteForByte(t *testing.T) {
	target := importTestDB(t)
	dump, err := openMySQLDump("testdata/otrs6_dump.sql")
	if err != nil {
		t.Fatal(err)
	}
	defer dump.Close()
	if err := runImport(dump, target.url, false, false, true); err != nil {
		t.Fatalf("import: %v", err)
	}
	src, err := openDBSource(target.url)
	if err != nil {
		t.Fatal(err)
	}
	defer src.Close()
	want := map[string]string{"31": dumpContent31, "32": dumpContent32, "33": dumpContent33}
	got := map[string]string{}
	if err := src.rows("article_data_mime_attachment", func(columns []string, row []dumpValue) error {
		got[string(row[indexOf(columns, "id")].data)] = string(row[indexOf(columns, "content")].data)
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if fmt.Sprint(got) != fmt.Sprint(want) {
		t.Errorf("attachments read through %s = %q, want %q", src.describe(), got, want)
	}
	var created string
	if err := src.rows("ticket", func(columns []string, row []dumpValue) error {
		if string(row[indexOf(columns, "id")].data) == "9" {
			created = string(row[indexOf(columns, "create_time")].data)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if created != "2014-03-06 10:00:00" {
		t.Errorf("ticket 9 create_time read as %q", created)
	}
}

// Every OTRS table of the GoatFlow schema has a decision in the import plan.
func TestImportPlanCoversSchema(t *testing.T) {
	re := regexp.MustCompile("(?i)CREATE TABLE (?:IF NOT EXISTS )?[`\"]?([a-z_0-9]+)")
	for _, driver := range []string{"mysql", "postgres"} {
		files, err := filepath.Glob(filepath.Join("..", "..", "migrations", driver, "*.up.sql"))
		if err != nil || len(files) == 0 {
			t.Fatalf("migrations/%s: %v", driver, err)
		}
		for _, f := range files {
			b, err := os.ReadFile(f)
			if err != nil {
				t.Fatal(err)
			}
			for _, m := range re.FindAllStringSubmatch(string(b), -1) {
				if name := strings.ToLower(m[1]); !strings.HasPrefix(name, "gk_") {
					if _, ok := planFor(name); !ok {
						t.Errorf("%s creates table %s, which the import plan does not cover", filepath.Base(f), name)
					}
				}
			}
		}
	}
	seen := map[string]bool{}
	for _, p := range importPlan {
		if seen[p.name] {
			t.Errorf("%s is planned twice", p.name)
		}
		seen[p.name] = true
		if (p.mode == modeSkip || p.mode == modeGoatFlow) && p.reason == "" {
			t.Errorf("%s is not imported without a reason", p.name)
		}
	}
}

type fixedDay struct{ y, m, d int }

func (f fixedDay) Now() ticketnumber.TimeParts {
	return ticketnumber.TimeParts{Year: f.y, Month: f.m, Day: f.d}
}

type fixedCounter int64

func (c fixedCounter) Add(context.Context, bool, int64) (int64, error) { return int64(c), nil }

// otrsDumpWithTicketNumbers writes a copy of the fixture whose three tickets
// carry tns and whose OTRS Ticket::NumberGenerator setting is module.
func otrsDumpWithTicketNumbers(t *testing.T, module string, tns []string) string {
	t.Helper()
	b, err := os.ReadFile("testdata/otrs6_dump.sql")
	if err != nil {
		t.Fatal(err)
	}
	for i, old := range []string{"2014030510000013", "2014030610000091", "2014030710000024"} {
		if bytes.Count(b, []byte("'"+old+"'")) != 1 {
			t.Fatalf("fixture ticket number %s not found once", old)
		}
		b = bytes.Replace(b, []byte("'"+old+"'"), []byte("'"+tns[i]+"'"), 1)
	}
	b = append(b, []byte("INSERT INTO `sysconfig_modified` (`id`, `sysconfig_default_id`, `name`, `user_id`, `is_valid`, "+
		"`user_modification_active`, `effective_value`, `is_dirty`, `reset_to_default`, `create_time`, `create_by`, `change_time`, `change_by`) "+
		"VALUES (13,604,'Ticket::NumberGenerator',NULL,1,0,'--- "+module+"\\n',0,0,'2014-01-01 00:00:00',1,'2014-01-01 00:00:00',1);\n")...)
	path := filepath.Join(t.TempDir(), "otrs.sql")
	if err := os.WriteFile(path, b, 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

// After an import, tickets created through the ticket repository with the
// generator OTRS used (SystemID 10) continue OTRS's numbering instead of
// reusing an imported ticket number, including the tickets OTRS created on
// the day of the migration.
func TestTicketNumbersContinueAfterImport(t *testing.T) {
	target := importTestDB(t)
	db := target.db
	ctx := context.Background()
	now := time.Now().UTC()
	today := fixedDay{now.Year(), int(now.Month()), now.Day()}

	otrsDateChecksum := func(counter int64) string {
		tn, err := ticketnumber.NewDateChecksum(ticketnumber.Config{SystemID: "10", MinCounterSize: 5}, today).
			Next(ctx, fixedCounter(counter))
		if err != nil {
			t.Fatal(err)
		}
		return tn
	}
	cases := []struct {
		name, module, generator string
		tns                     []string
		counter                 func(tn string) int64 // the counter in a new GoatFlow ticket number
		lastCounter             int64
	}{
		{
			name: "DateChecksum", module: "Kernel::System::Ticket::Number::DateChecksum", generator: "DateChecksum",
			// OTRS created three tickets today.
			tns: []string{otrsDateChecksum(1), otrsDateChecksum(2), otrsDateChecksum(3)},
			counter: func(tn string) int64 {
				n, _ := strconv.ParseInt(tn[len("YYYYMMDD10"):len(tn)-1], 10, 64)
				return n
			},
			lastCounter: 3,
		},
		{
			// OTRS AutoIncrement numbers are SystemID + counter padded to 5 digits.
			name: "AutoIncrement", module: "Kernel::System::Ticket::Number::AutoIncrement", generator: "Increment",
			tns: []string{"1000041", "1000042", "1000043"},
			counter: func(tn string) int64 {
				n, _ := strconv.ParseInt(strings.TrimPrefix(tn, "10"), 10, 64)
				return n
			},
			lastCounter: 43,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			src, err := openMySQLDump(otrsDumpWithTicketNumbers(t, c.module, c.tns))
			if err != nil {
				t.Fatal(err)
			}
			defer src.Close()
			if err := runImport(src, target.url, false, false, true); err != nil {
				t.Fatalf("import: %v", err)
			}
			lookups.Invalidate(db)

			gen, err := ticketnumber.Resolve(c.generator, "10", nil)
			if err != nil {
				t.Fatal(err)
			}
			repository.SetTicketNumberGenerator(gen, ticketnumber.NewDBStore(db, "10"))
			t.Cleanup(func() { repository.SetTicketNumberGenerator(nil, nil) })

			stateID, err := lookups.ID(ctx, db, lookups.StateLookup, lookups.StateNew)
			if err != nil {
				t.Fatal(err)
			}
			lockID, err := lookups.ID(ctx, db, lookups.LockType, lookups.LockUnlock)
			if err != nil {
				t.Fatal(err)
			}
			priorityID, err := lookups.ID(ctx, db, lookups.PriorityTable, "3 normal")
			if err != nil {
				t.Fatal(err)
			}
			var queueID int
			if err := db.QueryRow(database.ConvertPlaceholders("SELECT id FROM queue WHERE name = ?"), "Support").Scan(&queueID); err != nil {
				t.Fatal(err)
			}
			owner := 1
			last := c.lastCounter
			for i := 1; i <= 2; i++ {
				tk := models.Ticket{Title: fmt.Sprintf("after import %d", i), QueueID: queueID, TicketLockID: lockID,
					TicketStateID: stateID, TicketPriorityID: priorityID, UserID: &owner, ResponsibleUserID: &owner, CreateBy: 1, ChangeBy: 1}
				if err := repository.NewTicketRepository(db).Create(&tk); err != nil {
					t.Fatalf("create ticket %d after importing %v: %v", i, c.tns, err)
				}
				if containsString(c.tns, tk.TicketNumber) {
					t.Fatalf("new ticket number %s reuses an imported one", tk.TicketNumber)
				}
				// A counter the target already had may be higher than OTRS's; it only has to move on.
				if got := c.counter(tk.TicketNumber); got <= last {
					t.Errorf("new ticket %s has counter %d, want more than %d (OTRS's last counter %d)", tk.TicketNumber, got, last, c.lastCounter)
				} else {
					last = got
				}
			}
		})
	}
}
