package api

import (
	"encoding/json"
	"fmt"
	"math"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode"

	"velarix/core"
)

type sliceSelectionOptions struct {
	Format              string
	MaxFacts            int
	MaxChars            int
	Strategy            string
	Query               string
	IncludeDependencies bool
	IncludeInvalid      bool
}

type rankedSliceFact struct {
	Fact  *core.Fact
	Score float64
}

func parseSliceSelectionOptions(values url.Values) sliceSelectionOptions {
	opts := sliceSelectionOptions{
		Format:              strings.TrimSpace(values.Get("format")),
		MaxFacts:            50,
		MaxChars:            0,
		Strategy:            strings.ToLower(strings.TrimSpace(values.Get("strategy"))),
		Query:               strings.TrimSpace(values.Get("query")),
		IncludeDependencies: values.Get("include_dependencies") == "true",
		IncludeInvalid:      values.Get("include_invalid") == "true",
	}
	if opts.Strategy == "" {
		if opts.Query != "" {
			opts.Strategy = "hybrid"
		} else {
			opts.Strategy = "ranked"
		}
	}
	if raw := strings.TrimSpace(values.Get("max_facts")); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil && parsed > 0 {
			if parsed > 500 {
				parsed = 500
			}
			opts.MaxFacts = parsed
		}
	}
	if raw := strings.TrimSpace(values.Get("max_chars")); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil && parsed > 0 {
			opts.MaxChars = parsed
		}
	}
	if opts.Query != "" && values.Get("include_dependencies") == "" {
		opts.IncludeDependencies = true
	}
	return opts
}

func (o sliceSelectionOptions) cacheKey(sessionID string) string {
	return fmt.Sprintf(
		"%s|%s|%d|%d|%s|%s|%t|%t",
		sessionID,
		o.Format,
		o.MaxFacts,
		o.MaxChars,
		o.Strategy,
		strings.ToLower(o.Query),
		o.IncludeDependencies,
		o.IncludeInvalid,
	)
}

func selectBeliefSlice(engine *core.Engine, opts sliceSelectionOptions) []*core.Fact {
	decayHours := envFloat64("VELARIX_SLICE_FRESHNESS_DECAY_HOURS", 24)
	if decayHours <= 0 {
		decayHours = 24
	}
	weight := envFloat64("VELARIX_SLICE_FRESHNESS_WEIGHT", 0.2)
	if weight < 0 {
		weight = 0
	}
	if weight > 1 {
		weight = 1
	}
	nowMs := time.Now().UnixMilli()

	facts := engine.ListFacts()
	semanticScores := map[string]float64{}
	if opts.Query != "" && (opts.Strategy == "semantic" || opts.Strategy == "hybrid" || opts.Strategy == "query_aware") {
		semanticLimit := opts.MaxFacts * 4
		if semanticLimit < 20 {
			semanticLimit = 20
		}
		for _, match := range engine.SearchSimilarFacts(opts.Query, semanticLimit, !opts.IncludeInvalid) {
			semanticScores[match.FactID] = match.Score
		}
	}

	ranked := make([]rankedSliceFact, 0, len(facts))
	for _, fact := range facts {
		status := engine.GetStatus(fact.ID)
		if !opts.IncludeInvalid && status < core.ConfidenceThreshold {
			continue
		}
		fact.ResolvedStatus = status
		score := float64(status) * 3
		score += fact.EffectiveEntrenchment()
		if fact.RequiresHumanReview() {
			score += 0.2
		}
		if deps, err := engine.DependencyIDs(fact.ID, false); err == nil {
			score += float64(len(deps)) * 0.05
		}
		if opts.Query != "" {
			score += semanticScores[fact.ID] * 5
			score += lexicalFactScore(opts.Query, fact) * 2
		}

		// Freshness scoring: exponential decay with configurable half-life-ish
		// control. With the default (24h), a fact asserted within ~1 hour scores
		// near 1.0 and a fact ~1 week old scores near 0.1.
		freshness := 1.0
		if fact.AssertedAt > 0 && nowMs > fact.AssertedAt {
			ageHours := float64(nowMs-fact.AssertedAt) / (1000 * 60 * 60)
			freshness = math.Exp(-math.Ln10 * ageHours / (7 * decayHours))
			if freshness < 0 {
				freshness = 0
			}
			if freshness > 1 {
				freshness = 1
			}
		}
		// Add a bounded freshness component so it contributes meaningfully but
		// doesn't overwhelm validity/entrenchment/query relevance.
		score += weight * freshness * 3

		ranked = append(ranked, rankedSliceFact{Fact: fact, Score: score})
	}

	sort.Slice(ranked, func(i, j int) bool {
		if ranked[i].Score == ranked[j].Score {
			return ranked[i].Fact.ID < ranked[j].Fact.ID
		}
		return ranked[i].Score > ranked[j].Score
	})

	selected := make([]*core.Fact, 0, minInt(len(ranked), opts.MaxFacts))
	selectedIDs := map[string]struct{}{}
	remainingChars := opts.MaxChars
	addFact := func(f *core.Fact) bool {
		if f == nil {
			return true
		}
		if _, exists := selectedIDs[f.ID]; exists {
			return true
		}
		if len(selected) >= opts.MaxFacts {
			return false
		}
		estimated := estimateSliceFactSize(f)
		if remainingChars > 0 && len(selected) > 0 && estimated > remainingChars {
			return false
		}
		selected = append(selected, f)
		selectedIDs[f.ID] = struct{}{}
		if remainingChars > 0 {
			remainingChars -= estimated
		}
		return true
	}

	for _, candidate := range ranked {
		if !addFact(candidate.Fact) {
			break
		}
		if !opts.IncludeDependencies {
			continue
		}
		deps, err := engine.DependencyIDs(candidate.Fact.ID, false)
		if err != nil {
			continue
		}
		sort.Strings(deps)
		for _, depID := range deps {
			dep, ok := engine.GetFact(depID)
			if !ok {
				continue
			}
			dep.ResolvedStatus = engine.GetStatus(depID)
			if !opts.IncludeInvalid && dep.ResolvedStatus < core.ConfidenceThreshold {
				continue
			}
			if !addFact(dep) {
				break
			}
		}
		if len(selected) >= opts.MaxFacts {
			break
		}
	}

	return selected
}

func renderBeliefSliceMarkdown(facts []*core.Fact) string {
	if len(facts) == 0 {
		return "## Belief Slice\n_No matching beliefs._\n"
	}
	var b strings.Builder
	b.WriteString("## Belief Slice\n")
	for _, fact := range facts {
		payloadJSON, _ := json.MarshalIndent(fact.Payload, "", "  ")
		reviewStatus := strings.TrimSpace(fact.ReviewStatus)
		if reviewStatus == "" {
			reviewStatus = "n/a"
		}
		fmt.Fprintf(
			&b,
			"\n### %s\n- status: %.2f\n- root: %t\n- entrenchment: %.2f\n- review: %s\n",
			fact.ID,
			fact.ResolvedStatus,
			fact.IsRoot,
			fact.EffectiveEntrenchment(),
			reviewStatus,
		)
		if len(fact.JustificationSets) > 0 {
			justificationJSON, _ := json.Marshal(fact.JustificationSets)
			fmt.Fprintf(&b, "- justifications: `%s`\n", string(justificationJSON))
		}
		b.WriteString("```json\n")
		b.Write(payloadJSON)
		b.WriteString("\n```\n")
	}
	return b.String()
}

func lexicalFactScore(query string, fact *core.Fact) float64 {
	queryTokens := tokenizeSliceText(query)
	if len(queryTokens) == 0 || fact == nil {
		return 0
	}
	text := strings.ToLower(fmt.Sprintf("%s %v %v", fact.ID, fact.Payload, fact.Metadata))
	matches := 0
	for _, token := range queryTokens {
		if token != "" && strings.Contains(text, token) {
			matches++
		}
	}
	return float64(matches) / float64(len(queryTokens))
}

func tokenizeSliceText(text string) []string {
	cleaned := strings.Map(func(r rune) rune {
		switch {
		case unicode.IsLetter(r), unicode.IsNumber(r):
			return unicode.ToLower(r)
		case unicode.IsSpace(r):
			return ' '
		default:
			return ' '
		}
	}, text)
	return strings.Fields(cleaned)
}

func estimateSliceFactSize(f *core.Fact) int {
	if f == nil {
		return 0
	}
	payloadJSON, _ := json.Marshal(f.Payload)
	metadataJSON, _ := json.Marshal(f.Metadata)
	return len(f.ID) + len(payloadJSON) + len(metadataJSON) + 96
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}
