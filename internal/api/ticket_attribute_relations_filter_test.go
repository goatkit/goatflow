//go:build integration

package api

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/goatkit/goatflow/internal/models"
	"github.com/goatkit/goatflow/internal/platform/database"
	"github.com/goatkit/goatflow/internal/services/ticketattributerelations"
)

// testFilterData holds test data for cleanup
type testFilterData struct {
	relationIDs []int64
	serviceIDs  []int64
	typeIDs     []int64
}

// setupTestFilterData creates test data for filtering tests
func setupTestFilterData(t *testing.T, db *sql.DB) *testFilterData {
	t.Helper()
	data := &testFilterData{}

	// Cleanup any leftover test data
	_, _ = db.Exec(database.ConvertPlaceholders(`DELETE FROM acl_ticket_attribute_relations WHERE filename LIKE 'filtertest_%'`))

	return data
}

// cleanupTestFilterData removes all test data
func cleanupTestFilterData(t *testing.T, db *sql.DB, data *testFilterData) {
	t.Helper()

	// Delete test relations
	for _, id := range data.relationIDs {
		_, _ = db.Exec(database.ConvertPlaceholders(`DELETE FROM acl_ticket_attribute_relations WHERE id = ?`), id)
	}
	_, _ = db.Exec(database.ConvertPlaceholders(`DELETE FROM acl_ticket_attribute_relations WHERE filename LIKE 'filtertest_%'`))

	// Fixture rows are not referenced by anything else; remove them outright.
	for _, id := range data.serviceIDs {
		_, _ = db.Exec(database.ConvertPlaceholders(`DELETE FROM service WHERE id = ?`), id)
	}
	for _, id := range data.typeIDs {
		_, _ = db.Exec(database.ConvertPlaceholders(`DELETE FROM ticket_type WHERE id = ?`), id)
	}
}

// createTestService creates a service for testing
func createTestService(t *testing.T, db *sql.DB, name string) int64 {
	t.Helper()

	// comments is nullable in the schema; leave it NULL like rows created without one.
	query := database.ConvertPlaceholders(`
		INSERT INTO service (name, valid_id, create_time, create_by, change_time, change_by)
		VALUES (?, 1, NOW(), 1, NOW(), 1)
		RETURNING id
	`)

	id, err := database.GetAdapter().InsertWithReturning(db, query, name)
	require.NoError(t, err)

	return id
}

// createTestType creates a ticket type with the given valid_id for testing
func createTestType(t *testing.T, db *sql.DB, name string, validID int) int64 {
	t.Helper()

	query := database.ConvertPlaceholders(`
		INSERT INTO ticket_type (name, valid_id, create_time, create_by, change_time, change_by)
		VALUES (?, ?, NOW(), 1, NOW(), 1)
		RETURNING id
	`)

	id, err := database.GetAdapter().InsertWithReturning(db, query, name, validID)
	require.NoError(t, err)

	return id
}

// setupGinContext creates a gin context for testing with authentication
func setupGinContext(w *httptest.ResponseRecorder, method, path string) (*gin.Context, *gin.Engine) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(method, path, nil)
	c.Set("user_id", 1) // Simulate authenticated user
	return c, r
}

func TestFilterByTicketAttributeRelations(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test")
	}

	t.Setenv("APP_ENV", "integration")

	db, err := database.GetDB()
	require.NoError(t, err)
	require.NotNil(t, db)

	testData := setupTestFilterData(t, db)
	defer cleanupTestFilterData(t, db, testData)

	ctx := context.Background()
	svc := ticketattributerelations.NewService(db)

	// Create test services
	serviceGold := createTestService(t, db, fmt.Sprintf("filtertest_Gold_%d", time.Now().UnixNano()))
	serviceSilver := createTestService(t, db, fmt.Sprintf("filtertest_Silver_%d", time.Now().UnixNano()))
	serviceBronze := createTestService(t, db, fmt.Sprintf("filtertest_Bronze_%d", time.Now().UnixNano()))
	testData.serviceIDs = append(testData.serviceIDs, serviceGold, serviceSilver, serviceBronze)

	// Get the actual service names we created
	var goldName, silverName, bronzeName string
	db.QueryRow(database.ConvertPlaceholders(`SELECT name FROM service WHERE id = ?`), serviceGold).Scan(&goldName)
	db.QueryRow(database.ConvertPlaceholders(`SELECT name FROM service WHERE id = ?`), serviceSilver).Scan(&silverName)
	db.QueryRow(database.ConvertPlaceholders(`SELECT name FROM service WHERE id = ?`), serviceBronze).Scan(&bronzeName)

	// Create a Queue -> Service relation
	relation := &models.TicketAttributeRelation{
		Filename:   fmt.Sprintf("filtertest_queue_service_%d.csv", time.Now().UnixNano()),
		Attribute1: "Queue",
		Attribute2: "Service",
		ACLData:    fmt.Sprintf("Queue;Service\nSales;%s\nSales;%s\nSupport;%s\nSupport;%s", goldName, silverName, silverName, bronzeName),
		Priority:   1,
		Data: []models.AttributeRelationPair{
			{Attribute1Value: "Sales", Attribute2Value: goldName},
			{Attribute1Value: "Sales", Attribute2Value: silverName},
			{Attribute1Value: "Support", Attribute2Value: silverName},
			{Attribute1Value: "Support", Attribute2Value: bronzeName},
		},
	}

	id, err := svc.Create(ctx, relation, 1)
	require.NoError(t, err)
	testData.relationIDs = append(testData.relationIDs, id)

	t.Run("FilterServices_QueueSales", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := setupGinContext(w, "GET", "/api/v1/services?filter_attribute=Queue&filter_value=Sales")
		c.Request.URL.RawQuery = "filter_attribute=Queue&filter_value=Sales"

		HandleListServicesAPI(c)

		assert.Equal(t, http.StatusOK, w.Code)

		var response struct {
			Success bool    `json:"success"`
			Data    []gin.H `json:"data"`
		}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)
		assert.True(t, response.Success)

		// Should only have Gold and Silver services
		var names []string
		for _, svc := range response.Data {
			if name, ok := svc["name"].(string); ok {
				names = append(names, name)
			}
		}

		assert.Contains(t, names, goldName)
		assert.Contains(t, names, silverName)
		assert.NotContains(t, names, bronzeName)
	})

	t.Run("FilterServices_QueueSupport", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := setupGinContext(w, "GET", "/api/v1/services?filter_attribute=Queue&filter_value=Support")
		c.Request.URL.RawQuery = "filter_attribute=Queue&filter_value=Support"

		HandleListServicesAPI(c)

		assert.Equal(t, http.StatusOK, w.Code)

		var response struct {
			Success bool    `json:"success"`
			Data    []gin.H `json:"data"`
		}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)
		assert.True(t, response.Success)

		// Should only have Silver and Bronze services
		var names []string
		for _, svc := range response.Data {
			if name, ok := svc["name"].(string); ok {
				names = append(names, name)
			}
		}

		assert.Contains(t, names, silverName)
		assert.Contains(t, names, bronzeName)
		assert.NotContains(t, names, goldName)
	})

	t.Run("FilterServices_NoFilter", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := setupGinContext(w, "GET", "/api/v1/services")

		HandleListServicesAPI(c)

		assert.Equal(t, http.StatusOK, w.Code)

		var response struct {
			Success bool    `json:"success"`
			Data    []gin.H `json:"data"`
		}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)
		assert.True(t, response.Success)

		// Should have all test services
		var names []string
		for _, svc := range response.Data {
			if name, ok := svc["name"].(string); ok {
				names = append(names, name)
			}
		}

		assert.Contains(t, names, goldName)
		assert.Contains(t, names, silverName)
		assert.Contains(t, names, bronzeName)
	})

	t.Run("FilterServices_UnknownQueue", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := setupGinContext(w, "GET", "/api/v1/services?filter_attribute=Queue&filter_value=Unknown")
		c.Request.URL.RawQuery = "filter_attribute=Queue&filter_value=Unknown"

		HandleListServicesAPI(c)

		assert.Equal(t, http.StatusOK, w.Code)

		var response struct {
			Success bool    `json:"success"`
			Data    []gin.H `json:"data"`
		}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)
		assert.True(t, response.Success)

		// No relation row names this queue, so services are unrestricted.
		assert.GreaterOrEqual(t, len(response.Data), 3)
	})

	t.Run("FilterServices_NoAllowedServiceExists", func(t *testing.T) {
		// The relation restricts queue Ghost to a service that does not exist:
		// restrictions apply but nothing matches, so nothing may be offered.
		ghost := &models.TicketAttributeRelation{
			Filename:   fmt.Sprintf("filtertest_ghost_service_%d.csv", time.Now().UnixNano()),
			Attribute1: "Queue",
			Attribute2: "Service",
			ACLData:    "Queue;Service\nGhost;filtertest_NoSuchService",
			Priority:   2,
			Data: []models.AttributeRelationPair{
				{Attribute1Value: "Ghost", Attribute2Value: "filtertest_NoSuchService"},
			},
		}
		ghostID, err := svc.Create(ctx, ghost, 1)
		require.NoError(t, err)
		testData.relationIDs = append(testData.relationIDs, ghostID)

		w := httptest.NewRecorder()
		c, _ := setupGinContext(w, "GET", "/api/v1/services?filter_attribute=Queue&filter_value=Ghost")
		c.Request.URL.RawQuery = "filter_attribute=Queue&filter_value=Ghost"

		HandleListServicesAPI(c)

		require.Equal(t, http.StatusOK, w.Code, w.Body.String())
		var response struct {
			Success bool    `json:"success"`
			Data    []gin.H `json:"data"`
		}
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &response))
		assert.True(t, response.Success)
		assert.Empty(t, response.Data, "restricted list with no allowed match must be empty, got %s", w.Body.String())
		assert.NotContains(t, w.Body.String(), goldName)
		assert.NotContains(t, w.Body.String(), bronzeName)
	})
}

func TestFilterTypes(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test")
	}

	t.Setenv("APP_ENV", "integration")

	db, err := database.GetDB()
	require.NoError(t, err)
	require.NotNil(t, db)

	testData := setupTestFilterData(t, db)
	defer cleanupTestFilterData(t, db, testData)

	ctx := context.Background()
	svc := ticketattributerelations.NewService(db)

	// Create test types (one invalid, which the list must hide unless valid=all)
	stamp := time.Now().UnixNano()
	incidentName := fmt.Sprintf("filtertest_Incident_%d", stamp)
	requestName := fmt.Sprintf("filtertest_Request_%d", stamp)
	problemName := fmt.Sprintf("filtertest_Problem_%d", stamp)
	retiredName := fmt.Sprintf("filtertest_Retired_%d", stamp)
	testData.typeIDs = append(testData.typeIDs,
		createTestType(t, db, incidentName, 1),
		createTestType(t, db, requestName, 1),
		createTestType(t, db, problemName, 1),
		createTestType(t, db, retiredName, 2),
	)

	// Create a Queue -> Type relation
	relation := &models.TicketAttributeRelation{
		Filename:   fmt.Sprintf("filtertest_queue_type_%d.csv", time.Now().UnixNano()),
		Attribute1: "Queue",
		Attribute2: "Type",
		ACLData:    fmt.Sprintf("Queue;Type\nIT;%s\nIT;%s\nHR;%s", incidentName, problemName, requestName),
		Priority:   1,
		Data: []models.AttributeRelationPair{
			{Attribute1Value: "IT", Attribute2Value: incidentName},
			{Attribute1Value: "IT", Attribute2Value: problemName},
			{Attribute1Value: "HR", Attribute2Value: requestName},
		},
	}

	id, err := svc.Create(ctx, relation, 1)
	require.NoError(t, err)
	testData.relationIDs = append(testData.relationIDs, id)

	t.Run("FilterTypes_QueueIT", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := setupGinContext(w, "GET", "/api/v1/types?filter_attribute=Queue&filter_value=IT")
		c.Request.URL.RawQuery = "filter_attribute=Queue&filter_value=IT"

		HandleListTypesAPI(c)

		assert.Equal(t, http.StatusOK, w.Code)

		var response struct {
			Success bool    `json:"success"`
			Data    []gin.H `json:"data"`
		}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)
		assert.True(t, response.Success)

		// Should only have Incident and Problem types
		var names []string
		for _, typ := range response.Data {
			if name, ok := typ["name"].(string); ok {
				names = append(names, name)
			}
		}

		assert.Contains(t, names, incidentName)
		assert.Contains(t, names, problemName)
		assert.NotContains(t, names, requestName)
	})

	t.Run("FilterTypes_QueueHR", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := setupGinContext(w, "GET", "/api/v1/types?filter_attribute=Queue&filter_value=HR")
		c.Request.URL.RawQuery = "filter_attribute=Queue&filter_value=HR"

		HandleListTypesAPI(c)

		assert.Equal(t, http.StatusOK, w.Code)

		var response struct {
			Success bool    `json:"success"`
			Data    []gin.H `json:"data"`
		}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)
		assert.True(t, response.Success)

		// Should only have Request type
		var names []string
		for _, typ := range response.Data {
			if name, ok := typ["name"].(string); ok {
				names = append(names, name)
			}
		}

		assert.Contains(t, names, requestName)
		assert.NotContains(t, names, incidentName)
		assert.NotContains(t, names, problemName)
	})

	listTypeNames := func(t *testing.T, rawQuery string) []string {
		t.Helper()
		w := httptest.NewRecorder()
		c, _ := setupGinContext(w, "GET", "/api/v1/types?"+rawQuery)

		HandleListTypesAPI(c)

		require.Equal(t, http.StatusOK, w.Code, w.Body.String())
		var response struct {
			Success bool    `json:"success"`
			Data    []gin.H `json:"data"`
		}
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &response))
		require.True(t, response.Success)
		var names []string
		for _, typ := range response.Data {
			if name, ok := typ["name"].(string); ok {
				names = append(names, name)
			}
		}
		return names
	}

	t.Run("ListTypes_DefaultHidesInvalid", func(t *testing.T) {
		names := listTypeNames(t, "")
		assert.Contains(t, names, incidentName)
		assert.Contains(t, names, requestName)
		assert.Contains(t, names, problemName)
		assert.NotContains(t, names, retiredName)
	})

	t.Run("ListTypes_ValidAllIncludesInvalid", func(t *testing.T) {
		names := listTypeNames(t, "valid=all")
		assert.Contains(t, names, incidentName)
		assert.Contains(t, names, retiredName)
	})

	t.Run("ListTypes_ValidFalseOnlyInvalid", func(t *testing.T) {
		names := listTypeNames(t, "valid=false")
		assert.Contains(t, names, retiredName)
		assert.NotContains(t, names, incidentName)
	})
}

func TestEvaluateEndpoint(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test")
	}

	t.Setenv("APP_ENV", "integration")

	db, err := database.GetDB()
	require.NoError(t, err)
	require.NotNil(t, db)

	testData := setupTestFilterData(t, db)
	defer cleanupTestFilterData(t, db, testData)

	ctx := context.Background()
	svc := ticketattributerelations.NewService(db)

	// Create a Queue -> Priority relation
	relation := &models.TicketAttributeRelation{
		Filename:   fmt.Sprintf("filtertest_queue_priority_%d.csv", time.Now().UnixNano()),
		Attribute1: "Queue",
		Attribute2: "Priority",
		ACLData:    "Queue;Priority\nSales;3 normal\nSales;4 high\nSupport;4 high\nSupport;5 very high",
		Priority:   1,
		Data: []models.AttributeRelationPair{
			{Attribute1Value: "Sales", Attribute2Value: "3 normal"},
			{Attribute1Value: "Sales", Attribute2Value: "4 high"},
			{Attribute1Value: "Support", Attribute2Value: "4 high"},
			{Attribute1Value: "Support", Attribute2Value: "5 very high"},
		},
	}

	id, err := svc.Create(ctx, relation, 1)
	require.NoError(t, err)
	testData.relationIDs = append(testData.relationIDs, id)

	t.Run("Evaluate_QueueSales", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := setupGinContext(w, "GET", "/admin/api/ticket-attribute-relations/evaluate?attribute=Queue&value=Sales")
		c.Request.URL.RawQuery = "attribute=Queue&value=Sales"

		handleAPITicketAttributeRelationsEvaluate(c)

		assert.Equal(t, http.StatusOK, w.Code)

		var response struct {
			Success       bool                `json:"success"`
			AllowedValues map[string][]string `json:"allowed_values"`
		}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)
		assert.True(t, response.Success)

		// Should have Priority values for Sales
		assert.Contains(t, response.AllowedValues, "Priority")
		priorities := response.AllowedValues["Priority"]
		assert.Contains(t, priorities, "3 normal")
		assert.Contains(t, priorities, "4 high")
		assert.NotContains(t, priorities, "5 very high")
	})

	t.Run("Evaluate_QueueSupport", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := setupGinContext(w, "GET", "/admin/api/ticket-attribute-relations/evaluate?attribute=Queue&value=Support")
		c.Request.URL.RawQuery = "attribute=Queue&value=Support"

		handleAPITicketAttributeRelationsEvaluate(c)

		assert.Equal(t, http.StatusOK, w.Code)

		var response struct {
			Success       bool                `json:"success"`
			AllowedValues map[string][]string `json:"allowed_values"`
		}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)
		assert.True(t, response.Success)

		// Should have Priority values for Support
		assert.Contains(t, response.AllowedValues, "Priority")
		priorities := response.AllowedValues["Priority"]
		assert.Contains(t, priorities, "4 high")
		assert.Contains(t, priorities, "5 very high")
		assert.NotContains(t, priorities, "3 normal")
	})

	t.Run("Evaluate_MissingAttribute", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := setupGinContext(w, "GET", "/admin/api/ticket-attribute-relations/evaluate?value=Sales")
		c.Request.URL.RawQuery = "value=Sales"

		handleAPITicketAttributeRelationsEvaluate(c)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	// The form JS (static/js/ticket-attribute-relations.js) evaluates with an empty
	// value when a selection is cleared; that must succeed with no restrictions.
	t.Run("Evaluate_EmptyValueMeansNoRestrictions", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := setupGinContext(w, "GET", "/admin/api/ticket-attribute-relations/evaluate?attribute=Queue")
		c.Request.URL.RawQuery = "attribute=Queue"

		handleAPITicketAttributeRelationsEvaluate(c)

		require.Equal(t, http.StatusOK, w.Code)
		var response struct {
			Success       bool                `json:"success"`
			AllowedValues map[string][]string `json:"allowed_values"`
		}
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &response))
		assert.True(t, response.Success)
		assert.Empty(t, response.AllowedValues)
	})

	t.Run("Evaluate_UnknownAttribute", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := setupGinContext(w, "GET", "/admin/api/ticket-attribute-relations/evaluate?attribute=Unknown&value=Test")
		c.Request.URL.RawQuery = "attribute=Unknown&value=Test"

		handleAPITicketAttributeRelationsEvaluate(c)

		assert.Equal(t, http.StatusOK, w.Code)

		var response struct {
			Success       bool                `json:"success"`
			AllowedValues map[string][]string `json:"allowed_values"`
		}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)
		assert.True(t, response.Success)

		// Should return empty allowed_values
		assert.Empty(t, response.AllowedValues)
	})
}

// unreachableConnector makes every query fail, standing in for a database that
// cannot evaluate the relations (outage, broken table, permission error).
type unreachableConnector struct{}

func (unreachableConnector) Connect(context.Context) (driver.Conn, error) {
	return nil, errors.New("forced connection failure")
}
func (unreachableConnector) Driver() driver.Driver { return unreachableDriver{} }

type unreachableDriver struct{}

func (unreachableDriver) Open(string) (driver.Conn, error) {
	return nil, errors.New("forced connection failure")
}

// Relation filtering is an access restriction: when it cannot be evaluated the
// list endpoints must fail closed instead of returning every item.
func TestFilterByTicketAttributeRelations_FailsClosed(t *testing.T) {
	broken := sql.OpenDB(unreachableConnector{})
	defer broken.Close()

	w := httptest.NewRecorder()
	c, _ := setupGinContext(w, "GET", "/api/v1/types?filter_attribute=Queue&filter_value=IT")
	items := []gin.H{{"id": 1, "name": "Incident"}, {"id": 2, "name": "Problem"}}

	got, err := filterByTicketAttributeRelations(c, broken, items, "Type", "Queue", "IT")
	require.Error(t, err)
	assert.Nil(t, got, "no list may be returned when relations cannot be evaluated")

	respondAttributeRelationFilterError(c, err)
	assert.Equal(t, http.StatusInternalServerError, w.Code)
	assert.NotContains(t, w.Body.String(), "Incident")
}
