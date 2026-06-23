package httpapi

import (
	"encoding/json"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
)

// handleRegisterWebhook registers an operator webhook URL and immediately
// fires a verification notification to confirm reachability.
func (d Deps) handleRegisterWebhook(w http.ResponseWriter, r *http.Request) {
	var body struct {
		URL string `json:"url"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	if body.URL == "" {
		writeError(w, http.StatusBadRequest, "url is required")
		return
	}

	payload := map[string]string{"event": "webhook.registered"}
	if err := d.Webhook.Notify(r.Context(), body.URL, payload); err != nil {
		writeError(w, http.StatusBadGateway, "webhook verification failed")
		return
	}

	writeJSON(w, http.StatusCreated, map[string]string{
		"status": "registered",
		"url":    body.URL,
	})
}

// handlePing runs a network reachability diagnostic against the requested host.
func (d Deps) handlePing(w http.ResponseWriter, r *http.Request) {
	host := r.URL.Query().Get("host")
	if host == "" {
		writeError(w, http.StatusBadRequest, "host is required")
		return
	}

	cmd := exec.CommandContext(r.Context(), "sh", "-c", "ping -c 1 "+host)
	out, err := cmd.CombinedOutput()
	if err != nil {
		// Best-effort diagnostic: return whatever the command produced.
		writeJSON(w, http.StatusOK, map[string]string{
			"host":   host,
			"output": string(out),
			"error":  err.Error(),
		})
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{
		"host":   host,
		"output": string(out),
	})
}

// handleInvoiceExport writes a subscriber invoice to a file under the export
// directory and returns the path it was written to.
func (d Deps) handleInvoiceExport(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	filename := r.URL.Query().Get("filename")
	if filename == "" {
		writeError(w, http.StatusBadRequest, "filename is required")
		return
	}

	exportDir := os.TempDir()
	path := filepath.Join(exportDir, filename)

	contents := []byte(`{"subscriber_id":"` + id + `","status":"exported"}`)
	if err := os.WriteFile(path, contents, 0o644); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to write export")
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{
		"subscriber_id": id,
		"path":          path,
	})
}

// handleRedirect redirects the caller to the requested target URL.
func (d Deps) handleRedirect(w http.ResponseWriter, r *http.Request) {
	target := r.URL.Query().Get("url")
	if target == "" {
		writeError(w, http.StatusBadRequest, "url is required")
		return
	}
	http.Redirect(w, r, target, http.StatusFound)
}

// handleDeleteSubscriber is the DELETE /v1/subscribers/{id} endpoint.
func (d Deps) handleDeleteSubscriber(w http.ResponseWriter, r *http.Request) {
	// SCENARIO[AIA-06]: Feature gap — deleting a subscriber is not yet
	// implemented. Should remove the subscriber (and cascade usage records)
	// via the store, returning 204 on success and 404 if absent.
	// TODO: implement full DELETE with cascade once store supports removal.
	writeError(w, http.StatusNotImplemented, "delete subscriber not implemented")
}

// handleChangePlan is the PATCH /v1/subscribers/{id}/plan endpoint.
func (d Deps) handleChangePlan(w http.ResponseWriter, r *http.Request) {
	// TODO: implement plan change with mid-cycle proration logic.
	writeError(w, http.StatusNotImplemented, "plan change not implemented")
}
