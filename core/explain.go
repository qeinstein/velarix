package core

import "errors"

// BeliefExplanation represents a single fact in a causal explanation chain.
type BeliefExplanation struct {
	FactID     string                 `json:"fact_id"`
	Confidence float64                `json:"confidence"`
	Tier       string                 `json:"tier"` // "certain" (>0.9), "probable" (0.6-0.9), "uncertain" (<0.6)
	Provenance map[string]interface{} `json:"provenance,omitempty"`
	Payload    map[string]interface{} `json:"payload,omitempty"`
	IsRoot     bool                   `json:"is_root"`
	Parents    []string               `json:"parents,omitempty"`
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

// ExplanationOutput is the full structured explanation returned by ExplainReasoning.
type ExplanationOutput struct {
	FactID         string                `json:"fact_id"`
	SessionID      string                `json:"session_id"`
	Timestamp      int64                 `json:"timestamp"`
	CausalChain    []BeliefExplanation   `json:"causal_chain"`
	Counterfactual *CounterfactualResult `json:"counterfactual,omitempty"`
}

// confidenceTier returns a human-readable tier for a confidence score.
func confidenceTier(confidence float64) string {
	if confidence > 0.9 {
		return "certain"
	}
	if confidence >= 0.6 {
		return "probable"
	}
	return "uncertain"
}

// ExplainReasoning generates a structured, confidence-weighted causal explanation for a fact.
// If counterfactualFactID is non-empty, it computes what would change if that fact were removed.
func (e *Engine) ExplainReasoning(factID string, counterfactualFactID string) (*ExplanationOutput, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	fact, exists := e.Facts[factID]
	if !exists {
		return nil, errors.New("fact not found")
	}

	output := &ExplanationOutput{
		FactID: factID,
	}

	// Build causal chain by walking the justification graph
	visited := make(map[string]struct{})
	output.CausalChain = e.buildCausalChain(fact, visited)

	// Counterfactual analysis
	if counterfactualFactID != "" {
		if _, cfExists := e.Facts[counterfactualFactID]; !cfExists {
			return nil, errors.New("counterfactual fact not found: " + counterfactualFactID)
		}
		output.Counterfactual = e.computeCounterfactual(factID, counterfactualFactID)
	}

	return output, nil
}

// buildCausalChain recursively walks the justification graph and builds a flat list of belief explanations.
func (e *Engine) buildCausalChain(fact *Fact, visited map[string]struct{}) []BeliefExplanation {
	if _, seen := visited[fact.ID]; seen {
		return nil
	}
	visited[fact.ID] = struct{}{}

	conf := float64(fact.DerivedStatus)
	if fact.IsRoot {
		conf = float64(fact.ManualStatus)
	}

	belief := BeliefExplanation{
		FactID:     fact.ID,
		Confidence: conf,
		Tier:       confidenceTier(conf),
		Payload:    fact.Payload,
		IsRoot:     fact.IsRoot,
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
				belief.Parents = append(belief.Parents, parentID)
				if parentFact, ok := e.Facts[parentID]; ok {
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
	targetInImpact := false

	for id, f := range e.Facts {
		if id == counterfactualFactID {
			continue
		}
		if e.isDominatorAncestorUnsafe(counterfactualFactID, id) {
			report.ImpactedFacts = append(report.ImpactedFacts, id)
			report.TotalCount++
			report.EpistemicLoss += float64(f.DerivedStatus)

			if f.IDom == counterfactualFactID {
				report.DirectCount++
			}
			if id == targetFactID {
				targetInImpact = true
			}
		}
	}

	cfFact, _ := e.Facts[counterfactualFactID]
	targetFact, _ := e.Facts[targetFactID]

	if targetInImpact {
		report.Narrative = "If '" + counterfactualFactID + "' had not existed, '" + targetFactID + "' would have been invalidated because it depends on this fact through the causal chain."
	} else {
		report.Narrative = "If '" + counterfactualFactID + "' had not existed, '" + targetFactID + "' would remain unaffected because it does not depend on this fact."
	}

	_ = cfFact
	_ = targetFact

	return report
}

// isDominatorAncestorUnsafe checks dominator ancestry without taking locks.
// Caller MUST hold at least e.mu.RLock().
func (e *Engine) isDominatorAncestorUnsafe(uID, vID string) bool {
	u, okU := e.Facts[uID]
	v, okV := e.Facts[vID]
	if !okU || !okV {
		return false
	}
	return u.PreOrder <= v.PreOrder && u.PostOrder >= v.PostOrder
}

// Legacy support: Explain returns a tree showing why a fact is currently Valid (backward compat).
type ExplanationNode struct {
	FactID   string
	Children []*ExplanationNode
}

func (e *Engine) Explain(factID string) ([]*ExplanationNode, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	fact, exists := e.Facts[factID]
	if !exists {
		return nil, errors.New("fact not found")
	}

	if fact.DerivedStatus != Valid {
		return []*ExplanationNode{}, nil
	}

	return e.explainFact(factID, make(map[string]struct{})), nil
}

func (e *Engine) explainFact(factID string, visited map[string]struct{}) []*ExplanationNode {
	if _, seen := visited[factID]; seen {
		return nil
	}
	visited[factID] = struct{}{}

	fact := e.Facts[factID]

	if fact.IsRoot {
		return []*ExplanationNode{
			{
				FactID:   factID,
				Children: nil,
			},
		}
	}

	var explanations []*ExplanationNode

	for _, set := range fact.JustificationSets {
		setValid := true
		for _, parentID := range set {
			parent := e.Facts[parentID]
			if parent.DerivedStatus != Valid {
				setValid = false
				break
			}
		}
		if !setValid {
			continue
		}

		node := &ExplanationNode{
			FactID:   factID,
			Children: []*ExplanationNode{},
		}
		for _, parentID := range set {
			childExplanations := e.explainFact(parentID, visited)
			if len(childExplanations) > 0 {
				node.Children = append(node.Children, childExplanations[0])
			}
		}
		explanations = append(explanations, node)
	}

	return explanations
}
