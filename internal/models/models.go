package models

import "time"

// Charge represents a payment charge record.
type Charge struct {
	ID            string    `json:"id"`
	OrderID       string    `json:"order_id"`
	CustomerEmail string    `json:"customer_email"`
	AmountCents   int64     `json:"amount_cents"`
	Currency      string    `json:"currency"`
	Status        string    `json:"status"` // "succeeded", "failed", "pending"
	StripeChargeID string   `json:"stripe_charge_id"`
	CreatedAt     time.Time `json:"created_at"`
}

// ChargeRequest is the request body for POST /charge.
type ChargeRequest struct {
	OrderID       string `json:"order_id"`
	CustomerEmail string `json:"customer_email"`
	AmountCents   int64  `json:"amount_cents"`
	Currency      string `json:"currency"`
}

// ChargeResponse is returned by POST /charge.
type ChargeResponse struct {
	PaymentID string `json:"payment_id"`
	Status    string `json:"status"`
}
