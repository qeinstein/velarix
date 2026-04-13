package core

import (
	"fmt"
	"sort"
	"strings"
)

type ConsistencyIssue struct {
	Type               string   `json:"type"`
	Severity           string   `json:"severity"`
	FactIDs            []string `json:"fact_ids"`
	Message            string   `json:"message"`
	SuggestedAction    string   `json:"suggested_action,omitempty"`
	Source             string   `json:"source,omitempty"`
	VerifierModel      string   `json:"verifier_model,omitempty"`
	VerifierLabel      string   `json:"verifier_label,omitempty"`
	VerifierReason     string   `json:"verifier_reason,omitempty"`
	VerifierConfidence float64  `json:"verifier_confidence,omitempty"`
}

type ConsistencyReport struct {
	CheckedFactIDs []string           `json:"checked_fact_ids"`
	IssueCount     int                `json:"issue_count"`
	Issues         []ConsistencyIssue `json:"issues"`
}

type claimSignature struct {
	ClaimKey    string
	ClaimValue  string
	Subject     string
	Predicate   string
	Object      string
	Polarity    string
	Contradicts []string
}

func getMapString(m map[string]interface{}, key string) string {
	if m == nil {
		return ""
	}
	if v, ok := m[key].(string); ok {
		return strings.TrimSpace(v)
	}
	return ""
}

func getMapStringSlice(m map[string]interface{}, key string) []string {
	if m == nil {
		return nil
	}
	raw, ok := m[key]
	if !ok {
		return nil
	}
	switch v := raw.(type) {
	case []string:
		return append([]string(nil), v...)
	case []interface{}:
		out := make([]string, 0, len(v))
		for _, item := range v {
			if s, ok := item.(string); ok && strings.TrimSpace(s) != "" {
				out = append(out, strings.TrimSpace(s))
			}
		}
		return out
	default:
		return nil
	}
}

func extractClaimSignature(f *Fact) claimSignature {
	if f == nil {
		return claimSignature{}
	}
	sig := claimSignature{
		ClaimKey:   getMapString(f.Payload, "claim_key"),
		ClaimValue: getMapString(f.Payload, "claim_value"),
		Subject:    getMapString(f.Payload, "subject"),
		Predicate:  getMapString(f.Payload, "predicate"),
		Object:     getMapString(f.Payload, "object"),
		Polarity:   strings.ToLower(firstNonEmptyString(getMapString(f.Payload, "polarity"), getMapString(f.Metadata, "polarity"))),
	}
	if sig.ClaimKey == "" {
		sig.ClaimKey = getMapString(f.Metadata, "claim_key")
	}
	if sig.ClaimValue == "" {
		sig.ClaimValue = getMapString(f.Metadata, "claim_value")
	}
	if sig.Subject == "" {
		sig.Subject = getMapString(f.Metadata, "subject")
	}
	if sig.Predicate == "" {
		sig.Predicate = getMapString(f.Metadata, "predicate")
	}
	if sig.Object == "" {
		sig.Object = getMapString(f.Metadata, "object")
	}
	sig.Contradicts = append(sig.Contradicts, getMapStringSlice(f.Payload, "contradicts")...)
	sig.Contradicts = append(sig.Contradicts, getMapStringSlice(f.Metadata, "contradicts")...)
	if sig.Polarity == "" {
		sig.Polarity = "positive"
	}
	return sig
}

func firstNonEmptyString(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}

func containsString(values []string, target string) bool {
	for _, v := range values {
		if v == target {
			return true
		}
	}
	return false
}

func semanticNegationMismatch(a, b *Fact) bool {
	textA := strings.ToLower(factSemanticText(a))
	textB := strings.ToLower(factSemanticText(b))
	if textA == "" || textB == "" {
		return false
	}
	score := CosineSimilarity(EmbeddingForFact(a), EmbeddingForFact(b))
	if score < 0.92 {
		return false
	}
	hasNegA := strings.Contains(textA, " not ") || strings.Contains(textA, " no ") || strings.Contains(textA, " never ")
	hasNegB := strings.Contains(textB, " not ") || strings.Contains(textB, " no ") || strings.Contains(textB, " never ")
	return hasNegA != hasNegB
}

func contradictionIssueForFacts(a, b *Fact) (ConsistencyIssue, bool) {
	if a == nil || b == nil {
		return ConsistencyIssue{}, false
	}
	sigA := extractClaimSignature(a)
	sigB := extractClaimSignature(b)

	issue := ConsistencyIssue{
		Severity:        "high",
		FactIDs:         []string{a.ID, b.ID},
		SuggestedAction: "retract or downgrade one of the conflicting facts before execution",
	}

	switch {
	case containsString(sigA.Contradicts, b.ID) || containsString(sigB.Contradicts, a.ID):
		issue.Type = "explicit_contradiction"
		issue.Message = fmt.Sprintf("Facts %s and %s are marked as contradictory.", a.ID, b.ID)
		return issue, true
	case sigA.ClaimKey != "" && sigA.ClaimKey == sigB.ClaimKey && sigA.ClaimValue != "" && sigB.ClaimValue != "" && sigA.ClaimValue != sigB.ClaimValue:
		issue.Type = "claim_value_conflict"
		issue.Message = fmt.Sprintf("Facts %s and %s assert different values for claim_key=%s.", a.ID, b.ID, sigA.ClaimKey)
		return issue, true
	case sigA.Subject != "" && sigA.Predicate != "" && sigA.Subject == sigB.Subject && sigA.Predicate == sigB.Predicate && sigA.Object != "" && sigB.Object != "" && sigA.Object != sigB.Object:
		issue.Type = "predicate_object_conflict"
		issue.Message = fmt.Sprintf("Facts %s and %s disagree on %s/%s.", a.ID, b.ID, sigA.Subject, sigA.Predicate)
		return issue, true
	case sigA.Subject != "" && sigA.Predicate != "" && sigA.Object != "" && sigA.Subject == sigB.Subject && sigA.Predicate == sigB.Predicate && sigA.Object == sigB.Object && sigA.Polarity != sigB.Polarity:
		issue.Type = "polarity_conflict"
		issue.Message = fmt.Sprintf("Facts %s and %s assert opposite polarity for %s/%s/%s.", a.ID, b.ID, sigA.Subject, sigA.Predicate, sigA.Object)
		return issue, true
	case semanticNegationMismatch(a, b):
		issue.Type = "semantic_negation_conflict"
		issue.Severity = "medium"
		issue.Message = fmt.Sprintf("Facts %s and %s are semantically close but appear to negate each other.", a.ID, b.ID)
		return issue, true
	default:
		return ConsistencyIssue{}, false
	}
}

func uniqueSortedFactIDs(ids []string) []string {
	seen := map[string]struct{}{}
	out := make([]string, 0, len(ids))
	for _, id := range ids {
		id = strings.TrimSpace(id)
		if id == "" {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		out = append(out, id)
	}
	sort.Strings(out)
	return out
}

func (e *Engine) consistencyIssuesForIDsUnsafe(factIDs []string, includeInvalid bool) []ConsistencyIssue {
	ids := uniqueSortedFactIDs(factIDs)
	if len(ids) == 0 {
		for id := range e.Facts {
			ids = append(ids, id)
		}
		sort.Strings(ids)
	}

	issues := []ConsistencyIssue{}
	seenPairs := map[string]struct{}{}

	for i := 0; i < len(ids); i++ {
		a, ok := e.Facts[ids[i]]
		if !ok {
			continue
		}
		statusA := e.effectiveStatusUnsafe(a)
		if !includeInvalid && statusA < ConfidenceThreshold {
			continue
		}

		for j := i + 1; j < len(ids); j++ {
			b, ok := e.Facts[ids[j]]
			if !ok {
				continue
			}
			statusB := e.effectiveStatusUnsafe(b)
			if !includeInvalid && statusB < ConfidenceThreshold {
				continue
			}
			issue, ok := contradictionIssueForFacts(a, b)
			if !ok {
				continue
			}
			pairKey := issue.Type + ":" + ids[i] + ":" + ids[j]
			if _, exists := seenPairs[pairKey]; exists {
				continue
			}
			seenPairs[pairKey] = struct{}{}
			issues = append(issues, issue)
		}
	}

	sort.Slice(issues, func(i, j int) bool {
		if issues[i].Severity == issues[j].Severity {
			return strings.Join(issues[i].FactIDs, ",") < strings.Join(issues[j].FactIDs, ",")
		}
		return issues[i].Severity < issues[j].Severity
	})
	return issues
}

func (e *Engine) CheckConsistency(factIDs []string, includeInvalid bool) *ConsistencyReport {
	e.mu.RLock()
	defer e.mu.RUnlock()

	ids := uniqueSortedFactIDs(factIDs)
	if len(ids) == 0 {
		for id := range e.Facts {
			ids = append(ids, id)
		}
		sort.Strings(ids)
	}

	issues := e.consistencyIssuesForIDsUnsafe(ids, includeInvalid)
	return &ConsistencyReport{
		CheckedFactIDs: ids,
		IssueCount:     len(issues),
		Issues:         issues,
	}
}
