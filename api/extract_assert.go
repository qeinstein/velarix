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
	LLMOutput                string `json:"llm_output"`
	SessionContext           string `json:"session_context"`
	AutoRetractContradictions bool   `json:"auto_retract_contradictions"`
}

type extractAndAssertResponse struct {
	ExtractedCount         int                      `json:"extracted_count"`
	AssertedCount          int                      `json:"asserted_count"`
	SkippedCount           int                      `json:"skipped_count"`
	ContradictionsFound    []string                 `json:"contradictions_found"`
	ContradictionsRetracted []string                `json:"contradictions_retracted"`
	Facts                  []*core.Fact             `json:"facts"`
}

func (s *Server) handleExtractAndAssert(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	sessionID := r.PathValue("session_id")
	orgID := getOrgID(r)
	actorID := getActorID(r)

	engine, config, err := s.getEngine(sessionID, orgID)
	if err != nil {
		APIRequests.WithLabelValues("/v1/s/{session_id}/extract-and-assert", "403").Inc()
		http.Error(w, err.Error(), http.StatusForbidden)
		return
	}

	var body extractAndAssertRequest
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		APIRequests.WithLabelValues("/v1/s/{session_id}/extract-and-assert", "400").Inc()
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if strings.TrimSpace(body.LLMOutput) == "" {
		APIRequests.WithLabelValues("/v1/s/{session_id}/extract-and-assert", "400").Inc()
		http.Error(w, "llm_output is required", http.StatusBadRequest)
		return
	}

	// Record extraction latency — success and failure.
	extractStart := time.Now()
	extracted, extractErr := extractor.Extract(r.Context(), body.LLMOutput, body.SessionContext)
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
		http.Error(w, "extraction failed: "+extractErr.Error(), http.StatusBadGateway)
		return
	}

	// Sort: root facts first, derived facts after.
	// This ensures all parents exist before derived facts reference them.
	sort.SliceStable(extracted, func(i, j int) bool {
		if extracted[i].IsRoot && !extracted[j].IsRoot {
			return true
		}
		return false
	})

	resp := extractAndAssertResponse{
		ExtractedCount:          len(extracted),
		ContradictionsFound:     []string{},
		ContradictionsRetracted: []string{},
		Facts:                   []*core.Fact{},
	}

	assertedIDs := map[string]struct{}{}

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
			// Do not abort the batch — the fact is in memory. Journal failure is logged.
		}
		_ = s.Store.AppendOrgActivity(orgID, entry)

		fact.ResolvedStatus = engine.GetStatus(fact.ID)
		assertedIDs[fact.ID] = struct{}{}
		resp.AssertedCount++
		resp.Facts = append(resp.Facts, fact)
	}

	s.invalidateSliceCache(sessionID)
	go s.Store.IncrementOrgMetric(orgID, "facts_asserted", uint64(resp.AssertedCount))

	// Auto-retract contradictions if requested.
	if body.AutoRetractContradictions && resp.AssertedCount > 0 {
		assertedIDList := make([]string, 0, len(assertedIDs))
		for id := range assertedIDs {
			assertedIDList = append(assertedIDList, id)
		}
		sort.Strings(assertedIDList)

		consistencyReport := engine.CheckConsistency(assertedIDList, false)
		appendVerifierIssues(engine, consistencyReport, sessionID)

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

	// Refresh resolved statuses after potential retractions.
	for _, fact := range resp.Facts {
		fact.ResolvedStatus = engine.GetStatus(fact.ID)
	}

	s.checkSnapshotTrigger(sessionID, engine)
	s.syncSessionSearchDocuments(orgID, sessionID, engine, config)

	APIRequests.WithLabelValues("/v1/s/{session_id}/extract-and-assert", "200").Inc()
	writeJSON(w, http.StatusOK, resp)
}
