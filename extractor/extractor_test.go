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

// TestExtract_CorrectJSONParsing verifies that a well-formed extraction response
// is parsed into the correct number of ExtractedFact objects with all fields
// preserved exactly as returned by the model.
func TestExtract_CorrectJSONParsing(t *testing.T) {
	input := []extractor.ExtractedFact{
		{
			ID:         "paris-capital",
			Claim:      "Paris is the capital of France",
			ClaimKey:   "capital",
			ClaimValue: "Paris",
			Subject:    "France",
			Predicate:  "capital",
			Object:     "Paris",
			Polarity:   "positive",
			IsRoot:     true,
			SourceType: "llm_output",
			Confidence: 0.95,
		},
		{
			ID:         "france-europe",
			Claim:      "France is located in Europe",
			ClaimKey:   "location",
			ClaimValue: "Europe",
			Subject:    "France",
			Predicate:  "located_in",
			Object:     "Europe",
			Polarity:   "positive",
			IsRoot:     true,
			SourceType: "llm_output",
			Confidence: 0.90,
		},
	}
	factsJSON, _ := json.Marshal(input)
	srv := newJSONServer(t, http.StatusOK, chatCompletionBody(string(factsJSON)))

	t.Setenv("OPENAI_API_KEY", "test-key")
	t.Setenv("VELARIX_OPENAI_BASE_URL", srv.URL)

	got, err := extractor.Extract(context.Background(),
		"Paris is the capital of France and is located in Europe.", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 facts, got %d", len(got))
	}
	if got[0].ID != "paris-capital" {
		t.Errorf("fact[0].ID = %q, want %q", got[0].ID, "paris-capital")
	}
	if got[0].Confidence != 0.95 {
		t.Errorf("fact[0].Confidence = %f, want 0.95", got[0].Confidence)
	}
	if got[1].ClaimKey != "location" {
		t.Errorf("fact[1].ClaimKey = %q, want %q", got[1].ClaimKey, "location")
	}
	if got[1].Subject != "France" {
		t.Errorf("fact[1].Subject = %q, want %q", got[1].Subject, "France")
	}
}

// TestExtract_CorrectJSONParsing_MarkdownFences verifies that ```json ... ```
// fences are stripped before parsing when a model ignores the no-fence rule.
func TestExtract_CorrectJSONParsing_MarkdownFences(t *testing.T) {
	facts := []extractor.ExtractedFact{
		{ID: "fact-1", Claim: "The sky is blue", IsRoot: true, Confidence: 0.9},
	}
	factsJSON, _ := json.Marshal(facts)
	fenced := "```json\n" + string(factsJSON) + "\n```"
	srv := newJSONServer(t, http.StatusOK, chatCompletionBody(fenced))

	t.Setenv("OPENAI_API_KEY", "test-key")
	t.Setenv("VELARIX_OPENAI_BASE_URL", srv.URL)

	got, err := extractor.Extract(context.Background(), "The sky is blue.", "")
	if err != nil {
		t.Fatalf("expected fences to be stripped; got error: %v", err)
	}
	if len(got) != 1 || got[0].ID != "fact-1" {
		t.Fatalf("expected 1 fact with id fact-1, got %+v", got)
	}
}

// TestExtractedFact_RootsFirstSortOrder verifies that applying the same
// sort.SliceStable predicate used in the extract-and-assert handler to a
// mixed-order slice produces an ordering where all root facts precede all
// derived facts, with relative order within each tier preserved (stable).
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

	// All roots must appear before any derived fact.
	seenDerived := false
	for _, f := range facts {
		if seenDerived && f.IsRoot {
			t.Fatalf("root %q appeared after a non-root fact — sort invariant broken", f.ID)
		}
		if !f.IsRoot {
			seenDerived = true
		}
	}

	// Exactly the first two entries should be the two roots.
	if !facts[0].IsRoot || !facts[1].IsRoot {
		t.Errorf("expected first two facts to be roots, got: %s(root=%v) %s(root=%v)",
			facts[0].ID, facts[0].IsRoot, facts[1].ID, facts[1].IsRoot)
	}

	// SliceStable must preserve relative root order: root-1 before root-2.
	if facts[0].ID != "root-1" || facts[1].ID != "root-2" {
		t.Errorf("stable sort broke root order: got %s, %s; want root-1, root-2",
			facts[0].ID, facts[1].ID)
	}
}

// TestExtract_ToCoreFact_RootProperties verifies that a root ExtractedFact
// converts to a core.Fact with IsRoot=true and ManualStatus set to Confidence.
func TestExtract_ToCoreFact_RootProperties(t *testing.T) {
	ef := extractor.ExtractedFact{
		ID:         "earth-orbits-sun",
		Claim:      "The earth orbits the sun",
		ClaimKey:   "orbit",
		ClaimValue: "sun",
		Subject:    "earth",
		Predicate:  "orbits",
		Object:     "sun",
		Polarity:   "positive",
		IsRoot:     true,
		Confidence: 0.85,
		SourceType: "llm_output",
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
	if f.ID != "earth-orbits-sun" {
		t.Errorf("ID = %q, want %q", f.ID, "earth-orbits-sun")
	}
	if f.Payload["claim"] != "The earth orbits the sun" {
		t.Errorf("Payload[claim] = %v", f.Payload["claim"])
	}
}

// TestExtract_ToCoreFact_DerivedProperties verifies that a derived ExtractedFact
// converts to a core.Fact with one single-element AND-set per dependency entry.
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
	if len(f.JustificationSets[0]) != 1 || f.JustificationSets[0][0] != "premise-a" {
		t.Errorf("JustificationSets[0] = %v, want [premise-a]", f.JustificationSets[0])
	}
	if len(f.JustificationSets[1]) != 1 || f.JustificationSets[1][0] != "premise-b" {
		t.Errorf("JustificationSets[1] = %v, want [premise-b]", f.JustificationSets[1])
	}
}

// TestExtract_TimeoutError verifies that when the HTTP call is cancelled —
// exercising the same code path as the hardcoded 15s internal timeout —
// Extract returns an *ExtractionError with Cause == "timeout".
func TestExtract_TimeoutError(t *testing.T) {
	// Server that blocks until its own request context is cancelled.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-r.Context().Done()
	}))
	t.Cleanup(srv.Close)

	t.Setenv("OPENAI_API_KEY", "test-key")
	t.Setenv("VELARIX_OPENAI_BASE_URL", srv.URL)

	// Immediately cancel the caller context to simulate deadline expiry.
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := extractor.Extract(ctx, "some text", "")
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

// TestExtract_Non200Response verifies that a 4xx / 5xx HTTP response from the
// extraction model returns an *ExtractionError with Cause == "api_error".
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

			_, err := extractor.Extract(context.Background(), "some text", "")
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

// TestExtract_AtomicDecomposition verifies that when the extraction model
// returns three atomic facts from a single compound input sentence, all three
// are parsed and returned with distinct, non-empty IDs and claims.
func TestExtract_AtomicDecomposition(t *testing.T) {
	atomicFacts := []extractor.ExtractedFact{
		{
			ID:         "water-boils-100c",
			Claim:      "Water boils at 100 degrees Celsius",
			ClaimKey:   "boiling_point",
			ClaimValue: "100C",
			Subject:    "water",
			Predicate:  "boils_at",
			Object:     "100C",
			Polarity:   "positive",
			IsRoot:     true,
			SourceType: "llm_output",
			Confidence: 0.98,
		},
		{
			ID:         "water-freezes-0c",
			Claim:      "Water freezes at 0 degrees Celsius",
			ClaimKey:   "freezing_point",
			ClaimValue: "0C",
			Subject:    "water",
			Predicate:  "freezes_at",
			Object:     "0C",
			Polarity:   "positive",
			IsRoot:     true,
			SourceType: "llm_output",
			Confidence: 0.98,
		},
		{
			ID:         "water-is-h2o",
			Claim:      "Water is composed of H2O",
			ClaimKey:   "composition",
			ClaimValue: "H2O",
			Subject:    "water",
			Predicate:  "composed_of",
			Object:     "H2O",
			Polarity:   "positive",
			IsRoot:     true,
			SourceType: "llm_output",
			Confidence: 0.99,
		},
	}
	factsJSON, _ := json.Marshal(atomicFacts)
	srv := newJSONServer(t, http.StatusOK, chatCompletionBody(string(factsJSON)))

	t.Setenv("OPENAI_API_KEY", "test-key")
	t.Setenv("VELARIX_OPENAI_BASE_URL", srv.URL)

	got, err := extractor.Extract(context.Background(),
		"Water boils at 100 degrees Celsius, freezes at 0 degrees Celsius, and is composed of H2O.",
		"")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 3 {
		t.Fatalf("expected 3 atomic facts from compound sentence, got %d", len(got))
	}

	idSet := map[string]bool{}
	for _, f := range got {
		idSet[f.ID] = true
		if strings.TrimSpace(f.Claim) == "" {
			t.Errorf("fact %q has empty Claim", f.ID)
		}
	}
	for _, wantID := range []string{"water-boils-100c", "water-freezes-0c", "water-is-h2o"} {
		if !idSet[wantID] {
			t.Errorf("expected fact ID %q in decomposed results; got IDs: %v", wantID, idSet)
		}
	}
}

// TestExtract_MissingAPIKey verifies that the configuration guard fires when
// OPENAI_API_KEY is absent, returning an *ExtractionError{Cause: "configuration"}.
func TestExtract_MissingAPIKey(t *testing.T) {
	t.Setenv("OPENAI_API_KEY", "")

	_, err := extractor.Extract(context.Background(), "any text", "")
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

// TestExtract_SessionContextPrefixed verifies that sessionContext is prepended
// to the user message body so the extraction model has domain context.
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
		// Return empty facts array so Extract succeeds.
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(chatCompletionBody("[]"))
	}))
	t.Cleanup(srv.Close)

	t.Setenv("OPENAI_API_KEY", "test-key")
	t.Setenv("VELARIX_OPENAI_BASE_URL", srv.URL)

	_, _ = extractor.Extract(context.Background(), "the llm output text", "domain: medical")

	if !strings.Contains(capturedUserMsg, "domain: medical") {
		t.Errorf("session context not in user message; got: %q", capturedUserMsg)
	}
	if !strings.Contains(capturedUserMsg, "the llm output text") {
		t.Errorf("llm output missing from user message; got: %q", capturedUserMsg)
	}
}
