# Search Backends (Database, Zinc, Elasticsearch)

GoatFlow's search API (`POST /api/v1/search`) runs on one primary backend.
Code: `internal/api/search_handler.go` and `internal/platform/search/`.

## Backends

| Backend | When it is used | Notes |
|---------|-----------------|-------|
| `database` | Default. Used when no other backend is primary. | Searches the live `ticket`, `article` and `customer_user` data on MySQL/MariaDB or PostgreSQL. No index, no reindex. |
| `zinc` | `ZINC_ENDPOINT` set **and** `SEARCH_BACKEND=zinc` | Elasticsearch-compatible client. |
| `elasticsearch` | `ELASTICSEARCH_ENDPOINT` set **and** `SEARCH_BACKEND=elasticsearch` | Same client as Zinc. |

If an endpoint is set but `SEARCH_BACKEND` does not name it, the backend is
registered but not used for searches.

## Limits of the Zinc and Elasticsearch backends

GoatFlow does not write tickets or articles into Zinc or Elasticsearch.
`POST /api/v1/search/reindex` (admin only) answers 501 "Reindexing is not
supported for this search backend" for them. A search against an external
backend only finds documents you put into its index yourself.

For normal use, keep the default `database` backend.

## Environment variables

| Variable | Meaning |
|----------|---------|
| `SEARCH_BACKEND` | `zinc` or `elasticsearch` to make that backend primary. Unset = `database`. |
| `ZINC_ENDPOINT` | Zinc URL, e.g. `http://zinc:4080` |
| `ZINC_USERNAME` | Zinc user |
| `ZINC_PASSWORD` | Zinc password |
| `ELASTICSEARCH_ENDPOINT` | Elasticsearch URL |
| `ELASTICSEARCH_USERNAME` | Elasticsearch user |
| `ELASTICSEARCH_PASSWORD` | Elasticsearch password |

## Docker Compose

The `zinc` service in `docker-compose.yml` is commented out.

## Endpoints

| Method | Path | Access | Purpose |
|--------|------|--------|---------|
| `POST` | `/api/v1/search` | Agent | Search |
| `POST` | `/api/v1/search/reindex` | Admin | Reindex (no-op for `database`, 501 for others) |
| `GET` | `/api/v1/search/health` | Agent | Backend health |
