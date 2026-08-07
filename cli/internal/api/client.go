package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"time"
)

type Client struct {
	base string
	http *http.Client
}

func New(base string) *Client {
	return &Client{
		base: base,
		http: &http.Client{Timeout: 5 * time.Minute},
	}
}

type App struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Status    string `json:"status"`
	PublicURL string `json:"public_url,omitempty"`
}

type Deployment struct {
	ID         string `json:"id"`
	AppID      string `json:"app_id"`
	ImageTag   string `json:"image_tag"`
	Status     string `json:"status"`
	Port       int    `json:"port,omitempty"`
	CreatedAt  string `json:"created_at,omitempty"`
	DurationMs int    `json:"duration_ms,omitempty"`
}

func (c *Client) ListDeployments(app string) ([]Deployment, error) {
	var out []Deployment
	if err := c.doJSON(http.MethodGet, "/apps/"+app+"/deployments?limit=5", nil, "", &out); err != nil {
		return nil, err
	}
	return out, nil
}

func (c *Client) CreateApp(name string) (*App, error) {
	body, _ := json.Marshal(map[string]string{"name": name})
	var out App
	if err := c.doJSON(http.MethodPost, "/apps", bytes.NewReader(body), "application/json", &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) ListApps() ([]App, error) {
	var out []App
	if err := c.doJSON(http.MethodGet, "/apps", nil, "", &out); err != nil {
		return nil, err
	}
	return out, nil
}

func (c *Client) GetApp(name string) (*App, error) {
	var out App
	if err := c.doJSON(http.MethodGet, "/apps/"+name, nil, "", &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) Deploy(app string, tar io.Reader) (*Deployment, error) {
	pr, pw := io.Pipe()
	mw := multipart.NewWriter(pw)

	go func() {
		defer pw.Close()
		defer mw.Close()
		part, err := mw.CreateFormFile("source", "context.tar")
		if err != nil {
			_ = pw.CloseWithError(err)
			return
		}
		if _, err := io.Copy(part, tar); err != nil {
			_ = pw.CloseWithError(err)
			return
		}
	}()

	req, err := http.NewRequest(http.MethodPost, c.base+"/apps/"+app+"/deployments", pr)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", mw.FormDataContentType())

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return nil, httpErr(resp)
	}
	var out Deployment
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, fmt.Errorf("decode: %w", err)
	}
	return &out, nil
}

func (c *Client) GetDeployment(app, id string) (*Deployment, error) {
	var out Deployment
	if err := c.doJSON(http.MethodGet, "/apps/"+app+"/deployments/"+id, nil, "", &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) doJSON(method, path string, body io.Reader, contentType string, out any) error {
	req, err := http.NewRequest(method, c.base+path, body)
	if err != nil {
		return err
	}
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return httpErr(resp)
	}
	if out == nil || resp.ContentLength == 0 {
		return nil
	}
	return json.NewDecoder(resp.Body).Decode(out)
}

func httpErr(resp *http.Response) error {
	b, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<14))
	return fmt.Errorf("http %d: %s", resp.StatusCode, bytes.TrimSpace(b))
}
