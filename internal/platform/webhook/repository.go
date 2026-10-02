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

const webhookColumns = `id, name, url, secret_encrypted, secret_hint, events, header_hints, headers_encrypted,
	retry_count, timeout_seconds, valid_id, create_time, create_by, change_time, change_by`

type rowScanner interface {
	Scan(dest ...interface{}) error
}

// storedSecrets are the encrypted columns of a gk_webhook row. They are kept
// out of the JSON-serialisable Webhook struct.
type storedSecrets struct {
	secret  []byte
	headers []byte
}

// scanWebhook reads one gk_webhook row.
func scanWebhook(s rowScanner) (*Webhook, storedSecrets, error) {
	var (
		w          Webhook
		enc        storedSecrets
		hint       sql.NullString
		eventsJSON string
		hintsJSON  sql.NullString
		validID    int
	)
	if err := s.Scan(&w.ID, &w.Name, &w.URL, &enc.secret, &hint, &eventsJSON, &hintsJSON, &enc.headers,
		&w.RetryCount, &w.TimeoutSeconds, &validID, &w.CreatedAt, &w.CreatedBy, &w.UpdatedAt, &w.UpdatedBy); err != nil {
		return nil, storedSecrets{}, err
	}
	if err := json.Unmarshal([]byte(eventsJSON), &w.Events); err != nil {
		return nil, storedSecrets{}, fmt.Errorf("webhook %d: corrupt events column: %w", w.ID, err)
	}
	w.HeaderHints = map[string]string{}
	if hintsJSON.Valid && hintsJSON.String != "" {
		var raw map[string]string
		if err := json.Unmarshal([]byte(hintsJSON.String), &raw); err != nil {
			return nil, storedSecrets{}, fmt.Errorf("webhook %d: corrupt header_hints column: %w", w.ID, err)
		}
		for name, h := range raw {
			w.HeaderHints[name] = secureconfig.MaskedDisplay(h)
		}
	}
	w.HasSecret = len(enc.secret) > 0
	if w.HasSecret {
		w.SecretHint = secureconfig.MaskedDisplay(hint.String)
	}
	w.IsActive = validID == validActive
	return &w, enc, nil
}

func encryptValue(plain []byte) ([]byte, error) {
	key, err := secureconfig.GetKey()
	if err != nil {
		return nil, fmt.Errorf("secure config key: %w", err)
	}
	return secureconfig.Encrypt(plain, key)
}

func decryptValue(enc []byte, what string) ([]byte, error) {
	key, err := secureconfig.GetKey()
	if err != nil {
		return nil, fmt.Errorf("secure config key: %w", err)
	}
	plain, err := secureconfig.Decrypt(enc, key)
	if err != nil {
		return nil, fmt.Errorf("cannot decrypt webhook %s (is %s the same for every GoatFlow process?): %w",
			what, secureconfig.KeyEnvVar, err)
	}
	return plain, nil
}

func encryptSecret(secret string) ([]byte, interface{}, error) {
	if secret == "" {
		return nil, nil, nil
	}
	enc, err := encryptValue([]byte(secret))
	if err != nil {
		return nil, nil, err
	}
	return enc, secureconfig.ValueHint(secret), nil
}

func decryptSecret(enc []byte) (string, error) {
	if len(enc) == 0 {
		return "", nil
	}
	plain, err := decryptValue(enc, "secret")
	return string(plain), err
}

// headerHint is the part of a header value shown to admins: its last four
// characters, and nothing for values shorter than 12 characters, where four
// characters would give away too much of the value.
func headerHint(value string) string {
	r := []rune(value)
	if len(r) < 12 {
		return ""
	}
	return string(r[len(r)-4:])
}

// encryptHeaders returns the encrypted JSON of the header values and the JSON
// of their hints; both are NULL when there are no headers.
func encryptHeaders(headers map[string]string) (interface{}, interface{}, error) {
	if len(headers) == 0 {
		return nil, nil, nil
	}
	plain, err := json.Marshal(headers)
	if err != nil {
		return nil, nil, err
	}
	enc, err := encryptValue(plain)
	if err != nil {
		return nil, nil, err
	}
	hints := make(map[string]string, len(headers))
	for name, v := range headers {
		hints[name] = headerHint(v)
	}
	hintsJSON, err := json.Marshal(hints)
	if err != nil {
		return nil, nil, err
	}
	return enc, string(hintsJSON), nil
}

func decryptHeaders(enc []byte) (map[string]string, error) {
	headers := map[string]string{}
	if len(enc) == 0 {
		return headers, nil
	}
	plain, err := decryptValue(enc, "header values")
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(plain, &headers); err != nil {
		return nil, fmt.Errorf("corrupt webhook header values: %w", err)
	}
	return headers, nil
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
	headersEnc, headerHints, err := encryptHeaders(w.Headers)
	if err != nil {
		return nil, err
	}
	events, err := json.Marshal(w.Events)
	if err != nil {
		return nil, err
	}
	ts := now()
	id, err := database.GetAdapter().InsertWithReturning(r.db, database.ConvertPlaceholders(`
		INSERT INTO gk_webhook (name, url, secret_encrypted, secret_hint, events, header_hints, headers_encrypted,
			retry_count, timeout_seconds, valid_id, create_time, create_by, change_time, change_by)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?) RETURNING id`),
		w.Name, w.URL, enc, hint, string(events), headerHints, headersEnc,
		w.RetryCount, w.TimeoutSeconds, validID(w.IsActive), ts, userID, ts, userID)
	if err != nil {
		return nil, err
	}
	return r.Get(ctx, id)
}

// Get returns one webhook (without header values) or ErrNotFound.
func (r *Repository) Get(ctx context.Context, id int64) (*Webhook, error) {
	w, _, err := r.getWithSecrets(ctx, id)
	return w, err
}

func (r *Repository) getWithSecrets(ctx context.Context, id int64) (*Webhook, storedSecrets, error) {
	row := r.db.QueryRowContext(ctx, database.ConvertPlaceholders(
		`SELECT `+webhookColumns+` FROM gk_webhook WHERE id = ?`), id)
	w, enc, err := scanWebhook(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, storedSecrets{}, ErrNotFound
	}
	return w, enc, err
}

// HeaderValues returns the decrypted custom header values of a webhook, for
// updates that keep some stored values.
func (r *Repository) HeaderValues(ctx context.Context, id int64) (map[string]string, error) {
	_, enc, err := r.getWithSecrets(ctx, id)
	if err != nil {
		return nil, err
	}
	return decryptHeaders(enc.headers)
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

// Update stores a modified (already normalised) webhook. The custom headers
// are replaced when w.Headers is non-nil and kept otherwise.
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
	query := `UPDATE gk_webhook SET name = ?, url = ?, events = ?, retry_count = ?,
		timeout_seconds = ?, valid_id = ?, change_time = ?, change_by = ?`
	args := []interface{}{w.Name, w.URL, string(events), w.RetryCount,
		w.TimeoutSeconds, validID(w.IsActive), now(), userID}
	if w.Headers != nil {
		headersEnc, headerHints, err := encryptHeaders(w.Headers)
		if err != nil {
			return nil, err
		}
		query += `, header_hints = ?, headers_encrypted = ?`
		args = append(args, headerHints, headersEnc)
	}
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
	if _, err := r.db.ExecContext(ctx, database.ConvertPlaceholders(query), args...); err != nil {
		return nil, err
	}
	// MySQL reports 0 affected rows when nothing changed, so existence is
	// checked by reading the row back (ErrNotFound when it is gone).
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

// dueCondition selects deliveries that may be sent now: pending ones whose
// next attempt is due, and "delivering" ones whose sender has gone stale.
const dueCondition = `((status = ? AND next_attempt_time <= ?) OR (status = ? AND change_time < ?))`

func dueArgs(ts time.Time) []interface{} {
	return []interface{}{StatusPending, ts, StatusDelivering, ts.Add(-staleDeliveringAfter)}
}

// dueWebhooks returns the ids of webhooks with due deliveries, the webhook
// with the oldest due delivery first.
func (r *Repository) dueWebhooks(ctx context.Context) ([]int64, error) {
	rows, err := r.db.QueryContext(ctx, database.ConvertPlaceholders(`
		SELECT webhook_id, MIN(id) AS oldest FROM gk_webhook_delivery
		WHERE `+dueCondition+`
		GROUP BY webhook_id
		ORDER BY oldest`), dueArgs(now())...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var ids []int64
	for rows.Next() {
		var id, oldest int64
		if err := rows.Scan(&id, &oldest); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

// claimNext atomically moves the oldest due delivery of a webhook to
// "delivering" and returns its id; ok is false when none is due. The claim is
// a conditional UPDATE, so concurrent workers never send the same delivery
// twice.
func (r *Repository) claimNext(ctx context.Context, webhookID int64) (id int64, ok bool, err error) {
	for {
		ts := now()
		args := append([]interface{}{webhookID}, dueArgs(ts)...)
		err := r.db.QueryRowContext(ctx, database.ConvertPlaceholders(`
			SELECT id FROM gk_webhook_delivery
			WHERE webhook_id = ? AND `+dueCondition+`
			ORDER BY id LIMIT 1`), args...).Scan(&id)
		if errors.Is(err, sql.ErrNoRows) {
			return 0, false, nil
		}
		if err != nil {
			return 0, false, err
		}
		res, err := r.db.ExecContext(ctx, database.ConvertPlaceholders(`
			UPDATE gk_webhook_delivery SET status = ?, change_time = ?
			WHERE id = ? AND `+dueCondition),
			append([]interface{}{StatusDelivering, ts, id}, dueArgs(ts)...)...)
		if err != nil {
			return 0, false, err
		}
		if n, _ := res.RowsAffected(); n == 1 {
			return id, true, nil
		}
		// Another worker claimed it first; look for the next one.
	}
}

// pruneBatch bounds the rows deleted per statement.
const pruneBatch = 1000

// PruneDeliveries deletes delivered and failed deliveries created before
// cutoff and returns how many were deleted. Pending and in-flight deliveries
// are kept whatever their age.
func (r *Repository) PruneDeliveries(ctx context.Context, cutoff time.Time) (int, error) {
	total := 0
	for {
		rows, err := r.db.QueryContext(ctx, database.ConvertPlaceholders(`
			SELECT id FROM gk_webhook_delivery
			WHERE create_time < ? AND status IN (?, ?)
			ORDER BY id LIMIT ?`), cutoff, StatusDelivered, StatusFailed, pruneBatch)
		if err != nil {
			return total, err
		}
		var ids []interface{}
		for rows.Next() {
			var id int64
			if err := rows.Scan(&id); err != nil {
				_ = rows.Close() // the Scan error is returned
				return total, err
			}
			ids = append(ids, id)
		}
		_ = rows.Close() // read-only cursor fully consumed; rows.Err() below reports failures
		if err := rows.Err(); err != nil {
			return total, err
		}
		if len(ids) == 0 {
			return total, nil
		}
		placeholders := strings.TrimSuffix(strings.Repeat("?,", len(ids)), ",")
		res, err := r.db.ExecContext(ctx, database.ConvertPlaceholders(
			`DELETE FROM gk_webhook_delivery WHERE id IN (`+placeholders+`)`), ids...)
		if err != nil {
			return total, err
		}
		n, _ := res.RowsAffected()
		total += int(n)
		if len(ids) < pruneBatch {
			return total, nil
		}
	}
}

// attemptResult is the outcome of one HTTP attempt.
type attemptResult struct {
	status     string
	statusCode *int
	response   string
	errMsg     string
	durationMS int
	retryAfter time.Duration // with status pending: wait before the next attempt
	permanent  bool          // the failure cannot be fixed by retrying
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
