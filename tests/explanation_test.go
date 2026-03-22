package tests

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"testing"
	"time"

	"velarix/core"

	"github.com/dgraph-io/badger/v4"
)

// TestExplainEndpoint verifies the /explain endpoint returns a structured explanation.
func TestExplainEndpoint(t *testing.T) {
	_, ts := setupTestServer(t)
	defer ts.Close()

	client := &http.Client{}
	sessionID := "explain_test"

	// Assert root fact
	rootFact := core.Fact{
		ID:           "patient_consent",
		IsRoot:       true,
		ManualStatus: core.Valid,
		Payload:      map[string]interface{}{"type": "hipaa"},
	}
	body, _ := json.Marshal(rootFact)
	req, _ := http.NewRequest("POST", fmt.Sprintf("%s/v1/s/%s/facts", ts.URL, sessionID), bytes.NewBuffer(body))
	req.Header.Set("Authorization", "Bearer test_admin_key")
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	if err != nil || resp.StatusCode != http.StatusCreated {
		t.Fatalf("Failed to assert root fact: %v (Status: %d)", err, resp.StatusCode)
	}
	if resp != nil {
		resp.Body.Close()
	}

	// Assert derived fact
	derivedFact := core.Fact{
		ID:                "treatment_allowed",
		JustificationSets: [][]string{{"patient_consent"}},
		Payload:           map[string]interface{}{"treatment": "antibiotics"},
	}
	body, _ = json.Marshal(derivedFact)
	req, _ = http.NewRequest("POST", fmt.Sprintf("%s/v1/s/%s/facts", ts.URL, sessionID), bytes.NewBuffer(body))
	req.Header.Set("Authorization", "Bearer test_admin_key")
	req.Header.Set("Content-Type", "application/json")
	resp, err = client.Do(req)
	if err != nil || resp.StatusCode != http.StatusCreated {
		t.Fatalf("Failed to assert derived fact: %v (Status: %d)", err, resp.StatusCode)
	}
	if resp != nil {
		resp.Body.Close()
	}

	// Call explain endpoint
	req, _ = http.NewRequest("GET", fmt.Sprintf("%s/v1/s/%s/explain?fact_id=treatment_allowed", ts.URL, sessionID), nil)
	req.Header.Set("Authorization", "Bearer test_admin_key")
	resp, err = client.Do(req)
	if err != nil || resp.StatusCode != http.StatusOK {
		t.Fatalf("Explain endpoint failed: %v (Status: %d)", err, resp.StatusCode)
	}
	defer resp.Body.Close()

	var explanation core.ExplanationOutput
	json.NewDecoder(resp.Body).Decode(&explanation)

	if explanation.FactID != "treatment_allowed" {
		t.Fatalf("Expected fact_id 'treatment_allowed', got %s", explanation.FactID)
	}
	if len(explanation.CausalChain) < 2 {
		t.Fatalf("Expected at least 2 facts in causal chain, got %d", len(explanation.CausalChain))
	}

	// Verify confidence tiers
	for _, belief := range explanation.CausalChain {
		if belief.Confidence > 0.9 && belief.Tier != "certain" {
			t.Errorf("Fact %s: confidence %.2f should be 'certain', got %s", belief.FactID, belief.Confidence, belief.Tier)
		}
	}
}

// TestTimestampAnchoredExplanation verifies timestamp-anchored explanations return historical state.
func TestTimestampAnchoredExplanation(t *testing.T) {
	_, ts := setupTestServer(t)
	defer ts.Close()
	client := &http.Client{}
	sessionID := "timestamp_test"

	// Assert initial fact
	body, _ := json.Marshal(core.Fact{
		ID:           "fact_A",
		IsRoot:       true,
		ManualStatus: core.Valid,
		Payload:      map[string]interface{}{"value": "initial"},
	})
	req, _ := http.NewRequest("POST", fmt.Sprintf("%s/v1/s/%s/facts", ts.URL, sessionID), bytes.NewBuffer(body))
	req.Header.Set("Authorization", "Bearer test_admin_key")
	req.Header.Set("Content-Type", "application/json")
	resp, _ := client.Do(req)
	if resp != nil {
		resp.Body.Close()
	}

	// Record time after first fact
	time.Sleep(50 * time.Millisecond)
	midpointTime := time.Now()

	// Assert another fact after the midpoint
	time.Sleep(50 * time.Millisecond)
	body, _ = json.Marshal(core.Fact{
		ID:           "fact_B",
		IsRoot:       true,
		ManualStatus: core.Valid,
		Payload:      map[string]interface{}{"value": "late"},
	})
	req, _ = http.NewRequest("POST", fmt.Sprintf("%s/v1/s/%s/facts", ts.URL, sessionID), bytes.NewBuffer(body))
	req.Header.Set("Authorization", "Bearer test_admin_key")
	req.Header.Set("Content-Type", "application/json")
	resp, _ = client.Do(req)
	if resp != nil {
		resp.Body.Close()
	}

	fmt.Println("Explaining at current time")
	// Explain at current time — should see both facts
	req, _ = http.NewRequest("GET", fmt.Sprintf("%s/v1/s/%s/explain?fact_id=fact_B", ts.URL, sessionID), nil)
	req.Header.Set("Authorization", "Bearer test_admin_key")
	resp, err := client.Do(req)
	if err != nil || resp.StatusCode != http.StatusOK {
		t.Fatalf("Current explain failed: %v (Status: %d)", err, resp.StatusCode)
	}
	if resp != nil {
		resp.Body.Close()
	}

	// Explain at midpoint — fact_B should NOT exist
	ts_str := midpointTime.Format(time.RFC3339Nano)
	params := url.Values{}
	params.Set("fact_id", "fact_A")
	params.Set("timestamp", ts_str)
	req, _ = http.NewRequest("GET", fmt.Sprintf("%s/v1/s/%s/explain?%s", ts.URL, sessionID, params.Encode()), nil)
	req.Header.Set("Authorization", "Bearer test_admin_key")
	resp, err = client.Do(req)
	if err != nil || resp.StatusCode != http.StatusOK {
		t.Fatalf("Timestamp explain failed: %v (Status: %d)", err, resp.StatusCode)
	}
	defer resp.Body.Close()

	var explanation core.ExplanationOutput
	json.NewDecoder(resp.Body).Decode(&explanation)

	// At the midpoint, fact_B didn't exist yet, so the chain should only contain fact_A
	for _, belief := range explanation.CausalChain {
		if belief.FactID == "fact_B" {
			t.Fatalf("fact_B should NOT appear in explanation at midpoint timestamp")
		}
	}
}

// TestCounterfactualExplanation verifies counterfactual analysis.
func TestCounterfactualExplanation(t *testing.T) {
	_, ts := setupTestServer(t)
	defer ts.Close()

	client := &http.Client{}
	sessionID := "counterfactual_test"

	// Build causal chain: consent -> access -> treatment
	facts := []core.Fact{
		{ID: "consent", IsRoot: true, ManualStatus: core.Valid, Payload: map[string]interface{}{"type": "hipaa"}},
	}
	for _, f := range facts {
		body, _ := json.Marshal(f)
		req, _ := http.NewRequest("POST", fmt.Sprintf("%s/v1/s/%s/facts", ts.URL, sessionID), bytes.NewBuffer(body))
		req.Header.Set("Authorization", "Bearer test_admin_key")
		req.Header.Set("Content-Type", "application/json")
		resp, _ := client.Do(req)
		if resp != nil {
			resp.Body.Close()
		}
	}

	// Derived facts
	body, _ := json.Marshal(core.Fact{
		ID:                "access",
		JustificationSets: [][]string{{"consent"}},
		Payload:           map[string]interface{}{"level": "full"},
	})
	req, _ := http.NewRequest("POST", fmt.Sprintf("%s/v1/s/%s/facts", ts.URL, sessionID), bytes.NewBuffer(body))
	req.Header.Set("Authorization", "Bearer test_admin_key")
	req.Header.Set("Content-Type", "application/json")
	resp, _ := client.Do(req)
	if resp != nil {
		resp.Body.Close()
	}

	body, _ = json.Marshal(core.Fact{
		ID:                "treatment",
		JustificationSets: [][]string{{"access"}},
		Payload:           map[string]interface{}{"drug": "antibiotics"},
	})
	req, _ = http.NewRequest("POST", fmt.Sprintf("%s/v1/s/%s/facts", ts.URL, sessionID), bytes.NewBuffer(body))
	req.Header.Set("Authorization", "Bearer test_admin_key")
	req.Header.Set("Content-Type", "application/json")
	resp, _ = client.Do(req)
	if resp != nil {
		resp.Body.Close()
	}

	// Request counterfactual: what if consent was removed?
	req, _ = http.NewRequest("GET", fmt.Sprintf("%s/v1/s/%s/explain?fact_id=treatment&counterfactual_fact_id=consent", ts.URL, sessionID), nil)
	req.Header.Set("Authorization", "Bearer test_admin_key")
	resp, _ = client.Do(req)

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("Counterfactual explain failed: Status %d", resp.StatusCode)
	}
	defer resp.Body.Close()

	var explanation core.ExplanationOutput
	json.NewDecoder(resp.Body).Decode(&explanation)

	if explanation.Counterfactual == nil {
		t.Fatal("Expected counterfactual result, got nil")
	}
	if explanation.Counterfactual.RemovedFactID != "consent" {
		t.Fatalf("Expected removed_fact_id 'consent', got %s", explanation.Counterfactual.RemovedFactID)
	}
	if explanation.Counterfactual.TotalCount == 0 {
		t.Fatal("Expected at least some impacted facts in counterfactual")
	}
	if explanation.Counterfactual.Narrative == "" {
		t.Fatal("Expected narrative in counterfactual explanation")
	}
}

// TestExplanationHashVerification verifies that stored explanations have correct integrity hashes.
func TestExplanationHashVerification(t *testing.T) {
	_, ts := setupTestServer(t)
	defer ts.Close()

	client := &http.Client{}
	sessionID := "hash_test"

	// Setup a fact and generate an explanation
	body, _ := json.Marshal(core.Fact{
		ID:           "root_fact",
		IsRoot:       true,
		ManualStatus: core.Valid,
		Payload:      map[string]interface{}{"value": "test"},
	})
	req, _ := http.NewRequest("POST", fmt.Sprintf("%s/v1/s/%s/facts", ts.URL, sessionID), bytes.NewBuffer(body))
	req.Header.Set("Authorization", "Bearer test_admin_key")
	req.Header.Set("Content-Type", "application/json")
	resp, _ := client.Do(req)
	if resp != nil {
		resp.Body.Close()
	}

	// Generate explanation (this also stores it)
	req, _ = http.NewRequest("GET", fmt.Sprintf("%s/v1/s/%s/explain?fact_id=root_fact", ts.URL, sessionID), nil)
	req.Header.Set("Authorization", "Bearer test_admin_key")
	resp, _ = client.Do(req)
	if resp != nil {
		resp.Body.Close()
	}

	// Retrieve stored explanations
	req, _ = http.NewRequest("GET", fmt.Sprintf("%s/v1/s/%s/explanations", ts.URL, sessionID), nil)
	req.Header.Set("Authorization", "Bearer test_admin_key")
	resp, err := client.Do(req)
	if err != nil || resp.StatusCode != http.StatusOK {
		t.Fatalf("Failed to retrieve explanations: %v (Status: %d)", err, resp.StatusCode)
	}
	defer resp.Body.Close()

	var records []struct {
		ContentHash string `json:"content_hash"`
		Tampered    bool   `json:"tampered"`
	}
	json.NewDecoder(resp.Body).Decode(&records)

	if len(records) == 0 {
		t.Fatal("Expected at least one stored explanation")
	}

	for _, r := range records {
		if r.ContentHash == "" {
			t.Error("Expected non-empty content hash")
		}
		if r.Tampered {
			t.Error("Expected tampered=false for untouched explanation")
		}
	}
}

// TestTamperedExplanationDetection verifies that corrupted explanations are flagged.
func TestTamperedExplanationDetection(t *testing.T) {
	server, ts := setupTestServer(t)
	defer ts.Close()

	client := &http.Client{}
	sessionID := "tamper_test"

	// Setup and generate explanation
	body, _ := json.Marshal(core.Fact{
		ID:           "test_fact",
		IsRoot:       true,
		ManualStatus: core.Valid,
		Payload:      map[string]interface{}{"value": "original"},
	})
	req, _ := http.NewRequest("POST", fmt.Sprintf("%s/v1/s/%s/facts", ts.URL, sessionID), bytes.NewBuffer(body))
	req.Header.Set("Authorization", "Bearer test_admin_key")
	req.Header.Set("Content-Type", "application/json")
	resp, _ := client.Do(req)
	if resp != nil {
		resp.Body.Close()
	}

	req, _ = http.NewRequest("GET", fmt.Sprintf("%s/v1/s/%s/explain?fact_id=test_fact", ts.URL, sessionID), nil)
	req.Header.Set("Authorization", "Bearer test_admin_key")
	resp, _ = client.Do(req)
	if resp != nil {
		resp.Body.Close()
	}

	// Corrupt the stored explanation by modifying it directly in BadgerDB
	prefix := []byte(fmt.Sprintf("explanations:%s:", sessionID))
	err := server.Store.DB().Update(func(txn *badger.Txn) error {
		it := txn.NewIterator(badger.DefaultIteratorOptions)
		defer it.Close()

		for it.Seek(prefix); it.ValidForPrefix(prefix); it.Next() {
			key := it.Item().KeyCopy(nil)
			return it.Item().Value(func(v []byte) error {
				// Tamper: modify the content field while keeping the original hash
				var record map[string]interface{}
				json.Unmarshal(v, &record)
				record["content"] = map[string]interface{}{"tampered": true}
				corrupted, _ := json.Marshal(record)
				return txn.Set(key, corrupted)
			})
		}
		return nil
	})
	if err != nil {
		t.Fatalf("Failed to corrupt explanation: %v", err)
	}

	// Retrieve — should show tampered=true
	req, _ = http.NewRequest("GET", fmt.Sprintf("%s/v1/s/%s/explanations", ts.URL, sessionID), nil)
	req.Header.Set("Authorization", "Bearer test_admin_key")
	resp, err = client.Do(req)
	if err != nil || resp.StatusCode != http.StatusOK {
		t.Fatalf("Failed to retrieve explanations: %v (Status: %d)", err, resp.StatusCode)
	}
	defer resp.Body.Close()

	var records []struct {
		Tampered bool `json:"tampered"`
	}
	json.NewDecoder(resp.Body).Decode(&records)

	if len(records) == 0 {
		t.Fatal("Expected stored explanations")
	}

	foundTampered := false
	for _, r := range records {
		if r.Tampered {
			foundTampered = true
		}
	}
	if !foundTampered {
		t.Fatal("Expected at least one tampered explanation after corruption")
	}
}
