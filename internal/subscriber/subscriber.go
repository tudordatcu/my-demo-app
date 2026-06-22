// Package subscriber holds the telecom domain model (Plan, Subscriber,
// UsageRecord, Invoice), the status state machine, and the Store interface
// that persistence implementations satisfy. It imports only the standard
// library so that store, billing, and httpapi can depend on it without cycles.
package subscriber

import (
	"context"
	"errors"
	"time"
)

// Status is a subscriber's lifecycle state.
type Status string

// Subscriber lifecycle states.
const (
	StatusActive     Status = "active"
	StatusSuspended  Status = "suspended"
	StatusTerminated Status = "terminated"
)

// Plan describes a tariff: a monthly price plus included allowances and
// per-unit overage rates. All money is integer cents.
type Plan struct {
	ID                    string
	Name                  string
	MonthlyPriceCents     int64
	IncludedVoiceMinutes  int
	IncludedDataMB        int
	IncludedSMS           int
	OverageVoiceCents     int64
	OverageDataCentsPerMB int64
	OverageSMSCents       int64
}

// Subscriber is a SIM/line on a plan, owned by an account.
type Subscriber struct {
	ID             string
	MSISDN         string
	IMSI           string
	Name           string
	PlanID         string
	OwnerAccountID string
	Status         Status
	CreatedAt      time.Time
}

// UsageRecord is a single CDR. Kind is one of "voice", "data", "sms";
// Quantity is minutes, MB, or count respectively.
type UsageRecord struct {
	ID           string
	SubscriberID string
	Kind         string
	Quantity     int64
	Timestamp    time.Time
}

// InvoiceLine is one billed item.
type InvoiceLine struct {
	Description string
	AmountCents int64
}

// Invoice is the rated output for a subscriber over a billing period.
type Invoice struct {
	SubscriberID string
	PeriodStart  time.Time
	PeriodEnd    time.Time
	Lines        []InvoiceLine
	TotalCents   int64
}

// Sentinel errors returned across the domain so callers can branch with
// errors.Is and map them to HTTP status codes.
var (
	ErrNotFound                = errors.New("subscriber: not found")
	ErrValidation              = errors.New("subscriber: validation failed")
	ErrInvalidStatusTransition = errors.New("subscriber: invalid status transition")
)

// Store is the persistence seam. Implementations live in internal/store
// (in-memory and SQLite). Methods return the sentinel errors above where
// applicable.
type Store interface {
	CreatePlan(ctx context.Context, p Plan) error
	GetPlan(ctx context.Context, id string) (Plan, error)
	ListPlans(ctx context.Context, limit, offset int) ([]Plan, error)
	CreateSubscriber(ctx context.Context, s Subscriber) error
	GetSubscriber(ctx context.Context, id string) (Subscriber, error)
	ListSubscribers(ctx context.Context, search string, limit, offset int) ([]Subscriber, error)
	UpdateSubscriber(ctx context.Context, s Subscriber) error
	AddUsage(ctx context.Context, u UsageRecord) error
	ListUsage(ctx context.Context, subscriberID string, from, to time.Time) ([]UsageRecord, error)
	Ping(ctx context.Context) error
	Reset(ctx context.Context) error
}
