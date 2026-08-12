-- name: ClaimGitHubWebhookDelivery :one
INSERT INTO github_webhook_deliveries (delivery_id, event, repository_id, commit_sha)
VALUES (@delivery_id, @event, @repository_id, @commit_sha)
ON CONFLICT (delivery_id) DO NOTHING
RETURNING delivery_id;

-- name: CompleteGitHubWebhookDelivery :exec
UPDATE github_webhook_deliveries
SET status = @status,
    error_message = @error_message,
    processed_at = now()
WHERE delivery_id = @delivery_id;
