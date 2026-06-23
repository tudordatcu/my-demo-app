package httpapi

import (
	"net/http"

	"github.com/vodafone/vois-speechmark-demo/internal/auth"
)

// NewRouter is the single place routes are registered for the whole service.
// The middleware chain (outermost → innermost) is:
//
//	requestID → logging → recover → cors → [per-route: metrics → (auth?)]
//
// Per-route metrics and the auth wrapper are applied inside the mux so that
// the Prometheus "route" label is populated with the matched pattern rather
// than the empty string that a global metrics layer would see before routing.
func NewRouter(d Deps) http.Handler {
	mux := http.NewServeMux()

	authMW := auth.APIKeyMiddleware(d.Cfg, d.Logger)

	// authed wraps a handler with per-route metrics AND the API key middleware.
	authed := func(pattern string, h http.HandlerFunc) {
		mux.Handle(pattern, d.metrics(pattern, authMW(h)))
	}
	// unauthed wraps a handler with per-route metrics only (no auth).
	unauthed := func(pattern string, h http.Handler) {
		mux.Handle(pattern, d.metrics(pattern, h))
	}

	// Operational routes — unauthenticated.
	unauthed("GET /healthz", http.HandlerFunc(d.handleHealthz))
	unauthed("GET /readyz", http.HandlerFunc(d.handleReadyz))
	unauthed("GET /metrics", d.Metrics.Handler())

	// Plan routes.
	authed("POST /v1/plans", d.handleCreatePlan)
	authed("GET /v1/plans", d.handleListPlans)
	authed("GET /v1/plans/{id}", d.handleGetPlan)

	// Subscriber routes.
	authed("POST /v1/subscribers", d.handleCreateSubscriber)
	authed("GET /v1/subscribers", d.handleListSubscribers)
	authed("GET /v1/subscribers/{id}", d.handleGetSubscriber)
	authed("PATCH /v1/subscribers/{id}/status", d.handleChangeStatus)
	authed("POST /v1/subscribers/{id}/usage", d.handleAddUsage)
	authed("GET /v1/subscribers/{id}/invoice", d.handleGetInvoice)
	authed("GET /v1/subscribers/{id}/invoice/export", d.handleInvoiceExport)
	authed("DELETE /v1/subscribers/{id}", d.handleDeleteSubscriber)
	authed("PATCH /v1/subscribers/{id}/plan", d.handleChangePlan)
	authed("POST /v1/webhooks", d.handleRegisterWebhook)
	authed("GET /v1/diagnostics/ping", d.handlePing)
	authed("GET /v1/redirect", d.handleRedirect)

	// Admin — registered without the auth middleware.
	unauthed("POST /admin/reset", http.HandlerFunc(d.handleAdminReset))

	// Global chain: requestID → logging → recover → cors → mux.
	return chain(mux,
		requestID,
		d.logging,
		d.recover,
		cors,
	)
}
