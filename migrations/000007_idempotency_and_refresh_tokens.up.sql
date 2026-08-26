-- Stores the outcome of a state-changing request keyed by the client-
-- supplied Idempotency-Key header, so a retried request (network blip,
-- double-tap) replays the original response instead of double-booking
-- an appointment or double check-in.
-- response_body is BYTEA, not JSONB: it must replay byte-for-byte exactly
-- what the client received the first time, and JSONB silently reformats
-- (whitespace, key order) on storage, which would break that guarantee.
CREATE TABLE idempotency_keys (
    key            TEXT PRIMARY KEY,
    request_path   TEXT NOT NULL,
    request_hash   TEXT NOT NULL,
    status_code    INTEGER,
    response_body  BYTEA,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    expires_at     TIMESTAMPTZ NOT NULL
);

CREATE INDEX idx_idempotency_keys_expires_at ON idempotency_keys (expires_at);

-- Refresh tokens are stored hashed and revocable, so logout / password
-- reset can invalidate sessions server-side despite JWTs being stateless.
CREATE TABLE refresh_tokens (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id     UUID NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    token_hash  TEXT NOT NULL,
    revoked_at  TIMESTAMPTZ,
    expires_at  TIMESTAMPTZ NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX idx_refresh_tokens_token_hash ON refresh_tokens (token_hash);
CREATE INDEX idx_refresh_tokens_user_id ON refresh_tokens (user_id) WHERE revoked_at IS NULL;
