package core

import "testing"

func TestConfidenceTier(t *testing.T) {
	if confidenceTier(0.95, "") != "certain" {
		t.Error()
	}
	if confidenceTier(0.7, "") != "probable" {
		t.Error()
	}
	if confidenceTier(0.5, "") != "uncertain" {
		t.Error()
	}
	// Uncertain assertion kind caps tier at "probable" even at high confidence.
	if confidenceTier(0.95, AssertionKindUncertain) != "probable" {
		t.Errorf("expected uncertain kind to cap at probable")
	}
	if confidenceTier(0.5, AssertionKindUncertain) != "uncertain" {
		t.Error()
	}
}

func TestExplainReasoning(t *testing.T) {
	e := NewEngine()

	f1 := &Fact{
		ID:           "f1",
		IsRoot:       true,
		ManualStatus: Valid,
		Metadata: map[string]interface{}{
			"_provenance": map[string]interface{}{
				"source_type":    "user",
				"source_ref":     "ref1",
				"payload_hash":   "hash1",
				"policy_version": "v1",
			},
		},
	}
	e.AssertFact(f1)

	f2 := &Fact{
		ID:                "f2",
		JustificationSets: [][]string{{"f1", "!missing"}},
	}
	// "missing" is unknown, AssertFact will fail
	_ = f2

	f2Valid := &Fact{
		ID:                "f2",
		JustificationSets: [][]string{{"f1"}},
	}
	e.AssertFact(f2Valid)

	f3 := &Fact{
		ID:                "f3",
		JustificationSets: [][]string{{"f2", "!f1"}},
	}
	e.AssertFact(f3)

	_, err := e.ExplainReasoning("missing")
	if err == nil {
		t.Error()
	}

	out, err := e.ExplainReasoning("f3")
	if err != nil {
		t.Fatal(err)
	}

	if out.FactID != "f3" {
		t.Error()
	}

	if len(out.CausalChain) != 3 {
		t.Errorf("expected 3 items in causal chain, got %d", len(out.CausalChain))
	}

	if len(out.Sources) != 1 || out.Sources[0].SourceRef != "ref1" {
		t.Error("expected 1 source")
	}

	if len(out.PolicyVersions) != 1 || out.PolicyVersions[0] != "v1" {
		t.Error("expected 1 policy version")
	}

	// Test counterfactual
	e.recomputeDominators()
	cf := e.computeCounterfactual("f2", "f1")
	if cf.TotalCount == 0 {
		t.Error("expected impact")
	}
	if cf.Narrative == "" {
		t.Error("expected narrative")
	}

	cf2 := e.computeCounterfactual("f1", "f2")
	if cf2.TotalCount != 0 {
		t.Error("f2 does not impact f1")
	}
}

func TestEnrichForDecisionContextNilFact(t *testing.T) {
	out := &ExplanationOutput{}
	out.enrichForDecisionContext(nil, 1.0)
	if out.Summary != "" {
		t.Error()
	}
}
