package api

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestNormalizeEmail(t *testing.T) {
	if normalizeEmail(" Test@Example.com ") != "test@example.com" {
		t.Error("email normalization failed")
	}
}

func TestRequiredScopeForRequest(t *testing.T) {
	req, _ := http.NewRequest("GET", "/v1/org", nil)
	scope := requiredScopeForRequest(req)
	if scope != "read" {
		t.Errorf("expected read, got %s", scope)
	}

	req2, _ := http.NewRequest("POST", "/v1/org", nil)
	scope2 := requiredScopeForRequest(req2)
	if scope2 != "admin" {
		t.Errorf("expected admin, got %s", scope2)
	}

	req3, _ := http.NewRequest("GET", "/v1/org/backup", nil)
	scope3 := requiredScopeForRequest(req3)
	if scope3 != "admin" {
		t.Errorf("expected admin, got %s", scope3)
	}
}

func TestHasScope(t *testing.T) {
	if !hasScope([]string{"read", "write"}, "read") {
		t.Error("expected true")
	}
	if hasScope([]string{"read", "write"}, "admin") {
		t.Error("expected false")
	}
	if !hasScope([]string{"admin"}, "") {
		t.Error("expected true for empty want")
	}
}

func TestScopesForRole(t *testing.T) {
	adminScopes := scopesForRole("admin")
	if len(adminScopes) != 4 || adminScopes[3] != "admin" {
		t.Errorf("unexpected admin scopes: %v", adminScopes)
	}

	auditorScopes := scopesForRole("auditor")
	if len(auditorScopes) != 2 || auditorScopes[1] != "export" {
		t.Errorf("unexpected auditor scopes: %v", auditorScopes)
	}
}

func TestStatusRecorder(t *testing.T) {
	w := httptest.NewRecorder()
	sr := &statusRecorder{ResponseWriter: w, status: 200}
	sr.WriteHeader(404)
	if sr.status != 404 {
		t.Errorf("expected 404, got %d", sr.status)
	}
	sr.Write([]byte("not found"))
	if string(sr.body) != "not found" {
		t.Errorf("expected not found, got %s", string(sr.body))
	}
}
