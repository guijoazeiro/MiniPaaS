CREATE TABLE app_git_sources (
    app_id UUID PRIMARY KEY REFERENCES apps(id) ON DELETE CASCADE,
    repository TEXT NOT NULL,
    branch TEXT NOT NULL DEFAULT 'main',
    build_context TEXT NOT NULL DEFAULT '.',
    dockerfile_path TEXT NOT NULL DEFAULT 'Dockerfile',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

ALTER TABLE deployments
    ADD COLUMN source_type TEXT NOT NULL DEFAULT 'upload',
    ADD COLUMN repository TEXT,
    ADD COLUMN branch TEXT,
    ADD COLUMN commit_author TEXT,
    ADD COLUMN commit_message TEXT,
    ADD CONSTRAINT deployments_source_type_check CHECK (source_type IN ('upload', 'git'));
