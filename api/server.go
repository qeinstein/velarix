package api

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"math"
	"net/http"
	"os"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"velarix/core"
	"velarix/store"

	"github.com/golang-jwt/jwt/v5"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/xeipuuv/gojsonschema"
	"go.opentelemetry.io/otel"
)

type contextKey string

const orgIDKey contextKey = "org_id"
const actorIDKey contextKey = "actor_id"
const userRoleKey contextKey = "user_role"
const userEmailKey contextKey = "user_email"
const scopesKey contextKey = "scopes"

type sessionInfo struct {
	ID              string `json:"id"`
	FactCount       int    `json:"fact_count"`
	EnforcementMode string `json:"enforcement_mode"`
	Status          string `json:"status"`
}

func getOrgID(r *http.Request) string {
	val := r.Context().Value(orgIDKey)
	if val == nil {
		return ""
	}
	return val.(string)
}

func getActorID(r *http.Request) string {
	val := r.Context().Value(actorIDKey)
	if val == nil {
		return "system"
	}
	return val.(string)
}

func (s *Server) auditAdmin(sessionID string, actorID string, action string, payload map[string]interface{}) {
	entry := store.JournalEntry{Type: store.EventAdminAction, SessionID: sessionID, ActorID: actorID, Payload: map[string]interface{}{}}
	entry.Timestamp = time.Now().UnixMilli()
	entry.Payload["action"] = action
	for k, v := range payload {
		entry.Payload[k] = v
	}
	if err := s.Store.Append(entry); err != nil {
		slog.Error("Failed to record audit trail", "error", err, "action", action)
	}
}

type SliceCacheEntry struct {
	Facts     []*core.Fact
	Timestamp time.Time
}

type Server struct {
	mu         sync.RWMutex
	Engines    map[string]*core.Engine
	Configs    map[string]*store.SessionConfig
	Versions   map[string]int64
	LastAccess map[string]time.Time
	SliceCache map[string]*SliceCacheEntry
	Store      store.ServerStore
	StartTime  time.Time
	LiteMode   bool

	writeLimiters sync.Map // org_id -> chan struct{}
}

func (s *Server) invalidateSliceCache(sessionID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.SliceCache == nil {
		return
	}
	delete(s.SliceCache, sessionID)
	for key := range s.SliceCache {
		if strings.HasPrefix(key, sessionID+"|") {
			delete(s.SliceCache, key)
		}
	}
}

func (s *Server) getEngine(sessionID string, orgID string) (*core.Engine, *store.SessionConfig, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.LastAccess == nil {
		s.LastAccess = make(map[string]time.Time)
	}
	if s.Versions == nil {
		s.Versions = make(map[string]int64)
	}
	s.LastAccess[sessionID] = time.Now()

	currentVersion, _ := s.Store.GetSessionVersion(sessionID)
	engine, ok := s.Engines[sessionID]
	config := s.Configs[sessionID]

	if ok {
		storedOrg, err := s.Store.GetSessionOrganization(sessionID)
		if err == nil && storedOrg != orgID {
			return nil, nil, fmt.Errorf("unauthorized: session belongs to a different organization")
		}
		if s.Versions[sessionID] == currentVersion {
			return engine, config, nil
		}
	}

	storedOrg, err := s.Store.GetSessionOrganization(sessionID)
	if err != nil {
		if err := s.Store.SetSessionOrganization(sessionID, orgID); err != nil {
			return nil, nil, fmt.Errorf("failed to link session to org: %v", err)
		}
		_ = s.Store.UpsertOrgSessionIndex(orgID, sessionID, time.Now().UnixMilli())
		go s.Store.IncrementOrgMetric(orgID, "sessions_created", 1)
	} else if storedOrg != orgID {
		return nil, nil, fmt.Errorf("unauthorized: session belongs to a different organization")
	}
	currentVersion, _ = s.Store.GetSessionVersion(sessionID)

	_ = s.Store.TouchOrgSession(orgID, sessionID, 0)

	engine = core.NewEngine()
	config, err = s.Store.GetConfig(sessionID)
	if err != nil || config == nil {
		config = &store.SessionConfig{EnforcementMode: "strict"}
	}

	// Hybrid Boot: Try Snapshot first, then Journal Replay
	var lastSnapshotTS int64
	if snap, err := s.Store.GetLatestSnapshot(sessionID); err == nil && snap != nil {
		if err := engine.FromSnapshot(snap); err == nil {
			lastSnapshotTS = snap.Timestamp
		}
	}

	var history []store.JournalEntry
	if lastSnapshotTS > 0 {
		history, err = s.Store.GetSessionHistoryAfter(sessionID, lastSnapshotTS)
	} else {
		history, err = s.Store.GetSessionHistory(sessionID)
	}
	if err == nil {
		for _, entry := range history {
			switch entry.Type {
			case store.EventAssert:
				engine.AssertFact(entry.Fact)
			case store.EventInvalidate:
				engine.InvalidateRoot(entry.FactID)
			case store.EventRetract:
				reason := ""
				if entry.Payload != nil {
					if v, ok := entry.Payload["reason"].(string); ok {
						reason = v
					}
				}
				engine.RetractFact(entry.FactID, reason)
			case store.EventReview:
				status := ""
				reason := ""
				reviewedAt := entry.Timestamp
				if entry.Payload != nil {
					if v, ok := entry.Payload["status"].(string); ok {
						status = v
					}
					if v, ok := entry.Payload["reason"].(string); ok {
						reason = v
					}
					if v, ok := entry.Payload["reviewed_at"].(float64); ok {
						reviewedAt = int64(v)
					}
				}
				engine.SetFactReview(entry.FactID, status, reason, reviewedAt)
			}
		}
	}

	s.Engines[sessionID] = engine
	s.Configs[sessionID] = config
	s.Versions[sessionID] = currentVersion
	ActiveSessions.Set(float64(len(s.Engines)))

	return engine, config, nil
}

func (s *Server) maxConcurrentWrites() int {
	// Default is intentionally high to avoid surprising 503s in dev/test.
	// Production deployments should tune this to protect latency under bursty agent traffic.
	limit := 256
	if strings.TrimSpace(os.Getenv("VELARIX_ENV")) == "prod" {
		limit = 64
	}
	if v := strings.TrimSpace(os.Getenv("VELARIX_MAX_CONCURRENT_WRITES")); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 && n <= 10000 {
			limit = n
		}
	}
	return limit
}

func (s *Server) writeLimiter(orgID string) chan struct{} {
	if orgID == "" {
		orgID = "unknown"
	}
	limit := s.maxConcurrentWrites()
	if v, ok := s.writeLimiters.Load(orgID); ok {
		ch := v.(chan struct{})
		// If the limit changed between runs, keep existing channel; this is a best-effort limiter.
		_ = limit
		return ch
	}
	ch := make(chan struct{}, limit)
	actual, _ := s.writeLimiters.LoadOrStore(orgID, ch)
	return actual.(chan struct{})
}

func (s *Server) writeLimitMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet, http.MethodHead, http.MethodOptions:
			next.ServeHTTP(w, r)
			return
		}

		// Only apply after auth has populated org context.
		orgID := getOrgID(r)
		if orgID == "" {
			next.ServeHTTP(w, r)
			return
		}

		ch := s.writeLimiter(orgID)
		select {
		case ch <- struct{}{}:
			defer func() { <-ch }()
			next.ServeHTTP(w, r)
			return
		default:
			w.Header().Set("Retry-After", "1")
			w.Header().Set("X-Velarix-Backpressure", "1")
			http.Error(w, "backpressure: too many concurrent writes (retry)", http.StatusServiceUnavailable)
			return
		}
	})
}

func (s *Server) enableCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := strings.TrimSpace(r.Header.Get("Origin"))
		allowed := strings.TrimSpace(os.Getenv("VELARIX_ALLOWED_ORIGINS"))
		switch {
		case origin == "":
		case allowed == "" && isDevLikeEnv():
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Access-Control-Allow-Credentials", "true")
			addVaryHeader(w.Header(), "Origin")
		case allowed != "" && originAllowed(origin, allowed):
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Access-Control-Allow-Credentials", "true")
			addVaryHeader(w.Header(), "Origin")
		case r.Method == http.MethodOptions:
			http.Error(w, "origin not allowed", http.StatusForbidden)
			return
		}
		w.Header().Set("Access-Control-Allow-Methods", "POST, GET, OPTIONS, PUT, DELETE")
		w.Header().Set("Access-Control-Allow-Headers", "Accept, Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization, Idempotency-Key, X-Trace-Id")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (s *Server) securityHeadersMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("X-Frame-Options", "DENY")
		h.Set("Referrer-Policy", "no-referrer")
		h.Set("Permissions-Policy", "camera=(), microphone=(), geolocation=()")
		h.Set("Cross-Origin-Opener-Policy", "same-origin")
		h.Set("Content-Security-Policy", "default-src 'none'; frame-ancestors 'none'; base-uri 'none'")
		if !isDevLikeEnv() {
			h.Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
		}
		next.ServeHTTP(w, r)
	})
}

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func (s *Server) handleUpdateConfig(w http.ResponseWriter, r *http.Request) {
	sessionID := r.PathValue("session_id")
	orgID := getOrgID(r)
	_, config, err := s.getEngine(sessionID, orgID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusForbidden)
		return
	}

	var newConfig store.SessionConfig
	if err := json.NewDecoder(r.Body).Decode(&newConfig); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if newConfig.Schema != "" {
		if _, err := gojsonschema.NewSchema(gojsonschema.NewStringLoader(newConfig.Schema)); err != nil {
			http.Error(w, "invalid json schema: "+err.Error(), http.StatusBadRequest)
			return
		}
	}

	s.mu.Lock()
	if newConfig.Schema != "" {
		config.Schema = newConfig.Schema
	}
	if newConfig.EnforcementMode != "" {
		config.EnforcementMode = newConfig.EnforcementMode
	}
	s.mu.Unlock()

	if err := s.Store.SaveConfig(sessionID, config); err != nil {
		http.Error(w, "failed to persist config: "+err.Error(), http.StatusInternalServerError)
		return
	}

	actorID := getActorID(r)
	slog.Info("Session config updated", "session_id", sessionID, "org_id", orgID, "actor_id", actorID, "enforcement_mode", config.EnforcementMode)
	s.auditAdmin(sessionID, actorID, "update_config", map[string]interface{}{"enforcement_mode": config.EnforcementMode})
	s.createNotification(orgID, "config", "Session config updated", fmt.Sprintf("Session %s enforcement_mode=%s", sessionID, config.EnforcementMode))
	_ = s.Store.AppendOrgActivity(orgID, store.JournalEntry{
		Type:      store.EventAdminAction,
		SessionID: sessionID,
		ActorID:   actorID,
		Payload:   map[string]interface{}{"action": "update_config", "enforcement_mode": config.EnforcementMode},
		Timestamp: time.Now().UnixMilli(),
	})
	_ = s.Store.UpsertSearchDocuments([]store.SearchDocument{sessionSearchDocument(orgID, sessionID, config, time.Now().UnixMilli())})

	writeJSON(w, http.StatusOK, config)
}

func (s *Server) checkSnapshotTrigger(sessionID string, engine *core.Engine) {
	now := time.Now().UnixMilli()
	shouldSnap := false
	engine.Lock()
	if engine.MutationCount >= 50 {
		shouldSnap = true
		engine.MutationCount = 0
	} else if now-engine.LastSnapshotTime > 5*60*1000 && engine.MutationCount > 0 {
		shouldSnap = true
		engine.MutationCount = 0
	}
	engine.Unlock()

	if shouldSnap {
		go func() {
			snap, err := engine.ToSnapshot()
			if err != nil {
				return
			}
			s.Store.SaveSnapshot(sessionID, snap)
			engine.Lock()
			engine.LastSnapshotTime = now
			engine.Unlock()
		}()
	}
}

func (s *Server) handleAssertFact(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	sessionID := r.PathValue("session_id")
	orgID := getOrgID(r)
	engine, config, err := s.getEngine(sessionID, orgID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusForbidden)
		return
	}

	var fact core.Fact
	if err := json.NewDecoder(r.Body).Decode(&fact); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if fact.Payload != nil {
		if p, ok := fact.Payload["_provenance"]; ok {
			delete(fact.Payload, "_provenance")
			if fact.Metadata == nil {
				fact.Metadata = make(map[string]interface{})
			}
			fact.Metadata["_provenance"] = p
		}
	}
	applyFactGovernance(&fact, s.loadPolicyControls(orgID))

	if config.Schema != "" {
		schemaLoader := gojsonschema.NewStringLoader(config.Schema)
		documentLoader := gojsonschema.NewGoLoader(fact.Payload)
		result, _ := gojsonschema.Validate(schemaLoader, documentLoader)

		if !result.Valid() {
			go s.Store.IncrementOrgMetric(orgID, "schema_violations", 1)
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

	traceID := ""
	if tid := r.Context().Value(contextKey("trace_id")); tid != nil {
		traceID = tid.(string)
	}
	if err := engine.AssertFact(&fact); err != nil {
		slog.Warn("Fact assertion failed", "session_id", sessionID, "org_id", orgID, "trace_id", traceID, "error", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	s.invalidateSliceCache(sessionID)

	actorID := getActorID(r)
	entry := store.JournalEntry{Type: store.EventAssert, SessionID: sessionID, Fact: &fact, ActorID: actorID}
	if err := s.Store.Append(entry); err != nil {
		slog.Error("Failed to persist journal", "error", err)
		http.Error(w, "failed to persist journal", http.StatusInternalServerError)
		return
	}
	_ = s.Store.AppendOrgActivity(orgID, entry)

	slog.Info("Fact asserted", "session_id", sessionID, "fact_id", fact.ID, "actor_id", actorID, "trace_id", traceID)
	s.createNotification(orgID, "assert", "Fact asserted", fmt.Sprintf("Session %s: %s", sessionID, fact.ID))

	// Metrics
	duration := time.Since(start).Seconds() * 1000
	FactAssertionLatency.Observe(duration)
	go s.Store.IncrementOrgMetric(orgID, "facts_asserted", 1)
	if engine.GetStatus(fact.ID) < core.ConfidenceThreshold {
		go s.Store.IncrementOrgMetric(orgID, "facts_pruned", 1)
	}

	writeJSON(w, http.StatusCreated, fact)
	_ = s.Store.TouchOrgSession(orgID, sessionID, 1)
	s.checkSnapshotTrigger(sessionID, engine)
	s.syncFactSearchDocument(orgID, sessionID, config, &fact, engine.GetStatus(fact.ID))
}

func (s *Server) handleInvalidateRoot(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	sessionID := r.PathValue("session_id")
	orgID := getOrgID(r)
	engine, config, err := s.getEngine(sessionID, orgID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusForbidden)
		return
	}
	id := r.PathValue("id")
	actorID := getActorID(r)
	var body factMutationRequest
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil && err != io.EOF {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	fact, _ := engine.GetFact(id)
	if err := mutationRequiresOverride(fact, getUserRole(r), body.Force); err != nil {
		http.Error(w, err.Error(), http.StatusConflict)
		return
	}

	entry := store.JournalEntry{
		Type:      store.EventInvalidate,
		SessionID: sessionID,
		FactID:    id,
		ActorID:   actorID,
		Payload:   map[string]interface{}{"reason": body.Reason, "force": body.Force},
	}
	if err := s.Store.Append(entry); err != nil {
		slog.Error("Failed to persist journal", "error", err)
		http.Error(w, "failed to persist journal", http.StatusInternalServerError)
		return
	}
	_ = s.Store.AppendOrgActivity(orgID, entry)

	traceID := ""
	if tid := r.Context().Value(contextKey("trace_id")); tid != nil {
		traceID = tid.(string)
	}

	if err := engine.InvalidateRoot(id); err != nil {
		slog.Warn("Invalidation failed", "session_id", sessionID, "fact_id", id, "actor_id", actorID, "trace_id", traceID, "error", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	s.invalidateSliceCache(sessionID)

	slog.Info("Fact invalidated", "session_id", sessionID, "fact_id", id, "actor_id", actorID, "trace_id", traceID)
	s.createNotification(orgID, "invalidate", "Fact invalidated", fmt.Sprintf("Session %s: %s", sessionID, id))

	duration := time.Since(start).Seconds() * 1000
	PruneLatency.Observe(duration)

	writeJSON(w, http.StatusOK, map[string]string{"status": "invalidated"})
	_ = s.Store.TouchOrgSession(orgID, sessionID, 0)
	s.checkSnapshotTrigger(sessionID, engine)
	s.syncSessionSearchDocuments(orgID, sessionID, engine, config)
}

func (s *Server) handleGetFact(w http.ResponseWriter, r *http.Request) {
	sessionID := r.PathValue("session_id")
	orgID := getOrgID(r)
	engine, _, err := s.getEngine(sessionID, orgID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusForbidden)
		return
	}
	id := r.PathValue("id")
	fact, ok := engine.GetFact(id)
	if !ok {
		http.Error(w, "fact not found", http.StatusNotFound)
		return
	}
	fact.ResolvedStatus = engine.GetStatus(id)
	writeJSON(w, http.StatusOK, fact)
}

func (s *Server) handleExplain(w http.ResponseWriter, r *http.Request) {
	sessionID := r.PathValue("session_id")
	orgID := getOrgID(r)
	engine, _, err := s.getEngine(sessionID, orgID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusForbidden)
		return
	}
	id := r.PathValue("id")
	explanations, err := engine.Explain(id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	writeJSON(w, http.StatusOK, explanations)
}

func (s *Server) handleGetImpact(w http.ResponseWriter, r *http.Request) {
	sessionID := r.PathValue("session_id")
	orgID := getOrgID(r)
	engine, _, err := s.getEngine(sessionID, orgID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusForbidden)
		return
	}
	id := r.PathValue("id")
	writeJSON(w, http.StatusOK, engine.GetImpact(id))
}

func (s *Server) handleListFacts(w http.ResponseWriter, r *http.Request) {
	sessionID := r.PathValue("session_id")
	orgID := getOrgID(r)
	engine, _, err := s.getEngine(sessionID, orgID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusForbidden)
		return
	}
	validOnly := r.URL.Query().Get("valid") == "true"
	facts := engine.ListFacts()
	result := make([]*core.Fact, 0)
	for _, fact := range facts {
		status := engine.GetStatus(fact.ID)
		if validOnly && status < core.ConfidenceThreshold {
			continue
		}
		fact.ResolvedStatus = status
		result = append(result, fact)
	}
	writeJSON(w, http.StatusOK, result)
}

func (s *Server) handleAppendHistory(w http.ResponseWriter, r *http.Request) {
	sessionID := r.PathValue("session_id")
	orgID := getOrgID(r)
	storedOrg, err := s.Store.GetSessionOrganization(sessionID)
	if err != nil || storedOrg != orgID {
		http.Error(w, "unauthorized", http.StatusForbidden)
		return
	}

	var entry store.JournalEntry
	if err := json.NewDecoder(r.Body).Decode(&entry); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	entry.ActorID = getActorID(r)
	entry.SessionID = sessionID
	entry.Timestamp = time.Now().UnixMilli()
	if err := s.Store.Append(entry); err != nil {
		http.Error(w, "failed to persist history", http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusCreated, entry)
}

func (s *Server) handleGetHistory(w http.ResponseWriter, r *http.Request) {
	sessionID := r.PathValue("session_id")
	orgID := getOrgID(r)
	storedOrg, err := s.Store.GetSessionOrganization(sessionID)
	if err != nil || storedOrg != orgID {
		http.Error(w, "unauthorized", http.StatusForbidden)
		return
	}
	history, _ := s.Store.GetSessionHistory(sessionID)
	writeJSON(w, http.StatusOK, history)
}

func (s *Server) handleRevalidate(w http.ResponseWriter, r *http.Request) {
	sessionID := r.PathValue("session_id")
	orgID := getOrgID(r)
	engine, _, err := s.getEngine(sessionID, orgID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusForbidden)
		return
	}

	engine.Lock()
	engine.Facts = make(map[string]*core.Fact)
	engine.JustificationSets = make(map[string]*core.JustificationSet)
	engine.ChildrenIndex = make(map[string]map[string]struct{})
	engine.CollapsedRoots = make(map[string]struct{})
	engine.MutationCount = 0
	engine.Unlock()

	history, _ := s.Store.GetSessionHistory(sessionID)
	summary := struct{ Revalidated, Passed, Violations, Pruned int }{}

	for _, entry := range history {
		if entry.Type == store.EventAssert {
			summary.Revalidated++
			fact := entry.Fact
			if engine.AssertFact(fact) == nil {
				if engine.GetStatus(fact.ID) < core.ConfidenceThreshold {
					summary.Pruned++
				} else {
					summary.Passed++
				}
			} else {
				summary.Violations++
			}
		} else if entry.Type == store.EventInvalidate {
			engine.InvalidateRoot(entry.FactID)
		} else if entry.Type == store.EventRetract {
			reason := ""
			if entry.Payload != nil {
				if v, ok := entry.Payload["reason"].(string); ok {
					reason = v
				}
			}
			engine.RetractFact(entry.FactID, reason)
		} else if entry.Type == store.EventReview {
			status := ""
			reason := ""
			reviewedAt := entry.Timestamp
			if entry.Payload != nil {
				if v, ok := entry.Payload["status"].(string); ok {
					status = v
				}
				if v, ok := entry.Payload["reason"].(string); ok {
					reason = v
				}
				if v, ok := entry.Payload["reviewed_at"].(float64); ok {
					reviewedAt = int64(v)
				}
			}
			engine.SetFactReview(entry.FactID, status, reason, reviewedAt)
		}
	}

	go s.Store.IncrementOrgMetric(orgID, "revalidation_runs", 1)
	s.syncSessionSearchDocuments(orgID, sessionID, engine, &store.SessionConfig{})
	writeJSON(w, http.StatusOK, summary)
}

func (s *Server) handleGetSlice(w http.ResponseWriter, r *http.Request) {
	sessionID := r.PathValue("session_id")
	orgID := getOrgID(r)
	engine, _, err := s.getEngine(sessionID, orgID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusForbidden)
		return
	}

	opts := parseSliceSelectionOptions(r.URL.Query())
	cacheKey := opts.cacheKey(sessionID)

	s.mu.RLock()
	cache, ok := s.SliceCache[cacheKey]
	s.mu.RUnlock()

	var validFacts []*core.Fact
	if ok && time.Since(cache.Timestamp) < 30*time.Second {
		CacheRatio.WithLabelValues("hit").Inc()
		validFacts = cache.Facts
	} else {
		CacheRatio.WithLabelValues("miss").Inc()
		validFacts = selectBeliefSlice(engine, opts)
		s.mu.Lock()
		if s.SliceCache == nil {
			s.SliceCache = make(map[string]*SliceCacheEntry)
		}
		s.SliceCache[cacheKey] = &SliceCacheEntry{
			Facts:     validFacts,
			Timestamp: time.Now(),
		}
		s.mu.Unlock()
	}

	if opts.Format == "markdown" {
		w.Header().Set("Content-Type", "text/markdown")
		_, _ = w.Write([]byte(renderBeliefSliceMarkdown(validFacts)))
		return
	}

	writeJSON(w, http.StatusOK, validFacts)
}

func (s *Server) handleEvents(w http.ResponseWriter, r *http.Request) {
	sessionID := r.PathValue("session_id")
	orgID := getOrgID(r)
	engine, _, err := s.getEngine(sessionID, orgID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusForbidden)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	ch := engine.Subscribe()
	defer engine.Unsubscribe(ch)

	flusher, _ := w.(http.Flusher)
	fmt.Fprintf(w, "retry: 1000\n\n")
	flusher.Flush()

	for {
		select {
		case event, ok := <-ch:
			if !ok {
				return
			}
			data, _ := json.Marshal(event)
			fmt.Fprintf(w, "data: %s\n\n", data)
			flusher.Flush()
		case <-r.Context().Done():
			return
		}
	}
}

func (s *Server) handleListSessions(w http.ResponseWriter, r *http.Request) {
	orgID := getOrgID(r)
	cursor := strings.TrimSpace(r.URL.Query().Get("cursor"))
	limit := 50
	if raw := strings.TrimSpace(r.URL.Query().Get("limit")); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil && parsed > 0 && parsed <= 200 {
			limit = parsed
		}
	}

	items, nextCursor, err := s.Store.ListOrgSessions(orgID, cursor, limit)
	if err != nil {
		http.Error(w, "failed to list sessions", http.StatusInternalServerError)
		return
	}
	result := make([]sessionInfo, 0, len(items))
	for _, item := range items {
		info := sessionInfo{
			ID:        item.ID,
			FactCount: item.FactCount,
		}
		if config, err := s.Store.GetConfig(item.ID); err == nil && config != nil {
			info.EnforcementMode = config.EnforcementMode
		}
		result = append(result, info)
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"items":       result,
		"next_cursor": nextCursor,
	})
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"status":  "healthy",
		"version": "0.1.0",
		"uptime":  time.Since(s.StartTime).String(),
	})
}

func (s *Server) handleFullHealth(w http.ResponseWriter, r *http.Request) {
	val := r.Context().Value(userRoleKey)
	role := ""
	if val != nil {
		role = val.(string)
	}

	if role != "admin" {
		http.Error(w, "forbidden: admin role required", http.StatusForbidden)
		return
	}

	s.mu.RLock()
	sessionCount := len(s.Engines)
	sliceCacheCount := len(s.SliceCache)
	s.mu.RUnlock()

	backend := "unknown"
	storageConnected := s.Store != nil
	if reporter, ok := s.Store.(store.HealthReporter); ok {
		backend = reporter.BackendName()
		ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
		defer cancel()
		storageConnected = reporter.Ping(ctx) == nil
	}
	response := map[string]interface{}{
		"status":            "healthy",
		"storage_backend":   backend,
		"storage_connected": storageConnected,
		"badger_connected":  backend == "badger" && storageConnected,
		"ram_sessions":      sessionCount,
		"slice_cache_items": sliceCacheCount,
		"uptime":            time.Since(s.StartTime).String(),
	}
	if backend == "badger" {
		var stat syscall.Statfs_t
		wd, _ := os.Getwd()
		if err := syscall.Statfs(wd, &stat); err == nil {
			diskFree := stat.Bavail * uint64(stat.Bsize)
			diskTotal := stat.Blocks * uint64(stat.Bsize)
			diskUsed := diskTotal - diskFree
			BadgerDiskUsage.Set(float64(diskUsed))
			response["disk"] = map[string]interface{}{
				"free_bytes":    diskFree,
				"total_bytes":   diskTotal,
				"usage_percent": fmt.Sprintf("%.2f%%", (1-float64(diskFree)/float64(diskTotal))*100),
			}
		}
	}

	writeJSON(w, http.StatusOK, response)
}

func (s *Server) handleExport(w http.ResponseWriter, r *http.Request) {
	sessionID := r.PathValue("session_id")
	orgID := getOrgID(r)
	format := r.URL.Query().Get("format")

	engine, _, err := s.getEngine(sessionID, orgID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusForbidden)
		return
	}

	history, _ := s.Store.GetSessionHistory(sessionID)
	usage, _ := s.Store.GetOrgUsage(orgID)
	chainHead, _ := s.Store.GetSessionHistoryChainHead(sessionID)
	ct, filename, data, buildErr := buildSessionExport(sessionID, orgID, engine, history, usage, chainHead, format)
	if buildErr != nil {
		http.Error(w, buildErr.Error(), http.StatusBadRequest)
		return
	}
	w.Header().Set("Content-Type", ct)
	w.Header().Set("Content-Disposition", "attachment; filename="+filename)
	_, _ = w.Write(data)

	// Export access audit (org activity feed)
	_ = s.Store.AppendOrgActivity(orgID, store.JournalEntry{
		Type:      store.EventAdminAction,
		SessionID: sessionID,
		ActorID:   getActorID(r),
		Payload:   map[string]interface{}{"action": "export", "format": format, "filename": filename},
		Timestamp: time.Now().UnixMilli(),
	})
}

func (s *Server) handleGetUsage(w http.ResponseWriter, r *http.Request) {
	orgID := getOrgID(r)
	usage, _ := s.Store.GetOrgUsage(orgID)
	writeJSON(w, http.StatusOK, usage)
}

func (s *Server) checkRateLimit(apiKey string, limit int, window time.Duration) (bool, time.Duration) {
	if limit <= 0 {
		limit = 60
	}
	if window <= 0 {
		window = time.Minute
	}
	now := time.Now()
	windowStart := now.Add(-window)

	limits, err := s.Store.GetRateLimit(apiKey)
	if err != nil && err.Error() != "Key not found" {
		return false, window // Fail closed on DB error
	}

	var valid []time.Time
	var earliest time.Time
	for _, t := range limits {
		if t.After(windowStart) {
			valid = append(valid, t)
			if earliest.IsZero() || t.Before(earliest) {
				earliest = t
			}
		}
	}

	if len(valid) >= limit {
		retryAfter := time.Second
		if !earliest.IsZero() {
			retryAfter = time.Until(earliest.Add(window))
			if retryAfter < time.Second {
				retryAfter = time.Second
			}
		}
		return false, retryAfter
	}

	valid = append(valid, now)
	s.Store.SaveRateLimit(apiKey, valid)
	return true, 0
}

func (s *Server) authMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/health" ||
			strings.HasPrefix(r.URL.Path, "/auth/") ||
			strings.HasPrefix(r.URL.Path, "/v1/auth/") ||
			strings.HasPrefix(r.URL.Path, "/docs/") ||
			strings.HasPrefix(r.URL.Path, "/v1/docs/") ||
			strings.HasPrefix(r.URL.Path, "/v1/legal/") ||
			r.URL.Path == "/v1/invitations/accept" {
			next.ServeHTTP(w, r)
			return
		}

		token := authTokenFromRequest(r)
		if token == "" {
			http.Error(w, "Invalid or expired API key", http.StatusUnauthorized)
			return
		}
		tokenSum := sha256.Sum256([]byte(token))
		tokenHash := hex.EncodeToString(tokenSum[:])

		adminKey := strings.TrimSpace(os.Getenv("VELARIX_API_KEY"))
		if adminKey != "" && token == adminKey && bootstrapAdminEnabled() {
			bootstrapLimit := 300
			if isDevLikeEnv() {
				bootstrapLimit = 5000
			}
			if allowed, retryAfter := s.checkRateLimit("bootstrap:"+tokenHash, bootstrapLimit, time.Minute); !allowed {
				setRateLimitResponseHeaders(w, bootstrapLimit, time.Minute, retryAfter)
				http.Error(w, fmt.Sprintf("rate limit exceeded (%d requests/60s)", bootstrapLimit), http.StatusTooManyRequests)
				return
			}
			ctx := context.WithValue(r.Context(), orgIDKey, "admin")
			ctx = context.WithValue(ctx, actorIDKey, "system")
			ctx = context.WithValue(ctx, userRoleKey, "admin")
			ctx = context.WithValue(ctx, scopesKey, []string{"read", "write", "export", "admin"})
			traceID := r.Header.Get("X-Trace-Id")
			if traceID == "" {
				traceID = fmt.Sprintf("req-%d", time.Now().UnixNano())
			}
			ctx = context.WithValue(ctx, contextKey("trace_id"), traceID)
			w.Header().Set("X-Trace-Id", traceID)
			if !authorizeRequest(r, requiredScopeForRequest(r), []string{"read", "write", "export", "admin"}, "admin") {
				http.Error(w, "forbidden", http.StatusForbidden)
				return
			}
			next.ServeHTTP(w, r.WithContext(ctx))
			return
		}

		if signingKey, err := jwtSigningKey(); err == nil {
			claims := &Claims{}
			tkn, err := jwt.ParseWithClaims(token, claims, func(token *jwt.Token) (interface{}, error) { return signingKey, nil })
			if err == nil && tkn.Valid {
				user, err := s.Store.GetUser(normalizeEmail(claims.Email))
				if err != nil {
					http.Error(w, "User not found", http.StatusUnauthorized)
					return
				}
				if claims.TokenVersion != currentUserTokenVersion(user) {
					http.Error(w, "Invalid or expired API key", http.StatusUnauthorized)
					return
				}
				org, err := s.Store.GetOrganization(user.OrgID)
				if err != nil || org.IsSuspended {
					http.Error(w, "Invalid or expired API key", http.StatusUnauthorized)
					return
				}
				subscription, _ := s.Store.GetBilling(user.OrgID)
				limitRPM, limitWindow := effectiveRateLimitConfig(org, subscription)
				jwtRateKey := fmt.Sprintf("jwt:%s:%d", user.Email, claims.TokenVersion)
				if allowed, retryAfter := s.checkRateLimit(jwtRateKey, limitRPM, limitWindow); !allowed {
					setRateLimitResponseHeaders(w, limitRPM, limitWindow, retryAfter)
					http.Error(w, fmt.Sprintf("rate limit exceeded (%d requests/%ds)", limitRPM, int(limitWindow.Seconds())), http.StatusTooManyRequests)
					return
				}
				scopes := scopesForRole(user.Role)
				if !authorizeRequest(r, requiredScopeForRequest(r), scopes, user.Role) {
					http.Error(w, "forbidden", http.StatusForbidden)
					return
				}
				ctx := context.WithValue(r.Context(), orgIDKey, user.OrgID)
				ctx = context.WithValue(ctx, actorIDKey, user.Email)
				ctx = context.WithValue(ctx, userRoleKey, user.Role)
				ctx = context.WithValue(ctx, userEmailKey, user.Email)
				ctx = context.WithValue(ctx, scopesKey, scopes)
				traceID := r.Header.Get("X-Trace-Id")
				if traceID == "" {
					traceID = fmt.Sprintf("req-%d", time.Now().UnixNano())
				}
				ctx = context.WithValue(ctx, contextKey("trace_id"), traceID)
				w.Header().Set("X-Trace-Id", traceID)
				next.ServeHTTP(w, r.WithContext(ctx))
				return
			}
		}

		emailBytes, err := s.Store.GetAPIKeyOwnerByHash(tokenHash)
		if err != nil {
			// Backwards-compatible fallback for legacy plaintext key indexes.
			emailBytes, err = s.Store.GetAPIKeyOwner(token)
		}
		if err != nil {
			http.Error(w, "Invalid or expired API key", http.StatusUnauthorized)
			return
		}

		user, _ := s.Store.GetUser(string(emailBytes))
		org, err := s.Store.GetOrganization(user.OrgID)
		if err != nil || org.IsSuspended {
			http.Error(w, "Invalid or expired API key", http.StatusUnauthorized)
			return
		}
		keyValid := false
		actorID := "system"
		scopes := []string{"read", "write", "export"}
		for i, k := range user.Keys {
			matches := false
			if k.KeyHash != "" {
				matches = (k.KeyHash == tokenHash) || (k.ID == tokenHash)
			} else if k.ID != "" {
				matches = (k.ID == tokenHash)
			} else if k.Key != "" {
				matches = (k.Key == token)
			}
			if matches && !k.IsRevoked && k.ExpiresAt > time.Now().UnixMilli() {
				keyValid = true
				user.Keys[i].LastUsedAt = time.Now().UnixMilli()
				// Best-effort migration for legacy keys: store only hash + redacted prefix/last4.
				if user.Keys[i].KeyHash == "" {
					user.Keys[i].KeyHash = tokenHash
					user.Keys[i].ID = tokenHash
					if user.Keys[i].KeyPrefix == "" {
						n := 10
						if len(token) < n {
							n = len(token)
						}
						user.Keys[i].KeyPrefix = token[:n]
					}
					if user.Keys[i].KeyLast4 == "" {
						if len(token) <= 4 {
							user.Keys[i].KeyLast4 = token
						} else {
							user.Keys[i].KeyLast4 = token[len(token)-4:]
						}
					}
					if user.Keys[i].Key != "" {
						user.Keys[i].Key = ""
					}
					_ = s.Store.SaveAPIKeyHash(tokenHash, user.Email)
				}
				actorID = k.Label
				if actorID == "" {
					if k.KeyPrefix != "" {
						actorID = k.KeyPrefix
					} else {
						actorID = token[:min(len(token), 8)]
					}
				}
				if len(k.Scopes) > 0 {
					scopes = k.Scopes
				}
				break
			}
		}
		if !keyValid {
			http.Error(w, "Invalid or expired API key", http.StatusUnauthorized)
			return
		}
		subscription, _ := s.Store.GetBilling(user.OrgID)
		limitRPM, limitWindow := effectiveRateLimitConfig(org, subscription)
		if allowed, retryAfter := s.checkRateLimit(tokenHash, limitRPM, limitWindow); !allowed {
			seconds := int(math.Ceil(retryAfter.Seconds()))
			if seconds < 1 {
				seconds = 1
			}
			w.Header().Set("Retry-After", strconv.Itoa(seconds))
			w.Header().Set("X-Velarix-RateLimit-Limit", strconv.Itoa(limitRPM))
			w.Header().Set("X-Velarix-RateLimit-Window", strconv.Itoa(int(limitWindow.Seconds())))
			http.Error(w, fmt.Sprintf("rate limit exceeded (%d requests/%ds)", limitRPM, int(limitWindow.Seconds())), http.StatusTooManyRequests)
			return
		}
		if !authorizeRequest(r, requiredScopeForRequest(r), scopesForRoleCap(user.Role, scopes), user.Role) {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		// per-org api_requests are tracked in orgMetricsMiddleware
		go s.Store.SaveUser(user)

		ctx := context.WithValue(r.Context(), orgIDKey, user.OrgID)
		ctx = context.WithValue(ctx, actorIDKey, actorID)
		ctx = context.WithValue(ctx, userRoleKey, user.Role)
		ctx = context.WithValue(ctx, userEmailKey, user.Email)
		ctx = context.WithValue(ctx, scopesKey, scopesForRoleCap(user.Role, scopes))

		traceID := r.Header.Get("X-Trace-Id")
		if traceID == "" {
			traceID = fmt.Sprintf("req-%d", time.Now().UnixNano())
		}
		ctx = context.WithValue(ctx, contextKey("trace_id"), traceID)

		w.Header().Set("X-Trace-Id", traceID)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func scopesForRole(role string) []string {
	switch role {
	case "admin":
		return []string{"read", "write", "export", "admin"}
	case "auditor":
		return []string{"read", "export"}
	default:
		return []string{"read", "write", "export"}
	}
}

func scopesForRoleCap(role string, scopes []string) []string {
	allowed := map[string]bool{}
	for _, s := range scopesForRole(role) {
		allowed[s] = true
	}
	out := []string{}
	seen := map[string]bool{}
	for _, sc := range scopes {
		sc = strings.TrimSpace(strings.ToLower(sc))
		if sc == "" || !allowed[sc] || seen[sc] {
			continue
		}
		seen[sc] = true
		out = append(out, sc)
	}
	sort.Strings(out)
	if len(out) == 0 {
		out = []string{"read"}
	}
	return out
}

func hasScope(scopes []string, want string) bool {
	want = strings.TrimSpace(strings.ToLower(want))
	if want == "" {
		return true
	}
	for _, sc := range scopes {
		if strings.ToLower(sc) == want {
			return true
		}
	}
	return false
}

func requiredScopeForRequest(r *http.Request) string {
	path := r.URL.Path
	method := r.Method

	// Admin-only surfaces
	if path == "/v1/org/backup" || path == "/v1/org/restore" {
		return "admin"
	}
	if strings.HasPrefix(path, "/v1/keys") {
		return "admin"
	}
	if strings.HasPrefix(path, "/v1/org/invitations") {
		return "admin"
	}
	if strings.HasPrefix(path, "/v1/policies") {
		return "admin"
	}
	if strings.HasPrefix(path, "/v1/org/settings") && method != "GET" {
		return "admin"
	}
	if strings.HasPrefix(path, "/v1/org/access-logs") {
		// Access logs contain sensitive metadata; restrict to auditor/admin via `export` scope.
		return "export"
	}
	if strings.HasPrefix(path, "/v1/org/compliance-export") {
		return "export"
	}
	if path == "/v1/org" && method != "GET" {
		return "admin"
	}
	if path == "/v1/billing/subscription" && method != "GET" {
		return "admin"
	}

	// Exports are sensitive and should be explicitly granted
	if strings.Contains(path, "/export") || strings.Contains(path, "/export-jobs") {
		return "export"
	}

	if method == "GET" || method == "HEAD" || method == "OPTIONS" {
		return "read"
	}
	return "write"
}

func authorizeRequest(r *http.Request, requiredScope string, scopes []string, role string) bool {
	_ = role
	return hasScope(scopes, requiredScope)
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func (s *Server) PerformEvictionSweep() {
	s.mu.Lock()
	defer s.mu.Unlock()

	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	// Base eviction: anything older than 30 mins
	now := time.Now()
	for id, last := range s.LastAccess {
		if now.Sub(last) > 30*time.Minute {
			delete(s.Engines, id)
			delete(s.Configs, id)
			delete(s.Versions, id)
			delete(s.LastAccess, id)
		}
	}

	// Aggressive eviction: if heap > 1GB, evict down to 500MB or LRU
	// (Note: Simplified logic for this architecture)
	if m.HeapAlloc > 1024*1024*1024 && len(s.Engines) > 100 {
		type sessionAge struct {
			id   string
			last time.Time
		}
		var ages []sessionAge
		for id, last := range s.LastAccess {
			ages = append(ages, sessionAge{id, last})
		}
		sort.Slice(ages, func(i, j int) bool {
			return ages[i].last.Before(ages[j].last)
		})

		// Evict oldest 20% of sessions
		toEvict := len(ages) / 5
		for i := 0; i < toEvict; i++ {
			id := ages[i].id
			delete(s.Engines, id)
			delete(s.Configs, id)
			delete(s.Versions, id)
			delete(s.LastAccess, id)
		}
	}
	ActiveSessions.Set(float64(len(s.Engines)))
}

func (s *Server) StartEvictionTicker() {
	ticker := time.NewTicker(5 * time.Minute)
	go func() {
		for range ticker.C {
			s.PerformEvictionSweep()
		}
	}()
}

func (s *Server) StartBackupTicker() {
	ticker := time.NewTicker(24 * time.Hour)
	go func() {
		for range ticker.C {
			f, err := os.Create(fmt.Sprintf("velarix_backup_%d.bak", time.Now().Unix()))
			if err == nil {
				_, err = s.Store.Backup(f)
				f.Close()
				if err != nil {
					slog.Error("Automated backup failed", "error", err)
				} else {
					slog.Info("Automated backup completed")
				}
			}
		}
	}()
}

func (s *Server) StartRetentionTicker() {
	intervalMinutes := 60
	if raw := strings.TrimSpace(os.Getenv("VELARIX_RETENTION_SWEEP_INTERVAL_MINUTES")); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil && parsed > 0 {
			intervalMinutes = parsed
		}
	}

	ticker := time.NewTicker(time.Duration(intervalMinutes) * time.Minute)
	go func() {
		for range ticker.C {
			report, err := s.Store.EnforceRetention(time.Now())
			if err != nil {
				slog.Error("Retention sweep failed", "error", err)
				continue
			}
			if report != nil && (report.ActivityDeleted > 0 || report.AccessLogsDeleted > 0 || report.NotificationsDeleted > 0) {
				slog.Info("Retention sweep completed",
					"activity_deleted", report.ActivityDeleted,
					"access_logs_deleted", report.AccessLogsDeleted,
					"notifications_deleted", report.NotificationsDeleted,
				)
			}
		}
	}()
}

func (s *Server) handleBackup(w http.ResponseWriter, r *http.Request) {
	val := r.Context().Value(userRoleKey)
	role := ""
	if val != nil {
		role = val.(string)
	}
	if role != "admin" {
		http.Error(w, "forbidden: admin role required", http.StatusForbidden)
		return
	}

	buf := &bytes.Buffer{}
	if _, err := s.Store.Backup(buf); err != nil {
		slog.Error("Backup failed", "error", err)
		http.Error(w, "backup failed", http.StatusInternalServerError)
		return
	}
	hash := sha256.Sum256(buf.Bytes())
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Disposition", "attachment; filename=velarix_backup.bak")
	if _, err := w.Write(buf.Bytes()); err != nil {
		slog.Error("Backup write failed", "error", err)
		return
	}
	s.auditAdmin("admin", getActorID(r), "backup", map[string]interface{}{"hash": fmt.Sprintf("%x", hash[:])})
}

func (s *Server) handleRestore(w http.ResponseWriter, r *http.Request) {
	val := r.Context().Value(userRoleKey)
	role := ""
	if val != nil {
		role = val.(string)
	}
	if role != "admin" {
		http.Error(w, "forbidden: admin role required", http.StatusForbidden)
		return
	}

	if err := s.Store.Restore(r.Body); err != nil {
		http.Error(w, "Restore failed: "+err.Error(), http.StatusInternalServerError)
		return
	}
	s.auditAdmin("admin", getActorID(r), "restore", map[string]interface{}{})
	writeJSON(w, http.StatusOK, map[string]string{"status": "restored"})
}

type statusRecorder struct {
	http.ResponseWriter
	status int
	body   []byte
}

func (r *statusRecorder) WriteHeader(status int) {
	r.status = status
	r.ResponseWriter.WriteHeader(status)
}

func (r *statusRecorder) Write(b []byte) (int, error) {
	if r.status >= 400 {
		r.body = append(r.body, b...)
	}
	return r.ResponseWriter.Write(b)
}

func (s *Server) metricsAndLoggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rec := &statusRecorder{ResponseWriter: w, status: 200}
		start := time.Now()

		ctx, span := otel.Tracer("velarix/http").Start(r.Context(), r.Method+" "+r.URL.Path)
		defer span.End()

		next.ServeHTTP(rec, r.WithContext(ctx))

		duration := time.Since(start).Seconds()

		// SLOSuccessRate (99.9% target)
		if rec.status >= 500 {
			SLOSuccessRate.WithLabelValues("failure").Inc()
			slog.Error("Request failed", "method", r.Method, "path", r.URL.Path, "status", rec.status, "duration", duration, "error", strings.TrimSpace(string(rec.body)))
		} else {
			SLOSuccessRate.WithLabelValues("success").Inc()
			if rec.status >= 400 {
				slog.Warn("Client error", "method", r.Method, "path", r.URL.Path, "status", rec.status, "duration", duration, "error", strings.TrimSpace(string(rec.body)))
			}
		}

		APIRequests.WithLabelValues(r.URL.Path, fmt.Sprintf("%d", rec.status)).Inc()
	})
}

// orgMetricsMiddleware records per-organization usage breakdowns after auth has populated org context.
func (s *Server) orgMetricsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rec := &statusRecorder{ResponseWriter: w, status: 200}
		next.ServeHTTP(rec, r)

		orgID := getOrgID(r)
		if orgID == "" {
			return
		}
		endpoint := r.Pattern
		if endpoint == "" {
			endpoint = r.URL.Path
		}
		_ = s.Store.IncrementOrgMetric(orgID, "api_requests", 1)
		_ = s.Store.IncOrgRequestBreakdown(orgID, endpoint, rec.status, 1)
	})
}

func (s *Server) accessLogMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rec := &statusRecorder{ResponseWriter: w, status: 200}
		start := time.Now()
		next.ServeHTTP(rec, r)

		orgID := getOrgID(r)
		if orgID == "" {
			return
		}
		actorID := getActorID(r)
		role := getUserRole(r)
		traceID, _ := r.Context().Value(contextKey("trace_id")).(string)

		ip := clientIP(r)
		ua := r.UserAgent()

		endpoint := r.Pattern
		if endpoint == "" {
			endpoint = r.URL.Path
		}
		_ = s.Store.AppendAccessLog(orgID, store.AccessLogEntry{
			ID:         fmt.Sprintf("al_%d", time.Now().UnixNano()),
			ActorID:    actorID,
			ActorRole:  role,
			Method:     r.Method,
			Pattern:    endpoint,
			Path:       r.URL.Path,
			Status:     rec.status,
			DurationMs: time.Since(start).Milliseconds(),
			TraceID:    traceID,
			IP:         ip,
			UserAgent:  ua,
			CreatedAt:  time.Now().UnixMilli(),
		})
	})
}

type idempotencyRecorder struct {
	http.ResponseWriter
	status      int
	maxBodySize int
	body        bytes.Buffer
}

func (r *idempotencyRecorder) WriteHeader(status int) {
	r.status = status
	r.ResponseWriter.WriteHeader(status)
}

func (r *idempotencyRecorder) Write(b []byte) (int, error) {
	if r.maxBodySize > 0 && r.body.Len() < r.maxBodySize {
		remaining := r.maxBodySize - r.body.Len()
		if remaining > 0 {
			if len(b) <= remaining {
				r.body.Write(b)
			} else {
				r.body.Write(b[:remaining])
			}
		}
	}
	return r.ResponseWriter.Write(b)
}

func (s *Server) idempotencyMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case "POST", "PUT", "PATCH", "DELETE":
		default:
			next.ServeHTTP(w, r)
			return
		}

		key := strings.TrimSpace(r.Header.Get("Idempotency-Key"))
		if key == "" {
			next.ServeHTTP(w, r)
			return
		}

		orgID := getOrgID(r)
		if orgID == "" {
			next.ServeHTTP(w, r)
			return
		}

		sum := sha256.Sum256([]byte(r.Method + "|" + r.URL.Path + "|" + key))
		keyHash := hex.EncodeToString(sum[:])
		maxAge := idempotencyReplayWindow()

		if rec, err := s.Store.GetIdempotency(orgID, keyHash, maxAge); err == nil && rec != nil {
			if rec.ContentType != "" {
				w.Header().Set("Content-Type", rec.ContentType)
			}
			for hk, hv := range rec.Headers {
				if hk != "" && hv != "" {
					w.Header().Set(hk, hv)
				}
			}
			w.Header().Set("X-Idempotency-Replay", "true")
			w.Header().Set("X-Idempotency-Key", key)
			w.WriteHeader(rec.Status)
			if len(rec.Body) > 0 {
				_, _ = w.Write(rec.Body)
			}
			return
		}

		rec := &idempotencyRecorder{ResponseWriter: w, status: 200, maxBodySize: 1024 * 1024}
		next.ServeHTTP(rec, r)

		ct := rec.Header().Get("Content-Type")
		if persistIdempotencyResponse(rec.status, ct, rec.body.Len()) {
			_ = s.Store.SaveIdempotency(orgID, keyHash, &store.IdempotencyRecord{
				Status:      rec.status,
				ContentType: ct,
				Body:        rec.body.Bytes(),
				Headers:     capturedIdempotencyHeaders(rec.Header()),
				CreatedAt:   time.Now().UnixMilli(),
			})
		}
	})
}

func (s *Server) handleExplainReasoning(w http.ResponseWriter, r *http.Request) {
	sessionID := r.PathValue("session_id")
	orgID := getOrgID(r)

	factID := r.URL.Query().Get("fact_id")
	timestampStr := r.URL.Query().Get("timestamp")
	counterfactualFactID := r.URL.Query().Get("counterfactual_fact_id")

	var engine *core.Engine
	var err error
	var explanationTimestamp int64

	if timestampStr != "" {
		// Parse ISO8601 timestamp with nano precision support
		parsedTime, parseErr := time.Parse(time.RFC3339Nano, timestampStr)
		if parseErr != nil {
			http.Error(w, "invalid timestamp format, use ISO8601 (RFC3339)", http.StatusBadRequest)
			return
		}
		explanationTimestamp = parsedTime.UnixMilli()

		// Replay session state up to that point
		history, histErr := s.Store.GetSessionHistoryBefore(sessionID, explanationTimestamp)
		if histErr != nil {
			http.Error(w, "failed to retrieve history: "+histErr.Error(), http.StatusInternalServerError)
			return
		}

		// Build a temporary engine from that point in time
		engine = core.NewEngine()
		for _, entry := range history {
			switch entry.Type {
			case store.EventAssert:
				if entry.Fact != nil {
					engine.AssertFact(entry.Fact)
				}
			case store.EventInvalidate:
				engine.InvalidateRoot(entry.FactID)
			case store.EventRetract:
				reason := ""
				if entry.Payload != nil {
					if v, ok := entry.Payload["reason"].(string); ok {
						reason = v
					}
				}
				engine.RetractFact(entry.FactID, reason)
			case store.EventReview:
				status := ""
				reason := ""
				reviewedAt := entry.Timestamp
				if entry.Payload != nil {
					if v, ok := entry.Payload["status"].(string); ok {
						status = v
					}
					if v, ok := entry.Payload["reason"].(string); ok {
						reason = v
					}
					if v, ok := entry.Payload["reviewed_at"].(float64); ok {
						reviewedAt = int64(v)
					}
				}
				engine.SetFactReview(entry.FactID, status, reason, reviewedAt)
			}
		}
	} else {
		explanationTimestamp = time.Now().UnixMilli()
		var getErr error
		engine, _, getErr = s.getEngine(sessionID, orgID)
		if getErr != nil {
			http.Error(w, getErr.Error(), http.StatusForbidden)
			return
		}
	}

	if factID == "" {
		facts := engine.ListFacts()
		if len(facts) == 0 {
			http.Error(w, "no facts in session", http.StatusNotFound)
			return
		}
		// Use the last asserted fact
		factID = facts[len(facts)-1].ID
	}

	explanation, err := engine.ExplainReasoning(factID, counterfactualFactID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	explanation.SessionID = sessionID
	explanation.Timestamp = explanationTimestamp

	// Serialize and store immutably
	contentBytes, err := json.Marshal(explanation)
	if err != nil {
		http.Error(w, "failed to serialize explanation", http.StatusInternalServerError)
		return
	}

	s.Store.SaveExplanation(sessionID, contentBytes)

	writeJSON(w, http.StatusOK, explanation)
}

func (s *Server) handleGetExplanations(w http.ResponseWriter, r *http.Request) {
	sessionID := r.PathValue("session_id")
	orgID := getOrgID(r)

	// Verify org ownership
	storedOrg, err := s.Store.GetSessionOrganization(sessionID)
	if err != nil || storedOrg != orgID {
		http.Error(w, "unauthorized", http.StatusForbidden)
		return
	}

	records, err := s.Store.GetSessionExplanations(sessionID)
	if err != nil {
		http.Error(w, "failed to retrieve explanations: "+err.Error(), http.StatusInternalServerError)
		return
	}

	if records == nil {
		records = []store.ExplanationRecord{}
	}

	writeJSON(w, http.StatusOK, records)
}

func (s *Server) Routes() http.Handler {
	mux := http.NewServeMux()

	// Documentation & Version-Agnostic Metadata
	mux.HandleFunc("GET /docs/openapi.yaml", func(w http.ResponseWriter, r *http.Request) { http.ServeFile(w, r, "docs/openapi.yaml") })
	mux.HandleFunc("GET /docs/postman.json", func(w http.ResponseWriter, r *http.Request) { http.ServeFile(w, r, "docs/postman.json") })
	mux.Handle("GET /metrics", promhttp.Handler())
	mux.HandleFunc("GET /health", s.handleHealth)
	mux.HandleFunc("GET /health/full", s.handleFullHealth)

	// Admin Management Routes
	mux.HandleFunc("GET /v1/org/backup", s.handleBackup)
	mux.HandleFunc("POST /v1/org/restore", s.handleRestore)

	// V1 API Routes
	mux.HandleFunc("GET /v1/me", s.handleMe)
	mux.HandleFunc("POST /v1/me/change-password", s.handleChangePassword)
	mux.HandleFunc("GET /v1/me/onboarding", s.handleGetOnboarding)
	mux.HandleFunc("POST /v1/me/onboarding", s.handleUpdateOnboarding)

	mux.HandleFunc("GET /v1/org", s.handleGetOrg)
	mux.HandleFunc("PATCH /v1/org", s.handlePatchOrg)
	mux.HandleFunc("GET /v1/org/settings", s.handleGetOrgSettings)
	mux.HandleFunc("PATCH /v1/org/settings", s.handlePatchOrgSettings)
	mux.HandleFunc("GET /v1/org/usage", s.handleGetUsage)
	mux.HandleFunc("GET /v1/org/usage/timeseries", s.handleGetUsageTimeseries)
	mux.HandleFunc("GET /v1/org/usage/breakdown", s.handleGetUsageBreakdown)
	mux.HandleFunc("GET /v1/org/sessions", s.handleListOrgSessions)
	mux.HandleFunc("POST /v1/org/sessions", s.handleCreateSession)
	mux.HandleFunc("PATCH /v1/org/sessions/{id}", s.handlePatchSession)
	mux.HandleFunc("DELETE /v1/org/sessions/{id}", s.handleDeleteSession)
	mux.HandleFunc("GET /v1/org/activity", s.handleOrgActivity)
	mux.HandleFunc("GET /v1/org/access-logs", s.handleListAccessLogs)
	mux.HandleFunc("GET /v1/org/search", s.handleOrgSearch)
	mux.HandleFunc("GET /v1/org/decisions", s.handleListOrgDecisions)
	mux.HandleFunc("GET /v1/org/decisions/blocked", s.handleListBlockedOrgDecisions)
	mux.HandleFunc("GET /v1/org/compliance-export", s.handleComplianceExport)
	mux.HandleFunc("GET /v1/org/notifications", s.handleListNotifications)
	mux.HandleFunc("POST /v1/org/notifications/{id}/read", s.handleMarkNotificationRead)
	mux.HandleFunc("GET /v1/org/integrations", s.handleListIntegrations)
	mux.HandleFunc("POST /v1/org/integrations", s.handleCreateIntegration)
	mux.HandleFunc("PATCH /v1/org/integrations/{id}", s.handlePatchIntegration)
	mux.HandleFunc("DELETE /v1/org/integrations/{id}", s.handleDeleteIntegration)
	mux.HandleFunc("GET /v1/org/users", s.handleListOrgUsers)
	mux.HandleFunc("GET /v1/org/invitations", s.handleListInvitations)
	mux.HandleFunc("POST /v1/org/invitations", s.handleCreateInvitation)
	mux.HandleFunc("POST /v1/org/invitations/{id}/revoke", s.handleRevokeInvitation)
	mux.HandleFunc("POST /v1/invitations/accept", s.handleAcceptInvitation)
	mux.HandleFunc("GET /v1/billing/subscription", s.handleGetBilling)
	mux.HandleFunc("PATCH /v1/billing/subscription", s.handlePatchBilling)
	mux.HandleFunc("GET /v1/support/tickets", s.handleListTickets)
	mux.HandleFunc("POST /v1/support/tickets", s.handleCreateTicket)
	mux.HandleFunc("PATCH /v1/support/tickets/{id}", s.handlePatchTicket)
	mux.HandleFunc("GET /v1/policies", s.handleListPolicies)
	mux.HandleFunc("POST /v1/policies", s.handleCreatePolicy)
	mux.HandleFunc("PATCH /v1/policies/{id}", s.handlePatchPolicy)
	mux.HandleFunc("DELETE /v1/policies/{id}", s.handleDeletePolicy)
	mux.HandleFunc("GET /v1/legal/terms", s.handleLegalTerms)
	mux.HandleFunc("GET /v1/legal/privacy", s.handleLegalPrivacy)

	mux.HandleFunc("GET /v1/docs/pages", s.handleListDocsPages)
	mux.HandleFunc("GET /v1/docs/pages/{slug}", s.handleGetDocsPage)

	mux.HandleFunc("POST /v1/auth/register", s.handleRegister)
	mux.HandleFunc("POST /auth/register", s.handleRegister)
	mux.HandleFunc("POST /v1/auth/login", s.handleLogin)
	mux.HandleFunc("POST /auth/login", s.handleLogin)
	mux.HandleFunc("POST /v1/auth/logout", s.handleLogout)
	mux.HandleFunc("POST /auth/logout", s.handleLogout)
	mux.HandleFunc("POST /v1/auth/reset-request", s.handleResetRequest)
	mux.HandleFunc("POST /auth/reset-request", s.handleResetRequest)
	mux.HandleFunc("POST /v1/auth/reset-confirm", s.handleResetConfirm)
	mux.HandleFunc("POST /auth/reset-confirm", s.handleResetConfirm)
	mux.HandleFunc("POST /v1/keys/generate", s.handleGenerateKey)
	mux.HandleFunc("GET /v1/keys", s.handleListKeys)
	mux.HandleFunc("DELETE /v1/keys/{key}", s.handleRevokeKey)
	mux.HandleFunc("POST /v1/keys/{key}/rotate", s.handleRotateKey)
	mux.HandleFunc("GET /v1/sessions", s.handleListSessions)
	mux.HandleFunc("GET /v1/s/{session_id}/summary", s.handleGetSessionSummary)

	// Session-scoped V1 Routes
	mux.HandleFunc("POST /v1/s/{session_id}/facts", s.handleAssertFact)
	mux.HandleFunc("POST /v1/s/{session_id}/percepts", s.handleRecordPerception)
	mux.HandleFunc("POST /v1/s/{session_id}/facts/{id}/invalidate", s.handleInvalidateRoot)
	mux.HandleFunc("POST /v1/s/{session_id}/facts/{id}/retract", s.handleRetractFact)
	mux.HandleFunc("POST /v1/s/{session_id}/facts/{id}/review", s.handleReviewFact)
	mux.HandleFunc("GET /v1/s/{session_id}/facts/{id}", s.handleGetFact)
	mux.HandleFunc("GET /v1/s/{session_id}/facts/{id}/why", s.handleExplain)
	mux.HandleFunc("GET /v1/s/{session_id}/facts/{id}/impact", s.handleGetImpact)
	mux.HandleFunc("GET /v1/s/{session_id}/facts", s.handleListFacts)
	mux.HandleFunc("GET /v1/s/{session_id}/semantic-search", s.handleSemanticSearch)
	mux.HandleFunc("POST /v1/s/{session_id}/consistency-check", s.handleConsistencyCheck)
	mux.HandleFunc("POST /v1/s/{session_id}/config", s.handleUpdateConfig)
	mux.HandleFunc("POST /v1/s/{session_id}/revalidate", s.handleRevalidate)
	mux.HandleFunc("GET /v1/s/{session_id}/export", s.handleExport)
	mux.HandleFunc("POST /v1/s/{session_id}/export-jobs", s.handleCreateExportJob)
	mux.HandleFunc("GET /v1/s/{session_id}/export-jobs", s.handleListExportJobs)
	mux.HandleFunc("GET /v1/s/{session_id}/export-jobs/{id}", s.handleGetExportJob)
	mux.HandleFunc("GET /v1/s/{session_id}/export-jobs/{id}/download", s.handleDownloadExportJob)
	mux.HandleFunc("GET /v1/s/{session_id}/history", s.handleGetHistory)
	mux.HandleFunc("POST /v1/s/{session_id}/history", s.handleAppendHistory)
	mux.HandleFunc("GET /v1/s/{session_id}/slice", s.handleGetSlice)
	mux.HandleFunc("GET /v1/s/{session_id}/events", s.handleEvents)
	mux.HandleFunc("GET /v1/s/{session_id}/graph", s.handleGetGraph)
	mux.HandleFunc("GET /v1/s/{session_id}/explain", s.handleExplainReasoning)
	mux.HandleFunc("GET /v1/s/{session_id}/explanations", s.handleGetExplanations)
	mux.HandleFunc("POST /v1/s/{session_id}/reasoning-chains", s.handleRecordReasoningChain)
	mux.HandleFunc("GET /v1/s/{session_id}/reasoning-chains", s.handleListReasoningChains)
	mux.HandleFunc("POST /v1/s/{session_id}/reasoning-chains/{chain_id}/verify", s.handleVerifyReasoningChain)
	mux.HandleFunc("GET /v1/s/{session_id}/history/page", s.handleHistoryPage)
	mux.HandleFunc("GET /v1/s/{session_id}/config", s.handleGetConfig)
	mux.HandleFunc("POST /v1/s/{session_id}/decisions", s.handleCreateDecision)
	mux.HandleFunc("GET /v1/s/{session_id}/decisions", s.handleListSessionDecisions)
	mux.HandleFunc("GET /v1/s/{session_id}/decisions/{decision_id}", s.handleGetDecision)
	mux.HandleFunc("POST /v1/s/{session_id}/decisions/{decision_id}/recompute", s.handleRecomputeDecision)
	mux.HandleFunc("POST /v1/s/{session_id}/decisions/{decision_id}/execute-check", s.handleExecuteDecisionCheck)
	mux.HandleFunc("POST /v1/s/{session_id}/decisions/{decision_id}/execute", s.handleExecuteDecision)
	mux.HandleFunc("GET /v1/s/{session_id}/decisions/{decision_id}/lineage", s.handleDecisionLineage)
	mux.HandleFunc("GET /v1/s/{session_id}/decisions/{decision_id}/why-blocked", s.handleDecisionWhyBlocked)

	var h http.Handler = mux

	if s.LiteMode {
		mux = http.NewServeMux()
		mux.HandleFunc("GET /docs/openapi.yaml", func(w http.ResponseWriter, r *http.Request) { http.ServeFile(w, r, "docs/openapi.yaml") })
		mux.HandleFunc("GET /docs/postman.json", func(w http.ResponseWriter, r *http.Request) { http.ServeFile(w, r, "docs/postman.json") })
		mux.Handle("GET /metrics", promhttp.Handler())
		mux.HandleFunc("GET /health", s.handleHealth)
		mux.HandleFunc("GET /health/full", s.handleFullHealth)

		mux.HandleFunc("POST /v1/s/{session_id}/facts", s.handleAssertFact)
		mux.HandleFunc("POST /v1/s/{session_id}/percepts", s.handleRecordPerception)
		mux.HandleFunc("POST /v1/s/{session_id}/facts/{id}/invalidate", s.handleInvalidateRoot)
		mux.HandleFunc("POST /v1/s/{session_id}/facts/{id}/retract", s.handleRetractFact)
		mux.HandleFunc("POST /v1/s/{session_id}/facts/{id}/review", s.handleReviewFact)
		mux.HandleFunc("GET /v1/s/{session_id}/facts/{id}", s.handleGetFact)
		mux.HandleFunc("GET /v1/s/{session_id}/facts/{id}/why", s.handleExplain)
		mux.HandleFunc("GET /v1/s/{session_id}/facts/{id}/impact", s.handleGetImpact)
		mux.HandleFunc("GET /v1/s/{session_id}/facts", s.handleListFacts)
		mux.HandleFunc("GET /v1/s/{session_id}/semantic-search", s.handleSemanticSearch)
		mux.HandleFunc("POST /v1/s/{session_id}/consistency-check", s.handleConsistencyCheck)
		mux.HandleFunc("POST /v1/s/{session_id}/config", s.handleUpdateConfig)
		mux.HandleFunc("POST /v1/s/{session_id}/revalidate", s.handleRevalidate)
		mux.HandleFunc("GET /v1/s/{session_id}/export", s.handleExport)
		mux.HandleFunc("POST /v1/s/{session_id}/export-jobs", s.handleCreateExportJob)
		mux.HandleFunc("GET /v1/s/{session_id}/export-jobs", s.handleListExportJobs)
		mux.HandleFunc("GET /v1/s/{session_id}/export-jobs/{id}", s.handleGetExportJob)
		mux.HandleFunc("GET /v1/s/{session_id}/export-jobs/{id}/download", s.handleDownloadExportJob)
		mux.HandleFunc("GET /v1/s/{session_id}/history", s.handleGetHistory)
		mux.HandleFunc("POST /v1/s/{session_id}/history", s.handleAppendHistory)
		mux.HandleFunc("GET /v1/s/{session_id}/slice", s.handleGetSlice)
		mux.HandleFunc("GET /v1/s/{session_id}/events", s.handleEvents)
		mux.HandleFunc("GET /v1/s/{session_id}/graph", s.handleGetGraph)
		mux.HandleFunc("GET /v1/s/{session_id}/explain", s.handleExplainReasoning)
		mux.HandleFunc("GET /v1/s/{session_id}/explanations", s.handleGetExplanations)
		mux.HandleFunc("POST /v1/s/{session_id}/reasoning-chains", s.handleRecordReasoningChain)
		mux.HandleFunc("GET /v1/s/{session_id}/reasoning-chains", s.handleListReasoningChains)
		mux.HandleFunc("POST /v1/s/{session_id}/reasoning-chains/{chain_id}/verify", s.handleVerifyReasoningChain)
		mux.HandleFunc("GET /v1/s/{session_id}/history/page", s.handleHistoryPage)
		mux.HandleFunc("GET /v1/s/{session_id}/config", s.handleGetConfig)
		mux.HandleFunc("POST /v1/s/{session_id}/decisions", s.handleCreateDecision)
		mux.HandleFunc("GET /v1/s/{session_id}/decisions", s.handleListSessionDecisions)
		mux.HandleFunc("GET /v1/s/{session_id}/decisions/{decision_id}", s.handleGetDecision)
		mux.HandleFunc("POST /v1/s/{session_id}/decisions/{decision_id}/recompute", s.handleRecomputeDecision)
		mux.HandleFunc("POST /v1/s/{session_id}/decisions/{decision_id}/execute-check", s.handleExecuteDecisionCheck)
		mux.HandleFunc("POST /v1/s/{session_id}/decisions/{decision_id}/execute", s.handleExecuteDecision)
		mux.HandleFunc("GET /v1/s/{session_id}/decisions/{decision_id}/lineage", s.handleDecisionLineage)
		mux.HandleFunc("GET /v1/s/{session_id}/decisions/{decision_id}/why-blocked", s.handleDecisionWhyBlocked)
		h = mux
	}

	h = s.orgMetricsMiddleware(h)
	h = s.accessLogMiddleware(h)
	h = s.writeLimitMiddleware(h)
	h = s.idempotencyMiddleware(h)
	h = s.authMiddleware(h)
	h = s.securityHeadersMiddleware(h)
	h = s.enableCORS(h)
	h = s.metricsAndLoggingMiddleware(h)
	return h
}
