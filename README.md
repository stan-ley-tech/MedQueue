# MedQueue

A production-style backend for managing patient appointments and real-time clinic queues, written in Go against PostgreSQL and Redis.

Patients are booked into appointments, checked in, placed into a department queue, called by a clinician, and tracked through to completion — with the concurrency, auditability, and failure handling that a system managing a physical waiting room actually needs.

[![CI](https://github.com/stan-ley-tech/MedQueue/actions/workflows/ci.yml/badge.svg)](https://github.com/stan-ley-tech/MedQueue/actions/workflows/ci.yml)

## The problem

A walk-in clinic (or a hospital's outpatient department) runs on a physical queue: patients arrive, check in at the front desk, wait, and are called by whichever clinician is free next. Three things make this harder than a plain appointment calendar:

1. **The queue has to be fair and correct under real concurrency.** Multiple clinicians hit "call next" at the same moment; two patients can't be assigned to the same slot; an emergency case has to jump the line without the system losing track of who was there first.
2. **State has to be trustworthy.** An appointment that says "completed" needs an audit trail proving who checked the patient in, who called them, and when — this is a healthcare system, and "the database briefly disagreed with itself" is not an acceptable failure mode.
3. **The front desk, the waiting room display, and the clinician's console all need to see the same queue in real time**, without polling the database into the ground.

MedQueue is built around those three constraints, not around CRUD-ing five database tables behind a REST API.

## How it works

```
Patient created
      |
Appointment scheduled
      |
Patient checks in
      |
Added to department queue
      |
Queue prioritizes patients (emergency > urgent > normal, FIFO within a tier)
      |
Clinician calls next patient  <-- SELECT ... FOR UPDATE SKIP LOCKED, concurrency-safe
      |
Patient consultation starts
      |
Consultation completed
      |
Appointment completed
```

Every arrow above is a real state transition enforced by a state machine in code (`internal/domain/appointment.go`), not just a status string a client can set to anything. See [ARCHITECTURE.md](ARCHITECTURE.md) for the full design rationale, in particular **why PostgreSQL — not Redis — owns queue concurrency**, which is the single most important decision in this codebase.

## Stack

Go 1.25 · PostgreSQL 16 · Redis 7 · Docker Compose · chi (HTTP router) · pgx/v5 · go-redis/v9 · JWT (golang-jwt/v5) · gorilla/websocket · golang-migrate · go-playground/validator · GitHub Actions

## Features

- Patients, doctors, and departments with pagination and search
- Appointment scheduling with double-booking prevention and a validated status lifecycle
- Check-in that atomically transitions an appointment and enqueues it
- Priority queueing (normal / urgent / emergency), concurrency-safe "call next" via `FOR UPDATE SKIP LOCKED`
- Redis cache-aside for queue reads + pub/sub fan-out to WebSocket clients across API replicas
- JWT auth with rotating, revocable refresh tokens and role-based access control (admin / front_desk / clinician)
- Idempotency-Key support on booking and check-in
- Audit log for every state-changing action, written inside the same transaction as the change
- Background worker for appointment reminders, decoupled from the API process
- Rate limiting, structured JSON logging, consistent error envelope, request validation, graceful shutdown, health/readiness probes
- Unit, integration, e2e, and load/race tests; GitHub Actions CI

## Project layout

```
cmd/api          HTTP API entrypoint
cmd/worker       background reminder worker entrypoint
cmd/migrate      migration runner (go run ./cmd/migrate up)
internal/        application code — see ARCHITECTURE.md for the layering
migrations/      versioned SQL migrations (golang-migrate format)
docs/openapi.yaml  OpenAPI 3.0 spec
tests/           integration, e2e, load tests + shared test fixtures
```

## Database design

Seven tables, each migration named for the aggregate it introduces:

| Table | Purpose |
|---|---|
| `users` | staff accounts (admin / front_desk / clinician), email unique via `citext` |
| `departments` | clinic departments (Cardiology, ER, ...) |
| `doctors` | clinicians, optionally linked to a `users` row for login |
| `patients` | patient demographics, unique medical record number |
| `appointments` | the booking + its status lifecycle |
| `queue_entries` | one row per check-in; `UNIQUE(appointment_id)` is what makes double check-in impossible |
| `audit_logs` | immutable, append-only record of who did what |
| `idempotency_keys` / `refresh_tokens` | retry-safety and session management |

Indexes are built for the queries that actually run, not added generically — most notably a partial index on `queue_entries (department_id, priority DESC, checked_in_at ASC) WHERE status = 'waiting'`, which is exactly the shape of the "call next" query. Full schema: [`migrations/`](migrations).

## API

Full reference: [API.md](API.md). Machine-readable spec: [`docs/openapi.yaml`](docs/openapi.yaml) (served at `GET /openapi.yaml` by a running instance). REST + one WebSocket endpoint for live queue events; every response uses a consistent JSON error envelope; every list endpoint is paginated.

## Local setup

**Requirements:** Docker and Docker Compose. (Go 1.25+ only if you want to run things outside Docker.)

```bash
cp .env.example .env
docker compose up --build
```

This starts PostgreSQL, Redis, runs migrations once via the `migrate` service, then starts the API on `:8080` and the reminder worker. Check it's up:

```bash
curl http://localhost:8080/readyz
```

**Running without Docker:**

```bash
cp .env.example .env
# point DATABASE_URL / REDIS_ADDR in .env at your own Postgres/Redis
make migrate-up
make run          # API on :8080
make run-worker   # in a separate terminal
```

There's no self-service signup — staff accounts are provisioned by an admin via `POST /auth/register`. To bootstrap the very first admin, insert one directly:

```bash
docker compose exec postgres psql -U medqueue -d medqueue -c \
  "INSERT INTO users (email, password_hash, name, role) VALUES ('admin@clinic.example', crypt('changeme', gen_salt('bf', 12)), 'Admin', 'admin');"
```

(Requires the `pgcrypto` extension, already enabled by the first migration, to hash the password the same way the application's bcrypt cost expects.)

## Testing strategy

```bash
make test-unit         # pure logic, no external dependencies
make test-integration  # repositories against a real, migrated Postgres (needs docker compose up)
make test-e2e          # full HTTP API workflow, in-process server + real Postgres/Redis
make test-race         # everything, with the race detector
make test-load         # concurrent queue drain under load, throughput + correctness
```

`make test-integration` is what proves the concurrency claim: it fires dozens of simultaneous "call next" requests at a queue and asserts every waiting patient is claimed by exactly one caller, none twice, none dropped. `make test-load` re-asserts the same property at 500 entries / 50 concurrent workers and reports throughput. See [ARCHITECTURE.md#testing-strategy](ARCHITECTURE.md#testing-strategy) for what each layer is actually checking and why it's split this way.

CI runs all of the above (lint, unit+race, integration+e2e against real service containers, and a Docker image build) on every push — see [`.github/workflows/ci.yml`](.github/workflows/ci.yml).

## Scalability considerations

- The API is stateless — JWTs carry identity, WebSocket fan-out goes through Redis — so it scales horizontally behind a load balancer with no sticky sessions.
- `department_id` is the queue's natural partition key; every hot query is already scoped to it, which is where this would shard first if a single Postgres instance's throughput ever became the bottleneck.
- The reminder worker is a separate process from the API, so neither competes with the other for resources or availability.

Full discussion in [ARCHITECTURE.md](ARCHITECTURE.md).

## Engineering decisions worth knowing about

- **PostgreSQL, not Redis, is the queue's source of truth** (`SELECT ... FOR UPDATE SKIP LOCKED`); Redis is a cache-aside layer and a pub/sub bus, deliberately not a competing data structure that could drift from the database. Rationale: [ARCHITECTURE.md](ARCHITECTURE.md#why-postgresql-owns-queue-concurrency-not-redis).
- **Rate limiting is a fixed window**, not a sliding window/token bucket — a documented trade-off for simplicity at clinic scale, not an oversight (`internal/httpserver/middleware/rate_limit.go`).
- **Idempotency is opt-in per route** (booking and check-in), applied via middleware, not global — most endpoints don't need it and shouldn't pay for it.
- **Audit logging never fails the primary operation** — a logging failure is surfaced to the structured logger, not returned as a 500 to a clinician trying to complete a consultation.

## License

MIT
