package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"velarix/core"
	"velarix/store"
)

func newTestServer(m *MockStore) *Server {
	return &Server{
		Store:      m,
		Engines:    make(map[string]*core.Engine),
		Configs:    make(map[string]*store.SessionConfig),
		Versions:   make(map[string]int64),
		LastAccess: make(map[string]time.Time),
		StartTime:  time.Now(),
	}
}

func TestHandleHealth(t *testing.T) {
	s := newTestServer(&MockStore{})
	req, _ := http.NewRequest("GET", "/health", nil)
	rr := httptest.NewRecorder()
	s.handleHealth(rr, req)

	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v want %v", status, http.StatusOK)
	}
}

func TestHandleFullHealth(t *testing.T) {
	s := newTestServer(&MockStore{})
	req, _ := http.NewRequest("GET", "/health/full", nil)
	ctx := context.WithValue(req.Context(), userRoleKey, "admin")
	req = req.WithContext(ctx)
	rr := httptest.NewRecorder()
	s.handleFullHealth(rr, req)

	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v want %v", status, http.StatusOK)
	}
}

func TestHandleMe(t *testing.T) {
	s := newTestServer(&MockStore{})
	req, _ := http.NewRequest("GET", "/v1/me", nil)
	// Add context values that authMiddleware would add
	ctx := context.WithValue(req.Context(), actorIDKey, "user@example.com")
	ctx = context.WithValue(ctx, userRoleKey, "admin")
	req = req.WithContext(ctx)

	rr := httptest.NewRecorder()
	s.handleMe(rr, req)

	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v want %v", status, http.StatusOK)
	}
}

func TestHandleGetOrg(t *testing.T) {
	m := &MockStore{
		GetOrganizationFunc: func(id string) (*store.Organization, error) {
			return &store.Organization{ID: id, Name: "Test Org"}, nil
		},
	}
	s := newTestServer(m)
	req, _ := http.NewRequest("GET", "/v1/org", nil)
	ctx := context.WithValue(req.Context(), orgIDKey, "org123")
	req = req.WithContext(ctx)

	rr := httptest.NewRecorder()
	s.handleGetOrg(rr, req)

	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v want %v", status, http.StatusOK)
	}
}

func TestHandleLegal(t *testing.T) {
	s := newTestServer(&MockStore{})
	
	t.Run("Terms", func(t *testing.T) {
		req, _ := http.NewRequest("GET", "/v1/legal/terms", nil)
		rr := httptest.NewRecorder()
		s.handleLegalTerms(rr, req)
		if rr.Code != http.StatusOK { t.Errorf("got %d", rr.Code) }
	})

	t.Run("Privacy", func(t *testing.T) {
		req, _ := http.NewRequest("GET", "/v1/legal/privacy", nil)
		rr := httptest.NewRecorder()
		s.handleLegalPrivacy(rr, req)
		if rr.Code != http.StatusOK { t.Errorf("got %d", rr.Code) }
	})
}
