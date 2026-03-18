package tests

import (
	"fmt"
	"sync"
	"testing"
	"time"
	"velarix/api"
	"velarix/core"
	"velarix/store"
)

// Test A: 80k Cap Enforcement
func TestMemoryCapEnforcement(t *testing.T) {
	engine := core.NewEngine()

	// Fill to the limit
	for i := 0; i < core.MaxFactsPerSession; i++ {
		err := engine.AssertFact(&core.Fact{
			ID:     fmt.Sprintf("fact_%d", i),
			IsRoot: true,
			ManualStatus: core.Valid,
		})
		if err != nil {
			t.Fatalf("failed to insert fact %d: %v", i, err)
		}
	}

	// The 80,001st fact must fail
	err := engine.AssertFact(&core.Fact{
		ID:     "one_too_many",
		IsRoot: true,
		ManualStatus: core.Valid,
	})

	if err == nil {
		t.Fatal("expected error on 80,001st fact insertion, but got nil")
	}

	expectedErr := fmt.Sprintf("session memory cap exceeded (%d facts)", core.MaxFactsPerSession)
	if err.Error() != fmt.Sprintf("%s. please archive and start a new session", expectedErr) {
		t.Fatalf("unexpected error message: %v", err)
	}

	if len(engine.Facts) != core.MaxFactsPerSession {
		t.Fatalf("expected exactly %d facts, got %d", core.MaxFactsPerSession, len(engine.Facts))
	}
}

// Test B: TTL Eviction Accuracy
func TestTTLEviction(t *testing.T) {
	server := &api.Server{
		Engines:    make(map[string]*core.Engine),
		Configs:    make(map[string]*store.SessionConfig),
		LastAccess: make(map[string]time.Time),
	}

	// Session A: Stale (20 mins ago)
	server.Engines["stale"] = core.NewEngine()
	server.Configs["stale"] = &store.SessionConfig{}
	server.LastAccess["stale"] = time.Now().Add(-20 * time.Minute)

	// Session B: Active (just now)
	server.Engines["active"] = core.NewEngine()
	server.Configs["active"] = &store.SessionConfig{}
	server.LastAccess["active"] = time.Now()

	// Trigger sweep
	server.PerformEvictionSweep()

	if _, ok := server.Engines["stale"]; ok {
		t.Error("expected 'stale' session to be evicted, but it remains")
	}
	if _, ok := server.Engines["active"]; !ok {
		t.Error("expected 'active' session to remain, but it was evicted")
	}
}

// Test C: Parallel Invalidation Stress
func TestParallelRetraction(t *testing.T) {
	engine := core.NewEngine()

	// 1. Build a dense graph
	// Root -> 100 children -> each has 10 dependencies
	engine.AssertFact(&core.Fact{ID: "root", IsRoot: true, ManualStatus: core.Valid})
	
	for i := 0; i < 100; i++ {
		parentID := fmt.Sprintf("p_%d", i)
		engine.AssertFact(&core.Fact{
			ID: parentID,
			JustificationSets: [][]string{{"root"}},
		})
		for j := 0; j < 10; j++ {
			engine.AssertFact(&core.Fact{
				ID: fmt.Sprintf("child_%d_%d", i, j),
				JustificationSets: [][]string{{parentID}},
			})
		}
	}

	// 2. Stress it
	var wg sync.WaitGroup
	workers := 50
	iterations := 100

	wg.Add(workers * 2)

	// Writers: Concurrently toggle the root status
	for w := 0; w < workers; w++ {
		go func(id int) {
			defer wg.Done()
			for i := 0; i < iterations; i++ {
				if i % 2 == 0 {
					engine.InvalidateRoot("root")
				} else {
					// Manually update root back to valid
					engine.GetFact("root") // Just to touch it
					// Re-assertion is not supported by design yet, 
					// but we can simulate status checks.
				}
			}
		}(w)
	}

	// Readers: Concurrently check status and impact
	for w := 0; w < workers; w++ {
		go func(id int) {
			defer wg.Done()
			for i := 0; i < iterations; i++ {
				_ = engine.GetStatus("child_50_5")
				_ = engine.GetImpact("root")
			}
		}(w)
	}

	wg.Wait()
	// If we reached here without a panic or deadlock, the mutex structure is sound.
}

func TestSnapshotIntegrity(t *testing.T) {
	engine := core.NewEngine()
	engine.AssertFact(&core.Fact{ID: "F1", IsRoot: true, ManualStatus: core.Valid})
	
	// 1. Create Snapshot
	snap, err := engine.ToSnapshot()
	if err != nil {
		t.Fatal(err)
	}

	// 2. Restore in new engine
	engine2 := core.NewEngine()
	if err := engine2.FromSnapshot(snap); err != nil {
		t.Fatalf("failed to restore valid snapshot: %v", err)
	}
	if engine2.GetStatus("F1") != core.Valid {
		t.Fatal("restored state mismatch")
	}

	// 3. Corrupt Snapshot Data
	snap.Data[0] = ^snap.Data[0] // Flip first byte
	
	engine3 := core.NewEngine()
	err = engine3.FromSnapshot(snap)
	if err == nil {
		t.Fatal("expected error when restoring corrupted snapshot, but got nil")
	}
	if err.Error() != "snapshot integrity check failed: checksum mismatch" {
		t.Fatalf("unexpected error message: %v", err)
	}
}
