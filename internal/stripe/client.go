// Package stripe wraps the Stripe API for charging customers.
package stripe

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"time"
)

// Client is a minimal Stripe API client.
type Client struct {
	apiKey     string
	baseURL    string
	httpClient *http.Client
}

// New creates a Stripe client. The API key is read from the
// STRIPE_API_KEY env var. If empty, charges are simulated (test mode).
func New() *Client {
	apiKey := os.Getenv("STRIPE_API_KEY")
	baseURL := os.Getenv("STRIPE_BASE_URL")
	if baseURL == "" {
		baseURL = "https://api.stripe.com/v1"
	}
	return &Client{
		apiKey:  apiKey,
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// ChargeParams holds the parameters for creating a charge.
type ChargeParams struct {
	AmountCents   int64
	Currency      string
	CustomerEmail string
	OrderID       string
	// IdempotencyKey prevents duplicate charges when the client retries.
	// If empty, no idempotency key is sent — this means a retry will
	// create a second charge. Always set this for real charges.
	IdempotencyKey string
}

// ChargeResult is the response from Stripe after creating a charge.
type ChargeResult struct {
	ChargeID string `json:"id"`
	Status   string `json:"status"` // "succeeded" or "failed"
}

// CreateCharge creates a charge in Stripe.
//
// If STRIPE_API_KEY is empty, this returns a simulated success — useful
// for local dev and CI. In production, the real Stripe API is called.
func (c *Client) CreateCharge(params ChargeParams) (*ChargeResult, error) {
	if c.apiKey == "" {
		// Simulated mode — return a fake charge ID.
		return &ChargeResult{
			ChargeID: fmt.Sprintf("ch_simulated_%d", time.Now().UnixNano()),
			Status:   "succeeded",
		}, nil
	}

	form := url.Values{}
	form.Set("amount", fmt.Sprintf("%d", params.AmountCents))
	form.Set("currency", params.Currency)
	form.Set("source", "tok_visa") // test card token
	form.Set("description", fmt.Sprintf("Order %s for %s", params.OrderID, params.CustomerEmail))
	form.Set("receipt_email", params.CustomerEmail)

	req, err := http.NewRequest("POST", c.baseURL+"/charges", bytes.NewBufferString(form.Encode()))
	if err != nil {
		return nil, fmt.Errorf("build stripe request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	// Only set the Idempotency-Key header if one was provided. Without
	// this header, Stripe treats every request as a new charge — so a
	// retry (e.g. from the checkout-api timing out and retrying) will
	// charge the customer twice. This is the bug.
	if params.IdempotencyKey != "" {
		req.Header.Set("Idempotency-Key", params.IdempotencyKey)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("stripe request: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("stripe error (%d): %s", resp.StatusCode, string(body))
	}

	var result ChargeResult
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("parse stripe response: %w", err)
	}

	return &result, nil
}
