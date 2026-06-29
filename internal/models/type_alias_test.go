package models

import (
	"fmt"
	platformmodels "github.com/goatkit/goatflow/internal/platform/models"
	"testing"
)

func TestTypeAliasIdentity(t *testing.T) {
	var u1 User
	var u2 platformmodels.User
	if fmt.Sprintf("%T", u1) != fmt.Sprintf("%T", u2) {
		t.Error("User types not identical")
	}
	var g1 Group
	var g2 platformmodels.Group
	if fmt.Sprintf("%T", g1) != fmt.Sprintf("%T", g2) {
		t.Error("Group types not identical")
	}
	var r1 Role
	var r2 platformmodels.Role
	if fmt.Sprintf("%T", r1) != fmt.Sprintf("%T", r2) {
		t.Error("Role types not identical")
	}
	var s1 Session
	var s2 platformmodels.Session
	if fmt.Sprintf("%T", s1) != fmt.Sprintf("%T", s2) {
		t.Error("Session types not identical")
	}
	if RoleAdmin != platformmodels.RoleAdmin {
		t.Error("RoleAdmin const not identical")
	}
	if TokenPrefix != platformmodels.TokenPrefix {
		t.Error("TokenPrefix const not identical")
	}
}
