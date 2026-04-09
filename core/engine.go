package core

import (
	"bytes"
	"encoding/gob"
	"errors"
	"fmt"
	"hash/crc32"
	"reflect"
	"sort"
	"sync"
	"time"
)

const ConfidenceThreshold Status = 0.6

type ChangeEvent struct {
	FactID    string `json:"fact_id"`
	Status    Status `json:"status"`
	Timestamp int64  `json:"timestamp"`
}

const MaxFactsPerSession = 80000

// Engine is the authoritative runtime for Velarix.
type Engine struct {
	mu sync.RWMutex

	// Fact ID -> Fact
	Facts map[string]*Fact

	// JustificationSet ID -> JustificationSet
	JustificationSets map[string]*JustificationSet

	// Forward dependency graph: Parent Fact ID -> set of JustificationSet IDs
	ChildrenIndex map[string]map[string]struct{}

	// Root management for Dominator pruning
	CollapsedRoots  map[string]struct{}
	DirtyDominators bool
	RetractedFacts  map[string]string

	// Event listeners
	listeners []chan ChangeEvent

	// Snapshot tracking
	MutationCount    int
	LastSnapshotTime int64
}

// Snapshot represents a point-in-time binary state of the engine.
type Snapshot struct {
	Timestamp     int64
	MutationCount int
	Data          []byte
	Checksum      uint32
}

// NewEngine creates an empty velarix engine.
func NewEngine() *Engine {
	return &Engine{
		Facts:             make(map[string]*Fact),
		JustificationSets: make(map[string]*JustificationSet),
		ChildrenIndex:     make(map[string]map[string]struct{}),
		CollapsedRoots:    make(map[string]struct{}),
		RetractedFacts:    make(map[string]string),
	}
}

// Subscribe returns a channel that receives ChangeEvents.
func (e *Engine) Subscribe() chan ChangeEvent {
	e.mu.Lock()
	defer e.mu.Unlock()
	ch := make(chan ChangeEvent, 100)
	e.listeners = append(e.listeners, ch)
	return ch
}

// Unsubscribe removes a channel from the listeners list.
func (e *Engine) Unsubscribe(ch chan ChangeEvent) {
	e.mu.Lock()
	defer e.mu.Unlock()
	for i, l := range e.listeners {
		if l == ch {
			e.listeners = append(e.listeners[:i], e.listeners[i+1:]...)
			close(ch)
			return
		}
	}
}

// Lock manually locks the engine mutex.
func (e *Engine) Lock() {
	e.mu.Lock()
}

// Unlock manually unlocks the engine mutex.
func (e *Engine) Unlock() {
	e.mu.Unlock()
}

func (e *Engine) notify(factID string, status Status) {
	event := ChangeEvent{
		FactID:    factID,
		Status:    status,
		Timestamp: time.Now().UnixMilli(),
	}
	for _, ch := range e.listeners {
		select {
		case ch <- event:
		default:
			// Buffer full, drop event for this listener to prevent blocking
		}
	}
}

func (e *Engine) effectiveStatusUnsafe(fact *Fact) Status {
	if fact == nil {
		return Invalid
	}
	if _, ok := e.RetractedFacts[fact.ID]; ok {
		return Invalid
	}
	if fact.IsRoot {
		return fact.ManualStatus
	}
	return fact.DerivedStatus
}

// propagate processes state changes using a work queue and probabilistic logic.
// Callers MUST hold e.mu.Lock().
func (e *Engine) propagate(queue []string) {
	for len(queue) > 0 {
		factID := queue[0]
		queue = queue[1:]

		fact, exists := e.Facts[factID]
		if !exists {
			continue
		}

		newStatus := Invalid
		if _, retracted := e.RetractedFacts[fact.ID]; retracted {
			newStatus = Invalid
		} else if fact.IsRoot {
			newStatus = fact.ManualStatus
		} else {
			// Derived Fact: max(JustificationSets)
			maxConf := Invalid
			var validCount int
			for i := range fact.JustificationSets {
				jSetID := fmt.Sprintf("%s_jset_%d", fact.ID, i)
				jSet, ok := e.JustificationSets[jSetID]
				if !ok {
					continue
				}
				if jSet.Confidence >= ConfidenceThreshold {
					validCount++
					if jSet.Confidence > maxConf {
						maxConf = jSet.Confidence
					}
				}
			}
			fact.ValidJustificationCount = validCount
			newStatus = maxConf
		}

		if newStatus != fact.DerivedStatus {
			fact.DerivedStatus = newStatus
			e.notify(fact.ID, newStatus)

			// Recalculate children sets
			for jSetID := range e.ChildrenIndex[factID] {
				jSet, ok := e.JustificationSets[jSetID]
				if !ok {
					continue
				}

				childFact, ok := e.Facts[jSet.ChildFactID]
				if !ok {
					continue
				}

				// JustificationSet: min(all satisfied parent conditions)
				minConf := Valid // Start at 1.0
				validCount := 0
				for _, pID := range jSet.PositiveParentFactIDs {
					pFact, ok := e.Facts[pID]
					if !ok {
						continue
					}
					pStatus := e.effectiveStatusUnsafe(pFact)
					if depConf := dependencyConfidence(pStatus, false); depConf < minConf {
						minConf = depConf
					}
					if dependencySatisfied(pStatus, false) {
						validCount++
					}
				}
				for _, pID := range jSet.NegativeParentFactIDs {
					pFact, ok := e.Facts[pID]
					if !ok {
						continue
					}
					pStatus := e.effectiveStatusUnsafe(pFact)
					if depConf := dependencyConfidence(pStatus, true); depConf < minConf {
						minConf = depConf
					}
					if dependencySatisfied(pStatus, true) {
						validCount++
					}
				}

				oldValidParents := jSet.CurrentValidParents
				oldConfidence := jSet.Confidence

				jSet.CurrentValidParents = validCount

				// Set confidence only if all parents are above threshold
				if validCount == jSet.TargetValidParents {
					jSet.Confidence = minConf
				} else {
					jSet.Confidence = Invalid
				}

				if oldValidParents != jSet.CurrentValidParents || oldConfidence != jSet.Confidence {
					queue = append(queue, childFact.ID)
				}
			}
		}
	}
}

// AssertFact inserts a new fact and initializes its justification sets.
// This operation is idempotent: if the fact already exists with identical content, it returns nil.
func (e *Engine) AssertFact(f *Fact) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if len(e.Facts) >= MaxFactsPerSession {
		return fmt.Errorf("session memory cap exceeded (%d facts). please archive and start a new session", MaxFactsPerSession)
	}

	if existing, exists := e.Facts[f.ID]; exists {
		// Idempotency Check: if content matches, return nil
		if existing.IsRoot == f.IsRoot &&
			existing.ManualStatus == f.ManualStatus &&
			reflect.DeepEqual(existing.Payload, f.Payload) &&
			reflect.DeepEqual(existing.JustificationSets, f.JustificationSets) {
			return nil
		}
		return errors.New("a fact with this ID already exists with different content")
	}

	if !f.IsRoot && len(f.JustificationSets) == 0 {
		return errors.New("non-root fact must have at least one justification set")
	}

	for _, set := range f.JustificationSets {
		if len(set) == 0 {
			return errors.New("justification set cannot be empty")
		}
		for _, token := range set {
			parentID, err := normalizeDependencyToken(token)
			if err != nil {
				return err
			}
			if _, ok := e.Facts[parentID]; !ok {
				return errors.New("unknown parent fact: " + parentID)
			}
		}
	}

	if err := e.detectCycle(f); err != nil {
		return err
	}

	e.Facts[f.ID] = f
	e.DirtyDominators = true
	e.MutationCount++

	// Handle Root Premise
	if f.IsRoot {
		f.DerivedStatus = f.ManualStatus
		if f.ManualStatus < ConfidenceThreshold {
			e.CollapsedRoots[f.ID] = struct{}{}
		}
		e.notify(f.ID, f.DerivedStatus)
		e.propagate([]string{f.ID})
		return nil
	}

	f.DerivedStatus = Invalid
	f.ValidJustificationCount = 0

	for i, set := range f.JustificationSets {
		positiveParents, negativeParents, allParents, err := splitDependencySet(set)
		if err != nil {
			return err
		}
		jSetID := fmt.Sprintf("%s_jset_%d", f.ID, i)
		jSet := &JustificationSet{
			ID:                    jSetID,
			ChildFactID:           f.ID,
			ParentFactIDs:         allParents,
			PositiveParentFactIDs: positiveParents,
			NegativeParentFactIDs: negativeParents,
			TargetValidParents:    len(positiveParents) + len(negativeParents),
			CurrentValidParents:   0,
			Confidence:            Invalid,
		}

		minConf := Valid // 1.0
		validCount := 0

		for _, parentID := range positiveParents {
			pFact, ok := e.Facts[parentID]
			if !ok {
				continue
			}
			parentStatus := e.effectiveStatusUnsafe(pFact)
			if depConf := dependencyConfidence(parentStatus, false); depConf < minConf {
				minConf = depConf
			}
			if dependencySatisfied(parentStatus, false) {
				validCount++
			}

			if _, ok := e.ChildrenIndex[parentID]; !ok {
				e.ChildrenIndex[parentID] = make(map[string]struct{})
			}
			e.ChildrenIndex[parentID][jSetID] = struct{}{}
		}
		for _, parentID := range negativeParents {
			pFact, ok := e.Facts[parentID]
			if !ok {
				continue
			}
			parentStatus := e.effectiveStatusUnsafe(pFact)
			if depConf := dependencyConfidence(parentStatus, true); depConf < minConf {
				minConf = depConf
			}
			if dependencySatisfied(parentStatus, true) {
				validCount++
			}

			if _, ok := e.ChildrenIndex[parentID]; !ok {
				e.ChildrenIndex[parentID] = make(map[string]struct{})
			}
			e.ChildrenIndex[parentID][jSetID] = struct{}{}
		}

		jSet.CurrentValidParents = validCount
		if validCount == jSet.TargetValidParents {
			jSet.Confidence = minConf
			f.ValidJustificationCount++
		}

		e.JustificationSets[jSetID] = jSet
	}

	e.propagate([]string{f.ID})
	return nil
}

// InvalidateRoot manually marks a root fact as Invalid.
func (e *Engine) InvalidateRoot(factID string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	fact, exists := e.Facts[factID]
	if !exists {
		return errors.New("fact not found")
	}
	if !fact.IsRoot {
		return errors.New("cannot invalidate non-root fact")
	}
	if fact.ManualStatus == Invalid {
		return nil
	}

	fact.ManualStatus = Invalid
	e.CollapsedRoots[factID] = struct{}{}
	e.MutationCount++

	e.propagate([]string{factID})
	return nil
}

// RetractFact explicitly marks a fact as no longer usable for downstream reasoning.
func (e *Engine) RetractFact(factID string, reason string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if _, exists := e.Facts[factID]; !exists {
		return errors.New("fact not found")
	}
	if reason == "" {
		reason = "retracted"
	}
	if existingReason, exists := e.RetractedFacts[factID]; exists && existingReason == reason {
		return nil
	}

	e.RetractedFacts[factID] = reason
	e.MutationCount++
	e.propagate([]string{factID})
	return nil
}

// GetStatus returns the logically resolved status of a fact.
// It uses the Dominator Tree for O(1) pruning of deep chains.
func (e *Engine) GetStatus(factID string) Status {
	e.mu.Lock()
	if e.DirtyDominators {
		e.recomputeDominators()
	}
	e.mu.Unlock()

	e.mu.RLock()
	defer e.mu.RUnlock()

	fact, ok := e.Facts[factID]
	if !ok {
		return Invalid
	}

	currentStatus := e.effectiveStatusUnsafe(fact)
	if currentStatus < ConfidenceThreshold {
		return Invalid
	}

	for rootID := range e.CollapsedRoots {
		if e.isDominatorAncestor(rootID, factID) {
			return Invalid
		}
	}

	return currentStatus
}

// ImpactReport contains metrics for a potential retraction.
type ImpactReport struct {
	ImpactedIDs []string `json:"impacted_ids"`
	DirectCount int      `json:"direct_count"`
	TotalCount  int      `json:"total_count"`
	ActionCount int      `json:"action_count"`
	Loss        float64  `json:"epistemic_loss"`
}

// GetImpact returns a list of fact IDs that would be invalidated if factID was invalidated.
func (e *Engine) GetImpact(factID string) *ImpactReport {
	e.mu.RLock()
	defer e.mu.RUnlock()

	// 1. Setup simulation state
	// factID -> simulated DerivedStatus
	simStatus := make(map[string]Status)
	for id, f := range e.Facts {
		simStatus[id] = e.effectiveStatusUnsafe(f)
	}

	// jSetID -> simulated Confidence
	simJSetConf := make(map[string]Status)
	for id, js := range e.JustificationSets {
		simJSetConf[id] = js.Confidence
	}

	// 2. Simulate Invalidation
	simStatus[factID] = Invalid
	queue := []string{factID}
	impacted := make(map[string]struct{})
	impacted[factID] = struct{}{}

	// 3. Propagate simulation
	for len(queue) > 0 {
		uID := queue[0]
		queue = queue[1:]

		// For each dependent JustificationSet
		for jSetID := range e.ChildrenIndex[uID] {
			js := e.JustificationSets[jSetID]

			// Recalculate JSet Confidence
			minConf := Valid
			validCount := 0
			for _, pID := range js.PositiveParentFactIDs {
				pStatus := simStatus[pID]
				if depConf := dependencyConfidence(pStatus, false); depConf < minConf {
					minConf = depConf
				}
				if dependencySatisfied(pStatus, false) {
					validCount++
				}
			}
			for _, pID := range js.NegativeParentFactIDs {
				pStatus := simStatus[pID]
				if depConf := dependencyConfidence(pStatus, true); depConf < minConf {
					minConf = depConf
				}
				if dependencySatisfied(pStatus, true) {
					validCount++
				}
			}

			newJSetConf := Invalid
			if validCount == js.TargetValidParents {
				newJSetConf = minConf
			}

			if newJSetConf != simJSetConf[jSetID] {
				simJSetConf[jSetID] = newJSetConf

				// Recalculate Child Fact Status
				childID := js.ChildFactID
				childFact := e.Facts[childID]

				maxConf := Invalid
				for i := range childFact.JustificationSets {
					jsID := fmt.Sprintf("%s_jset_%d", childID, i)
					conf := simJSetConf[jsID]
					if conf > maxConf {
						maxConf = conf
					}
				}

				if maxConf != simStatus[childID] {
					simStatus[childID] = maxConf
					if maxConf < ConfidenceThreshold {
						if _, exists := impacted[childID]; !exists {
							impacted[childID] = struct{}{}
							queue = append(queue, childID)
						}
					}
				}
			}
		}
	}

	// 4. Build Report
	report := &ImpactReport{
		ImpactedIDs: make([]string, 0, len(impacted)),
		TotalCount:  len(impacted),
	}
	for id := range impacted {
		report.ImpactedIDs = append(report.ImpactedIDs, id)
		report.Loss += float64(e.Facts[id].DerivedStatus)

		// Direct child check (simplified for simulation)
		if f, ok := e.Facts[id]; ok && f.IDom == factID {
			report.DirectCount++
		}
		if f, ok := e.Facts[id]; ok && f.Payload != nil && f.Payload["type"] == "action" {
			report.ActionCount++
		}
	}

	return report
}

// GetFact returns a copy of a fact, locking for safety.
func (e *Engine) GetFact(factID string) (*Fact, bool) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	f, ok := e.Facts[factID]
	return f, ok
}

// ToSnapshot serializes the engine into a Snapshot struct with CRC32 integrity.
func (e *Engine) ToSnapshot() (*Snapshot, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	// Data to serialize
	payload := struct {
		Facts             map[string]*Fact
		JustificationSets map[string]*JustificationSet
		ChildrenIndex     map[string]map[string]struct{}
		CollapsedRoots    map[string]struct{}
		RetractedFacts    map[string]string
		MutationCount     int
	}{
		Facts:             e.Facts,
		JustificationSets: e.JustificationSets,
		ChildrenIndex:     e.ChildrenIndex,
		CollapsedRoots:    e.CollapsedRoots,
		RetractedFacts:    e.RetractedFacts,
		MutationCount:     e.MutationCount,
	}

	var buf bytes.Buffer
	enc := gob.NewEncoder(&buf)
	if err := enc.Encode(payload); err != nil {
		return nil, fmt.Errorf("failed to encode engine state: %v", err)
	}

	data := buf.Bytes()
	checksum := crc32.ChecksumIEEE(data)

	return &Snapshot{
		Timestamp:     time.Now().UnixMilli(),
		MutationCount: e.MutationCount,
		Data:          data,
		Checksum:      checksum,
	}, nil
}

// FromSnapshot restores the engine state from a binary Snapshot and verifies integrity.
func (e *Engine) FromSnapshot(snap *Snapshot) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	// 1. Verify Integrity
	actualChecksum := crc32.ChecksumIEEE(snap.Data)
	if actualChecksum != snap.Checksum {
		return errors.New("snapshot integrity check failed: checksum mismatch")
	}

	// 2. Decode Data
	buf := bytes.NewBuffer(snap.Data)
	dec := gob.NewDecoder(buf)

	var payload struct {
		Facts             map[string]*Fact
		JustificationSets map[string]*JustificationSet
		ChildrenIndex     map[string]map[string]struct{}
		CollapsedRoots    map[string]struct{}
		RetractedFacts    map[string]string
		MutationCount     int
	}

	if err := dec.Decode(&payload); err != nil {
		return fmt.Errorf("failed to decode snapshot data: %v", err)
	}

	// 3. Update Engine State
	e.Facts = payload.Facts
	e.JustificationSets = payload.JustificationSets
	e.ChildrenIndex = payload.ChildrenIndex
	e.CollapsedRoots = payload.CollapsedRoots
	if payload.RetractedFacts == nil {
		payload.RetractedFacts = make(map[string]string)
	}
	e.RetractedFacts = payload.RetractedFacts
	e.MutationCount = payload.MutationCount
	e.DirtyDominators = true // Force recompute on next access

	return nil
}

// ListFacts returns all facts in the engine.
func (e *Engine) ListFacts() []*Fact {
	e.mu.RLock()
	defer e.mu.RUnlock()

	var results []*Fact
	for _, f := range e.Facts {
		results = append(results, f)
	}
	return results
}

// DependencyIDs returns the transitive fact dependency set for the given fact.
func (e *Engine) DependencyIDs(factID string, includeSelf bool) ([]string, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	if _, ok := e.Facts[factID]; !ok {
		return nil, errors.New("fact not found")
	}

	seen := map[string]struct{}{}
	var walk func(string)
	walk = func(id string) {
		fact, ok := e.Facts[id]
		if !ok {
			return
		}
		if !includeSelf && id == factID {
			// keep walking parents without adding the target fact itself
		} else {
			if _, exists := seen[id]; exists {
				return
			}
			seen[id] = struct{}{}
		}
		for _, set := range fact.JustificationSets {
			for _, token := range set {
				parentID, err := normalizeDependencyToken(token)
				if err != nil {
					continue
				}
				walk(parentID)
			}
		}
	}
	walk(factID)

	out := make([]string, 0, len(seen))
	for id := range seen {
		if !includeSelf && id == factID {
			continue
		}
		out = append(out, id)
	}
	sort.Strings(out)
	return out, nil
}
