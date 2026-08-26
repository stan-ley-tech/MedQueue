# API Reference

Base URL: `http://localhost:8080/api/v1`. Full machine-readable spec: [`docs/openapi.yaml`](docs/openapi.yaml), also served at `GET /openapi.yaml` by a running instance.

## Conventions

**Auth**: every endpoint except `/auth/login`, `/auth/refresh`, and `/auth/logout` requires `Authorization: Bearer <access_token>`.

**Roles**: `admin`, `front_desk`, `clinician`. Each write endpoint below lists which roles may call it; reads are open to any authenticated role.

**Pagination**: list endpoints accept `?limit=20&offset=0` (`limit` capped at 100) and return:

```json
{ "items": [...], "total": 137, "limit": 20, "offset": 0 }
```

**Errors**: every error response, regardless of status code, has the same shape:

```json
{ "code": "VALIDATION_ERROR", "message": "request failed validation", "fields": { "email": "must be a valid email address" } }
```

`fields` is only present for `VALIDATION_ERROR`. `code` is stable and meant to be branched on programmatically; `message` is for humans and may reword over time.

| HTTP Status | code | Meaning |
|---|---|---|
| 401 | `UNAUTHORIZED` | missing/invalid/expired token |
| 403 | `FORBIDDEN` | authenticated, but the role can't do this |
| 404 | `NOT_FOUND` | resource doesn't exist |
| 409 | `CONFLICT` | duplicate resource, invalid state transition, double check-in |
| 422 | `VALIDATION_ERROR` | request body failed validation |
| 429 | `RATE_LIMITED` | too many requests |
| 500 | `INTERNAL_ERROR` | unexpected server error |
| 503 | `SERVICE_UNAVAILABLE` | a dependency is down |

**Idempotency**: `POST /appointments` and `POST /appointments/{id}/check-in` accept an optional `Idempotency-Key: <client-generated-uuid>` header. Retrying the exact same request with the same key returns the original response (marked with `Idempotency-Replayed: true`) instead of performing the operation again.

## Auth

| Method | Path | Role | Description |
|---|---|---|---|
| POST | `/auth/register` | admin | create a staff account |
| POST | `/auth/login` | — | exchange email/password for a session |
| POST | `/auth/refresh` | — | rotate a refresh token for a new session |
| POST | `/auth/logout` | — | revoke a refresh token |

```http
POST /auth/login
Content-Type: application/json

{ "email": "front-desk@clinic.example", "password": "correct horse battery staple" }
```

```json
{
  "access_token": "eyJhbGciOi...",
  "access_token_expires_at": "2026-08-26T10:15:00Z",
  "refresh_token": "9f1c2b...",
  "refresh_token_expires_at": "2026-09-02T10:00:00Z",
  "user": { "id": "...", "email": "front-desk@clinic.example", "name": "...", "role": "front_desk" }
}
```

Refresh tokens are single-use: `POST /auth/refresh` revokes the presented token and issues a new pair, so a stolen-then-replayed refresh token stops working the moment the legitimate client refreshes.

## Departments

| Method | Path | Role | Description |
|---|---|---|---|
| GET | `/departments` | any | list, `?search=` |
| GET | `/departments/{id}` | any | get one |
| POST | `/departments` | admin | create |
| PUT | `/departments/{id}` | admin | update |

## Doctors

| Method | Path | Role | Description |
|---|---|---|---|
| GET | `/doctors` | any | list, `?department_id=&active=` |
| GET | `/doctors/{id}` | any | get one |
| POST | `/doctors` | admin | create; optionally links a `user_id` for clinician login |
| PUT | `/doctors/{id}` | admin | update |

## Patients

| Method | Path | Role | Description |
|---|---|---|---|
| GET | `/patients` | any | list, `?search=` (matches name, MRN, phone) |
| GET | `/patients/{id}` | any | get one |
| POST | `/patients` | admin, front_desk | register a new patient |
| PUT | `/patients/{id}` | admin, front_desk | update demographics |

## Appointments

| Method | Path | Role | Description |
|---|---|---|---|
| GET | `/appointments` | any | list, filters below |
| GET | `/appointments/{id}` | any | get one |
| POST | `/appointments` | admin, front_desk | schedule (idempotent) |
| PUT | `/appointments/{id}` | admin, front_desk | reschedule (only while still `scheduled`) |
| POST | `/appointments/{id}/cancel` | admin, front_desk | cancel |
| POST | `/appointments/{id}/no-show` | admin, front_desk | mark a no-show before check-in |
| POST | `/appointments/{id}/check-in` | admin, front_desk | check in and enqueue (idempotent) |

List filters: `patient_id`, `doctor_id`, `department_id`, `status`, `from`, `to` (RFC3339).

```http
POST /appointments
Idempotency-Key: 3fae7e2a-...
Content-Type: application/json

{
  "patient_id": "...", "doctor_id": "...", "department_id": "...",
  "scheduled_at": "2026-09-01T14:30:00Z", "reason": "annual checkup"
}
```

### Appointment status lifecycle

```
scheduled -> checked_in -> in_queue -> in_consultation -> completed
     \-> cancelled            \-> cancelled       (terminal, no further transitions)
     \-> no_show               \-> no_show
```

Any transition not shown above is rejected with `409 CONFLICT`. See `internal/domain/appointment.go`.

## Queue

| Method | Path | Role | Description |
|---|---|---|---|
| GET | `/departments/{id}/queue` | any | current waiting line, priority-ordered |
| POST | `/departments/{id}/queue/call-next` | admin, clinician | atomically claim the next patient |
| POST | `/queue/{id}/start` | admin, clinician | begin consultation for a called patient |
| POST | `/queue/{id}/complete` | admin, clinician | finish consultation, optional `notes` |
| POST | `/queue/{id}/requeue` | admin, clinician | return an unresponsive called patient to the back of the line |
| POST | `/queue/{id}/no-show` | admin, clinician | drop an unresponsive patient, marks the appointment `no_show` |

`priority` on check-in is `0` (normal, default), `1` (urgent), or `2` (emergency). Within the same priority, first checked in is first called.

```http
POST /appointments/{appointment_id}/check-in
Idempotency-Key: 91b4...
Content-Type: application/json

{ "priority": 1 }
```

`call-next` returns `200 { "message": "no patients waiting" }` instead of a queue entry when the department's line is empty — this is not an error.

### Real-time updates

```
GET /ws/departments/{department_id}/queue?token=<access_token>
```

Upgrades to a WebSocket. The server pushes a JSON message on every queue state change in that department:

```json
{
  "type": "patient_called",
  "department_id": "...",
  "entry": { "id": "...", "patient_name": "Grace Hopper", "status": "called", "queue_number": 42, "...": "..." },
  "waiting_count": 3,
  "occurred_at": "2026-08-26T10:05:00Z"
}
```

`type` is one of `patient_checked_in`, `patient_called`, `consultation_started`, `consultation_completed`, `patient_requeued`, `patient_no_show`. The connection is server-to-client only (no client-sent commands); a dropped connection should reconnect and re-fetch `GET /departments/{id}/queue` to resync.

## Audit log

| Method | Path | Role | Description |
|---|---|---|---|
| GET | `/audit-logs` | admin | filterable by `entity_type`, `entity_id`, `actor_id` |

## Health

| Method | Path | Description |
|---|---|---|
| GET | `/healthz` | liveness — always 200 if the process is up |
| GET | `/readyz` | readiness — 200 only if PostgreSQL and Redis both respond |
