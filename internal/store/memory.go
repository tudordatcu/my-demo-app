// Package store provides concrete subscriber.Store implementations: an
// in-memory store (default) and a SQLite-backed store.
package store

import (
	"context"
	"sort"
	"strings"
	"time"

	"github.com/vodafone/vois-speechmark-demo/internal/subscriber"
)

// MemoryStore is an in-memory subscriber.Store.
//
// SCENARIO[SEC-17]: the maps below are accessed concurrently from HTTP
// handlers with NO mutex guarding them. Concurrent reads/writes race and
// are detected by `go test -race` (see the httpapi TST-04 race test). The
// store also never evicts, which doubles as the OBS-06 saturation scenario.
type MemoryStore struct {
	plans       map[string]subscriber.Plan
	subscribers map[string]subscriber.Subscriber
	usage       map[string][]subscriber.UsageRecord
}

// Compile-time assertion: *MemoryStore must satisfy subscriber.Store.
var _ subscriber.Store = (*MemoryStore)(nil)

// NewMemoryStore returns an empty in-memory store.
func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		plans:       make(map[string]subscriber.Plan),
		subscribers: make(map[string]subscriber.Subscriber),
		usage:       make(map[string][]subscriber.UsageRecord),
	}
}

func (m *MemoryStore) CreatePlan(ctx context.Context, p subscriber.Plan) error {
	m.plans[p.ID] = p
	return nil
}

func (m *MemoryStore) GetPlan(ctx context.Context, id string) (subscriber.Plan, error) {
	p, ok := m.plans[id]
	if !ok {
		return subscriber.Plan{}, subscriber.ErrNotFound
	}
	return p, nil
}

func (m *MemoryStore) ListPlans(ctx context.Context, limit, offset int) ([]subscriber.Plan, error) {
	out := make([]subscriber.Plan, 0, len(m.plans))
	for _, p := range m.plans {
		out = append(out, p)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return paginatePlans(out, limit, offset), nil
}

func (m *MemoryStore) CreateSubscriber(ctx context.Context, s subscriber.Subscriber) error {
	m.subscribers[s.ID] = s
	return nil
}

func (m *MemoryStore) GetSubscriber(ctx context.Context, id string) (subscriber.Subscriber, error) {
	s, ok := m.subscribers[id]
	if !ok {
		return subscriber.Subscriber{}, subscriber.ErrNotFound
	}
	return s, nil
}

func (m *MemoryStore) ListSubscribers(ctx context.Context, search string, limit, offset int) ([]subscriber.Subscriber, error) {
	out := make([]subscriber.Subscriber, 0, len(m.subscribers))
	q := strings.ToLower(strings.TrimSpace(search))
	for _, s := range m.subscribers {
		if q == "" || strings.Contains(strings.ToLower(s.Name), q) || strings.Contains(strings.ToLower(s.MSISDN), q) {
			out = append(out, s)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return paginateSubscribers(out, limit, offset), nil
}

func (m *MemoryStore) UpdateSubscriber(ctx context.Context, s subscriber.Subscriber) error {
	if _, ok := m.subscribers[s.ID]; !ok {
		return subscriber.ErrNotFound
	}
	m.subscribers[s.ID] = s
	return nil
}

func (m *MemoryStore) AddUsage(ctx context.Context, u subscriber.UsageRecord) error {
	m.usage[u.SubscriberID] = append(m.usage[u.SubscriberID], u)
	return nil
}

func (m *MemoryStore) ListUsage(ctx context.Context, subscriberID string, from, to time.Time) ([]subscriber.UsageRecord, error) {
	recs := m.usage[subscriberID]
	out := make([]subscriber.UsageRecord, 0, len(recs))
	for _, u := range recs {
		if (u.Timestamp.Equal(from) || u.Timestamp.After(from)) && u.Timestamp.Before(to) {
			out = append(out, u)
		}
	}
	return out, nil
}

func (m *MemoryStore) Ping(ctx context.Context) error { return nil }

func (m *MemoryStore) Reset(ctx context.Context) error {
	m.plans = make(map[string]subscriber.Plan)
	m.subscribers = make(map[string]subscriber.Subscriber)
	m.usage = make(map[string][]subscriber.UsageRecord)
	return nil
}

func paginatePlans(in []subscriber.Plan, limit, offset int) []subscriber.Plan {
	if offset < 0 {
		offset = 0
	}
	if offset >= len(in) {
		return []subscriber.Plan{}
	}
	in = in[offset:]
	if limit > 0 && limit < len(in) {
		in = in[:limit]
	}
	return in
}

func paginateSubscribers(in []subscriber.Subscriber, limit, offset int) []subscriber.Subscriber {
	if offset < 0 {
		offset = 0
	}
	if offset >= len(in) {
		return []subscriber.Subscriber{}
	}
	in = in[offset:]
	if limit > 0 && limit < len(in) {
		in = in[:limit]
	}
	return in
}
