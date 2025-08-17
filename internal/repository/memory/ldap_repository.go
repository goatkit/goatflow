package memory

import (
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/gotrs-io/gotrs-ce/internal/models"
	"github.com/gotrs-io/gotrs-ce/internal/repository"
)

// LDAPRepository is an in-memory implementation of LDAPRepository
type LDAPRepository struct {
	mu               sync.RWMutex
	configs          map[int]*models.LDAPConfiguration
	syncHistory      map[int]*models.LDAPSyncHistory
	userMappings     map[int]*models.LDAPUserMapping
	groupMappings    map[int]*models.LDAPGroupMapping
	authLogs         map[int]*models.LDAPAuthenticationLog
	connectionTests  []*models.LDAPConnectionTest
	nextConfigID     int
	nextSyncID       int
	nextUserMapID    int
	nextGroupMapID   int
	nextAuthLogID    int
}

// NewLDAPRepository creates a new in-memory LDAP repository
func NewLDAPRepository() repository.LDAPRepository {
	return &LDAPRepository{
		configs:         make(map[int]*models.LDAPConfiguration),
		syncHistory:     make(map[int]*models.LDAPSyncHistory),
		userMappings:    make(map[int]*models.LDAPUserMapping),
		groupMappings:   make(map[int]*models.LDAPGroupMapping),
		authLogs:        make(map[int]*models.LDAPAuthenticationLog),
		connectionTests: make([]*models.LDAPConnectionTest, 0),
		nextConfigID:    1,
		nextSyncID:      1,
		nextUserMapID:   1,
		nextGroupMapID:  1,
		nextAuthLogID:   1,
	}
}

// Configuration management

func (r *LDAPRepository) CreateConfig(config *models.LDAPConfiguration) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	config.ID = r.nextConfigID
	r.nextConfigID++
	config.CreatedAt = time.Now()
	config.UpdatedAt = time.Now()

	r.configs[config.ID] = config
	return nil
}

func (r *LDAPRepository) UpdateConfig(config *models.LDAPConfiguration) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.configs[config.ID]; !exists {
		return fmt.Errorf("configuration not found")
	}

	config.UpdatedAt = time.Now()
	r.configs[config.ID] = config
	return nil
}

func (r *LDAPRepository) GetConfigByID(id int) (*models.LDAPConfiguration, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	config, exists := r.configs[id]
	if !exists {
		return nil, fmt.Errorf("configuration not found")
	}

	return config, nil
}

func (r *LDAPRepository) GetActiveConfig() (*models.LDAPConfiguration, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	for _, config := range r.configs {
		if config.IsActive {
			return config, nil
		}
	}

	return nil, fmt.Errorf("no active configuration found")
}

func (r *LDAPRepository) ListConfigs() ([]*models.LDAPConfiguration, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	configs := make([]*models.LDAPConfiguration, 0, len(r.configs))
	for _, config := range r.configs {
		configs = append(configs, config)
	}

	// Sort by ID
	sort.Slice(configs, func(i, j int) bool {
		return configs[i].ID < configs[j].ID
	})

	return configs, nil
}

func (r *LDAPRepository) DeleteConfig(id int) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.configs[id]; !exists {
		return fmt.Errorf("configuration not found")
	}

	delete(r.configs, id)
	return nil
}

// Sync history

func (r *LDAPRepository) CreateSyncHistory(history *models.LDAPSyncHistory) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	history.ID = r.nextSyncID
	r.nextSyncID++
	history.CreatedAt = time.Now()

	r.syncHistory[history.ID] = history
	return nil
}

func (r *LDAPRepository) UpdateSyncHistory(history *models.LDAPSyncHistory) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.syncHistory[history.ID]; !exists {
		return fmt.Errorf("sync history not found")
	}

	r.syncHistory[history.ID] = history
	return nil
}

func (r *LDAPRepository) GetSyncHistoryByID(id int) (*models.LDAPSyncHistory, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	history, exists := r.syncHistory[id]
	if !exists {
		return nil, fmt.Errorf("sync history not found")
	}

	return history, nil
}

func (r *LDAPRepository) ListSyncHistory(configID int, limit, offset int) ([]*models.LDAPSyncHistory, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var histories []*models.LDAPSyncHistory
	for _, history := range r.syncHistory {
		if history.ConfigID == configID {
			histories = append(histories, history)
		}
	}

	// Sort by start time descending
	sort.Slice(histories, func(i, j int) bool {
		return histories[i].StartTime.After(histories[j].StartTime)
	})

	// Apply pagination
	start := offset
	if start > len(histories) {
		return []*models.LDAPSyncHistory{}, nil
	}

	end := start + limit
	if end > len(histories) {
		end = len(histories)
	}

	return histories[start:end], nil
}

func (r *LDAPRepository) GetLatestSyncHistory(configID int) (*models.LDAPSyncHistory, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var latest *models.LDAPSyncHistory
	for _, history := range r.syncHistory {
		if history.ConfigID == configID {
			if latest == nil || history.StartTime.After(latest.StartTime) {
				latest = history
			}
		}
	}

	if latest == nil {
		return nil, fmt.Errorf("no sync history found")
	}

	return latest, nil
}

func (r *LDAPRepository) DeleteOldSyncHistory(configID int, keepDays int) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	cutoff := time.Now().AddDate(0, 0, -keepDays)

	for id, history := range r.syncHistory {
		if history.ConfigID == configID && history.StartTime.Before(cutoff) {
			delete(r.syncHistory, id)
		}
	}

	return nil
}

// User mappings

func (r *LDAPRepository) CreateUserMapping(mapping *models.LDAPUserMapping) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	mapping.ID = r.nextUserMapID
	r.nextUserMapID++
	mapping.CreatedAt = time.Now()
	mapping.UpdatedAt = time.Now()

	r.userMappings[mapping.ID] = mapping
	return nil
}

func (r *LDAPRepository) UpdateUserMapping(mapping *models.LDAPUserMapping) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.userMappings[mapping.ID]; !exists {
		return fmt.Errorf("user mapping not found")
	}

	mapping.UpdatedAt = time.Now()
	r.userMappings[mapping.ID] = mapping
	return nil
}

func (r *LDAPRepository) GetUserMappingByUserID(userID int) (*models.LDAPUserMapping, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	for _, mapping := range r.userMappings {
		if mapping.UserID == userID {
			return mapping, nil
		}
	}

	return nil, fmt.Errorf("user mapping not found")
}

func (r *LDAPRepository) GetUserMappingByLDAPDN(configID int, dn string) (*models.LDAPUserMapping, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	for _, mapping := range r.userMappings {
		if mapping.ConfigID == configID && mapping.LDAPUserDN == dn {
			return mapping, nil
		}
	}

	return nil, fmt.Errorf("user mapping not found")
}

func (r *LDAPRepository) GetUserMappingByLDAPGUID(configID int, guid string) (*models.LDAPUserMapping, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	for _, mapping := range r.userMappings {
		if mapping.ConfigID == configID && mapping.LDAPObjectGUID == guid {
			return mapping, nil
		}
	}

	return nil, fmt.Errorf("user mapping not found")
}

func (r *LDAPRepository) ListUserMappings(configID int) ([]*models.LDAPUserMapping, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var mappings []*models.LDAPUserMapping
	for _, mapping := range r.userMappings {
		if mapping.ConfigID == configID {
			mappings = append(mappings, mapping)
		}
	}

	return mappings, nil
}

func (r *LDAPRepository) DeleteUserMapping(id int) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.userMappings[id]; !exists {
		return fmt.Errorf("user mapping not found")
	}

	delete(r.userMappings, id)
	return nil
}

// Group mappings

func (r *LDAPRepository) CreateGroupMapping(mapping *models.LDAPGroupMapping) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	mapping.ID = r.nextGroupMapID
	r.nextGroupMapID++
	mapping.CreatedAt = time.Now()
	mapping.UpdatedAt = time.Now()

	r.groupMappings[mapping.ID] = mapping
	return nil
}

func (r *LDAPRepository) UpdateGroupMapping(mapping *models.LDAPGroupMapping) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.groupMappings[mapping.ID]; !exists {
		return fmt.Errorf("group mapping not found")
	}

	mapping.UpdatedAt = time.Now()
	r.groupMappings[mapping.ID] = mapping
	return nil
}

func (r *LDAPRepository) GetGroupMappingByGroupID(groupID int) (*models.LDAPGroupMapping, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	for _, mapping := range r.groupMappings {
		if mapping.GroupID == groupID {
			return mapping, nil
		}
	}

	return nil, fmt.Errorf("group mapping not found")
}

func (r *LDAPRepository) GetGroupMappingByLDAPDN(configID int, dn string) (*models.LDAPGroupMapping, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	for _, mapping := range r.groupMappings {
		if mapping.ConfigID == configID && mapping.LDAPGroupDN == dn {
			return mapping, nil
		}
	}

	return nil, fmt.Errorf("group mapping not found")
}

func (r *LDAPRepository) GetGroupMappingByLDAPGUID(configID int, guid string) (*models.LDAPGroupMapping, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	for _, mapping := range r.groupMappings {
		if mapping.ConfigID == configID && mapping.LDAPObjectGUID == guid {
			return mapping, nil
		}
	}

	return nil, fmt.Errorf("group mapping not found")
}

func (r *LDAPRepository) ListGroupMappings(configID int) ([]*models.LDAPGroupMapping, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var mappings []*models.LDAPGroupMapping
	for _, mapping := range r.groupMappings {
		if mapping.ConfigID == configID {
			mappings = append(mappings, mapping)
		}
	}

	return mappings, nil
}

func (r *LDAPRepository) DeleteGroupMapping(id int) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.groupMappings[id]; !exists {
		return fmt.Errorf("group mapping not found")
	}

	delete(r.groupMappings, id)
	return nil
}

// Authentication logs

func (r *LDAPRepository) CreateAuthLog(log *models.LDAPAuthenticationLog) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	log.ID = r.nextAuthLogID
	r.nextAuthLogID++
	log.CreatedAt = time.Now()

	r.authLogs[log.ID] = log
	return nil
}

func (r *LDAPRepository) GetAuthLogsByUsername(username string, limit int) ([]*models.LDAPAuthenticationLog, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var logs []*models.LDAPAuthenticationLog
	for _, log := range r.authLogs {
		if log.Username == username {
			logs = append(logs, log)
		}
	}

	// Sort by auth time descending
	sort.Slice(logs, func(i, j int) bool {
		return logs[i].AuthTime.After(logs[j].AuthTime)
	})

	// Apply limit
	if limit > 0 && len(logs) > limit {
		logs = logs[:limit]
	}

	return logs, nil
}

func (r *LDAPRepository) GetAuthLogsByTimeRange(configID int, startTime, endTime time.Time) ([]*models.LDAPAuthenticationLog, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var logs []*models.LDAPAuthenticationLog
	for _, log := range r.authLogs {
		if log.ConfigID == configID &&
			log.AuthTime.After(startTime) &&
			log.AuthTime.Before(endTime) {
			logs = append(logs, log)
		}
	}

	// Sort by auth time descending
	sort.Slice(logs, func(i, j int) bool {
		return logs[i].AuthTime.After(logs[j].AuthTime)
	})

	return logs, nil
}

func (r *LDAPRepository) DeleteOldAuthLogs(configID int, keepDays int) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	cutoff := time.Now().AddDate(0, 0, -keepDays)

	for id, log := range r.authLogs {
		if log.ConfigID == configID && log.AuthTime.Before(cutoff) {
			delete(r.authLogs, id)
		}
	}

	return nil
}

// Statistics

func (r *LDAPRepository) GetSyncStatistics(configID int) (*models.LDAPSyncStatistics, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	stats := &models.LDAPSyncStatistics{
		ConfigID: configID,
	}

	var totalDuration int64
	var durations []int64

	for _, history := range r.syncHistory {
		if history.ConfigID != configID {
			continue
		}

		stats.TotalSyncs++
		if history.Status == models.LDAPSyncStatusCompleted {
			stats.SuccessfulSyncs++
		} else if history.Status == models.LDAPSyncStatusFailed {
			stats.FailedSyncs++
		}

		stats.TotalUsersCreated += history.UsersCreated
		stats.TotalUsersUpdated += history.UsersUpdated
		stats.TotalGroupsCreated += history.GroupsCreated
		stats.TotalGroupsUpdated += history.GroupsUpdated

		if stats.LastSyncAt == nil || history.StartTime.After(*stats.LastSyncAt) {
			stats.LastSyncAt = &history.StartTime
		}

		if history.Status == models.LDAPSyncStatusCompleted {
			if stats.LastSuccessfulSync == nil || history.StartTime.After(*stats.LastSuccessfulSync) {
				stats.LastSuccessfulSync = &history.StartTime
			}
		}

		if history.Duration > 0 {
			totalDuration += history.Duration
			durations = append(durations, history.Duration)
		}
	}

	if len(durations) > 0 {
		stats.AverageDuration = totalDuration / int64(len(durations))
	}

	if stats.TotalSyncs > 0 {
		stats.ErrorRate = float64(stats.FailedSyncs) / float64(stats.TotalSyncs)
	}

	return stats, nil
}

func (r *LDAPRepository) GetAuthStatistics(configID int, days int) (map[string]interface{}, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	startTime := time.Now().AddDate(0, 0, -days)
	stats := map[string]interface{}{
		"total_attempts":      0,
		"successful_attempts": 0,
		"failed_attempts":     0,
		"unique_users":        make(map[string]bool),
		"success_rate":        0.0,
	}

	uniqueUsers := make(map[string]bool)

	for _, log := range r.authLogs {
		if log.ConfigID != configID || log.AuthTime.Before(startTime) {
			continue
		}

		stats["total_attempts"] = stats["total_attempts"].(int) + 1
		uniqueUsers[log.Username] = true

		if log.Success {
			stats["successful_attempts"] = stats["successful_attempts"].(int) + 1
		} else {
			stats["failed_attempts"] = stats["failed_attempts"].(int) + 1
		}
	}

	stats["unique_users"] = len(uniqueUsers)

	total := stats["total_attempts"].(int)
	if total > 0 {
		successful := stats["successful_attempts"].(int)
		stats["success_rate"] = float64(successful) / float64(total)
	}

	return stats, nil
}

// Testing

func (r *LDAPRepository) RecordConnectionTest(test *models.LDAPConnectionTest) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	test.TestedAt = time.Now()
	r.connectionTests = append(r.connectionTests, test)

	// Keep only last 100 tests
	if len(r.connectionTests) > 100 {
		r.connectionTests = r.connectionTests[len(r.connectionTests)-100:]
	}

	return nil
}