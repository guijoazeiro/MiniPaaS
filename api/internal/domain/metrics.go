package domain

import (
	"time"

	"github.com/google/uuid"
)

// RuntimeMetrics describes the last snapshot collected from the active
// container. Values are intentionally expressed in API-friendly units so the
// dashboard does not need to know Docker's internal stats format.
type RuntimeMetrics struct {
	ContainerID      string     `json:"container_id,omitempty"`
	State            string     `json:"state"`
	RestartCount     int        `json:"restart_count"`
	UptimeSeconds    int64      `json:"uptime_seconds"`
	StartedAt        *time.Time `json:"started_at,omitempty"`
	CPUPercent       float64    `json:"cpu_percent"`
	MemoryUsageBytes uint64     `json:"memory_usage_bytes"`
	MemoryLimitBytes uint64     `json:"memory_limit_bytes"`
	MemoryPercent    float64    `json:"memory_percent"`
	NetworkRxBytes   uint64     `json:"network_rx_bytes"`
	NetworkTxBytes   uint64     `json:"network_tx_bytes"`
	BlockReadBytes   uint64     `json:"block_read_bytes"`
	BlockWriteBytes  uint64     `json:"block_write_bytes"`
	Pids             uint64     `json:"pids"`
}

type DeploymentMetrics struct {
	Total             int     `json:"total"`
	Successful        int     `json:"successful"`
	Failed            int     `json:"failed"`
	InProgress        int     `json:"in_progress"`
	SuccessRate       float64 `json:"success_rate"`
	AverageDurationMs int64   `json:"average_duration_ms"`
}

type HealthCheckFailure struct {
	DeploymentID uuid.UUID `json:"deployment_id"`
	Message      string    `json:"message"`
	CreatedAt    time.Time `json:"created_at"`
}

type AppMetrics struct {
	AppName             string               `json:"app_name"`
	CollectedAt         time.Time            `json:"collected_at"`
	Runtime             *RuntimeMetrics      `json:"runtime,omitempty"`
	Deployments         DeploymentMetrics    `json:"deployments"`
	HealthCheckFailures []HealthCheckFailure `json:"health_check_failures"`
}

type MetricsFrame struct {
	Type    string         `json:"type"`
	TS      time.Time      `json:"ts"`
	Runtime RuntimeMetrics `json:"runtime"`
}
