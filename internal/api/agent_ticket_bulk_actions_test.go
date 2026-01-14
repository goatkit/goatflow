package api

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Unit tests for Bulk Ticket Actions - Data Structures and JSON Binding

func TestBulkActionResult_Structure(t *testing.T) {
	result := BulkActionResult{
		Success:   true,
		Total:     5,
		Succeeded: 4,
		Failed:    1,
		Errors:    []string{"Ticket 3: not found"},
	}

	assert.True(t, result.Success)
	assert.Equal(t, 5, result.Total)
	assert.Equal(t, 4, result.Succeeded)
	assert.Equal(t, 1, result.Failed)
	assert.Len(t, result.Errors, 1)
	assert.Equal(t, "Ticket 3: not found", result.Errors[0])
}

func TestBulkActionResult_PartialSuccess(t *testing.T) {
	result := BulkActionResult{
		Success:   false,
		Total:     10,
		Succeeded: 7,
		Failed:    3,
		Errors: []string{
			"Ticket 1: not found",
			"Ticket 5: update failed",
			"Ticket 9: permission denied",
		},
	}

	assert.False(t, result.Success)
	assert.Equal(t, 10, result.Total)
	assert.Equal(t, 7, result.Succeeded)
	assert.Equal(t, 3, result.Failed)
	assert.Len(t, result.Errors, 3)
}

func TestBulkActionRequest_JSONBinding(t *testing.T) {
	jsonData := `{"ticket_ids": [1, 2, 3]}`

	var req BulkActionRequest
	err := json.Unmarshal([]byte(jsonData), &req)

	require.NoError(t, err)
	assert.Equal(t, []int{1, 2, 3}, req.TicketIDs)
	assert.Len(t, req.TicketIDs, 3)
}

func TestBulkActionRequest_EmptyTickets(t *testing.T) {
	jsonData := `{"ticket_ids": []}`

	var req BulkActionRequest
	err := json.Unmarshal([]byte(jsonData), &req)

	require.NoError(t, err)
	assert.Empty(t, req.TicketIDs)
}

func TestBulkStatusRequest_JSONBinding(t *testing.T) {
	jsonData := `{"ticket_ids": [1, 2], "status_id": 5, "pending_until": 1704067200}`

	var req BulkStatusRequest
	err := json.Unmarshal([]byte(jsonData), &req)

	require.NoError(t, err)
	assert.Equal(t, []int{1, 2}, req.TicketIDs)
	assert.Equal(t, 5, req.StatusID)
	assert.Equal(t, int64(1704067200), req.PendingUntil)
}

func TestBulkStatusRequest_NoPendingUntil(t *testing.T) {
	jsonData := `{"ticket_ids": [1, 2], "status_id": 3}`

	var req BulkStatusRequest
	err := json.Unmarshal([]byte(jsonData), &req)

	require.NoError(t, err)
	assert.Equal(t, []int{1, 2}, req.TicketIDs)
	assert.Equal(t, 3, req.StatusID)
	assert.Equal(t, int64(0), req.PendingUntil)
}

func TestBulkPriorityRequest_JSONBinding(t *testing.T) {
	jsonData := `{"ticket_ids": [10, 20, 30], "priority_id": 2}`

	var req BulkPriorityRequest
	err := json.Unmarshal([]byte(jsonData), &req)

	require.NoError(t, err)
	assert.Equal(t, []int{10, 20, 30}, req.TicketIDs)
	assert.Equal(t, 2, req.PriorityID)
}

func TestBulkQueueRequest_JSONBinding(t *testing.T) {
	jsonData := `{"ticket_ids": [5, 6, 7, 8], "queue_id": 4}`

	var req BulkQueueRequest
	err := json.Unmarshal([]byte(jsonData), &req)

	require.NoError(t, err)
	assert.Equal(t, []int{5, 6, 7, 8}, req.TicketIDs)
	assert.Equal(t, 4, req.QueueID)
}

func TestBulkAssignRequest_JSONBinding(t *testing.T) {
	jsonData := `{"ticket_ids": [100, 200], "user_id": 15}`

	var req BulkAssignRequest
	err := json.Unmarshal([]byte(jsonData), &req)

	require.NoError(t, err)
	assert.Equal(t, []int{100, 200}, req.TicketIDs)
	assert.Equal(t, 15, req.UserID)
}

func TestBulkLockRequest_JSONBinding_Lock(t *testing.T) {
	jsonData := `{"ticket_ids": [1, 2, 3], "lock": true}`

	var req BulkLockRequest
	err := json.Unmarshal([]byte(jsonData), &req)

	require.NoError(t, err)
	assert.Equal(t, []int{1, 2, 3}, req.TicketIDs)
	assert.True(t, req.Lock)
}

func TestBulkLockRequest_JSONBinding_Unlock(t *testing.T) {
	jsonData := `{"ticket_ids": [4, 5], "lock": false}`

	var req BulkLockRequest
	err := json.Unmarshal([]byte(jsonData), &req)

	require.NoError(t, err)
	assert.Equal(t, []int{4, 5}, req.TicketIDs)
	assert.False(t, req.Lock)
}

func TestBulkMergeRequest_JSONBinding(t *testing.T) {
	jsonData := `{"ticket_ids": [1, 2, 3], "target_ticket_id": 1}`

	var req BulkMergeRequest
	err := json.Unmarshal([]byte(jsonData), &req)

	require.NoError(t, err)
	assert.Equal(t, []int{1, 2, 3}, req.TicketIDs)
	assert.Equal(t, 1, req.TargetTicketID)
}

func TestBulkMergeRequest_MultipleTickets(t *testing.T) {
	jsonData := `{"ticket_ids": [10, 20, 30, 40, 50], "target_ticket_id": 10}`

	var req BulkMergeRequest
	err := json.Unmarshal([]byte(jsonData), &req)

	require.NoError(t, err)
	assert.Len(t, req.TicketIDs, 5)
	assert.Equal(t, 10, req.TargetTicketID)
}

func TestBulkActionResult_JSONSerialization(t *testing.T) {
	result := BulkActionResult{
		Success:   true,
		Total:     3,
		Succeeded: 3,
		Failed:    0,
		Errors:    nil,
	}

	data, err := json.Marshal(result)
	require.NoError(t, err)

	var parsed BulkActionResult
	err = json.Unmarshal(data, &parsed)
	require.NoError(t, err)

	assert.Equal(t, result.Success, parsed.Success)
	assert.Equal(t, result.Total, parsed.Total)
	assert.Equal(t, result.Succeeded, parsed.Succeeded)
	assert.Equal(t, result.Failed, parsed.Failed)
}

func TestBulkActionResult_JSONWithErrors(t *testing.T) {
	result := BulkActionResult{
		Success:   false,
		Total:     5,
		Succeeded: 2,
		Failed:    3,
		Errors:    []string{"Error 1", "Error 2", "Error 3"},
	}

	data, err := json.Marshal(result)
	require.NoError(t, err)

	// Verify JSON contains expected fields
	jsonStr := string(data)
	assert.Contains(t, jsonStr, `"success":false`)
	assert.Contains(t, jsonStr, `"total":5`)
	assert.Contains(t, jsonStr, `"succeeded":2`)
	assert.Contains(t, jsonStr, `"failed":3`)
	assert.Contains(t, jsonStr, `"errors"`)
}

func TestBulkStatusRequest_FormBinding(t *testing.T) {
	// Test that form tags work for URL-encoded form data
	req := BulkStatusRequest{}
	req.TicketIDs = []int{1, 2, 3}
	req.StatusID = 5
	req.PendingUntil = 1704067200

	assert.Equal(t, []int{1, 2, 3}, req.TicketIDs)
	assert.Equal(t, 5, req.StatusID)
	assert.Equal(t, int64(1704067200), req.PendingUntil)
}

func TestBulkPriorityRequest_FormBinding(t *testing.T) {
	req := BulkPriorityRequest{}
	req.TicketIDs = []int{10, 20}
	req.PriorityID = 3

	assert.Equal(t, []int{10, 20}, req.TicketIDs)
	assert.Equal(t, 3, req.PriorityID)
}

func TestBulkQueueRequest_FormBinding(t *testing.T) {
	req := BulkQueueRequest{}
	req.TicketIDs = []int{100}
	req.QueueID = 7

	assert.Equal(t, []int{100}, req.TicketIDs)
	assert.Equal(t, 7, req.QueueID)
}

func TestBulkAssignRequest_FormBinding(t *testing.T) {
	req := BulkAssignRequest{}
	req.TicketIDs = []int{50, 60, 70}
	req.UserID = 25

	assert.Equal(t, []int{50, 60, 70}, req.TicketIDs)
	assert.Equal(t, 25, req.UserID)
}

func TestBulkLockRequest_FormBinding(t *testing.T) {
	req := BulkLockRequest{}
	req.TicketIDs = []int{1, 2}
	req.Lock = true

	assert.Equal(t, []int{1, 2}, req.TicketIDs)
	assert.True(t, req.Lock)
}

func TestBulkMergeRequest_FormBinding(t *testing.T) {
	req := BulkMergeRequest{}
	req.TicketIDs = []int{1, 2, 3}
	req.TargetTicketID = 1

	assert.Equal(t, []int{1, 2, 3}, req.TicketIDs)
	assert.Equal(t, 1, req.TargetTicketID)
}
