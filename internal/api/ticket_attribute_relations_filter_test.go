//go:build integration

package api

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/goatkit/goatflow/internal/database"
	"github.com/goatkit/goatflow/internal/models"
	"github.com/goatkit/goatflow/internal/services/ticketattributerelations"
)

// testFilterData holds test data for cleanup
type testFilterData struct {
	relationIDs []int64
	serviceIDs  []int64
	slaIDs      []int64
	typeIDs     []int64
}

// setupTestFilterData creates test data for filtering tests
func setupTestFilterData(t *testing.T, db *sql.DB) *testFilterData {
	t.Helper()
	data := &testFilterData{}

	// Cleanup any leftover test data
	_, _ = db.Exec(`DELETE FROM acl_ticket_attribute_relations WHERE filename LIKE 'filtertest_%'`)

	return data
}

// cleanupTestFilterData removes all test data
func cleanupTestFilterData(t *testing.T, db *sql.DB, data *testFilterData) {
	t.Helper()

	// Delete test relations
	for _, id := range data.relationIDs {
		_, _ = db.Exec(database.ConvertPlaceholders(`DELETE FROM acl_ticket_attribute_relations WHERE id = ?`), id)
	}
	_, _ = db.Exec(`DELETE FROM acl_ticket_attribute_relations WHERE filename LIKE 'filtertest_%'`)

	// Delete test services (soft delete by setting valid_id = 2)
	for _, id := range data.serviceIDs {
		_, _ = db.Exec(database.ConvertPlaceholders(`UPDATE service SET valid_id = 2 WHERE id = ?`), id)
	}

	// Delete test SLAs (soft delete)
	for _, id := range data.slaIDs {
		_, _ = db.Exec(database.ConvertPlaceholders(`UPDATE sla SET valid_id = 2 WHERE id = ?`), id)
	}

	// Delete test types (soft delete)
	for _, id := range data.typeIDs {
		_, _ = db.Exec(database.ConvertPlaceholders(`UPDATE ticket_type SET valid_id = 2 WHERE id = ?`), id)
	}
}

// createTestService creates a service for testing
func createTestService(t *testing.T, db *sql.DB, name string) int64 {
	t.Helper()

	query := database.ConvertPlaceholders(`
		INSERT INTO service (name, valid_id, comments, create_time, create_by, change_time, change_by)
		VALUES (?, 1, 'Test service', NOW(), 1, NOW(), 1)
	`)

	result, err := db.Exec(query, name)
	require.NoError(t, err)

	id, err := result.LastInsertId()
	require.NoError(t, err)

	return id
}

// createTestSLA creates an SLA for testing
func createTestSLA(t *testing.T, db *sql.DB, name string) int64 {
	t.Helper()

	query := database.ConvertPlaceholders(`
		INSERT INTO sla (name, first_response_time, solution_time, valid_id, create_time, create_by, change_time, change_by)
		VALUES (?, 60, 240, 1, NOW(), 1, NOW(), 1)
	`)

	result, err := db.Exec(query, name)
	require.NoError(t, err)

	id, err := result.LastInsertId()
	require.NoError(t, err)

	return id
}

// createTestType creates a ticket type for testing
func createTestType(t *testing.T, db *sql.DB, name string) int64 {
	t.Helper()

	query := database.ConvertPlaceholders(`
		INSERT INTO ticket_type (name, valid_id, create_time, create_by, change_time, change_by)
		VALUES (?, 1, NOW(), 1, NOW(), 1)
	`)

	result, err := db.Exec(query, name)
	require.NoError(t, err)

	id, err := result.LastInsertId()
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
			Success bool     `json:"success"`
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
			Success bool     `json:"success"`
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
			Success bool     `json:"success"`
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
			Success bool     `json:"success"`
			Data    []gin.H `json:"data"`
		}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)
		assert.True(t, response.Success)

		// When no matches, should return all items (fallback behavior)
		assert.GreaterOrEqual(t, len(response.Data), 3)
	})
}

func TestFilterSLAs(t *testing.T) {
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

	// Create test SLAs
	slaPremium := createTestSLA(t, db, fmt.Sprintf("filtertest_Premium_%d", time.Now().UnixNano()))
	slaStandard := createTestSLA(t, db, fmt.Sprintf("filtertest_Standard_%d", time.Now().UnixNano()))
	slaBasic := createTestSLA(t, db, fmt.Sprintf("filtertest_Basic_%d", time.Now().UnixNano()))
	testData.slaIDs = append(testData.slaIDs, slaPremium, slaStandard, slaBasic)

	// Get the actual SLA names
	var premiumName, standardName, basicName string
	db.QueryRow(database.ConvertPlaceholders(`SELECT name FROM sla WHERE id = ?`), slaPremium).Scan(&premiumName)
	db.QueryRow(database.ConvertPlaceholders(`SELECT name FROM sla WHERE id = ?`), slaStandard).Scan(&standardName)
	db.QueryRow(database.ConvertPlaceholders(`SELECT name FROM sla WHERE id = ?`), slaBasic).Scan(&basicName)

	// Create a Service -> SLA relation
	relation := &models.TicketAttributeRelation{
		Filename:   fmt.Sprintf("filtertest_service_sla_%d.csv", time.Now().UnixNano()),
		Attribute1: "Service",
		Attribute2: "SLA",
		ACLData:    fmt.Sprintf("Service;SLA\nEnterprise;%s\nEnterprise;%s\nStarter;%s", premiumName, standardName, basicName),
		Priority:   1,
		Data: []models.AttributeRelationPair{
			{Attribute1Value: "Enterprise", Attribute2Value: premiumName},
			{Attribute1Value: "Enterprise", Attribute2Value: standardName},
			{Attribute1Value: "Starter", Attribute2Value: basicName},
		},
	}

	id, err := svc.Create(ctx, relation, 1)
	require.NoError(t, err)
	testData.relationIDs = append(testData.relationIDs, id)

	t.Run("FilterSLAs_ServiceEnterprise", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := setupGinContext(w, "GET", "/api/v1/slas?filter_attribute=Service&filter_value=Enterprise")
		c.Request.URL.RawQuery = "filter_attribute=Service&filter_value=Enterprise"

		HandleListSLAsAPI(c)

		assert.Equal(t, http.StatusOK, w.Code)

		var response struct {
			SLAs  []gin.H `json:"slas"`
			Total int     `json:"total"`
		}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)

		// Should only have Premium and Standard SLAs
		var names []string
		for _, sla := range response.SLAs {
			if name, ok := sla["name"].(string); ok {
				names = append(names, name)
			}
		}

		assert.Contains(t, names, premiumName)
		assert.Contains(t, names, standardName)
		assert.NotContains(t, names, basicName)
	})

	t.Run("FilterSLAs_ServiceStarter", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := setupGinContext(w, "GET", "/api/v1/slas?filter_attribute=Service&filter_value=Starter")
		c.Request.URL.RawQuery = "filter_attribute=Service&filter_value=Starter"

		HandleListSLAsAPI(c)

		assert.Equal(t, http.StatusOK, w.Code)

		var response struct {
			SLAs  []gin.H `json:"slas"`
			Total int     `json:"total"`
		}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)

		// Should only have Basic SLA
		var names []string
		for _, sla := range response.SLAs {
			if name, ok := sla["name"].(string); ok {
				names = append(names, name)
			}
		}

		assert.Contains(t, names, basicName)
		assert.NotContains(t, names, premiumName)
		assert.NotContains(t, names, standardName)
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

	// Create test types
	typeIncident := createTestType(t, db, fmt.Sprintf("filtertest_Incident_%d", time.Now().UnixNano()))
	typeRequest := createTestType(t, db, fmt.Sprintf("filtertest_Request_%d", time.Now().UnixNano()))
	typeProblem := createTestType(t, db, fmt.Sprintf("filtertest_Problem_%d", time.Now().UnixNano()))
	testData.typeIDs = append(testData.typeIDs, typeIncident, typeRequest, typeProblem)

	// Get the actual type names
	var incidentName, requestName, problemName string
	db.QueryRow(database.ConvertPlaceholders(`SELECT name FROM ticket_type WHERE id = ?`), typeIncident).Scan(&incidentName)
	db.QueryRow(database.ConvertPlaceholders(`SELECT name FROM ticket_type WHERE id = ?`), typeRequest).Scan(&requestName)
	db.QueryRow(database.ConvertPlaceholders(`SELECT name FROM ticket_type WHERE id = ?`), typeProblem).Scan(&problemName)

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
			Success bool     `json:"success"`
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
			Success bool     `json:"success"`
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

	t.Run("Evaluate_MissingValue", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := setupGinContext(w, "GET", "/admin/api/ticket-attribute-relations/evaluate?attribute=Queue")
		c.Request.URL.RawQuery = "attribute=Queue"

		handleAPITicketAttributeRelationsEvaluate(c)

		assert.Equal(t, http.StatusBadRequest, w.Code)
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
