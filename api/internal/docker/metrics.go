package docker

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
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
	NetworkRxBytes   uint64
	NetworkTxBytes   uint64
	BlockReadBytes   uint64
	BlockWriteBytes  uint64
	Pids             uint64
}

type ContainerRuntime struct {
	State        string
	Running      bool
	RestartCount int
	StartedAt    *time.Time
}

func (c *Client) InspectContainerRuntime(ctx context.Context, id string) (ContainerRuntime, error) {
	inspect, err := c.cli.ContainerInspect(ctx, id)
	if err != nil {
		return ContainerRuntime{}, fmt.Errorf("docker.InspectContainerRuntime: %w", err)
	}
	runtime := ContainerRuntime{RestartCount: inspect.RestartCount}
	if inspect.State == nil {
		runtime.State = "unknown"
		return runtime, nil
	}
	runtime.State = string(inspect.State.Status)
	runtime.Running = inspect.State.Running
	if started, parseErr := time.Parse(time.RFC3339Nano, inspect.State.StartedAt); parseErr == nil && !started.IsZero() {
		runtime.StartedAt = &started
	}
	return runtime, nil
}

func (c *Client) StreamContainerStats(ctx context.Context, id string) (io.ReadCloser, error) {
	reader, err := c.cli.ContainerStats(ctx, id, true)
	if err != nil {
		return nil, fmt.Errorf("docker.StreamContainerStats: %w", err)
	}
	return reader.Body, nil
}

// InspectMetrics returns a point-in-time snapshot. Docker's one-shot stats
// endpoint avoids keeping a stream open for every dashboard refresh.
func (c *Client) InspectMetrics(ctx context.Context, id string) (ContainerMetrics, error) {
	runtime, err := c.InspectContainerRuntime(ctx, id)
	if err != nil {
		return ContainerMetrics{}, err
	}
	metrics := ContainerMetrics{State: runtime.State, Running: runtime.Running, RestartCount: runtime.RestartCount, StartedAt: runtime.StartedAt}
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
	applyIOMetrics(&metrics, stats)
	return metrics, nil
}

func MetricsFromStats(stats container.StatsResponse, runtime ContainerRuntime) ContainerMetrics {
	metrics := ContainerMetrics{State: runtime.State, Running: runtime.Running, RestartCount: runtime.RestartCount, StartedAt: runtime.StartedAt}
	metrics.CPUPercent = cpuPercent(stats)
	metrics.MemoryUsageBytes = stats.MemoryStats.Usage
	metrics.MemoryLimitBytes = stats.MemoryStats.Limit
	if metrics.MemoryLimitBytes > 0 {
		metrics.MemoryPercent = float64(metrics.MemoryUsageBytes) / float64(metrics.MemoryLimitBytes) * 100
	}
	applyIOMetrics(&metrics, stats)
	return metrics
}

func applyIOMetrics(metrics *ContainerMetrics, stats container.StatsResponse) {
	for _, network := range stats.Networks {
		metrics.NetworkRxBytes += network.RxBytes
		metrics.NetworkTxBytes += network.TxBytes
	}
	for _, item := range stats.BlkioStats.IoServiceBytesRecursive {
		switch item.Op {
		case "read", "Read":
			metrics.BlockReadBytes += item.Value
		case "write", "Write":
			metrics.BlockWriteBytes += item.Value
		}
	}
	metrics.Pids = stats.PidsStats.Current
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
