package core

import (
	"testing"
)

func TestSemanticNegationMismatch(t *testing.T) {
	a := &Fact{ID: "a", Payload: map[string]interface{}{"text": "the patient is responding well to the clinical treatment"}}
	b := &Fact{ID: "b", Payload: map[string]interface{}{"text": "the patient is not responding well to the clinical treatment"}}

	if !semanticNegationMismatch(a, b) {
		t.Errorf("expected mismatch")
	}

	c := &Fact{ID: "c", Payload: map[string]interface{}{"text": "completely unrelated sentence about something else entirely"}}
	if semanticNegationMismatch(a, c) {
		t.Error("should not match semantically")
	}
}

func TestContradictionIssueForFacts(t *testing.T) {
	f1 := &Fact{ID: "f1"}
	f2 := &Fact{ID: "f2"}

	_, ok := contradictionIssueForFacts(nil, f2)
	if ok {
		t.Error()
	}

	f1.Payload = map[string]interface{}{"contradicts": []string{"f2"}}
	issue, ok := contradictionIssueForFacts(f1, f2)
	if !ok || issue.Type != "explicit_contradiction" {
		t.Error()
	}

	f1.Payload = map[string]interface{}{"claim_key": "k", "claim_value": "v1"}
	f2.Payload = map[string]interface{}{"claim_key": "k", "claim_value": "v2"}
	issue, ok = contradictionIssueForFacts(f1, f2)
	if !ok || issue.Type != "claim_value_conflict" {
		t.Error()
	}

	f1.Payload = map[string]interface{}{"subject": "s", "predicate": "p", "object": "o1"}
	f2.Payload = map[string]interface{}{"subject": "s", "predicate": "p", "object": "o2"}
	issue, ok = contradictionIssueForFacts(f1, f2)
	if !ok || issue.Type != "predicate_object_conflict" {
		t.Error()
	}

	f1.Payload = map[string]interface{}{"subject": "s", "predicate": "p", "object": "o", "polarity": "positive"}
	f2.Payload = map[string]interface{}{"subject": "s", "predicate": "p", "object": "o", "polarity": "negative"}
	issue, ok = contradictionIssueForFacts(f1, f2)
	if !ok || issue.Type != "polarity_conflict" {
		t.Error()
	}

	// Test semantic negation conflict
	f1.Payload = map[string]interface{}{"text": "the patient is responding well to the clinical treatment"}
	f2.Payload = map[string]interface{}{"text": "the patient is not responding well to the clinical treatment"}
	issue, ok = contradictionIssueForFacts(f1, f2)
	if !ok || issue.Type != "semantic_negation_conflict" {
		t.Errorf("expected semantic_negation_conflict")
	}
}

func TestMetadataString_TrimsStringValues(t *testing.T) {
	m := map[string]interface{}{"k1": " v1 ", "k2": 123}
	if MetadataString(m, "k1") != "v1" {
		t.Error()
	}
}

func TestMapStringSlice(t *testing.T) {
	m := map[string]interface{}{
		"s1": []string{"a", "b"},
	}
	s1 := mapStringSlice(m, "s1")
	if len(s1) != 2 || s1[0] != "a" {
		t.Error()
	}
}

func TestExtractClaimSignature(t *testing.T) {
	f := &Fact{
		Payload: map[string]interface{}{"claim_key": "k1", "polarity": "Negative"},
	}
	sig := extractClaimSignature(f)
	if sig.ClaimKey != "k1" || sig.Polarity != "negative" {
		t.Error()
	}
}

func TestFirstNonEmptyString(t *testing.T) {
	if firstNonEmptyString(" ", " a ", " b ") != "a" {
		t.Error()
	}
}

func TestContainsString(t *testing.T) {
	if !containsString([]string{"a", "b"}, "b") {
		t.Error()
	}
}

func TestUniqueSortedFactIDs(t *testing.T) {
	out := uniqueSortedFactIDs([]string{" b ", "a", "b", ""})
	if len(out) != 2 || out[0] != "a" || out[1] != "b" {
		t.Error(out)
	}
}

func TestEngine_CheckConsistency(t *testing.T) {
	e := NewEngine()
	f1 := &Fact{ID: "f1", IsRoot: true, ManualStatus: Valid, Payload: map[string]interface{}{"claim_key": "k", "claim_value": "v1"}}
	f2 := &Fact{ID: "f2", IsRoot: true, ManualStatus: Valid, Payload: map[string]interface{}{"claim_key": "k", "claim_value": "v2"}}
	e.AssertFact(f1)
	e.AssertFact(f2)

	report := e.CheckConsistency(nil, false)
	if report.IssueCount != 1 {
		t.Errorf("expected 1 issue, got %d", report.IssueCount)
	}
}
