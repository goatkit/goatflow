package repository

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/goatkit/goatflow/internal/models"
)

func TestMemoryLookupRepository_Queues(t *testing.T) {
	ctx := context.Background()
	repo := NewMemoryLookupRepository()

	t.Run("GetQueues returns default queues", func(t *testing.T) {
		queues, err := repo.GetQueues(ctx)
		require.NoError(t, err)
		assert.NotEmpty(t, queues)
		assert.GreaterOrEqual(t, len(queues), 6)
	})

	t.Run("GetQueueByID returns correct queue", func(t *testing.T) {
		queue, err := repo.GetQueueByID(ctx, 1)
		require.NoError(t, err)
		assert.NotNil(t, queue)
		assert.Equal(t, 1, queue.ID)
		assert.NotEmpty(t, queue.Name)
	})

	t.Run("GetQueueByID returns error for non-existent", func(t *testing.T) {
		queue, err := repo.GetQueueByID(ctx, 999)
		assert.Error(t, err)
		assert.Nil(t, queue)
	})

	t.Run("CreateQueue adds new queue", func(t *testing.T) {
		newQueue := &models.QueueInfo{
			Name:        "Test Queue",
			Description: "Test Description",
			Active:      true,
		}

		err := repo.CreateQueue(ctx, newQueue)
		require.NoError(t, err)
		assert.NotZero(t, newQueue.ID)

		// Verify it was added
		queue, err := repo.GetQueueByID(ctx, newQueue.ID)
		require.NoError(t, err)
		assert.Equal(t, "Test Queue", queue.Name)
	})

	t.Run("UpdateQueue modifies existing queue", func(t *testing.T) {
		// Get an existing queue
		queue, err := repo.GetQueueByID(ctx, 1)
		require.NoError(t, err)

		// Modify it
		queue.Description = "Updated Description"
		err = repo.UpdateQueue(ctx, queue)
		require.NoError(t, err)

		// Verify the change
		updated, err := repo.GetQueueByID(ctx, 1)
		require.NoError(t, err)
		assert.Equal(t, "Updated Description", updated.Description)
	})

	t.Run("DeleteQueue removes queue", func(t *testing.T) {
		// Create a queue to delete
		newQueue := &models.QueueInfo{
			Name:   "To Delete",
			Active: true,
		}
		err := repo.CreateQueue(ctx, newQueue)
		require.NoError(t, err)

		// Delete it
		err = repo.DeleteQueue(ctx, newQueue.ID)
		require.NoError(t, err)

		// Verify it's gone
		_, err = repo.GetQueueByID(ctx, newQueue.ID)
		assert.Error(t, err)
	})
}

func TestMemoryLookupRepository_Priorities(t *testing.T) {
	ctx := context.Background()
	repo := NewMemoryLookupRepository()

	t.Run("GetPriorities returns all priorities", func(t *testing.T) {
		priorities, err := repo.GetPriorities(ctx)
		require.NoError(t, err)
		assert.Equal(t, 4, len(priorities))

		// Verify order
		expectedValues := []string{"low", "normal", "high", "urgent"}
		for i, p := range priorities {
			assert.Equal(t, expectedValues[i], p.Value)
		}
	})

	t.Run("GetPriorityByID returns correct priority", func(t *testing.T) {
		priority, err := repo.GetPriorityByID(ctx, 2)
		require.NoError(t, err)
		assert.NotNil(t, priority)
		assert.Equal(t, "normal", priority.Value)
	})

	t.Run("UpdatePriority modifies label only", func(t *testing.T) {
		priority, err := repo.GetPriorityByID(ctx, 1)
		require.NoError(t, err)

		originalValue := priority.Value
		priority.Label = "Very Low"

		err = repo.UpdatePriority(ctx, priority)
		require.NoError(t, err)

		updated, err := repo.GetPriorityByID(ctx, 1)
		require.NoError(t, err)
		assert.Equal(t, "Very Low", updated.Label)
		assert.Equal(t, originalValue, updated.Value) // Value should not change
	})
}

func TestMemoryLookupRepository_Types(t *testing.T) {
	ctx := context.Background()
	repo := NewMemoryLookupRepository()

	t.Run("GetTypes returns all types", func(t *testing.T) {
		types, err := repo.GetTypes(ctx)
		require.NoError(t, err)
		assert.Equal(t, 5, len(types))
	})

	t.Run("CreateType adds new type", func(t *testing.T) {
		newType := &models.LookupItem{
			Value:  "custom",
			Label:  "Custom Type",
			Order:  6,
			Active: true,
		}

		err := repo.CreateType(ctx, newType)
		require.NoError(t, err)
		assert.NotZero(t, newType.ID)

		types, err := repo.GetTypes(ctx)
		require.NoError(t, err)
		assert.Equal(t, 6, len(types))
	})

	t.Run("UpdateType modifies existing type", func(t *testing.T) {
		typ, err := repo.GetTypeByID(ctx, 1)
		require.NoError(t, err)

		typ.Label = "Critical Incident"
		err = repo.UpdateType(ctx, typ)
		require.NoError(t, err)

		updated, err := repo.GetTypeByID(ctx, 1)
		require.NoError(t, err)
		assert.Equal(t, "Critical Incident", updated.Label)
	})

	t.Run("DeleteType removes type", func(t *testing.T) {
		// Create a type to delete
		newType := &models.LookupItem{
			Value:  "temp",
			Label:  "Temporary",
			Order:  10,
			Active: true,
		}
		err := repo.CreateType(ctx, newType)
		require.NoError(t, err)

		// Delete it
		err = repo.DeleteType(ctx, newType.ID)
		require.NoError(t, err)

		// Verify it's gone
		_, err = repo.GetTypeByID(ctx, newType.ID)
		assert.Error(t, err)
	})
}

func TestMemoryLookupRepository_Statuses(t *testing.T) {
	ctx := context.Background()
	repo := NewMemoryLookupRepository()

	t.Run("GetStatuses returns workflow statuses", func(t *testing.T) {
		statuses, err := repo.GetStatuses(ctx)
		require.NoError(t, err)
		assert.Equal(t, 5, len(statuses))

		// Verify workflow order
		expectedValues := []string{"new", "open", "pending", "resolved", "closed"}
		for i, s := range statuses {
			assert.Equal(t, expectedValues[i], s.Value)
		}
	})

	t.Run("UpdateStatus only allows label changes", func(t *testing.T) {
		status, err := repo.GetStatusByID(ctx, 1)
		require.NoError(t, err)

		originalValue := status.Value
		status.Label = "Brand New"
		status.Value = "should_not_change" // Try to change value

		err = repo.UpdateStatus(ctx, status)
		require.NoError(t, err)

		updated, err := repo.GetStatusByID(ctx, 1)
		require.NoError(t, err)
		assert.Equal(t, "Brand New", updated.Label)
		assert.Equal(t, originalValue, updated.Value) // Value should not change
	})
}

func TestMemoryLookupRepository_AuditLog(t *testing.T) {
	ctx := context.Background()
	repo := NewMemoryLookupRepository()

	t.Run("LogChange creates audit entry", func(t *testing.T) {
		log := &LookupAuditLog{
			EntityType: "queue",
			EntityID:   1,
			Action:     "update",
			OldValue:   `{"name":"Old"}`,
			NewValue:   `{"name":"New"}`,
			UserID:     1,
			UserEmail:  "admin@test.com",
			Timestamp:  time.Now(),
			IPAddress:  "127.0.0.1",
		}

		err := repo.LogChange(ctx, log)
		require.NoError(t, err)
		assert.NotZero(t, log.ID)
	})

	t.Run("GetAuditLogs retrieves logs for entity", func(t *testing.T) {
		// Create several logs
		for i := 0; i < 5; i++ {
			log := &LookupAuditLog{
				EntityType: "queue",
				EntityID:   1,
				Action:     "update",
				OldValue:   "old",
				NewValue:   "new",
				UserID:     1,
				UserEmail:  "admin@test.com",
				Timestamp:  time.Now(),
			}
			repo.LogChange(ctx, log)
		}

		// Retrieve them
		logs, err := repo.GetAuditLogs(ctx, "queue", 1, 10)
		require.NoError(t, err)
		assert.GreaterOrEqual(t, len(logs), 5)

		// Should be in reverse chronological order
		for i := 1; i < len(logs); i++ {
			assert.True(t, logs[i-1].Timestamp.After(logs[i].Timestamp) ||
				logs[i-1].Timestamp.Equal(logs[i].Timestamp))
		}
	})

	t.Run("GetAuditLogs respects limit", func(t *testing.T) {
		logs, err := repo.GetAuditLogs(ctx, "queue", 1, 2)
		require.NoError(t, err)
		assert.LessOrEqual(t, len(logs), 2)
	})
}

func TestMemoryLookupRepository_ExportImport(t *testing.T) {
	ctx := context.Background()
	repo := NewMemoryLookupRepository()

	t.Run("ExportConfiguration captures all data", func(t *testing.T) {
		// Add some custom data
		newQueue := &models.QueueInfo{
			Name:        "Export Test",
			Description: "Test Export",
			Active:      true,
		}
		repo.CreateQueue(ctx, newQueue)

		// Export
		config, err := repo.ExportConfiguration(ctx)
		require.NoError(t, err)
		assert.NotNil(t, config)
		assert.NotEmpty(t, config.Version)
		assert.NotZero(t, config.ExportedAt)
		assert.NotEmpty(t, config.Queues)
		assert.NotEmpty(t, config.Priorities)
		assert.NotEmpty(t, config.Types)
		assert.NotEmpty(t, config.Statuses)

		// Verify custom queue is included
		found := false
		for _, q := range config.Queues {
			if q.Name == "Export Test" {
				found = true
				break
			}
		}
		assert.True(t, found, "Custom queue should be in export")
	})

	t.Run("ImportConfiguration replaces data", func(t *testing.T) {
		// Create a configuration to import
		config := &LookupConfiguration{
			Version:    "1.0",
			ExportedAt: time.Now(),
			ExportedBy: "test@example.com",
			Queues: []models.QueueInfo{
				{ID: 1, Name: "Imported Queue 1", Active: true},
				{ID: 2, Name: "Imported Queue 2", Active: true},
			},
			Priorities: []models.LookupItem{
				{ID: 1, Value: "low", Label: "Low Priority", Order: 1, Active: true},
				{ID: 2, Value: "high", Label: "High Priority", Order: 2, Active: true},
			},
			Types: []models.LookupItem{
				{ID: 1, Value: "bug", Label: "Bug", Order: 1, Active: true},
			},
			Statuses: []models.LookupItem{
				{ID: 1, Value: "new", Label: "New", Order: 1, Active: true},
				{ID: 2, Value: "closed", Label: "Closed", Order: 2, Active: true},
			},
		}

		// Import
		err := repo.ImportConfiguration(ctx, config)
		require.NoError(t, err)

		// Verify data was replaced
		queues, _ := repo.GetQueues(ctx)
		assert.Equal(t, 2, len(queues))
		assert.Equal(t, "Imported Queue 1", queues[0].Name)

		priorities, _ := repo.GetPriorities(ctx)
		assert.Equal(t, 2, len(priorities))

		types, _ := repo.GetTypes(ctx)
		assert.Equal(t, 1, len(types))

		statuses, _ := repo.GetStatuses(ctx)
		assert.Equal(t, 2, len(statuses))
	})

	t.Run("Export and Import round trip", func(t *testing.T) {
		repo1 := NewMemoryLookupRepository()
		repo2 := NewMemoryLookupRepository()

		// Export from repo1
		config, err := repo1.ExportConfiguration(ctx)
		require.NoError(t, err)

		// Import to repo2
		err = repo2.ImportConfiguration(ctx, config)
		require.NoError(t, err)

		// Compare data
		queues1, _ := repo1.GetQueues(ctx)
		queues2, _ := repo2.GetQueues(ctx)
		assert.Equal(t, len(queues1), len(queues2))

		priorities1, _ := repo1.GetPriorities(ctx)
		priorities2, _ := repo2.GetPriorities(ctx)
		assert.Equal(t, len(priorities1), len(priorities2))
	})
}

func TestMemoryLookupRepository_Concurrency(t *testing.T) {
	ctx := context.Background()
	repo := NewMemoryLookupRepository()

	// Test concurrent reads and writes
	done := make(chan bool, 100)

	for i := 0; i < 20; i++ {
		// Concurrent reads
		go func() {
			_, err := repo.GetQueues(ctx)
			assert.NoError(t, err)
			done <- true
		}()

		go func() {
			_, err := repo.GetPriorities(ctx)
			assert.NoError(t, err)
			done <- true
		}()

		// Concurrent writes
		go func(id int) {
			queue := &models.QueueInfo{
				Name:   "Concurrent Queue",
				Active: true,
			}
			err := repo.CreateQueue(ctx, queue)
			assert.NoError(t, err)
			done <- true
		}(i)

		// Concurrent audit logs
		go func(id int) {
			log := &LookupAuditLog{
				EntityType: "queue",
				EntityID:   id,
				Action:     "create",
				UserID:     1,
				Timestamp:  time.Now(),
			}
			err := repo.LogChange(ctx, log)
			assert.NoError(t, err)
			done <- true
		}(i)

		// Concurrent updates
		go func() {
			priority, err := repo.GetPriorityByID(ctx, 1)
			if err == nil {
				priority.Label = "Updated"
				repo.UpdatePriority(ctx, priority)
			}
			done <- true
		}()
	}

	// Wait for all operations
	for i := 0; i < 100; i++ {
		<-done
	}

	// Verify data integrity
	queues, err := repo.GetQueues(ctx)
	assert.NoError(t, err)
	assert.NotEmpty(t, queues)
}

func TestLookupConfiguration_JSON(t *testing.T) {
	config := &LookupConfiguration{
		Version:    "1.0",
		ExportedAt: time.Now(),
		ExportedBy: "test@example.com",
		Queues: []models.QueueInfo{
			{ID: 1, Name: "Test", Active: true},
		},
		Priorities: []models.LookupItem{
			{ID: 1, Value: "low", Label: "Low", Order: 1, Active: true},
		},
	}

	// Marshal to JSON
	data, err := json.Marshal(config)
	require.NoError(t, err)
	assert.NotEmpty(t, data)

	// Unmarshal back
	var loaded LookupConfiguration
	err = json.Unmarshal(data, &loaded)
	require.NoError(t, err)
	assert.Equal(t, config.Version, loaded.Version)
	assert.Equal(t, len(config.Queues), len(loaded.Queues))
}
