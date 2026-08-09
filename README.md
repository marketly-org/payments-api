# payments-api

Payment processing service for the **Marketly** e-commerce platform.

Charges customers via Stripe, records charges in Postgres.

## Stack

- **Go 1.22** + net/http
- **lib/pq** for Postgres
- **Stripe API** for payment processing

## Endpoints

| Method | Path | Description |
|--------|------|-------------|
| POST | `/charge` | Create a charge |
| GET | `/charges?id={id}` | Fetch a charge by ID |
| GET | `/health` | Liveness probe |
| GET | `/ready` | Readiness probe (checks DB) |

## Local development

```bash
go run ./cmd/server
```

## Tests

```bash
go test ./... -race
```
