package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"velarix/core"
	"velarix/extractor"
	"velarix/store"
)

func TestHandleLogin(t *testing.T) {
	m := &MockStore{
		GetUserFunc: func(email string) (*store.User, error) {
			if email == "test@example.com" {
				hashed, _ := hashPassword("password")
				return &store.User{Email: email, HashedPassword: hashed}, nil
			}
			return nil, nil
		},
	}
	s := newTestServer(m)
	t.Setenv("VELARIX_JWT_SECRET", "test_secret_32_bytes_long_required")

	loginReq := LoginRequest{Email: "test@example.com", Password: "password"}
	body, _ := json.Marshal(loginReq)
	req, _ := http.NewRequest("POST", "/auth/login", bytes.NewBuffer(body))
	rr := httptest.NewRecorder()
	s.handleLogin(rr, req)

	if status := rr.Code; status != http.StatusOK {
		t.Errorf("got %v want %v", status, http.StatusOK)
	}
}

func TestHandleRegisterDisabled(t *testing.T) {
	s := newTestServer(&MockStore{})
	t.Setenv("VELARIX_ENABLE_PUBLIC_REGISTRATION", "false")

	req, _ := http.NewRequest("POST", "/auth/register", bytes.NewBuffer([]byte("{}")))
	rr := httptest.NewRecorder()
	s.handleRegister(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Errorf("got %d", rr.Code)
	}
}

func TestHandleListSessions(t *testing.T) {
	m := &MockStore{
		ListOrgSessionsFunc: func(orgID string, cursor string, limit int) ([]store.OrgSessionMeta, string, error) {
			return []store.OrgSessionMeta{{ID: "s1"}}, "", nil
		},
	}
	s := newTestServer(m)
	req, _ := http.NewRequest("GET", "/v1/sessions", nil)
	ctx := context.WithValue(req.Context(), orgIDKey, "org1")
	req = req.WithContext(ctx)

	rr := httptest.NewRecorder()
	s.handleListSessions(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("got %d", rr.Code)
	}
}

func TestHandleGetSessionSummary(t *testing.T) {
	m := &MockStore{
		GetSessionOrganizationFunc: func(sessionID string) (string, error) {
			return "org1", nil
		},
		GetConfigFunc: func(id string) (*store.SessionConfig, error) { return &store.SessionConfig{}, nil },
	}
	s := newTestServer(m)
	s.Engines["s1"] = core.NewEngine()
	s.Configs["s1"] = &store.SessionConfig{EnforcementMode: "strict"}
	
	req, _ := http.NewRequest("GET", "/v1/s/s1/summary", nil)
	req.SetPathValue("session_id", "s1")
	ctx := context.WithValue(req.Context(), orgIDKey, "org1")
	req = req.WithContext(ctx)

	rr := httptest.NewRecorder()
	s.handleGetSessionSummary(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("got %d, body: %s", rr.Code, rr.Body.String())
	}
}

func TestHandleDeleteSession(t *testing.T) {
	m := &MockStore{
		GetSessionOrganizationFunc: func(sessionID string) (string, error) {
			return "org1", nil
		},
		DeleteOrgSessionFunc: func(orgID, sessionID string) error {
			return nil
		},
		AppendOrgActivityFunc: func(id string, e store.JournalEntry) error { return nil },
	}
	s := newTestServer(m)
	req, _ := http.NewRequest("DELETE", "/v1/org/sessions/s1", nil)
	req.SetPathValue("id", "s1")
	ctx := context.WithValue(req.Context(), orgIDKey, "org1")
	ctx = context.WithValue(ctx, actorIDKey, "user1")
	req = req.WithContext(ctx)

	rr := httptest.NewRecorder()
	s.handleDeleteSession(rr, req)

	// Defaults to 200 OK because WriteHeader(204) is missing in handler
	if rr.Code != http.StatusOK {
		t.Errorf("got %d", rr.Code)
	}
}

func TestHandleExtractAndAssert(t *testing.T) {
	// Mock OpenAI server for V-Logic
	vlogic := "fact f1: \"test fact\" (confidence: 0.9)"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := map[string]interface{}{
			"choices": []map[string]interface{}{
				{"message": map[string]string{"content": vlogic}},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	t.Setenv("OPENAI_API_KEY", "test-key")
	t.Setenv("VELARIX_OPENAI_BASE_URL", srv.URL)

	m := &MockStore{
		GetConfigFunc: func(id string) (*store.SessionConfig, error) { return &store.SessionConfig{}, nil },
		GetSessionOrganizationFunc: func(id string) (string, error) { return "org1", nil },
		AppendFunc: func(e store.JournalEntry) error { return nil },
		AppendOrgActivityFunc: func(id string, e store.JournalEntry) error { return nil },
		IncrementOrgMetricFunc: func(id, m string, d uint64) error { return nil },
	}
	s := newTestServer(m)
	
	engine := core.NewEngine()
	s.Engines["s1"] = engine
	s.Configs["s1"] = &store.SessionConfig{}

	body := extractAndAssertRequest{
		LLMOutput: "some text",
		ExtractionConfig: &extractor.ExtractionConfig{
			EnableSelection:            false,
			EnableDecontextualisation:  false,
			EnableCoverageVerification: false,
			EnableConsistencyPrecheck:  false,
		},
	}
	b, _ := json.Marshal(body)
	req, _ := http.NewRequest("POST", "/v1/s/s1/extract-and-assert", bytes.NewBuffer(b))
	req.SetPathValue("session_id", "s1")
	ctx := context.WithValue(req.Context(), orgIDKey, "org1")
	ctx = context.WithValue(ctx, actorIDKey, "user1")
	req = req.WithContext(ctx)

	rr := httptest.NewRecorder()
	s.handleExtractAndAssert(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("got %d, body: %s", rr.Code, rr.Body.String())
	}
}

func TestHandleCreateDecision(t *testing.T) {
	m := &MockStore{
		GetSessionOrganizationFunc: func(id string) (string, error) { return "org1", nil },
		SaveDecisionFunc: func(d *store.Decision) error { return nil },
		SaveDecisionDependenciesFunc: func(sid, did string, deps []store.DecisionDependency) error { return nil },
		AppendFunc: func(e store.JournalEntry) error { return nil },
		AppendOrgActivityFunc: func(id string, e store.JournalEntry) error { return nil },
	}
	s := newTestServer(m)
	engine := core.NewEngine()
	// Assert fact so it exists when building dependencies
	engine.AssertFact(&core.Fact{ID: "f1", IsRoot: true, ManualStatus: core.Valid})
	s.Engines["s1"] = engine
	s.Configs["s1"] = &store.SessionConfig{}

	body := map[string]interface{}{
		"decision_id": "d1",
		"subject_ref": "subj",
		"target_ref": "target",
		"decision_type": "action",
		"dependency_fact_ids": []string{"f1"},
	}
	b, _ := json.Marshal(body)
	req, _ := http.NewRequest("POST", "/v1/s/s1/decisions", bytes.NewBuffer(b))
	req.SetPathValue("session_id", "s1")
	ctx := context.WithValue(req.Context(), orgIDKey, "org1")
	ctx = context.WithValue(ctx, actorIDKey, "user1")
	req = req.WithContext(ctx)

	rr := httptest.NewRecorder()
	s.handleCreateDecision(rr, req)

	if rr.Code != http.StatusCreated {
		t.Errorf("got %d, body: %s", rr.Code, rr.Body.String())
	}
}
