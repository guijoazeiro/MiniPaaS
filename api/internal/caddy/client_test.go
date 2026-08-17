package caddy

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
)

type fakeCaddy struct {
	mu                sync.Mutex
	requests          []recorded
	hasServer         bool
	unknownIDOnDelete bool
	unknownIDOnPatch  bool
	httpAppMissing    bool
}

type recorded struct {
	Method, Path string
	Body         string
}

func (f *fakeCaddy) handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		f.mu.Lock()
		f.requests = append(f.requests, recorded{r.Method, r.URL.Path, string(body)})
		f.mu.Unlock()

		switch {
		case r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/servers/srv0"):
			if f.hasServer {
				_, _ = w.Write([]byte(`{"listen":[":80"],"routes":[]}`))
			} else if f.httpAppMissing {
				w.WriteHeader(http.StatusBadRequest)
				_, _ = w.Write([]byte(`{"error":"invalid traversal path at: config/apps/http"}`))
			} else {
				_, _ = w.Write([]byte(`null`))
			}
		case r.Method == http.MethodDelete && strings.HasPrefix(r.URL.Path, "/id/"):
			if f.unknownIDOnDelete {
				w.WriteHeader(http.StatusInternalServerError)
				_, _ = w.Write([]byte(`{"error":"unknown object id 'minipaas-x'"}`))
				return
			}
			w.WriteHeader(http.StatusOK)
		case r.Method == http.MethodPut && strings.HasPrefix(r.URL.Path, "/id/"):
			w.WriteHeader(http.StatusOK)
		case r.Method == http.MethodPatch && strings.HasPrefix(r.URL.Path, "/id/"):
			if f.unknownIDOnPatch {
				w.WriteHeader(http.StatusNotFound)
				_, _ = w.Write([]byte(`{"error":"unknown object id 'minipaas-x'"}`))
				return
			}
			w.WriteHeader(http.StatusOK)
		default:
			w.WriteHeader(http.StatusOK)
		}
	})
}

func TestSwitchRouteUsesAtomicIDUpdate(t *testing.T) {
	f := &fakeCaddy{}
	c := newClient(t, f)

	url, err := c.SwitchRoute(context.Background(), "hello", 32772)
	if err != nil {
		t.Fatal(err)
	}
	if url != "https://hello.example.dev" {
		t.Fatalf("url = %q", url)
	}
	if len(f.requests) != 1 || f.requests[0].Method != http.MethodPatch || f.requests[0].Path != "/id/minipaas-hello" {
		t.Fatalf("requests = %+v", f.requests)
	}
	var route map[string]any
	if err := json.Unmarshal([]byte(f.requests[0].Body), &route); err != nil {
		t.Fatal(err)
	}
	handle := route["handle"].([]any)[0].(map[string]any)
	upstream := handle["upstreams"].([]any)[0].(map[string]any)
	if upstream["dial"] != "localhost:32772" {
		t.Fatalf("upstream = %v", upstream["dial"])
	}
}

func TestSwitchRouteCreatesWhenIDDoesNotExist(t *testing.T) {
	f := &fakeCaddy{unknownIDOnPatch: true}
	c := newClient(t, f)

	if _, err := c.SwitchRoute(context.Background(), "hello", 32772); err != nil {
		t.Fatal(err)
	}
	if len(f.requests) != 2 || f.requests[0].Method != http.MethodPatch || f.requests[1].Method != http.MethodPost {
		t.Fatalf("requests = %+v", f.requests)
	}
	if f.requests[1].Path != "/config/apps/http/servers/srv0/routes" {
		t.Fatalf("create request = %+v", f.requests[1])
	}
}

func TestSwitchCustomRouteUsesStableDomainID(t *testing.T) {
	f := &fakeCaddy{}
	c := newClient(t, f)
	if err := c.SwitchCustomRoute(context.Background(), "domain-123", "api.example.com", 4321); err != nil {
		t.Fatal(err)
	}
	if len(f.requests) != 1 || f.requests[0].Method != http.MethodPatch || f.requests[0].Path != "/id/minipaas-domain-domain-123" {
		t.Fatalf("requests = %+v", f.requests)
	}
	if err := c.RemoveCustomRoute(context.Background(), "domain-123"); err != nil {
		t.Fatal(err)
	}
	if len(f.requests) != 2 || f.requests[1].Method != http.MethodDelete {
		t.Fatalf("remove requests = %+v", f.requests)
	}
}

func newClient(t *testing.T, f *fakeCaddy) *Client {
	t.Helper()
	srv := httptest.NewServer(f.handler())
	t.Cleanup(srv.Close)
	return New(srv.URL, "example.dev")
}

func TestEnsureBaseBootstrapsWhenMissing(t *testing.T) {
	f := &fakeCaddy{hasServer: false, httpAppMissing: true}
	c := newClient(t, f)

	if err := c.EnsureBase(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(f.requests) != 2 {
		t.Fatalf("got %d requests, want 2 (probe + put)", len(f.requests))
	}
	put := f.requests[1]
	if put.Method != http.MethodPut || put.Path != "/config/apps/http" {
		t.Fatalf("wrong bootstrap request: %+v", put)
	}
}

func TestEnsureBaseAddsServerToExistingHTTPApp(t *testing.T) {
	f := &fakeCaddy{hasServer: false}
	c := newClient(t, f)

	if err := c.EnsureBase(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(f.requests) != 2 {
		t.Fatalf("got %d requests, want 2 (probe + put)", len(f.requests))
	}
	put := f.requests[1]
	if put.Method != http.MethodPut || put.Path != "/config/apps/http/servers/srv0" {
		t.Fatalf("wrong server creation request: %+v", put)
	}
}

func TestEnsureBaseSkipsWhenPresent(t *testing.T) {
	f := &fakeCaddy{hasServer: true}
	c := newClient(t, f)

	if err := c.EnsureBase(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(f.requests) != 1 {
		t.Fatalf("got %d requests, want 1 (probe only)", len(f.requests))
	}
}

func TestUpsertRouteContract(t *testing.T) {
	f := &fakeCaddy{}
	c := newClient(t, f)

	url, err := c.UpsertRoute(context.Background(), "hello", 32771)
	if err != nil {
		t.Fatal(err)
	}
	if url != "https://hello.example.dev" {
		t.Errorf("url = %q, want https://hello.example.dev", url)
	}

	if len(f.requests) != 2 {
		t.Fatalf("got %d requests, want 2", len(f.requests))
	}
	if f.requests[0].Method != http.MethodDelete || f.requests[0].Path != "/id/minipaas-hello" {
		t.Errorf("first request = %+v", f.requests[0])
	}
	post := f.requests[1]
	if post.Method != http.MethodPost || post.Path != "/config/apps/http/servers/srv0/routes" {
		t.Fatalf("second request = %+v", post)
	}

	var route map[string]any
	if err := json.Unmarshal([]byte(post.Body), &route); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if route["@id"] != "minipaas-hello" {
		t.Errorf("@id = %v", route["@id"])
	}
	matches := route["match"].([]any)
	host := matches[0].(map[string]any)["host"].([]any)[0]
	if host != "hello.example.dev" {
		t.Errorf("host = %v", host)
	}
	handle := route["handle"].([]any)[0].(map[string]any)
	if handle["handler"] != "reverse_proxy" {
		t.Errorf("handler = %v", handle["handler"])
	}
	upstream := handle["upstreams"].([]any)[0].(map[string]any)
	if upstream["dial"] != "localhost:32771" {
		t.Errorf("dial = %v", upstream["dial"])
	}
}

func TestRemoveRouteTolerantOfMissingID(t *testing.T) {
	f := &fakeCaddy{unknownIDOnDelete: true}
	c := newClient(t, f)

	if err := c.RemoveRoute(context.Background(), "gone"); err != nil {
		t.Fatalf("unknown-id delete should be a no-op, got err: %v", err)
	}
}
