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
			reason := ""
			if entry.Payload != nil {
				if v, ok := entry.Payload["reason"].(string); ok {
					reason = v
				}
			}
			if err := engine.RetractFact(entry.FactID, reason); err != nil {
				return fmt.Errorf("line %d [Session: %s]: failed to replay retract: %w", lineNum, entry.SessionID, err)
			}

		case EventReview:
			status := ""
			reason := ""
			reviewedAt := entry.Timestamp
			if entry.Payload != nil {
				if v, ok := entry.Payload["status"].(string); ok {
					status = v
				}
				if v, ok := entry.Payload["reason"].(string); ok {
					reason = v
				}
				if v, ok := entry.Payload["reviewed_at"].(float64); ok {
					reviewedAt = int64(v)
				}
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
			var newConfidence float64
			if entry.Payload != nil {
				if v, ok := entry.Payload["confidence"].(float64); ok {
					newConfidence = v
				}
			}
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

		default:
			// Future event types must never be silently lost.
			slog.Warn("Replaying journal: unknown event type encountered — skipping",
				"line", lineNum, "session_id", entry.SessionID, "type", entry.Type)
		}
	}

	return scanner.Err()
}
