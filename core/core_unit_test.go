package core

import (
	"testing"
)

func TestDependencyConfidence(t *testing.T) {
	if c := dependencyConfidence(1.0, false); c != 1.0 {
		t.Errorf("expected 1.0, got %f", c)
	}
	if c := dependencyConfidence(Invalid, true); c != Valid {
		t.Errorf("expected Valid for Invalid negated, got %f", c)
	}
}

func TestNormalizeDependencyToken(t *testing.T) {
	id, err := normalizeDependencyToken("!foo")
	if err != nil {
		t.Fatal(err)
	}
	if id != "foo" {
		t.Errorf("expected foo, got %s", id)
	}

	id2, err2 := normalizeDependencyToken("bar")
	if err2 != nil {
		t.Fatal(err2)
	}
	if id2 != "bar" {
		t.Errorf("expected bar, got %s", id2)
	}

	_, err3 := normalizeDependencyToken("  ")
	if err3 == nil {
		t.Error("expected error for empty token")
	}
}

func TestDependencySatisfied(t *testing.T) {
	if !dependencySatisfied(1.0, false) {
		t.Error("expected true for 1.0 false")
	}
	if dependencySatisfied(0.5, false) {
		t.Error("expected false for 0.5 false")
	}
	if !dependencySatisfied(0.5, true) {
		t.Error("expected true for 0.5 true")
	}
}

func TestParseDependencyRef(t *testing.T) {
	ref, err := ParseDependencyRef("!dep1")
	if err != nil {
		t.Fatal(err)
	}
	if !ref.Negated || ref.FactID != "dep1" {
		t.Errorf("unexpected ref: %+v", ref)
	}
}

func TestNewEngineInit(t *testing.T) {
	e := NewEngine()
	if e == nil {
		t.Fatal("engine is nil")
	}
	if e.Facts == nil || e.JustificationSets == nil {
		t.Error("engine maps not initialized")
	}
}
