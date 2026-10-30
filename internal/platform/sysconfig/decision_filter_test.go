package sysconfig

import (
	"errors"
	"strings"
	"testing"
)

func validDecisionConfig() DecisionFilterConfig {
	cfg := DefaultDecisionFilterConfig()
	cfg.Enabled = true
	cfg.Queue = DecisionFilterField{Enabled: true, Options: []DecisionFilterOption{{Name: "Billing"}, {Name: "Raw"}}}
	return cfg
}

func TestDecisionFilterNormalizeDropsBlankAndDuplicateOptions(t *testing.T) {
	cfg := validDecisionConfig()
	cfg.Queue.Options = []DecisionFilterOption{
		{Name: " Raw ", Description: " fallback "}, {Name: ""}, {Name: "Billing"}, {Name: "Raw", Description: "second copy"},
	}
	cfg.Normalize()
	got := cfg.Queue.Options
	if len(got) != 2 || got[0] != (DecisionFilterOption{Name: "Raw", Description: "fallback"}) || got[1].Name != "Billing" {
		t.Fatalf("options = %+v, want [Raw(fallback) Billing] in order", got)
	}
}

func TestDecisionFilterValidate(t *testing.T) {
	tooMany := validDecisionConfig()
	tooMany.Queue.Options = nil
	for i := 0; i <= DecisionFilterMaxOptions; i++ {
		tooMany.Queue.Options = append(tooMany.Queue.Options, DecisionFilterOption{Name: string(rune('a'+i%26)) + strings.Repeat("x", i)})
	}
	cases := map[string]struct {
		mutate func(*DecisionFilterConfig)
		want   string
	}{
		"valid":                 {func(*DecisionFilterConfig) {}, ""},
		"disabled with nothing": {func(c *DecisionFilterConfig) { c.Enabled = false; c.Queue = DecisionFilterField{} }, ""},
		"enabled with nothing":  {func(c *DecisionFilterConfig) { c.Queue.Enabled = false }, "at least one"},
		"one option":            {func(c *DecisionFilterConfig) { c.Queue.Options = c.Queue.Options[:1] }, "queue needs between 2"},
		"too many options":      {func(c *DecisionFilterConfig) { *c = tooMany }, "queue needs between 2"},
		"threshold above 1":     {func(c *DecisionFilterConfig) { c.Threshold = 1.5 }, "threshold"},
		"threshold 0 allowed":   {func(c *DecisionFilterConfig) { c.Threshold = 0 }, ""},
		"timeout 0":             {func(c *DecisionFilterConfig) { c.TimeoutSeconds = 0 }, "timeout_seconds"},
		"timeout too long":      {func(c *DecisionFilterConfig) { c.TimeoutSeconds = DecisionFilterMaxTimeout + 1 }, "timeout_seconds"},
		"disabled field with one option": {func(c *DecisionFilterConfig) {
			c.Type = DecisionFilterField{Enabled: false, Options: []DecisionFilterOption{{Name: "Incident"}}}
		}, ""},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			cfg := validDecisionConfig()
			c.mutate(&cfg)
			cfg.Normalize()
			err := cfg.Validate()
			if c.want == "" {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), c.want) {
				t.Fatalf("err = %v, want containing %q", err, c.want)
			}
		})
	}
}

func TestSaveDecisionFilterConfigValidatesBeforeStorage(t *testing.T) {
	invalid := validDecisionConfig()
	invalid.Queue.Options = invalid.Queue.Options[:1]
	// With no database, an invalid config must still be reported as the
	// caller's mistake (a 400), not as storage being unavailable.
	var verr *DecisionFilterValidationError
	if err := SaveDecisionFilterConfig(nil, invalid, 1); !errors.As(err, &verr) {
		t.Fatalf("invalid config: err = %v, want a validation error", err)
	}
	if err := SaveDecisionFilterConfig(nil, validDecisionConfig(), 1); err == nil || errors.As(err, &verr) {
		t.Fatalf("valid config, no db: err = %v, want a storage error", err)
	}
}
