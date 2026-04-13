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

// ConfidenceThreshold is the minimum status treated as currently valid.
const ConfidenceThreshold Status = 0.6

// NilArgumentError reports a required nil argument.
type NilArgumentError struct {
	Argument string
}

// Error formats the nil-argument error.
func (e *NilArgumentError) Error() string {
	return fmt.Sprintf("%s cannot be nil", e.Argument)
}

// ChangeEvent is emitted when a fact's effective status changes.
type ChangeEvent struct {
	FactID    string `json:"fact_id"`
	Status    Status `json:"status"`
	Timestamp int64  `json:"timestamp"`
}

// MaxFactsPerSession is the hard in-memory cap enforced per session.
const MaxFactsPerSession = 80000 // Absolute cap on number of facts to prevent OOM. In practice, performance degradation starts around 50k facts with complex justifications, so this is a safety limit. Users should archive and start a new session if they hit this limit.

// Engine is the authoritative runtime for Velarix.
type Engine struct {
	mu sync.RWMutex

	// Fact ID -> Fact
	Facts map[string]*Fact

	// JustificationSet ID -> JustificationSet
	JustificationSets map[string]*JustificationSet

	// Forward dependency graph: Parent Fact ID -> set of JustificationSet IDs
	ChildrenIndex map[string]map[string]struct{}

	// Reverse justification index: Child Fact ID -> set of JustificationSet IDs.
	// Allows O(1) lookup of all JustificationSets that justify a given fact,
	// replacing the previous O(|JustificationSets|) full scan in getIncomingJustificationSets
	// and eliminating fmt.Sprintf ID reconstruction in the propagation hot path.
	ChildJSetIndex map[string]map[string]struct{}

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
		ChildJSetIndex:    make(map[string]map[string]struct{}),
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
//
// Optimisations over the naive implementation:
//   - Queue deduplication: each fact ID appears at most once in the queue at any
//     time, so diamond dependencies do not cause redundant re-evaluations.
//   - OR short-circuit: when a derived fact's best justification set reaches
//     confidence 1.0, further sets cannot improve the result.
//   - AND short-circuit for confidence: once the running minimum confidence for
//     a justification set reaches Invalid (0.0), remaining parent confidence
//     contributions are skipped (validCount is still computed fully).
//   - No fmt.Sprintf: JustificationSet IDs for a fact are looked up directly
//     via ChildJSetIndex instead of being reconstructed by string formatting.
func (e *Engine) propagate(queue []string) {
	// Deduplicate the initial queue so concurrent enqueue of the same root
	// doesn't cause multiple passes over the same downstream subgraph.
	inQueue := make(map[string]bool, len(queue))
	for _, id := range queue {
		inQueue[id] = true
	}

	head := 0
	for head < len(queue) {
		factID := queue[head]
		head++
		delete(inQueue, factID)

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
			// Derived Fact: max over all justification sets.
			// Short-circuit as soon as we reach Valid (1.0) — nothing can exceed it.
			maxConf := Invalid
			var validCount int
			for jSetID := range e.ChildJSetIndex[fact.ID] {
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
				if maxConf == Valid {
					break // OR short-circuit: cannot improve beyond 1.0
				}
			}
			fact.ValidJustificationCount = validCount
			newStatus = maxConf
		}

		if newStatus != fact.DerivedStatus {
			fact.DerivedStatus = newStatus
			e.notify(fact.ID, newStatus)

			// Recalculate dependent justification sets and enqueue changed children.
			for jSetID := range e.ChildrenIndex[factID] {
				jSet, ok := e.JustificationSets[jSetID]
				if !ok {
					continue
				}

				childFact, ok := e.Facts[jSet.ChildFactID]
				if !ok {
					continue
				}

				// JustificationSet confidence: min of all parent confidences when
				// all parents are satisfied.  We compute validCount in full (needed
				// for the TargetValidParents comparison) but skip depConf updates
				// once minConf has already reached Invalid — it cannot go lower.
				minConf := Valid
				validCount := 0

				for _, pID := range jSet.PositiveParentFactIDs {
					pFact, ok := e.Facts[pID]
					if !ok {
						continue
					}
					pStatus := e.effectiveStatusUnsafe(pFact)
					if dependencySatisfied(pStatus, false) {
						validCount++
					}
					if minConf > Invalid {
						// AND short-circuit for confidence accumulation
						if depConf := dependencyConfidence(pStatus, false); depConf < minConf {
							minConf = depConf
						}
					}
				}
				for _, pID := range jSet.NegativeParentFactIDs {
					pFact, ok := e.Facts[pID]
					if !ok {
						continue
					}
					pStatus := e.effectiveStatusUnsafe(pFact)
					if dependencySatisfied(pStatus, true) {
						validCount++
					}
					if minConf > Invalid {
						if depConf := dependencyConfidence(pStatus, true); depConf < minConf {
							minConf = depConf
						}
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
					childID := childFact.ID
					if !inQueue[childID] {
						queue = append(queue, childID)
						inQueue[childID] = true
					}
				}
			}
		}
	}
}

// assertFactInner is the core fact-insertion logic shared by AssertFact and
// AssertFacts.  It returns the fact ID that should be used as the propagation
// seed (empty string if no propagation is needed, e.g. idempotent re-assert).
// Callers MUST hold e.mu.Lock().
func (e *Engine) assertFactInner(f *Fact) (string, error) {
	if len(e.Facts) >= MaxFactsPerSession {
		return "", fmt.Errorf("session memory cap exceeded (%d facts). please archive and start a new session", MaxFactsPerSession)
	}

	if existing, exists := e.Facts[f.ID]; exists {
		// Idempotency Check: if content matches, return nil
		if existing.IsRoot == f.IsRoot &&
			existing.ManualStatus == f.ManualStatus &&
			reflect.DeepEqual(existing.Payload, f.Payload) &&
			reflect.DeepEqual(existing.JustificationSets, f.JustificationSets) {
			return "", nil
		}
		return "", errors.New("a fact with this ID already exists with different content")
	}

	if !f.IsRoot && len(f.JustificationSets) == 0 {
		return "", errors.New("non-root fact must have at least one justification set")
	}

	for _, set := range f.JustificationSets {
		if len(set) == 0 {
			return "", errors.New("justification set cannot be empty")
		}
		for _, token := range set {
			parentID, err := normalizeDependencyToken(token)
			if err != nil {
				return "", err
			}
			if _, ok := e.Facts[parentID]; !ok {
				return "", errors.New("unknown parent fact: " + parentID)
			}
		}
	}

	if err := e.detectCycle(f); err != nil {
		return "", err
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
		return f.ID, nil
	}

	f.DerivedStatus = Invalid
	f.ValidJustificationCount = 0

	for i, set := range f.JustificationSets {
		positiveParents, negativeParents, allParents, err := splitDependencySet(set)
		if err != nil {
			return "", err
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

		// Populate the reverse child→jSet index
		if _, ok := e.ChildJSetIndex[f.ID]; !ok {
			e.ChildJSetIndex[f.ID] = make(map[string]struct{})
		}
		e.ChildJSetIndex[f.ID][jSetID] = struct{}{}

		jSet.CurrentValidParents = validCount
		if validCount == jSet.TargetValidParents {
			jSet.Confidence = minConf
			f.ValidJustificationCount++
		}

		e.JustificationSets[jSetID] = jSet
	}

	return f.ID, nil
}

// AssertFact inserts a new fact and initializes its justification sets.
// This operation is idempotent: if the fact already exists with identical content, it returns nil.
func (e *Engine) AssertFact(f *Fact) error {
	if f == nil {
		return &NilArgumentError{Argument: "fact"}
	}

	e.mu.Lock()
	defer e.mu.Unlock()

	seedID, err := e.assertFactInner(f)
	if err != nil {
		return err
	}
	if seedID != "" {
		e.propagate([]string{seedID})
	}
	return nil
}

// AssertFacts bulk-asserts multiple facts with a single propagation pass.
// This is significantly more efficient than calling AssertFact in a loop when
// asserting many facts at once, because downstream propagation runs once across
// all seeds rather than once per fact.
//
// Facts are validated and inserted in order. If any fact fails, the error is
// returned immediately; already-inserted facts in the batch are not rolled back.
func (e *Engine) AssertFacts(facts []*Fact) error {
	for _, f := range facts {
		if f == nil {
			return &NilArgumentError{Argument: "fact"}
		}
	}

	e.mu.Lock()
	defer e.mu.Unlock()

	seeds := make([]string, 0, len(facts))
	for _, f := range facts {
		seedID, err := e.assertFactInner(f)
		if err != nil {
			return err
		}
		if seedID != "" {
			seeds = append(seeds, seedID)
		}
	}
	if len(seeds) > 0 {
		e.propagate(seeds)
	}
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

// SetRootConfidence updates the ManualStatus of a root fact and propagates the
// change through the dependency graph. Used during journal replay for
// confidence_adjusted events.
func (e *Engine) SetRootConfidence(factID string, confidence Status) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	fact, ok := e.Facts[factID]
	if !ok {
		return errors.New("fact not found")
	}
	if !fact.IsRoot {
		return errors.New("SetRootConfidence: only root facts support manual confidence adjustment")
	}
	if fact.ManualStatus == confidence {
		return nil
	}
	fact.ManualStatus = confidence
	if confidence < ConfidenceThreshold {
		e.CollapsedRoots[factID] = struct{}{}
	} else {
		delete(e.CollapsedRoots, factID)
	}
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

// getStatusLocked reads the effective status for factID.
// Caller must hold at least e.mu.RLock (or e.mu.Lock for the dirty-dominator path).
func (e *Engine) getStatusLocked(factID string) Status {
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

// GetStatus returns the logically resolved status of a fact.
// It uses the Dominator Tree for O(1) pruning of deep chains.
//
// Lock strategy: if the dominator tree is clean, a shared read lock is used
// throughout.  If it is dirty, a write lock is held for both recompute and
// read to avoid the TOCTOU window that the previous Lock→Unlock→RLock pattern
// created.
func (e *Engine) GetStatus(factID string) Status {
	// Fast path: dominators clean — use a shared read lock throughout.
	e.mu.RLock()
	if !e.DirtyDominators {
		defer e.mu.RUnlock()
		return e.getStatusLocked(factID)
	}
	e.mu.RUnlock()

	// Slow path: recompute under write lock, then read within the same critical section.
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.DirtyDominators { // re-check: another goroutine may have beaten us here
		e.recomputeDominators()
	}
	return e.getStatusLocked(factID)
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
//
// The simulation uses lazy status maps: entries are only populated for facts
// actually visited during propagation, avoiding the previous O(n) upfront copy
// of all 80,000 fact statuses.
func (e *Engine) GetImpact(factID string) (*ImpactReport, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	if _, ok := e.Facts[factID]; !ok {
		return nil, errors.New("fact not found")
	}

	// Lazy simulation state: fall back to the live engine values for unvisited nodes.
	simStatus := make(map[string]Status)
	getSimStatus := func(id string) Status {
		if s, ok := simStatus[id]; ok {
			return s
		}
		if f, ok := e.Facts[id]; ok {
			return e.effectiveStatusUnsafe(f)
		}
		return Invalid
	}

	simJSetConf := make(map[string]Status)
	getSimJSetConf := func(id string) Status {
		if s, ok := simJSetConf[id]; ok {
			return s
		}
		if js, ok := e.JustificationSets[id]; ok {
			return js.Confidence
		}
		return Invalid
	}

	// Simulate invalidation of the target fact.
	simStatus[factID] = Invalid
	queue := []string{factID}
	impacted := make(map[string]struct{})
	impacted[factID] = struct{}{}

	// Propagate simulation forward.
	for len(queue) > 0 {
		uID := queue[0]
		queue = queue[1:]

		for jSetID := range e.ChildrenIndex[uID] {
			js := e.JustificationSets[jSetID]

			// Recalculate jSet confidence using simulated parent statuses
			minConf := Valid
			validCount := 0
			for _, pID := range js.PositiveParentFactIDs {
				pStatus := getSimStatus(pID)
				if depConf := dependencyConfidence(pStatus, false); depConf < minConf {
					minConf = depConf
				}
				if dependencySatisfied(pStatus, false) {
					validCount++
				}
			}
			for _, pID := range js.NegativeParentFactIDs {
				pStatus := getSimStatus(pID)
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

			if newJSetConf != getSimJSetConf(jSetID) {
				simJSetConf[jSetID] = newJSetConf

				// Recalculate child fact status using ChildJSetIndex (no fmt.Sprintf)
				childID := js.ChildFactID
				childFact := e.Facts[childID]

				maxConf := Invalid
				for jsID := range e.ChildJSetIndex[childID] {
					if conf := getSimJSetConf(jsID); conf > maxConf {
						maxConf = conf
					}
				}

				if maxConf != getSimStatus(childID) {
					simStatus[childID] = maxConf
					if maxConf < ConfidenceThreshold {
						if _, exists := impacted[childID]; !exists {
							impacted[childID] = struct{}{}
							queue = append(queue, childID)
						}
					}
				}

				_ = childFact // used for IDom check below via e.Facts lookup
			}
		}
	}

	// Build Report
	report := &ImpactReport{
		ImpactedIDs: make([]string, 0, len(impacted)),
		TotalCount:  len(impacted),
	}
	for id := range impacted {
		report.ImpactedIDs = append(report.ImpactedIDs, id)
		fact, ok := e.Facts[id]
		if !ok {
			continue
		}
		report.Loss += float64(fact.DerivedStatus)

		// Direct child check (simplified for simulation)
		if fact.IDom == factID {
			report.DirectCount++
		}
		if fact.Payload != nil && fact.Payload["type"] == "action" {
			report.ActionCount++
		}
	}

	return report, nil
}

func cloneStringSlice(values []string) []string {
	if values == nil {
		return nil
	}
	cloned := make([]string, len(values))
	copy(cloned, values)
	return cloned
}

func cloneFloat64Slice(values []float64) []float64 {
	if values == nil {
		return nil
	}
	cloned := make([]float64, len(values))
	copy(cloned, values)
	return cloned
}

func cloneStringMatrix(values [][]string) [][]string {
	if values == nil {
		return nil
	}
	cloned := make([][]string, len(values))
	for i := range values {
		cloned[i] = cloneStringSlice(values[i])
	}
	return cloned
}

func cloneDynamicValue(value interface{}) interface{} {
	switch typed := value.(type) {
	case map[string]interface{}:
		return cloneStringInterfaceMap(typed)
	case []interface{}:
		cloned := make([]interface{}, len(typed))
		for i := range typed {
			cloned[i] = cloneDynamicValue(typed[i])
		}
		return cloned
	case []string:
		return cloneStringSlice(typed)
	case []float64:
		return cloneFloat64Slice(typed)
	default:
		return typed
	}
}

func cloneStringInterfaceMap(values map[string]interface{}) map[string]interface{} {
	if values == nil {
		return nil
	}
	cloned := make(map[string]interface{}, len(values))
	for key, value := range values {
		cloned[key] = cloneDynamicValue(value)
	}
	return cloned
}

func cloneFact(f *Fact) *Fact {
	if f == nil {
		return nil
	}
	cloned := *f
	cloned.Payload = cloneStringInterfaceMap(f.Payload)
	cloned.Metadata = cloneStringInterfaceMap(f.Metadata)
	cloned.Embedding = cloneFloat64Slice(f.Embedding)
	cloned.JustificationSets = cloneStringMatrix(f.JustificationSets)
	cloned.ValidationErrors = cloneStringSlice(f.ValidationErrors)
	return &cloned
}

func buildValidationEngine(facts map[string]*Fact) (*Engine, error) {
	validation := NewEngine()
	validation.Facts = facts

	for id, fact := range facts {
		if fact == nil {
			return nil, fmt.Errorf("snapshot contains nil fact: %s", id)
		}
		for i, set := range fact.JustificationSets {
			positiveParents, negativeParents, allParents, err := splitDependencySet(set)
			if err != nil {
				return nil, err
			}
			for _, parentID := range allParents {
				if _, ok := facts[parentID]; !ok {
					return nil, errors.New("unknown parent fact: " + parentID)
				}
				if _, ok := validation.ChildrenIndex[parentID]; !ok {
					validation.ChildrenIndex[parentID] = make(map[string]struct{})
				}
				jSetID := fmt.Sprintf("%s_jset_%d", fact.ID, i)
				validation.ChildrenIndex[parentID][jSetID] = struct{}{}
				validation.JustificationSets[jSetID] = &JustificationSet{
					ID:                    jSetID,
					ChildFactID:           fact.ID,
					ParentFactIDs:         allParents,
					PositiveParentFactIDs: positiveParents,
					NegativeParentFactIDs: negativeParents,
				}
			}
		}
	}

	// Build the child→jSet reverse index so getIncomingJustificationSets works
	// during cycle detection (reachable uses ChildrenIndex, but recomputeDominators
	// would need ChildJSetIndex if triggered — rebuild defensively).
	validation.rebuildChildJSetIndex()

	return validation, nil
}

// rebuildChildJSetIndex reconstructs the ChildJSetIndex from JustificationSets.
// Called after FromSnapshot to restore the derived index without serialising it.
// Callers MUST hold e.mu.Lock() or call before the engine is shared.
func (e *Engine) rebuildChildJSetIndex() {
	e.ChildJSetIndex = make(map[string]map[string]struct{}, len(e.Facts))
	for id, js := range e.JustificationSets {
		if _, ok := e.ChildJSetIndex[js.ChildFactID]; !ok {
			e.ChildJSetIndex[js.ChildFactID] = make(map[string]struct{})
		}
		e.ChildJSetIndex[js.ChildFactID][id] = struct{}{}
	}
}

// GetFact returns a copy of a fact, locking for safety.
func (e *Engine) GetFact(factID string) (*Fact, bool) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	f, ok := e.Facts[factID]
	if !ok {
		return nil, false
	}
	return cloneFact(f), true
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
	if snap == nil {
		return &NilArgumentError{Argument: "snapshot"}
	}

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

	if payload.Facts == nil {
		payload.Facts = make(map[string]*Fact)
	}
	if len(payload.Facts) > MaxFactsPerSession {
		return fmt.Errorf("snapshot contains %d facts, exceeds session memory cap (%d)", len(payload.Facts), MaxFactsPerSession)
	}
	validation, err := buildValidationEngine(payload.Facts)
	if err != nil {
		return err
	}
	for _, fact := range validation.Facts {
		if err := validation.detectCycle(fact); err != nil {
			return err
		}
	}

	// 3. Update Engine State
	e.Facts = payload.Facts
	e.JustificationSets = payload.JustificationSets
	e.ChildrenIndex = payload.ChildrenIndex
	e.CollapsedRoots = payload.CollapsedRoots
	if e.JustificationSets == nil {
		e.JustificationSets = make(map[string]*JustificationSet)
	}
	if e.ChildrenIndex == nil {
		e.ChildrenIndex = make(map[string]map[string]struct{})
	}
	if e.CollapsedRoots == nil {
		e.CollapsedRoots = make(map[string]struct{})
	}
	if payload.RetractedFacts == nil {
		payload.RetractedFacts = make(map[string]string)
	}
	e.RetractedFacts = payload.RetractedFacts
	e.MutationCount = payload.MutationCount

	// Rebuild the derived reverse index (not stored in snapshot)
	e.rebuildChildJSetIndex()

	e.DirtyDominators = true // Force recompute on next access
	return nil
}

// ListFacts returns all facts in the engine.
func (e *Engine) ListFacts() []*Fact {
	e.mu.RLock()
	defer e.mu.RUnlock()

	// Pre-allocate to avoid repeated slice growth.
	results := make([]*Fact, 0, len(e.Facts))
	for _, f := range e.Facts {
		results = append(results, cloneFact(f))
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

	// Add the start node to seen unconditionally so that any shared sub-graph
	// path that re-encounters factID is correctly short-circuited rather than
	// re-walked.  It is filtered from the output below if !includeSelf.
	var walk func(string)
	walk = func(id string) {
		if _, exists := seen[id]; exists {
			return
		}
		seen[id] = struct{}{}
		fact, ok := e.Facts[id]
		if !ok {
			return
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
