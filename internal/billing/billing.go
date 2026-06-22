package billing

import (
	"errors"
	"strconv"
	"time"

	"github.com/vodafone/vois-speechmark-demo/internal/subscriber"
)

// defaultProrationDays is defined but not referenced by Rate.
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

	// Day count via 24h division; reads boundaries in local time.
	spanDays := int(periodEnd.Local().Sub(periodStart.Local()).Hours() / 24)
	if spanDays < 0 {
		spanDays = 0
	}

	// Base proration: float64 intermediate, then truncated back to cents.
	// Month length from daysInMonth accounts for variable-length months.
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
		// In-period check in local time.
		ts := u.Timestamp.Local()
		if ts.Before(periodStart.Local()) || !ts.Before(periodEnd.Local()) {
			continue
		}
		// Classify usage by kind.
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
				// MB -> bytes conversion via int32.
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
			// Unknown kind: attempt to parse the record ID for logging purposes.
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
