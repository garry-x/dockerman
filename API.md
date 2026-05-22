# Dockerman HTTP API

Base URL: `http://<host>:5001/api/v1`

## Response format

All responses use the same envelope:

```json
{"success": true, "data": ...}
{"success": false, "error": "<message>"}
```

HTTP status codes: `200` OK, `400` Bad Request, `403` Forbidden, `404` Not Found, `500` Internal Server Error, `503` Service Unavailable.

## Permissions

| Source | Detection | Access |
|--------|-----------|--------|
| **Localhost** | `127.0.0.1`/`::1` + `X-Dockerman-Token` header | Full read/write |
| **Container** | RemoteAddr matches Docker network IP | Self only: read/write own container |
| **Remote** | Everything else | Read-only: GET endpoints |
| **Unauthenticated localhost** | `127.0.0.1` without token | Blocked: container-specific reads, all writes |

> **Note:** `{id}` in paths accepts both container ID prefix and container name.

## Authentication

When dockerman is installed via systemd (`install`), a random auth token is generated and stored at `/var/lib/dockerman/auth`. Requests from `localhost` must include this token in the `X-Dockerman-Token` header. This prevents host-network containers from impersonating localhost.

```bash
# Read the token
cat /var/lib/dockerman/auth

# Include in requests
curl -H "X-Dockerman-Token: <token>" http://localhost:5001/api/v1/...
```

---

## Health

**`GET /api/v1/health`**

Ping the Docker daemon. Open to all sources.

```bash
curl http://localhost:5001/api/v1/health
```

```json
{"success": true, "data": "ok"}
```

---

## Scan

**`POST /api/v1/containers/scan`**

Scan all Docker containers and persist metadata to the database. **Localhost only.**

```bash
TOKEN=$(cat /var/lib/dockerman/auth)
curl -X POST -H "X-Dockerman-Token: $TOKEN" http://localhost:5001/api/v1/containers/scan
```

```json
{
  "success": true,
  "data": [
    {
      "id": "abc123def456",
      "name": "my-container",
      "image": "ubuntu:24.04",
      "status": "running",
      "ports": ["8080/tcp"],
      "labels": {},
      "created_at": "2026-04-20T23:25:18+08:00",
      "scanned_at": "2026-05-22T17:41:19+08:00"
    }
  ]
}
```

---

## List containers

**`GET /api/v1/containers`**

List all containers from the database. Container sources see only their own container.

```bash
curl http://localhost:5001/api/v1/containers
```

```json
{
  "success": true,
  "data": [
    {"id": "abc123def456", "name": "my-container", "image": "ubuntu:24.04", "status": "running", "ports": ["8080/tcp"], ...}
  ]
}
```

---

## Get container

**`GET /api/v1/containers/{id}`**

Get a container by ID prefix or name. Container sources can only view themselves.

```bash
# By ID prefix
curl http://localhost:5001/api/v1/containers/abc123def456

# By name
curl http://localhost:5001/api/v1/containers/my-container
```

```json
{
  "success": true,
  "data": {
    "id": "abc123def456",
    "name": "my-container",
    "image": "ubuntu:24.04",
    "status": "running",
    "ports": ["8080/tcp"],
    "labels": {},
    "created_at": "2026-04-20T23:25:18+08:00",
    "scanned_at": "2026-05-22T17:41:19+08:00"
  }
}
```

---

## Container info

**`GET /api/v1/containers/{id}/info`**

Same as Get container. Container sources can only view themselves.

```bash
# By name
curl http://localhost:5001/api/v1/containers/my-container/info

# By ID
curl http://localhost:5001/api/v1/containers/abc123def456/info
```

```json
{
  "success": true,
  "data": {
    "id": "abc123def456",
    "name": "my-container",
    "image": "ubuntu:24.04",
    "status": "running",
    "ports": ["8080/tcp"],
    ...
  }
}
```

---

## Start container

**`POST /api/v1/containers/{id}/start`**

Start a container. Container sources can only start themselves.

```bash
TOKEN=$(cat /var/lib/dockerman/auth)

# Start by name
curl -X POST -H "X-Dockerman-Token: $TOKEN" \
  http://localhost:5001/api/v1/containers/my-container/start

# Start by ID
curl -X POST -H "X-Dockerman-Token: $TOKEN" \
  http://localhost:5001/api/v1/containers/abc123def456/start
```

```json
{"success": true, "data": "started"}
```

---

## Stop container

**`POST /api/v1/containers/{id}/stop`**

Stop a container. Container sources can only stop themselves.

```bash
TOKEN=$(cat /var/lib/dockerman/auth)

curl -X POST -H "X-Dockerman-Token: $TOKEN" \
  http://localhost:5001/api/v1/containers/my-container/stop
```

```json
{"success": true, "data": "stopped"}
```

---

## Restart container

**`POST /api/v1/containers/{id}/restart`**

Stop then start a container. Container sources can only restart themselves.

```bash
TOKEN=$(cat /var/lib/dockerman/auth)

curl -X POST -H "X-Dockerman-Token: $TOKEN" \
  http://localhost:5001/api/v1/containers/my-container/restart
```

```json
{"success": true, "data": "restarted"}
```

---

## Remove container

**`DELETE /api/v1/containers/{id}`**

Remove a container. Container sources can only remove themselves.

```bash
TOKEN=$(cat /var/lib/dockerman/auth)

curl -X DELETE -H "X-Dockerman-Token: $TOKEN" \
  http://localhost:5001/api/v1/containers/my-container
```

```json
{"success": true, "data": "removed"}
```

---

## Exec command in container

**`POST /api/v1/exec/{id}`**

Execute a command in a running container. Request body must contain a `command` array. Container sources can only exec in themselves.

```bash
TOKEN=$(cat /var/lib/dockerman/auth)

# Run 'ls /'
curl -X POST -H "X-Dockerman-Token: $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"command": ["ls", "/"]}' \
  http://localhost:5001/api/v1/exec/my-container

# Run 'echo hello'
curl -X POST -H "X-Dockerman-Token: $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"command": ["echo", "hello"]}' \
  http://localhost:5001/api/v1/exec/my-container
```

```json
{"success": true, "data": "bin\nboot\ndev\netc\nhome\nlib\n..."}
```

---

## Start all containers

**`POST /api/v1/containers/start-all`**

Start all containers in the database. **Localhost only.**

```bash
TOKEN=$(cat /var/lib/dockerman/auth)
curl -X POST -H "X-Dockerman-Token: $TOKEN" http://localhost:5001/api/v1/containers/start-all
```

```json
{
  "success": true,
  "data": [
    "started: abc123def456",
    "started: 789ghi012jkl"
  ]
}
```

---

## Stop all containers

**`POST /api/v1/containers/stop-all`**

Stop all containers in the database. **Localhost only.**

```bash
TOKEN=$(cat /var/lib/dockerman/auth)
curl -X POST -H "X-Dockerman-Token: $TOKEN" http://localhost:5001/api/v1/containers/stop-all
```

```json
{
  "success": true,
  "data": [
    "stopped: abc123def456",
    "stopped: 789ghi012jkl"
  ]
}
```

---

## Remove all containers

**`DELETE /api/v1/containers/all`**

Remove all containers in the database. **Localhost only.**

```bash
TOKEN=$(cat /var/lib/dockerman/auth)
curl -X DELETE -H "X-Dockerman-Token: $TOKEN" http://localhost:5001/api/v1/containers/all
```

```json
{
  "success": true,
  "data": [
    "removed: abc123def456",
    "removed: 789ghi012jkl"
  ]
}
```

---

## Error examples

```bash
# Unauthenticated localhost (host-network container) reading
curl http://localhost:5001/api/v1/containers/789ghi012jkl/info
# → 403 {"success":false,"error":"access denied"}

# Container trying to stop another container
curl -X POST http://<host-ip>:5001/api/v1/containers/other-container/stop
# → 403 {"success":false,"error":"container can only operate on itself"}

# Remote write attempt
curl -X POST http://<host>:5001/api/v1/containers/my-container/stop
# → 403 {"success":false,"error":"write operations require localhost or container origin"}

# Invalid exec request
TOKEN=$(cat /var/lib/dockerman/auth)
curl -X POST -H "X-Dockerman-Token: $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{}' \
  http://localhost:5001/api/v1/exec/my-container
# → 400 {"success":false,"error":"missing command in request body"}
```
