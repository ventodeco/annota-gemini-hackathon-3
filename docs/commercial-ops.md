# ANNOTA Commercial Operations Runbook

## CI Verification

The `CI` workflow verifies pushes to `dev` and PRs targeting `dev` or the frozen
`master` branch. It has read-only repository permissions and does not reference
`ENV_PRODUCTION`, log into a registry, publish images, or connect to the host.
Tests use fixture configuration, not production credentials.

Backend CI supplies `POSTGRES_TEST_DSN` to PostgreSQL 14 and explicitly runs
`TestRunProductionMigrations_Postgres` before the backend tests, vet, and build.
Without that variable, the integration test skips locally. PostgreSQL 14 is the
existing CI baseline, not a claim about the deployed database version. Qualify
the eventual release against the actual target major before deployment.

Frontend CI installs the committed root Bun workspace with
`bun install --frozen-lockfile`, then runs the configured Vitest script, lint,
and build in `web/`. CI and the frontend image builder use Bun 1.4.0. Dependency
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

Backend checks require migrations in the final image, successful startup, and
an exact match between migration ledger entries and the source SQL inventory.
Frontend checks request the entry document, referenced JS/CSS, PWA manifest,
PDF worker, and `/login`. Asset responses must have appropriate content types
and must not contain an HTML fallback document. Image IDs are printed for the
verification record. These checks do not establish live login or AI readiness.

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
- Restore order: database first, uploaded files second, then run `/healthz` and smoke-test login, upload, annotation, and PDF open flows.

## Secrets Rotation

- Rotate `JWT_SECRET`, Google OAuth secret, Gemini API key, Redis credentials, and database credentials on a scheduled cadence or immediately after exposure.
- When rotating `JWT_SECRET`, expect existing sessions to be invalidated.
- Production must use `APP_ENV=production`, explicit `ALLOWED_ORIGINS`, and secure HTTPS origins.

## Support And Data Requests

- User lookup key: email address from Google OAuth.
- For deletion requests, ask the user to use the in-app Delete account action first. If manual deletion is needed, delete the user row so cascades remove scans, annotations, documents, and related rows, then remove files in `UPLOAD_DIR`.
- For abuse investigations, collect request IDs, user ID, target route, timestamp, and AI rate-limit status from structured logs.

## Billing And Entitlements

- Current entitlement foundation exposes `/v1/entitlements/me` with the active plan and configured usage limits.
- Until Stripe or another billing provider is added, all users are treated as `free` and constrained by `AI_RATE_LIMIT`, `AI_RATE_LIMIT_WINDOW_SECONDS`, and `MAX_UPLOAD_SIZE`.
- Paid plans should map provider subscription state to entitlements before raising limits.
