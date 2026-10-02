package main

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/goatkit/goatflow/internal/platform/database"
	"github.com/goatkit/goatflow/internal/storage"
)

// migrateOptions controls a backend-to-backend copy.
type migrateOptions struct {
	DeleteSource bool      // remove the source copy once the target holds all of it
	DryRun       bool      // report what would be copied, change nothing
	Tolerant     bool      // continue with the next article after an error
	ClosedBefore time.Time // only tickets in a closed state changed before this time
	CreatedAfter time.Time // only tickets created after this time
	Sleep        time.Duration
	Out          io.Writer
	Verbose      bool
}

// report counts what a migrate or verify run saw.
type report struct {
	Articles      int // articles examined
	Copied        int // attachments and plain emails written to the target
	AlreadyThere  int // items the target already held
	Missing       int // verify: items absent from the target
	SourceDeleted int // articles whose source copy was removed
	Failed        int // articles that hit an error
}

// item is one attachment or the plain email of an article.
type item struct {
	plain      bool
	attachment storage.Attachment
	content    []byte
}

func (it item) sameAs(o item) bool {
	if it.plain || o.plain {
		return it.plain == o.plain && bytes.Equal(it.content, o.content)
	}
	return it.attachment.ContentType == o.attachment.ContentType &&
		it.attachment.ContentID == o.attachment.ContentID &&
		bytes.Equal(it.content, o.content)
}

func (it item) String() string {
	if it.plain {
		return "plain email"
	}
	return fmt.Sprintf("attachment %q", it.attachment.Filename)
}

// loadItems reads every attachment and the plain email of an article.
func loadItems(ctx context.Context, s storage.ArticleStore, articleID int64) ([]item, error) {
	list, err := s.ListAttachments(ctx, articleID)
	if err != nil {
		return nil, err
	}
	items := make([]item, 0, len(list)+1)
	for _, a := range list {
		meta, content, err := s.GetAttachment(ctx, articleID, a.FileID)
		if err != nil {
			return nil, err
		}
		items = append(items, item{attachment: meta, content: content})
	}
	raw, err := s.ReadPlain(ctx, articleID)
	switch {
	case err == nil:
		items = append(items, item{plain: true, content: raw})
	case !errors.Is(err, storage.ErrNotFound):
		return nil, err
	}
	return items, nil
}

// missingItems returns the source items the target does not hold, matching
// each target item at most once (an article may carry identical attachments).
func missingItems(src, dst []item) []item {
	used := make([]bool, len(dst))
	var missing []item
	for _, s := range src {
		found := false
		for i, d := range dst {
			if !used[i] && s.sameAs(d) {
				used[i], found = true, true
				break
			}
		}
		if !found {
			missing = append(missing, s)
		}
	}
	return missing
}

// selectArticles returns the ids of the articles to process, oldest first.
func selectArticles(ctx context.Context, db *sql.DB, opts migrateOptions) ([]int64, error) {
	query := `
		SELECT a.id
		FROM article a
		JOIN ticket t ON t.id = a.ticket_id
		JOIN ticket_state ts ON ts.id = t.ticket_state_id
		JOIN ticket_state_type tst ON tst.id = ts.type_id
		WHERE 1 = 1`
	var args []interface{}
	if !opts.ClosedBefore.IsZero() {
		query += " AND tst.name = 'closed' AND t.change_time < ?"
		args = append(args, opts.ClosedBefore)
	}
	if !opts.CreatedAfter.IsZero() {
		query += " AND t.create_time > ?"
		args = append(args, opts.CreatedAfter)
	}
	query += " ORDER BY a.id"

	rows, err := db.QueryContext(ctx, database.ConvertPlaceholders(query), args...)
	if err != nil {
		return nil, fmt.Errorf("select articles: %w", err)
	}
	defer rows.Close()
	var ids []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("select articles: %w", err)
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

// migrate copies every selected article's attachments and plain email from
// src to dst. Items dst already holds are skipped, so a run can be repeated or
// resumed after an interruption. With DeleteSource the source copy of an
// article is removed only after dst holds all of it.
func migrate(ctx context.Context, db *sql.DB, src, dst storage.ArticleStore, opts migrateOptions) (report, error) {
	var rep report
	ids, err := selectArticles(ctx, db, opts)
	if err != nil {
		return rep, err
	}
	for _, id := range ids {
		rep.Articles++
		if err := migrateArticle(ctx, src, dst, id, opts, &rep); err != nil {
			rep.Failed++
			fmt.Fprintf(opts.Out, "article %d: %v\n", id, err)
			if !opts.Tolerant {
				return rep, fmt.Errorf("article %d: %w", id, err)
			}
		}
		if opts.Sleep > 0 {
			time.Sleep(opts.Sleep)
		}
	}
	return rep, nil
}

func migrateArticle(ctx context.Context, src, dst storage.ArticleStore, id int64, opts migrateOptions, rep *report) error {
	srcItems, err := loadItems(ctx, src, id)
	if err != nil {
		return fmt.Errorf("read %s: %w", src.Backend(), err)
	}
	if len(srcItems) == 0 {
		return nil
	}
	dstItems, err := loadItems(ctx, dst, id)
	if err != nil {
		return fmt.Errorf("read %s: %w", dst.Backend(), err)
	}
	missing := missingItems(srcItems, dstItems)
	rep.AlreadyThere += len(srcItems) - len(missing)

	for _, it := range missing {
		if opts.Verbose || opts.DryRun {
			fmt.Fprintf(opts.Out, "article %d: copy %s to %s\n", id, it, dst.Backend())
		}
		if opts.DryRun {
			rep.Copied++
			continue
		}
		if it.plain {
			err = dst.WritePlain(ctx, id, it.content, 0)
		} else {
			a := it.attachment
			_, err = dst.WriteAttachment(ctx, id, storage.NewAttachment{
				Filename: a.Filename, ContentType: a.ContentType, ContentID: a.ContentID,
				ContentAlternative: a.ContentAlternative, Disposition: a.Disposition, Content: it.content,
			})
		}
		if err != nil {
			return fmt.Errorf("write %s to %s: %w", it, dst.Backend(), err)
		}
		rep.Copied++
	}

	if !opts.DeleteSource || opts.DryRun {
		return nil
	}
	dstItems, err = loadItems(ctx, dst, id)
	if err != nil {
		return fmt.Errorf("re-read %s: %w", dst.Backend(), err)
	}
	if left := missingItems(srcItems, dstItems); len(left) > 0 {
		return fmt.Errorf("%s still lacks %d item(s); source kept", dst.Backend(), len(left))
	}
	if err := src.DeleteArticle(ctx, id); err != nil {
		return fmt.Errorf("delete source copy: %w", err)
	}
	rep.SourceDeleted++
	return nil
}

// verify reports every selected article item held by src that dst lacks.
func verify(ctx context.Context, db *sql.DB, src, dst storage.ArticleStore, opts migrateOptions) (report, error) {
	var rep report
	ids, err := selectArticles(ctx, db, opts)
	if err != nil {
		return rep, err
	}
	for _, id := range ids {
		rep.Articles++
		srcItems, err := loadItems(ctx, src, id)
		if err == nil {
			var dstItems []item
			if dstItems, err = loadItems(ctx, dst, id); err == nil {
				missing := missingItems(srcItems, dstItems)
				rep.AlreadyThere += len(srcItems) - len(missing)
				rep.Missing += len(missing)
				for _, it := range missing {
					fmt.Fprintf(opts.Out, "article %d: %s missing in %s\n", id, it, dst.Backend())
				}
			}
		}
		if err != nil {
			rep.Failed++
			fmt.Fprintf(opts.Out, "article %d: %v\n", id, err)
		}
	}
	return rep, nil
}
