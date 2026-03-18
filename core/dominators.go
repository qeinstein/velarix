package core

import (
	"sort"
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
			for _, jsID := range e.getIncomingJustificationSets(id) {
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

	// Sort adjacency list for deterministic ordering
	for _, children := range adj {
		sort.Strings(children)
	}
	sort.Strings(roots)

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
func (e *Engine) lca(id1, id2 string) string {
	if id1 == "" { return id2 }
	if id2 == "" { return id1 }
	if id1 == id2 { return id1 }

	// Simple path climbing for LCA (can be optimized further if needed)
	path1 := e.getDominatorPath(id1)
	path2 := e.getDominatorPath(id2)

	i := len(path1) - 1
	j := len(path2) - 1
	var lastCommon string

	for i >= 0 && j >= 0 {
		if path1[i] == path2[j] {
			lastCommon = path1[i]
		} else {
			break
		}
		i--
		j--
	}
	return lastCommon
}

func (e *Engine) getDominatorPath(id string) []string {
	var path []string
	curr := id
	for curr != "" {
		path = append(path, curr)
		if f, ok := e.Facts[curr]; ok {
			curr = f.IDom
		} else if js, ok := e.JustificationSets[curr]; ok {
			curr = js.IDom
		} else {
			break
		}
	}
	return path
}

func (e *Engine) topologicalSort() []string {
	visited := make(map[string]bool)
	var order []string
	
	var visit func(string)
	visit = func(id string) {
		if visited[id] { return }
		visited[id] = true
		
		if _, ok := e.Facts[id]; ok {
			for jsID := range e.ChildrenIndex[id] {
				visit(jsID)
			}
		} else if js, ok := e.JustificationSets[id]; ok {
			visit(js.ChildFactID)
		}
		order = append([]string{id}, order...)
	}

	for id, f := range e.Facts {
		if f.IsRoot {
			visit(id)
		}
	}
	return order
}

// getIncomingJustificationSets finds all sets that justify a given fact.
func (e *Engine) getIncomingJustificationSets(factID string) []string {
	var results []string
	for id, js := range e.JustificationSets {
		if js.ChildFactID == factID {
			results = append(results, id)
		}
	}
	return results
}

// IsAncestor checks if 'u' is an ancestor of 'v' in the Dominator Tree in O(1).
func (e *Engine) isDominatorAncestor(uID, vID string) bool {
	u, okU := e.Facts[uID]
	v, okV := e.Facts[vID]
	if !okU || !okV {
		return false
	}
	// Root condition: every node is dominated by its ancestors.
	return u.PreOrder <= v.PreOrder && u.PostOrder >= v.PostOrder
}
