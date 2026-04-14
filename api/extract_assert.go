package api

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"sort"
	"strings"
	"time"

	"velarix/core"
	"velarix/extractor"
	"velarix/store"
)

type extractAndAssertRequest struct {
	LLMOutput                 string                      `json:"llm_output"`
	SessionContext            string                      `json:"session_context"`
	AutoRetractContradictions bool                        `json:"auto_retract_contradictions"`
	ExtractionConfig          *extractor.ExtractionConfig `json:"extraction_config,omitempty"`
}

type extractAndAssertResponse struct {
	ExtractedCount             int                     `json:"extracted_count"`
	AssertedCount              int                     `json:"asserted_count"`
	SkippedCount               int                     `json:"skipped_count"`
	PreAssertionContradictions []core.ConsistencyIssue `json:"pre_assertion_contradictions,omitempty"`
	ContradictionsFound        []string                `json:"contradictions_found"`
	ContradictionsRetracted    []string                `json:"contradictions_retracted"`
	Facts                      []*core.Fact            `json:"facts"`
}

func (s *Server) handleExtractAndAssert(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	sessionID := r.PathValue("session_id")
	orgID := getOrgID(r)
	actorID := getActorID(r)

	engine, config, err := s.getEngine(sessionID, orgID)
	if err != nil {
		APIRequests.WithLabelValues("/v1/s/{session_id}/extract-and-assert", "403").Inc()
		writeError(w, http.StatusForbidden, err.Error())
		return
	}

	body, ok := decodeExtractAndAssertRequest(w, r)
	if !ok {
		APIRequests.WithLabelValues("/v1/s/{session_id}/extract-and-assert", "400").Inc()
		return
	}
	if strings.TrimSpace(body.LLMOutput) == "" {
		APIRequests.WithLabelValues("/v1/s/{session_id}/extract-and-assert", "400").Inc()
		writeError(w, http.StatusBadRequest, "llm_output is required")
		return
	}

	// Record extraction latency — success and failure.
	extractStart := time.Now()
	extractionResult, extractErr := extractor.Extract(r.Context(), body.LLMOutput, body.SessionContext, body.ExtractionConfig)
	ExtractionLatency.Observe(float64(time.Since(extractStart).Milliseconds()))

	if extractErr != nil {
		slog.Warn("Fact extraction failed",
			"session_id", sessionID,
			"org_id", orgID,
			"actor_id", actorID,
			"error", extractErr,
			"elapsed_ms", time.Since(start).Milliseconds(),
		)
		APIRequests.WithLabelValues("/v1/s/{session_id}/extract-and-assert", "502").Inc()
		writeError(w, http.StatusBadGateway, "extraction failed: "+extractErr.Error())
		return
	}

	extracted := extractionResult.Facts
	recordExtractionMetrics(extractStart, extractionResult, body.ExtractionConfig, len(extracted))

	// Topological sort: ensure all dependencies are asserted before derived facts.
	extracted = toposortExtractedFacts(extracted)

	resp := extractAndAssertResponse{
		ExtractedCount:             len(extracted),
		PreAssertionContradictions: extractionResult.PreAssertionContradictions,
		ContradictionsFound:        []string{},
		ContradictionsRetracted:    []string{},
		Facts:                      []*core.Fact{},
	}

	assertedIDs := s.assertExtractedFacts(sessionID, orgID, actorID, engine, extracted, &resp)

	s.invalidateSliceCache(sessionID)
	go s.Store.IncrementOrgMetric(orgID, "facts_asserted", uint64(resp.AssertedCount))

	// Auto-retract contradictions if requested.
	if body.AutoRetractContradictions && resp.AssertedCount > 0 {
		s.autoRetractContradictions(sessionID, orgID, actorID, engine, assertedIDs, &resp)
	}

	// Refresh resolved statuses after potential retractions.
	for _, fact := range resp.Facts {
		fact.ResolvedStatus = engine.GetStatus(fact.ID)
	}

	s.checkSnapshotTrigger(sessionID, engine)
	s.syncSessionSearchDocuments(orgID, sessionID, engine, config)

	APIRequests.WithLabelValues("/v1/s/{session_id}/extract-and-assert", "200").Inc()
	writeJSON(w, http.StatusOK, resp)
}

func decodeExtractAndAssertRequest(w http.ResponseWriter, r *http.Request) (extractAndAssertRequest, bool) {
	r.Body = http.MaxBytesReader(w, r.Body, 2*1024*1024)
	var body extractAndAssertRequest
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return extractAndAssertRequest{}, false
	}
	return body, true
}

func recordExtractionMetrics(extractStart time.Time, extractionResult *extractor.ExtractionResult, requestCfg *extractor.ExtractionConfig, extractedCount int) {
	if extractionResult == nil {
		return
	}

	ExtractionStage1DiscardedTotal.Add(float64(extractionResult.Stats.Stage1Discarded))
	ExtractionStage2UnresolvedTotal.Add(float64(extractionResult.Stats.Stage2UnresolvedRefs))
	ExtractionStage3EdgesProposedTotal.Add(float64(extractionResult.Stats.Stage3EdgesProposed))
	ExtractionStage3EdgesAcceptedTotal.Add(float64(extractionResult.Stats.Stage3EdgesAccepted))
	ExtractionStage3EdgesRejectedTotal.Add(float64(extractionResult.Stats.Stage3EdgesRejected))
	ExtractionStage4MissedClaimsTotal.Add(float64(extractionResult.Stats.Stage4MissedClaims))
	ExtractionStage5ContradictionsTotal.Add(float64(extractionResult.Stats.Stage5Contradictions))

	cfg := requestCfg
	if cfg == nil {
		defaultCfg := extractor.DefaultExtractionConfig()
		cfg = &defaultCfg
	}
	if cfg.Tier == extractor.TierSRL || cfg.Tier == extractor.TierHybrid {
		SRLExtractionLatency.Observe(float64(time.Since(extractStart).Milliseconds()))
		SRLFactsExtractedTotal.Add(float64(extractedCount))
		SRLEdgesProposedTotal.Add(float64(extractionResult.Stats.Stage3EdgesProposed))
		SRLEdgesAcceptedTotal.Add(float64(extractionResult.Stats.Stage3EdgesAccepted))
		SRLEdgesRejectedTotal.Add(float64(extractionResult.Stats.Stage3EdgesRejected))
	}
}

func toposortExtractedFacts(extracted []extractor.ExtractedFact) []extractor.ExtractedFact {
	var sorted []extractor.ExtractedFact
	visited := map[string]bool{}
	var visit func(ef extractor.ExtractedFact)

	factMap := map[string]extractor.ExtractedFact{}
	for _, ef := range extracted {
		factMap[ef.ID] = ef
	}

	visit = func(ef extractor.ExtractedFact) {
		if visited[ef.ID] {
			return
		}
		visited[ef.ID] = true
		for _, dep := range ef.DependsOn {
			if depFact, exists := factMap[dep]; exists {
				visit(depFact)
			}
		}
		sorted = append(sorted, ef)
	}

	for _, ef := range extracted {
		visit(ef)
	}
	return sorted
}

func (s *Server) assertExtractedFacts(sessionID, orgID, actorID string, engine *core.Engine, extracted []extractor.ExtractedFact, resp *extractAndAssertResponse) map[string]struct{} {
	assertedIDs := map[string]struct{}{}
	if s == nil || engine == nil || resp == nil {
		return assertedIDs
	}

	for _, ef := range extracted {
		fact := ef.ToCoreFact()
		applyFactGovernance(fact, s.loadPolicyControls(orgID))

		if err := engine.AssertFact(fact); err != nil {
			var cycleErr *core.CycleError
			if errors.As(err, &cycleErr) {
				slog.Warn("Skipping extracted fact: cycle detected",
					"session_id", sessionID,
					"fact_id", fact.ID,
					"cycle_path", cycleErr.Path,
				)
			} else {
				slog.Warn("Skipping extracted fact: assertion failed",
					"session_id", sessionID,
					"fact_id", fact.ID,
					"error", err,
				)
			}
			resp.SkippedCount++
			continue
		}
		if s.GlobalTruth != nil {
			s.GlobalTruth.IndexFactDependencies(sessionID, fact)
		}
		s.maybeStartVerification(sessionID, orgID, engine, fact)

		entry := store.JournalEntry{
			Type:      store.EventAssert,
			SessionID: sessionID,
			Fact:      fact,
			ActorID:   actorID,
		}
		if err := s.Store.Append(entry); err != nil {
			slog.Error("Failed to persist journal for extracted fact",
				"session_id", sessionID,
				"fact_id", fact.ID,
				"error", err,
			)
		}
		_ = s.Store.AppendOrgActivity(orgID, entry)

		fact.ResolvedStatus = engine.GetStatus(fact.ID)
		assertedIDs[fact.ID] = struct{}{}
		resp.AssertedCount++
		resp.Facts = append(resp.Facts, fact)
	}
	return assertedIDs
}

func (s *Server) autoRetractContradictions(sessionID, orgID, actorID string, engine *core.Engine, assertedIDs map[string]struct{}, resp *extractAndAssertResponse) {
	assertedIDList := make([]string, 0, len(assertedIDs))
	for id := range assertedIDs {
		assertedIDList = append(assertedIDList, id)
	}
	sort.Strings(assertedIDList)

	consistencyReport := engine.CheckConsistency(assertedIDList, false)
	appendVerifierIssues(engine, consistencyReport, sessionID)
	if consistencyReport != nil && consistencyReport.IssueCount > 0 {
		s.flagFactsForReviewOnIssues(sessionID, orgID, engine, consistencyReport.Issues, "contradiction_detected")
	}

	seen := map[string]struct{}{}
	for _, issue := range consistencyReport.Issues {
		if len(issue.FactIDs) != 2 {
			continue
		}
		pairKey := issue.FactIDs[0] + "|" + issue.FactIDs[1]
		if _, already := seen[pairKey]; already {
			continue
		}
		seen[pairKey] = struct{}{}
		resp.ContradictionsFound = append(resp.ContradictionsFound, pairKey)

		factA, okA := engine.GetFact(issue.FactIDs[0])
		factB, okB := engine.GetFact(issue.FactIDs[1])
		if !okA || !okB {
			continue
		}

		var toRetract *core.Fact
		var conflictingID string
		if factA.EffectiveEntrenchment() <= factB.EffectiveEntrenchment() {
			toRetract = factA
			conflictingID = factB.ID
		} else {
			toRetract = factB
			conflictingID = factA.ID
		}

		retractEntry := store.JournalEntry{
			Type:      store.EventRetract,
			SessionID: sessionID,
			FactID:    toRetract.ID,
			ActorID:   actorID,
			Payload:   map[string]interface{}{"reason": "auto_retract_contradiction"},
		}
		if err := s.Store.Append(retractEntry); err != nil {
			slog.Error("Failed to persist auto-retract journal entry",
				"session_id", sessionID,
				"fact_id", toRetract.ID,
				"error", err,
			)
		}
		_ = s.Store.AppendOrgActivity(orgID, retractEntry)

		if err := engine.RetractFact(toRetract.ID, "auto_retract_contradiction"); err == nil {
			slog.Info("Auto-retracted fact",
				"session_id", sessionID,
				"fact_id", toRetract.ID,
				"reason", "auto_retract_contradiction",
				"conflicting_fact_id", conflictingID,
				"timestamp", time.Now().UnixMilli(),
			)
			AutoRetractionsTotal.Inc()
			resp.ContradictionsRetracted = append(resp.ContradictionsRetracted, toRetract.ID)
		}
	}
}
