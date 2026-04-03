package tests

import (
	"bytes"
	"encoding/json"
	"net/http"
	"testing"
	"velarix/store"
)

func TestRBACEnforcement(t *testing.T) {
	server := setupTestServer(t)

	// 1. Setup a regular member user
	memberUser := &store.User{
		Email: "member@example.com",
		OrgID: "test_org",
		Role:  "member",
		Keys: []store.APIKey{
			{Key: "member_key", Label: "member_actor", IsRevoked: false, ExpiresAt: 9999999999999},
		},
	}
	server.Store.SaveUser(memberUser)

	// 2. Attempt admin operation (backup) with member key
	resp := performAuthenticatedRequest(t, server, http.MethodGet, "/v1/org/backup", "member_key", nil)
	if resp.Code != http.StatusForbidden {
		t.Fatalf("Expected 403 Forbidden for member accessing backup, got %d", resp.Code)
	}

	// 3. Attempt admin operation (full health) with member key
	resp = performAuthenticatedRequest(t, server, http.MethodGet, "/health/full", "member_key", nil)
	if resp.Code != http.StatusForbidden {
		t.Fatalf("Expected 403 Forbidden for member accessing full health, got %d", resp.Code)
	}
}

func TestBackupRestore(t *testing.T) {
	server := setupTestServer(t)

	// 1. Assert a fact
	fact := map[string]interface{}{"id": "F1", "is_root": true, "manual_status": 1.0}
	body, _ := json.Marshal(fact)
	resp := performRequest(t, server, http.MethodPost, "/v1/s/sess1/facts", body)
	if resp.Code != http.StatusCreated {
		t.Fatalf("Failed to create fact before backup: %d", resp.Code)
	}

	// 2. Perform Backup
	resp = performRequest(t, server, http.MethodGet, "/v1/org/backup", nil)
	if resp.Code != http.StatusOK {
		t.Fatalf("Backup failed: status %d body %s", resp.Code, resp.Body.String())
	}
	
	var backupData bytes.Buffer
	backupData.ReadFrom(resp.Body)

	// 3. Clear Database by creating a new server (simulating disaster)
	server2 := setupTestServer(t)

	// 4. Perform Restore
	resp = performRequest(t, server2, http.MethodPost, "/v1/org/restore", backupData.Bytes())
	if resp.Code != http.StatusOK {
		t.Fatalf("Restore failed: status %d body %s", resp.Code, resp.Body.String())
	}

	// 5. Verify data exists after restore
	resp = performRequest(t, server2, http.MethodGet, "/v1/s/sess1/facts/F1", nil)
	if resp.Code != http.StatusOK {
		t.Fatalf("Failed to retrieve fact after restore: status %d body %s", resp.Code, resp.Body.String())
	}
	
	var restoredFact struct { ID string }
	json.NewDecoder(resp.Body).Decode(&restoredFact)
	if restoredFact.ID != "F1" {
		t.Fatalf("Expected fact F1 after restore, got %s", restoredFact.ID)
	}
}
