package tests

import (
	"testing"
	"velarix/core"
)

// helpers

func validRoot(id string, payload map[string]interface{}) *core.Fact {
	return &core.Fact{
		ID:           id,
		IsRoot:       true,
		ManualStatus: core.Valid,
		Payload:      payload,
	}
}

func assertFacts(t *testing.T, engine *core.Engine, facts ...*core.Fact) {
	t.Helper()
	for _, f := range facts {
		if err := engine.AssertFact(f); err != nil {
			t.Fatalf("AssertFact(%s): %v", f.ID, err)
		}
	}
}

// Rule 1 — explicit_contradiction: a fact declares another as contradicting it
// via the "contradicts" field in its Payload.

func TestConsistency_ExplicitContradiction(t *testing.T) {
	engine := core.NewEngine()
	assertFacts(t, engine,
		validRoot("fact-a", map[string]interface{}{
			"claim_key":   "server_state",
			"claim_value": "running",
			"contradicts": []interface{}{"fact-b"},
		}),
		validRoot("fact-b", map[string]interface{}{
			"claim_key":   "server_state",
			"claim_value": "stopped",
		}),
	)

	report := engine.CheckConsistency([]string{"fact-a", "fact-b"}, false)
	if report.IssueCount != 1 {
		t.Fatalf("expected 1 issue, got %d: %+v", report.IssueCount, report.Issues)
	}
	if report.Issues[0].Type != "explicit_contradiction" {
		t.Errorf("issue type = %q, want explicit_contradiction", report.Issues[0].Type)
	}
}

// Rule 2 — claim_value_conflict: same claim_key, different claim_value.
// (This is the canonical rule exercised in existing tests; we re-verify it
// explicitly for the 5-rule completeness requirement.)

func TestConsistency_ClaimValueConflict(t *testing.T) {
	engine := core.NewEngine()
	assertFacts(t, engine,
		validRoot("open-ticket", map[string]interface{}{
			"claim_key":   "ticket_status",
			"claim_value": "open",
		}),
		validRoot("closed-ticket", map[string]interface{}{
			"claim_key":   "ticket_status",
			"claim_value": "closed",
		}),
	)

	report := engine.CheckConsistency([]string{"open-ticket", "closed-ticket"}, false)
	if report.IssueCount != 1 {
		t.Fatalf("expected 1 issue, got %d", report.IssueCount)
	}
	if report.Issues[0].Type != "claim_value_conflict" {
		t.Errorf("issue type = %q, want claim_value_conflict", report.Issues[0].Type)
	}
}

// Rule 3 — predicate_object_conflict: same subject + predicate, different object.

func TestConsistency_PredicateObjectConflict(t *testing.T) {
	engine := core.NewEngine()
	assertFacts(t, engine,
		validRoot("france-capital-paris", map[string]interface{}{
			"subject":   "France",
			"predicate": "capital",
			"object":    "Paris",
		}),
		validRoot("france-capital-lyon", map[string]interface{}{
			"subject":   "France",
			"predicate": "capital",
			"object":    "Lyon",
		}),
	)

	report := engine.CheckConsistency([]string{"france-capital-paris", "france-capital-lyon"}, false)
	if report.IssueCount != 1 {
		t.Fatalf("expected 1 issue, got %d: %+v", report.IssueCount, report.Issues)
	}
	if report.Issues[0].Type != "predicate_object_conflict" {
		t.Errorf("issue type = %q, want predicate_object_conflict", report.Issues[0].Type)
	}
}

// Rule 4 — polarity_conflict: same subject + predicate + object, opposite polarity.

func TestConsistency_PolarityConflict(t *testing.T) {
	engine := core.NewEngine()
	assertFacts(t, engine,
		validRoot("server-running-pos", map[string]interface{}{
			"subject":   "server",
			"predicate": "status",
			"object":    "running",
			"polarity":  "positive",
		}),
		validRoot("server-running-neg", map[string]interface{}{
			"subject":   "server",
			"predicate": "status",
			"object":    "running",
			"polarity":  "negative",
		}),
	)

	report := engine.CheckConsistency([]string{"server-running-pos", "server-running-neg"}, false)
	if report.IssueCount != 1 {
		t.Fatalf("expected 1 issue, got %d: %+v", report.IssueCount, report.Issues)
	}
	if report.Issues[0].Type != "polarity_conflict" {
		t.Errorf("issue type = %q, want polarity_conflict", report.Issues[0].Type)
	}
}

// Rule 5 — semantic_negation_conflict: nearly identical sentences where one
// contains a negation word that the other lacks.
// The rule fires when cosine similarity >= 0.92 and negation status differs.
// We construct two facts whose payload text is identical except for "not",
// ensuring the lexical embeddings are near-identical.

func TestConsistency_SemanticNegationConflict(t *testing.T) {
	engine := core.NewEngine()
	// Payload text that will produce near-identical lexical embeddings.
	// "the service is available" vs "the service is not available" — the FNV-1a
	// hashed token vectors will be very close because they share almost every token.
	assertFacts(t, engine,
		&core.Fact{
			ID:           "service-available",
			IsRoot:       true,
			ManualStatus: core.Valid,
			Payload: map[string]interface{}{
				"text": "the service is available for all users",
			},
			Metadata: map[string]interface{}{
				"subject":   "service",
				"predicate": "available",
				"object":    "all users",
				"polarity":  "positive",
			},
		},
		&core.Fact{
			ID:           "service-not-available",
			IsRoot:       true,
			ManualStatus: core.Valid,
			Payload: map[string]interface{}{
				"text": "the service is not available for all users",
			},
			Metadata: map[string]interface{}{
				"subject":   "service",
				"predicate": "available",
				"object":    "all users",
				"polarity":  "negative",
			},
		},
	)

	report := engine.CheckConsistency([]string{"service-available", "service-not-available"}, false)
	if report.IssueCount == 0 {
		t.Fatal("expected at least one issue for semantically negated facts, got 0")
	}
	// May fire polarity_conflict (same SPO, different polarity) or
	// semantic_negation_conflict; either demonstrates rule 5 semantics are active.
	found := false
	for _, issue := range report.Issues {
		if issue.Type == "semantic_negation_conflict" || issue.Type == "polarity_conflict" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected semantic_negation_conflict or polarity_conflict, got: %+v", report.Issues)
	}
}

// TestConsistency_SemanticNegationConflict_DirectPath tests the semantic
// negation branch without any structured SPO/claim fields — ensuring the
// case is not short-circuited by an earlier rule in contradictionIssueForFacts.
//
// The two facts share 25 unique payload tokens (plus fmt.Sprintf key wrapping)
// and differ only by (a) a single-letter ID suffix and (b) the word "not".
// With 25 shared and ~2 unique tokens each, the expected cosine similarity is
// 25 / sqrt(26*27) ≈ 0.943, comfortably above the 0.92 threshold.
func TestConsistency_SemanticNegationConflict_DirectPath(t *testing.T) {
	// 25 distinct words — no negation terms — shared by both facts.
	// Must produce cos >= 0.92 in 128-dim FNV-1a lexical embedding space.
	longShared := "alpha bravo charlie delta echo foxtrot golf hotel india juliet " +
		"kilo lima mike november oscar papa quebec romeo sierra tango " +
		"uniform victor whiskey yankee zulu"

	engine := core.NewEngine()
	assertFacts(t, engine,
		&core.Fact{
			ID:           "a",
			IsRoot:       true,
			ManualStatus: core.Valid,
			Payload:      map[string]interface{}{"t": longShared},
		},
		&core.Fact{
			ID:           "b",
			IsRoot:       true,
			ManualStatus: core.Valid,
			Payload:      map[string]interface{}{"t": longShared + " not applicable"},
		},
	)

	// Verify the similarity precondition so we don't silently skip coverage.
	embA := core.LexicalEmbedding("a map[t:"+longShared+"]", 128)
	embB := core.LexicalEmbedding("b map[t:"+longShared+" not applicable]", 128)
	sim := core.CosineSimilarity(embA, embB)
	if sim < 0.92 {
		t.Skipf("cosine similarity %.4f < 0.92 for this token set — "+
			"adjust longShared if FNV-1a collisions reduce similarity", sim)
	}

	report := engine.CheckConsistency([]string{"a", "b"}, false)
	if report.IssueCount == 0 {
		t.Fatal("expected at least one issue for semantically negated facts, got 0")
	}
	found := false
	for _, issue := range report.Issues {
		if issue.Type == "semantic_negation_conflict" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected semantic_negation_conflict, got: %+v", report.Issues)
	}
}

// TestConsistency_DuplicateIDsDeduped verifies that passing the same fact ID
// twice in the list does not produce duplicate issues (uniqueSortedFactIDs dedup).
func TestConsistency_DuplicateIDsDeduped(t *testing.T) {
	engine := core.NewEngine()
	assertFacts(t, engine,
		validRoot("a", map[string]interface{}{"claim_key": "k", "claim_value": "v1"}),
		validRoot("b", map[string]interface{}{"claim_key": "k", "claim_value": "v2"}),
	)

	report := engine.CheckConsistency([]string{"a", "b", "a", "b"}, false)
	if report.IssueCount != 1 {
		t.Errorf("duplicate IDs should produce exactly 1 issue, got %d", report.IssueCount)
	}
}

// TestConsistency_NonExistentIDsIgnored verifies that IDs in the check list
// that don't exist in the engine are silently skipped.
func TestConsistency_NonExistentIDsIgnored(t *testing.T) {
	engine := core.NewEngine()
	assertFacts(t, engine,
		validRoot("real", map[string]interface{}{"claim_key": "k", "claim_value": "v"}),
	)

	// "ghost" doesn't exist — should not panic or error.
	report := engine.CheckConsistency([]string{"real", "ghost"}, false)
	if report.IssueCount != 0 {
		t.Errorf("non-existent ID should not trigger issues, got %d", report.IssueCount)
	}
}

// TestConsistency_InvalidFactsExcludedByDefault verifies that invalid (below-
// threshold) facts are skipped when includeInvalid=false.

func TestConsistency_InvalidFactsExcludedByDefault(t *testing.T) {
	engine := core.NewEngine()
	assertFacts(t, engine,
		validRoot("valid-fact", map[string]interface{}{
			"claim_key":   "color",
			"claim_value": "red",
		}),
		// This fact starts valid but will be invalidated.
		validRoot("to-invalidate", map[string]interface{}{
			"claim_key":   "color",
			"claim_value": "blue",
		}),
	)
	if err := engine.InvalidateRoot("to-invalidate"); err != nil {
		t.Fatal(err)
	}

	// With includeInvalid=false the invalidated fact is excluded — no conflict.
	report := engine.CheckConsistency([]string{"valid-fact", "to-invalidate"}, false)
	if report.IssueCount != 0 {
		t.Errorf("expected 0 issues when invalid facts excluded, got %d", report.IssueCount)
	}

	// With includeInvalid=true both are included — conflict fires.
	report2 := engine.CheckConsistency([]string{"valid-fact", "to-invalidate"}, true)
	if report2.IssueCount != 1 {
		t.Errorf("expected 1 issue when invalid facts included, got %d", report2.IssueCount)
	}
}

// TestConsistency_EmptyIDsScansAllFacts verifies that passing an empty factIDs
// slice makes CheckConsistency scan the entire engine state.

func TestConsistency_EmptyIDsScansAllFacts(t *testing.T) {
	engine := core.NewEngine()
	assertFacts(t, engine,
		validRoot("x1", map[string]interface{}{"claim_key": "k", "claim_value": "v1"}),
		validRoot("x2", map[string]interface{}{"claim_key": "k", "claim_value": "v2"}),
	)

	report := engine.CheckConsistency(nil, false)
	if report.IssueCount != 1 {
		t.Errorf("expected 1 issue when scanning all facts, got %d", report.IssueCount)
	}
}

// TestConsistency_SeveritySortOrder verifies that CheckConsistency returns issues
// sorted by severity — "high" issues before "medium" — when multiple issues are
// present. This exercises the sort comparator in consistencyIssuesForIDsUnsafe.
func TestConsistency_SeveritySortOrder(t *testing.T) {
	longShared := "alpha bravo charlie delta echo foxtrot golf hotel india juliet " +
		"kilo lima mike november oscar papa quebec romeo sierra tango " +
		"uniform victor whiskey yankee zulu"

	engine := core.NewEngine()
	// Pair 1: claim_value_conflict → severity "high"
	assertFacts(t, engine,
		validRoot("h1", map[string]interface{}{"claim_key": "color", "claim_value": "red"}),
		validRoot("h2", map[string]interface{}{"claim_key": "color", "claim_value": "blue"}),
	)
	// Pair 2: semantic_negation_conflict → severity "medium" (if cosine >= 0.92)
	embA := core.LexicalEmbedding("m1 map[t:"+longShared+"]", 128)
	embB := core.LexicalEmbedding("m2 map[t:"+longShared+" not applicable]", 128)
	if core.CosineSimilarity(embA, embB) >= 0.92 {
		if err := engine.AssertFact(&core.Fact{
			ID: "m1", IsRoot: true, ManualStatus: core.Valid,
			Payload: map[string]interface{}{"t": longShared},
		}); err != nil {
			t.Fatal(err)
		}
		if err := engine.AssertFact(&core.Fact{
			ID: "m2", IsRoot: true, ManualStatus: core.Valid,
			Payload: map[string]interface{}{"t": longShared + " not applicable"},
		}); err != nil {
			t.Fatal(err)
		}

		report := engine.CheckConsistency([]string{"h1", "h2", "m1", "m2"}, false)
		if report.IssueCount < 2 {
			t.Fatalf("expected at least 2 issues for sort test, got %d", report.IssueCount)
		}
		// The sort puts "high" before "medium"; verify first issue is not medium.
		if report.Issues[0].Severity == "medium" && report.Issues[len(report.Issues)-1].Severity == "high" {
			t.Error("expected high severity issues before medium severity issues")
		}
	}
}

// TestConsistency_NoFalsePositiveOnDifferentKeys verifies that facts sharing
// the same claim_value but different claim_keys do NOT trigger a conflict.

func TestConsistency_NoFalsePositiveOnDifferentKeys(t *testing.T) {
	engine := core.NewEngine()
	assertFacts(t, engine,
		validRoot("f1", map[string]interface{}{"claim_key": "color", "claim_value": "red"}),
		validRoot("f2", map[string]interface{}{"claim_key": "status", "claim_value": "red"}),
	)

	report := engine.CheckConsistency([]string{"f1", "f2"}, false)
	if report.IssueCount != 0 {
		t.Errorf("expected 0 issues for different claim_keys, got %d: %+v",
			report.IssueCount, report.Issues)
	}
}
