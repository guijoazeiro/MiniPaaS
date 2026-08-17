CREATE TABLE api_tokens (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id      UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name         TEXT NOT NULL CHECK (char_length(name) BETWEEN 1 AND 64),
    token_hash   TEXT NOT NULL UNIQUE CHECK (char_length(token_hash) = 64),
    token_prefix TEXT NOT NULL CHECK (char_length(token_prefix) BETWEEN 5 AND 32),
    scopes       TEXT[] NOT NULL DEFAULT ARRAY['read']::TEXT[],
    expires_at   TIMESTAMPTZ,
    revoked_at   TIMESTAMPTZ,
    last_used_at TIMESTAMPTZ,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX api_tokens_user_id_idx ON api_tokens (user_id, created_at DESC);
CREATE INDEX api_tokens_active_hash_idx ON api_tokens (token_hash)
    WHERE revoked_at IS NULL;
