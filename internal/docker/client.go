package docker

import (
	"bytes"
	"context"
	"fmt"
	"strings"
	"time"

	"dockerman/internal/model"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/strslice"
	dockerclient "github.com/docker/docker/client"
)

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
	return err
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
			info.Ports = append(info.Ports, fmt.Sprintf("%d/%s", p.PublicPort, p.Type))
		}

		info.SetScannedNow()
		info.CreatedAt = time.Unix(ctr.Created, 0).Format(time.RFC3339)

		result = append(result, info)
	}
	return result, nil
}

func (c *Client) Start(ctx context.Context, id string) error {
	return c.cli.ContainerStart(ctx, id, container.StartOptions{})
}

func (c *Client) Stop(ctx context.Context, id string) error {
	timeout := 10
	return c.cli.ContainerStop(ctx, id, container.StopOptions{Timeout: &timeout})
}

func (c *Client) Remove(ctx context.Context, id string, force bool) error {
	return c.cli.ContainerRemove(ctx, id, container.RemoveOptions{Force: force})
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

	var buf bytes.Buffer
	_, err = buf.ReadFrom(attachResp.Reader)
	if err != nil {
		return "", fmt.Errorf("read exec output: %w", err)
	}
	return buf.String(), nil
}

func (c *Client) Inspect(ctx context.Context, id string) (types.ContainerJSON, error) {
	info, err := c.cli.ContainerInspect(ctx, id)
	if err != nil {
		return types.ContainerJSON{}, fmt.Errorf("inspect container: %w", err)
	}
	return info, nil
}

func (c *Client) FindContainerByIP(ctx context.Context, ip string) (string, error) {
	containers, err := c.cli.ContainerList(ctx, container.ListOptions{All: false})
	if err != nil {
		return "", fmt.Errorf("list containers: %w", err)
	}
	for _, ctr := range containers {
		info, err := c.cli.ContainerInspect(ctx, ctr.ID)
		if err != nil {
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
