package core

import "testing"

func TestDominatorsAndLCA(t *testing.T) {
	e := NewEngine()
	e.AssertFact(&Fact{ID: "r1", IsRoot: true, ManualStatus: Valid})
	e.AssertFact(&Fact{ID: "r2", IsRoot: true, ManualStatus: Valid})
	
	e.AssertFact(&Fact{ID: "n1", JustificationSets: [][]string{{"r1", "r2"}}})
	e.AssertFact(&Fact{ID: "n2", JustificationSets: [][]string{{"n1"}}})
	
	e.recomputeDominators()
	
	if !e.isDominatorAncestor("n1", "n2") {
		t.Error("n1 should dominate n2")
	}
	
	if e.isDominatorAncestor("n2", "n1") {
		t.Error("n2 should not dominate n1")
	}

	if e.isDominatorAncestor("unknown", "n1") {
		t.Error("unknown node")
	}

	if lca := e.lca("n1", "n2"); lca != "n1" {
		t.Errorf("lca: %s", lca)
	}

	if lca := e.lca("", "n1"); lca != "n1" {
		t.Error()
	}
	if lca := e.lca("n1", ""); lca != "n1" {
		t.Error()
	}
	if lca := e.lca("n1", "n1"); lca != "n1" {
		t.Error()
	}
}
