package store

import (
	"bufio"
	"encoding/json"
	"os"

	"causaldb/core"
)


type EventType string

const (
	EventAssert     EventType = "assert"
	EventInvalidate EventType = "invalidate"
)

type JournalEntry struct {
	Type   EventType   `json:"type"`
	Fact   *core.Fact  `json:"fact,omitempty"`
	FactID string      `json:"fact_id,omitempty"`
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


func (j *Journal) AppendAssert(f *core.Fact) error {
	entry := JournalEntry{
		Type: EventAssert,
		Fact: f,
	}

	return j.append(entry)
}


func (j *Journal) AppendInvalidate(factID string) error {
	entry := JournalEntry{
		Type:   EventInvalidate,
		FactID: factID,
	}

	return j.append(entry)
}


func (j *Journal) append(entry JournalEntry) error {
	bytes, err := json.Marshal(entry)
	if err != nil {
		return err
	}

	_, err = j.file.Write(append(bytes, '\n'))
	return err
}


func Replay(path string, engine *core.Engine) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)

	for scanner.Scan() {
		var entry JournalEntry
		if err := json.Unmarshal(scanner.Bytes(), &entry); err != nil {
			return err
		}

		switch entry.Type {
		case EventAssert:
			if err := engine.AssertFact(entry.Fact); err != nil {
				return err
			}

		case EventInvalidate:
			if err := engine.InvalidateRoot(entry.FactID); err != nil {
				return err
			}
		}
	}

	return scanner.Err()
}

