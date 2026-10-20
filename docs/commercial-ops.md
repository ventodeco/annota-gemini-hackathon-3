# ANNOTA Commercial Operations Runbook

## CI Verification

The `CI` workflow verifies pushes to `dev` and PRs targeting `dev` or the frozen
`master` branch. It has read-only repository permissions and does not reference
`ENV_PRODUCTION`, log into a registry, publish images, or connect to the host.
Tests use fixture configuration, not production credentials.

Backend CI supplies `POSTGRES_TEST_DSN` to PostgreSQL 14 and explicitly runs
every storage test ending in `Postgres` before the backend tests, vet, and
build. That step fails if any of them reports a skip. Without that variable,
these integration tests skip locally. PostgreSQL 14 is the
existing CI baseline, not a claim about the deployed database version. Qualify
the eventual release against the actual target major before deployment.

Frontend CI installs the committed root Bun workspace with
`bun install --frozen-lockfile`, then runs the configured Vitest script, lint,
and build in `web/`. CI and the frontend image builder use Bun 1.4.3. Dependency
modernization remains tracked separately in #36/#37.

### Final Image Checks

Run from the repository root with Docker, Python 3, and network access for image
pulls. The backend context is `backend/`, protected by `backend/.dockerignore`.
The frontend context is the root, protected by `.dockerignore`; building from
`web/` alone cannot resolve the committed workspace lockfile.

```bash
docker build -f backend/Dockerfile -t annota-be:issue-43 backend
docker build -f web/Dockerfile -t annota-fe:issue-43 .
bash scripts/ops/smoke-images.sh
```

Use `BACKEND_IMAGE` and `FRONTEND_IMAGE` to check different local image references.
The optional `backend` or `frontend` argument runs only that component, as CI
does. `SMOKE_POSTGRES_IMAGE`, `SMOKE_REDIS_IMAGE`, and `SMOKE_PROBE_IMAGE` select
dependency/probe images; defaults are `postgres:14`, `redis:7-alpine`, and
`python:3.12-alpine`.

The script creates a unique internal Docker network and disposable containers.
HTTP probes run inside that network; no service ports are published to the host.
It removes its service/probe containers, anonymous database volumes, and network
on exit, including failures. It does not connect to production storage or
services. Provider URLs point to a closed local port, with fixture values used
only to satisfy startup validation.

Backend checks require migrations in the final image, a ready `/readyz`, and
an exact match between migration ledger entries and the source SQL inventory.
They then stop Redis and PostgreSQL in turn, requiring `/readyz` to return 503
while `/healthz` stays 200, and `/readyz` to return 200 after each restart.
Frontend checks request the entry document, referenced JS/CSS, PWA manifest,
PDF worker, and `/login`. Asset responses must have appropriate content types
and must not contain an HTML fallback document. Image IDs are printed for the
verification record. Existing third-party stylesheet links are reported
separately; the check requires local generated CSS and makes no font-provider
requests. These checks do not establish live login or AI readiness.

## Liveness And Readiness

- `/healthz` is process liveness. It returns `200 ok` whenever the server
  handles requests and does not check dependencies. Use it to decide whether to
  restart the container.
- `/readyz` is dependency readiness. It returns `200 ready` only when
  PostgreSQL and Redis both respond within 2 seconds; otherwise `503 not
  ready`. The server enforces that deadline itself, so a stalled dependency
  yields 503 rather than a late success. Concurrent requests share one
  in-flight probe per dependency. The response never includes error details;
  the failing dependency is logged server-side. It makes no AI provider calls. Use it to accept a release
  and to route traffic.
- Neither endpoint requires authentication. Both are excluded from the
  frontend fallback in the Go server.

## Database Migrations

The backend applies `migrations/*.sql` in name order at startup, and refuses to
start when the directory is missing or contains no SQL files. Each migration's
SQL and its `schema_migrations` row commit in one PostgreSQL transaction that
holds a database-wide advisory lock. Consequences for operators and authors:

- A failed migration leaves no partial schema change and no ledger row. Fix
  the SQL in a new release and start again; do not edit the ledger by hand.
- Containers starting at the same time serialize on the lock and apply each
  migration once. A starter that waits longer than 60 seconds for the lock
  exits with a `migration lock` error; check for a stuck or long-running
  migration in the other starter before retrying.
- Migrations must be valid inside a transaction. Do not use
  `CREATE INDEX CONCURRENTLY` or other statements PostgreSQL forbids in a
  transaction block.
- Applied migrations that an older image does not contain are left alone, so a
  schema-compatible previous image can still start during rollback.

## Release And Rollback

Release publication and deployment automation are not enabled by this CI-only
change. Issue #43 tracks the remaining runtime checks, pinned release records,
staging qualification, rollback, and matched backup/restore work in the
[implementation plan](superpowers/plans/2026-10-20-issue-43-beta-release-readiness.md).
Do not treat a passing CI run as authorization to deploy production.

### Repository Settings Checkpoint

The frozen `master` branch still contains the old workflow. Updating `dev`
does not change that copy. Before claiming repository-wide release safety,
the operator must configure `ENV_PRODUCTION` to allow only `dev`, require an
appropriate deployment reviewer, and prevent self-approval. This blocks jobs
using that environment on `master` and PR merge refs. Audit credential scopes
without exposing their values. Do not trigger the unsafe old workflow as a test.

The approved plan includes switching GitHub's default branch to `dev` after
required checks pass on its intended head. Record the SHA and check URLs,
verify branch protections and open PR bases, and keep `master` frozen. Manual
release workflows can be activated only after they exist on the default branch.
Neither repository settings nor host configuration are changed by this PR.

The current authenticated collaborator, `naufaldi`, has repository write access;
`ventodeco` is the repository administrator. These settings need administrator
execution. The concrete proposed environment policy is: allow branch `dev`
only, require `ventodeco` as deployment reviewer, and disallow self-review.
Confirm reviewer availability before applying it; a run started by that same
reviewer cannot be approved by them under this policy.

For future releases, retain both previous component image digests and the
corresponding configuration. An image rollback requires a compatible database
schema; incompatible schema changes require the reviewed database/files
recovery procedure, not just restarting an old binary.

## Backup And Restore

- Postgres: run a daily `pg_dump` for the application database and test restore into a staging database weekly.
- Uploads: snapshot `UPLOAD_DIR` or object-storage bucket daily. Keep file backups aligned with database backup timestamps.
- Restore order: database first, uploaded files second, then check `/readyz` and smoke-test login, upload, annotation, and PDF open flows.

## Secrets Rotation

- Rotate `JWT_SECRET`, Google OAuth secret, AI provider keys (Gemini, OpenRouter, MiniMax, routed `api_key_env` values), Redis credentials, and database credentials on a scheduled cadence or immediately after exposure.
- When rotating `JWT_SECRET`, expect existing sessions to be invalidated. The Clerk migration (#39) replaces `JWT_SECRET` and `GOOGLE_OAUTH_*`. Revoke the old Google OAuth client after cutover.
- Production must use `APP_ENV=production`, explicit `ALLOWED_ORIGINS`, and secure HTTPS origins. In production the backend refuses to start when the database password is a known default (`password`, `postgres`, or the built-in `gemini_password` used when `POSTGRES_PASSWORD` is unset).
- `docker-compose.yml` is for local development only and binds to loopback. Never reuse its credentials elsewhere.

Where credentials live:

- Runtime values are in the production host's own compose/env configuration, outside this repository.
- CI/CD credentials (`DOCKERHUB_TOKEN`, `VPS_HOST`, `VPS_USER`, `VPS_SSH_KEY`) are in the GitHub `ENV_PRODUCTION` environment. Administration is tracked in #43/#50.
- Developer values are in the untracked `backend/.env`, which git and Docker ignore.

Rotation steps for an exposed credential:

1. Issue a replacement at the provider (or database/Redis), and revoke the old value there once the deploy uses the new one. Revoking first causes an outage, but if active abuse is suspected, revoke immediately.
2. Update the production host configuration or GitHub secret, then restart or redeploy.
3. Verify without printing values: `/readyz` returns 200 after database or Redis rotation, a small AI request succeeds after a provider key rotation, and login works after an auth secret rotation.
4. Record the date, credential name, provider, and who rotated it in the tracking issue. Never record the value.

If a secret is ever committed, rotate it first. Rewriting git history does not un-expose a pushed value. A force-push rewrite needs owner approval and coordination with every clone and open PR. For the #12 assessment and its scope limits, see [the evidence](evidence/issue-12/secret-exposure-assessment.md).

## Support And Data Requests

- User lookup key: email address from Google OAuth.
- For deletion requests, ask the user to use the in-app Delete account action first. If manual deletion is needed, delete the user row so cascades remove scans, annotations, documents, and related rows, then remove files in `UPLOAD_DIR`.
- For abuse investigations, collect request IDs, user ID, target route, timestamp, and AI rate-limit status from structured logs.

## Billing And Entitlements

- Current entitlement foundation exposes `/v1/entitlements/me` with the active plan and configured usage limits.
- Until Stripe or another billing provider is added, all users are treated as `free` and constrained by `AI_RATE_LIMIT`, `AI_RATE_LIMIT_WINDOW_SECONDS`, and `MAX_UPLOAD_SIZE`.
- Paid plans should map provider subscription state to entitlements before raising limits.
