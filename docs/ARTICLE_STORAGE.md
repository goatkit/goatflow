# Article Storage

GoatFlow stores article attachments (and raw inbound emails) the way OTRS/Znuny
does, with two interchangeable backends:

| Backend | `storage.type` | Where content lives |
|---------|----------------|---------------------|
| ArticleStorageDB (default) | `db` | `article_data_mime_attachment` and `article_data_mime_plain` |
| ArticleStorageFS | `fs` | `<storage root>/var/article/<YYYY/MM/DD>/<article_id>/` |

The article body itself always stays in `article_data_mime.a_body`, for both
backends, exactly as in OTRS.

## Configuration

```yaml
storage:
    type: db            # or fs
    local:
        path: /app/storage
```

Environment variables override the file: `STORAGE_TYPE` (`db`/`fs`) and
`STORAGE_PATH` (storage root). The FS tree is always `<storage root>/var/article`.
GoatFlow refuses to start with an unknown backend.

## Filesystem layout (ArticleStorageFS)

The layout is OTRS's own, so an existing OTRS/Znuny `var/article` tree can be
mounted at `<storage root>/var/article` and is read as-is:

```
var/article/2014/03/05/1234/        <- article_data_mime.content_path / article id
    Report.pdf                      <- attachment content
    Report.pdf.content_type         <- e.g. application/pdf
    Report.pdf.disposition          <- inline / attachment (optional)
    image001.png.content_id         <- <image001.png@01D...> (optional)
    image001.png.content_alternative
    plain.txt                       <- raw email
```

- The date directory comes from `article_data_mime.content_path` (written by
  OTRS and by GoatFlow when an article is created); when it is empty GoatFlow
  uses the article's `create_time`.
- Files without a `.content_type` sidecar are read in the pre-sidecar OTRS
  format (content type on the first line).
- Attachments are numbered like OTRS: file id = position in the sorted file
  list (dot files, sidecars and `plain.txt` excluded). In the DB backend the
  file id is the `article_data_mime_attachment.id`. Attachment URLs therefore
  always carry the article id:
  `/api/tickets/<ticket>/articles/<article_id>/attachments/<file_id>`
  (customer portal: `/customer/tickets/<ticket>/articles/<article_id>/attachments/<file_id>`).
- A filename an article already uses gets the OTRS `-1`, `-2`, … suffix.
  Attachment names are made safe for the filesystem (no path separators,
  control characters or leading dots).
- File writes are not part of the database transaction: if an article insert
  rolls back after its attachment was written, the file stays on disk.

## Switching backends

`goatflow-storage` copies content between the backends. It ships in the backend
image and the toolbox, and reads the same `DB_*` / `DB_DRIVER` and
`STORAGE_PATH` environment as the application.

```bash
# What each backend holds
docker compose exec backend ./goatflow-storage status

# 1. Copy everything into the target (repeatable: items already there are skipped)
docker compose exec backend ./goatflow-storage migrate -target FS
# 2. Check the target holds a byte-identical copy of every item (exit 1 if not)
docker compose exec backend ./goatflow-storage verify -target FS
# 3. Set storage.type: fs (or STORAGE_TYPE=fs) and restart GoatFlow
# 4. Optionally free the old copies; each article's source is deleted only
#    after the target holds all of it
docker compose exec backend ./goatflow-storage migrate -target FS -delete-source
```

Options: `-dry-run`, `-tolerant` (continue after an error), `-sleep-ms` (pause
between articles), `-closed-before` / `-created-after` (limit to tickets; date
or RFC 3339 time), `-article-dir` (FS root, default `$STORAGE_PATH/var/article`),
`-verbose`. The tool keeps no state of its own: re-running a migration resumes
it, and `verify` is the progress report.

## Adding a backend

Every caller uses the `storage.ArticleStore` interface
(`internal/storage/backend.go`) obtained from `storage.ForDB(db)`. A new
backend (for example S3-compatible object storage) implements that interface,
gets a `storage.type` value in `storage.Configure`/`storage.New`, and works
with every request path and with `goatflow-storage` without touching callers.
