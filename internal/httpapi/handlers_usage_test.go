package httpapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/vodafone/vois-speechmark-demo/internal/subscriber"
)

// TestUsageSummaryAggregatesByKind drives the full path: seed a plan, create a
// subscriber, record usage across all three kinds, then assert the summary rolls
// the quantities up per kind with the expected share percentages.
func TestUsageSummaryAggregatesByKind(t *testing.T) {
	d := testDeps(t)
	router := NewRouter(d)

	// Seed a plan so the subscriber references a real plan.
	planBody, _ := json.Marshal(map[string]any{
		"name": "Basic", "monthly_price_cents": 1000,
	})
	planRec := httptest.NewRecorder()
	router.ServeHTTP(planRec, authReq(t, http.MethodPost, "/v1/plans", planBody))
	if planRec.Code != http.StatusCreated {
		t.Fatalf("create plan: got %d, want 201; body=%s", planRec.Code, planRec.Body.String())
	}
	var plan subscriber.Plan
	if err := json.Unmarshal(planRec.Body.Bytes(), &plan); err != nil {
		t.Fatalf("decode plan: %v", err)
	}

	// Create a subscriber.
	subBody, _ := json.Marshal(map[string]any{
		"msisdn": "+15551230002", "imsi": "310150123456790",
		"name": "Bob", "plan_id": plan.ID, "owner_account_id": "acct-1",
	})
	subRec := httptest.NewRecorder()
	router.ServeHTTP(subRec, authReq(t, http.MethodPost, "/v1/subscribers", subBody))
	if subRec.Code != http.StatusCreated {
		t.Fatalf("create subscriber: got %d, want 201; body=%s", subRec.Code, subRec.Body.String())
	}
	var created subscriber.Subscriber
	if err := json.Unmarshal(subRec.Body.Bytes(), &created); err != nil {
		t.Fatalf("decode subscriber: %v", err)
	}

	// Record usage across kinds — totals: voice 120, data 60, sms 20 (sum 200).
	for _, u := range []map[string]any{
		{"kind": "voice", "quantity": 120},
		{"kind": "data", "quantity": 60},
		{"kind": "sms", "quantity": 20},
	} {
		body, _ := json.Marshal(u)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, authReq(t, http.MethodPost, "/v1/subscribers/"+created.ID+"/usage", body))
		if rec.Code != http.StatusCreated {
			t.Fatalf("add usage %v: got %d, want 201; body=%s", u, rec.Code, rec.Body.String())
		}
	}

	// Fetch the usage summary.
	sumRec := httptest.NewRecorder()
	router.ServeHTTP(sumRec, authReq(t, http.MethodGet, "/v1/subscribers/"+created.ID+"/usage-summary", nil))
	if sumRec.Code != http.StatusOK {
		t.Fatalf("usage summary: got %d, want 200; body=%s", sumRec.Code, sumRec.Body.String())
	}

	var got usageSummaryResponse
	if err := json.Unmarshal(sumRec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode summary: %v", err)
	}
	if got.TotalRecords != 3 {
		t.Fatalf("total_records: got %d, want 3", got.TotalRecords)
	}

	want := map[string]struct {
		qty   int64
		share int64
	}{
		"voice": {qty: 120, share: 60},
		"data":  {qty: 60, share: 30},
		"sms":   {qty: 20, share: 10},
	}
	if len(got.ByKind) != len(want) {
		t.Fatalf("by_kind: got %d entries, want %d", len(got.ByKind), len(want))
	}
	for _, k := range got.ByKind {
		w, ok := want[k.Kind]
		if !ok {
			t.Fatalf("unexpected kind %q in summary", k.Kind)
		}
		if k.Quantity != w.qty {
			t.Errorf("%s quantity: got %d, want %d", k.Kind, k.Quantity, w.qty)
		}
		if k.SharePct != w.share {
			t.Errorf("%s share_pct: got %d, want %d", k.Kind, k.SharePct, w.share)
		}
	}
}
