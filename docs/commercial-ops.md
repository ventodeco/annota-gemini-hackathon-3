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

CI only verifies: it builds and checks the final images but never publishes or
deploys them, and no release automation exists yet. Issue #43 tracks the remaining runtime checks, pinned release records,
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

- Rotate `CLERK_SECRET_KEY` (Clerk Dashboard > API keys), AI provider keys (Gemini, OpenRouter, MiniMax, routed `api_key_env` values), Redis credentials, and database credentials on a scheduled cadence or immediately after exposure.
- Session tokens are signed by Clerk, not by ANNOTA; rotating the Clerk secret key does not end user sessions. Revoke sessions in the Clerk Dashboard when needed. The Clerk migration (#39) removed `JWT_SECRET` and `GOOGLE_OAUTH_*`; revoke the old Google OAuth client after cutover.
- Production must use `APP_ENV=production`, explicit `ALLOWED_ORIGINS`, and secure HTTPS origins. In production the backend refuses to start when the database password is a known default (`password`, `postgres`, or the built-in `gemini_password` used when `POSTGRES_PASSWORD` is unset).
- `docker-compose.yml` is for local development only and binds to loopback. Never reuse its credentials elsewhere.

Where credentials live:

- Runtime values are in the production host's own compose/env configuration, outside this repository.
- CI/CD credentials (`DOCKERHUB_TOKEN`, `VPS_HOST`, `VPS_USER`, `VPS_SSH_KEY`) are in the GitHub `ENV_PRODUCTION` environment. Administration is tracked in #43/#50.
- Developer values are in the untracked `backend/.env`, which git and Docker ignore.

Rotation steps for an exposed credential:

1. Issue a replacement at the provider (or database/Redis), and revoke the old value there once the deploy uses the new one. Revoking first causes an outage, but if active abuse is suspected, revoke immediately.
2. Update the production host configuration or GitHub secret, then restart or redeploy.
3. Verify without printing values: `/readyz` returns 200 after database or Redis rotation, a small AI request succeeds after a provider key rotation, and Clerk sign-in plus an authenticated API call work after a Clerk key rotation.
4. Record the date, credential name, provider, and who rotated it in the tracking issue. Never record the value.

If a secret is ever committed, rotate it first. Rewriting git history does not un-expose a pushed value. A force-push rewrite needs owner approval and coordination with every clone and open PR. For the #12 assessment and its scope limits, see [the evidence](evidence/issue-12/secret-exposure-assessment.md).

## Support And Data Requests

- User lookup key: `users.clerk_user_id` (Clerk user ID) or the Clerk-verified email address.
- For deletion requests, ask the user to use the in-app Delete account action first. Manual deletion and Clerk reconciliation steps are in [`clerk-auth.md`](clerk-auth.md#deletion-and-reconciliation).
- For abuse investigations, collect request IDs, user ID, target route, timestamp, and AI rate-limit status from structured logs.
- For failed operations, ask for the **Reference** shown in the app and follow [`beta-triage.md`](beta-triage.md). It also covers OCR, AI latency/cost and proposed alert thresholds.

## Privacy And Data Lifecycle

The user-facing summary is the in-app Privacy Policy (`web/src/pages/LegalPage.tsx`). Keep the two in sync.

| Data | Where | Removed by |
| --- | --- | --- |
| Profile: Clerk user ID, email, avatar URL, preferred language | `users` | Account deletion (row deleted, Clerk ID tombstoned) |
| Page images | `UPLOAD_DIR/{scanID}.{jpg,png,webp}` + `scans` | Scan or account deletion |
| PDFs | `UPLOAD_DIR/documents/{documentID}.pdf` + `documents` | Document or account deletion |
| OCR / extracted page text | `scans.full_ocr_text` | Scan, document (page scans) or account deletion |
| Saved annotations (selection, context, AI explanation) | `annotations` | Annotation or account deletion only |
| Reading progress | `documents` | Document or account deletion |

- **Isolation.** Every `/v1/*` route requires a Clerk session and checks ownership with the numeric `users.id`.
  Another account's ID gets 403 or an empty list, never content.
- **Caching.** `/v1/*` responses are `Cache-Control: no-store`, every authenticated web request uses
  `fetch(..., { cache: 'no-store' })` so older HTTP-cached responses are never reused. `GET /v1/users/me`, which
  the app calls at each sign-in and account switch, sends `Clear-Site-Data: "cache"`. That evicts entries
  earlier builds cached (images were `private, max-age=300`), which a copied URL could otherwise still open.
  The header needs HTTPS (or localhost). Browsers apply it to a separate API origin only on credentialed
  requests, so the profile request uses `credentials: 'include'`. The API allows that only for its exact
  `ALLOWED_ORIGINS`; wildcard origins are rejected in production. The service worker caches only the app
  shell. The web app clears its in-memory query cache on sign-out, account switch and deletion. At startup it
  removes the `api-cache` storage and the unscoped `auth_token`/`savedAnnotations` keys left by older builds.
- **Deletion order.**
  - Scan and document deletion remove files first. If a file can't be removed, the API returns 500, keeps the
    rows, and a retry finishes.
  - Document deletion removes the PDF, then deletes the page scans and the document row in one transaction. The
    document row is locked first (`FOR UPDATE`), so a page scan created at the same moment is either removed
    with it or rejected.
  - Failed or malformed uploads remove their row and file before returning an error. PDF and multipart parser
    errors are not logged because they can quote file or request bytes.
  - Uploads hold a shared per-account PostgreSQL advisory lock from creating the row until the file is
    recorded. Account deletion holds it exclusively while it removes files and rows, so an upload can't
    publish a file the deletion has already passed.
  - Single scan and document deletion also hold the lock exclusively, so an upload of that same item can't
    write its file after the row is gone.
  - Account deletion lists the account's files in one unpaginated query. Concurrent reordering, such as a
    reading-progress update, can't make it skip a file.
  - Once acquired, the lock is held even if the client disconnects, until the upload's file and record work
    finishes.
  - Each lock holder or waiter keeps one database connection open, and the holder's queries borrow more.
    Keep the Go pool uncapped (the current default). PostgreSQL's `max_connections` then makes extra
    attempts fail fast, and the upload returns 500 and releases its lock.
  - Capping `MaxOpenConns` safely would also require limiting how many requests can hold *or wait for*
    account locks at once, with spare connections reserved for their queries. A larger cap alone can still
    deadlock, for example one upload holding the lock while deletion waiters take every other connection.
- **Retained annotations.** Annotations are separate learning items. Deleting their scan sets
  `annotations.scan_id` to NULL. Deleting their document keeps `document_id` as a historical reference (no FK;
  IDs are never reused). They are removed when the user deletes them or the account.
- **In-flight AI work.**
  - OCR runs in the background for up to 5 minutes after upload.
  - If the scan or account is deleted first, the provider call isn't cancelled. Its result is discarded,
    because the update matches no row.
  - Content already sent to a provider is subject to that provider's terms.
- **Logs.**
  - Request logs carry the request ID, user ID, method, path, status and duration, plus the short JSON error
    message for 4xx/5xx responses.
  - Routed AI attempts log provider, model, status, usage and cost metadata.
  - AI provider errors carry only a classification and status code, never the provider's response body or
    message.
  - Uploaded content, OCR text and selections are not logged.
- **Backups.** Database and upload backups (see Backup And Restore) keep deleted data until they expire. The
  backup retention period is an owner decision and is not yet set; state it in the Privacy Policy once
  chosen.

### Orphaned upload sweep (read-only)

Builds before #41 could leave files without a row:

- malformed PDF uploads;
- failed image writes;
- scan or document deletes that continued after a file error;
- an upload racing account deletion.

List candidates once after deploying, review them, and delete them manually:

```sh
cd "$UPLOAD_DIR"
ls | sed -nE 's/^([0-9]+)\.(jpg|png|webp|bin)$/\1/p' | sort -u > /tmp/scan-file-ids
psql "$DATABASE_URL" -Atc 'SELECT id FROM scans' | sort -u > /tmp/scan-row-ids
comm -23 /tmp/scan-file-ids /tmp/scan-row-ids    # image files with no scan row
ls documents | sed -nE 's/^([0-9]+)\.pdf$/\1/p' | sort -u > /tmp/doc-file-ids
psql "$DATABASE_URL" -Atc 'SELECT id FROM documents' | sort -u > /tmp/doc-row-ids
comm -23 /tmp/doc-file-ids /tmp/doc-row-ids      # PDFs with no document row
```

Older scans store `/uploads/...` in `scans.image_url`. Their file is still named `{scanID}.{ext}`, so the listing covers them.

## Public Beta Launch Gates

The repository work for the beta baseline (#33) has merged into `dev`, and CI passes. Merged code is not live
readiness. Do not invite beta users until every row below is done, and record each result in its issue
without credential values. Production changes and deploys stay separate owner decisions.

| Gate | In the repository (`dev`) | Still needed before beta | Issues |
| --- | --- | --- | --- |
| Release safety | CI verifies only; final images carry migrations and assets; PostgreSQL tests run in CI; atomic migrations; `/readyz` | Administrator `ENV_PRODUCTION` protection and default branch `dev` (see Repository Settings Checkpoint); read-only host inspection; a pinned release record with staging and rollback | #43, #50 |
| Backup and restore | Policy outline only (Backup And Restore) | Real backup/restore scripts for the inspected host layout; approved off-host destination, schedule and alerts; an isolated restore; backup retention stated in the Privacy Policy | #43, #41 |
| Host and origins | Production refuses a default database password; local Compose binds PostgreSQL/Redis to localhost | `APP_ENV=production`, a non-default database password, private PostgreSQL/Redis, uploads persisted across container replacement, HTTPS `FRONTEND_BASE_URL`/`ALLOWED_ORIGINS`/`CLERK_AUTHORIZED_PARTIES` | #12, #41, #43 |
| Credentials | Exposure assessment and rotation runbook (Secrets Rotation) | Rotation decision and record; revoke the old Google OAuth client after the Clerk cutover | #12, #39 |
| Sign-in | Clerk Bearer sessions with Google and email; legacy Google linking by verified subject; tombstoned deletion | Development and production Clerk instances (custom Google credentials in production, email code, origins), keys set privately; live desktop and mobile sign-in; legacy inventory SQL and cutover notice ([`clerk-auth.md`](clerk-auth.md)) | #39 |
| Privacy and isolation | Two-account isolation tests, no-store caching, file-first deletion, accurate Privacy Policy | Live two-account sign-in, switch and delete, including Clerk-side removal; confirm production AI routes match the Privacy Policy and review the providers' terms; operator contact for data requests; run the orphaned upload sweep once after deploy | #41, #80 |
| AI routes and models | Configurable routing, `qualify-routes`, provenance, evaluation set ([`ai-routing.md`](ai-routing.md)) | Gateway keys; one bounded live pass per candidate; defaults chosen from passing results. The current OCR default `baidu/qianfan-ocr-fast:free` is not served, so OCR needs a qualified route. Speech voice and listening check | #34, #35 |
| Usage and cost | Owner-approved daily caps ([`usage-quotas.md`](usage-quotas.md)) | Reconcile provider-reported cost after live calls. The caps limit operations, not spend | #42 |
| Reader journey | OCR always settles; PDF page identity; error states; intro, sample and first-use guide; lazy signed-in routes; bounded PDF page rendering | One fresh live user: upload, select, explain, listen, save, reopen, delete; real iOS Safari and Android Chrome | #40, #45 |
| Support and alerts | Copyable request references, user-attributed logs, triage guide ([`beta-triage.md`](beta-triage.md)) | Choose the alert tool, owner and destination, then prove one controlled alert; the Contact Us page and report destination | #44, #80 |
| Dependencies | Dependabot configuration; Go and Bun runtimes current | Dependabot stays inactive until the default branch is `dev`; confirm alerts and security updates are on. TypeScript 7 and ESLint 10 wait for their plugins; the `braces` advisory (via the shadcn CLI) has no patched release | #36, #50 |

These are disclosed limitations, not launch gates:

- explanations are English only;
- image-only PDFs are not supported;
- PDFs whose fonts were embedded as Type 3 may extract garbled text;
- explanation and speech reservations stranded by a crash mid-request stay charged until the UTC reset.

The Nix development shell is maintained by teammates and is not a launch gate.

## Billing And Entitlements

- `/v1/entitlements/me` returns the active plan and the reader's daily usage allowance.
- There is no billing provider yet, so every user is `free`. The persistent per-feature daily quotas and the
  global beta budget in [`usage-quotas.md`](usage-quotas.md) apply, together with the `AI_RATE_LIMIT` burst
  throttle and `MAX_UPLOAD_SIZE`.
- Paid plans should map provider subscription state to entitlements before raising limits.
