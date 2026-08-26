# Architecture

This document explains how MedQueue is put together and why, at the level of detail someone would need before making a non-trivial change to it.

## Layering

```
cmd/api        entrypoint: wiring, HTTP server, graceful shutdown
cmd/worker     entrypoint: background reminder loop
cmd/migrate    entrypoint: schema migrations

internal/handler      HTTP transport: decode, validate, call a service, encode
internal/service      business logic, transaction boundaries, orchestration
internal/repository   PostgreSQL access, one file per aggregate
internal/domain       plain structs and the appointment/queue state machines
internal/cache        Redis: queue snapshot cache + pub/sub
internal/ws           WebSocket hub: Redis pub/sub -> connected clients
internal/httpserver   response/error envelope, request context helpers
internal/httpserver/middleware   auth, RBAC, rate limiting, idempotency, logging
internal/router       route table, the one package allowed to import both
                       handler and httpserver/middleware
internal/auth         JWT issuance/verification, password hashing
internal/config       environment variable loading and validation
```

Dependencies point one way: `handler -> service -> repository -> db`. Handlers never touch SQL; services never touch `net/http`; repositories never contain business rules. `internal/domain` has no dependencies on anything else in the module, so it can be imported everywhere without risk of a cycle.

Every repository is defined as an interface in `internal/repository/interfaces.go` before it's implemented. Services depend on those interfaces, not on `*repository.PatientPG` etc. — that's what makes `internal/service` testable with in-memory fakes instead of a real database, and it's why `internal/service/queue_service.go` depends on a `QueueCache` interface (`internal/service/ports.go`) instead of `*cache.QueueCache` directly.

## Request lifecycle

A write request (say, `POST /appointments/{id}/check-in`) flows as:

1. **Middleware chain**: request ID assignment, panic recovery, structured logging, CORS, rate limiting, JWT authentication, RBAC role check, and — for this specific route — idempotency key handling.
2. **Handler**: decodes and validates the JSON body against struct tags, resolves the caller's `Actor` (user ID, role, and doctor ID if they're a clinician) from the JWT claims, and calls `QueueService.CheckIn`.
3. **Service**: opens a PostgreSQL transaction, transitions the appointment `scheduled -> checked_in -> in_queue`, creates the queue entry, records an audit log entry, all inside that one transaction. If any step fails, the whole thing rolls back.
4. **Post-commit**: only after the transaction commits does the service invalidate the Redis snapshot cache for that department and publish a `patient_checked_in` event.
5. **Response**: the handler serializes the result into the response DTO and the consistent error envelope handles anything that went wrong along the way.

The ordering in step 4 is deliberate: cache invalidation and event publication happen *after* commit, never before or interleaved with it, so a WebSocket client or a cache reader can never observe a state that the database then fails to persist.

## Why PostgreSQL owns queue concurrency, not Redis

The obvious way to build a "real-time queue" is a Redis sorted set: `ZADD` to enqueue, `ZPOPMIN` to call next. It's fast and it's the first thing most people reach for. MedQueue deliberately doesn't do that, for one reason: a sorted set has no relationship to the `appointments` row it represents. The moment a "call next" operation needs to also transition an appointment's status, you have two systems that must agree, no transaction that spans both, and a class of bugs where Redis says "called" and Postgres says "still waiting" after a partial failure — in a system whose entire job is telling a patient where they stand in line, that's not acceptable.

Instead:

- **PostgreSQL is the single source of truth** for the queue. `QueueRepository.CallNext` (`internal/repository/queue_pg.go`) runs:

  ```sql
  UPDATE queue_entries
  SET status = 'called', doctor_id = COALESCE($2, doctor_id), called_at = now()
  WHERE id = (
      SELECT id FROM queue_entries
      WHERE department_id = $1 AND status = 'waiting'
      ORDER BY priority DESC, checked_in_at ASC
      FOR UPDATE SKIP LOCKED
      LIMIT 1
  )
  RETURNING ...
  ```

  `FOR UPDATE SKIP LOCKED` is what makes this safe under concurrency: if two clinicians (or two API replicas) call this at the same instant, the second transaction's row lock attempt on the row the first transaction already selected doesn't block and doesn't retry — it's skipped, and the second transaction moves on to the next candidate row. Nobody is ever double-assigned, and nobody blocks waiting for a lock to release. This is verified directly in `tests/integration/queue_concurrency_test.go`, which fires 60 concurrent callers at a 40-entry queue and asserts every entry is claimed by exactly one caller.

- **Redis is a cache and a broadcast channel, not a data structure with its own opinion about ordering.** `internal/cache/queue_cache.go` implements cache-aside: `GetSnapshot`/`SetSnapshot` cache the department's waiting list for 30 seconds so a busy dashboard polling `GET /departments/{id}/queue` doesn't hit Postgres on every request, and every mutation calls `InvalidateSnapshot` so the next read is never stale for longer than one round trip. Separately, `Publish` pushes a `QueueEvent` onto a Redis pub/sub channel (`queue:{departmentID}:events`) that the WebSocket hub subscribes to.

- If Redis is down, `QueueService.Snapshot` falls back to querying PostgreSQL directly and logs a warning — the queue keeps working, just without the cache's latency benefit and without live WebSocket updates. If PostgreSQL is down, nothing works, which is correct: it's the actual database.

## WebSocket fan-out across replicas

The WebSocket hub (`internal/ws`) does not hold any queue state. Each API process subscribes once, at startup, to Redis pub/sub with a pattern subscription (`PSUBSCRIBE queue:*:events`), and rebroadcasts every message it receives to whichever locally-connected clients are watching that department. This means the API is horizontally scalable: a client connected to replica A gets updates for a change made through replica B, because both replicas are subscribed to the same Redis channel. Nothing about the WebSocket layer assumes a single process.

Browsers can't set an `Authorization` header on a WebSocket handshake, so the access token travels as `?token=` on the upgrade request instead and is validated with the same JWT parsing logic as the header-based flow (`handler.QueueEvents`).

## Transactions and the `db.Pool.WithTx` pattern

`internal/db/tx.go` defines `DBTX`, an interface satisfied by both `*pgxpool.Pool` and `pgx.Tx`. Every repository resolves its actual querier via `pool.Querier(ctx)`, which checks whether a transaction has been stashed in the context by `WithTx` and uses it if so, or falls back to the plain pool. The effect: repository methods don't know or care whether they're running inside a transaction. A service composes several repository calls inside `pool.WithTx(ctx, func(ctx) error { ... })`, and every one of those calls transparently joins the same transaction. This is how `QueueService.CheckIn` gets two appointment status transitions and a queue entry insert to commit or roll back as one unit, without repositories needing transaction-aware method variants.

## Idempotency

`POST /appointments` (scheduling) and `POST /appointments/{id}/check-in` accept an `Idempotency-Key` header. The middleware (`internal/httpserver/middleware/idempotency.go`) hashes the request body, and calls `IdempotencyRepository.Reserve`, which does a single `INSERT ... ON CONFLICT (key) DO NOTHING`: if the key is new, the request proceeds normally and the middleware records the eventual response against that key; if the key was already used, the original response is replayed byte-for-byte instead of re-running the operation. This is what stops a client's retry-after-timeout from double-booking a slot or double-enqueuing a patient. The `ON CONFLICT` insert is what makes the "claim" step itself race-free between two concurrent requests bearing the same key — see `tests/integration/idempotency_repository_test.go`.

## RBAC model

Three roles: `admin`, `front_desk`, `clinician`. Enforcement happens in two places for different reasons:

- **Route-level** (`internal/httpserver/middleware.RequireRole`): coarse checks like "only admins can create departments" — a role either can or can't call an endpoint at all.
- **Actor threading**: every service method that mutates state takes a `service.Actor` (user ID, role, and — for clinicians — their linked doctor ID) built from the JWT claims. `QueueService.CallNext` uses `actor.DoctorID` to attribute a call to the clinician who made it, not a value the client supplies, so a clinician can't claim a patient on another clinician's behalf by editing a request body.

## Audit logging

`AuditService.Record` is called from inside the same transaction as the change it documents wherever a transaction is already open (department creation, appointment scheduling, queue transitions), so the audit trail and the mutation it describes commit or roll back together. It intentionally never returns an error to its caller — a logging failure must not fail the primary operation — and instead logs the failure through the structured logger for alerting.

## Configuration and graceful shutdown

All configuration is environment variables (`internal/config`), validated at startup: a missing `DATABASE_URL` or, in production, a missing/duplicate JWT secret, fails the process immediately instead of surfacing as a confusing runtime error later. `cmd/api/main.go` listens for `SIGINT`/`SIGTERM`, stops accepting new connections via `http.Server.Shutdown`, and gives in-flight requests up to `SHUTDOWN_TIMEOUT_SECONDS` to finish before exiting. `cmd/worker/main.go` follows the same signal-handling pattern for its scan loop.

## Scalability considerations

- **API is stateless and horizontally scalable.** No in-memory session state; JWTs carry identity, WebSocket fan-out goes through Redis, so any number of replicas behind a load balancer works without sticky sessions.
- **The queue's natural partition key is `department_id`.** Every hot query (`CallNext`, `ListWaiting`, the cache snapshot) is scoped to a single department, so this is the axis along which the system would shard if a single Postgres instance's write throughput ever became the bottleneck — which, for a clinic-scale system (tens of departments, hundreds of concurrent patients), is far beyond what a single well-indexed Postgres instance handles.
- **Indexes are chosen to match the actual query shapes**, not added generically: a partial index on `queue_entries (department_id, priority DESC, checked_in_at ASC) WHERE status = 'waiting'` is exactly the `CallNext` query; a partial index on `appointments (scheduled_at) WHERE status = 'scheduled' AND reminder_sent_at IS NULL` is exactly the reminder worker's scan. See `migrations/` for all of them.
- **The reminder worker runs as a separate process** from the API, so a slow notification provider or a large reminder backlog never competes with API request latency, and either process can be scaled or restarted independently.
- **Rate limiting is a deliberate simplification** (fixed window over Redis `INCR`+`EXPIRE`, not a sliding window or token bucket) — see the comment in `internal/httpserver/middleware/rate_limit.go` for the trade-off and when it would need to be revisited.

## Testing strategy

- **Unit** (`internal/**/*_test.go`): pure logic with no external dependencies — the appointment/queue state machines, JWT round-tripping, request validation. Run with `make test-unit`.
- **Integration** (`tests/integration`, build tag `integration`): repository behavior against a real, migrated PostgreSQL instance, including the concurrency test for `CallNext` and the idempotency claim-check race. Run with `make test-integration`.
- **E2E** (`tests/e2e`, build tag `e2e`): the full HTTP API, in-process (`httptest.Server`) but backed by the real database and Redis, driving the entire patient journey through actual HTTP calls exactly as a client would. Run with `make test-e2e`.
- **Load** (`tests/load`, build tag `load`): seeds a few hundred waiting queue entries and drains them with dozens of concurrent workers, re-asserting the no-double-claim property at a scale where a locking bug is far more likely to surface, and reporting throughput. Run with `make test-load`.
- **Race detector**: unit and integration tests both run with `-race` in CI.

CI (`.github/workflows/ci.yml`) runs lint/vet and unit tests on every push, then integration and e2e tests against real Postgres/Redis service containers, then verifies both Docker images build.
