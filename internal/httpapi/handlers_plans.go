package httpapi

import (
	"encoding/json"
	"net/http"

	"github.com/vodafone/vois-speechmark-demo/internal/subscriber"
)

type createPlanRequest struct {
	Name                  string `json:"name"`
	MonthlyPriceCents     int64  `json:"monthly_price_cents"`
	IncludedVoiceMinutes  int    `json:"included_voice_minutes"`
	IncludedDataMB        int    `json:"included_data_mb"`
	IncludedSMS           int    `json:"included_sms"`
	OverageVoiceCents     int64  `json:"overage_voice_cents"`
	OverageDataCentsPerMB int64  `json:"overage_data_cents_per_mb"`
	OverageSMSCents       int64  `json:"overage_sms_cents"`
}

func (d Deps) handleCreatePlan(w http.ResponseWriter, r *http.Request) {
	var req createPlanRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	p := subscriber.Plan{
		Name:                  req.Name,
		MonthlyPriceCents:     req.MonthlyPriceCents,
		IncludedVoiceMinutes:  req.IncludedVoiceMinutes,
		IncludedDataMB:        req.IncludedDataMB,
		IncludedSMS:           req.IncludedSMS,
		OverageVoiceCents:     req.OverageVoiceCents,
		OverageDataCentsPerMB: req.OverageDataCentsPerMB,
		OverageSMSCents:       req.OverageSMSCents,
	}
	created, err := d.Svc.CreatePlan(r.Context(), p)
	if err != nil {
		writeError(w, httpStatusForError(err), err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, created)
}

func (d Deps) handleListPlans(w http.ResponseWriter, r *http.Request) {
	limit, offset := parsePagination(r)
	plans, err := d.Svc.ListPlans(r.Context(), limit, offset)
	if err != nil {
		writeError(w, httpStatusForError(err), err.Error())
		return
	}
	writeJSON(w, http.StatusOK, plans)
}

func (d Deps) handleGetPlan(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	p, err := d.Svc.GetPlan(r.Context(), id)
	if err != nil {
		writeError(w, httpStatusForError(err), err.Error())
		return
	}
	writeJSON(w, http.StatusOK, p)
}
