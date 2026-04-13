package core

import (
	"strings"
	"testing"
)

func TestEngine_SubscribeUnsubscribe(t *testing.T) {
	e := NewEngine()
	ch := e.Subscribe()

	e.Lock()
	e.notify("fact1", Valid)
	e.Unlock()

	event := <-ch
	if event.FactID != "fact1" || event.Status != Valid {
		t.Errorf("unexpected event: %+v", event)
	}

	e.Unsubscribe(ch)

	e.Lock()
	e.notify("fact2", Valid) // Should not block
	e.Unlock()
}

func TestEngine_AssertFact_Basic(t *testing.T) {
	e := NewEngine()

	// Test nil
	if err := e.AssertFact(nil); err == nil {
		t.Error("expected error for nil fact")
	}

	// Test max cap
	e.Facts = make(map[string]*Fact)
	for i := 0; i < MaxFactsPerSession; i++ {
		e.Facts["dummy"] = &Fact{}
	}
	if err := e.AssertFact(&Fact{ID: "new"}); err == nil {
		t.Error("expected cap error")
	}
	e.Facts = make(map[string]*Fact)

	// Test root fact
	root := &Fact{ID: "root1", IsRoot: true, ManualStatus: Valid}
	if err := e.AssertFact(root); err != nil {
		t.Fatal(err)
	}
	if e.GetStatus("root1") != Valid {
		t.Error("expected Valid status")
	}

	// Idempotency
	if err := e.AssertFact(root); err != nil {
		t.Error("expected nil for identical fact, got", err)
	}

	// Same ID, different content
	root2 := &Fact{ID: "root1", IsRoot: true, ManualStatus: Invalid}
	if err := e.AssertFact(root2); err == nil {
		t.Error("expected error for different content")
	}

	// Non-root without justification
	if err := e.AssertFact(&Fact{ID: "derived1"}); err == nil {
		t.Error("expected error for derived without justification")
	}

	// Non-root with empty justification
	if err := e.AssertFact(&Fact{ID: "derived1", JustificationSets: [][]string{{}}}); err == nil {
		t.Error("expected error for empty justification set")
	}

	// Non-root with unknown parent
	if err := e.AssertFact(&Fact{ID: "derived1", JustificationSets: [][]string{{"unknown_parent"}}}); err == nil {
		t.Error("expected error for unknown parent")
	}
}

func TestEngine_AssertFact_Propagate(t *testing.T) {
	e := NewEngine()
	e.AssertFact(&Fact{ID: "root1", IsRoot: true, ManualStatus: Valid})
	e.AssertFact(&Fact{ID: "root2", IsRoot: true, ManualStatus: Invalid})

	// Derived from root1
	f1 := &Fact{
		ID:                "derived1",
		JustificationSets: [][]string{{"root1"}},
	}
	if err := e.AssertFact(f1); err != nil {
		t.Fatal(err)
	}
	if e.GetStatus("derived1") != Valid {
		t.Error("expected derived1 to be Valid")
	}

	// Derived from root2 (Invalid)
	f2 := &Fact{
		ID:                "derived2",
		JustificationSets: [][]string{{"root2"}},
	}
	e.AssertFact(f2)
	if e.GetStatus("derived2") == Valid {
		t.Error("expected derived2 to be Invalid")
	}

	// Derived from !root2
	f3 := &Fact{
		ID:                "derived3",
		JustificationSets: [][]string{{"!root2"}},
	}
	e.AssertFact(f3)
	// Because of dominator pruning, GetStatus returns Invalid (since root2 is Invalid and in CollapsedRoots).
	// But the DerivedStatus should be Valid.
	if e.GetStatus("derived3") != Invalid {
		t.Error("expected derived3 to be Invalid due to dominator pruning")
	}
	if f, _ := e.GetFact("derived3"); f.DerivedStatus != Valid {
		t.Error("expected derived3 DerivedStatus to be Valid")
	}

	// Multiple sets (OR)
	f4 := &Fact{
		ID:                "derived4",
		JustificationSets: [][]string{{"root2"}, {"root1"}},
	}
	e.AssertFact(f4)
	if e.GetStatus("derived4") != Valid {
		t.Error("expected derived4 to be Valid (OR)")
	}
}

func TestEngine_InvalidateRoot_And_Confidence(t *testing.T) {
	e := NewEngine()
	e.AssertFact(&Fact{ID: "r1", IsRoot: true, ManualStatus: Valid})
	e.AssertFact(&Fact{ID: "d1", JustificationSets: [][]string{{"r1"}}})

	if e.GetStatus("d1") != Valid {
		t.Error("d1 should be Valid")
	}

	if err := e.InvalidateRoot("nonexistent"); err == nil {
		t.Error("expected error for nonexistent")
	}
	if err := e.InvalidateRoot("d1"); err == nil {
		t.Error("expected error for non-root")
	}

	e.InvalidateRoot("r1")
	if e.GetStatus("d1") != Invalid {
		t.Error("d1 should be Invalid after root invalidated")
	}

	e.SetRootConfidence("r1", Valid)
	if e.GetStatus("d1") != Valid {
		t.Error("d1 should be Valid after root confidence restored")
	}

	if err := e.SetRootConfidence("d1", Valid); err == nil {
		t.Error("SetRootConfidence on non-root should fail")
	}
}

func TestEngine_RetractFact(t *testing.T) {
	e := NewEngine()
	e.AssertFact(&Fact{ID: "r1", IsRoot: true, ManualStatus: Valid})
	e.AssertFact(&Fact{ID: "d1", JustificationSets: [][]string{{"r1"}}})

	if err := e.RetractFact("nonexistent", ""); err == nil {
		t.Error("expected error")
	}

	e.RetractFact("r1", "test reason")
	if e.GetStatus("d1") != Invalid {
		t.Error("d1 should be Invalid after r1 retracted")
	}

	e.RetractFact("r1", "test reason") // idempotent check
}

func TestEngine_GetImpact(t *testing.T) {
	e := NewEngine()
	e.AssertFact(&Fact{ID: "r1", IsRoot: true, ManualStatus: Valid, Payload: map[string]interface{}{"type": "action"}})
	e.AssertFact(&Fact{ID: "r2", IsRoot: true, ManualStatus: Valid})
	e.AssertFact(&Fact{ID: "d1", JustificationSets: [][]string{{"r1", "r2"}}, Payload: map[string]interface{}{"type": "action"}})

	report, err := e.GetImpact("r1")
	if err != nil {
		t.Fatal(err)
	}
	if report.TotalCount != 2 {
		t.Errorf("expected 2 impacted (r1, d1), got %d", report.TotalCount)
	}
	if report.ActionCount != 2 {
		t.Errorf("expected 2 action count, got %d", report.ActionCount)
	}

	_, err = e.GetImpact("unknown")
	if err == nil {
		t.Error("expected error for unknown fact")
	}
}

func TestCloneFunctions(t *testing.T) {
	v1 := cloneStringSlice(nil)
	if v1 != nil {
		t.Error("clone nil string slice failed")
	}
	v2 := cloneFloat64Slice(nil)
	if v2 != nil {
		t.Error("clone nil float slice failed")
	}
	v3 := cloneStringMatrix(nil)
	if v3 != nil {
		t.Error("clone nil matrix failed")
	}

	m1 := cloneStringMatrix([][]string{{"a"}})
	if len(m1) != 1 || m1[0][0] != "a" {
		t.Error("clone matrix failed")
	}

	d1 := cloneDynamicValue(map[string]interface{}{"k": []interface{}{"a", 1.0}, "s": []string{"a"}, "f": []float64{1.0}})
	if d1 == nil {
		t.Error("clone dynamic failed")
	}

	f1 := cloneFact(nil)
	if f1 != nil {
		t.Error("clone nil fact failed")
	}
}

func TestEngine_Snapshot(t *testing.T) {
	e := NewEngine()
	e.AssertFact(&Fact{ID: "r1", IsRoot: true, ManualStatus: Valid})
	e.AssertFact(&Fact{ID: "d1", JustificationSets: [][]string{{"r1"}}})

	snap, err := e.ToSnapshot()
	if err != nil {
		t.Fatal(err)
	}

	e2 := NewEngine()
	if err := e2.FromSnapshot(nil); err == nil {
		t.Error("expected error for nil snapshot")
	}

	snap.Checksum = 0 // break checksum
	if err := e2.FromSnapshot(snap); err == nil {
		t.Error("expected error for broken checksum")
	}

	snap, _ = e.ToSnapshot() // fix checksum
	if err := e2.FromSnapshot(snap); err != nil {
		t.Fatal(err)
	}

	if e2.GetStatus("d1") != Valid {
		t.Error("d1 should be valid in restored engine")
	}

	facts := e2.ListFacts()
	if len(facts) != 2 {
		t.Error("expected 2 facts")
	}

	deps, err := e2.DependencyIDs("d1", true)
	if err != nil {
		t.Fatal(err)
	}
	if len(deps) != 2 {
		t.Errorf("expected 2 deps, got %d", len(deps))
	}
	_, err = e2.DependencyIDs("unknown", false)
	if err == nil {
		t.Error("expected error for unknown dep")
	}
}

func TestBuildValidationEngineNilFact(t *testing.T) {
	facts := map[string]*Fact{
		"f1": nil,
	}
	_, err := buildValidationEngine(facts)
	if err == nil {
		t.Error("expected error for nil fact")
	}
}

func TestBuildValidationEngineUnknownParent(t *testing.T) {
	facts := map[string]*Fact{
		"f1": {ID: "f1", JustificationSets: [][]string{{"unknown"}}},
	}
	_, err := buildValidationEngine(facts)
	if err == nil || !strings.Contains(err.Error(), "unknown parent fact") {
		t.Error("expected unknown parent fact error")
	}
}
