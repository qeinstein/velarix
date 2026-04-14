package store

import (
	"bufio"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"time"

	"velarix/core"
)

type EventType string

const (
	EventAssert               EventType = "assert"
	EventInvalidate           EventType = "invalidate"
	EventRetract              EventType = "retract"
	EventReview               EventType = "review"
	EventCycleViolation       EventType = "cycle_violation"
	EventSnapshotCorruption   EventType = "snapshot_corruption"
	EventConfidenceAdjusted   EventType = "confidence_adjusted"
	EventRevalidationComplete EventType = "revalidation_complete"
	EventAdminAction          EventType = "admin_action"
	EventDecisionRecord       EventType = "decision_record"
	// EventFactExpired is written by the retention sweep when a fact's ValidUntil
	// has passed. On replay, the fact is retracted with reason "expired" so that
	// downstream dependents are re-propagated correctly.
	EventFactExpired EventType = "fact_expired"

	// EventFactVerification records a verification status update for a fact.
	// On replay, the fact's verification metadata is re-applied so execution
	// gating and grounding checks remain consistent across reloads.
	EventFactVerification EventType = "fact_verification"
)

type JournalEntry struct {
	Type      EventType              `json:"type"`
	SessionID string                 `json:"session_id"`
	ActorID   string                 `json:"actor_id,omitempty"` // ID of the user or API key that performed the action
	Fact      *core.Fact             `json:"fact,omitempty"`
	FactID    string                 `json:"fact_id,omitempty"`
	Payload   map[string]interface{} `json:"payload,omitempty"`
	Timestamp int64                  `json:"timestamp"`
}

type Journal struct {
	file *os.File
}

func OpenJournal(path string) (*Journal, error) {
	file, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		return nil, err
	}

	return &Journal{file: file}, nil
}

func (j *Journal) AppendAssert(sessionID string, f *core.Fact) error {
	entry := JournalEntry{
		Type:      EventAssert,
		SessionID: sessionID,
		Fact:      f,
	}

	return j.append(entry)
}

func (j *Journal) AppendInvalidate(sessionID string, factID string) error {
	entry := JournalEntry{
		Type:      EventInvalidate,
		SessionID: sessionID,
		FactID:    factID,
	}

	return j.append(entry)
}

func (j *Journal) append(entry JournalEntry) error {
	entry.Timestamp = time.Now().UnixMilli()
	bytes, err := json.Marshal(entry)
	if err != nil {
		return err
	}

	if _, err := j.file.Write(append(bytes, '\n')); err != nil {
		return err
	}
	return j.file.Sync()
}

func (j *Journal) ReadHistory() ([]JournalEntry, error) {
	// Re-open for reading
	file, err := os.Open(j.file.Name())
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var entries []JournalEntry
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		var entry JournalEntry
		if err := json.Unmarshal(scanner.Bytes(), &entry); err != nil {
			return nil, err
		}
		entries = append(entries, entry)
	}

	return entries, scanner.Err()
}

func Replay(path string, engines map[string]*core.Engine) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)

	lineNum := 0
	for scanner.Scan() {
		lineNum++
		var entry JournalEntry
		if err := json.Unmarshal(scanner.Bytes(), &entry); err != nil {
			return fmt.Errorf("line %d: corrupt journal entry: %w", lineNum, err)
		}

		if err := replayEntry(lineNum, entry, engines); err != nil {
			return err
		}
	}

	return scanner.Err()
}

func replayEntry(lineNum int, entry JournalEntry, engines map[string]*core.Engine) error {
	// STRICT CHECK: Orphaned entry detection
	if entry.SessionID == "" {
		return fmt.Errorf("line %d: critical failure - journal entry has no SessionID", lineNum)
	}

	engine, ok := engines[entry.SessionID]
	if !ok {
		// Lazy initialize engine during replay
		engine = core.NewEngine()
		engines[entry.SessionID] = engine
	}

	switch entry.Type {
	case EventAssert:
		if err := engine.AssertFact(entry.Fact); err != nil {
			return fmt.Errorf("line %d [Session: %s]: failed to replay assert: %w", lineNum, entry.SessionID, err)
		}

	case EventInvalidate:
		if err := engine.InvalidateRoot(entry.FactID); err != nil {
			return fmt.Errorf("line %d [Session: %s]: failed to replay invalidate: %w", lineNum, entry.SessionID, err)
		}

	case EventRetract:
		reason := payloadString(entry.Payload, "reason")
		if err := engine.RetractFact(entry.FactID, reason); err != nil {
			return fmt.Errorf("line %d [Session: %s]: failed to replay retract: %w", lineNum, entry.SessionID, err)
		}

	case EventReview:
		status := payloadString(entry.Payload, "status")
		reason := payloadString(entry.Payload, "reason")
		reviewedAt := entry.Timestamp
		if v, ok := payloadInt64FromFloat(entry.Payload, "reviewed_at"); ok {
			reviewedAt = v
		}
		if err := engine.SetFactReview(entry.FactID, status, reason, reviewedAt); err != nil {
			return fmt.Errorf("line %d [Session: %s]: failed to replay review: %w", lineNum, entry.SessionID, err)
		}

	case EventCycleViolation:
		// Cycle violations are validation failures, not state mutations.
		// No engine action is needed; log for audit visibility.
		slog.Debug("Replaying journal: skipping cycle_violation event (no state mutation)",
			"line", lineNum, "session_id", entry.SessionID, "fact_id", entry.FactID)

	case EventSnapshotCorruption:
		// Snapshot corruption is a critical storage integrity event.
		// Log at Error level with full payload so operators can investigate.
		// The session should be flagged for manual review.
		slog.Error("Replaying journal: snapshot_corruption event detected — session requires manual review",
			"line", lineNum, "session_id", entry.SessionID, "payload", entry.Payload, "timestamp", entry.Timestamp)

	case EventConfidenceAdjusted:
		// Re-apply a confidence adjustment to a root fact's ManualStatus.
		newConfidence, _ := payloadFloat64(entry.Payload, "confidence")
		factID := entry.FactID
		if factID == "" && entry.Fact != nil {
			factID = entry.Fact.ID
		}
		if factID != "" && newConfidence > 0 {
			if err := engine.SetRootConfidence(factID, core.Status(newConfidence)); err != nil {
				slog.Warn("Replaying journal: confidence_adjusted apply failed",
					"line", lineNum, "session_id", entry.SessionID, "fact_id", factID, "error", err)
			}
		} else {
			slog.Debug("Replaying journal: skipping confidence_adjusted event — missing fact_id or confidence",
				"line", lineNum, "session_id", entry.SessionID)
		}

	case EventRevalidationComplete:
		// Informational event — marks that a full session revalidation ran.
		// No state to mutate; the revalidation itself is the state change.
		slog.Debug("Replaying journal: skipping revalidation_complete event (informational only)",
			"line", lineNum, "session_id", entry.SessionID)

	case EventAdminAction:
		// Admin actions are auditable but not replayable state mutations.
		actorID := entry.ActorID
		if actorID == "" {
			actorID = "unknown"
		}
		slog.Info("Replaying journal: admin_action event (audit only, no state mutation)",
			"line", lineNum, "session_id", entry.SessionID, "actor_id", actorID, "payload", entry.Payload)

	case EventDecisionRecord:
		// Decision records are audit trail only — not engine state.
		slog.Debug("Replaying journal: skipping decision_record event (audit trail only)",
			"line", lineNum, "session_id", entry.SessionID, "fact_id", entry.FactID)

	case EventFactExpired:
		// Re-apply a temporal-decay retraction written by SweepExpiredFacts.
		factID := entry.FactID
		if factID == "" && entry.Fact != nil {
			factID = entry.Fact.ID
		}
		if factID != "" {
			if err := engine.RetractFact(factID, "expired"); err != nil {
				slog.Warn("Replaying journal: fact_expired retraction failed",
					"line", lineNum, "session_id", entry.SessionID, "fact_id", factID, "error", err)
			}
		} else {
			slog.Warn("Replaying journal: skipping fact_expired event — missing fact_id",
				"line", lineNum, "session_id", entry.SessionID)
		}

	case EventFactVerification:
		// Re-apply verification metadata for grounding/execution gating.
		factID := entry.FactID
		if factID == "" && entry.Fact != nil {
			factID = entry.Fact.ID
		}
		status := payloadString(entry.Payload, "status")
		method := payloadString(entry.Payload, "method")
		sourceRef := payloadString(entry.Payload, "source_ref")
		reason := payloadString(entry.Payload, "reason")
		verifiedAt := entry.Timestamp
		if v, ok := payloadInt64FromFloat(entry.Payload, "verified_at"); ok && v > 0 {
			verifiedAt = v
		}
		if factID != "" {
			if err := engine.SetFactVerification(factID, status, method, sourceRef, reason, verifiedAt); err != nil {
				slog.Warn("Replaying journal: fact_verification apply failed",
					"line", lineNum, "session_id", entry.SessionID, "fact_id", factID, "error", err)
			}
		} else {
			slog.Warn("Replaying journal: skipping fact_verification event — missing fact_id",
				"line", lineNum, "session_id", entry.SessionID)
		}

	default:
		// Future event types must never be silently lost.
		slog.Warn("Replaying journal: unknown event type encountered — skipping",
			"line", lineNum, "session_id", entry.SessionID, "type", entry.Type)
	}
	return nil
}

func payloadString(payload map[string]interface{}, key string) string {
	if payload == nil {
		return ""
	}
	v, ok := payload[key]
	if !ok {
		return ""
	}
	s, ok := v.(string)
	if !ok {
		return ""
	}
	return s
}

func payloadFloat64(payload map[string]interface{}, key string) (float64, bool) {
	if payload == nil {
		return 0, false
	}
	v, ok := payload[key]
	if !ok {
		return 0, false
	}
	f, ok := v.(float64)
	return f, ok
}

func payloadInt64FromFloat(payload map[string]interface{}, key string) (int64, bool) {
	f, ok := payloadFloat64(payload, key)
	if !ok {
		return 0, false
	}
	return int64(f), true
}
