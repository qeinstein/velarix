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

func TestReasoningAndConsistencyContracts(t *testing.T) {
	server, _ := setupTestServer(t)
	sessionID := "reasoning_contract_session"

	assertFact := func(body map[string]interface{}) {
		payload, _ := json.Marshal(body)
		resp := performRequest(t, server, http.MethodPost, "/v1/s/"+sessionID+"/facts", payload)
		if resp.Code != http.StatusCreated {
			t.Fatalf("failed to create fact: status=%d body=%s", resp.Code, resp.Body.String())
		}
	}

	assertFact(map[string]interface{}{
		"id":            "belief.weather.sunny",
		"is_root":       true,
		"manual_status": 1.0,
		"payload": map[string]interface{}{
			"claim_key":   "weather_outlook",
			"claim_value": "sunny",
			"text":        "the weather is sunny",
		},
	})
	assertFact(map[string]interface{}{
		"id":            "belief.weather.rainy",
		"is_root":       true,
		"manual_status": 1.0,
		"payload": map[string]interface{}{
			"claim_key":   "weather_outlook",
			"claim_value": "rainy",
			"text":        "the weather is rainy",
		},
	})

	resp := performRequest(t, server, http.MethodGet, "/v1/s/"+sessionID+"/semantic-search?q=sunny&limit=1", nil)
	if resp.Code != http.StatusOK {
		t.Fatalf("semantic search failed: status=%d body=%s", resp.Code, resp.Body.String())
	}
	var semanticResults []map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&semanticResults); err != nil {
		t.Fatalf("failed to decode semantic search response: %v", err)
	}
	if len(semanticResults) == 0 || semanticResults[0]["fact_id"] != "belief.weather.sunny" {
		t.Fatalf("unexpected semantic search results: %+v", semanticResults)
	}

	resp = performRequest(t, server, http.MethodPost, "/v1/s/"+sessionID+"/consistency-check", []byte(`{}`))
	if resp.Code != http.StatusOK {
		t.Fatalf("consistency check failed: status=%d body=%s", resp.Code, resp.Body.String())
	}
	var consistencyReport map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&consistencyReport); err != nil {
		t.Fatalf("failed to decode consistency report: %v", err)
	}
	if consistencyReport["issue_count"].(float64) < 1 {
		t.Fatalf("expected at least one consistency issue, got %+v", consistencyReport)
	}

	chainBody, _ := json.Marshal(map[string]interface{}{
		"chain_id": "chain_weather",
		"summary":  "weather chain with later contradiction",
		"steps": []map[string]interface{}{
			{"id": "step_1", "content": "initial belief", "output_fact_id": "belief.weather.sunny"},
			{"id": "step_2", "content": "later contradictory belief", "output_fact_id": "belief.weather.rainy"},
		},
	})
	resp = performRequest(t, server, http.MethodPost, "/v1/s/"+sessionID+"/reasoning-chains", chainBody)
	if resp.Code != http.StatusCreated {
		t.Fatalf("failed to record reasoning chain: status=%d body=%s", resp.Code, resp.Body.String())
	}

	resp = performRequest(t, server, http.MethodPost, "/v1/s/"+sessionID+"/reasoning-chains/chain_weather/verify", []byte(`{"auto_retract": true}`))
	if resp.Code != http.StatusOK {
		t.Fatalf("failed to verify reasoning chain: status=%d body=%s", resp.Code, resp.Body.String())
	}
	var auditReport map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&auditReport); err != nil {
		t.Fatalf("failed to decode reasoning audit: %v", err)
	}
	if auditReport["valid"].(bool) {
		t.Fatalf("expected reasoning audit to fail on contradiction: %+v", auditReport)
	}

	resp = performRequest(t, server, http.MethodGet, "/v1/s/"+sessionID+"/facts/belief.weather.sunny", nil)
	if resp.Code != http.StatusOK {
		t.Fatalf("failed to fetch retracted fact: status=%d body=%s", resp.Code, resp.Body.String())
	}
	var factResp struct {
		ID             string  `json:"id"`
		ResolvedStatus float64 `json:"resolved_status"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&factResp); err != nil {
		t.Fatalf("failed to decode fact response: %v", err)
	}
	if factResp.ResolvedStatus != 0.0 {
		t.Fatalf("expected earlier contradicted fact to be retracted, got %+v", factResp)
	}
}

func TestConsistencyCheckUsesModelVerifierWhenConfigured(t *testing.T) {
	server, _ := setupTestServer(t)
	sessionID := "verifier_contract_session"

	verifyServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/chat/completions" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"choices": []map[string]interface{}{
				{
					"message": map[string]interface{}{
						"content": `{"label":"contradiction","confidence":0.97,"reason":"Both facts cannot be true at the same time."}`,
					},
				},
			},
		})
	}))
	defer verifyServer.Close()

	t.Setenv("OPENAI_API_KEY", "test-key")
	t.Setenv("VELARIX_VERIFIER_MODEL", "gpt-4o-mini")
	t.Setenv("VELARIX_OPENAI_BASE_URL", verifyServer.URL)

	for _, fact := range []map[string]interface{}{
		{
			"id":            "shipment_arrived",
			"is_root":       true,
			"manual_status": 1.0,
			"payload": map[string]interface{}{
				"text": "Alice is available tomorrow for the warehouse shift.",
			},
		},
		{
			"id":            "shipment_in_transit",
			"is_root":       true,
			"manual_status": 1.0,
			"payload": map[string]interface{}{
				"text": "Alice is unavailable tomorrow for the warehouse shift.",
			},
		},
	} {
		body, _ := json.Marshal(fact)
		resp := performRequest(t, server, http.MethodPost, "/v1/s/"+sessionID+"/facts", body)
		if resp.Code != http.StatusCreated {
			t.Fatalf("failed to create verifier fact: status=%d body=%s", resp.Code, resp.Body.String())
		}
	}

	resp := performRequest(t, server, http.MethodPost, "/v1/s/"+sessionID+"/consistency-check", []byte(`{}`))
	if resp.Code != http.StatusOK {
		t.Fatalf("consistency check failed: status=%d body=%s", resp.Code, resp.Body.String())
	}
	var report map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&report); err != nil {
		t.Fatalf("failed to decode verifier report: %v", err)
	}
	if report["issue_count"].(float64) < 1 {
		t.Fatalf("expected verifier-backed issue, got %+v", report)
	}
	issues, _ := report["issues"].([]interface{})
	found := false
	for _, raw := range issues {
		issue, _ := raw.(map[string]interface{})
		if issue["type"] == "model_verifier_contradiction" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected model_verifier_contradiction issue, got %+v", report)
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
