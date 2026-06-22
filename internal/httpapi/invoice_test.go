package httpapi

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"go.opentelemetry.io/otel/trace/noop"

	"github.com/vodafone/vois-speechmark-demo/internal/config"
	"github.com/vodafone/vois-speechmark-demo/internal/observability"
	"github.com/vodafone/vois-speechmark-demo/internal/store"
	"github.com/vodafone/vois-speechmark-demo/internal/subscriber"
	"github.com/vodafone/vois-speechmark-demo/internal/webhook"
)

// newInvoiceTestDeps builds a fully-wired Deps backed by an in-memory store,
// a noop tracer, a discard logger, and a Carrier that NEVER fails (rate 0).
func newInvoiceTestDeps(t *testing.T) (Deps, *store.MemoryStore) {
	t.Helper()
	st := store.NewMemoryStore()
	cfg := config.Config{APIKey: "test-key"}
	d := Deps{
		Svc:     subscriber.NewService(st),
		Store:   st,
		Metrics: observability.NewMetrics(prometheus.NewRegistry()),
		Tracer:  noop.NewTracerProvider().Tracer("test"),
		Logger:  slog.New(slog.NewTextHandler(io.Discard, nil)),
		Cfg:     cfg,
		Webhook: webhook.New(),
		Carrier: NewCarrierClient(0),
	}
	return d, st
}

// seedInvoiceFixture creates a plan, an active subscriber, and one usage
// record that exceeds the plan's voice allowance so the invoice is non-zero.
func seedInvoiceFixture(t *testing.T, d Deps) subscriber.Subscriber {
	t.Helper()
	ctx := t.Context()
	plan, err := d.Svc.CreatePlan(ctx, subscriber.Plan{
		Name:                  "Test Plan",
		MonthlyPriceCents:     1000,
		IncludedVoiceMinutes:  100,
		IncludedDataMB:        1000,
		IncludedSMS:           100,
		OverageVoiceCents:     5,
		OverageDataCentsPerMB: 1,
		OverageSMSCents:       10,
	})
	if err != nil {
		t.Fatalf("CreatePlan: %v", err)
	}
	sub, err := d.Svc.CreateSubscriber(ctx, subscriber.CreateSubscriberInput{
		MSISDN:         "+15551230001",
		IMSI:           "001010000000001",
		Name:           "Invoice Tester",
		PlanID:         plan.ID,
		OwnerAccountID: "acct-1",
	})
	if err != nil {
		t.Fatalf("CreateSubscriber: %v", err)
	}
	// 150 voice minutes => 50 over the 100 included.
	if _, err := d.Svc.AddUsage(ctx, sub.ID, "voice", 150, time.Now().UTC()); err != nil {
		t.Fatalf("AddUsage: %v", err)
	}
	return sub
}

func TestHandleGetInvoice_OK(t *testing.T) {
	d, _ := newInvoiceTestDeps(t)
	router := NewRouter(d)
	sub := seedInvoiceFixture(t, d)

	req := httptest.NewRequest(http.MethodGet, "/v1/subscribers/"+sub.ID+"/invoice", nil)
	req.Header.Set("Authorization", "Bearer "+d.Cfg.APIKey)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status: got %d want 200, body=%s", rec.Code, rec.Body.String())
	}

	var inv subscriber.Invoice
	if err := json.Unmarshal(rec.Body.Bytes(), &inv); err != nil {
		t.Fatalf("decode invoice: %v, body=%s", err, rec.Body.String())
	}
	if inv.SubscriberID != sub.ID {
		t.Errorf("SubscriberID: got %q want %q", inv.SubscriberID, sub.ID)
	}
	if inv.TotalCents <= 0 {
		t.Errorf("TotalCents: got %d, want > 0", inv.TotalCents)
	}
	if len(inv.Lines) == 0 {
		t.Errorf("Lines: got 0, want at least one line item")
	}
	if inv.PeriodEnd.Before(inv.PeriodStart) {
		t.Errorf("period: end %v before start %v", inv.PeriodEnd, inv.PeriodStart)
	}
}

func TestHandleGetInvoice_NotFound(t *testing.T) {
	d, _ := newInvoiceTestDeps(t)
	router := NewRouter(d)

	req := httptest.NewRequest(http.MethodGet, "/v1/subscribers/does-not-exist/invoice", nil)
	req.Header.Set("Authorization", "Bearer "+d.Cfg.APIKey)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status: got %d want 404, body=%s", rec.Code, rec.Body.String())
	}
}

func TestHandleGetInvoice_Unauthorized(t *testing.T) {
	d, _ := newInvoiceTestDeps(t)
	router := NewRouter(d)
	sub := seedInvoiceFixture(t, d)

	// No Authorization header => auth middleware must reject before the handler.
	req := httptest.NewRequest(http.MethodGet, "/v1/subscribers/"+sub.ID+"/invoice", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status: got %d want 401, body=%s", rec.Code, rec.Body.String())
	}
}
