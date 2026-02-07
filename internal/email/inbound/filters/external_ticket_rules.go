package filters

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"
)

// ExternalTicketRule defines a regex that extracts a GoatFlow ticket number from partner emails.
type ExternalTicketRule struct {
	Name          string
	Pattern       *regexp.Regexp
	SearchSubject bool
	SearchBody    bool
	Headers       []string
}

// LoadExternalTicketRules loads regex rules from a YAML file. Missing files return an empty list.
func LoadExternalTicketRules(path string) ([]ExternalTicketRule, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return nil, nil
	}
	data, err := os.ReadFile(filepath.Clean(path))
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}
	var cfg externalTicketRuleConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}
	rules := cfg.compile()
	if len(rules) == 0 {
		return nil, errors.New("external ticket rule config has no valid entries")
	}
	return rules, nil
}

type externalTicketRuleConfig struct {
	Rules []externalTicketRuleEntry `yaml:"rules"`
}

type externalTicketRuleEntry struct {
	Name          string   `yaml:"name"`
	Pattern       string   `yaml:"pattern"`
	SearchSubject bool     `yaml:"search_subject"`
	SearchBody    bool     `yaml:"search_body"`
	Headers       []string `yaml:"headers"`
}

func (cfg externalTicketRuleConfig) compile() []ExternalTicketRule {
	rules := make([]ExternalTicketRule, 0, len(cfg.Rules))
	for _, entry := range cfg.Rules {
		pattern := strings.TrimSpace(entry.Pattern)
		if pattern == "" {
			continue
		}
		re, err := regexp.Compile(pattern)
		if err != nil {
			continue
		}
		rule := ExternalTicketRule{
			Name:          strings.TrimSpace(entry.Name),
			Pattern:       re,
			SearchSubject: entry.SearchSubject,
			SearchBody:    entry.SearchBody,
		}
		for _, header := range entry.Headers {
			header = strings.TrimSpace(header)
			if header == "" {
				continue
			}
			rule.Headers = append(rule.Headers, header)
		}
		if !rule.SearchSubject && !rule.SearchBody && len(rule.Headers) == 0 {
			continue
		}
		rules = append(rules, rule)
	}
	return rules
}
