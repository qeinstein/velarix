package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"velarix/core"
	"velarix/store"
)

// privateRanges are CIDR blocks that must never be reached by outbound webhook calls.
var privateRanges []*net.IPNet

func init() {
	for _, cidr := range []string{
		"10.0.0.0/8",
		"172.16.0.0/12",
		"192.168.0.0/16",
		"127.0.0.0/8",
		"::1/128",
		"fc00::/7",
		"169.254.0.0/16", // link-local / GCE metadata
		"100.64.0.0/10",  // shared address space
	} {
		_, block, _ := net.ParseCIDR(cidr)
		if block != nil {
			privateRanges = append(privateRanges, block)
		}
	}
}

func isPrivateIP(ip net.IP) bool {
	for _, block := range privateRanges {
		if block.Contains(ip) {
			return true
		}
	}
	return false
}

// validateWebhookURL blocks SSRF by ensuring the resolved URL does not point at
// private/loopback/link-local addresses. Returns an error if the URL is unsafe.
func validateWebhookURL(rawURL string) error {
	u, err := url.Parse(rawURL)
	if err != nil {
		return fmt.Errorf("invalid webhook url: %w", err)
	}
	if u.Scheme != "https" && !isDevLikeEnv() {
		return fmt.Errorf("webhook url must use https in production")
	}
	host := u.Hostname()
	if host == "" {
		return fmt.Errorf("webhook url has no host")
	}
	// If the host is a raw IP, validate directly.
	if ip := net.ParseIP(host); ip != nil {
		if isPrivateIP(ip) {
			return fmt.Errorf("webhook url resolves to a private/reserved address")
		}
		return nil
	}
	// Otherwise resolve and check all A/AAAA records.
	addrs, err := net.LookupHost(host)
	if err != nil {
		return fmt.Errorf("webhook url host resolution failed: %w", err)
	}
	for _, addr := range addrs {
		if ip := net.ParseIP(addr); ip != nil && isPrivateIP(ip) {
			return fmt.Errorf("webhook url resolves to a private/reserved address (%s)", addr)
		}
	}
	return nil
}

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
	webhookURL := verificationWebhookURL()
	if webhookURL == "" {
		return
	}
	if err := validateWebhookURL(webhookURL); err != nil {
		slog.Error("verification webhook blocked: unsafe URL", "error", err)
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

		req, err := http.NewRequestWithContext(ctx, http.MethodPost, webhookURL, bytes.NewReader(payload))
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
