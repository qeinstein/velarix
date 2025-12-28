package core

// Authoritative Runtime
import "errors"

type Engine struct{
	//Fact ID -- Fact
	Facts map[string]*Fact

	// Foward deoendency graph
	// parent id -- set(child_ids)
	ChildrenIndex map[string]map[string]struct{}
}

// New Engine creates an empty casual engine
func NewEngine() *Engine{
	return &Engine{
		Facts: make(map[string]*Fact),
		ChildrenIndex: make(map[string]map[string]struct{}),
	}
}

// Implementation for evaluating nodes below
func (e *Engine) evaluateNode(factID string, visited map[string]struct{}) {
	// Re-entrancy guard
	if _, seen := visited[factID]; seen{
		return
	}

	visited[factID] = struct{}{}
	fact, exists := e.Facts[factID]
	if !exists {
		return 	// this should never happen if the engine is used correctly
	}

	oldStatus := fact.DerivedStatus

	// First need to compute the new derived status
	if fact.IsRoot {
		fact.DerivedStatus = fact.ManualStatus
	}else {
		newStatus := Invalid	// 0

		for _, justificationSet := range fact.JustificationSets {
			setSatisfied := true

			for _, parentID := range justificationSet{
				parent, ok := e.Facts[parentID]
				
				if !ok || parent.DerivedStatus != Valid {
					setSatisfied =  false

					break
				}
			}

			// OR-of-ANDSs.. one satisfied set is enough
			if setSatisfied {
				newStatus = Valid
				break
			}
		}

		fact.DerivedStatus =  newStatus
	}

	// propagate only if state changed
	if fact.DerivedStatus != oldStatus{
		children := e.ChildrenIndex[factID]
		for childID := range children {
			e.evaluateNode(childID, visited)
		}
	}
}


func (e *Engine) AssertFact(f *Fact) error {
	// Id must be unique
	if _, exists := e.Facts[f.ID]; exists {
		return errors.New("a fact with this ID already exists")
	}

	// Non-root facts must have  justification sets
	if !f.IsRoot && len(f.JustificationSets) == 0 {
		return errors.New("non-root fact must have at least one justification set")
	}

	// validate justification sets
	for _, set := range f.JustificationSets {
		if len(set) == 0{
			return errors.New("justification set cannot be empty")
		}

		for _, parentID := range set{
			if _, ok := e.Facts[parentID]; !ok {   
				return errors.New("unknown parent fact: "+ parentID)  
			}
		}
	}

	if err := e.detectCycle(f); err != nil {
		return err
	}

	// insert fact
	e.Facts[f.ID] = f

	// Initialize derived status pessimistically
	f.DerivedStatus	= Invalid

	// Update Children index
	for _, set := range f.JustificationSets{
		for _, parentID := range set {
			if _, ok := e.ChildrenIndex[parentID]; !ok {
				e.ChildrenIndex[parentID] = make(map[string]struct{})
			}
			e.ChildrenIndex[parentID][f.ID] = struct{}{}
		}
	}

	e.evaluateNode(f.ID, make(map[string]struct{}))
	
	return nil
}


func (e *Engine) InvalidateRoot(factID string) error {
	fact, exists := e.Facts[factID]
	if !exists {
		return errors.New("fact not found")
	}

	if !fact.IsRoot {
		return errors.New("cannot invalidate non-root fact")
	}

	// avoid unnecesary riples
	if fact.ManualStatus == Invalid {
		return nil
	}

	fact.ManualStatus = Invalid

	// Re-evaluate starting from this root
	e.evaluateNode(factID, make(map[string]struct{}))

	return nil
}
