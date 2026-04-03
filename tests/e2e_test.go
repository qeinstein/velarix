package tests

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"velarix/api"
	"velarix/core"
	"velarix/store"

	"github.com/dgraph-io/badger/v4"
)

func setupTestServer(t *testing.T) *api.Server {
	dbPath := t.TempDir()

	os.Setenv("VELARIX_API_KEY", "test_admin_key")

	badgerStore, err := store.OpenBadger(dbPath, nil)
	if err != nil {
		t.Fatalf("Failed to open test BadgerDB: %v", err)
	}

	server := &api.Server{
		Engines:    make(map[string]*core.Engine),
		Configs:    make(map[string]*store.SessionConfig),
		LastAccess: make(map[string]time.Time),
		Store:      badgerStore,
		StartTime:  time.Now(),
	}

	// Setup a default user and org for testing
	user := &store.User{
		Email: "test@example.com",
		OrgID: "test_org",
		Keys: []store.APIKey{
			{Key: "test_key", Label: "test_actor", IsRevoked: false},
		},
	}
	server.Store.SaveUser(user)
	server.Store.SaveOrganization(&store.Organization{ID: "test_org", Name: "Test Org"})

	return server
}

func performAuthenticatedRequest(t *testing.T, server *api.Server, method string, path string, authToken string, body []byte) *httptest.ResponseRecorder {
	t.Helper()

	var requestBody *bytes.Reader
	if body == nil {
		requestBody = bytes.NewReader(nil)
	} else {
		requestBody = bytes.NewReader(body)
	}

	req := httptest.NewRequest(method, path, requestBody)
	if authToken != "" {
		req.Header.Set("Authorization", "Bearer "+authToken)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	recorder := httptest.NewRecorder()
	server.Routes().ServeHTTP(recorder, req)
	return recorder
}

func performRequest(t *testing.T, server *api.Server, method string, path string, body []byte) *httptest.ResponseRecorder {
	t.Helper()
	return performAuthenticatedRequest(t, server, method, path, "test_admin_key", body)
}

func TestJournalResilience(t *testing.T) {
	server := setupTestServer(t)

	// 1. Write a valid entry
	server.Store.Append(store.JournalEntry{Type: store.EventAssert, SessionID: "sess_1", Fact: &core.Fact{ID: "F1", IsRoot: true}})
	
	// 2. Write a corrupt entry
	err := server.Store.DB().Update(func(txn *badger.Txn) error {
		return txn.Set([]byte("s:sess_1:h:9999999999"), []byte(`{"type":"assert", BAD JSON THIS IS CORRUPT`))
	})
	if err != nil {
		t.Fatal(err)
	}

	// 3. Write another valid entry
	server.Store.Append(store.JournalEntry{Type: store.EventAssert, SessionID: "sess_1", Fact: &core.Fact{ID: "F2", IsRoot: true}})

	engines := make(map[string]*core.Engine)
	configsRaw := make(map[string][]byte)
	err = server.Store.ReplayAll(engines, configsRaw)

	if err != nil {
		t.Fatalf("ReplayAll failed unexpectedly: %v", err)
	}

	engine, ok := engines["sess_1"]
	if !ok {
		t.Fatal("Session not loaded after replay")
	}

	if _, ok := engine.GetFact("F1"); !ok {
		t.Error("Fact F1 (before corruption) was lost")
	}
	if _, ok := engine.GetFact("F2"); !ok {
		t.Error("Fact F2 (after corruption) was lost")
	}
}

func TestE2ELifecycle(t *testing.T) {
	server := setupTestServer(t)
	sessionID := "e2e_session"

	// 1. Assert Root Fact
	rootFact := core.Fact{
		ID:           "patient_consented",
		IsRoot:       true,
		ManualStatus: core.Valid,
		Payload:      map[string]interface{}{"consent_type": "hipaa"},
	}
	body, _ := json.Marshal(rootFact)
	resp := performRequest(t, server, http.MethodPost, "/v1/s/"+sessionID+"/facts", body)
	if resp.Code != http.StatusCreated {
		t.Fatalf("Failed to assert root fact (status: %d body: %s)", resp.Code, resp.Body.String())
	}

	// 2. Assert Derived Fact
	derivedFact := core.Fact{
		ID: "access_granted",
		JustificationSets: [][]string{{"patient_consented"}},
		Payload: map[string]interface{}{"access_level": "full"},
	}
	body, _ = json.Marshal(derivedFact)
	resp = performRequest(t, server, http.MethodPost, "/v1/s/"+sessionID+"/facts", body)
	if resp.Code != http.StatusCreated {
		t.Fatalf("Failed to assert derived fact (status: %d body: %s)", resp.Code, resp.Body.String())
	}

	// 3. Verify Status
	resp = performRequest(t, server, http.MethodGet, "/v1/s/"+sessionID+"/facts/access_granted", nil)

	var statusResp struct {
		ResolvedStatus float64 `json:"resolved_status"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&statusResp); err != nil {
		t.Fatalf("Failed to decode status response: %v", err)
	}
	if statusResp.ResolvedStatus != 1.0 {
		t.Fatalf("Expected access_granted to be valid (1.0), got %f", statusResp.ResolvedStatus)
	}

	// 4. Invalidate Root
	resp = performRequest(t, server, http.MethodPost, "/v1/s/"+sessionID+"/facts/patient_consented/invalidate", nil)
	if resp.Code != http.StatusOK {
		t.Fatalf("Failed to invalidate root (status: %d body: %s)", resp.Code, resp.Body.String())
	}

	// 5. Verify Invalidation Cascaded
	resp = performRequest(t, server, http.MethodGet, "/v1/s/"+sessionID+"/facts/access_granted", nil)
	if err := json.NewDecoder(resp.Body).Decode(&statusResp); err != nil {
		t.Fatalf("Failed to decode invalidated status response: %v", err)
	}
	if statusResp.ResolvedStatus != 0.0 {
		t.Fatalf("Expected access_granted to be invalid (0.0) after root invalidation, got %f", statusResp.ResolvedStatus)
	}
}
