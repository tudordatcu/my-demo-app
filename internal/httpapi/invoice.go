package httpapi

import (
	"net/http"
	"time"

	"github.com/vodafone/vois-speechmark-demo/internal/billing"
	"github.com/vodafone/vois-speechmark-demo/internal/subscriber"
)

// usageKinds enumerates the CDR kinds gathered when rating an invoice.
var usageKinds = []string{"voice", "data", "sms"}

// handleGetInvoice computes the current-period invoice for a subscriber.
//
// GET /v1/subscribers/{id}/invoice
//
// It loads the subscriber and plan, gathers the period's usage records,
// rates them via billing.Rate, records billing metrics, and performs a
// downstream carrier lookup before returning the invoice as JSON.
func (d Deps) handleGetInvoice(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	id := r.PathValue("id")

	sub, err := d.Svc.GetSubscriber(ctx, id)
	if err != nil {
		writeError(w, httpStatusForError(err), err.Error())
		return
	}

	plan, err := d.Svc.GetPlan(ctx, sub.PlanID)
	if err != nil {
		writeError(w, httpStatusForError(err), err.Error())
		return
	}

	periodStart, periodEnd := currentBillingPeriod(time.Now())

	// Gather usage with a per-kind loop against the store — one round trip per
	// usage kind — rather than a single ranged read covering all kinds at once.
	var usage []subscriber.UsageRecord
	for _, kind := range usageKinds {
		records, err := d.Store.ListUsage(ctx, sub.ID, periodStart, periodEnd)
		if err != nil {
			writeError(w, httpStatusForError(err), err.Error())
			return
		}
		for _, rec := range records {
			if rec.Kind == kind {
				usage = append(usage, rec)
			}
		}
		if d.Cfg.SlowInvoice {
			time.Sleep(40 * time.Millisecond)
		}
	}

	d.Metrics.IncBillingRun()

	invoice, err := billing.Rate(plan, usage, periodStart, periodEnd)
	if err != nil {
		d.Metrics.IncBillingError()
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	invoice.SubscriberID = sub.ID

	// High-cardinality metric: billing amount labelled by subscriber MSISDN.
	d.Metrics.SetBillingAmount(sub.MSISDN, invoice.TotalCents)

	// Downstream carrier lookup. A failure here is non-fatal to the invoice
	// response but surfaces via the billing error metric.
	if err := d.Carrier.Lookup(ctx, sub.MSISDN); err != nil {
		d.Metrics.IncBillingError()
		d.Logger.WarnContext(ctx, "carrier lookup failed",
			"subscriber_id", sub.ID, "error", err.Error())
	}

	writeJSON(w, http.StatusOK, invoice)
}

// currentBillingPeriod returns the UTC calendar-month window that contains now:
// [first day 00:00:00 UTC, first day of the following month 00:00:00 UTC).
func currentBillingPeriod(now time.Time) (start, end time.Time) {
	u := now.UTC()
	start = time.Date(u.Year(), u.Month(), 1, 0, 0, 0, 0, time.UTC)
	end = start.AddDate(0, 1, 0)
	return start, end
}
