package core

import (
	"errors"
	"sort"
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
}

// NewGlobalTruth returns an empty GlobalTruth store.
func NewGlobalTruth() *GlobalTruth {
	return &GlobalTruth{
		facts:       make(map[string]*Fact),
		retracted:   make(map[string]string),
		subscribers: make(map[string]*Engine),
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

	gt.facts[fact.ID] = cloneFact(fact)
	delete(gt.retracted, fact.ID)
	version := gt.version.Add(1)

	// Build the broadcast copy once; clone per subscriber to prevent aliasing.
	broadcast := cloneFact(fact)
	broadcast.Metadata = withGlobalTruthFlag(broadcast.Metadata)

	for _, eng := range gt.subscribers {
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

	for _, eng := range gt.subscribers {
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
