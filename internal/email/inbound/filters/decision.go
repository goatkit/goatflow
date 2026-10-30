package filters

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"
	"unicode/utf8"

	gomail "github.com/emersion/go-message/mail"

	"github.com/goatkit/goatflow/internal/platform/sysconfig"
)

// PluginCaller invokes a plugin function. It matches plugin.Manager.Call so
// this package does not import the plugin runtime.
type PluginCaller interface {
	Call(ctx context.Context, pluginName, fn string, args []byte) ([]byte, error)
}

const (
	decisionPlugin   = "goatkit-llm"
	decisionFunction = "llm_decide"

	// decisionBodyChars bounds the body text sent as state: enough to
	// classify an email, small enough for a 4B decision model's context and
	// a short request on a serial server.
	decisionBodyChars = 4000
	decisionBodyBytes = 64 * 1024

	// decisionPollReserve is left of the poll deadline for ticket creation
	// after the decision; a call that would eat into it is skipped.
	decisionPollReserve = 15 * time.Second
	decisionMinCall     = 2 * time.Second
)

// decisionFields maps each question to the annotation its answer sets and
// the annotations that mean an explicit rule already decided it.
var decisionFields = []struct {
	name         string
	annotation   string
	decidedBy    []string
	instructions string
	field        func(*sysconfig.DecisionFilterConfig) sysconfig.DecisionFilterField
}{
	{
		name:         "queue",
		annotation:   AnnotationQueueNameOverride,
		decidedBy:    []string{AnnotationQueueIDOverride, AnnotationQueueNameOverride},
		instructions: "Which queue should handle this email?",
		field:        func(c *sysconfig.DecisionFilterConfig) sysconfig.DecisionFilterField { return c.Queue },
	},
	{
		name:         "priority",
		annotation:   AnnotationPriorityNameOverride,
		decidedBy:    []string{AnnotationPriorityIDOverride, AnnotationPriorityNameOverride},
		instructions: "What priority should this email get?",
		field:        func(c *sysconfig.DecisionFilterConfig) sysconfig.DecisionFilterField { return c.Priority },
	},
	{
		name:         "type",
		annotation:   AnnotationTypeOverride,
		decidedBy:    []string{AnnotationTypeOverride},
		instructions: "What type of request is this email?",
		field:        func(c *sysconfig.DecisionFilterConfig) sysconfig.DecisionFilterField { return c.Type },
	},
}

// DecisionFilter classifies new inbound mail with goatkit-llm's llm_decide:
// one choice question per enabled field (queue, priority, type), answered in
// one request. Answers at or above the configured confidence set the same
// name annotations a postmaster filter rule would; anything else leaves the
// message on normal routing.
//
// It fails open: a disabled or missing plugin, a timeout, or an upstream
// error is logged and the message continues unchanged, so mail intake never
// depends on the decision provider.
type DecisionFilter struct {
	plugins func() PluginCaller
	config  func() (sysconfig.DecisionFilterConfig, error)
	logger  *log.Logger
	body    *BodyTokenFilter
}

// NewDecisionFilter builds the filter. plugins is read on every message, so
// the plugin manager may be created after the filter chain.
func NewDecisionFilter(plugins func() PluginCaller, config func() (sysconfig.DecisionFilterConfig, error), logger *log.Logger) *DecisionFilter {
	return &DecisionFilter{
		plugins: plugins,
		config:  config,
		logger:  logger,
		body:    &BodyTokenFilter{logger: logger, limit: decisionBodyBytes},
	}
}

// ID implements Filter.
func (f *DecisionFilter) ID() string { return "decision" }

// Apply implements Filter. It never returns an error.
func (f *DecisionFilter) Apply(ctx context.Context, m *MessageContext) error {
	if m == nil || m.Message == nil || len(m.Message.Raw) == 0 {
		return nil
	}
	if annotationSet(m, AnnotationIgnoreMessage) || annotationSet(m, AnnotationFollowUpTicketNumber) {
		return nil
	}
	cfg, err := f.config()
	if err != nil {
		f.logf("decision: config unreadable, skipped: %v", err)
		return nil
	}
	if !cfg.Enabled {
		return nil
	}

	questions, asked := f.questions(&cfg, m)
	if len(asked) == 0 {
		return nil
	}
	var caller PluginCaller
	if f.plugins != nil {
		caller = f.plugins()
	}
	if caller == nil {
		f.logf("decision: plugin manager unavailable, skipped %s", m.Message.UID)
		return nil
	}

	timeout := time.Duration(cfg.TimeoutSeconds) * time.Second
	if deadline, ok := ctx.Deadline(); ok {
		if left := time.Until(deadline) - decisionPollReserve; left < timeout {
			timeout = left
		}
	}
	if timeout < decisionMinCall {
		f.logf("decision: poll deadline too close, skipped %s", m.Message.UID)
		return nil
	}

	args, err := f.request(&cfg, m.Message.Raw, questions, timeout)
	if err != nil {
		f.logf("decision: build request for %s: %v", m.Message.UID, err)
		return nil
	}
	callCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	start := time.Now()
	raw, err := caller.Call(callCtx, decisionPlugin, decisionFunction, args)
	if err != nil {
		f.logf("decision: %s.%s failed for %s after %s, skipped: %v", decisionPlugin, decisionFunction, m.Message.UID, time.Since(start).Round(time.Millisecond), err)
		return nil
	}
	answers, err := parseDecisionAnswers(raw)
	if err != nil {
		f.logf("decision: %s for %s, skipped: %v", decisionFunction, m.Message.UID, err)
		return nil
	}
	f.apply(&cfg, m, asked, answers, time.Since(start))
	return nil
}

// questions renders the System One questions for every enabled field that
// no earlier filter has decided, and returns the indexes it asked.
func (f *DecisionFilter) questions(cfg *sysconfig.DecisionFilterConfig, m *MessageContext) (json.RawMessage, []int) {
	var buf bytes.Buffer
	var asked []int
	buf.WriteByte('{')
	for i, d := range decisionFields {
		field := d.field(cfg)
		if !field.Enabled || len(field.Options) < 2 || decided(m, d.decidedBy) {
			continue
		}
		instructions := field.Instructions
		if instructions == "" {
			instructions = d.instructions
		}
		if len(asked) > 0 {
			buf.WriteByte(',')
		}
		asked = append(asked, i)
		writeJSON(&buf, d.name)
		buf.WriteString(`:{"type":"choice","instructions":`)
		writeJSON(&buf, instructions)
		// criteria is an object in option order: the order fixes the option
		// letters, so it must not go through a (sorted) Go map.
		buf.WriteString(`,"criteria":{`)
		for j, o := range field.Options {
			if j > 0 {
				buf.WriteByte(',')
			}
			writeJSON(&buf, o.Name)
			buf.WriteByte(':')
			if o.Description == "" {
				buf.WriteString("null")
			} else {
				writeJSON(&buf, o.Description)
			}
		}
		buf.WriteString("}}")
	}
	buf.WriteByte('}')
	return buf.Bytes(), asked
}

func (f *DecisionFilter) request(cfg *sysconfig.DecisionFilterConfig, raw []byte, questions json.RawMessage, timeout time.Duration) ([]byte, error) {
	state := struct {
		From    string `json:"from,omitempty"`
		Subject string `json:"subject,omitempty"`
		Body    string `json:"body,omitempty"`
	}{Body: truncateRunes(strings.TrimSpace(f.body.extractBody(raw)), decisionBodyChars)}
	if r, err := gomail.CreateReader(bytes.NewReader(raw)); err == nil {
		state.Subject, _ = r.Header.Subject()
		if from, ferr := r.Header.AddressList("From"); ferr == nil && len(from) > 0 {
			state.From = from[0].String()
		}
	}
	req := struct {
		ProviderID     int64           `json:"provider_id,omitempty"`
		TimeoutSeconds int             `json:"timeout_seconds"`
		State          any             `json:"state"`
		Questions      json.RawMessage `json:"questions"`
	}{cfg.ProviderID, int(timeout / time.Second), state, questions}
	if req.TimeoutSeconds < 1 {
		req.TimeoutSeconds = 1
	}
	return json.Marshal(req)
}

type decisionAnswer struct {
	Choice     string  `json:"choice"`
	Confidence float64 `json:"confidence"`
}

// parseDecisionAnswers reads llm_decide's response. goatkit-llm reports
// failures in the body ({"error": ..., "status": ...}) rather than as a call
// error.
func parseDecisionAnswers(raw []byte) (map[string]decisionAnswer, error) {
	var resp struct {
		Answers map[string]decisionAnswer `json:"answers"`
		Error   string                    `json:"error"`
		Status  int                       `json:"status"`
	}
	if err := json.Unmarshal(raw, &resp); err != nil {
		return nil, fmt.Errorf("unreadable response: %w", err)
	}
	if resp.Error != "" {
		return nil, fmt.Errorf("status %d: %s", resp.Status, resp.Error)
	}
	if resp.Answers == nil {
		return nil, fmt.Errorf("response has no answers")
	}
	return resp.Answers, nil
}

func (f *DecisionFilter) apply(cfg *sysconfig.DecisionFilterConfig, m *MessageContext, asked []int, answers map[string]decisionAnswer, took time.Duration) {
	if m.Annotations == nil {
		m.Annotations = make(map[string]any)
	}
	parts := make([]string, 0, len(asked))
	for _, i := range asked {
		d := decisionFields[i]
		a, ok := answers[d.name]
		if !ok {
			parts = append(parts, d.name+"=missing")
			continue
		}
		verdict := "below threshold"
		switch {
		case !hasOption(d.field(cfg), a.Choice):
			verdict = "not a configured option"
		case a.Confidence >= cfg.Threshold:
			m.Annotations[d.annotation] = a.Choice
			verdict = "applied"
		}
		parts = append(parts, fmt.Sprintf("%s=%q confidence %.2f %s", d.name, a.Choice, a.Confidence, verdict))
	}
	f.logf("decision: %s in %s (threshold %.2f): %s", m.Message.UID, took.Round(time.Millisecond), cfg.Threshold, strings.Join(parts, ", "))
}

func hasOption(field sysconfig.DecisionFilterField, name string) bool {
	for _, o := range field.Options {
		if o.Name == name {
			return true
		}
	}
	return false
}

func decided(m *MessageContext, keys []string) bool {
	for _, k := range keys {
		if annotationSet(m, k) {
			return true
		}
	}
	return false
}

// annotationSet reports a present, non-empty, non-false annotation.
func annotationSet(m *MessageContext, key string) bool {
	if m.Annotations == nil {
		return false
	}
	switch v := m.Annotations[key].(type) {
	case nil:
		return false
	case bool:
		return v
	case string:
		return strings.TrimSpace(v) != ""
	default:
		return true
	}
}

func writeJSON(buf *bytes.Buffer, s string) {
	b, _ := json.Marshal(s)
	buf.Write(b)
}

func truncateRunes(s string, n int) string {
	if utf8.RuneCountInString(s) <= n {
		return s
	}
	r := []rune(s)
	return string(r[:n])
}

func (f *DecisionFilter) logf(format string, args ...any) {
	if f.logger != nil {
		f.logger.Printf(format, args...)
	}
}
