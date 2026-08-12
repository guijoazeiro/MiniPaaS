CREATE TABLE github_installations (
    installation_id BIGINT PRIMARY KEY,
    account_login TEXT NOT NULL,
    account_type TEXT NOT NULL,
    repository_selection TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

ALTER TABLE app_git_sources
    ADD COLUMN access_mode TEXT NOT NULL DEFAULT 'public',
    ADD COLUMN github_installation_id BIGINT REFERENCES github_installations(installation_id) ON DELETE RESTRICT,
    ADD COLUMN github_repository_id BIGINT,
    ADD COLUMN private BOOLEAN NOT NULL DEFAULT false,
    ADD CONSTRAINT app_git_sources_access_mode_check CHECK (access_mode IN ('public', 'github_app')),
    ADD CONSTRAINT app_git_sources_github_app_check CHECK (
        (access_mode = 'public' AND github_installation_id IS NULL AND github_repository_id IS NULL)
        OR
        (access_mode = 'github_app' AND github_installation_id IS NOT NULL AND github_repository_id IS NOT NULL)
    );

CREATE INDEX app_git_sources_github_installation_idx
    ON app_git_sources(github_installation_id)
    WHERE github_installation_id IS NOT NULL;
