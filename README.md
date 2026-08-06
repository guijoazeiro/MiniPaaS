# MiniPaaS

A self-hosted deployment platform inspired by Render and Railway. Deploy containerized applications via CLI or dashboard, with automatic subdomains, HTTPS, real-time log streaming, and rollback — all running on a single VPS.

> Portfolio project. Built to exercise production-grade backend patterns: Docker orchestration, dynamic reverse proxy management, encrypted secrets, WebSocket streaming, and a CLI-first API validated before any UI is written.

## Status

| Phase | Focus | State |
|---|---|---|
| 0 | Foundation (API skeleton, migrations, sqlc, Postgres) | ✅ done |
| 1 | Docker orchestration + deploy via CLI | ✅ done |
| 2 | Dynamic reverse proxy (Caddy) + HTTPS | ⏳ next |
| 3 | Auth (JWT) + encrypted env vars (AES-256-GCM) | — |
| 4 | WebSocket log streaming | — |
| 5 | Rollback | — |
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
| CLI | Go + Cobra | Single-binary target, easy release |
| Dashboard | Next.js 14 (App Router) | Ships last, consumes the same API as the CLI |

## Quick start

Prerequisites: Go 1.26+, Docker Desktop / Engine, [sqlc](https://sqlc.dev).

```bash
git clone https://github.com/guijoazeiro/minipaas
cd minipaas
cp .env.example .env
# generate a real key: openssl rand -hex 32 → paste into ENCRYPTION_KEY
```

Start Postgres:

```bash
docker compose up -d
```

Generate sqlc code, run migrations, start the API:

```bash
cd api
sqlc generate
go run ./cmd/migrate up
go run ./cmd/server
```

Build and use the CLI:

```bash
cd cli
go build -o minip .
./minip apps create hello
./minip deploy ../hello-world --app hello --wait
```

The last command tars the directory, uploads it, builds the image, starts a container on a random host port, and returns the port so you can `curl localhost:<port>`.

## API (phase 1)

All endpoints are unauthenticated until phase 3.

```
GET    /health
POST   /apps                          { name } → App
GET    /apps                          → []App
GET    /apps/:name                    → App
DELETE /apps/:name                    → 204   (stops container, removes row)
POST   /apps/:name/deployments        multipart source=<tar> → 202 Deployment
GET    /apps/:name/deployments        → []Deployment
GET    /apps/:name/deployments/:id    → Deployment
```

Deployment status transitions:

```
pending → building → running    (happy path)
                   ↘ failed     (build or start failed)
running           → superseded  (replaced by a newer deploy)
                  → rolled_back (intentional rollback — phase 5)
```

## CLI (phase 1)

```bash
minip apps create <name>              # create app
minip apps list                       # list apps
minip deploy [path] --app <name>      # tarball + upload (default path = .)
minip deploy ... --wait               # poll until running or failed
```

Config: `--host` flag or `MINIPAAS_HOST` env (defaults to `http://localhost:8080`).

## Project layout

```
minipaas/
├── api/
│   ├── cmd/
│   │   ├── server/         # HTTP entrypoint (Gin)
│   │   └── migrate/        # golang-migrate runner
│   ├── internal/
│   │   ├── config/         # env → struct
│   │   ├── domain/         # pure types + sentinel errors (App, Deployment...)
│   │   ├── store/          # store interfaces
│   │   │   └── postgres/   # concrete stores (+ sqlc/ generated code)
│   │   ├── docker/         # Docker Engine wrapper (build, run, stop, logs)
│   │   ├── service/        # business logic (app, deployment)
│   │   ├── handler/        # Gin handlers (translate domain errors → HTTP)
│   │   ├── caddy/          # Caddy Admin wrapper (phase 2)
│   │   ├── crypto/         # AES-256-GCM (phase 3)
│   │   └── ws/             # WebSocket log stream (phase 4)
│   ├── sql/
│   │   ├── migrations/     # golang-migrate files (*.up.sql / *.down.sql)
│   │   └── queries/        # sqlc source queries
│   └── sqlc.yaml
├── cli/                    # separate Go module — Cobra CLI
│   ├── cmd/                # root, apps, deploy
│   ├── internal/
│   │   ├── api/            # HTTP client
│   │   └── tarball/        # dir → tar packer
│   └── main.go
├── dashboard/              # Next.js UI (phase 7)
├── hello-world/            # tiny sample app for smoke-testing deploys
├── docker-compose.yml      # Postgres for local dev
└── .env.example
```

## Configuration

All configuration lives in environment variables, loaded from `.env` at startup. Both the server and the migrate runner look for `.env` in the current directory and one level up, so running from either the repo root or `api/` works.

| Variable | Required | Default | Purpose |
|---|---|---|---|
| `DATABASE_URL` | yes | — | Postgres DSN |
| `BASE_DOMAIN` | yes | — | e.g. `minipaas.yourdomain.com` — apps become `<name>.<BASE_DOMAIN>` (phase 2) |
| `ENCRYPTION_KEY` | yes | — | 32-byte hex (64 chars). Generate with `openssl rand -hex 32` |
| `JWT_SECRET` | yes | — | Signing secret for auth tokens (phase 3) |
| `PORT` | no | `:8080` | HTTP listen address |
| `DOCKER_HOST` | no | auto-detect | Leave unset for the Docker SDK to pick the OS default (npipe on Windows, unix socket on Linux/Mac) |
| `CADDY_ADMIN_URL` | no | `http://localhost:2019` | Must be localhost — never expose publicly |
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

# Tests (once they exist)
go test ./... -race
```

## License

MIT
