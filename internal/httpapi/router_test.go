package httpapi

import (
	"bytes"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	"go.opentelemetry.io/otel/trace/noop"

	"github.com/vodafone/vois-speechmark-demo/internal/config"
	"github.com/vodafone/vois-speechmark-demo/internal/observability"
	"github.com/vodafone/vois-speechmark-demo/internal/store"
	"github.com/vodafone/vois-speechmark-demo/internal/subscriber"
	"github.com/vodafone/vois-speechmark-demo/internal/webhook"
)

func testDeps(t *testing.T) Deps {
	t.Helper()
	st := store.NewMemoryStore()
	cfg := config.Config{APIKey: "test-key"}
	return Deps{
		Svc:     subscriber.NewService(st),
		Store:   st,
		Metrics: observability.NewMetrics(prometheus.NewRegistry()),
		Tracer:  noop.NewTracerProvider().Tracer("test"),
		Logger:  slog.New(slog.NewTextHandler(io.Discard, nil)),
		Cfg:     cfg,
		Webhook: webhook.New(),
		Carrier: NewCarrierClient(0),
	}
}

// authReq builds a request carrying the test bearer token.
func authReq(t *testing.T, method, target string, body []byte) *http.Request {
	t.Helper()
	var r *http.Request
	if body != nil {
		r = httptest.NewRequest(method, target, bytes.NewReader(body))
		r.Header.Set("Content-Type", "application/json")
	} else {
		r = httptest.NewRequest(method, target, nil)
	}
	r.Header.Set("Authorization", "Bearer test-key")
	return r
}

func TestHealthz(t *testing.T) {
	router := NewRouter(testDeps(t))
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/healthz", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("healthz: got status %d, want 200", rec.Code)
	}
}

func TestCreateGetListSubscriber(t *testing.T) {
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
		"msisdn": "+15551230001", "imsi": "310150123456789",
		"name": "Alice", "plan_id": plan.ID, "owner_account_id": "acct-1",
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
	if created.ID == "" {
		t.Fatal("create subscriber: empty ID")
	}

	// Get it back.
	getRec := httptest.NewRecorder()
	router.ServeHTTP(getRec, authReq(t, http.MethodGet, "/v1/subscribers/"+created.ID, nil))
	if getRec.Code != http.StatusOK {
		t.Fatalf("get subscriber: got %d, want 200; body=%s", getRec.Code, getRec.Body.String())
	}
	var fetched subscriber.Subscriber
	if err := json.Unmarshal(getRec.Body.Bytes(), &fetched); err != nil {
		t.Fatalf("decode fetched: %v", err)
	}
	if fetched.ID != created.ID {
		t.Fatalf("get subscriber: got id %q, want %q", fetched.ID, created.ID)
	}

	// List.
	listRec := httptest.NewRecorder()
	router.ServeHTTP(listRec, authReq(t, http.MethodGet, "/v1/subscribers", nil))
	if listRec.Code != http.StatusOK {
		t.Fatalf("list subscribers: got %d, want 200; body=%s", listRec.Code, listRec.Body.String())
	}
	var list []subscriber.Subscriber
	if err := json.Unmarshal(listRec.Body.Bytes(), &list); err != nil {
		t.Fatalf("decode list: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("list subscribers: got %d, want 1", len(list))
	}
}

func TestGetSubscriberNotFound(t *testing.T) {
	router := NewRouter(testDeps(t))
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, authReq(t, http.MethodGet, "/v1/subscribers/does-not-exist", nil))
	if rec.Code != http.StatusNotFound {
		t.Fatalf("get missing: got %d, want 404", rec.Code)
	}
}

func TestAuthRequiredOnV1(t *testing.T) {
	router := NewRouter(testDeps(t))
	rec := httptest.NewRecorder()
	// No Authorization header -> auth middleware rejects.
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/v1/subscribers", nil))
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("unauth /v1: got %d, want 401", rec.Code)
	}
}
