package core

import (
	"fmt"
	"math"
	"sort"
	"strings"
	"unicode"
)

const defaultEmbeddingDimensions = 128

type SemanticMatch struct {
	FactID         string                 `json:"fact_id"`
	Score          float64                `json:"score"`
	ResolvedStatus float64                `json:"resolved_status"`
	IsRetracted    bool                   `json:"is_retracted"`
	Payload        map[string]interface{} `json:"payload,omitempty"`
}

func tokenizeText(text string) []string {
	cleaned := strings.Map(func(r rune) rune {
		switch {
		case unicode.IsLetter(r), unicode.IsNumber(r):
			return unicode.ToLower(r)
		case unicode.IsSpace(r):
			return ' '
		default:
			return ' '
		}
	}, text)
	return strings.Fields(cleaned)
}

func LexicalEmbedding(text string, dims int) []float64 {
	if dims <= 0 {
		dims = defaultEmbeddingDimensions
	}
	vec := make([]float64, dims)
	for _, token := range tokenizeText(text) {
		if token == "" {
			continue
		}
		var hash uint32 = 2166136261
		for _, r := range token {
			hash ^= uint32(r)
			hash *= 16777619
		}
		idx := int(hash % uint32(dims))
		sign := 1.0
		if (hash/uint32(dims))%2 == 1 {
			sign = -1.0
		}
		vec[idx] += sign
	}
	return NormalizeVector(vec)
}

func NormalizeVector(vec []float64) []float64 {
	if len(vec) == 0 {
		return vec
	}
	var norm float64
	for _, v := range vec {
		norm += v * v
	}
	if norm == 0 {
		return vec
	}
	norm = math.Sqrt(norm)
	out := make([]float64, len(vec))
	for i, v := range vec {
		out[i] = v / norm
	}
	return out
}

func CosineSimilarity(a, b []float64) float64 {
	if len(a) == 0 || len(b) == 0 {
		return 0
	}
	n := len(a)
	if len(b) < n {
		n = len(b)
	}
	var dot float64
	for i := 0; i < n; i++ {
		dot += a[i] * b[i]
	}
	if dot > 1 {
		return 1
	}
	if dot < -1 {
		return -1
	}
	return dot
}

func factSemanticText(f *Fact) string {
	if f == nil {
		return ""
	}
	parts := []string{f.ID}
	if f.Payload != nil {
		parts = append(parts, fmt.Sprintf("%v", f.Payload))
	}
	if f.Metadata != nil {
		for _, key := range []string{"source_ref", "source_type", "subject", "predicate", "object", "claim_key", "claim_value"} {
			if v, ok := f.Metadata[key]; ok {
				parts = append(parts, fmt.Sprintf("%v", v))
			}
		}
	}
	return strings.Join(parts, " ")
}

func EmbeddingForFact(f *Fact) []float64 {
	if f == nil {
		return nil
	}
	if len(f.Embedding) > 0 {
		return NormalizeVector(f.Embedding)
	}
	return LexicalEmbedding(factSemanticText(f), defaultEmbeddingDimensions)
}

func (e *Engine) SearchSimilarFacts(query string, limit int, validOnly bool) []SemanticMatch {
	e.mu.RLock()
	defer e.mu.RUnlock()

	if limit <= 0 {
		limit = 10
	}
	queryVec := LexicalEmbedding(query, defaultEmbeddingDimensions)

	matches := make([]SemanticMatch, 0, len(e.Facts))
	for _, fact := range e.Facts {
		status := e.effectiveStatusUnsafe(fact)
		if validOnly && status < ConfidenceThreshold {
			continue
		}
		score := 0.0
		if strings.TrimSpace(query) != "" {
			score = CosineSimilarity(queryVec, EmbeddingForFact(fact))
		}
		_, isRetracted := e.RetractedFacts[fact.ID]
		matches = append(matches, SemanticMatch{
			FactID:         fact.ID,
			Score:          score,
			ResolvedStatus: float64(status),
			IsRetracted:    isRetracted,
			Payload:        fact.Payload,
		})
	}

	sort.Slice(matches, func(i, j int) bool {
		if matches[i].Score == matches[j].Score {
			return matches[i].FactID < matches[j].FactID
		}
		return matches[i].Score > matches[j].Score
	})

	if len(matches) > limit {
		matches = matches[:limit]
	}
	return matches
}
