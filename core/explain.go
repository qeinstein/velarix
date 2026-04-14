package core

import (
	"errors"
	"fmt"
	"sort"
	"strings"
)

// BeliefExplanation represents a single fact in a causal explanation chain.
type BeliefExplanation struct {
	FactID     string  `json:"fact_id"`
	Confidence float64 `json:"confidence"`
	// Tier is one of "certain" (>0.9), "probable" (0.6–0.9), "uncertain" (<0.6).
	// For facts with AssertionKind "uncertain", the tier is capped at "probable"
	// regardless of the numeric confidence, reflecting epistemic hedging.
	Tier           string                 `json:"tier"`
	AssertionKind  string                 `json:"assertion_kind,omitempty"`
	Provenance     map[string]interface{} `json:"provenance,omitempty"`
	Payload        map[string]interface{} `json:"payload,omitempty"`
	IsRoot         bool                   `json:"is_root"`
	Parents        []string               `json:"parents,omitempty"`
	NegatedParents []string               `json:"negated_parents,omitempty"`
}

// CounterfactualResult describes what would change if a specific fact were removed.
type CounterfactualResult struct {
	RemovedFactID string   `json:"removed_fact_id"`
	ImpactedFacts []string `json:"impacted_facts"`
	DirectCount   int      `json:"direct_count"`
	TotalCount    int      `json:"total_count"`
	EpistemicLoss float64  `json:"epistemic_loss"`
	Narrative     string   `json:"narrative"`
}

// ExplanationSource is documented here.
type ExplanationSource struct {
	FactID        string `json:"fact_id"`
	SourceType    string `json:"source_type,omitempty"`
	SourceRef     string `json:"source_ref,omitempty"`
	PayloadHash   string `json:"payload_hash,omitempty"`
	PolicyVersion string `json:"policy_version,omitempty"`
}

// ExplanationOutput is the full structured explanation returned by ExplainReasoning.
type ExplanationOutput struct {
	FactID             string                 `json:"fact_id"`
	SessionID          string                 `json:"session_id"`
	Timestamp          int64                  `json:"timestamp"`
	Summary            string                 `json:"summary,omitempty"`
	Structured         map[string]interface{} `json:"structured,omitempty"`
	InvalidatedFactIDs []string               `json:"invalidated_fact_ids,omitempty"`
	Sources            []ExplanationSource    `json:"sources,omitempty"`
	PolicyVersions     []string               `json:"policy_versions,omitempty"`
	CausalChain        []BeliefExplanation    `json:"causal_chain"`
	Counterfactual     *CounterfactualResult  `json:"counterfactual,omitempty"`
}

// confidenceTier returns a human-readable tier for a confidence score.
// When assertionKind is "uncertain" the tier is capped at "probable" even if
// the numeric confidence exceeds 0.9, because the fact itself is epistemically
// hedged (e.g. "X is probably the CEO"). This prevents the engine from
// presenting logically consistent but factually nuanced claims as "certain".
func confidenceTier(confidence float64, assertionKind string) string {
	if assertionKind == AssertionKindUncertain {
		if confidence >= float64(ConfidenceThreshold) {
			return "probable"
		}
		return "uncertain"
	}
	if confidence > 0.9 {
		return "certain"
	}
	if confidence >= float64(ConfidenceThreshold) {
		return "probable"
	}
	return "uncertain"
}

// ExplainReasoning generates a structured, confidence-weighted causal explanation
// for a fact by walking the justification graph. This is the canonical explain
// method; the legacy Explain() tree method has been removed.
func (e *Engine) ExplainReasoning(factID string) (*ExplanationOutput, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	fact, exists := e.Facts[factID]
	if !exists {
		return nil, errors.New("fact not found")
	}

	output := &ExplanationOutput{
		FactID: factID,
	}

	// Build causal chain by walking the justification graph.
	visited := make(map[string]struct{})
	output.CausalChain = e.buildCausalChain(fact, visited)

	output.enrichForDecisionContext(fact, float64(e.effectiveStatusUnsafe(fact)))

	return output, nil
}

func (o *ExplanationOutput) enrichForDecisionContext(fact *Fact, currentStatus float64) {
	if fact == nil {
		return
	}
	invalidated := map[string]struct{}{}
	policyVersions := map[string]struct{}{}
	sources := []ExplanationSource{}

	for _, belief := range o.CausalChain {
		if belief.Confidence < float64(ConfidenceThreshold) {
			invalidated[belief.FactID] = struct{}{}
		}

		source := ExplanationSource{FactID: belief.FactID}
		if belief.Provenance != nil {
			if v, ok := belief.Provenance["source_type"].(string); ok {
				source.SourceType = v
			}
			if v, ok := belief.Provenance["source_ref"].(string); ok {
				source.SourceRef = v
			}
			if v, ok := belief.Provenance["payload_hash"].(string); ok {
				source.PayloadHash = v
			}
			if v, ok := belief.Provenance["policy_version"].(string); ok {
				source.PolicyVersion = v
				policyVersions[v] = struct{}{}
			}
		}
		if source.SourceType != "" || source.SourceRef != "" || source.PayloadHash != "" || source.PolicyVersion != "" {
			sources = append(sources, source)
		}
	}

	o.InvalidatedFactIDs = make([]string, 0, len(invalidated))
	for id := range invalidated {
		o.InvalidatedFactIDs = append(o.InvalidatedFactIDs, id)
	}
	sort.Strings(o.InvalidatedFactIDs)
	o.Sources = sources

	o.PolicyVersions = make([]string, 0, len(policyVersions))
	for version := range policyVersions {
		o.PolicyVersions = append(o.PolicyVersions, version)
	}
	sort.Strings(o.PolicyVersions)

	if currentStatus < float64(ConfidenceThreshold) {
		o.Summary = fmt.Sprintf("Fact %s is blocked or stale because one or more dependencies are no longer valid.", fact.ID)
	} else {
		o.Summary = fmt.Sprintf("Fact %s is supported by the current dependency set.", fact.ID)
	}
	o.Structured = map[string]interface{}{
		"fact_id":              fact.ID,
		"current_status":       currentStatus,
		"invalidated_fact_ids": o.InvalidatedFactIDs,
		"policy_versions":      o.PolicyVersions,
		"source_count":         len(o.Sources),
	}
}

// buildCausalChain recursively walks the justification graph and builds a flat list of belief explanations.
func (e *Engine) buildCausalChain(fact *Fact, visited map[string]struct{}) []BeliefExplanation {
	if _, seen := visited[fact.ID]; seen {
		return nil
	}
	visited[fact.ID] = struct{}{}

	conf := float64(e.effectiveStatusUnsafe(fact))

	belief := BeliefExplanation{
		FactID:        fact.ID,
		Confidence:    conf,
		Tier:          confidenceTier(conf, fact.AssertionKind),
		AssertionKind: fact.AssertionKind,
		Payload:       fact.Payload,
		IsRoot:        fact.IsRoot,
	}

	// Attach provenance from metadata if available
	if fact.Metadata != nil {
		if prov, ok := fact.Metadata["_provenance"]; ok {
			if provMap, ok := prov.(map[string]interface{}); ok {
				belief.Provenance = provMap
			}
		}
	}

	var chain []BeliefExplanation

	// For non-root facts, walk parents
	if !fact.IsRoot {
		for _, set := range fact.JustificationSets {
			for _, parentID := range set {
				isNegated := strings.HasPrefix(parentID, "!")
				cleanID := strings.TrimPrefix(parentID, "!")

				if isNegated {
					belief.NegatedParents = append(belief.NegatedParents, cleanID)
				} else {
					belief.Parents = append(belief.Parents, cleanID)
				}
				if parentFact, ok := e.Facts[cleanID]; ok {
					chain = append(chain, e.buildCausalChain(parentFact, visited)...)
				}
			}
		}
	}

	// The fact itself goes first, followed by its ancestors
	return append([]BeliefExplanation{belief}, chain...)
}

// computeCounterfactual uses impact analysis to determine what would change
// if the given counterfactual fact were removed.
func (e *Engine) computeCounterfactual(targetFactID, counterfactualFactID string) *CounterfactualResult {
	// Use the existing GetImpact logic (note: GetImpact takes its own lock, but we hold RLock
	// so we need to compute impact inline here without double-locking)
	report := &CounterfactualResult{
		RemovedFactID: counterfactualFactID,
		ImpactedFacts: []string{},
	}

	// Check if target fact is in the impact zone of the counterfactual fact
	isTargetImpacted := false

	for id, f := range e.Facts {
		if id == counterfactualFactID {
			continue
		}
		if e.isDominatorAncestor(counterfactualFactID, id) {
			report.ImpactedFacts = append(report.ImpactedFacts, id)
			report.TotalCount++
			report.EpistemicLoss += float64(f.DerivedStatus)

			if f.IDom == counterfactualFactID {
				report.DirectCount++
			}
			if id == targetFactID {
				isTargetImpacted = true
			}
		}
	}

	if isTargetImpacted {
		report.Narrative = "If '" + counterfactualFactID + "' had not existed, '" + targetFactID + "' would have been invalidated because it depends on this fact through the causal chain."
	} else {
		report.Narrative = "If '" + counterfactualFactID + "' had not existed, '" + targetFactID + "' would remain unaffected because it does not depend on this fact."
	}

	return report
}
