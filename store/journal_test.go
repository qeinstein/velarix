package store_test

import (
	"encoding/json"
	"os"
	"testing"

	"velarix/core"
	"velarix/store"
)

func writeJournalLines(t *testing.T, path string, entries []store.JournalEntry) {
	t.Helper()
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	for _, e := range entries {
		b, _ := json.Marshal(e)
		f.Write(append(b, '\n'))
	}
}

func fact(id string, isRoot bool, conf float64) *core.Fact {
	return &core.Fact{
		ID:           id,
		IsRoot:       isRoot,
		ManualStatus: core.Status(conf),
	}
}

// TestJournalReplay_AllNineEventTypes writes all 9 event types to a journal
// file and replays them, verifying the resulting engine state for each.
func TestJournalReplay_AllNineEventTypes(t *testing.T) {
	tmp := t.TempDir()
	path := tmp + "/test.journal"

	const sid = "sess-1"

	entries := []store.JournalEntry{
		// 1. assert root1 (valid)
		{Type: store.EventAssert, SessionID: sid, Fact: fact("root1", true, 0.9)},
		// 2. assert root2 (valid) — used for confidence_adjusted later
		{Type: store.EventAssert, SessionID: sid, Fact: fact("root2", true, 0.9)},
		// 3. assert derived1 depending on root1
		{Type: store.EventAssert, SessionID: sid, Fact: &core.Fact{
			ID:                "derived1",
			IsRoot:            false,
			JustificationSets: [][]string{{"root1"}},
		}},
		// 4. invalidate root1 → derived1 should become invalid
		{Type: store.EventInvalidate, SessionID: sid, FactID: "root1"},
		// 5. retract derived1
		{Type: store.EventRetract, SessionID: sid, FactID: "derived1",
			Payload: map[string]interface{}{"reason": "test_retract"}},
		// 6. review root2
		{Type: store.EventReview, SessionID: sid, FactID: "root2",
			Payload: map[string]interface{}{"status": "approved", "reason": "looks good", "reviewed_at": float64(1700000000000)}},
		// 7. cycle_violation — skip, no state mutation
		{Type: store.EventCycleViolation, SessionID: sid, FactID: "hypothetical-fact"},
		// 8. snapshot_corruption — skip (logged only)
		{Type: store.EventSnapshotCorruption, SessionID: sid,
			Payload: map[string]interface{}{"detail": "checksum mismatch"}},
		// 9a. confidence_adjusted — update root2 to 0.75
		{Type: store.EventConfidenceAdjusted, SessionID: sid, FactID: "root2",
			Payload: map[string]interface{}{"confidence": 0.75}},
		// 9b. revalidation_complete — skip (informational)
		{Type: store.EventRevalidationComplete, SessionID: sid},
		// 9c. admin_action — skip (audit only)
		{Type: store.EventAdminAction, SessionID: sid, ActorID: "admin-user",
			Payload: map[string]interface{}{"action": "manual_override"}},
		// 9d. decision_record — skip (audit trail)
		{Type: store.EventDecisionRecord, SessionID: sid, FactID: "decision-123"},
	}

	writeJournalLines(t, path, entries)

	engines := map[string]*core.Engine{}
	if err := store.Replay(path, engines); err != nil {
		t.Fatalf("Replay failed: %v", err)
	}

	engine, ok := engines[sid]
	if !ok {
		t.Fatal("no engine created for session")
	}

	// root1: invalidated → below threshold
	if engine.GetStatus("root1") >= core.ConfidenceThreshold {
		t.Errorf("root1 should be below threshold after invalidation, got %v", engine.GetStatus("root1"))
	}

	// derived1: retracted → below threshold
	if engine.GetStatus("derived1") >= core.ConfidenceThreshold {
		t.Errorf("derived1 should be below threshold after retraction, got %v", engine.GetStatus("derived1"))
	}

	// root2: confidence_adjusted to 0.75 → above threshold
	if engine.GetStatus("root2") < core.ConfidenceThreshold {
		t.Errorf("root2 should be above threshold after confidence_adjusted to 0.75, got %v", engine.GetStatus("root2"))
	}
	root2, exists := engine.GetFact("root2")
	if !exists {
		t.Fatal("root2 not found in engine")
	}
	if float64(root2.ManualStatus) != 0.75 {
		t.Errorf("root2.ManualStatus = %v, want 0.75", root2.ManualStatus)
	}

	// root2: review should have set ReviewStatus
	if root2.ReviewStatus != "approved" {
		t.Errorf("root2.ReviewStatus = %q, want %q", root2.ReviewStatus, "approved")
	}
}

// TestJournalReplay_UnknownEventType verifies that an unknown event type does
// not cause Replay to return an error — it is skipped with a warning.
func TestJournalReplay_UnknownEventType(t *testing.T) {
	tmp := t.TempDir()
	path := tmp + "/test.journal"

	entries := []store.JournalEntry{
		{Type: store.EventAssert, SessionID: "s1", Fact: fact("r1", true, 0.9)},
		{Type: store.EventType("future_event_type_v2"), SessionID: "s1", FactID: "r1"},
	}
	writeJournalLines(t, path, entries)

	engines := map[string]*core.Engine{}
	if err := store.Replay(path, engines); err != nil {
		t.Fatalf("unknown event type should not abort replay, got: %v", err)
	}
	// The assert should still have been applied.
	if engines["s1"].GetStatus("r1") < core.ConfidenceThreshold {
		t.Errorf("expected r1 to be above threshold after assert + unknown skip, got %v", engines["s1"].GetStatus("r1"))
	}
}

// TestJournalReplay_MultipleSessionsIsolated verifies that events for different
// session IDs produce separate, independent engines.
func TestJournalReplay_MultipleSessionsIsolated(t *testing.T) {
	tmp := t.TempDir()
	path := tmp + "/test.journal"

	entries := []store.JournalEntry{
		{Type: store.EventAssert, SessionID: "sa", Fact: fact("fact-a", true, 0.9)},
		{Type: store.EventAssert, SessionID: "sb", Fact: fact("fact-b", true, 0.9)},
		{Type: store.EventInvalidate, SessionID: "sa", FactID: "fact-a"},
	}
	writeJournalLines(t, path, entries)

	engines := map[string]*core.Engine{}
	if err := store.Replay(path, engines); err != nil {
		t.Fatalf("Replay failed: %v", err)
	}

	if engines["sa"].GetStatus("fact-a") >= core.ConfidenceThreshold {
		t.Errorf("sa: fact-a should be below threshold after invalidation, got %v", engines["sa"].GetStatus("fact-a"))
	}
	if engines["sb"].GetStatus("fact-b") < core.ConfidenceThreshold {
		t.Errorf("sb: fact-b should remain above threshold — invalidation in sa must not affect sb, got %v", engines["sb"].GetStatus("fact-b"))
	}
}

// TestJournalReplay_ConfidenceAdjustedBelowThreshold verifies that
// confidence_adjusted with a value < 0.6 marks the root as Invalid.
func TestJournalReplay_ConfidenceAdjustedBelowThreshold(t *testing.T) {
	tmp := t.TempDir()
	path := tmp + "/test.journal"

	entries := []store.JournalEntry{
		{Type: store.EventAssert, SessionID: "s1", Fact: fact("root1", true, 0.9)},
		{Type: store.EventConfidenceAdjusted, SessionID: "s1", FactID: "root1",
			Payload: map[string]interface{}{"confidence": 0.2}},
	}
	writeJournalLines(t, path, entries)

	engines := map[string]*core.Engine{}
	if err := store.Replay(path, engines); err != nil {
		t.Fatalf("Replay failed: %v", err)
	}
	if engines["s1"].GetStatus("root1") >= core.ConfidenceThreshold {
		t.Errorf("root1 should be below threshold after confidence drop to 0.2, got %v", engines["s1"].GetStatus("root1"))
	}
}
