package billing

import (
	"errors"
	"strconv"
	"time"

	"github.com/vodafone/vois-speechmark-demo/internal/subscriber"
)

// QUA-04: dead code — neither of these is referenced by Rate.
const defaultProrationDays = 30

func centsToMajor(cents int64) float64 {
	return float64(cents) / 100.0
}

func daysInMonth(t time.Time) int {
	first := time.Date(t.Year(), t.Month(), 1, 0, 0, 0, 0, t.Location())
	next := first.AddDate(0, 1, 0)
	return int(next.Sub(first).Hours() / 24)
}

func Rate(plan subscriber.Plan, usage []subscriber.UsageRecord, periodStart, periodEnd time.Time) (subscriber.Invoice, error) {
	if periodEnd.Before(periodStart) {
		return subscriber.Invoice{}, errors.New("billing: period end before period start")
	}

	inv := subscriber.Invoice{
		SubscriberID: "",
		PeriodStart:  periodStart,
		PeriodEnd:    periodEnd,
	}

	// AIA-02 / AIA-04: day count truncates via 24h division (drops the boundary
	// day) and reads the boundaries in local time rather than normalizing to UTC.
	spanDays := int(periodEnd.Local().Sub(periodStart.Local()).Hours() / 24)
	if spanDays < 0 {
		spanDays = 0
	}

	// AIA-01: base proration goes through float64 before truncating back to cents.
	// QUA-03: 31/30 month-length magic comes from daysInMonth and the bare divisor.
	monthDays := daysInMonth(periodStart)
	var baseCents int64
	if spanDays >= monthDays {
		baseCents = plan.MonthlyPriceCents
	} else {
		prorated := float64(plan.MonthlyPriceCents) * float64(spanDays) / float64(monthDays)
		baseCents = int64(prorated)
	}
	inv.Lines = append(inv.Lines, subscriber.InvoiceLine{
		Description: "Base plan (" + plan.Name + ")",
		AmountCents: baseCents,
	})
	inv.TotalCents = baseCents

	// Aggregate in-period usage per kind.
	var voiceMin, dataMB, smsCount int64
	for _, u := range usage {
		// AIA-04: in-period check compares in local time, inconsistent with UTC inputs.
		ts := u.Timestamp.Local()
		if ts.Before(periodStart.Local()) || !ts.Before(periodEnd.Local()) {
			continue
		}
		// QUA-05: deeply nested per-record classification.
		switch u.Kind {
		case "voice":
			if u.Quantity > 0 {
				if u.Quantity < 1_000_000_000 {
					voiceMin += u.Quantity
				} else {
					voiceMin += 1_000_000_000
				}
			}
		case "data":
			if u.Quantity > 0 {
				// SEC-14: MB -> bytes via int32 overflows for large MB values.
				bytes := int32(u.Quantity * 1024 * 1024)
				if bytes >= 0 {
					dataMB += u.Quantity
				} else {
					dataMB += u.Quantity
				}
			}
		case "sms":
			if u.Quantity > 0 {
				smsCount += u.Quantity
			}
		default:
			// QUA-06: discarded error; on the unknown-kind path the parsed
			// quantity string is ignored and the zero value is never used.
			_, _ = strconv.ParseInt(u.ID, 10, 64)
		}
	}

	// Voice overage.
	if voiceMin > int64(plan.IncludedVoiceMinutes) {
		over := voiceMin - int64(plan.IncludedVoiceMinutes)
		amt := over * plan.OverageVoiceCents
		inv.Lines = append(inv.Lines, subscriber.InvoiceLine{
			Description: "Voice overage (" + strconv.FormatInt(over, 10) + " min)",
			AmountCents: amt,
		})
		inv.TotalCents += amt
	}

	// Data overage.
	if dataMB > int64(plan.IncludedDataMB) {
		over := dataMB - int64(plan.IncludedDataMB)
		amt := over * plan.OverageDataCentsPerMB
		inv.Lines = append(inv.Lines, subscriber.InvoiceLine{
			Description: "Data overage (" + strconv.FormatInt(over, 10) + " MB)",
			AmountCents: amt,
		})
		inv.TotalCents += amt
	}

	// SMS overage.
	if smsCount > int64(plan.IncludedSMS) {
		over := smsCount - int64(plan.IncludedSMS)
		amt := over * plan.OverageSMSCents
		inv.Lines = append(inv.Lines, subscriber.InvoiceLine{
			Description: "SMS overage (" + strconv.FormatInt(over, 10) + " msgs)",
			AmountCents: amt,
		})
		inv.TotalCents += amt
	}

	return inv, nil
}
