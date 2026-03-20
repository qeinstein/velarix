package tests

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"testing"

	"velarix/core"
)

func TestStressAssertions(t *testing.T) {
	_, ts := setupTestServer(t)
	defer ts.Close()

	client := &http.Client{}
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
				req, _ := http.NewRequest("POST", fmt.Sprintf("%s/v1/s/%s/facts", ts.URL, sessionID), bytes.NewBuffer(body))
				req.Header.Set("Authorization", "Bearer test_admin_key")
				req.Header.Set("Content-Type", "application/json")

				resp, err := client.Do(req)
				if err != nil {
					errCh <- err
					continue
				}
				if resp.StatusCode != http.StatusCreated {
					errCh <- fmt.Errorf("unexpected status code: %d", resp.StatusCode)
				}
				resp.Body.Close()
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
