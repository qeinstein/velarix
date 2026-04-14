package api

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"velarix/core"
)

type contradictionPair struct {
	Left  *core.Fact
	Right *core.Fact
	Score float64
}

type openAIContradictionVerifier struct {
	apiKey  string
	baseURL string
	model   string
	client  *http.Client
}

type chatCompletionResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
}

type verifierFinding struct {
	Label      string  `json:"label"`
	Confidence float64 `json:"confidence"`
	Reason     string  `json:"reason"`
}

// verifierCallError carries a machine-readable reason so callers can
// increment the right Prometheus label without string-matching.
type verifierCallError struct {
	Reason string // "timeout" | "http" | "api_error" | "parse_error"
	Msg    string
}

func (e *verifierCallError) Error() string {
	return fmt.Sprintf("verifier %s: %s", e.Reason, e.Msg)
}

func contradictionVerifierFromEnv() *openAIContradictionVerifier {
	apiKey := strings.TrimSpace(os.Getenv("OPENAI_API_KEY"))
	model := strings.TrimSpace(os.Getenv("VELARIX_VERIFIER_MODEL"))
	if apiKey == "" || model == "" {
		return nil
	}
	baseURL := strings.TrimSpace(os.Getenv("VELARIX_OPENAI_BASE_URL"))
	if baseURL == "" {
		baseURL = "https://api.openai.com/v1"
	}
	return &openAIContradictionVerifier{
		apiKey:  apiKey,
		baseURL: strings.TrimRight(baseURL, "/"),
		model:   model,
		client:  &http.Client{Timeout: 20 * time.Second},
	}
}

func contradictionPairCandidates(engine *core.Engine, factIDs []string, maxPairs int) []contradictionPair {
	facts := []*core.Fact{}
	seen := map[string]struct{}{}
	if len(factIDs) == 0 {
		for _, fact := range engine.ListFacts() {
			factIDs = append(factIDs, fact.ID)
		}
	}
	for _, id := range factIDs {
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		fact, ok := engine.GetFact(id)
		if !ok {
			continue
		}
		if engine.GetStatus(id) < core.ConfidenceThreshold {
			continue
		}
		facts = append(facts, fact)
	}

	pairs := []contradictionPair{}
	for i := 0; i < len(facts); i++ {
		for j := i + 1; j < len(facts); j++ {
			left := facts[i]
			right := facts[j]
			score := core.CosineSimilarity(core.EmbeddingForFact(left), core.EmbeddingForFact(right))
			if score < 0.6 && !sharesStructuredClaim(left, right) {
				continue
			}
			pairs = append(pairs, contradictionPair{Left: left, Right: right, Score: score})
		}
	}
	sort.Slice(pairs, func(i, j int) bool {
		if pairs[i].Score == pairs[j].Score {
			if pairs[i].Left.ID == pairs[j].Left.ID {
				return pairs[i].Right.ID < pairs[j].Right.ID
			}
			return pairs[i].Left.ID < pairs[j].Left.ID
		}
		return pairs[i].Score > pairs[j].Score
	})
	if maxPairs <= 0 {
		maxPairs = 4
	}
	if len(pairs) > maxPairs {
		pairs = pairs[:maxPairs]
	}
	return pairs
}

func sharesStructuredClaim(a, b *core.Fact) bool {
	if a == nil || b == nil {
		return false
	}
	for _, key := range []string{"claim_key", "subject", "predicate"} {
		av := ""
		if a.Payload != nil {
			if v, ok := a.Payload[key].(string); ok {
				av = strings.TrimSpace(v)
			}
		}
		bv := ""
		if b.Payload != nil {
			if v, ok := b.Payload[key].(string); ok {
				bv = strings.TrimSpace(v)
			}
		}
		if av != "" && av == bv {
			return true
		}
	}
	return false
}

func factSummaryJSON(f *core.Fact) string {
	if f == nil {
		return "{}"
	}
	raw, _ := json.Marshal(map[string]interface{}{
		"id":       f.ID,
		"payload":  f.Payload,
		"metadata": f.Metadata,
	})
	return string(raw)
}

func (v *openAIContradictionVerifier) verifyPair(pair contradictionPair) (*verifierFinding, error) {
	body := map[string]interface{}{
		"model":       v.model,
		"temperature": 0,
		"messages": []map[string]string{
			{
				"role":    "system",
				"content": "You are a contradiction classifier for agent memory. Decide whether two facts contradict each other. Respond with JSON only: {\"label\":\"contradiction|neutral|support\",\"confidence\":0..1,\"reason\":\"short explanation\"}.",
			},
			{
				"role":    "user",
				"content": fmt.Sprintf("Fact A:\n%s\n\nFact B:\n%s\n\nReturn JSON only.", factSummaryJSON(pair.Left), factSummaryJSON(pair.Right)),
			},
		},
	}
	payload, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest(http.MethodPost, v.baseURL+"/chat/completions", bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+v.apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := v.client.Do(req)
	if err != nil {
		reason := "http"
		if req.Context().Err() != nil {
			reason = "timeout"
		}
		return nil, &verifierCallError{Reason: reason, Msg: err.Error()}
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return nil, &verifierCallError{
			Reason: "api_error",
			Msg:    fmt.Sprintf("verifier returned HTTP %d", resp.StatusCode),
		}
	}

	var parsed chatCompletionResponse
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		return nil, &verifierCallError{Reason: "parse_error", Msg: err.Error()}
	}
	if len(parsed.Choices) == 0 {
		return nil, &verifierCallError{Reason: "parse_error", Msg: "verifier returned no choices"}
	}
	content := strings.TrimSpace(parsed.Choices[0].Message.Content)
	var finding verifierFinding
	if err := json.Unmarshal([]byte(content), &finding); err != nil {
		return nil, &verifierCallError{Reason: "parse_error", Msg: "failed to unmarshal finding: " + err.Error()}
	}
	finding.Label = strings.ToLower(strings.TrimSpace(finding.Label))
	return &finding, nil
}

// appendVerifierIssues enriches report with LLM-verified contradictions.
// sessionID is used in structured log messages so failures can be traced to a
// specific session. Verifier failures are logged at slog.Warn — never silently
// dropped — and incremented in the velarix_verifier_failures_total counter.
func appendVerifierIssues(engine *core.Engine, report *core.ConsistencyReport, sessionID string) {
	verifier := contradictionVerifierFromEnv()
	if verifier == nil || report == nil {
		return
	}

	maxPairs := 4
	if raw := strings.TrimSpace(os.Getenv("VELARIX_VERIFIER_MAX_PAIRS")); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil && parsed > 0 && parsed <= 20 {
			maxPairs = parsed
		}
	}

	existing := map[string]struct{}{}
	for _, issue := range report.Issues {
		if len(issue.FactIDs) != 2 {
			continue
		}
		key := issue.FactIDs[0] + "|" + issue.FactIDs[1]
		existing[key] = struct{}{}
		existing[issue.FactIDs[1]+"|"+issue.FactIDs[0]] = struct{}{}
	}

	pairs := contradictionPairCandidates(engine, report.CheckedFactIDs, maxPairs)
	for _, pair := range pairs {
		if _, ok := existing[pair.Left.ID+"|"+pair.Right.ID]; ok {
			continue
		}
		finding, err := verifier.verifyPair(pair)
		if err != nil {
			// Classify the failure reason for the Prometheus counter.
			reason := "api_error"
			var vcErr *verifierCallError
			if errors.As(err, &vcErr) {
				reason = vcErr.Reason
			}
			slog.Warn("Consistency verifier call failed",
				"session_id", sessionID,
				"fact_id_a", pair.Left.ID,
				"fact_id_b", pair.Right.ID,
				"reason", reason,
				"error", err,
				"timestamp", time.Now().UnixMilli(),
			)
			VerifierFailures.WithLabelValues(reason).Inc()
			continue
		}
		if finding == nil || finding.Label != "contradiction" {
			continue
		}
		report.Issues = append(report.Issues, core.ConsistencyIssue{
			Type:               "model_verifier_contradiction",
			Severity:           "high",
			FactIDs:            []string{pair.Left.ID, pair.Right.ID},
			Message:            fmt.Sprintf("Verifier model judged %s and %s to be contradictory.", pair.Left.ID, pair.Right.ID),
			SuggestedAction:    "retract or revise one of the contradictory facts",
			Source:             "openai_verifier",
			VerifierModel:      verifier.model,
			VerifierLabel:      finding.Label,
			VerifierReason:     finding.Reason,
			VerifierConfidence: finding.Confidence,
		})
	}
	report.IssueCount = len(report.Issues)
}
