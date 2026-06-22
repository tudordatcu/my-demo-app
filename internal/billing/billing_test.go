package billing

import (
	"testing"
	"time"

	"github.com/vodafone/vois-speechmark-demo/internal/subscriber"
)

func testPlan() subscriber.Plan {
	return subscriber.Plan{
		ID:                    "plan-std",
		Name:                  "Standard",
		MonthlyPriceCents:     2000,
		IncludedVoiceMinutes:  100,
		IncludedDataMB:        1000,
		IncludedSMS:           50,
		OverageVoiceCents:     5,
		OverageDataCentsPerMB: 1,
		OverageSMSCents:       2,
	}
}

func usage(kind string, qty int64, ts time.Time) subscriber.UsageRecord {
	return subscriber.UsageRecord{ID: "u", SubscriberID: "s", Kind: kind, Quantity: qty, Timestamp: ts}
}

func TestRate(t *testing.T) {
	// Full-month period: Jan 1 .. Feb 1 2026 (UTC). The 24h-division day count
	// makes this a 31-day span counted as 31 here, so proration = full base.
	fullStart := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	fullEnd := time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC)
	inPeriod := time.Date(2026, 1, 15, 12, 0, 0, 0, time.UTC)

	tests := []struct {
		name        string
		plan        subscriber.Plan
		usage       []subscriber.UsageRecord
		start, end  time.Time
		wantTotal   int64
		wantLineLen int
	}{
		{
			name:        "full month, no usage, base only",
			plan:        testPlan(),
			usage:       nil,
			start:       fullStart,
			end:         fullEnd,
			wantTotal:   2000,
			wantLineLen: 1, // base line only
		},
		{
			name:  "within allowances, no overage",
			plan:  testPlan(),
			usage: []subscriber.UsageRecord{usage("voice", 50, inPeriod), usage("data", 500, inPeriod), usage("sms", 10, inPeriod)},
			start: fullStart, end: fullEnd,
			wantTotal:   2000,
			wantLineLen: 1, // only base; no overage lines when under allowance
		},
		{
			name:  "voice overage only",
			plan:  testPlan(),
			usage: []subscriber.UsageRecord{usage("voice", 120, inPeriod)},
			start: fullStart, end: fullEnd,
			// 20 over * 5c = 100c overage + 2000 base
			wantTotal:   2100,
			wantLineLen: 2,
		},
		{
			name:  "sms overage only",
			plan:  testPlan(),
			usage: []subscriber.UsageRecord{usage("sms", 70, inPeriod)},
			start: fullStart, end: fullEnd,
			// 20 over * 2c = 40c + 2000 base
			wantTotal:   2040,
			wantLineLen: 2,
		},
		{
			name:  "data overage only",
			plan:  testPlan(),
			usage: []subscriber.UsageRecord{usage("data", 1500, inPeriod)},
			start: fullStart, end: fullEnd,
			// 500 over MB * 1c = 500c + 2000 base
			wantTotal:   2500,
			wantLineLen: 2,
		},
		{
			name:  "usage outside period is excluded",
			plan:  testPlan(),
			usage: []subscriber.UsageRecord{usage("voice", 999, time.Date(2025, 12, 1, 0, 0, 0, 0, time.UTC))},
			start: fullStart, end: fullEnd,
			wantTotal:   2000,
			wantLineLen: 1,
		},
		{
			name:  "unknown kind is ignored",
			plan:  testPlan(),
			usage: []subscriber.UsageRecord{usage("fax", 9999, inPeriod)},
			start: fullStart, end: fullEnd,
			wantTotal:   2000,
			wantLineLen: 1,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			inv, err := Rate(tc.plan, tc.usage, tc.start, tc.end)
			if err != nil {
				t.Fatalf("Rate() unexpected error: %v", err)
			}
			if inv.TotalCents != tc.wantTotal {
				t.Errorf("TotalCents = %d, want %d", inv.TotalCents, tc.wantTotal)
			}
			if len(inv.Lines) != tc.wantLineLen {
				t.Errorf("len(Lines) = %d, want %d (lines=%+v)", len(inv.Lines), tc.wantLineLen, inv.Lines)
			}
			// Total must equal the sum of line amounts.
			var sum int64
			for _, l := range inv.Lines {
				sum += l.AmountCents
			}
			if sum != inv.TotalCents {
				t.Errorf("sum(Lines)=%d != TotalCents=%d", sum, inv.TotalCents)
			}
			if !inv.PeriodStart.Equal(tc.start) || !inv.PeriodEnd.Equal(tc.end) {
				t.Errorf("period = [%v,%v], want [%v,%v]", inv.PeriodStart, inv.PeriodEnd, tc.start, tc.end)
			}
		})
	}
}

func TestRateEmptyPeriodErrors(t *testing.T) {
	// end before start is invalid input.
	start := time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC)
	if _, err := Rate(testPlan(), nil, start, end); err == nil {
		t.Fatal("expected error for end before start, got nil")
	}
}

func TestRateProrationHalfMonth(t *testing.T) {
	// 15-day partial period: Jan 1 .. Jan 16 2026. The 24h-division day count
	// gives a span of 15 days, prorated base = 2000*15/31 via
	// the float path = 967c (truncated). No usage -> total == prorated base.
	start := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(2026, 1, 16, 0, 0, 0, 0, time.UTC)
	inv, err := Rate(testPlan(), nil, start, end)
	if err != nil {
		t.Fatalf("Rate() unexpected error: %v", err)
	}
	if want := int64(967); inv.TotalCents != want {
		t.Errorf("prorated TotalCents = %d, want %d", inv.TotalCents, want)
	}
}
