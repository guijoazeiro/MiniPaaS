package docker

import (
	"testing"

	"github.com/docker/docker/api/types/container"
)

func TestCPUPercent(t *testing.T) {
	stats := container.StatsResponse{
		CPUStats:    container.CPUStats{CPUUsage: container.CPUUsage{TotalUsage: 250}, SystemUsage: 1_250, OnlineCPUs: 2},
		PreCPUStats: container.CPUStats{CPUUsage: container.CPUUsage{TotalUsage: 150}, SystemUsage: 1_050},
	}
	if got := cpuPercent(stats); got != 100 {
		t.Fatalf("cpuPercent = %v, want 100", got)
	}
}

func TestCPUPercentWithoutBaselineIsZero(t *testing.T) {
	if got := cpuPercent(container.StatsResponse{}); got != 0 {
		t.Fatalf("cpuPercent = %v, want 0", got)
	}
}

func TestMetricsFromStatsIncludesIO(t *testing.T) {
	stats := container.StatsResponse{
		Networks: map[string]container.NetworkStats{
			"eth0": {RxBytes: 120, TxBytes: 45},
			"eth1": {RxBytes: 8, TxBytes: 5},
		},
		BlkioStats: container.BlkioStats{IoServiceBytesRecursive: []container.BlkioStatEntry{
			{Op: "read", Value: 900},
			{Op: "write", Value: 300},
		}},
		PidsStats: container.PidsStats{Current: 4},
	}
	metrics := MetricsFromStats(stats, ContainerRuntime{State: "running", Running: true})
	if metrics.NetworkRxBytes != 128 || metrics.NetworkTxBytes != 50 || metrics.BlockReadBytes != 900 || metrics.BlockWriteBytes != 300 || metrics.Pids != 4 {
		t.Fatalf("metrics = %#v", metrics)
	}
}
