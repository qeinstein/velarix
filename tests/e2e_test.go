package tests

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"velarix/api"
	"velarix/core"
	"velarix/store"

	"github.com/dgraph-io/badger/v4"
)

func setupTestServer(t *testing.T) (*api.Server, *store.BadgerStore) {
	dbPath := t.TempDir()

	t.Setenv("VELARIX_API_KEY", "test_admin_key")
	t.Setenv("VELARIX_ENV", "test")
	t.Setenv("VELARIX_JWT_SECRET", "test_jwt_secret_32_bytes_minimum_value")
	t.Setenv("VELARIX_ENABLE_PUBLIC_REGISTRATION", "true")
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

func TestVerificationGatesDecisionExecution(t *testing.T) {
	server, _ := setupTestServer(t)

	// Enable grounding policy: allow llm_output but require verified.
	policyBody, _ := json.Marshal(map[string]interface{}{
		"name":    "grounding",
		"enabled": true,
		"rules": map[string]interface{}{
			"grounding_require_verified":         true,
			"grounding_allowed_source_types":     []string{"llm_output", "perception", "user"},
			"verification_required_source_types": []string{"llm_output"},
		},
	})
	resp := performRequest(t, server, http.MethodPost, "/v1/policies", policyBody)
	if resp.Code != http.StatusCreated {
		t.Fatalf("failed to create policy: status=%d body=%s", resp.Code, resp.Body.String())
	}

	sessionID := "verification_gate_session"

	// Root fact from an untrusted source (LLM output) starts unverified.
	rootBody, _ := json.Marshal(map[string]interface{}{
		"id":            "ceo_claim",
		"is_root":       true,
		"manual_status": 1.0,
		"payload":       map[string]interface{}{"claim_key": "ceo", "claim_value": "alice"},
		"metadata":      map[string]interface{}{"source_type": "llm_output"},
	})
	resp = performRequest(t, server, http.MethodPost, "/v1/s/"+sessionID+"/facts", rootBody)
	if resp.Code != http.StatusCreated {
		t.Fatalf("failed to assert root: status=%d body=%s", resp.Code, resp.Body.String())
	}

	// Execution-critical derived fact (decision.*) depends on the root.
	decisionFactBody, _ := json.Marshal(map[string]interface{}{
		"id":                 "decision.release_payment",
		"justification_sets": [][]string{{"ceo_claim"}},
		"payload":            map[string]interface{}{"type": "action", "summary": "release payment"},
	})
	resp = performRequest(t, server, http.MethodPost, "/v1/s/"+sessionID+"/facts", decisionFactBody)
	if resp.Code != http.StatusCreated {
		t.Fatalf("failed to assert decision fact: status=%d body=%s", resp.Code, resp.Body.String())
	}

	createDecisionBody, _ := json.Marshal(map[string]interface{}{
		"decision_type": "verification_gate",
		"fact_id":       "decision.release_payment",
		"subject_ref":   "vendor:1",
		"target_ref":    "invoice:1",
	})
	resp = performRequest(t, server, http.MethodPost, "/v1/s/"+sessionID+"/decisions", createDecisionBody)
	if resp.Code != http.StatusCreated {
		t.Fatalf("failed to create decision: status=%d body=%s", resp.Code, resp.Body.String())
	}
	var decision store.Decision
	if err := json.NewDecoder(resp.Body).Decode(&decision); err != nil {
		t.Fatalf("failed to decode decision: %v", err)
	}

	// Execute-check should be blocked because ceo_claim is unverified.
	resp = performRequest(t, server, http.MethodPost, "/v1/s/"+sessionID+"/decisions/"+decision.ID+"/execute-check", nil)
	if resp.Code != http.StatusOK {
		t.Fatalf("execute-check failed: status=%d body=%s", resp.Code, resp.Body.String())
	}
	var check store.DecisionCheck
	if err := json.NewDecoder(resp.Body).Decode(&check); err != nil {
		t.Fatalf("failed to decode check: %v", err)
	}
	if check.Executable {
		t.Fatalf("expected blocked decision before verification, got executable")
	}
	found := false
	for _, rc := range check.ReasonCodes {
		if rc == "unverified_dependency" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected reason code unverified_dependency, got %+v", check.ReasonCodes)
	}

	// Verify the root fact, then re-check.
	verifyBody, _ := json.Marshal(map[string]interface{}{
		"status":     "verified",
		"method":     "test",
		"source_ref": "unit",
	})
	resp = performRequest(t, server, http.MethodPost, "/v1/s/"+sessionID+"/facts/ceo_claim/verify", verifyBody)
	if resp.Code != http.StatusOK {
		t.Fatalf("verify failed: status=%d body=%s", resp.Code, resp.Body.String())
	}

	resp = performRequest(t, server, http.MethodPost, "/v1/s/"+sessionID+"/decisions/"+decision.ID+"/execute-check", nil)
	if resp.Code != http.StatusOK {
		t.Fatalf("execute-check after verify failed: status=%d body=%s", resp.Code, resp.Body.String())
	}
	if err := json.NewDecoder(resp.Body).Decode(&check); err != nil {
		t.Fatalf("failed to decode check after verify: %v", err)
	}
	if !check.Executable {
		t.Fatalf("expected executable decision after verification, got blocked: %+v", check.ReasonCodes)
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

func TestDecisionBlockedUntilHumanReviewApproved(t *testing.T) {
	server, _ := setupTestServer(t)
	sessionID := "review_required_session"

	err := server.Store.SavePolicy("admin", &store.Policy{
		ID:      "review_gate",
		Name:    "Review Gate",
		Enabled: true,
		Rules: map[string]interface{}{
			"review_source_types":          []string{"external_tool"},
			"protected_mutation_threshold": 0.8,
		},
	})
	if err != nil {
		t.Fatalf("failed to seed policy: %v", err)
	}

	factBody, _ := json.Marshal(map[string]interface{}{
		"id":            "tool_claim_ready",
		"is_root":       true,
		"manual_status": 1.0,
		"entrenchment":  0.95,
		"metadata": map[string]interface{}{
			"source_type": "external_tool",
		},
		"payload": map[string]interface{}{"summary": "external tool confirmed readiness"},
	})
	resp := performRequest(t, server, http.MethodPost, "/v1/s/"+sessionID+"/facts", factBody)
	if resp.Code != http.StatusCreated {
		t.Fatalf("failed to create governed fact: status=%d body=%s", resp.Code, resp.Body.String())
	}

	createDecisionBody, _ := json.Marshal(map[string]interface{}{
		"decision_type":       "approval_release",
		"fact_id":             "tool_claim_ready",
		"subject_ref":         "invoice-99",
		"target_ref":          "vendor-99",
		"dependency_fact_ids": []string{"tool_claim_ready"},
	})
	resp = performRequest(t, server, http.MethodPost, "/v1/s/"+sessionID+"/decisions", createDecisionBody)
	if resp.Code != http.StatusCreated {
		t.Fatalf("failed to create decision: status=%d body=%s", resp.Code, resp.Body.String())
	}
	var decision store.Decision
	if err := json.NewDecoder(resp.Body).Decode(&decision); err != nil {
		t.Fatalf("failed to decode decision: %v", err)
	}

	resp = performRequest(t, server, http.MethodPost, "/v1/s/"+sessionID+"/decisions/"+decision.ID+"/execute-check", nil)
	if resp.Code != http.StatusOK {
		t.Fatalf("expected execute-check response before review: status=%d body=%s", resp.Code, resp.Body.String())
	}
	var blockedCheck store.DecisionCheck
	if err := json.NewDecoder(resp.Body).Decode(&blockedCheck); err != nil {
		t.Fatalf("failed to decode blocked decision check: %v", err)
	}
	if blockedCheck.Executable {
		t.Fatalf("expected decision to remain blocked before review, got %+v", blockedCheck)
	}

	reviewBody, _ := json.Marshal(map[string]interface{}{"status": "approved", "reason": "human verified benchmark fact"})
	resp = performRequest(t, server, http.MethodPost, "/v1/s/"+sessionID+"/facts/tool_claim_ready/review", reviewBody)
	if resp.Code != http.StatusOK {
		t.Fatalf("failed to approve fact review: status=%d body=%s", resp.Code, resp.Body.String())
	}

	resp = performRequest(t, server, http.MethodPost, "/v1/s/"+sessionID+"/decisions/"+decision.ID+"/execute-check", nil)
	if resp.Code != http.StatusOK {
		t.Fatalf("expected execute-check to pass after review: status=%d body=%s", resp.Code, resp.Body.String())
	}
}

func TestQueryAwareSliceReturnsRelevantFacts(t *testing.T) {
	server, _ := setupTestServer(t)
	sessionID := "slice_query_session"

	facts := []map[string]interface{}{
		{
			"id":            "invoice_risk",
			"is_root":       true,
			"manual_status": 1.0,
			"payload":       map[string]interface{}{"text": "Invoice 42 has a payment risk escalation and needs review"},
		},
		{
			"id":            "vendor_profile",
			"is_root":       true,
			"manual_status": 1.0,
			"payload":       map[string]interface{}{"text": "Vendor profile is verified and stable"},
		},
		{
			"id":            "deployment_status",
			"is_root":       true,
			"manual_status": 1.0,
			"payload":       map[string]interface{}{"text": "Deployment window is green"},
		},
	}
	for _, fact := range facts {
		body, _ := json.Marshal(fact)
		resp := performRequest(t, server, http.MethodPost, "/v1/s/"+sessionID+"/facts", body)
		if resp.Code != http.StatusCreated {
			t.Fatalf("failed to seed slice fact: status=%d body=%s", resp.Code, resp.Body.String())
		}
	}

	resp := performRequest(t, server, http.MethodGet, "/v1/s/"+sessionID+"/slice?format=json&query=invoice+risk+review&strategy=hybrid&max_facts=2&include_dependencies=true", nil)
	if resp.Code != http.StatusOK {
		t.Fatalf("failed to fetch query-aware slice: status=%d body=%s", resp.Code, resp.Body.String())
	}

	var slice []*core.Fact
	if err := json.NewDecoder(resp.Body).Decode(&slice); err != nil {
		t.Fatalf("failed to decode slice: %v", err)
	}
	if len(slice) == 0 || len(slice) > 2 {
		t.Fatalf("expected 1-2 facts in slice, got %d", len(slice))
	}
	found := false
	for _, fact := range slice {
		if fact.ID == "invoice_risk" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected invoice_risk to be selected in query-aware slice, got %+v", slice)
	}
}

func TestLongHorizonContradictionMission(t *testing.T) {
	server, _ := setupTestServer(t)
	sessionID := "long_horizon_session"
	topics := []string{"research.evidence_ranker", "coding.build_api", "tool_use.deploy_gate"}
	currentRootIDs := map[string]string{}
	currentPlanIDs := map[string]string{}

	for _, topic := range topics {
		body, _ := json.Marshal(map[string]interface{}{
			"id":            topic + ".ready",
			"is_root":       true,
			"manual_status": 1.0,
			"payload":       map[string]interface{}{"topic": topic, "kind": "prerequisite"},
		})
		resp := performRequest(t, server, http.MethodPost, "/v1/s/"+sessionID+"/facts", body)
		if resp.Code != http.StatusCreated {
			t.Fatalf("failed to seed prerequisite: status=%d body=%s", resp.Code, resp.Body.String())
		}
	}

	for step := 0; step < 90; step++ {
		topic := topics[step%len(topics)]
		version := (step / len(topics)) + 1
		rootID := fmt.Sprintf("%s.obs.v%d", topic, version)
		planID := fmt.Sprintf("%s.plan.v%d", topic, version)

		if previousRoot := currentRootIDs[topic]; previousRoot != "" {
			resp := performRequest(t, server, http.MethodPost, "/v1/s/"+sessionID+"/facts/"+previousRoot+"/invalidate", nil)
			if resp.Code != http.StatusOK {
				t.Fatalf("failed to invalidate prior root: status=%d body=%s", resp.Code, resp.Body.String())
			}
		}

		rootBody, _ := json.Marshal(map[string]interface{}{
			"id":            rootID,
			"is_root":       true,
			"manual_status": 1.0,
			"payload":       map[string]interface{}{"topic": topic, "version": version},
		})
		resp := performRequest(t, server, http.MethodPost, "/v1/s/"+sessionID+"/facts", rootBody)
		if resp.Code != http.StatusCreated {
			t.Fatalf("failed to assert horizon root: status=%d body=%s", resp.Code, resp.Body.String())
		}

		planBody, _ := json.Marshal(map[string]interface{}{
			"id":                 planID,
			"justification_sets": [][]string{{topic + ".ready", rootID}},
			"payload":            map[string]interface{}{"topic": topic, "version": version, "kind": "plan"},
		})
		resp = performRequest(t, server, http.MethodPost, "/v1/s/"+sessionID+"/facts", planBody)
		if resp.Code != http.StatusCreated {
			t.Fatalf("failed to assert horizon plan: status=%d body=%s", resp.Code, resp.Body.String())
		}

		if previousPlan := currentPlanIDs[topic]; previousPlan != "" {
			resp = performRequest(t, server, http.MethodGet, "/v1/s/"+sessionID+"/facts/"+previousPlan, nil)
			if resp.Code != http.StatusOK {
				t.Fatalf("failed to inspect prior plan: status=%d body=%s", resp.Code, resp.Body.String())
			}
			var fact core.Fact
			if err := json.NewDecoder(resp.Body).Decode(&fact); err != nil {
				t.Fatalf("failed to decode prior plan: %v", err)
			}
			if fact.ResolvedStatus != core.Invalid {
				t.Fatalf("expected prior plan to be invalidated after contradiction, got %.2f", fact.ResolvedStatus)
			}
		}

		currentRootIDs[topic] = rootID
		currentPlanIDs[topic] = planID
	}

	for _, topic := range topics {
		resp := performRequest(t, server, http.MethodGet, "/v1/s/"+sessionID+"/slice?format=json&query="+topic+"&strategy=hybrid&max_facts=4&include_dependencies=true", nil)
		if resp.Code != http.StatusOK {
			t.Fatalf("failed to fetch horizon slice: status=%d body=%s", resp.Code, resp.Body.String())
		}
		var slice []*core.Fact
		if err := json.NewDecoder(resp.Body).Decode(&slice); err != nil {
			t.Fatalf("failed to decode horizon slice: %v", err)
		}
		expectedPlan := currentPlanIDs[topic]
		found := false
		for _, fact := range slice {
			if fact.ID == expectedPlan {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("expected latest plan %s to appear in slice for topic %s", expectedPlan, topic)
		}
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
