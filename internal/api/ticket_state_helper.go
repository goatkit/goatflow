package api

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/goatkit/goatflow/internal/config"
	"github.com/goatkit/goatflow/internal/models"
	"github.com/goatkit/goatflow/internal/repository"
)

func resolveTicketState(repo *repository.TicketRepository, nextState string, nextStateID int) (int, *models.TicketState, error) {
	if repo == nil {
		return 0, nil, fmt.Errorf("nil ticket repository")
	}

	if nextStateID > 0 {
		return nextStateID, nil, nil
	}

	trimmed := strings.TrimSpace(nextState)
	if trimmed == "" {
		return 0, nil, nil
	}

	if id, err := strconv.Atoi(trimmed); err == nil {
		return id, nil, nil
	}

	aliases := normalizeStateAliases(trimmed)
	states, err := repo.GetTicketStates()
	if err != nil {
		return 0, nil, err
	}
	for _, st := range states {
		if aliasMatch(aliases, normalizeStateAliases(st.Name)) {
			stateCopy := st
			return int(stateCopy.ID), &stateCopy, nil
		}
	}
	return 0, nil, nil
}

func normalizeStateAliases(value string) []string {
	base := strings.ToLower(strings.TrimSpace(value))
	if base == "" {
		return nil
	}
	collapsed := strings.Join(strings.Fields(base), " ")
	aliasSet := map[string]struct{}{
		collapsed: {},
	}
	variants := []string{
		strings.ReplaceAll(collapsed, " ", "_"),
		strings.ReplaceAll(collapsed, " ", "-"),
		strings.ReplaceAll(collapsed, "-", " "),
		strings.ReplaceAll(collapsed, "_", " "),
		strings.ReplaceAll(collapsed, "-", "_"),
		strings.ReplaceAll(collapsed, "_", "-"),
	}
	for _, v := range variants {
		v = strings.TrimSpace(v)
		if v == "" {
			continue
		}
		aliasSet[v] = struct{}{}
	}
	aliases := make([]string, 0, len(aliasSet))
	for alias := range aliasSet {
		aliases = append(aliases, alias)
	}
	return aliases
}

func aliasMatch(left, right []string) bool {
	if len(left) == 0 || len(right) == 0 {
		return false
	}
	for _, l := range left {
		for _, r := range right {
			if l == r {
				return true
			}
		}
	}
	return false
}

func parsePendingUntil(value string) int {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return 0
	}
	if t, err := time.Parse(time.RFC3339, trimmed); err == nil {
		return int(t.Unix())
	}

	loc := time.UTC
	if cfg := config.Get(); cfg != nil && cfg.App.Timezone != "" {
		if tz, err := time.LoadLocation(cfg.App.Timezone); err == nil {
			loc = tz
		}
	}

	layouts := []string{
		"2006-01-02T15:04",
		"2006-01-02 15:04",
		"2006-01-02T15:04:05",
		"2006-01-02 15:04:05",
	}
	for _, layout := range layouts {
		if t, err := time.ParseInLocation(layout, trimmed, loc); err == nil {
			return int(t.In(time.UTC).Unix())
		}
	}
	return 0
}

func isPendingState(state *models.TicketState) bool {
	if state == nil {
		return false
	}
	return state.TypeID == 4 || state.TypeID == 5
}

func loadTicketState(repo *repository.TicketRepository, stateID int) (*models.TicketState, error) {
	if repo == nil || stateID <= 0 {
		return nil, nil //nolint:nilnil
	}
	st, err := repo.GetTicketStateByID(stateID)
	if err != nil {
		return nil, err
	}
	return st, nil
}
