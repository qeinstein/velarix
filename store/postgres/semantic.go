package postgres

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"velarix/core"
)

const semanticVectorDims = 128

func normalizeSemanticEmbedding(vec []float64) []float64 {
	if len(vec) == semanticVectorDims {
		return vec
	}
	out := make([]float64, semanticVectorDims)
	copy(out, vec)
	return out
}

func pgVectorLiteral(vec []float64) string {
	vec = normalizeSemanticEmbedding(core.NormalizeVector(vec))
	parts := make([]string, len(vec))
	for i, v := range vec {
		parts[i] = fmt.Sprintf("%f", v)
	}
	return "[" + strings.Join(parts, ",") + "]"
}

func (s *Store) semanticVectorColumnAvailable() bool {
	s.vectorColumnOnce.Do(func() {
		_ = s.pool.QueryRow(context.Background(), `
			SELECT EXISTS (
				SELECT 1
				FROM information_schema.columns
				WHERE table_name = 'semantic_fact_embeddings' AND column_name = 'embedding_vector'
			)
		`).Scan(&s.vectorColumnReady)
	})
	return s.vectorColumnReady
}

func (s *Store) UpsertFactEmbedding(orgID, sessionID string, fact *core.Fact, status core.Status) error {
	if fact == nil || strings.TrimSpace(orgID) == "" || strings.TrimSpace(sessionID) == "" || strings.TrimSpace(fact.ID) == "" {
		return nil
	}
	embedding := normalizeSemanticEmbedding(core.EmbeddingForFact(fact))
	embeddingJSON, err := json.Marshal(embedding)
	if err != nil {
		return err
	}
	doc, err := json.Marshal(fact)
	if err != nil {
		return err
	}
	now := s.nowMillis()

	if s.semanticVectorColumnAvailable() {
		_, err = s.pool.Exec(context.Background(), `
			INSERT INTO semantic_fact_embeddings (
				org_id, session_id, fact_id, updated_at, resolved_status, embedding_json, embedding_vector, doc
			)
			VALUES ($1, $2, $3, $4, $5, $6::jsonb, $7::vector, $8::jsonb)
			ON CONFLICT (org_id, session_id, fact_id)
			DO UPDATE SET
				updated_at = EXCLUDED.updated_at,
				resolved_status = EXCLUDED.resolved_status,
				embedding_json = EXCLUDED.embedding_json,
				embedding_vector = EXCLUDED.embedding_vector,
				doc = EXCLUDED.doc
		`, orgID, sessionID, fact.ID, now, float64(status), embeddingJSON, pgVectorLiteral(embedding), doc)
		return err
	}

	_, err = s.pool.Exec(context.Background(), `
		INSERT INTO semantic_fact_embeddings (
			org_id, session_id, fact_id, updated_at, resolved_status, embedding_json, doc
		)
		VALUES ($1, $2, $3, $4, $5, $6::jsonb, $7::jsonb)
		ON CONFLICT (org_id, session_id, fact_id)
		DO UPDATE SET
			updated_at = EXCLUDED.updated_at,
			resolved_status = EXCLUDED.resolved_status,
			embedding_json = EXCLUDED.embedding_json,
			doc = EXCLUDED.doc
	`, orgID, sessionID, fact.ID, now, float64(status), embeddingJSON, doc)
	return err
}

func (s *Store) SemanticSearchFacts(orgID, sessionID string, queryEmbedding []float64, limit int, validOnly bool) ([]core.SemanticMatch, error) {
	if limit <= 0 {
		limit = 10
	}
	queryEmbedding = normalizeSemanticEmbedding(core.NormalizeVector(queryEmbedding))

	if s.semanticVectorColumnAvailable() {
		minStatus := 0.0
		if validOnly {
			minStatus = float64(core.ConfidenceThreshold)
		}
		rows, err := s.pool.Query(context.Background(), `
			SELECT fact_id, resolved_status, doc, 1 - (embedding_vector <=> $3::vector) AS score
			FROM semantic_fact_embeddings
			WHERE org_id = $1
			  AND session_id = $2
			  AND resolved_status >= $4
			ORDER BY embedding_vector <=> $3::vector, updated_at DESC, fact_id DESC
			LIMIT $5
		`, orgID, sessionID, pgVectorLiteral(queryEmbedding), minStatus, limit)
		if err == nil {
			defer rows.Close()
			matches := []core.SemanticMatch{}
			for rows.Next() {
				var factID string
				var status float64
				var raw []byte
				var score float64
				if err := rows.Scan(&factID, &status, &raw, &score); err != nil {
					return nil, err
				}
				var fact core.Fact
				if err := json.Unmarshal(raw, &fact); err != nil {
					return nil, err
				}
				matches = append(matches, core.SemanticMatch{
					FactID:         factID,
					Score:          score,
					ResolvedStatus: status,
					IsRetracted:    status < float64(core.ConfidenceThreshold),
					Payload:        fact.Payload,
				})
			}
			return matches, rows.Err()
		}
	}

	rows, err := s.pool.Query(context.Background(), `
		SELECT fact_id, resolved_status, doc
		FROM semantic_fact_embeddings
		WHERE org_id = $1
		  AND session_id = $2
		ORDER BY updated_at DESC, fact_id DESC
		LIMIT $3
	`, orgID, sessionID, max(limit*20, 200))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	matches := []core.SemanticMatch{}
	for rows.Next() {
		var factID string
		var status float64
		var raw []byte
		if err := rows.Scan(&factID, &status, &raw); err != nil {
			return nil, err
		}
		if validOnly && status < float64(core.ConfidenceThreshold) {
			continue
		}
		var fact core.Fact
		if err := json.Unmarshal(raw, &fact); err != nil {
			return nil, err
		}
		score := core.CosineSimilarity(queryEmbedding, normalizeSemanticEmbedding(core.EmbeddingForFact(&fact)))
		matches = append(matches, core.SemanticMatch{
			FactID:         factID,
			Score:          score,
			ResolvedStatus: status,
			IsRetracted:    status < float64(core.ConfidenceThreshold),
			Payload:        fact.Payload,
		})
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	sortMatches(matches)
	if len(matches) > limit {
		matches = matches[:limit]
	}
	return matches, nil
}

func sortMatches(matches []core.SemanticMatch) {
	for i := 0; i < len(matches); i++ {
		for j := i + 1; j < len(matches); j++ {
			if matches[j].Score > matches[i].Score || (matches[j].Score == matches[i].Score && matches[j].FactID < matches[i].FactID) {
				matches[i], matches[j] = matches[j], matches[i]
			}
		}
	}
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
