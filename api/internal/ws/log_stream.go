package ws

import (
	"bytes"
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/docker/docker/pkg/stdcopy"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/guijoazeiro/MiniPaaS/api/internal/docker"
	"github.com/guijoazeiro/MiniPaaS/api/internal/domain"
)

const (
	writeWait  = 10 * time.Second
	pongWait   = 60 * time.Second
	pingPeriod = 54 * time.Second
)

type Frame struct {
	TS     time.Time `json:"ts"`
	Stream string    `json:"stream"`
	Line   string    `json:"line"`
}

type AppLookup interface {
	GetByName(ctx context.Context, name string) (domain.App, error)
}

type DockerLogs interface {
	StreamLogs(ctx context.Context, id string, opts docker.LogOptions) (io.ReadCloser, error)
}

type DeploymentLookup interface {
	GetActive(ctx context.Context, appID uuid.UUID) (domain.Deployment, error)
	ListByApp(ctx context.Context, appID uuid.UUID, limit int) ([]domain.Deployment, error)
}

type Handler struct {
	apps     AppLookup
	docker   DockerLogs
	deps     DeploymentLookup
	upgrader websocket.Upgrader
	log      *slog.Logger
}

func New(apps AppLookup, dk DockerLogs, deps DeploymentLookup, log *slog.Logger, dashboardOrigin ...string) *Handler {
	return &Handler{
		apps:   apps,
		docker: dk,
		deps:   deps,
		upgrader: websocket.Upgrader{
			CheckOrigin: websocketOriginChecker(dashboardOrigin...),
		},
		log: log,
	}
}

func (h *Handler) Serve(c *gin.Context) {
	name := c.Param("name")
	app, err := h.apps.GetByName(c.Request.Context(), name)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	dep, err := h.logDeployment(c.Request.Context(), app.ID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "no deployment with available logs"})
		return
	}
	if dep.ContainerID == "" {
		c.JSON(http.StatusConflict, gin.H{"error": "deployment has no container yet"})
		return
	}

	follow := c.Query("follow") == "true"
	tail := c.DefaultQuery("tail", "100")

	conn, err := h.upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		h.log.Error("ws upgrade", "err", err)
		return
	}
	writeMu := &sync.Mutex{}
	defer func() {
		writeMu.Lock()
		defer writeMu.Unlock()
		_ = conn.WriteControl(websocket.CloseMessage,
			websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""),
			time.Now().Add(writeWait))
		_ = conn.Close()
	}()

	streamCtx, cancel := context.WithCancel(context.Background())
	defer cancel()

	_ = conn.SetReadDeadline(time.Now().Add(pongWait))
	conn.SetPongHandler(func(string) error {
		return conn.SetReadDeadline(time.Now().Add(pongWait))
	})

	go func() {
		for {
			if _, _, err := conn.NextReader(); err != nil {
				cancel()
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
				err := conn.WriteControl(websocket.PingMessage, nil, time.Now().Add(writeWait))
				writeMu.Unlock()
				if err != nil {
					cancel()
					return
				}
			}
		}
	}()

	rc, err := h.docker.StreamLogs(streamCtx, dep.ContainerID, docker.LogOptions{Follow: follow, Tail: tail})
	if err != nil {
		writeMu.Lock()
		_ = conn.WriteJSON(gin.H{"error": "docker: " + err.Error()})
		writeMu.Unlock()
		return
	}
	defer rc.Close()

	sendFrame := func(stream, line string) {
		writeMu.Lock()
		defer writeMu.Unlock()
		_ = conn.SetWriteDeadline(time.Now().Add(writeWait))
		if err := conn.WriteJSON(Frame{TS: time.Now().UTC(), Stream: stream, Line: line}); err != nil {
			cancel()
		}
	}

	out := &lineWriter{stream: "stdout", onLine: sendFrame}
	errW := &lineWriter{stream: "stderr", onLine: sendFrame}

	if _, err := stdcopy.StdCopy(out, errW, rc); err != nil && !isClosedErr(err) {
		h.log.Warn("stdcopy", "app", name, "err", err)
	}
	out.flush()
	errW.flush()
}

func (h *Handler) logDeployment(ctx context.Context, appID uuid.UUID) (domain.Deployment, error) {
	dep, err := h.deps.GetActive(ctx, appID)
	if err == nil {
		return dep, nil
	}
	if !errors.Is(err, domain.ErrDeploymentNotFound) {
		return domain.Deployment{}, err
	}

	deployments, err := h.deps.ListByApp(ctx, appID, 50)
	if err != nil {
		return domain.Deployment{}, err
	}
	for _, candidate := range deployments {
		if candidate.Status == domain.DeploymentStatusFailed && candidate.ContainerID != "" {
			return candidate, nil
		}
	}
	return domain.Deployment{}, domain.ErrDeploymentNotFound
}

type lineWriter struct {
	stream string
	onLine func(stream, line string)
	buf    []byte
}

func (w *lineWriter) Write(p []byte) (int, error) {
	w.buf = append(w.buf, p...)
	for {
		i := bytes.IndexByte(w.buf, '\n')
		if i < 0 {
			break
		}
		line := string(w.buf[:i])
		line = strings.TrimRight(line, "\r")
		w.onLine(w.stream, line)
		w.buf = w.buf[i+1:]
	}
	return len(p), nil
}

func (w *lineWriter) flush() {
	if len(w.buf) == 0 {
		return
	}
	w.onLine(w.stream, string(w.buf))
	w.buf = nil
}

func isClosedErr(err error) bool {
	if err == nil {
		return false
	}
	return errors.Is(err, context.Canceled) ||
		errors.Is(err, io.EOF) ||
		websocket.IsCloseError(err,
			websocket.CloseNormalClosure,
			websocket.CloseGoingAway,
			websocket.CloseNoStatusReceived,
		)
}
