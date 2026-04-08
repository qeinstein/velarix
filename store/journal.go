package store

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"velarix/core"
)

type EventType string

const (
	EventAssert               EventType = "assert"
	EventInvalidate           EventType = "invalidate"
	EventRetract              EventType = "retract"
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
	file, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
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
		}
	}

	return scanner.Err()
}
