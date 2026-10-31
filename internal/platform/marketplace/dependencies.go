package marketplace

import (
	"fmt"
	"strings"
)

// DependencyStep is one plugin a marketplace install puts in place because
// another plugin needs it.
type DependencyStep struct {
	Entry      *PluginEntry
	Version    VersionEntry
	RequiredBy string
}

// PlanDependencies lists the plugins entry needs that are not installed yet,
// directly or through another dependency, each at the newest version this
// GoatFlow can run, in install order (a plugin's dependencies before it). It
// fails before anything is downloaded when a dependency is not in the index,
// has no version this GoatFlow can run, or the dependencies form a cycle. An
// installed plugin satisfies a dependency at any version.
func (c *Client) PlanDependencies(entry *PluginEntry) ([]DependencyStep, error) {
	if len(entry.Dependencies) == 0 {
		return nil, nil
	}
	index, err := c.FetchIndex()
	if err != nil {
		return nil, err
	}
	installed, err := c.ListInstalled()
	if err != nil {
		return nil, err
	}
	have := make(map[string]bool, len(installed))
	for _, inst := range installed {
		have[inst.Name] = true
	}
	byName := make(map[string]*PluginEntry, len(index.Plugins))
	for i := range index.Plugins {
		byName[index.Plugins[i].Name] = &index.Plugins[i]
	}

	const visiting, planned = 1, 2
	state := map[string]int{entry.Name: visiting}
	var steps []DependencyStep
	var visit func(p *PluginEntry, chain []string) error
	visit = func(p *PluginEntry, chain []string) error {
		for _, dep := range p.Dependencies {
			if have[dep] || state[dep] == planned {
				continue
			}
			if state[dep] == visiting {
				return fmt.Errorf("circular plugin dependency: %s -> %s", strings.Join(chain, " -> "), dep)
			}
			d := byName[dep]
			if d == nil {
				return fmt.Errorf("%s needs plugin %q, which is not in the marketplace", p.Name, dep)
			}
			v, err := d.ResolveInstall()
			if err != nil {
				return fmt.Errorf("%s needs plugin %s: %w", p.Name, dep, err)
			}
			state[dep] = visiting
			if err := visit(d, append(chain, dep)); err != nil {
				return err
			}
			state[dep] = planned
			steps = append(steps, DependencyStep{Entry: d, Version: v, RequiredBy: p.Name})
		}
		return nil
	}
	if err := visit(entry, []string{entry.Name}); err != nil {
		return nil, err
	}
	return steps, nil
}

// InstallDependencies installs steps in order and returns the ones installed.
// It stops at the first failure; dependencies installed before it stay, as
// each one works on its own.
func (c *Client) InstallDependencies(steps []DependencyStep) ([]DependencyStep, error) {
	done := make([]DependencyStep, 0, len(steps))
	for _, s := range steps {
		if err := c.installVersion(s.Entry, s.Version.Version); err != nil {
			return done, fmt.Errorf("install dependency %s v%s (needed by %s): %w", s.Entry.Name, s.Version.Version, s.RequiredBy, err)
		}
		done = append(done, s)
	}
	return done, nil
}
