package tests

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"

	"velarix/api"
	"velarix/store"
)

// seedTenant creates an isolated user+org and seeds an API key for that tenant.
func seedTenant(t *testing.T, server *api.Server, email, orgID, apiKey string) {
	t.Helper()
	user := &store.User{
		Email: email,
		OrgID: orgID,
		Role:  "admin",
		Keys:  []store.APIKey{{Key: apiKey, Label: "test", IsRevoked: false, ExpiresAt: 9999999999999, Scopes: []string{"read", "write", "export", "admin"}}},
	}
	if err := server.Store.SaveUser(user); err != nil {
		t.Fatalf("seedTenant SaveUser: %v", err)
	}
	org := &store.Organization{ID: orgID, Name: orgID}
	if err := server.Store.SaveOrganization(org); err != nil {
		t.Fatalf("seedTenant SaveOrganization: %v", err)
	}
}

// crossTenantAssertFact asserts a fact into the given session using the given API key.
func crossTenantAssertFact(t *testing.T, server *api.Server, apiKey, sessionID, factID string) {
	t.Helper()
	body, _ := json.Marshal(map[string]interface{}{"id": factID, "is_root": true, "manual_status": 1.0})
	resp := performAuthenticatedRequest(t, server, http.MethodPost, fmt.Sprintf("/v1/s/%s/facts", sessionID), apiKey, body)
	if resp.Code != http.StatusCreated {
		t.Fatalf("crossTenantAssertFact: unexpected status %d (body: %s)", resp.Code, resp.Body.String())
	}
}

// TestCrossTenantSessionIsolation verifies tenant A cannot read tenant B's sessions.
func TestCrossTenantSessionIsolation(t *testing.T) {
	server, _ := setupTestServer(t)

	seedTenant(t, server, "alice@a.com", "org_a", "key_a")
	seedTenant(t, server, "bob@b.com", "org_b", "key_b")

	crossTenantAssertFact(t, server, "key_a", "sess_a1", "fact_a1")

	// Tenant B tries to read tenant A's session graph — must be forbidden.
	resp := performAuthenticatedRequest(t, server, http.MethodGet, "/v1/s/sess_a1/graph", "key_b", nil)
	if resp.Code != http.StatusForbidden {
		t.Errorf("Expected 403 for cross-tenant graph access, got %d", resp.Code)
	}
}

// TestCrossTenantFactReadIsolation verifies tenant B cannot read tenant A's facts.
func TestCrossTenantFactReadIsolation(t *testing.T) {
	server, _ := setupTestServer(t)

	seedTenant(t, server, "alice@a.com", "org_a", "key_a")
	seedTenant(t, server, "bob@b.com", "org_b", "key_b")

	crossTenantAssertFact(t, server, "key_a", "sess_a2", "secret_fact")

	resp := performAuthenticatedRequest(t, server, http.MethodGet, "/v1/s/sess_a2/facts/secret_fact", "key_b", nil)
	if resp.Code != http.StatusForbidden {
		t.Errorf("Expected 403 for cross-tenant fact read, got %d (body: %s)", resp.Code, resp.Body.String())
	}
}

// TestCrossTenantHistoryIsolation verifies tenant B cannot read tenant A's session history.
func TestCrossTenantHistoryIsolation(t *testing.T) {
	server, _ := setupTestServer(t)

	seedTenant(t, server, "alice@a.com", "org_a", "key_a")
	seedTenant(t, server, "bob@b.com", "org_b", "key_b")

	crossTenantAssertFact(t, server, "key_a", "sess_a3", "fact_hist")

	resp := performAuthenticatedRequest(t, server, http.MethodGet, "/v1/s/sess_a3/history", "key_b", nil)
	if resp.Code != http.StatusForbidden {
		t.Errorf("Expected 403 for cross-tenant history access, got %d", resp.Code)
	}
}

// TestCrossTenantFactInvalidationIsolation verifies tenant B cannot invalidate tenant A's facts.
func TestCrossTenantFactInvalidationIsolation(t *testing.T) {
	server, _ := setupTestServer(t)

	seedTenant(t, server, "alice@a.com", "org_a", "key_a")
	seedTenant(t, server, "bob@b.com", "org_b", "key_b")

	crossTenantAssertFact(t, server, "key_a", "sess_a4", "root_fact")

	resp := performAuthenticatedRequest(t, server, http.MethodPost, "/v1/s/sess_a4/facts/root_fact/invalidate", "key_b", nil)
	if resp.Code != http.StatusForbidden {
		t.Errorf("Expected 403 for cross-tenant invalidation, got %d", resp.Code)
	}
}

// TestCrossTenantDecisionIsolation verifies tenant B cannot read or modify tenant A's decisions.
func TestCrossTenantDecisionIsolation(t *testing.T) {
	server, _ := setupTestServer(t)

	seedTenant(t, server, "alice@a.com", "org_a", "key_a")
	seedTenant(t, server, "bob@b.com", "org_b", "key_b")

	crossTenantAssertFact(t, server, "key_a", "sess_a5", "dep_fact")

	// Create a decision in tenant A's session.
	decBody, _ := json.Marshal(map[string]interface{}{
		"id":             "dec_a1",
		"title":          "Approve payment",
		"decision_type":  "approval",
		"conclusion":     "payment approved",
		"dependency_ids": []string{"dep_fact"},
	})
	resp := performAuthenticatedRequest(t, server, http.MethodPost, "/v1/s/sess_a5/decisions", "key_a", decBody)
	if resp.Code != http.StatusCreated {
		t.Fatalf("Tenant A decision creation failed: %d %s", resp.Code, resp.Body.String())
	}

	// Tenant B tries to read the decision.
	resp2 := performAuthenticatedRequest(t, server, http.MethodGet, "/v1/s/sess_a5/decisions/dec_a1", "key_b", nil)
	if resp2.Code != http.StatusForbidden {
		t.Errorf("Expected 403 for cross-tenant decision read, got %d", resp2.Code)
	}
}

// TestCrossTenantOrgDataIsolation verifies org session lists are scoped to the requesting tenant.
func TestCrossTenantOrgDataIsolation(t *testing.T) {
	server, _ := setupTestServer(t)

	seedTenant(t, server, "alice@a.com", "org_a", "key_a")
	seedTenant(t, server, "bob@b.com", "org_b", "key_b")

	// Tenant B asserts a fact creating a session visible only to tenant B.
	crossTenantAssertFact(t, server, "key_b", "sess_b1", "b_secret")

	// Tenant A lists their sessions — must succeed.
	resp := performAuthenticatedRequest(t, server, http.MethodGet, "/v1/org/sessions", "key_a", nil)
	if resp.Code != http.StatusOK {
		t.Fatalf("Tenant A session list failed: %d", resp.Code)
	}

	// Tenant A's session list must NOT contain tenant B's session.
	var sessResp map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&sessResp)
	items, _ := sessResp["items"].([]interface{})
	for _, item := range items {
		m, _ := item.(map[string]interface{})
		if id, _ := m["id"].(string); id == "sess_b1" {
			t.Errorf("Tenant A's session list contains tenant B's session sess_b1 — isolation breach!")
		}
	}
}

// TestCrossTenantInvitationIsolation verifies tenant B cannot revoke tenant A's invitations.
func TestCrossTenantInvitationIsolation(t *testing.T) {
	server, _ := setupTestServer(t)

	seedTenant(t, server, "alice@a.com", "org_a", "key_a")
	seedTenant(t, server, "bob@b.com", "org_b", "key_b")

	// Tenant A creates an invitation.
	invBody, _ := json.Marshal(map[string]string{"email": "guest@a.com", "role": "member"})
	resp := performAuthenticatedRequest(t, server, http.MethodPost, "/v1/org/invitations", "key_a", invBody)
	if resp.Code != http.StatusCreated {
		t.Fatalf("Tenant A invitation creation failed: %d %s", resp.Code, resp.Body.String())
	}
	var invResp map[string]interface{}
	json.NewDecoder(bytes.NewReader(resp.Body.Bytes())).Decode(&invResp)
	inv, _ := invResp["invitation"].(map[string]interface{})
	invID, _ := inv["id"].(string)

	// Tenant B tries to revoke tenant A's invitation — must be rejected.
	resp2 := performAuthenticatedRequest(t, server, http.MethodPost, fmt.Sprintf("/v1/org/invitations/%s/revoke", invID), "key_b", nil)
	if resp2.Code == http.StatusOK {
		t.Errorf("Tenant B was able to revoke tenant A's invitation (id=%s) — isolation breach!", invID)
	}
}

// TestCrossTenantSliceIsolation verifies tenant B cannot retrieve a belief slice from tenant A's session.
func TestCrossTenantSliceIsolation(t *testing.T) {
	server, _ := setupTestServer(t)

	seedTenant(t, server, "alice@a.com", "org_a", "key_a")
	seedTenant(t, server, "bob@b.com", "org_b", "key_b")

	crossTenantAssertFact(t, server, "key_a", "sess_a6", "slice_fact")

	resp := performAuthenticatedRequest(t, server, http.MethodGet, "/v1/s/sess_a6/slice?query=test", "key_b", nil)
	if resp.Code != http.StatusForbidden {
		t.Errorf("Expected 403 for cross-tenant slice access, got %d", resp.Code)
	}
}

// TestCrossTenantAuditLogIsolation verifies org activity logs are scoped to the requesting tenant.
func TestCrossTenantAuditLogIsolation(t *testing.T) {
	server, _ := setupTestServer(t)

	seedTenant(t, server, "alice@a.com", "org_a", "key_a")
	seedTenant(t, server, "bob@b.com", "org_b", "key_b")

	// Generate activity in tenant A.
	crossTenantAssertFact(t, server, "key_a", "sess_a7", "audit_fact")

	// Tenant B reads their own activity — should return 200 but contain no tenant A events.
	resp := performAuthenticatedRequest(t, server, http.MethodGet, "/v1/org/activity", "key_b", nil)
	if resp.Code != http.StatusOK {
		t.Fatalf("Tenant B org activity request failed: %d", resp.Code)
	}

	var actResp map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&actResp)
	items, _ := actResp["items"].([]interface{})
	for _, item := range items {
		m, _ := item.(map[string]interface{})
		if sid, _ := m["session_id"].(string); sid == "sess_a7" {
			t.Errorf("Tenant B's activity log contains tenant A's session event — isolation breach!")
		}
	}
}

// TestCrossTenantSessionDeleteIsolation verifies tenant B cannot archive tenant A's sessions.
func TestCrossTenantSessionDeleteIsolation(t *testing.T) {
	server, _ := setupTestServer(t)

	seedTenant(t, server, "alice@a.com", "org_a", "key_a")
	seedTenant(t, server, "bob@b.com", "org_b", "key_b")

	crossTenantAssertFact(t, server, "key_a", "sess_a8", "delete_fact")

	resp := performAuthenticatedRequest(t, server, http.MethodDelete, "/v1/org/sessions/sess_a8", "key_b", nil)
	if resp.Code == http.StatusOK {
		t.Errorf("Tenant B was able to archive tenant A's session — isolation breach!")
	}
}

// TestCrossTenantExplainIsolation verifies tenant B cannot get explanations from tenant A's session.
func TestCrossTenantExplainIsolation(t *testing.T) {
	server, _ := setupTestServer(t)

	seedTenant(t, server, "alice@a.com", "org_a", "key_a")
	seedTenant(t, server, "bob@b.com", "org_b", "key_b")

	crossTenantAssertFact(t, server, "key_a", "sess_a9", "explain_fact")

	resp := performAuthenticatedRequest(t, server, http.MethodGet, "/v1/s/sess_a9/explain?fact_id=explain_fact", "key_b", nil)
	if resp.Code != http.StatusForbidden {
		t.Errorf("Expected 403 for cross-tenant explain access, got %d", resp.Code)
	}
}
