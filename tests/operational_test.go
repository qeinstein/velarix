package tests

import (
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"velarix/store"
)

type slowAppendStore struct {
	store.RuntimeStore
	started chan struct{}
	release chan struct{}
}

func (s *slowAppendStore) Append(entry store.JournalEntry) error {
	select {
	case s.started <- struct{}{}:
	default:
	}
	<-s.release
	return s.RuntimeStore.Append(entry)
}

func TestIdempotencyReplayPreventsDuplicateWrites(t *testing.T) {
	server, _ := setupTestServer(t)
	sessionID := "idempotency_session"
	body, _ := json.Marshal(map[string]interface{}{
		"id":            "idem_fact",
		"is_root":       true,
		"manual_status": 1.0,
	})

	headers := map[string]string{"Idempotency-Key": "fact-write-1"}
	first := performAuthenticatedRequestWithHeaders(t, server, http.MethodPost, "/v1/s/"+sessionID+"/facts", "test_key", body, headers)
	if first.Code != http.StatusCreated {
		t.Fatalf("expected first idempotent write to succeed, got %d body=%s", first.Code, first.Body.String())
	}

	second := performAuthenticatedRequestWithHeaders(t, server, http.MethodPost, "/v1/s/"+sessionID+"/facts", "test_key", body, headers)
	if second.Code != http.StatusCreated {
		t.Fatalf("expected replayed idempotent write to keep original status, got %d body=%s", second.Code, second.Body.String())
	}
	if second.Header().Get("X-Idempotency-Replay") != "true" {
		t.Fatalf("expected replay header on idempotent retry")
	}

	history := performAuthenticatedRequest(t, server, http.MethodGet, "/v1/s/"+sessionID+"/history", "test_key", nil)
	if history.Code != http.StatusOK {
		t.Fatalf("expected history fetch to succeed, got %d body=%s", history.Code, history.Body.String())
	}
	var entries []store.JournalEntry
	if err := json.NewDecoder(history.Body).Decode(&entries); err != nil {
		t.Fatalf("failed to decode history: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected exactly one persisted journal entry after replay, got %d", len(entries))
	}
}

func TestBackpressureReturnsRetryHeaders(t *testing.T) {
	t.Setenv("VELARIX_MAX_CONCURRENT_WRITES", "1")

	_, badgerStore := setupTestServer(t)
	slowStore := &slowAppendStore{
		RuntimeStore: badgerStore,
		started:      make(chan struct{}, 1),
		release:      make(chan struct{}),
	}
	server := newTestServerWithStore(slowStore)
	seedDefaultTestIdentity(server)

	bodyOne, _ := json.Marshal(map[string]interface{}{
		"id":            "slow_fact_one",
		"is_root":       true,
		"manual_status": 1.0,
	})
	bodyTwo, _ := json.Marshal(map[string]interface{}{
		"id":            "slow_fact_two",
		"is_root":       true,
		"manual_status": 1.0,
	})

	firstRecorder := make(chan int, 1)

	go func() {
		resp := performAuthenticatedRequest(t, server, http.MethodPost, "/v1/s/backpressure_session/facts", "test_key", bodyOne)
		firstRecorder <- resp.Code
	}()

	select {
	case <-slowStore.started:
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for first write to acquire backpressure slot")
	}

	second := performAuthenticatedRequest(t, server, http.MethodPost, "/v1/s/backpressure_session/facts", "test_key", bodyTwo)
	if second.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected backpressure response, got %d body=%s", second.Code, second.Body.String())
	}
	if second.Header().Get("Retry-After") != "1" {
		t.Fatalf("expected Retry-After=1, got %q", second.Header().Get("Retry-After"))
	}
	if second.Header().Get("X-Velarix-Backpressure") != "1" {
		t.Fatalf("expected X-Velarix-Backpressure header")
	}

	close(slowStore.release)

	select {
	case code := <-firstRecorder:
		if code != http.StatusCreated {
			t.Fatalf("expected first write to complete successfully, got %d", code)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for first write to complete")
	}
}
