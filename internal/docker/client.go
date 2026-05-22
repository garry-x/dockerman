package docker

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"strings"
	"time"

	"dockerman/internal/model"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/strslice"
	dockerclient "github.com/docker/docker/client"
	"github.com/docker/docker/pkg/stdcopy"
)

const maxExecOutput = 1 << 20 // 1 MB

type Client struct {
	cli *dockerclient.Client
}

func NewClient() (*Client, error) {
	cli, err := dockerclient.NewClientWithOpts(dockerclient.FromEnv, dockerclient.WithAPIVersionNegotiation())
	if err != nil {
		return nil, fmt.Errorf("create docker client: %w", err)
	}
	return &Client{cli: cli}, nil
}

func (c *Client) Ping(ctx context.Context) error {
	_, err := c.cli.Ping(ctx)
	if err != nil {
		return fmt.Errorf("docker ping: %w", err)
	}
	return nil
}

func (c *Client) ScanAll(ctx context.Context) ([]model.ContainerInfo, error) {
	containers, err := c.cli.ContainerList(ctx, container.ListOptions{All: true})
	if err != nil {
		return nil, fmt.Errorf("list containers: %w", err)
	}

	result := make([]model.ContainerInfo, 0, len(containers))
	for _, ctr := range containers {
		info := model.ContainerInfo{
			ID:     ctr.ID[:12],
			Image:  ctr.Image,
			Status: ctr.State,
			Ports:  make([]string, 0),
			Labels: ctr.Labels,
		}

		if len(ctr.Names) > 0 {
			info.Name = strings.TrimPrefix(ctr.Names[0], "/")
		}

		for _, p := range ctr.Ports {
			if p.PublicPort == 0 {
				continue
			}
			info.Ports = append(info.Ports, fmt.Sprintf("%d/%s", p.PublicPort, p.Type))
		}

		info.SetScannedNow()
		info.CreatedAt = time.Unix(ctr.Created, 0).Format(time.RFC3339)

		result = append(result, info)
	}
	return result, nil
}

func (c *Client) Start(ctx context.Context, id string) error {
	if err := c.cli.ContainerStart(ctx, id, container.StartOptions{}); err != nil {
		return fmt.Errorf("start container %s: %w", id, err)
	}
	return nil
}

func (c *Client) Stop(ctx context.Context, id string) error {
	timeout := 10
	if err := c.cli.ContainerStop(ctx, id, container.StopOptions{Timeout: &timeout}); err != nil {
		return fmt.Errorf("stop container %s: %w", id, err)
	}
	return nil
}

func (c *Client) Remove(ctx context.Context, id string, force bool) error {
	if err := c.cli.ContainerRemove(ctx, id, container.RemoveOptions{Force: force}); err != nil {
		return fmt.Errorf("remove container %s: %w", id, err)
	}
	return nil
}

func (c *Client) Exec(ctx context.Context, id string, cmd []string) (string, error) {
	execConfig := types.ExecConfig{
		Cmd:          strslice.StrSlice(cmd),
		AttachStdout: true,
		AttachStderr: true,
	}
	execResp, err := c.cli.ContainerExecCreate(ctx, id, execConfig)
	if err != nil {
		return "", fmt.Errorf("exec create: %w", err)
	}

	attachResp, err := c.cli.ContainerExecAttach(ctx, execResp.ID, types.ExecStartCheck{})
	if err != nil {
		return "", fmt.Errorf("exec attach: %w", err)
	}
	defer attachResp.Close()

	var stdout, stderr bytes.Buffer
	_, err = stdcopy.StdCopy(&stdout, &stderr, io.LimitReader(attachResp.Reader, maxExecOutput))
	if err != nil {
		return "", fmt.Errorf("read exec output: %w", err)
	}

	if stderr.Len() > 0 {
		return stdout.String() + "\n" + stderr.String(), nil
	}
	return stdout.String(), nil
}

func (c *Client) Inspect(ctx context.Context, id string) (types.ContainerJSON, error) {
	info, err := c.cli.ContainerInspect(ctx, id)
	if err != nil {
		return types.ContainerJSON{}, fmt.Errorf("inspect container %s: %w", id, err)
	}
	return info, nil
}

func (c *Client) FindContainerByIP(ctx context.Context, ip string) (string, error) {
	containers, err := c.cli.ContainerList(ctx, container.ListOptions{All: false})
	if err != nil {
		return "", fmt.Errorf("list containers for IP lookup: %w", err)
	}
	for _, ctr := range containers {
		info, err := c.cli.ContainerInspect(ctx, ctr.ID)
		if err != nil {
			continue
		}
		if info.NetworkSettings == nil {
			continue
		}
		for _, net := range info.NetworkSettings.Networks {
			if net.IPAddress == ip {
				return ctr.ID[:12], nil
			}
		}
	}
	return "", nil
}

func (c *Client) Close() error {
	return c.cli.Close()
}
