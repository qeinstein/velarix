package postgres

import (
	"context"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"

	"velarix/store"
)

func (s *Store) SaveDecision(decision *store.Decision) error {
	if decision == nil {
		return fmt.Errorf("decision is required")
	}
	if decision.CreatedAt == 0 {
		decision.CreatedAt = s.nowMillis()
	}
	if decision.UpdatedAt == 0 {
		decision.UpdatedAt = decision.CreatedAt
	}
	raw, err := marshalJSON(decision)
	if err != nil {
		return err
	}
	_, err = s.pool.Exec(context.Background(), `
		INSERT INTO decisions (session_id, decision_id, org_id, status, execution_status, subject_ref, target_ref, created_at, updated_at, doc)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10::jsonb)
		ON CONFLICT (session_id, decision_id)
		DO UPDATE SET
		    org_id = EXCLUDED.org_id,
		    status = EXCLUDED.status,
		    execution_status = EXCLUDED.execution_status,
		    subject_ref = EXCLUDED.subject_ref,
		    target_ref = EXCLUDED.target_ref,
		    created_at = EXCLUDED.created_at,
		    updated_at = EXCLUDED.updated_at,
		    doc = EXCLUDED.doc
	`, decision.SessionID, decision.ID, decision.OrgID, decision.Status, decision.ExecutionStatus, decision.SubjectRef, decision.TargetRef, decision.CreatedAt, decision.UpdatedAt, raw)
	return err
}

func (s *Store) GetDecision(sessionID string, decisionID string) (*store.Decision, error) {
	var raw []byte
	err := s.pool.QueryRow(context.Background(), `
		SELECT doc
		FROM decisions
		WHERE session_id = $1 AND decision_id = $2
	`, sessionID, decisionID).Scan(&raw)
	if err != nil {
		return nil, err
	}
	var decision store.Decision
	if err := decodeJSON(raw, &decision); err != nil {
		return nil, err
	}
	return &decision, nil
}

func decisionMatchesFilter(decision store.Decision, filter store.DecisionListFilter) bool {
	if filter.Status != "" && decision.Status != filter.Status && decision.ExecutionStatus != filter.Status {
		return false
	}
	if filter.SubjectRef != "" && decision.SubjectRef != filter.SubjectRef {
		return false
	}
	if filter.FromMs != 0 && decision.CreatedAt < filter.FromMs {
		return false
	}
	if filter.ToMs != 0 && decision.CreatedAt > filter.ToMs {
		return false
	}
	return true
}

func (s *Store) ListSessionDecisions(sessionID string, filter store.DecisionListFilter) ([]store.Decision, error) {
	query, args := buildDecisionListQuery("session_id = $1", []interface{}{sessionID}, filter)
	rows, err := s.pool.Query(context.Background(), query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return decodeDocRows[store.Decision](rows)
}

func (s *Store) ListOrgDecisions(orgID string, filter store.DecisionListFilter) ([]store.Decision, error) {
	query, args := buildDecisionListQuery("org_id = $1", []interface{}{orgID}, filter)
	rows, err := s.pool.Query(context.Background(), query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return decodeDocRows[store.Decision](rows)
}

func buildDecisionListQuery(scopeClause string, args []interface{}, filter store.DecisionListFilter) (string, []interface{}) {
	clauses := []string{scopeClause}
	argPos := len(args) + 1
	if filter.Status != "" {
		clauses = append(clauses, fmt.Sprintf("(status = $%d OR execution_status = $%d)", argPos, argPos))
		args = append(args, filter.Status)
		argPos++
	}
	if filter.SubjectRef != "" {
		clauses = append(clauses, fmt.Sprintf("subject_ref = $%d", argPos))
		args = append(args, filter.SubjectRef)
		argPos++
	}
	if filter.FromMs != 0 {
		clauses = append(clauses, fmt.Sprintf("created_at >= $%d", argPos))
		args = append(args, filter.FromMs)
		argPos++
	}
	if filter.ToMs != 0 {
		clauses = append(clauses, fmt.Sprintf("created_at <= $%d", argPos))
		args = append(args, filter.ToMs)
		argPos++
	}
	limit := filter.Limit
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	query := fmt.Sprintf(`
		SELECT doc
		FROM decisions
		WHERE %s
		ORDER BY created_at DESC, decision_id DESC
		LIMIT $%d
	`, strings.Join(clauses, " AND "), argPos)
	args = append(args, limit)
	return query, args
}

func (s *Store) SaveDecisionDependencies(sessionID string, decisionID string, deps []store.DecisionDependency) error {
	ctx := context.Background()
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	if _, err := tx.Exec(ctx, `DELETE FROM decision_dependencies WHERE session_id = $1 AND decision_id = $2`, sessionID, decisionID); err != nil {
		return err
	}
	for _, dep := range deps {
		dep.SessionID = sessionID
		dep.DecisionID = decisionID
		raw, err := marshalJSON(dep)
		if err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `
			INSERT INTO decision_dependencies (session_id, decision_id, fact_id, dependency_type, required_status, doc)
			VALUES ($1, $2, $3, $4, $5, $6::jsonb)
		`, sessionID, decisionID, dep.FactID, dep.DependencyType, dep.RequiredStatus, raw); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}

func (s *Store) GetDecisionDependencies(sessionID string, decisionID string) ([]store.DecisionDependency, error) {
	rows, err := s.pool.Query(context.Background(), `
		SELECT doc
		FROM decision_dependencies
		WHERE session_id = $1 AND decision_id = $2
		ORDER BY fact_id ASC
	`, sessionID, decisionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return decodeDocRows[store.DecisionDependency](rows)
}

func (s *Store) SaveDecisionCheck(sessionID string, decisionID string, check *store.DecisionCheck) error {
	if check == nil {
		return fmt.Errorf("decision check is required")
	}
	if check.CheckedAt == 0 {
		check.CheckedAt = s.nowMillis()
	}
	check.SessionID = sessionID
	check.DecisionID = decisionID
	raw, err := marshalJSON(check)
	if err != nil {
		return err
	}
	_, err = s.pool.Exec(context.Background(), `
		INSERT INTO decision_checks (session_id, decision_id, checked_at, doc)
		VALUES ($1, $2, $3, $4::jsonb)
	`, sessionID, decisionID, check.CheckedAt, raw)
	return err
}

func (s *Store) GetLatestDecisionCheck(sessionID string, decisionID string) (*store.DecisionCheck, error) {
	var raw []byte
	err := s.pool.QueryRow(context.Background(), `
		SELECT doc
		FROM decision_checks
		WHERE session_id = $1 AND decision_id = $2
		ORDER BY checked_at DESC
		LIMIT 1
	`, sessionID, decisionID).Scan(&raw)
	if err != nil {
		return nil, err
	}
	var check store.DecisionCheck
	if err := decodeJSON(raw, &check); err != nil {
		return nil, err
	}
	return &check, nil
}

func (s *Store) UpsertSearchDocuments(docs []store.SearchDocument) error {
	ctx := context.Background()
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	for _, doc := range docs {
		if doc.ID == "" || doc.OrgID == "" {
			continue
		}
		if doc.CreatedAt == 0 {
			doc.CreatedAt = s.nowMillis()
		}
		if doc.UpdatedAt == 0 {
			doc.UpdatedAt = doc.CreatedAt
		}
		raw, err := marshalJSON(doc)
		if err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `
			INSERT INTO search_documents (
			    org_id, document_id, document_type, session_id, status, subject_ref, target_ref, fact_id, decision_id, title, body, created_at, updated_at, doc
			)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14::jsonb)
			ON CONFLICT (org_id, document_id)
			DO UPDATE SET
			    document_type = EXCLUDED.document_type,
			    session_id = EXCLUDED.session_id,
			    status = EXCLUDED.status,
			    subject_ref = EXCLUDED.subject_ref,
			    target_ref = EXCLUDED.target_ref,
			    fact_id = EXCLUDED.fact_id,
			    decision_id = EXCLUDED.decision_id,
			    title = EXCLUDED.title,
			    body = EXCLUDED.body,
			    created_at = EXCLUDED.created_at,
			    updated_at = EXCLUDED.updated_at,
			    doc = EXCLUDED.doc
		`, doc.OrgID, doc.ID, doc.DocumentType, doc.SessionID, doc.Status, doc.SubjectRef, doc.TargetRef, doc.FactID, doc.DecisionID, doc.Title, doc.Body, doc.CreatedAt, doc.UpdatedAt, raw); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}

func searchDocumentMatches(doc store.SearchDocument, filter store.SearchDocumentsFilter) bool {
	if filter.DocumentType != "" && doc.DocumentType != filter.DocumentType {
		return false
	}
	if filter.Status != "" && doc.Status != filter.Status {
		return false
	}
	if filter.SubjectRef != "" && doc.SubjectRef != filter.SubjectRef {
		return false
	}
	if filter.FromMs != 0 && doc.UpdatedAt < filter.FromMs {
		return false
	}
	if filter.ToMs != 0 && doc.UpdatedAt > filter.ToMs {
		return false
	}
	q := strings.TrimSpace(strings.ToLower(filter.Query))
	if q == "" {
		return true
	}
	return strings.Contains(strings.ToLower(doc.Title), q) ||
		strings.Contains(strings.ToLower(doc.Body), q) ||
		strings.Contains(strings.ToLower(doc.SubjectRef), q) ||
		strings.Contains(strings.ToLower(doc.TargetRef), q) ||
		strings.Contains(strings.ToLower(doc.FactID), q) ||
		strings.Contains(strings.ToLower(doc.DecisionID), q)
}

func (s *Store) SearchDocuments(orgID string, filter store.SearchDocumentsFilter) ([]store.SearchDocument, string, error) {
	clauses := []string{"org_id = $1"}
	args := []interface{}{orgID}
	argPos := 2
	if filter.DocumentType != "" {
		clauses = append(clauses, fmt.Sprintf("document_type = $%d", argPos))
		args = append(args, filter.DocumentType)
		argPos++
	}
	if filter.Status != "" {
		clauses = append(clauses, fmt.Sprintf("status = $%d", argPos))
		args = append(args, filter.Status)
		argPos++
	}
	if filter.SubjectRef != "" {
		clauses = append(clauses, fmt.Sprintf("subject_ref = $%d", argPos))
		args = append(args, filter.SubjectRef)
		argPos++
	}
	if filter.FromMs != 0 {
		clauses = append(clauses, fmt.Sprintf("updated_at >= $%d", argPos))
		args = append(args, filter.FromMs)
		argPos++
	}
	if filter.ToMs != 0 {
		clauses = append(clauses, fmt.Sprintf("updated_at <= $%d", argPos))
		args = append(args, filter.ToMs)
		argPos++
	}
	q := strings.TrimSpace(strings.ToLower(filter.Query))
	if q != "" {
		clauses = append(clauses, fmt.Sprintf(`(
			LOWER(title) LIKE $%d OR
			LOWER(body) LIKE $%d OR
			LOWER(subject_ref) LIKE $%d OR
			LOWER(target_ref) LIKE $%d OR
			LOWER(fact_id) LIKE $%d OR
			LOWER(decision_id) LIKE $%d
		)`, argPos, argPos, argPos, argPos, argPos, argPos))
		args = append(args, "%"+q+"%")
		argPos++
	}
	limit := filter.Limit
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	offset := parseOffsetCursor(filter.Cursor)
	query := fmt.Sprintf(`
		SELECT doc
		FROM search_documents
		WHERE %s
		ORDER BY updated_at DESC, document_id DESC
		LIMIT $%d OFFSET $%d
	`, strings.Join(clauses, " AND "), argPos, argPos+1)
	args = append(args, limit+1, offset)
	rows, err := s.pool.Query(context.Background(), query, args...)
	if err != nil {
		return nil, "", err
	}
	defer rows.Close()

	items := []store.SearchDocument{}
	for rows.Next() {
		var raw []byte
		if err := rows.Scan(&raw); err != nil {
			return nil, "", err
		}
		var item store.SearchDocument
		if err := decodeJSON(raw, &item); err != nil {
			continue
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, "", err
	}
	next := ""
	if len(items) > limit {
		items = items[:limit]
		next = fmt.Sprintf("o:%d", offset+limit)
	}
	return items, next, nil
}
