CREATE TABLE custom_domains (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    app_id UUID NOT NULL REFERENCES apps(id) ON DELETE CASCADE,
    hostname TEXT NOT NULL UNIQUE,
    status TEXT NOT NULL DEFAULT 'pending',
    last_error TEXT,
    verified_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT custom_domains_status_check
        CHECK (status IN ('pending', 'verified', 'active', 'error'))
);

CREATE INDEX custom_domains_app_id_idx ON custom_domains(app_id);
