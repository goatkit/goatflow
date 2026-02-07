//go:build integration

package repository

import (
	"context"
	"testing"

	"github.com/goatkit/goatflow/internal/models"
)

func TestWebserviceRepository_CRUD(t *testing.T) {
	db, err := getTestDB()
	if err != nil {
		t.Fatalf("failed to get test db: %v", err)
	}
	defer db.Close()

	repo := NewWebserviceRepository(db)
	ctx := context.Background()

	// Test Create
	ws := &models.WebserviceConfig{
		Name:    "TestWebservice",
		ValidID: 1,
		Config: &models.WebserviceConfigData{
			Description:  "Test webservice for unit tests",
			RemoteSystem: "TestSystem",
			Debugger: models.DebuggerConfig{
				DebugThreshold: "debug",
				TestMode:       "1",
			},
			Requester: models.RequesterConfig{
				Transport: models.TransportConfig{
					Type: "HTTP::REST",
					Config: models.TransportHTTPConfig{
						Host:           "https://api.example.com",
						DefaultCommand: "GET",
						Timeout:        "30",
					},
				},
				Invoker: map[string]models.InvokerConfig{
					"TestSearch": {
						Type:        "Test::Search",
						Description: "Search invoker",
					},
					"TestGet": {
						Type:        "Test::Get",
						Description: "Get invoker",
					},
				},
			},
		},
	}

	// Clean up any existing test webservice
	if existing, _ := repo.GetByName(ctx, "TestWebservice"); existing != nil {
		repo.Delete(ctx, existing.ID)
	}

	id, err := repo.Create(ctx, ws, 1)
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	if id == 0 {
		t.Fatal("Create returned zero ID")
	}
	defer repo.Delete(ctx, id)

	// Test GetByID
	retrieved, err := repo.GetByID(ctx, id)
	if err != nil {
		t.Fatalf("GetByID failed: %v", err)
	}
	if retrieved.Name != ws.Name {
		t.Errorf("Name mismatch: got %q, want %q", retrieved.Name, ws.Name)
	}
	if retrieved.Config.Description != ws.Config.Description {
		t.Errorf("Description mismatch: got %q, want %q", retrieved.Config.Description, ws.Config.Description)
	}
	if retrieved.Config.Requester.Transport.Type != "HTTP::REST" {
		t.Errorf("Transport type mismatch: got %q, want %q", retrieved.Config.Requester.Transport.Type, "HTTP::REST")
	}
	if len(retrieved.Config.Requester.Invoker) != 2 {
		t.Errorf("Invoker count mismatch: got %d, want 2", len(retrieved.Config.Requester.Invoker))
	}

	// Test GetByName
	byName, err := repo.GetByName(ctx, "TestWebservice")
	if err != nil {
		t.Fatalf("GetByName failed: %v", err)
	}
	if byName.ID != id {
		t.Errorf("ID mismatch: got %d, want %d", byName.ID, id)
	}

	// Test Exists
	exists, err := repo.Exists(ctx, "TestWebservice")
	if err != nil {
		t.Fatalf("Exists failed: %v", err)
	}
	if !exists {
		t.Error("Exists returned false for existing webservice")
	}

	notExists, err := repo.Exists(ctx, "NonExistent")
	if err != nil {
		t.Fatalf("Exists failed: %v", err)
	}
	if notExists {
		t.Error("Exists returned true for non-existent webservice")
	}

	// Test List
	list, err := repo.List(ctx)
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	found := false
	for _, w := range list {
		if w.ID == id {
			found = true
			break
		}
	}
	if !found {
		t.Error("List did not include created webservice")
	}

	// Test ListValid
	validList, err := repo.ListValid(ctx)
	if err != nil {
		t.Fatalf("ListValid failed: %v", err)
	}
	found = false
	for _, w := range validList {
		if w.ID == id {
			found = true
			break
		}
	}
	if !found {
		t.Error("ListValid did not include valid webservice")
	}

	// Test Update
	retrieved.Config.Description = "Updated description"
	retrieved.Config.Requester.Transport.Config.Host = "https://api2.example.com"
	err = repo.Update(ctx, retrieved, 1)
	if err != nil {
		t.Fatalf("Update failed: %v", err)
	}

	updated, err := repo.GetByID(ctx, id)
	if err != nil {
		t.Fatalf("GetByID after update failed: %v", err)
	}
	if updated.Config.Description != "Updated description" {
		t.Errorf("Description not updated: got %q", updated.Config.Description)
	}
	if updated.Config.Requester.Transport.Config.Host != "https://api2.example.com" {
		t.Errorf("Host not updated: got %q", updated.Config.Requester.Transport.Config.Host)
	}

	// Test GetHistory
	history, err := repo.GetHistory(ctx, id)
	if err != nil {
		t.Fatalf("GetHistory failed: %v", err)
	}
	if len(history) < 1 {
		t.Error("History should have at least one entry")
	}

	// Test GetValidWebservicesForField
	forField, err := repo.GetValidWebservicesForField(ctx)
	if err != nil {
		t.Fatalf("GetValidWebservicesForField failed: %v", err)
	}
	found = false
	for _, w := range forField {
		if w.ID == id {
			found = true
			break
		}
	}
	if !found {
		t.Error("GetValidWebservicesForField did not include webservice with invokers")
	}
}

func TestWebserviceRepository_ExistsExcluding(t *testing.T) {
	db, err := getTestDB()
	if err != nil {
		t.Fatalf("failed to get test db: %v", err)
	}
	defer db.Close()

	repo := NewWebserviceRepository(db)
	ctx := context.Background()

	// Clean up any existing test webservices
	if existing, _ := repo.GetByName(ctx, "Webservice1"); existing != nil {
		repo.Delete(ctx, existing.ID)
	}
	if existing, _ := repo.GetByName(ctx, "Webservice2"); existing != nil {
		repo.Delete(ctx, existing.ID)
	}

	// Create first webservice
	ws1 := &models.WebserviceConfig{
		Name:    "Webservice1",
		ValidID: 1,
		Config:  &models.WebserviceConfigData{},
	}
	id1, err := repo.Create(ctx, ws1, 1)
	if err != nil {
		t.Fatalf("Create ws1 failed: %v", err)
	}
	defer repo.Delete(ctx, id1)

	// Create second webservice
	ws2 := &models.WebserviceConfig{
		Name:    "Webservice2",
		ValidID: 1,
		Config:  &models.WebserviceConfigData{},
	}
	id2, err := repo.Create(ctx, ws2, 1)
	if err != nil {
		t.Fatalf("Create ws2 failed: %v", err)
	}
	defer repo.Delete(ctx, id2)

	// Check ExistsExcluding
	exists, err := repo.ExistsExcluding(ctx, "Webservice1", id2)
	if err != nil {
		t.Fatalf("ExistsExcluding failed: %v", err)
	}
	if !exists {
		t.Error("ExistsExcluding should return true when name exists with different ID")
	}

	exists, err = repo.ExistsExcluding(ctx, "Webservice1", id1)
	if err != nil {
		t.Fatalf("ExistsExcluding failed: %v", err)
	}
	if exists {
		t.Error("ExistsExcluding should return false when name exists but is excluded")
	}
}

func TestWebserviceConfig_HelperMethods(t *testing.T) {
	ws := &models.WebserviceConfig{
		ValidID: 1,
		Config: &models.WebserviceConfigData{
			Requester: models.RequesterConfig{
				Transport: models.TransportConfig{
					Type: "HTTP::REST",
					Config: models.TransportHTTPConfig{
						Host: "https://api.example.com",
					},
				},
				Invoker: map[string]models.InvokerConfig{
					"Search": {Type: "Test::Search"},
					"Get":    {Type: "Test::Get"},
				},
			},
			Provider: models.ProviderConfig{
				Operation: map[string]models.OperationConfig{
					"TicketGet": {Type: "Ticket::Get"},
				},
			},
		},
	}

	// Test IsValid
	if !ws.IsValid() {
		t.Error("IsValid should return true for ValidID=1")
	}
	ws.ValidID = 2
	if ws.IsValid() {
		t.Error("IsValid should return false for ValidID!=1")
	}
	ws.ValidID = 1

	// Test GetInvoker
	inv := ws.GetInvoker("Search")
	if inv == nil {
		t.Error("GetInvoker should return invoker")
	}
	if inv.Type != "Test::Search" {
		t.Errorf("GetInvoker type mismatch: got %q", inv.Type)
	}
	inv = ws.GetInvoker("NonExistent")
	if inv != nil {
		t.Error("GetInvoker should return nil for non-existent invoker")
	}

	// Test GetOperation
	op := ws.GetOperation("TicketGet")
	if op == nil {
		t.Error("GetOperation should return operation")
	}
	if op.Type != "Ticket::Get" {
		t.Errorf("GetOperation type mismatch: got %q", op.Type)
	}
	op = ws.GetOperation("NonExistent")
	if op != nil {
		t.Error("GetOperation should return nil for non-existent operation")
	}

	// Test InvokerNames
	names := ws.InvokerNames()
	if len(names) != 2 {
		t.Errorf("InvokerNames count mismatch: got %d, want 2", len(names))
	}

	// Test OperationNames
	opNames := ws.OperationNames()
	if len(opNames) != 1 {
		t.Errorf("OperationNames count mismatch: got %d, want 1", len(opNames))
	}

	// Test TransportType
	if ws.TransportType() != "HTTP::REST" {
		t.Errorf("TransportType mismatch: got %q", ws.TransportType())
	}

	// Test RequesterHost
	if ws.RequesterHost() != "https://api.example.com" {
		t.Errorf("RequesterHost mismatch: got %q", ws.RequesterHost())
	}
}
