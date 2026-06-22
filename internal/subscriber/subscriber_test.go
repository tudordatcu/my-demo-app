package subscriber

import (
	"context"
	"errors"
	"testing"
	"time"
)

// staticStoreCheck asserts (at compile time) that any type satisfying the
// methods below satisfies Store. It also guards the exact method set/signatures.
type staticStoreCheck struct{}

func (staticStoreCheck) CreatePlan(ctx context.Context, p Plan) error { return nil }
func (staticStoreCheck) GetPlan(ctx context.Context, id string) (Plan, error) {
	return Plan{}, nil
}
func (staticStoreCheck) ListPlans(ctx context.Context, limit, offset int) ([]Plan, error) {
	return nil, nil
}
func (staticStoreCheck) CreateSubscriber(ctx context.Context, s Subscriber) error { return nil }
func (staticStoreCheck) GetSubscriber(ctx context.Context, id string) (Subscriber, error) {
	return Subscriber{}, nil
}
func (staticStoreCheck) ListSubscribers(ctx context.Context, search string, limit, offset int) ([]Subscriber, error) {
	return nil, nil
}
func (staticStoreCheck) UpdateSubscriber(ctx context.Context, s Subscriber) error { return nil }
func (staticStoreCheck) AddUsage(ctx context.Context, u UsageRecord) error        { return nil }
func (staticStoreCheck) ListUsage(ctx context.Context, subscriberID string, from, to time.Time) ([]UsageRecord, error) {
	return nil, nil
}
func (staticStoreCheck) Ping(ctx context.Context) error  { return nil }
func (staticStoreCheck) Reset(ctx context.Context) error { return nil }

var _ Store = staticStoreCheck{}

func TestSentinelErrorsDistinct(t *testing.T) {
	if errors.Is(ErrNotFound, ErrValidation) {
		t.Fatal("ErrNotFound and ErrValidation must be distinct")
	}
	if errors.Is(ErrValidation, ErrInvalidStatusTransition) {
		t.Fatal("ErrValidation and ErrInvalidStatusTransition must be distinct")
	}
	if errors.Is(ErrNotFound, ErrInvalidStatusTransition) {
		t.Fatal("ErrNotFound and ErrInvalidStatusTransition must be distinct")
	}
}

func TestStatusConstants(t *testing.T) {
	cases := map[Status]string{
		StatusActive:     "active",
		StatusSuspended:  "suspended",
		StatusTerminated: "terminated",
	}
	for got, want := range cases {
		if string(got) != want {
			t.Errorf("status %q: want %q", string(got), want)
		}
	}
}

func TestModelFieldsCompile(t *testing.T) {
	now := time.Now()
	p := Plan{ID: "p1", Name: "Basic", MonthlyPriceCents: 1000, IncludedVoiceMinutes: 100, IncludedDataMB: 500, IncludedSMS: 50, OverageVoiceCents: 5, OverageDataCentsPerMB: 1, OverageSMSCents: 2}
	s := Subscriber{ID: "s1", MSISDN: "+100", IMSI: "001", Name: "A", PlanID: p.ID, OwnerAccountID: "acct1", Status: StatusActive, CreatedAt: now}
	u := UsageRecord{ID: "u1", SubscriberID: s.ID, Kind: "voice", Quantity: 10, Timestamp: now}
	inv := Invoice{SubscriberID: s.ID, PeriodStart: now, PeriodEnd: now, Lines: []InvoiceLine{{Description: "base", AmountCents: 1000}}, TotalCents: 1000}
	if p.MonthlyPriceCents != 1000 || s.Status != StatusActive || u.Quantity != 10 || inv.TotalCents != 1000 {
		t.Fatal("model field round-trip mismatch")
	}
}
