package extractor

import (
	"context"
	"encoding/json"
	"strings"
	"sync"
	"testing"

	"velarix/core"
)

type mockLLM struct {
	mu sync.Mutex

	stage1Response string
	stage2Response string
	stage3ABySentence map[string]string
	stage3BResponder  func(parentClaim, childClaim string) (dependencyCheckResponse, bool)
	stage4Response string
}

func (m *mockLLM) Chat(_ context.Context, _ string, messages []ChatMessage) (string, error) {
	system := ""
	if len(messages) > 0 {
		system = messages[0].Content
	}
	user := ""
	for _, msg := range messages {
		if msg.Role == "user" {
			user = msg.Content
			break
		}
	}

	switch {
	case strings.HasPrefix(system, "STAGE 1"):
		m.mu.Lock()
		defer m.mu.Unlock()
		return m.stage1Response, nil
	case strings.HasPrefix(system, "STAGE 2"):
		m.mu.Lock()
		defer m.mu.Unlock()
		return m.stage2Response, nil
	case strings.HasPrefix(system, "STAGE 3A"):
		sentence := strings.TrimSpace(afterMarker(user, "SENTENCE:"))
		m.mu.Lock()
		defer m.mu.Unlock()
		if m.stage3ABySentence == nil {
			return "[]", nil
		}
		if resp, ok := m.stage3ABySentence[sentence]; ok {
			return resp, nil
		}
		return "[]", nil
	case strings.HasPrefix(system, "STAGE 3B"):
		parent := strings.TrimSpace(between(user, "PARENT:\n", "\n\nCHILD:\n"))
		child := strings.TrimSpace(afterMarker(user, "CHILD:\n"))
		if m.stage3BResponder != nil {
			if resp, ok := m.stage3BResponder(parent, child); ok {
				b, _ := json.Marshal(resp)
				return string(b), nil
			}
		}
		b, _ := json.Marshal(dependencyCheckResponse{Depends: "no", Confidence: 0.9, Justification: "no dependency"})
		return string(b), nil
	case strings.HasPrefix(system, "STAGE 4"):
		m.mu.Lock()
		defer m.mu.Unlock()
		return m.stage4Response, nil
	default:
		// Legacy baseline V-Logic compiler calls.
		return "fact f1: \"test\" (confidence: 0.9)", nil
	}
}

func afterMarker(s, marker string) string {
	idx := strings.Index(s, marker)
	if idx < 0 {
		return ""
	}
	return s[idx+len(marker):]
}

func between(s, start, end string) string {
	i := strings.Index(s, start)
	if i < 0 {
		return ""
	}
	rest := s[i+len(start):]
	j := strings.Index(rest, end)
	if j < 0 {
		return rest
	}
	return rest[:j]
}

func TestStage1SelectionFiltersNonVerifiable(t *testing.T) {
	llm := &mockLLM{
		stage1Response: `[
			{"sentence_index":0,"sentence":"Paris is the capital of France.","category":"verifiable","reason":"direct factual claim"},
			{"sentence_index":1,"sentence":"I love Paris.","category":"discard","reason":"opinion"},
			{"sentence_index":2,"sentence":"Is Paris in France?","category":"discard","reason":"question"}
		]`,
	}

	cfg := DefaultExtractionConfig()
	cfg.EnableSelection = true

	out, err := Stage1SentenceSelection(context.Background(), llm, "Paris is the capital of France. I love Paris. Is Paris in France?", "", cfg)
	if err != nil {
		t.Fatalf("Stage1SentenceSelection error: %v", err)
	}
	if len(out.Selected) != 1 {
		t.Fatalf("selected=%d, want 1", len(out.Selected))
	}
	if len(out.Discarded) != 2 {
		t.Fatalf("discarded=%d, want 2", len(out.Discarded))
	}
	if out.Selected[0].OriginalSentence != "Paris is the capital of France." {
		t.Fatalf("kept sentence=%q, want factual claim", out.Selected[0].OriginalSentence)
	}
}

func TestStage2DecontextualisationResolvesPronouns(t *testing.T) {
	llm := &mockLLM{
		stage2Response: `[
			{"sentence_index":1,"original":"She won the prize.","decontextualised":"Marie Curie won the prize.","unresolved_references":[],"confidence":0.95}
		]`,
	}
	cfg := DefaultExtractionConfig()
	cfg.EnableDecontextualisation = true

	selected := []SelectedSentence{
		{SentenceIndex: 1, OriginalSentence: "She won the prize."},
	}

	out, err := Stage2Decontextualise(context.Background(), llm, "Marie Curie was the scientist. She won the prize.", selected, cfg)
	if err != nil {
		t.Fatalf("Stage2Decontextualise error: %v", err)
	}
	if len(out.Sentences) != 1 {
		t.Fatalf("sentences=%d, want 1", len(out.Sentences))
	}
	if out.Sentences[0].Decontextualised != "Marie Curie won the prize." {
		t.Fatalf("decontextualised=%q, want resolved", out.Sentences[0].Decontextualised)
	}
}

func TestStage3BRejectsCyclicDependency(t *testing.T) {
	llm := &mockLLM{
		stage3BResponder: func(parentClaim, childClaim string) (dependencyCheckResponse, bool) {
			if (parentClaim == "Alpha equals Beta." && childClaim == "Beta equals Alpha.") ||
				(parentClaim == "Beta equals Alpha." && childClaim == "Alpha equals Beta.") {
				return dependencyCheckResponse{Depends: "yes", Confidence: 0.9, Justification: "mutual dependency"}, true
			}
			return dependencyCheckResponse{}, false
		},
	}
	cfg := DefaultExtractionConfig()
	cfg.MaxDependencyCheckConcurrency = 4

	facts := []AtomicFact{
		{ExtractedFact: ExtractedFact{ID: "a", Claim: "Alpha equals Beta.", Subject: "alpha", Predicate: "equals", Object: "beta", Confidence: 0.9, AssertionKind: core.AssertionKindEmpirical}},
		{ExtractedFact: ExtractedFact{ID: "b", Claim: "Beta equals Alpha.", Subject: "beta", Predicate: "equals", Object: "alpha", Confidence: 0.9, AssertionKind: core.AssertionKindEmpirical}},
	}

	out, stats, err := Stage3BInferDependencies(context.Background(), llm, facts, cfg)
	if err != nil {
		t.Fatalf("Stage3BInferDependencies error: %v", err)
	}
	if stats.Proposed != 2 {
		t.Fatalf("proposed=%d, want 2", stats.Proposed)
	}
	if stats.Accepted != 1 {
		t.Fatalf("accepted=%d, want 1", stats.Accepted)
	}
	if stats.Rejected != 1 {
		t.Fatalf("rejected=%d, want 1", stats.Rejected)
	}

	var a, b ExtractedFact
	for _, f := range out {
		if f.ID == "a" {
			a = f
		}
		if f.ID == "b" {
			b = f
		}
	}
	if a.IsRoot == b.IsRoot {
		t.Fatalf("expected one root and one derived, got a.IsRoot=%v b.IsRoot=%v", a.IsRoot, b.IsRoot)
	}
}

func TestStage3BAcceptsValidDependency(t *testing.T) {
	llm := &mockLLM{
		stage3BResponder: func(parentClaim, childClaim string) (dependencyCheckResponse, bool) {
			if parentClaim == "Marie Curie was a scientist." && childClaim == "Marie Curie won the Nobel Prize." {
				return dependencyCheckResponse{Depends: "yes", Confidence: 0.9, Justification: "winning depends on being the same person"}, true
			}
			return dependencyCheckResponse{}, false
		},
	}
	cfg := DefaultExtractionConfig()

	facts := []AtomicFact{
		{ExtractedFact: ExtractedFact{ID: "f1", Claim: "Marie Curie was a scientist.", Subject: "marie curie", Predicate: "was", Object: "scientist", Confidence: 0.9, AssertionKind: core.AssertionKindEmpirical}},
		{ExtractedFact: ExtractedFact{ID: "f2", Claim: "Marie Curie won the Nobel Prize.", Subject: "marie curie", Predicate: "won", Object: "nobel prize", Confidence: 0.9, AssertionKind: core.AssertionKindEmpirical}},
	}

	out, stats, err := Stage3BInferDependencies(context.Background(), llm, facts, cfg)
	if err != nil {
		t.Fatalf("Stage3BInferDependencies error: %v", err)
	}
	if stats.Accepted != 1 {
		t.Fatalf("accepted=%d, want 1", stats.Accepted)
	}
	var child ExtractedFact
	for _, f := range out {
		if f.ID == "f2" {
			child = f
		}
	}
	if child.IsRoot {
		t.Fatalf("child should be derived")
	}
	if len(child.DependsOn) != 1 || child.DependsOn[0] != "f1" {
		t.Fatalf("depends_on=%v, want [f1]", child.DependsOn)
	}
}

func TestStage4CoverageFindsGenuinelyMissedClaim(t *testing.T) {
	llm := &mockLLM{
		stage4Response: `[
			{
				"missed_claim":"Water freezes at 0 degrees Celsius.",
				"suggested_fact":{"id":"water-freezes-0c","subject":"water","predicate":"freezes_at","object":"0°C","claim":"Water freezes at 0 degrees Celsius.","assertion_kind":"empirical","confidence":0.95},
				"confidence":0.9
			}
		]`,
	}
	cfg := DefaultExtractionConfig()
	cfg.EnableCoverageVerification = true

	startFacts := []ExtractedFact{
		{ID: "water-boils-100c", Claim: "Water boils at 100 degrees Celsius.", Subject: "water", Predicate: "boils_at", Object: "100°C", AssertionKind: core.AssertionKindEmpirical, Confidence: 0.95, IsRoot: true},
	}

	out, stats, err := Stage4CoverageVerification(context.Background(), llm, "Water boils at 100 degrees Celsius. Water freezes at 0 degrees Celsius.", startFacts, cfg)
	if err != nil {
		t.Fatalf("Stage4CoverageVerification error: %v", err)
	}
	if stats.MissedClaims != 1 {
		t.Fatalf("missed_claims=%d, want 1", stats.MissedClaims)
	}
	if len(out) != 2 {
		t.Fatalf("facts=%d, want 2", len(out))
	}
	found := false
	for _, f := range out {
		if f.ID == "water-freezes-0c" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected recovered fact water-freezes-0c")
	}
}

func TestStage5ContradictionDetectedBeforeAssertion(t *testing.T) {
	facts := []ExtractedFact{
		{
			ID:            "p1",
			Claim:         "Paris is the capital of France.",
			Subject:       "Paris",
			Predicate:     "is_capital_of",
			Object:        "France",
			Polarity:      "positive",
			IsRoot:        true,
			Confidence:    0.9,
			AssertionKind: core.AssertionKindEmpirical,
		},
		{
			ID:            "p2",
			Claim:         "Paris is the capital of Germany.",
			Subject:       "Paris",
			Predicate:     "is_capital_of",
			Object:        "Germany",
			Polarity:      "positive",
			IsRoot:        true,
			Confidence:    0.9,
			AssertionKind: core.AssertionKindEmpirical,
		},
	}
	issues, err := Stage5ConsistencyPrecheck(facts)
	if err != nil {
		t.Fatalf("Stage5ConsistencyPrecheck error: %v", err)
	}
	if len(issues) == 0 {
		t.Fatalf("expected contradiction issues, got 0")
	}
	found := false
	for _, issue := range issues {
		if issue.Type == "predicate_object_conflict" && len(issue.FactIDs) == 2 {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected predicate_object_conflict, got %+v", issues)
	}
}

func TestFullPipelineEndToEnd(t *testing.T) {
	llm := &mockLLM{
		stage1Response: `[
			{"sentence_index":0,"sentence":"Marie Curie was a scientist.","category":"verifiable","reason":"fact"},
			{"sentence_index":1,"sentence":"She won a prize.","category":"verifiable","reason":"fact"},
			{"sentence_index":2,"sentence":"The prize was the Nobel Prize.","category":"verifiable","reason":"fact"},
			{"sentence_index":3,"sentence":"The Nobel Prize is awarded in Stockholm.","category":"verifiable","reason":"fact"},
			{"sentence_index":4,"sentence":"Therefore, Marie Curie won the Nobel Prize.","category":"verifiable","reason":"derived"}
		]`,
		stage2Response: `[
			{"sentence_index":0,"original":"Marie Curie was a scientist.","decontextualised":"Marie Curie was a scientist.","unresolved_references":[],"confidence":0.95},
			{"sentence_index":1,"original":"She won a prize.","decontextualised":"Marie Curie won a prize.","unresolved_references":[],"confidence":0.95},
			{"sentence_index":2,"original":"The prize was the Nobel Prize.","decontextualised":"The prize Marie Curie won was the Nobel Prize.","unresolved_references":[],"confidence":0.95},
			{"sentence_index":3,"original":"The Nobel Prize is awarded in Stockholm.","decontextualised":"The Nobel Prize is awarded in Stockholm.","unresolved_references":[],"confidence":0.95},
			{"sentence_index":4,"original":"Therefore, Marie Curie won the Nobel Prize.","decontextualised":"Marie Curie won the Nobel Prize.","unresolved_references":[],"confidence":0.95}
		]`,
		stage3ABySentence: map[string]string{
			"Marie Curie was a scientist.": `[{"id":"curie-scientist","subject":"Marie Curie","predicate":"was","object":"scientist","claim":"Marie Curie was a scientist.","assertion_kind":"empirical","confidence":0.9}]`,
			"Marie Curie won a prize.": `[{"id":"curie-won-prize","subject":"Marie Curie","predicate":"won_prize","object":"a prize","claim":"Marie Curie won a prize.","assertion_kind":"empirical","confidence":0.9}]`,
			"The prize Marie Curie won was the Nobel Prize.": `[{"id":"curie-prize-nobel","subject":"the prize Marie Curie won","predicate":"was","object":"the Nobel Prize","claim":"The prize Marie Curie won was the Nobel Prize.","assertion_kind":"empirical","confidence":0.9}]`,
			"The Nobel Prize is awarded in Stockholm.": `[{"id":"nobel-stockholm","subject":"the Nobel Prize","predicate":"is_awarded_in","object":"Stockholm","claim":"The Nobel Prize is awarded in Stockholm.","assertion_kind":"empirical","confidence":0.9}]`,
			"Marie Curie won the Nobel Prize.": `[{"id":"curie-won-nobel","subject":"Marie Curie","predicate":"won","object":"the Nobel Prize","claim":"Marie Curie won the Nobel Prize.","assertion_kind":"empirical","confidence":0.9}]`,
		},
		stage3BResponder: func(parentClaim, childClaim string) (dependencyCheckResponse, bool) {
			if parentClaim == "Marie Curie won a prize." && childClaim == "The prize Marie Curie won was the Nobel Prize." {
				return dependencyCheckResponse{Depends: "yes", Confidence: 0.9, Justification: "identifies the prize"}, true
			}
			if parentClaim == "The prize Marie Curie won was the Nobel Prize." && childClaim == "Marie Curie won the Nobel Prize." {
				return dependencyCheckResponse{Depends: "yes", Confidence: 0.9, Justification: "substitutes prize identity"}, true
			}
			return dependencyCheckResponse{}, false
		},
		stage4Response: `[]`,
	}

	cfg := DefaultExtractionConfig()
	cfg.EnableSelection = true
	cfg.EnableDecontextualisation = true
	cfg.EnableCoverageVerification = true
	cfg.EnableConsistencyPrecheck = true
	cfg.DependencyConfidenceThreshold = 0.65

	doc := strings.Join([]string{
		"Marie Curie was a scientist.",
		"She won a prize.",
		"The prize was the Nobel Prize.",
		"The Nobel Prize is awarded in Stockholm.",
		"Therefore, Marie Curie won the Nobel Prize.",
	}, " ")

	res, err := RunPipeline(context.Background(), llm, doc, "", &cfg)
	if err != nil {
		t.Fatalf("RunPipeline error: %v", err)
	}
	if res == nil || len(res.Facts) == 0 {
		t.Fatalf("expected facts")
	}
	if len(res.PreAssertionContradictions) != 0 {
		t.Fatalf("expected no pre-assertion contradictions, got %v", res.PreAssertionContradictions)
	}

	byID := map[string]ExtractedFact{}
	for _, f := range res.Facts {
		byID[f.ID] = f
	}
	if byID["curie-won-nobel"].IsRoot {
		t.Fatalf("curie-won-nobel should be derived")
	}
	if len(byID["curie-won-nobel"].DependsOn) == 0 {
		t.Fatalf("curie-won-nobel should have depends_on")
	}
}
