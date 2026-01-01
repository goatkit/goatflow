// Package mailaccountmeta provides mail account metadata parsing and serialization.
package mailaccountmeta

import (
	"encoding/json"
	"strings"
)

const Separator = "\n\n-- GOTRS META --\n"

type Metadata struct {
	DispatchingMode     string `json:"dispatching_mode,omitempty"`
	AllowTrustedHeaders *bool  `json:"allow_trusted_headers,omitempty"`
	PollIntervalSeconds *int   `json:"poll_interval_seconds,omitempty"`
}

func EncodeComment(comment string, meta Metadata) string {
	if meta.isZero() {
		return comment
	}
	payload, err := json.Marshal(meta)
	if err != nil {
		return comment
	}
	trimmed := strings.TrimRight(comment, "\n")
	return trimmed + Separator + string(payload)
}

func DecodeComment(raw string) (string, Metadata) {
	var meta Metadata
	if raw == "" {
		return "", meta
	}
	idx := strings.LastIndex(raw, Separator)
	if idx == -1 {
		return raw, meta
	}
	jsonCandidate := strings.TrimSpace(raw[idx+len(Separator):])
	if jsonCandidate == "" || !strings.HasPrefix(jsonCandidate, "{") {
		return raw, Metadata{}
	}
	if err := json.Unmarshal([]byte(jsonCandidate), &meta); err != nil {
		return raw, Metadata{}
	}
	comment := strings.TrimRight(raw[:idx], "\n")
	return comment, meta
}

func (m Metadata) isZero() bool {
	return m.DispatchingMode == "" && m.AllowTrustedHeaders == nil && m.PollIntervalSeconds == nil
}
