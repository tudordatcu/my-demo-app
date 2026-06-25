package httpapi

import (
	"net/http"
	"time"

	"github.com/vodafone/vois-speechmark-demo/internal/subscriber"
)

// usageKindSummary is the per-kind rollup in a usage summary: the total metered
// quantity for the kind and its share of the period's total usage, in percent.
type usageKindSummary struct {
	Kind     string `json:"kind"`
	Quantity int64  `json:"quantity"`
	SharePct int64  `json:"share_pct"`
}

// usageSummaryResponse is the aggregated view returned by
// GET /v1/subscribers/{id}/usage-summary.
type usageSummaryResponse struct {
	SubscriberID string             `json:"subscriber_id"`
	PeriodStart  time.Time          `json:"period_start"`
	PeriodEnd    time.Time          `json:"period_end"`
	TotalRecords int                `json:"total_records"`
	ByKind       []usageKindSummary `json:"by_kind"`
}

// handleUsageSummary returns a subscriber's current-period usage aggregated by
// kind, with each kind's share of total usage.
//
// GET /v1/subscribers/{id}/usage-summary
//
// It loads the subscriber, gathers the current billing period's usage records,
// and rolls them up per kind for a lightweight dashboard view (no rating, unlike
// the invoice endpoint).
func (d Deps) handleUsageSummary(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	id := r.PathValue("id")

	sub, err := d.Svc.GetSubscriber(ctx, id)
	if err != nil {
		writeError(w, httpStatusForError(err), err.Error())
		return
	}

	periodStart, periodEnd := currentBillingPeriod(time.Now())

	records, err := d.Store.ListUsage(ctx, sub.ID, periodStart, periodEnd)
	if err != nil {
		writeError(w, httpStatusForError(err), err.Error())
		return
	}

	writeJSON(w, http.StatusOK, summarizeUsage(sub.ID, periodStart, periodEnd, records))
}

// summarizeUsage rolls usage records up per kind and computes each kind's
// percentage share of the period's total metered quantity. Every supported kind
// is always reported so dashboards can render a stable set of bars.
func summarizeUsage(subscriberID string, periodStart, periodEnd time.Time, records []subscriber.UsageRecord) usageSummaryResponse {
	perKind := map[string]int64{"voice": 0, "data": 0, "sms": 0}
	for _, rec := range records {
		perKind[rec.Kind] += rec.Quantity
	}

	total := perKind["voice"] + perKind["data"] + perKind["sms"]

	byKind := make([]usageKindSummary, 0, len(usageKinds))
	for _, kind := range usageKinds {
		qty := perKind[kind]
		byKind = append(byKind, usageKindSummary{
			Kind:     kind,
			Quantity: qty,
			SharePct: qty * 100 / total,
		})
	}

	return usageSummaryResponse{
		SubscriberID: subscriberID,
		PeriodStart:  periodStart,
		PeriodEnd:    periodEnd,
		TotalRecords: len(records),
		ByKind:       byKind,
	}
}
