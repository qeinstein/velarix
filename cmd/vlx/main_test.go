package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCLI_RunStatus_HitsHealthEndpoint(t *testing.T) {
	var hitPath string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hitPath = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"healthy"}`))
	}))
	defer server.Close()

	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	cli := &cli{
		baseURL:    server.URL,
		apiKey:     "token",
		httpClient: server.Client(),
		stdout:     stdout,
		stderr:     stderr,
	}

	if err := cli.run([]string{"status"}); err != nil {
		t.Fatalf("status command failed: %v", err)
	}
	if hitPath != "/health" {
		t.Fatalf("expected /health, got %s", hitPath)
	}
	if !bytes.Contains(stdout.Bytes(), []byte(`"status": "healthy"`)) {
		t.Fatalf("unexpected stdout: %s", stdout.String())
	}
}

func TestCLI_RunSlice_BuildsQueryParameters(t *testing.T) {
	var query string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		query = r.URL.RawQuery
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[]`))
	}))
	defer server.Close()

	cli := &cli{
		baseURL:    server.URL,
		apiKey:     "token",
		httpClient: server.Client(),
		stdout:     &bytes.Buffer{},
		stderr:     &bytes.Buffer{},
	}

	err := cli.run([]string{
		"slice",
		"--session", "sess_1",
		"--query", "invoice risk",
		"--strategy", "hybrid",
		"--max-facts", "7",
		"--max-chars", "900",
	})
	if err != nil {
		t.Fatalf("slice command failed: %v", err)
	}

	expectedFragments := []string{
		"query=invoice+risk",
		"strategy=hybrid",
		"max_facts=7",
		"max_chars=900",
		"include_dependencies=true",
	}
	for _, fragment := range expectedFragments {
		if !bytes.Contains([]byte(query), []byte(fragment)) {
			t.Fatalf("expected query to contain %q, got %q", fragment, query)
		}
	}
}

func TestCLI_RunReview_PostsJSON(t *testing.T) {
	var authHeader string
	var payload map[string]interface{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authHeader = r.Header.Get("Authorization")
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("failed to decode body: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"fact_1","review_status":"approved"}`))
	}))
	defer server.Close()

	stdout := &bytes.Buffer{}
	cli := &cli{
		baseURL:    server.URL,
		apiKey:     "test_key",
		httpClient: server.Client(),
		stdout:     stdout,
		stderr:     &bytes.Buffer{},
	}

	err := cli.run([]string{
		"review",
		"--session", "sess_2",
		"--fact", "fact_1",
		"--status", "approved",
		"--reason", "human verified",
	})
	if err != nil {
		t.Fatalf("review command failed: %v", err)
	}
	if authHeader != "Bearer test_key" {
		t.Fatalf("expected authorization header to be set, got %q", authHeader)
	}
	if payload["status"] != "approved" {
		t.Fatalf("expected approved status payload, got %+v", payload)
	}
	if payload["reason"] != "human verified" {
		t.Fatalf("expected reason payload, got %+v", payload)
	}
}
