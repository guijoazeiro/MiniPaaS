CREATE TABLE audit_events (
    id          BIGSERIAL PRIMARY KEY,
    user_id     UUID REFERENCES users(id) ON DELETE SET NULL,
    action      TEXT NOT NULL,
    method      TEXT NOT NULL,
    path        TEXT NOT NULL,
    status_code INTEGER NOT NULL,
    request_id  TEXT NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX audit_events_created_at_idx ON audit_events (created_at DESC, id DESC);
CREATE INDEX audit_events_user_id_idx ON audit_events (user_id, created_at DESC);
