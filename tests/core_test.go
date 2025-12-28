package tests

import (
	"testing"

	"causaldb/core"
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

	if b.DerivedStatus != core.Valid {
		t.Fatalf("expected B to be valid")
	}

	// Invalidate A
	if err := engine.InvalidateRoot("A"); err != nil {
		t.Fatal(err)
	}

	if b.DerivedStatus != core.Invalid {
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

	if b.DerivedStatus != core.Valid {
		t.Fatalf("expected B to be valid initially")
	}

	// Invalidate A
	engine.InvalidateRoot("A")

	if b.DerivedStatus != core.Valid {
		t.Fatalf("expected B to remain valid due to C")
	}

	// Invalidate C
	engine.InvalidateRoot("C")

	if b.DerivedStatus != core.Invalid {
		t.Fatalf("expected B to be invalid after both supports removed")
	}
}



func TestExplain(t *testing.T) {
	engine := core.NewEngine()

	a := &core.Fact{
		ID:           "A",
		IsRoot:       true,
		ManualStatus: core.Valid,
	}
	engine.AssertFact(a)

	c := &core.Fact{
		ID:           "C",
		IsRoot:       true,
		ManualStatus: core.Valid,
	}
	engine.AssertFact(c)

	b := &core.Fact{
		ID: "B",
		JustificationSets: [][]string{
			{"A"},
			{"C"},
		},
	}
	engine.AssertFact(b)

	explanations, err := engine.Explain("B")
	if err != nil {
		t.Fatal(err)
	}

	if len(explanations) != 2 {
		t.Fatalf("expected 2 explanations, got %d", len(explanations))
	}
}
