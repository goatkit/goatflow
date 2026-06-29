package apierrors

import (
	"net/http"
	"testing"
)

func TestRegistry_CoreCodesRegistered(t *testing.T) {
	// Core codes should be registered via init()
	codes := Registry.All()
	if len(codes) == 0 {
		t.Fatal("No codes registered")
	}

	// Check a few core codes exist
	mustExist := []string{
		CodeUnauthorized,
		CodeForbidden,
		CodeNotFound,
		CodeInvalidRequest,
		CodeInternalError,
		CodeTokenNotFound,
	}

	for _, code := range mustExist {
		if _, ok := Registry.Get(code); !ok {
			t.Errorf("Core code %q not registered", code)
		}
	}
}

func TestRegistry_Namespacing(t *testing.T) {
	// All core codes should be in "core" namespace
	coreCodes := Registry.ByNamespace("core")
	if len(coreCodes) == 0 {
		t.Fatal("No codes in 'core' namespace")
	}

	for _, code := range coreCodes {
		if len(code.Code) < 5 || code.Code[:5] != "core:" {
			t.Errorf("Code %q should have 'core:' prefix", code.Code)
		}
	}
}

func TestRegistry_HTTPStatus(t *testing.T) {
	tests := []struct {
		code   string
		status int
	}{
		{CodeUnauthorized, http.StatusUnauthorized},
		{CodeForbidden, http.StatusForbidden},
		{CodeNotFound, http.StatusNotFound},
		{CodeInvalidRequest, http.StatusBadRequest},
		{CodeInternalError, http.StatusInternalServerError},
		{CodeRateLimited, http.StatusTooManyRequests},
	}

	for _, tt := range tests {
		t.Run(tt.code, func(t *testing.T) {
			if got := Registry.HTTPStatus(tt.code); got != tt.status {
				t.Errorf("HTTPStatus(%q) = %d, want %d", tt.code, got, tt.status)
			}
		})
	}
}

func TestRegistry_UnknownCode(t *testing.T) {
	// Unknown code should return 500 status
	status := Registry.HTTPStatus("unknown:code")
	if status != http.StatusInternalServerError {
		t.Errorf("HTTPStatus for unknown code = %d, want %d", status, http.StatusInternalServerError)
	}

	// Unknown code message should be the code itself
	msg := Registry.Message("unknown:code")
	if msg != "unknown:code" {
		t.Errorf("Message for unknown code = %q, want %q", msg, "unknown:code")
	}
}

func TestRegistry_RegisterPlugin(t *testing.T) {
	// Create a mock plugin enumerator
	mockPlugin := &mockEnumerator{
		codes: []ErrorCode{
			{Code: "test_error", Message: "Test error", HTTPStatus: 400},
			{Code: "another_error", Message: "Another error", HTTPStatus: 500},
		},
	}

	Registry.RegisterPlugin("testplugin", mockPlugin)

	// Check codes are registered with prefix
	code, ok := Registry.Get("testplugin:test_error")
	if !ok {
		t.Fatal("Plugin code not registered")
	}
	if code.Message != "Test error" {
		t.Errorf("Message = %q, want %q", code.Message, "Test error")
	}
	if code.HTTPStatus != 400 {
		t.Errorf("HTTPStatus = %d, want %d", code.HTTPStatus, 400)
	}

	// Check namespace
	pluginCodes := Registry.ByNamespace("testplugin")
	if len(pluginCodes) != 2 {
		t.Errorf("ByNamespace(testplugin) returned %d codes, want 2", len(pluginCodes))
	}
}

type mockEnumerator struct {
	codes []ErrorCode
}

func (m *mockEnumerator) EnumerateErrors() []ErrorCode {
	return m.codes
}
