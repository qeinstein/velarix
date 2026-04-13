package api

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"velarix/core"
	"velarix/store"
)

type meResponse struct {
	Email string `json:"email"`
	OrgID string `json:"org_id"`
	Role  string `json:"role"`
}

func (s *Server) handleMe(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, meResponse{
		Email: getUserEmail(r),
		OrgID: getOrgID(r),
		Role:  getUserRole(r),
	})
}

func (s *Server) handleGetOrg(w http.ResponseWriter, r *http.Request) {
	orgID := getOrgID(r)
	org, err := s.Store.GetOrganization(orgID)
	if err != nil {
		http.Error(w, "org not found", http.StatusNotFound)
		return
	}
	writeJSON(w, http.StatusOK, org)
}

type patchOrgRequest struct {
	Name        *string `json:"name,omitempty"`
	IsSuspended *bool   `json:"is_suspended,omitempty"`
}

func (s *Server) handlePatchOrg(w http.ResponseWriter, r *http.Request) {
	if getUserRole(r) != "admin" {
		http.Error(w, "forbidden: admin role required", http.StatusForbidden)
		return
	}
	orgID := getOrgID(r)
	org, err := s.Store.GetOrganization(orgID)
	if err != nil {
		http.Error(w, "org not found", http.StatusNotFound)
		return
	}

	var body patchOrgRequest
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}
	if body.Name != nil {
		n := strings.TrimSpace(*body.Name)
		if n == "" {
			http.Error(w, "name cannot be empty", http.StatusBadRequest)
			return
		}
		org.Name = n
	}
	if body.IsSuspended != nil {
		org.IsSuspended = *body.IsSuspended
	}
	if err := s.Store.SaveOrganization(org); err != nil {
		http.Error(w, "failed to persist org", http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, org)
}

func (s *Server) handleListOrgSessions(w http.ResponseWriter, r *http.Request) {
	orgID := getOrgID(r)
	cursor := r.URL.Query().Get("cursor")
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	items, next, err := s.Store.ListOrgSessions(orgID, cursor, limit)
	if err != nil {
		http.Error(w, "failed to list sessions", http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"items": items, "next_cursor": next})
}

func randID(nBytes int) string {
	b := make([]byte, nBytes)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

func (s *Server) createNotification(orgID string, typ string, title string, body string) {
	if orgID == "" {
		return
	}
	n := &store.Notification{
		ID:        randID(6),
		Type:      typ,
		Title:     title,
		Body:      body,
		CreatedAt: time.Now().UnixMilli(),
	}
	_ = s.Store.SaveNotification(orgID, n)
}

func (s *Server) handleListNotifications(w http.ResponseWriter, r *http.Request) {
	orgID := getOrgID(r)
	cursor := r.URL.Query().Get("cursor")
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	items, next, err := s.Store.ListNotifications(orgID, cursor, limit)
	if err != nil {
		http.Error(w, "failed to list notifications", http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"items": items, "next_cursor": next})
}

func (s *Server) handleMarkNotificationRead(w http.ResponseWriter, r *http.Request) {
	orgID := getOrgID(r)
	id := r.PathValue("id")
	if id == "" {
		http.Error(w, "missing id", http.StatusBadRequest)
		return
	}
	if err := s.Store.MarkNotificationRead(orgID, id); err != nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "read"})
}

type integrationCreateRequest struct {
	Name    string                 `json:"name"`
	Kind    string                 `json:"kind"`
	Enabled bool                   `json:"enabled"`
	Config  map[string]interface{} `json:"config,omitempty"`
}

func (s *Server) handleListIntegrations(w http.ResponseWriter, r *http.Request) {
	orgID := getOrgID(r)
	list, err := s.Store.ListIntegrations(orgID)
	if err != nil {
		http.Error(w, "failed to list integrations", http.StatusInternalServerError)
		return
	}
	sort.Slice(list, func(i, j int) bool { return list[i].UpdatedAt > list[j].UpdatedAt })
	writeJSON(w, http.StatusOK, list)
}

func (s *Server) handleCreateIntegration(w http.ResponseWriter, r *http.Request) {
	if getUserRole(r) != "admin" {
		http.Error(w, "forbidden: admin role required", http.StatusForbidden)
		return
	}
	orgID := getOrgID(r)
	var body integrationCreateRequest
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}
	body.Name = strings.TrimSpace(body.Name)
	body.Kind = strings.TrimSpace(body.Kind)
	if body.Name == "" || body.Kind == "" {
		http.Error(w, "name and kind are required", http.StatusBadRequest)
		return
	}
	now := time.Now().UnixMilli()
	integ := &store.Integration{
		ID:        "int_" + randID(8),
		Name:      body.Name,
		Kind:      body.Kind,
		Enabled:   body.Enabled,
		Config:    body.Config,
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := s.Store.SaveIntegration(orgID, integ); err != nil {
		http.Error(w, "failed to persist", http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusCreated, integ)
}

type integrationPatchRequest struct {
	Name    *string                `json:"name,omitempty"`
	Enabled *bool                  `json:"enabled,omitempty"`
	Config  map[string]interface{} `json:"config,omitempty"`
}

func (s *Server) handlePatchIntegration(w http.ResponseWriter, r *http.Request) {
	if getUserRole(r) != "admin" {
		http.Error(w, "forbidden: admin role required", http.StatusForbidden)
		return
	}
	orgID := getOrgID(r)
	id := r.PathValue("id")
	integ, err := s.Store.GetIntegration(orgID, id)
	if err != nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	var body integrationPatchRequest
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}
	if body.Name != nil {
		n := strings.TrimSpace(*body.Name)
		if n == "" {
			http.Error(w, "name cannot be empty", http.StatusBadRequest)
			return
		}
		integ.Name = n
	}
	if body.Enabled != nil {
		integ.Enabled = *body.Enabled
	}
	if body.Config != nil {
		integ.Config = body.Config
	}
	integ.UpdatedAt = time.Now().UnixMilli()
	if err := s.Store.SaveIntegration(orgID, integ); err != nil {
		http.Error(w, "failed to persist", http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, integ)
}

func (s *Server) handleDeleteIntegration(w http.ResponseWriter, r *http.Request) {
	if getUserRole(r) != "admin" {
		http.Error(w, "forbidden: admin role required", http.StatusForbidden)
		return
	}
	orgID := getOrgID(r)
	id := r.PathValue("id")
	if err := s.Store.DeleteIntegration(orgID, id); err != nil {
		http.Error(w, "failed to delete", http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

func (s *Server) handleGetConfig(w http.ResponseWriter, r *http.Request) {
	sessionID := r.PathValue("session_id")
	orgID := getOrgID(r)
	_, config, err := s.getEngine(sessionID, orgID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusForbidden)
		return
	}
	writeJSON(w, http.StatusOK, config)
}

func (s *Server) handleHistoryPage(w http.ResponseWriter, r *http.Request) {
	sessionID := r.PathValue("session_id")
	orgID := getOrgID(r)
	storedOrg, err := s.Store.GetSessionOrganization(sessionID)
	if err != nil || storedOrg != orgID {
		http.Error(w, "unauthorized", http.StatusForbidden)
		return
	}

	cursor := r.URL.Query().Get("cursor")
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	q := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("q")))
	typ := strings.TrimSpace(r.URL.Query().Get("type"))
	var fromMs, toMs int64
	if v := r.URL.Query().Get("from"); v != "" {
		if parsed, err := strconv.ParseInt(v, 10, 64); err == nil {
			fromMs = parsed
		}
	}
	if v := r.URL.Query().Get("to"); v != "" {
		if parsed, err := strconv.ParseInt(v, 10, 64); err == nil {
			toMs = parsed
		}
	}

	entries, next, err := s.Store.GetSessionHistoryPage(sessionID, cursor, limit, fromMs, toMs, typ, q)
	if err != nil {
		http.Error(w, "failed to fetch history", http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"items": entries, "next_cursor": next})
}

type docsPage struct {
	Slug  string `json:"slug"`
	Title string `json:"title"`
}

func (s *Server) handleListDocsPages(w http.ResponseWriter, r *http.Request) {
	root := "markdown"
	pages := []docsPage{}
	_ = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		if !strings.HasSuffix(d.Name(), ".md") {
			return nil
		}
		slug := strings.TrimSuffix(d.Name(), ".md")
		title := slug
		pages = append(pages, docsPage{Slug: slug, Title: title})
		return nil
	})
	sort.Slice(pages, func(i, j int) bool { return pages[i].Slug < pages[j].Slug })
	writeJSON(w, http.StatusOK, pages)
}

func (s *Server) handleGetDocsPage(w http.ResponseWriter, r *http.Request) {
	slug := r.PathValue("slug")
	if slug == "" {
		http.Error(w, "missing slug", http.StatusBadRequest)
		return
	}
	path := filepath.Join("markdown", slug+".md")
	b, err := os.ReadFile(path)
	if err != nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"slug": slug, "content": string(b)})
}

type changePasswordRequest struct {
	CurrentPassword string `json:"current_password"`
	NewPassword     string `json:"new_password"`
}

func (s *Server) handleChangePassword(w http.ResponseWriter, r *http.Request) {
	email := getUserEmail(r)
	if email == "" {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	var body changePasswordRequest
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}
	if len(body.NewPassword) < 8 {
		http.Error(w, "new_password too short", http.StatusBadRequest)
		return
	}

	user, err := s.Store.GetUser(email)
	if err != nil {
		http.Error(w, "user not found", http.StatusNotFound)
		return
	}
	match, err := comparePassword(body.CurrentPassword, user.HashedPassword)
	if err != nil || !match {
		http.Error(w, "invalid credentials", http.StatusUnauthorized)
		return
	}
	hashed, err := hashPassword(body.NewPassword)
	if err != nil {
		http.Error(w, "hashing failure", http.StatusInternalServerError)
		return
	}
	user.HashedPassword = hashed
	if err := s.Store.SaveUser(user); err != nil {
		http.Error(w, "failed to persist", http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "password updated"})
}

func (s *Server) handleGetOnboarding(w http.ResponseWriter, r *http.Request) {
	email := getUserEmail(r)
	if email == "" {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	user, err := s.Store.GetUser(email)
	if err != nil {
		http.Error(w, "user not found", http.StatusNotFound)
		return
	}
	if user.Onboarding == nil {
		user.Onboarding = map[string]bool{}
	}
	writeJSON(w, http.StatusOK, user.Onboarding)
}

func (s *Server) handleUpdateOnboarding(w http.ResponseWriter, r *http.Request) {
	email := getUserEmail(r)
	if email == "" {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	user, err := s.Store.GetUser(email)
	if err != nil {
		http.Error(w, "user not found", http.StatusNotFound)
		return
	}
	var body map[string]bool
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}
	if user.Onboarding == nil {
		user.Onboarding = map[string]bool{}
	}
	for k, v := range body {
		user.Onboarding[k] = v
	}
	if err := s.Store.SaveUser(user); err != nil {
		http.Error(w, "failed to persist", http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, user.Onboarding)
}

type graphNode struct {
	ID     string  `json:"id"`
	Status float64 `json:"status"`
	IsRoot bool    `json:"is_root"`
	X      float64 `json:"x,omitempty"`
	Y      float64 `json:"y,omitempty"`
	Depth  int     `json:"depth,omitempty"`
}
type graphEdge struct {
	From string `json:"from"`
	To   string `json:"to"`
}
type graphResponse struct {
	Nodes []graphNode `json:"nodes"`
	Edges []graphEdge `json:"edges"`
}

func (s *Server) handleGetGraph(w http.ResponseWriter, r *http.Request) {
	sessionID := r.PathValue("session_id")
	orgID := getOrgID(r)
	engine, _, err := s.getEngine(sessionID, orgID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusForbidden)
		return
	}
	maxNodes, _ := strconv.Atoi(r.URL.Query().Get("max_nodes"))
	if maxNodes <= 0 || maxNodes > 5000 {
		maxNodes = 500
	}

	facts := engine.ListFacts()
	if len(facts) > maxNodes {
		facts = facts[:maxNodes]
	}

	nodes := make([]graphNode, 0, len(facts))
	edges := make([]graphEdge, 0)
	parentToChildren := map[string][]string{}
	for _, f := range facts {
		nodes = append(nodes, graphNode{ID: f.ID, Status: float64(engine.GetStatus(f.ID)), IsRoot: f.IsRoot})
		for _, js := range f.JustificationSets {
			for _, parent := range js {
				edges = append(edges, graphEdge{From: f.ID, To: parent})
				parentToChildren[parent] = append(parentToChildren[parent], f.ID)
			}
		}
	}

	// Deterministic layout based on depth from roots (derived from fact.IsRoot).
	depth := map[string]int{}
	queue := []string{}
	for _, n := range nodes {
		if n.IsRoot {
			depth[n.ID] = 0
			queue = append(queue, n.ID)
		}
	}
	for len(queue) > 0 {
		id := queue[0]
		queue = queue[1:]
		d := depth[id]
		for _, child := range parentToChildren[id] {
			if existing, ok := depth[child]; ok && existing <= d+1 {
				continue
			}
			depth[child] = d + 1
			queue = append(queue, child)
		}
	}
	maxDepth := 0
	levels := map[int][]string{}
	for _, n := range nodes {
		d := depth[n.ID]
		levels[d] = append(levels[d], n.ID)
		if d > maxDepth {
			maxDepth = d
		}
	}
	for d := 0; d <= maxDepth; d++ {
		sort.Strings(levels[d])
	}
	pos := map[string]struct{ x, y float64 }{}
	for d := 0; d <= maxDepth; d++ {
		ids := levels[d]
		if len(ids) == 0 {
			continue
		}
		for i, id := range ids {
			x := float64(i+1) / float64(len(ids)+1)
			y := float64(d+1) / float64(maxDepth+2)
			pos[id] = struct{ x, y float64 }{x: x, y: y}
		}
	}
	for i := range nodes {
		d := depth[nodes[i].ID]
		nodes[i].Depth = d
		if p, ok := pos[nodes[i].ID]; ok {
			nodes[i].X = p.x
			nodes[i].Y = p.y
		}
	}
	writeJSON(w, http.StatusOK, graphResponse{Nodes: nodes, Edges: edges})
}

type searchResult struct {
	Type      string                 `json:"type"`
	SessionID string                 `json:"session_id,omitempty"`
	FactID    string                 `json:"fact_id,omitempty"`
	Timestamp int64                  `json:"timestamp,omitempty"`
	Snippet   string                 `json:"snippet,omitempty"`
	Payload   map[string]interface{} `json:"payload,omitempty"`
}

func (s *Server) handleOrgSearch(w http.ResponseWriter, r *http.Request) {
	orgID := getOrgID(r)
	query := strings.TrimSpace(r.URL.Query().Get("q"))
	if query == "" {
		writeJSON(w, http.StatusOK, map[string]interface{}{"items": []searchResult{}, "next_cursor": ""})
		return
	}
	filter := store.SearchDocumentsFilter{
		Query:        query,
		DocumentType: strings.TrimSpace(r.URL.Query().Get("type")),
		Status:       strings.TrimSpace(r.URL.Query().Get("status")),
		SubjectRef:   strings.TrimSpace(r.URL.Query().Get("subject")),
		Cursor:       strings.TrimSpace(r.URL.Query().Get("cursor")),
		Limit:        50,
	}
	if limit, err := strconv.Atoi(r.URL.Query().Get("limit")); err == nil && limit > 0 && limit <= 200 {
		filter.Limit = limit
	}
	items, next, err := s.Store.SearchDocuments(orgID, filter)
	if err != nil {
		http.Error(w, "failed to search documents", http.StatusInternalServerError)
		return
	}
	results := make([]searchResult, 0, len(items))
	for _, doc := range items {
		results = append(results, searchResultFromDocument(doc))
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"items": results, "next_cursor": next})
}

func (s *Server) handleOrgActivity(w http.ResponseWriter, r *http.Request) {
	orgID := getOrgID(r)
	cursor := strings.TrimSpace(r.URL.Query().Get("cursor"))
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	org, _ := s.Store.GetOrganization(orgID)
	retDays := orgSettingInt(org, "retention_days_activity", 30)
	cutoff := time.Now().Add(-time.Duration(retDays) * 24 * time.Hour).UnixMilli()

	items, next, err := s.Store.ListOrgActivityPage(orgID, cursor, limit)
	if err != nil {
		http.Error(w, "failed to list activity", http.StatusInternalServerError)
		return
	}
	filtered := make([]store.JournalEntry, 0, len(items))
	hitCutoff := false
	for _, it := range items {
		if it.Timestamp > 0 && it.Timestamp < cutoff {
			hitCutoff = true
			continue
		}
		filtered = append(filtered, it)
	}
	if hitCutoff {
		next = ""
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"items": filtered, "next_cursor": next})
}

func (s *Server) handleListAccessLogs(w http.ResponseWriter, r *http.Request) {
	orgID := getOrgID(r)
	cursor := strings.TrimSpace(r.URL.Query().Get("cursor"))
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	org, _ := s.Store.GetOrganization(orgID)
	retDays := orgSettingInt(org, "retention_days_access_logs", 30)
	cutoff := time.Now().Add(-time.Duration(retDays) * 24 * time.Hour).UnixMilli()

	items, next, err := s.Store.ListAccessLogsPage(orgID, cursor, limit)
	if err != nil {
		http.Error(w, "failed to list access logs", http.StatusInternalServerError)
		return
	}
	filtered := make([]store.AccessLogEntry, 0, len(items))
	hitCutoff := false
	for _, it := range items {
		if it.CreatedAt > 0 && it.CreatedAt < cutoff {
			hitCutoff = true
			continue
		}
		filtered = append(filtered, it)
	}
	if hitCutoff {
		next = ""
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"items": filtered, "next_cursor": next})
}

type createSessionRequest struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

// Add an optional way to create a session record without asserting a fact.
func (s *Server) handleCreateSession(w http.ResponseWriter, r *http.Request) {
	orgID := getOrgID(r)
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	id := "s_" + hex.EncodeToString(b)

	var body createSessionRequest
	_ = json.NewDecoder(r.Body).Decode(&body)
	body.Name = strings.TrimSpace(body.Name)
	body.Description = strings.TrimSpace(body.Description)

	if err := s.Store.SetSessionOrganization(id, orgID); err != nil {
		http.Error(w, "failed to create session", http.StatusInternalServerError)
		return
	}
	now := time.Now().UnixMilli()
	_ = s.Store.UpsertOrgSessionIndex(orgID, id, now)
	if body.Name != "" || body.Description != "" {
		_ = s.Store.PatchOrgSessionMeta(orgID, id, body.Name, body.Description)
	}
	go s.Store.IncrementOrgMetric(orgID, "sessions_created", 1)

	config := &store.SessionConfig{EnforcementMode: "strict"}
	_ = s.Store.UpsertSearchDocuments([]store.SearchDocument{sessionSearchDocument(orgID, id, config, now)})
	writeJSON(w, http.StatusCreated, map[string]interface{}{"id": id, "name": body.Name, "description": body.Description})
}

type patchSessionRequest struct {
	Name        *string `json:"name,omitempty"`
	Description *string `json:"description,omitempty"`
}

func (s *Server) handlePatchSession(w http.ResponseWriter, r *http.Request) {
	orgID := getOrgID(r)
	sessionID := r.PathValue("id")
	if sessionID == "" {
		http.Error(w, "missing session id", http.StatusBadRequest)
		return
	}
	storedOrg, err := s.Store.GetSessionOrganization(sessionID)
	if err != nil || storedOrg != orgID {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	var body patchSessionRequest
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}
	sessions, _, _ := s.Store.ListOrgSessions(orgID, "", 200)
	var current store.OrgSessionMeta
	for _, sess := range sessions {
		if sess.ID == sessionID {
			current = sess
			break
		}
	}
	name := current.Name
	description := current.Description
	if body.Name != nil {
		name = strings.TrimSpace(*body.Name)
	}
	if body.Description != nil {
		description = strings.TrimSpace(*body.Description)
	}
	if err := s.Store.PatchOrgSessionMeta(orgID, sessionID, name, description); err != nil {
		http.Error(w, "failed to update session", http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"id": sessionID, "name": name, "description": description})
}

func (s *Server) handleDeleteSession(w http.ResponseWriter, r *http.Request) {
	orgID := getOrgID(r)
	sessionID := r.PathValue("id")
	if sessionID == "" {
		http.Error(w, "missing session id", http.StatusBadRequest)
		return
	}
	storedOrg, err := s.Store.GetSessionOrganization(sessionID)
	if err != nil || storedOrg != orgID {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	if err := s.Store.DeleteOrgSession(orgID, sessionID); err != nil {
		http.Error(w, "failed to archive session", http.StatusInternalServerError)
		return
	}
	s.mu.Lock()
	delete(s.Engines, sessionID)
	delete(s.Configs, sessionID)
	delete(s.Versions, sessionID)
	delete(s.LastAccess, sessionID)
	delete(s.SliceCache, sessionID)
	s.mu.Unlock()
	_ = s.Store.AppendOrgActivity(orgID, store.JournalEntry{
		Type:      store.EventAdminAction,
		SessionID: sessionID,
		ActorID:   getActorID(r),
		Payload:   map[string]interface{}{"action": "session_archived", "session_id": sessionID},
		Timestamp: time.Now().UnixMilli(),
	})
	writeJSON(w, http.StatusOK, map[string]string{"status": "archived", "id": sessionID})
}

// Minimal endpoint to support backend-driven "health map" style stats per session.
func (s *Server) handleGetSessionSummary(w http.ResponseWriter, r *http.Request) {
	sessionID := r.PathValue("session_id")
	orgID := getOrgID(r)
	engine, cfg, err := s.getEngine(sessionID, orgID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusForbidden)
		return
	}
	facts := engine.ListFacts()
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"id":               sessionID,
		"fact_count":       len(facts),
		"enforcement_mode": cfg.EnforcementMode,
		"schema_set":       cfg.Schema != "",
		"status":           "loaded",
	})
}

// Ensure compiler uses core import (avoid unused when building without graph handlers in some configs).
var _ = core.ConfidenceThreshold

func (s *Server) handleGetOrgSettings(w http.ResponseWriter, r *http.Request) {
	orgID := getOrgID(r)
	org, err := s.Store.GetOrganization(orgID)
	if err != nil {
		http.Error(w, "org not found", http.StatusNotFound)
		return
	}
	if org.Settings == nil {
		org.Settings = map[string]interface{}{}
	}
	// Defaults (returned, not necessarily persisted).
	if _, ok := org.Settings["retention_days_activity"]; !ok {
		org.Settings["retention_days_activity"] = 30
	}
	if _, ok := org.Settings["retention_days_access_logs"]; !ok {
		org.Settings["retention_days_access_logs"] = 30
	}
	if _, ok := org.Settings["retention_days_notifications"]; !ok {
		org.Settings["retention_days_notifications"] = 30
	}
	if _, ok := org.Settings["rate_limit_rpm"]; !ok {
		org.Settings["rate_limit_rpm"] = 60
	}
	if _, ok := org.Settings["rate_limit_window_seconds"]; !ok {
		org.Settings["rate_limit_window_seconds"] = 60
	}
	writeJSON(w, http.StatusOK, org.Settings)
}

func (s *Server) handlePatchOrgSettings(w http.ResponseWriter, r *http.Request) {
	if getUserRole(r) != "admin" {
		http.Error(w, "forbidden: admin role required", http.StatusForbidden)
		return
	}
	orgID := getOrgID(r)
	org, err := s.Store.GetOrganization(orgID)
	if err != nil {
		http.Error(w, "org not found", http.StatusNotFound)
		return
	}
	var body map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}
	if org.Settings == nil {
		org.Settings = map[string]interface{}{}
	}
	allowed := map[string]bool{
		"retention_days_activity":      true,
		"retention_days_access_logs":   true,
		"retention_days_notifications": true,
		"rate_limit_rpm":               true,
		"rate_limit_window_seconds":    true,
	}
	for k := range body {
		if !allowed[k] {
			http.Error(w, "unknown setting: "+k, http.StatusBadRequest)
			return
		}
	}
	// Validate retention values (1..3650 days).
	for _, key := range []string{"retention_days_activity", "retention_days_access_logs", "retention_days_notifications"} {
		if v, ok := body[key]; ok {
			n, ok := asInt(v)
			if !ok || n < 1 || n > 3650 {
				http.Error(w, "invalid "+key+" (expected integer 1..3650)", http.StatusBadRequest)
				return
			}
		}
	}
	for _, key := range []string{"rate_limit_rpm", "rate_limit_window_seconds"} {
		if v, ok := body[key]; ok {
			n, ok := asInt(v)
			if !ok || n < 1 || n > 100000 {
				http.Error(w, "invalid "+key+" (expected integer 1..100000)", http.StatusBadRequest)
				return
			}
		}
	}
	for k, v := range body {
		org.Settings[k] = v
	}
	if err := s.Store.SaveOrganization(org); err != nil {
		http.Error(w, "failed to persist", http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, org.Settings)
}

func asInt(v interface{}) (int, bool) {
	switch t := v.(type) {
	case float64:
		return int(t), true
	case int:
		return t, true
	case int64:
		return int(t), true
	case json.Number:
		i, err := t.Int64()
		return int(i), err == nil
	default:
		return 0, false
	}
}

func orgSettingInt(org *store.Organization, key string, def int) int {
	if org == nil || org.Settings == nil {
		return def
	}
	v, ok := org.Settings[key]
	if !ok {
		return def
	}
	if n, ok := asInt(v); ok {
		return n
	}
	return def
}

func (s *Server) handleGetBilling(w http.ResponseWriter, r *http.Request) {
	orgID := getOrgID(r)
	sub, err := s.Store.GetBilling(orgID)
	if err != nil {
		// default
		writeJSON(w, http.StatusOK, store.BillingSubscription{
			Plan:         "free",
			Status:       "active",
			BillingEmail: getUserEmail(r),
			UpdatedAt:    time.Now().UnixMilli(),
		})
		return
	}
	writeJSON(w, http.StatusOK, sub)
}

type patchBillingRequest struct {
	Plan         *string `json:"plan,omitempty"`
	Status       *string `json:"status,omitempty"`
	BillingEmail *string `json:"billing_email,omitempty"`
}

func (s *Server) handlePatchBilling(w http.ResponseWriter, r *http.Request) {
	if getUserRole(r) != "admin" {
		http.Error(w, "forbidden: admin role required", http.StatusForbidden)
		return
	}
	orgID := getOrgID(r)
	sub, _ := s.Store.GetBilling(orgID)
	if sub == nil {
		sub = &store.BillingSubscription{Plan: "free", Status: "active", BillingEmail: getUserEmail(r)}
	}
	var body patchBillingRequest
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}
	if body.Plan != nil {
		sub.Plan = strings.TrimSpace(*body.Plan)
	}
	if body.Status != nil {
		sub.Status = strings.TrimSpace(*body.Status)
	}
	if body.BillingEmail != nil {
		sub.BillingEmail = strings.TrimSpace(*body.BillingEmail)
	}
	sub.UpdatedAt = time.Now().UnixMilli()
	if err := s.Store.SaveBilling(orgID, sub); err != nil {
		http.Error(w, "failed to persist", http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, sub)
}

func (s *Server) handleListOrgUsers(w http.ResponseWriter, r *http.Request) {
	if getUserRole(r) != "admin" {
		http.Error(w, "forbidden: admin role required", http.StatusForbidden)
		return
	}
	orgID := getOrgID(r)
	emails, err := s.Store.ListOrgUsers(orgID)
	if err != nil {
		http.Error(w, "failed to list users", http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"items": emails})
}

type createInviteRequest struct {
	Email string `json:"email"`
	Role  string `json:"role"`
}

type invitationView struct {
	ID         string `json:"id"`
	Email      string `json:"email"`
	Role       string `json:"role"`
	CreatedAt  int64  `json:"created_at"`
	ExpiresAt  int64  `json:"expires_at"`
	AcceptedAt int64  `json:"accepted_at,omitempty"`
	RevokedAt  int64  `json:"revoked_at,omitempty"`
}

func sanitizeInvitation(inv *store.Invitation) invitationView {
	return invitationView{
		ID:         inv.ID,
		Email:      inv.Email,
		Role:       inv.Role,
		CreatedAt:  inv.CreatedAt,
		ExpiresAt:  inv.ExpiresAt,
		AcceptedAt: inv.AcceptedAt,
		RevokedAt:  inv.RevokedAt,
	}
}

func (s *Server) handleCreateInvitation(w http.ResponseWriter, r *http.Request) {
	if getUserRole(r) != "admin" {
		http.Error(w, "forbidden: admin role required", http.StatusForbidden)
		return
	}
	orgID := getOrgID(r)
	var body createInviteRequest
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}
	body.Email = strings.TrimSpace(strings.ToLower(body.Email))
	if body.Email == "" {
		http.Error(w, "email required", http.StatusBadRequest)
		return
	}
	role := strings.TrimSpace(body.Role)
	if role == "" {
		role = "member"
	}
	if role != "admin" && role != "member" && role != "auditor" {
		http.Error(w, "invalid role", http.StatusBadRequest)
		return
	}
	now := time.Now().UnixMilli()
	token := randID(8)
	inv := &store.Invitation{
		ID:        "inv_" + randID(6),
		Email:     body.Email,
		Role:      role,
		Token:     token,
		CreatedAt: now,
		ExpiresAt: time.Now().Add(7 * 24 * time.Hour).UnixMilli(),
	}
	if err := s.Store.SaveInvitation(orgID, inv); err != nil {
		http.Error(w, "failed to persist", http.StatusInternalServerError)
		return
	}
	_ = s.Store.AppendOrgActivity(orgID, store.JournalEntry{
		Type:    store.EventAdminAction,
		ActorID: getActorID(r),
		Payload: map[string]interface{}{
			"action":        "invitation_created",
			"invitation_id": inv.ID,
			"email":         inv.Email,
			"role":          inv.Role,
			"expires_at":    inv.ExpiresAt,
		},
		Timestamp: now,
	})
	writeJSON(w, http.StatusCreated, map[string]interface{}{
		"invitation":   sanitizeInvitation(inv),
		"invite_token": token,
	})
}

func (s *Server) handleListInvitations(w http.ResponseWriter, r *http.Request) {
	if getUserRole(r) != "admin" {
		http.Error(w, "forbidden: admin role required", http.StatusForbidden)
		return
	}
	orgID := getOrgID(r)
	list, err := s.Store.ListInvitations(orgID)
	if err != nil {
		http.Error(w, "failed to list invites", http.StatusInternalServerError)
		return
	}
	items := make([]invitationView, 0, len(list))
	for i := range list {
		items = append(items, sanitizeInvitation(&list[i]))
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"items": items})
}

func (s *Server) handleRevokeInvitation(w http.ResponseWriter, r *http.Request) {
	if getUserRole(r) != "admin" {
		http.Error(w, "forbidden: admin role required", http.StatusForbidden)
		return
	}
	orgID := getOrgID(r)
	id := r.PathValue("id")
	inv, err := s.Store.GetInvitation(orgID, id)
	if err != nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	inv.RevokedAt = time.Now().UnixMilli()
	if err := s.Store.UpdateInvitation(orgID, inv); err != nil {
		http.Error(w, "failed to persist", http.StatusInternalServerError)
		return
	}
	_ = s.Store.AppendOrgActivity(orgID, store.JournalEntry{
		Type:    store.EventAdminAction,
		ActorID: getActorID(r),
		Payload: map[string]interface{}{
			"action":        "invitation_revoked",
			"invitation_id": inv.ID,
			"email":         inv.Email,
		},
		Timestamp: inv.RevokedAt,
	})
	writeJSON(w, http.StatusOK, map[string]interface{}{"status": "revoked", "invitation": sanitizeInvitation(inv)})
}

type acceptInviteRequest struct {
	Token    string `json:"token"`
	Password string `json:"password"`
}

func (s *Server) handleAcceptInvitation(w http.ResponseWriter, r *http.Request) {
	var body acceptInviteRequest
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}
	if body.Token == "" || len(body.Password) < 8 {
		http.Error(w, "token and password required", http.StatusBadRequest)
		return
	}
	orgID, inv, err := s.Store.GetInvitationByToken(body.Token)
	if err != nil || inv == nil {
		http.Error(w, "invalid invite token", http.StatusUnauthorized)
		return
	}
	now := time.Now().UnixMilli()
	if inv.RevokedAt != 0 || inv.AcceptedAt != 0 || now > inv.ExpiresAt {
		_ = s.Store.AppendOrgActivity(orgID, store.JournalEntry{
			Type:    store.EventAdminAction,
			ActorID: "system",
			Payload: map[string]interface{}{
				"action":        "invitation_accept_rejected",
				"invitation_id": inv.ID,
				"email":         inv.Email,
				"reason":        "expired_or_revoked",
			},
			Timestamp: now,
		})
		http.Error(w, "invite expired or revoked", http.StatusUnauthorized)
		return
	}
	if existingUser, _ := s.Store.GetUser(inv.Email); existingUser != nil {
		http.Error(w, "user already exists", http.StatusConflict)
		return
	}

	hashed, err := hashPassword(body.Password)
	if err != nil {
		http.Error(w, "hashing failure", http.StatusInternalServerError)
		return
	}
	user := &store.User{
		Email:          inv.Email,
		HashedPassword: hashed,
		OrgID:          orgID,
		Role:           inv.Role,
		Keys:           []store.APIKey{},
	}
	if err := s.Store.SaveUser(user); err != nil {
		http.Error(w, "failed to create user", http.StatusInternalServerError)
		return
	}
	inv.AcceptedAt = now
	_ = s.Store.UpdateInvitation(orgID, inv)
	_ = s.Store.AppendOrgActivity(orgID, store.JournalEntry{
		Type:    store.EventAdminAction,
		ActorID: inv.Email,
		Payload: map[string]interface{}{
			"action":        "invitation_accepted",
			"invitation_id": inv.ID,
			"email":         inv.Email,
			"role":          inv.Role,
		},
		Timestamp: now,
	})
	writeJSON(w, http.StatusOK, map[string]string{"status": "accepted"})
}

type ticketCreateRequest struct {
	Subject string `json:"subject"`
	Body    string `json:"body"`
}

func (s *Server) handleListTickets(w http.ResponseWriter, r *http.Request) {
	orgID := getOrgID(r)
	list, err := s.Store.ListTickets(orgID)
	if err != nil {
		http.Error(w, "failed to list tickets", http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"items": list})
}

func (s *Server) handleCreateTicket(w http.ResponseWriter, r *http.Request) {
	orgID := getOrgID(r)
	var body ticketCreateRequest
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}
	body.Subject = strings.TrimSpace(body.Subject)
	body.Body = strings.TrimSpace(body.Body)
	if body.Subject == "" || body.Body == "" {
		http.Error(w, "subject and body required", http.StatusBadRequest)
		return
	}
	now := time.Now().UnixMilli()
	t := &store.SupportTicket{
		ID:        "t_" + randID(7),
		Subject:   body.Subject,
		Body:      body.Body,
		Status:    "open",
		CreatedAt: now,
		UpdatedAt: now,
		CreatedBy: getActorID(r),
	}
	if err := s.Store.SaveTicket(orgID, t); err != nil {
		http.Error(w, "failed to persist", http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusCreated, t)
}

type ticketPatchRequest struct {
	Status *string `json:"status,omitempty"`
}

func (s *Server) handlePatchTicket(w http.ResponseWriter, r *http.Request) {
	orgID := getOrgID(r)
	id := r.PathValue("id")
	t, err := s.Store.GetTicket(orgID, id)
	if err != nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	var body ticketPatchRequest
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}
	if body.Status != nil {
		st := strings.TrimSpace(*body.Status)
		if st != "open" && st != "closed" {
			http.Error(w, "invalid status", http.StatusBadRequest)
			return
		}
		t.Status = st
	}
	t.UpdatedAt = time.Now().UnixMilli()
	if err := s.Store.SaveTicket(orgID, t); err != nil {
		http.Error(w, "failed to persist", http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, t)
}

func (s *Server) handleListPolicies(w http.ResponseWriter, r *http.Request) {
	orgID := getOrgID(r)
	list, err := s.Store.ListPolicies(orgID)
	if err != nil {
		http.Error(w, "failed to list policies", http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"items": list})
}

type policyCreateRequest struct {
	Name    string                 `json:"name"`
	Enabled bool                   `json:"enabled"`
	Rules   map[string]interface{} `json:"rules,omitempty"`
}

func (s *Server) handleCreatePolicy(w http.ResponseWriter, r *http.Request) {
	if getUserRole(r) != "admin" {
		http.Error(w, "forbidden: admin role required", http.StatusForbidden)
		return
	}
	orgID := getOrgID(r)
	var body policyCreateRequest
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}
	body.Name = strings.TrimSpace(body.Name)
	if body.Name == "" {
		http.Error(w, "name required", http.StatusBadRequest)
		return
	}
	now := time.Now().UnixMilli()
	p := &store.Policy{
		ID:        "pol_" + randID(8),
		Name:      body.Name,
		Enabled:   body.Enabled,
		Rules:     body.Rules,
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := s.Store.SavePolicy(orgID, p); err != nil {
		http.Error(w, "failed to persist", http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusCreated, p)
}

type policyPatchRequest struct {
	Name    *string                `json:"name,omitempty"`
	Enabled *bool                  `json:"enabled,omitempty"`
	Rules   map[string]interface{} `json:"rules,omitempty"`
}

func (s *Server) handlePatchPolicy(w http.ResponseWriter, r *http.Request) {
	if getUserRole(r) != "admin" {
		http.Error(w, "forbidden: admin role required", http.StatusForbidden)
		return
	}
	orgID := getOrgID(r)
	id := r.PathValue("id")
	p, err := s.Store.GetPolicy(orgID, id)
	if err != nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	var body policyPatchRequest
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}
	if body.Name != nil {
		n := strings.TrimSpace(*body.Name)
		if n == "" {
			http.Error(w, "name cannot be empty", http.StatusBadRequest)
			return
		}
		p.Name = n
	}
	if body.Enabled != nil {
		p.Enabled = *body.Enabled
	}
	if body.Rules != nil {
		p.Rules = body.Rules
	}
	p.UpdatedAt = time.Now().UnixMilli()
	if err := s.Store.SavePolicy(orgID, p); err != nil {
		http.Error(w, "failed to persist", http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, p)
}

func (s *Server) handleDeletePolicy(w http.ResponseWriter, r *http.Request) {
	if getUserRole(r) != "admin" {
		http.Error(w, "forbidden: admin role required", http.StatusForbidden)
		return
	}
	orgID := getOrgID(r)
	id := r.PathValue("id")
	if err := s.Store.DeletePolicy(orgID, id); err != nil {
		http.Error(w, "failed to delete", http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

func (s *Server) handleLegalTerms(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"content": "Terms are not yet defined for this deployment."})
}

func (s *Server) handleLegalPrivacy(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"content": "Privacy policy is not yet defined for this deployment."})
}
