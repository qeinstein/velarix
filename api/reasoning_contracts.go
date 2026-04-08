package api

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"velarix/core"
	"velarix/store"

	"github.com/xeipuuv/gojsonschema"
)

type reasoningEnvelope struct {
	Kind              string                     `json:"kind"`
	ReasoningChain    *core.ReasoningChain       `json:"reasoning_chain,omitempty"`
	ReasoningAudit    *core.ReasoningAuditReport `json:"reasoning_audit,omitempty"`
	ConsistencyReport *core.ConsistencyReport    `json:"consistency_report,omitempty"`
}

type reasoningChainRecord struct {
	Chain       *core.ReasoningChain `json:"chain"`
	Timestamp   int64                `json:"timestamp"`
	ContentHash string               `json:"content_hash"`
	Tampered    bool                 `json:"tampered"`
}

type perceptionRequest struct {
	ID         string                 `json:"id"`
	Payload    map[string]interface{} `json:"payload"`
	Confidence float64                `json:"confidence"`
	Modality   string                 `json:"modality,omitempty"`
	Provider   string                 `json:"provider,omitempty"`
	Model      string                 `json:"model,omitempty"`
	Embedding  []float64              `json:"embedding,omitempty"`
	Metadata   map[string]interface{} `json:"metadata,omitempty"`
}

type retractFactRequest struct {
	Reason string `json:"reason"`
}

type consistencyCheckRequest struct {
	FactIDs        []string `json:"fact_ids"`
	MaxFacts       int      `json:"max_facts"`
	IncludeInvalid bool     `json:"include_invalid"`
}

type verifyReasoningChainRequest struct {
	AutoRetract bool `json:"auto_retract"`
}

func saveReasoningEnvelope(s *Server, sessionID string, envelope reasoningEnvelope) error {
	content, err := json.Marshal(envelope)
	if err != nil {
		return err
	}
	_, err = s.Store.SaveExplanation(sessionID, content)
	return err
}

func decodeReasoningEnvelope(record store.ExplanationRecord) (*reasoningEnvelope, error) {
	var envelope reasoningEnvelope
	if err := json.Unmarshal(record.Content, &envelope); err != nil {
		return nil, err
	}
	return &envelope, nil
}

func findReasoningChainRecord(records []store.ExplanationRecord, chainID string) (*core.ReasoningChain, error) {
	for _, record := range records {
		envelope, err := decodeReasoningEnvelope(record)
		if err != nil || envelope == nil || envelope.Kind != "reasoning_chain" || envelope.ReasoningChain == nil {
			continue
		}
		if envelope.ReasoningChain.ChainID == chainID {
			return envelope.ReasoningChain, nil
		}
	}
	return nil, fmt.Errorf("reasoning chain not found")
}

func (s *Server) handleRecordPerception(w http.ResponseWriter, r *http.Request) {
	sessionID := r.PathValue("session_id")
	orgID := getOrgID(r)
	engine, config, err := s.getEngine(sessionID, orgID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusForbidden)
		return
	}

	var body perceptionRequest
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if strings.TrimSpace(body.ID) == "" {
		http.Error(w, "id is required", http.StatusBadRequest)
		return
	}
	if body.Confidence == 0 {
		body.Confidence = 0.75
	}

	fact := core.Fact{
		ID:           strings.TrimSpace(body.ID),
		IsRoot:       true,
		ManualStatus: core.Status(body.Confidence),
		Payload:      body.Payload,
		Metadata:     body.Metadata,
		Embedding:    body.Embedding,
	}
	if fact.Metadata == nil {
		fact.Metadata = map[string]interface{}{}
	}
	if fact.Payload != nil {
		if p, ok := fact.Payload["_provenance"]; ok {
			delete(fact.Payload, "_provenance")
			fact.Metadata["_provenance"] = p
		}
	}
	fact.Metadata["source_type"] = "perception"
	if body.Modality != "" {
		fact.Metadata["modality"] = body.Modality
	}
	if body.Provider != "" {
		fact.Metadata["provider"] = body.Provider
	}
	if body.Model != "" {
		fact.Metadata["model"] = body.Model
	}
	if config.Schema != "" {
		schemaLoader := gojsonschema.NewStringLoader(config.Schema)
		documentLoader := gojsonschema.NewGoLoader(fact.Payload)
		result, _ := gojsonschema.Validate(schemaLoader, documentLoader)
		if !result.Valid() {
			if config.EnforcementMode == "strict" {
				writeJSON(w, http.StatusBadRequest, map[string]interface{}{"error": "schema validation failed"})
				return
			}
			var errMsgs []string
			for _, desc := range result.Errors() {
				errMsgs = append(errMsgs, desc.String())
			}
			fact.ValidationErrors = errMsgs
		}
	}

	if err := engine.AssertFact(&fact); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	actorID := getActorID(r)
	entry := store.JournalEntry{Type: store.EventAssert, SessionID: sessionID, Fact: &fact, ActorID: actorID}
	if err := s.Store.Append(entry); err != nil {
		http.Error(w, "failed to persist journal", http.StatusInternalServerError)
		return
	}
	_ = s.Store.AppendOrgActivity(orgID, entry)

	s.mu.Lock()
	if s.SliceCache != nil {
		delete(s.SliceCache, sessionID)
	}
	s.mu.Unlock()

	writeJSON(w, http.StatusCreated, fact)
	s.checkSnapshotTrigger(sessionID, engine)
	s.syncFactSearchDocument(orgID, sessionID, config, &fact, engine.GetStatus(fact.ID))
}

func (s *Server) handleRetractFact(w http.ResponseWriter, r *http.Request) {
	sessionID := r.PathValue("session_id")
	orgID := getOrgID(r)
	engine, config, err := s.getEngine(sessionID, orgID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusForbidden)
		return
	}

	factID := r.PathValue("id")
	if _, ok := engine.GetFact(factID); !ok {
		http.Error(w, "fact not found", http.StatusNotFound)
		return
	}

	var body retractFactRequest
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil && err != io.EOF {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	entry := store.JournalEntry{
		Type:      store.EventRetract,
		SessionID: sessionID,
		FactID:    factID,
		ActorID:   getActorID(r),
		Payload:   map[string]interface{}{"reason": body.Reason},
	}
	if err := s.Store.Append(entry); err != nil {
		http.Error(w, "failed to persist journal", http.StatusInternalServerError)
		return
	}
	_ = s.Store.AppendOrgActivity(orgID, entry)

	if err := engine.RetractFact(factID, body.Reason); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	s.mu.Lock()
	if s.SliceCache != nil {
		delete(s.SliceCache, sessionID)
	}
	s.mu.Unlock()

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"status":  "retracted",
		"fact_id": factID,
		"reason":  body.Reason,
	})
	s.checkSnapshotTrigger(sessionID, engine)
	s.syncSessionSearchDocuments(orgID, sessionID, engine, config)
}

func (s *Server) handleSemanticSearch(w http.ResponseWriter, r *http.Request) {
	sessionID := r.PathValue("session_id")
	orgID := getOrgID(r)
	engine, _, err := s.getEngine(sessionID, orgID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusForbidden)
		return
	}

	query := strings.TrimSpace(r.URL.Query().Get("q"))
	limit := 10
	if raw := strings.TrimSpace(r.URL.Query().Get("limit")); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil && parsed > 0 && parsed <= 100 {
			limit = parsed
		}
	}
	validOnly := strings.TrimSpace(r.URL.Query().Get("valid_only")) != "false"
	if semanticStore, ok := s.Store.(store.SemanticStore); ok {
		matches, err := semanticStore.SemanticSearchFacts(orgID, sessionID, core.LexicalEmbedding(query, 128), limit, validOnly)
		if err == nil {
			writeJSON(w, http.StatusOK, matches)
			return
		}
	}
	writeJSON(w, http.StatusOK, engine.SearchSimilarFacts(query, limit, validOnly))
}

func (s *Server) handleConsistencyCheck(w http.ResponseWriter, r *http.Request) {
	sessionID := r.PathValue("session_id")
	orgID := getOrgID(r)
	engine, _, err := s.getEngine(sessionID, orgID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusForbidden)
		return
	}

	var body consistencyCheckRequest
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil && err != io.EOF {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	factIDs := uniqueStrings(body.FactIDs)
	if len(factIDs) == 0 {
		facts := engine.ListFacts()
		sort.Slice(facts, func(i, j int) bool { return facts[i].ID < facts[j].ID })
		for _, fact := range facts {
			factIDs = append(factIDs, fact.ID)
		}
	}
	if body.MaxFacts > 0 && len(factIDs) > body.MaxFacts {
		factIDs = factIDs[:body.MaxFacts]
	}

	report := engine.CheckConsistency(factIDs, body.IncludeInvalid)
	appendVerifierIssues(engine, report)
	_ = saveReasoningEnvelope(s, sessionID, reasoningEnvelope{
		Kind:              "consistency_report",
		ConsistencyReport: report,
	})
	writeJSON(w, http.StatusOK, report)
}

func (s *Server) handleRecordReasoningChain(w http.ResponseWriter, r *http.Request) {
	sessionID := r.PathValue("session_id")
	orgID := getOrgID(r)
	if _, _, err := s.getEngine(sessionID, orgID); err != nil {
		http.Error(w, err.Error(), http.StatusForbidden)
		return
	}

	var chain core.ReasoningChain
	if err := json.NewDecoder(r.Body).Decode(&chain); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if strings.TrimSpace(chain.ChainID) == "" {
		chain.ChainID = fmt.Sprintf("rc_%d", time.Now().UnixNano())
	}
	if chain.CreatedAt == 0 {
		chain.CreatedAt = time.Now().UnixMilli()
	}
	for i := range chain.Steps {
		if strings.TrimSpace(chain.Steps[i].ID) == "" {
			chain.Steps[i].ID = fmt.Sprintf("%s_step_%d", chain.ChainID, i+1)
		}
	}

	if err := saveReasoningEnvelope(s, sessionID, reasoningEnvelope{
		Kind:           "reasoning_chain",
		ReasoningChain: &chain,
	}); err != nil {
		http.Error(w, "failed to persist reasoning chain", http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusCreated, chain)
}

func (s *Server) handleListReasoningChains(w http.ResponseWriter, r *http.Request) {
	sessionID := r.PathValue("session_id")
	orgID := getOrgID(r)
	storedOrg, err := s.Store.GetSessionOrganization(sessionID)
	if err != nil || storedOrg != orgID {
		http.Error(w, "unauthorized", http.StatusForbidden)
		return
	}

	records, err := s.Store.GetSessionExplanations(sessionID)
	if err != nil {
		http.Error(w, "failed to retrieve reasoning chains", http.StatusInternalServerError)
		return
	}

	items := []reasoningChainRecord{}
	for _, record := range records {
		envelope, err := decodeReasoningEnvelope(record)
		if err != nil || envelope == nil || envelope.Kind != "reasoning_chain" || envelope.ReasoningChain == nil {
			continue
		}
		items = append(items, reasoningChainRecord{
			Chain:       envelope.ReasoningChain,
			Timestamp:   record.Timestamp,
			ContentHash: record.ContentHash,
			Tampered:    record.Tampered,
		})
	}
	sort.Slice(items, func(i, j int) bool { return items[i].Timestamp > items[j].Timestamp })
	writeJSON(w, http.StatusOK, map[string]interface{}{"items": items})
}

func (s *Server) handleVerifyReasoningChain(w http.ResponseWriter, r *http.Request) {
	sessionID := r.PathValue("session_id")
	orgID := getOrgID(r)
	engine, config, err := s.getEngine(sessionID, orgID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusForbidden)
		return
	}

	records, err := s.Store.GetSessionExplanations(sessionID)
	if err != nil {
		http.Error(w, "failed to retrieve reasoning chains", http.StatusInternalServerError)
		return
	}
	chain, err := findReasoningChainRecord(records, r.PathValue("chain_id"))
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	var body verifyReasoningChainRequest
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil && err != io.EOF {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	report := engine.AuditReasoningChain(chain)
	chainFactIDs := []string{}
	for _, step := range chain.Steps {
		if strings.TrimSpace(step.OutputFactID) != "" {
			chainFactIDs = append(chainFactIDs, step.OutputFactID)
		}
	}
	if len(chainFactIDs) > 1 {
		consistencyReport := engine.CheckConsistency(chainFactIDs, false)
		appendVerifierIssues(engine, consistencyReport)
		if consistencyReport.IssueCount > 0 {
			report.Valid = false
			report.Issues = append(report.Issues, consistencyReport.Issues...)
			candidates := map[string]struct{}{}
			for _, existing := range report.RetractCandidateFactIDs {
				candidates[existing] = struct{}{}
			}
			for _, issue := range consistencyReport.Issues {
				for _, factID := range issue.FactIDs {
					candidates[factID] = struct{}{}
				}
			}
			report.RetractCandidateFactIDs = report.RetractCandidateFactIDs[:0]
			for factID := range candidates {
				report.RetractCandidateFactIDs = append(report.RetractCandidateFactIDs, factID)
			}
			sort.Strings(report.RetractCandidateFactIDs)
		}
	}
	if body.AutoRetract {
		for _, factID := range report.RetractCandidateFactIDs {
			if _, ok := engine.GetFact(factID); !ok {
				continue
			}
			entry := store.JournalEntry{
				Type:      store.EventRetract,
				SessionID: sessionID,
				FactID:    factID,
				ActorID:   getActorID(r),
				Payload:   map[string]interface{}{"reason": "reasoning_chain_verification"},
			}
			if err := s.Store.Append(entry); err != nil {
				continue
			}
			_ = s.Store.AppendOrgActivity(orgID, entry)
			if err := engine.RetractFact(factID, "reasoning_chain_verification"); err == nil {
				report.AutoRetractedFactIDs = append(report.AutoRetractedFactIDs, factID)
			}
		}
		sort.Strings(report.AutoRetractedFactIDs)
		s.mu.Lock()
		if s.SliceCache != nil {
			delete(s.SliceCache, sessionID)
		}
		s.mu.Unlock()
		s.syncSessionSearchDocuments(orgID, sessionID, engine, config)
	}

	_ = saveReasoningEnvelope(s, sessionID, reasoningEnvelope{
		Kind:           "reasoning_audit",
		ReasoningAudit: report,
	})
	writeJSON(w, http.StatusOK, report)
}
