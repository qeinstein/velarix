package core

import (
	"fmt"
	"strings"
)

const negatedDependencyPrefix = "!"

// DependencyRef is documented here.
type DependencyRef struct {
	FactID   string
	Negated  bool
	Original string
}

// ParseDependencyRef is documented here.
func ParseDependencyRef(raw string) (DependencyRef, error) {
	token := strings.TrimSpace(raw)
	if token == "" {
		return DependencyRef{}, fmt.Errorf("dependency token cannot be empty")
	}
	ref := DependencyRef{Original: token}
	if strings.HasPrefix(token, negatedDependencyPrefix) {
		ref.Negated = true
		token = strings.TrimSpace(strings.TrimPrefix(token, negatedDependencyPrefix))
	}
	if token == "" {
		return DependencyRef{}, fmt.Errorf("dependency token %q is missing a fact id", raw)
	}
	ref.FactID = token
	return ref, nil
}

func normalizeDependencyToken(raw string) (string, error) {
	ref, err := ParseDependencyRef(raw)
	if err != nil {
		return "", err
	}
	return ref.FactID, nil
}

func splitDependencySet(tokens []string) (positive []string, negative []string, all []string, err error) {
	seen := map[string]struct{}{}
	for _, raw := range tokens {
		ref, parseErr := ParseDependencyRef(raw)
		if parseErr != nil {
			return nil, nil, nil, parseErr
		}
		key := ref.FactID
		if ref.Negated {
			key = negatedDependencyPrefix + key
		}
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		all = append(all, ref.FactID)
		if ref.Negated {
			negative = append(negative, ref.FactID)
			continue
		}
		positive = append(positive, ref.FactID)
	}
	return positive, negative, all, nil
}

func dependencyConfidence(status Status, negated bool) Status {
	if !negated {
		return status
	}
	if status <= Invalid {
		return Valid
	}
	if status >= Valid {
		return Invalid
	}
	return Valid - status
}

// dependencySatisfied resolves whether a dependency is satisfied,
// taking into account AssertionKind scoping rules.
//
// A hypothetical or fictional fact cannot ground an empirical/uncertain derived
// fact. However, hypothetical/fictional derived facts may depend on parents in
// the same scope.
func dependencySatisfied(parent *Fact, parentStatus Status, negated bool, childAssertionKind string) bool {
	// Treat "" as empirical scope.
	childScope := childAssertionKind
	if strings.TrimSpace(childScope) == "" {
		childScope = AssertionKindEmpirical
	}

	if parent != nil {
		parentScope := strings.TrimSpace(parent.AssertionKind)
		if parentScope == "" {
			parentScope = AssertionKindEmpirical
		}
		if (parentScope == AssertionKindFictional || parentScope == AssertionKindHypothetical) &&
			(childScope == AssertionKindEmpirical || childScope == AssertionKindUncertain) {
			return false
		}
	}

	if negated {
		return parentStatus < ConfidenceThreshold
	}
	return parentStatus >= ConfidenceThreshold
}
