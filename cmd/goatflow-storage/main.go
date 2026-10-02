// Package main is goatflow-storage: inspect, copy and verify article
// attachments and raw emails between the database (ArticleStorageDB) and an
// OTRS-layout filesystem tree (ArticleStorageFS).
//
// Switching backends: run "migrate -target FS" (repeatable; it skips what is
// already copied), run "verify -target FS", set storage.type (or STORAGE_TYPE)
// to fs and restart, then optionally re-run migrate with -delete-source.
package main

import (
	"context"
	"database/sql"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	_ "github.com/go-sql-driver/mysql"
	_ "github.com/lib/pq"

	"github.com/goatkit/goatflow/internal/platform/database"
	"github.com/goatkit/goatflow/internal/platform/dbconfig"
	"github.com/goatkit/goatflow/internal/storage"
)

const usage = `Usage: goatflow-storage <status|migrate|verify> [options]

Commands:
  status   Count attachments and raw emails held by each backend
  migrate  Copy every article's attachments and raw email into -target from
           the other backend; items already in the target are skipped
  verify   Report items held by the other backend that -target lacks
           (exit status 1 when anything is missing)

Options:
`

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 || strings.HasPrefix(args[0], "-") {
		fmt.Fprint(stderr, usage)
		return 2
	}
	command := args[0]

	defaultPort := "5432"
	if database.IsMySQL() {
		defaultPort = "3306"
	}
	fset := flag.NewFlagSet("goatflow-storage", flag.ContinueOnError)
	fset.SetOutput(stderr)
	fset.Usage = func() {
		fmt.Fprint(stderr, usage)
		fset.PrintDefaults()
	}
	var (
		dbHost       = fset.String("db-host", dbconfig.EnvDefault("HOST", "localhost"), "database host")
		dbPort       = fset.String("db-port", dbconfig.EnvDefault("PORT", defaultPort), "database port")
		dbName       = fset.String("db-name", dbconfig.EnvDefault("NAME", "goatflow"), "database name")
		dbUser       = fset.String("db-user", dbconfig.EnvDefault("USER", "goatflow"), "database user")
		dbPassword   = fset.String("db-password", dbconfig.Env("PASSWORD"), "database password")
		articleDir   = fset.String("article-dir", storage.ConfigFromApp(nil).ArticleDir, "ArticleStorageFS root (OTRS var/article)")
		target       = fset.String("target", "", "target backend for migrate/verify: DB or FS")
		deleteSource = fset.Bool("delete-source", false, "migrate: remove an article's source copy once the target holds all of it")
		dryRun       = fset.Bool("dry-run", false, "migrate: report what would be copied without writing")
		tolerant     = fset.Bool("tolerant", false, "migrate: continue with the next article after an error")
		sleepMs      = fset.Int("sleep-ms", 0, "migrate: pause between articles, in milliseconds")
		closedBefore = fset.String("closed-before", "", "only tickets in a closed state last changed before this date (YYYY-MM-DD or RFC 3339)")
		createdAfter = fset.String("created-after", "", "only tickets created after this date (YYYY-MM-DD or RFC 3339)")
		verbose      = fset.Bool("verbose", false, "print every copied item")
	)
	if err := fset.Parse(args[1:]); err != nil {
		return 2
	}

	opts := migrateOptions{
		DeleteSource: *deleteSource,
		DryRun:       *dryRun,
		Tolerant:     *tolerant,
		Sleep:        time.Duration(*sleepMs) * time.Millisecond,
		Out:          stdout,
		Verbose:      *verbose,
	}
	var err error
	if opts.ClosedBefore, err = parseDate(*closedBefore); err != nil {
		fmt.Fprintf(stderr, "-closed-before: %v\n", err)
		return 2
	}
	if opts.CreatedAfter, err = parseDate(*createdAfter); err != nil {
		fmt.Fprintf(stderr, "-created-after: %v\n", err)
		return 2
	}

	switch command {
	case "status":
	case "migrate", "verify":
		if t := strings.ToUpper(*target); t != storage.BackendDB && t != storage.BackendFS {
			fmt.Fprintf(stderr, "-target must be DB or FS, got %q\n", *target)
			return 2
		}
	default:
		fmt.Fprintf(stderr, "unknown command %q\n\n", command)
		fset.Usage()
		return 2
	}

	db, err := openDB(*dbHost, *dbPort, *dbUser, *dbPassword, *dbName)
	if err != nil {
		fmt.Fprintf(stderr, "database: %v\n", err)
		return 1
	}
	defer db.Close()

	ctx := context.Background()
	switch command {
	case "status":
		err = showStatus(ctx, db, *articleDir, stdout)
	case "migrate", "verify":
		src, dst := backends(db, *target, *articleDir)
		var rep report
		if command == "migrate" {
			rep, err = migrate(ctx, db, src, dst, opts)
			printReport(stdout, command, src, dst, rep)
			if err == nil && rep.Failed == 0 && !opts.DryRun {
				fmt.Fprintf(stdout, "Run \"verify -target %s\", then set storage.type (or STORAGE_TYPE) to %s and restart GoatFlow.\n",
					dst.Backend(), strings.ToLower(dst.Backend()))
			}
		} else {
			rep, err = verify(ctx, db, src, dst, opts)
			printReport(stdout, command, src, dst, rep)
		}
		if err == nil && (rep.Failed > 0 || rep.Missing > 0) {
			err = errors.New("not every item is in the target")
		}
	}
	if err != nil {
		fmt.Fprintf(stderr, "%s: %v\n", command, err)
		return 1
	}
	return 0
}

func parseDate(s string) (time.Time, error) {
	if s == "" {
		return time.Time{}, nil
	}
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return t, nil
	}
	return time.Parse("2006-01-02", s)
}

// openDB connects with the driver selected by DB_DRIVER, which also decides
// how database.ConvertPlaceholders rewrites the SQL.
func openDB(host, port, user, password, name string) (*sql.DB, error) {
	var db *sql.DB
	var err error
	switch {
	case database.IsMySQL():
		db, err = sql.Open("mysql", fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?parseTime=true", user, password, host, port, name))
	case database.IsPostgreSQL():
		db, err = sql.Open("postgres", fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=%s",
			host, port, user, password, name, dbconfig.EnvDefault("SSLMODE", "disable")))
	default:
		return nil, fmt.Errorf("unsupported DB_DRIVER %q (want mysql, mariadb or postgres)", database.GetDBDriver())
	}
	if err != nil {
		return nil, err
	}
	if err := db.Ping(); err != nil {
		db.Close()
		return nil, err
	}
	return db, nil
}

// backends returns (source, target) for a validated target name.
func backends(db *sql.DB, target, articleDir string) (src, dst storage.ArticleStore) {
	dbStore := storage.NewDatabaseStore(db)
	fsStore := storage.NewFilesystemStore(articleDir, db)
	if strings.EqualFold(target, storage.BackendFS) {
		return dbStore, fsStore
	}
	return fsStore, dbStore
}

func showStatus(ctx context.Context, db *sql.DB, articleDir string, out io.Writer) error {
	dbFiles, dbSize, err := storage.NewDatabaseStore(db).Stats(ctx)
	if err != nil {
		return err
	}
	fsFiles, fsSize, err := storage.NewFilesystemStore(articleDir, db).Stats()
	if err != nil {
		return err
	}
	fmt.Fprintf(out, "Database (ArticleStorageDB):   %d files, %s\n", dbFiles, formatBytes(dbSize))
	fmt.Fprintf(out, "Filesystem (ArticleStorageFS): %d files, %s in %s\n", fsFiles, formatBytes(fsSize), articleDir)
	return nil
}

func printReport(out io.Writer, command string, src, dst storage.ArticleStore, rep report) {
	fmt.Fprintf(out, "%s %s -> %s: %d articles", command, src.Backend(), dst.Backend(), rep.Articles)
	if command == "migrate" {
		fmt.Fprintf(out, ", %d items copied, %d already present, %d source copies deleted, %d failed\n",
			rep.Copied, rep.AlreadyThere, rep.SourceDeleted, rep.Failed)
		return
	}
	fmt.Fprintf(out, ", %d items present, %d missing, %d failed\n", rep.AlreadyThere, rep.Missing, rep.Failed)
}

func formatBytes(n int64) string {
	const unit = 1024
	if n < unit {
		return fmt.Sprintf("%d B", n)
	}
	div, exp := int64(unit), 0
	for m := n / unit; m >= unit; m /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(n)/float64(div), "KMGTPE"[exp])
}
