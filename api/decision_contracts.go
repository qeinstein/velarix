package api

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	apimodels "velarix/api/models"
	"velarix/core"
	"velarix/store"

	"github.com/golang-jwt/jwt/v5"
)

const executionTokenTTL = 30 * time.Second

type executionTokenClaims struct {
	SessionID       string `json:"session_id"`
	DecisionID      string `json:"decision_id"`
	OrgID           string `json:"org_id"`
	DecisionVersion int64  `json:"decision_version"`
	SessionVersion  int64  `json:"session_version"`
	CheckTimestamp  int64  `json:"check_timestamp"`
	jwt.RegisteredClaims
}

func newDecisionID() string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return "dec_" + hex.EncodeToString(b)
}

func factMetadataString(fact *core.Fact, key string) string {
	if fact == nil || fact.Metadata == nil {
		return ""
	}
	if v, ok := fact.Metadata[key].(string); ok {
		return v
	}
	return ""
}

func decisionSearchDocument(decision *store.Decision) store.SearchDocument {
	bodyParts := []string{decision.DecisionType, decision.RecommendedAction, decision.SubjectRef, decision.TargetRef, decision.ExplanationSummary}
	return store.SearchDocument{
		ID:           "decision:" + decision.ID,
		OrgID:        decision.OrgID,
		SessionID:    decision.SessionID,
		DocumentType: "decision",
		Title:        decision.DecisionType,
		Body:         strings.TrimSpace(strings.Join(bodyParts, " ")),
		Status:       decision.Status,
		SubjectRef:   decision.SubjectRef,
		TargetRef:    decision.TargetRef,
		DecisionID:   decision.ID,
		CreatedAt:    decision.CreatedAt,
		UpdatedAt:    decision.UpdatedAt,
		Metadata: map[string]interface{}{
			"execution_status":   decision.ExecutionStatus,
			"recommended_action": decision.RecommendedAction,
			"policy_version":     decision.PolicyVersion,
		},
	}
}

func decisionFactSearchDocuments(orgID, sessionID string, fact *core.Fact, updatedAt int64) []store.SearchDocument {
	if fact == nil {
		return nil
	}
	payload, _ := json.Marshal(fact.Payload)
	return []store.SearchDocument{
		{
			ID:           "fact:" + sessionID + ":" + fact.ID,
			OrgID:        orgID,
			SessionID:    sessionID,
			DocumentType: "fact",
			Title:        fact.ID,
			Body:         string(payload),
			Status:       fmt.Sprintf("%.2f", fact.ResolvedStatus),
			FactID:       fact.ID,
			CreatedAt:    updatedAt,
			UpdatedAt:    updatedAt,
		},
	}
}

func sessionSearchDocument(orgID, sessionID string, config *store.SessionConfig, updatedAt int64) store.SearchDocument {
	mode := ""
	if config != nil {
		mode = config.EnforcementMode
	}
	return store.SearchDocument{
		ID:           "session:" + sessionID,
		OrgID:        orgID,
		SessionID:    sessionID,
		DocumentType: "session",
		Title:        sessionID,
		Body:         "session " + sessionID,
		Status:       mode,
		CreatedAt:    updatedAt,
		UpdatedAt:    updatedAt,
	}
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
	sort.Strings(out)
	return out
}

func (s *Server) buildDecisionDependencies(engine *core.Engine, decision *store.Decision, dependencyIDs []string) ([]store.DecisionDependency, error) {
	if decision == nil {
		return nil, fmt.Errorf("decision is required")
	}
	if decision.FactID != "" && len(dependencyIDs) == 0 {
		ids, err := engine.DependencyIDs(decision.FactID, true)
		if err != nil {
			return nil, err
		}
		dependencyIDs = ids
	}
	dependencyIDs = uniqueStrings(dependencyIDs)
	deps := make([]store.DecisionDependency, 0, len(dependencyIDs))
	for _, factID := range dependencyIDs {
		fact, _ := engine.GetFact(factID)
		currentStatus := float64(engine.GetStatus(factID))
		sourceType := core.FactSourceType(fact)
		verificationStatus := core.FactVerificationStatus(fact)
		verifiedAt := int64(0)
		if fact != nil && fact.Metadata != nil {
			if v, ok := fact.Metadata["verified_at"].(float64); ok && int64(v) > 0 {
				verifiedAt = int64(v)
			} else if v, ok := fact.Metadata["verified_at"].(int64); ok && v > 0 {
				verifiedAt = v
			} else if v, ok := fact.Metadata["verified_at"].(int); ok && v > 0 {
				verifiedAt = int64(v)
			}
		}
		assertedAt := int64(0)
		if fact != nil {
			assertedAt = fact.AssertedAt
		}
		deps = append(deps, store.DecisionDependency{
			DecisionID:         decision.ID,
			SessionID:          decision.SessionID,
			FactID:             factID,
			DependencyType:     "fact",
			RequiredStatus:     "valid",
			CurrentStatus:      currentStatus,
			SourceType:         sourceType,
			SourceRef:          factMetadataString(fact, "source_ref"),
			VerificationStatus: verificationStatus,
			VerifiedAt:         verifiedAt,
			AssertedAt:         assertedAt,
			PolicyVersion:      firstNonEmpty(decision.PolicyVersion, factMetadataString(fact, "policy_version")),
			ExplanationHint:    factMetadataString(fact, "explanation_hint"),
			Entrenchment:       fact.EffectiveEntrenchment(),
			ReviewStatus:       fact.ReviewStatus,
			ReviewRequired:     fact.RequiresHumanReview(),
		})
	}
	return deps, nil
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}

func decisionVersion(decision *store.Decision) int64 {
	if decision == nil {
		return 0
	}
	if decision.UpdatedAt > 0 {
		return decision.UpdatedAt
	}
	return decision.CreatedAt
}

func (s *Server) issueExecutionToken(orgID string, decision *store.Decision, check *store.DecisionCheck) (string, error) {
	if decision == nil || check == nil {
		return "", fmt.Errorf("decision and check are required")
	}
	expiresAt := time.UnixMilli(check.ExpiresAt)
	claims := executionTokenClaims{
		SessionID:       decision.SessionID,
		DecisionID:      decision.ID,
		OrgID:           orgID,
		DecisionVersion: check.DecisionVersion,
		SessionVersion:  check.SessionVersion,
		CheckTimestamp:  check.CheckedAt,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(expiresAt),
			IssuedAt:  jwt.NewNumericDate(time.UnixMilli(check.CheckedAt)),
			NotBefore: jwt.NewNumericDate(time.UnixMilli(check.CheckedAt)),
			Subject:   decision.ID,
		},
	}
	signingKey, err := jwtSigningKey()
	if err != nil {
		return "", err
	}
	return jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(signingKey)
}

func (s *Server) parseExecutionToken(raw string) (*executionTokenClaims, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, fmt.Errorf("execution_token is required")
	}
	claims := &executionTokenClaims{}
	signingKey, err := jwtSigningKey()
	if err != nil {
		return nil, err
	}
	token, err := jwt.ParseWithClaims(raw, claims, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method")
		}
		return signingKey, nil
	})
	if err != nil {
		return nil, err
	}
	if !token.Valid {
		return nil, fmt.Errorf("invalid execution token")
	}
	return claims, nil
}

func (s *Server) computeDecisionCheck(engine *core.Engine, decision *store.Decision, deps []store.DecisionDependency) *store.DecisionCheck {
	check := &store.DecisionCheck{
		DecisionID:          decision.ID,
		SessionID:           decision.SessionID,
		Executable:          true,
		BlockedBy:           []store.DecisionBlocker{},
		ReasonCodes:         []string{},
		CheckedAt:           time.Now().UnixMilli(),
		DependencySnapshots: make([]store.DecisionDependency, 0, len(deps)),
	}

	controls := s.loadPolicyControls(decision.OrgID)
	now := time.Now().UnixMilli()

	for _, dep := range deps {
		snapshot := dep
		currentStatus := float64(engine.GetStatus(dep.FactID))
		fact, ok := engine.GetFact(dep.FactID)
		if !ok {
			currentStatus = 0
		}
		sourceType := core.FactSourceType(fact)
		verificationStatus := core.FactVerificationStatus(fact)
		verifiedAt := int64(0)
		assertedAt := int64(0)
		if fact != nil {
			assertedAt = fact.AssertedAt
			if fact.Metadata != nil {
				if v, ok := fact.Metadata["verified_at"].(float64); ok && int64(v) > 0 {
					verifiedAt = int64(v)
				} else if v, ok := fact.Metadata["verified_at"].(int64); ok && v > 0 {
					verifiedAt = v
				} else if v, ok := fact.Metadata["verified_at"].(int); ok && v > 0 {
					verifiedAt = int64(v)
				}
			}
		}
		snapshot.CurrentStatus = currentStatus
		snapshot.SourceType = sourceType
		snapshot.VerificationStatus = verificationStatus
		snapshot.VerifiedAt = verifiedAt
		snapshot.AssertedAt = assertedAt
		check.DependencySnapshots = append(check.DependencySnapshots, snapshot)

		if currentStatus < float64(core.ConfidenceThreshold) {
			check.Executable = false
			reasonCode := "dependency_invalid"
			if currentStatus == 0 {
				reasonCode = "dependency_missing_or_invalid"
			}
			check.ReasonCodes = append(check.ReasonCodes, reasonCode)
			check.BlockedBy = append(check.BlockedBy, store.DecisionBlocker{
				FactID:             dep.FactID,
				DependencyType:     dep.DependencyType,
				RequiredStatus:     dep.RequiredStatus,
				CurrentStatus:      currentStatus,
				ReasonCode:         reasonCode,
				SourceType:         sourceType,
				SourceRef:          dep.SourceRef,
				VerificationStatus: verificationStatus,
				VerifiedAt:         verifiedAt,
				AssertedAt:         assertedAt,
				PolicyVersion:      dep.PolicyVersion,
				ExplanationHint:    dep.ExplanationHint,
				Entrenchment:       dep.Entrenchment,
				ReviewStatus:       dep.ReviewStatus,
				ReviewRequired:     dep.ReviewRequired,
			})
			continue
		}

		// Grounding / execution gating checks apply to root premises (or facts
		// explicitly marked as requiring verification). Derived facts are gated
		// by their dependency validity rather than their own provenance.
		groundingRelevant := fact != nil && (fact.IsRoot || core.MetadataBool(fact.Metadata, "requires_verification") || strings.TrimSpace(sourceType) != "")

		if groundingRelevant && len(controls.GroundingAllowedSourceTypes) > 0 {
			if _, ok := controls.GroundingAllowedSourceTypes[sourceType]; !ok {
				check.Executable = false
				check.ReasonCodes = append(check.ReasonCodes, "untrusted_source")
				check.BlockedBy = append(check.BlockedBy, store.DecisionBlocker{
					FactID:             dep.FactID,
					DependencyType:     dep.DependencyType,
					RequiredStatus:     dep.RequiredStatus,
					CurrentStatus:      currentStatus,
					ReasonCode:         "untrusted_source",
					SourceType:         sourceType,
					SourceRef:          dep.SourceRef,
					VerificationStatus: verificationStatus,
					VerifiedAt:         verifiedAt,
					AssertedAt:         assertedAt,
					PolicyVersion:      dep.PolicyVersion,
					ExplanationHint:    dep.ExplanationHint,
					Entrenchment:       dep.Entrenchment,
					ReviewStatus:       dep.ReviewStatus,
					ReviewRequired:     dep.ReviewRequired,
				})
			}
		}
		if groundingRelevant && controls.GroundingRequireVerified {
			if verificationStatus != core.VerificationVerified {
				check.Executable = false
				check.ReasonCodes = append(check.ReasonCodes, "unverified_dependency")
				check.BlockedBy = append(check.BlockedBy, store.DecisionBlocker{
					FactID:             dep.FactID,
					DependencyType:     dep.DependencyType,
					RequiredStatus:     dep.RequiredStatus,
					CurrentStatus:      currentStatus,
					ReasonCode:         "unverified_dependency",
					SourceType:         sourceType,
					SourceRef:          dep.SourceRef,
					VerificationStatus: verificationStatus,
					VerifiedAt:         verifiedAt,
					AssertedAt:         assertedAt,
					PolicyVersion:      dep.PolicyVersion,
					ExplanationHint:    dep.ExplanationHint,
					Entrenchment:       dep.Entrenchment,
					ReviewStatus:       dep.ReviewStatus,
					ReviewRequired:     dep.ReviewRequired,
				})
			}
		}
		if groundingRelevant && controls.GroundingMaxAgeSeconds > 0 && assertedAt > 0 {
			if now-assertedAt > controls.GroundingMaxAgeSeconds*1000 {
				check.Executable = false
				check.ReasonCodes = append(check.ReasonCodes, "dependency_too_old")
				check.BlockedBy = append(check.BlockedBy, store.DecisionBlocker{
					FactID:             dep.FactID,
					DependencyType:     dep.DependencyType,
					RequiredStatus:     dep.RequiredStatus,
					CurrentStatus:      currentStatus,
					ReasonCode:         "dependency_too_old",
					SourceType:         sourceType,
					SourceRef:          dep.SourceRef,
					VerificationStatus: verificationStatus,
					VerifiedAt:         verifiedAt,
					AssertedAt:         assertedAt,
					PolicyVersion:      dep.PolicyVersion,
					ExplanationHint:    dep.ExplanationHint,
					Entrenchment:       dep.Entrenchment,
					ReviewStatus:       dep.ReviewStatus,
					ReviewRequired:     dep.ReviewRequired,
				})
			}
		}

		if dep.ReviewRequired {
			check.Executable = false
			check.ReasonCodes = append(check.ReasonCodes, "human_review_required")
			check.BlockedBy = append(check.BlockedBy, store.DecisionBlocker{
				FactID:             dep.FactID,
				DependencyType:     dep.DependencyType,
				RequiredStatus:     dep.RequiredStatus,
				CurrentStatus:      currentStatus,
				ReasonCode:         "human_review_required",
				SourceType:         sourceType,
				SourceRef:          dep.SourceRef,
				VerificationStatus: verificationStatus,
				VerifiedAt:         verifiedAt,
				AssertedAt:         assertedAt,
				PolicyVersion:      dep.PolicyVersion,
				ExplanationHint:    dep.ExplanationHint,
				Entrenchment:       dep.Entrenchment,
				ReviewStatus:       dep.ReviewStatus,
				ReviewRequired:     true,
			})
		}
	}

	// Optional quorum: decision metadata may request N verified independent
	// sources for a claim_key before execution.
	if decision != nil && decision.Metadata != nil {
		if raw, ok := decision.Metadata["verification_quorum_by_claim_key"]; ok {
			quorum := map[string]int{}
			switch v := raw.(type) {
			case map[string]int:
				for k, n := range v {
					quorum[strings.TrimSpace(k)] = n
				}
			case map[string]interface{}:
				for k, val := range v {
					n := 0
					switch vv := val.(type) {
					case float64:
						n = int(vv)
					case int:
						n = vv
					case string:
						if parsed, err := strconv.Atoi(strings.TrimSpace(vv)); err == nil {
							n = parsed
						}
					}
					if strings.TrimSpace(k) != "" && n > 0 {
						quorum[strings.TrimSpace(k)] = n
					}
				}
			}

			for claimKey, needed := range quorum {
				if claimKey == "" || needed <= 1 {
					continue
				}
				refs := map[string]struct{}{}
				for _, snap := range check.DependencySnapshots {
					if snap.VerificationStatus != core.VerificationVerified {
						continue
					}
					f, ok := engine.GetFact(snap.FactID)
					if !ok {
						continue
					}
					if core.FactClaimKey(f) != claimKey {
						continue
					}
					ref := strings.TrimSpace(snap.SourceRef)
					if ref == "" {
						ref = strings.TrimSpace(core.MetadataString(f.Metadata, "verification_source_ref"))
					}
					if ref == "" {
						ref = snap.SourceType
					}
					refs[ref] = struct{}{}
				}
				if len(refs) < needed {
					check.Executable = false
					check.ReasonCodes = append(check.ReasonCodes, "verification_quorum_not_met")
					check.BlockedBy = append(check.BlockedBy, store.DecisionBlocker{
						FactID:          decision.FactID,
						DependencyType:  "quorum",
						RequiredStatus:  fmt.Sprintf("verified_quorum_%d", needed),
						CurrentStatus:   float64(len(refs)),
						ReasonCode:      "verification_quorum_not_met",
						ExplanationHint: fmt.Sprintf("claim_key=%s requires %d independent verified sources, have %d", claimKey, needed, len(refs)),
					})
				}
			}
		}
	}

	check.ReasonCodes = uniqueStrings(check.ReasonCodes)
	if decision.FactID != "" {
		if explanation, err := engine.ExplainReasoning(decision.FactID); err == nil {
			check.ExplanationSummary = explanation.Summary
		}
	}
	if check.ExplanationSummary == "" {
		if check.Executable {
			check.ExplanationSummary = "Decision is executable against the current dependency set."
		} else {
			check.ExplanationSummary = "Decision is blocked because one or more dependencies are stale."
		}
	}
	return check
}

func decisionStatusFromCheck(check *store.DecisionCheck, existingExecutionStatus string) (string, string) {
	if check == nil {
		return "active", existingExecutionStatus
	}
	if check.Executable {
		if existingExecutionStatus == "executed" {
			return "executed", "executed"
		}
		return "active", "pending"
	}
	return "blocked", "blocked"
}

func (s *Server) persistDecisionReadModels(decision *store.Decision, deps []store.DecisionDependency) {
	if decision == nil {
		return
	}
	docs := []store.SearchDocument{decisionSearchDocument(decision)}
	for _, dep := range deps {
		docs = append(docs, store.SearchDocument{
			ID:           fmt.Sprintf("dependency:%s:%s", decision.ID, dep.FactID),
			OrgID:        decision.OrgID,
			SessionID:    decision.SessionID,
			DocumentType: "dependency",
			Title:        dep.FactID,
			Body:         firstNonEmpty(dep.ExplanationHint, dep.SourceRef, dep.PolicyVersion),
			Status:       dep.RequiredStatus,
			FactID:       dep.FactID,
			DecisionID:   decision.ID,
			SubjectRef:   decision.SubjectRef,
			CreatedAt:    decision.CreatedAt,
			UpdatedAt:    decision.UpdatedAt,
		})
	}
	_ = s.Store.UpsertSearchDocuments(docs)
}

func parseDecisionFilter(r *http.Request) store.DecisionListFilter {
	filter := store.DecisionListFilter{Limit: 50}
	filter.Status = strings.TrimSpace(r.URL.Query().Get("status"))
	filter.SubjectRef = strings.TrimSpace(r.URL.Query().Get("subject"))
	if v := r.URL.Query().Get("from"); v != "" {
		if parsed, err := strconv.ParseInt(v, 10, 64); err == nil {
			filter.FromMs = parsed
		}
	}
	if v := r.URL.Query().Get("to"); v != "" {
		if parsed, err := strconv.ParseInt(v, 10, 64); err == nil {
			filter.ToMs = parsed
		}
	}
	if v := r.URL.Query().Get("limit"); v != "" {
		if parsed, err := strconv.Atoi(v); err == nil && parsed > 0 && parsed <= 200 {
			filter.Limit = parsed
		}
	}
	return filter
}

func (s *Server) handleCreateDecision(w http.ResponseWriter, r *http.Request) {
	sessionID := r.PathValue("session_id")
	orgID := getOrgID(r)
	engine, config, err := s.getEngine(sessionID, orgID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusForbidden)
		return
	}

	var body apimodels.CreateDecisionRequest
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}
	if strings.TrimSpace(body.DecisionType) == "" {
		http.Error(w, "decision_type is required", http.StatusBadRequest)
		return
	}
	decisionID := strings.TrimSpace(body.DecisionID)
	if decisionID == "" {
		decisionID = newDecisionID()
	}
	now := time.Now().UnixMilli()
	decision := &store.Decision{
		ID:                 decisionID,
		SessionID:          sessionID,
		OrgID:              orgID,
		FactID:             strings.TrimSpace(body.FactID),
		DecisionType:       strings.TrimSpace(body.DecisionType),
		SubjectRef:         strings.TrimSpace(body.SubjectRef),
		TargetRef:          strings.TrimSpace(body.TargetRef),
		RecommendedAction:  strings.TrimSpace(body.RecommendedAction),
		PolicyVersion:      firstNonEmpty(strings.TrimSpace(body.PolicyVersion), config.Schema),
		ExplanationSummary: strings.TrimSpace(body.ExplanationSummary),
		Status:             "active",
		ExecutionStatus:    "pending",
		CreatedBy:          getActorID(r),
		CreatedAt:          now,
		UpdatedAt:          now,
		Metadata:           body.Metadata,
	}
	deps, err := s.buildDecisionDependencies(engine, decision, body.DependencyFactIDs)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	check := s.computeDecisionCheck(engine, decision, deps)
	decision.Status, decision.ExecutionStatus = decisionStatusFromCheck(check, decision.ExecutionStatus)
	decision.LastCheckedAt = check.CheckedAt
	if decision.ExplanationSummary == "" {
		decision.ExplanationSummary = check.ExplanationSummary
	}
	if err := s.Store.SaveDecision(decision); err != nil {
		http.Error(w, "failed to save decision", http.StatusInternalServerError)
		return
	}
	if err := s.Store.SaveDecisionDependencies(sessionID, decision.ID, deps); err != nil {
		http.Error(w, "failed to save decision dependencies", http.StatusInternalServerError)
		return
	}
	if err := s.Store.SaveDecisionCheck(sessionID, decision.ID, check); err != nil {
		http.Error(w, "failed to save decision check", http.StatusInternalServerError)
		return
	}
	s.persistDecisionReadModels(decision, deps)
	_ = s.Store.AppendOrgActivity(orgID, store.JournalEntry{
		Type:      store.EventDecisionRecord,
		SessionID: sessionID,
		ActorID:   getActorID(r),
		Payload: map[string]interface{}{
			"action":      "decision_created",
			"decision_id": decision.ID,
			"status":      decision.Status,
		},
		Timestamp: now,
	})
	writeJSON(w, http.StatusCreated, decision)
}

func (s *Server) handleGetDecision(w http.ResponseWriter, r *http.Request) {
	sessionID := r.PathValue("session_id")
	orgID := getOrgID(r)
	storedOrg, err := s.Store.GetSessionOrganization(sessionID)
	if err != nil || storedOrg != orgID {
		http.Error(w, "unauthorized", http.StatusForbidden)
		return
	}
	decision, err := s.Store.GetDecision(sessionID, r.PathValue("decision_id"))
	if err != nil {
		http.Error(w, "decision not found", http.StatusNotFound)
		return
	}
	writeJSON(w, http.StatusOK, decision)
}

func (s *Server) handleListSessionDecisions(w http.ResponseWriter, r *http.Request) {
	sessionID := r.PathValue("session_id")
	orgID := getOrgID(r)
	storedOrg, err := s.Store.GetSessionOrganization(sessionID)
	if err != nil || storedOrg != orgID {
		http.Error(w, "unauthorized", http.StatusForbidden)
		return
	}
	items, err := s.Store.ListSessionDecisions(sessionID, parseDecisionFilter(r))
	if err != nil {
		http.Error(w, "failed to list decisions", http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"items": items})
}

func (s *Server) handleRecomputeDecision(w http.ResponseWriter, r *http.Request) {
	sessionID := r.PathValue("session_id")
	orgID := getOrgID(r)
	engine, _, err := s.getEngine(sessionID, orgID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusForbidden)
		return
	}
	decisionID := r.PathValue("decision_id")
	decision, err := s.Store.GetDecision(sessionID, decisionID)
	if err != nil {
		http.Error(w, "decision not found", http.StatusNotFound)
		return
	}
	var body apimodels.RecomputeDecisionRequest
	_ = json.NewDecoder(r.Body).Decode(&body)
	if strings.TrimSpace(body.FactID) != "" {
		decision.FactID = strings.TrimSpace(body.FactID)
	}
	deps, err := s.buildDecisionDependencies(engine, decision, body.DependencyFactIDs)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	check := s.computeDecisionCheck(engine, decision, deps)
	decision.LastCheckedAt = check.CheckedAt
	decision.Status, decision.ExecutionStatus = decisionStatusFromCheck(check, decision.ExecutionStatus)
	decision.ExplanationSummary = check.ExplanationSummary
	decision.UpdatedAt = time.Now().UnixMilli()
	if err := s.Store.SaveDecision(decision); err != nil {
		http.Error(w, "failed to save decision", http.StatusInternalServerError)
		return
	}
	if err := s.Store.SaveDecisionDependencies(sessionID, decision.ID, deps); err != nil {
		http.Error(w, "failed to save dependencies", http.StatusInternalServerError)
		return
	}
	if err := s.Store.SaveDecisionCheck(sessionID, decision.ID, check); err != nil {
		http.Error(w, "failed to save decision check", http.StatusInternalServerError)
		return
	}
	s.persistDecisionReadModels(decision, deps)
	writeJSON(w, http.StatusOK, map[string]interface{}{"decision": decision, "check": check})
}

func (s *Server) handleExecuteDecisionCheck(w http.ResponseWriter, r *http.Request) {
	sessionID := r.PathValue("session_id")
	orgID := getOrgID(r)
	engine, _, err := s.getEngine(sessionID, orgID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusForbidden)
		return
	}
	decisionID := r.PathValue("decision_id")
	decision, err := s.Store.GetDecision(sessionID, decisionID)
	if err != nil {
		http.Error(w, "decision not found", http.StatusNotFound)
		return
	}
	deps, err := s.Store.GetDecisionDependencies(sessionID, decisionID)
	if err != nil {
		http.Error(w, "failed to read dependencies", http.StatusInternalServerError)
		return
	}
	check := s.computeDecisionCheck(engine, decision, deps)
	check.SessionVersion, _ = s.Store.GetSessionVersion(sessionID)
	decision.LastCheckedAt = check.CheckedAt
	decision.Status, decision.ExecutionStatus = decisionStatusFromCheck(check, decision.ExecutionStatus)
	decision.ExplanationSummary = check.ExplanationSummary
	decision.UpdatedAt = time.Now().UnixMilli()
	check.DecisionVersion = decisionVersion(decision)
	if check.Executable {
		check.ExpiresAt = check.CheckedAt + executionTokenTTL.Milliseconds()
		token, err := s.issueExecutionToken(orgID, decision, check)
		if err != nil {
			http.Error(w, "failed to issue execution token", http.StatusInternalServerError)
			return
		}
		check.ExecutionToken = token
	}
	_ = s.Store.SaveDecision(decision)
	_ = s.Store.SaveDecisionCheck(sessionID, decisionID, check)
	s.persistDecisionReadModels(decision, deps)
	writeJSON(w, http.StatusOK, check)
}

func (s *Server) handleExecuteDecision(w http.ResponseWriter, r *http.Request) {
	sessionID := r.PathValue("session_id")
	orgID := getOrgID(r)
	engine, _, err := s.getEngine(sessionID, orgID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusForbidden)
		return
	}
	decisionID := r.PathValue("decision_id")
	decision, err := s.Store.GetDecision(sessionID, decisionID)
	if err != nil {
		http.Error(w, "decision not found", http.StatusNotFound)
		return
	}
	deps, err := s.Store.GetDecisionDependencies(sessionID, decisionID)
	if err != nil {
		http.Error(w, "failed to read dependencies", http.StatusInternalServerError)
		return
	}
	if decision.ExecutionStatus == "executed" {
		http.Error(w, "decision already executed", http.StatusConflict)
		return
	}
	var body apimodels.ExecuteDecisionRequest
	if r.Body != nil {
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil && !errors.Is(err, io.EOF) {
			http.Error(w, "invalid request", http.StatusBadRequest)
			return
		}
	}
	claims, err := s.parseExecutionToken(body.ExecutionToken)
	if err != nil {
		http.Error(w, err.Error(), http.StatusConflict)
		return
	}
	currentSessionVersion, _ := s.Store.GetSessionVersion(sessionID)
	if claims.SessionID != sessionID || claims.DecisionID != decisionID || claims.OrgID != orgID {
		http.Error(w, "execution token does not match this decision", http.StatusConflict)
		return
	}
	if claims.SessionVersion != currentSessionVersion {
		http.Error(w, "execution token is stale; run execute-check again", http.StatusConflict)
		return
	}
	if claims.DecisionVersion != decisionVersion(decision) || claims.CheckTimestamp != decision.LastCheckedAt {
		http.Error(w, "execution token no longer matches the latest decision state", http.StatusConflict)
		return
	}
	check := s.computeDecisionCheck(engine, decision, deps)
	check.SessionVersion = currentSessionVersion
	check.DecisionVersion = claims.DecisionVersion
	_ = s.Store.SaveDecisionCheck(sessionID, decisionID, check)
	now := time.Now().UnixMilli()
	if !check.Executable {
		decision.Status = "blocked"
		decision.ExecutionStatus = "blocked"
		decision.LastCheckedAt = check.CheckedAt
		decision.ExplanationSummary = check.ExplanationSummary
		decision.UpdatedAt = now
		_ = s.Store.SaveDecision(decision)
		s.persistDecisionReadModels(decision, deps)
		_ = s.Store.AppendOrgActivity(orgID, store.JournalEntry{
			Type:      store.EventDecisionRecord,
			SessionID: sessionID,
			ActorID:   getActorID(r),
			Payload: map[string]interface{}{
				"action":       "decision_execute_blocked",
				"decision_id":  decision.ID,
				"reason_codes": check.ReasonCodes,
			},
			Timestamp: now,
		})
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusConflict)
		_ = json.NewEncoder(w).Encode(check)
		return
	}
	decision.Status = "executed"
	decision.ExecutionStatus = "executed"
	decision.LastCheckedAt = check.CheckedAt
	decision.ExecutedAt = now
	decision.ExecutedBy = getActorID(r)
	decision.UpdatedAt = now
	decision.ExplanationSummary = check.ExplanationSummary
	if err := s.Store.SaveDecision(decision); err != nil {
		http.Error(w, "failed to save decision", http.StatusInternalServerError)
		return
	}
	s.persistDecisionReadModels(decision, deps)
	_ = s.Store.AppendOrgActivity(orgID, store.JournalEntry{
		Type:      store.EventDecisionRecord,
		SessionID: sessionID,
		ActorID:   getActorID(r),
		Payload: map[string]interface{}{
			"action":       "decision_executed",
			"decision_id":  decision.ID,
			"checked_at":   check.CheckedAt,
			"dependencies": check.DependencySnapshots,
		},
		Timestamp: now,
	})
	writeJSON(w, http.StatusOK, map[string]interface{}{"decision": decision, "check": check})
}

func (s *Server) handleDecisionLineage(w http.ResponseWriter, r *http.Request) {
	sessionID := r.PathValue("session_id")
	orgID := getOrgID(r)
	storedOrg, err := s.Store.GetSessionOrganization(sessionID)
	if err != nil || storedOrg != orgID {
		http.Error(w, "unauthorized", http.StatusForbidden)
		return
	}
	decisionID := r.PathValue("decision_id")
	decision, err := s.Store.GetDecision(sessionID, decisionID)
	if err != nil {
		http.Error(w, "decision not found", http.StatusNotFound)
		return
	}
	deps, _ := s.Store.GetDecisionDependencies(sessionID, decisionID)
	check, _ := s.Store.GetLatestDecisionCheck(sessionID, decisionID)
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"decision":      decision,
		"dependencies":  deps,
		"latest_check":  check,
		"decision_fact": decision.FactID,
	})
}

func (s *Server) handleDecisionWhyBlocked(w http.ResponseWriter, r *http.Request) {
	sessionID := r.PathValue("session_id")
	orgID := getOrgID(r)
	engine, _, err := s.getEngine(sessionID, orgID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusForbidden)
		return
	}
	decisionID := r.PathValue("decision_id")
	decision, err := s.Store.GetDecision(sessionID, decisionID)
	if err != nil {
		http.Error(w, "decision not found", http.StatusNotFound)
		return
	}
	deps, _ := s.Store.GetDecisionDependencies(sessionID, decisionID)
	check, _ := s.Store.GetLatestDecisionCheck(sessionID, decisionID)
	if check == nil {
		check = s.computeDecisionCheck(engine, decision, deps)
		_ = s.Store.SaveDecisionCheck(sessionID, decisionID, check)
	}
	var explanation interface{}
	if decision.FactID != "" {
		if out, err := engine.ExplainReasoning(decision.FactID); err == nil {
			explanation = out
		}
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"decision":     decision,
		"check":        check,
		"blocked_by":   check.BlockedBy,
		"reason_codes": check.ReasonCodes,
		"explanation":  explanation,
	})
}

func (s *Server) handleListOrgDecisions(w http.ResponseWriter, r *http.Request) {
	orgID := getOrgID(r)
	items, err := s.Store.ListOrgDecisions(orgID, parseDecisionFilter(r))
	if err != nil {
		http.Error(w, "failed to list decisions", http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"items": items})
}

func (s *Server) handleListBlockedOrgDecisions(w http.ResponseWriter, r *http.Request) {
	orgID := getOrgID(r)
	filter := parseDecisionFilter(r)
	filter.Status = "blocked"
	items, err := s.Store.ListOrgDecisions(orgID, filter)
	if err != nil {
		http.Error(w, "failed to list decisions", http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"items": items})
}
