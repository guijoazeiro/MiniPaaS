package ws

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/guijoazeiro/MiniPaaS/api/internal/docker"
	"github.com/guijoazeiro/MiniPaaS/api/internal/domain"
)

type DockerMetricsStream interface {
	InspectContainerRuntime(ctx context.Context, id string) (docker.ContainerRuntime, error)
	StreamContainerStats(ctx context.Context, id string) (io.ReadCloser, error)
}

type MetricsHub struct {
	docker  DockerMetricsStream
	mu      sync.Mutex
	streams map[string]*metricsContainerStream
}

type metricsContainerStream struct {
	ctx         context.Context
	cancel      context.CancelFunc
	done        chan struct{}
	subscribers map[chan domain.MetricsFrame]struct{}
}

func NewMetricsHub(dockerClient DockerMetricsStream) *MetricsHub {
	return &MetricsHub{docker: dockerClient, streams: make(map[string]*metricsContainerStream)}
}

func (h *MetricsHub) Subscribe(containerID string) (<-chan domain.MetricsFrame, <-chan struct{}, func()) {
	frames := make(chan domain.MetricsFrame, 8)
	h.mu.Lock()
	stream, exists := h.streams[containerID]
	start := false
	if !exists {
		streamCtx, cancel := context.WithCancel(context.Background())
		stream = &metricsContainerStream{ctx: streamCtx, cancel: cancel, done: make(chan struct{}), subscribers: make(map[chan domain.MetricsFrame]struct{})}
		h.streams[containerID] = stream
		start = true
	}
	stream.subscribers[frames] = struct{}{}
	h.mu.Unlock()
	if start {
		go h.run(containerID, stream)
	}

	var once sync.Once
	unsubscribe := func() {
		once.Do(func() {
			h.mu.Lock()
			defer h.mu.Unlock()
			current, ok := h.streams[containerID]
			if !ok || current != stream {
				return
			}
			delete(stream.subscribers, frames)
			if len(stream.subscribers) == 0 {
				delete(h.streams, containerID)
				stream.cancel()
			}
		})
	}
	return frames, stream.done, unsubscribe
}

func (h *MetricsHub) run(containerID string, stream *metricsContainerStream) {
	ctx := stream.ctx
	defer stream.cancel()
	defer func() {
		h.mu.Lock()
		if current, ok := h.streams[containerID]; ok && current == stream {
			delete(h.streams, containerID)
		}
		close(stream.done)
		h.mu.Unlock()
	}()

	runtime, err := h.docker.InspectContainerRuntime(ctx, containerID)
	if err != nil {
		return
	}
	reader, err := h.docker.StreamContainerStats(ctx, containerID)
	if err != nil {
		return
	}
	defer reader.Close()

	decoder := json.NewDecoder(reader)
	for sample := 0; ; sample++ {
		var stats container.StatsResponse
		if err := decoder.Decode(&stats); err != nil {
			return
		}
		if sample%5 == 0 {
			if refreshed, refreshErr := h.docker.InspectContainerRuntime(ctx, containerID); refreshErr == nil {
				runtime = refreshed
			}
		}
		metrics := docker.MetricsFromStats(stats, runtime)
		h.publish(containerID, domain.MetricsFrame{
			Type: "metrics",
			TS:   time.Now().UTC(),
			Runtime: domain.RuntimeMetrics{
				ContainerID:      containerID,
				State:            metrics.State,
				RestartCount:     metrics.RestartCount,
				StartedAt:        metrics.StartedAt,
				CPUPercent:       metrics.CPUPercent,
				MemoryUsageBytes: metrics.MemoryUsageBytes,
				MemoryLimitBytes: metrics.MemoryLimitBytes,
				MemoryPercent:    metrics.MemoryPercent,
				NetworkRxBytes:   metrics.NetworkRxBytes,
				NetworkTxBytes:   metrics.NetworkTxBytes,
				BlockReadBytes:   metrics.BlockReadBytes,
				BlockWriteBytes:  metrics.BlockWriteBytes,
				Pids:             metrics.Pids,
			},
		})
	}
}

func (h *MetricsHub) publish(containerID string, frame domain.MetricsFrame) {
	h.mu.Lock()
	stream, ok := h.streams[containerID]
	if !ok {
		h.mu.Unlock()
		return
	}
	subscribers := make([]chan domain.MetricsFrame, 0, len(stream.subscribers))
	for subscriber := range stream.subscribers {
		subscribers = append(subscribers, subscriber)
	}
	h.mu.Unlock()

	for _, subscriber := range subscribers {
		select {
		case subscriber <- frame:
		default:
			select {
			case <-subscriber:
			default:
			}
			select {
			case subscriber <- frame:
			default:
			}
		}
	}
}

type MetricsStreamHandler struct {
	apps     AppLookup
	deps     DeploymentLookup
	hub      *MetricsHub
	upgrader websocket.Upgrader
	log      *slog.Logger
}

func NewMetricsStreamHandler(apps AppLookup, deps DeploymentLookup, dockerClient DockerMetricsStream, log *slog.Logger) *MetricsStreamHandler {
	return &MetricsStreamHandler{
		apps:     apps,
		deps:     deps,
		hub:      NewMetricsHub(dockerClient),
		upgrader: websocket.Upgrader{CheckOrigin: func(r *http.Request) bool { return true }},
		log:      log,
	}
}

func (h *MetricsStreamHandler) Serve(c *gin.Context) {
	app, err := h.apps.GetByName(c.Request.Context(), c.Param("name"))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	deployment, err := metricsDeployment(c.Request.Context(), h.deps, app.ID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "no deployment with an available container"})
		return
	}

	conn, err := h.upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		h.log.Error("metrics ws upgrade", "err", err)
		return
	}
	writeMu := &sync.Mutex{}
	defer func() {
		writeMu.Lock()
		defer writeMu.Unlock()
		_ = conn.WriteControl(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""), time.Now().Add(writeWait))
		_ = conn.Close()
	}()

	streamCtx, cancel := context.WithCancel(context.Background())
	defer cancel()
	frames, done, unsubscribe := h.hub.Subscribe(deployment.ContainerID)
	defer unsubscribe()

	_ = conn.SetReadDeadline(time.Now().Add(pongWait))
	conn.SetPongHandler(func(string) error { return conn.SetReadDeadline(time.Now().Add(pongWait)) })
	go func() {
		for {
			if _, _, readErr := conn.NextReader(); readErr != nil {
				cancel()
				unsubscribe()
				return
			}
		}
	}()
	go func() {
		ticker := time.NewTicker(pingPeriod)
		defer ticker.Stop()
		for {
			select {
			case <-streamCtx.Done():
				return
			case <-ticker.C:
				writeMu.Lock()
				pingErr := conn.WriteControl(websocket.PingMessage, nil, time.Now().Add(writeWait))
				writeMu.Unlock()
				if pingErr != nil {
					cancel()
					return
				}
			}
		}
	}()

	for {
		select {
		case <-streamCtx.Done():
			return
		case <-done:
			return
		case frame := <-frames:
			writeMu.Lock()
			_ = conn.SetWriteDeadline(time.Now().Add(writeWait))
			if writeErr := conn.WriteJSON(frame); writeErr != nil {
				writeMu.Unlock()
				cancel()
				return
			}
			writeMu.Unlock()
		}
	}
}

func metricsDeployment(ctx context.Context, deps DeploymentLookup, appID uuid.UUID) (domain.Deployment, error) {
	if active, err := deps.GetActive(ctx, appID); err == nil && active.ContainerID != "" {
		return active, nil
	}
	items, err := deps.ListByApp(ctx, appID, 50)
	if err != nil {
		return domain.Deployment{}, err
	}
	for _, item := range items {
		if item.ContainerID != "" {
			return item, nil
		}
	}
	return domain.Deployment{}, domain.ErrDeploymentNotFound
}
