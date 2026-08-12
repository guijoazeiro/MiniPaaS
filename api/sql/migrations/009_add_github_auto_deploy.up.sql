ALTER TABLE app_git_sources
    ADD COLUMN auto_deploy BOOLEAN NOT NULL DEFAULT false;

CREATE TABLE github_webhook_deliveries (
    delivery_id TEXT PRIMARY KEY,
    event TEXT NOT NULL,
    repository_id BIGINT,
    commit_sha TEXT,
    status TEXT NOT NULL DEFAULT 'received',
    error_message TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    processed_at TIMESTAMPTZ,
    CONSTRAINT github_webhook_deliveries_status_check
        CHECK (status IN ('received', 'ignored', 'accepted', 'failed'))
);

ALTER TABLE deployments
    ADD COLUMN trigger_type TEXT NOT NULL DEFAULT 'manual',
    ADD COLUMN github_delivery_id TEXT,
    ADD CONSTRAINT deployments_trigger_type_check CHECK (trigger_type IN ('manual', 'webhook'));

CREATE INDEX app_git_sources_auto_deploy_repository_idx
    ON app_git_sources(github_repository_id)
    WHERE auto_deploy = true;

CREATE INDEX deployments_github_delivery_idx
    ON deployments(github_delivery_id)
    WHERE github_delivery_id IS NOT NULL;
