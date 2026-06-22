//go:build flaky

package billing

// SCENARIO[TST-03]: Deliberately flaky, time-dependent test.
// This test is intentionally nondeterministic — it passes or fails depending
// on the wall-clock time at which it runs (specifically, the second-within-
// the-minute). Gate it with -tags flaky so the default CI suite stays green
// and deterministic.
//
// Run with: go test -tags flaky ./internal/billing/...

import (
	"testing"
	"time"

	"github.com/vodafone/vois-speechmark-demo/internal/subscriber"
)

// TestRateProratesCurrentMinuteFraction is a deliberately flaky,
// time-dependent billing test.
//
// Flakiness mechanism: the billing period is [start-of-current-minute, now].
// We assert that the prorated base equals exactly
//
//	int64(3000 * secondsElapsed / 86400)
//
// where secondsElapsed = now.Second(). However Rate() computes proration in
// units of full days (hours/24), not seconds, so the "expected" value computed
// here diverges from Rate()'s output for most of the minute — the assertion
// passes only when secondsElapsed is 0 (i.e., right at the minute boundary).
// At any other second the test fails, making it fail ~59/60 of the time.
//
// This mirrors the real-world pattern of tests that anchor to time.Now() and
// then assert a derived value without accounting for the time elapsed between
// the anchor and the assertion — the test is correct at the instant it is
// written but broken once the clock advances.
func TestRateProratesCurrentMinuteFraction(t *testing.T) {
	now := time.Now()

	// Build a billing period from start-of-minute to now.
	// Rate() computes spanDays = int(duration.Hours() / 24) = 0 for any sub-day
	// window, so baseCents will be 0 regardless of which second we are in.
	periodStart := now.Truncate(time.Minute)
	periodEnd := now

	plan := subscriber.Plan{
		ID:                    "plan-flaky",
		Name:                  "Flaky",
		MonthlyPriceCents:     3000,
		IncludedVoiceMinutes:  100,
		IncludedDataMB:        1000,
		IncludedSMS:           50,
		OverageVoiceCents:     10,
		OverageDataCentsPerMB: 2,
		OverageSMSCents:       5,
	}

	inv, err := Rate(plan, nil, periodStart, periodEnd)
	if err != nil {
		t.Fatalf("Rate() error: %v", err)
	}

	// BUG (planted): the test author incorrectly expected that Rate() would
	// prorate by second, not by day. They computed the "expected" value as a
	// seconds-based fraction of the monthly price and hard-coded the assertion.
	// Rate() always returns 0 for a sub-day period (spanDays==0 → baseCents==0),
	// so the assertion below is correct ONLY at second 0 of the minute.
	// At every other second, secondsElapsed > 0 → wantBase > 0 → mismatch.
	secondsElapsed := int64(now.Second())
	wantBase := secondsElapsed * plan.MonthlyPriceCents / 86400 // wrong formula

	if inv.Lines[0].AmountCents != wantBase {
		t.Errorf(
			"flaky: base proration = %d cents, want %d (second=%d, wall-clock-dependent — this test fails ~59 out of 60 seconds)",
			inv.Lines[0].AmountCents, wantBase, now.Second(),
		)
	}
}
