package benchmark

import (
	"bufio"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	truthfulQAURL = "https://raw.githubusercontent.com/sylinrl/TruthfulQA/main/TruthfulQA.csv"
	haluEvalURL   = "https://datasets-server.huggingface.co/rows?dataset=pminervini%2FHaluEval&config=qa_samples&split=data&offset=0&length=10000"
)

// TruthfulQAQuestion represents a single row from the TruthfulQA CSV.
type TruthfulQAQuestion struct {
	Question         string
	BestAnswer       string
	CorrectAnswers   []string
	IncorrectAnswers []string
	Category         string
}

// HaluEvalQuestion represents a single QA sample from the HaluEval dataset.
type HaluEvalQuestion struct {
	Question     string
	Answer       string
	Hallucinated bool
}

// LoadTruthfulQA loads TruthfulQA from the cached file, downloading it first
// if not present. Cache path: benchmark/data/truthfulqa.csv.
func LoadTruthfulQA(cachePath string) ([]TruthfulQAQuestion, error) {
	if err := ensureDataFile(cachePath, truthfulQAURL); err != nil {
		return nil, fmt.Errorf("ensure truthfulqa cache: %w", err)
	}
	return parseTruthfulQACSV(cachePath)
}

// LoadHaluEval loads HaluEval QA samples from the cached file, downloading
// via the HuggingFace Datasets Server API if not present.
// Cache path: benchmark/data/halueval_qa.json.
func LoadHaluEval(cachePath string) ([]HaluEvalQuestion, error) {
	if err := ensureDataFile(cachePath, haluEvalURL); err != nil {
		return nil, fmt.Errorf("ensure halueval cache: %w", err)
	}
	return parseHaluEvalJSON(cachePath)
}

// ensureDataFile downloads url to path if path does not exist yet.
func ensureDataFile(path, url string) error {
	if _, err := os.Stat(path); err == nil {
		return nil // already cached
	}
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	slog.Info("Downloading dataset", "url", url, "dest", path)

	client := &http.Client{Timeout: 120 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return fmt.Errorf("download %s: %w", url, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return fmt.Errorf("download %s returned HTTP %d", url, resp.StatusCode)
	}

	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	if _, err := io.Copy(f, resp.Body); err != nil {
		return err
	}
	slog.Info("Dataset cached", "path", path)
	return nil
}

// parseTruthfulQACSV parses the TruthfulQA CSV file. The format is:
//
//	Type,Category,Question,Best Answer,Correct Answers,Incorrect Answers,Source
func parseTruthfulQACSV(path string) ([]TruthfulQAQuestion, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	r := csv.NewReader(bufio.NewReader(f))
	r.LazyQuotes = true
	r.FieldsPerRecord = -1

	headers, err := r.Read()
	if err != nil {
		return nil, fmt.Errorf("read header: %w", err)
	}

	// Build column index.
	colIdx := map[string]int{}
	for i, h := range headers {
		colIdx[strings.TrimSpace(strings.ToLower(h))] = i
	}

	questionCol := colIdx["question"]
	bestAnswerCol, hasBest := colIdx["best answer"]
	correctCol, hasCorrect := colIdx["correct answers"]
	incorrectCol, hasIncorrect := colIdx["incorrect answers"]
	categoryCol, hasCategory := colIdx["category"]

	var out []TruthfulQAQuestion
	for {
		record, err := r.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			continue // skip malformed rows
		}
		if questionCol >= len(record) {
			continue
		}

		q := TruthfulQAQuestion{
			Question: strings.TrimSpace(record[questionCol]),
		}
		if q.Question == "" {
			continue
		}
		if hasBest && bestAnswerCol < len(record) {
			q.BestAnswer = strings.TrimSpace(record[bestAnswerCol])
		}
		if hasCorrect && correctCol < len(record) {
			q.CorrectAnswers = splitSemicolon(record[correctCol])
		}
		if hasIncorrect && incorrectCol < len(record) {
			q.IncorrectAnswers = splitSemicolon(record[incorrectCol])
		}
		if hasCategory && categoryCol < len(record) {
			q.Category = strings.TrimSpace(record[categoryCol])
		}
		out = append(out, q)
	}
	return out, nil
}

// parseHaluEvalJSON parses the HuggingFace Datasets Server JSON response.
// The API returns: {"rows": [{"row": {"question": "...", "answer": "...",
// "hallucination": "yes"|"no"}}, ...]}
func parseHaluEvalJSON(path string) ([]HaluEvalQuestion, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	// Try the HuggingFace Datasets Server envelope first.
	var envelope struct {
		Rows []struct {
			Row json.RawMessage `json:"row"`
		} `json:"rows"`
	}
	if err := json.Unmarshal(data, &envelope); err == nil && len(envelope.Rows) > 0 {
		var out []HaluEvalQuestion
		for _, row := range envelope.Rows {
			q, err := parseHaluEvalRow(row.Row)
			if err != nil {
				continue
			}
			out = append(out, q)
		}
		return out, nil
	}

	// Fallback: plain JSON array of objects.
	var rows []json.RawMessage
	if err := json.Unmarshal(data, &rows); err != nil {
		return nil, fmt.Errorf("parse halueval json: %w", err)
	}
	var out []HaluEvalQuestion
	for _, row := range rows {
		q, err := parseHaluEvalRow(row)
		if err != nil {
			continue
		}
		out = append(out, q)
	}
	return out, nil
}

func parseHaluEvalRow(raw json.RawMessage) (HaluEvalQuestion, error) {
	var m map[string]interface{}
	if err := json.Unmarshal(raw, &m); err != nil {
		return HaluEvalQuestion{}, err
	}

	q := HaluEvalQuestion{}
	for _, key := range []string{"question", "query", "input"} {
		if v, ok := m[key].(string); ok && v != "" {
			q.Question = v
			break
		}
	}
	for _, key := range []string{"answer", "response", "output"} {
		if v, ok := m[key].(string); ok && v != "" {
			q.Answer = v
			break
		}
	}
	// hallucination field: "yes"|"no" or bool
	for _, key := range []string{"hallucination", "hallucinated", "label"} {
		if v, ok := m[key]; ok {
			switch val := v.(type) {
			case bool:
				q.Hallucinated = val
			case string:
				q.Hallucinated = strings.EqualFold(strings.TrimSpace(val), "yes") ||
					strings.EqualFold(strings.TrimSpace(val), "true") ||
					strings.EqualFold(strings.TrimSpace(val), "1")
			}
			break
		}
	}

	if q.Question == "" {
		return HaluEvalQuestion{}, fmt.Errorf("no question field found")
	}
	return q, nil
}

func splitSemicolon(s string) []string {
	var out []string
	for _, part := range strings.Split(s, ";") {
		part = strings.TrimSpace(part)
		if part != "" {
			out = append(out, part)
		}
	}
	return out
}
