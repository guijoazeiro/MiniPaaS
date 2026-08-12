DROP INDEX deployments_github_delivery_idx;
DROP INDEX app_git_sources_auto_deploy_repository_idx;

ALTER TABLE deployments
    DROP CONSTRAINT deployments_trigger_type_check,
    DROP COLUMN github_delivery_id,
    DROP COLUMN trigger_type;

DROP TABLE github_webhook_deliveries;

ALTER TABLE app_git_sources
    DROP COLUMN auto_deploy;
