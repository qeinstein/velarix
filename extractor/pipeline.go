package extractor

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"regexp"
	"sort"
	"strings"
	"sync"
	"unicode"

	"velarix/core"
)

// ExtractionTier selects which extraction pipeline to use.
type ExtractionTier int

const (
	// TierSRL uses the classical NLP pipeline (spaCy + AllenNLP SRL). Fast,
	// deterministic, zero marginal cost. This is the default tier.
	TierSRL ExtractionTier = 1
	// TierHybrid runs the SRL pipeline first and falls back to a single LLM
	// decomposition call for sentences where SRL produces no facts or facts
	// with final_confidence < 0.5.
	TierHybrid ExtractionTier = 2
	// TierFullLLM uses the existing five-stage LLM pipeline.
	TierFullLLM ExtractionTier = 3
)

// ExtractionConfig configures the extraction pipeline.
//
// The Tier field selects the extraction strategy:
//   - TierSRL (1): classical NLP pipeline — default
//   - TierHybrid (2): SRL with LLM fallback for low-confidence sentences
//   - TierFullLLM (3): existing five-stage LLM pipeline
//
// When Tier is TierFullLLM (or when all optional stages are disabled), the
// five-stage pipeline runs as before:
//   1) Sentence selection (verifiable|hedged|discard)
//   2) Coreference resolution + decontextualisation
//   3) Atomic decomposition + TMS-constrained dependency inference
//   4) Coverage verification (missed-claim recovery)
//   5) Consistency pre-check
//
// Defaults:
//   - Tier: TierSRL
//   - Stage 1/2/4/5 enabled (for LLM tiers)
//   - DependencyConfidenceThreshold: 0.65
//   - DecontextualisationConfidenceThreshold: 0.7
//   - CoverageConfidenceThreshold: 0.7
//   - MaxDependencyCheckConcurrency: 10
//   - SelectionModel / ExtractionModel: VELARIX_EXTRACTOR_MODEL (fallback gpt-4o-mini)
type ExtractionConfig struct {
	// Tier selects the extraction pipeline. Default is TierSRL.
	Tier ExtractionTier

	// SRLServiceURL is the base URL of the Python SRL extraction service.
	// Only used when Tier is TierSRL or TierHybrid.
	// Defaults to VELARIX_SRL_SERVICE_URL env or http://localhost:8090.
	SRLServiceURL string

	EnableSelection            bool
	EnableDecontextualisation  bool
	EnableCoverageVerification bool
	EnableConsistencyPrecheck  bool

	DependencyConfidenceThreshold           float64
	DecontextualisationConfidenceThreshold  float64
	CoverageConfidenceThreshold             float64
	MaxDependencyCheckConcurrency           int
	SelectionModel                          string
	ExtractionModel                         string
}

func DefaultExtractionConfig() ExtractionConfig {
	model := strings.TrimSpace(os.Getenv("VELARIX_EXTRACTOR_MODEL"))
	if model == "" {
		model = "gpt-4o-mini"
	}
	tierEnv := strings.TrimSpace(os.Getenv("VELARIX_EXTRACTION_TIER"))
	defaultTier := TierSRL
	switch tierEnv {
	case "2":
		defaultTier = TierHybrid
	case "3":
		defaultTier = TierFullLLM
	}

	return ExtractionConfig{
		Tier:                       defaultTier,
		EnableSelection:            true,
		EnableDecontextualisation:  true,
		EnableCoverageVerification: true,
		EnableConsistencyPrecheck:  true,

		DependencyConfidenceThreshold:          0.65,
		DecontextualisationConfidenceThreshold: 0.7,
		CoverageConfidenceThreshold:            0.7,
		MaxDependencyCheckConcurrency:          10,
		SelectionModel:                         model,
		ExtractionModel:                        model,
	}
}

func normalizeExtractionConfig(cfg *ExtractionConfig) ExtractionConfig {
	out := DefaultExtractionConfig()
	if cfg == nil {
		return out
	}
	if cfg.Tier > 0 {
		out.Tier = cfg.Tier
	}
	if strings.TrimSpace(cfg.SRLServiceURL) != "" {
		out.SRLServiceURL = strings.TrimSpace(cfg.SRLServiceURL)
	}

	out.EnableSelection = cfg.EnableSelection
	out.EnableDecontextualisation = cfg.EnableDecontextualisation
	out.EnableCoverageVerification = cfg.EnableCoverageVerification
	out.EnableConsistencyPrecheck = cfg.EnableConsistencyPrecheck

	if cfg.DependencyConfidenceThreshold > 0 {
		out.DependencyConfidenceThreshold = cfg.DependencyConfidenceThreshold
	}
	if cfg.DecontextualisationConfidenceThreshold > 0 {
		out.DecontextualisationConfidenceThreshold = cfg.DecontextualisationConfidenceThreshold
	}
	if cfg.CoverageConfidenceThreshold > 0 {
		out.CoverageConfidenceThreshold = cfg.CoverageConfidenceThreshold
	}
	if cfg.MaxDependencyCheckConcurrency > 0 {
		out.MaxDependencyCheckConcurrency = cfg.MaxDependencyCheckConcurrency
	}
	if strings.TrimSpace(cfg.SelectionModel) != "" {
		out.SelectionModel = strings.TrimSpace(cfg.SelectionModel)
	}
	if strings.TrimSpace(cfg.ExtractionModel) != "" {
		out.ExtractionModel = strings.TrimSpace(cfg.ExtractionModel)
	}
	return out
}

type ExtractionStats struct {
	Stage1Discarded      int `json:"stage1_discarded"`
	Stage2UnresolvedRefs int `json:"stage2_unresolved_refs"`

	Stage3EdgesProposed int `json:"stage3_edges_proposed"`
	Stage3EdgesAccepted int `json:"stage3_edges_accepted"`
	Stage3EdgesRejected int `json:"stage3_edges_rejected"`

	Stage4MissedClaims int `json:"stage4_missed_claims"`
	Stage5Contradictions int `json:"stage5_contradictions"`
}

type ExtractionResult struct {
	Facts                    []ExtractedFact          `json:"facts"`
	PreAssertionContradictions []core.ConsistencyIssue `json:"pre_assertion_contradictions,omitempty"`
	Stats                    ExtractionStats          `json:"stats"`
}

// LLMClient abstracts LLM calls so stages can be unit-tested with mocks.
type LLMClient interface {
	Chat(ctx context.Context, model string, messages []ChatMessage) (string, error)
}

type ChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// RunPipeline executes the five-stage extraction pipeline.
// The "baseline" configuration (all optional stages disabled) runs the legacy
// single-pass V-Logic compiler to preserve prior behavior.
func RunPipeline(ctx context.Context, llm LLMClient, llmOutput string, sessionContext string, cfg *ExtractionConfig) (*ExtractionResult, error) {
	c := normalizeExtractionConfig(cfg)

	// Baseline configuration: all optional stages disabled => legacy single-pass.
	if !c.EnableSelection && !c.EnableDecontextualisation && !c.EnableCoverageVerification && !c.EnableConsistencyPrecheck {
		facts, err := extractLegacyVLogic(ctx, llm, llmOutput, sessionContext, c.ExtractionModel)
		if err != nil {
			return nil, err
		}
		return &ExtractionResult{Facts: facts}, nil
	}

	stats := ExtractionStats{}

	selection, err := Stage1SentenceSelection(ctx, llm, llmOutput, sessionContext, c)
	if err != nil {
		return nil, err
	}
	stats.Stage1Discarded += len(selection.Discarded)

	decontext, err := Stage2Decontextualise(ctx, llm, llmOutput, selection.Selected, c)
	if err != nil {
		return nil, err
	}
	stats.Stage2UnresolvedRefs += decontext.UnresolvedRefs

	atomicFacts, err := Stage3AAtomicDecompose(ctx, llm, decontext.Sentences, c)
	if err != nil {
		return nil, err
	}

	facts, depStats, err := Stage3BInferDependencies(ctx, llm, atomicFacts, c)
	if err != nil {
		return nil, err
	}
	stats.Stage3EdgesProposed += depStats.Proposed
	stats.Stage3EdgesAccepted += depStats.Accepted
	stats.Stage3EdgesRejected += depStats.Rejected

	if c.EnableCoverageVerification {
		covered, stage4Stats, err := Stage4CoverageVerification(ctx, llm, llmOutput, facts, c)
		if err != nil {
			return nil, err
		}
		stats.Stage4MissedClaims += stage4Stats.MissedClaims
		if stage4Stats.MissedClaims > 0 {
			// Re-run dependency inference after coverage append.
			covered2, depStats2, err := Stage3BInferDependencies(ctx, llm, covered, c)
			if err != nil {
				return nil, err
			}
			facts = covered2
			stats.Stage3EdgesProposed += depStats2.Proposed
			stats.Stage3EdgesAccepted += depStats2.Accepted
			stats.Stage3EdgesRejected += depStats2.Rejected
		} else {
			facts = covered
		}
	}

	var contradictions []core.ConsistencyIssue
	if c.EnableConsistencyPrecheck {
		issues, err := Stage5ConsistencyPrecheck(facts)
		if err != nil {
			return nil, err
		}
		contradictions = issues
		stats.Stage5Contradictions += len(issues)
	}

	return &ExtractionResult{
		Facts:                    facts,
		PreAssertionContradictions: contradictions,
		Stats:                    stats,
	}, nil
}

// ----------------------------
// Stage 1 — Sentence Selection
// ----------------------------

type Stage1SelectionItem struct {
	SentenceIndex int    `json:"sentence_index"`
	Sentence      string `json:"sentence"`
	Category      string `json:"category"`
	Reason        string `json:"reason"`
	// Optional hint so downstream facts inherit hypothetical/fictional scope.
	AssertionKind string `json:"assertion_kind,omitempty"`
}

type SelectedSentence struct {
	SentenceIndex     int
	OriginalSentence  string
	Category          string
	Reason            string
	AssertionKindHint string
}

type Stage1Result struct {
	Selected   []SelectedSentence
	Discarded  []Stage1SelectionItem
}

func Stage1SentenceSelection(ctx context.Context, llm LLMClient, llmOutput string, sessionContext string, cfg ExtractionConfig) (*Stage1Result, error) {
	sentences := splitSentences(llmOutput)
	if len(sentences) == 0 {
		return &Stage1Result{}, nil
	}

	// If selection is disabled, pass everything through as verifiable.
	if !cfg.EnableSelection {
		out := make([]SelectedSentence, 0, len(sentences))
		for i, s := range sentences {
			out = append(out, SelectedSentence{
				SentenceIndex:    i,
				OriginalSentence: strings.TrimSpace(s),
				Category:         "verifiable",
				Reason:           "selection disabled",
			})
		}
		return &Stage1Result{Selected: out}, nil
	}

	type inputSentence struct {
		SentenceIndex int    `json:"sentence_index"`
		Sentence      string `json:"sentence"`
	}
	input := make([]inputSentence, 0, len(sentences))
	for i, s := range sentences {
		s = strings.TrimSpace(s)
		if s == "" {
			continue
		}
		input = append(input, inputSentence{SentenceIndex: i, Sentence: s})
	}
	inputJSON, _ := json.Marshal(input)

	user := strings.Builder{}
	if strings.TrimSpace(sessionContext) != "" {
		user.WriteString("SESSION CONTEXT:\n")
		user.WriteString(sessionContext)
		user.WriteString("\n\n")
	}
	user.WriteString("FULL LLM OUTPUT:\n")
	user.WriteString(llmOutput)
	user.WriteString("\n\nSENTENCES (JSON):\n")
	user.Write(inputJSON)
	user.WriteString("\n\nClassify each sentence as:\n- verifiable\n- hedged\n- discard\n\nReturn ONLY a JSON array. Each element MUST have: sentence_index (int), sentence (string), category (string), reason (string).\nIf the sentence is clearly hypothetical or fictional, include assertion_kind as one of: hypothetical|fictional.\n")

	system := "STAGE 1 — Sentence Selection. Classify sentences for factual extraction."

	raw, err := llm.Chat(ctx, cfg.SelectionModel, []ChatMessage{
		{Role: "system", Content: system},
		{Role: "user", Content: user.String()},
	})
	if err != nil {
		return nil, err
	}

	var items []Stage1SelectionItem
	if err := unmarshalLLMJSON(raw, &items); err != nil {
		// Fallback: keep all sentences.
		slog.Warn("stage1 selection parse failed; falling back to keep-all", "error", err)
		out := make([]SelectedSentence, 0, len(input))
		for _, s := range input {
			out = append(out, SelectedSentence{
				SentenceIndex:    s.SentenceIndex,
				OriginalSentence: s.Sentence,
				Category:         "verifiable",
				Reason:           "fallback keep-all on parse failure",
			})
		}
		return &Stage1Result{Selected: out}, nil
	}

	byIndex := map[int]Stage1SelectionItem{}
	for _, item := range items {
		byIndex[item.SentenceIndex] = item
	}

	selected := []SelectedSentence{}
	discarded := []Stage1SelectionItem{}
	for _, s := range input {
		item, ok := byIndex[s.SentenceIndex]
		if !ok {
			// Missing entries are treated as verifiable.
			selected = append(selected, SelectedSentence{
				SentenceIndex:    s.SentenceIndex,
				OriginalSentence: s.Sentence,
				Category:         "verifiable",
				Reason:           "missing from stage1 output",
			})
			continue
		}

		cat := strings.ToLower(strings.TrimSpace(item.Category))
		kindHint := normalizeAssertionKind(item.AssertionKind)
		// Best-effort parsing from reason if assertion_kind omitted.
		if kindHint == "" {
			kindHint = assertionKindHintFromText(item.Reason + " " + item.Sentence)
		}

		switch cat {
		case "verifiable", "hedged":
			if cat == "hedged" && kindHint == "" {
				kindHint = core.AssertionKindUncertain
			}
			selected = append(selected, SelectedSentence{
				SentenceIndex:     item.SentenceIndex,
				OriginalSentence:  strings.TrimSpace(item.Sentence),
				Category:          cat,
				Reason:            strings.TrimSpace(item.Reason),
				AssertionKindHint: kindHint,
			})
		default:
			discarded = append(discarded, item)
			slog.Debug("stage1 discarded sentence", "sentence_index", item.SentenceIndex, "reason", item.Reason)
		}
	}

	return &Stage1Result{Selected: selected, Discarded: discarded}, nil
}

func normalizeAssertionKind(kind string) string {
	switch strings.ToLower(strings.TrimSpace(kind)) {
	case core.AssertionKindEmpirical, core.AssertionKindUncertain, core.AssertionKindHypothetical, core.AssertionKindFictional:
		return strings.ToLower(strings.TrimSpace(kind))
	default:
		return ""
	}
}

func assertionKindHintFromText(text string) string {
	lt := strings.ToLower(text)
	switch {
	case strings.Contains(lt, "fiction") || strings.Contains(lt, "fictional") || strings.Contains(lt, "story") || strings.Contains(lt, "narrative"):
		return core.AssertionKindFictional
	case strings.Contains(lt, "hypothetical") || strings.Contains(lt, "conditional") || strings.Contains(lt, "suppose") || strings.Contains(lt, "assume") || strings.Contains(lt, "if "):
		return core.AssertionKindHypothetical
	default:
		return ""
	}
}

// ---------------------------------------------------------
// Stage 2 — Coreference Resolution / Decontextualisation
// ---------------------------------------------------------

type Stage2Item struct {
	SentenceIndex        int      `json:"sentence_index"`
	Original             string   `json:"original"`
	Decontextualised     string   `json:"decontextualised"`
	UnresolvedReferences []string `json:"unresolved_references"`
	Confidence           float64  `json:"confidence"`
}

type DecontextualisedSentence struct {
	SentenceIndex         int
	Original              string
	Decontextualised      string
	ForceUncertain        bool
	AssertionKindHint     string
}

type Stage2Result struct {
	Sentences      []DecontextualisedSentence
	UnresolvedRefs int
}

func Stage2Decontextualise(ctx context.Context, llm LLMClient, fullOutput string, selected []SelectedSentence, cfg ExtractionConfig) (*Stage2Result, error) {
	if len(selected) == 0 {
		return &Stage2Result{}, nil
	}

	// If decontextualisation is disabled, pass through verbatim with high confidence.
	if !cfg.EnableDecontextualisation {
		out := make([]DecontextualisedSentence, 0, len(selected))
		for _, s := range selected {
			out = append(out, DecontextualisedSentence{
				SentenceIndex:     s.SentenceIndex,
				Original:          s.OriginalSentence,
				Decontextualised:  s.OriginalSentence,
				ForceUncertain:    false,
				AssertionKindHint: s.AssertionKindHint,
			})
		}
		return &Stage2Result{Sentences: out}, nil
	}

	type toRewrite struct {
		SentenceIndex int    `json:"sentence_index"`
		Sentence      string `json:"sentence"`
	}
	in := make([]toRewrite, 0, len(selected))
	for _, s := range selected {
		in = append(in, toRewrite{SentenceIndex: s.SentenceIndex, Sentence: s.OriginalSentence})
	}
	inJSON, _ := json.Marshal(in)

	user := strings.Builder{}
	user.WriteString("FULL ORIGINAL LLM OUTPUT:\n")
	user.WriteString(fullOutput)
	user.WriteString("\n\nSENTENCES TO DECONTEXTUALISE (JSON):\n")
	user.Write(inJSON)
	user.WriteString("\n\nRewrite each sentence so it is fully self-contained.\n- Resolve pronouns and implicit references using the full document context.\n- Preserve meaning exactly.\n- If a reference cannot be confidently resolved, list it in unresolved_references.\nReturn ONLY a JSON array with: sentence_index, original, decontextualised, unresolved_references (array), confidence (0.0-1.0).\n")

	system := "STAGE 2 — Coreference Resolution and Decontextualisation."

	raw, err := llm.Chat(ctx, cfg.ExtractionModel, []ChatMessage{
		{Role: "system", Content: system},
		{Role: "user", Content: user.String()},
	})
	if err != nil {
		return nil, err
	}

	var items []Stage2Item
	if err := unmarshalLLMJSON(raw, &items); err != nil {
		// Fallback: pass through and force uncertain (since we failed to resolve).
		slog.Warn("stage2 parse failed; falling back to passthrough uncertain", "error", err)
		out := make([]DecontextualisedSentence, 0, len(selected))
		for _, s := range selected {
			out = append(out, DecontextualisedSentence{
				SentenceIndex:     s.SentenceIndex,
				Original:          s.OriginalSentence,
				Decontextualised:  s.OriginalSentence,
				ForceUncertain:    true,
				AssertionKindHint: s.AssertionKindHint,
			})
		}
		return &Stage2Result{Sentences: out}, nil
	}

	byIndex := map[int]Stage2Item{}
	for _, item := range items {
		byIndex[item.SentenceIndex] = item
	}

	var unresolvedCount int
	out := make([]DecontextualisedSentence, 0, len(selected))
	for _, s := range selected {
		item, ok := byIndex[s.SentenceIndex]
		if !ok {
			out = append(out, DecontextualisedSentence{
				SentenceIndex:     s.SentenceIndex,
				Original:          s.OriginalSentence,
				Decontextualised:  s.OriginalSentence,
				ForceUncertain:    true,
				AssertionKindHint: s.AssertionKindHint,
			})
			continue
		}
		unresolvedCount += len(item.UnresolvedReferences)
		force := item.Confidence < cfg.DecontextualisationConfidenceThreshold || len(item.UnresolvedReferences) > 0
		out = append(out, DecontextualisedSentence{
			SentenceIndex:     item.SentenceIndex,
			Original:          strings.TrimSpace(item.Original),
			Decontextualised:  strings.TrimSpace(firstNonEmpty(item.Decontextualised, item.Original, s.OriginalSentence)),
			ForceUncertain:    force,
			AssertionKindHint: s.AssertionKindHint,
		})
	}

	return &Stage2Result{Sentences: out, UnresolvedRefs: unresolvedCount}, nil
}

// -------------------------------------------------------------
// Stage 3A — Atomic decomposition (no dependencies yet)
// -------------------------------------------------------------

type Atomic5Tuple struct {
	ID            string  `json:"id"`
	Subject       string  `json:"subject"`
	Predicate     string  `json:"predicate"`
	Object        string  `json:"object"`
	Claim         string  `json:"claim"`
	AssertionKind string  `json:"assertion_kind"`
	Confidence    float64 `json:"confidence"`
}

type AtomicFact struct {
	ExtractedFact
	SourceSentenceIndex int
	ForceUncertain      bool
	AssertionKindHint   string
}

func Stage3AAtomicDecompose(ctx context.Context, llm LLMClient, sentences []DecontextualisedSentence, cfg ExtractionConfig) ([]AtomicFact, error) {
	if len(sentences) == 0 {
		return nil, nil
	}

	var out []AtomicFact
	seenIDs := map[string]struct{}{}
	for _, s := range sentences {
		user := strings.Builder{}
		user.WriteString("Decompose this sentence into atomic, irreducible factual claims.\n")
		user.WriteString("Return ONLY a JSON array of 5-tuples. Each element MUST include:\n")
		user.WriteString("- id (slug)\n- subject\n- predicate\n- object\n- claim (plain English sentence)\n- assertion_kind (empirical|uncertain|hypothetical|fictional)\n- confidence (0.0-1.0)\n")
		user.WriteString("\nCRITICAL: Do NOT infer dependencies yet. Decompose only.\n\nSENTENCE:\n")
		user.WriteString(s.Decontextualised)

		system := "STAGE 3A — Atomic Decomposition (no dependencies)."

		raw, err := llm.Chat(ctx, cfg.ExtractionModel, []ChatMessage{
			{Role: "system", Content: system},
			{Role: "user", Content: user.String()},
		})
		if err != nil {
			return nil, err
		}

		var tuples []Atomic5Tuple
		if err := unmarshalLLMJSON(raw, &tuples); err != nil {
			return nil, err
		}

		for i, t := range tuples {
			id := strings.TrimSpace(t.ID)
			if id == "" {
				id = slugify(fmt.Sprintf("fact-%d-%d", s.SentenceIndex, i))
			} else {
				id = slugify(id)
			}
			// Ensure global uniqueness.
			if _, ok := seenIDs[id]; ok {
				id = uniqueSlug(id, seenIDs)
			}
			seenIDs[id] = struct{}{}

			kind := normalizeAssertionKind(t.AssertionKind)
			if kind == "" {
				kind = core.AssertionKindEmpirical
			}

			ef := ExtractedFact{
				ID:            id,
				Claim:         strings.TrimSpace(t.Claim),
				Subject:       strings.TrimSpace(t.Subject),
				Predicate:     strings.TrimSpace(t.Predicate),
				Object:        strings.TrimSpace(t.Object),
				Confidence:    clamp01(t.Confidence, 0.75),
				AssertionKind: kind,
				IsRoot:        true,
				DependsOn:     nil,
				SourceType:    "pipeline_stage3a",
				Polarity:      "positive",
			}

			out = append(out, AtomicFact{
				ExtractedFact:        ef,
				SourceSentenceIndex:  s.SentenceIndex,
				ForceUncertain:       s.ForceUncertain,
				AssertionKindHint:    s.AssertionKindHint,
			})
		}
	}

	return out, nil
}

// ------------------------------------------------------------------
// Stage 3B — TMS-constrained dependency inference (graph construction)
// ------------------------------------------------------------------

type DependencyProposal struct {
	ParentID     string
	ChildID      string
	Confidence   float64
	Justification string
}

type Stage3BStats struct {
	Proposed int
	Accepted int
	Rejected int
}

type dependencyCheckResponse struct {
	Depends      string  `json:"depends"`
	Confidence   float64 `json:"confidence"`
	Justification string `json:"justification"`
}

func Stage3BInferDependencies(ctx context.Context, llm LLMClient, factsIn interface{}, cfg ExtractionConfig) ([]ExtractedFact, Stage3BStats, error) {
	// factsIn is []AtomicFact (from 3A) or []ExtractedFact (from stage 4 append)
	var atomic []AtomicFact
	switch v := factsIn.(type) {
	case []AtomicFact:
		atomic = v
	case []ExtractedFact:
		atomic = make([]AtomicFact, 0, len(v))
		for _, f := range v {
			atomic = append(atomic, AtomicFact{ExtractedFact: f})
		}
	default:
		return nil, Stage3BStats{}, fmt.Errorf("stage3b: unsupported input type %T", factsIn)
	}

	if len(atomic) == 0 {
		return nil, Stage3BStats{}, nil
	}

	// Candidate pruning by lexical overlap.
	type pair struct {
		Parent AtomicFact
		Child  AtomicFact
	}
	pairs := make([]pair, 0, len(atomic)*2)
	for i := range atomic {
		for j := range atomic {
			if i == j {
				continue
			}
			parent := atomic[i]
			child := atomic[j]
			if !shouldCheckDependency(parent, child) {
				continue
			}
			pairs = append(pairs, pair{Parent: parent, Child: child})
		}
	}

	sem := make(chan struct{}, cfg.MaxDependencyCheckConcurrency)
	var mu sync.Mutex
	var proposals []DependencyProposal
	var stats Stage3BStats
	var firstErr error
	var wg sync.WaitGroup

	for _, p := range pairs {
		if ctx.Err() != nil {
			break
		}
		wg.Add(1)
		go func(p pair) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			user := strings.Builder{}
			user.WriteString("Answer whether the CHILD fact logically depends on the PARENT fact.\n")
			user.WriteString("Return ONLY JSON: {\"depends\":\"yes\"|\"no\",\"confidence\":<0.0-1.0>,\"justification\":\"one sentence\"}\n\n")
			user.WriteString("PARENT:\n")
			user.WriteString(p.Parent.Claim)
			user.WriteString("\n\nCHILD:\n")
			user.WriteString(p.Child.Claim)

			raw, err := llm.Chat(ctx, cfg.ExtractionModel, []ChatMessage{
				{Role: "system", Content: "STAGE 3B — Dependency Inference (pairwise check)."},
				{Role: "user", Content: user.String()},
			})
			if err != nil {
				mu.Lock()
				if firstErr == nil {
					firstErr = err
				}
				mu.Unlock()
				return
			}

			var resp dependencyCheckResponse
			if err := unmarshalLLMJSON(raw, &resp); err != nil {
				mu.Lock()
				if firstErr == nil {
					firstErr = err
				}
				mu.Unlock()
				return
			}

			if strings.EqualFold(strings.TrimSpace(resp.Depends), "yes") && resp.Confidence >= cfg.DependencyConfidenceThreshold {
				prop := DependencyProposal{
					ParentID:     p.Parent.ID,
					ChildID:      p.Child.ID,
					Confidence:   resp.Confidence,
					Justification: strings.TrimSpace(resp.Justification),
				}
				mu.Lock()
				stats.Proposed++
				proposals = append(proposals, prop)
				mu.Unlock()
			}
		}(p)
	}
	wg.Wait()
	if firstErr != nil {
		return nil, Stage3BStats{}, firstErr
	}

	// Sort proposals: highest confidence first for deterministic acceptance.
	sort.Slice(proposals, func(i, j int) bool {
		if proposals[i].Confidence == proposals[j].Confidence {
			if proposals[i].ParentID == proposals[j].ParentID {
				return proposals[i].ChildID < proposals[j].ChildID
			}
			return proposals[i].ParentID < proposals[j].ParentID
		}
		return proposals[i].Confidence > proposals[j].Confidence
	})

	// Validate edges via symbolic constraints (acyclic + assertable) using Engine.
	acceptedParents := map[string][]string{}
	for _, f := range atomic {
		acceptedParents[f.ID] = nil
	}

	for _, prop := range proposals {
		if prop.ParentID == prop.ChildID {
			stats.Rejected++
			continue
		}
		if contains(acceptedParents[prop.ChildID], prop.ParentID) {
			continue
		}
		next := cloneParentsMap(acceptedParents)
		next[prop.ChildID] = append(next[prop.ChildID], prop.ParentID)
		if err := validateDependencyGraphWithEngine(atomic, next); err != nil {
			stats.Rejected++
			continue
		}
		acceptedParents = next
		stats.Accepted++
	}

	// Build final extracted facts with depends_on/is_root.
	out := make([]ExtractedFact, 0, len(atomic))
	for _, f := range atomic {
		ef := f.ExtractedFact
		parents := uniqueStrings(acceptedParents[ef.ID])
		ef.DependsOn = parents
		ef.IsRoot = len(parents) == 0
		ef.SourceType = firstNonEmpty(ef.SourceType, "pipeline_stage3")

		// Propagate assertion_kind from stage hints.
		ak := normalizeAssertionKind(ef.AssertionKind)
		if f.ForceUncertain {
			ak = core.AssertionKindUncertain
		} else if hint := normalizeAssertionKind(f.AssertionKindHint); hint != "" {
			// Only override into hypothetical/fictional/uncertain scopes.
			if hint == core.AssertionKindHypothetical || hint == core.AssertionKindFictional || hint == core.AssertionKindUncertain {
				ak = hint
			}
		}
		if ak == "" {
			ak = core.AssertionKindEmpirical
		}
		ef.AssertionKind = ak

		out = append(out, ef)
	}

	// Stable ordering: roots first, then derived; within each, by ID.
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].IsRoot != out[j].IsRoot {
			return out[i].IsRoot && !out[j].IsRoot
		}
		return out[i].ID < out[j].ID
	})

	return out, stats, nil
}

func validateDependencyGraphWithEngine(facts []AtomicFact, parents map[string][]string) error {
	// First ensure the graph is acyclic and yields a topological ordering.
	order, err := topoOrderFromParents(parents)
	if err != nil {
		return err
	}

	engine := core.NewEngine()

	byID := map[string]AtomicFact{}
	for _, f := range facts {
		byID[f.ID] = f
	}

	for _, id := range order {
		f, ok := byID[id]
		if !ok {
			return fmt.Errorf("unknown fact id in ordering: %s", id)
		}
		p := uniqueStrings(parents[id])

		cf := &core.Fact{
			ID:            id,
			IsRoot:        len(p) == 0,
			AssertionKind: normalizeAssertionKind(f.AssertionKind),
			Payload:       map[string]interface{}{"claim": f.Claim, "subject": f.Subject, "predicate": f.Predicate, "object": f.Object, "polarity": "positive"},
			Metadata:      map[string]interface{}{"source_type": "pipeline_stage3b_validator"},
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
			return err
		}
	}
	return nil
}

func topoOrderFromParents(parents map[string][]string) ([]string, error) {
	inDegree := map[string]int{}
	children := map[string][]string{}
	for id := range parents {
		inDegree[id] = 0
	}
	for child, ps := range parents {
		uniq := uniqueStrings(ps)
		inDegree[child] = len(uniq)
		for _, parent := range uniq {
			children[parent] = append(children[parent], child)
		}
	}
	queue := make([]string, 0, len(inDegree))
	for id, deg := range inDegree {
		if deg == 0 {
			queue = append(queue, id)
		}
	}
	sort.Strings(queue)

	var order []string
	for len(queue) > 0 {
		id := queue[0]
		queue = queue[1:]
		order = append(order, id)
		for _, child := range children[id] {
			inDegree[child]--
			if inDegree[child] == 0 {
				queue = append(queue, child)
			}
		}
		sort.Strings(queue)
	}
	if len(order) != len(inDegree) {
		return nil, errors.New("cycle detected in dependency graph")
	}
	return order, nil
}

func shouldCheckDependency(parent AtomicFact, child AtomicFact) bool {
	// "Only check pairs where the subject or object of B overlaps with the subject, predicate, or object of A"
	bTokens := tokenSet(child.Subject + " " + child.Object)
	if len(bTokens) == 0 {
		bTokens = tokenSet(child.Claim)
	}
	if len(bTokens) == 0 {
		return false
	}

	aTokens := tokenSet(parent.Subject + " " + parent.Predicate + " " + parent.Object)
	if len(aTokens) == 0 {
		aTokens = tokenSet(parent.Claim)
	}
	if len(aTokens) == 0 {
		return false
	}
	for tok := range bTokens {
		if _, ok := aTokens[tok]; ok {
			return true
		}
	}
	return false
}

// ---------------------------------------
// Stage 4 — Coverage Verification Pass
// ---------------------------------------

type coverageMiss struct {
	MissedClaim   string      `json:"missed_claim"`
	SuggestedFact Atomic5Tuple `json:"suggested_fact"`
	Confidence    float64     `json:"confidence"`
}

type Stage4Stats struct {
	MissedClaims int
}

func Stage4CoverageVerification(ctx context.Context, llm LLMClient, originalOutput string, facts []ExtractedFact, cfg ExtractionConfig) ([]ExtractedFact, Stage4Stats, error) {
	if !cfg.EnableCoverageVerification {
		return facts, Stage4Stats{}, nil
	}
	if strings.TrimSpace(originalOutput) == "" {
		return facts, Stage4Stats{}, nil
	}

	var factSummaries []string
	for _, f := range facts {
		if strings.TrimSpace(f.Claim) != "" {
			factSummaries = append(factSummaries, fmt.Sprintf("- %s (%s)", strings.TrimSpace(f.Claim), f.ID))
			continue
		}
		factSummaries = append(factSummaries, fmt.Sprintf("- %s/%s/%s (%s)", f.Subject, f.Predicate, f.Object, f.ID))
	}

	user := strings.Builder{}
	user.WriteString("Find any verifiable claim in the ORIGINAL text that is NOT covered by the extracted facts summary.\n")
	user.WriteString("Return ONLY a JSON array. Each element MUST have:\n")
	user.WriteString("- missed_claim (string)\n- suggested_fact (an atomic 5-tuple with fields: id, subject, predicate, object, claim, assertion_kind, confidence)\n- confidence (0.0-1.0)\n\n")
	user.WriteString("ORIGINAL:\n")
	user.WriteString(originalOutput)
	user.WriteString("\n\nEXTRACTED FACTS SUMMARY:\n")
	user.WriteString(strings.Join(factSummaries, "\n"))

	raw, err := llm.Chat(ctx, cfg.ExtractionModel, []ChatMessage{
		{Role: "system", Content: "STAGE 4 — Coverage Verification Pass."},
		{Role: "user", Content: user.String()},
	})
	if err != nil {
		return nil, Stage4Stats{}, err
	}

	var missed []coverageMiss
	if err := unmarshalLLMJSON(raw, &missed); err != nil {
		// Coverage is best-effort; parse failures should not fail extraction.
		slog.Warn("stage4 parse failed; skipping coverage append", "error", err)
		return facts, Stage4Stats{}, nil
	}

	existingIDs := map[string]struct{}{}
	for _, f := range facts {
		existingIDs[f.ID] = struct{}{}
	}

	stats := Stage4Stats{}
	for _, m := range missed {
		if m.Confidence < cfg.CoverageConfidenceThreshold {
			continue
		}
		stats.MissedClaims++
		sf := m.SuggestedFact
		id := slugify(firstNonEmpty(sf.ID, sf.Claim, m.MissedClaim))
		if _, ok := existingIDs[id]; ok {
			id = uniqueSlug(id, existingIDs)
		}
		existingIDs[id] = struct{}{}

		kind := normalizeAssertionKind(sf.AssertionKind)
		if kind == "" {
			kind = core.AssertionKindEmpirical
		}
		facts = append(facts, ExtractedFact{
			ID:            id,
			Claim:         strings.TrimSpace(sf.Claim),
			Subject:       strings.TrimSpace(sf.Subject),
			Predicate:     strings.TrimSpace(sf.Predicate),
			Object:        strings.TrimSpace(sf.Object),
			Confidence:    clamp01(sf.Confidence, 0.75),
			AssertionKind: kind,
			IsRoot:        true,
			DependsOn:     nil,
			SourceType:    "pipeline_stage4",
			Polarity:      "positive",
		})
	}

	return facts, stats, nil
}

// ---------------------------------
// Stage 5 — Consistency Pre-check
// ---------------------------------

func Stage5ConsistencyPrecheck(facts []ExtractedFact) ([]core.ConsistencyIssue, error) {
	if len(facts) == 0 {
		return nil, nil
	}

	// Assert roots-first in dependency order to satisfy Engine requirements.
	parents := map[string][]string{}
	byID := make(map[string]ExtractedFact, len(facts))
	for _, f := range facts {
		parents[f.ID] = append([]string(nil), f.DependsOn...)
		byID[f.ID] = f
	}
	order, err := topoOrderFromParents(parents)
	if err != nil {
		// If the extractor produced a cycle (should not happen), surface it.
		return nil, err
	}

	engine := core.NewEngine()
	for _, id := range order {
		ef, ok := byID[id]
		if !ok {
			return nil, fmt.Errorf("consistency precheck: missing fact id %s", id)
		}
		cf := ef.ToCoreFact()
		if err := engine.AssertFact(cf); err != nil {
			return nil, err
		}
	}

	report := engine.CheckConsistency(nil, true)
	if report == nil || report.IssueCount == 0 {
		return nil, nil
	}
	return report.Issues, nil
}

// ----------------------------
// Utilities (sentence + JSON)
// ----------------------------

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}

func clamp01(v float64, fallback float64) float64 {
	if v <= 0 || v > 1 {
		return fallback
	}
	return v
}

func unmarshalLLMJSON(raw string, out interface{}) error {
	s := strings.TrimSpace(stripMarkdownFences(raw))
	// Best-effort: if the model wrapped extra text, extract the first JSON array/object.
	s = extractFirstJSONValue(s)
	dec := json.NewDecoder(strings.NewReader(s))
	dec.DisallowUnknownFields()
	if err := dec.Decode(out); err != nil {
		// Retry without DisallowUnknownFields for forward-compat.
		dec2 := json.NewDecoder(strings.NewReader(s))
		if err2 := dec2.Decode(out); err2 != nil {
			return err
		}
		return nil
	}
	return nil
}

func extractFirstJSONValue(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return s
	}
	// Heuristic: find first '{' or '[' and last matching '}' or ']'.
	start := strings.IndexAny(s, "[{")
	if start < 0 {
		return s
	}
	trimmed := s[start:]
	// If already valid JSON, return.
	var js interface{}
	if json.Unmarshal([]byte(trimmed), &js) == nil {
		return trimmed
	}
	// Otherwise, truncate to last '}' or ']'.
	endObj := strings.LastIndex(trimmed, "}")
	endArr := strings.LastIndex(trimmed, "]")
	end := endObj
	if endArr > end {
		end = endArr
	}
	if end > 0 {
		return strings.TrimSpace(trimmed[:end+1])
	}
	return trimmed
}

var slugNonAlnum = regexp.MustCompile(`[^a-z0-9]+`)

func slugify(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	if s == "" {
		return ""
	}
	s = slugNonAlnum.ReplaceAllString(s, "-")
	s = strings.Trim(s, "-")
	if s == "" {
		return ""
	}
	return s
}

func uniqueSlug(base string, seen map[string]struct{}) string {
	if _, ok := seen[base]; !ok {
		return base
	}
	for i := 2; i < 10000; i++ {
		cand := fmt.Sprintf("%s-%d", base, i)
		if _, ok := seen[cand]; !ok {
			return cand
		}
	}
	return fmt.Sprintf("%s-%d", base, len(seen)+1)
}

func uniqueStrings(values []string) []string {
	seen := map[string]struct{}{}
	out := make([]string, 0, len(values))
	for _, v := range values {
		v = strings.TrimSpace(v)
		if v == "" {
			continue
		}
		if _, ok := seen[v]; ok {
			continue
		}
		seen[v] = struct{}{}
		out = append(out, v)
	}
	return out
}

func contains(values []string, target string) bool {
	for _, v := range values {
		if v == target {
			return true
		}
	}
	return false
}

func cloneParentsMap(m map[string][]string) map[string][]string {
	out := make(map[string][]string, len(m))
	for k, v := range m {
		out[k] = append([]string(nil), v...)
	}
	return out
}

func tokenSet(s string) map[string]struct{} {
	s = strings.ToLower(s)
	var b strings.Builder
	for _, r := range s {
		if unicode.IsLetter(r) || unicode.IsNumber(r) || r == ' ' {
			b.WriteRune(r)
			continue
		}
		b.WriteRune(' ')
	}
	parts := strings.Fields(b.String())
	out := map[string]struct{}{}
	for _, p := range parts {
		if len(p) <= 2 {
			continue
		}
		switch p {
		case "the", "and", "or", "but", "for", "with", "from", "that", "this", "was", "were", "are", "is", "in", "on", "at", "to", "of", "a", "an":
			continue
		}
		out[p] = struct{}{}
	}
	return out
}

// splitSentences is a lightweight sentence splitter tuned for LLM output.
// It preserves punctuation for downstream prompts while keeping indices stable.
func splitSentences(text string) []string {
	text = strings.TrimSpace(text)
	if text == "" {
		return nil
	}
	var sentences []string
	var buf strings.Builder
	runes := []rune(text)
	for i := 0; i < len(runes); i++ {
		r := runes[i]
		buf.WriteRune(r)

		if r == '.' || r == '!' || r == '?' {
			// Avoid splitting on common abbreviations.
			windowStart := i - 6
			if windowStart < 0 {
				windowStart = 0
			}
			window := strings.ToLower(string(runes[windowStart : i+1]))
			if strings.Contains(window, "e.g.") || strings.Contains(window, "i.e.") || strings.Contains(window, "dr.") || strings.Contains(window, "mr.") || strings.Contains(window, "ms.") {
				continue
			}
			// Split if next is whitespace or end.
			if i+1 == len(runes) || unicode.IsSpace(runes[i+1]) {
				s := strings.TrimSpace(buf.String())
				if s != "" {
					sentences = append(sentences, s)
				}
				buf.Reset()
			}
		}
		if r == '\n' {
			// Paragraph breaks behave like soft sentence boundaries.
			if strings.HasSuffix(buf.String(), "\n\n") {
				s := strings.TrimSpace(buf.String())
				if s != "" {
					sentences = append(sentences, s)
				}
				buf.Reset()
			}
		}
	}
	rest := strings.TrimSpace(buf.String())
	if rest != "" {
		sentences = append(sentences, rest)
	}
	return sentences
}
