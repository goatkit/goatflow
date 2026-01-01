//go:build integration

// Package integration provides SMTP4dev client for email integration testing.
package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
)

// SMTP4DevClient is a minimal client for smtp4dev v3 API (v1 schema).
// It is used only in integration tests.
type SMTP4DevClient struct {
	base   string
	client *http.Client
}

func NewSMTP4DevClient(base string, httpClient *http.Client) *SMTP4DevClient {
	if base == "" {
		base = "http://localhost:8025/api/v3"
	}
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	return &SMTP4DevClient{base: trimTrailingSlash(base), client: httpClient}
}

type Mailbox struct {
	ID            string `json:"id"`
	Name          string `json:"name"`
	Login         string `json:"login"`
	Password      string `json:"password,omitempty"`
	MessagesCount int    `json:"messagesCount"`
}

type createMailboxRequest struct {
	Name     string `json:"name"`
	Login    string `json:"login"`
	Password string `json:"password"`
}

type Message struct {
	ID        string   `json:"id"`
	Subject   string   `json:"subject"`
	From      string   `json:"from"`
	To        []string `json:"to"`
	MailboxID string   `json:"mailboxId"`
}

func (c *SMTP4DevClient) ListMailboxes(ctx context.Context) ([]Mailbox, error) {
	var boxes []Mailbox
	if err := c.do(ctx, http.MethodGet, "/mailboxes", nil, &boxes); err != nil {
		return nil, err
	}
	return boxes, nil
}

func (c *SMTP4DevClient) FindMailboxByLogin(ctx context.Context, login string) (*Mailbox, error) {
	boxes, err := c.ListMailboxes(ctx)
	if err != nil {
		return nil, err
	}
	for i := range boxes {
		if boxes[i].Login == login {
			return &boxes[i], nil
		}
	}
	return nil, nil
}

func (c *SMTP4DevClient) CreateMailbox(ctx context.Context, name, login, password string) (*Mailbox, error) {
	req := createMailboxRequest{Name: name, Login: login, Password: password}
	var box Mailbox
	if err := c.do(ctx, http.MethodPost, "/mailboxes", req, &box); err != nil {
		return nil, err
	}
	return &box, nil
}

func (c *SMTP4DevClient) DeleteMailbox(ctx context.Context, id string) error {
	return c.do(ctx, http.MethodDelete, fmt.Sprintf("/mailboxes/%s", url.PathEscape(id)), nil, nil)
}

func (c *SMTP4DevClient) DeleteAllMessages(ctx context.Context) error {
	return c.do(ctx, http.MethodDelete, "/messages", nil, nil)
}

func (c *SMTP4DevClient) ListMessages(ctx context.Context, mailboxID string) ([]Message, error) {
	path := "/messages"
	if mailboxID != "" {
		path += "?mailboxId=" + url.QueryEscape(mailboxID)
	}
	var msgs []Message
	if err := c.do(ctx, http.MethodGet, path, nil, &msgs); err != nil {
		return nil, err
	}
	return msgs, nil
}

func (c *SMTP4DevClient) do(ctx context.Context, method, path string, body interface{}, out interface{}) error {
	var reader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return err
		}
		reader = bytes.NewReader(data)
	}

	req, err := http.NewRequestWithContext(ctx, method, c.base+path, reader)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode >= 300 {
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		return fmt.Errorf("smtp4dev %s %s failed: %s (%s)", method, path, resp.Status, string(b))
	}

	if out == nil {
		return nil
	}
	dec := json.NewDecoder(resp.Body)
	return dec.Decode(out)
}

func trimTrailingSlash(base string) string {
	for len(base) > 0 && base[len(base)-1] == '/' {
		base = base[:len(base)-1]
	}
	return base
}
