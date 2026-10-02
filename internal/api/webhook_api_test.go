package api

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/goatkit/goatflow/internal/platform/database"
	"github.com/goatkit/goatflow/internal/platform/routing"
	"github.com/goatkit/goatflow/internal/platform/secureconfig"
	"github.com/goatkit/goatflow/internal/platform/shared"
	"github.com/goatkit/goatflow/internal/platform/webhook"
)

// webhookTestDB returns the test database; webhook tests need the real
// gk_webhook* schema from migration 000028.
func webhookTestDB(t *testing.T) *sql.DB {
	t.Helper()
	if os.Getenv("TEST_DB_HOST") == "" {
		t.Skip("webhook tests need a test database (TEST_DB_HOST)")
	}
	require.NoError(t, database.InitTestDB())
	db, err := database.GetDB()
	require.NoError(t, err)
	require.NotNil(t, db)
	return db
}

// webhookReceiver is an HTTP endpoint that records what it receives. It
// listens on 127.0.0.1, so tests that deliver to it opt in to private targets.
type webhookReceiver struct {
	mu     sync.Mutex
	status int
	got    []receivedRequest
	server *httptest.Server
}

type receivedRequest struct {
	header http.Header
	body   []byte
}

func newWebhookReceiver(t *testing.T) *webhookReceiver {
	t.Setenv(webhook.AllowPrivateTargetsEnv, "true")
	r := &webhookReceiver{status: http.StatusOK}
	r.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		body, _ := io.ReadAll(req.Body)
		r.mu.Lock()
		r.got = append(r.got, receivedRequest{header: req.Header.Clone(), body: body})
		status := r.status
		r.mu.Unlock()
		w.WriteHeader(status)
		_, _ = w.Write([]byte(`{"received":true}`))
	}))
	t.Cleanup(r.server.Close)
	return r
}

func (r *webhookReceiver) setStatus(code int) {
	r.mu.Lock()
	r.status = code
	r.mu.Unlock()
}

func (r *webhookReceiver) requests() []receivedRequest {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]receivedRequest(nil), r.got...)
}

// webhookAdminRouter registers the webhook handlers behind a stub that
// authenticates user 1 (the YAML route test covers the real middleware).
func webhookAdminRouter() *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(func(c *gin.Context) { c.Set("user_id", 1); c.Next() })
	g := r.Group("/api/v1")
	g.GET("/webhooks", handleWebhookList)
	g.POST("/webhooks", handleWebhookCreate)
	g.GET("/webhooks/events", handleWebhookEvents)
	g.GET("/webhook-deliveries/:id", handleWebhookDeliveryGet)
	g.POST("/webhook-deliveries/:id/redeliver", handleWebhookRedeliver)
	g.GET("/webhooks/:id", handleWebhookGet)
	g.PUT("/webhooks/:id", handleWebhookUpdate)
	g.DELETE("/webhooks/:id", handleWebhookDelete)
	g.POST("/webhooks/:id/test", handleWebhookTest)
	g.GET("/webhooks/:id/deliveries", handleWebhookDeliveries)
	return r
}

type apiEnvelope struct {
	Success bool            `json:"success"`
	Data    json.RawMessage `json:"data"`
	Error   string          `json:"error"`
}

func doWebhookRequest(t *testing.T, r http.Handler, method, path string, body interface{}, header ...string) (int, apiEnvelope) {
	t.Helper()
	var reader io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		require.NoError(t, err)
		reader = bytes.NewReader(b)
	}
	req := httptest.NewRequest(method, path, reader)
	req.Header.Set("Content-Type", "application/json")
	for i := 0; i+1 < len(header); i += 2 {
		req.Header.Set(header[i], header[i+1])
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	var env apiEnvelope
	_ = json.Unmarshal(w.Body.Bytes(), &env)
	return w.Code, env
}

func decodeWebhook(t *testing.T, raw json.RawMessage) webhook.Webhook {
	t.Helper()
	var w webhook.Webhook
	require.NoError(t, json.Unmarshal(raw, &w), string(raw))
	return w
}

func decodeDelivery(t *testing.T, raw json.RawMessage) webhook.Delivery {
	t.Helper()
	var d webhook.Delivery
	require.NoError(t, json.Unmarshal(raw, &d), string(raw))
	return d
}

// headerHintsOf returns the header_hints object of a webhook response.
func headerHintsOf(t *testing.T, raw json.RawMessage) map[string]string {
	t.Helper()
	var w struct {
		HeaderHints map[string]string `json:"header_hints"`
	}
	require.NoError(t, json.Unmarshal(raw, &w), string(raw))
	return w.HeaderHints
}

func uniqueWebhookName(t *testing.T) string {
	return fmt.Sprintf("%s-%d", t.Name(), time.Now().UnixNano())
}

// cleanupIfCreated deletes a webhook a create request made, also when the
// test expected the request to be rejected, so failures leave no rows behind.
func cleanupIfCreated(t *testing.T, db *sql.DB, code int, env apiEnvelope) {
	if code == http.StatusCreated {
		deleteWebhookOnCleanup(t, db, decodeWebhook(t, env.Data).ID)
	}
}

func deleteWebhookOnCleanup(t *testing.T, db *sql.DB, id int64) {
	t.Cleanup(func() {
		_, _ = db.Exec(database.ConvertPlaceholders(`DELETE FROM gk_webhook WHERE id = ?`), id)
	})
}

func TestWebhookAPI_CreateValidatesInput(t *testing.T) {
	db := webhookTestDB(t)
	r := webhookAdminRouter()
	valid := func() map[string]interface{} {
		return map[string]interface{}{
			"name":   uniqueWebhookName(t),
			"url":    "https://example.com/hook",
			"events": []string{"ticket.created"},
		}
	}
	cases := []struct {
		name   string
		mutate func(map[string]interface{})
		want   string
	}{
		{"missing name", func(b map[string]interface{}) { delete(b, "name") }, "name is required"},
		{"missing events", func(b map[string]interface{}) { delete(b, "events") }, "at least one event is required"},
		{"unknown event", func(b map[string]interface{}) { b["events"] = []string{"ticket.exploded"} }, `unknown event "ticket.exploded"`},
		{"non-http url", func(b map[string]interface{}) { b["url"] = "ftp://example.com/x" }, "url must be an absolute http or https URL"},
		{"relative url", func(b map[string]interface{}) { b["url"] = "/hook" }, "url must be an absolute http or https URL"},
		{"short secret", func(b map[string]interface{}) { b["secret"] = "tooshort" }, "secret must be between 16 and 512 characters"},
		{"reserved header", func(b map[string]interface{}) { b["headers"] = map[string]string{"X-Webhook-Signature": "x"} },
			`header "X-Webhook-Signature" is set by GoatFlow and cannot be overridden`},
		{"header value with line break", func(b map[string]interface{}) { b["headers"] = map[string]string{"X-Team": "a\r\nX-Evil: 1"} },
			`value of header "X-Team" contains a control character`},
		{"header without value", func(b map[string]interface{}) { b["headers"] = map[string]interface{}{"X-Team": nil} },
			`header "X-Team" has no stored value to keep; send a value`},
		{"timeout too long", func(b map[string]interface{}) { b["timeout_seconds"] = 61 }, "timeout_seconds must be between 1 and 60"},
		{"too many retries", func(b map[string]interface{}) { b["retry_count"] = 11 }, "retry_count must be between 0 and 10"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			body := valid()
			tc.mutate(body)
			code, env := doWebhookRequest(t, r, http.MethodPost, "/api/v1/webhooks", body)
			cleanupIfCreated(t, db, code, env)
			assert.Equal(t, http.StatusBadRequest, code)
			assert.False(t, env.Success)
			assert.Equal(t, tc.want, env.Error)
		})
	}
}

func TestWebhookAPI_CRUD(t *testing.T) {
	db := webhookTestDB(t)
	r := webhookAdminRouter()
	const secret = "s3cret-signing-key-0123"
	const apiKey = "key-0123456789-abcd"
	name := uniqueWebhookName(t)

	code, env := doWebhookRequest(t, r, http.MethodPost, "/api/v1/webhooks", map[string]interface{}{
		"name":    name,
		"url":     "https://example.com/hook",
		"secret":  secret,
		"events":  []string{"ticket.created", "article.created", "ticket.created"},
		"headers": map[string]string{"X-Team": "support", "X-Api-Key": apiKey},
	})
	require.Equal(t, http.StatusCreated, code, env.Error)
	created := decodeWebhook(t, env.Data)
	deleteWebhookOnCleanup(t, db, created.ID)

	assert.Equal(t, name, created.Name)
	assert.Equal(t, []string{"ticket.created", "article.created"}, created.Events, "duplicates removed, order kept")
	// Header values are write-only: short values get no hint, long ones their last 4 characters.
	assert.Equal(t, map[string]string{"X-Team": "••••••••", "X-Api-Key": "••••••••abcd"}, headerHintsOf(t, env.Data))
	assert.NotContains(t, string(env.Data), apiKey, "header values must never be returned")
	assert.NotContains(t, string(env.Data), "support", "header values must never be returned")
	assert.Equal(t, webhook.DefaultRetryCount, created.RetryCount)
	assert.Equal(t, webhook.DefaultTimeoutSeconds, created.TimeoutSeconds)
	assert.True(t, created.IsActive)
	assert.True(t, created.HasSecret)
	assert.Equal(t, "••••••••0123", created.SecretHint)
	assert.NotContains(t, string(env.Data), secret, "secret must never be returned")
	assert.Equal(t, 1, created.CreatedBy)

	key, err := secureconfig.GetKey()
	require.NoError(t, err)
	// The secret and the header values are stored encrypted, not in clear text.
	var stored, storedHeaders []byte
	require.NoError(t, db.QueryRow(database.ConvertPlaceholders(
		`SELECT secret_encrypted, headers_encrypted FROM gk_webhook WHERE id = ?`), created.ID).Scan(&stored, &storedHeaders))
	assert.NotContains(t, string(stored), secret)
	plain, err := secureconfig.Decrypt(stored, key)
	require.NoError(t, err)
	assert.Equal(t, secret, string(plain))
	assert.NotContains(t, string(storedHeaders), apiKey)
	storedHeaderValues := func() map[string]string {
		t.Helper()
		var enc []byte
		require.NoError(t, db.QueryRow(database.ConvertPlaceholders(
			`SELECT headers_encrypted FROM gk_webhook WHERE id = ?`), created.ID).Scan(&enc))
		if len(enc) == 0 {
			return map[string]string{}
		}
		plain, err := secureconfig.Decrypt(enc, key)
		require.NoError(t, err)
		var values map[string]string
		require.NoError(t, json.Unmarshal(plain, &values))
		return values
	}
	assert.Equal(t, map[string]string{"X-Team": "support", "X-Api-Key": apiKey}, storedHeaderValues())
	t.Run("duplicate name is a conflict", func(t *testing.T) {
		code, env := doWebhookRequest(t, r, http.MethodPost, "/api/v1/webhooks", map[string]interface{}{
			"name": name, "url": "https://example.com/other", "events": []string{"ticket.closed"},
		})
		assert.Equal(t, http.StatusConflict, code)
		assert.Equal(t, webhook.ErrDuplicateName.Error(), env.Error)
	})

	t.Run("get", func(t *testing.T) {
		code, env := doWebhookRequest(t, r, http.MethodGet, fmt.Sprintf("/api/v1/webhooks/%d", created.ID), nil)
		require.Equal(t, http.StatusOK, code)
		assert.Equal(t, created, decodeWebhook(t, env.Data))
	})

	t.Run("partial update keeps omitted fields", func(t *testing.T) {
		code, env := doWebhookRequest(t, r, http.MethodPut, fmt.Sprintf("/api/v1/webhooks/%d", created.ID), map[string]interface{}{
			"is_active":       false,
			"timeout_seconds": 5,
			"secret":          "",
		})
		require.Equal(t, http.StatusOK, code, env.Error)
		updated := decodeWebhook(t, env.Data)
		assert.False(t, updated.IsActive)
		assert.Equal(t, 5, updated.TimeoutSeconds)
		assert.False(t, updated.HasSecret, "empty secret removes it")
		assert.Empty(t, updated.SecretHint)
		assert.Equal(t, created.URL, updated.URL)
		assert.Equal(t, created.Events, updated.Events)
		assert.Equal(t, map[string]string{"X-Team": "••••••••", "X-Api-Key": "••••••••abcd"}, headerHintsOf(t, env.Data))
		assert.Equal(t, map[string]string{"X-Team": "support", "X-Api-Key": apiKey}, storedHeaderValues())
	})

	t.Run("header update: null keeps, value replaces, omitted name removes", func(t *testing.T) {
		code, env := doWebhookRequest(t, r, http.MethodPut, fmt.Sprintf("/api/v1/webhooks/%d", created.ID), map[string]interface{}{
			"headers": map[string]interface{}{"X-Api-Key": nil, "X-Region": "eu-west-1-primary"},
		})
		require.Equal(t, http.StatusOK, code, env.Error)
		assert.Equal(t, map[string]string{"X-Api-Key": "••••••••abcd", "X-Region": "••••••••mary"}, headerHintsOf(t, env.Data))
		assert.Equal(t, map[string]string{"X-Api-Key": apiKey, "X-Region": "eu-west-1-primary"}, storedHeaderValues())

		code, env = doWebhookRequest(t, r, http.MethodPut, fmt.Sprintf("/api/v1/webhooks/%d", created.ID), map[string]interface{}{
			"headers": map[string]interface{}{"X-Unknown": nil},
		})
		assert.Equal(t, http.StatusBadRequest, code)
		assert.Equal(t, `header "X-Unknown" has no stored value to keep; send a value`, env.Error)
		assert.Equal(t, map[string]string{"X-Api-Key": apiKey, "X-Region": "eu-west-1-primary"}, storedHeaderValues(), "a rejected update changes nothing")

		code, env = doWebhookRequest(t, r, http.MethodPut, fmt.Sprintf("/api/v1/webhooks/%d", created.ID), map[string]interface{}{
			"headers": map[string]interface{}{},
		})
		require.Equal(t, http.StatusOK, code, env.Error)
		assert.Equal(t, map[string]string{}, headerHintsOf(t, env.Data))
		assert.Empty(t, storedHeaderValues())
	})

	t.Run("update validates", func(t *testing.T) {
		code, env := doWebhookRequest(t, r, http.MethodPut, fmt.Sprintf("/api/v1/webhooks/%d", created.ID), map[string]interface{}{
			"events": []string{},
		})
		assert.Equal(t, http.StatusBadRequest, code)
		assert.Equal(t, "at least one event is required", env.Error)
	})

	t.Run("list filters by active", func(t *testing.T) {
		ids := func(path string) []int64 {
			code, env := doWebhookRequest(t, r, http.MethodGet, path, nil)
			require.Equal(t, http.StatusOK, code)
			var list []webhook.Webhook
			require.NoError(t, json.Unmarshal(env.Data, &list))
			out := []int64{}
			for _, w := range list {
				out = append(out, w.ID)
			}
			return out
		}
		assert.Contains(t, ids("/api/v1/webhooks"), created.ID)
		assert.Contains(t, ids("/api/v1/webhooks?active=false"), created.ID)
		assert.NotContains(t, ids("/api/v1/webhooks?active=true"), created.ID)
		code, _ := doWebhookRequest(t, r, http.MethodGet, "/api/v1/webhooks?active=maybe", nil)
		assert.Equal(t, http.StatusBadRequest, code)
	})

	t.Run("delete removes webhook and its deliveries", func(t *testing.T) {
		_, err := database.GetAdapter().InsertWithReturning(db, database.ConvertPlaceholders(`
			INSERT INTO gk_webhook_delivery (webhook_id, event_type, payload, status, attempts, create_time, change_time)
			VALUES (?, 'ticket.created', '{}', 'failed', 1, ?, ?) RETURNING id`), created.ID, time.Now(), time.Now())
		require.NoError(t, err)

		code, _ := doWebhookRequest(t, r, http.MethodDelete, fmt.Sprintf("/api/v1/webhooks/%d", created.ID), nil)
		require.Equal(t, http.StatusOK, code)
		code, _ = doWebhookRequest(t, r, http.MethodGet, fmt.Sprintf("/api/v1/webhooks/%d", created.ID), nil)
		assert.Equal(t, http.StatusNotFound, code)
		var n int
		require.NoError(t, db.QueryRow(database.ConvertPlaceholders(
			`SELECT COUNT(*) FROM gk_webhook_delivery WHERE webhook_id = ?`), created.ID).Scan(&n))
		assert.Zero(t, n)
		code, _ = doWebhookRequest(t, r, http.MethodDelete, fmt.Sprintf("/api/v1/webhooks/%d", created.ID), nil)
		assert.Equal(t, http.StatusNotFound, code)
	})
}

func TestWebhookAPI_TestDeliveryAndRedeliver(t *testing.T) {
	db := webhookTestDB(t)
	r := webhookAdminRouter()
	recv := newWebhookReceiver(t)
	const secret = "another-signing-secret"

	code, env := doWebhookRequest(t, r, http.MethodPost, "/api/v1/webhooks", map[string]interface{}{
		"name":    uniqueWebhookName(t),
		"url":     recv.server.URL + "/hook",
		"secret":  secret,
		"events":  []string{"ticket.created"},
		"headers": map[string]string{"X-Team": "support"},
	})
	require.Equal(t, http.StatusCreated, code, env.Error)
	wh := decodeWebhook(t, env.Data)
	deleteWebhookOnCleanup(t, db, wh.ID)

	// Successful test delivery: signed, recorded, custom headers sent.
	code, env = doWebhookRequest(t, r, http.MethodPost, fmt.Sprintf("/api/v1/webhooks/%d/test", wh.ID), nil)
	require.Equal(t, http.StatusOK, code, env.Error)
	first := decodeDelivery(t, env.Data)
	assert.True(t, first.Success)
	assert.Equal(t, webhook.StatusDelivered, first.Status)
	require.NotNil(t, first.StatusCode)
	assert.Equal(t, http.StatusOK, *first.StatusCode)
	assert.Equal(t, 1, first.Attempts)
	assert.Equal(t, webhook.TestEvent, first.Event)
	assert.NotNil(t, first.DeliveredAt)

	got := recv.requests()
	require.Len(t, got, 1)
	assert.Equal(t, webhook.Sign(secret, got[0].body), got[0].header.Get(webhook.SignatureHeader))
	assert.Equal(t, webhook.TestEvent, got[0].header.Get(webhook.EventHeader))
	assert.Equal(t, fmt.Sprint(first.ID), got[0].header.Get(webhook.DeliveryHeader))
	assert.Equal(t, "support", got[0].header.Get("X-Team"))
	assert.Equal(t, "application/json", got[0].header.Get("Content-Type"))
	var envelope struct {
		Event string                 `json:"event"`
		Data  map[string]interface{} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(got[0].body, &envelope))
	assert.Equal(t, webhook.TestEvent, envelope.Event)
	assert.Equal(t, float64(wh.ID), envelope.Data["webhook_id"])

	// Failing endpoint: recorded as failed, no automatic retry for tests.
	recv.setStatus(http.StatusInternalServerError)
	code, env = doWebhookRequest(t, r, http.MethodPost, fmt.Sprintf("/api/v1/webhooks/%d/test", wh.ID), nil)
	require.Equal(t, http.StatusOK, code, env.Error)
	failed := decodeDelivery(t, env.Data)
	assert.False(t, failed.Success)
	assert.Equal(t, webhook.StatusFailed, failed.Status)
	require.NotNil(t, failed.StatusCode)
	assert.Equal(t, http.StatusInternalServerError, *failed.StatusCode)
	assert.Equal(t, "endpoint returned HTTP 500", failed.Error)
	assert.Nil(t, failed.NextAttemptAt)

	// Delivery detail carries payload and response.
	code, env = doWebhookRequest(t, r, http.MethodGet, fmt.Sprintf("/api/v1/webhook-deliveries/%d", failed.ID), nil)
	require.Equal(t, http.StatusOK, code)
	detail := decodeDelivery(t, env.Data)
	assert.Equal(t, string(recv.requests()[1].body), detail.Payload)
	assert.Equal(t, `{"received":true}`, detail.Response)

	// Redeliver once the endpoint recovers: new delivery, same payload.
	recv.setStatus(http.StatusNoContent)
	code, env = doWebhookRequest(t, r, http.MethodPost, fmt.Sprintf("/api/v1/webhook-deliveries/%d/redeliver", failed.ID), nil)
	require.Equal(t, http.StatusOK, code, env.Error)
	redelivered := decodeDelivery(t, env.Data)
	assert.NotEqual(t, failed.ID, redelivered.ID)
	assert.True(t, redelivered.Success)
	assert.Equal(t, webhook.StatusDelivered, redelivered.Status)
	got = recv.requests()
	require.Len(t, got, 3)
	assert.Equal(t, got[1].body, got[2].body, "redelivery sends the original payload")
	assert.Equal(t, fmt.Sprint(redelivered.ID), got[2].header.Get(webhook.DeliveryHeader))
	assert.Equal(t, webhook.Sign(secret, got[2].body), got[2].header.Get(webhook.SignatureHeader))

	// The failed delivery is unchanged.
	var status string
	require.NoError(t, db.QueryRow(database.ConvertPlaceholders(
		`SELECT status FROM gk_webhook_delivery WHERE id = ?`), failed.ID).Scan(&status))
	assert.Equal(t, webhook.StatusFailed, status)

	// Delivery log: newest first, without bodies.
	code, env = doWebhookRequest(t, r, http.MethodGet, fmt.Sprintf("/api/v1/webhooks/%d/deliveries", wh.ID), nil)
	require.Equal(t, http.StatusOK, code)
	var list []webhook.Delivery
	require.NoError(t, json.Unmarshal(env.Data, &list))
	require.Len(t, list, 3)
	assert.Equal(t, []int64{redelivered.ID, failed.ID, first.ID}, []int64{list[0].ID, list[1].ID, list[2].ID})
	assert.Empty(t, list[0].Payload)
	assert.True(t, list[0].Success)
	assert.False(t, list[1].Success)

	code, env = doWebhookRequest(t, r, http.MethodGet, fmt.Sprintf("/api/v1/webhooks/%d/deliveries?limit=1", wh.ID), nil)
	require.Equal(t, http.StatusOK, code)
	require.NoError(t, json.Unmarshal(env.Data, &list))
	assert.Len(t, list, 1)

	code, _ = doWebhookRequest(t, r, http.MethodPost, "/api/v1/webhook-deliveries/999999999/redeliver", nil)
	assert.Equal(t, http.StatusNotFound, code)
	code, _ = doWebhookRequest(t, r, http.MethodPost, "/api/v1/webhooks/999999999/test", nil)
	assert.Equal(t, http.StatusNotFound, code)
	code, _ = doWebhookRequest(t, r, http.MethodGet, "/api/v1/webhooks/999999999/deliveries", nil)
	assert.Equal(t, http.StatusNotFound, code)
}

func TestWebhookAPI_UnreachableEndpointIsRecorded(t *testing.T) {
	db := webhookTestDB(t)
	r := webhookAdminRouter()
	t.Setenv(webhook.AllowPrivateTargetsEnv, "true")
	closed := httptest.NewServer(http.NotFoundHandler())
	url := closed.URL
	closed.Close()

	code, env := doWebhookRequest(t, r, http.MethodPost, "/api/v1/webhooks", map[string]interface{}{
		"name": uniqueWebhookName(t), "url": url, "events": []string{"ticket.created"},
	})
	require.Equal(t, http.StatusCreated, code, env.Error)
	wh := decodeWebhook(t, env.Data)
	deleteWebhookOnCleanup(t, db, wh.ID)

	code, env = doWebhookRequest(t, r, http.MethodPost, fmt.Sprintf("/api/v1/webhooks/%d/test", wh.ID), nil)
	require.Equal(t, http.StatusOK, code)
	d := decodeDelivery(t, env.Data)
	assert.Equal(t, webhook.StatusFailed, d.Status)
	assert.Nil(t, d.StatusCode)
	assert.Contains(t, d.Error, "connection refused")
	assert.Equal(t, 1, d.Attempts)
}

// TestWebhookAPI_RejectsInternalTargets: without the operator opt-in a webhook
// cannot point at loopback, private, link-local (cloud metadata) or other
// internal addresses, neither when saved nor when delivered.
func TestWebhookAPI_RejectsInternalTargets(t *testing.T) {
	db := webhookTestDB(t)
	r := webhookAdminRouter()
	t.Setenv(webhook.AllowPrivateTargetsEnv, "")

	create := func(url string) (int, apiEnvelope) {
		code, env := doWebhookRequest(t, r, http.MethodPost, "/api/v1/webhooks", map[string]interface{}{
			"name": uniqueWebhookName(t), "url": url, "events": []string{"ticket.created"},
		})
		cleanupIfCreated(t, db, code, env)
		return code, env
	}
	for _, url := range []string{
		"http://169.254.169.254/latest/meta-data/",
		"http://127.0.0.1:8080/hook",
		"http://10.1.2.3/hook",
		"http://172.16.0.1/hook",
		"http://192.168.1.10/hook",
		"http://[::1]/hook",
		"http://[fd00:ec2::254]/hook",
		"http://[::ffff:127.0.0.1]/hook",
		"http://0.0.0.0:9000/hook",
		"http://100.100.100.200/hook",
		"http://localhost:8080/hook",
		"http://api.localhost/hook",
	} {
		code, env := create(url)
		assert.Equal(t, http.StatusBadRequest, code, "%s: %s", url, env.Error)
		assert.Contains(t, env.Error, "GOATFLOW_WEBHOOK_ALLOW_PRIVATE_TARGETS", url)
	}
	for _, url := range []string{"http://172.32.0.1/hook", "http://8.8.8.8/hook", "https://example.com/hook"} {
		code, env := create(url)
		require.Equal(t, http.StatusCreated, code, "%s: %s", url, env.Error)
	}

	// Updating a webhook to an internal address is rejected too.
	code, env := create("https://example.com/other")
	require.Equal(t, http.StatusCreated, code, env.Error)
	public := decodeWebhook(t, env.Data)
	code, env = doWebhookRequest(t, r, http.MethodPut, fmt.Sprintf("/api/v1/webhooks/%d", public.ID),
		map[string]interface{}{"url": "http://169.254.169.254/latest/meta-data/"})
	assert.Equal(t, http.StatusBadRequest, code, env.Error)

	// A host name is checked after DNS resolution on every delivery: a
	// webhook saved while internal targets were allowed is refused once they
	// are not, the endpoint receives nothing and the failure is not retried.
	recv := newWebhookReceiver(t) // allows internal targets
	_, port, err := net.SplitHostPort(strings.TrimPrefix(recv.server.URL, "http://"))
	require.NoError(t, err)
	code, env = create("http://localhost:" + port + "/hook")
	require.Equal(t, http.StatusCreated, code, env.Error)
	internal := decodeWebhook(t, env.Data)

	t.Setenv(webhook.AllowPrivateTargetsEnv, "false")
	code, env = doWebhookRequest(t, r, http.MethodPost, fmt.Sprintf("/api/v1/webhooks/%d/test", internal.ID), nil)
	require.Equal(t, http.StatusOK, code, env.Error)
	d := decodeDelivery(t, env.Data)
	assert.Equal(t, webhook.StatusFailed, d.Status)
	assert.Nil(t, d.StatusCode)
	assert.Contains(t, d.Error, "webhook host localhost resolves to")
	assert.Contains(t, d.Error, "GOATFLOW_WEBHOOK_ALLOW_PRIVATE_TARGETS")
	assert.Empty(t, recv.requests(), "the internal endpoint must not be called")

	t.Setenv(webhook.AllowPrivateTargetsEnv, "true")
	code, env = doWebhookRequest(t, r, http.MethodPost, fmt.Sprintf("/api/v1/webhooks/%d/test", internal.ID), nil)
	require.Equal(t, http.StatusOK, code, env.Error)
	assert.Equal(t, webhook.StatusDelivered, decodeDelivery(t, env.Data).Status, "the opt-in allows internal targets")
	assert.Len(t, recv.requests(), 1)
}

// TestWebhookRoutes exercises the YAML routes with the real auth and admin
// middleware: before routes/api-webhooks.yaml the endpoints did not exist.
func TestWebhookRoutes(t *testing.T) {
	db := webhookTestDB(t)
	gin.SetMode(gin.TestMode)
	router := gin.New()
	require.NoError(t, routing.LoadYAMLRoutesForTesting(router))

	jwt := shared.GetJWTManager()
	adminToken, err := jwt.GenerateToken(1, "root@localhost", "Admin", 0)
	require.NoError(t, err)
	agentToken, err := jwt.GenerateToken(1, "root@localhost", "Agent", 0)
	require.NoError(t, err)

	code, _ := doWebhookRequest(t, router, http.MethodGet, "/api/v1/webhooks", nil)
	assert.Equal(t, http.StatusUnauthorized, code)
	code, _ = doWebhookRequest(t, router, http.MethodGet, "/api/v1/webhooks", nil, "Authorization", "Bearer "+agentToken)
	assert.Equal(t, http.StatusForbidden, code)

	admin := []string{"Authorization", "Bearer " + adminToken}
	code, env := doWebhookRequest(t, router, http.MethodGet, "/api/v1/webhooks/events", nil, admin...)
	require.Equal(t, http.StatusOK, code)
	assert.Contains(t, string(env.Data), `"ticket.created"`)

	code, env = doWebhookRequest(t, router, http.MethodPost, "/api/v1/webhooks", map[string]interface{}{
		"name": uniqueWebhookName(t), "url": "https://example.com/hook", "events": []string{"ticket.closed"},
	}, admin...)
	require.Equal(t, http.StatusCreated, code, env.Error)
	wh := decodeWebhook(t, env.Data)
	deleteWebhookOnCleanup(t, db, wh.ID)

	code, env = doWebhookRequest(t, router, http.MethodGet, fmt.Sprintf("/api/v1/webhooks/%d", wh.ID), nil, admin...)
	require.Equal(t, http.StatusOK, code)
	assert.Equal(t, wh.ID, decodeWebhook(t, env.Data).ID)
	code, _ = doWebhookRequest(t, router, http.MethodGet, fmt.Sprintf("/api/v1/webhooks/%d/deliveries", wh.ID), nil, admin...)
	assert.Equal(t, http.StatusOK, code)
}
