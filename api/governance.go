package api

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
	"time"

	"velarix/core"
	"velarix/store"
)

type policyControlSet struct {
	PolicyIDs                  []string
	HumanReviewThreshold       float64
	ProtectedMutationThreshold float64
	DefaultEntrenchment        float64
	AutoEntrenchmentBySource   map[string]float64
	ReviewSourceTypes          map[string]struct{}
	ReviewFactIDs              map[string]struct{}
	ProtectedFactIDs           map[string]struct{}
}

type factReviewRequest struct {
	Status string `json:"status"`
	Reason string `json:"reason,omitempty"`
}

type factMutationRequest struct {
	Reason string `json:"reason,omitempty"`
	Force  bool   `json:"force,omitempty"`
}

func floatRule(rules map[string]interface{}, key string, fallback float64) float64 {
	if rules == nil {
		return fallback
	}
	raw, ok := rules[key]
	if !ok {
		return fallback
	}
	switch v := raw.(type) {
	case float64:
		return v
	case int:
		return float64(v)
	case int64:
		return float64(v)
	case json.Number:
		if parsed, err := v.Float64(); err == nil {
			return parsed
		}
	case string:
		if parsed, err := json.Number(strings.TrimSpace(v)).Float64(); err == nil {
			return parsed
		}
	}
	return fallback
}

func stringSliceRule(rules map[string]interface{}, key string) []string {
	if rules == nil {
		return nil
	}
	raw, ok := rules[key]
	if !ok {
		return nil
	}
	switch v := raw.(type) {
	case []string:
		return append([]string(nil), v...)
	case []interface{}:
		out := make([]string, 0, len(v))
		for _, item := range v {
			if text, ok := item.(string); ok {
				text = strings.TrimSpace(text)
				if text != "" {
					out = append(out, text)
				}
			}
		}
		return out
	default:
		return nil
	}
}

func floatMapRule(rules map[string]interface{}, key string) map[string]float64 {
	if rules == nil {
		return nil
	}
	raw, ok := rules[key]
	if !ok {
		return nil
	}
	result := map[string]float64{}
	switch v := raw.(type) {
	case map[string]float64:
		for k, value := range v {
			result[strings.TrimSpace(k)] = value
		}
	case map[string]interface{}:
		for k, value := range v {
			result[strings.TrimSpace(k)] = floatRule(map[string]interface{}{"value": value}, "value", 0)
		}
	}
	if len(result) == 0 {
		return nil
	}
	return result
}

func (s *Server) loadPolicyControls(orgID string) policyControlSet {
	controls := policyControlSet{
		AutoEntrenchmentBySource: map[string]float64{},
		ReviewSourceTypes:        map[string]struct{}{},
		ReviewFactIDs:            map[string]struct{}{},
		ProtectedFactIDs:         map[string]struct{}{},
	}
	policies, err := s.Store.ListPolicies(orgID)
	if err != nil {
		return controls
	}
	for _, policy := range policies {
		if !policy.Enabled {
			continue
		}
		controls.PolicyIDs = append(controls.PolicyIDs, policy.ID)
		if v := floatRule(policy.Rules, "human_review_threshold", 0); v > 0 {
			if controls.HumanReviewThreshold == 0 || v < controls.HumanReviewThreshold {
				controls.HumanReviewThreshold = v
			}
		}
		if v := floatRule(policy.Rules, "protected_mutation_threshold", 0); v > 0 {
			if controls.ProtectedMutationThreshold == 0 || v < controls.ProtectedMutationThreshold {
				controls.ProtectedMutationThreshold = v
			}
		}
		if v := floatRule(policy.Rules, "default_entrenchment", 0); v > controls.DefaultEntrenchment {
			controls.DefaultEntrenchment = v
		}
		for sourceType, value := range floatMapRule(policy.Rules, "entrenchment_by_source_type") {
			if existing, ok := controls.AutoEntrenchmentBySource[sourceType]; !ok || value > existing {
				controls.AutoEntrenchmentBySource[sourceType] = value
			}
		}
		for _, sourceType := range stringSliceRule(policy.Rules, "review_source_types") {
			controls.ReviewSourceTypes[sourceType] = struct{}{}
		}
		for _, factID := range stringSliceRule(policy.Rules, "review_fact_ids") {
			controls.ReviewFactIDs[factID] = struct{}{}
		}
		for _, factID := range stringSliceRule(policy.Rules, "protected_fact_ids") {
			controls.ProtectedFactIDs[factID] = struct{}{}
		}
	}
	sort.Strings(controls.PolicyIDs)
	return controls
}

func applyFactGovernance(fact *core.Fact, controls policyControlSet) {
	if fact == nil {
		return
	}
	if fact.Metadata == nil {
		fact.Metadata = map[string]interface{}{}
	}
	if fact.Entrenchment <= 0 {
		sourceType := strings.TrimSpace(core.MetadataString(fact.Metadata, "source_type"))
		if sourceType != "" {
			if entrenchment, ok := controls.AutoEntrenchmentBySource[sourceType]; ok && entrenchment > fact.Entrenchment {
				fact.Entrenchment = entrenchment
			}
		}
		if fact.Entrenchment <= 0 && controls.DefaultEntrenchment > 0 {
			fact.Entrenchment = controls.DefaultEntrenchment
		}
	}
	fact.Entrenchment = core.ClampUnitFloat(fact.Entrenchment)
	reviewRequired := core.MetadataBool(fact.Metadata, "requires_human_review")
	if _, ok := controls.ReviewFactIDs[fact.ID]; ok {
		reviewRequired = true
	}
	if sourceType := strings.TrimSpace(core.MetadataString(fact.Metadata, "source_type")); sourceType != "" {
		if _, ok := controls.ReviewSourceTypes[sourceType]; ok {
			reviewRequired = true
		}
	}
	if controls.HumanReviewThreshold > 0 && fact.EffectiveEntrenchment() >= controls.HumanReviewThreshold {
		reviewRequired = true
	}
	if reviewRequired {
		fact.Metadata["requires_human_review"] = true
		if core.NormalizeReviewStatus(fact.ReviewStatus) == "" {
			fact.ReviewStatus = core.ReviewPending
			fact.Metadata["review_status"] = fact.ReviewStatus
		}
	}
	protectedMutation := core.MetadataBool(fact.Metadata, "protected_mutation")
	if _, ok := controls.ProtectedFactIDs[fact.ID]; ok {
		protectedMutation = true
	}
	if controls.ProtectedMutationThreshold > 0 && fact.EffectiveEntrenchment() >= controls.ProtectedMutationThreshold {
		protectedMutation = true
	}
	if protectedMutation {
		fact.Metadata["protected_mutation"] = true
	}
	if len(controls.PolicyIDs) > 0 {
		fact.Metadata["policy_ids"] = append([]string(nil), controls.PolicyIDs...)
	}
}

func mutationRequiresOverride(fact *core.Fact, actorRole string, force bool) error {
	if fact == nil {
		return nil
	}
	protected := core.MetadataBool(fact.Metadata, "protected_mutation")
	if !protected && fact.EffectiveEntrenchment() < 0.9 {
		return nil
	}
	if actorRole == "admin" && force {
		return nil
	}
	return fmt.Errorf("fact %s is protected by governance controls; admin force=true is required", fact.ID)
}

func (s *Server) handleReviewFact(w http.ResponseWriter, r *http.Request) {
	if getUserRole(r) != "admin" {
		http.Error(w, "forbidden: admin role required", http.StatusForbidden)
		return
	}
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

	var body factReviewRequest
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil && err != io.EOF {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}
	status := core.NormalizeReviewStatus(body.Status)
	if status == "" {
		http.Error(w, "invalid review status", http.StatusBadRequest)
		return
	}
	reviewedAt := time.Now().UnixMilli()
	if err := engine.SetFactReview(factID, status, body.Reason, reviewedAt); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	entry := store.JournalEntry{
		Type:      store.EventReview,
		SessionID: sessionID,
		FactID:    factID,
		ActorID:   getActorID(r),
		Payload: map[string]interface{}{
			"status":      status,
			"reason":      strings.TrimSpace(body.Reason),
			"reviewed_at": reviewedAt,
		},
	}
	if err := s.Store.Append(entry); err != nil {
		http.Error(w, "failed to persist review", http.StatusInternalServerError)
		return
	}
	_ = s.Store.AppendOrgActivity(orgID, entry)
	s.invalidateSliceCache(sessionID)
	s.syncSessionSearchDocuments(orgID, sessionID, engine, config)
	fact, _ := engine.GetFact(factID)
	fact.ResolvedStatus = engine.GetStatus(factID)
	writeJSON(w, http.StatusOK, fact)
}
