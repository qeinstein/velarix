package core

import (
	"bytes"
	"encoding/gob"
	"errors"
	"fmt"
	"hash/crc32"
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
		if fact.IsRoot {
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

				// JustificationSet: min(Parents)
				minConf := Valid // Start at 1.0
				validCount := 0
				for _, pID := range jSet.ParentFactIDs {
					pFact, ok := e.Facts[pID]
					if !ok {
						continue
					}
					pStatus := pFact.DerivedStatus
					if pStatus < minConf {
						minConf = pStatus
					}
					if pStatus >= ConfidenceThreshold {
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
func (e *Engine) AssertFact(f *Fact) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if len(e.Facts) >= MaxFactsPerSession {
		return fmt.Errorf("session memory cap exceeded (%d facts). please archive and start a new session", MaxFactsPerSession)
	}

	if _, exists := e.Facts[f.ID]; exists {
		return errors.New("a fact with this ID already exists")
	}

	if !f.IsRoot && len(f.JustificationSets) == 0 {
		return errors.New("non-root fact must have at least one justification set")
	}

	for _, set := range f.JustificationSets {
		if len(set) == 0 {
			return errors.New("justification set cannot be empty")
		}
		for _, parentID := range set {
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
		jSetID := fmt.Sprintf("%s_jset_%d", f.ID, i)
		jSet := &JustificationSet{
			ID:                  jSetID,
			ChildFactID:         f.ID,
			ParentFactIDs:       set,
			TargetValidParents:  len(set),
			CurrentValidParents: 0,
			Confidence:          Invalid,
		}

		minConf := Valid // 1.0
		validCount := 0

		for _, parentID := range set {
			pFact, ok := e.Facts[parentID]
			if !ok {
				continue
			}
			parentStatus := pFact.DerivedStatus
			if parentStatus < minConf {
				minConf = parentStatus
			}
			if parentStatus >= ConfidenceThreshold {
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

	if fact.DerivedStatus < ConfidenceThreshold {
		return Invalid
	}

	for rootID := range e.CollapsedRoots {
		if e.isDominatorAncestor(rootID, factID) {
			return Invalid
		}
	}

	return fact.DerivedStatus
}

// GetImpact returns a list of fact IDs that would be invalidated if factID was invalidated.
func (e *Engine) GetImpact(factID string) []string {
	e.mu.Lock()
	if e.DirtyDominators {
		e.recomputeDominators()
	}
	e.mu.Unlock()

	e.mu.RLock()
	defer e.mu.RUnlock()

	var impact []string
	for id := range e.Facts {
		if id == factID {
			continue
		}
		if e.isDominatorAncestor(factID, id) {
			impact = append(impact, id)
		}
	}

	return impact
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
		MutationCount     int
	}{
		Facts:             e.Facts,
		JustificationSets: e.JustificationSets,
		ChildrenIndex:     e.ChildrenIndex,
		CollapsedRoots:    e.CollapsedRoots,
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
