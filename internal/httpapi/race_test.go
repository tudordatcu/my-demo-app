//go:build race

package httpapi

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
)

// TestConcurrentCreateSubscriber fires many concurrent POSTs through the real
// router via a live httptest.Server. TST-04 / SEC-17: run with -race to observe
// the data race in the in-memory store's unguarded maps and the service's
// non-atomic ID counter. Without -race the test passes (it only asserts HTTP
// success), which is the intended CI signal: functional suite green, race
// detector red on the planted store race.
func TestConcurrentCreateSubscriber(t *testing.T) {
	d := testDeps(t)
	srv := httptest.NewServer(NewRouter(d))
	defer srv.Close()

	client := srv.Client()

	// Seed a plan to reference.
	planPayload := `{"name":"Basic","monthly_price_cents":1000}`
	req, _ := http.NewRequest(http.MethodPost, srv.URL+"/v1/plans", strings.NewReader(planPayload))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer test-key")
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("seed plan request: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("seed plan: got %d, want 201", resp.StatusCode)
	}

	// Re-fetch the plan ID by listing plans.
	listReq, _ := http.NewRequest(http.MethodGet, srv.URL+"/v1/plans", nil)
	listReq.Header.Set("Authorization", "Bearer test-key")
	listResp, err := client.Do(listReq)
	if err != nil {
		t.Fatalf("list plans: %v", err)
	}
	var plans []struct {
		ID string `json:"ID"`
	}
	_ = json.NewDecoder(listResp.Body).Decode(&plans)
	listResp.Body.Close()
	if len(plans) == 0 {
		t.Fatal("list plans: expected at least one plan")
	}
	planID := plans[0].ID

	const n = 32
	var wg sync.WaitGroup
	wg.Add(n)
	for i := 0; i < n; i++ {
		go func(i int) {
			defer wg.Done()
			payload := fmt.Sprintf(
				`{"msisdn":"+1555%07d","imsi":"31015%010d","name":"racer","plan_id":%q,"owner_account_id":"acct-1"}`,
				i, i, planID,
			)
			r, _ := http.NewRequest(http.MethodPost, srv.URL+"/v1/subscribers", strings.NewReader(payload))
			r.Header.Set("Content-Type", "application/json")
			r.Header.Set("Authorization", "Bearer test-key")
			res, err := client.Do(r)
			if err != nil {
				t.Errorf("concurrent create #%d: request error: %v", i, err)
				return
			}
			res.Body.Close()
			if res.StatusCode != http.StatusCreated {
				t.Errorf("concurrent create #%d: got %d, want 201", i, res.StatusCode)
			}
		}(i)
	}
	wg.Wait()
}
