package domain

// BuildQueueStats describes the in-memory build scheduler. The queue is
// intentionally process-local; deployments remain persisted in PostgreSQL.
type BuildQueueStats struct {
	Limit  int `json:"limit"`
	Active int `json:"active"`
	Queued int `json:"queued"`
}

// CapacitySnapshot is the small operational view used by the dashboard and
// automation clients. It exposes counts and configured limits, never secrets.
type CapacitySnapshot struct {
	AppsTotal                 int             `json:"apps_total"`
	AppsRunning               int             `json:"apps_running"`
	MaxAppsPerUser            int             `json:"max_apps_per_user"`
	Builds                    BuildQueueStats `json:"builds"`
	ContainerMemoryLimitBytes int64           `json:"container_memory_limit_bytes"`
	ContainerNanoCPUs         int64           `json:"container_nano_cpus"`
	ContainerPidsLimit        int64           `json:"container_pids_limit"`
}
