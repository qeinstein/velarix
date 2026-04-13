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
	ID            string   `json:"id"`
	Claim         string   `json:"claim"`
	ClaimKey      string   `json:"claim_key"`
	ClaimValue    string   `json:"claim_value"`
	Subject       string   `json:"subject"`
	Predicate     string   `json:"predicate"`
	Object        string   `json:"object"`
	Polarity      string   `json:"polarity"`
	IsRoot        bool     `json:"is_root"`
	DependsOn     []string `json:"depends_on"`
	SourceType    string   `json:"source_type"`
	Confidence    float64  `json:"confidence"`
	AssertionKind string   `json:"assertion_kind"`
}

// ToCoreFact converts an ExtractedFact into a core.Fact ready for engine
// assertion. Root facts get empty justification sets. Derived facts get a
// single-element AND-set per dependency entry ([][]string{{dep_id}}).
func (ef *ExtractedFact) ToCoreFact() *core.Fact {
	f := &core.Fact{
		ID:            ef.ID,
		IsRoot:        ef.IsRoot,
		AssertionKind: strings.TrimSpace(ef.AssertionKind),
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
	if f.AssertionKind == "" {
		f.AssertionKind = core.AssertionKindEmpirical
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

const vLogicSystemPrompt = `You are a Neuro-Symbolic V-Logic Compiler. Your job is to extract atomic assertions from the text and compile them into a strict V-Logic DSL script.

CRITICAL RULES:
1. Output ONLY valid V-Logic code. No markdown fences, no explanations.
2. V-Logic has exactly two statement types: 'fact' and 'derive'.
3. 'fact' is for root premises directly stated in the text.
   Syntax: fact <id>: "<claim>" (confidence: <float>, assertion_kind: <kind>)
   Example: fact invoice_1042_paid: "Invoice 1042 is paid" (confidence: 0.9, assertion_kind: empirical)
4. 'derive' is for inferences that depend on other facts.
   Syntax: derive <id>: "<claim>" (assertion_kind: <kind>) requires (<comma_separated_ids>) rejects (<comma_separated_ids>)
   Example: derive payment_released: "Release payment" (assertion_kind: empirical) requires (invoice_1042_paid) rejects (vendor_blocked)
   (Note: requires or rejects can be omitted if empty. Example: derive p1: "..." requires (f1))
5. IDs must be unique slug-format strings (lowercase, underscores or hyphens).
6. Decompose compound claims into separate atomic facts.
7. NEVER output circular dependencies (A requires B, B requires A).
8. Every statement MUST include assertion_kind with one of:
   - empirical: direct factual claim about the real world asserted as true
   - uncertain: hedged factual claim (e.g. "I think", "possibly", "might be", "I believe", "approximately")
   - hypothetical: conditional/speculative claim (e.g. "if", "suppose", "assume", "were to", "could")
   - fictional: claim in a clearly fictional/creative/story context
`

// Extract sends llmOutput to an OpenAI-compatible endpoint and parses the
// V-Logic response into a slice of ExtractedFacts. It employs a Neuro-Symbolic
// feedback loop, retrying up to 3 times if the Go compiler detects syntax or
// topological errors in the LLM's generated V-Logic.
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

	var lastCompilerError string
	var finalFacts []ExtractedFact

	// Neuro-Symbolic Loop (up to 3 retries)
	for attempt := 1; attempt <= 3; attempt++ {
		messages := []map[string]string{
			{"role": "system", "content": vLogicSystemPrompt},
			{"role": "user", "content": userContent},
		}

		if lastCompilerError != "" {
			messages = append(messages, map[string]string{
				"role":    "user",
				"content": fmt.Sprintf("COMPILER ERROR from your previous attempt:\n%s\nPlease rewrite the V-Logic script to fix this error.", lastCompilerError),
			})
		}

		body := map[string]interface{}{
			"model":       model,
			"temperature": 0,
			"messages":    messages,
		}
		payload, err := json.Marshal(body)
		if err != nil {
			return nil, &ExtractionError{Cause: "serialization", Detail: err.Error()}
		}

		timeoutCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
		req, err := http.NewRequestWithContext(timeoutCtx, http.MethodPost, baseURL+"/chat/completions", bytes.NewReader(payload))
		if err != nil {
			cancel()
			return nil, &ExtractionError{Cause: "request_build", Detail: err.Error()}
		}
		req.Header.Set("Authorization", "Bearer "+apiKey)
		req.Header.Set("Content-Type", "application/json")

		client := &http.Client{}
		resp, err := client.Do(req)
		if err != nil {
			if timeoutCtx.Err() != nil {
				cancel()
				return nil, &ExtractionError{Cause: "timeout", Detail: "extraction HTTP call timed out after 15s"}
			}
			cancel()
			return nil, &ExtractionError{Cause: "http", Detail: err.Error()}
		}

		if resp.StatusCode >= 300 {
			resp.Body.Close()
			cancel()
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
			resp.Body.Close()
			cancel()
			return nil, &ExtractionError{Cause: "parse_error", Detail: "failed to decode chat completion response: " + err.Error()}
		}
		resp.Body.Close()
		cancel()

		if len(chatResp.Choices) == 0 {
			return nil, &ExtractionError{Cause: "parse_error", Detail: "extraction model returned no choices"}
		}

		content := strings.TrimSpace(chatResp.Choices[0].Message.Content)
		content = stripMarkdownFences(content)

		facts, err := ParseVLogic(content)
		if err != nil {
			lastCompilerError = err.Error()
			continue
		}

		// Topological sort & cycle detection (Dry Run Compilation Phase)
		visited := map[string]bool{}
		var visit func(ef ExtractedFact)

		factMap := map[string]ExtractedFact{}
		for _, ef := range facts {
			factMap[ef.ID] = ef
		}

		var cycleErr string
		var inStack = map[string]bool{}
		var sorted []ExtractedFact

		visit = func(ef ExtractedFact) {
			if cycleErr != "" {
				return
			}
			if inStack[ef.ID] {
				cycleErr = "Circular dependency detected involving " + ef.ID
				return
			}
			if visited[ef.ID] {
				return
			}
			inStack[ef.ID] = true
			for _, dep := range ef.DependsOn {
				cleanDep := strings.TrimPrefix(dep, "!")
				if depFact, exists := factMap[cleanDep]; exists {
					visit(depFact)
				}
			}
			inStack[ef.ID] = false
			visited[ef.ID] = true
			sorted = append(sorted, ef)
		}

		for _, ef := range facts {
			visit(ef)
		}

		if cycleErr != "" {
			lastCompilerError = cycleErr
			continue
		}

		// Sanitize fallbacks
		for i := range sorted {
			if sorted[i].ID == "" {
				sorted[i].ID = fmt.Sprintf("fact-%d", i)
			}
			if sorted[i].SourceType == "" {
				sorted[i].SourceType = "v-logic"
			}
			if sorted[i].Polarity == "" {
				sorted[i].Polarity = "positive"
			}
			if sorted[i].Confidence <= 0 || sorted[i].Confidence > 1 {
				sorted[i].Confidence = 0.75
			}
		}

		finalFacts = sorted
		break
	}

	if finalFacts == nil {
		return nil, &ExtractionError{Cause: "compiler_error", Detail: "failed to compile V-Logic after 3 attempts. Last error: " + lastCompilerError}
	}

	return finalFacts, nil
}

// stripMarkdownFences removes wrappers in case the model ignores the no-markdown-fences instruction.
func stripMarkdownFences(s string) string {
	s = strings.TrimSpace(s)
	for _, prefix := range []string{"```vlogic", "```text", "```"} {
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
