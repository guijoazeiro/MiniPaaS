# MiniPaaS

A self-hosted deployment platform inspired by Render and Railway. Deploy containerized applications via CLI or dashboard, with automatic subdomains, HTTPS, real-time log streaming, and rollback — all running on a single VPS.

> Portfolio project. Built to exercise production-grade backend patterns: Docker orchestration, dynamic reverse proxy management, encrypted secrets, WebSocket streaming, and a CLI-first API validated before any UI is written.

## Status

| Phase | Focus | State |
|---|---|---|
| 0 | Foundation (API skeleton, migrations, sqlc, Postgres) | ✅ done |
| 1 | Docker orchestration + deploy via CLI | ✅ done |
| 2 | Dynamic reverse proxy (Caddy) + HTTPS | ✅ done |
| 3 | Auth (JWT) + encrypted env vars (AES-256-GCM) | ✅ done |
| 4 | WebSocket log streaming | ✅ done |
| 5 | Rollback + image retention | ✅ done |
| 6 | Health checks | ✅ done |
| 7 | Dashboard (Next.js) | ✅ done |
| 8 | Polish, CI, release | ✅ done |

## Stack

| Layer | Choice | Why |
|---|---|---|
| API | Go + Gin | Concurrency maps naturally to deploy/log workloads; single-binary deploy |
| DB | PostgreSQL 16 | Boring, correct, has `gen_random_uuid()` and `BYTEA` for encrypted secrets |
| Data access | [sqlc](https://sqlc.dev) | Type-safe SQL, no ORM runtime magic |
| Migrations | [golang-migrate](https://github.com/golang-migrate/migrate) | Versioned, reversible |
| Container runtime | Docker Engine API (Go SDK) | No Kubernetes — this runs on a $6 VPS |
| Reverse proxy | Caddy (Admin API) | Runtime route changes without config reloads; auto-ACME |
| Auth | JWT (HS256) + bcrypt | stdlib crypto everywhere, no session storage |
| Secrets | AES-256-GCM | Authenticated encryption, random nonce per record |
| Log streaming | gorilla/websocket + docker/stdcopy | Multiplexed stdout/stderr → per-line JSON frames |
| CLI | Go + Cobra | Single-binary target, easy release |
| Dashboard | Next.js 14 (App Router) | Ships last, consumes the same API as the CLI |

## Quick start

Prerequisites: Go 1.26+, Docker Desktop / Engine, Caddy 2.x, [sqlc](https://sqlc.dev).

```bash
git clone https://github.com/guijoazeiro/minipaas
cd minipaas
# generate a real key: openssl rand -hex 32 → paste into ENCRYPTION_KEY
```

Copy the example configuration for your shell:

```powershell
# Windows PowerShell
Copy-Item .env.example .env
```

```bash
# Linux/macOS
cp .env.example .env
```

On first startup the API seeds the admin user from `ADMIN_USERNAME` / `ADMIN_PASSWORD` (see `.env.example`).

## End-to-end walkthrough

Full path from empty machine to `minip logs hello -f` streaming live output. The walkthrough uses PowerShell syntax for the CLI; on Linux/macOS, replace `.\minip.exe` with `./minip`.

### 1. Start Postgres

Runs the DB in the background via `docker-compose.yml` at the repo root.

```bash
docker compose up -d
```

Verify it's healthy:

```bash
docker compose ps
```

### 2. Start Caddy

New terminal (leave it running):

```bash
caddy run --config Caddyfile
```

The `Caddyfile` only enables the Admin API on `localhost:2019`; every route is added at runtime by the API.

### 3. Generate sqlc code and run migrations

`sqlc generate` produces the typed query code in `api/internal/store/postgres/sqlc/`. It is checked into the repo, but re-run it after editing anything in `api/sql/`.

```bash
cd api
sqlc generate
go run ./cmd/migrate up
```

Expected: `migrate ok cmd=up`.

### 4. Start the API

Same terminal or a new one — it takes over the current shell:

```bash
go run ./cmd/server
```

Expected first-run logs (JSON, one per line):

```
seeded admin user username=admin
http listening addr=:8080
```

Sanity check from another terminal:

```bash
curl localhost:8080/health
# {"status":"ok"}
```

### 5. Build the CLI

New terminal:

```bash
cd cli
go build -o minip.exe .        # Linux/macOS: -o minip
```

### 6. Log in

```powershell
.\minip.exe login
```

Answers when prompted:
- `host` — press Enter to accept `http://localhost:8080`
- `username` — `admin` (whatever you put in `ADMIN_USERNAME`)
- `password` — `admin` (from `ADMIN_PASSWORD`)

Success prints `logged in as admin`. The token is stored in the OS user config directory: `%AppData%\minip\config.json` on Windows and `~/.config/minip/config.json` on Linux/macOS. Subsequent commands read it from there.

### 7. Create an app and set an env var

```powershell
.\minip.exe apps create hello
.\minip.exe env set hello GREETING="hello from minipaas"
.\minip.exe env list hello
```

`env list` shows the key + `updated_at`. Values are **never** returned by the API. At-rest check:

```bash
docker compose exec postgres psql -U postgres -d minipaas -c "SELECT key, encode(value,'hex') FROM env_vars;"
```

Column `value` is always ciphertext — never plaintext.

### 8. Deploy the sample app

The repo ships with a minimal Node.js app under `hello-world/` that logs a heartbeat every second (stdout) and an "even beat" every 3 seconds (stderr).

```powershell
.\minip.exe deploy ..\hello-world --app hello --wait
```

Output ends with something like `running on host port 57123`. That's the host port Docker bound to the container's `:8080`.

### 9. Sanity-check the container

```bash
curl localhost:<the port from step 8>
# hello
```

You should also see the request logged to the container's stdout in step 10.

### 10. Stream logs

Tail the last 100 lines (then exits normally):

```powershell
.\minip.exe logs hello
```

Follow mode (streams until `Ctrl+C`):

```powershell
.\minip.exe logs hello -f
```

While `-f` is running, hit the container again from another terminal:

```bash
curl localhost:<port>/foo
```

The `GET /foo` line appears live, interleaved with the heartbeat. To show just one stream in PowerShell:

```powershell
.\minip.exe logs hello -f 2>$null  # stdout only
.\minip.exe logs hello -f 1>$null  # stderr only (the event lines)
```

`-f` first prints the selected tail and then keeps streaming while the container produces output. If a container has crashed, `minip logs` still retrieves the saved output from the latest failed deployment for diagnosis.

### 11. Inspect the app

```powershell
.\minip.exe apps info hello
```

Shows status, live container state (`running`, `exited`, etc.), public URL (`https://hello.<BASE_DOMAIN>` — served by Caddy), and recent deployments (`running`, `superseded`, `failed`, `rolled_back`).

### 12. Test health checks

For a faster local check, add these values to `.env` and restart the API:

```env
HEALTH_CHECK_INTERVAL=3s
RESTART_POLICY=on-failure
RESTART_MAX_RETRIES=3
```

After deploying `hello`, inspect the policy Docker received:

```powershell
docker inspect --format '{{.HostConfig.RestartPolicy.Name}} max={{.HostConfig.RestartPolicy.MaximumRetryCount}}' minipaas-hello
# on-failure max=3
```

To simulate a crash, kill the container a few times in quick succession:

```powershell
1..4 | ForEach-Object {
  docker kill minipaas-hello
  Start-Sleep -Seconds 1
}
```

After the restart limit is exhausted, wait at least one health-check interval and inspect the app:

```powershell
.\minip.exe apps info hello
docker inspect --format '{{.State.Status}} restartCount={{.RestartCount}}' minipaas-hello
```

Expected result: the app and deployment become `failed`, and `container: exited` appears in `apps info`. You can still run `.\minip.exe logs hello` to inspect the crash output.

### 13. Roll back to a previous deployment

Make a visible change to `hello-world/server.js` (e.g. change the heartbeat message), redeploy, then roll back:

```powershell
.\minip.exe deploy ..\hello-world --app hello --wait     # creates v2, v1 becomes superseded
.\minip.exe rollback hello                                # interactive picker; select v1
.\minip.exe logs hello -f                                 # container now runs the old code
.\minip.exe apps info hello                               # v1 back to running, v2 → rolled_back
```

Non-interactive form:

```powershell
.\minip.exe rollback hello --to <deployment-id>
```

Rollback reuses the target's cached Docker image, so it takes seconds — no rebuild.

### 14. Clean up

```powershell
.\minip.exe apps list                   # inspect remaining apps
# From the api terminal: Ctrl+C to stop the server
# From the caddy terminal: Ctrl+C to stop caddy
docker compose down                     # stop Postgres; keep the volume
# Or use `docker compose down -v` instead to erase the database volume.
```

There is no `minip apps delete` command yet. To remove an app and its deployment history, call the API directly:

```powershell
curl.exe -X DELETE http://localhost:8080/apps/hello -H "Authorization: Bearer <token>"
```

## API

All endpoints require `Authorization: Bearer <token>` except `/health` and `/auth/login`.

```
GET    /health                        → { status: "ok" }
POST   /auth/login                    { username, password } → { token, expires_at }

POST   /apps                          { name } → App
GET    /apps                          → []App
GET    /apps/:name                    → App (includes current `container_state` when deployed)
DELETE /apps/:name                    → 204   (stops container, removes Caddy route, removes row)

POST   /apps/:name/deployments        multipart source=<tar> → 202 Deployment (build runs in background)
GET    /apps/:name/deployments        → []Deployment
GET    /apps/:name/deployments/:id    → Deployment

GET    /apps/:name/env                → []{ key, updated_at }        (values never returned)
PUT    /apps/:name/env/:key           { value } → 204                (AES-256-GCM at rest)
DELETE /apps/:name/env/:key           → 204

POST   /apps/:name/rollback           { deployment_id } → Deployment (restored)

WS     /apps/:name/logs?follow=true&tail=100     (active deployment, or latest failed deployment with logs)
       frames: { "ts": "...", "stream": "stdout|stderr", "line": "..." }
```

Rollback is synchronous: the API responds after Docker starts the selected image and Caddy updates its route.

Deployment status transitions:

```
pending → building → running    (happy path)
                   ↘ failed     (build or start failed)
running           → superseded  (replaced by a newer deploy)
                  → rolled_back (intentional rollback — phase 5)
                  → failed      (container exited, dead, or missing — health check)
```

## CLI

```bash
minip login                            # host + username + password → OS user config directory
minip apps create <name>
minip apps list
minip apps info <name>                 # status + container state + public URL + recent deployments
minip deploy [path] --app <name>       # tarball + upload (default path = .)
minip deploy ... --wait                # poll until running or failed
minip env list <app>                   # keys only, never values
minip env set <app> KEY=value [KEY=value ...]
minip env unset <app> KEY
minip logs <app>                       # last 100 lines; works for latest failed deployment too
minip logs <app> -f                    # follow until Ctrl+C or the container exits
minip logs <app> --tail all -f         # backfill everything, then follow
minip rollback <app>                   # interactive picker of eligible deployments
minip rollback <app> --to <id>         # skip the picker
```

Host resolution order: `--host` flag → `MINIPAAS_HOST` env → saved config → `http://localhost:8080`.

## Project layout

```
minipaas/
├── api/
│   ├── cmd/
│   │   ├── server/                    # HTTP entrypoint (Gin)
│   │   └── migrate/                   # golang-migrate runner
│   ├── internal/
│   │   ├── config/                    # env → struct + CADDY_ADMIN_URL localhost validation
│   │   ├── domain/                    # pure types + sentinel errors
│   │   ├── store/                     # store interfaces (App, Deployment, User, Env)
│   │   │   └── postgres/              # concrete stores (+ sqlc/ generated code)
│   │   ├── docker/                    # Docker Engine wrapper (build, run, stop, logs)
│   │   ├── caddy/                     # Caddy Admin API wrapper (route upsert/remove)
│   │   ├── crypto/                    # AES-256-GCM cipher
│   │   ├── health/                    # periodic Docker inspection + failure detection
│   │   ├── ws/                        # WebSocket log streaming + Docker log demux
│   │   ├── service/                   # business logic (app, deployment, auth, env)
│   │   └── handler/                   # Gin handlers + JWT middleware
│   ├── sql/
│   │   ├── migrations/                # golang-migrate files (*.up.sql / *.down.sql)
│   │   └── queries/                   # sqlc source queries
│   └── sqlc.yaml
├── cli/                               # separate Go module — Cobra CLI
│   ├── cmd/                           # root, login, apps, env, deploy, logs, rollback
│   ├── internal/
│   │   ├── api/                       # HTTP client (adds Authorization: Bearer)
│   │   ├── config/                    # ~/.config/minip/config.json (0600)
│   │   └── tarball/                   # dir → tar packer
│   └── main.go
├── dashboard/                         # Next.js UI (phase 7)
├── hello-world/                       # sample Node.js app used by the walkthrough
├── Caddyfile                          # minimal — enables the admin API
├── docker-compose.yml                 # Postgres for local dev
└── .env.example
```

## Configuration

All configuration lives in environment variables, loaded from `.env` at startup. Both the server and the migrate runner look for `.env` in the current directory and one level up, so running from either the repo root or `api/` works.

| Variable | Required | Default | Purpose |
|---|---|---|---|
| `DATABASE_URL` | yes | — | Postgres DSN |
| `BASE_DOMAIN` | yes | — | e.g. `minipaas.yourdomain.com` — apps become `<name>.<BASE_DOMAIN>` |
| `ENCRYPTION_KEY` | yes | — | 32-byte hex (64 chars). Generate with `openssl rand -hex 32` |
| `JWT_SECRET` | yes | — | Signing secret for auth tokens |
| `PORT` | no | `:8080` | HTTP listen address |
| `DOCKER_HOST` | no | auto-detect | Leave unset — the Docker SDK picks npipe on Windows, unix socket on Linux/Mac |
| `CADDY_ADMIN_URL` | no | `http://localhost:2019` | **Must** be localhost — the API refuses non-loopback values |
| `TOKEN_TTL` | no | `24h` | JWT lifetime — any `time.ParseDuration` value |
| `ADMIN_USERNAME` | no | — | First-run admin seed. Ignored once a user exists |
| `ADMIN_PASSWORD` | no | — | First-run admin seed. Ignored once a user exists |
| `IMAGE_RETENTION` | no | `5` | How many recent deployment images to keep per app. Older ones are pruned (best-effort) after each successful deploy — Docker refuses to delete in-use images, so the active one is always safe. |
| `HEALTH_CHECK_INTERVAL` | no | `30s` | How often running containers are inspected; `exited`, `dead`, and missing containers are marked failed. |
| `MAX_DEPLOY_SIZE_MB` | no | `100` | Maximum accepted deployment source upload size in MiB. Requests over the limit receive HTTP 413. |
| `RESTART_POLICY` | no | `on-failure` | Docker restart policy for app containers: `no`, `always`, `on-failure`, or `unless-stopped`. |
| `RESTART_MAX_RETRIES` | no | `3` | Maximum retries used by Docker's `on-failure` restart policy. |
| `DASHBOARD_ORIGIN` | no | `http://localhost:3000` | Browser origin allowed to authenticate with the API dashboard cookie. |
| `LOG_LEVEL` | no | `info` | `debug` \| `info` \| `warn` \| `error` |

## Development

```bash
# API
cd api

# Regenerate sqlc code after editing sql/queries/*.sql or sql/migrations/*.sql
sqlc generate

# Migrations
go run ./cmd/migrate up            # apply all pending
go run ./cmd/migrate down 1        # revert one
go run ./cmd/migrate version       # print current version
go run ./cmd/migrate force <v>     # unlock a dirty state

# Build binaries
go build -o bin/server  ./cmd/server
go build -o bin/migrate ./cmd/migrate

# CLI (separate module)
cd ../cli
go build -o minip .
```

### Tests

The default automated tests are unit tests, so they do not require Docker or PostgreSQL. The end-to-end walkthrough above remains the manual Docker/Postgres/Caddy validation path.

```bash
# From the repo root — runs the api module tests
go test ./...

# CLI module (separate go.mod)
cd cli && go test ./...

# Both
go test ./... && (cd cli && go test ./...)

# PostgreSQL integration test (after `docker compose up -d` and migrations)
DATABASE_URL='postgres://postgres:postgres@localhost:5432/minipaas?sslmode=disable' go test -count=1 -tags=integration ./api/internal/store/postgres
```

Coverage today:
- `crypto` — AES roundtrip, tamper detection, key size
- `service/auth` — login (happy/wrong password/unknown user), JWT roundtrip, tampered token, seed idempotency
- `service/env` — encrypt/decrypt roundtrip, **at-rest plaintext check**, key validation, app isolation
- `caddy` — bootstrap probe + upsert JSON contract + tolerant delete
- `ws` — line splitter + end-to-end WS handler with a fake Docker stream (via `stdcopy.NewStdWriter`) proving demux, ordering, and frame format
- `health` — exited/missing containers transition their deployment and app to `failed`; current container state lookup
- `cli/tarball` — kept/skipped paths, slash-normalized entries
- `store/postgres` (integration tag) — app persistence, uniqueness constraints, status and public URL updates against a real PostgreSQL instance

`-race` needs CGO (not enabled on Windows by default); CI runs it on Linux. GitHub Actions also vets the API, runs the CLI and dashboard checks, applies the migrations, and runs the PostgreSQL integration test.

### Releases

Pushing a tag in the `v*` format (for example, `v0.1.0`) starts the release workflow. It builds the API server, migration command, and CLI for Linux amd64, macOS amd64/arm64, and Windows amd64, then creates a GitHub release with those binaries.

## License

MIT
