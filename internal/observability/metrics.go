package observability

import (
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// Metrics holds the Prometheus registry and the service's collectors.
type Metrics struct {
	Registry *prometheus.Registry

	httpRequestsTotal   *prometheus.CounterVec
	httpRequestDuration *prometheus.HistogramVec
	httpInFlight        prometheus.Gauge
	subscribersCreated  prometheus.Counter
	usageIngested       prometheus.Counter
	billingRuns         prometheus.Counter
	billingErrors       prometheus.Counter
	billingAmountCents  *prometheus.GaugeVec
}

// NewMetrics constructs and registers all collectors on the given registry.
func NewMetrics(reg *prometheus.Registry) *Metrics {
	m := &Metrics{
		Registry: reg,
		httpRequestsTotal: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "vois_http_requests_total",
			Help: "Total HTTP requests by method, route, and status.",
		}, []string{"method", "route", "status"}),
		httpRequestDuration: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Name:    "vois_http_request_duration_seconds",
			Help:    "HTTP request duration in seconds by route.",
			Buckets: prometheus.DefBuckets,
		}, []string{"route"}),
		httpInFlight: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "vois_http_in_flight_requests",
			Help: "Number of in-flight HTTP requests.",
		}),
		subscribersCreated: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "vois_subscribers_created_total",
			Help: "Total subscribers created.",
		}),
		usageIngested: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "vois_usage_records_ingested_total",
			Help: "Total usage records ingested.",
		}),
		billingRuns: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "vois_billing_runs_total",
			Help: "Total billing/invoice runs.",
		}),
		billingErrors: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "vois_billing_errors_total",
			Help: "Total billing/invoice errors.",
		}),
		// OBS-07: labelling a gauge by msisdn is a high-cardinality anti-pattern.
		billingAmountCents: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "vois_billing_amount_cents",
			Help: "Most recent invoice total in cents, labelled by msisdn.",
		}, []string{"msisdn"}),
	}

	reg.MustRegister(
		m.httpRequestsTotal,
		m.httpRequestDuration,
		m.httpInFlight,
		m.subscribersCreated,
		m.usageIngested,
		m.billingRuns,
		m.billingErrors,
		m.billingAmountCents,
	)
	return m
}

// Handler returns an http.Handler exposing this registry in Prometheus format.
func (m *Metrics) Handler() http.Handler {
	return promhttp.HandlerFor(m.Registry, promhttp.HandlerOpts{})
}

// ObserveRequest records a completed request's count and duration.
func (m *Metrics) ObserveRequest(route, method string, status int, dur time.Duration) {
	m.httpRequestsTotal.WithLabelValues(method, route, statusLabel(status)).Inc()
	m.httpRequestDuration.WithLabelValues(route).Observe(dur.Seconds())
}

// IncInFlight increments the in-flight request gauge.
func (m *Metrics) IncInFlight() { m.httpInFlight.Inc() }

// DecInFlight decrements the in-flight request gauge.
func (m *Metrics) DecInFlight() { m.httpInFlight.Dec() }

// IncSubscribersCreated counts a created subscriber.
func (m *Metrics) IncSubscribersCreated() { m.subscribersCreated.Inc() }

// IncUsageIngested counts an ingested usage record.
func (m *Metrics) IncUsageIngested() { m.usageIngested.Inc() }

// IncBillingRun counts a billing run.
func (m *Metrics) IncBillingRun() { m.billingRuns.Inc() }

// IncBillingError counts a billing error.
func (m *Metrics) IncBillingError() { m.billingErrors.Inc() }

// SetBillingAmount records the latest invoice total for a subscriber, labelled
// by msisdn (OBS-07 high-cardinality anti-pattern).
func (m *Metrics) SetBillingAmount(msisdn string, cents int64) {
	m.billingAmountCents.WithLabelValues(msisdn).Set(float64(cents))
}

func statusLabel(status int) string {
	switch {
	case status >= 500:
		return "5xx"
	case status >= 400:
		return "4xx"
	case status >= 300:
		return "3xx"
	case status >= 200:
		return "2xx"
	default:
		return "1xx"
	}
}
