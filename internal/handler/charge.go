// Package handler implements the HTTP handlers for the payments-api.
package handler

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/google/uuid"

	"github.com/marketly-org/payments-api/internal/models"
	"github.com/marketly-org/payments-api/internal/store"
	"github.com/marketly-org/payments-api/internal/stripe"
)

// Handler holds the store + stripe client.
type Handler struct {
	store  *store.Store
	stripe *stripe.Client
}

// New creates a new Handler.
func New(s *store.Store, c *stripe.Client) *Handler {
	return &Handler{store: s, stripe: c}
}

// Charge handles POST /charge.
// This endpoint creates a charge in Stripe and records it in Postgres.
// checkout-api times out waiting for this response and retries, Stripe
// processes the charge again — double-charging the customer. After
// enough retries, Stripe rate-limits the API key (429), which cascades
// back to the checkout-api as a timeout, which triggers more retries.
// The fix is to generate an idempotency key (e.g. uuid.New().String())
func (h *Handler) Charge(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req models.ChargeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid JSON", http.StatusBadRequest)
		return
	}

	if req.OrderID == "" || req.CustomerEmail == "" || req.AmountCents <= 0 {
		http.Error(w, "order_id, customer_email, and positive amount_cents required", http.StatusBadRequest)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()

	// Generate an idempotency key to prevent duplicate charges on retries.
	idempotencyKey := uuid.New().String()

	// Call Stripe to create the charge with retry on rate limit errors.
	var result *stripe.ChargeResult
	var err error
	maxRetries := 3
	backoff := 500 * time.Millisecond
	for i := 0; i <= maxRetries; i++ {
		result, err = h.stripe.CreateCharge(stripe.ChargeParams{
			AmountCents:    req.AmountCents,
			Currency:       req.Currency,
			CustomerEmail:  req.CustomerEmail,
			OrderID:        req.OrderID,
			IdempotencyKey: idempotencyKey,
		})
		if err == nil {
			break
		}
		// Check if error is a rate limit error (HTTP 429).
		if i == maxRetries || !isRateLimitError(err) {
			break
		}
		time.Sleep(backoff)
		backoff *= 2
	}
	if err != nil {
		http.Error(w, fmt.Sprintf("stripe charge failed: %v", err), http.StatusBadGateway)
		return
	}

	// Persist the charge.
	charge := &models.Charge{
		ID:             "pay_" + uuid.New().String(),
		OrderID:        req.OrderID,
		CustomerEmail:  req.CustomerEmail,
		AmountCents:    req.AmountCents,
		Currency:       req.Currency,
		Status:         result.Status,
		StripeChargeID: result.ChargeID,
		CreatedAt:      time.Now().UTC(),
	}

	if err := h.store.SaveCharge(ctx, charge); err != nil {
		http.Error(w, fmt.Sprintf("save charge failed: %v", err), http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusCreated, models.ChargeResponse{
		PaymentID: charge.ID,
		Status:    charge.Status,
	})
}

// GetCharge handles GET /charges/{id}.
func (h *Handler) GetCharge(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	id := r.URL.Query().Get("id")
	if id == "" {
		http.Error(w, "id required", http.StatusBadRequest)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	charge, err := h.store.GetCharge(ctx, id)
	if err != nil {
		if err == sql.ErrNoRows {
			http.Error(w, "charge not found", http.StatusNotFound)
			return
		}
		http.Error(w, fmt.Sprintf("get charge failed: %v", err), http.StatusInternalServerError)
		return
	}
	if charge == nil {
		http.Error(w, "charge not found", http.StatusNotFound)
		return
	}

	writeJSON(w, http.StatusOK, charge)
}

// Health handles GET /health.
func (h *Handler) Health(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{
		"status":  "ok",
		"service": "payments-api",
		"version": "1.0.0",
	})
}

// Ready handles GET /ready.
func (h *Handler) Ready(w http.ResponseWriter, _ *http.Request) {
	if err := h.store.DB().Ping(); err != nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{
			"status": "not_ready",
			"error":  err.Error(),
		})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ready"})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

// isRateLimitError returns true if the error message indicates a Stripe rate limit error.
func isRateLimitError(err error) bool {
	if err == nil {
		return false
	}
	// Stripe returns 429 status code for rate limit errors.
	// The error string contains "stripe error (429):" or "rate_limited"
	errStr := err.Error()
	if (len(errStr) >= 17 && errStr[:17] == "stripe error (429):") || containsIgnoreCase(errStr, "rate_limited") {
		return true
	}
	return false
}

// containsIgnoreCase checks if s contains substr case-insensitively.
func containsIgnoreCase(s, substr string) bool {
	sLower := []rune{}
	for _, r := range s {
		if 'A' <= r && r <= 'Z' {
			r += 'a' - 'A'
		}
		sLower = append(sLower, r)
	}
	subLower := []rune{}
	for _, r := range substr {
		if 'A' <= r && r <= 'Z' {
			r += 'a' - 'A'
		}
		subLower = append(subLower, r)
	}
	for i := 0; i <= len(sLower)-len(subLower); i++ {
		match := true
		for j := 0; j < len(subLower); j++ {
			if sLower[i+j] != subLower[j] {
				match = false
				break
			}
		}
		if match {
			return true
		}
	}
	return false
}
