# Build stage
FROM golang:1.22-alpine AS builder

WORKDIR /app

RUN apk add --no-cache git ca-certificates

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /payments-api ./cmd/server

# Runtime stage
FROM alpine:3.20 AS runtime

RUN apk --no-cache add ca-certificates
RUN addgroup -S app && adduser -S app -G app

WORKDIR /app

COPY --from=builder /payments-api .
USER app

EXPOSE 8080

HEALTHCHECK --interval=10s --timeout=3s --start-period=5s --retries=3 \
    CMD wget -qO- http://localhost:8080/health || exit 1

CMD ["./payments-api"]
