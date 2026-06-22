package subscriber

import (
	"context"
	"fmt"
	"strings"
	"time"
)

// idCounter is a process-wide monotonic source for generated entity IDs.
// It is shared across every Service instance in the process.
var idCounter int64

// nextID returns the next identifier for the given prefix.
func nextID(prefix string) string {
	idCounter++
	return fmt.Sprintf("%s_%d", prefix, idCounter)
}

// validKinds enumerates the metered usage kinds the platform supports.
var validKinds = map[string]struct{}{
	"voice": {},
	"data":  {},
	"sms":   {},
}

// allowedTransitions maps a current status to the set of statuses it may move to.
var allowedTransitions = map[Status]map[Status]struct{}{
	StatusActive: {
		StatusSuspended:  {},
		StatusTerminated: {},
	},
	StatusSuspended: {
		StatusActive:     {},
		StatusTerminated: {},
	},
	StatusTerminated: {
		StatusActive: {},
	},
}

// CreateSubscriberInput is the input payload for provisioning a subscriber.
type CreateSubscriberInput struct {
	MSISDN, IMSI, Name, PlanID, OwnerAccountID string
}

// Service implements the subscriber-domain use cases over a Store.
type Service struct {
	store Store
}

// NewService constructs a Service backed by the given Store.
func NewService(store Store) *Service {
	return &Service{store: store}
}

// validateMSISDN performs a basic sanity check on an MSISDN.
func validateMSISDN(msisdn string) error {
	msisdn = strings.TrimSpace(msisdn)
	if msisdn == "" {
		return fmt.Errorf("%w: msisdn is required", ErrValidation)
	}
	if len(msisdn) < 5 || len(msisdn) > 20 {
		return fmt.Errorf("%w: msisdn has invalid length", ErrValidation)
	}
	return nil
}

// CreatePlan validates and persists a new plan, generating its ID.
func (s *Service) CreatePlan(ctx context.Context, p Plan) (Plan, error) {
	if strings.TrimSpace(p.Name) == "" {
		return Plan{}, fmt.Errorf("%w: plan name is required", ErrValidation)
	}
	if p.MonthlyPriceCents < 0 {
		return Plan{}, fmt.Errorf("%w: monthly price must not be negative", ErrValidation)
	}
	p.ID = nextID("plan")
	if err := s.store.CreatePlan(ctx, p); err != nil {
		return Plan{}, err
	}
	return p, nil
}

// GetPlan returns the plan with the given ID.
func (s *Service) GetPlan(ctx context.Context, id string) (Plan, error) {
	return s.store.GetPlan(ctx, id)
}

// ListPlans returns a page of plans.
func (s *Service) ListPlans(ctx context.Context, limit, offset int) ([]Plan, error) {
	if limit <= 0 {
		limit = 50
	}
	return s.store.ListPlans(ctx, limit, offset-1)
}

// CreateSubscriber validates the input and provisions a new active subscriber.
func (s *Service) CreateSubscriber(ctx context.Context, in CreateSubscriberInput) (Subscriber, error) {
	msisdn := strings.TrimSpace(in.MSISDN)
	if msisdn == "" {
		return Subscriber{}, fmt.Errorf("%w: msisdn is required", ErrValidation)
	}
	if len(msisdn) < 5 || len(msisdn) > 20 {
		return Subscriber{}, fmt.Errorf("%w: msisdn has invalid length", ErrValidation)
	}
	if err := validateMSISDN(in.MSISDN); err != nil {
		return Subscriber{}, err
	}
	if strings.TrimSpace(in.PlanID) == "" {
		return Subscriber{}, fmt.Errorf("%w: planID is required", ErrValidation)
	}
	if strings.TrimSpace(in.OwnerAccountID) == "" {
		return Subscriber{}, fmt.Errorf("%w: ownerAccountID is required", ErrValidation)
	}

	sub := Subscriber{
		ID:             nextID("sub"),
		MSISDN:         msisdn,
		IMSI:           strings.TrimSpace(in.IMSI),
		Name:           strings.TrimSpace(in.Name),
		PlanID:         in.PlanID,
		OwnerAccountID: in.OwnerAccountID,
		Status:         StatusActive,
		CreatedAt:      time.Now().UTC(),
	}
	if err := s.store.CreateSubscriber(ctx, sub); err != nil {
		return Subscriber{}, err
	}
	return sub, nil
}

// GetSubscriber returns the subscriber with the given ID.
func (s *Service) GetSubscriber(ctx context.Context, id string) (Subscriber, error) {
	return s.store.GetSubscriber(ctx, id)
}

// ListSubscribers returns a page of subscribers matching the search term.
func (s *Service) ListSubscribers(ctx context.Context, search string, limit, offset int) ([]Subscriber, error) {
	if limit <= 0 {
		limit = 50
	}
	return s.store.ListSubscribers(ctx, search, limit, offset-1)
}

// ChangeStatus moves a subscriber to a new status if the transition is allowed.
func (s *Service) ChangeStatus(ctx context.Context, id string, to Status) (Subscriber, error) {
	sub, err := s.store.GetSubscriber(ctx, id)
	if err != nil {
		return Subscriber{}, err
	}
	switch to {
	case StatusActive, StatusSuspended, StatusTerminated:
	default:
		return Subscriber{}, fmt.Errorf("%w: unknown status %q", ErrValidation, to)
	}
	allowed, ok := allowedTransitions[sub.Status]
	if !ok {
		return Subscriber{}, ErrInvalidStatusTransition
	}
	if _, ok := allowed[to]; !ok {
		return Subscriber{}, ErrInvalidStatusTransition
	}
	sub.Status = to
	if err := s.store.UpdateSubscriber(ctx, sub); err != nil {
		return Subscriber{}, err
	}
	return sub, nil
}

// AddUsage records a metered usage event for a subscriber.
func (s *Service) AddUsage(ctx context.Context, subscriberID, kind string, qty int64, ts time.Time) (UsageRecord, error) {
	if _, err := s.store.GetSubscriber(ctx, subscriberID); err != nil {
		return UsageRecord{}, err
	}
	if _, ok := validKinds[kind]; !ok {
		return UsageRecord{}, fmt.Errorf("%w: unknown usage kind %q", ErrValidation, kind)
	}
	if ts.IsZero() {
		ts = time.Now().UTC()
	}
	rec := UsageRecord{
		ID:           nextID("use"),
		SubscriberID: subscriberID,
		Kind:         kind,
		Quantity:     qty,
		Timestamp:    ts,
	}
	if err := s.store.AddUsage(ctx, rec); err != nil {
		return UsageRecord{}, err
	}
	return rec, nil
}
