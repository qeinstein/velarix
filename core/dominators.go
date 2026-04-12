package core

import (
	"slices"
)

// recomputeDominators builds the Dominator Tree for the AND/OR graph.
// It also assigns PreOrder and PostOrder indices for O(1) ancestor checks.
// Callers MUST hold e.mu.Lock().
func (e *Engine) recomputeDominators() {
	// 1. Topological Sort
	order := e.topologicalSort()

	// 2. Reset IDoms
	for _, f := range e.Facts {
		f.IDom = ""
	}
	for _, js := range e.JustificationSets {
		js.IDom = ""
	}

	// 3. Compute IDoms using LCA
	// Roots have no IDom.
	for _, id := range order {
		if f, ok := e.Facts[id]; ok {
			if f.IsRoot {
				continue
			}
			// IDom(Fact) = LCA of all its JustificationSets
			var commonIDom string
			first := true
			for jsID := range e.ChildJSetIndex[id] {
				js := e.JustificationSets[jsID]
				if first {
					commonIDom = js.IDom
					first = false
				} else {
					commonIDom = e.lca(commonIDom, js.IDom)
				}
			}
			f.IDom = commonIDom
		} else if js, ok := e.JustificationSets[id]; ok {
			// IDom(JustificationSet) = LCA of all its ParentFacts
			var commonIDom string
			first := true
			for _, pID := range js.ParentFactIDs {
				if first {
					commonIDom = pID
					first = false
				} else {
					commonIDom = e.lca(commonIDom, pID)
				}
			}
			js.IDom = commonIDom
		}
	}

	// 4. Build Dominator Tree Adjacency List and Compute Intervals
	e.computeDominatorIntervals()

	e.DirtyDominators = false
}

// computeDominatorIntervals assigns PreOrder and PostOrder indices for O(1) checks.
func (e *Engine) computeDominatorIntervals() {
	adj := make(map[string][]string)
	var roots []string

	for id, f := range e.Facts {
		if f.IDom == "" {
			roots = append(roots, id)
		} else {
			adj[f.IDom] = append(adj[f.IDom], id)
		}
	}
	for id, js := range e.JustificationSets {
		if js.IDom != "" {
			adj[js.IDom] = append(adj[js.IDom], id)
		}
	}

	// Deterministic ordering removed: PreOrder/PostOrder interval correctness does
	// not depend on DFS child-visit order, only on proper nesting.  Sorting every
	// adjacency list on every recompute (O(k log k) per node) is unnecessary work.

	timer := 0
	var dfs func(string)
	dfs = func(u string) {
		timer++
		if f, ok := e.Facts[u]; ok {
			f.PreOrder = timer
		}
		// Note: JustificationSets also need intervals if we check them,
		// but usually we only query Fact status.

		for _, v := range adj[u] {
			dfs(v)
		}

		timer++
		if f, ok := e.Facts[u]; ok {
			f.PostOrder = timer
		}
	}

	for _, root := range roots {
		dfs(root)
	}
}

// lca finds the Lowest Common Ancestor in the current Dominator Tree.
//
// Uses a two-phase approach: build an ancestor set for id1 by walking its
// IDom chain (O(depth), one map allocation), then walk id2's chain until a
// member of that set is found.  This avoids the previous approach of
// allocating two path slices and scanning them in reverse.
func (e *Engine) lca(id1, id2 string) string {
	if id1 == "" {
		return id2
	}
	if id2 == "" {
		return id1
	}
	if id1 == id2 {
		return id1
	}

	// Build ancestor set for id1
	ancestors := make(map[string]struct{})
	for curr := id1; curr != ""; {
		ancestors[curr] = struct{}{}
		if f, ok := e.Facts[curr]; ok {
			curr = f.IDom
		} else if js, ok := e.JustificationSets[curr]; ok {
			curr = js.IDom
		} else {
			break
		}
	}

	// Walk id2's chain until we hit a member of id1's ancestor set
	for curr := id2; curr != ""; {
		if _, ok := ancestors[curr]; ok {
			return curr
		}
		if f, ok := e.Facts[curr]; ok {
			curr = f.IDom
		} else if js, ok := e.JustificationSets[curr]; ok {
			curr = js.IDom
		} else {
			break
		}
	}
	return ""
}

func (e *Engine) topologicalSort() []string {
	visited := make(map[string]bool)
	// Collect in post-order (append), then reverse once — O(n) total.
	// The previous approach prepended on every visit: O(n²).
	order := make([]string, 0, len(e.Facts)+len(e.JustificationSets))

	var visit func(string)
	visit = func(id string) {
		if visited[id] {
			return
		}
		visited[id] = true

		if _, ok := e.Facts[id]; ok {
			for jsID := range e.ChildrenIndex[id] {
				visit(jsID)
			}
		} else if js, ok := e.JustificationSets[id]; ok {
			visit(js.ChildFactID)
		}
		order = append(order, id)
	}

	for id := range e.Facts {
		visit(id)
	}

	// Reverse in-place: post-order → topological order
	slices.Reverse(order)
	return order
}

// getIncomingJustificationSets returns all JustificationSet IDs that justify factID.
// Uses the ChildJSetIndex reverse index for O(1) lookup instead of a full scan.
func (e *Engine) getIncomingJustificationSets(factID string) []string {
	jSets := e.ChildJSetIndex[factID]
	if len(jSets) == 0 {
		return nil
	}
	results := make([]string, 0, len(jSets))
	for id := range jSets {
		results = append(results, id)
	}
	return results
}

// isDominatorAncestor checks if 'u' is an ancestor of 'v' in the Dominator Tree in O(1).
func (e *Engine) isDominatorAncestor(uID, vID string) bool {
	u, okU := e.Facts[uID]
	v, okV := e.Facts[vID]
	if !okU || !okV {
		return false
	}
	// Root condition: every node is dominated by its ancestors.
	return u.PreOrder <= v.PreOrder && u.PostOrder >= v.PostOrder
}
