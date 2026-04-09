package tests

import (
	"testing"
	"velarix/core"
)

func TestComplexClinicalContraindication(t *testing.T) {
	engine := core.NewEngine()

	// 1. Patient has a reported Penicillin Allergy (Root Fact)
	engine.AssertFact(&core.Fact{
		ID:           "allergy_penicillin",
		IsRoot:       true,
		ManualStatus: core.Valid,
		Payload:      map[string]interface{}{"severity": "high"},
	})

	// 2. Patient has a confirmed Bacterial Infection (Root Fact)
	engine.AssertFact(&core.Fact{
		ID:           "infection_bacterial",
		IsRoot:       true,
		ManualStatus: core.Valid,
		Payload:      map[string]interface{}{"type": "strep"},
	})

	// 3. Antibiotics are indicated for bacterial infection (Derived)
	engine.AssertFact(&core.Fact{
		ID: "antibiotics_indicated",
		JustificationSets: [][]string{
			{"infection_bacterial"},
		},
	})

	// 4. Amoxicillin is a candidate (Derived from antibiotics_indicated)
	engine.AssertFact(&core.Fact{
		ID: "amoxicillin_candidate",
		JustificationSets: [][]string{
			{"antibiotics_indicated"},
		},
	})

	// 5. Contraindication: If allergy_penicillin, then Amoxicillin is NOT safe (Logic check)
	// Velarix now supports native negated dependencies (e.g. "!risk_flag"), but this test
	// still exercises the explicit safety-gate modeling pattern because it remains useful
	// for workflows that want a first-class reviewable safety fact.

	engine.AssertFact(&core.Fact{
		ID:           "safety_check_passed",
		IsRoot:       true,
		ManualStatus: core.Valid,
	})

	engine.AssertFact(&core.Fact{
		ID: "prescribe_amoxicillin",
		JustificationSets: [][]string{
			{"amoxicillin_candidate", "safety_check_passed"},
		},
	})

	if engine.GetStatus("prescribe_amoxicillin") != core.Valid {
		t.Fatal("expected prescribe_amoxicillin to be valid initially")
	}

	// 6. Impact Analysis: What happens if we invalidate the safety check?
	// In Velarix, GetImpact uses the Dominator Tree.
	// If a fact has MULTIPLE justification sets (OR logic),
	// invalidating one might not invalidate the fact if another set is still valid.

	// In this test: prescribe_amoxicillin depends on (amoxicillin_candidate AND safety_check_passed).
	// It only has ONE justification set. So it SHOULD be impacted.

	report := engine.GetImpact("safety_check_passed")

	found := false
	for _, id := range report.ImpactedIDs {
		if id == "prescribe_amoxicillin" {
			found = true
			break
		}
	}
	if !found {
		// Log engine state for debugging if it fails again
		t.Logf("Engine Facts: %+v", engine.Facts["prescribe_amoxicillin"])
		t.Fatal("expected prescribe_amoxicillin to be impacted by safety_check_passed invalidation")
	}

	// 7. Perform Invalidation
	engine.InvalidateRoot("safety_check_passed")

	if engine.GetStatus("prescribe_amoxicillin") != core.Invalid {
		t.Fatal("expected prescribe_amoxicillin to be invalid after safety check failed")
	}

	// 8. Test "Epistemic Recovery": If the allergy was a false positive and we re-validate safety
	// Note: We can't "re-validate" a root once invalidated in the same session without re-asserting
	// (or replaying history). Let's test idempotency and re-assertion.

	// In Velarix, once a root is invalidated, it stays invalidated in that engine instance.
	// To "undo", we'd usually start a new session or use the 'revalidate' logic.
}

func TestCycleDetection(t *testing.T) {
	engine := core.NewEngine()

	engine.AssertFact(&core.Fact{ID: "A", IsRoot: true, ManualStatus: core.Valid})
	engine.AssertFact(&core.Fact{ID: "B", JustificationSets: [][]string{{"A"}}})

	// Try to create a cycle: C depends on B, and A (root) is changed to depend on C?
	// Actually, AssertFact prevents re-asserting a fact with different content.
	// Let's try to assert C depending on B, then D depending on C and B.

	engine.AssertFact(&core.Fact{ID: "C", JustificationSets: [][]string{{"B"}}})

	// This should fail: D depends on D (self-cycle)
	err := engine.AssertFact(&core.Fact{
		ID:                "D",
		JustificationSets: [][]string{{"D"}},
	})
	if err == nil {
		t.Fatal("expected error when asserting self-cycle")
	}

	// This should fail: D depends on C, and C depends on B, and B depends on A...
	// Wait, to make a real cycle we need a non-root to point back.

	// Let's try: A -> B -> C -> A (where A is now derived)
	engine2 := core.NewEngine()
	engine2.AssertFact(&core.Fact{ID: "X", IsRoot: true, ManualStatus: core.Valid})
	engine2.AssertFact(&core.Fact{ID: "A", JustificationSets: [][]string{{"X"}}}) // A is now derived
	engine2.AssertFact(&core.Fact{ID: "B", JustificationSets: [][]string{{"A"}}})

	err = engine2.AssertFact(&core.Fact{
		ID:                "C",
		JustificationSets: [][]string{{"B", "A"}}, // This is fine, DAG
	})
	if err != nil {
		t.Fatal(err)
	}

	err = engine2.AssertFact(&core.Fact{
		ID:                "B_cycle",
		JustificationSets: [][]string{{"B"}},
	})
	// The current cycle detection in engine.go calls detectCycle(f) BEFORE adding to engine.Facts.
	// So it checks if any of f's parents eventually lead back to f.

	err = engine2.AssertFact(&core.Fact{
		ID:                "Cycle",
		JustificationSets: [][]string{{"Cycle"}},
	})
	if err == nil {
		t.Fatal("expected error on self-cycle")
	}
}
