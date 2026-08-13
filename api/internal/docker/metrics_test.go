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
