package ws

import (
	"bytes"
	"context"
	"io"
	"sync/atomic"
	"testing"
	"time"

	"github.com/guijoazeiro/MiniPaaS/api/internal/docker"
	"github.com/guijoazeiro/MiniPaaS/api/internal/domain"
)

type fakeMetricsStreamDocker struct {
	streams atomic.Int32
	payload string
}

func (f *fakeMetricsStreamDocker) InspectContainerRuntime(context.Context, string) (docker.ContainerRuntime, error) {
	return docker.ContainerRuntime{State: "running", Running: true}, nil
}

func (f *fakeMetricsStreamDocker) StreamContainerStats(context.Context, string) (io.ReadCloser, error) {
	f.streams.Add(1)
	return io.NopCloser(bytes.NewBufferString(f.payload)), nil
}

func TestMetricsHubSharesContainerStream(t *testing.T) {
	dockerClient := &fakeMetricsStreamDocker{payload: `{"cpu_stats":{"cpu_usage":{"total_usage":200},"system_cpu_usage":2000,"online_cpus":1},"precpu_stats":{"cpu_usage":{"total_usage":100},"system_cpu_usage":1000}}`}
	hub := NewMetricsHub(dockerClient)
	framesA, doneA, unsubscribeA := hub.Subscribe("container")
	defer unsubscribeA()
	framesB, doneB, unsubscribeB := hub.Subscribe("container")
	defer unsubscribeB()

	for name, frames := range map[string]<-chan domain.MetricsFrame{
		"a": framesA,
		"b": framesB,
	} {
		select {
		case frame := <-frames:
			if frame.Type != "metrics" || frame.Runtime.CPUPercent != 10 {
				t.Fatalf("%s frame = %#v", name, frame)
			}
		case <-time.After(2 * time.Second):
			t.Fatalf("timed out waiting for %s frame", name)
		}
	}
	if got := dockerClient.streams.Load(); got != 1 {
		t.Fatalf("stats streams = %d, want 1", got)
	}
	select {
	case <-doneA:
	case <-time.After(2 * time.Second):
		t.Fatal("stream A did not finish")
	}
	select {
	case <-doneB:
	case <-time.After(2 * time.Second):
		t.Fatal("stream B did not finish")
	}
}
