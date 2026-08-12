DROP INDEX app_git_sources_github_installation_idx;

ALTER TABLE app_git_sources
    DROP CONSTRAINT app_git_sources_github_app_check,
    DROP CONSTRAINT app_git_sources_access_mode_check,
    DROP COLUMN private,
    DROP COLUMN github_repository_id,
    DROP COLUMN github_installation_id,
    DROP COLUMN access_mode;

DROP TABLE github_installations;
