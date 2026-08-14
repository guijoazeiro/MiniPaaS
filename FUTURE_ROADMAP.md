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

Phase 9.1 delivered public repositories. Phase 9.2 adds private repositories through a GitHub App, short-lived installation tokens, repository selection in the dashboard, and equivalent CLI configuration for existing installations.

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

## Later opportunities

These ideas can add value after the core Git and safe-deployment experience is solid:

- Deploy an existing OCI/Docker image without rebuilding source.
- Docker layer caching or a remote build cache.
- A deployment queue with per-host concurrency control.
- Notifications for deployment results through email, Slack, or webhooks.
- Team accounts, roles, and per-application permissions.
- Personal API tokens separate from login sessions.
- Persistent volumes with an explicit backup strategy.
- Preview environments for pull requests.
- GitLab and generic Git provider support.
- Application templates for common stacks.
- Optional buildpack-style detection for projects without a `Dockerfile`; this should come only after Dockerfile-based deployments are mature.
- Host resource quotas and capacity warnings.
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
