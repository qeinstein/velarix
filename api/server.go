package api

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/csv"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log/slog"
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
	"github.com/jung-kurt/gofpdf"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/xeipuuv/gojsonschema"
	"go.opentelemetry.io/otel"
)

type contextKey string

const orgIDKey contextKey = "org_id"
const actorIDKey contextKey = "actor_id"
const userRoleKey contextKey = "user_role"
const userEmailKey contextKey = "user_email"

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
	LastAccess map[string]time.Time
	SliceCache map[string]*SliceCacheEntry
	Store      *store.BadgerStore
	StartTime  time.Time
}

func (s *Server) getEngine(sessionID string, orgID string) (*core.Engine, *store.SessionConfig, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.LastAccess == nil {
		s.LastAccess = make(map[string]time.Time)
	}
	s.LastAccess[sessionID] = time.Now()

	engine, ok := s.Engines[sessionID]
	config := s.Configs[sessionID]

	if ok {
		storedOrg, err := s.Store.GetSessionOrganization(sessionID)
		if err == nil && storedOrg != orgID {
			return nil, nil, fmt.Errorf("unauthorized: session belongs to a different organization")
		}
		return engine, config, nil
	}

	storedOrg, err := s.Store.GetSessionOrganization(sessionID)
	if err != nil {
		if err := s.Store.SetSessionOrganization(sessionID, orgID); err != nil {
			return nil, nil, fmt.Errorf("failed to link session to org: %v", err)
		}
		go s.Store.IncrementMetric(orgID, "sessions_created")
	} else if storedOrg != orgID {
		return nil, nil, fmt.Errorf("unauthorized: session belongs to a different organization")
	}

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

	history, err := s.Store.GetSessionHistory(sessionID)
	if err == nil {
		for _, entry := range history {
			// Only replay events AFTER the snapshot
			if entry.Timestamp <= lastSnapshotTS {
				continue
			}
			switch entry.Type {
			case store.EventAssert:
				engine.AssertFact(entry.Fact)
			case store.EventInvalidate:
				engine.InvalidateRoot(entry.FactID)
			}
		}
	}

	s.Engines[sessionID] = engine
	s.Configs[sessionID] = config
	ActiveSessions.Set(float64(len(s.Engines)))

	return engine, config, nil
}

func (s *Server) enableCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "POST, GET, OPTIONS, PUT, DELETE")
		w.Header().Set("Access-Control-Allow-Headers", "Accept, Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization")
		if r.Method == "OPTIONS" {
			return
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

	if config.Schema != "" {
		schemaLoader := gojsonschema.NewStringLoader(config.Schema)
		documentLoader := gojsonschema.NewGoLoader(fact.Payload)
		result, _ := gojsonschema.Validate(schemaLoader, documentLoader)

		if !result.Valid() {
			go s.Store.IncrementMetric(orgID, "schema_violations")
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

	s.mu.Lock()
	if s.SliceCache != nil {
		delete(s.SliceCache, sessionID)
	}
	s.mu.Unlock()

	actorID := getActorID(r)
	entry := store.JournalEntry{Type: store.EventAssert, SessionID: sessionID, Fact: &fact, ActorID: actorID}
	if err := s.Store.Append(entry); err != nil {
		slog.Error("Failed to persist journal", "error", err)
		http.Error(w, "failed to persist journal", http.StatusInternalServerError)
		return
	}

	slog.Info("Fact asserted", "session_id", sessionID, "fact_id", fact.ID, "actor_id", actorID, "trace_id", traceID)

	// Metrics
	duration := time.Since(start).Seconds() * 1000
	FactAssertionLatency.Observe(duration)
	go s.Store.IncrementMetric(orgID, "facts_asserted")
	if engine.GetStatus(fact.ID) < core.ConfidenceThreshold {
		go s.Store.IncrementMetric(orgID, "facts_pruned")
	}

	writeJSON(w, http.StatusCreated, fact)
	s.checkSnapshotTrigger(sessionID, engine)
}

func (s *Server) handleInvalidateRoot(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	sessionID := r.PathValue("session_id")
	orgID := getOrgID(r)
	engine, _, err := s.getEngine(sessionID, orgID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusForbidden)
		return
	}
	id := r.PathValue("id")
	actorID := getActorID(r)

	entry := store.JournalEntry{Type: store.EventInvalidate, SessionID: sessionID, FactID: id, ActorID: actorID}
	if err := s.Store.Append(entry); err != nil {
		slog.Error("Failed to persist journal", "error", err)
		http.Error(w, "failed to persist journal", http.StatusInternalServerError)
		return
	}

	traceID := ""
	if tid := r.Context().Value(contextKey("trace_id")); tid != nil {
		traceID = tid.(string)
	}

	if err := engine.InvalidateRoot(id); err != nil {
		slog.Warn("Invalidation failed", "session_id", sessionID, "fact_id", id, "actor_id", actorID, "trace_id", traceID, "error", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	s.mu.Lock()
	if s.SliceCache != nil {
		delete(s.SliceCache, sessionID)
	}
	s.mu.Unlock()

	slog.Info("Fact invalidated", "session_id", sessionID, "fact_id", id, "actor_id", actorID, "trace_id", traceID)

	duration := time.Since(start).Seconds() * 1000
	PruneLatency.Observe(duration)

	writeJSON(w, http.StatusOK, map[string]string{"status": "invalidated"})
	s.checkSnapshotTrigger(sessionID, engine)
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
		}
	}

	go s.Store.IncrementMetric(orgID, "revalidation_runs")
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

	format := r.URL.Query().Get("format")

	s.mu.RLock()
	cache, ok := s.SliceCache[sessionID]
	s.mu.RUnlock()

	var validFacts []*core.Fact
	if ok && time.Since(cache.Timestamp) < 30*time.Second {
		CacheRatio.WithLabelValues("hit").Inc()
		validFacts = cache.Facts
	} else {
		CacheRatio.WithLabelValues("miss").Inc()
		facts := engine.ListFacts()
		maxFacts := 0
		if mf := r.URL.Query().Get("max_facts"); mf != "" {
			if v, err := strconv.Atoi(mf); err == nil && v > 0 {
				maxFacts = v
			}
		}
		for _, f := range facts {
			if engine.GetStatus(f.ID) >= core.ConfidenceThreshold {
				validFacts = append(validFacts, f)
			}
		}
		if maxFacts > 0 && len(validFacts) > maxFacts {
			validFacts = validFacts[:maxFacts]
		}
		s.mu.Lock()
		if s.SliceCache == nil {
			s.SliceCache = make(map[string]*SliceCacheEntry)
		}
		s.SliceCache[sessionID] = &SliceCacheEntry{
			Facts:     validFacts,
			Timestamp: time.Now(),
		}
		s.mu.Unlock()
	}

	if format == "markdown" {
		w.Header().Set("Content-Type", "text/markdown")
		for _, f := range validFacts {
			payloadJSON, _ := json.MarshalIndent(f.Payload, "", "  ")
			fmt.Fprintf(w, "## Fact: %s\n```json\n%s\n```\n\n", f.ID, string(payloadJSON))
		}
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
	s.mu.RLock()
	defer s.mu.RUnlock()

	var result []sessionInfo
	for id := range s.Engines {
		storedOrg, err := s.Store.GetSessionOrganization(id)
		if err == nil && storedOrg == orgID {
			result = append(result, sessionInfo{ID: id})
		}
	}
	writeJSON(w, http.StatusOK, result)
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
	s.mu.RUnlock()

	var stat syscall.Statfs_t
	wd, _ := os.Getwd()
	syscall.Statfs(wd, &stat)

	diskFree := stat.Bavail * uint64(stat.Bsize)
	diskTotal := stat.Blocks * uint64(stat.Bsize)
	diskUsed := diskTotal - diskFree
	BadgerDiskUsage.Set(float64(diskUsed))

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"status":           "healthy",
		"badger_connected": s.Store != nil,
		"ram_sessions":     sessionCount,
		"disk": map[string]interface{}{
			"free_bytes":    diskFree,
			"total_bytes":   diskTotal,
			"usage_percent": fmt.Sprintf("%.2f%%", (1-float64(diskFree)/float64(diskTotal))*100),
		},
		"uptime": time.Since(s.StartTime).String(),
	})
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
	var records [][]string
	for _, entry := range history {
		factID := entry.FactID
		if factID == "" && entry.Fact != nil {
			factID = entry.Fact.ID
		}

		confidence := "1.0"
		if entry.Fact != nil {
			confidence = fmt.Sprintf("%.2f", entry.Fact.ManualStatus)
		}

		records = append(records, []string{
			time.UnixMilli(entry.Timestamp).Format(time.RFC3339),
			string(entry.Type),
			sessionID,
			orgID,
			entry.ActorID,
			factID,
			confidence,
			fmt.Sprintf("%.2f", engine.GetStatus(factID)),
		})
	}

	h := sha256.New()
	for _, row := range records {
		h.Write([]byte(strings.Join(row, ",")))
	}
	hashSum := hex.EncodeToString(h.Sum(nil))

	if format == "csv" {
		w.Header().Set("Content-Type", "text/csv")
		w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=velarix_audit_%s.csv", sessionID))
		writer := csv.NewWriter(w)
		writer.Write([]string{"VERIFICATION_HASH", hashSum})
		writer.Write([]string{"timestamp", "event_type", "session_id", "org_id", "actor_id", "fact_id", "confidence", "current_status"})
		writer.WriteAll(records)
		writer.Flush()
		return
	}

	if format == "pdf" {
		usage, _ := s.Store.GetOrgUsage(orgID)
		pdf := gofpdf.New("P", "mm", "A4", "")
		pdf.AddPage()
		pdf.SetFont("Courier", "B", 8)
		pdf.Cell(0, 10, "Verification Hash: "+hashSum)
		pdf.Ln(10)
		pdf.SetFont("Arial", "B", 16)
		pdf.Cell(0, 10, "SOC2 Compliance Export - Velarix Audit Log")
		pdf.Ln(12)
		pdf.SetFont("Arial", "", 10)
		pdf.Cell(0, 10, fmt.Sprintf("Total Facts: %d | API Requests: %d", usage["facts_asserted"], usage["api_requests"]))
		pdf.Ln(12)
		for _, row := range records {
			pdf.Cell(0, 5, fmt.Sprintf("[%s] %s | Actor: %s | Fact: %s | Status: %s", row[0], row[1], row[4], row[5], row[8]))
			pdf.Ln(4)
			if pdf.GetY() > 270 {
				pdf.AddPage()
			}
		}
		w.Header().Set("Content-Type", "application/pdf")
		w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=velarix_audit_%s.pdf", sessionID))
		pdf.Output(w)
		return
	}
}

func (s *Server) handleGetUsage(w http.ResponseWriter, r *http.Request) {
	orgID := getOrgID(r)
	usage, _ := s.Store.GetOrgUsage(orgID)
	writeJSON(w, http.StatusOK, usage)
}

func (s *Server) checkRateLimit(apiKey string) bool {
	now := time.Now()
	minuteAgo := now.Add(-1 * time.Minute)

	limits, err := s.Store.GetRateLimit(apiKey)
	if err != nil && err.Error() != "Key not found" {
		return false // Fail closed on DB error
	}

	var valid []time.Time
	for _, t := range limits {
		if t.After(minuteAgo) {
			valid = append(valid, t)
		}
	}

	if len(valid) >= 60 {
		return false
	}

	valid = append(valid, now)
	s.Store.SaveRateLimit(apiKey, valid)
	return true
}

func (s *Server) authMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/health" || strings.HasPrefix(r.URL.Path, "/auth/") || strings.HasPrefix(r.URL.Path, "/v1/auth/") || strings.HasPrefix(r.URL.Path, "/docs/") {
			next.ServeHTTP(w, r)
			return
		}

		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			http.Error(w, "Invalid or expired API key", http.StatusUnauthorized)
			return
		}
		token := strings.TrimPrefix(authHeader, "Bearer ")

		adminKey := os.Getenv("VELARIX_API_KEY")
		if adminKey != "" && token == adminKey {
			ctx := context.WithValue(r.Context(), orgIDKey, "admin")
			ctx = context.WithValue(ctx, actorIDKey, "system")
			ctx = context.WithValue(ctx, userRoleKey, "admin")
			next.ServeHTTP(w, r.WithContext(ctx))
			return
		}

		claims := &Claims{}
		tkn, err := jwt.ParseWithClaims(token, claims, func(token *jwt.Token) (interface{}, error) { return jwtKey, nil })
		if err == nil && tkn.Valid {
			user, err := s.Store.GetUser(claims.Email)
			if err != nil {
				http.Error(w, "User not found", http.StatusUnauthorized)
				return
			}
			ctx := context.WithValue(r.Context(), orgIDKey, user.OrgID)
			ctx = context.WithValue(ctx, actorIDKey, user.Email)
			ctx = context.WithValue(ctx, userRoleKey, user.Role)
			ctx = context.WithValue(ctx, userEmailKey, user.Email)
			next.ServeHTTP(w, r.WithContext(ctx))
			return
		}

		emailBytes, err := s.Store.GetAPIKeyOwner(token)
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
		for i, k := range user.Keys {
			if k.Key == token && !k.IsRevoked && k.ExpiresAt > time.Now().UnixMilli() {
				keyValid = true
				user.Keys[i].LastUsedAt = time.Now().UnixMilli()
				actorID = k.Label
				if actorID == "" {
					actorID = token[:min(len(token), 8)]
				}
				break
			}
		}
		if !keyValid {
			http.Error(w, "Invalid or expired API key", http.StatusUnauthorized)
			return
		}
		if !s.checkRateLimit(token) {
			http.Error(w, "rate limit exceeded (60 rpm)", http.StatusTooManyRequests)
			return
		}
		go s.Store.IncrementMetric(user.OrgID, "api_requests")
		go s.Store.SaveUser(user)

		ctx := context.WithValue(r.Context(), orgIDKey, user.OrgID)
		ctx = context.WithValue(ctx, actorIDKey, actorID)
		ctx = context.WithValue(ctx, userRoleKey, user.Role)
		ctx = context.WithValue(ctx, userEmailKey, user.Email)

		traceID := r.Header.Get("X-Trace-Id")
		if traceID == "" {
			traceID = fmt.Sprintf("req-%d", time.Now().UnixNano())
		}
		ctx = context.WithValue(ctx, contextKey("trace_id"), traceID)

		w.Header().Set("X-Trace-Id", traceID)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
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
	mux.HandleFunc("GET /docs/openapi.yaml", func(w http.ResponseWriter, r *http.Request) { http.ServeFile(w, r, "docs/swagger.yaml") })
	mux.HandleFunc("GET /docs/postman.json", func(w http.ResponseWriter, r *http.Request) { http.ServeFile(w, r, "docs/postman.json") })
	mux.Handle("GET /metrics", promhttp.Handler())
	mux.HandleFunc("GET /health", s.handleHealth)
	mux.HandleFunc("GET /health/full", s.handleFullHealth)

	// Admin Management Routes
	mux.HandleFunc("GET /v1/org/backup", s.handleBackup)
	mux.HandleFunc("POST /v1/org/restore", s.handleRestore)

	// V1 API Routes
	mux.HandleFunc("GET /v1/org/usage", s.handleGetUsage)
	mux.HandleFunc("POST /v1/auth/register", s.handleRegister)
	mux.HandleFunc("POST /auth/register", s.handleRegister)
	mux.HandleFunc("POST /v1/auth/login", s.handleLogin)
	mux.HandleFunc("POST /auth/login", s.handleLogin)
	mux.HandleFunc("POST /v1/auth/reset-request", s.handleResetRequest)
	mux.HandleFunc("POST /auth/reset-request", s.handleResetRequest)
	mux.HandleFunc("POST /v1/auth/reset-confirm", s.handleResetConfirm)
	mux.HandleFunc("POST /auth/reset-confirm", s.handleResetConfirm)
	mux.HandleFunc("POST /v1/keys/generate", s.handleGenerateKey)
	mux.HandleFunc("GET /v1/keys", s.handleListKeys)
	mux.HandleFunc("DELETE /v1/keys/{key}", s.handleRevokeKey)
	mux.HandleFunc("POST /v1/keys/{key}/rotate", s.handleRotateKey)
	mux.HandleFunc("GET /v1/sessions", s.handleListSessions)

	// Session-scoped V1 Routes
	mux.HandleFunc("POST /v1/s/{session_id}/facts", s.handleAssertFact)
	mux.HandleFunc("POST /v1/s/{session_id}/facts/{id}/invalidate", s.handleInvalidateRoot)
	mux.HandleFunc("GET /v1/s/{session_id}/facts/{id}", s.handleGetFact)
	mux.HandleFunc("GET /v1/s/{session_id}/facts/{id}/why", s.handleExplain)
	mux.HandleFunc("GET /v1/s/{session_id}/facts/{id}/impact", s.handleGetImpact)
	mux.HandleFunc("GET /v1/s/{session_id}/facts", s.handleListFacts)
	mux.HandleFunc("POST /v1/s/{session_id}/config", s.handleUpdateConfig)
	mux.HandleFunc("POST /v1/s/{session_id}/revalidate", s.handleRevalidate)
	mux.HandleFunc("GET /v1/s/{session_id}/export", s.handleExport)
	mux.HandleFunc("GET /v1/s/{session_id}/history", s.handleGetHistory)
	mux.HandleFunc("POST /v1/s/{session_id}/history", s.handleAppendHistory)
	mux.HandleFunc("GET /v1/s/{session_id}/slice", s.handleGetSlice)
	mux.HandleFunc("GET /v1/s/{session_id}/events", s.handleEvents)
	mux.HandleFunc("GET /v1/s/{session_id}/explain", s.handleExplainReasoning)
	mux.HandleFunc("GET /v1/s/{session_id}/explanations", s.handleGetExplanations)

	return s.metricsAndLoggingMiddleware(s.enableCORS(s.authMiddleware(mux)))
}
