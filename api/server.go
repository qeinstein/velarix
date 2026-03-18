package api

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"velarix/core"
	"velarix/store"

	"github.com/xeipuuv/gojsonschema"
)

type Server struct {
	mu          sync.RWMutex
	Engines     map[string]*core.Engine
	Configs     map[string]*store.SessionConfig
	LastAccess  map[string]time.Time
	Store       *store.BadgerStore
	StartTime   time.Time
	RateLimit   map[string][]time.Time // APIKey -> timestamps of requests
}

func (s *Server) getEngine(sessionID string) (*core.Engine, *store.SessionConfig) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.LastAccess == nil {
		s.LastAccess = make(map[string]time.Time)
	}
	s.LastAccess[sessionID] = time.Now()

	engine, ok := s.Engines[sessionID]
	config := s.Configs[sessionID]

	if ok {
		return engine, config
	}

	// Lazy Hydration: Load from BadgerDB
	engine = core.NewEngine()
	
	// Load Config
	config, err := s.Store.GetConfig(sessionID)
	if err != nil {
		// Default config if not found
		config = &store.SessionConfig{EnforcementMode: "strict"}
	}

	// 1. Try Snapshot first
	lastReplayTime := int64(0)
	snap, err := s.Store.GetLatestSnapshot(sessionID)
	if err == nil {
		if err := engine.FromSnapshot(snap); err == nil {
			lastReplayTime = snap.Timestamp
		} else {
			// Integrity failure! Log it and fall back
			s.Store.Append(store.JournalEntry{
				Type:      store.EventSnapshotCorruption,
				SessionID: sessionID,
				Timestamp: time.Now().UnixMilli(),
			})
		}
	}

	// 2. Replay Journal (either Delta or Full)
	history, err := s.Store.GetSessionHistory(sessionID)
	if err == nil {
		for _, entry := range history {
			// Skip entries already covered by snapshot
			if entry.Timestamp <= lastReplayTime {
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
	
	return engine, config
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
	_, config := s.getEngine(sessionID)

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

	// Persist config to Badger
	if err := s.Store.SaveConfig(sessionID, config); err != nil {
		http.Error(w, "failed to persist config: "+err.Error(), http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusOK, config)
}

func (s *Server) checkSnapshotTrigger(sessionID string, engine *core.Engine) {
	// Snapshot every 50 mutations OR every 5 minutes if there were mutations
	now := time.Now().UnixMilli()
	shouldSnap := false

	engine.Lock() // We need a full lock to check MutationCount accurately
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
	sessionID := r.PathValue("session_id")
	engine, config := s.getEngine(sessionID)

	var fact core.Fact
	if err := json.NewDecoder(r.Body).Decode(&fact); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Payload Size Cap (64KB)
	payloadBytes, _ := json.Marshal(fact.Payload)
	if len(payloadBytes) > 64*1024 {
		http.Error(w, "payload too large (max 64KB)", http.StatusRequestEntityTooLarge)
		return
	}

	// Provenance Middleware: Extract _provenance before validation
	var provenance interface{}
	if fact.Payload != nil {
		if p, ok := fact.Payload["_provenance"]; ok {
			provenance = p
			delete(fact.Payload, "_provenance")
			if fact.Metadata == nil {
				fact.Metadata = make(map[string]interface{})
			}
			fact.Metadata["_provenance"] = provenance
		}
	}

	// Schema Validation (now ignores _provenance)
	if config.Schema != "" {
		schemaLoader := gojsonschema.NewStringLoader(config.Schema)
		documentLoader := gojsonschema.NewGoLoader(fact.Payload)
		
		result, err := gojsonschema.Validate(schemaLoader, documentLoader)
		if err != nil {
			http.Error(w, "schema validation internal error: "+err.Error(), http.StatusInternalServerError)
			return
		}

		if !result.Valid() {
			var errMsgs []string
			for _, desc := range result.Errors() {
				errMsgs = append(errMsgs, desc.String())
			}

			if config.EnforcementMode == "strict" {
				writeJSON(w, http.StatusBadRequest, map[string]interface{}{
					"error":  "schema validation failed",
					"details": errMsgs,
				})
				return
			} else {
				fact.ValidationErrors = errMsgs
			}
		}
	}

	if err := engine.AssertFact(&fact); err != nil {
		// Check if it's a cycle error
		if cycleErr, ok := err.(*core.CycleError); ok {
			// Persist the violation to the journal
			violationEntry := store.JournalEntry{
				Type:      store.EventCycleViolation,
				SessionID: sessionID,
				FactID:    fact.ID,
				Fact: &core.Fact{
					ID: fact.ID,
					ValidationErrors: []string{cycleErr.Error()},
				},
			}
			s.Store.Append(violationEntry)
			
			writeJSON(w, http.StatusBadRequest, map[string]string{
				"error": cycleErr.Error(),
			})
			return
		}
		
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Persist successful assert to Badger
	entry := store.JournalEntry{
		Type:      store.EventAssert,
		SessionID: sessionID,
		Fact:      &fact,
	}
	if err := s.Store.Append(entry); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusCreated, fact)
	s.checkSnapshotTrigger(sessionID, engine)
}

func (s *Server) handleInvalidateRoot(w http.ResponseWriter, r *http.Request) {
	sessionID := r.PathValue("session_id")
	engine, _ := s.getEngine(sessionID)
	id := r.PathValue("id")

	// Persist to Badger
	entry := store.JournalEntry{
		Type:      store.EventInvalidate,
		SessionID: sessionID,
		FactID:    id,
	}
	if err := s.Store.Append(entry); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if err := engine.InvalidateRoot(id); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "invalidated"})
	s.checkSnapshotTrigger(sessionID, engine)
}

func (s *Server) handleGetFact(w http.ResponseWriter, r *http.Request) {
	sessionID := r.PathValue("session_id")
	engine, _ := s.getEngine(sessionID)
	id := r.PathValue("id")

	fact, ok := engine.GetFact(id)
	if !ok {
		http.Error(w, "fact not found", http.StatusNotFound)
		return
	}

	status := engine.GetStatus(id)
	
	response := struct {
		*core.Fact
		ResolvedStatus core.Status `json:"resolved_status"`
	}{
		Fact:           fact,
		ResolvedStatus: status,
	}

	writeJSON(w, http.StatusOK, response)
}

func (s *Server) handleExplain(w http.ResponseWriter, r *http.Request) {
	sessionID := r.PathValue("session_id")
	engine, _ := s.getEngine(sessionID)
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
	engine, _ := s.getEngine(sessionID)
	id := r.PathValue("id")

	impact := engine.GetImpact(id)
	if impact == nil {
		impact = []string{}
	}
	writeJSON(w, http.StatusOK, impact)
}

func (s *Server) handleListFacts(w http.ResponseWriter, r *http.Request) {
	sessionID := r.PathValue("session_id")
	engine, _ := s.getEngine(sessionID)
	validOnly := r.URL.Query().Get("valid") == "true"

	facts := engine.ListFacts()
	result := make([]interface{}, 0)

	for _, fact := range facts {
		status := engine.GetStatus(fact.ID)
		if validOnly && status < core.ConfidenceThreshold {
			continue
		}

		result = append(result, struct {			*core.Fact
			ResolvedStatus core.Status `json:"resolved_status"`
		}{
			Fact:           fact,
			ResolvedStatus: status,
		})
	}

	writeJSON(w, http.StatusOK, result)
}

func (s *Server) handleAppendHistory(w http.ResponseWriter, r *http.Request) {
	sessionID := r.PathValue("session_id")
	
	var entry store.JournalEntry
	if err := json.NewDecoder(r.Body).Decode(&entry); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	entry.SessionID = sessionID
	entry.Timestamp = time.Now().UnixMilli()

	if err := s.Store.Append(entry); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusCreated, entry)
}

func (s *Server) handleGetHistory(w http.ResponseWriter, r *http.Request) {
	sessionID := r.PathValue("session_id")
	
	history, err := s.Store.GetSessionHistory(sessionID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusOK, history)
}

func sanitizeMarkdown(input string) string {
	// Escape backticks to prevent breaking out of code blocks
	return strings.ReplaceAll(input, "`", "\\` ")
}

func (s *Server) handleRevalidate(w http.ResponseWriter, r *http.Request) {
	sessionID := r.PathValue("session_id")
	
	// 1. Lock Session
	s.mu.Lock()
	engine, ok := s.Engines[sessionID]
	config := s.Configs[sessionID]
	if !ok {
		s.mu.Unlock()
		s.getEngine(sessionID) // Trigger lazy hydration first
		s.mu.Lock()
		engine = s.Engines[sessionID]
		config = s.Configs[sessionID]
	}
	defer s.mu.Unlock()

	// 2. Clear current engine state (retaining session_id and metadata)
	engine.Lock()
	engine.Facts = make(map[string]*core.Fact)
	engine.JustificationSets = make(map[string]*core.JustificationSet)
	engine.ChildrenIndex = make(map[string]map[string]struct{})
	engine.CollapsedRoots = make(map[string]struct{})
	engine.MutationCount = 0
	engine.Unlock()

	// 3. Reload History
	history, err := s.Store.GetSessionHistory(sessionID)
	if err != nil {
		http.Error(w, "failed to load history: "+err.Error(), http.StatusInternalServerError)
		return
	}

	summary := struct {
		Revalidated int `json:"revalidated"`
		Passed      int `json:"passed"`
		Violations  int `json:"violations"`
		Pruned      int `json:"pruned"`
	}{}

	// 4. Re-run validation logic for every EventAssert
	for _, entry := range history {
		if entry.Type == store.EventAssert {
			summary.Revalidated++
			fact := entry.Fact
			
			// Re-run Provenance Middleware
			if fact.Payload != nil {
				delete(fact.Payload, "_provenance")
				if fact.Metadata != nil {
					if p, ok := fact.Metadata["_provenance"]; ok {
						fact.Payload["_provenance"] = p
					}
				}
			}
			// (Strip it again just to be safe before validation)
			delete(fact.Payload, "_provenance")

			// Re-run Schema Validation
			validSchema := true
			if config.Schema != "" {
				schemaLoader := gojsonschema.NewStringLoader(config.Schema)
				documentLoader := gojsonschema.NewGoLoader(fact.Payload)
				res, _ := gojsonschema.Validate(schemaLoader, documentLoader)
				if !res.Valid() {
					validSchema = false
				}
			}

			if !validSchema {
				summary.Violations++
				continue
			}

			// Re-run Cycle Detection & Assertion
			if err := engine.AssertFact(fact); err != nil {
				summary.Violations++
				continue
			}

			// Check Status (Pruning)
			if engine.GetStatus(fact.ID) < core.ConfidenceThreshold {
				summary.Pruned++
			} else {
				summary.Passed++
			}
		} else if entry.Type == store.EventInvalidate {
			engine.InvalidateRoot(entry.FactID)
		}
	}

	// 5. Append completion event to journal
	completionEntry := store.JournalEntry{
		Type:      store.EventRevalidationComplete,
		SessionID: sessionID,
		FactID:    "system", // Tagged as system event
		Fact:      &core.Fact{ID: "system", Payload: map[string]interface{}{"summary": summary}},
		Timestamp: time.Now().UnixMilli(),
	}
	s.Store.Append(completionEntry)

	writeJSON(w, http.StatusOK, summary)
}

func (s *Server) handleGetSlice(w http.ResponseWriter, r *http.Request) {
	sessionID := r.PathValue("session_id")
	engine, _ := s.getEngine(sessionID)

	format := r.URL.Query().Get("format")
	if format == "" {
		format = "json"
	}

	maxFacts := 50
	if m := r.URL.Query().Get("max_facts"); m != "" {
		fmt.Sscanf(m, "%d", &maxFacts)
	}

	facts := engine.ListFacts()
	var validFacts []*core.Fact
	for _, f := range facts {
		if engine.GetStatus(f.ID) >= core.ConfidenceThreshold {
			validFacts = append(validFacts, f)
		}
	}

	if len(validFacts) > maxFacts {
		validFacts = validFacts[:maxFacts]
	}

	if format == "markdown" {
		w.Header().Set("Content-Type", "text/markdown")
		for _, f := range validFacts {
			payloadJSON, _ := json.MarshalIndent(f.Payload, "", "  ")
			// Sanitize the ID and the payload JSON string to prevent prompt injection
			fmt.Fprintf(w, "## Fact: %s\n```json\n%s\n```\n\n", sanitizeMarkdown(f.ID), string(payloadJSON))
		}
		return
	}

	type sliceEntry struct {
		ID      string                 `json:"id"`
		Payload map[string]interface{} `json:"payload"`
	}
	var result []sliceEntry
	for _, f := range validFacts {
		result = append(result, sliceEntry{ID: f.ID, Payload: f.Payload})
	}
	writeJSON(w, http.StatusOK, result)
}

func (s *Server) handleEvents(w http.ResponseWriter, r *http.Request) {
	sessionID := r.PathValue("session_id")
	engine, _ := s.getEngine(sessionID)

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	ch := engine.Subscribe()
	defer engine.Unsubscribe(ch)

	fmt.Fprintf(w, "retry: 1000\n\n")
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming unsupported", http.StatusInternalServerError)
		return
	}
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
	s.mu.RLock()
	defer s.mu.RUnlock()

	type sessionInfo struct {
		ID              string `json:"id"`
		FactCount       int    `json:"fact_count"`
		EnforcementMode string `json:"enforcement_mode"`
		Status          string `json:"status"`
	}

	result := make([]sessionInfo, 0)
	for id, engine := range s.Engines {
		config := s.Configs[id]
		facts := engine.ListFacts()

		status := "healthy"
		hasViolations := false
		for _, f := range facts {
			if len(f.ValidationErrors) > 0 {
				hasViolations = true
				break
			}
		}

		if hasViolations {
			status = "warn"
		}

		result = append(result, sessionInfo{
			ID:              id,
			FactCount:       len(facts),
			EnforcementMode: config.EnforcementMode,
			Status:          status,
		})
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

func (s *Server) handleGenerateKey(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Email string `json:"email"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Email == "" {
		http.Error(w, "valid email required", http.StatusBadRequest)
		return
	}

	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	key := "vx_" + hex.EncodeToString(b)

	if err := s.Store.SaveAPIKey(key, body.Email); err != nil {
		http.Error(w, "failed to save key", http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusCreated, map[string]string{"api_key": key})
}

func (s *Server) checkRateLimit(apiKey string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.RateLimit == nil {
		s.RateLimit = make(map[string][]time.Time)
	}

	now := time.Now()
	minuteAgo := now.Add(-1 * time.Minute)

	// Clean up old entries
	var valid []time.Time
	for _, t := range s.RateLimit[apiKey] {
		if t.After(minuteAgo) {
			valid = append(valid, t)
		}
	}

	if len(valid) >= 60 { // Max 60 requests per minute
		return false
	}

	s.RateLimit[apiKey] = append(valid, now)
	return true
}

func (s *Server) authMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Public routes
		if r.URL.Path == "/health" || r.URL.Path == "/keys/generate" {
			next.ServeHTTP(w, r)
			return
		}

		adminKey := os.Getenv("VELARIX_API_KEY")
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" || !strings.HasPrefix(authHeader, "Bearer ") {
			http.Error(w, "missing or invalid authorization header. Get a key at velarix.dev/keys", http.StatusUnauthorized)
			return
		}

		token := strings.TrimPrefix(authHeader, "Bearer ")

		// Check Admin Key first
		if adminKey != "" && token == adminKey {
			next.ServeHTTP(w, r)
			return
		}

		// Check against BadgerDB for generated keys
		isValid, err := s.Store.ValidateAPIKey(token)
		if err != nil || !isValid {
			http.Error(w, "invalid api key. Get a key at velarix.dev/keys", http.StatusUnauthorized)
			return
		}

		// Apply rate limiting
		if !s.checkRateLimit(token) {
			http.Error(w, "rate limit exceeded (60 rpm)", http.StatusTooManyRequests)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func (s *Server) PerformEvictionSweep() {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	now := time.Now()
	for id, last := range s.LastAccess {
		if now.Sub(last) > 15*time.Minute {
			delete(s.Engines, id)
			delete(s.Configs, id)
			delete(s.LastAccess, id)
		}
	}
}

func (s *Server) StartEvictionTicker() {
	ticker := time.NewTicker(5 * time.Minute)
	go func() {
		for range ticker.C {
			s.PerformEvictionSweep()
		}
	}()
}

func (s *Server) Routes() http.Handler {
	mux := http.NewServeMux()

	// Global Routes
	mux.HandleFunc("GET /health", s.handleHealth)
	mux.HandleFunc("POST /keys/generate", s.handleGenerateKey)
	mux.HandleFunc("GET /sessions", s.handleListSessions)

	mux.HandleFunc("POST /s/{session_id}/facts", s.handleAssertFact)
	mux.HandleFunc("POST /s/{session_id}/facts/{id}/invalidate", s.handleInvalidateRoot)
	mux.HandleFunc("GET /s/{session_id}/facts/{id}", s.handleGetFact)
	mux.HandleFunc("GET /s/{session_id}/facts/{id}/why", s.handleExplain)
	mux.HandleFunc("GET /s/{session_id}/facts/{id}/impact", s.handleGetImpact)
	mux.HandleFunc("GET /s/{session_id}/facts", s.handleListFacts)
	mux.HandleFunc("POST /s/{session_id}/config", s.handleUpdateConfig)
	mux.HandleFunc("POST /s/{session_id}/revalidate", s.handleRevalidate)
	mux.HandleFunc("GET /s/{session_id}/history", s.handleGetHistory)
	mux.HandleFunc("POST /s/{session_id}/history", s.handleAppendHistory)
	mux.HandleFunc("GET /s/{session_id}/slice", s.handleGetSlice)
	mux.HandleFunc("GET /s/{session_id}/events", s.handleEvents)

	return s.enableCORS(s.authMiddleware(mux))
}
