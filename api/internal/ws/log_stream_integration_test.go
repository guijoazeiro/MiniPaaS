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

type fakeAppLookup struct{ app domain.App }

func (f fakeAppLookup) GetByName(_ context.Context, name string) (domain.App, error) {
	if name != f.app.Name {
		return domain.App{}, domain.ErrAppNotFound
	}
	return f.app, nil
}

type fakeDockerLogs struct{ payload []byte }

func (f fakeDockerLogs) StreamLogs(_ context.Context, _ string, _ docker.LogOptions) (io.ReadCloser, error) {
	pr, pw := io.Pipe()
	go func() {
		_, _ = pw.Write(f.payload)
		_ = pw.Close()
	}()
	return pr, nil
}

type fakeDeploymentLookup struct {
	active      domain.Deployment
	activeErr   error
	deployments []domain.Deployment
}

func (f fakeDeploymentLookup) GetActive(_ context.Context, _ uuid.UUID) (domain.Deployment, error) {
	return f.active, f.activeErr
}

func (f fakeDeploymentLookup) ListByApp(_ context.Context, _ uuid.UUID, _ int) ([]domain.Deployment, error) {
	return f.deployments, nil
}

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
		fakeDeploymentLookup{active: dep},
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
	var closeErr error
	_ = conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	for {
		_, msg, err := conn.ReadMessage()
		if err != nil {
			closeErr = err
			break
		}
		var f Frame
		if err := json.Unmarshal(msg, &f); err != nil {
			t.Fatalf("decode: %v (raw=%s)", err, msg)
		}
		got = append(got, f)
	}
	if !websocket.IsCloseError(closeErr, websocket.CloseNormalClosure) {
		t.Fatalf("close error = %v, want normal closure", closeErr)
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
		fakeDeploymentLookup{},
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
