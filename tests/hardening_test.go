package tests

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
	"velarix/store"
)

func TestRBACEnforcement(t *testing.T) {
	server, ts := setupTestServer(t)
	defer ts.Close()

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

	client := &http.Client{}

	// 2. Attempt admin operation (backup) with member key
	req, _ := http.NewRequest("GET", fmt.Sprintf("%s/v1/org/backup", ts.URL), nil)
	req.Header.Set("Authorization", "Bearer member_key")
	resp, err := client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("Expected 403 Forbidden for member accessing backup, got %d", resp.StatusCode)
	}

	// 3. Attempt admin operation (full health) with member key
	req, _ = http.NewRequest("GET", fmt.Sprintf("%s/health/full", ts.URL), nil)
	req.Header.Set("Authorization", "Bearer member_key")
	resp, err = client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("Expected 403 Forbidden for member accessing full health, got %d", resp.StatusCode)
	}
}

func TestBackupRestore(t *testing.T) {
	_, ts := setupTestServer(t)
	defer ts.Close()

	client := &http.Client{}

	// 1. Assert a fact
	fact := map[string]interface{}{"id": "F1", "is_root": true, "manual_status": 1.0}
	body, _ := json.Marshal(fact)
	req, _ := http.NewRequest("POST", fmt.Sprintf("%s/v1/s/sess1/facts", ts.URL), bytes.NewBuffer(body))
	req.Header.Set("Authorization", "Bearer test_admin_key")
	req.Header.Set("Content-Type", "application/json")
	client.Do(req)

	// 2. Perform Backup
	req, _ = http.NewRequest("GET", fmt.Sprintf("%s/v1/org/backup", ts.URL), nil)
	req.Header.Set("Authorization", "Bearer test_admin_key")
	resp, err := client.Do(req)
	if err != nil || resp.StatusCode != http.StatusOK {
		t.Fatalf("Backup failed: %v (Status: %d)", err, resp.StatusCode)
	}
	
	var backupData bytes.Buffer
	backupData.ReadFrom(resp.Body)
	resp.Body.Close()

	// 3. Clear Database by creating a new server (simulating disaster)
	ts.Close()
	_, ts2 := setupTestServer(t)
	defer ts2.Close()

	// 4. Perform Restore
	req, _ = http.NewRequest("POST", fmt.Sprintf("%s/v1/org/restore", ts2.URL), &backupData)
	req.Header.Set("Authorization", "Bearer test_admin_key")
	resp, err = client.Do(req)
	if err != nil || resp.StatusCode != http.StatusOK {
		t.Fatalf("Restore failed: %v (Status: %d)", err, resp.StatusCode)
	}

	// 5. Verify data exists after restore
	req, _ = http.NewRequest("GET", fmt.Sprintf("%s/v1/s/sess1/facts/F1", ts2.URL), nil)
	req.Header.Set("Authorization", "Bearer test_admin_key")
	resp, err = client.Do(req)
	if err != nil || resp.StatusCode != http.StatusOK {
		t.Fatalf("Failed to retrieve fact after restore: %v (Status: %d)", err, resp.StatusCode)
	}
	
	var restoredFact struct { ID string }
	json.NewDecoder(resp.Body).Decode(&restoredFact)
	if restoredFact.ID != "F1" {
		t.Fatalf("Expected fact F1 after restore, got %s", restoredFact.ID)
	}
}
