package extractor

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"time"

	"velarix/core"
)

// ---------------------------------------------------------------------------
// SRL Service Client — calls the Python SRL microservice
// ---------------------------------------------------------------------------

// SRLServiceClient communicates with the Python SRL extraction service.
type SRLServiceClient struct {
	BaseURL    string
	HTTPClient *http.Client
}

// NewSRLServiceClient creates a client for the SRL extraction microservice.
func NewSRLServiceClient(baseURL string) *SRLServiceClient {
	if baseURL == "" {
		baseURL = strings.TrimSpace(os.Getenv("VELARIX_SRL_SERVICE_URL"))
	}
	if baseURL == "" {
		baseURL = "http://localhost:8090"
	}
	return &SRLServiceClient{
		BaseURL:    strings.TrimRight(baseURL, "/"),
		HTTPClient: &http.Client{Timeout: 30 * time.Second},
	}
}

// SRLExtractRequest is sent to the Python SRL service.
type SRLExtractRequest struct {
	Text               string `json:"text"`
	SessionContext     string `json:"session_context"`
	VelarixInternalURL string `json:"velarix_internal_url"`
}

// SRLFactModifiers are temporal/location/causal/manner modifiers from SRL.
type SRLFactModifiers struct {
	Temporal string `json:"temporal"`
	Location string `json:"location"`
	Causal   string `json:"causal"`
	Manner   string `json:"manner"`
}

// SRLFactResult is a single fact returned by the Python SRL service.
type SRLFactResult struct {
	ID                  string            `json:"id"`
	Subject             string            `json:"subject"`
	Predicate           string            `json:"predicate"`
	Object              string            `json:"object"`
	Claim               string            `json:"claim"`
	Confidence          float64           `json:"confidence"`
	AssertionKind       string            `json:"assertion_kind"`
	Modifiers           SRLFactModifiers  `json:"modifiers"`
	SourceSentenceIndex int               `json:"source_sentence_index"`
	SRLConfidence       float64           `json:"srl_confidence"`
	EntityTypes         map[string]string `json:"entity_types"`
	DependsOn           []string          `json:"depends_on"`
	IsRoot              bool              `json:"is_root"`
	Polarity            string            `json:"polarity"`
}

// SRLConflictPair records two facts that are ambiguous parse alternatives.
type SRLConflictPair struct {
	FactAID string `json:"fact_a_id"`
	FactBID string `json:"fact_b_id"`
	Reason  string `json:"reason"`
}

// SRLCoreferenceEntry captures a resolved pronoun reference.
type SRLCoreferenceEntry struct {
	PronounSpan string  `json:"pronoun_span"`
	Antecedent  string  `json:"antecedent"`
	Confidence  float64 `json:"confidence"`
	Resolved    bool    `json:"resolved"`
}

// SRLExtractionStats are pipeline-level statistics from the SRL service.
type SRLExtractionStats struct {
	SimplifiedSentences  int `json:"simplified_sentences"`
	CoreferencesResolved int `json:"coreferences_resolved"`
	EntitiesFound        int `json:"entities_found"`
	FactsExtracted       int `json:"facts_extracted"`
	EdgesProposed        int `json:"edges_proposed"`
	EdgesAccepted        int `json:"edges_accepted"`
	EdgesRejected        int `json:"edges_rejected"`
	AmbiguousPairs       int `json:"ambiguous_pairs"`
	FallbackSentences    int `json:"fallback_sentences"`
}

// SRLExtractResponse is the full response from the Python SRL service.
type SRLExtractResponse struct {
	Facts          []SRLFactResult       `json:"facts"`
	ConflictPairs  []SRLConflictPair     `json:"conflict_pairs"`
	CoreferenceMap []SRLCoreferenceEntry `json:"coreference_map"`
	Stats          SRLExtractionStats    `json:"stats"`
}

// Extract calls the Python SRL service and returns the structured response.
func (c *SRLServiceClient) Extract(ctx context.Context, text, sessionContext string) (*SRLExtractResponse, error) {
	// Determine the internal URL for TMS validation callbacks.
	internalURL := strings.TrimSpace(os.Getenv("VELARIX_INTERNAL_URL"))
	if internalURL == "" {
		port := strings.TrimSpace(os.Getenv("PORT"))
		if port == "" {
			port = "8080"
		}
		internalURL = "http://localhost:" + port
	}

	reqBody := SRLExtractRequest{
		Text:               text,
		SessionContext:     sessionContext,
		VelarixInternalURL: internalURL,
	}
	payload, err := json.Marshal(reqBody)
	if err != nil {
		return nil, &ExtractionError{Cause: "serialization", Detail: err.Error()}
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.BaseURL+"/extract", bytes.NewReader(payload))
	if err != nil {
		return nil, &ExtractionError{Cause: "request_build", Detail: err.Error()}
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.HTTPClient.Do(httpReq)
	if err != nil {
		return nil, &ExtractionError{Cause: "srl_service_unreachable", Detail: err.Error()}
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		return nil, &ExtractionError{
			Cause:  "srl_service_error",
			Detail: fmt.Sprintf("SRL service returned HTTP %d", resp.StatusCode),
		}
	}

	var result SRLExtractResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, &ExtractionError{Cause: "srl_parse_error", Detail: err.Error()}
	}
	return &result, nil
}

// ---------------------------------------------------------------------------
// RunSRLPipeline — Tier 1 entry point
// ---------------------------------------------------------------------------

// RunSRLPipeline calls the Python SRL microservice and converts the response
// into the standard ExtractionResult format used by the rest of Velarix.
func RunSRLPipeline(ctx context.Context, text string, sessionContext string, cfg *ExtractionConfig) (*ExtractionResult, error) {
	c := normalizeExtractionConfig(cfg)
	client := NewSRLServiceClient(c.SRLServiceURL)

	srlResp, err := client.Extract(ctx, text, sessionContext)
	if err != nil {
		return nil, err
	}

	facts := convertSRLFacts(srlResp.Facts)

	stats := ExtractionStats{
		Stage3EdgesProposed: srlResp.Stats.EdgesProposed,
		Stage3EdgesAccepted: srlResp.Stats.EdgesAccepted,
		Stage3EdgesRejected: srlResp.Stats.EdgesRejected,
	}

	return &ExtractionResult{
		Facts: facts,
		Stats: stats,
	}, nil
}

// convertSRLFacts converts SRL service facts into ExtractedFact structs.
func convertSRLFacts(srlFacts []SRLFactResult) []ExtractedFact {
	facts := make([]ExtractedFact, 0, len(srlFacts))
	seenIDs := map[string]struct{}{}

	for _, sf := range srlFacts {
		id := slugify(sf.ID)
		if id == "" {
			id = slugify(fmt.Sprintf("srl-fact-%d", sf.SourceSentenceIndex))
		}
		if _, ok := seenIDs[id]; ok {
			id = uniqueSlug(id, seenIDs)
		}
		seenIDs[id] = struct{}{}

		kind := normalizeAssertionKind(sf.AssertionKind)
		if kind == "" {
			kind = core.AssertionKindEmpirical
		}

		polarity := sf.Polarity
		if polarity == "" {
			polarity = "positive"
		}

		ef := ExtractedFact{
			ID:            id,
			Claim:         strings.TrimSpace(sf.Claim),
			Subject:       strings.TrimSpace(sf.Subject),
			Predicate:     strings.TrimSpace(sf.Predicate),
			Object:        strings.TrimSpace(sf.Object),
			Confidence:    clamp01(sf.Confidence, 0.7),
			AssertionKind: kind,
			IsRoot:        sf.IsRoot,
			DependsOn:     sf.DependsOn,
			SourceType:    "srl_pipeline",
			Polarity:      polarity,
		}

		facts = append(facts, ef)
	}

	return facts
}

// ---------------------------------------------------------------------------
// RunHybridPipeline — Tier 2 entry point
// ---------------------------------------------------------------------------

// RunHybridPipeline runs the SRL pipeline first, then falls back to LLM
// decomposition for sentences where SRL produces no facts or low-confidence
// facts (final_confidence < 0.5).
func RunHybridPipeline(ctx context.Context, llm LLMClient, text string, sessionContext string, cfg *ExtractionConfig) (*ExtractionResult, error) {
	c := normalizeExtractionConfig(cfg)
	client := NewSRLServiceClient(c.SRLServiceURL)

	srlResp, err := client.Extract(ctx, text, sessionContext)
	if err != nil {
		// If the SRL service is completely unavailable, fall through to full LLM.
		slog.Warn("SRL service unavailable in hybrid mode; falling back to full LLM pipeline", "error", err)
		return RunPipeline(ctx, llm, text, sessionContext, cfg)
	}

	// Identify sentences that need LLM fallback.
	sentenceCovered := map[int]bool{}
	for _, f := range srlResp.Facts {
		if f.Confidence >= 0.5 {
			sentenceCovered[f.SourceSentenceIndex] = true
		}
	}

	// Split text into sentences and find uncovered ones.
	sentences := splitSentences(text)
	var fallbackSentences []string
	for i, s := range sentences {
		if !sentenceCovered[i] {
			fallbackSentences = append(fallbackSentences, s)
		}
	}

	// Convert SRL facts
	srlFacts := convertSRLFacts(srlResp.Facts)

	stats := ExtractionStats{
		Stage3EdgesProposed: srlResp.Stats.EdgesProposed,
		Stage3EdgesAccepted: srlResp.Stats.EdgesAccepted,
		Stage3EdgesRejected: srlResp.Stats.EdgesRejected,
	}

	// If all sentences are covered, return SRL results directly.
	if len(fallbackSentences) == 0 {
		return &ExtractionResult{
			Facts: srlFacts,
			Stats: stats,
		}, nil
	}

	// Run LLM decomposition on uncovered sentences only.
	fallbackText := strings.Join(fallbackSentences, " ")
	slog.Info("Hybrid mode: falling back to LLM for uncovered sentences",
		"total_sentences", len(sentences),
		"fallback_sentences", len(fallbackSentences),
	)

	llmResult, err := RunPipeline(ctx, llm, fallbackText, sessionContext, cfg)
	if err != nil {
		// If LLM fallback fails, return SRL results alone.
		slog.Warn("Hybrid mode: LLM fallback failed; using SRL results only", "error", err)
		return &ExtractionResult{
			Facts: srlFacts,
			Stats: stats,
		}, nil
	}

	// Merge: SRL facts first, then LLM facts (deduplicated by ID).
	seenIDs := map[string]struct{}{}
	for _, f := range srlFacts {
		seenIDs[f.ID] = struct{}{}
	}

	merged := srlFacts
	for _, f := range llmResult.Facts {
		if _, ok := seenIDs[f.ID]; ok {
			f.ID = uniqueSlug(f.ID, seenIDs)
		}
		seenIDs[f.ID] = struct{}{}
		f.SourceType = "hybrid_llm_fallback"
		merged = append(merged, f)
	}

	// Merge stats
	stats.Stage1Discarded += llmResult.Stats.Stage1Discarded
	stats.Stage2UnresolvedRefs += llmResult.Stats.Stage2UnresolvedRefs
	stats.Stage3EdgesProposed += llmResult.Stats.Stage3EdgesProposed
	stats.Stage3EdgesAccepted += llmResult.Stats.Stage3EdgesAccepted
	stats.Stage3EdgesRejected += llmResult.Stats.Stage3EdgesRejected
	stats.Stage4MissedClaims += llmResult.Stats.Stage4MissedClaims
	stats.Stage5Contradictions += llmResult.Stats.Stage5Contradictions

	return &ExtractionResult{
		Facts:                    merged,
		PreAssertionContradictions: llmResult.PreAssertionContradictions,
		Stats:                    stats,
	}, nil
}

// ---------------------------------------------------------------------------
// ValidateDependency — used by the internal Go endpoint
// ---------------------------------------------------------------------------

// ValidateDependencyRequest is the payload for POST /internal/validate-dependency.
type ValidateDependencyRequest struct {
	ParentID string                 `json:"parent_id"`
	ChildID  string                 `json:"child_id"`
	Facts    []ValidateDependencyFact `json:"facts"`
}

// ValidateDependencyFact is a minimal fact representation for validation.
type ValidateDependencyFact struct {
	ID            string   `json:"id"`
	Claim         string   `json:"claim"`
	Subject       string   `json:"subject"`
	Predicate     string   `json:"predicate"`
	Object        string   `json:"object"`
	Confidence    float64  `json:"confidence"`
	AssertionKind string   `json:"assertion_kind"`
	DependsOn     []string `json:"depends_on"`
	IsRoot        bool     `json:"is_root"`
}

// ValidateDependencyResponse indicates whether the proposed edge is accepted.
type ValidateDependencyResponse struct {
	Accepted bool   `json:"accepted"`
	Reason   string `json:"reason"`
}

// ValidateDependency checks whether adding an edge (parent→child) would create
// a cycle or constraint violation in a temporary in-memory TMS engine.
func ValidateDependency(req ValidateDependencyRequest) ValidateDependencyResponse {
	if req.ParentID == req.ChildID {
		return ValidateDependencyResponse{Accepted: false, Reason: "self-loop"}
	}

	// Build a parents map from existing facts + proposed edge.
	parents := map[string][]string{}
	for _, f := range req.Facts {
		parents[f.ID] = append([]string(nil), f.DependsOn...)
	}
	// Ensure both IDs exist in the map.
	if _, ok := parents[req.ParentID]; !ok {
		parents[req.ParentID] = nil
	}
	if _, ok := parents[req.ChildID]; !ok {
		parents[req.ChildID] = nil
	}

	// Add proposed edge.
	parents[req.ChildID] = append(parents[req.ChildID], req.ParentID)

	// Check for cycles.
	_, err := topoOrderFromParents(parents)
	if err != nil {
		return ValidateDependencyResponse{Accepted: false, Reason: "cycle_detected"}
	}

	// Try asserting into a temporary engine.
	engine := core.NewEngine()
	order, _ := topoOrderFromParents(parents)

	byID := map[string]ValidateDependencyFact{}
	for _, f := range req.Facts {
		byID[f.ID] = f
	}

	for _, id := range order {
		f, ok := byID[id]
		if !ok {
			continue
		}
		p := uniqueStrings(parents[id])

		cf := &core.Fact{
			ID:            id,
			IsRoot:        len(p) == 0,
			AssertionKind: normalizeAssertionKind(f.AssertionKind),
			Payload:       map[string]interface{}{"claim": f.Claim, "subject": f.Subject, "predicate": f.Predicate, "object": f.Object},
			Metadata:      map[string]interface{}{"source_type": "srl_validation"},
		}
		if cf.AssertionKind == "" {
			cf.AssertionKind = core.AssertionKindEmpirical
		}
		if cf.IsRoot {
			cf.ManualStatus = core.Status(clamp01(f.Confidence, 0.75))
		} else {
			for _, parentID := range p {
				cf.JustificationSets = append(cf.JustificationSets, []string{parentID})
			}
		}
		if err := engine.AssertFact(cf); err != nil {
			return ValidateDependencyResponse{Accepted: false, Reason: err.Error()}
		}
	}

	return ValidateDependencyResponse{Accepted: true, Reason: "valid"}
}
