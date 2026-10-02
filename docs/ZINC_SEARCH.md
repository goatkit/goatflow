# Search Backends (Database, Zinc, Elasticsearch)

GoatFlow's search API (`POST /api/v1/search`) runs on one backend, chosen with
`SEARCH_BACKEND`. Code: `internal/api/search_handler.go` and
`internal/platform/search/`.

| Backend | `SEARCH_BACKEND` | How it searches |
|---------|------------------|-----------------|
| `database` | unset or `database` (default) | Reads the live `ticket`, `article_data_mime` and `customer_user` rows on MySQL/MariaDB or PostgreSQL. Every word must occur as a substring (`LIKE`). No index. |
| `zinc` | `zinc` | Searches a Zinc index through Zinc's Elasticsearch-compatible API (`<ZINC_ENDPOINT>/es`). |
| `elasticsearch` | `elasticsearch` | Searches an Elasticsearch index. |

Any other value, or `zinc`/`elasticsearch` without its endpoint, is a
configuration error: search, reindex and health answer 503 "Search backend is
misconfigured: ..." instead of falling back to another backend.

## What is searched

All backends search the same things and return the same hit format:

- **ticket**: ticket number, title, and the subjects and bodies of the ticket's articles
- **article**: subject, body and sender
- **customer**: login, email, first and last name

With Zinc and Elasticsearch every word of the query must match a whole word
(token) in one of those fields, and hits are ranked by the engine's relevance
score (ticket number and title, article subject, customer login and email
weigh more). The database backend matches substrings instead.

## Permissions

Ticket and article hits are limited to the queues the caller may read (the
same queue access the ticket list uses; admins see all queues). Customers are
not queue-restricted. Customer accounts cannot use the search API.

With Zinc and Elasticsearch the queue filter is applied in the index query,
and every ticket and article hit is then checked against the database before
it is returned: hits whose ticket was moved to a queue the caller cannot read,
or whose row was deleted, are dropped even if the index has not caught up yet.
Documents of deleted rows found this way are removed from the index.

## Keeping the index in sync

The runner (`goats -mode runner`) has a `search-index` task that runs every
30 seconds when `SEARCH_BACKEND` is `zinc` or `elasticsearch`:

- **First run** (no index yet, or an index built by an older GoatFlow version):
  creates the indices and indexes every ticket, article and customer user.
  Until that has finished, searches answer 503 "The ... search index has not
  been built yet" rather than returning partial results.
- **Every run**: reindexes tickets whose `ticket` or `article` row changed
  (by `change_time`) or that got new `ticket_history` entries (queue moves,
  state and title changes, ...), and customer users whose `customer_user`
  row changed. Changes are visible to searches within about a minute.
- **Every 10 minutes**: compares document counts with the database and removes
  documents of deleted tickets, articles and customer users.
- **After an outage**: the task looks back 26 hours. If the index was not
  synced for longer than 24 hours (runner stopped, service down), the next run
  rebuilds it completely.

Migration `000034_search_sync_indexes` adds `change_time` indexes on
`ticket`, `article` and `customer_user` so these checks stay cheap.

The backend and the runner must have the same `SEARCH_*`, `ZINC_*` or
`ELASTICSEARCH_*` settings. In Helm, set them in `backend.extraEnv` and
`runner.extraEnv`.

### Indices

GoatFlow creates four indices, named `<SEARCH_INDEX_PREFIX><type>`:
`goatflow_ticket`, `goatflow_article`, `goatflow_customer` and
`goatflow_meta` (build state). Give each GoatFlow installation that shares a
search cluster its own prefix. Deleting the indices is safe: the next runner
run rebuilds them.

## Reindex

`POST /api/v1/search/reindex` (admins only; others get 403) starts a full
rebuild in the background and answers `202 Accepted`. A second request while
one is running answers `409`. Progress and the last error are reported by
`GET /api/v1/search/health` under `reindex`. The index stays searchable during
a rebuild once it has been built before. With the database backend the call
answers 200 "Database backend does not require reindexing".

## When the service is down

Searches never return an empty result in place of an error:

| Situation | `POST /api/v1/search` | `GET /api/v1/search/health` |
|-----------|-----------------------|-----------------------------|
| Service unreachable or answering 5xx | 503 `Search backend zinc is unavailable: service unreachable` (or `service error`) | 503, `status: unhealthy` |
| Wrong credentials | 503 `... unavailable: authentication failed` | 503, `status: unhealthy` |
| Index not built yet | 503 `The zinc search index has not been built yet. ...` | 503, `status: indexing` |
| Healthy | 200 with hits | 200, `status: healthy`, index `last_sync` and document counts |

The runner logs failed sync runs (`search-index: zinc sync failed: ...`) and
retries every 30 seconds.

## Environment variables

| Variable | Meaning |
|----------|---------|
| `SEARCH_BACKEND` | `database` (default), `zinc` or `elasticsearch` |
| `SEARCH_INDEX_PREFIX` | Index name prefix, default `goatflow_` |
| `ZINC_ENDPOINT` | Zinc base URL, e.g. `http://zinc:4080` (GoatFlow adds `/es`) |
| `ZINC_USER` | Zinc user (also creates Zinc's first admin in Docker Compose) |
| `ZINC_PASSWORD` | Zinc password |
| `ELASTICSEARCH_ENDPOINT` | Elasticsearch base URL, e.g. `https://es:9200` |
| `ELASTICSEARCH_USERNAME` | Elasticsearch user (empty: no authentication) |
| `ELASTICSEARCH_PASSWORD` | Elasticsearch password |

HTTPS endpoints must present a certificate the system trust store accepts.

## Docker Compose

The `zinc` service is in the `search` profile:

```bash
# .env: SEARCH_BACKEND=zinc, ZINC_ENDPOINT=http://zinc:4080, ZINC_USER, ZINC_PASSWORD
docker compose --profile search up -d zinc
docker compose up -d backend runner
```

## Endpoints

| Method | Path | Access | Purpose |
|--------|------|--------|---------|
| `POST` | `/api/v1/search` | Agent | Search |
| `POST` | `/api/v1/search/reindex` | Admin | Start a full rebuild (Zinc/Elasticsearch); nothing to do for `database` |
| `GET` | `/api/v1/search/health` | Agent | Backend health, index state, reindex progress |

## Tests

`make test-search-integration` runs `TestExternalBackendIntegration` against
real Zinc and Elasticsearch containers (testcontainers) and the test database.
