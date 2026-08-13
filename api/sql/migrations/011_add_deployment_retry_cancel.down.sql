DROP INDEX deployments_retry_of_idx;

ALTER TABLE deployments
    DROP COLUMN cancel_requested,
    DROP COLUMN retry_of,
    DROP COLUMN attempt;
