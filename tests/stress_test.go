package tests

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"testing"

	"velarix/core"
)

func TestStressAssertions(t *testing.T) {
	server := setupTestServer(t)
	sessionID := "stress_session"

	var wg sync.WaitGroup
	workers := 50
	requestsPerWorker := 20

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
