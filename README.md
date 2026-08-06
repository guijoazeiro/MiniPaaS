# MiniPaaS

A self-hosted deployment platform inspired by Render and Railway. Deploy containerized applications via CLI or dashboard, with automatic subdomains, HTTPS, real-time log streaming, and rollback — all running on a single VPS.

> Portfolio project. Built to exercise production-grade backend patterns: Docker orchestration, dynamic reverse proxy management, encrypted secrets, WebSocket streaming, and a CLI-first API validated before any UI is written.

## Status

| Phase | Focus | State |
|---|---|---|
| 0 | Foundation (API skeleton, migrations, sqlc, Postgres) | ✅ done |
| 1 | Docker orchestration + deploy via CLI | ⏳ next |
| 2 | Dynamic reverse proxy (Caddy) + HTTPS | — |
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
| CLI | Go + Cobra | Same binary target as the server |
| Dashboard | Next.js 14 (App Router) | Ships last, consumes the same API as the CLI |

## Quick start

Prerequisites: Go 1.22+, Docker, [sqlc](https://sqlc.dev), and [golang-migrate](https://github.com/golang-migrate/migrate) *(only if you want to run migrations outside the API — otherwise `cmd/migrate` handles it)*.

```bash
git clone https://github.com/guijoazeiro/minipaas
cd minipaas
cp .env.example .env
# generate a real key: openssl rand -hex 32 → replace ENCRYPTION_KEY in .env
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

```bash
curl localhost:8080/health
# {"status":"ok"}
```

## Project layout

```
minipaas/
├── api/
│   ├── cmd/
│   │   ├── server/         # HTTP entrypoint (Gin)
│   │   └── migrate/        # golang-migrate runner
│   ├── internal/
│   │   ├── config/         # env → struct
│   │   ├── store/postgres/ # concrete stores (+ sqlc/ generated code)
│   │   ├── docker/         # Docker Engine wrapper (phase 1)
│   │   ├── caddy/          # Caddy Admin wrapper (phase 2)
│   │   ├── crypto/         # AES-256-GCM (phase 3)
│   │   └── ws/             # WebSocket log stream (phase 4)
│   ├── sql/
│   │   ├── migrations/     # golang-migrate files (*.up.sql / *.down.sql)
│   │   └── queries/        # sqlc source queries
│   └── sqlc.yaml
├── cli/                    # minip CLI (phase 1+)
├── dashboard/              # Next.js UI (phase 7)
├── docker-compose.yml      # Postgres for local dev
└── .env.example
```

## Configuration

All configuration lives in environment variables, loaded from `.env` at startup.

| Variable | Required | Default | Purpose |
|---|---|---|---|
| `DATABASE_URL` | yes | — | Postgres DSN |
| `BASE_DOMAIN` | yes | — | e.g. `minipaas.yourdomain.com` — apps become `<name>.<BASE_DOMAIN>` |
| `ENCRYPTION_KEY` | yes | — | 32-byte hex (64 chars). Generate with `openssl rand -hex 32` |
| `JWT_SECRET` | yes | — | Signing secret for auth tokens |
| `PORT` | no | `:8080` | HTTP listen address |
| `DOCKER_HOST` | no | `unix:///var/run/docker.sock` | Docker Engine endpoint |
| `CADDY_ADMIN_URL` | no | `http://localhost:2019` | Must be localhost — never expose publicly |
| `LOG_LEVEL` | no | `info` | `debug` \| `info` \| `warn` \| `error` |

## Development

```bash
# Regenerate sqlc code after editing sql/queries/*.sql or sql/migrations/*.sql
cd api && sqlc generate

# Migrations
go run ./cmd/migrate up            # apply all pending
go run ./cmd/migrate down 1        # revert one
go run ./cmd/migrate version       # print current version
go run ./cmd/migrate force <v>     # unlock a dirty state

# Build binaries
go build -o bin/server  ./cmd/server
go build -o bin/migrate ./cmd/migrate

# Tests (once they exist)
go test ./... -race
```

## License

MIT
