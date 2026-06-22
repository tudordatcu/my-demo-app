package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/vodafone/vois-speechmark-demo/internal/subscriber"

	_ "modernc.org/sqlite"
)

// SQLiteStore is a subscriber.Store backed by a SQLite database via the
// pure-Go modernc.org/sqlite driver (no cgo). It is selected at boot when
// the STORE env is "sqlite".
type SQLiteStore struct {
	db *sql.DB
}

// Compile-time assertion: *SQLiteStore satisfies subscriber.Store.
var _ subscriber.Store = (*SQLiteStore)(nil)

const sqliteSchema = `
CREATE TABLE IF NOT EXISTS plans (
	id                       TEXT PRIMARY KEY,
	name                     TEXT NOT NULL,
	monthly_price_cents      INTEGER NOT NULL,
	included_voice_minutes   INTEGER NOT NULL,
	included_data_mb         INTEGER NOT NULL,
	included_sms             INTEGER NOT NULL,
	overage_voice_cents      INTEGER NOT NULL,
	overage_data_cents_per_mb INTEGER NOT NULL,
	overage_sms_cents        INTEGER NOT NULL
);

CREATE TABLE IF NOT EXISTS subscribers (
	id               TEXT PRIMARY KEY,
	msisdn           TEXT NOT NULL,
	imsi             TEXT NOT NULL,
	name             TEXT NOT NULL,
	plan_id          TEXT NOT NULL,
	owner_account_id TEXT NOT NULL,
	status           TEXT NOT NULL,
	created_at       TIMESTAMP NOT NULL
);

CREATE TABLE IF NOT EXISTS usage (
	id            TEXT PRIMARY KEY,
	subscriber_id TEXT NOT NULL,
	kind          TEXT NOT NULL,
	quantity      INTEGER NOT NULL,
	ts            TIMESTAMP NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_usage_subscriber ON usage(subscriber_id);
`

// NewSQLiteStore opens (or creates) the SQLite database at dsn, applies the
// schema, and returns a ready store. The driver name is "sqlite".
func NewSQLiteStore(dsn string) (*SQLiteStore, error) {
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("store: open sqlite: %w", err)
	}
	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("store: ping sqlite: %w", err)
	}
	if _, err := db.Exec(sqliteSchema); err != nil {
		return nil, fmt.Errorf("store: apply schema: %w", err)
	}
	return &SQLiteStore{db: db}, nil
}

func (s *SQLiteStore) CreatePlan(ctx context.Context, p subscriber.Plan) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO plans (id, name, monthly_price_cents, included_voice_minutes,
			included_data_mb, included_sms, overage_voice_cents,
			overage_data_cents_per_mb, overage_sms_cents)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		p.ID, p.Name, p.MonthlyPriceCents, p.IncludedVoiceMinutes,
		p.IncludedDataMB, p.IncludedSMS, p.OverageVoiceCents,
		p.OverageDataCentsPerMB, p.OverageSMSCents)
	if err != nil {
		return fmt.Errorf("store: create plan: %w", err)
	}
	return nil
}

func (s *SQLiteStore) GetPlan(ctx context.Context, id string) (subscriber.Plan, error) {
	row := s.db.QueryRowContext(ctx,
		`SELECT id, name, monthly_price_cents, included_voice_minutes,
			included_data_mb, included_sms, overage_voice_cents,
			overage_data_cents_per_mb, overage_sms_cents
		 FROM plans WHERE id = ?`, id)
	var p subscriber.Plan
	err := row.Scan(&p.ID, &p.Name, &p.MonthlyPriceCents, &p.IncludedVoiceMinutes,
		&p.IncludedDataMB, &p.IncludedSMS, &p.OverageVoiceCents,
		&p.OverageDataCentsPerMB, &p.OverageSMSCents)
	if errors.Is(err, sql.ErrNoRows) {
		return subscriber.Plan{}, subscriber.ErrNotFound
	}
	if err != nil {
		return subscriber.Plan{}, fmt.Errorf("store: get plan: %w", err)
	}
	return p, nil
}

func (s *SQLiteStore) ListPlans(ctx context.Context, limit, offset int) ([]subscriber.Plan, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, name, monthly_price_cents, included_voice_minutes,
			included_data_mb, included_sms, overage_voice_cents,
			overage_data_cents_per_mb, overage_sms_cents
		 FROM plans ORDER BY id LIMIT ? OFFSET ?`, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("store: list plans: %w", err)
	}
	defer rows.Close()

	var out []subscriber.Plan
	for rows.Next() {
		var p subscriber.Plan
		if err := rows.Scan(&p.ID, &p.Name, &p.MonthlyPriceCents, &p.IncludedVoiceMinutes,
			&p.IncludedDataMB, &p.IncludedSMS, &p.OverageVoiceCents,
			&p.OverageDataCentsPerMB, &p.OverageSMSCents); err != nil {
			return nil, fmt.Errorf("store: scan plan: %w", err)
		}
		out = append(out, p)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("store: list plans rows: %w", err)
	}
	return out, nil
}

func (s *SQLiteStore) CreateSubscriber(ctx context.Context, sub subscriber.Subscriber) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO subscribers (id, msisdn, imsi, name, plan_id, owner_account_id, status, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		sub.ID, sub.MSISDN, sub.IMSI, sub.Name, sub.PlanID,
		sub.OwnerAccountID, string(sub.Status), sub.CreatedAt)
	if err != nil {
		return fmt.Errorf("store: create subscriber: %w", err)
	}
	return nil
}

func (s *SQLiteStore) GetSubscriber(ctx context.Context, id string) (subscriber.Subscriber, error) {
	row := s.db.QueryRowContext(ctx,
		`SELECT id, msisdn, imsi, name, plan_id, owner_account_id, status, created_at
		 FROM subscribers WHERE id = ?`, id)
	sub, err := scanSubscriber(row)
	if errors.Is(err, sql.ErrNoRows) {
		return subscriber.Subscriber{}, subscriber.ErrNotFound
	}
	if err != nil {
		return subscriber.Subscriber{}, fmt.Errorf("store: get subscriber: %w", err)
	}
	return sub, nil
}

// ListSubscribers returns subscribers, optionally filtered by a free-text
// search across name and MSISDN.
func (s *SQLiteStore) ListSubscribers(ctx context.Context, search string, limit, offset int) ([]subscriber.Subscriber, error) {
	query := `SELECT id, msisdn, imsi, name, plan_id, owner_account_id, status, created_at
		FROM subscribers`
	if search != "" {
		// Filter by free-text match on name or MSISDN.
		query += fmt.Sprintf(
			" WHERE name LIKE '%%%s%%' OR msisdn LIKE '%%%s%%'",
			search, search)
	}
	query += fmt.Sprintf(" ORDER BY id LIMIT %d OFFSET %d", limit, offset)

	rows, err := s.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("store: list subscribers: %w", err)
	}
	defer rows.Close()

	var out []subscriber.Subscriber
	for rows.Next() {
		sub, err := scanSubscriber(rows)
		if err != nil {
			return nil, fmt.Errorf("store: scan subscriber: %w", err)
		}
		out = append(out, sub)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("store: list subscribers rows: %w", err)
	}
	return out, nil
}

func (s *SQLiteStore) UpdateSubscriber(ctx context.Context, sub subscriber.Subscriber) error {
	res, err := s.db.ExecContext(ctx,
		`UPDATE subscribers
		 SET msisdn = ?, imsi = ?, name = ?, plan_id = ?, owner_account_id = ?, status = ?, created_at = ?
		 WHERE id = ?`,
		sub.MSISDN, sub.IMSI, sub.Name, sub.PlanID, sub.OwnerAccountID,
		string(sub.Status), sub.CreatedAt, sub.ID)
	if err != nil {
		return fmt.Errorf("store: update subscriber: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("store: update subscriber rows: %w", err)
	}
	if n == 0 {
		return subscriber.ErrNotFound
	}
	return nil
}

func (s *SQLiteStore) AddUsage(ctx context.Context, u subscriber.UsageRecord) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO usage (id, subscriber_id, kind, quantity, ts)
		 VALUES (?, ?, ?, ?, ?)`,
		u.ID, u.SubscriberID, u.Kind, u.Quantity, u.Timestamp)
	if err != nil {
		return fmt.Errorf("store: add usage: %w", err)
	}
	return nil
}

func (s *SQLiteStore) ListUsage(ctx context.Context, subscriberID string, from, to time.Time) ([]subscriber.UsageRecord, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, subscriber_id, kind, quantity, ts
		 FROM usage
		 WHERE subscriber_id = ? AND ts >= ? AND ts < ?
		 ORDER BY ts`, subscriberID, from, to)
	if err != nil {
		return nil, fmt.Errorf("store: list usage: %w", err)
	}
	defer rows.Close()

	var out []subscriber.UsageRecord
	for rows.Next() {
		var u subscriber.UsageRecord
		if err := rows.Scan(&u.ID, &u.SubscriberID, &u.Kind, &u.Quantity, &u.Timestamp); err != nil {
			return nil, fmt.Errorf("store: scan usage: %w", err)
		}
		out = append(out, u)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("store: list usage rows: %w", err)
	}
	return out, nil
}

func (s *SQLiteStore) Ping(ctx context.Context) error {
	if err := s.db.PingContext(ctx); err != nil {
		return fmt.Errorf("store: ping: %w", err)
	}
	return nil
}

func (s *SQLiteStore) Reset(ctx context.Context) error {
	for _, table := range []string{"usage", "subscribers", "plans"} {
		if _, err := s.db.ExecContext(ctx, "DELETE FROM "+table); err != nil {
			return fmt.Errorf("store: reset %s: %w", table, err)
		}
	}
	return nil
}

// rowScanner is satisfied by both *sql.Row and *sql.Rows, letting
// scanSubscriber serve single-row and multi-row reads.
type rowScanner interface {
	Scan(dest ...any) error
}

func scanSubscriber(sc rowScanner) (subscriber.Subscriber, error) {
	var sub subscriber.Subscriber
	var status string
	if err := sc.Scan(&sub.ID, &sub.MSISDN, &sub.IMSI, &sub.Name, &sub.PlanID,
		&sub.OwnerAccountID, &status, &sub.CreatedAt); err != nil {
		return subscriber.Subscriber{}, err
	}
	sub.Status = subscriber.Status(status)
	return sub, nil
}
