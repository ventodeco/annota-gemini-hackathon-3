# Issue 43 PR 2: Runtime Release Checks

**Issue:** https://github.com/ventodeco/annota-gemini-hackathon-3/issues/43 (part of #33)

**Status:** Plan v1 (`9ffeeae`) implemented. The user removed the separate plan
review gate on 2026-10-20; the cancelled plan-review task returned no
findings. GPT-6.1 Sol reviews the PR only.

**Base:** `dev` at `59500daf3443b95fa96f089c893ea162d33c3394`.

**Goal:** A started backend leaves the database consistent when migrations fail or
several containers start together, and operators can tell a live process from a
usable one.

**User story:** As the operator, I want failed or overlapping starts to leave
migration state consistent, and readiness to fail while PostgreSQL or Redis is
unavailable, so that a release is accepted only when it can serve users.

## What is already done (not repeated here)

- PR #46: verification-only CI, migrations in the backend image, locked
  workspace frontend image, MIME fixes, final-image smoke checks, and the
  explicit PostgreSQL migration test without a skip.
- PR #48 (#15): `004_add_annotations_scan_id_index.sql` and PostgreSQL
  fresh/upgrade/missing-index coverage. The index item of the original
  plan's Task 3 is complete.
- PR #47 (#36/#37): Go 1.27.3 and Bun 1.4.3 toolchains.

## Scope of this PR

### 1. Atomic, serialized migrations (`backend/internal/storage/migrate.go`)

Current defect: `executeMigration` commits the SQL, then `recordMigration`
writes the ledger through the pool outside that transaction. A failed ledger
write leaves applied DDL with no ledger row; the next start re-runs it.
Two simultaneous starters on a fresh database both see "not applied"; migration
001 uses plain `CREATE TABLE`, so one fails, and `CREATE TABLE IF NOT EXISTS
schema_migrations` itself can race.

Change, keeping `RunMigrations(db *sql.DB, migrationsDir string) error`:

- Fail when the directory contains no `.sql` files (a missing directory
  already fails). Starting with no schema is never valid for this app.
- Run the ledger creation and each migration in its own transaction that first
  takes `pg_advisory_xact_lock(<fixed key>)`. Inside the lock: check the
  ledger, execute the SQL, insert the ledger row, commit. The lock is released
  at commit/rollback, so no session lock can leak onto a pooled connection.
- Bound the lock wait with `SET LOCAL lock_timeout` before the lock call, then
  reset it with `SET LOCAL lock_timeout TO DEFAULT` so migration DDL keeps the
  database's normal behavior. Verified locally on PostgreSQL 17 that
  `lock_timeout` cancels an advisory-lock wait.
- Every query in the runner goes through the transaction; nothing uses the pool
  directly while a transaction is open.
- Applied migrations that are no longer in the image's directory are left
  alone, so a compatible older image can still start (image rollback).
- The runner becomes PostgreSQL-only. Document that migrations must be
  transactional (no `CREATE INDEX CONCURRENTLY`).

Tests (`migrate_test.go`, all PostgreSQL, skipped without
`POSTGRES_TEST_DSN`, each in an isolated schema):

- SQL failure rollback: 002 creates a table then fails; table absent, only 001
  recorded, error returned.
- Ledger failure rollback: a pre-created `schema_migrations` with a CHECK that
  rejects `002_*.sql`; 002's table is absent and unrecorded. This fails on the
  current runner (DDL is committed), establishing the baseline.
- Concurrent starters: four goroutines with separate `*sql.DB` handles
  (`search_path` in the DSN) run the production migrations on a fresh schema;
  all succeed and each migration is recorded exactly once.
- Empty directory: returns an error and records nothing.
- Lock wait bound: hold the advisory lock from another session; `RunMigrations`
  fails within the bound instead of hanging (the bound is a package variable
  shortened in the test).
- Keep `TestRunProductionMigrations_Postgres` and the `readMigrationFiles`
  tests. Remove the three SQLite `RunMigrations` tests, which assert the
  partial-commit defect and cannot use advisory locks. Also remove the unused
  `backend/internal/testutil/helpers.go`, whose only purpose was running
  `RunMigrations` against SQLite (no callers).

### 2. Readiness separate from liveness

- Keep `/healthz` as process liveness (`200 ok`, no dependency checks).
- Add `/readyz` outside auth: PostgreSQL `PingContext` and Redis `PING`, each
  under one 2-second request deadline. `200 ready` when both succeed; otherwise
  `503 not ready`, `Cache-Control: no-store`. The response never includes error
  text, DSNs, or hosts; the failing dependency name and error are logged
  server-side. No AI provider calls.
- Add `Ping(ctx) error` to `storage.RedisClient` (only `redisClientImpl`
  implements it). Handler in `backend/internal/handlers/readiness.go` takes
  named check functions so tests use fakes.
- Tests: both healthy; PostgreSQL failing; Redis failing; a hanging check is
  cut off by the deadline; body has no error detail. Redis `Ping` tested with
  miniredis (up and closed).

### 3. Go static-asset serving (`backend/internal/handlers/frontend.go`)

The catch-all in `main.go` rewrites every non-root path to `/`, so
`/assets/app.js` returns HTML. This path is used only when running the Go
binary next to a built `web/dist` (the production frontend is nginx).

- Serve an existing regular file from the dist directory as is.
- Return `404` for a missing path whose last segment has a file extension
  (missing JS chunk, image, manifest).
- Return `index.html` for other paths (SPA deep links).
- Keep `404` for `/v1/`, `/healthz`, and `/readyz` prefixes, and the existing
  `503` message when `index.html` is absent. Path cleaning prevents traversal.
- Tests with a temp dist: JS/CSS/manifest served with their bodies and types,
  deep link returns the entry document, missing asset 404, reserved prefixes
  404, `../` traversal does not escape, missing build 503.

### 4. Final-image smoke and CI

- `scripts/ops/smoke-images.sh` backend: wait for `/readyz` 200; stop Redis,
  expect `/readyz` 503 while `/healthz` stays 200; start Redis, expect 200;
  repeat with PostgreSQL. Bounded waits, same disposable network and cleanup.
- `.github/workflows/ci-cd.yml`: run every `...Postgres` test in the explicit
  step (`-run 'Postgres$'`) with `-v`, and fail the step if any reports
  `--- SKIP`.

### 5. Docs

- `docs/commercial-ops.md`: readiness vs liveness for deploy checks; migration
  locking, atomicity, transactional-only rule, and failure recovery (fix SQL in
  a new release; failed migration leaves no ledger row and no partial DDL;
  lock timeout means another starter is migrating). Correct the stale Bun
  1.4.0 reference to 1.4.3.
- `backend/docs/openapi.yaml`: add `/readyz`.
- This file: record review task IDs, commands, results, and CI run URLs.

## Out of scope and blocked (issue #43 stays open)

- PR 3, pinned release/staging/rollback: needs administrator protection of
  `ENV_PRODUCTION`, the approved default-branch switch to `dev` (manual
  release workflows must exist on the default branch), and authorized
  read-only inspection of the host (Compose projects, volumes, PostgreSQL
  major, upload paths, Caddy).
- PR 4, scheduled backups/alerts/off-host copies: needs the host convention
  and an approved backup destination and alert channel. A disposable restore
  drill in CI is possible without the host and is a candidate follow-up PR.
- Upload persistence across container replacement, production domains, Clerk
  (#39), and AI routes (#34/#35).

## Overlaps

- #44 monitoring will consume `/readyz`; its contract is defined here.
- #39 (Clerk) may remove Redis; then drop the Redis readiness check with it.
- #40 changes storage transactions and may add migrations, which must follow
  the transactional-only rule.

## Edge cases

Missing or empty migration directory; failed SQL; failed ledger insert;
first-ever simultaneous starts; lock wait exceeding the bound; already-applied
history; older image against newer ledger; dependency lost after startup;
hanging ping; missing asset vs deep link; missing frontend build; traversal.

## Verification

- Local (Go 1.27.3, disposable PostgreSQL 17): focused new tests first and
  confirm the ledger-failure and concurrent tests fail on the current runner,
  then `go test ./...`, `go vet ./...`, `go build ./cmd/server`, `gofmt`.
- `bash -n` on the smoke script (Docker and shellcheck are not installed
  locally).
- CI on PostgreSQL 14: the explicit Postgres step shows every new test passing
  without skip; final-image smoke shows readiness transitions.
- Backend-only change: no screenshots.

**Test-debugging budget:** #43 used about 16.5 of its 30 minutes in PR #46
(02:48:15 to 03:04:54 UTC). About 13.5 minutes remain for this PR, shared with
reviewers, unless the coordinator rules otherwise. Failures are recorded in #43.

## Definition of done for this PR

SQL and ledger agree after success or failure; overlapping starts serialize
with a bounded wait; `/readyz` returns 503 while either dependency is down
and 200 after recovery, while `/healthz` stays 200; Go static serving returns
real assets and 404s missing ones; CI and final-image checks pass; independent
review is clean; evidence is recorded here and in #43.

## Implementation evidence

Local environment: Go 1.27.3 (official darwin-arm64 archive, SHA-256 matched
go.dev), disposable PostgreSQL 17.x cluster on `127.0.0.1:55433`, macOS arm64.
Docker, shellcheck, and actionlint are not installed locally.

Baseline (new tests against the `dev` runner, with only the new lock
identifiers shimmed in so it compiles):

| Test | Result on `dev` runner |
| --- | --- |
| `RollsBackFailedSQL` | pass (single-statement transaction already rolled back) |
| `RollsBackSQLWhenLedgerWriteFails` | FAIL: migration SQL committed without its ledger row |
| `ConcurrentStarters` | FAIL: `duplicate key value violates unique constraint "pg_type_typname_nsp_index"` creating `schema_migrations` |
| `RejectsEmptyDirectory` | FAIL: empty directory accepted |
| `BoundsLockWait` | FAIL: no lock taken |

After the change, every `Postgres$` storage test passed three consecutive
runs. `go test -count=1 ./...` with `POSTGRES_TEST_DSN`, `go vet ./...`,
`go build ./cmd/server`, and `go test -race` for `internal/storage` and
`internal/handlers` pass. Readiness and frontend handler tests pass. YAML
parses; `bash -n` passes on the smoke script; its new HTTP status probe was
exercised against a local HTTP server (404 and 200 matched).

Also verified on PostgreSQL 17 that `lock_timeout` cancels a
`pg_advisory_lock` wait (`canceling statement due to lock timeout`).

Test-debugging time used in this PR so far: none (the baseline failures were
intended reproductions, not debugging). CI evidence, image IDs, and review
task IDs are recorded below as they complete.

### CI and review

- Pending.
