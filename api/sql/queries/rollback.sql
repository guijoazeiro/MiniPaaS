-- name: RecordRollback :exec
INSERT INTO rollback_history (app_id, from_deployment, to_deployment, triggered_by)
VALUES (@app_id, @from_deployment, @to_deployment, @triggered_by);
