package core

import (
	"fmt"
	"strings"
)

// CycleError represents a circular dependency error with the exact path.
type CycleError struct {
	Path []string
}

func (e *CycleError) Error() string {
	return fmt.Sprintf("Justification cycle detected. Fact '%s' cannot depend on '%s' because a cycle exists: %s. Please retract one of these facts to proceed.", 
		e.Path[len(e.Path)-1], e.Path[0], strings.Join(e.Path, " -> "))
}

// detectCycle checks if adding newFact would create a circular dependency.
// Callers MUST hold e.mu.Lock() or e.mu.RLock().
func (e *Engine) detectCycle(newFact *Fact) error {
	newID := newFact.ID

	for _, set := range newFact.JustificationSets {
		for _, parentID := range set {
			visited := make(map[string]struct{})
			if path := e.reachable(parentID, newID, visited); path != nil {
				// The path goes from parentID to newID. 
				// The cycle is newID -> parentID -> ... -> newID
				fullPath := append([]string{newID}, path...)
				return &CycleError{Path: fullPath}
			}
		}
	}

	return nil
}

func (e *Engine) reachable(fromID, targetID string, visited map[string]struct{}) []string {
	if fromID == targetID {
		return []string{targetID}
	}

	if _, seen := visited[fromID]; seen {
		return nil
	}
	visited[fromID] = struct{}{}

	jSets := e.ChildrenIndex[fromID]
	for jSetID := range jSets {
		jSet, ok := e.JustificationSets[jSetID]
		if !ok {
			continue
		}
		if subPath := e.reachable(jSet.ChildFactID, targetID, visited); subPath != nil {
			return append([]string{fromID}, subPath...)
		}
	}

	return nil
}
