package api

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"velarix/core"
	"velarix/store"
)

type factVerifyRequest struct {
	Status     string `json:"status"`
	Method     string `json:"method,omitempty"`
	SourceRef  string `json:"source_ref,omitempty"`
	Reason     string `json:"reason,omitempty"`
	VerifiedAt int64  `json:"verified_at,omitempty"`
}

func verificationWebhookURL() string {
	return strings.TrimSpace(os.Getenv("VELARIX_VERIFICATION_WEBHOOK_URL"))
}

func verificationWebhookTimeout() time.Duration {
	secs := 5
	if raw := strings.TrimSpace(os.Getenv("VELARIX_VERIFICATION_WEBHOOK_TIMEOUT_SECONDS")); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil && parsed > 0 && parsed <= 60 {
			secs = parsed
		}
	}
	return time.Duration(secs) * time.Second
}

func (s *Server) handleVerifyFact(w http.ResponseWriter, r *http.Request) {
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

	var body factVerifyRequest
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil && err != io.EOF {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}
	verifiedAt := body.VerifiedAt
	if verifiedAt == 0 {
		verifiedAt = time.Now().UnixMilli()
	}
	if err := engine.SetFactVerification(factID, body.Status, body.Method, body.SourceRef, body.Reason, verifiedAt); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	entry := store.JournalEntry{
		Type:      store.EventFactVerification,
		SessionID: sessionID,
		ActorID:   getActorID(r),
		FactID:    factID,
		Payload: map[string]interface{}{
			"status":      strings.TrimSpace(body.Status),
			"method":      strings.TrimSpace(body.Method),
			"source_ref":  strings.TrimSpace(body.SourceRef),
			"reason":      strings.TrimSpace(body.Reason),
			"verified_at": verifiedAt,
		},
		Timestamp: time.Now().UnixMilli(),
	}
	_ = s.Store.Append(entry)
	_ = s.Store.AppendOrgActivity(orgID, entry)

	s.invalidateSliceCache(sessionID)
	s.checkSnapshotTrigger(sessionID, engine)
	s.syncSessionSearchDocuments(orgID, sessionID, engine, config)
	fact, _ := engine.GetFact(factID)
	fact.ResolvedStatus = engine.GetStatus(factID)
	writeJSON(w, http.StatusOK, fact)
}

func (s *Server) maybeStartVerification(sessionID string, orgID string, engine *core.Engine, fact *core.Fact) {
	if fact == nil || engine == nil {
		return
	}
	if !core.MetadataBool(fact.Metadata, "requires_verification") {
		return
	}
	if core.FactVerificationStatus(fact) == core.VerificationVerified {
		return
	}
	url := verificationWebhookURL()
	if url == "" {
		return
	}

	go func() {
		timeout := verificationWebhookTimeout()
		ctx, cancel := context.WithTimeout(context.Background(), timeout)
		defer cancel()

		factCopy, _ := engine.GetFact(fact.ID)
		if factCopy == nil {
			return
		}

		body := map[string]interface{}{
			"session_id": sessionID,
			"org_id":     orgID,
			"fact":       factCopy,
		}
		payload, _ := json.Marshal(body)

		req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(payload))
		if err != nil {
			return
		}
		req.Header.Set("Content-Type", "application/json")

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			slog.Warn("verification webhook failed", "session_id", sessionID, "fact_id", factCopy.ID, "error", err)
			return
		}
		defer resp.Body.Close()
		if resp.StatusCode >= 300 {
			b, _ := io.ReadAll(resp.Body)
			slog.Warn("verification webhook returned error",
				"session_id", sessionID, "fact_id", factCopy.ID, "status", resp.StatusCode, "body", strings.TrimSpace(string(b)))
			return
		}

		var out factVerifyRequest
		if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
			slog.Warn("verification webhook response decode failed", "session_id", sessionID, "fact_id", factCopy.ID, "error", err)
			return
		}
		if strings.TrimSpace(out.Status) == "" {
			return
		}
		verifiedAt := out.VerifiedAt
		if verifiedAt == 0 {
			verifiedAt = time.Now().UnixMilli()
		}
		if err := engine.SetFactVerification(factCopy.ID, out.Status, out.Method, out.SourceRef, out.Reason, verifiedAt); err != nil {
			return
		}

		entry := store.JournalEntry{
			Type:      store.EventFactVerification,
			SessionID: sessionID,
			ActorID:   "system",
			FactID:    factCopy.ID,
			Payload: map[string]interface{}{
				"status":      strings.TrimSpace(out.Status),
				"method":      strings.TrimSpace(out.Method),
				"source_ref":  strings.TrimSpace(out.SourceRef),
				"reason":      strings.TrimSpace(out.Reason),
				"verified_at": verifiedAt,
			},
			Timestamp: time.Now().UnixMilli(),
		}
		if err := s.Store.Append(entry); err != nil {
			slog.Warn("failed to persist fact_verification journal entry", "session_id", sessionID, "fact_id", factCopy.ID, "error", err)
		}
		_ = s.Store.AppendOrgActivity(orgID, entry)
		s.invalidateSliceCache(sessionID)
	}()
}
