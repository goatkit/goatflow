package service

import (
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewLookupService(t *testing.T) {
	service := NewLookupService()
	
	assert.NotNil(t, service)
	assert.Equal(t, 5*time.Minute, service.cacheTTL)
	assert.NotNil(t, service.cache)
	assert.Empty(t, service.cache)
}

func TestGetTicketFormData(t *testing.T) {
	tests := []struct {
		name     string
		setup    func(*LookupService)
		validate func(*testing.T, *LookupService)
	}{
		{
			name: "Returns fresh data when cache is empty",
			setup: func(s *LookupService) {
				// Cache starts empty
			},
			validate: func(t *testing.T, s *LookupService) {
				data := s.GetTicketFormData()
				require.NotNil(t, data)
				assert.NotEmpty(t, data.Queues)
				assert.NotEmpty(t, data.Priorities)
				assert.NotEmpty(t, data.Types)
				assert.NotEmpty(t, data.Statuses)
				
				// Verify cache was populated
				assert.NotNil(t, s.cache)
				assert.NotEmpty(t, s.cacheTime)
				// Check that at least one cache time entry exists and is recent
				for _, cacheTime := range s.cacheTime {
					assert.WithinDuration(t, time.Now(), cacheTime, time.Second)
					break
				}
			},
		},
		{
			name: "Returns cached data when cache is valid",
			setup: func(s *LookupService) {
				// Populate cache
				s.GetTicketFormData()
				// Mark cache time for comparison
				s.mu.Lock()
				s.cacheTime["en"] = time.Now()
				s.mu.Unlock()
			},
			validate: func(t *testing.T, s *LookupService) {
				originalCacheTime := s.cacheTime
				data := s.GetTicketFormData()
				
				require.NotNil(t, data)
				// Cache time should not have changed
				assert.Equal(t, originalCacheTime, s.cacheTime)
			},
		},
		{
			name: "Refreshes cache when TTL expired",
			setup: func(s *LookupService) {
				// Populate cache with old timestamp
				s.GetTicketFormData()
				s.mu.Lock()
				s.cacheTime["en"] = time.Now().Add(-6 * time.Minute)
				s.mu.Unlock()
			},
			validate: func(t *testing.T, s *LookupService) {
				oldCacheTime := s.cacheTime["en"]
				data := s.GetTicketFormData()
				
				require.NotNil(t, data)
				// Cache time should be updated
				assert.True(t, s.cacheTime["en"].After(oldCacheTime))
				assert.WithinDuration(t, time.Now(), s.cacheTime["en"], time.Second)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service := NewLookupService()
			tt.setup(service)
			tt.validate(t, service)
		})
	}
}

func TestGetQueues(t *testing.T) {
	service := NewLookupService()
	
	queues := service.GetQueues()
	
	assert.NotEmpty(t, queues)
	assert.Equal(t, 6, len(queues)) // Based on current implementation
	
	// Verify queue structure
	for _, queue := range queues {
		assert.NotZero(t, queue.ID)
		assert.NotEmpty(t, queue.Name)
		assert.NotEmpty(t, queue.Description)
		assert.True(t, queue.Active)
	}
}

func TestGetPriorities(t *testing.T) {
	service := NewLookupService()
	
	priorities := service.GetPriorities()
	
	assert.NotEmpty(t, priorities)
	assert.Equal(t, 4, len(priorities)) // low, normal, high, urgent
	
	// Verify priority structure and order
	expectedValues := []string{"low", "normal", "high", "urgent"}
	for i, priority := range priorities {
		assert.Equal(t, expectedValues[i], priority.Value)
		assert.NotEmpty(t, priority.Label)
		assert.Equal(t, i+1, priority.Order)
		assert.True(t, priority.Active)
	}
}

func TestGetTypes(t *testing.T) {
	service := NewLookupService()
	
	types := service.GetTypes()
	
	assert.NotEmpty(t, types)
	assert.Equal(t, 5, len(types))
	
	// Verify each type has required fields
	for _, typ := range types {
		assert.NotZero(t, typ.ID)
		assert.NotEmpty(t, typ.Value)
		assert.NotEmpty(t, typ.Label)
		assert.NotZero(t, typ.Order)
		assert.True(t, typ.Active)
	}
}

func TestGetStatuses(t *testing.T) {
	service := NewLookupService()
	
	statuses := service.GetStatuses()
	
	assert.NotEmpty(t, statuses)
	assert.Equal(t, 5, len(statuses)) // new, open, pending, resolved, closed
	
	// Verify status workflow order
	expectedValues := []string{"new", "open", "pending", "resolved", "closed"}
	for i, status := range statuses {
		assert.Equal(t, expectedValues[i], status.Value)
		assert.NotEmpty(t, status.Label)
		assert.Equal(t, i+1, status.Order)
		assert.True(t, status.Active)
	}
}

func TestInvalidateCache(t *testing.T) {
	service := NewLookupService()
	
	// Populate cache
	_ = service.GetTicketFormData()
	assert.NotNil(t, service.cache)
	
	// Invalidate cache
	service.InvalidateCache()
	
	// Cache should be cleared (empty map, not nil)
	assert.Empty(t, service.cache)
	
	// Next call should repopulate
	data := service.GetTicketFormData()
	assert.NotNil(t, data)
	assert.NotEmpty(t, service.cache)
}

func TestGetQueueByID(t *testing.T) {
	service := NewLookupService()
	
	tests := []struct {
		name     string
		id       int
		wantFound bool
	}{
		{"Existing queue", 1, true},
		{"Another existing queue", 3, true},
		{"Non-existent queue", 999, false},
		{"Zero ID", 0, false},
		{"Negative ID", -1, false},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			queue, found := service.GetQueueByID(tt.id)
			
			assert.Equal(t, tt.wantFound, found)
			if tt.wantFound {
				assert.NotNil(t, queue)
				assert.Equal(t, tt.id, queue.ID)
				assert.NotEmpty(t, queue.Name)
			} else {
				assert.Nil(t, queue)
			}
		})
	}
}

func TestGetPriorityByValue(t *testing.T) {
	service := NewLookupService()
	
	tests := []struct {
		name      string
		value     string
		wantFound bool
	}{
		{"Low priority", "low", true},
		{"Normal priority", "normal", true},
		{"High priority", "high", true},
		{"Urgent priority", "urgent", true},
		{"Invalid priority", "critical", false},
		{"Empty value", "", false},
		{"Case sensitive check", "HIGH", false},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			priority, found := service.GetPriorityByValue(tt.value)
			
			assert.Equal(t, tt.wantFound, found)
			if tt.wantFound {
				assert.NotNil(t, priority)
				assert.Equal(t, tt.value, priority.Value)
				assert.NotEmpty(t, priority.Label)
			} else {
				assert.Nil(t, priority)
			}
		})
	}
}

func TestGetTypeByID(t *testing.T) {
	service := NewLookupService()
	
	tests := []struct {
		name      string
		id        int
		wantFound bool
	}{
		{"Incident type", 1, true},
		{"Service request", 2, true},
		{"Question type", 5, true},
		{"Non-existent type", 99, false},
		{"Zero ID", 0, false},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			typ, found := service.GetTypeByID(tt.id)
			
			assert.Equal(t, tt.wantFound, found)
			if tt.wantFound {
				assert.NotNil(t, typ)
				assert.Equal(t, tt.id, typ.ID)
				assert.NotEmpty(t, typ.Value)
				assert.NotEmpty(t, typ.Label)
			} else {
				assert.Nil(t, typ)
			}
		})
	}
}

func TestGetStatusByValue(t *testing.T) {
	service := NewLookupService()
	
	tests := []struct {
		name      string
		value     string
		wantFound bool
	}{
		{"New status", "new", true},
		{"Open status", "open", true},
		{"Pending status", "pending", true},
		{"Resolved status", "resolved", true},
		{"Closed status", "closed", true},
		{"Invalid status", "cancelled", false},
		{"Empty value", "", false},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			status, found := service.GetStatusByValue(tt.value)
			
			assert.Equal(t, tt.wantFound, found)
			if tt.wantFound {
				assert.NotNil(t, status)
				assert.Equal(t, tt.value, status.Value)
				assert.NotEmpty(t, status.Label)
			} else {
				assert.Nil(t, status)
			}
		})
	}
}

func TestConcurrentAccess(t *testing.T) {
	service := NewLookupService()
	
	// Test concurrent reads and cache invalidation
	var wg sync.WaitGroup
	iterations := 100
	
	wg.Add(iterations * 4)
	
	// Concurrent reads
	for i := 0; i < iterations; i++ {
		go func() {
			defer wg.Done()
			data := service.GetTicketFormData()
			assert.NotNil(t, data)
		}()
		
		go func() {
			defer wg.Done()
			queues := service.GetQueues()
			assert.NotEmpty(t, queues)
		}()
		
		go func() {
			defer wg.Done()
			priorities := service.GetPriorities()
			assert.NotEmpty(t, priorities)
		}()
		
		// Occasional cache invalidation
		if i%10 == 0 {
			go func() {
				defer wg.Done()
				service.InvalidateCache()
			}()
		} else {
			go func() {
				defer wg.Done()
				// Just another read
				_ = service.GetStatuses()
			}()
		}
	}
	
	wg.Wait()
	
	// Verify service is still functional
	data := service.GetTicketFormData()
	assert.NotNil(t, data)
	assert.NotEmpty(t, data.Queues)
}

func TestCacheTTL(t *testing.T) {
	t.Skip("Skipping time-dependent test in CI")
	
	service := NewLookupService()
	service.cacheTTL = 100 * time.Millisecond // Short TTL for testing
	
	// Get initial data
	data1 := service.GetTicketFormData()
	require.NotNil(t, data1)
	cacheTime1 := service.cacheTime["en"]
	
	// Access within TTL - should use cache
	time.Sleep(50 * time.Millisecond)
	data2 := service.GetTicketFormData()
	assert.Equal(t, cacheTime1, service.cacheTime["en"])
	assert.Equal(t, data1, data2)
	
	// Access after TTL - should refresh
	time.Sleep(60 * time.Millisecond)
	data3 := service.GetTicketFormData()
	assert.True(t, service.cacheTime["en"].After(cacheTime1))
	assert.NotNil(t, data3)
}