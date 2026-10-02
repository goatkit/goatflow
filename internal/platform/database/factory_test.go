package database_test

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/goatkit/goatflow/internal/platform/database"
)

func validConfig(t database.DatabaseType) database.DatabaseConfig {
	return database.DatabaseConfig{
		Type:     t,
		Host:     "db.example.invalid",
		Port:     "1",
		Database: "goatflow",
		Username: "goatflow",
	}
}

func TestSupportedTypesListsOnlyImplementedDatabases(t *testing.T) {
	got := database.NewDatabaseFactory().GetSupportedTypes()
	want := []database.DatabaseType{database.PostgreSQL, database.MySQL}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("GetSupportedTypes() = %v, want %v", got, want)
	}
}

func TestPlannedDatabaseTypesAreNotImplemented(t *testing.T) {
	ctx := context.Background()
	for _, dbType := range []database.DatabaseType{database.Oracle, database.SQLServer} {
		t.Run(string(dbType), func(t *testing.T) {
			db, err := database.NewDatabaseFactory().Create(validConfig(dbType))
			if err != nil {
				t.Fatalf("Create: %v", err)
			}
			if db.GetType() != dbType {
				t.Fatalf("GetType() = %q, want %q", db.GetType(), dbType)
			}

			err = db.Connect()
			if !errors.Is(err, database.ErrDatabaseNotImplemented) {
				t.Fatalf("Connect() error = %v, want ErrDatabaseNotImplemented", err)
			}
			if !strings.Contains(err.Error(), string(dbType)) {
				t.Errorf("Connect() error %q does not name the database type", err)
			}

			if _, err := db.Query(ctx, "SELECT 1"); !errors.Is(err, database.ErrDatabaseNotImplemented) {
				t.Errorf("Query() error = %v, want ErrDatabaseNotImplemented", err)
			}
			if _, err := db.Exec(ctx, "SELECT 1"); !errors.Is(err, database.ErrDatabaseNotImplemented) {
				t.Errorf("Exec() error = %v, want ErrDatabaseNotImplemented", err)
			}
			if _, err := db.Begin(ctx); !errors.Is(err, database.ErrDatabaseNotImplemented) {
				t.Errorf("Begin() error = %v, want ErrDatabaseNotImplemented", err)
			}
			if db.IsHealthy() {
				t.Error("IsHealthy() = true for an unimplemented database")
			}

			if f := database.GetDatabaseFeatures(dbType); f != (database.DatabaseFeatures{}) {
				t.Errorf("GetDatabaseFeatures(%s) claims capabilities: %+v", dbType, f)
			}
		})
	}
}

func TestUnknownDatabaseTypeIsRejected(t *testing.T) {
	_, err := database.NewDatabaseFactory().Create(validConfig("sqlite"))
	if err == nil {
		t.Fatal("Create(sqlite) succeeded, want error")
	}
	if errors.Is(err, database.ErrDatabaseNotImplemented) {
		t.Fatalf("Create(sqlite) error = %v, want unsupported-type error, not ErrDatabaseNotImplemented", err)
	}
	if !strings.Contains(err.Error(), "supported types: postgresql, mysql") {
		t.Errorf("Create(sqlite) error %q does not list the supported types", err)
	}
}
