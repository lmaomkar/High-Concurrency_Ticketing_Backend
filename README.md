# lewly-pgpyewj

High-concurrency ticketing backend in Go with Postgres, Redis, Kafka, Prometheus, and Swagger.

## Quick start

Prereqs: Docker, Docker Compose

```bash
docker-compose up --build
```

Server: http://localhost:8080

Docs: http://localhost:8080/docs (serves `docs/openapi.yaml`)

Prometheus: http://localhost:9090

Grafana: http://localhost:3000

## Env

Copy `.env.example` → `.env` and adjust.

Key vars:
- `POSTGRES_URL`, `REDIS_ADDR`, `KAFKA_BROKERS`, `JWT_SECRET`, `SMTP_*`

## Migrations

SQL migrations live in `cmd/migrate/migrations`.


## Architecture

- Gin HTTP API (stateless)
- Postgres source of truth
- Redis token bucket for fast reservations
- Kafka for finalize workflow (bookings topic, DLQ)
- Prometheus metrics, Grafana dashboards

## Booking flow

1) API reserves via Redis token bucket (Lua) → creates pending booking → publishes finalize to Kafka → 202 Accepted
2) Worker consumes, transactionally finalizes using `SELECT ... FOR UPDATE`, updates counters, and confirms.
3) If sold out, user auto-waitlisted; cancellation triggers promotion.

## Security

JWT middleware for admin endpoints. Do not store payment details (out of scope).

## Deployment

Containerized via Dockerfile. Example CI in `.github/workflows/ci.yml`. Deploy to Render/Railway using Docker image and env vars.

