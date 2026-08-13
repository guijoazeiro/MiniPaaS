CREATE TABLE deployment_logs (
    id BIGSERIAL PRIMARY KEY,
    deployment_id UUID NOT NULL REFERENCES deployments(id) ON DELETE CASCADE,
    stage TEXT NOT NULL,
    stream TEXT NOT NULL DEFAULT 'system',
    message TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT deployment_logs_stage_check CHECK (stage IN ('queued', 'cloning', 'building', 'starting', 'health_check', 'publishing', 'cleanup', 'runtime', 'error')),
    CONSTRAINT deployment_logs_stream_check CHECK (stream IN ('stdout', 'stderr', 'system'))
);

CREATE INDEX deployment_logs_deployment_id_id_idx ON deployment_logs(deployment_id, id);
