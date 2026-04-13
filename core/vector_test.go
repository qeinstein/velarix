package core

import (
	"math"
	"testing"
)

func TestTokenizeText(t *testing.T) {
	tokens := tokenizeText("Hello, World! 123")
	if len(tokens) != 3 || tokens[0] != "hello" || tokens[1] != "world" || tokens[2] != "123" {
		t.Errorf("unexpected tokens: %v", tokens)
	}
}

func TestLexicalEmbedding(t *testing.T) {
	vec := LexicalEmbedding("test", 0)
	if len(vec) != defaultEmbeddingDimensions {
		t.Error("expected default dimensions")
	}

	vec2 := LexicalEmbedding("test  !@#", 10)
	if len(vec2) != 10 {
		t.Error("expected 10 dimensions")
	}

	norm := 0.0
	for _, v := range vec2 {
		norm += v * v
	}
	if math.Abs(norm-1.0) > 1e-6 && len(tokenizeText("test")) > 0 {
		t.Error("expected normalized vector")
	}
}

func TestNormalizeVector(t *testing.T) {
	vec := NormalizeVector(nil)
	if len(vec) != 0 {
		t.Error()
	}
	vec = NormalizeVector([]float64{0, 0})
	if vec[0] != 0 {
		t.Error()
	}
	vec = NormalizeVector([]float64{3, 4})
	if vec[0] != 0.6 || vec[1] != 0.8 {
		t.Error()
	}
}

func TestCosineSimilarity(t *testing.T) {
	if CosineSimilarity(nil, nil) != 0 {
		t.Error()
	}
	v1 := []float64{1, 0}
	v2 := []float64{0, 1}
	if CosineSimilarity(v1, v2) != 0 {
		t.Error()
	}
	if CosineSimilarity(v1, v1) != 1 {
		t.Error()
	}
	if CosineSimilarity(v1, []float64{-1, 0}) != -1 {
		t.Error()
	}
}

func TestFactSemanticText(t *testing.T) {
	if factSemanticText(nil) != "" {
		t.Error()
	}
	f := &Fact{
		ID:      "f1",
		Payload: map[string]interface{}{"a": "b"},
		Metadata: map[string]interface{}{
			"subject": "s",
		},
	}
	text := factSemanticText(f)
	if text == "" {
		t.Error()
	}
}

func TestEmbeddingForFact(t *testing.T) {
	if EmbeddingForFact(nil) != nil {
		t.Error()
	}
	f := &Fact{ID: "f1", Embedding: []float64{3, 4}}
	emb := EmbeddingForFact(f)
	if emb[0] != 0.6 {
		t.Error()
	}

	f2 := &Fact{ID: "f2", Payload: map[string]interface{}{"a": "b"}}
	emb2 := EmbeddingForFact(f2)
	if len(emb2) != defaultEmbeddingDimensions {
		t.Error()
	}
}

func TestEngine_SearchSimilarFacts(t *testing.T) {
	e := NewEngine()
	e.AssertFact(&Fact{ID: "f1", IsRoot: true, ManualStatus: Valid, Payload: map[string]interface{}{"text": "apple banana"}})
	e.AssertFact(&Fact{ID: "f2", IsRoot: true, ManualStatus: Invalid, Payload: map[string]interface{}{"text": "apple cherry"}})
	
	matches := e.SearchSimilarFacts("apple", 0, false)
	if len(matches) != 2 {
		t.Errorf("expected 2 matches, got %d", len(matches))
	}

	matchesValid := e.SearchSimilarFacts("apple", 10, true)
	if len(matchesValid) != 1 || matchesValid[0].FactID != "f1" {
		t.Error("expected 1 valid match")
	}

	matchesLimit := e.SearchSimilarFacts("apple", 1, false)
	if len(matchesLimit) != 1 {
		t.Error("expected 1 limit match")
	}
}
