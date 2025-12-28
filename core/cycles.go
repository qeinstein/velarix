package core

import "errors"

func (e *Engine) detectCycle(newFact *Fact) error {
	newID := newFact.ID

	for _, set := range newFact.JustificationSets{
		for _, parentID := range set{
			visited := make(map[string]struct{})
			if e.reachable(parentID, newID, visited){		//implement reachable down
				return errors.New("cycle detected involving fact: " + newID)
			}
		}
	}

	return nil
}		// fails first on first detected cycle and isolayes traversal per parent


func (e *Engine) reachable(fromID, targetID string, visited map[string]struct{}) bool {
	if fromID == targetID{
		return true
	}

	if _, seen := visited[fromID]; seen {
		return false
	}
	visited[fromID] = struct{}{}

	children := e.ChildrenIndex[fromID]
	for childID := range children {
		if e.reachable(childID, targetID, visited){
			return true
		}
	}

	return false
}


