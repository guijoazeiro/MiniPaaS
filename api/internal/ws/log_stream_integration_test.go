package ws

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/docker/docker/pkg/stdcopy"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/guijoazeiro/MiniPaaS/api/internal/docker"
	"github.com/guijoazeiro/MiniPaaS/api/internal/domain"
)

// fakeAppLookup returns a fixed app for any name.
type fakeAppLookup struct{ app domain.App }

func (f fakeAppLookup) GetByName(_ context.Context, name string) (domain.App, error) {
	if name != f.app.Name {
		return domain.App{}, domain.ErrAppNotFound
	}
	return f.app, nil
}

// fakeDockerLogs returns a stream that emits the pre-baked bytes and closes.
type fakeDockerLogs struct{ payload []byte }

func (f fakeDockerLogs) StreamLogs(_ context.Context, _ string, _ docker.LogOptions) (io.ReadCloser, error) {
	pr, pw := io.Pipe()
	go func() {
		_, _ = pw.Write(f.payload)
		_ = pw.Close()
	}()
	return pr, nil
}

// makeDockerStream builds a byte slice in Docker's multiplexed log format
// with the given stdout / stderr lines interleaved.
func makeDockerStream(t *testing.T, entries []struct{ stream, line string }) []byte {
	t.Helper()
	buf := &strings.Builder{}
	stdout := stdcopy.NewStdWriter(stringWriter{buf}, stdcopy.Stdout)
	stderr := stdcopy.NewStdWriter(stringWriter{buf}, stdcopy.Stderr)
	for _, e := range entries {
		w := stdout
		if e.stream == "stderr" {
			w = stderr
		}
		if _, err := w.Write([]byte(e.line + "\n")); err != nil {
			t.Fatal(err)
		}
	}
	return []byte(buf.String())
}

// stringWriter adapts strings.Builder to io.Writer (Builder already implements it,
// but keeping an explicit wrapper avoids import gymnastics).
type stringWriter struct{ b *strings.Builder }

func (s stringWriter) Write(p []byte) (int, error) { return s.b.Write(p) }

func TestServeEndToEnd(t *testing.T) {
	gin.SetMode(gin.TestMode)

	app := domain.App{ID: uuid.New(), Name: "myapp"}
	dep := domain.Deployment{ID: uuid.New(), AppID: app.ID, ContainerID: "abc123", Status: domain.DeploymentStatusRunning}

	payload := makeDockerStream(t, []struct{ stream, line string }{
		{"stdout", "boot ok"},
		{"stderr", "warn: cache miss"},
		{"stdout", "req 200"},
	})

	h := New(
		fakeAppLookup{app: app},
		fakeDockerLogs{payload: payload},
		func(_ context.Context, id uuid.UUID) (domain.Deployment, error) {
			if id != app.ID {
				return domain.Deployment{}, domain.ErrDeploymentNotFound
			}
			return dep, nil
		},
		slog.New(slog.NewTextHandler(io.Discard, nil)),
	)

	r := gin.New()
	r.GET("/apps/:name/logs", h.Serve)
	srv := httptest.NewServer(r)
	defer srv.Close()

	wsURL := "ws" + strings.TrimPrefix(srv.URL, "http") + "/apps/myapp/logs"
	conn, resp, err := websocket.DefaultDialer.Dial(wsURL, http.Header{})
	if err != nil {
		if resp != nil {
			t.Fatalf("dial: %v (status %s)", err, resp.Status)
		}
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close()

	var got []Frame
	_ = conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	for {
		_, msg, err := conn.ReadMessage()
		if err != nil {
			break
		}
		var f Frame
		if err := json.Unmarshal(msg, &f); err != nil {
			t.Fatalf("decode: %v (raw=%s)", err, msg)
		}
		got = append(got, f)
	}

	if len(got) != 3 {
		t.Fatalf("got %d frames, want 3: %+v", len(got), got)
	}
	want := []struct{ stream, line string }{
		{"stdout", "boot ok"},
		{"stderr", "warn: cache miss"},
		{"stdout", "req 200"},
	}
	for i, w := range want {
		if got[i].Stream != w.stream || got[i].Line != w.line {
			t.Errorf("frame %d = {%s, %q}, want {%s, %q}", i, got[i].Stream, got[i].Line, w.stream, w.line)
		}
	}
}

func TestServeUnknownApp(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := New(
		fakeAppLookup{app: domain.App{Name: "other"}},
		fakeDockerLogs{},
		func(context.Context, uuid.UUID) (domain.Deployment, error) { return domain.Deployment{}, nil },
		slog.New(slog.NewTextHandler(io.Discard, nil)),
	)
	r := gin.New()
	r.GET("/apps/:name/logs", h.Serve)
	srv := httptest.NewServer(r)
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/apps/ghost/logs")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", resp.StatusCode)
	}
}
