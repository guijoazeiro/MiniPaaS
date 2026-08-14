package handler

import (
	"context"
	"log/slog"
	"net/http"
	"sort"
	"time"

	"github.com/gin-gonic/gin"
)

type ReadinessProbe func(context.Context) error

type ReadinessHandler struct {
	probes  map[string]ReadinessProbe
	timeout time.Duration
	log     *slog.Logger
}

func NewReadinessHandler(timeout time.Duration, probes map[string]ReadinessProbe, log *slog.Logger) *ReadinessHandler {
	if timeout <= 0 {
		timeout = 3 * time.Second
	}
	copyProbes := make(map[string]ReadinessProbe, len(probes))
	for name, probe := range probes {
		copyProbes[name] = probe
	}
	return &ReadinessHandler{probes: copyProbes, timeout: timeout, log: log}
}

func (h *ReadinessHandler) Serve(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), h.timeout)
	defer cancel()

	dependencies := make(map[string]string, len(h.probes))
	ready := true
	names := make([]string, 0, len(h.probes))
	for name := range h.probes {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		probe := h.probes[name]
		if probe == nil {
			dependencies[name] = "not_configured"
			ready = false
			continue
		}
		if err := probe(ctx); err != nil {
			dependencies[name] = "error"
			ready = false
			if h.log != nil {
				h.log.Warn("readiness probe failed", "dependency", name, "err", err)
			}
			continue
		}
		dependencies[name] = "ok"
	}

	status := http.StatusOK
	state := "ready"
	if !ready {
		status = http.StatusServiceUnavailable
		state = "not_ready"
	}
	c.JSON(status, gin.H{"status": state, "dependencies": dependencies})
}
