package caddy

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

const serverName = "srv0"

type Client struct {
	base       string
	baseDomain string
	http       *http.Client
}

func New(adminURL, baseDomain string) *Client {
	return &Client{
		base:       adminURL,
		baseDomain: baseDomain,
		http:       &http.Client{Timeout: 10 * time.Second},
	}
}

func (c *Client) EnsureBase(ctx context.Context) error {
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet,
		c.base+"/config/apps/http/servers/"+serverName, nil)
	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("caddy.EnsureBase: probe: %w", err)
	}
	body, _ := io.ReadAll(resp.Body)
	_ = resp.Body.Close()

	if resp.StatusCode == http.StatusOK && !isNull(body) {
		return nil
	}

	cfg := map[string]any{
		serverName: map[string]any{
			"listen":          []string{":80", ":443"},
			"routes":          []any{},
			"automatic_https": map[string]any{"disable_redirects": false},
		},
	}
	return c.put(ctx, "/config/apps/http/servers", cfg, "EnsureBase")
}

func (c *Client) UpsertRoute(ctx context.Context, appName string, port int) (string, error) {
	id := routeID(appName)
	host := appName + "." + c.baseDomain

	if err := c.deleteByID(ctx, id); err != nil {
		return "", fmt.Errorf("caddy.UpsertRoute: delete existing: %w", err)
	}

	route := map[string]any{
		"@id":   id,
		"match": []map[string]any{{"host": []string{host}}},
		"handle": []map[string]any{{
			"handler":   "reverse_proxy",
			"upstreams": []map[string]any{{"dial": fmt.Sprintf("localhost:%d", port)}},
		}},
		"terminal": true,
	}

	if err := c.post(ctx, "/config/apps/http/servers/"+serverName+"/routes", route, "UpsertRoute"); err != nil {
		return "", err
	}
	return "https://" + host, nil
}

func (c *Client) RemoveRoute(ctx context.Context, appName string) error {
	return c.deleteByID(ctx, routeID(appName))
}

func routeID(appName string) string { return "minipaas-" + appName }

func (c *Client) deleteByID(ctx context.Context, id string) error {
	req, _ := http.NewRequestWithContext(ctx, http.MethodDelete, c.base+"/id/"+id, nil)
	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("delete /id/%s: %w", id, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusOK {
		return nil
	}
	body, _ := io.ReadAll(resp.Body)

	if bytes.Contains(bytes.ToLower(body), []byte("unknown object id")) {
		return nil
	}
	return fmt.Errorf("delete /id/%s: %d %s", id, resp.StatusCode, bytes.TrimSpace(body))
}

func (c *Client) put(ctx context.Context, path string, body any, op string) error {
	return c.do(ctx, http.MethodPut, path, body, op)
}

func (c *Client) post(ctx context.Context, path string, body any, op string) error {
	return c.do(ctx, http.MethodPost, path, body, op)
}

func (c *Client) do(ctx context.Context, method, path string, body any, op string) error {
	buf, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("caddy.%s: marshal: %w", op, err)
	}
	req, _ := http.NewRequestWithContext(ctx, method, c.base+path, bytes.NewReader(buf))
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("caddy.%s: %w", op, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		errBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("caddy.%s: %d %s", op, resp.StatusCode, bytes.TrimSpace(errBody))
	}
	return nil
}

func isNull(b []byte) bool {
	return bytes.Equal(bytes.TrimSpace(b), []byte("null"))
}
