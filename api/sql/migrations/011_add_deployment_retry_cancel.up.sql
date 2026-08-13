ALTER TABLE deployments
    ADD COLUMN attempt INTEGER NOT NULL DEFAULT 1,
    ADD COLUMN retry_of UUID REFERENCES deployments(id) ON DELETE SET NULL,
    ADD COLUMN cancel_requested BOOLEAN NOT NULL DEFAULT false;

CREATE INDEX deployments_retry_of_idx ON deployments(retry_of) WHERE retry_of IS NOT NULL;
