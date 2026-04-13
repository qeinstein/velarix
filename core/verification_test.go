package core

import "testing"

func TestVerificationRecomputesDescendants(t *testing.T) {
	e := NewEngine()

	// Root fact asserted as true but initially unverified.
	if err := e.AssertFact(&Fact{
		ID:           "root_a",
		IsRoot:       true,
		ManualStatus: 1.0,
		Metadata: map[string]interface{}{
			"source_type":           "llm_output",
			"requires_verification": true,
		},
	}); err != nil {
		t.Fatal(err)
	}

	// Derived fact requires verified grounding from its dependencies.
	if err := e.AssertFact(&Fact{
		ID: "derived_b",
		JustificationSets: [][]string{
			{"root_a"},
		},
		Metadata: map[string]interface{}{
			"grounding_require_verified": true,
		},
	}); err != nil {
		t.Fatal(err)
	}

	if got := e.GetStatus("derived_b"); got >= ConfidenceThreshold {
		t.Fatalf("expected derived_b to be invalid before verification, got status=%v", got)
	}

	// Now verify the root; this should immediately re-evaluate the child even
	// though the parent's DerivedStatus did not change.
	if err := e.SetFactVerification("root_a", "verified", "human", "test", "ok", 0); err != nil {
		t.Fatal(err)
	}

	if got := e.GetStatus("derived_b"); got < ConfidenceThreshold {
		t.Fatalf("expected derived_b to become valid after verification, got status=%v", got)
	}
}
