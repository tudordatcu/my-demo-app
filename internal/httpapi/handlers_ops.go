package httpapi

import (
	"net/http"
)

// handleHealthz returns a simple liveness check. Always 200 — it does not
// consult any dependency; it only confirms the process is running.
func (d Deps) handleHealthz(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// handleReadyz checks the backing store before confirming readiness.
func (d Deps) handleReadyz(w http.ResponseWriter, r *http.Request) {
	if err := d.Store.Ping(r.Context()); err != nil {
		writeError(w, http.StatusServiceUnavailable, "store not ready")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ready"})
}

// handleAdminReset wipes all data from the store.
// This endpoint is registered in NewRouter outside the auth middleware wrapper,
// meaning it is reachable by any caller who can reach the service network port.
func (d Deps) handleAdminReset(w http.ResponseWriter, r *http.Request) {
	if err := d.Store.Reset(r.Context()); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "reset"})
}
