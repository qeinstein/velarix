package core

import (
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// GlobalTruth is a versioned, shared fact namespace that multiple engines
// (sessions / agents) can subscribe to.
//
// # Design
//
// Every call to AssertGlobal or RetractGlobal increments a monotonic version
// counter and synchronously broadcasts the mutation to all subscribed engines
// by calling their AssertFact / RetractFact methods. Subscribers therefore
// maintain a local copy of every global fact and can evaluate it at O(1)
// without cross-engine coordination on the hot path.
//
// # Read-your-writes consistency
//
// AssertGlobal and RetractGlobal return the version number that reflects the
// mutation. Callers that need to guarantee a downstream reader has seen the
// write can call WaitForVersion(version, deadline) on the same GlobalTruth
// instance before handing off work.
//
// # Graph partitioning
//
// Partition() computes the connected components of the global fact dependency
// graph using Union-Find. Each component is a slice of fact IDs with no
// dependency edges crossing component boundaries. Components can be assigned
// to independent agents that share this GlobalTruth without any intra-component
// coordination — agents in different components never need to exchange facts.
type GlobalTruth struct {
	mu          sync.RWMutex
	facts       map[string]*Fact
	retracted   map[string]string // factID -> reason
	version     atomic.Int64
	subscribers map[string]*Engine // sessionID -> engine

	// Reverse index: globalFactID -> sessionIDs that reference it.
	// Used to do selective fan-out on global fact updates.
	depIndex map[string]map[string]struct{}

	// Session-local index for efficient removal on Unsubscribe.
	sessionDeps map[string]map[string]struct{} // sessionID -> set(globalFactID)
}

// NewGlobalTruth returns an empty GlobalTruth store.
func NewGlobalTruth() *GlobalTruth {
	return &GlobalTruth{
		facts:       make(map[string]*Fact),
		retracted:   make(map[string]string),
		subscribers: make(map[string]*Engine),
		depIndex:    make(map[string]map[string]struct{}),
		sessionDeps: make(map[string]map[string]struct{}),
	}
}

// Version returns the current monotonic mutation counter.
// The counter increments once per AssertGlobal or RetractGlobal call,
// regardless of whether the fact content actually changed.
func (gt *GlobalTruth) Version() int64 {
	return gt.version.Load()
}

// Subscribe registers engine to receive all future global fact mutations.
// On registration, the engine is immediately replayed with every non-retracted
// global fact that already exists, so it starts in a consistent state.
//
// Subscribing the same sessionID twice replaces the previous registration.
func (gt *GlobalTruth) Subscribe(sessionID string, engine *Engine) error {
	if sessionID == "" {
		return errors.New("sessionID cannot be empty")
	}
	if engine == nil {
		return errors.New("engine cannot be nil")
	}

	gt.mu.Lock()
	defer gt.mu.Unlock()

	gt.subscribers[sessionID] = engine

	gt.rebuildSessionIndexLocked(sessionID, engine)

	// Replay all existing non-retracted global facts into the new subscriber
	// so it starts in a consistent state without a separate bootstrap step.
	for _, fact := range gt.facts {
		if _, isRetracted := gt.retracted[fact.ID]; isRetracted {
			continue
		}
		f := cloneFact(fact)
		f.Metadata = withGlobalTruthFlag(f.Metadata)
		// idempotent — ignore already-exists errors from a re-subscription
		_ = engine.AssertFact(f)
	}
	return nil
}

// Unsubscribe removes the engine registered under sessionID from the
// subscriber set. Future global mutations will not be forwarded to it.
func (gt *GlobalTruth) Unsubscribe(sessionID string) {
	gt.mu.Lock()
	defer gt.mu.Unlock()
	delete(gt.subscribers, sessionID)
	gt.removeSessionIndexLocked(sessionID)
}

// AssertGlobal adds or replaces a fact in the global namespace and broadcasts
// it to every subscribed engine. Returns the version number after the mutation.
//
// The fact is tagged with metadata["_global_truth"] = true so that per-session
// code can distinguish locally-asserted facts from globally-propagated ones.
func (gt *GlobalTruth) AssertGlobal(fact *Fact) (int64, error) {
	if fact == nil {
		return 0, errors.New("fact cannot be nil")
	}

	gt.mu.Lock()
	defer gt.mu.Unlock()

	_, existed := gt.facts[fact.ID]
	gt.facts[fact.ID] = cloneFact(fact)
	delete(gt.retracted, fact.ID)
	version := gt.version.Add(1)

	// Build the broadcast copy once; clone per subscriber to prevent aliasing.
	broadcast := cloneFact(fact)
	broadcast.Metadata = withGlobalTruthFlag(broadcast.Metadata)

	// Fan-out strategy:
	// - New global facts are broadcast to all subscribers so they can be used as
	//   dependencies without requiring a separate fetch path.
	// - Updates to existing global facts are broadcast selectively based on the
	//   dependency index and global connected-components.
	targets := map[string]*Engine{}
	selective := false
	if !existed {
		for sid, eng := range gt.subscribers {
			targets[sid] = eng
		}
	} else {
		selective = true
		for _, sid := range gt.affectedSubscribersLocked(fact.ID) {
			if eng, ok := gt.subscribers[sid]; ok {
				targets[sid] = eng
			}
		}
		// If we couldn't compute any affected subscribers, fall back to full
		// broadcast to preserve correctness.
		if len(targets) == 0 {
			selective = false
			for sid, eng := range gt.subscribers {
				targets[sid] = eng
			}
		}
	}

	GlobalFanoutTotal.WithLabelValues(fmt.Sprintf("%t", selective)).Inc()

	for _, eng := range targets {
		_ = eng.AssertFact(cloneFact(broadcast))
	}

	return version, nil
}

// RetractGlobal removes a fact from the global namespace and propagates the
// retraction to every subscribed engine. Returns the version number after the
// mutation.
func (gt *GlobalTruth) RetractGlobal(factID string, reason string) (int64, error) {
	if factID == "" {
		return 0, errors.New("factID cannot be empty")
	}
	if reason == "" {
		reason = "global_retract"
	}

	gt.mu.Lock()
	defer gt.mu.Unlock()

	if _, ok := gt.facts[factID]; !ok {
		return 0, errors.New("global fact not found: " + factID)
	}

	gt.retracted[factID] = reason
	version := gt.version.Add(1)

	targets := gt.subscribers
	selective := false
	affected := gt.affectedSubscribersLocked(factID)
	if len(affected) > 0 && len(affected) < len(gt.subscribers) {
		selective = true
		targets = map[string]*Engine{}
		for _, sid := range affected {
			if eng, ok := gt.subscribers[sid]; ok {
				targets[sid] = eng
			}
		}
	}
	GlobalFanoutTotal.WithLabelValues(fmt.Sprintf("%t", selective)).Inc()

	for _, eng := range targets {
		_ = eng.RetractFact(factID, reason)
	}

	return version, nil
}

// ListGlobalFacts returns a snapshot of all non-retracted global facts.
func (gt *GlobalTruth) ListGlobalFacts() []*Fact {
	gt.mu.RLock()
	defer gt.mu.RUnlock()

	out := make([]*Fact, 0, len(gt.facts))
	for id, f := range gt.facts {
		if _, isRetracted := gt.retracted[id]; isRetracted {
			continue
		}
		out = append(out, cloneFact(f))
	}
	return out
}

// GetGlobalFact returns a snapshot of a global fact, if present and not retracted.
func (gt *GlobalTruth) GetGlobalFact(factID string) (*Fact, bool) {
	gt.mu.RLock()
	defer gt.mu.RUnlock()
	f, ok := gt.facts[factID]
	if !ok {
		return nil, false
	}
	if _, isRetracted := gt.retracted[factID]; isRetracted {
		return nil, false
	}
	return cloneFact(f), true
}

// SubscriberCount returns the number of currently registered engines.
func (gt *GlobalTruth) SubscriberCount() int {
	gt.mu.RLock()
	defer gt.mu.RUnlock()
	return len(gt.subscribers)
}

// Partition computes the connected components of the global fact dependency
// graph and returns each component as a sorted slice of fact IDs.
//
// Two facts are in the same component if there is any dependency path between
// them (positive or negative). Facts with no dependencies form singleton
// components.
//
// Use Partition to decide which agents can work independently:
//   - Assign each component to one agent (or one Engine).
//   - Agents in different components share this GlobalTruth but never need to
//     read each other's outputs, so there is no intra-component lock contention.
//   - When a global fact changes, only the engines subscribed to that component
//     need to be notified — future work can use this to implement selective
//     broadcast rather than full fan-out.
func (gt *GlobalTruth) Partition() [][]string {
	gt.mu.RLock()
	defer gt.mu.RUnlock()
	return gt.partitionLocked()
}

func (gt *GlobalTruth) partitionLocked() [][]string {
	// Union-Find with path compression.
	parent := make(map[string]string, len(gt.facts))
	for id := range gt.facts {
		parent[id] = id
	}

	var find func(string) string
	find = func(x string) string {
		if parent[x] != x {
			parent[x] = find(parent[x])
		}
		return parent[x]
	}

	union := func(x, y string) {
		px, py := find(x), find(y)
		if px != py {
			parent[px] = py
		}
	}

	for _, fact := range gt.facts {
		for _, set := range fact.JustificationSets {
			for _, token := range set {
				parentID := token
				if len(parentID) > 0 && parentID[0] == '!' {
					parentID = parentID[1:]
				}
				if _, ok := gt.facts[parentID]; ok {
					union(fact.ID, parentID)
				}
			}
		}
	}

	groups := make(map[string][]string)
	for id := range gt.facts {
		root := find(id)
		groups[root] = append(groups[root], id)
	}

	result := make([][]string, 0, len(groups))
	for _, ids := range groups {
		sort.Strings(ids)
		result = append(result, ids)
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i][0] < result[j][0]
	})
	return result
}

// WaitForVersion blocks until the GlobalTruth version reaches at least
// minVersion, or the deadline is exceeded.
//
// Use this for read-your-writes consistency: after AssertGlobal returns
// version V, call WaitForVersion(V, deadline) before handing off work to a
// consumer that must see the new fact. Because AssertGlobal broadcasts
// synchronously, this typically returns immediately; the method exists to make
// the consistency contract explicit in calling code.
func (gt *GlobalTruth) WaitForVersion(minVersion int64, deadline time.Time) error {
	for {
		if gt.version.Load() >= minVersion {
			return nil
		}
		if time.Now().After(deadline) {
			return errors.New("timeout waiting for global truth version")
		}
		time.Sleep(time.Millisecond)
	}
}

func (gt *GlobalTruth) removeSessionIndexLocked(sessionID string) {
	deps := gt.sessionDeps[sessionID]
	delete(gt.sessionDeps, sessionID)
	for factID := range deps {
		if sessions, ok := gt.depIndex[factID]; ok {
			delete(sessions, sessionID)
			if len(sessions) == 0 {
				delete(gt.depIndex, factID)
			}
		}
	}
}

func (gt *GlobalTruth) rebuildSessionIndexLocked(sessionID string, engine *Engine) {
	gt.removeSessionIndexLocked(sessionID)
	if engine == nil {
		return
	}
	deps := map[string]struct{}{}
	for _, fact := range engine.ListFacts() {
		if fact == nil {
			continue
		}
		for _, set := range fact.JustificationSets {
			for _, token := range set {
				parentID := token
				if len(parentID) > 0 && parentID[0] == '!' {
					parentID = parentID[1:]
				}
				parentID = strings.TrimSpace(parentID)
				if parentID == "" {
					continue
				}
				if _, isGlobal := gt.facts[parentID]; !isGlobal {
					continue
				}
				deps[parentID] = struct{}{}
			}
		}
	}
	if len(deps) == 0 {
		return
	}
	gt.sessionDeps[sessionID] = deps
	for factID := range deps {
		if _, ok := gt.depIndex[factID]; !ok {
			gt.depIndex[factID] = map[string]struct{}{}
		}
		gt.depIndex[factID][sessionID] = struct{}{}
	}
}

// IndexFactDependencies incrementally records that sessionID references one or
// more global facts (directly) via fact.JustificationSets.
func (gt *GlobalTruth) IndexFactDependencies(sessionID string, fact *Fact) {
	if gt == nil || sessionID == "" || fact == nil {
		return
	}
	gt.mu.Lock()
	defer gt.mu.Unlock()

	if _, ok := gt.sessionDeps[sessionID]; !ok {
		gt.sessionDeps[sessionID] = map[string]struct{}{}
	}
	for _, set := range fact.JustificationSets {
		for _, token := range set {
			parentID := token
			if len(parentID) > 0 && parentID[0] == '!' {
				parentID = parentID[1:]
			}
			parentID = strings.TrimSpace(parentID)
			if parentID == "" {
				continue
			}
			if _, isGlobal := gt.facts[parentID]; !isGlobal {
				continue
			}
			gt.sessionDeps[sessionID][parentID] = struct{}{}
			if _, ok := gt.depIndex[parentID]; !ok {
				gt.depIndex[parentID] = map[string]struct{}{}
			}
			gt.depIndex[parentID][sessionID] = struct{}{}
		}
	}
}

func (gt *GlobalTruth) affectedSubscribersLocked(globalFactID string) []string {
	// Use Partition() connected-components on global facts to include
	// transitive influence through global fact dependencies.
	components := gt.partitionLocked()
	componentIDs := map[string]struct{}{globalFactID: {}}
	for _, comp := range components {
		for _, id := range comp {
			if id == globalFactID {
				componentIDs = map[string]struct{}{}
				for _, cid := range comp {
					componentIDs[cid] = struct{}{}
				}
				break
			}
		}
		if _, ok := componentIDs[globalFactID]; !ok {
			break
		}
	}

	seen := map[string]struct{}{}
	for factID := range componentIDs {
		for sid := range gt.depIndex[factID] {
			seen[sid] = struct{}{}
		}
	}
	out := make([]string, 0, len(seen))
	for sid := range seen {
		out = append(out, sid)
	}
	sort.Strings(out)
	return out
}

// withGlobalTruthFlag returns meta with "_global_truth" = true added.
// If meta is nil, a new map is allocated.
func withGlobalTruthFlag(meta map[string]interface{}) map[string]interface{} {
	out := make(map[string]interface{}, len(meta)+1)
	for k, v := range meta {
		out[k] = v
	}
	out["_global_truth"] = true
	return out
}
