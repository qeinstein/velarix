package core

import (
	"fmt"
	"strconv"
	"strings"
	"time"
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
func dependencySatisfied(parent *Fact, parentStatus Status, negated bool, child *Fact) bool {
	// Negated dependencies are satisfied by parent invalidity/absence; grounding
	// and scope checks are irrelevant for that polarity.
	if negated {
		return parentStatus < ConfidenceThreshold
	}

	// Treat "" as empirical scope.
	childScope := AssertionKindEmpirical
	if child != nil && strings.TrimSpace(child.AssertionKind) != "" {
		childScope = strings.TrimSpace(child.AssertionKind)
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

	// Grounding policy: execution-critical / action facts can enforce stronger
	// provenance+verification requirements on their dependencies.
	if child != nil && child.Metadata != nil {
		allowed, _ := child.Metadata["grounding_allowed_source_types"].([]string)
		if len(allowed) == 0 {
			// Handle []interface{} from JSON decode.
			if raw, ok := child.Metadata["grounding_allowed_source_types"].([]interface{}); ok {
				for _, item := range raw {
					if s, ok := item.(string); ok && strings.TrimSpace(s) != "" {
						allowed = append(allowed, strings.TrimSpace(s))
					}
				}
			}
		}
		if len(allowed) > 0 && parent != nil {
			st := FactSourceType(parent)
			ok := false
			for _, a := range allowed {
				if strings.EqualFold(strings.TrimSpace(a), st) {
					ok = true
					break
				}
			}
			if !ok {
				return false
			}
		}

		if MetadataBool(child.Metadata, "grounding_require_verified") && parent != nil {
			if FactVerificationStatus(parent) != VerificationVerified {
				return false
			}
		}

		maxAgeSeconds := int64(0)
		if raw, ok := child.Metadata["grounding_max_age_seconds"]; ok {
			switch v := raw.(type) {
			case float64:
				maxAgeSeconds = int64(v)
			case int64:
				maxAgeSeconds = v
			case int:
				maxAgeSeconds = int64(v)
			case string:
				if parsed, err := strconv.ParseInt(strings.TrimSpace(v), 10, 64); err == nil {
					maxAgeSeconds = parsed
				}
			}
		}
		if maxAgeSeconds > 0 && parent != nil && parent.AssertedAt > 0 {
			if time.Now().UnixMilli()-parent.AssertedAt > maxAgeSeconds*1000 {
				return false
			}
		}
	}

	return parentStatus >= ConfidenceThreshold
}
