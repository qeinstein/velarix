// Package extractor converts raw LLM text output into structured FactInput
// objects that can be posted directly to POST /v1/s/{id}/perceptions or the
// extract-and-assert endpoint. It is the fact-extraction layer Velarix
// previously lacked.
package extractor

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"velarix/core"
)

// ExtractionError is a typed error returned when the extraction call fails so
// callers can distinguish extraction failures from downstream assertion errors.
type ExtractionError struct {
	Cause  string
	Detail string
}

func (e *ExtractionError) Error() string {
	if e.Detail != "" {
		return fmt.Sprintf("extraction error (%s): %s", e.Cause, e.Detail)
	}
	return fmt.Sprintf("extraction error (%s)", e.Cause)
}

// ExtractedFact is the raw parsed output of one atomic claim returned by the
// extraction model before it is converted to a core.Fact.
type ExtractedFact struct {
	ID          string   `json:"id"`
	Claim       string   `json:"claim"`
	ClaimKey    string   `json:"claim_key"`
	ClaimValue  string   `json:"claim_value"`
	Subject     string   `json:"subject"`
	Predicate   string   `json:"predicate"`
	Object      string   `json:"object"`
	Polarity    string   `json:"polarity"`
	IsRoot      bool     `json:"is_root"`
	DependsOn   []string `json:"depends_on"`
	SourceType  string   `json:"source_type"`
	Confidence  float64  `json:"confidence"`
}

// ToCoreFact converts an ExtractedFact into a core.Fact ready for engine
// assertion. Root facts get empty justification sets. Derived facts get a
// single-element AND-set per dependency entry ([][]string{{dep_id}}).
func (ef *ExtractedFact) ToCoreFact() *core.Fact {
	f := &core.Fact{
		ID:     ef.ID,
		IsRoot: ef.IsRoot,
		Payload: map[string]interface{}{
			"claim":       ef.Claim,
			"claim_key":   ef.ClaimKey,
			"claim_value": ef.ClaimValue,
			"subject":     ef.Subject,
			"predicate":   ef.Predicate,
			"object":      ef.Object,
			"polarity":    ef.Polarity,
		},
		Metadata: map[string]interface{}{
			"source_type": ef.SourceType,
			"claim_key":   ef.ClaimKey,
			"claim_value": ef.ClaimValue,
			"subject":     ef.Subject,
			"predicate":   ef.Predicate,
			"object":      ef.Object,
			"polarity":    ef.Polarity,
		},
	}
	if ef.Polarity == "" {
		f.Payload["polarity"] = "positive"
		f.Metadata["polarity"] = "positive"
	}
	if ef.SourceType == "" {
		f.Metadata["source_type"] = "llm_output"
		f.Payload["source_type"] = "llm_output"
	}

	if ef.IsRoot {
		conf := ef.Confidence
		if conf <= 0 {
			conf = 0.75
		}
		f.ManualStatus = core.Status(conf)
	} else {
		// Each dependency becomes a single-element AND-set.
		for _, depID := range ef.DependsOn {
			depID = strings.TrimSpace(depID)
			if depID != "" {
				f.JustificationSets = append(f.JustificationSets, []string{depID})
			}
		}
		// Non-root with no deps listed — treat as root with medium confidence.
		if len(f.JustificationSets) == 0 {
			f.IsRoot = true
			f.ManualStatus = core.Status(ef.Confidence)
			if f.ManualStatus <= 0 {
				f.ManualStatus = 0.7
			}
		}
	}
	return f
}

const extractionSystemPrompt = `You are a precise fact-extraction system. Your only job is to decompose the provided text into an array of atomic factual assertions.

CRITICAL RULES:
1. Return ONLY valid JSON — no preamble, no explanation, no markdown fences.
2. Decompose compound claims into separate atomic facts. "Paris is the capital of France and has a population of 2 million" must become TWO objects, not one.
3. Each fact must be a single, standalone, checkable assertion.
4. Assign a unique slug-format id to each fact (lowercase, hyphens, no spaces). Example: "paris-capital-france".
5. polarity must be exactly "positive" or "negative".
6. source_type must be exactly "llm_output".
7. confidence is a float 0.0–1.0 representing how confidently the source text asserts this claim.
8. is_root is true for direct premises stated in the input context; false for inferences derived from other extracted facts.
9. depends_on lists IDs of other extracted facts this one logically requires. Leave empty for root facts.

Return a JSON array where each element has exactly these fields:
[
  {
    "id": "slug-format-string",
    "claim": "the atomic factual assertion as plain text",
    "claim_key": "short_snake_case_label",
    "claim_value": "the asserted value",
    "subject": "what the claim is about",
    "predicate": "the relationship or property",
    "object": "the value or object",
    "polarity": "positive",
    "is_root": true,
    "depends_on": [],
    "source_type": "llm_output",
    "confidence": 0.9
  }
]`

// Extract sends llmOutput to an OpenAI-compatible endpoint and parses the
// structured JSON response into a slice of ExtractedFacts. sessionContext is
// optional — it gives the extractor domain context for better categorisation.
//
// Temperature is hardcoded to 0 for determinism. Timeout is 15 seconds.
// On failure, a typed *ExtractionError is always returned.
func Extract(ctx context.Context, llmOutput string, sessionContext string) ([]ExtractedFact, error) {
	apiKey := strings.TrimSpace(os.Getenv("OPENAI_API_KEY"))
	if apiKey == "" {
		return nil, &ExtractionError{Cause: "configuration", Detail: "OPENAI_API_KEY is not set"}
	}
	baseURL := strings.TrimSpace(os.Getenv("VELARIX_OPENAI_BASE_URL"))
	if baseURL == "" {
		baseURL = "https://api.openai.com/v1"
	}
	baseURL = strings.TrimRight(baseURL, "/")

	model := strings.TrimSpace(os.Getenv("VELARIX_VERIFIER_MODEL"))
	if model == "" {
		model = "gpt-4o-mini"
	}

	userContent := llmOutput
	if strings.TrimSpace(sessionContext) != "" {
		userContent = fmt.Sprintf("SESSION CONTEXT: %s\n\nLLM OUTPUT TO EXTRACT FROM:\n%s", sessionContext, llmOutput)
	}

	body := map[string]interface{}{
		"model":       model,
		"temperature": 0,
		"messages": []map[string]string{
			{"role": "system", "content": extractionSystemPrompt},
			{"role": "user", "content": userContent},
		},
	}
	payload, err := json.Marshal(body)
	if err != nil {
		return nil, &ExtractionError{Cause: "serialization", Detail: err.Error()}
	}

	// Enforce 15-second timeout even if ctx has a longer deadline.
	timeoutCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(timeoutCtx, http.MethodPost, baseURL+"/chat/completions", bytes.NewReader(payload))
	if err != nil {
		return nil, &ExtractionError{Cause: "request_build", Detail: err.Error()}
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		if timeoutCtx.Err() != nil {
			return nil, &ExtractionError{Cause: "timeout", Detail: "extraction HTTP call timed out after 15s"}
		}
		return nil, &ExtractionError{Cause: "http", Detail: err.Error()}
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		return nil, &ExtractionError{
			Cause:  "api_error",
			Detail: fmt.Sprintf("extraction model returned HTTP %d", resp.StatusCode),
		}
	}

	var chatResp struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&chatResp); err != nil {
		return nil, &ExtractionError{Cause: "parse_error", Detail: "failed to decode chat completion response: " + err.Error()}
	}
	if len(chatResp.Choices) == 0 {
		return nil, &ExtractionError{Cause: "parse_error", Detail: "extraction model returned no choices"}
	}

	content := strings.TrimSpace(chatResp.Choices[0].Message.Content)
	// Strip markdown fences if model disobeyed the instruction.
	content = stripMarkdownFences(content)

	var facts []ExtractedFact
	if err := json.Unmarshal([]byte(content), &facts); err != nil {
		return nil, &ExtractionError{Cause: "parse_error", Detail: "failed to parse extracted facts JSON: " + err.Error()}
	}

	// Sanitize: ensure required fields have fallbacks.
	for i := range facts {
		facts[i].ID = strings.TrimSpace(facts[i].ID)
		if facts[i].ID == "" {
			facts[i].ID = fmt.Sprintf("fact-%d", i)
		}
		if facts[i].SourceType == "" {
			facts[i].SourceType = "llm_output"
		}
		if facts[i].Polarity == "" {
			facts[i].Polarity = "positive"
		}
		if facts[i].Confidence <= 0 || facts[i].Confidence > 1 {
			facts[i].Confidence = 0.75
		}
	}

	return facts, nil
}

// stripMarkdownFences removes ```json ... ``` or ``` ... ``` wrappers in case
// the model ignores the no-markdown-fences instruction.
func stripMarkdownFences(s string) string {
	s = strings.TrimSpace(s)
	for _, prefix := range []string{"```json", "```"} {
		if strings.HasPrefix(s, prefix) {
			s = strings.TrimPrefix(s, prefix)
			break
		}
	}
	if strings.HasSuffix(s, "```") {
		s = strings.TrimSuffix(s, "```")
	}
	return strings.TrimSpace(s)
}
