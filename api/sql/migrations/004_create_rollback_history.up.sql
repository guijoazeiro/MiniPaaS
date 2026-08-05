CREATE TABLE rollback_history (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    app_id          UUID NOT NULL REFERENCES apps(id) ON DELETE CASCADE,
    from_deployment UUID REFERENCES deployments(id) ON DELETE SET NULL,
    to_deployment   UUID REFERENCES deployments(id) ON DELETE SET NULL,
    triggered_by    TEXT,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_rollback_history_app_id ON rollback_history(app_id);
