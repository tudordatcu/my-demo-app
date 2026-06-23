package observability

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

func TestInitLoggerReturnsLogger(t *testing.T) {
	for _, lvl := range []string{"debug", "info", "warn", "error", "bogus"} {
		if got := InitLogger(lvl); got == nil {
			t.Errorf("InitLogger(%q) returned nil", lvl)
		}
	}
}

func TestInitTracerStdout(t *testing.T) {
	tr, shutdown, err := InitTracer(context.Background(), "")
	if err != nil {
		t.Fatalf("InitTracer stdout: %v", err)
	}
	if tr == nil {
		t.Fatal("InitTracer returned nil tracer")
	}
	if shutdown == nil {
		t.Fatal("InitTracer returned nil shutdown")
	}
	if err := shutdown(context.Background()); err != nil {
		t.Errorf("shutdown: %v", err)
	}
}

func TestNewMetricsAndHandler(t *testing.T) {
	m := NewMetrics(prometheus.NewRegistry())
	if m == nil {
		t.Fatal("NewMetrics returned nil")
	}
	if m.Registry == nil {
		t.Fatal("Metrics.Registry is nil")
	}
	if m.Handler() == nil {
		t.Fatal("Metrics.Handler returned nil")
	}
}

func TestMetricsIncrementsExposed(t *testing.T) {
	m := NewMetrics(prometheus.NewRegistry())

	// Exercise every increment path; none should panic.
	m.ObserveRequest("/v1/plans", "GET", 200, 12*time.Millisecond)
	m.IncInFlight()
	m.DecInFlight()
	m.IncSubscribersCreated()
	m.IncUsageIngested()
	m.IncBillingRun()
	m.IncBillingError()
	m.SetBillingAmount("+15551234567", 4242)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	m.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("metrics handler status = %d, want 200", rec.Code)
	}
	body := rec.Body.String()
	for _, name := range []string{
		"vois_http_requests_total",
		"vois_http_request_duration_seconds",
		"vois_http_in_flight_requests",
		"vois_subscribers_created_total",
		"vois_usage_records_ingested_total",
		"vois_billing_runs_total",
		"vois_billing_errors_total",
		"vois_billing_amount_cents",
	} {
		if !strings.Contains(body, name) {
			t.Errorf("metrics output missing %q", name)
		}
	}
}

func TestLoggerToDiscard(t *testing.T) {
	// Documents the test-harness constructor used elsewhere.
	l := slog.New(slog.NewTextHandler(io.Discard, nil))
	l.Info("smoke")
}
