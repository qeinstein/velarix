package tests

import (
	"bytes"
	"encoding/json"
	"io"
	"log/slog"
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

func setupTestServer(t *testing.T) (*api.Server, *store.BadgerStore) {
	dbPath := t.TempDir()

	os.Setenv("VELARIX_API_KEY", "test_admin_key")
	slog.SetDefault(slog.New(slog.NewJSONHandler(io.Discard, nil)))

	badgerStore, err := store.OpenBadger(dbPath, nil)
	if err != nil {
		t.Fatalf("Failed to open test BadgerDB: %v", err)
	}

	server := newTestServerWithStore(badgerStore)
	seedDefaultTestIdentity(server)

	return server, badgerStore
}

func newTestServerWithStore(runtimeStore store.ServerStore) *api.Server {
	server := &api.Server{
		Engines:    make(map[string]*core.Engine),
		Configs:    make(map[string]*store.SessionConfig),
		Versions:   make(map[string]int64),
		LastAccess: make(map[string]time.Time),
		SliceCache: make(map[string]*api.SliceCacheEntry),
		Store:      runtimeStore,
		StartTime:  time.Now(),
	}
	return server
}

func seedDefaultTestIdentity(server *api.Server) {
	user := &store.User{
		Email: "test@example.com",
		OrgID: "test_org",
		Keys: []store.APIKey{
			{Key: "test_key", Label: "test_actor", IsRevoked: false, ExpiresAt: 9999999999999},
		},
	}
	server.Store.SaveUser(user)
	server.Store.SaveOrganization(&store.Organization{ID: "test_org", Name: "Test Org"})
}

func performAuthenticatedRequest(t *testing.T, server *api.Server, method string, path string, authToken string, body []byte) *httptest.ResponseRecorder {
	return performAuthenticatedRequestWithHeaders(t, server, method, path, authToken, body, nil)
}

func performAuthenticatedRequestWithHeaders(t *testing.T, server *api.Server, method string, path string, authToken string, body []byte, headers map[string]string) *httptest.ResponseRecorder {
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
	for key, value := range headers {
		req.Header.Set(key, value)
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
	server, badgerStore := setupTestServer(t)

	// 1. Write a valid entry
	server.Store.Append(store.JournalEntry{Type: store.EventAssert, SessionID: "sess_1", Fact: &core.Fact{ID: "F1", IsRoot: true}})

	// 2. Write a corrupt entry
	err := badgerStore.DB().Update(func(txn *badger.Txn) error {
		return txn.Set([]byte("s:sess_1:h:9999999999"), []byte(`{"type":"assert", BAD JSON THIS IS CORRUPT`))
	})
	if err != nil {
		t.Fatal(err)
	}

	// 3. Write another valid entry
	server.Store.Append(store.JournalEntry{Type: store.EventAssert, SessionID: "sess_1", Fact: &core.Fact{ID: "F2", IsRoot: true}})

	engines := make(map[string]*core.Engine)
	configsRaw := make(map[string][]byte)
	err = badgerStore.ReplayAll(engines, configsRaw)

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
	server, _ := setupTestServer(t)
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
		ID:                "access_granted",
		JustificationSets: [][]string{{"patient_consented"}},
		Payload:           map[string]interface{}{"access_level": "full"},
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

func TestDecisionLifecycleContract(t *testing.T) {
	server, _ := setupTestServer(t)
	sessionID := "decision_contract_session"

	rootFact := map[string]interface{}{
		"id":            "approval_context_ready",
		"is_root":       true,
		"manual_status": 1.0,
		"payload":       map[string]interface{}{"summary": "context ready"},
	}
	body, _ := json.Marshal(rootFact)
	resp := performRequest(t, server, http.MethodPost, "/v1/s/"+sessionID+"/facts", body)
	if resp.Code != http.StatusCreated {
		t.Fatalf("failed to create root fact: status=%d body=%s", resp.Code, resp.Body.String())
	}

	derivedFact := map[string]interface{}{
		"id":                 "decision.payment_ready",
		"justification_sets": [][]string{{"approval_context_ready"}},
		"payload":            map[string]interface{}{"summary": "payment can be released"},
	}
	body, _ = json.Marshal(derivedFact)
	resp = performRequest(t, server, http.MethodPost, "/v1/s/"+sessionID+"/facts", body)
	if resp.Code != http.StatusCreated {
		t.Fatalf("failed to create derived fact: status=%d body=%s", resp.Code, resp.Body.String())
	}

	createDecision := map[string]interface{}{
		"decision_type":       "approval_release",
		"fact_id":             "decision.payment_ready",
		"subject_ref":         "invoice-42",
		"target_ref":          "vendor-7",
		"recommended_action":  "release_payment",
		"dependency_fact_ids": []string{"approval_context_ready"},
	}
	body, _ = json.Marshal(createDecision)
	resp = performRequest(t, server, http.MethodPost, "/v1/s/"+sessionID+"/decisions", body)
	if resp.Code != http.StatusCreated {
		t.Fatalf("failed to create decision: status=%d body=%s", resp.Code, resp.Body.String())
	}
	var decision store.Decision
	if err := json.NewDecoder(resp.Body).Decode(&decision); err != nil {
		t.Fatalf("failed to decode decision: %v", err)
	}

	resp = performRequest(t, server, http.MethodGet, "/v1/s/"+sessionID+"/decisions/"+decision.ID, nil)
	if resp.Code != http.StatusOK {
		t.Fatalf("failed to fetch decision: status=%d body=%s", resp.Code, resp.Body.String())
	}

	resp = performRequest(t, server, http.MethodPost, "/v1/s/"+sessionID+"/decisions/"+decision.ID+"/execute-check", nil)
	if resp.Code != http.StatusOK {
		t.Fatalf("failed to check decision: status=%d body=%s", resp.Code, resp.Body.String())
	}
	var check store.DecisionCheck
	if err := json.NewDecoder(resp.Body).Decode(&check); err != nil {
		t.Fatalf("failed to decode decision check: %v", err)
	}
	if !check.Executable {
		t.Fatalf("expected executable decision, got blocked: %+v", check)
	}

	resp = performRequest(t, server, http.MethodPost, "/v1/s/"+sessionID+"/facts/approval_context_ready/invalidate", nil)
	if resp.Code != http.StatusOK {
		t.Fatalf("failed to invalidate root fact: status=%d body=%s", resp.Code, resp.Body.String())
	}

	resp = performRequest(t, server, http.MethodPost, "/v1/s/"+sessionID+"/decisions/"+decision.ID+"/execute-check", nil)
	if resp.Code != http.StatusOK {
		t.Fatalf("failed to re-check blocked decision: status=%d body=%s", resp.Code, resp.Body.String())
	}
	if err := json.NewDecoder(resp.Body).Decode(&check); err != nil {
		t.Fatalf("failed to decode blocked decision check: %v", err)
	}
	if check.Executable {
		t.Fatalf("expected blocked decision after invalidation, got executable")
	}

	resp = performRequest(t, server, http.MethodPost, "/v1/s/"+sessionID+"/decisions/"+decision.ID+"/execute", []byte(`{}`))
	if resp.Code != http.StatusConflict {
		t.Fatalf("expected blocked execution conflict, got status=%d body=%s", resp.Code, resp.Body.String())
	}
}

func TestMultiInstanceDecisionRefresh(t *testing.T) {
	serverOne, sharedStore := setupTestServer(t)
	serverTwo := newTestServerWithStore(sharedStore)
	seedDefaultTestIdentity(serverTwo)

	sessionID := "multi_instance_session"

	rootFact := map[string]interface{}{
		"id":            "shared_root",
		"is_root":       true,
		"manual_status": 1.0,
	}
	body, _ := json.Marshal(rootFact)
	resp := performRequest(t, serverOne, http.MethodPost, "/v1/s/"+sessionID+"/facts", body)
	if resp.Code != http.StatusCreated {
		t.Fatalf("failed to create root fact: status=%d body=%s", resp.Code, resp.Body.String())
	}

	derivedFact := map[string]interface{}{
		"id":                 "shared_decision_fact",
		"justification_sets": [][]string{{"shared_root"}},
	}
	body, _ = json.Marshal(derivedFact)
	resp = performRequest(t, serverOne, http.MethodPost, "/v1/s/"+sessionID+"/facts", body)
	if resp.Code != http.StatusCreated {
		t.Fatalf("failed to create derived fact: status=%d body=%s", resp.Code, resp.Body.String())
	}

	createDecision := map[string]interface{}{
		"decision_type":       "multi_instance_check",
		"fact_id":             "shared_decision_fact",
		"subject_ref":         "invoice-shared",
		"target_ref":          "vendor-shared",
		"dependency_fact_ids": []string{"shared_root"},
	}
	body, _ = json.Marshal(createDecision)
	resp = performRequest(t, serverOne, http.MethodPost, "/v1/s/"+sessionID+"/decisions", body)
	if resp.Code != http.StatusCreated {
		t.Fatalf("failed to create decision: status=%d body=%s", resp.Code, resp.Body.String())
	}
	var decision store.Decision
	if err := json.NewDecoder(resp.Body).Decode(&decision); err != nil {
		t.Fatalf("failed to decode decision: %v", err)
	}

	resp = performRequest(t, serverTwo, http.MethodPost, "/v1/s/"+sessionID+"/decisions/"+decision.ID+"/execute-check", nil)
	if resp.Code != http.StatusOK {
		t.Fatalf("failed to warm second server decision cache: status=%d body=%s", resp.Code, resp.Body.String())
	}

	resp = performRequest(t, serverOne, http.MethodPost, "/v1/s/"+sessionID+"/facts/shared_root/invalidate", nil)
	if resp.Code != http.StatusOK {
		t.Fatalf("failed to invalidate shared root: status=%d body=%s", resp.Code, resp.Body.String())
	}

	resp = performRequest(t, serverTwo, http.MethodPost, "/v1/s/"+sessionID+"/decisions/"+decision.ID+"/execute-check", nil)
	if resp.Code != http.StatusOK {
		t.Fatalf("failed to refresh second server decision view: status=%d body=%s", resp.Code, resp.Body.String())
	}
	var check store.DecisionCheck
	if err := json.NewDecoder(resp.Body).Decode(&check); err != nil {
		t.Fatalf("failed to decode refreshed decision check: %v", err)
	}
	if check.Executable {
		t.Fatalf("expected second server to observe blocked decision after version refresh")
	}
}

func TestDecisionExecutionRequiresFreshToken(t *testing.T) {
	server, _ := setupTestServer(t)
	sessionID := "decision_execute_token_session"

	rootFact := map[string]interface{}{
		"id":            "token_root",
		"is_root":       true,
		"manual_status": 1.0,
	}
	body, _ := json.Marshal(rootFact)
	resp := performRequest(t, server, http.MethodPost, "/v1/s/"+sessionID+"/facts", body)
	if resp.Code != http.StatusCreated {
		t.Fatalf("failed to create root fact: status=%d body=%s", resp.Code, resp.Body.String())
	}

	derivedFact := map[string]interface{}{
		"id":                 "token_ready_fact",
		"justification_sets": [][]string{{"token_root"}},
	}
	body, _ = json.Marshal(derivedFact)
	resp = performRequest(t, server, http.MethodPost, "/v1/s/"+sessionID+"/facts", body)
	if resp.Code != http.StatusCreated {
		t.Fatalf("failed to create derived fact: status=%d body=%s", resp.Code, resp.Body.String())
	}

	createDecision := map[string]interface{}{
		"decision_type":       "execute_with_token",
		"fact_id":             "token_ready_fact",
		"subject_ref":         "invoice-token",
		"target_ref":          "vendor-token",
		"dependency_fact_ids": []string{"token_root"},
	}
	body, _ = json.Marshal(createDecision)
	resp = performRequest(t, server, http.MethodPost, "/v1/s/"+sessionID+"/decisions", body)
	if resp.Code != http.StatusCreated {
		t.Fatalf("failed to create decision: status=%d body=%s", resp.Code, resp.Body.String())
	}
	var decision store.Decision
	if err := json.NewDecoder(resp.Body).Decode(&decision); err != nil {
		t.Fatalf("failed to decode decision: %v", err)
	}

	resp = performRequest(t, server, http.MethodPost, "/v1/s/"+sessionID+"/decisions/"+decision.ID+"/execute-check", nil)
	if resp.Code != http.StatusOK {
		t.Fatalf("failed to check decision: status=%d body=%s", resp.Code, resp.Body.String())
	}
	var check store.DecisionCheck
	if err := json.NewDecoder(resp.Body).Decode(&check); err != nil {
		t.Fatalf("failed to decode decision check: %v", err)
	}
	if !check.Executable || check.ExecutionToken == "" {
		t.Fatalf("expected executable decision with execution token, got %+v", check)
	}

	body, _ = json.Marshal(map[string]interface{}{"execution_token": check.ExecutionToken})
	resp = performRequest(t, server, http.MethodPost, "/v1/s/"+sessionID+"/decisions/"+decision.ID+"/execute", body)
	if resp.Code != http.StatusOK {
		t.Fatalf("expected successful execution, got status=%d body=%s", resp.Code, resp.Body.String())
	}

	resp = performRequest(t, server, http.MethodPost, "/v1/s/"+sessionID+"/decisions/"+decision.ID+"/execute", body)
	if resp.Code != http.StatusConflict {
		t.Fatalf("expected second execution to fail, got status=%d body=%s", resp.Code, resp.Body.String())
	}
}
