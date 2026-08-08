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
| 5 | Rollback | ⏳ next |
| 6 | Health checks | — |
| 7 | Dashboard (Next.js) | — |
| 8 | Polish, CI, release | — |

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
cp .env.example .env
# generate a real key: openssl rand -hex 32 → paste into ENCRYPTION_KEY
```

On first startup the API seeds the admin user from `ADMIN_USERNAME` / `ADMIN_PASSWORD` (see `.env.example`).

## End-to-end walkthrough

Full path from empty machine to `minip logs hello -f` streaming live output. Each step is required — nothing is optional here.

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

```bash
./minip.exe login
```

Answers when prompted:
- `host` — press Enter to accept `http://localhost:8080`
- `username` — `admin` (whatever you put in `ADMIN_USERNAME`)
- `password` — `admin` (from `ADMIN_PASSWORD`)

Success prints `logged in as admin`. The token lands in `~/.config/minip/config.json` (mode 0600) — subsequent commands read it from there.

### 7. Create an app and set an env var

```bash
./minip.exe apps create hello
./minip.exe env set hello GREETING="hello from minipaas"
./minip.exe env list hello
```

`env list` shows the key + `updated_at`. Values are **never** returned by the API. At-rest check:

```bash
docker compose exec postgres psql -U postgres -d minipaas -c "SELECT key, encode(value,'hex') FROM env_vars;"
```

Column `value` is always ciphertext — never plaintext.

### 8. Deploy the sample app

The repo ships with a minimal Node.js app under `hello-world/` that logs a heartbeat every second (stdout) and an "even beat" every 3 seconds (stderr).

```bash
./minip.exe deploy ../hello-world --app hello --wait
```

Output ends with something like `running on host port 57123`. That's the host port Docker bound to the container's `:8080`.

### 9. Sanity-check the container

```bash
curl localhost:<the port from step 8>
# hello
```

You should also see the request logged to the container's stdout in step 10.

### 10. Stream logs

Tail last 100 lines (finishes immediately):

```bash
./minip.exe logs hello
```

Follow mode (streams until `Ctrl+C`):

```bash
./minip.exe logs hello -f
```

While `-f` is running, hit the container again from another terminal:

```bash
curl localhost:<port>/foo
```

The `GET /foo` line appears live, interleaved with the heartbeat. Split streams if you want:

```bash
./minip.exe logs hello -f 2>/dev/null   # stdout only
./minip.exe logs hello -f 1>/dev/null   # stderr only (even beats)
```

### 11. Inspect the app

```bash
./minip.exe apps info hello
```

Shows status, public URL (`https://hello.<BASE_DOMAIN>` — served by Caddy), and the last few deployments with their statuses (`running`, `superseded`, `failed`).

### 12. Clean up

```bash
./minip.exe apps list                   # confirm
# From the api terminal: Ctrl+C to stop the server
# From the caddy terminal: Ctrl+C to stop caddy
docker compose down                     # tear down Postgres (keeps the volume)
docker compose down -v                  # wipes the volume too
```

## API

All endpoints require `Authorization: Bearer <token>` except `/health` and `/auth/login`.

```
GET    /health                        → { status: "ok" }
POST   /auth/login                    { username, password } → { token, expires_at }

POST   /apps                          { name } → App
GET    /apps                          → []App
GET    /apps/:name                    → App
DELETE /apps/:name                    → 204   (stops container, removes Caddy route, removes row)

POST   /apps/:name/deployments        multipart source=<tar> → 202 Deployment (build runs in background)
GET    /apps/:name/deployments        → []Deployment
GET    /apps/:name/deployments/:id    → Deployment

GET    /apps/:name/env                → []{ key, updated_at }        (values never returned)
PUT    /apps/:name/env/:key           { value } → 204                (AES-256-GCM at rest)
DELETE /apps/:name/env/:key           → 204

WS     /apps/:name/logs?follow=true&tail=100
       frames: { "ts": "...", "stream": "stdout|stderr", "line": "..." }
```

Deployment status transitions:

```
pending → building → running    (happy path)
                   ↘ failed     (build or start failed)
running           → superseded  (replaced by a newer deploy)
                  → rolled_back (intentional rollback — phase 5)
```

## CLI

```bash
minip login                            # host + username + password → ~/.config/minip/config.json (0600)
minip apps create <name>
minip apps list
minip apps info <name>                 # status + public URL + recent deployments
minip deploy [path] --app <name>       # tarball + upload (default path = .)
minip deploy ... --wait                # poll until running or failed
minip env list <app>                   # keys only, never values
minip env set <app> KEY=value [KEY=value ...]
minip env unset <app> KEY
minip logs <app>                       # last 100 lines
minip logs <app> -f                    # follow until Ctrl+C
minip logs <app> --tail all -f         # backfill everything, then follow
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
│   │   ├── ws/                        # WebSocket log streaming + Docker log demux
│   │   ├── service/                   # business logic (app, deployment, auth, env)
│   │   └── handler/                   # Gin handlers + JWT middleware
│   ├── sql/
│   │   ├── migrations/                # golang-migrate files (*.up.sql / *.down.sql)
│   │   └── queries/                   # sqlc source queries
│   └── sqlc.yaml
├── cli/                               # separate Go module — Cobra CLI
│   ├── cmd/                           # root, login, apps, env, deploy, logs
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

Unit tests only for now — no Docker or Postgres required. Integration tests land in phase 8.

```bash
# From the repo root — runs the api module tests
go test ./...

# CLI module (separate go.mod)
cd cli && go test ./...

# Both
go test ./... && (cd cli && go test ./...)
```

Coverage today:
- `crypto` — AES roundtrip, tamper detection, key size
- `service/auth` — login (happy/wrong password/unknown user), JWT roundtrip, tampered token, seed idempotency
- `service/env` — encrypt/decrypt roundtrip, **at-rest plaintext check**, key validation, app isolation
- `caddy` — bootstrap probe + upsert JSON contract + tolerant delete
- `ws` — line splitter + end-to-end WS handler with a fake Docker stream (via `stdcopy.NewStdWriter`) proving demux, ordering, and frame format
- `cli/tarball` — kept/skipped paths, slash-normalized entries

`-race` needs CGO (not enabled on Windows by default); CI (phase 8) will run it on Linux.

## License

MIT
