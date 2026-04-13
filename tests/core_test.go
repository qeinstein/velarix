package tests

import (
	"fmt"
	"github.com/xeipuuv/gojsonschema"
	"reflect"
	"sync"
	"testing"
	"time"
	"velarix/core"
)

func TestSimpleInvalidationChain(t *testing.T) {
	engine := core.NewEngine()

	// Root A
	a := &core.Fact{
		ID:           "A",
		IsRoot:       true,
		ManualStatus: core.Valid,
	}
	if err := engine.AssertFact(a); err != nil {
		t.Fatal(err)
	}

	// B depends on A
	b := &core.Fact{
		ID: "B",
		JustificationSets: [][]string{
			{"A"},
		},
	}
	if err := engine.AssertFact(b); err != nil {
		t.Fatal(err)
	}

	if engine.GetStatus("B") != core.Valid {
		t.Fatalf("expected B to be valid")
	}

	// Invalidate A
	if err := engine.InvalidateRoot("A"); err != nil {
		t.Fatal(err)
	}

	if engine.GetStatus("B") != core.Invalid {
		t.Fatalf("expected B to be invalid after A invalidation")
	}
}

func TestExpiredFactInvalidatesDescendants(t *testing.T) {
	engine := core.NewEngine()

	root := &core.Fact{
		ID:           "root_expiring",
		IsRoot:       true,
		ManualStatus: core.Valid,
		ValidUntil:   time.Now().Add(100 * time.Millisecond).UnixMilli(),
	}
	if err := engine.AssertFact(root); err != nil {
		t.Fatal(err)
	}

	child := &core.Fact{
		ID: "child_derived",
		JustificationSets: [][]string{
			{"root_expiring"},
		},
	}
	if err := engine.AssertFact(child); err != nil {
		t.Fatal(err)
	}
	if st := engine.GetStatus(child.ID); st < core.ConfidenceThreshold {
		t.Fatalf("expected child to be valid before expiry, got %.4f", st)
	}

	time.Sleep(200 * time.Millisecond)
	engine.SweepExpiredFacts()

	f, ok := engine.GetFact(child.ID)
	if !ok {
		t.Fatalf("missing child fact")
	}
	if f.DerivedStatus >= core.ConfidenceThreshold {
		t.Fatalf("expected child DerivedStatus to be below threshold after expiry, got %.4f", f.DerivedStatus)
	}
}

func TestHypotheticalFactCannotGroundEmpiricalDerived(t *testing.T) {
	engine := core.NewEngine()

	hyp := &core.Fact{
		ID:            "hyp_root",
		IsRoot:        true,
		ManualStatus:  core.Valid,
		AssertionKind: core.AssertionKindHypothetical,
	}
	if err := engine.AssertFact(hyp); err != nil {
		t.Fatal(err)
	}

	derived := &core.Fact{
		ID:            "emp_derived",
		AssertionKind: core.AssertionKindEmpirical,
		JustificationSets: [][]string{
			{"hyp_root"},
		},
	}
	if err := engine.AssertFact(derived); err != nil {
		t.Fatal(err)
	}
	f, ok := engine.GetFact(derived.ID)
	if !ok {
		t.Fatalf("missing derived fact")
	}
	if f.DerivedStatus != core.Invalid {
		t.Fatalf("expected DerivedStatus=0 for empirical derived from hypothetical parent, got %.4f", f.DerivedStatus)
	}
}

func TestGlobalFactFansOutToActiveSessions(t *testing.T) {
	gt := core.NewGlobalTruth()
	e1 := core.NewEngine()
	e2 := core.NewEngine()

	if err := gt.Subscribe("s1", e1); err != nil {
		t.Fatal(err)
	}
	if err := gt.Subscribe("s2", e2); err != nil {
		t.Fatal(err)
	}

	g0 := &core.Fact{ID: "g0", IsRoot: true, ManualStatus: core.Valid}
	if _, err := gt.AssertGlobal(g0); err != nil {
		t.Fatal(err)
	}

	if st := e1.GetStatus("g0"); st < core.ConfidenceThreshold {
		t.Fatalf("expected session s1 to see global fact g0 as valid, got %.4f", st)
	}
	if st := e2.GetStatus("g0"); st < core.ConfidenceThreshold {
		t.Fatalf("expected session s2 to see global fact g0 as valid, got %.4f", st)
	}
}

func TestFrameProblem(t *testing.T) {
	engine := core.NewEngine()

	// Root A
	a := &core.Fact{
		ID:           "A",
		IsRoot:       true,
		ManualStatus: core.Valid,
	}
	engine.AssertFact(a)

	// Root C
	c := &core.Fact{
		ID:           "C",
		IsRoot:       true,
		ManualStatus: core.Valid,
	}
	engine.AssertFact(c)

	// B depends on A OR C
	b := &core.Fact{
		ID: "B",
		JustificationSets: [][]string{
			{"A"},
			{"C"},
		},
	}
	engine.AssertFact(b)

	if engine.GetStatus("B") != core.Valid {
		t.Fatalf("expected B to be valid initially")
	}

	// Invalidate A
	engine.InvalidateRoot("A")

	if engine.GetStatus("B") != core.Valid {
		t.Fatalf("expected B to remain valid due to C")
	}

	// Invalidate C
	engine.InvalidateRoot("C")

	if engine.GetStatus("B") != core.Invalid {
		t.Fatalf("expected B to be invalid after both supports removed")
	}
}

func TestConfidenceThresholdBoundary(t *testing.T) {
	engine := core.NewEngine()

	if err := engine.AssertFact(&core.Fact{
		ID:           "exact_parent",
		IsRoot:       true,
		ManualStatus: core.ConfidenceThreshold,
	}); err != nil {
		t.Fatal(err)
	}
	if err := engine.AssertFact(&core.Fact{
		ID:                "exact_child",
		JustificationSets: [][]string{{"exact_parent"}},
	}); err != nil {
		t.Fatal(err)
	}
	if got := engine.GetStatus("exact_child"); got != core.ConfidenceThreshold {
		t.Fatalf("expected exact_child to pass at threshold %.4f, got %.4f", core.ConfidenceThreshold, got)
	}

	if err := engine.AssertFact(&core.Fact{
		ID:           "below_parent",
		IsRoot:       true,
		ManualStatus: core.Status(0.5999),
	}); err != nil {
		t.Fatal(err)
	}
	if err := engine.AssertFact(&core.Fact{
		ID:                "below_child",
		JustificationSets: [][]string{{"below_parent"}},
	}); err != nil {
		t.Fatal(err)
	}
	if got := engine.GetStatus("below_child"); got != core.Invalid {
		t.Fatalf("expected below_child to fail below threshold, got %.4f", got)
	}
}

func TestDeepDominatorChain(t *testing.T) {
	engine := core.NewEngine()

	// Root Fact 0
	engine.AssertFact(&core.Fact{
		ID:           "Fact0",
		IsRoot:       true,
		ManualStatus: core.Valid,
	})

	// Chain of 100 facts
	for i := 1; i <= 100; i++ {
		engine.AssertFact(&core.Fact{
			ID: fmt.Sprintf("Fact%d", i),
			JustificationSets: [][]string{
				{fmt.Sprintf("Fact%d", i-1)},
			},
		})
	}

	if engine.GetStatus("Fact100") != core.Valid {
		t.Fatalf("expected Fact100 to be valid")
	}

	// Invalidate the root
	engine.InvalidateRoot("Fact0")

	// Even if propagation were slow, the Dominator Tree check
	// would identify the collapse.
	if engine.GetStatus("Fact100") != core.Invalid {
		t.Fatalf("expected Fact100 to be invalid via dominator collapse")
	}
}

func TestDiamondDependencyDominator(t *testing.T) {
	engine := core.NewEngine()

	if err := engine.AssertFact(&core.Fact{ID: "A", IsRoot: true, ManualStatus: core.Valid}); err != nil {
		t.Fatal(err)
	}
	if err := engine.AssertFact(&core.Fact{ID: "B", JustificationSets: [][]string{{"A"}}}); err != nil {
		t.Fatal(err)
	}
	if err := engine.AssertFact(&core.Fact{ID: "C", JustificationSets: [][]string{{"A"}}}); err != nil {
		t.Fatal(err)
	}
	if err := engine.AssertFact(&core.Fact{ID: "D", JustificationSets: [][]string{{"B"}, {"C"}}}); err != nil {
		t.Fatal(err)
	}

	if got := engine.GetStatus("D"); got != core.Valid {
		t.Fatalf("expected D to be valid, got %.2f", got)
	}
	fact, ok := engine.GetFact("D")
	if !ok {
		t.Fatal("expected to retrieve D")
	}
	if fact.IDom != "A" {
		t.Fatalf("expected D.IDom to resolve to A, got %q", fact.IDom)
	}
}

func TestGlobalRevalidation(t *testing.T) {
	// We'll test this via the API logic in a real integration test later,
	// but here we can simulate the "Clear and Re-Assert" logic.
	engine := core.NewEngine()

	// 1. Assert a fact that will later fail schema
	f1 := &core.Fact{ID: "F1", IsRoot: true, ManualStatus: core.Valid, Payload: map[string]interface{}{"age": "invalid_string"}}
	engine.AssertFact(f1)

	if engine.GetStatus("F1") != core.Valid {
		t.Fatal("expected F1 to be valid initially")
	}

	// 2. Simulate "Revalidation" with a strict schema (age must be int)
	// In the real server, this happens by clearing and re-playing from Journal.
	engine2 := core.NewEngine()

	// Logic from handleRevalidate:
	passed := 0
	violations := 0

	history := []*core.Fact{f1}
	schema := `{"type": "object", "properties": {"age": {"type": "integer"}}}`

	for _, f := range history {
		loader := gojsonschema.NewStringLoader(schema)
		doc := gojsonschema.NewGoLoader(f.Payload)
		res, _ := gojsonschema.Validate(loader, doc)

		if !res.Valid() {
			violations++
			continue
		}
		engine2.AssertFact(f)
		passed++
	}

	if violations != 1 {
		t.Fatalf("expected 1 violation due to schema change, got %d", violations)
	}
	if passed != 0 {
		t.Fatal("expected 0 passed facts")
	}
}

func TestSnapshotCorruption(t *testing.T) {
	engine := core.NewEngine()
	engine.AssertFact(&core.Fact{ID: "S1", IsRoot: true, ManualStatus: core.Valid})

	snap, err := engine.ToSnapshot()
	if err != nil {
		t.Fatalf("failed to create snapshot: %v", err)
	}

	// Corrupt the checksum
	snap.Checksum = snap.Checksum + 1

	engine2 := core.NewEngine()
	err = engine2.FromSnapshot(snap)
	if err == nil {
		t.Fatal("expected snapshot recovery to fail due to checksum mismatch")
	}
	if err.Error() != "snapshot integrity check failed: checksum mismatch" {
		t.Fatalf("unexpected error message: %v", err)
	}
}

func TestRetractFactInvalidatesChildren(t *testing.T) {
	engine := core.NewEngine()

	if err := engine.AssertFact(&core.Fact{ID: "root", IsRoot: true, ManualStatus: core.Valid}); err != nil {
		t.Fatal(err)
	}
	if err := engine.AssertFact(&core.Fact{ID: "derived", JustificationSets: [][]string{{"root"}}}); err != nil {
		t.Fatal(err)
	}
	if engine.GetStatus("derived") != core.Valid {
		t.Fatalf("expected derived to start valid")
	}

	if err := engine.RetractFact("root", "contradicted"); err != nil {
		t.Fatal(err)
	}
	if engine.GetStatus("root") != core.Invalid {
		t.Fatalf("expected retracted root to be invalid")
	}
	if engine.GetStatus("derived") != core.Invalid {
		t.Fatalf("expected child fact to be invalid after root retraction")
	}
}

func TestRetractFactInvalidatesTransitiveDescendants(t *testing.T) {
	engine := core.NewEngine()

	if err := engine.AssertFact(&core.Fact{ID: "chain_root", IsRoot: true, ManualStatus: core.Valid}); err != nil {
		t.Fatal(err)
	}
	if err := engine.AssertFact(&core.Fact{ID: "chain_1", JustificationSets: [][]string{{"chain_root"}}}); err != nil {
		t.Fatal(err)
	}
	if err := engine.AssertFact(&core.Fact{ID: "chain_2", JustificationSets: [][]string{{"chain_1"}}}); err != nil {
		t.Fatal(err)
	}
	if err := engine.AssertFact(&core.Fact{ID: "chain_3", JustificationSets: [][]string{{"chain_2"}}}); err != nil {
		t.Fatal(err)
	}

	if err := engine.RetractFact("chain_root", "retracted for test"); err != nil {
		t.Fatal(err)
	}
	for _, factID := range []string{"chain_1", "chain_2", "chain_3"} {
		if got := engine.GetStatus(factID); got != core.Invalid {
			t.Fatalf("expected %s to be invalid after transitive retraction, got %.2f", factID, got)
		}
	}
}

func TestNegativeDependencyBecomesValidWhenRiskIsInvalidated(t *testing.T) {
	engine := core.NewEngine()

	if err := engine.AssertFact(&core.Fact{ID: "policy_ready", IsRoot: true, ManualStatus: core.Valid}); err != nil {
		t.Fatal(err)
	}
	if err := engine.AssertFact(&core.Fact{ID: "risk_flag", IsRoot: true, ManualStatus: core.Valid}); err != nil {
		t.Fatal(err)
	}
	if err := engine.AssertFact(&core.Fact{
		ID:                "safe_to_execute",
		JustificationSets: [][]string{{"policy_ready", "!risk_flag"}},
	}); err != nil {
		t.Fatal(err)
	}

	if got := engine.GetStatus("safe_to_execute"); got != core.Invalid {
		t.Fatalf("expected safe_to_execute to start invalid while risk is present, got %.2f", got)
	}

	if err := engine.InvalidateRoot("risk_flag"); err != nil {
		t.Fatal(err)
	}
	if got := engine.GetStatus("safe_to_execute"); got != core.Valid {
		t.Fatalf("expected safe_to_execute to become valid after risk invalidation, got %.2f", got)
	}
}

func TestNegativeDependencyFailsAtThresholdBoundary(t *testing.T) {
	engine := core.NewEngine()

	if err := engine.AssertFact(&core.Fact{ID: "policy_gate", IsRoot: true, ManualStatus: core.Valid}); err != nil {
		t.Fatal(err)
	}
	if err := engine.AssertFact(&core.Fact{ID: "risk_boundary", IsRoot: true, ManualStatus: core.ConfidenceThreshold}); err != nil {
		t.Fatal(err)
	}
	if err := engine.AssertFact(&core.Fact{
		ID:                "blocked_by_boundary",
		JustificationSets: [][]string{{"policy_gate", "!risk_boundary"}},
	}); err != nil {
		t.Fatal(err)
	}

	if got := engine.GetStatus("blocked_by_boundary"); got != core.Invalid {
		t.Fatalf("expected negative dependency to fail at threshold boundary, got %.2f", got)
	}
}

func TestConsistencyCheckDetectsClaimConflict(t *testing.T) {
	engine := core.NewEngine()

	engine.AssertFact(&core.Fact{
		ID:           "claim_a",
		IsRoot:       true,
		ManualStatus: core.Valid,
		Payload: map[string]interface{}{
			"claim_key":   "ticket_status",
			"claim_value": "open",
		},
	})
	engine.AssertFact(&core.Fact{
		ID:           "claim_b",
		IsRoot:       true,
		ManualStatus: core.Valid,
		Payload: map[string]interface{}{
			"claim_key":   "ticket_status",
			"claim_value": "closed",
		},
	})

	report := engine.CheckConsistency([]string{"claim_a", "claim_b"}, false)
	if report.IssueCount != 1 {
		t.Fatalf("expected exactly one contradiction, got %+v", report)
	}
	if report.Issues[0].Type != "claim_value_conflict" {
		t.Fatalf("expected claim_value_conflict, got %+v", report.Issues[0])
	}
}

func TestSemanticSearchFindsRelevantFact(t *testing.T) {
	engine := core.NewEngine()

	engine.AssertFact(&core.Fact{
		ID:           "apple_fact",
		IsRoot:       true,
		ManualStatus: core.Valid,
		Payload:      map[string]interface{}{"text": "red apple on the kitchen table"},
	})
	engine.AssertFact(&core.Fact{
		ID:           "car_fact",
		IsRoot:       true,
		ManualStatus: core.Valid,
		Payload:      map[string]interface{}{"text": "blue sedan in the garage"},
	})

	results := engine.SearchSimilarFacts("apple", 2, true)
	if len(results) == 0 {
		t.Fatalf("expected at least one semantic search result")
	}
	if results[0].FactID != "apple_fact" {
		t.Fatalf("expected apple_fact to rank first, got %+v", results)
	}
}

func TestGetFactAndListFactsReturnDeepCopies(t *testing.T) {
	engine := core.NewEngine()

	if err := engine.AssertFact(&core.Fact{
		ID:           "root_copy_parent",
		IsRoot:       true,
		ManualStatus: core.Valid,
	}); err != nil {
		t.Fatal(err)
	}

	originalPayload := map[string]interface{}{
		"text": "original",
		"nested": map[string]interface{}{
			"value": "unchanged",
		},
	}
	originalMetadata := map[string]interface{}{
		"source": "original",
		"nested": map[string]interface{}{
			"tag": "stable",
		},
	}
	originalEmbedding := []float64{0.1, 0.2, 0.3}
	originalSets := [][]string{{"root_copy_parent"}}
	originalValidation := []string{"original"}

	if err := engine.AssertFact(&core.Fact{
		ID:                "copy_target",
		Payload:           originalPayload,
		Metadata:          originalMetadata,
		Embedding:         originalEmbedding,
		JustificationSets: originalSets,
		ValidationErrors:  originalValidation,
	}); err != nil {
		t.Fatal(err)
	}

	got, ok := engine.GetFact("copy_target")
	if !ok {
		t.Fatal("expected GetFact to find copy_target")
	}
	got.Payload["text"] = "mutated"
	got.Payload["nested"].(map[string]interface{})["value"] = "mutated"
	got.Metadata["source"] = "mutated"
	got.Metadata["nested"].(map[string]interface{})["tag"] = "mutated"
	got.Embedding[0] = 9.9
	got.JustificationSets[0][0] = "mutated_parent"
	got.ValidationErrors[0] = "mutated"

	internal, ok := engine.GetFact("copy_target")
	if !ok {
		t.Fatal("expected GetFact to still find copy_target")
	}
	if !reflect.DeepEqual(internal.Payload, originalPayload) {
		t.Fatalf("expected internal payload to remain unchanged, got %+v", internal.Payload)
	}
	if !reflect.DeepEqual(internal.Metadata, originalMetadata) {
		t.Fatalf("expected internal metadata to remain unchanged, got %+v", internal.Metadata)
	}
	if !reflect.DeepEqual(internal.Embedding, originalEmbedding) {
		t.Fatalf("expected internal embedding to remain unchanged, got %+v", internal.Embedding)
	}
	if !reflect.DeepEqual(internal.JustificationSets, originalSets) {
		t.Fatalf("expected internal justification sets to remain unchanged, got %+v", internal.JustificationSets)
	}
	if !reflect.DeepEqual(internal.ValidationErrors, originalValidation) {
		t.Fatalf("expected internal validation errors to remain unchanged, got %+v", internal.ValidationErrors)
	}

	listed := engine.ListFacts()
	var listedFact *core.Fact
	for _, fact := range listed {
		if fact.ID == "copy_target" {
			listedFact = fact
			break
		}
	}
	if listedFact == nil {
		t.Fatal("expected ListFacts to include copy_target")
	}
	listedFact.Payload["text"] = "mutated_from_list"
	listedFact.Metadata["source"] = "mutated_from_list"
	listedFact.Embedding[1] = 8.8
	listedFact.JustificationSets[0][0] = "mutated_from_list"

	internal, ok = engine.GetFact("copy_target")
	if !ok {
		t.Fatal("expected GetFact to still find copy_target after ListFacts mutation")
	}
	if !reflect.DeepEqual(internal.Payload, originalPayload) {
		t.Fatalf("expected internal payload to remain unchanged after ListFacts mutation, got %+v", internal.Payload)
	}
	if !reflect.DeepEqual(internal.Metadata, originalMetadata) {
		t.Fatalf("expected internal metadata to remain unchanged after ListFacts mutation, got %+v", internal.Metadata)
	}
	if !reflect.DeepEqual(internal.Embedding, originalEmbedding) {
		t.Fatalf("expected internal embedding to remain unchanged after ListFacts mutation, got %+v", internal.Embedding)
	}
	if !reflect.DeepEqual(internal.JustificationSets, originalSets) {
		t.Fatalf("expected internal justification sets to remain unchanged after ListFacts mutation, got %+v", internal.JustificationSets)
	}
}

// TestExplainReasoningCausalChain verifies that ExplainReasoning returns a
// causal_chain containing exactly the two root parent facts for a derived fact
// that depends on both.
func TestExplainReasoningCausalChain(t *testing.T) {
	engine := core.NewEngine()

	if err := engine.AssertFact(&core.Fact{
		ID:           "root_a",
		IsRoot:       true,
		ManualStatus: core.Valid,
	}); err != nil {
		t.Fatal("asserting root_a:", err)
	}
	if err := engine.AssertFact(&core.Fact{
		ID:           "root_b",
		IsRoot:       true,
		ManualStatus: core.Valid,
	}); err != nil {
		t.Fatal("asserting root_b:", err)
	}
	// derived_c depends on both roots in a single AND-set.
	if err := engine.AssertFact(&core.Fact{
		ID:                "derived_c",
		JustificationSets: [][]string{{"root_a", "root_b"}},
	}); err != nil {
		t.Fatal("asserting derived_c:", err)
	}

	if engine.GetStatus("derived_c") != core.Valid {
		t.Fatal("expected derived_c to be valid")
	}

	output, err := engine.ExplainReasoning("derived_c")
	if err != nil {
		t.Fatal("ExplainReasoning:", err)
	}
	if output == nil {
		t.Fatal("expected non-nil ExplanationOutput")
	}
	if len(output.CausalChain) != 3 {
		t.Fatalf("expected causal_chain to contain exactly 3 facts (derived_c + 2 roots), got %d", len(output.CausalChain))
	}

	// The first entry in the chain is always the target fact itself.
	if output.CausalChain[0].FactID != "derived_c" {
		t.Fatalf("expected first causal_chain entry to be derived_c, got %s", output.CausalChain[0].FactID)
	}

	// Verify both roots appear as parents on the derived fact's entry.
	parentIDs := map[string]bool{}
	for _, p := range output.CausalChain[0].Parents {
		parentIDs[p] = true
	}
	if !parentIDs["root_a"] {
		t.Errorf("expected root_a in derived_c parents, got %v", output.CausalChain[0].Parents)
	}
	if !parentIDs["root_b"] {
		t.Errorf("expected root_b in derived_c parents, got %v", output.CausalChain[0].Parents)
	}

	// Both roots must appear somewhere in the chain.
	chainIDs := map[string]bool{}
	for _, b := range output.CausalChain {
		chainIDs[b.FactID] = true
	}
	if !chainIDs["root_a"] || !chainIDs["root_b"] {
		t.Fatalf("expected root_a and root_b in causal chain, got %v", chainIDs)
	}
}

func TestReasoningAuditMarksEarlierContradictionForRetraction(t *testing.T) {
	engine := core.NewEngine()

	engine.AssertFact(&core.Fact{
		ID:           "weather_sunny",
		IsRoot:       true,
		ManualStatus: core.Valid,
		Payload: map[string]interface{}{
			"claim_key":   "weather_outlook",
			"claim_value": "sunny",
		},
	})
	engine.AssertFact(&core.Fact{
		ID:           "weather_rainy",
		IsRoot:       true,
		ManualStatus: core.Valid,
		Payload: map[string]interface{}{
			"claim_key":   "weather_outlook",
			"claim_value": "rainy",
		},
	})

	report := engine.AuditReasoningChain(&core.ReasoningChain{
		ChainID: "chain_weather",
		Steps: []core.ReasoningStep{
			{ID: "step_1", Content: "initial weather belief", OutputFactID: "weather_sunny"},
			{ID: "step_2", Content: "later contradictory weather belief", OutputFactID: "weather_rainy"},
		},
	})

	if report.Valid {
		t.Fatalf("expected reasoning audit to fail on contradiction")
	}
	found := false
	for _, factID := range report.RetractCandidateFactIDs {
		if factID == "weather_sunny" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected earlier output fact to be marked for retraction, got %+v", report)
	}
}

func TestAssertFactRejectsTwoNodeAndMultiNodeCycleAttempts(t *testing.T) {
	twoNode := core.NewEngine()
	if err := twoNode.AssertFact(&core.Fact{ID: "A", IsRoot: true, ManualStatus: core.Valid}); err != nil {
		t.Fatal(err)
	}
	if err := twoNode.AssertFact(&core.Fact{ID: "B", JustificationSets: [][]string{{"A"}}}); err != nil {
		t.Fatal(err)
	}
	if err := twoNode.AssertFact(&core.Fact{ID: "A", JustificationSets: [][]string{{"B"}}}); err == nil {
		t.Fatal("expected two-node cycle attempt to be rejected")
	}
	if len(twoNode.ListFacts()) != 2 {
		t.Fatalf("expected two-node graph to remain unchanged, got %d facts", len(twoNode.ListFacts()))
	}
	if got := twoNode.GetStatus("B"); got != core.Valid {
		t.Fatalf("expected B to remain valid after rejected two-node cycle attempt, got %.2f", got)
	}

	multiNode := core.NewEngine()
	if err := multiNode.AssertFact(&core.Fact{ID: "X", IsRoot: true, ManualStatus: core.Valid}); err != nil {
		t.Fatal(err)
	}
	if err := multiNode.AssertFact(&core.Fact{ID: "Y", JustificationSets: [][]string{{"X"}}}); err != nil {
		t.Fatal(err)
	}
	if err := multiNode.AssertFact(&core.Fact{ID: "Z", JustificationSets: [][]string{{"Y"}}}); err != nil {
		t.Fatal(err)
	}
	if err := multiNode.AssertFact(&core.Fact{ID: "X", JustificationSets: [][]string{{"Z"}}}); err == nil {
		t.Fatal("expected multi-node cycle attempt to be rejected")
	}
	if len(multiNode.ListFacts()) != 3 {
		t.Fatalf("expected multi-node graph to remain unchanged, got %d facts", len(multiNode.ListFacts()))
	}
	if got := multiNode.GetStatus("Z"); got != core.Valid {
		t.Fatalf("expected Z to remain valid after rejected multi-node cycle attempt, got %.2f", got)
	}
}

func TestSetFactReviewIncrementsMutationCount(t *testing.T) {
	engine := core.NewEngine()

	if err := engine.AssertFact(&core.Fact{ID: "review_target", IsRoot: true, ManualStatus: core.Valid}); err != nil {
		t.Fatal(err)
	}
	before := engine.MutationCount

	if err := engine.SetFactReview("review_target", core.ReviewApproved, "approved by test", 12345); err != nil {
		t.Fatal(err)
	}
	if got := engine.MutationCount; got != before+1 {
		t.Fatalf("expected MutationCount to increment after review, got %d want %d", got, before+1)
	}

	if err := engine.SetFactReview("review_target", core.ReviewApproved, "same status should be ignored", 67890); err != nil {
		t.Fatal(err)
	}
	if got := engine.MutationCount; got != before+1 {
		t.Fatalf("expected idempotent review status to leave MutationCount unchanged, got %d want %d", got, before+1)
	}
}

func TestMemoryCapConcurrentAssertFact(t *testing.T) {
	engine := core.NewEngine()

	for i := 0; i < core.MaxFactsPerSession-5; i++ {
		if err := engine.AssertFact(&core.Fact{
			ID:           fmt.Sprintf("seed_%d", i),
			IsRoot:       true,
			ManualStatus: core.Valid,
		}); err != nil {
			t.Fatalf("failed seeding fact %d: %v", i, err)
		}
	}

	var wg sync.WaitGroup
	results := make(chan error, 10)
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			results <- engine.AssertFact(&core.Fact{
				ID:           fmt.Sprintf("concurrent_%d", i),
				IsRoot:       true,
				ManualStatus: core.Valid,
			})
		}(i)
	}
	wg.Wait()
	close(results)

	successes := 0
	failures := 0
	for err := range results {
		if err == nil {
			successes++
			continue
		}
		failures++
	}

	if successes != 5 {
		t.Fatalf("expected exactly 5 concurrent assertions to succeed, got %d successes and %d failures", successes, failures)
	}
	if got := len(engine.ListFacts()); got != core.MaxFactsPerSession {
		t.Fatalf("expected final fact count to cap at %d, got %d", core.MaxFactsPerSession, got)
	}
}
