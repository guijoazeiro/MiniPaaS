package docker

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"time"

	"github.com/docker/docker/api/types/container"
)

type ContainerMetrics struct {
	State            string
	Running          bool
	RestartCount     int
	StartedAt        *time.Time
	CPUPercent       float64
	MemoryUsageBytes uint64
	MemoryLimitBytes uint64
	MemoryPercent    float64
}

// InspectMetrics returns a point-in-time snapshot. Docker's one-shot stats
// endpoint avoids keeping a stream open for every dashboard refresh.
func (c *Client) InspectMetrics(ctx context.Context, id string) (ContainerMetrics, error) {
	inspect, err := c.cli.ContainerInspect(ctx, id)
	if err != nil {
		return ContainerMetrics{}, fmt.Errorf("docker.InspectMetrics: inspect: %w", err)
	}
	metrics := ContainerMetrics{RestartCount: inspect.RestartCount}
	if inspect.State == nil {
		metrics.State = "unknown"
		return metrics, nil
	}
	metrics.State = string(inspect.State.Status)
	metrics.Running = inspect.State.Running
	if started, parseErr := time.Parse(time.RFC3339Nano, inspect.State.StartedAt); parseErr == nil && !started.IsZero() {
		metrics.StartedAt = &started
	}
	if !metrics.Running {
		return metrics, nil
	}

	reader, err := c.cli.ContainerStatsOneShot(ctx, id)
	if err != nil {
		return ContainerMetrics{}, fmt.Errorf("docker.InspectMetrics: stats: %w", err)
	}
	defer reader.Body.Close()
	var stats container.StatsResponse
	if err := json.NewDecoder(reader.Body).Decode(&stats); err != nil {
		return ContainerMetrics{}, fmt.Errorf("docker.InspectMetrics: decode stats: %w", err)
	}
	metrics.CPUPercent = cpuPercent(stats)
	metrics.MemoryUsageBytes = stats.MemoryStats.Usage
	metrics.MemoryLimitBytes = stats.MemoryStats.Limit
	if metrics.MemoryLimitBytes > 0 {
		metrics.MemoryPercent = float64(metrics.MemoryUsageBytes) / float64(metrics.MemoryLimitBytes) * 100
	}
	return metrics, nil
}

func cpuPercent(stats container.StatsResponse) float64 {
	if stats.CPUStats.CPUUsage.TotalUsage <= stats.PreCPUStats.CPUUsage.TotalUsage || stats.CPUStats.SystemUsage <= stats.PreCPUStats.SystemUsage {
		return 0
	}
	cpuDelta := float64(stats.CPUStats.CPUUsage.TotalUsage - stats.PreCPUStats.CPUUsage.TotalUsage)
	systemDelta := float64(stats.CPUStats.SystemUsage - stats.PreCPUStats.SystemUsage)
	onlineCPUs := stats.CPUStats.OnlineCPUs
	if onlineCPUs == 0 {
		onlineCPUs = uint32(len(stats.CPUStats.CPUUsage.PercpuUsage))
	}
	if onlineCPUs == 0 {
		return 0
	}
	percent := cpuDelta / systemDelta * float64(onlineCPUs) * 100
	if math.IsNaN(percent) || math.IsInf(percent, 0) {
		return 0
	}
	return percent
}
