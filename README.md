# dockerman

Docker container management tool with CLI and HTTP API. Scan, start, stop, remove, inspect, and exec into local Docker containers — either from the command line or via REST endpoints.

## Quick start

```bash
# Build
go build -o bin/dockerman ./cmd/dockerman

# Or use the wrapper (auto-builds on first run)
./dockerman scan
./dockerman list
./dockerman start <id>
./dockerman serve
```

## CLI

```
dockerman [--db <path>] <command> [args...]

  --db <path>    JSON database path (default: /var/lib/dockerman/containers.json)
                 Also configurable via CONTAINER_DB environment variable.

Commands:
  scan                           Discover all Docker containers, persist metadata
  list                           List containers from the database
  start    <id|all>              Start container(s) by ID/name, or all
  stop     <id|all>              Stop container(s) by ID/name, or all
  rm       <id|all>              Remove container(s) by ID/name, or all
                                   --force, -f    Force remove running containers
  exec     <id> <command...>     Execute a command in a running container
  inspect  <id>                  Show detailed container info (network, IP, etc.)
  serve                          Start the HTTP API server
                                   --port, -p 8080    Listen port
                                   --host 0.0.0.0     Listen host
  install-systemd                Install as a systemd service
                                   --port 8080         API server port
                                   --bin <path>        Binary path override
                                   --db <path>         JSON db path
                                   --force             Skip overwrite prompt
                                   --no-enable         Install but don't enable
  uninstall-systemd              Uninstall the systemd service
```

Container IDs match by short-ID prefix, exact name, or the literal `all`.

## HTTP API

Base path: `http://<host>:<port>/api/v1`

All responses follow `{"success": true, "data": <...>}` or `{"success": false, "error": "<message>"}`.

### Read endpoints (all sources)

| Method | Path | Description |
|--------|------|-------------|
| `GET` | `/api/v1/health` | Health check (pings Docker daemon) |
| `GET` | `/api/v1/containers` | List all containers |
| `GET` | `/api/v1/containers/{id}` | Get container by ID or name |

### Write endpoints (localhost or container-origin only)

| Method | Path | Description |
|--------|------|-------------|
| `POST` | `/api/v1/containers/{id}/start` | Start a container |
| `POST` | `/api/v1/containers/{id}/stop` | Stop a container |
| `POST` | `/api/v1/containers/{id}/restart` | Restart a container |
| `DELETE` | `/api/v1/containers/{id}` | Remove a container |
| `POST` | `/api/v1/exec/{id}` | Execute a command (body: `{"command": ["..."]}`) |

### Bulk endpoints (localhost only)

| Method | Path | Description |
|--------|------|-------------|
| `POST` | `/api/v1/containers/scan` | Scan all Docker containers |
| `POST` | `/api/v1/containers/start-all` | Start all containers |
| `POST` | `/api/v1/containers/stop-all` | Stop all containers |
| `DELETE` | `/api/v1/containers/all` | Remove all containers |

### Permission model

| Source | Scope | Permissions |
|--------|-------|-------------|
| Localhost (`127.0.0.1`/`::1`) | Full | All reads, writes, scans, bulk ops |
| Container origin | Self-only | Read/write/exec on own container; cannot operate on others or bulk |
| Remote | Read-only | GET health and containers only |

## systemd service

```bash
# Install (runs on port 8080 by default)
./dockerman install-systemd

# Custom port and binary
./dockerman install-systemd --port 9090 --bin /usr/local/bin/dockerman

# Uninstall
./dockerman uninstall-systemd
```

## Data storage

Container metadata is persisted as JSON at the path specified by `--db` (default `/var/lib/dockerman/containers.json`). Each scan overwrites this file with the current state of all local Docker containers.

## Dependencies

- Go 1.22+
- Docker daemon (local, accessed via Docker SDK)
- Linux for systemd integration (CLI and API work on any platform with Docker)
