package tests

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"sync"
	"testing"

	"velarix/core"
)

func TestStressAssertions(t *testing.T) {
	server, _ := setupTestServer(t)
	sessionID := "stress_session"

	var wg sync.WaitGroup
	workers := 10
	requestsPerWorker := 10
	if raw := os.Getenv("VELARIX_STRESS_WORKERS"); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil && parsed > 0 {
			workers = parsed
		}
	}
	if raw := os.Getenv("VELARIX_STRESS_REQUESTS_PER_WORKER"); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil && parsed > 0 {
			requestsPerWorker = parsed
		}
	}

	errCh := make(chan error, workers*requestsPerWorker)

	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			for j := 0; j < requestsPerWorker; j++ {
				factID := fmt.Sprintf("fact_%d_%d", workerID, j)
				fact := core.Fact{
					ID:           factID,
					IsRoot:       true,
					ManualStatus: core.Valid,
					Payload:      map[string]interface{}{"worker": workerID, "req": j},
				}
				body, _ := json.Marshal(fact)
				resp := performRequest(t, server, http.MethodPost, "/v1/s/"+sessionID+"/facts", body)
				if resp.Code != http.StatusCreated {
					errCh <- fmt.Errorf("unexpected status code: %d", resp.Code)
				}
			}
		}(i)
	}

	wg.Wait()
	close(errCh)

	for err := range errCh {
		if err != nil {
			t.Errorf("Stress test failed with error: %v", err)
		}
	}
}
