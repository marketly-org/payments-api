// Package main is the entry point for the payments-api.
package main

import (
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/marketly-org/payments-api/internal/handler"
	"github.com/marketly-org/payments-api/internal/store"
	"github.com/marketly-org/payments-api/internal/stripe"
)

func main() {
	dsn := os.Getenv("PAYMENTS_DATABASE_URL")
	if dsn == "" {
		dsn = "postgresql://marketly:marketly@localhost:5432/payments"
	}

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	s, err := store.New(dsn)
	if err != nil {
		log.Fatalf("failed to connect to database: %v", err)
	}
	defer s.Close()

	if err := s.InitSchema(); err != nil {
		log.Fatalf("failed to init schema: %v", err)
	}

	client := stripe.New()
	h := handler.New(s, client)

	mux := http.NewServeMux()
	mux.HandleFunc("/charge", h.Charge)
	mux.HandleFunc("/charges", h.GetCharge)
	mux.HandleFunc("/health", h.Health)
	mux.HandleFunc("/ready", h.Ready)

	log.Printf("payments-api starting on :%s", port)
	if err := http.ListenAndServe(fmt.Sprintf(":%s", port), mux); err != nil {
		log.Fatalf("server error: %v", err)
	}
}
