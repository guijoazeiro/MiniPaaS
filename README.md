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
| 9.1 | Deploy from public GitHub repositories | ✅ done |
| 9.1.1 | Dashboard navigation and operational UX | 🚧 implemented, awaiting manual validation |
| 10.1 | Signed GitHub webhooks and auto-deploy | ✅ done |
| 10.2 | Persistent build logs | ✅ done |
| 10.3 | Retry and cancellation | ✅ done |
| 11 | Zero-downtime deployments | 🚧 implemented, awaiting end-to-end validation |
| 12.1 | Custom domains | 🚧 implemented, awaiting DNS/HTTPS validation |
| 12.2 | Operational metrics | 🚧 implemented, awaiting manual validation |
| 12.3 | Real-time metrics stream | 🚧 implemented, awaiting manual validation |

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
| Metrics streaming | Docker stats + gorilla/websocket | Shared per-container snapshots for live CPU, memory, network, and disk charts |
| CLI | Go + Cobra | Single-binary target, easy release |
| Dashboard | Next.js 16 / Vinext (App Router) | Route-based control plane consuming the same API as the CLI |

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

### 5. Start the dashboard

In another terminal, install the dashboard dependencies and start the local control plane:

```powershell
cd dashboard
npm.cmd install
npm.cmd run dev
```

Open `http://localhost:3000`. The dashboard uses an HTTP-only cookie created by `POST /auth/web-login`; it does not expose the login token to browser JavaScript. The API must keep `DASHBOARD_ORIGIN=http://localhost:3000` (or the exact origin you use). If the API runs on another address, create `dashboard/.env.local` with:

```env
NEXT_PUBLIC_MINIPAAS_API_URL=http://localhost:8080
```

### 6. Build the CLI

New terminal:

```bash
cd cli
go build -o minip.exe .        # Linux/macOS: -o minip
```

### 7. Log in

```powershell
.\minip.exe login
```

Answers when prompted:
- `host` — press Enter to accept `http://localhost:8080`
- `username` — `admin` (whatever you put in `ADMIN_USERNAME`)
- `password` — `admin` (from `ADMIN_PASSWORD`)

Success prints `logged in as admin`. The token is stored in the OS user config directory: `%AppData%\minip\config.json` on Windows and `~/.config/minip/config.json` on Linux/macOS. Subsequent commands read it from there.

### 8. Create an app and set an env var

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

### 9. Deploy the sample app

The repo ships with a minimal Node.js app under `hello-world/` that logs a heartbeat every second (stdout) and an "even beat" every 3 seconds (stderr).

```powershell
.\minip.exe deploy ..\hello-world --app hello --wait
```

Output ends with something like `running on host port 57123`. That's the host port Docker bound to the container's `:8080`.

### 10. Sanity-check the container

```bash
curl localhost:<the port from step 8>
# hello
```

You should also see the request logged to the container's stdout in step 10.

### 11. Stream logs

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

### 12. Inspect the app

```powershell
.\minip.exe apps info hello
```

Shows status, live container state (`running`, `exited`, etc.), public URL (`https://hello.<BASE_DOMAIN>` — served by Caddy), and recent deployments (`running`, `superseded`, `failed`, `rolled_back`).

### 13. Test health checks

For a faster local check, add these values to `.env` and restart the API:

```env
HEALTH_CHECK_INTERVAL=3s
DEPLOY_READY_TIMEOUT=60s
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

### 14. Roll back to a previous deployment

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

### 15. Clean up

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
POST   /apps/:name/stop               → 204   (stops runtime and route; preserves app configuration and history)
GET    /apps/:name/metrics            → AppMetrics snapshot from Docker + deployment history
GET    /apps/:name/domains             → []CustomDomain
POST   /apps/:name/domains             { hostname } → CustomDomain (pending DNS verification)
POST   /apps/:name/domains/:id/verify  → CustomDomain (verified/active after DNS + route validation)
DELETE /apps/:name/domains/:id         → 204

POST   /apps/:name/deployments        multipart source=<tar> → 202 Deployment (build runs in background)
POST   /apps/:name/deployments/git    { branch? } → 202 Deployment (clones the connected public repository)
GET    /apps/:name/deployments        → []Deployment
GET    /apps/:name/deployments/:id    → Deployment
GET    /deployments?page=1&per_page=50&app=&status=
                                       → { items, page, per_page, total }

PUT    /apps/:name/source/git         { repository, branch?, build_context?, dockerfile_path? } → GitSource
GET    /apps/:name/source/git         → GitSource
DELETE /apps/:name/source/git         → 204

GET    /apps/:name/env                → []{ key, updated_at }        (values never returned)
PUT    /apps/:name/env/:key           { value } → 204                (AES-256-GCM at rest)
DELETE /apps/:name/env/:key           → 204

POST   /apps/:name/rollback           { deployment_id } → Deployment (restored)
POST   /apps/:name/deployments/:id/retry  → Deployment (Git deployment only)
POST   /apps/:name/deployments/:id/cancel → Deployment (pending/building only)

WS     /apps/:name/logs?follow=true&tail=100     (active deployment, or latest failed deployment with logs)
       frames: { "ts": "...", "stream": "stdout|stderr", "line": "..." }
WS     /apps/:name/metrics/stream                (active container; authenticated)
       frames: { "type": "metrics", "ts": "...", "runtime": { ... } }
GET    /apps/:name/deployments/:id/logs?after=0&limit=500 (persisted build events)
```

Rollback is synchronous: the API responds after Docker starts the selected image and Caddy updates its route.

Deployment status transitions:

```
pending → building → running    (happy path)
pending → cancel_requested → cancelled
building → cancel_requested → cancelled
                   ↘ failed     (build or start failed)
running           → superseded  (replaced by a newer deploy)
                  → rolled_back (intentional rollback — phase 5)
                  → failed      (container exited, dead, or missing — health check)
                  → stopped     (manually stopped; application data is preserved)
```

A stopped deployment can be started again from the dashboard with **Reativar**, which reuses the retained image through the rollback path.

## Dashboard navigation

After login, the dashboard opens the project directory instead of selecting the most recent application automatically.

- `/dashboard/projects` lists every project and its current state.
- `/dashboard/projects/:name` contains overview, deployment history, logs, and configuration tabs.
- `/dashboard/deployments` lists deployments from every project with project/status filters and pagination.
- `/dashboard/logs` provides a dedicated live console with a project selector. Scrolling up pauses automatic following; **Ir para o final** resumes it.
- `/dashboard/metrics` provides a project selector and live Docker-style charts for CPU, memory, network, and disk. The page keeps the latest 120 samples in browser memory and reconnects automatically when the WebSocket is interrupted.
- The project **Configurações** tab manages custom domains and encrypted environment variables.

Theme and logout controls live in the top navigation. The dark theme uses a near-black base and translucent blur across navigation and operational surfaces. Very low-opacity, solid ambient forms are restricted to the page background so the glass remains perceptible; buttons, text, cards, and controls do not use decorative gradients.

## CLI

```bash
minip login                            # host + username + password → OS user config directory
minip apps create <name>
minip apps list
minip apps info <name>                 # status + container state + public URL + recent deployments
minip apps retry <name> <deployment-id> # retry a failed/cancelled Git deployment
minip apps cancel <name> <deployment-id> # cancel a pending/building deployment
minip apps connect-github <name> --repo owner/repository
minip apps connect-github <name> --repo owner/repository --branch main --context services/api --dockerfile Dockerfile
minip apps github-installations
minip apps github-repositories <installation-id>
minip apps connect-github <name> --installation <id> --repository-id <id>
minip apps auto-deploy <name> on
minip apps git-source <name>            # connected repository and build settings
minip apps disconnect-github <name>
minip deploy [path] --app <name>       # tarball + upload (default path = .)
minip deploy --git --app <name> --wait # clone and deploy the connected public GitHub repository
minip deploy --git --app <name> --branch release/v1 --wait
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

### Public GitHub deployment contract

Phase 9.1 accepts public repositories hosted on `github.com`. The API stores the canonical `owner/repository` identifier and constructs the clone URL itself; arbitrary Git hosts and URLs are rejected. A `Dockerfile` is required.

- `--context` defaults to `.` and is relative to the repository root.
- `--dockerfile` defaults to `Dockerfile` and is relative to the selected build context.
- Absolute paths and path traversal (`..`) are rejected.
- The clone is shallow and single-branch. Repository metadata (SHA, author, commit message, branch) is recorded on the deployment.
- Git submodules and Git LFS objects are not fetched in Phase 9.1.
- Tar uploads remain supported and use the root `Dockerfile` as before.

Example for a monorepo whose application lives in `services/api`:

```powershell
.\minip.exe apps connect-github hello --repo owner/repository --branch main --context services/api --dockerfile docker/Dockerfile
.\minip.exe deploy --git --app hello --wait
.\minip.exe apps info hello
```

### Private repositories with a GitHub App

Phase 9.2 supports private repositories through an instance-owned GitHub App. MiniPaaS stores only installation and repository identifiers. It requests a repository-scoped installation token when listing or cloning and never persists or returns that token.

Create a GitHub App with:

- Setup URL: `http://localhost:8080/integrations/github/callback` (replace the API origin outside local development).
- Repository permission **Contents: Read-only**. Metadata read access is included by GitHub.
- For auto-deploy, enable the webhook, subscribe only to **Push**, and use `<PUBLIC_API_URL>/integrations/github/webhook` as its URL.
- Installation limited to the accounts and repositories that this MiniPaaS instance may deploy.

Generate a private key for the App, store the PEM outside the repository, and set `GITHUB_APP_ID`, `GITHUB_APP_SLUG`, and `GITHUB_APP_PRIVATE_KEY_PATH`. Restart the API, open a project's **Configurações → GitHub → GitHub App**, install/authorize the App, and select the repository.

The CLI can reuse an installation created through the dashboard:

```powershell
.\minip.exe apps github-installations
.\minip.exe apps github-repositories 12345678
.\minip.exe apps connect-github hello --installation 12345678 --repository-id 987654321 --branch main
.\minip.exe deploy --git --app hello --wait
```

The browser callback requires an authenticated MiniPaaS dashboard session and a signed, short-lived state value. This Phase 9.2 flow assumes the GitHub App is private and controlled by the same administrator as the self-hosted MiniPaaS instance.

### Signed push auto-deploy

Phase 10.1 can create a deployment automatically when GitHub sends a push for the repository and branch configured on an application. Configure a random, high-entropy webhook secret in the GitHub App and expose the API through HTTPS; GitHub cannot deliver events directly to `localhost`.

```env
GITHUB_WEBHOOK_SECRET=use-a-long-random-value
```

In the GitHub App settings:

- Set **Webhook URL** to `https://your-public-api.example/integrations/github/webhook`.
- Set **Webhook secret** to exactly the same value as `GITHUB_WEBHOOK_SECRET`.
- Keep **Active** enabled and subscribe to the **Push** event.

During local development, GitHub cannot call `localhost` directly. With the API running on port `8080`, open another terminal and start a temporary HTTPS tunnel using the Wrangler already installed by the dashboard:

```powershell
cd dashboard
npx.cmd wrangler tunnel quick-start http://localhost:8080
```

Wrangler prints a temporary public URL similar to:

```text
https://random-words.trycloudflare.com
```

Append the MiniPaaS webhook path and use the result as the GitHub App **Webhook URL**:

```text
https://random-words.trycloudflare.com/integrations/github/webhook
```

Keep the tunnel terminal open while testing. A quick-tunnel URL changes whenever the process is restarted, so update the GitHub App Webhook URL after starting a new tunnel. The **Setup URL** remains `http://localhost:8080/integrations/github/callback`, because it is opened by the local browser rather than called by GitHub's servers.

After restarting the API, enable auto-deploy from **Projeto → Configurações → GitHub** or with:

```powershell
.\minip.exe apps auto-deploy hello on
```

MiniPaaS validates `X-Hub-Signature-256` with HMAC-SHA256 before processing the body, deduplicates deliveries using `X-GitHub-Delivery`, and only starts a deployment when repository, installation, and branch all match. Deleting a branch, pushing another branch, or replaying the same delivery does not create another deployment. Releases created from a push expose `trigger_type=webhook` and their GitHub delivery ID.

### Persistent build logs (Phase 10.2)

Each deployment now records ordered build events in PostgreSQL. Events include the source clone, Docker build output, container start, health-check handoff, route publication, cleanup, and failures. Runtime logs remain available through the existing WebSocket stream; build history is independent and can be opened after a deployment has finished.

The API endpoint is:

```text
GET /apps/:name/deployments/:id/logs?after=0&limit=500
```

In the dashboard, open **Deployments → Logs de build** for a release. The CLI exposes the same history without starting a live stream:

```powershell
.\minip.exe logs hello --deployment <deployment-id>
```

Apply migration 010 before starting the API:

```powershell
cd api
go run ./cmd/migrate up
```

### Retry and cancellation (Phase 10.3)

Deployments can be cancelled while they are pending or building. Cancellation is persisted, interrupts the active clone/build context when the API process is running, and is finalized as `cancelled` without marking the application as failed. A deployment that is already running should be stopped through the existing application stop action.

Retries create a new deployment linked to the previous one through `retry_of` and increment `attempt`. They are currently available for Git deployments because the configured repository can be cloned again; manual tar uploads are not retained as rebuild artifacts.

Apply migration 011 before starting the API:

```powershell
cd api
go run ./cmd/migrate up
```

### Zero-downtime deployments (Phase 11)

Deployments now use a candidate rollout when Docker and the persistent deployment store support the Phase 11 capabilities:

1. Build the new image while the current container keeps serving traffic.
2. Start the new container with a unique candidate name and port.
3. Wait up to `DEPLOY_READY_TIMEOUT` for the candidate to be running and accepting TCP connections.
4. Replace the Caddy route atomically, then promote the candidate in PostgreSQL.
5. Stop and remove the previous container only after the route and deployment record point to the candidate.

If the candidate exits, fails readiness, the route cannot be switched, or the promotion is interrupted, the candidate is removed and the previous release remains active. Deployments are serialized per application during the rollout portion so concurrent builds cannot orphan an already-promoted container.

Candidate container and port metadata is persisted by migration 012. On API startup, any candidate left by an interrupted process is removed and the route is restored to the last committed running deployment.

Apply the migration before starting the API:

```powershell
cd api
go run ./cmd/migrate up
```

Readiness currently checks the published TCP port, so the application must bind its configured internal port (8080 by default) and listen on all interfaces inside the container. HTTP path checks and per-application startup policies remain future configuration work.

### Custom domains (Phase 12.1)

Applications can have one or more custom hostnames in addition to the default `<app>.<BASE_DOMAIN>` URL. The dashboard exposes this under **Configurações → Domínios customizados** and the API provides:

```text
GET    /apps/:name/domains
POST   /apps/:name/domains                 { "hostname": "api.example.com" }
POST   /apps/:name/domains/:id/verify
DELETE /apps/:name/domains/:id
```

Create the DNS record before verifying the domain. Records start as `pending`; after DNS resolution and route activation they become `verified` or `active`, and a failed check is recorded as `error`. MiniPaaS resolves the hostname, optionally compares the result with `PUBLIC_IP`, and then creates the Caddy route. Caddy handles HTTPS automatically when the hostname points to the server and ports 80/443 are reachable. Set `PUBLIC_IP` in production for strict ownership validation; when it is empty, local development accepts any hostname that resolves.

For a local-only test, `nip.io` maps a hostname to an IP encoded in the name. With the API and Caddy running on the same machine, add `api.127.0.0.1.nip.io` in **Configurações → Domínios customizados**, verify it, and test the route:

```powershell
Resolve-DnsName api.127.0.0.1.nip.io
curl.exe -i -H "Host: api.127.0.0.1.nip.io" http://localhost/
```

The application must be listening on the internal port expected by its Dockerfile/runtime (`8080` by default). For example, a `PORT=9000` environment variable makes the container listen on a different port and results in a Caddy `502 Bad Gateway` until the application is changed back to `PORT=8080` or the runtime port contract is updated.

Apply migration 013 before starting the API:

```powershell
cd api
go run ./cmd/migrate up
```

### Operational metrics (Phase 12.2)

Project details expose a lightweight, on-demand metrics snapshot at:

```text
GET /apps/:name/metrics
```

The response includes current container CPU and memory usage, uptime, restart count, deployment success/failure summary, average deployment duration, and recent health-check failures. The dashboard renders these values under **Visão geral → Métricas operacionais**. The overview requests a fresh snapshot during its normal project refresh (currently every five seconds); it is not a historical time-series. Metrics are collected directly from Docker when requested; no Prometheus service or additional database migration is required.

### Real-time metrics (Phase 12.3)

The dedicated **Métricas** page opens an authenticated WebSocket stream for the selected application:

```text
GET /apps/:name/metrics/stream
```

The backend shares one Docker stats stream per active container between connected viewers. A stream starts when the first viewer subscribes and is cancelled when the last viewer leaves, so opening the same metrics page in multiple tabs does not create one Docker stats stream per tab. Docker emits samples at roughly one-second intervals; the exact cadence is controlled by the Docker Engine.

The dashboard keeps a rolling window of the latest 120 samples for CPU, memory, network, and disk charts, reconnects automatically, and loads `GET /apps/:name/metrics` first as the initial snapshot/fallback. No time-series samples are persisted in PostgreSQL in this phase.

The WebSocket frame has this shape:

```json
{
  "type": "metrics",
  "ts": "2026-08-13T20:00:00Z",
  "runtime": {
    "container_id": "a1b2c3d4e5f6",
    "state": "running",
    "restart_count": 0,
    "uptime_seconds": 600,
    "cpu_percent": 0.12,
    "memory_usage_bytes": 12400000,
    "memory_limit_bytes": 4026531840,
    "memory_percent": 0.31,
    "network_rx_bytes": 1070,
    "network_tx_bytes": 264,
    "block_read_bytes": 5720000,
    "block_write_bytes": 0,
    "pids": 2
  }
}
```

To validate the live stream locally, keep the API and dashboard running, open **Métricas**, select an application with a running container, and generate temporary CPU load in another PowerShell terminal:

```powershell
docker ps --filter "name=minipaas-nodetest" --format "{{.Names}}"
docker exec <container-name> node -e "const end=Date.now()+30000; while(Date.now()<end){}"
```

Replace `<container-name>` with the value returned by the first command. The CPU card and chart should change during the 30-second workload. The dashboard updates from the WebSocket stream, while the project overview continues to use the snapshot endpoint.

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
│   │   ├── ws/                        # WebSocket logs + shared metrics streams
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
| `PUBLIC_IP` | no | — | Public IPv4/IPv6 used to validate custom-domain DNS. Leave empty only for local development. |
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
| `DEPLOY_READY_TIMEOUT` | no | `60s` | Maximum time a new candidate container may take to accept TCP connections before the rollout is failed and the current release remains active. |
| `MAX_DEPLOY_SIZE_MB` | no | `100` | Maximum accepted deployment source upload size in MiB. Requests over the limit receive HTTP 413. |
| `MAX_REPOSITORY_SIZE_MB` | no | `250` | Maximum unpacked build-context size accepted from a Git repository. |
| `GIT_CLONE_TIMEOUT` | no | `10m` | Maximum time allowed for a GitHub clone. The Docker build keeps its existing lifecycle. |
| `GITHUB_APP_ID` | conditional | — | Numeric App ID used for private repository access. Must be set with the slug and private-key path. |
| `GITHUB_APP_SLUG` | conditional | — | Slug from the GitHub App settings page. |
| `GITHUB_APP_PRIVATE_KEY_PATH` | conditional | — | Absolute path to the GitHub App private-key PEM. Keep this file outside the repository. |
| `GITHUB_WEBHOOK_SECRET` | no | — | Enables signed GitHub push webhooks and auto-deploy. Must match the secret configured on the GitHub App. |
| `RESTART_POLICY` | no | `on-failure` | Docker restart policy for app containers: `no`, `always`, `on-failure`, or `unless-stopped`. |
| `RESTART_MAX_RETRIES` | no | `3` | Maximum retries used by Docker's `on-failure` restart policy. |
| `DASHBOARD_ORIGIN` | no | `http://localhost:3000` | Browser origin allowed to authenticate with the API dashboard cookie. |
| `RATE_LIMIT_WINDOW` | no | `1m` | Fixed window used by the in-memory protection for sensitive endpoints. |
| `AUTH_RATE_LIMIT` | no | `10` | Maximum login and web-login requests per client address during the rate window. |
| `WEBHOOK_RATE_LIMIT` | no | `120` | Maximum GitHub webhook requests per client address during the rate window. |
| `CONTAINER_MEMORY_LIMIT_MB` | no | `0` | Optional memory cap applied to every app container, in MiB. `0` means unlimited. |
| `CONTAINER_NANO_CPUS` | no | `0` | Optional Docker NanoCPUs cap applied to every app container. `0` means unlimited. |
| `CONTAINER_PIDS_LIMIT` | no | `0` | Optional maximum number of processes per app container. `0` means unlimited. |
| `LOG_LEVEL` | no | `info` | `debug` \| `info` \| `warn` \| `error` |

Authentication (`/auth/login` and `/auth/web-login`) and the GitHub webhook endpoint return `429 Too Many Requests` with `Retry-After` when their fixed-window limit is reached. The limiter is intentionally in memory and per API process; a multi-instance deployment must move this state to a shared store. The key uses the direct peer address and does not trust arbitrary `X-Forwarded-For` headers.

Every API response includes an `X-Request-ID` UUID. A valid incoming UUID is preserved for correlation; malformed values are replaced. The same ID is included in the structured HTTP request log entry.

When configured, `CONTAINER_MEMORY_LIMIT_MB`, `CONTAINER_NANO_CPUS`, and `CONTAINER_PIDS_LIMIT` are applied to both new deployments and rollbacks. For NanoCPUs, `1_000_000_000` represents one CPU. A value of `0` leaves that resource unlimited; per-application limits and build-time isolation are still future work.

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

# Dashboard (from dashboard/)
npm test
npm run lint

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
- `docker` / `ws` — CPU calculation, network/disk aggregation, and shared metrics-stream subscription coverage
- `health` — exited/missing containers transition their deployment and app to `failed`; current container state lookup
- `cli/tarball` — kept/skipped paths, slash-normalized entries
- `store/postgres` (integration tag) — app persistence, uniqueness constraints, status and public URL updates against a real PostgreSQL instance

`-race` needs CGO (not enabled on Windows by default); CI runs it on Linux. GitHub Actions also vets the API, runs the CLI and dashboard checks, applies the migrations, and runs the PostgreSQL integration test.

### Releases

Pushing a tag in the `v*` format (for example, `v0.1.0`) starts the release workflow. It builds the API server, migration command, and CLI for Linux amd64, macOS amd64/arm64, and Windows amd64, then creates a GitHub release with those binaries.

## License

MIT
