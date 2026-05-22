# Docker Container Manager — Design Spec

## Overview

A Go-based tool (`dockerman`) that manages local Docker containers via CLI and HTTP API. Supports scanning, starting, stopping, and removing containers individually or in bulk (all). Container metadata is persisted to a JSON file.

- **Tech stack**: Go, Docker SDK for Go, cobra (CLI), gorilla/mux (HTTP), systemd (daemon)
- **Distribution**: Single binary + root-level bash wrapper
- **Data store**: JSON file at `/var/lib/dockerman/containers.json` (default)

## Project Structure

```
container-master/
├── dockerman                     # Bash entry-wrapper
├── go.mod
├── go.sum
├── cmd/dockerman/main.go         # Entrypoint: CLI or HTTP serve
├── internal/
│   ├── model/container.go        # ContainerInfo struct
│   ├── docker/
│   │   ├── client.go             # Docker SDK wrapper (scan, start, stop, rm, exec, inspect)
│   │   └── client_test.go
│   ├── store/
│   │   ├── json.go               # JSON file load / save / get-by-id
│   │   └── json_test.go
│   ├── server/
│   │   ├── server.go             # HTTP server setup + router
│   │   ├── handler.go            # Route handlers
│   │   ├── middleware.go          # Container origin detection + permission enforcement
│   │   └── server_test.go
│   └── cli/
│       └── commands.go           # cobra subcommands
└── containers.json               # Default data file (generated at runtime)
```

## Data Model

```go
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
```

- `Status`: one of `running`, `exited`, `paused`, `created`, `restarting`, `removing`, `dead`
- `ScannedAt`: timestamp of last scan

## Store (JSON File)

- `Load(path) ([]ContainerInfo, error)` — read all containers from file
- `Save(path, containers []ContainerInfo) error` — overwrite file with full list
- `GetByID(path, id string) (*ContainerInfo, error)` — find single container by id or name
- Path: flag `--db` or env `CONTAINER_DB`, default `/var/lib/dockerman/containers.json`
- Scan replaces the full file (sync with Docker reality). Non-existent file returns empty list.

## Docker SDK Layer

Wrapper functions:
- `ScanAll(ctx) ([]ContainerInfo, error)` — list ALL containers (running + stopped), map to ContainerInfo
- `Start(ctx, id string) error`
- `Stop(ctx, id string) error`
- `Remove(ctx, id string, force bool) error`
- `Exec(ctx, id string, cmd []string) (string, error)` — run command, return stdout
- `Inspect(ctx, id string) (types.ContainerJSON, error)` — low-level inspect
- `FindContainerByIP(ctx, ip string) (string, error)` — find running container ID by IP, used by auth middleware

## CLI (cobra)

```
dockerman [global-flags] <command> [args...]

Global flags:
  --db <path>    JSON database path (default: /var/lib/dockerman/containers.json)

Commands:
  scan            Scan all local containers and persist to JSON file
  list            List containers from JSON file
  start  <id|all> Start container(s)
  stop   <id|all> Stop container(s)
  rm     <id|all> Remove stopped container(s)
  exec   <id>     Execute command in container (passes remaining args as command)
  inspect <id>    Show container details
  serve           Start HTTP API server
    --port <port>  Listen port (default: 8080)
    --host <host>  Listen host (default: 0.0.0.0)
  install-systemd  Install systemd service unit
    --port <port>  HTTP port for the service (default: 8080)
    --db   <path>  JSON db path for the service (default: /var/lib/dockerman/containers.json)
    --bin  <path>  Binary path override (default: os.Executable())
    --force        Overwrite existing unit without prompt
    --no-enable    Install unit file but do not enable the service
  uninstall-systemd  Remove systemd service unit (stops + disables + deletes unit file)
```

**id matching**:
1. Exact name match (name is unique in Docker)
2. CID prefix match (Docker-standard short-ID matching, at least enough chars to be unambiguous)
3. `all` literal — operate on all containers in the JSON file

## HTTP API

Base: `http://<host>:<port>/api/v1`

### Endpoints

| Method   | Path                       | Description              |
|----------|----------------------------|--------------------------|
| `POST`   | `/api/v1/containers/scan`  | Scan and persist          |
| `GET`    | `/api/v1/containers`       | List all containers       |
| `GET`    | `/api/v1/containers/{id}`  | Get single container      |
| `POST`   | `/api/v1/containers/{id}/start`   | Start container    |
| `POST`   | `/api/v1/containers/{id}/stop`    | Stop container     |
| `POST`   | `/api/v1/containers/{id}/restart` | Restart container  |
| `DELETE` | `/api/v1/containers/{id}`         | Remove container   |
| `POST`   | `/api/v1/containers/start-all`    | Start all          |
| `POST`   | `/api/v1/containers/stop-all`     | Stop all           |
| `DELETE` | `/api/v1/containers/all`         | Remove all stopped  |
| `POST`   | `/api/v1/exec/{id}`               | Exec command in container |
| `GET`    | `/api/v1/health`                  | Health check       |

### Response Format

```json
{"success": true, "data": <...>}
{"success": false, "error": "<message>"}
```

HTTP status codes: 200 (success), 400 (bad request), 403 (forbidden), 404 (not found), 500 (internal error).

## Permission Model (HTTP Only)

Three-tier access control enforced by middleware:

| Source       | Detection                    | Permissions                              |
|-------------|------------------------------|------------------------------------------|
| localhost    | RemoteAddr is `127.0.0.1` / `::1` | **Full** — all read + write + scan + all ops |
| Container    | RemoteAddr IP matches a running container's network IP | **Self only** — read/write but only on own container; `all` ops return 403; list returns only self |
| Other remote | Everything else              | **Read-only** — GET endpoints only; any write/scan/exec returns 403 |

- Container origin detection: on each request, iterate all running containers' `NetworkSettings.Networks` to find the source IP
- No caching — container IPs can change
- `GET /containers` from inside a container returns only the source container in the list
- `POST /api/v1/containers/scan` over HTTP: localhost-only (other sources return 403)
- CLI always has full permissions (not subject to HTTP auth middleware)

## systemd Integration

### install-systemd

1. Determine paths: binary (`os.Executable()` or `--bin`), db (`--db`), port (`--port`)
2. Generate unit file:

```ini
[Unit]
Description=Docker Container Manager
After=docker.service network.target
Requires=docker.service

[Service]
Type=simple
ExecStart=<binary-path> serve --port <port> --db <db-path>
Restart=always
RestartSec=5

[Install]
WantedBy=multi-user.target
```

3. Write to `/etc/systemd/system/dockerman.service`
4. If file exists and `--force` not set, prompt for confirmation
5. Run `systemctl daemon-reload`
6. Unless `--no-enable`, run `systemctl enable dockerman`
7. Requires root; if not root, suggest re-running with sudo

### uninstall-systemd

1. `systemctl stop dockerman`
2. `systemctl disable dockerman`
3. Remove `/etc/systemd/system/dockerman.service`
4. `systemctl daemon-reload`

## Bash Wrapper (`dockerman`)

Root-level convenience script that auto-builds and runs the Go binary:

```bash
#!/bin/bash
BINARY="./bin/dockerman"
if [ ! -f "$BINARY" ]; then
    echo "Building dockerman..."
    go build -o "$BINARY" ./cmd/dockerman
fi
exec "$BINARY" "$@"
```

## Error Handling

- Docker daemon unreachable: return clear error message, no crash
- JSON file missing: scan creates parent dir + file; list returns empty list
- Ambiguous container ID: return candidates, ask user to be more specific
- Nonexistent container: return 404 / not-found error
- Invalid state transition (e.g. rm running container): return descriptive error
- Permission denied (HTTP): return 403 with message explaining the restriction

## Testing Strategy

- `docker/`: unit tests with Docker SDK mock client
- `store/`: table-driven tests with temporary JSON files
- `server/`: httptest + mock docker client, including permission middleware scenarios
- `cli/`: integration tests validating argument parsing and command dispatch
