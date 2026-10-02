package webhook

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/goatkit/goatflow/internal/platform/database"
	"github.com/goatkit/goatflow/internal/platform/secureconfig"
)

const (
	minSecretLength = 16
	maxSecretLength = 512
)

// ValidateSecret checks a signing secret supplied by an administrator.
// An empty secret means "unsigned".
func ValidateSecret(secret string) error {
	if secret == "" {
		return nil
	}
	if len(secret) < minSecretLength || len(secret) > maxSecretLength {
		return invalid("secret must be between %d and %d characters", minSecretLength, maxSecretLength)
	}
	return nil
}

// Repository persists webhooks, deliveries and event-source cursors.
type Repository struct {
	db *sql.DB
}

// NewRepository returns a repository on db.
func NewRepository(db *sql.DB) *Repository {
	return &Repository{db: db}
}

// now returns the current time at second precision: DATETIME columns drop
// fractions (MySQL rounds them), so comparisons must use whole seconds.
func now() time.Time {
	return time.Now().UTC().Truncate(time.Second)
}

const webhookColumns = `id, name, url, secret_encrypted, secret_hint, events, headers,
	retry_count, timeout_seconds, valid_id, create_time, create_by, change_time, change_by`

type rowScanner interface {
	Scan(dest ...interface{}) error
}

// scanWebhook reads one gk_webhook row; the encrypted secret is returned
// separately so it never travels inside the JSON-serialisable struct.
func scanWebhook(s rowScanner) (*Webhook, []byte, error) {
	var (
		w          Webhook
		secret     []byte
		hint       sql.NullString
		eventsJSON string
		headers    sql.NullString
		validID    int
	)
	if err := s.Scan(&w.ID, &w.Name, &w.URL, &secret, &hint, &eventsJSON, &headers,
		&w.RetryCount, &w.TimeoutSeconds, &validID, &w.CreatedAt, &w.CreatedBy, &w.UpdatedAt, &w.UpdatedBy); err != nil {
		return nil, nil, err
	}
	if err := json.Unmarshal([]byte(eventsJSON), &w.Events); err != nil {
		return nil, nil, fmt.Errorf("webhook %d: corrupt events column: %w", w.ID, err)
	}
	w.Headers = map[string]string{}
	if headers.Valid && headers.String != "" {
		if err := json.Unmarshal([]byte(headers.String), &w.Headers); err != nil {
			return nil, nil, fmt.Errorf("webhook %d: corrupt headers column: %w", w.ID, err)
		}
	}
	w.HasSecret = len(secret) > 0
	if w.HasSecret {
		w.SecretHint = secureconfig.MaskedDisplay(hint.String)
	}
	w.IsActive = validID == validActive
	return &w, secret, nil
}

func encryptSecret(secret string) ([]byte, interface{}, error) {
	if secret == "" {
		return nil, nil, nil
	}
	key, err := secureconfig.GetKey()
	if err != nil {
		return nil, nil, fmt.Errorf("secure config key: %w", err)
	}
	enc, err := secureconfig.Encrypt([]byte(secret), key)
	if err != nil {
		return nil, nil, err
	}
	return enc, secureconfig.ValueHint(secret), nil
}

func decryptSecret(enc []byte) (string, error) {
	if len(enc) == 0 {
		return "", nil
	}
	key, err := secureconfig.GetKey()
	if err != nil {
		return "", fmt.Errorf("secure config key: %w", err)
	}
	plain, err := secureconfig.Decrypt(enc, key)
	if err != nil {
		return "", fmt.Errorf("cannot decrypt webhook secret (is %s the same for every GoatFlow process?): %w",
			secureconfig.KeyEnvVar, err)
	}
	return string(plain), nil
}

func validID(active bool) int {
	if active {
		return validActive
	}
	return validInactive
}

func (r *Repository) nameTaken(ctx context.Context, name string, exceptID int64) (bool, error) {
	var id int64
	err := r.db.QueryRowContext(ctx, database.ConvertPlaceholders(
		`SELECT id FROM gk_webhook WHERE name = ? AND id <> ?`), name, exceptID).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	return err == nil, err
}

// Create stores a new webhook (already normalised) and returns it.
func (r *Repository) Create(ctx context.Context, w *Webhook, secret string, userID int) (*Webhook, error) {
	if taken, err := r.nameTaken(ctx, w.Name, 0); err != nil {
		return nil, err
	} else if taken {
		return nil, ErrDuplicateName
	}
	enc, hint, err := encryptSecret(secret)
	if err != nil {
		return nil, err
	}
	events, err := json.Marshal(w.Events)
	if err != nil {
		return nil, err
	}
	headers, err := json.Marshal(w.Headers)
	if err != nil {
		return nil, err
	}
	ts := now()
	id, err := database.GetAdapter().InsertWithReturning(r.db, database.ConvertPlaceholders(`
		INSERT INTO gk_webhook (name, url, secret_encrypted, secret_hint, events, headers,
			retry_count, timeout_seconds, valid_id, create_time, create_by, change_time, change_by)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?) RETURNING id`),
		w.Name, w.URL, enc, hint, string(events), string(headers),
		w.RetryCount, w.TimeoutSeconds, validID(w.IsActive), ts, userID, ts, userID)
	if err != nil {
		return nil, err
	}
	return r.Get(ctx, id)
}

// Get returns one webhook or ErrNotFound.
func (r *Repository) Get(ctx context.Context, id int64) (*Webhook, error) {
	w, _, err := r.getWithSecret(ctx, id)
	return w, err
}

func (r *Repository) getWithSecret(ctx context.Context, id int64) (*Webhook, []byte, error) {
	row := r.db.QueryRowContext(ctx, database.ConvertPlaceholders(
		`SELECT `+webhookColumns+` FROM gk_webhook WHERE id = ?`), id)
	w, secret, err := scanWebhook(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil, ErrNotFound
	}
	return w, secret, err
}

// List returns all webhooks ordered by name; active filters by state when set.
func (r *Repository) List(ctx context.Context, active *bool) ([]*Webhook, error) {
	query := `SELECT ` + webhookColumns + ` FROM gk_webhook`
	var args []interface{}
	if active != nil {
		query += ` WHERE valid_id = ?`
		args = append(args, validID(*active))
	}
	query += ` ORDER BY name`
	rows, err := r.db.QueryContext(ctx, database.ConvertPlaceholders(query), args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []*Webhook{}
	for rows.Next() {
		w, _, err := scanWebhook(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, w)
	}
	return out, rows.Err()
}

// SecretChange tells Update what to do with the stored secret.
type SecretChange struct {
	Set   bool   // replace the secret with Value ("" removes it)
	Value string // new secret when Set
}

// Update stores a modified (already normalised) webhook.
func (r *Repository) Update(ctx context.Context, w *Webhook, secret SecretChange, userID int) (*Webhook, error) {
	if taken, err := r.nameTaken(ctx, w.Name, w.ID); err != nil {
		return nil, err
	} else if taken {
		return nil, ErrDuplicateName
	}
	events, err := json.Marshal(w.Events)
	if err != nil {
		return nil, err
	}
	headers, err := json.Marshal(w.Headers)
	if err != nil {
		return nil, err
	}
	query := `UPDATE gk_webhook SET name = ?, url = ?, events = ?, headers = ?, retry_count = ?,
		timeout_seconds = ?, valid_id = ?, change_time = ?, change_by = ?`
	args := []interface{}{w.Name, w.URL, string(events), string(headers), w.RetryCount,
		w.TimeoutSeconds, validID(w.IsActive), now(), userID}
	if secret.Set {
		enc, hint, err := encryptSecret(secret.Value)
		if err != nil {
			return nil, err
		}
		query += `, secret_encrypted = ?, secret_hint = ?`
		args = append(args, enc, hint)
	}
	query += ` WHERE id = ?`
	args = append(args, w.ID)
	res, err := r.db.ExecContext(ctx, database.ConvertPlaceholders(query), args...)
	if err != nil {
		return nil, err
	}
	if n, err := res.RowsAffected(); err == nil && n == 0 {
		return nil, ErrNotFound
	}
	return r.Get(ctx, w.ID)
}

// Delete removes a webhook and (by cascade) its delivery log.
func (r *Repository) Delete(ctx context.Context, id int64) error {
	res, err := r.db.ExecContext(ctx, database.ConvertPlaceholders(`DELETE FROM gk_webhook WHERE id = ?`), id)
	if err != nil {
		return err
	}
	if n, err := res.RowsAffected(); err == nil && n == 0 {
		return ErrNotFound
	}
	return nil
}

// insertDelivery records a delivery in the given status and returns its id.
func (r *Repository) insertDelivery(ctx context.Context, tx *sql.Tx, webhookID int64, event, payload, status string) (int64, error) {
	ts := now()
	query := database.ConvertPlaceholders(`
		INSERT INTO gk_webhook_delivery (webhook_id, event_type, payload, status, attempts,
			next_attempt_time, create_time, change_time)
		VALUES (?, ?, ?, ?, 0, ?, ?, ?) RETURNING id`)
	args := []interface{}{webhookID, event, payload, status, ts, ts, ts}
	if tx != nil {
		return database.GetAdapter().InsertWithReturningTx(tx, query, args...)
	}
	return database.GetAdapter().InsertWithReturning(r.db, query, args...)
}

const deliveryColumns = `id, webhook_id, event_type, payload, status, attempts, status_code,
	response_body, error_message, duration_ms, next_attempt_time, delivered_time, create_time, change_time`

func scanDelivery(s rowScanner) (*Delivery, error) {
	var (
		d          Delivery
		statusCode sql.NullInt64
		response   sql.NullString
		errMsg     sql.NullString
		duration   sql.NullInt64
		next       sql.NullTime
		delivered  sql.NullTime
	)
	if err := s.Scan(&d.ID, &d.WebhookID, &d.Event, &d.Payload, &d.Status, &d.Attempts, &statusCode,
		&response, &errMsg, &duration, &next, &delivered, &d.CreatedAt, &d.UpdatedAt); err != nil {
		return nil, err
	}
	if statusCode.Valid {
		v := int(statusCode.Int64)
		d.StatusCode = &v
	}
	if duration.Valid {
		v := int(duration.Int64)
		d.DurationMS = &v
	}
	d.Response = response.String
	d.Error = errMsg.String
	if next.Valid && d.Status == StatusPending {
		t := next.Time
		d.NextAttemptAt = &t
	}
	if delivered.Valid {
		t := delivered.Time
		d.DeliveredAt = &t
	}
	d.Success = d.Status == StatusDelivered
	return &d, nil
}

// GetDelivery returns one delivery including payload and response, or ErrNotFound.
func (r *Repository) GetDelivery(ctx context.Context, id int64) (*Delivery, error) {
	row := r.db.QueryRowContext(ctx, database.ConvertPlaceholders(
		`SELECT `+deliveryColumns+` FROM gk_webhook_delivery WHERE id = ?`), id)
	d, err := scanDelivery(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	return d, err
}

// ListDeliveries returns the newest deliveries of a webhook without payload
// and response bodies (fetch a single delivery for those).
func (r *Repository) ListDeliveries(ctx context.Context, webhookID int64, limit int) ([]*Delivery, error) {
	rows, err := r.db.QueryContext(ctx, database.ConvertPlaceholders(
		`SELECT `+deliveryColumns+` FROM gk_webhook_delivery WHERE webhook_id = ?
		ORDER BY id DESC LIMIT ?`), webhookID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []*Delivery{}
	for rows.Next() {
		d, err := scanDelivery(rows)
		if err != nil {
			return nil, err
		}
		d.Payload = ""
		d.Response = ""
		out = append(out, d)
	}
	return out, rows.Err()
}

// staleDeliveringAfter is how long a delivery may sit in "delivering" before
// another worker assumes the sender died and takes it over. It is well above
// MaxTimeoutSeconds so a live send is never duplicated.
const staleDeliveringAfter = 10 * time.Minute

// claimDue atomically moves up to limit due deliveries to "delivering" and
// returns their ids. Each row is claimed with a conditional UPDATE, so
// concurrent workers never send the same delivery twice.
func (r *Repository) claimDue(ctx context.Context, limit int) ([]int64, error) {
	ts := now()
	stale := ts.Add(-staleDeliveringAfter)
	rows, err := r.db.QueryContext(ctx, database.ConvertPlaceholders(`
		SELECT id FROM gk_webhook_delivery
		WHERE (status = ? AND next_attempt_time <= ?) OR (status = ? AND change_time < ?)
		ORDER BY id LIMIT ?`), StatusPending, ts, StatusDelivering, stale, limit)
	if err != nil {
		return nil, err
	}
	var candidates []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return nil, err
		}
		candidates = append(candidates, id)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}
	claimed := make([]int64, 0, len(candidates))
	for _, id := range candidates {
		res, err := r.db.ExecContext(ctx, database.ConvertPlaceholders(`
			UPDATE gk_webhook_delivery SET status = ?, change_time = ?
			WHERE id = ? AND ((status = ? AND next_attempt_time <= ?) OR (status = ? AND change_time < ?))`),
			StatusDelivering, ts, id, StatusPending, ts, StatusDelivering, stale)
		if err != nil {
			return claimed, err
		}
		if n, _ := res.RowsAffected(); n == 1 {
			claimed = append(claimed, id)
		}
	}
	return claimed, nil
}

// attemptResult is the outcome of one HTTP attempt.
type attemptResult struct {
	status     string
	statusCode *int
	response   string
	errMsg     string
	durationMS int
	retryAfter time.Duration // with status pending: wait before the next attempt
}

func (r *Repository) recordAttempt(ctx context.Context, id int64, res attemptResult) error {
	ts := now()
	var delivered interface{}
	if res.status == StatusDelivered {
		delivered = ts
	}
	var next interface{}
	if res.status == StatusPending {
		next = ts.Add(res.retryAfter)
	}
	var code interface{}
	if res.statusCode != nil {
		code = *res.statusCode
	}
	var errMsg interface{}
	if res.errMsg != "" {
		errMsg = truncate(res.errMsg, maxErrorLength)
	}
	_, err := r.db.ExecContext(ctx, database.ConvertPlaceholders(`
		UPDATE gk_webhook_delivery SET status = ?, attempts = attempts + 1, status_code = ?,
			response_body = ?, error_message = ?, duration_ms = ?, next_attempt_time = ?,
			delivered_time = ?, change_time = ?
		WHERE id = ?`),
		res.status, code, truncate(res.response, maxResponseBodyBytes), errMsg, res.durationMS, next, delivered, ts, id)
	return err
}

// failDelivery marks a delivery failed without an HTTP attempt.
func (r *Repository) failDelivery(ctx context.Context, id int64, reason string) error {
	_, err := r.db.ExecContext(ctx, database.ConvertPlaceholders(`
		UPDATE gk_webhook_delivery SET status = ?, error_message = ?, next_attempt_time = ?, change_time = ?
		WHERE id = ?`), StatusFailed, truncate(reason, maxErrorLength), nil, now(), id)
	return err
}

// Cursor returns the last processed id of an event source. When the source
// has no cursor yet it is created at start() so only later rows become events.
func (r *Repository) Cursor(ctx context.Context, source string, start func(context.Context) (int64, error)) (int64, error) {
	var last int64
	err := r.db.QueryRowContext(ctx, database.ConvertPlaceholders(
		`SELECT last_id FROM gk_webhook_event_cursor WHERE source = ?`), source).Scan(&last)
	if err == nil {
		return last, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return 0, err
	}
	initial, err := start(ctx)
	if err != nil {
		return 0, err
	}
	if _, err := r.db.ExecContext(ctx, database.ConvertPlaceholders(
		`INSERT INTO gk_webhook_event_cursor (source, last_id, change_time) VALUES (?, ?, ?)`),
		source, initial, now()); err != nil {
		// Another worker created it concurrently; use its value.
		if rerr := r.db.QueryRowContext(ctx, database.ConvertPlaceholders(
			`SELECT last_id FROM gk_webhook_event_cursor WHERE source = ?`), source).Scan(&last); rerr == nil {
			return last, nil
		}
		return 0, err
	}
	return initial, nil
}

// AdvanceCursor moves a cursor from "from" to "to" inside tx. It returns false
// when another worker already moved it, in which case the caller must roll
// back: the row lock taken here serialises concurrent dispatchers, so events
// are enqueued exactly once.
func (r *Repository) AdvanceCursor(ctx context.Context, tx *sql.Tx, source string, from, to int64) (bool, error) {
	res, err := tx.ExecContext(ctx, database.ConvertPlaceholders(
		`UPDATE gk_webhook_event_cursor SET last_id = ?, change_time = ? WHERE source = ? AND last_id = ?`),
		to, now(), source, from)
	if err != nil {
		return false, err
	}
	n, err := res.RowsAffected()
	return n == 1, err
}

// truncate makes s storable in a text column: valid UTF-8 (MySQL utf8mb4
// rejects anything else), no NUL bytes (PostgreSQL rejects them) and at most n
// bytes, cut on a rune boundary.
func truncate(s string, n int) string {
	s = strings.ReplaceAll(strings.ToValidUTF8(s, "\uFFFD"), "\x00", "")
	if len(s) <= n {
		return s
	}
	for n > 0 && !utf8.RuneStart(s[n]) {
		n--
	}
	return s[:n]
}
