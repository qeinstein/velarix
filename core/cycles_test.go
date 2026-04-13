package core

import (
	"testing"
)

func TestDetectCycle(t *testing.T) {
	e := NewEngine()
	e.AssertFact(&Fact{ID: "a", IsRoot: true, ManualStatus: Valid})
	e.AssertFact(&Fact{ID: "b", JustificationSets: [][]string{{"a"}}})

	// Add c depending on b
	err := e.AssertFact(&Fact{ID: "c", JustificationSets: [][]string{{"b"}}})
	if err != nil {
		t.Fatal(err)
	}

	// Try to add d depending on c
	e.AssertFact(&Fact{ID: "d", JustificationSets: [][]string{{"c"}}})

	// Now try to update b to depend on d (cycle: b -> d -> c -> b)
	err = e.detectCycle(&Fact{ID: "b", JustificationSets: [][]string{{"d"}}})
	if err == nil {
		t.Fatal("expected cycle error")
	}
	cycleErr, ok := err.(*CycleError)
	if !ok {
		t.Fatalf("expected CycleError, got %T", err)
	}
	if cycleErr.Error() == "" {
		t.Error("expected non-empty error string")
	}

	// Try to update a to depend on a (self cycle)
	err = e.detectCycle(&Fact{ID: "a", JustificationSets: [][]string{{"a"}}})
	if err == nil {
		t.Fatal("expected self cycle error")
	}

	// Nil arg
	if err := e.detectCycle(nil); err == nil {
		t.Fatal("expected nil arg error")
	}
}

func TestSplitDependencySet(t *testing.T) {
	pos, neg, all, err := splitDependencySet([]string{"a", "!b", "a"})
	if err != nil {
		t.Fatal(err)
	}
	if len(pos) != 1 || pos[0] != "a" {
		t.Errorf("pos: %v", pos)
	}
	if len(neg) != 1 || neg[0] != "b" {
		t.Errorf("neg: %v", neg)
	}
	if len(all) != 2 {
		t.Errorf("all: %v", all)
	}

	_, _, _, err = splitDependencySet([]string{"  "})
	if err == nil {
		t.Error("expected parse error")
	}
}
