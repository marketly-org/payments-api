// Package store provides the Postgres persistence layer for charges.
package store

import (
        "context"
        "database/sql"
        "fmt"
        "time"

        _ "github.com/lib/pq"

        "github.com/marketly-org/payments-api/internal/models"
)

// Store wraps the database connection.
type Store struct {
        db *sql.DB
}

// New creates a new Store connected to the given Postgres DSN.
func New(dsn string) (*Store, error) {
        db, err := sql.Open("postgres", dsn)
        if err != nil {
                return nil, fmt.Errorf("open db: %w", err)
        }
        db.SetMaxOpenConns(10)
        db.SetMaxIdleConns(2)
        db.SetConnMaxLifetime(5 * time.Minute)

        if err := db.Ping(); err != nil {
                return nil, fmt.Errorf("ping db: %w", err)
        }

        return &Store{db: db}, nil
}

// InitSchema creates the charges table if it doesn't exist.
func (s *Store) InitSchema() error {
        _, err := s.db.Exec(`
                CREATE TABLE IF NOT EXISTS charges (
                        id TEXT PRIMARY KEY,
                        order_id TEXT NOT NULL,
                        customer_email TEXT NOT NULL,
                        amount_cents BIGINT NOT NULL,
                        currency TEXT NOT NULL DEFAULT 'USD',
                        status TEXT NOT NULL,
                        stripe_charge_id TEXT,
                        created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
                )
        `)
        if err != nil {
                return fmt.Errorf("create charges table: %w", err)
        }

        _, err = s.db.Exec("CREATE INDEX IF NOT EXISTS idx_charges_order ON charges(order_id)")
        return err
}

// SaveCharge persists a charge record.
func (s *Store) SaveCharge(ctx context.Context, c *models.Charge) error {
        _, err := s.db.ExecContext(ctx, `
                INSERT INTO charges (id, order_id, customer_email, amount_cents, currency, status, stripe_charge_id, created_at)
                VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
        `, c.ID, c.OrderID, c.CustomerEmail, c.AmountCents, c.Currency, c.Status, c.StripeChargeID, c.CreatedAt)
        if err != nil {
                return fmt.Errorf("insert charge: %w", err)
        }
        return nil
}

// GetCharge fetches a charge by ID.
func (s *Store) GetCharge(ctx context.Context, id string) (*models.Charge, error) {
        var c models.Charge
        err := s.db.QueryRowContext(ctx, `
                SELECT id, order_id, customer_email, amount_cents, currency, status, stripe_charge_id, created_at
                FROM charges WHERE id = $1
        `, id).Scan(&c.ID, &c.OrderID, &c.CustomerEmail, &c.AmountCents, &c.Currency, &c.Status, &c.StripeChargeID, &c.CreatedAt)
        if err == sql.ErrNoRows {
                return nil, nil
        }
        if err != nil {
                return nil, fmt.Errorf("get charge: %w", err)
        }
        return &c, nil
}

// Close closes the database connection.
func (s *Store) Close() error {
        return s.db.Close()
}

// DB returns the underlying *sql.DB. Used by health checks.
func (s *Store) DB() *sql.DB {
        return s.db
}
