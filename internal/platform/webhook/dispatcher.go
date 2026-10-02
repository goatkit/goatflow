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
	"sync"
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

// NewDispatcher returns a dispatcher using repo. Deliveries do not follow
// redirects and only reach internal addresses when AllowPrivateTargetsEnv
// allows it (see target.go).
func NewDispatcher(repo *Repository) *Dispatcher {
	return &Dispatcher{repo: repo, client: deliveryClient}
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

// MaxParallelWebhooks bounds how many webhooks one ProcessDue call sends to
// at the same time. Deliveries to one webhook are sent one at a time, oldest
// first, so a slow or unreachable endpoint only delays its own deliveries.
const MaxParallelWebhooks = 8

// ProcessDue sends the due deliveries of every webhook, applying the retry
// policy of each, until none is due or ctx ends. Up to MaxParallelWebhooks
// webhooks are served concurrently. Deliveries still due when ctx ends stay
// pending for the next call; that is not an error. It returns the number of
// delivery attempts made.
func (d *Dispatcher) ProcessDue(ctx context.Context) (int, error) {
	hooks, err := d.repo.dueWebhooks(ctx)
	if err != nil {
		return 0, err
	}
	var (
		wg        sync.WaitGroup
		mu        sync.Mutex
		attempted int
		firstErr  error
	)
	slots := make(chan struct{}, MaxParallelWebhooks)
	for _, id := range hooks {
		select {
		case slots <- struct{}{}:
		case <-ctx.Done():
		}
		if ctx.Err() != nil {
			break
		}
		wg.Add(1)
		go func(webhookID int64) {
			defer func() { <-slots; wg.Done() }()
			n, err := d.drain(ctx, webhookID)
			mu.Lock()
			attempted += n
			if err != nil && firstErr == nil {
				firstErr = err
			}
			mu.Unlock()
		}(id)
	}
	wg.Wait()
	return attempted, firstErr
}

// drain sends the due deliveries of one webhook in order until none is due or
// ctx ends.
func (d *Dispatcher) drain(ctx context.Context, webhookID int64) (int, error) {
	n := 0
	for ctx.Err() == nil {
		id, ok, err := d.repo.claimNext(ctx, webhookID)
		if err != nil {
			if ctx.Err() != nil {
				return n, nil
			}
			return n, err
		}
		if !ok {
			return n, nil
		}
		n++
		if _, err := d.send(ctx, id, true); err != nil {
			return n, err
		}
	}
	return n, nil
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

// recordTimeout bounds the database writes that record an attempt. They run
// detached from the caller's context: a send that used up the run's deadline
// must still be recorded, or the delivery would sit in "delivering" until
// another worker takes it over.
const recordTimeout = 30 * time.Second

// send performs one HTTP attempt for a delivery already in "delivering" state
// and records the outcome. With retry, a failed attempt is rescheduled while
// the webhook's retry budget lasts; otherwise it is final.
func (d *Dispatcher) send(ctx context.Context, id int64, retry bool) (*Delivery, error) {
	del, err := d.repo.GetDelivery(ctx, id)
	if err != nil {
		return nil, err
	}
	w, enc, err := d.repo.getWithSecrets(ctx, del.WebhookID)
	if err != nil {
		return nil, err
	}
	if retry && !w.IsActive {
		if err := d.repo.failDelivery(ctx, id, "webhook is inactive"); err != nil {
			return nil, err
		}
		return d.repo.GetDelivery(ctx, id)
	}
	secret, err := decryptSecret(enc.secret)
	if err == nil {
		w.Headers, err = decryptHeaders(enc.headers)
	}
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
		if retry && !res.permanent && attempts <= w.RetryCount {
			res.status = StatusPending
			res.retryAfter = backoff(attempts)
		}
	}
	rctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), recordTimeout)
	defer cancel()
	if err := d.repo.recordAttempt(rctx, id, res); err != nil {
		return nil, err
	}
	return d.repo.GetDelivery(rctx, id)
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
		// A blocked address will not become reachable by retrying.
		res.permanent = isBlockedTarget(err)
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
