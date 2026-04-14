package extractor_test

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"sort"
	"strings"
	"testing"

	"velarix/extractor"
)

// chatCompletionBody wraps content in a minimal OpenAI chat-completion envelope.
func chatCompletionBody(content string) []byte {
	resp := map[string]interface{}{
		"choices": []map[string]interface{}{
			{"message": map[string]string{"content": content}},
		},
	}
	b, _ := json.Marshal(resp)
	return b
}

// newJSONServer starts a test HTTP server that responds with the given HTTP
// status and body. It is automatically closed via t.Cleanup.
func newJSONServer(t *testing.T, status int, body []byte) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_, _ = w.Write(body)
	}))
	t.Cleanup(srv.Close)
	return srv
}

// TestExtract_CorrectVLogicParsing verifies that a well-formed extraction response
// is parsed into the correct number of ExtractedFact objects.
func TestExtract_CorrectVLogicParsing(t *testing.T) {
	vlogic := `fact paris-capital: "Paris is the capital of France" (confidence: 0.95)
fact france-europe: "France is located in Europe" (confidence: 0.90)`

	srv := newJSONServer(t, http.StatusOK, chatCompletionBody(vlogic))

	t.Setenv("OPENAI_API_KEY", "test-key")
	t.Setenv("VELARIX_OPENAI_BASE_URL", srv.URL)

	cfg := &extractor.ExtractionConfig{Tier: extractor.TierFullLLM, EnableSelection: false, EnableDecontextualisation: false, EnableCoverageVerification: false, EnableConsistencyPrecheck: false}
	res, err := extractor.Extract(context.Background(),
		"Paris is the capital of France and is located in Europe.", "", cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got := res.Facts
	if len(got) != 2 {
		t.Fatalf("expected 2 facts, got %d", len(got))
	}

	// Topological sort preserves order if independent, but we just verify they exist
	if got[0].ID != "paris-capital" && got[1].ID != "paris-capital" {
		t.Errorf("missing paris-capital fact")
	}
	if got[0].Confidence != 0.95 && got[1].Confidence != 0.95 {
		t.Errorf("missing 0.95 confidence")
	}
}

// TestExtract_CorrectVLogicParsing_MarkdownFences verifies that fences are stripped.
func TestExtract_CorrectVLogicParsing_MarkdownFences(t *testing.T) {
	vlogic := "```vlogic\nfact fact-1: \"The sky is blue\" (confidence: 0.9)\n```"
	srv := newJSONServer(t, http.StatusOK, chatCompletionBody(vlogic))

	t.Setenv("OPENAI_API_KEY", "test-key")
	t.Setenv("VELARIX_OPENAI_BASE_URL", srv.URL)

	cfg := &extractor.ExtractionConfig{Tier: extractor.TierFullLLM, EnableSelection: false, EnableDecontextualisation: false, EnableCoverageVerification: false, EnableConsistencyPrecheck: false}
	res, err := extractor.Extract(context.Background(), "The sky is blue.", "", cfg)
	if err != nil {
		t.Fatalf("expected fences to be stripped; got error: %v", err)
	}
	got := res.Facts
	if len(got) != 1 || got[0].ID != "fact-1" {
		t.Fatalf("expected 1 fact with id fact-1, got %+v", got)
	}
}

// TestExtractedFact_RootsFirstSortOrder verifies that applying the same
// sort.SliceStable predicate used in the extract-and-assert handler to a
// mixed-order slice produces an ordering where all root facts precede all
// derived facts.
func TestExtractedFact_RootsFirstSortOrder(t *testing.T) {
	facts := []extractor.ExtractedFact{
		{ID: "derived-1", IsRoot: false},
		{ID: "root-1", IsRoot: true},
		{ID: "derived-2", IsRoot: false},
		{ID: "root-2", IsRoot: true},
		{ID: "derived-3", IsRoot: false},
	}

	sort.SliceStable(facts, func(i, j int) bool {
		return facts[i].IsRoot && !facts[j].IsRoot
	})

	seenDerived := false
	for _, f := range facts {
		if seenDerived && f.IsRoot {
			t.Fatalf("root %q appeared after a non-root fact", f.ID)
		}
		if !f.IsRoot {
			seenDerived = true
		}
	}
}

func TestExtract_ToCoreFact_RootProperties(t *testing.T) {
	ef := extractor.ExtractedFact{
		ID:         "earth-orbits-sun",
		Claim:      "The earth orbits the sun",
		IsRoot:     true,
		Confidence: 0.85,
		SourceType: "v-logic",
	}

	f := ef.ToCoreFact()
	if !f.IsRoot {
		t.Error("expected ToCoreFact to produce a root fact")
	}
	if len(f.JustificationSets) != 0 {
		t.Errorf("root fact must have no JustificationSets, got %v", f.JustificationSets)
	}
	if float64(f.ManualStatus) != 0.85 {
		t.Errorf("ManualStatus = %v, want 0.85", f.ManualStatus)
	}
}

func TestExtract_ToCoreFact_DerivedProperties(t *testing.T) {
	ef := extractor.ExtractedFact{
		ID:        "inference-1",
		Claim:     "Therefore X follows",
		IsRoot:    false,
		DependsOn: []string{"premise-a", "premise-b"},
	}

	f := ef.ToCoreFact()
	if f.IsRoot {
		t.Error("expected derived fact, got IsRoot=true")
	}
	if len(f.JustificationSets) != 2 {
		t.Fatalf("expected 2 JustificationSets (one per dep), got %d", len(f.JustificationSets))
	}
}

func TestExtract_AssertionKindClassificationToCoreFact(t *testing.T) {
	vlogic := `fact emp: "Alice is the CEO of Acme" (confidence: 0.9, assertion_kind: empirical)
fact hedged: "I think Bob is the CFO of Acme" (confidence: 0.6, assertion_kind: uncertain)
fact conditional: "If revenue grows, hiring will increase" (confidence: 0.8, assertion_kind: hypothetical)
fact story: "In the story, gravity pulls upward" (confidence: 0.9, assertion_kind: fictional)`

	extracted, err := extractor.ParseVLogic(vlogic)
	if err != nil {
		t.Fatalf("ParseVLogic error: %v", err)
	}
	if len(extracted) != 4 {
		t.Fatalf("expected 4 extracted facts, got %d", len(extracted))
	}

	got := map[string]string{}
	for _, ef := range extracted {
		got[ef.ID] = ef.ToCoreFact().AssertionKind
	}
	if got["emp"] != "empirical" {
		t.Fatalf("emp assertion_kind=%q, want empirical", got["emp"])
	}
	if got["hedged"] != "uncertain" {
		t.Fatalf("hedged assertion_kind=%q, want uncertain", got["hedged"])
	}
	if got["conditional"] != "hypothetical" {
		t.Fatalf("conditional assertion_kind=%q, want hypothetical", got["conditional"])
	}
	if got["story"] != "fictional" {
		t.Fatalf("story assertion_kind=%q, want fictional", got["story"])
	}
}

func TestExtract_TimeoutError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-r.Context().Done()
	}))
	t.Cleanup(srv.Close)

	t.Setenv("OPENAI_API_KEY", "test-key")
	t.Setenv("VELARIX_OPENAI_BASE_URL", srv.URL)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	cfg := &extractor.ExtractionConfig{Tier: extractor.TierFullLLM, EnableSelection: false, EnableDecontextualisation: false, EnableCoverageVerification: false, EnableConsistencyPrecheck: false}
	_, err := extractor.Extract(ctx, "some text", "", cfg)
	if err == nil {
		t.Fatal("expected error on cancelled context, got nil")
	}
	var extractErr *extractor.ExtractionError
	if !errors.As(err, &extractErr) {
		t.Fatalf("expected *extractor.ExtractionError, got %T: %v", err, err)
	}
	if extractErr.Cause != "timeout" {
		t.Errorf("Cause = %q, want %q", extractErr.Cause, "timeout")
	}
}

func TestExtract_Non200Response(t *testing.T) {
	cases := []struct {
		name   string
		status int
	}{
		{"rate_limited_429", http.StatusTooManyRequests},
		{"server_error_500", http.StatusInternalServerError},
		{"forbidden_403", http.StatusForbidden},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			srv := newJSONServer(t, tc.status, []byte(`{"error":"test error"}`))

			t.Setenv("OPENAI_API_KEY", "test-key")
			t.Setenv("VELARIX_OPENAI_BASE_URL", srv.URL)

			cfg := &extractor.ExtractionConfig{Tier: extractor.TierFullLLM, EnableSelection: false, EnableDecontextualisation: false, EnableCoverageVerification: false, EnableConsistencyPrecheck: false}
			_, err := extractor.Extract(context.Background(), "some text", "", cfg)
			if err == nil {
				t.Fatalf("expected error for HTTP %d, got nil", tc.status)
			}
			var extractErr *extractor.ExtractionError
			if !errors.As(err, &extractErr) {
				t.Fatalf("expected *extractor.ExtractionError, got %T: %v", err, err)
			}
			if extractErr.Cause != "api_error" {
				t.Errorf("Cause = %q, want %q", extractErr.Cause, "api_error")
			}
		})
	}
}

func TestExtract_AtomicDecomposition(t *testing.T) {
	vlogic := `fact water-boils-100c: "Water boils at 100 degrees Celsius" (confidence: 0.98)
fact water-freezes-0c: "Water freezes at 0 degrees Celsius" (confidence: 0.98)
fact water-is-h2o: "Water is composed of H2O" (confidence: 0.99)`

	srv := newJSONServer(t, http.StatusOK, chatCompletionBody(vlogic))

	t.Setenv("OPENAI_API_KEY", "test-key")
	t.Setenv("VELARIX_OPENAI_BASE_URL", srv.URL)

	cfg := &extractor.ExtractionConfig{Tier: extractor.TierFullLLM, EnableSelection: false, EnableDecontextualisation: false, EnableCoverageVerification: false, EnableConsistencyPrecheck: false}
	res, err := extractor.Extract(context.Background(),
		"Water boils at 100 degrees Celsius, freezes at 0 degrees Celsius, and is composed of H2O.",
		"", cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got := res.Facts
	if len(got) != 3 {
		t.Fatalf("expected 3 atomic facts from compound sentence, got %d", len(got))
	}
}

func TestExtract_MissingAPIKey(t *testing.T) {
	t.Setenv("OPENAI_API_KEY", "")

	cfg := &extractor.ExtractionConfig{Tier: extractor.TierFullLLM, EnableSelection: false, EnableDecontextualisation: false, EnableCoverageVerification: false, EnableConsistencyPrecheck: false}
	_, err := extractor.Extract(context.Background(), "any text", "", cfg)
	if err == nil {
		t.Fatal("expected error when API key is missing, got nil")
	}
	var extractErr *extractor.ExtractionError
	if !errors.As(err, &extractErr) {
		t.Fatalf("expected *extractor.ExtractionError, got %T: %v", err, err)
	}
	if extractErr.Cause != "configuration" {
		t.Errorf("Cause = %q, want %q", extractErr.Cause, "configuration")
	}
}

func TestExtract_SessionContextPrefixed(t *testing.T) {
	var capturedUserMsg string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var req struct {
			Messages []struct {
				Role    string `json:"role"`
				Content string `json:"content"`
			} `json:"messages"`
		}
		if err := json.Unmarshal(body, &req); err == nil {
			for _, m := range req.Messages {
				if m.Role == "user" {
					capturedUserMsg = m.Content
				}
			}
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(chatCompletionBody("fact f1: \"test\""))
	}))
	t.Cleanup(srv.Close)

	t.Setenv("OPENAI_API_KEY", "test-key")
	t.Setenv("VELARIX_OPENAI_BASE_URL", srv.URL)

	cfg := &extractor.ExtractionConfig{Tier: extractor.TierFullLLM, EnableSelection: false, EnableDecontextualisation: false, EnableCoverageVerification: false, EnableConsistencyPrecheck: false}
	_, _ = extractor.Extract(context.Background(), "the llm output text", "domain: medical", cfg)

	if !strings.Contains(capturedUserMsg, "domain: medical") {
		t.Errorf("session context not in user message; got: %q", capturedUserMsg)
	}
	if !strings.Contains(capturedUserMsg, "the llm output text") {
		t.Errorf("llm output missing from user message; got: %q", capturedUserMsg)
	}
}
