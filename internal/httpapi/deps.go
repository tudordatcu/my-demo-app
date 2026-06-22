package httpapi

import (
	"log/slog"

	"go.opentelemetry.io/otel/trace"

	"github.com/vodafone/vois-speechmark-demo/internal/config"
	"github.com/vodafone/vois-speechmark-demo/internal/observability"
	"github.com/vodafone/vois-speechmark-demo/internal/subscriber"
	"github.com/vodafone/vois-speechmark-demo/internal/webhook"
)

// Deps is the dependency set threaded into every HTTP handler. Per the
// mandatory convention, EVERY handler in this package is a method on Deps:
//
//	func (d Deps) handleXxx(w http.ResponseWriter, r *http.Request)
//
// Defined exactly once here. No other file in package httpapi redefines it.
type Deps struct {
	Svc     *subscriber.Service
	Store   subscriber.Store
	Metrics *observability.Metrics
	Tracer  trace.Tracer
	Logger  *slog.Logger
	Cfg     config.Config
	Webhook *webhook.Client
	Carrier *CarrierClient
}
