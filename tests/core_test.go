package tests

import (
	"fmt"
	"github.com/xeipuuv/gojsonschema"
	"testing"
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
