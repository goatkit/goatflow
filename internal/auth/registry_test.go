package auth

import (
    "context"
    "os"
    "testing"
    "database/sql"
)

// simpleFakeProvider for ordering tests
type simpleFakeProvider struct { name string; priority int }
func (p *simpleFakeProvider) Authenticate(ctx context.Context, u, pw string) (*UserStub, error) { return nil, ErrInvalidCredentials }

// We can't import models.User here without pulling other deps in tests below; use real interface implementations instead.

// Re-implement minimal methods to satisfy interface using models.User would require import; instead embed logic in separate test file.
