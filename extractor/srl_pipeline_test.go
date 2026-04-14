package extractor

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// ---------------------------------------------------------------------------
// Mock SRL service helpers
// ---------------------------------------------------------------------------

func newMockSRLServer(t *testing.T, response SRLExtractResponse) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/extract" {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
}

func newMockValidationServer(t *testing.T, accept bool, reason string) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/internal/validate-dependency" {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(ValidateDependencyResponse{
			Accepted: accept,
			Reason:   reason,
		})
	}))
}

// ---------------------------------------------------------------------------
// Test 1: Clause simplification splits relative clause
// ---------------------------------------------------------------------------
func TestClauseSimplificationSplitsRelativeClause(t *testing.T) {
	// The SRL service is expected to simplify:
	//   "The CEO, who founded Acme Corp, resigned yesterday."
	// into multiple facts (main clause + relative clause).
	resp := SRLExtractResponse{
		Facts: []SRLFactResult{
			{
				ID:                  "ceo-resigned",
				Subject:             "The CEO",
				Predicate:           "resigned",
				Object:              "yesterday",
				Claim:               "The CEO resigned yesterday.",
				Confidence:          0.85,
				AssertionKind:       "empirical",
				SRLConfidence:       0.85,
				SourceSentenceIndex: 0,
				IsRoot:              true,
			},
			{
				ID:                  "ceo-founded-acme",
				Subject:             "The CEO",
				Predicate:           "founded",
				Object:              "Acme Corp",
				Claim:               "The CEO founded Acme Corp.",
				Confidence:          0.80,
				AssertionKind:       "empirical",
				SRLConfidence:       0.80,
				SourceSentenceIndex: 0,
				IsRoot:              true,
			},
		},
		Stats: SRLExtractionStats{SimplifiedSentences: 2, FactsExtracted: 2},
	}
	srv := newMockSRLServer(t, resp)
	defer srv.Close()

	cfg := &ExtractionConfig{
		Tier:          TierSRL,
		SRLServiceURL: srv.URL,
	}
	result, err := RunSRLPipeline(context.Background(), "The CEO, who founded Acme Corp, resigned yesterday.", "", cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Facts) != 2 {
		t.Fatalf("expected 2 facts from clause split, got %d", len(result.Facts))
	}
	if result.Facts[0].ID != "ceo-resigned" {
		t.Errorf("expected first fact ID 'ceo-resigned', got %q", result.Facts[0].ID)
	}
	if result.Facts[1].ID != "ceo-founded-acme" {
		t.Errorf("expected second fact ID 'ceo-founded-acme', got %q", result.Facts[1].ID)
	}
}

// ---------------------------------------------------------------------------
// Test 2: Coreference resolves pronouns
// ---------------------------------------------------------------------------
func TestCoreferenceResolvesPronouns(t *testing.T) {
	resp := SRLExtractResponse{
		Facts: []SRLFactResult{
			{
				ID:            "john-joined",
				Subject:       "John",
				Predicate:     "joined",
				Object:        "the team",
				Claim:         "John joined the team.",
				Confidence:    0.90,
				AssertionKind: "empirical",
				SRLConfidence: 0.90,
				IsRoot:        true,
			},
			{
				ID:            "john-led-project",
				Subject:       "John",
				Predicate:     "led",
				Object:        "the project",
				Claim:         "John led the project.",
				Confidence:    0.88,
				AssertionKind: "empirical",
				SRLConfidence: 0.85,
				IsRoot:        true,
			},
		},
		CoreferenceMap: []SRLCoreferenceEntry{
			{
				PronounSpan: "He",
				Antecedent:  "John",
				Confidence:  0.92,
				Resolved:    true,
			},
		},
		Stats: SRLExtractionStats{CoreferencesResolved: 1, FactsExtracted: 2},
	}
	srv := newMockSRLServer(t, resp)
	defer srv.Close()

	cfg := &ExtractionConfig{
		Tier:          TierSRL,
		SRLServiceURL: srv.URL,
	}
	result, err := RunSRLPipeline(context.Background(), "John joined the team. He led the project.", "", cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Verify pronoun was resolved — "He" → "John" in the second fact.
	found := false
	for _, f := range result.Facts {
		if f.Subject == "John" && f.Predicate == "led" {
			found = true
		}
	}
	if !found {
		t.Error("expected coreference-resolved fact with subject 'John' and predicate 'led'")
	}
}

// ---------------------------------------------------------------------------
// Test 3: SRL extracts correct triple
// ---------------------------------------------------------------------------
func TestSRLExtractsCorrectTriple(t *testing.T) {
	resp := SRLExtractResponse{
		Facts: []SRLFactResult{
			{
				ID:            "apple-acquired-beats",
				Subject:       "Apple",
				Predicate:     "acquired",
				Object:        "Beats Electronics",
				Claim:         "[ARG0: Apple] [V: acquired] [ARG1: Beats Electronics] [ARGM-TMP: in 2014]",
				Confidence:    0.92,
				AssertionKind: "empirical",
				SRLConfidence: 0.92,
				Modifiers:     SRLFactModifiers{Temporal: "in 2014"},
				EntityTypes:   map[string]string{"subject": "ORG", "object": "ORG"},
				IsRoot:        true,
			},
		},
		Stats: SRLExtractionStats{FactsExtracted: 1, EntitiesFound: 2},
	}
	srv := newMockSRLServer(t, resp)
	defer srv.Close()

	cfg := &ExtractionConfig{
		Tier:          TierSRL,
		SRLServiceURL: srv.URL,
	}
	result, err := RunSRLPipeline(context.Background(), "Apple acquired Beats Electronics in 2014.", "", cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Facts) != 1 {
		t.Fatalf("expected 1 fact, got %d", len(result.Facts))
	}
	f := result.Facts[0]
	if f.Subject != "Apple" {
		t.Errorf("expected subject 'Apple', got %q", f.Subject)
	}
	if f.Predicate != "acquired" {
		t.Errorf("expected predicate 'acquired', got %q", f.Predicate)
	}
	if f.Object != "Beats Electronics" {
		t.Errorf("expected object 'Beats Electronics', got %q", f.Object)
	}
}

// ---------------------------------------------------------------------------
// Test 4: Discourse classifier detects causal connective
// ---------------------------------------------------------------------------
func TestDiscourseClassifierDetectsCausalConnective(t *testing.T) {
	resp := SRLExtractResponse{
		Facts: []SRLFactResult{
			{
				ID:            "stock-fell",
				Subject:       "The stock price",
				Predicate:     "fell",
				Object:        "sharply",
				Claim:         "The stock price fell sharply.",
				Confidence:    0.85,
				AssertionKind: "empirical",
				SRLConfidence: 0.85,
				IsRoot:        true,
			},
			{
				ID:            "earnings-missed",
				Subject:       "The company",
				Predicate:     "missed",
				Object:        "earnings estimates",
				Claim:         "The company missed earnings estimates.",
				Confidence:    0.88,
				AssertionKind: "empirical",
				SRLConfidence: 0.88,
				DependsOn:     []string{"stock-fell"},
				IsRoot:        false,
			},
		},
		Stats: SRLExtractionStats{
			FactsExtracted: 2,
			EdgesProposed:  1,
			EdgesAccepted:  1,
		},
	}
	srv := newMockSRLServer(t, resp)
	defer srv.Close()

	cfg := &ExtractionConfig{
		Tier:          TierSRL,
		SRLServiceURL: srv.URL,
	}
	result, err := RunSRLPipeline(context.Background(), "The stock price fell sharply because the company missed earnings estimates.", "", cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Verify a causal dependency was created.
	var derived *ExtractedFact
	for i, f := range result.Facts {
		if !f.IsRoot {
			derived = &result.Facts[i]
			break
		}
	}
	if derived == nil {
		t.Fatal("expected at least one derived fact with causal dependency")
	}
	if len(derived.DependsOn) == 0 {
		t.Error("expected derived fact to have DependsOn set")
	}
	if result.Stats.Stage3EdgesAccepted < 1 {
		t.Error("expected at least 1 accepted edge")
	}
}

// ---------------------------------------------------------------------------
// Test 5: Entity overlap produces candidate edge
// ---------------------------------------------------------------------------
func TestEntityOverlapProducesCandidateEdge(t *testing.T) {
	resp := SRLExtractResponse{
		Facts: []SRLFactResult{
			{
				ID: "fact-a", Subject: "Acme Corp", Predicate: "launched", Object: "a new product",
				Claim: "Acme Corp launched a new product.", Confidence: 0.85,
				AssertionKind: "empirical", SRLConfidence: 0.85, IsRoot: true,
				EntityTypes: map[string]string{"subject": "ORG"},
			},
			{
				ID: "fact-b", Subject: "Acme Corp", Predicate: "hired", Object: "50 engineers",
				Claim: "Acme Corp hired 50 engineers.", Confidence: 0.80,
				AssertionKind: "empirical", SRLConfidence: 0.80, IsRoot: false,
				DependsOn:   []string{"fact-a"},
				EntityTypes: map[string]string{"subject": "ORG"},
			},
		},
		Stats: SRLExtractionStats{
			FactsExtracted: 2,
			EdgesProposed:  1,
			EdgesAccepted:  1,
		},
	}
	srv := newMockSRLServer(t, resp)
	defer srv.Close()

	cfg := &ExtractionConfig{
		Tier:          TierSRL,
		SRLServiceURL: srv.URL,
	}
	result, err := RunSRLPipeline(context.Background(), "Acme Corp launched a new product. Acme Corp hired 50 engineers.", "", cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// At least one fact should be derived (entity overlap created a dependency).
	hasDerived := false
	for _, f := range result.Facts {
		if !f.IsRoot {
			hasDerived = true
		}
	}
	if !hasDerived {
		t.Error("expected entity overlap to produce at least one derived fact")
	}
}

// ---------------------------------------------------------------------------
// Test 6: TMS validation rejects cycle
// ---------------------------------------------------------------------------
func TestTMSValidationRejectsCycle(t *testing.T) {
	// fact-a depends on fact-b (existing edge).
	// Propose fact-b depends on fact-a → creates cycle: A→B→A.
	req := ValidateDependencyRequest{
		ParentID: "fact-a",
		ChildID:  "fact-b",
		Facts: []ValidateDependencyFact{
			{
				ID: "fact-a", Claim: "A is true", Confidence: 0.9,
				AssertionKind: "empirical", DependsOn: []string{"fact-b"}, IsRoot: false,
			},
			{
				ID: "fact-b", Claim: "B is true", Confidence: 0.9,
				AssertionKind: "empirical", DependsOn: []string{}, IsRoot: true,
			},
		},
	}
	result := ValidateDependency(req)
	if result.Accepted {
		t.Error("expected cycle to be rejected, but it was accepted")
	}
	if !strings.Contains(result.Reason, "cycle") {
		t.Errorf("expected reason to mention 'cycle', got %q", result.Reason)
	}
}

// ---------------------------------------------------------------------------
// Test 7: Ambiguous parse produces hypothetical pair
// ---------------------------------------------------------------------------
func TestAmbiguousParseProducesHypotheticalPair(t *testing.T) {
	resp := SRLExtractResponse{
		Facts: []SRLFactResult{
			{
				ID: "interp-1", Subject: "The bank", Predicate: "approved", Object: "the loan",
				Claim: "The bank approved the loan.", Confidence: 0.45,
				AssertionKind: "hypothetical", SRLConfidence: 0.45, IsRoot: true,
				SourceSentenceIndex: 0,
			},
			{
				ID: "interp-2", Subject: "The bank", Predicate: "denied", Object: "the loan",
				Claim: "The bank denied the loan.", Confidence: 0.42,
				AssertionKind: "hypothetical", SRLConfidence: 0.42, IsRoot: true,
				SourceSentenceIndex: 0,
			},
		},
		ConflictPairs: []SRLConflictPair{
			{FactAID: "interp-1", FactBID: "interp-2", Reason: "ambiguous_parse"},
		},
		Stats: SRLExtractionStats{FactsExtracted: 2, AmbiguousPairs: 1},
	}
	srv := newMockSRLServer(t, resp)
	defer srv.Close()

	cfg := &ExtractionConfig{
		Tier:          TierSRL,
		SRLServiceURL: srv.URL,
	}
	result, err := RunSRLPipeline(context.Background(), "The bank approved the loan.", "", cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Facts) < 2 {
		t.Fatalf("expected 2 hypothetical facts, got %d", len(result.Facts))
	}
	for _, f := range result.Facts {
		if f.AssertionKind != "hypothetical" {
			t.Errorf("expected fact %q to be hypothetical, got %q", f.ID, f.AssertionKind)
		}
	}
}

// ---------------------------------------------------------------------------
// Test 8: Tier 2 hybrid falls back for low confidence
// ---------------------------------------------------------------------------
func TestTierHybridFallsBackForLowConfidence(t *testing.T) {
	// SRL service returns one low-confidence fact.
	srlResp := SRLExtractResponse{
		Facts: []SRLFactResult{
			{
				ID: "low-conf-fact", Subject: "Something", Predicate: "happened", Object: "",
				Claim: "Something happened.", Confidence: 0.3,
				AssertionKind: "uncertain", SRLConfidence: 0.3, IsRoot: true,
				SourceSentenceIndex: 0,
			},
		},
		Stats: SRLExtractionStats{FactsExtracted: 1},
	}
	srlSrv := newMockSRLServer(t, srlResp)
	defer srlSrv.Close()

	// Mock LLM just returns a canned V-Logic response.
	llm := &mockLLMClient{
		response: `fact something_happened: "Something happened" (confidence: 0.9, assertion_kind: empirical)`,
	}

	cfg := &ExtractionConfig{
		Tier:          TierHybrid,
		SRLServiceURL: srlSrv.URL,
	}
	result, err := RunHybridPipeline(context.Background(), llm, "Something happened.", "", cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Result should contain both SRL facts and LLM fallback facts.
	if len(result.Facts) < 1 {
		t.Fatal("expected at least 1 fact from hybrid pipeline")
	}
	// The low-confidence fact from SRL should still be present.
	hasSRL := false
	for _, f := range result.Facts {
		if f.SourceType == "srl_pipeline" {
			hasSRL = true
		}
	}
	if !hasSRL {
		t.Error("expected SRL-sourced facts in hybrid result")
	}
}

// mockLLMClient implements LLMClient for testing.
type mockLLMClient struct {
	response string
	err      error
}

func (m *mockLLMClient) Chat(ctx context.Context, model string, messages []ChatMessage) (string, error) {
	if m.err != nil {
		return "", m.err
	}
	return m.response, nil
}

// ---------------------------------------------------------------------------
// Test 9: Full SRL pipeline end-to-end
// ---------------------------------------------------------------------------
func TestFullSRLPipelineEndToEnd(t *testing.T) {
	resp := SRLExtractResponse{
		Facts: []SRLFactResult{
			{
				ID: "tesla-reported-revenue", Subject: "Tesla", Predicate: "reported", Object: "quarterly revenue of $25 billion",
				Claim: "Tesla reported quarterly revenue of $25 billion.", Confidence: 0.92,
				AssertionKind: "empirical", SRLConfidence: 0.92, IsRoot: true,
				EntityTypes: map[string]string{"subject": "ORG", "object": "MONEY"},
			},
			{
				ID: "revenue-exceeded-expectations", Subject: "The revenue", Predicate: "exceeded", Object: "analyst expectations",
				Claim: "The revenue exceeded analyst expectations.", Confidence: 0.88,
				AssertionKind: "empirical", SRLConfidence: 0.88,
				DependsOn: []string{"tesla-reported-revenue"}, IsRoot: false,
			},
			{
				ID: "shares-rose", Subject: "Shares", Predicate: "rose", Object: "8% in after-hours trading",
				Claim: "Shares rose 8% in after-hours trading.", Confidence: 0.90,
				AssertionKind: "empirical", SRLConfidence: 0.90,
				DependsOn: []string{"revenue-exceeded-expectations"}, IsRoot: false,
				Modifiers: SRLFactModifiers{Temporal: "in after-hours trading"},
			},
			{
				ID: "musk-attributed-growth", Subject: "Elon Musk", Predicate: "attributed", Object: "the growth to strong demand in China",
				Claim: "Elon Musk attributed the growth to strong demand in China.", Confidence: 0.85,
				AssertionKind: "empirical", SRLConfidence: 0.85,
				DependsOn: []string{"tesla-reported-revenue"}, IsRoot: false,
				EntityTypes: map[string]string{"subject": "PERSON"},
				Modifiers:   SRLFactModifiers{Location: "in China"},
			},
			{
				ID: "analysts-expect-growth", Subject: "Analysts", Predicate: "expect", Object: "continued growth next quarter",
				Claim: "Analysts expect continued growth next quarter.", Confidence: 0.70,
				AssertionKind: "uncertain", SRLConfidence: 0.70, IsRoot: true,
				Modifiers: SRLFactModifiers{Temporal: "next quarter"},
			},
		},
		CoreferenceMap: []SRLCoreferenceEntry{
			{PronounSpan: "He", Antecedent: "Elon Musk", Confidence: 0.95, Resolved: true},
		},
		Stats: SRLExtractionStats{
			SimplifiedSentences:  6,
			CoreferencesResolved: 1,
			EntitiesFound:        5,
			FactsExtracted:       5,
			EdgesProposed:        4,
			EdgesAccepted:        3,
			EdgesRejected:        1,
		},
	}
	srv := newMockSRLServer(t, resp)
	defer srv.Close()

	cfg := &ExtractionConfig{
		Tier:          TierSRL,
		SRLServiceURL: srv.URL,
	}

	text := `Tesla reported quarterly revenue of $25 billion. The revenue exceeded analyst expectations.
Shares rose 8% in after-hours trading. Elon Musk attributed the growth to strong demand in China.
He praised the team's execution. Analysts expect continued growth next quarter.`

	result, err := RunSRLPipeline(context.Background(), text, "earnings call analysis", cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify all 5 facts extracted.
	if len(result.Facts) != 5 {
		t.Fatalf("expected 5 facts, got %d", len(result.Facts))
	}

	// Verify edge stats.
	if result.Stats.Stage3EdgesProposed != 4 {
		t.Errorf("expected 4 edges proposed, got %d", result.Stats.Stage3EdgesProposed)
	}
	if result.Stats.Stage3EdgesAccepted != 3 {
		t.Errorf("expected 3 edges accepted, got %d", result.Stats.Stage3EdgesAccepted)
	}
	if result.Stats.Stage3EdgesRejected != 1 {
		t.Errorf("expected 1 edge rejected, got %d", result.Stats.Stage3EdgesRejected)
	}

	// Verify root / derived classification.
	roots := 0
	derived := 0
	for _, f := range result.Facts {
		if f.IsRoot {
			roots++
		} else {
			derived++
		}
	}
	if roots != 2 {
		t.Errorf("expected 2 root facts, got %d", roots)
	}
	if derived != 3 {
		t.Errorf("expected 3 derived facts, got %d", derived)
	}

	// Verify uncertain classification.
	var uncertainFact *ExtractedFact
	for i, f := range result.Facts {
		if f.AssertionKind == "uncertain" {
			uncertainFact = &result.Facts[i]
		}
	}
	if uncertainFact == nil {
		t.Error("expected at least one uncertain fact (analyst expectation)")
	}

	// Verify ToCoreFact conversion works for each fact.
	for _, f := range result.Facts {
		cf := f.ToCoreFact()
		if cf.ID == "" {
			t.Error("ToCoreFact produced empty ID")
		}
		if cf.AssertionKind == "" {
			t.Errorf("ToCoreFact for %s produced empty assertion_kind", cf.ID)
		}
		if !f.IsRoot && len(cf.JustificationSets) == 0 {
			t.Errorf("ToCoreFact for derived fact %s should have justification sets", cf.ID)
		}
	}
}

// ---------------------------------------------------------------------------
// Test: SRL service unreachable
// ---------------------------------------------------------------------------
func TestSRLServiceUnreachableReturnsError(t *testing.T) {
	cfg := &ExtractionConfig{
		Tier:          TierSRL,
		SRLServiceURL: "http://localhost:1", // nothing listening
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	_, err := RunSRLPipeline(ctx, "Test text.", "", cfg)
	if err == nil {
		t.Fatal("expected error when SRL service is unreachable")
	}
	if !strings.Contains(err.Error(), "srl_service_unreachable") {
		t.Errorf("expected 'srl_service_unreachable' in error, got: %v", err)
	}
}

// ---------------------------------------------------------------------------
// Test: ValidateDependency accepts valid edge
// ---------------------------------------------------------------------------
func TestTMSValidationAcceptsValidEdge(t *testing.T) {
	req := ValidateDependencyRequest{
		ParentID: "fact-a",
		ChildID:  "fact-b",
		Facts: []ValidateDependencyFact{
			{ID: "fact-a", Claim: "A is true", Confidence: 0.9, AssertionKind: "empirical", IsRoot: true},
			{ID: "fact-b", Claim: "B follows from A", Confidence: 0.8, AssertionKind: "empirical", DependsOn: []string{}, IsRoot: true},
		},
	}
	result := ValidateDependency(req)
	if !result.Accepted {
		t.Errorf("expected valid edge to be accepted, got rejected: %s", result.Reason)
	}
}

// Suppress unused import warnings for test helpers.
var _ = fmt.Sprintf
