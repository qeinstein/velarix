package core

import "errors"

type ExplanationNode struct {
	FactID   string
	Children []*ExplanationNode
}

// Explain returns a tree showing why a fact is currently Valid.
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
	// Prevent infinite recursion (defensive)
	if _, seen := visited[factID]; seen {
		return nil
	}
	visited[factID] = struct{}{}

	fact := e.Facts[factID]

	// Base case: root fact
	if fact.IsRoot {
		return []*ExplanationNode{
			{
				FactID:   factID,
				Children: nil,
			},
		}
	}

	var explanations []*ExplanationNode

	// For each justification set (using the original slices for explanation traversal)
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

		// Build explanation subtree for this justification set
		node := &ExplanationNode{
			FactID:   factID,
			Children: []*ExplanationNode{},
		}

		for _, parentID := range set {
			childExplanations := e.explainFact(parentID, visited)

			// Each parent should yield exactly one explanation path
			if len(childExplanations) > 0 {
				node.Children = append(node.Children, childExplanations[0])
			}
		}

		explanations = append(explanations, node)
	}

	return explanations
}
