# MiniPaaS — Future Roadmap

This document keeps the product and engineering improvements considered after the initial release. The sequence favors user value while keeping the platform centered on predictable Docker-based deployments.

## Product direction

MiniPaaS should evolve from a container executor into a complete deployment experience while preserving these principles:

- A `Dockerfile` remains the deployment contract.
- CLI, dashboard, and API expose the same capabilities.
- Deployments are reproducible and linked to an immutable source revision.
- A failed release must not replace a healthy application.
- Credentials and application secrets must never appear in responses or logs.

## Phase 9 — Deploy from GitHub

Allow an application to connect to a GitHub repository and start a deployment without manually creating or uploading a tar archive.

### MVP capabilities

- Connect a repository URL to an application.
- Select a branch, defaulting to `main`.
- Configure the build context and `Dockerfile` path for monorepos.
- Start a Git deployment from the CLI and dashboard.
- Perform a shallow clone into an isolated temporary directory.
- Resolve the requested branch or reference to an immutable commit SHA before building.
- Store the commit SHA, author, message, repository, and branch on the deployment.
- Validate that the selected build context contains the configured `Dockerfile`.
- Always remove temporary source files after success, failure, cancellation, or timeout.
- Keep tar uploads available as an alternative deployment source.

### Repository access

- Support public repositories first, without credentials.
- Add private repositories afterward.
- Prefer a GitHub App with short-lived installation tokens for private access.
- If a personal access token is supported initially, encrypt it at rest and never return it through the API.

Phase 9.1 delivered public repositories. Phase 9.2 adds private repositories through a GitHub App, short-lived installation tokens, repository selection in the dashboard, equivalent CLI configuration for existing installations, and per-user ownership for GitHub App installations.

## Phase 10 — Webhooks, auto-deploy, and build visibility

Turn GitHub pushes into traceable, automatic deployments.

### Automation

- Enable or disable auto-deploy per application.
- Accept GitHub push webhooks only for the configured repository and branch.
- Validate the GitHub webhook signature.
- Deduplicate repeated webhook deliveries.
- Show which push and commit triggered each deployment.
- Allow a failed deployment to be retried manually.
- Support deployment cancellation when the build/runtime layer allows it safely.

Phase 10.1 delivers signed GitHub App push webhooks, per-application auto-deploy, branch/repository filtering, delivery deduplication, and deployment trigger metadata. Phase 10.3 adds persisted cancellation, interruption of active clone/build work, and manual retries for Git deployments linked through `retry_of` and `attempt`.

### Build visibility

- Separate source/clone logs, Docker build logs, deployment events, and application runtime logs.
- Persist build logs so failures remain available after the process finishes.
- Display explicit stages such as cloning, building, starting, checking health, publishing, and cleaning up.
- Stream build progress to the CLI and dashboard.

Phase 10.2 delivers ordered persistent build events and historical deployment log views in the dashboard and CLI.

## Phase 11 — Zero-downtime deployments

The baseline rollout is implemented in the API. End-to-end validation with a running Docker/Caddy stack remains before marking this phase complete.

Make releases safe enough for continuously available applications.

### Deployment strategy

1. Build the new image without disturbing the current container.
2. Start a candidate container on a new internal port.
3. Wait for its readiness/health check to succeed.
4. Switch the Caddy route to the candidate container.
5. Mark the new deployment as running.
6. Stop and remove the previous container after a grace period.

If the candidate fails, MiniPaaS keeps the previous deployment serving traffic and records the new deployment as failed.

The implementation persists candidate container metadata, serializes the promotion portion per application, restores routes and removes orphan candidates after an API restart, and exposes `DEPLOY_READY_TIMEOUT` for the TCP readiness deadline. Optional global runtime caps are now available through `CONTAINER_MEMORY_LIMIT_MB`, `CONTAINER_NANO_CPUS`, and `CONTAINER_PIDS_LIMIT`; per-application limits, HTTP health paths, and startup grace periods remain follow-up configuration work.

### Application configuration

- Internal application port.
- Health-check path.
- Health-check timeout and retry policy.
- Startup grace period.
- CPU and memory limits.
- Build arguments that are explicitly safe to expose to the build process.

## Phase 12 — Custom domains and metrics

### Custom domains

Phase 12.1 implements custom-domain persistence, DNS verification, Caddy route lifecycle, and dashboard management. The remaining validation is against a real DNS record and public HTTPS endpoint.

- Attach one or more custom domains to an application.
- Keep the default MiniPaaS subdomain available.
- Display pending, verified, and active DNS states.
- Verify that DNS points to the MiniPaaS host before activation.
- Configure the Caddy route and HTTPS certificate automatically.
- Remove routes and certificates safely when a domain is detached.

### Operational metrics

Phase 12.2 implements an on-demand metrics snapshot from Docker and deployment history, with a project-level dashboard panel. Prometheus-compatible export and long-term time-series storage remain future work.

- CPU and memory usage.
- Container restart count.
- Uptime and current container state.
- Deployment duration and success/failure rate.
- Recent health-check failures.
- Small per-application charts in the dashboard.

The initial metrics scope remains lightweight; Prometheus-compatible export can be added later.

### Real-time metrics

Phase 12.3 adds a shared Docker stats WebSocket per active container, automatic client reconnection, and a dedicated dashboard page with rolling CPU, memory, network, and disk charts. Long-term time-series persistence and alerting remain future work.

## Cross-cutting security and reliability

These controls should be implemented alongside the phases above rather than postponed to a final security pass:

- Allow only supported Git hosts and block repository URLs that resolve to loopback, link-local, or private infrastructure when appropriate, preventing SSRF.
- ✅ Limit repository size, upload size, build duration, and concurrent builds. Build-log retention remains a follow-up policy.
- Use shallow clones and avoid fetching unnecessary Git history.
- Never place repository credentials, environment variables, or build secrets in command output or persisted logs.
- Encrypt stored credentials and support credential rotation/revocation.
- Prefer short-lived GitHub App credentials over long-lived personal tokens.
- Validate webhook signatures and protect against replay/duplicate events.
- Run builds with constrained CPU, memory, network, and filesystem access where feasible.
- Guarantee cleanup of temporary directories, failed containers, and unused images.
- ✅ Record an audit trail for mutating API requests, including deploys, rollbacks, environment changes, domain changes, and authentication events.
- ✅ Add database backup and restore documentation before treating the platform as production-ready.

Phase 13 now provides request correlation, persisted audit events for mutating API requests, sensitive-endpoint rate limiting, bounded Docker builds, optional container resource caps, readiness probes, startup reconciliation of labeled orphan containers, and documented PostgreSQL backup/restore procedures. Distributed rate-limit state and per-application resource settings remain follow-up work for a multi-tenant production release.

## Phase 14.1 — Accounts, ownership, and safe deletion

This phase adds the minimum account and multi-user foundation needed by the dashboard:

- public account registration with username/password validation and rate limiting;
- authenticated profile lookup and username/password updates;
- ownership on applications, with owner-scoped app, deployment, log, environment, domain, metrics, and audit queries;
- ownership on GitHub App installations, with callback state bound to the MiniPaaS user and repository access filtered accordingly;
- migration `015_add_app_owners` to assign existing applications to the first configured user while preserving internal/system access for reconciliation jobs;
- destructive application deletion through the API, dashboard danger zone, and `minip apps delete <name> --yes` CLI command;
- runtime-first deletion: Caddy routes and containers are cleaned before the database row is removed, and cleanup errors preserve the row for recovery;
- dedicated dashboard routes for registration and account settings.

Manual validation remains for the complete browser flow: register a second user, confirm that each account sees only its own applications, update credentials, and delete an application after confirming the exact name.

## Phase 15.1 — API tokens for automation and CI/CD

This phase adds personal API tokens without weakening the existing dashboard/session authentication:

- migration `017_create_api_tokens` stores only a SHA-256 token hash, a display prefix, scopes, creation/usage timestamps, expiration, and revocation state;
- generated secrets use the `mpat_` prefix and at least 256 bits of cryptographic entropy;
- the raw token is returned only by `POST /me/tokens` at creation time and is never returned by listing, revocation, logs, audit events, or later API responses;
- `GET /me/tokens` and `DELETE /me/tokens/:id` are owner-scoped and require a dashboard/JWT session, so an automation token cannot create or revoke credentials;
- `read`, `deploy`, and `manage` scopes are enforced centrally with deny-by-default route policies;
- API-token requests keep the same user ownership boundaries as dashboard requests and cannot elevate the owner's permissions;
- the CLI supports `MINIPAAS_TOKEN` with priority over `config.json`, without persisting environment-provided secrets, plus `minip tokens create/list/revoke`;
- the Account page can create, list, copy once, and revoke tokens with confirmation;
- `last_used_at` is updated when a token authenticates, while expired or revoked tokens are rejected immediately.

Use a short-lived token with the smallest required scope in each CI/CD system, store it in the platform's secret manager, and rotate/revoke it when a workflow or person no longer needs access. Long-term token analytics, organization/team roles, and distributed rate limiting remain future improvements.

## Phase 16 — Capacity, deployment queue, and observability

This phase closes the learning project with a small operational layer built on the existing deployment and metrics primitives:

- `MAX_APPS_PER_USER` applies a simple ownership-aware application quota and returns a clear capacity error when it is reached;
- `MAX_CONCURRENT_BUILDS` is backed by a FIFO in-memory scheduler that reports active and waiting builds and removes cancelled waiters safely;
- `GET /capacity` exposes non-sensitive application counts, queue counters, and configured container limits for the authenticated owner;
- the Projects page polls the capacity snapshot alongside application status and shows active builds, queued deployments, and the per-user application limit;
- existing Docker metrics, request IDs, audit events, persistent build events, and health-check failures remain the observability foundation rather than introducing Prometheus or long-term time-series storage.

The queue is intentionally process-local for this educational project. PostgreSQL remains the source of truth for deployment state, while a restarted API process rebuilds its in-memory scheduler from new deployment work. Distributed queues, historical metrics retention, alerting, and team quotas remain outside the current scope.

## Later opportunities

These ideas can add value after the core Git and safe-deployment experience is solid:

- Deploy an existing OCI/Docker image without rebuilding source.
- Docker layer caching or a remote build cache.
- Notifications for deployment results through email, Slack, or webhooks.
- Team accounts, roles, and per-application permissions.
- Persistent volumes with an explicit backup strategy.
- Preview environments for pull requests.
- GitLab and generic Git provider support.
- Application templates for common stacks.
- Optional buildpack-style detection for projects without a `Dockerfile`; this should come only after Dockerfile-based deployments are mature.
- Backup verification and disaster-recovery exercises.

## Recommended order

The strongest next sequence for product value and portfolio depth is:

1. Manual GitHub deployment from a public repository.
2. Private repositories through a GitHub App.
3. Signed webhooks and auto-deploy.
4. Persistent and streamed build logs.
5. Zero-downtime route switching with readiness checks.
6. Custom domains and lightweight metrics.
7. Backend hardening: limits, correlation, readiness, reconciliation, and recovery drills.
