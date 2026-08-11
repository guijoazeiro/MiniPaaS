ALTER TABLE deployments
    DROP CONSTRAINT deployments_source_type_check,
    DROP COLUMN commit_message,
    DROP COLUMN commit_author,
    DROP COLUMN branch,
    DROP COLUMN repository,
    DROP COLUMN source_type;

DROP TABLE app_git_sources;
