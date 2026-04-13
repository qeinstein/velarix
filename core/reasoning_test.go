package core

import "testing"

func TestUniqueIDs(t *testing.T) {
	out := uniqueIDs([]string{"a", "b"}, []string{"b", "c"})
	if len(out) != 3 || out[0] != "a" || out[1] != "b" || out[2] != "c" {
		t.Error()
	}
}

func TestAuditReasoningChain(t *testing.T) {
	e := NewEngine()
	
	report := e.AuditReasoningChain(nil)
	if report.Valid {
		t.Error("expected invalid for nil chain")
	}

	// f1 and f2 conflict (same key, different values)
	f1 := &Fact{ID: "f1", IsRoot: true, ManualStatus: Valid, Payload: map[string]interface{}{"claim_key": "k", "claim_value": "v1"}}
	f2 := &Fact{ID: "f2", IsRoot: true, ManualStatus: Valid, Payload: map[string]interface{}{"claim_key": "k", "claim_value": "v2"}}
	f3 := &Fact{ID: "f3", IsRoot: true, ManualStatus: Invalid}
	e.AssertFact(f1)
	e.AssertFact(f2)
	e.AssertFact(f3)

	chain := &ReasoningChain{
		ChainID: "chain1",
		Steps: []ReasoningStep{
			{
				ID: "step1",
				EvidenceFactIDs: []string{"f1"},
				OutputFactID: "f1",
			},
			{
				ID: "step2",
				EvidenceFactIDs: []string{"f1"},
				OutputFactID: "f2", // Conflict with prior output f1
			},
			{
				ID: "step3",
				EvidenceFactIDs: []string{"f3", "missing"},
				// NO OutputFactID here to avoid multiple missing facts
			},
		},
	}

	report = e.AuditReasoningChain(chain)
	if report.Valid {
		t.Error("expected invalid chain")
	}

	if len(report.StepAudits) != 3 {
		t.Fatalf("expected 3 step audits, got %d", len(report.StepAudits))
	}

	sa2 := report.StepAudits[1]
	if len(sa2.ConsistencyFindings) == 0 {
		t.Error("expected consistency findings between f1 and f2 in step 2")
	}

	sa3 := report.StepAudits[2]
	if len(sa3.MissingFactIDs) != 1 || sa3.MissingFactIDs[0] != "missing" {
		t.Errorf("expected 1 missing fact 'missing', got %v", sa3.MissingFactIDs)
	}
	if len(sa3.InvalidFactIDs) != 1 || sa3.InvalidFactIDs[0] != "f3" {
		t.Error("expected invalid fact")
	}

	if len(report.RetractCandidateFactIDs) == 0 {
		t.Error("expected retract candidates")
	}
}
