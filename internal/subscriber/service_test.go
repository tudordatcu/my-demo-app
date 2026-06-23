package subscriber

import (
	"context"
	"errors"
	"testing"
	"time"
)

// fakeStore is a minimal in-memory Store used only for service unit tests.
type fakeStore struct {
	plans       map[string]Plan
	subscribers map[string]Subscriber
	usage       []UsageRecord
}

func newFakeStore() *fakeStore {
	return &fakeStore{
		plans:       map[string]Plan{},
		subscribers: map[string]Subscriber{},
	}
}

func (f *fakeStore) CreatePlan(_ context.Context, p Plan) error {
	f.plans[p.ID] = p
	return nil
}

func (f *fakeStore) GetPlan(_ context.Context, id string) (Plan, error) {
	p, ok := f.plans[id]
	if !ok {
		return Plan{}, ErrNotFound
	}
	return p, nil
}

func (f *fakeStore) ListPlans(_ context.Context, limit, offset int) ([]Plan, error) {
	out := make([]Plan, 0, len(f.plans))
	for _, p := range f.plans {
		out = append(out, p)
	}
	if offset < 0 {
		offset = 0
	}
	if offset > len(out) {
		offset = len(out)
	}
	out = out[offset:]
	if limit >= 0 && limit < len(out) {
		out = out[:limit]
	}
	return out, nil
}

func (f *fakeStore) CreateSubscriber(_ context.Context, s Subscriber) error {
	f.subscribers[s.ID] = s
	return nil
}

func (f *fakeStore) GetSubscriber(_ context.Context, id string) (Subscriber, error) {
	s, ok := f.subscribers[id]
	if !ok {
		return Subscriber{}, ErrNotFound
	}
	return s, nil
}

func (f *fakeStore) ListSubscribers(_ context.Context, search string, limit, offset int) ([]Subscriber, error) {
	out := make([]Subscriber, 0, len(f.subscribers))
	for _, s := range f.subscribers {
		out = append(out, s)
	}
	if offset < 0 {
		offset = 0
	}
	if offset > len(out) {
		offset = len(out)
	}
	out = out[offset:]
	if limit >= 0 && limit < len(out) {
		out = out[:limit]
	}
	return out, nil
}

func (f *fakeStore) UpdateSubscriber(_ context.Context, s Subscriber) error {
	if _, ok := f.subscribers[s.ID]; !ok {
		return ErrNotFound
	}
	f.subscribers[s.ID] = s
	return nil
}

func (f *fakeStore) AddUsage(_ context.Context, u UsageRecord) error {
	f.usage = append(f.usage, u)
	return nil
}

func (f *fakeStore) ListUsage(_ context.Context, subscriberID string, from, to time.Time) ([]UsageRecord, error) {
	out := make([]UsageRecord, 0)
	for _, u := range f.usage {
		if u.SubscriberID == subscriberID {
			out = append(out, u)
		}
	}
	return out, nil
}

func (f *fakeStore) Ping(_ context.Context) error  { return nil }
func (f *fakeStore) Reset(_ context.Context) error { return nil }

func seedPlan(t *testing.T, svc *Service) Plan {
	t.Helper()
	p, err := svc.CreatePlan(context.Background(), Plan{
		Name:                 "Pay Monthly 20GB",
		MonthlyPriceCents:    2000,
		IncludedVoiceMinutes: 500,
		IncludedDataMB:       20000,
		IncludedSMS:          1000,
	})
	if err != nil {
		t.Fatalf("CreatePlan: unexpected error: %v", err)
	}
	if p.ID == "" {
		t.Fatalf("CreatePlan: expected generated ID, got empty")
	}
	return p
}

func mustCreateSubscriber(t *testing.T, svc *Service, planID string) Subscriber {
	t.Helper()
	sub, err := svc.CreateSubscriber(context.Background(), CreateSubscriberInput{
		MSISDN:         "+447700900123",
		IMSI:           "234157000000001",
		Name:           "Ada Lovelace",
		PlanID:         planID,
		OwnerAccountID: "acct-1",
	})
	if err != nil {
		t.Fatalf("CreateSubscriber: unexpected error: %v", err)
	}
	return sub
}

func TestCreatePlanGeneratesIDAndPersists(t *testing.T) {
	svc := NewService(newFakeStore())
	p := seedPlan(t, svc)

	got, err := svc.GetPlan(context.Background(), p.ID)
	if err != nil {
		t.Fatalf("GetPlan: unexpected error: %v", err)
	}
	if got.ID != p.ID {
		t.Fatalf("GetPlan: ID mismatch: got %q want %q", got.ID, p.ID)
	}
	if got.Name != "Pay Monthly 20GB" {
		t.Fatalf("GetPlan: Name mismatch: got %q", got.Name)
	}
	if got.MonthlyPriceCents != 2000 {
		t.Fatalf("GetPlan: price mismatch: got %d", got.MonthlyPriceCents)
	}
}

func TestGetPlanNotFound(t *testing.T) {
	svc := NewService(newFakeStore())
	_, err := svc.GetPlan(context.Background(), "nope")
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("GetPlan: expected ErrNotFound, got %v", err)
	}
}

func TestCreateSubscriberHappyPath(t *testing.T) {
	svc := NewService(newFakeStore())
	p := seedPlan(t, svc)
	sub := mustCreateSubscriber(t, svc, p.ID)

	if sub.ID == "" {
		t.Fatalf("CreateSubscriber: expected generated ID, got empty")
	}
	if sub.Status != StatusActive {
		t.Fatalf("CreateSubscriber: expected default status active, got %q", sub.Status)
	}
	if sub.CreatedAt.IsZero() {
		t.Fatalf("CreateSubscriber: expected CreatedAt to be set")
	}
	if sub.MSISDN != "+447700900123" {
		t.Fatalf("CreateSubscriber: MSISDN mismatch: got %q", sub.MSISDN)
	}

	got, err := svc.GetSubscriber(context.Background(), sub.ID)
	if err != nil {
		t.Fatalf("GetSubscriber: unexpected error: %v", err)
	}
	if got.ID != sub.ID {
		t.Fatalf("GetSubscriber: ID mismatch: got %q want %q", got.ID, sub.ID)
	}
}

func TestCreateSubscriberEmptyMSISDNRejected(t *testing.T) {
	svc := NewService(newFakeStore())
	p := seedPlan(t, svc)
	_, err := svc.CreateSubscriber(context.Background(), CreateSubscriberInput{
		MSISDN:         "",
		IMSI:           "234157000000001",
		Name:           "No Number",
		PlanID:         p.ID,
		OwnerAccountID: "acct-1",
	})
	if !errors.Is(err, ErrValidation) {
		t.Fatalf("CreateSubscriber: expected ErrValidation for empty MSISDN, got %v", err)
	}
}

func TestGetSubscriberNotFound(t *testing.T) {
	svc := NewService(newFakeStore())
	_, err := svc.GetSubscriber(context.Background(), "missing")
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("GetSubscriber: expected ErrNotFound, got %v", err)
	}
}

func TestChangeStatusValidTransitions(t *testing.T) {
	cases := []struct {
		name string
		from Status
		to   Status
	}{
		{"active to suspended", StatusActive, StatusSuspended},
		{"suspended to active", StatusSuspended, StatusActive},
		{"active to terminated", StatusActive, StatusTerminated},
		{"suspended to terminated", StatusSuspended, StatusTerminated},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			svc := NewService(newFakeStore())
			p := seedPlan(t, svc)
			sub := mustCreateSubscriber(t, svc, p.ID)

			// Drive the subscriber into the desired starting state first.
			if tc.from != StatusActive {
				if _, err := svc.ChangeStatus(context.Background(), sub.ID, tc.from); err != nil {
					t.Fatalf("setup ChangeStatus to %q: unexpected error: %v", tc.from, err)
				}
			}

			got, err := svc.ChangeStatus(context.Background(), sub.ID, tc.to)
			if err != nil {
				t.Fatalf("ChangeStatus %q->%q: unexpected error: %v", tc.from, tc.to, err)
			}
			if got.Status != tc.to {
				t.Fatalf("ChangeStatus: expected status %q, got %q", tc.to, got.Status)
			}
		})
	}
}

func TestChangeStatusActiveToActiveRejected(t *testing.T) {
	svc := NewService(newFakeStore())
	p := seedPlan(t, svc)
	sub := mustCreateSubscriber(t, svc, p.ID)

	_, err := svc.ChangeStatus(context.Background(), sub.ID, StatusActive)
	if !errors.Is(err, ErrInvalidStatusTransition) {
		t.Fatalf("ChangeStatus active->active: expected ErrInvalidStatusTransition, got %v", err)
	}
}

func TestChangeStatusUnknownSubscriber(t *testing.T) {
	svc := NewService(newFakeStore())
	_, err := svc.ChangeStatus(context.Background(), "missing", StatusSuspended)
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("ChangeStatus: expected ErrNotFound, got %v", err)
	}
}

func TestAddUsageHappyPath(t *testing.T) {
	svc := NewService(newFakeStore())
	p := seedPlan(t, svc)
	sub := mustCreateSubscriber(t, svc, p.ID)

	ts := time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC)
	rec, err := svc.AddUsage(context.Background(), sub.ID, "voice", 30, ts)
	if err != nil {
		t.Fatalf("AddUsage: unexpected error: %v", err)
	}
	if rec.ID == "" {
		t.Fatalf("AddUsage: expected generated ID, got empty")
	}
	if rec.SubscriberID != sub.ID {
		t.Fatalf("AddUsage: SubscriberID mismatch: got %q want %q", rec.SubscriberID, sub.ID)
	}
	if rec.Kind != "voice" || rec.Quantity != 30 {
		t.Fatalf("AddUsage: record mismatch: got kind=%q qty=%d", rec.Kind, rec.Quantity)
	}
	if !rec.Timestamp.Equal(ts) {
		t.Fatalf("AddUsage: timestamp mismatch: got %v want %v", rec.Timestamp, ts)
	}
}

func TestAddUsageInvalidKindRejected(t *testing.T) {
	svc := NewService(newFakeStore())
	p := seedPlan(t, svc)
	sub := mustCreateSubscriber(t, svc, p.ID)

	_, err := svc.AddUsage(context.Background(), sub.ID, "carrier-pigeon", 5, time.Now())
	if !errors.Is(err, ErrValidation) {
		t.Fatalf("AddUsage: expected ErrValidation for bad kind, got %v", err)
	}
}

func TestAddUsageUnknownSubscriber(t *testing.T) {
	svc := NewService(newFakeStore())
	_, err := svc.AddUsage(context.Background(), "missing", "voice", 5, time.Now())
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("AddUsage: expected ErrNotFound, got %v", err)
	}
}
