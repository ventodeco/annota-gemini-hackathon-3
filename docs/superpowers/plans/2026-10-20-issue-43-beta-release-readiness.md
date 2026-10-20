# Issue 43 Beta Release Readiness Implementation Plan

> **For agentic workers:** Use superpowers:executing-plans to implement this plan task by task after owner approval. Execute inline unless the owner separately requests delegation.

**Status:** Implementation approved on 2026-10-20. PR 1 is in progress; live settings and production actions retain the checkpoints below.

**Issue:** https://github.com/ventodeco/annota-gemini-hackathon-3/issues/43

**Goal:** Verify one identifiable ANNOTA release before deployment, preserve uploaded material across replacement, and demonstrate recovery of the database and files together.

**Architecture:** Keep the Go API, React/Vite frontend, PostgreSQL, Redis, and Docker Compose deployment. Separate unprivileged verification from operator-triggered publication and deployment. Release records identify both images by digest and one source commit; recovery uses a matched database/file backup.

**Tech stack:** GitHub Actions, Go database/sql, PostgreSQL, Bun workspaces, Docker, nginx, Docker Compose, and the existing Caddy Docker label convention.

## Scope and decisions

Use four delivery PRs against `dev`:

1. CI and final-image packaging, including enabling existing PostgreSQL migration coverage.
2. Runtime release checks: atomic migrations, concurrent startup protection, readiness, and static assets.
3. Commit-pinned publication, staging deployment, persistence, and rollback rehearsal.
4. Scheduled backup, isolated restore, and recovery evidence.

This order gives each PR an independent acceptance target. A single PR for all of #43 would mix CI changes, data correctness, host configuration, and recovery operations, making review and failures harder to isolate. Replacing the hosting platform would expand the task without resolving the underlying packaging and data-consistency defects.

**Proposed release policy:** PRs and ordinary pushes verify only. A manual release selects a full commit reachable from `dev`, reruns required checks for that exact commit, publishes both images, and records their digests. Deploying production is a separate, explicit operator action. Keep the current DockerHub repositories unless the owner requests a registry change.

**Owner-approved plan decision (2026-10-20):** Include switching GitHub's default branch from `master` to `dev` after the required checks pass on the intended `dev` head. Keep `master` frozen. Manual `workflow_dispatch` workflows must exist on the default branch, so the switch must precede release-workflow activation. The owner approved including this step in the plan; this planning turn does not execute the settings change or approve application implementation.

## Global constraints

- Preserve unrelated local changes in `AGENTS.md` and `docs/research/`.
- Application changes start only after plan approval; production deployment and host configuration require the corresponding explicit operator action.
- Keep issue #43 open until deployment and recovery evidence meet its complete Definition of Done.
- Keep dependency modernization in #36/#37, Clerk integration in #39, and AI routing in #34/#35. Coordinate their final configuration before beta launch.
- Do not perform a PostgreSQL major-version upgrade incidentally. The repository currently uses PostgreSQL 13 in local Compose, 14 in CI, and 16 in Nix. Inventory the real target major before staging qualification; use matching supported test and backup tools. An unsupported deployed major requires a separately reviewed upgrade before beta release.
- If using Nix, enter `nix develop` at the root before development commands. Always prefix shell commands with `rtk`; use `rtk proxy` where a command has no RTK adapter.
- Build with the existing Bun workspace and committed root `bun.lock`. Pin the selected Bun version consistently in CI and the Docker builder, recording it in the runbook. Use a Go builder satisfying `backend/go.mod`'s Go 1.25.5 floor, and coordinate version changes with #37.
- No real AI requests are needed for packaging, migration, or dependency-readiness checks. Synthetic local fixtures may satisfy startup validation without making upstream requests. Live auth/AI checks remain pending until configured privately.
- Keep credentials outside source control, command output, build arguments, and browser artifacts. Frontend public build variables remain distinct from backend secrets.
- After the first failing test/tooling check, the total test-debugging budget is 30 minutes across this task. Stop earlier for unavailable prerequisites; record the command, useful failure, suspected cause, attempts, and next action in #43. Do not weaken checks.
- Record browser-visible verification with desktop/mobile screenshots. Infrastructure and database checks need commands, responses, and logs, not unrelated screenshots. Pause for owner direction if screenshots contradict the approved outcome.

## Evidence from the current checkout

- `.github/workflows/ci-cd.yml` combines tests, registry publication, and SSH deployment for PR and push events. Both build jobs use `ENV_PRODUCTION`; publishing uses `push: true` and updates `latest` without an event/branch guard.
- `backend/Dockerfile` copies the binary into the final image but omits `migrations/`, which `backend/cmd/server/main.go` requires on startup.
- `web/Dockerfile` copies `web/bun.lock*`, installs Bun globally through npm, and installs as a standalone frontend. The committed dependency graph is a root Bun workspace.
- The backend Docker context is `backend/`, so root `.dockerignore` does not protect that build context. The frontend uses the root context, so `web/.dockerignore` does not protect it. Review both actual contexts.
- `RunMigrations` commits migration SQL before separately inserting its ledger row. Two starters can both decide a migration has not been applied.
- Some SQLite migration tests intentionally expect the ledger write to fail. These assertions describe the current partial-commit defect and must be replaced by valid production behavior checks when the runner changes.
- `TestRunProductionMigrations_Postgres` skips without `POSTGRES_TEST_DSN`; CI starts PostgreSQL but never supplies it.
- `/healthz` always responds `200 ok`; it does not observe current PostgreSQL/Redis availability.
- The Go SPA handler rewrites JS/CSS/manifest requests to `/`, causing existing static files to return the HTML document.
- Only a development database/Redis Compose file is tracked. Existing SSH deployment references separate host directories whose configuration has not been inspected.
- GitHub's live `master` workflow has the same unguarded publishing/deployment path. `ENV_PRODUCTION` currently has no protection rules or deployment-branch restrictions. Updating the workflow on `dev` alone cannot neutralize the frozen branch's workflow.
- Go, Nix, Docker, actionlint, and shellcheck are unavailable in the current shell. Bun is present. Planning did not run application tests, build images, connect to the host, or change repository settings.

## PR 1: Safe CI and complete container artifacts

### Task 1: Make ordinary CI verify without production privileges

**User Story:** As the maintainer, I want PR checks to verify the app without publishing images or touching production, so that proposed changes cannot trigger a release.

**Files:** Modify `.github/workflows/ci-cd.yml` and `docs/commercial-ops.md`. Add `.github/workflows/release.yml` only in PR 3.

**Interfaces:** CI produces pass/fail results for the checked-out event commit. It consumes no registry/SSH secrets and references no production environment. Backend CI receives `POSTGRES_TEST_DSN` for its isolated service database. Later release jobs reuse the same verification commands for the selected source commit.

- [ ] Replace combined test/build/push jobs with backend and frontend verification jobs. Remove registry login, publication, SSH deployment, and `ENV_PRODUCTION` from ordinary CI.
- [ ] Present the concrete `ENV_PRODUCTION` protection change for owner approval: permit only `dev`, require a deployment reviewer, and prevent self-approval. Audit secret scopes by metadata without reading values. Apply and verify this guard through the approved settings action before claiming repository-wide release safety; it blocks the old frozen-branch and PR-ref jobs without editing `master`. Do not trigger the unsafe legacy workflow merely to test the guard.
- [ ] Keep PR verification for `dev` and legacy `master` targets, and push verification for `dev`; add `permissions: contents: read`. Do not introduce `pull_request_target`.
- [ ] Install frontend dependencies from the root with `bun install --frozen-lockfile`; execute scripts in `web/`. Use fixed CI-only frontend URLs rather than production environment variables.
- [ ] Supply a service-local PostgreSQL DSN and fail the PostgreSQL job immediately when it is missing. Run the existing production migration test explicitly with verbose output, then the backend checks once.
- [ ] Build both final Docker images with `push: false` and run the packaging checks in Task 2. Include the root lockfile in dependency-cache keys.
- [ ] Validate workflow syntax with `rtk proxy actionlint .github/workflows/ci-cd.yml`; inspect one real PR run after opening the implementation PR. Save its URL and confirm no registry login, push, SSH step, or production deployment job exists in that run.

**Verification commands:** From `backend/`, run `rtk proxy go test -count=1 -v ./internal/storage -run '^TestRunProductionMigrations_Postgres$'` with the CI service DSN, followed by `rtk proxy go test ./...`, `rtk proxy go vet ./...`, and `rtk proxy go build ./cmd/server`. From the root run `rtk proxy bun install --frozen-lockfile`; from `web/` run `rtk proxy bun run test`, `rtk proxy bun run lint`, and `rtk proxy bun run build`.

**Edge Cases:** Same-repository PR, fork PR without secrets, push to `dev`, missing DSN, dependency lock mismatch, failing image build, and cancelled verification.

**Definition of Done:** A PR run executes all relevant checks and the real PostgreSQL migration test without a skip, requires no production credentials, and has no publishing/deployment path. Verified environment restrictions also prevent the frozen `master` workflow and PR refs from starting production jobs; without the approved settings change, report that legacy exposure as unresolved.

### Task 2: Build complete final images from the actual contexts

**User Story:** As the operator, I want verified images to contain everything required at runtime, so that a successful source build also produces a startable release.

**Files:** Modify `backend/Dockerfile`, `web/Dockerfile`, `.dockerignore`, and `docs/commercial-ops.md`; create `backend/.dockerignore` and `scripts/ops/smoke-images.sh`. Keep `web/.dockerignore` only for a documented build context where it is actually used.

**Interfaces:** Backend build context remains `backend/`; frontend build context remains the root. Runtime backend path is `/app/migrations`; frontend output remains nginx static files. The smoke script consumes the two locally built image references and uses isolated containers/volumes.

- [ ] Establish the failing packaging evidence on a Docker-capable runner: backend image lacks `/app/migrations`; frontend build cannot use the committed workspace lock as written. Record concrete build output rather than writing tests that match Dockerfile text.
- [ ] Copy `migrations/` into the backend final image. Add context-appropriate ignores excluding local env files, upload data, dependencies, built output, and local binaries without excluding SQL migrations or required build inputs.
- [ ] Use a pinned official Bun builder, copy root `package.json`, root `bun.lock`, and `web/package.json` before the frozen root install, then copy the frontend build inputs and build from `/app/web`. Preserve existing public Vite build variables.
- [ ] Build images with `rtk proxy docker build -f backend/Dockerfile -t annota-be:issue-43 backend` and `rtk proxy docker build -f web/Dockerfile -t annota-fe:issue-43 .`.
- [ ] Implement `scripts/ops/smoke-images.sh` to start isolated PostgreSQL/Redis and final images using local-only startup fixtures, wait with a bounded timeout, verify migration ledger entries, and request frontend HTML, referenced JS/CSS, manifest, and PDF worker assets. Clean up only its own resources on exit.
- [ ] Require JS/CSS/manifest responses to have the appropriate content type and non-HTML body; verify a client route returns the SPA document. Record actual image IDs and commands. Never call paid AI routes in this smoke check.

**Edge Cases:** Missing migrations, changed root lockfile, environment files inside backend context, stale local `node_modules`, missing PDF worker, and startup blocked by missing local services.

**Definition of Done:** Both images build cleanly from committed inputs; the final backend applies migrations, and the final nginx image serves the generated application and assets. CI executes `rtk proxy bash scripts/ops/smoke-images.sh` successfully.

## PR 2: Runtime checks that protect a release

### Task 3: Make migration application atomic and safe under concurrent startup

**User Story:** As the operator, I want failed or overlapping application starts to leave a consistent database, so that retrying a release does not corrupt migration state.

**Files:** Modify `backend/internal/storage/migrate.go` and `backend/internal/storage/migrate_test.go`; create `backend/migrations/004_ensure_annotation_scan_index.sql` if inspection confirms the existing-schema repair is required. Update `docs/commercial-ops.md` with migration failure recovery.

**Interfaces:** Preserve `RunMigrations(db *sql.DB, migrationsDir string) error`. On PostgreSQL, hold one dedicated connection and a database-scoped advisory lock across ledger creation, status checks, and application. Apply each migration's SQL and ledger insertion in the same transaction. Bound lock waits and release the lock before returning the connection.

- [ ] Add PostgreSQL regressions for fresh schema, upgrade with existing rows, repeat invocation, SQL failure rollback, ledger insertion failure rollback, and two concurrent starters. Each test gets isolated schema state; concurrent test connections use the same intended search path and distinct sessions.
- [ ] Run the new focused cases and establish the current partial-commit/race failure before changing the runner.
- [ ] Implement the dedicated connection and startup lock before creating the ledger, then transactional SQL plus ledger writes. Use transaction-scoped queries consistently; do not issue ledger queries through the unrelated connection pool.
- [ ] Replace SQLite assertions that expect partial success with valid transaction-helper tests or PostgreSQL runner tests. Preserve meaningful file-discovery tests and unrelated storage tests; do not simulate PostgreSQL advisory locks in SQLite.
- [ ] Verify `idx_annotations_scan_id` on an existing database where migration 001 is already recorded but the index is absent. Repair with an additive migration containing `CREATE INDEX IF NOT EXISTS idx_annotations_scan_id ON annotations(scan_id);`; do not rewrite applied history.
- [ ] Rerun focused PostgreSQL cases, then backend test/vet/build once. Record no duplicate ledger rows, no committed DDL after a failed ledger insert, preserved fixture ownership/data, and bounded concurrent startup.

**Edge Cases:** Empty migration directory, missing directory, failed SQL, failed bookkeeping, cancelled or timed-out lock wait, first-ever simultaneous startup, and already-applied historical migrations.

**Definition of Done:** SQL changes and the ledger agree after success or failure; overlapping starts serialize safely; an existing database upgrades without losing rows and receives required indexes.

### Task 4: Separate liveness, dependency readiness, and static asset behavior

**User Story:** As the operator, I want deployment checks to identify a usable app, so that a running process with broken dependencies or assets is not accepted as a successful release.

**Files:** Modify `backend/cmd/server/main.go`, `backend/internal/storage/redis.go`, relevant mocks/tests implementing `RedisClient`, `backend/docs/openapi.yaml`, and `docs/commercial-ops.md`. Create focused HTTP helpers/tests under `backend/internal/handlers/` for readiness and frontend serving. Adjust `web/nginx.conf` caching only where release smoke checks expose stale entry-document behavior.

**Interfaces:** Keep `/healthz` as process liveness. Add `/readyz` with bounded PostgreSQL `PingContext` and Redis ping checks; respond `200` only when both succeed, otherwise `503` with a generic response. Expose a small `Ping(context.Context) error` capability on the Redis boundary. No live AI requests belong in readiness. Treat persistent upload permissions as a startup/smoke check.

- [ ] Add HTTP tests for healthy dependencies, failed PostgreSQL, failed Redis, and request cancellation/deadline. Add static-serving tests for real JS/CSS/manifest files, SPA routes, absent assets, and API/health route isolation.
- [ ] Extract only the handlers needed to test these behaviors; keep unrelated startup and routing code intact. Prevent error responses from exposing connection strings or provider secrets.
- [ ] Serve existing static files directly in the Go hosting path. Return the entry document for valid SPA navigation and `404` for absent assets; do not rewrite asset paths to HTML.
- [ ] Extend final-image smoke checks to stop each dependency and observe `/readyz` become `503` while `/healthz` still reports liveness, then recover after restart. Verify upload-directory write permissions under the deployed runtime identity.
- [ ] Test both nginx and Go hosting paths with a built frontend. In the collaborative browser capture desktop and mobile login/route smoke evidence under `docs/evidence/issue-43/` during execution, using sanitized local/staging data.

**Edge Cases:** Dependency loss after startup, hanging ping, missing frontend build, missing JS chunk, direct deep link, cached entry document/service worker after a release, and API requests accidentally returning HTML.

**Definition of Done:** Dependency failures prevent readiness, process liveness remains separate, static assets have correct bodies/content types, and browser navigation loads the expected release on desktop and mobile. Save response/log evidence and actual screenshot paths.

## PR 3: An identifiable release and reversible staging deployment

### Task 5: Publish and deploy the same verified source revision

**User Story:** As the operator, I want to select and recover one complete release, so that frontend/backend versions and uploaded files stay coherent during updates.

**Files:** Create `.github/workflows/release.yml`, `docker-compose.prod.yml`, `scripts/ops/deploy.sh`, and `scripts/ops/smoke-release.sh`; update `docs/commercial-ops.md` and public environment examples. Keep repository environment changes and host installation as explicit execution steps.

**Interfaces:** Release input is a full source commit reachable from `dev`, plus an approved target environment. Output is `release.json` containing source SHA, both image digests, public frontend build configuration identity, migration inventory, and verification run URL. The deployment script consumes that record and resolves both images by digest. Record the prior release before replacing containers.

- [ ] Execute the approved plan direction at the repository-settings checkpoint: verify required checks passed on the intended `dev` head, record its SHA and check-run URLs, then change the default branch to `dev`. Verify the setting, branch protection, and open PR bases; preserve `master` and do not merge into it. Confirm the manual release workflow is present on default `dev` before activation. Do not dispatch a release as part of changing the default branch.
- [ ] Audit the proposed repository-environment rules; restrict releases to `dev` and production jobs to their protected environment. Use one deployment concurrency group per target with `cancel-in-progress: false` and a host lock shared with backups.
- [ ] Pin checkout to the validated source commit, rerun PR 1/2 checks, and publish only after success. Record registry-returned digests; do not deploy `latest` or treat a mutable SHA tag alone as immutable. Supply public Vite environment configuration at build time and preserve it in the record.
- [ ] Qualify each environment-specific frontend artifact after its final build. Different baked API/app URLs produce different digests, so passing a staging build does not qualify a rebuilt production image. Run final-image smoke checks for the production-configured artifact before an approved production promotion.
- [ ] Before writing host paths into scripts, inspect the actual ANNOTA Compose projects, image references, database major, upload mounts and stored file paths, Caddy networks, domains, free space, environment files, and backup facilities through authorized read-only access. Do not assume the old CI SSH paths or the dated skill inventory describe the current host.
- [ ] Define Compose backend/frontend services using pinned images, persistent PostgreSQL/upload storage, private-only database/Redis networking, and Caddy labels on the application services. Preserve existing stored file-path compatibility or include an explicit, tested path migration. Do not reuse a production volume or domain for staging.
- [ ] Deploy first to isolated staging using `docker compose pull` followed by targeted `up -d`; avoid routine `down`. Reject missing volumes, incompatible schema, absent release records, failed pull, or invalid configuration before replacing running containers.
- [ ] Require a valid matched backup before a data-changing release. Wait for readiness and run bounded asset, authenticated ownership, upload/persistence, and source-reopen smoke checks. Record expected interruption for the single-instance Compose strategy; do not promise zero downtime.
- [ ] Rehearse replacing both application images with the previous recorded digests. Confirm schema compatibility before image rollback. Incompatible migrations require the matched database/files recovery procedure from PR 4 and explicit operator action; never silently restore data and discard newer writes.
- [ ] Demonstrate failed readiness leaves the release marked failed, prevents further promotion, preserves logs and the previous record, and gives the operator concrete recovery instructions. Verify image mismatch and overlapping deploy attempts are rejected.

**Verification:** Run `rtk proxy docker compose -f docker-compose.prod.yml config --quiet` using the target's private environment loading; run `rtk proxy bash scripts/ops/smoke-release.sh` against isolated staging. Record both running digests, migration state, readiness responses, persistence hashes before/after replacement, and rollback results. Capture desktop/mobile staging screenshots for the frontend smoke.

**Edge Cases:** Wrong source commit, PR merge SHA, partial image publication, mutable tags, different frontend build URLs, overlapping deploy/backup, missing upload volume, bad readiness, incompatible database schema, and stale PWA cache.

**Definition of Done:** Staging runs both images from one verified source revision, retains uploaded files across container replacement, and can return to a demonstrated compatible prior release. Production rollout remains a separate operator decision; live Clerk/AI readiness depends on #39/#34/#35.

## PR 4: Matched backup and demonstrated recovery

### Task 6: Schedule protected backups and restore into isolation

**User Story:** As the operator, I want a verified copy of learning material and ownership data, so that a host or release failure can be recovered without mixing database and file versions.

**Files:** Create `scripts/ops/backup.sh`, `scripts/ops/restore.sh`, `ops/systemd/annota-backup.service`, `ops/systemd/annota-backup.timer`, `ops/systemd/annota-restore-check.service`, `ops/systemd/annota-restore-check.timer`, and `ops/systemd/annota-backup-failure.service`; update `docs/commercial-ops.md` with installation, restore, and evidence instructions.

**Interfaces:** Each backup is one completed set containing a PostgreSQL custom-format dump, upload archive, checksums, release record, database major, and UTC capture interval. Restore consumes one completed set, validates it, and requires an explicitly selected isolated target. Backup paths and alert credentials are private host configuration.

- [ ] Propose daily backups, 14 successful daily sets, and a weekly isolated restore check. Treat a 24-hour maximum scheduled-backup data-loss window as the proposed beta target; measure recovery duration in the drill before promising a recovery-time target. Confirm storage capacity and interruption duration during staging qualification.
- [ ] For the beta, use a brief maintenance window: acquire the same host lock as deployment, stop and drain all ANNOTA writers including background OCR, dump PostgreSQL and archive uploads while writes remain paused, then restart writers even when backup fails. A live `pg_dump` alone does not synchronize external files.
- [ ] Write to a private temporary set with `umask 077`, capture checksums and metadata, and publish the completed set atomically only after every artifact succeeds. Retain the last successful set on failure; remove only expired complete sets after a new success.
- [ ] Copy completed sets to an operator-approved encrypted off-host destination with restricted access. Verify integrity and copy success before retention pruning. A same-host copy alone does not satisfy host-loss recovery.
- [ ] Install a systemd timer with persistent catch-up and an `OnFailure` alert action using the approved private destination. Test notification failure visibly; never count a timer/service exit alone as actionable alert delivery. Configure backup-age monitoring to detect timers that never run.
- [ ] Schedule the weekly restore-check service against disposable resources only, with bounded resource use, cleanup, the shared operations lock, and an alert on failure. Its target allowlist must exclude production database and upload volumes.
- [ ] Restore a synthetic representative user, PDF, image, annotations, preferences, reading progress, and a second user into isolated database/volumes. Check file hashes, foreign-key ownership, API access isolation, readiness, and document/annotation reopening. Do not connect restored sessions, email, or provider calls to production services.
- [ ] Exercise missing archive, checksum mismatch, incompatible dump client/server versions, disk full, interrupted backup, overlapping deploy, permission denied, and failed off-host copy. Restoration refuses destructive production targets by default.
- [ ] Record commands, timestamps, hashes, counts, image digests, actual downtime, recovery duration, and alert receipt in `docs/evidence/issue-43/restore-drill.md` during execution. Use sanitized fixtures and no private user material. Install production scheduling only through the separately approved host action.

**Verification:** Run shell syntax and shellcheck on the scripts, run backup/restore against disposable staging volumes, and verify the recovered material through storage/API checks and desktop/mobile browser screenshots. Use `rtk proxy systemd-analyze verify` for units on Linux and trigger one deliberate staging failure to prove the alert arrives. Record paths to actual evidence rather than checking empty artifact files into the repository.

**Edge Cases:** Application writes during backup, detached OCR still writing, file/DB snapshot mismatch, failed restart, missed timer, full disk, absent off-host storage, expired backups, and restore into the wrong environment.

**Definition of Done:** A scheduled backup produces a protected, complete, off-host set; failure and stale-backup alerts arrive; one isolated restore recovers representative content with correct ownership and measured recovery time.

## Delivery and approval checkpoints

- Approve this plan before application/configuration implementation.
- The owner has approved including the default-branch switch to `dev` after passing checks. Execute it only at the repository-settings checkpoint during authorized implementation; no further branch-choice clarification is needed.
- Implement PR 1 first on an isolated feature branch based on current `dev`. Reference #43 in commits and the PR without closing the whole issue. Do not include unrelated local edits.
- At each subsequent PR, inspect predecessor changes and report tests passed, blocked prerequisites, and remaining #43 acceptance criteria. Register every implementation PR with this T3 thread.
- Changes to the default branch, protected environment, host services, domains, production secrets, and production rollout remain explicit concrete operator checkpoints. Plan approval alone is not a production deployment instruction.
- Final beta release checks include the configured Clerk instance/origins and selected AI routes after their owning issues are ready. Do not duplicate those implementations in #43 or mark #43 complete with these release blockers still unresolved.
- Review all diffs and required checks once per meaningful change; repeat or broaden verification only for new changes, failures, or concrete unresolved concerns.

## Primary references

- [GitHub deployment environments and concurrency](https://docs.github.com/en/actions/how-tos/deploy/configure-and-manage-deployments/control-deployments)
- [GitHub manual workflows and the default-branch requirement](https://docs.github.com/en/actions/how-tos/manage-workflow-runs/manually-run-a-workflow)
- [Bun workspace installation](https://bun.sh/docs/pm/workspaces)
- [Bun frozen installs](https://bun.sh/docs/pm/cli/install)
- [PostgreSQL advisory locks](https://www.postgresql.org/docs/17/explicit-locking.html)
- [PostgreSQL pg_dump snapshot behavior](https://www.postgresql.org/docs/17/app-pgdump.html)
