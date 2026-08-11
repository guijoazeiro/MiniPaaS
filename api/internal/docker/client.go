package docker

import (
	"context"
	"fmt"
	"io"

	"github.com/containerd/errdefs"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/client"
	"github.com/docker/go-connections/nat"
)

type RunOptions struct {
	Image             string
	Name              string
	Env               map[string]string
	ContainerPort     string
	RestartPolicy     string
	RestartMaxRetries int
}

type ContainerInfo struct {
	ID   string
	Port int
}

type ContainerState struct {
	Status  string
	Running bool
}

type Client struct {
	cli *client.Client
}

func New(host string) (*Client, error) {
	opts := []client.Opt{client.FromEnv, client.WithAPIVersionNegotiation()}
	if host != "" {
		opts = append(opts, client.WithHost(host))
	}
	cli, err := client.NewClientWithOpts(opts...)
	if err != nil {
		return nil, fmt.Errorf("docker.New: %w", err)
	}
	return &Client{cli: cli}, nil
}

func (c *Client) Close() error { return c.cli.Close() }

func (c *Client) BuildImage(ctx context.Context, tar io.Reader, tag string) (io.ReadCloser, error) {
	return c.BuildImageWithDockerfile(ctx, tar, tag, "Dockerfile")
}

func (c *Client) BuildImageWithDockerfile(ctx context.Context, tar io.Reader, tag, dockerfile string) (io.ReadCloser, error) {
	resp, err := c.cli.ImageBuild(ctx, tar, ImageBuildOptions(tag, dockerfile))
	if err != nil {
		return nil, fmt.Errorf("docker.BuildImage: %w", err)
	}
	return resp.Body, nil
}

func (c *Client) RunContainer(ctx context.Context, opts RunOptions) (ContainerInfo, error) {
	containerPort := opts.ContainerPort
	if containerPort == "" {
		containerPort = "8080/tcp"
	}
	proto, portNum := splitPortProto(containerPort)
	natPort, err := nat.NewPort(proto, portNum)
	if err != nil {
		return ContainerInfo{}, fmt.Errorf("docker.RunContainer: parse port %q: %w", containerPort, err)
	}

	restart := opts.RestartPolicy
	if restart == "" {
		restart = "unless-stopped"
	}

	cfg := &container.Config{
		Image:        opts.Image,
		Env:          envSlice(opts.Env),
		ExposedPorts: nat.PortSet{natPort: struct{}{}},
	}
	hostCfg := &container.HostConfig{
		PortBindings: nat.PortMap{
			natPort: []nat.PortBinding{{HostIP: "0.0.0.0", HostPort: ""}},
		},
		RestartPolicy: container.RestartPolicy{Name: container.RestartPolicyMode(restart)},
	}
	hostCfg.RestartPolicy.MaximumRetryCount = opts.RestartMaxRetries

	created, err := c.cli.ContainerCreate(ctx, cfg, hostCfg, &network.NetworkingConfig{}, nil, opts.Name)
	if err != nil {
		return ContainerInfo{}, fmt.Errorf("docker.RunContainer: create: %w", err)
	}

	if err := c.cli.ContainerStart(ctx, created.ID, container.StartOptions{}); err != nil {
		_ = c.cli.ContainerRemove(ctx, created.ID, container.RemoveOptions{Force: true})
		return ContainerInfo{}, fmt.Errorf("docker.RunContainer: start: %w", err)
	}

	inspect, err := c.cli.ContainerInspect(ctx, created.ID)
	if err != nil {
		return ContainerInfo{}, fmt.Errorf("docker.RunContainer: inspect: %w", err)
	}

	port, err := hostPort(inspect.NetworkSettings.Ports, natPort)
	if err != nil {
		return ContainerInfo{}, fmt.Errorf("docker.RunContainer: %w", err)
	}

	return ContainerInfo{ID: created.ID, Port: port}, nil
}

func (c *Client) InspectContainer(ctx context.Context, id string) (ContainerState, error) {
	inspect, err := c.cli.ContainerInspect(ctx, id)
	if err != nil {
		if errdefs.IsNotFound(err) {
			return ContainerState{Status: "missing"}, nil
		}
		return ContainerState{}, fmt.Errorf("docker.InspectContainer: %w", err)
	}
	if inspect.State == nil {
		return ContainerState{Status: "unknown"}, nil
	}
	return ContainerState{Status: inspect.State.Status, Running: inspect.State.Running}, nil
}

func (c *Client) StopContainer(ctx context.Context, id string) error {
	if err := c.cli.ContainerStop(ctx, id, container.StopOptions{}); err != nil {
		return fmt.Errorf("docker.StopContainer: %w", err)
	}
	return nil
}

func (c *Client) RemoveContainer(ctx context.Context, id string) error {
	err := c.cli.ContainerRemove(ctx, id, container.RemoveOptions{Force: true, RemoveVolumes: true})
	if err != nil {
		return fmt.Errorf("docker.RemoveContainer: %w", err)
	}
	return nil
}

type LogOptions struct {
	Follow bool
	Tail   string
}

func (c *Client) StreamLogs(ctx context.Context, id string, opts LogOptions) (io.ReadCloser, error) {
	tail := opts.Tail
	if tail == "" {
		tail = "100"
	}
	rc, err := c.cli.ContainerLogs(ctx, id, container.LogsOptions{
		ShowStdout: true, ShowStderr: true,
		Follow: opts.Follow,
		Tail:   tail,
	})
	if err != nil {
		return nil, fmt.Errorf("docker.StreamLogs: %w", err)
	}
	return rc, nil
}

func (c *Client) RemoveImage(ctx context.Context, ref string) error {
	_, err := c.cli.ImageRemove(ctx, ref, image.RemoveOptions{Force: true, PruneChildren: true})
	if err != nil {
		return fmt.Errorf("docker.RemoveImage: %w", err)
	}
	return nil
}
