DROP INDEX deployments_candidate_idx;

ALTER TABLE deployments
    DROP COLUMN candidate_port,
    DROP COLUMN candidate_container_id;
