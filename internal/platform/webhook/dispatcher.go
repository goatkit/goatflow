package webhook

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"
)

// Sign returns the X-Webhook-Signature value for body: "sha256=<hex>".
func Sign(secret string, body []byte) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	return "sha256=" + hex.EncodeToString(mac.Sum(nil))
}

// Retry backoff: 30s, 1m, 2m, 4m, ... capped at one hour.
const (
	baseBackoff = 30 * time.Second
	maxBackoff  = time.Hour
)

// backoff returns the wait after the given number of failed attempts.
func backoff(failedAttempts int) time.Duration {
	d := baseBackoff
	for i := 1; i < failedAttempts; i++ {
		d *= 2
		if d >= maxBackoff {
			return maxBackoff
		}
	}
	return d
}

// Dispatcher fans events out to subscribed webhooks and delivers them.
type Dispatcher struct {
	repo   *Repository
	client *http.Client
}

// NewDispatcher returns a dispatcher using repo. The HTTP client does not
// follow redirects: a webhook URL must point at the receiving endpoint.
func NewDispatcher(repo *Repository) *Dispatcher {
	return &Dispatcher{
		repo: repo,
		client: &http.Client{
			CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
		},
	}
}

func marshalEnvelope(event string, occurredAt time.Time, data interface{}) (string, error) {
	body, err := json.Marshal(Envelope{Event: event, OccurredAt: occurredAt.UTC(), Data: data})
	if err != nil {
		return "", fmt.Errorf("marshal %s payload: %w", event, err)
	}
	return string(body), nil
}

// Enqueue records a pending delivery of event for every active webhook in
// hooks that subscribes to it and returns how many were queued. Callers load
// hooks once (Repository.List) per batch of events. With a non-nil tx the rows
// are written inside that transaction.
func (d *Dispatcher) Enqueue(ctx context.Context, tx *sql.Tx, hooks []*Webhook, event string, occurredAt time.Time, data interface{}) (int, error) {
	var payload string
	queued := 0
	for _, w := range hooks {
		if !w.IsActive || !w.Subscribes(event) {
			continue
		}
		if payload == "" {
			p, err := marshalEnvelope(event, occurredAt, data)
			if err != nil {
				return 0, err
			}
			payload = p
		}
		if _, err := d.repo.insertDelivery(ctx, tx, w.ID, event, payload, StatusPending); err != nil {
			return 0, err
		}
		queued++
	}
	return queued, nil
}

// ProcessDue sends up to limit deliveries that are due, applying the retry
// policy of each webhook. It returns the number of deliveries attempted.
func (d *Dispatcher) ProcessDue(ctx context.Context, limit int) (int, error) {
	ids, err := d.repo.claimDue(ctx, limit)
	for i, id := range ids {
		if ctx.Err() != nil {
			return i, ctx.Err()
		}
		if _, serr := d.send(ctx, id, true); serr != nil && err == nil {
			err = serr
		}
	}
	return len(ids), err
}

// Test sends a webhook.test event to the webhook immediately, records it as
// a delivery (single attempt, no retries) and returns the result.
func (d *Dispatcher) Test(ctx context.Context, webhookID int64) (*Delivery, error) {
	w, err := d.repo.Get(ctx, webhookID)
	if err != nil {
		return nil, err
	}
	payload, err := marshalEnvelope(TestEvent, now(), map[string]interface{}{
		"webhook_id": w.ID,
		"name":       w.Name,
		"message":    "Test delivery from GoatFlow",
	})
	if err != nil {
		return nil, err
	}
	id, err := d.repo.insertDelivery(ctx, nil, w.ID, TestEvent, payload, StatusDelivering)
	if err != nil {
		return nil, err
	}
	return d.send(ctx, id, false)
}

// Redeliver sends the payload of an earlier delivery again as a new delivery
// (single attempt, no retries) and returns the new delivery.
func (d *Dispatcher) Redeliver(ctx context.Context, deliveryID int64) (*Delivery, error) {
	orig, err := d.repo.GetDelivery(ctx, deliveryID)
	if err != nil {
		return nil, err
	}
	id, err := d.repo.insertDelivery(ctx, nil, orig.WebhookID, orig.Event, orig.Payload, StatusDelivering)
	if err != nil {
		return nil, err
	}
	return d.send(ctx, id, false)
}

// send performs one HTTP attempt for a delivery already in "delivering" state
// and records the outcome. With retry, a failed attempt is rescheduled while
// the webhook's retry budget lasts; otherwise it is final.
func (d *Dispatcher) send(ctx context.Context, id int64, retry bool) (*Delivery, error) {
	del, err := d.repo.GetDelivery(ctx, id)
	if err != nil {
		return nil, err
	}
	w, encSecret, err := d.repo.getWithSecret(ctx, del.WebhookID)
	if err != nil {
		return nil, err
	}
	if retry && !w.IsActive {
		if err := d.repo.failDelivery(ctx, id, "webhook is inactive"); err != nil {
			return nil, err
		}
		return d.repo.GetDelivery(ctx, id)
	}
	secret, err := decryptSecret(encSecret)
	if err != nil {
		if ferr := d.repo.failDelivery(ctx, id, err.Error()); ferr != nil {
			return nil, ferr
		}
		return d.repo.GetDelivery(ctx, id)
	}

	res := d.post(ctx, w, secret, del)
	attempts := del.Attempts + 1
	if res.status != StatusDelivered {
		res.status = StatusFailed
		if retry && attempts <= w.RetryCount {
			res.status = StatusPending
			res.retryAfter = backoff(attempts)
		}
	}
	if err := d.repo.recordAttempt(ctx, id, res); err != nil {
		return nil, err
	}
	return d.repo.GetDelivery(ctx, id)
}

// post sends the delivery's payload and classifies the response: any 2xx is
// delivered, everything else (including transport errors) is a failure.
func (d *Dispatcher) post(ctx context.Context, w *Webhook, secret string, del *Delivery) attemptResult {
	ctx, cancel := context.WithTimeout(ctx, time.Duration(w.TimeoutSeconds)*time.Second)
	defer cancel()

	body := []byte(del.Payload)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, w.URL, bytes.NewReader(body))
	if err != nil {
		return attemptResult{status: StatusFailed, errMsg: err.Error()}
	}
	for k, v := range w.Headers {
		req.Header.Set(k, v)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", UserAgent)
	req.Header.Set(EventHeader, del.Event)
	req.Header.Set(DeliveryHeader, strconv.FormatInt(del.ID, 10))
	if secret != "" {
		req.Header.Set(SignatureHeader, Sign(secret, body))
	}

	start := time.Now()
	resp, err := d.client.Do(req)
	res := attemptResult{}
	if err != nil {
		res.durationMS = int(time.Since(start).Milliseconds())
		res.status = StatusFailed
		res.errMsg = err.Error()
		if errors.Is(err, context.DeadlineExceeded) {
			res.errMsg = fmt.Sprintf("no response within %d seconds", w.TimeoutSeconds)
		}
		return res
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(io.LimitReader(resp.Body, maxResponseBodyBytes))
	res.durationMS = int(time.Since(start).Milliseconds())
	code := resp.StatusCode
	res.statusCode = &code
	res.response = string(respBody)
	if code >= 200 && code < 300 {
		res.status = StatusDelivered
	} else {
		res.status = StatusFailed
		res.errMsg = fmt.Sprintf("endpoint returned HTTP %d", code)
	}
	return res
}
