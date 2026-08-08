package ws

import (
	"bytes"
	"context"
	"fmt"
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

type Handler struct {
	apps     AppLookup
	docker   DockerLogs
	getDep   func(ctx context.Context, appID uuid.UUID) (domain.Deployment, error)
	upgrader websocket.Upgrader
	log      *slog.Logger
}

func New(apps AppLookup, dk DockerLogs, getDep func(ctx context.Context, appID uuid.UUID) (domain.Deployment, error), log *slog.Logger) *Handler {
	return &Handler{
		apps:   apps,
		docker: dk,
		getDep: getDep,
		upgrader: websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool { return true },
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
	dep, err := h.getDep(c.Request.Context(), app.ID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "no active deployment"})
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
	defer conn.Close()

	streamCtx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		for {
			if _, _, err := conn.NextReader(); err != nil {
				cancel()
				return
			}
		}
	}()

	rc, err := h.docker.StreamLogs(streamCtx, dep.ContainerID, docker.LogOptions{Follow: follow, Tail: tail})
	if err != nil {
		_ = conn.WriteJSON(gin.H{"error": "docker: " + err.Error()})
		return
	}
	defer rc.Close()

	writeMu := &sync.Mutex{}
	sendFrame := func(stream, line string) {
		writeMu.Lock()
		defer writeMu.Unlock()
		_ = conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
		_ = conn.WriteJSON(Frame{TS: time.Now().UTC(), Stream: stream, Line: line})
	}

	out := &lineWriter{stream: "stdout", onLine: sendFrame}
	errW := &lineWriter{stream: "stderr", onLine: sendFrame}

	if _, err := stdcopy.StdCopy(out, errW, rc); err != nil && !isClosedErr(err) {
		h.log.Warn("stdcopy", "app", name, "err", err)
	}
	out.flush()
	errW.flush()
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
	msg := err.Error()
	return strings.Contains(msg, "context canceled") ||
		strings.Contains(msg, "use of closed") ||
		strings.Contains(msg, "EOF") ||
		strings.Contains(msg, fmt.Sprint(io.EOF))
}
