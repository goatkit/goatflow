//go:build integration

// Package ticketattributerelations provides management of ticket attribute relationships.
package ticketattributerelations

import (
	"context"
	"database/sql"
	"fmt"
	"testing"
	"time"

	"github.com/gotrs-io/gotrs-ce/internal/database"
	"github.com/gotrs-io/gotrs-ce/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Test data structure for cleanup
type testRelationData struct {
	relationIDs []int64
}

// setupTestRelationData creates test relations for integration tests
func setupTestRelationData(t *testing.T, db *sql.DB) *testRelationData {
	t.Helper()
	data := &testRelationData{}

	// Cleanup any leftover test data
	_, _ = db.Exec(`DELETE FROM acl_ticket_attribute_relations WHERE filename LIKE 'inttest_%'`)

	return data
}

// cleanupTestRelationData removes all test relations
func cleanupTestRelationData(t *testing.T, db *sql.DB, data *testRelationData) {
	t.Helper()
	for _, id := range data.relationIDs {
		_, _ = db.Exec(`DELETE FROM acl_ticket_attribute_relations WHERE id = $1`, id)
	}
	// Also cleanup by filename pattern
	_, _ = db.Exec(`DELETE FROM acl_ticket_attribute_relations WHERE filename LIKE 'inttest_%'`)
}

func TestTicketAttributeRelationsIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test")
	}

	t.Setenv("APP_ENV", "integration")

	db, err := database.GetDB()
	require.NoError(t, err)
	require.NotNil(t, db)

	testData := setupTestRelationData(t, db)
	defer cleanupTestRelationData(t, db, testData)

	svc := NewService(db)
	ctx := context.Background()

	t.Run("CreateRelation", func(t *testing.T) {
		// Create a Queue -> DynamicField_Category relation
		relation := &models.TicketAttributeRelation{
			Filename:   fmt.Sprintf("inttest_queue_category_%d.csv", time.Now().UnixNano()),
			Attribute1: "Queue",
			Attribute2: "DynamicField_Category",
			ACLData:    "Queue;DynamicField_Category\nSales;Quote\nSales;Opportunity\nSupport;Bug\nSupport;Feature\nSupport;Question",
			Priority:   1,
			Data: []models.AttributeRelationPair{
				{Attribute1Value: "Sales", Attribute2Value: "Quote"},
				{Attribute1Value: "Sales", Attribute2Value: "Opportunity"},
				{Attribute1Value: "Support", Attribute2Value: "Bug"},
				{Attribute1Value: "Support", Attribute2Value: "Feature"},
				{Attribute1Value: "Support", Attribute2Value: "Question"},
			},
		}

		id, err := svc.Create(ctx, relation, 1) // User ID 1
		require.NoError(t, err)
		assert.Greater(t, id, int64(0), "Should return valid ID")

		testData.relationIDs = append(testData.relationIDs, id)
	})

	t.Run("GetRelation", func(t *testing.T) {
		if len(testData.relationIDs) == 0 {
			t.Skip("No relation created in previous test")
		}

		relation, err := svc.GetByID(ctx, testData.relationIDs[0])
		require.NoError(t, err)
		require.NotNil(t, relation)

		assert.Equal(t, "Queue", relation.Attribute1)
		assert.Equal(t, "DynamicField_Category", relation.Attribute2)
		assert.Len(t, relation.Data, 5)
	})

	t.Run("EvaluateRelations_QueueSales", func(t *testing.T) {
		// When Queue = "Sales", should return DynamicField_Category = ["Quote", "Opportunity"]
		result, err := svc.EvaluateRelations(ctx, "Queue", "Sales")
		require.NoError(t, err)

		assert.Contains(t, result, "DynamicField_Category")
		allowedValues := result["DynamicField_Category"]
		assert.Len(t, allowedValues, 2)
		assert.Contains(t, allowedValues, "Quote")
		assert.Contains(t, allowedValues, "Opportunity")
	})

	t.Run("EvaluateRelations_QueueSupport", func(t *testing.T) {
		// When Queue = "Support", should return DynamicField_Category = ["Bug", "Feature", "Question"]
		result, err := svc.EvaluateRelations(ctx, "Queue", "Support")
		require.NoError(t, err)

		assert.Contains(t, result, "DynamicField_Category")
		allowedValues := result["DynamicField_Category"]
		assert.Len(t, allowedValues, 3)
		assert.Contains(t, allowedValues, "Bug")
		assert.Contains(t, allowedValues, "Feature")
		assert.Contains(t, allowedValues, "Question")
	})

	t.Run("EvaluateRelations_UnknownQueue", func(t *testing.T) {
		// When Queue = "Unknown", should return empty (no matching relation)
		result, err := svc.EvaluateRelations(ctx, "Queue", "Unknown")
		require.NoError(t, err)

		// Should have no results for DynamicField_Category
		if values, ok := result["DynamicField_Category"]; ok {
			assert.Empty(t, values)
		}
	})

	t.Run("EvaluateRelations_DifferentAttribute", func(t *testing.T) {
		// When evaluating State (not in our test data), should return empty
		result, err := svc.EvaluateRelations(ctx, "State", "open")
		require.NoError(t, err)

		// Should have no results
		assert.Empty(t, result)
	})

	t.Run("CreateSecondRelation_StatePriority", func(t *testing.T) {
		// Create a State -> Priority relation
		relation := &models.TicketAttributeRelation{
			Filename:   fmt.Sprintf("inttest_state_priority_%d.csv", time.Now().UnixNano()),
			Attribute1: "State",
			Attribute2: "Priority",
			ACLData:    "State;Priority\nnew;3 normal\nnew;4 high\nnew;5 very high\nopen;4 high\nopen;5 very high\nclosed;1 very low\nclosed;2 low",
			Priority:   2,
			Data: []models.AttributeRelationPair{
				{Attribute1Value: "new", Attribute2Value: "3 normal"},
				{Attribute1Value: "new", Attribute2Value: "4 high"},
				{Attribute1Value: "new", Attribute2Value: "5 very high"},
				{Attribute1Value: "open", Attribute2Value: "4 high"},
				{Attribute1Value: "open", Attribute2Value: "5 very high"},
				{Attribute1Value: "closed", Attribute2Value: "1 very low"},
				{Attribute1Value: "closed", Attribute2Value: "2 low"},
			},
		}

		id, err := svc.Create(ctx, relation, 1)
		require.NoError(t, err)
		assert.Greater(t, id, int64(0))

		testData.relationIDs = append(testData.relationIDs, id)
	})

	t.Run("EvaluateRelations_StateNew", func(t *testing.T) {
		// When State = "new", should return Priority = ["3 normal", "4 high", "5 very high"]
		result, err := svc.EvaluateRelations(ctx, "State", "new")
		require.NoError(t, err)

		assert.Contains(t, result, "Priority")
		allowedValues := result["Priority"]
		assert.Len(t, allowedValues, 3)
		assert.Contains(t, allowedValues, "3 normal")
		assert.Contains(t, allowedValues, "4 high")
		assert.Contains(t, allowedValues, "5 very high")
	})

	t.Run("EvaluateRelations_StateOpen", func(t *testing.T) {
		// When State = "open", should return Priority = ["4 high", "5 very high"]
		result, err := svc.EvaluateRelations(ctx, "State", "open")
		require.NoError(t, err)

		assert.Contains(t, result, "Priority")
		allowedValues := result["Priority"]
		assert.Len(t, allowedValues, 2)
		assert.Contains(t, allowedValues, "4 high")
		assert.Contains(t, allowedValues, "5 very high")
	})

	t.Run("EvaluateRelations_StateClosed", func(t *testing.T) {
		// When State = "closed", should return Priority = ["1 very low", "2 low"]
		result, err := svc.EvaluateRelations(ctx, "State", "closed")
		require.NoError(t, err)

		assert.Contains(t, result, "Priority")
		allowedValues := result["Priority"]
		assert.Len(t, allowedValues, 2)
		assert.Contains(t, allowedValues, "1 very low")
		assert.Contains(t, allowedValues, "2 low")
	})

	t.Run("GetAllRelations", func(t *testing.T) {
		relations, err := svc.GetAll(ctx)
		require.NoError(t, err)

		// Should have at least our 2 test relations
		testCount := 0
		for _, r := range relations {
			if r.Filename[:8] == "inttest_" {
				testCount++
			}
		}
		assert.GreaterOrEqual(t, testCount, 2)
	})

	t.Run("UpdateRelation", func(t *testing.T) {
		if len(testData.relationIDs) == 0 {
			t.Skip("No relation created in previous test")
		}

		// Update the first relation with a new priority
		updates := map[string]interface{}{
			"priority": int64(2),
		}
		err := svc.Update(ctx, testData.relationIDs[0], updates, 1)
		require.NoError(t, err)

		// Verify the update
		relation, err := svc.GetByID(ctx, testData.relationIDs[0])
		require.NoError(t, err)
		assert.NotNil(t, relation)
	})

	t.Run("DeleteRelation", func(t *testing.T) {
		if len(testData.relationIDs) == 0 {
			t.Skip("No relation created in previous test")
		}

		// Delete the first relation
		err := svc.Delete(ctx, testData.relationIDs[0])
		require.NoError(t, err)

		// Verify deletion - should get nil result
		result, err := svc.GetByID(ctx, testData.relationIDs[0])
		require.NoError(t, err)
		assert.Nil(t, result)

		// Remove from our tracking list
		testData.relationIDs = testData.relationIDs[1:]
	})
}

func TestMultipleRelationsSameAttribute(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test")
	}

	t.Setenv("APP_ENV", "integration")

	db, err := database.GetDB()
	require.NoError(t, err)
	require.NotNil(t, db)

	testData := setupTestRelationData(t, db)
	defer cleanupTestRelationData(t, db, testData)

	svc := NewService(db)
	ctx := context.Background()

	// Create two relations that both affect DynamicField_Category based on Queue
	// This tests the intersection behavior when multiple relations apply

	// Relation 1: Queue -> DynamicField_Category (primary mapping)
	relation1 := &models.TicketAttributeRelation{
		Filename:   fmt.Sprintf("inttest_multi_1_%d.csv", time.Now().UnixNano()),
		Attribute1: "Queue",
		Attribute2: "DynamicField_Category",
		ACLData:    "Queue;DynamicField_Category\nSales;Quote\nSales;Contract\nSales;Opportunity",
		Priority:   1,
		Data: []models.AttributeRelationPair{
			{Attribute1Value: "Sales", Attribute2Value: "Quote"},
			{Attribute1Value: "Sales", Attribute2Value: "Contract"},
			{Attribute1Value: "Sales", Attribute2Value: "Opportunity"},
		},
	}

	id1, err := svc.Create(ctx, relation1, 1)
	require.NoError(t, err)
	testData.relationIDs = append(testData.relationIDs, id1)

	// Relation 2: Queue -> DynamicField_Category (restrictive mapping - same attribute)
	relation2 := &models.TicketAttributeRelation{
		Filename:   fmt.Sprintf("inttest_multi_2_%d.csv", time.Now().UnixNano()),
		Attribute1: "Queue",
		Attribute2: "DynamicField_Category",
		ACLData:    "Queue;DynamicField_Category\nSales;Quote\nSales;Contract",
		Priority:   2,
		Data: []models.AttributeRelationPair{
			{Attribute1Value: "Sales", Attribute2Value: "Quote"},
			{Attribute1Value: "Sales", Attribute2Value: "Contract"},
			// Note: Opportunity is NOT in this relation
		},
	}

	id2, err := svc.Create(ctx, relation2, 1)
	require.NoError(t, err)
	testData.relationIDs = append(testData.relationIDs, id2)

	t.Run("EvaluateMultipleRelations_Intersection", func(t *testing.T) {
		// When Queue = "Sales" with two relations, should return intersection
		// Relation 1 allows: Quote, Contract, Opportunity
		// Relation 2 allows: Quote, Contract
		// Intersection: Quote, Contract (Opportunity is excluded)
		result, err := svc.EvaluateRelations(ctx, "Queue", "Sales")
		require.NoError(t, err)

		assert.Contains(t, result, "DynamicField_Category")
		allowedValues := result["DynamicField_Category"]

		// Should only have values present in BOTH relations
		assert.Contains(t, allowedValues, "Quote")
		assert.Contains(t, allowedValues, "Contract")
		assert.NotContains(t, allowedValues, "Opportunity") // Not in relation 2, so excluded
		assert.Len(t, allowedValues, 2)
	})
}

func TestFilenameUniqueness(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test")
	}

	t.Setenv("APP_ENV", "integration")

	db, err := database.GetDB()
	require.NoError(t, err)
	require.NotNil(t, db)

	testData := setupTestRelationData(t, db)
	defer cleanupTestRelationData(t, db, testData)

	svc := NewService(db)
	ctx := context.Background()

	// Create first relation
	uniqueFilename := fmt.Sprintf("inttest_unique_%d.csv", time.Now().UnixNano())
	relation1 := &models.TicketAttributeRelation{
		Filename:   uniqueFilename,
		Attribute1: "Queue",
		Attribute2: "State",
		ACLData:    "Queue;State\nSales;open",
		Priority:   1,
		Data: []models.AttributeRelationPair{
			{Attribute1Value: "Sales", Attribute2Value: "open"},
		},
	}

	id1, err := svc.Create(ctx, relation1, 1)
	require.NoError(t, err)
	testData.relationIDs = append(testData.relationIDs, id1)

	// Try to create second relation with same filename - should fail
	relation2 := &models.TicketAttributeRelation{
		Filename:   uniqueFilename, // Same filename
		Attribute1: "Queue",
		Attribute2: "Priority",
		ACLData:    "Queue;Priority\nSales;high",
		Priority:   2,
		Data: []models.AttributeRelationPair{
			{Attribute1Value: "Sales", Attribute2Value: "high"},
		},
	}

	_, err = svc.Create(ctx, relation2, 1)
	assert.Error(t, err, "Should fail when creating relation with duplicate filename")
}

func TestPriorityOrdering(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test")
	}

	t.Setenv("APP_ENV", "integration")

	db, err := database.GetDB()
	require.NoError(t, err)
	require.NotNil(t, db)

	testData := setupTestRelationData(t, db)
	defer cleanupTestRelationData(t, db, testData)

	svc := NewService(db)
	ctx := context.Background()

	// Create relations with different priorities
	priorities := []int64{5, 1, 3}
	for i, priority := range priorities {
		relation := &models.TicketAttributeRelation{
			Filename:   fmt.Sprintf("inttest_priority_%d_%d.csv", priority, time.Now().UnixNano()),
			Attribute1: "Queue",
			Attribute2: "State",
			ACLData:    fmt.Sprintf("Queue;State\nSales;state%d", i),
			Priority:   priority,
			Data: []models.AttributeRelationPair{
				{Attribute1Value: "Sales", Attribute2Value: fmt.Sprintf("state%d", i)},
			},
		}

		id, err := svc.Create(ctx, relation, 1)
		require.NoError(t, err)
		testData.relationIDs = append(testData.relationIDs, id)
	}

	// Get all relations and verify they are ordered by priority
	relations, err := svc.GetAll(ctx)
	require.NoError(t, err)

	// Filter to just our test relations
	var testRelations []*models.TicketAttributeRelation
	for _, r := range relations {
		if len(r.Filename) > 16 && r.Filename[:16] == "inttest_priority" {
			testRelations = append(testRelations, r)
		}
	}

	require.Len(t, testRelations, 3)

	// Verify ascending order by priority
	for i := 0; i < len(testRelations)-1; i++ {
		assert.LessOrEqual(t, testRelations[i].Priority, testRelations[i+1].Priority,
			"Relations should be ordered by priority (ascending)")
	}
}
