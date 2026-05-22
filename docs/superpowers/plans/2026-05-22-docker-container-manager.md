# Docker Container Manager Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build a Go CLI + HTTP tool that manages local Docker containers — scan, start, stop, remove — with container-origin-aware HTTP permissions and systemd integration.

**Architecture:** Single binary with layered internal packages: model → store → docker client → CLI handlers + HTTP server. CLI and HTTP share the same docker+store layer. HTTP middleware enforces 3-tier permissions (localhost/container/remote).

**Tech Stack:** Go 1.22+, Docker SDK for Go, cobra (CLI), gorilla/mux (HTTP), systemd

---

### Task 1: Initialize Go module and project skeleton

**Files:**
- Create: `go.mod`
- Create: `cmd/dockerman/main.go`
- Create: `internal/model/container.go`
- Create: `internal/docker/client.go`
- Create: `internal/store/json.go`
- Create: `internal/server/server.go`
- Create: `internal/cli/commands.go`
- Create: `dockerman` (bash wrapper)

- [ ] **Step 1: Initialize Go module**

Run: `cd /data/code/container-master && go mod init dockerman`
Expected: Creates go.mod

- [ ] **Step 2: Create model package**

Write `internal/model/container.go`:

```go
package model

import "time"

type ContainerInfo struct {
	ID        string            `json:"id"`
	Name      string            `json:"name"`
	Image     string            `json:"image"`
	Status    string            `json:"status"`
	Ports     []string          `json:"ports"`
	Labels    map[string]string `json:"labels"`
	CreatedAt string            `json:"created_at"`
	ScannedAt string            `json:"scanned_at"`
}

func NewContainerInfo() *ContainerInfo {
	return &ContainerInfo{
		Ports:  []string{},
		Labels: make(map[string]string),
	}
}

func (c *ContainerInfo) SetScannedNow() {
	c.ScannedAt = time.Now().Format(time.RFC3339)
}
```

- [ ] **Step 3: Create stub files for all packages**

Write `internal/docker/client.go`:

```go
package docker

type Client struct {
	// Will hold Docker SDK client
}
```

Write `internal/store/json.go`:

```go
package store

type JSONStore struct {
	Path string
}
```

Write `internal/server/server.go`:

```go
package server

type Server struct {
	// Will hold HTTP server and dependencies
}
```

Write `internal/cli/commands.go`:

```go
package cli

import "github.com/spf13/cobra"

func NewRootCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "dockerman",
		Short: "Docker Container Manager",
	}
}
```

- [ ] **Step 4: Create main.go entrypoint**

Write `cmd/dockerman/main.go`:

```go
package main

import (
	"fmt"
	"os"

	"dockerman/internal/cli"
)

func main() {
	if err := cli.NewRootCmd().Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
```

- [ ] **Step 5: Create bash wrapper**

Write `dockerman`:

```bash
#!/bin/bash
set -e
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
BINARY="$SCRIPT_DIR/bin/dockerman"
if [ ! -f "$BINARY" ]; then
	echo "Building dockerman..."
	cd "$SCRIPT_DIR" && go build -o "$BINARY" ./cmd/dockerman
fi
exec "$BINARY" "$@"
```

Run: `chmod +x /data/code/container-master/dockerman`

- [ ] **Step 6: Install dependencies and verify build**

Run: `cd /data/code/container-master && go get github.com/spf13/cobra@latest github.com/gorilla/mux@latest github.com/docker/docker/client@latest`
Run: `go mod tidy`
Run: `go build ./cmd/dockerman`
Expected: Build succeeds with no errors

- [ ] **Step 7: Commit**

```bash
git add -A
git commit -m "feat: initialize Go module and project skeleton"
```

---

### Task 2: Implement JSON store

**Files:**
- Create: `internal/store/json.go`
- Create: `internal/store/json_test.go`

- [ ] **Step 1: Implement JSONStore**

Write `internal/store/json.go`:

```go
package store

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"dockerman/internal/model"
)

type JSONStore struct {
	Path string
}

func NewJSONStore(path string) *JSONStore {
	return &JSONStore{Path: path}
}

func (s *JSONStore) Load() ([]model.ContainerInfo, error) {
	data, err := os.ReadFile(s.Path)
	if err != nil {
		if os.IsNotExist(err) {
			return []model.ContainerInfo{}, nil
		}
		return nil, fmt.Errorf("read store file: %w", err)
	}
	if len(data) == 0 {
		return []model.ContainerInfo{}, nil
	}
	var containers []model.ContainerInfo
	if err := json.Unmarshal(data, &containers); err != nil {
		return nil, fmt.Errorf("parse store file: %w", err)
	}
	return containers, nil
}

func (s *JSONStore) Save(containers []model.ContainerInfo) error {
	dir := filepath.Dir(s.Path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("create store directory: %w", err)
	}
	data, err := json.MarshalIndent(containers, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal containers: %w", err)
	}
	if err := os.WriteFile(s.Path, data, 0644); err != nil {
		return fmt.Errorf("write store file: %w", err)
	}
	return nil
}

func (s *JSONStore) GetByID(id string) (*model.ContainerInfo, error) {
	containers, err := s.Load()
	if err != nil {
		return nil, err
	}
	for i, c := range containers {
		if c.ID == id || c.Name == id {
			return &containers[i], nil
		}
	}
	return nil, fmt.Errorf("container %q not found in store", id)
}
```

- [ ] **Step 2: Write tests**

Write `internal/store/json_test.go`:

```go
package store

import (
	"os"
	"path/filepath"
	"testing"

	"dockerman/internal/model"
)

func TestLoad_NonexistentFile_ReturnsEmpty(t *testing.T) {
	s := NewJSONStore(filepath.Join(t.TempDir(), "nonexistent.json"))
	containers, err := s.Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(containers) != 0 {
		t.Fatalf("expected empty list, got %d items", len(containers))
	}
}

func TestSaveAndLoad_RoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "containers.json")
	s := NewJSONStore(path)

	input := []model.ContainerInfo{
		{ID: "abc123", Name: "web", Image: "nginx", Status: "running"},
		{ID: "def456", Name: "db", Image: "postgres", Status: "exited"},
	}

	if err := s.Save(input); err != nil {
		t.Fatalf("save failed: %v", err)
	}

	loaded, err := s.Load()
	if err != nil {
		t.Fatalf("load failed: %v", err)
	}
	if len(loaded) != 2 {
		t.Fatalf("expected 2 containers, got %d", len(loaded))
	}
	if loaded[0].ID != "abc123" || loaded[1].ID != "def456" {
		t.Fatalf("data mismatch: %+v", loaded)
	}
}

func TestGetByID_Found(t *testing.T) {
	path := filepath.Join(t.TempDir(), "containers.json")
	s := NewJSONStore(path)
	s.Save([]model.ContainerInfo{
		{ID: "abc123", Name: "web", Image: "nginx"},
	})

	c, err := s.GetByID("abc123")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if c.Name != "web" {
		t.Fatalf("expected web, got %s", c.Name)
	}
}

func TestGetByID_ByName(t *testing.T) {
	path := filepath.Join(t.TempDir(), "containers.json")
	s := NewJSONStore(path)
	s.Save([]model.ContainerInfo{
		{ID: "abc123", Name: "web", Image: "nginx"},
	})

	c, err := s.GetByID("web")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if c.ID != "abc123" {
		t.Fatalf("expected abc123, got %s", c.ID)
	}
}

func TestGetByID_NotFound(t *testing.T) {
	s := NewJSONStore(filepath.Join(t.TempDir(), "nonexistent.json"))
	_, err := s.GetByID("missing")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestSave_CreatesDirectory(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "subdir", "data")
	path := filepath.Join(dir, "containers.json")
	s := NewJSONStore(path)

	err := s.Save([]model.ContainerInfo{{ID: "test"}})
	if err != nil {
		t.Fatalf("save failed: %v", err)
	}
	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Fatal("file was not created")
	}
}
```

- [ ] **Step 3: Run tests**

Run: `cd /data/code/container-master && go test ./internal/store/... -v`
Expected: All tests PASS

- [ ] **Step 4: Commit**

```bash
git add internal/store/
git commit -m "feat: implement JSON store with load/save/get-by-id"
```

---

### Task 3: Implement Docker SDK client

**Files:**
- Create: `internal/docker/client.go`
- Create: `internal/docker/client_test.go`

- [ ] **Step 1: Implement Docker client wrapper**

Write `internal/docker/client.go`:

```go
package docker

import (
	"bytes"
	"context"
	"fmt"
	"strings"

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
	execConfig := container.ExecOptions{
		Cmd:          strslice.StrSlice(cmd),
		AttachStdout: true,
		AttachStderr: true,
	}
	execResp, err := c.cli.ContainerExecCreate(ctx, id, execConfig)
	if err != nil {
		return "", fmt.Errorf("exec create: %w", err)
	}

	attachResp, err := c.cli.ContainerExecAttach(ctx, execResp.ID, container.ExecAttachOptions{})
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
```

Make sure `import "time"` is added after `"strings"`.

- [ ] **Step 2: Add time import**

The `ScanAll` function uses `time.Unix` — verify `"time"` is in the import block.

Run: `cd /data/code/container-master && go build ./internal/docker`
Expected: Build succeeds

- [ ] **Step 3: Write mock-based tests**

Write `internal/docker/client_test.go`:

```go
package docker

import (
	"testing"
)

func TestNewClient_FromEnv(t *testing.T) {
	// Integration test - requires Docker daemon running
	// Skip in CI environments without Docker
	t.Skip("requires Docker daemon")
}
```

- [ ] **Step 4: Commit**

```bash
git add internal/docker/
git commit -m "feat: implement Docker SDK client wrapper"
```

---

### Task 4: Implement CLI commands (cobra)

**Files:**
- Create: `internal/cli/commands.go` (full implementation)

- [ ] **Step 1: Implement all cobra commands**

Write `internal/cli/commands.go`:

```go
package cli

import (
	"context"
	"fmt"
	"os"
	"strings"
	"text/tabwriter"

	"dockerman/internal/docker"
	"dockerman/internal/store"

	"github.com/spf13/cobra"
)

var dbPath string

func NewRootCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "dockerman",
		Short: "Docker Container Manager",
		Long:  "CLI tool for managing local Docker containers with persistent container list.",
	}
	cmd.PersistentFlags().StringVar(&dbPath, "db", defaultDBPath(), "JSON database file path")
	cmd.AddCommand(
		scanCmd(),
		listCmd(),
		startCmd(),
		stopCmd(),
		rmCmd(),
		execCmd(),
		inspectCmd(),
		serveCmd(),
		installSystemdCmd(),
		uninstallSystemdCmd(),
	)
	return cmd
}

func defaultDBPath() string {
	if p := os.Getenv("CONTAINER_DB"); p != "" {
		return p
	}
	return "/var/lib/dockerman/containers.json"
}

func getDockerClient() (*docker.Client, error) {
	return docker.NewClient()
}

func resolveIDs(dockerCli *docker.Client, jsonStore *store.JSONStore, arg string) ([]string, error) {
	if strings.ToLower(arg) == "all" {
		containers, err := jsonStore.Load()
		if err != nil {
			return nil, err
		}
		ids := make([]string, 0, len(containers))
		for _, c := range containers {
			ids = append(ids, c.ID)
		}
		return ids, nil
	}

	// Try exact match first
	_, err := dockerCli.Inspect(context.Background(), arg)
	if err == nil {
		return []string{arg}, nil
	}

	// Try store match
	ctr, err := jsonStore.GetByID(arg)
	if err == nil {
		_, err := dockerCli.Inspect(context.Background(), ctr.ID)
		if err == nil {
			return []string{ctr.ID}, nil
		}
	}

	return nil, fmt.Errorf("container %q not found", arg)
}

func scanCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "scan",
		Short: "Scan all local Docker containers and persist to JSON file",
		RunE: func(cmd *cobra.Command, args []string) error {
			dockerCli, err := getDockerClient()
			if err != nil {
				return err
			}
			defer dockerCli.Close()

			containers, err := dockerCli.ScanAll(cmd.Context())
			if err != nil {
				return err
			}

			s := store.NewJSONStore(dbPath)
			if err := s.Save(containers); err != nil {
				return err
			}

			fmt.Printf("Scanned %d containers, saved to %s\n", len(containers), dbPath)
			return nil
		},
	}
}

func listCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List containers from JSON store",
		RunE: func(cmd *cobra.Command, args []string) error {
			s := store.NewJSONStore(dbPath)
			containers, err := s.Load()
			if err != nil {
				return err
			}
			if len(containers) == 0 {
				fmt.Println("No containers found. Run 'dockerman scan' first.")
				return nil
			}
			w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
			fmt.Fprintln(w, "ID\tNAME\tIMAGE\tSTATUS\tPORTS")
			for _, c := range containers {
				fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n",
					c.ID, c.Name, c.Image, c.Status, strings.Join(c.Ports, ", "))
			}
			w.Flush()
			return nil
		},
	}
}

func startCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "start <id|all>",
		Short: "Start container(s)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			dockerCli, err := getDockerClient()
			if err != nil {
				return err
			}
			defer dockerCli.Close()

			s := store.NewJSONStore(dbPath)
			ids, err := resolveIDs(dockerCli, s, args[0])
			if err != nil {
				return err
			}

			for _, id := range ids {
				if err := dockerCli.Start(cmd.Context(), id); err != nil {
					fmt.Fprintf(os.Stderr, "Failed to start %s: %v\n", id, err)
					continue
				}
				fmt.Printf("Started %s\n", id)
			}
			return nil
		},
	}
}

func stopCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "stop <id|all>",
		Short: "Stop container(s)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			dockerCli, err := getDockerClient()
			if err != nil {
				return err
			}
			defer dockerCli.Close()

			s := store.NewJSONStore(dbPath)
			ids, err := resolveIDs(dockerCli, s, args[0])
			if err != nil {
				return err
			}

			for _, id := range ids {
				if err := dockerCli.Stop(cmd.Context(), id); err != nil {
					fmt.Fprintf(os.Stderr, "Failed to stop %s: %v\n", id, err)
					continue
				}
				fmt.Printf("Stopped %s\n", id)
			}
			return nil
		},
	}
}

func rmCmd() *cobra.Command {
	var force bool
	cmd := &cobra.Command{
		Use:   "rm <id|all>",
		Short: "Remove container(s)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			dockerCli, err := getDockerClient()
			if err != nil {
				return err
			}
			defer dockerCli.Close()

			s := store.NewJSONStore(dbPath)
			ids, err := resolveIDs(dockerCli, s, args[0])
			if err != nil {
				return err
			}

			for _, id := range ids {
				if err := dockerCli.Remove(cmd.Context(), id, force); err != nil {
					fmt.Fprintf(os.Stderr, "Failed to remove %s: %v\n", id, err)
					continue
				}
				fmt.Printf("Removed %s\n", id)
			}
			return nil
		},
	}
	cmd.Flags().BoolVarP(&force, "force", "f", false, "Force remove running containers")
	return cmd
}

func execCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "exec <id> [command...]",
		Short: "Execute command in container",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			dockerCli, err := getDockerClient()
			if err != nil {
				return err
			}
			defer dockerCli.Close()

			id := args[0]
			var execCmd []string
			if len(args) > 1 {
				execCmd = args[1:]
			} else {
				execCmd = []string{"/bin/sh"}
			}

			output, err := dockerCli.Exec(cmd.Context(), id, execCmd)
			if err != nil {
				return err
			}
			fmt.Print(output)
			return nil
		},
	}
}

func inspectCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "inspect <id>",
		Short: "Show container details",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			dockerCli, err := getDockerClient()
			if err != nil {
				return err
			}
			defer dockerCli.Close()

			s := store.NewJSONStore(dbPath)
			ids, err := resolveIDs(dockerCli, s, args[0])
			if err != nil {
				return err
			}

			info, err := dockerCli.Inspect(cmd.Context(), ids[0])
			if err != nil {
				return err
			}

			fmt.Printf("ID:      %s\n", info.ID[:12])
			fmt.Printf("Name:    %s\n", strings.TrimPrefix(info.Name, "/"))
			fmt.Printf("Image:   %s\n", info.Config.Image)
			fmt.Printf("Status:  %s\n", info.State.Status)
			fmt.Printf("Created: %s\n", info.Created)
			fmt.Printf("IPs:\n")
			for netName, net := range info.NetworkSettings.Networks {
				fmt.Printf("  %s: %s\n", netName, net.IPAddress)
			}
			return nil
		},
	}
}

func serveCmd() *cobra.Command {
	var port int
	var host string
	cmd := &cobra.Command{
		Use:   "serve",
		Short: "Start HTTP API server",
		RunE: func(cmd *cobra.Command, args []string) error {
			srv, err := NewServer(host, port, dbPath)
			if err != nil {
				return err
			}
			return srv.Start()
		},
	}
	cmd.Flags().IntVar(&port, "port", 8080, "Listen port")
	cmd.Flags().StringVar(&host, "host", "0.0.0.0", "Listen host")
	return cmd
}
```

Note: `NewServer` is defined in a separate file within the `cli` package for now (Task 5 refactors into `internal/server`).

- [ ] **Step 2: Verify build**

Run: `cd /data/code/container-master && go build ./cmd/dockerman 2>&1`
Expected: Errors about undefined `NewServer`, `installSystemdCmd`, `uninstallSystemdCmd` — we will add these next

---

### Task 5: Implement HTTP server with permission middleware

**Files:**
- Create: `internal/server/server.go`
- Create: `internal/server/handler.go`
- Create: `internal/server/middleware.go`
- Modify: `internal/cli/commands.go` (wire up server)

- [ ] **Step 1: Implement permission middleware**

Write `internal/server/middleware.go`:

```go
package server

import (
	"context"
	"net"
	"net/http"
	"strings"

	"dockerman/internal/docker"
)

type contextKey string

const (
	ctxKeySourceType contextKey = "source_type"
	ctxKeyContainerID contextKey = "container_id"
)

type SourceType int

const (
	SourceLocalhost SourceType = iota
	SourceContainer
	SourceRemote
)

func ContainerAuthMiddleware(dockerCli *docker.Client) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ip, _, _ := net.SplitHostPort(r.RemoteAddr)
			if ip == "" {
				ip = r.RemoteAddr
			}

			if ip == "127.0.0.1" || ip == "::1" {
				ctx := context.WithValue(r.Context(), ctxKeySourceType, SourceLocalhost)
				next.ServeHTTP(w, r.WithContext(ctx))
				return
			}

			containerID, err := dockerCli.FindContainerByIP(r.Context(), ip)
			if err == nil && containerID != "" {
				ctx := context.WithValue(r.Context(), ctxKeySourceType, SourceContainer)
				ctx = context.WithValue(ctx, ctxKeyContainerID, containerID)
				next.ServeHTTP(w, r.WithContext(ctx))
				return
			}

			ctx := context.WithValue(r.Context(), ctxKeySourceType, SourceRemote)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func GetSourceType(r *http.Request) SourceType {
	v, ok := r.Context().Value(ctxKeySourceType).(SourceType)
	if !ok {
		return SourceRemote
	}
	return v
}

func GetSourceContainerID(r *http.Request) string {
	v, _ := r.Context().Value(ctxKeyContainerID).(string)
	return v
}

func isWriteMethod(method string) bool {
	return method == http.MethodPost || method == http.MethodDelete || method == http.MethodPut || method == http.MethodPatch
}

// RequireWriteAccess checks write permission. localhost: full, container: self only, remote: denied.
func RequireWriteAccess(targetContainerID string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			src := GetSourceType(r)
			switch src {
			case SourceLocalhost:
				next.ServeHTTP(w, r)
			case SourceContainer:
				myID := GetSourceContainerID(r)
				if targetContainerID == myID {
					next.ServeHTTP(w, r)
				} else {
					writeJSON(w, http.StatusForbidden, apiResponse{Success: false, Error: "container can only operate on itself"})
				}
			default:
				writeJSON(w, http.StatusForbidden, apiResponse{Success: false, Error: "write operations require localhost or container-origin"})
			}
		})
	}
}

// RequireWriteAllAccess blocks container-origin from all-ops.
func RequireWriteAllAccess() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			src := GetSourceType(r)
			switch src {
			case SourceLocalhost:
				next.ServeHTTP(w, r)
			case SourceContainer:
				writeJSON(w, http.StatusForbidden, apiResponse{Success: false, Error: "container cannot operate on all containers"})
			default:
				writeJSON(w, http.StatusForbidden, apiResponse{Success: false, Error: "write operations require localhost or container-origin"})
			}
		})
	}
}

// RequireReadAccess blocks nothing for reads (all sources allowed).
func RequireReadAccess() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			next.ServeHTTP(w, r)
		})
	}
}

// FilterListForContainer filters listing to only show the source container.
func FilterListForContainer(dockerCli *docker.Client) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			src := GetSourceType(r)
			if src == SourceContainer {
				// Let handler deal with filtering; set a context hint
				ctx := context.WithValue(r.Context(), contextKey("filter_self"), true)
				next.ServeHTTP(w, r.WithContext(ctx))
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func ShouldFilterSelf(r *http.Request) bool {
	v, _ := r.Context().Value(contextKey("filter_self")).(bool)
	return v
}
```

- [ ] **Step 2: Implement HTTP handlers**

Write `internal/server/handler.go`:

```go
package server

import (
	"encoding/json"
	"net/http"
	"strings"

	"dockerman/internal/docker"
	"dockerman/internal/model"
	"dockerman/internal/store"

	"github.com/gorilla/mux"
)

type apiResponse struct {
	Success bool        `json:"success"`
	Data    interface{} `json:"data,omitempty"`
	Error   string      `json:"error,omitempty"`
}

type Handler struct {
	dockerCli *docker.Client
	store     *store.JSONStore
}

func NewHandler(dockerCli *docker.Client, s *store.JSONStore) *Handler {
	return &Handler{dockerCli: dockerCli, store: s}
}

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func (h *Handler) RegisterRoutes(r *mux.Router) {
	// Read endpoints — open to all sources
	r.HandleFunc("/api/v1/health", h.Health).Methods("GET")
	r.HandleFunc("/api/v1/containers", h.ListContainers).Methods("GET")
	r.HandleFunc("/api/v1/containers/{id}", h.GetContainer).Methods("GET")

	// Scan — localhost only
	r.Handle("/api/v1/containers/scan", RequireWriteAllAccess()(http.HandlerFunc(h.Scan))).Methods("POST")

	// Single-container write ops
	singleWrite := r.PathPrefix("/api/v1/containers/{id}").Subrouter()
	singleWrite.Use(mux.MiddlewareFunc(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			id := mux.Vars(r)["id"]
			RequireWriteAccess(id)(next).ServeHTTP(w, r)
		})
	}))
	singleWrite.HandleFunc("/{id}/start", h.StartContainer).Methods("POST")
	singleWrite.HandleFunc("/{id}/stop", h.StopContainer).Methods("POST")
	singleWrite.HandleFunc("/{id}/restart", h.RestartContainer).Methods("POST")
	singleWrite.HandleFunc("/{id}", h.RemoveContainer).Methods("DELETE")

	// All-container ops — localhost only
	r.Handle("/api/v1/containers/start-all", RequireWriteAllAccess()(http.HandlerFunc(h.StartAll))).Methods("POST")
	r.Handle("/api/v1/containers/stop-all", RequireWriteAllAccess()(http.HandlerFunc(h.StopAll))).Methods("POST")
	r.Handle("/api/v1/containers/all", RequireWriteAllAccess()(http.HandlerFunc(h.RemoveAll))).Methods("DELETE")

	// Exec
	r.Handle("/api/v1/exec/{id}", RequireWriteAccess(mux.Vars(r)["id"])(http.HandlerFunc(h.ExecContainer))).Methods("POST")
}

func (h *Handler) Health(w http.ResponseWriter, r *http.Request) {
	if err := h.dockerCli.Ping(r.Context()); err != nil {
		writeJSON(w, http.StatusServiceUnavailable, apiResponse{Success: false, Error: "docker daemon unreachable"})
		return
	}
	writeJSON(w, http.StatusOK, apiResponse{Success: true, Data: "ok"})
}

func (h *Handler) Scan(w http.ResponseWriter, r *http.Request) {
	containers, err := h.dockerCli.ScanAll(r.Context())
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiResponse{Success: false, Error: err.Error()})
		return
	}
	if err := h.store.Save(containers); err != nil {
		writeJSON(w, http.StatusInternalServerError, apiResponse{Success: false, Error: err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, apiResponse{Success: true, Data: containers})
}

func (h *Handler) ListContainers(w http.ResponseWriter, r *http.Request) {
	containers, err := h.store.Load()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiResponse{Success: false, Error: err.Error()})
		return
	}

	if ShouldFilterSelf(r) {
		myID := GetSourceContainerID(r)
		filtered := make([]model.ContainerInfo, 0)
		for _, c := range containers {
			if c.ID == myID || c.Name == myID {
				filtered = append(filtered, c)
			}
		}
		writeJSON(w, http.StatusOK, apiResponse{Success: true, Data: filtered})
		return
	}

	writeJSON(w, http.StatusOK, apiResponse{Success: true, Data: containers})
}

func (h *Handler) GetContainer(w http.ResponseWriter, r *http.Request) {
	id := mux.Vars(r)["id"]

	// Container source can only see itself
	if src := GetSourceType(r); src == SourceContainer {
		myID := GetSourceContainerID(r)
		if id != myID {
			// Try matching by name
			ctr, err := h.store.GetByID(id)
			if err != nil || (ctr.ID != myID && ctr.Name != myID) {
				writeJSON(w, http.StatusForbidden, apiResponse{Success: false, Error: "container can only view itself"})
				return
			}
		}
	}

	ctr, err := h.store.GetByID(id)
	if err != nil {
		writeJSON(w, http.StatusNotFound, apiResponse{Success: false, Error: err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, apiResponse{Success: true, Data: ctr})
}

func (h *Handler) StartContainer(w http.ResponseWriter, r *http.Request) {
	id := mux.Vars(r)["id"]
	if err := h.dockerCli.Start(r.Context(), id); err != nil {
		writeJSON(w, http.StatusInternalServerError, apiResponse{Success: false, Error: err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, apiResponse{Success: true, Data: "started"})
}

func (h *Handler) StopContainer(w http.ResponseWriter, r *http.Request) {
	id := mux.Vars(r)["id"]
	if err := h.dockerCli.Stop(r.Context(), id); err != nil {
		writeJSON(w, http.StatusInternalServerError, apiResponse{Success: false, Error: err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, apiResponse{Success: true, Data: "stopped"})
}

func (h *Handler) RestartContainer(w http.ResponseWriter, r *http.Request) {
	id := mux.Vars(r)["id"]
	if err := h.dockerCli.Stop(r.Context(), id); err != nil {
		writeJSON(w, http.StatusInternalServerError, apiResponse{Success: false, Error: err.Error()})
		return
	}
	if err := h.dockerCli.Start(r.Context(), id); err != nil {
		writeJSON(w, http.StatusInternalServerError, apiResponse{Success: false, Error: err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, apiResponse{Success: true, Data: "restarted"})
}

func (h *Handler) RemoveContainer(w http.ResponseWriter, r *http.Request) {
	id := mux.Vars(r)["id"]
	if err := h.dockerCli.Remove(r.Context(), id, false); err != nil {
		writeJSON(w, http.StatusInternalServerError, apiResponse{Success: false, Error: err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, apiResponse{Success: true, Data: "removed"})
}

func (h *Handler) StartAll(w http.ResponseWriter, r *http.Request) {
	containers, err := h.store.Load()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiResponse{Success: false, Error: err.Error()})
		return
	}
	results := make([]string, 0)
	for _, c := range containers {
		if err := h.dockerCli.Start(r.Context(), c.ID); err != nil {
			results = append(results, "failed: "+c.ID+" - "+err.Error())
		} else {
			results = append(results, "started: "+c.ID)
		}
	}
	writeJSON(w, http.StatusOK, apiResponse{Success: true, Data: results})
}

func (h *Handler) StopAll(w http.ResponseWriter, r *http.Request) {
	containers, err := h.store.Load()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiResponse{Success: false, Error: err.Error()})
		return
	}
	results := make([]string, 0)
	for _, c := range containers {
		if err := h.dockerCli.Stop(r.Context(), c.ID); err != nil {
			results = append(results, "failed: "+c.ID+" - "+err.Error())
		} else {
			results = append(results, "stopped: "+c.ID)
		}
	}
	writeJSON(w, http.StatusOK, apiResponse{Success: true, Data: results})
}

func (h *Handler) RemoveAll(w http.ResponseWriter, r *http.Request) {
	containers, err := h.store.Load()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiResponse{Success: false, Error: err.Error()})
		return
	}
	results := make([]string, 0)
	for _, c := range containers {
		if err := h.dockerCli.Remove(r.Context(), c.ID, false); err != nil {
			results = append(results, "failed: "+c.ID+" - "+err.Error())
		} else {
			results = append(results, "removed: "+c.ID)
		}
	}
	writeJSON(w, http.StatusOK, apiResponse{Success: true, Data: results})
}

func (h *Handler) ExecContainer(w http.ResponseWriter, r *http.Request) {
	id := mux.Vars(r)["id"]
	var body struct {
		Command []string `json:"command"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || len(body.Command) == 0 {
		writeJSON(w, http.StatusBadRequest, apiResponse{Success: false, Error: "missing command in request body"})
		return
	}
	output, err := h.dockerCli.Exec(r.Context(), id, body.Command)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiResponse{Success: false, Error: err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, apiResponse{Success: true, Data: output})
}
```

- [ ] **Step 3: Implement server setup**

Write `internal/server/server.go`:

```go
package server

import (
	"fmt"
	"net/http"

	"dockerman/internal/docker"
	"dockerman/internal/store"

	"github.com/gorilla/mux"
)

type Server struct {
	host      string
	port      int
	dbPath    string
	dockerCli *docker.Client
	store     *store.JSONStore
}

func NewServer(host string, port int, dbPath string) (*Server, error) {
	dockerCli, err := docker.NewClient()
	if err != nil {
		return nil, err
	}
	return &Server{
		host:      host,
		port:      port,
		dbPath:    dbPath,
		dockerCli: dockerCli,
		store:     store.NewJSONStore(dbPath),
	}, nil
}

func (s *Server) Start() error {
	r := mux.NewRouter()

	// Auth middleware on all routes
	r.Use(ContainerAuthMiddleware(s.dockerCli))
	r.Use(FilterListForContainer(s.dockerCli))

	handler := NewHandler(s.dockerCli, s.store)
	handler.RegisterRoutes(r)

	addr := fmt.Sprintf("%s:%d", s.host, s.port)
	fmt.Printf("Docker Manager HTTP server listening on %s\n", addr)
	fmt.Printf("Database: %s\n", s.dbPath)
	return http.ListenAndServe(addr, r)
}
```

- [ ] **Step 4: Update CLI to wire up server**

Modify `internal/cli/commands.go` — replace `NewServer` call in `serveCmd` with importing `dockerman/internal/server`:

```go
import (
	// ... existing imports ...
	"dockerman/internal/server"
)

// In serveCmd RunE:
srv, err := server.NewServer(host, port, dbPath)
if err != nil {
	return err
}
return srv.Start()
```

- [ ] **Step 5: Verify build**

Run: `cd /data/code/container-master && go build ./...`
Expected: Build succeeds

- [ ] **Step 6: Commit**

```bash
git add internal/server/ internal/cli/
git commit -m "feat: implement HTTP server with container-origin permissions"
```

---

### Task 6: Implement systemd install/uninstall

**Files:**
- Create: `internal/cli/systemd.go`

- [ ] **Step 1: Implement systemd commands**

Write `internal/cli/systemd.go`:

```go
package cli

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/spf13/cobra"
)

const systemdUnitContent = `[Unit]
Description=Docker Container Manager
After=docker.service network.target
Requires=docker.service

[Service]
Type=simple
ExecStart=%s serve --port %d --db %s
Restart=always
RestartSec=5

[Install]
WantedBy=multi-user.target
`

func installSystemdCmd() *cobra.Command {
	var port int
	var binPath string
	var force bool
	var noEnable bool

	cmd := &cobra.Command{
		Use:   "install-systemd",
		Short: "Install systemd service unit for dockerman HTTP server",
		RunE: func(cmd *cobra.Command, args []string) error {
			if os.Geteuid() != 0 {
				return fmt.Errorf("this command requires root privileges; re-run with sudo")
			}

			if binPath == "" {
				exe, err := os.Executable()
				if err != nil {
					return fmt.Errorf("cannot determine binary path: %w; use --bin flag", err)
				}
				binPath = exe
			}

			unitPath := "/etc/systemd/system/dockerman.service"

			if _, err := os.Stat(unitPath); err == nil && !force {
				fmt.Printf("Unit file %s already exists. Overwrite? (y/N): ", unitPath)
				var answer string
				fmt.Scanln(&answer)
				if answer != "y" && answer != "Y" {
					fmt.Println("Aborted.")
					return nil
				}
			}

			content := fmt.Sprintf(systemdUnitContent, binPath, port, dbPath)
			if err := os.WriteFile(unitPath, []byte(content), 0644); err != nil {
				return fmt.Errorf("write unit file: %w", err)
			}
			fmt.Printf("Installed %s\n", unitPath)

			if err := exec.Command("systemctl", "daemon-reload").Run(); err != nil {
				return fmt.Errorf("daemon-reload failed: %w", err)
			}
			fmt.Println("systemd daemon reloaded")

			if !noEnable {
				if err := exec.Command("systemctl", "enable", "dockerman").Run(); err != nil {
					return fmt.Errorf("enable service failed: %w", err)
				}
				fmt.Println("Service enabled (dockerman.service)")
			}

			fmt.Println("\nStart with: systemctl start dockerman")
			fmt.Println("Status with: systemctl status dockerman")
			return nil
		},
	}

	cmd.Flags().IntVar(&port, "port", 8080, "HTTP server port")
	cmd.Flags().StringVar(&binPath, "bin", "", "Binary path (default: current executable)")
	cmd.Flags().BoolVar(&force, "force", false, "Overwrite existing unit file without prompt")
	cmd.Flags().BoolVar(&noEnable, "no-enable", false, "Install unit file but do not enable the service")
	return cmd
}

func uninstallSystemdCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "uninstall-systemd",
		Short: "Remove systemd service unit",
		RunE: func(cmd *cobra.Command, args []string) error {
			if os.Geteuid() != 0 {
				return fmt.Errorf("this command requires root privileges; re-run with sudo")
			}

			unitPath := "/etc/systemd/system/dockerman.service"

			fmt.Println("Stopping dockerman service...")
			exec.Command("systemctl", "stop", "dockerman").Run()

			fmt.Println("Disabling dockerman service...")
			exec.Command("systemctl", "disable", "dockerman").Run()

			if _, err := os.Stat(unitPath); err == nil {
				if err := os.Remove(unitPath); err != nil {
					return fmt.Errorf("remove unit file: %w", err)
				}
				fmt.Printf("Removed %s\n", unitPath)
			}

			if err := exec.Command("systemctl", "daemon-reload").Run(); err != nil {
				return fmt.Errorf("daemon-reload failed: %w", err)
			}
			fmt.Println("systemd daemon reloaded")
			fmt.Println("Dockerman systemd service uninstalled.")
			return nil
		},
	}
}
```

- [ ] **Step 2: Verify build**

Run: `cd /data/code/container-master && go build ./...`
Expected: Build succeeds

- [ ] {**Step 3: Fix middleware import in handler.go (unused strings import)**

Run: `cd /data/code/container-master && go build ./... 2>&1`
Expected: Clean build

- [ ] **Step 4: Commit**

```bash
git add internal/cli/systemd.go
git commit -m "feat: add systemd install/uninstall commands"
```

---

### Task 7: Wire everything together — final main.go and bash wrapper

**Files:**
- Modify: `cmd/dockerman/main.go`
- Modify: `dockerman` (bash wrapper)

- [ ] **Step 1: Verify main.go is correct**

Run: `cd /data/code/container-master && go build -o bin/dockerman ./cmd/dockerman`
Expected: Build succeeds, binary at `bin/dockerman`

- [ ] **Step 2: Test bash wrapper**

Run: `cd /data/code/container-master && ./dockerman --help`
Expected: Displays cobra help with all subcommands

- [ ] **Step 3: Commit**

```bash
git add -A
git commit -m "feat: wire final build and bash wrapper"
```

---

### Task 8: End-to-end verification and cleanup

- [ ] **Step 1: Run all tests**

Run: `cd /data/code/container-master && go test ./... -v`
Expected: All tests PASS

- [ ] **Step 2: Build final binary**

Run: `cd /data/code/container-master && go build -o bin/dockerman ./cmd/dockerman`
Expected: Clean build

- [ ] **Step 3: Verify CLI help**

Run: `./bin/dockerman --help`
Expected: Lists all commands: scan, list, start, stop, rm, exec, inspect, serve, install-systemd, uninstall-systemd

- [ ] **Step 4: Verify subcommand help**

Run: `./bin/dockerman scan --help`
Expected: Shows scan command help

- [ ] **Step 5: Remove unused imports / run go mod tidy**

Run: `cd /data/code/container-master && go mod tidy`
Expected: Clean go.mod and go.sum

- [ ] **Step 6: Final commit**

```bash
git add -A
git commit -m "chore: go mod tidy, final cleanup"
```
