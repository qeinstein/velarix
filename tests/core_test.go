package tests

import (
	"testing"
	"fmt"
	"velarix/core"
	"github.com/xeipuuv/gojsonschema"
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
