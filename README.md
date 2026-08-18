# MiniPaaS

[Leia esta documentação em português (Brasil)](README.pt-BR.md)

A self-hosted deployment control plane inspired by Render and Railway. MiniPaaS deploys Dockerfile-based applications from a local archive or GitHub, publishes them behind Caddy with automatic HTTPS, and exposes the complete lifecycle through both a CLI and a dashboard.

This is a finished educational portfolio project: the scope is intentionally focused on one host and a small operational surface, while still exercising the backend patterns found in real infrastructure products. The implementation includes deployments, GitHub integration, zero-downtime promotion, encrypted environment variables, live logs and metrics, multi-user ownership, API tokens, capacity limits, and a visible deployment queue.

## What this project demonstrates

- **Control-plane architecture:** Gin handlers, application services, domain types, PostgreSQL stores, versioned migrations, and generated `sqlc` queries are kept in separate layers.
- **Container orchestration:** Docker image builds, candidate containers, readiness checks, restart policies, cleanup, rollback, and safe route promotion are coordinated without Kubernetes.
- **Asynchronous workflows:** deployments run in the background with context cancellation, bounded Docker build concurrency, FIFO queueing, retry, and cancellation states.
- **Secure configuration:** environment values are encrypted with AES-256-GCM, credentials use bcrypt, dashboard sessions use HTTP-only cookies, and automation uses scoped `mpat_` tokens stored only as hashes.
- **Real-time operation:** WebSockets stream runtime logs and shared Docker metrics; persistent build events keep completed deployment history inspectable.
- **Multi-user boundaries:** applications, deployments, logs, metrics, GitHub installations, domains, and audit events are owner-scoped.
- **Infrastructure integration:** Caddy's Admin API manages routes at runtime, while GitHub Apps and signed webhooks enable private repositories and auto-deploy.
- **Operational safety:** request IDs, audit events, rate limiting, readiness probes, health checks, resource limits, application quotas, capacity snapshots, and orphan-container reconciliation are included.

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

The same HTTP API is the contract for the dashboard, CLI, GitHub webhooks, and CI/CD automation. This keeps authentication, ownership, validation, and deployment behavior consistent across every entry point.

## Feature map

| Area | Included capabilities |
|---|---|
| Deployments | Tar uploads, public/private GitHub repositories, Dockerfile validation, build logs, retries, cancellation, rollback, zero-downtime candidate promotion |
| Runtime | Docker restart policies, TCP readiness checks, automatic subdomains, Caddy route lifecycle, custom domains and HTTPS |
| Observability | Live stdout/stderr logs, persisted build events, Docker metrics streams, project metrics, health-check failures, request IDs and audit trail |
| Security | JWT login, HTTP-only dashboard session, bcrypt passwords, ownership isolation, AES-256-GCM environment values, scoped API tokens, rate limits |
| Operations | FIFO build queue, per-user application quota, CPU/memory/PID caps, capacity endpoint, startup reconciliation and safe destructive deletion |
| Interfaces | Cobra CLI, dashboard routes for projects/deployments/logs/metrics/account, GitHub App installation flow, CI/CD examples |

## Quick start

Prerequisites: Go 1.26+, Docker Desktop / Engine, Caddy 2.x, [sqlc](https://sqlc.dev).

```bash
git clone https://github.com/guijoazeiro/minipaas
cd minipaas
# generate a real key: openssl rand -hex 32 → paste into ENCRYPTION_KEY
```

Copy the example configuration for your shell:

```bash
# Linux, macOS, and Git Bash
cp .env.example .env
```

On first startup the API seeds the admin user from `ADMIN_USERNAME` / `ADMIN_PASSWORD` (see `.env.example`).

## End-to-end walkthrough

Full path from empty machine to `minip logs hello -f` streaming live output. The commands use POSIX shell syntax and run on Linux, macOS, and Git Bash on Windows.

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
On startup, the API checks that Caddy's HTTP app and `srv0` server exist. If Caddy was started with an empty configuration, the API bootstraps them before publishing the first application route.

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

```bash
cd dashboard
npm install
npm run dev
```

Open `http://localhost:3000`. The dashboard uses an HTTP-only cookie created by `POST /auth/web-login`; it does not expose the login token to browser JavaScript. The API must keep `DASHBOARD_ORIGIN=http://localhost:3000` (or the exact origin you use). If the API runs on another address, create `dashboard/.env.local` with:

```env
NEXT_PUBLIC_MINIPAAS_API_URL=http://localhost:8080
```

### 6. Build the CLI

New terminal:

```bash
cd cli
go build -o minip .
```

### 7. Log in

```bash
./minip login
```

Answers when prompted:
- `host` — press Enter to accept `http://localhost:8080`
- `username` — `admin` (whatever you put in `ADMIN_USERNAME`)
- `password` — `admin` (from `ADMIN_PASSWORD`)

Success prints `logged in as admin`. The token is stored in the OS user config directory: `%AppData%\minip\config.json` on Windows and `~/.config/minip/config.json` on Linux/macOS. Subsequent commands read it from there.

### 8. Create an app and set an env var

```bash
./minip apps create hello
./minip env set hello GREETING="hello from minipaas"
./minip env list hello
```

`env list` shows the key + `updated_at`. Values are **never** returned by the API. At-rest check:

```bash
docker compose exec postgres psql -U postgres -d minipaas -c "SELECT key, encode(value,'hex') FROM env_vars;"
```

Column `value` is always ciphertext — never plaintext.

### 9. Deploy the sample app

The repo ships with a minimal Node.js app under `hello-world/` that logs a heartbeat every second (stdout) and an "even beat" every 3 seconds (stderr).

```bash
./minip deploy ../hello-world --app hello --wait
```

Output ends with something like `running on host port 57123`. That's the host port Docker bound to the container's `:8080`.

### 10. Sanity-check the container

```bash
curl localhost:<the port from step 9>
# hello
```

You should also see the request logged to the container's stdout in step 10.

### 11. Stream logs

Tail the last 100 lines (then exits normally):

```bash
./minip logs hello
```

Follow mode (streams until `Ctrl+C`):

```bash
./minip logs hello -f
```

While `-f` is running, hit the container again from another terminal:

```bash
curl localhost:<port>/foo
```

The `GET /foo` line appears live, interleaved with the heartbeat. To show just one stream:

```bash
./minip logs hello -f 2>/dev/null  # stdout only
./minip logs hello -f 1>/dev/null  # stderr only (the event lines)
```

`-f` first prints the selected tail and then keeps streaming while the container produces output. If a container has crashed, `minip logs` still retrieves the saved output from the latest failed deployment for diagnosis.

### 12. Inspect the app

```bash
./minip apps info hello
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

```bash
container="$(docker ps --filter "name=minipaas-hello" --format "{{.Names}}" | head -n 1)"
docker inspect --format '{{.HostConfig.RestartPolicy.Name}} max={{.HostConfig.RestartPolicy.MaximumRetryCount}}' "$container"
# on-failure max=3
```

To simulate a crash, kill the container a few times in quick succession:

```bash
for i in 1 2 3 4; do
  docker kill "$container"
  sleep 1
done
```

After the restart limit is exhausted, wait at least one health-check interval and inspect the app:

```bash
./minip apps info hello
docker inspect --format '{{.State.Status}} restartCount={{.RestartCount}}' "$container"
```

Expected result: the app and deployment become `failed`, and `container: exited` appears in `apps info`. You can still run `./minip logs hello` to inspect the crash output.

### 14. Roll back to a previous deployment

Make a visible change to `hello-world/server.js` (e.g. change the heartbeat message), redeploy, then roll back:

```bash
./minip deploy ../hello-world --app hello --wait     # creates v2, v1 becomes superseded
./minip rollback hello                               # interactive picker; select v1
./minip logs hello -f                                # container now runs the old code
./minip apps info hello                              # v1 back to running, v2 -> rolled_back
```

Non-interactive form:

```bash
./minip rollback hello --to <deployment-id>
```

Rollback reuses the target's cached Docker image, so it takes seconds — no rebuild. The target is started as a candidate and checked before the route changes; the previous container is stopped only after the rollback is promoted successfully.

### 15. Clean up

```bash
./minip apps list                   # inspect remaining apps
# From the api terminal: Ctrl+C to stop the server
# From the caddy terminal: Ctrl+C to stop caddy
docker compose down                     # stop Postgres; keep the volume
# Or use `docker compose down -v` instead to erase the database volume.
```

To remove an app and its deployment history, require an explicit confirmation:

```bash
./minip apps delete hello --yes
```

## API

All protected endpoints require `Authorization: Bearer <token>` (or the dashboard HTTP-only cookie). The public endpoints are `/health`, `/ready`, `/auth/login`, `/auth/web-login`, `/auth/register`, `/auth/logout`, and the signed GitHub webhook receiver.

```
GET    /health                        → { status: "ok" } (database health check)
GET    /ready                         → dependency readiness for database, Docker, and Caddy
POST   /auth/login                    { username, password } → { token, expires_at }
POST   /auth/register                 { username, password } → User

GET    /me                            → User (without password hash)
PATCH  /me                            { username } → User
PATCH  /me/password                   { current_password, new_password } → 204
POST   /me/tokens                     { name, scopes?, expires_at? } → token (raw secret returned once; session only)
GET    /me/tokens                     → []APIToken (metadata only; session only)
DELETE /me/tokens/:id                 → 204 (session only)

POST   /apps                          { name } → App
GET    /apps                          → []App
GET    /capacity                      → current app/build capacity and queue counters
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
GET    /audit?limit=50&offset=0         → recent mutating request audit events

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

Registration accepts usernames with 3–64 characters (`A–Z`, `a–z`, digits, `_`, `-`, or `.`) and passwords with at least 8 characters. The dashboard sends the user to login after a successful registration; changing the username or password renews the browser session cookie.

Applications created through an authenticated request are owned by that user. Application lists, project details, deployments, logs, environment variables, custom domains, metrics, and audit events are scoped to the authenticated owner. Existing applications are assigned to the first configured user by migration `015`; internal jobs and legacy system contexts can still access unowned records for reconciliation and webhook processing.

### API tokens for automation and CI/CD

The API also accepts personal, opaque automation tokens with the `mpat_` prefix. A token contains at least 256 bits of cryptographically secure randomness. MiniPaaS stores only its SHA-256 hash and a short display prefix; the raw secret is returned exactly once when the token is created and cannot be recovered later.

Token management is intentionally session-only. Use the dashboard at **Conta → API tokens** or the following endpoints with the normal JWT/session cookie:

```
POST   /me/tokens       { name, scopes?, expires_at? } → token metadata + raw token (one time)
GET    /me/tokens       → token metadata (never the raw token)
DELETE /me/tokens/:id   → 204 (revoke immediately)
```

The available scopes are:

- `read` — read projects, deployments, logs, metrics, capacity, Git sources, environment-key names, domains, audit events, and GitHub installation/repository metadata;
- `deploy` — upload or trigger deployments, retry/cancel deployments, and roll back;
- `manage` — create, stop, and delete applications, configure Git sources, environment values, and custom domains.

Scopes are explicit and deny-by-default for API tokens. A token is always restricted to the owning MiniPaaS user's resources and cannot grant more access than that user has. Profile/credential changes, API-token management, and GitHub App installation/authorization remain session-only operations.

For CLI and CI/CD use, set `MINIPAAS_TOKEN`. It takes precedence over the token saved by `minip login` in `config.json`, and a value read from the environment is never written back automatically:

```bash
export MINIPAAS_HOST="http://localhost:8080"
export MINIPAAS_TOKEN="mpat_..."
./minip apps list
./minip deploy --git --app hello --wait
```

Create and revoke tokens from the CLI while authenticated with an existing session:

```bash
./minip tokens create ci-deploy --scope read --scope deploy --expires-at 2027-01-01T00:00:00Z
./minip tokens list
./minip tokens revoke <token-id> --yes
```

The `tokens create` output contains a warning that the secret is shown once. Store it in a password manager or the CI/CD secret store; do not paste it into source control, issue text, build logs, or command-line arguments that may be retained by a runner.

GitHub Actions example for an application already connected to its GitHub repository in MiniPaaS:

```yaml
name: MiniPaaS deploy

on:
  push:
    branches: [main]

jobs:
  deploy:
    runs-on: ubuntu-latest
    env:
      MINIPAAS_HOST: ${{ secrets.MINIPAAS_HOST }}
      MINIPAAS_TOKEN: ${{ secrets.MINIPAAS_TOKEN }}
      MINIPAAS_APP: hello
    steps:
      - name: Trigger deployment
        run: |
          curl --fail-with-body --silent --show-error \
            --request POST \
            --header "Authorization: Bearer ${MINIPAAS_TOKEN}" \
            --header "Content-Type: application/json" \
            "${MINIPAAS_HOST}/apps/${MINIPAAS_APP}/deployments/git"
```

The token used by this workflow needs at least `deploy`; the connected GitHub source and branch determine what is cloned. A generic CI/CD runner can use the same bearer header, for example to list deployments with `read`:

```bash
curl --fail-with-body --silent --show-error \
  --header "Authorization: Bearer ${MINIPAAS_TOKEN}" \
  "${MINIPAAS_HOST}/apps/${MINIPAAS_APP}/deployments"
```

Never include the token in `echo`/debug output. Revoking it invalidates subsequent requests immediately; expiration is checked on every authentication attempt.

Deleting an application is intentionally destructive. `DELETE /apps/:name` removes its Caddy routes and runtime before deleting the database row; if runtime cleanup fails, the database row is preserved so the orphan can be recovered safely. The dashboard exposes this action in the project's **Zona de perigo** after requiring the exact application name.

Rollback is synchronous: the API responds after Docker starts the selected image and Caddy updates its route.

Deployment status transitions:

```
pending → building → running    (happy path)
pending → cancel_requested → cancelled
building → cancel_requested → cancelled
                   ↘ failed     (build or start failed)
running           → superseded  (replaced by a newer deploy)
                  → rolled_back (intentional rollback)
                  → failed      (container exited, dead, or missing — health check)
                  → stopped     (manually stopped; application data is preserved)
```

A stopped deployment can be started again from the dashboard with **Reativar**, which reuses the retained image through the rollback path.

## Dashboard navigation

After login, the dashboard opens the project directory instead of selecting the most recent application automatically.

- `/dashboard/projects` lists every project and its current state.
- Project status is revalidated automatically every five seconds without a full page reload. Use **Atualizar** in the project directory header for an immediate refresh; the current route, scroll position, and selection are preserved.
- `/dashboard/projects/:name` contains overview, deployment history, logs, and configuration tabs.
- `/dashboard/deployments` lists deployments from every project with project/status filters and pagination.
- `/dashboard/logs` provides a dedicated live console with a project selector. Scrolling up pauses automatic following; **Ir para o final** resumes it.
- `/dashboard/metrics` provides a project selector and live Docker-style charts for CPU, memory, network, and disk. The page keeps the latest 120 samples in browser memory and reconnects automatically when the WebSocket is interrupted.
- `/dashboard/account` shows the authenticated user's profile, credential controls, and GitHub App installations. A user can connect another GitHub account or organization without sharing installations with other MiniPaaS users.
- The project **Configurações** tab manages custom domains and encrypted environment variables.

Theme and logout controls live in the top navigation. The interface uses a terminal-green identity in both themes. The dark theme uses a near-black base and translucent blur across navigation and operational surfaces. Toast notifications appear in a fixed glass viewport so they do not move page content; success messages dismiss automatically and errors remain until closed. Very low-opacity, solid ambient forms are restricted to the page background so the glass remains perceptible; buttons, text, cards, and controls do not use decorative gradients.

## CLI

```bash
minip login                            # host + username + password → OS user config directory
minip apps create <name>
minip apps list
minip apps info <name>                 # status + container state + public URL + recent deployments
minip apps delete <name> --yes         # stop runtime, remove routes, and delete app history
minip apps retry <name> <deployment-id> # retry a failed/cancelled Git deployment
minip apps cancel <name> <deployment-id> # cancel a pending/building deployment
minip apps connect-github <name> --repo owner/repository
minip apps connect-github <name> --repo owner/repository --branch main --context services/api --dockerfile Dockerfile
minip apps github-installations       # installations autorizadas pela conta atual
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
minip tokens create <name>             # create; raw secret is printed once
minip tokens list                       # metadata only: prefix, scopes, expiry, usage, status
minip tokens revoke <id> --yes          # revoke immediately
```

Host resolution order: `--host` flag → `MINIPAAS_HOST` env → saved config → `http://localhost:8080`. Authentication token resolution is `MINIPAAS_TOKEN` env → token saved by `minip login` in `config.json`; the environment value is never persisted automatically.

### Public GitHub deployment contract

Phase 9.1 accepts public repositories hosted on `github.com`. The API stores the canonical `owner/repository` identifier and constructs the clone URL itself; arbitrary Git hosts and URLs are rejected. A `Dockerfile` is required.

- `--context` defaults to `.` and is relative to the repository root.
- `--dockerfile` defaults to `Dockerfile` and is relative to the selected build context.
- Absolute paths and path traversal (`..`) are rejected.
- The clone is shallow and single-branch. Repository metadata (SHA, author, commit message, branch) is recorded on the deployment.
- Git submodules and Git LFS objects are not fetched in Phase 9.1.
- Tar uploads remain supported and use the root `Dockerfile` as before.

Example for a monorepo whose application lives in `services/api`:

```bash
./minip apps connect-github hello --repo owner/repository --branch main --context services/api --dockerfile docker/Dockerfile
./minip deploy --git --app hello --wait
./minip apps info hello
```

### Private repositories with a GitHub App

Phase 9.2 supports private repositories through an instance-owned GitHub App. Each MiniPaaS user can connect the App from **Conta → Contas do GitHub** or from the first-access onboarding prompt, then select the authorized installation in a project. MiniPaaS stores the installation owner and exposes only that user's installations. It requests a repository-scoped installation token when listing or cloning and never persists or returns that token.

Create a GitHub App with:

- Setup URL: `http://localhost:8080/integrations/github/callback` (replace the API origin outside local development).
- Repository permission **Contents: Read-only**. Metadata read access is included by GitHub.
- For auto-deploy, enable the webhook, subscribe only to **Push**, and use `<PUBLIC_API_URL>/integrations/github/webhook` as its URL.
- Installation limited to the accounts and repositories that this MiniPaaS instance may deploy.

Generate a private key for the App, store the PEM outside the repository, and set `GITHUB_APP_ID`, `GITHUB_APP_SLUG`, and `GITHUB_APP_PRIVATE_KEY_PATH`. Restart the API, open **Conta → Contas do GitHub** (or a project's **Configurações → GitHub → GitHub App**), install/authorize the App, and select the repository. If multiple GitHub accounts or organizations must install the same App, change the GitHub App visibility to public; a private App can only be installed by its owning account or organization.

The CLI can reuse an installation created through the dashboard:

```bash
./minip apps github-installations
./minip apps github-repositories 12345678
./minip apps connect-github hello --installation 12345678 --repository-id 987654321 --branch main
./minip deploy --git --app hello --wait
```

The browser callback requires an authenticated MiniPaaS dashboard session and a signed, short-lived state value bound to the target (account or application) and the MiniPaaS user. Installation ownership is enforced in PostgreSQL, so the same GitHub installation cannot be silently claimed by another MiniPaaS user.

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

```bash
cd dashboard
npx wrangler tunnel quick-start http://localhost:8080
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

```bash
./minip apps auto-deploy hello on
```

MiniPaaS validates `X-Hub-Signature-256` with HMAC-SHA256 before processing the body, deduplicates deliveries using `X-GitHub-Delivery`, and only starts a deployment when repository, installation, and branch all match. Deleting a branch, pushing another branch, or replaying the same delivery does not create another deployment. Releases created from a push expose `trigger_type=webhook` and their GitHub delivery ID.

### Persistent build logs (Phase 10.2)

Each deployment now records ordered build events in PostgreSQL. Events include the source clone, Docker build output, container start, health-check handoff, route publication, cleanup, and failures. Runtime logs remain available through the existing WebSocket stream; build history is independent and can be opened after a deployment has finished.

The API endpoint is:

```text
GET /apps/:name/deployments/:id/logs?after=0&limit=500
```

In the dashboard, open **Deployments → Logs de build** for a release. The CLI exposes the same history without starting a live stream:

```bash
./minip logs hello --deployment <deployment-id>
```

Apply migration 010 before starting the API:

```bash
cd api
go run ./cmd/migrate up
```

### Retry and cancellation (Phase 10.3)

Deployments can be cancelled while they are pending or building. Cancellation is persisted, interrupts the active clone/build context when the API process is running, and is finalized as `cancelled` without marking the application as failed. A deployment that is already running should be stopped through the existing application stop action.

Retries create a new deployment linked to the previous one through `retry_of` and increment `attempt`. They are currently available for Git deployments because the configured repository can be cloned again; manual tar uploads are not retained as rebuild artifacts.

Apply migration 011 before starting the API:

```bash
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

```bash
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

Hostnames are normalized with IDNA to DNS-compatible punycode before validation, so internationalized names such as `média.example.com` are stored and routed as their ASCII representation.

For a local-only test, `nip.io` maps a hostname to an IP encoded in the name. With the API and Caddy running on the same machine, add `api.127.0.0.1.nip.io` in **Configurações → Domínios customizados**, verify it, and test the route:

```bash
nslookup api.127.0.0.1.nip.io
curl -i -H "Host: api.127.0.0.1.nip.io" http://localhost/
```

The application must be listening on the internal port expected by its Dockerfile/runtime (`8080` by default). For example, a `PORT=9000` environment variable makes the container listen on a different port and results in a Caddy `502 Bad Gateway` until the application is changed back to `PORT=8080` or the runtime port contract is updated.

Apply migration 013 before starting the API:

```bash
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

To validate the live stream locally, keep the API and dashboard running, open **Métricas**, select an application with a running container, and generate temporary CPU load in another terminal:

```bash
docker ps --filter "name=minipaas-nodetest" --format "{{.Names}}"
docker exec <container-name> node -e "const end=Date.now()+30000; while(Date.now()<end){}"
```

Replace `<container-name>` with the value returned by the first command. The CPU card and chart should change during the 30-second workload. The dashboard updates from the WebSocket stream, while the project overview continues to use the snapshot endpoint.

### Capacity, deployment queue, and observability (Phase 16)

The final operational increment keeps the platform intentionally small while making its limits visible. Set `MAX_APPS_PER_USER` to enforce an ownership-aware application quota; `0` disables the quota. When the limit is reached, application creation returns `429 Too Many Requests` instead of silently overcommitting the instance.

Docker builds use a FIFO in-memory scheduler bounded by `MAX_CONCURRENT_BUILDS`. A deployment waiting for a slot remains `pending` and appears as **Na fila** in the dashboard and CLI. Cancelling a pending deployment removes its waiter safely; active builds continue to respect their build timeout and cancellation context.

The authenticated capacity snapshot is available at:

```text
GET /capacity
```

It reports the current user's application count, the configured application quota, active and queued builds, the concurrency limit, and configured container resource caps. The Projects page polls this endpoint together with application status and displays the result in **Capacidade da plataforma** without a full page reload.

The queue is deliberately process-local because MiniPaaS is designed here as a single-host learning project. PostgreSQL remains the source of truth for deployment records; distributed queues, long-term metrics retention, alerting, team roles, and multi-host scheduling are outside this final scope.

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
│   │   ├── store/                     # ownership-aware store interfaces and PostgreSQL adapters
│   │   │   └── postgres/              # concrete stores (+ sqlc/ generated code)
│   │   ├── docker/                    # Docker Engine wrapper (build, run, stop, logs)
│   │   ├── caddy/                     # Caddy Admin API wrapper (route upsert/remove)
│   │   ├── crypto/                    # AES-256-GCM cipher
│   │   ├── health/                    # periodic Docker inspection + failure detection
│   │   ├── ws/                        # WebSocket logs + shared metrics streams
│   │   ├── service/                   # business logic, queue, metrics, auth, GitHub and deployments
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
├── dashboard/                         # Next.js route-based control plane
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
| `GIT_CLONE_TIMEOUT` | no | `10m` | Maximum time allowed for a GitHub clone. |
| `BUILD_TIMEOUT` | no | `15m` | Maximum time allowed for one Docker image build before the deployment is failed. |
| `MAX_CONCURRENT_BUILDS` | no | `2` | Maximum number of Docker image builds running in parallel. Additional deployments remain queued until a slot is available. |
| `MAX_APPS_PER_USER` | no | `20` | Maximum number of applications owned by one user. `0` disables this quota. |
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
| `READINESS_TIMEOUT` | no | `3s` | Maximum time allowed for the `/ready` dependency probes. |
| `LOG_LEVEL` | no | `info` | `debug` \| `info` \| `warn` \| `error` |

Authentication (`/auth/login` and `/auth/web-login`) and the GitHub webhook endpoint return `429 Too Many Requests` with `Retry-After` when their fixed-window limit is reached. The limiter is intentionally in memory and per API process; a multi-instance deployment must move this state to a shared store. The key uses the direct peer address and does not trust arbitrary `X-Forwarded-For` headers.

Every API response includes an `X-Request-ID` UUID. A valid incoming UUID is preserved for correlation; malformed values are replaced. The same ID is included in the structured HTTP request log entry.

When configured, `CONTAINER_MEMORY_LIMIT_MB`, `CONTAINER_NANO_CPUS`, and `CONTAINER_PIDS_LIMIT` are applied to both new deployments and rollbacks. For NanoCPUs, `1_000_000_000` represents one CPU. A value of `0` leaves that resource unlimited. Docker builds are bounded by `BUILD_TIMEOUT` and share a FIFO `MAX_CONCURRENT_BUILDS` queue. The authenticated `GET /capacity` endpoint and the Projects page expose active builds, queued deployments, application counts, and configured limits. Application creation returns `429` when `MAX_APPS_PER_USER` is reached.

The API also reconciles labeled MiniPaaS containers at startup. It preserves every container referenced by a running deployment or candidate rollout and removes only stale containers with the `com.minipaas.managed=true` label. Containers created outside MiniPaaS are never touched.

### PostgreSQL backup and restore

Keep backups outside Git (the repository ignores `backups/`). With the local Compose database running:

```bash
mkdir -p backups
docker compose exec -T postgres pg_dump -U postgres -d minipaas --format=custom > "backups/minipaas-$(date +%Y%m%d-%H%M%S).dump"
```

To restore into a disposable or maintenance database, stop the API first, create the target database, and use:

```bash
docker compose exec -T postgres createdb -U postgres minipaas_restore
docker compose cp backups/minipaas-YYYYMMDD-HHMMSS.dump postgres:/tmp/minipaas.restore.dump
docker compose exec -T postgres pg_restore -U postgres -d minipaas_restore --clean --if-exists /tmp/minipaas.restore.dump
```

Verify the restore by running the migrations/status checks and the PostgreSQL integration tests against the restored DSN. A backup is only considered usable after a restore has been tested; do not overwrite the live database during a first recovery exercise.

### Audit trail

Every mutating HTTP request (`POST`, `PUT`, `PATCH`, or `DELETE`) is recorded in PostgreSQL after it completes. The record contains the authenticated user when available, route, status code, request ID, and timestamp; request bodies are never persisted, so passwords, environment values, and source contents are excluded. Authenticated operators can inspect recent entries with `GET /audit?limit=50&offset=0`.

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
go vet ./...

# CLI module (separate go.mod)
cd cli && go test ./...

# Dashboard (from dashboard/)
npm test
npm run lint
npm run build

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
- `handler/middleware` — request IDs and concurrent rate-limit enforcement
- `handler` — readiness responses, capacity snapshots, and explicit quota-exhaustion errors
- `service` — rollout serialization and safe managed-container reconciliation
- `service` — FIFO build queue ordering/cancellation, application quotas, and capacity snapshots
- `health` — exited/missing containers transition their deployment and app to `failed`; current container state lookup
- `cli/tarball` — kept/skipped paths, slash-normalized entries
- `store/postgres` (integration tag) — app persistence, uniqueness constraints, status and public URL updates against a real PostgreSQL instance

`-race` needs CGO (not enabled on Windows by default); CI runs it on Linux. GitHub Actions also vets the API, runs the CLI and dashboard checks, applies the migrations, and runs the PostgreSQL integration test.

### CI and releases

The repository includes two workflows:

- **CI** runs API vetting and race-enabled tests on Linux, applies the migrations against PostgreSQL, executes the PostgreSQL integration suite, tests the CLI, and runs dashboard lint/build tests.
- **Release** runs when a `v*` tag is pushed and publishes cross-platform API server, migration, and CLI binaries for Linux amd64, macOS amd64/arm64, and Windows amd64.

### Release artifacts

Pushing a tag in the `v*` format (for example, `v0.1.0`) starts the release workflow. It builds the API server, migration command, and CLI for Linux amd64, macOS amd64/arm64, and Windows amd64, then creates a GitHub release with those binaries.

## License

MIT
