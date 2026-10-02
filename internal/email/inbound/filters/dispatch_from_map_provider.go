package filters

import (
	"errors"
	"io/fs"
	"os"
	"strconv"
	"strings"
	"sync"

	"gopkg.in/yaml.v3"
)

// FileDispatchRuleProvider loads dispatch rules from a YAML document.
type FileDispatchRuleProvider struct {
	mu    sync.RWMutex
	rules map[int][]DispatchRule
}

// NewFileDispatchRuleProvider loads rules from the provided path. Missing files return (nil, nil).
func NewFileDispatchRuleProvider(path string) (*FileDispatchRuleProvider, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return nil, nil //nolint:nilnil
	}
	data, err := os.ReadFile(path) // #nosec G304 -- dispatch-map path comes from operator config, not request input
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, nil //nolint:nilnil
		}
		return nil, err
	}
	var cfg dispatchConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}
	provider := &FileDispatchRuleProvider{rules: make(map[int][]DispatchRule)}
	provider.applyConfig(cfg)
	return provider, nil
}

// RulesFor implements DispatchRuleProvider.
func (p *FileDispatchRuleProvider) RulesFor(accountID int) []DispatchRule {
	if p == nil {
		return nil
	}
	p.mu.RLock()
	defer p.mu.RUnlock()
	if rules, ok := p.rules[accountID]; ok {
		copyRules := make([]DispatchRule, len(rules))
		copy(copyRules, rules)
		return copyRules
	}
	return nil
}

func (p *FileDispatchRuleProvider) applyConfig(cfg dispatchConfig) {
	p.mu.Lock()
	defer p.mu.Unlock()
	for key, list := range cfg.Accounts {
		id, err := strconv.Atoi(strings.TrimSpace(key))
		if err != nil || id <= 0 {
			continue
		}
		for idx := range list {
			list[idx].Match = strings.TrimSpace(list[idx].Match)
			list[idx].QueueName = strings.TrimSpace(list[idx].QueueName)
		}
		p.rules[id] = append([]DispatchRule(nil), list...)
	}
}

type dispatchConfig struct {
	Accounts map[string][]DispatchRule `yaml:"accounts"`
}
