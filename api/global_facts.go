package api

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"velarix/core"
)

func (s *Server) requireAdmin(w http.ResponseWriter, r *http.Request) bool {
	if getUserRole(r) != "admin" {
		http.Error(w, "forbidden: admin role required", http.StatusForbidden)
		return false
	}
	return true
}

func (s *Server) handleGlobalAssertFact(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdmin(w, r) {
		return
	}
	if s.GlobalTruth == nil {
		http.Error(w, "global truth is not enabled", http.StatusServiceUnavailable)
		return
	}

	var fact core.Fact
	if err := json.NewDecoder(r.Body).Decode(&fact); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	fact.ID = strings.TrimSpace(fact.ID)
	if fact.ID == "" {
		http.Error(w, "id is required", http.StatusBadRequest)
		return
	}
	moveProvenanceFromPayloadToMetadata(&fact)

	applyFactGovernance(&fact, s.loadPolicyControls(getOrgID(r)))

	version, err := s.GlobalTruth.AssertGlobal(&fact)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	_ = s.GlobalTruth.WaitForVersion(version, time.Now().Add(2*time.Second))
	writeJSON(w, http.StatusCreated, map[string]interface{}{
		"version": version,
		"fact":    fact,
	})
}

func (s *Server) handleGlobalRetractFact(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdmin(w, r) {
		return
	}
	if s.GlobalTruth == nil {
		http.Error(w, "global truth is not enabled", http.StatusServiceUnavailable)
		return
	}

	factID := strings.TrimSpace(r.PathValue("fact_id"))
	if factID == "" {
		http.Error(w, "fact_id is required", http.StatusBadRequest)
		return
	}
	version, err := s.GlobalTruth.RetractGlobal(factID, "global_retract")
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"status":  "retracted",
		"fact_id": factID,
		"version": version,
	})
}

func (s *Server) handleGlobalListFacts(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdmin(w, r) {
		return
	}
	if s.GlobalTruth == nil {
		http.Error(w, "global truth is not enabled", http.StatusServiceUnavailable)
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"items": s.GlobalTruth.ListGlobalFacts(),
	})
}

func (s *Server) handleGlobalGetFact(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdmin(w, r) {
		return
	}
	if s.GlobalTruth == nil {
		http.Error(w, "global truth is not enabled", http.StatusServiceUnavailable)
		return
	}
	factID := strings.TrimSpace(r.PathValue("fact_id"))
	if factID == "" {
		http.Error(w, "fact_id is required", http.StatusBadRequest)
		return
	}
	f, ok := s.GlobalTruth.GetGlobalFact(factID)
	if !ok {
		http.Error(w, "fact not found", http.StatusNotFound)
		return
	}
	writeJSON(w, http.StatusOK, f)
}
