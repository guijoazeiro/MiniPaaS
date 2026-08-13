ALTER TABLE deployments
    ADD COLUMN candidate_container_id TEXT,
    ADD COLUMN candidate_port INTEGER;

CREATE INDEX deployments_candidate_idx
    ON deployments(candidate_container_id)
    WHERE candidate_container_id IS NOT NULL;
