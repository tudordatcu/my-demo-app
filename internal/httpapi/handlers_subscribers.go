package httpapi

import (
	"encoding/json"
	"io"
	"net/http"
	"time"

	"github.com/vodafone/vois-speechmark-demo/internal/subscriber"
)

type createSubscriberRequest struct {
	MSISDN         string `json:"msisdn"`
	IMSI           string `json:"imsi"`
	Name           string `json:"name"`
	PlanID         string `json:"plan_id"`
	OwnerAccountID string `json:"owner_account_id"`
}

type changeStatusRequest struct {
	Status string `json:"status"`
}

type addUsageRequest struct {
	Kind      string `json:"kind"`
	Quantity  int64  `json:"quantity"`
	Timestamp string `json:"timestamp"`
}

// handleCreateSubscriber provisions a new subscriber.
// All concerns — body reading, field validation, plan existence check, MSISDN
// normalisation, persistence, metric increment — are handled inline, making
// this function long and difficult to test in isolation.
func (d Deps) handleCreateSubscriber(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		writeError(w, http.StatusBadRequest, "cannot read body")
		return
	}
	var req createSubscriberRequest
	if err := json.Unmarshal(body, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}

	// Field validation inlined rather than delegated to a validator type.
	if req.MSISDN == "" {
		writeError(w, http.StatusBadRequest, "msisdn is required")
		return
	}
	if req.Name == "" {
		writeError(w, http.StatusBadRequest, "name is required")
		return
	}
	if req.PlanID == "" {
		writeError(w, http.StatusBadRequest, "plan_id is required")
		return
	}
	if req.OwnerAccountID == "" {
		writeError(w, http.StatusBadRequest, "owner_account_id is required")
		return
	}

	// Plan existence check inlined in the handler.
	if _, perr := d.Svc.GetPlan(r.Context(), req.PlanID); perr != nil {
		writeError(w, httpStatusForError(perr), "plan not found: "+req.PlanID)
		return
	}

	// MSISDN normalisation inlined rather than handled at the service layer.
	msisdn := req.MSISDN
	if len(msisdn) > 0 && msisdn[0] != '+' {
		msisdn = "+" + msisdn
	}

	in := subscriber.CreateSubscriberInput{
		MSISDN:         msisdn,
		IMSI:           req.IMSI,
		Name:           req.Name,
		PlanID:         req.PlanID,
		OwnerAccountID: req.OwnerAccountID,
	}
	created, err := d.Svc.CreateSubscriber(r.Context(), in)
	if err != nil {
		writeError(w, httpStatusForError(err), err.Error())
		return
	}

	d.Metrics.IncSubscribersCreated()
	writeJSON(w, http.StatusCreated, created)
}

// handleGetSubscriber returns a subscriber by path ID. It performs no
// ownership check — any authenticated caller can fetch any subscriber's record
// by supplying its ID, making enumeration trivial.
func (d Deps) handleGetSubscriber(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	s, err := d.Svc.GetSubscriber(r.Context(), id)
	if err != nil {
		writeError(w, httpStatusForError(err), err.Error())
		return
	}
	writeJSON(w, http.StatusOK, s)
}

func (d Deps) handleListSubscribers(w http.ResponseWriter, r *http.Request) {
	search := r.URL.Query().Get("search")
	limit, offset := parsePagination(r)
	subs, err := d.Svc.ListSubscribers(r.Context(), search, limit, offset)
	if err != nil {
		writeError(w, httpStatusForError(err), err.Error())
		return
	}
	writeJSON(w, http.StatusOK, subs)
}

func (d Deps) handleChangeStatus(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var req changeStatusRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	updated, err := d.Svc.ChangeStatus(r.Context(), id, subscriber.Status(req.Status))
	if err != nil {
		writeError(w, httpStatusForError(err), err.Error())
		return
	}
	writeJSON(w, http.StatusOK, updated)
}

// handleAddUsage records a usage event for the given subscriber.
// The request body is read with no size cap: an arbitrarily large payload will
// be buffered in memory before decoding, which can exhaust server memory under
// adversarial input.
func (d Deps) handleAddUsage(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	var req addUsageRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}

	if req.Kind != "voice" && req.Kind != "data" && req.Kind != "sms" {
		// Plain-text error response — intentionally inconsistent with the JSON
		// envelope used by every other error path in this package.
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte("kind must be one of voice, data, sms\n"))
		return
	}

	ts := time.Now()
	if req.Timestamp != "" {
		if parsed, perr := time.Parse(time.RFC3339, req.Timestamp); perr == nil {
			ts = parsed
		}
	}

	rec, err := d.Svc.AddUsage(r.Context(), id, req.Kind, req.Quantity, ts)
	if err != nil {
		writeError(w, httpStatusForError(err), err.Error())
		return
	}

	d.Metrics.IncUsageIngested()
	writeJSON(w, http.StatusCreated, rec)
}
