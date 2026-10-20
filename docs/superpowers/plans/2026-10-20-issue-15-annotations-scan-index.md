# Annotation Scan Index Upgrade Implementation Plan

> **For agentic workers:** Use superpowers:executing-plans to implement this plan after owner approval. Steps use checkbox syntax for tracking.

**Goal:** Ensure both new and previously migrated PostgreSQL databases have the annotation `scan_id` index requested in [issue #15](https://github.com/ventodeco/annota-gemini-hackathon-3/issues/15).

**Architecture:** Add an idempotent migration after the existing three migrations. Extend the existing PostgreSQL migration test with fresh-install and upgrade scenarios, using real production SQL and isolated test schemas. Keep query behavior and the migration runner unchanged.

**Tech Stack:** Go, database/sql, lib/pq, PostgreSQL, GitHub Actions.

**Status:** Implementation approved by the owner on 2026-10-20. Migration and regression coverage are implemented, verified, and reviewed. The scoped implementation is ready for integration into `dev`.

## Global Constraints

- Branch: `fix/issue-15-annotations-scan-index`, created from fetched `origin/dev` at `39349ec0ab501bf4798547b8b81ec53a4d02f273`.
- Preserve existing changes in `AGENTS.md`, the issue #43 plan, and `docs/research/`.
- Do not rewrite migrations `001` through `003` or change annotation queries.
- Use production PostgreSQL SQL for migration verification. SQLite is not evidence of PostgreSQL upgrade correctness.
- Prefix shell commands with `rtk`; enter `nix develop` from the repository root before development commands when using the project's Nix environment.
- Test debugging is limited to 30 minutes total, starting at the first failing test or tooling error. Record a blocker in issue #15 with the exact command, first useful failure, suspected cause, attempted fixes, and next action. Never claim a skipped PostgreSQL test passed.
- Owner approved implementation of this plan with “work on it” on 2026-10-20.

## Evidence and decision

`backend/migrations/001_initial_database_schema.sql` already creates `idx_annotations_scan_id`. However, `RunMigrations` in `backend/internal/storage/migrate.go` skips migrations recorded by filename in `schema_migrations`. A database that applied `001` before the index was added never executes that new statement.

`GetAnnotationsByUserIDAndScanID` in `backend/internal/storage/db.go` filters by both user and scan. An index on `scan_id` supplies an access path for scan filtering, although PostgreSQL can legitimately choose a sequential scan for small tables. This plan does not promise a measured latency improvement without production query evidence.

The existing SQLite fixture explicitly includes the index and cannot verify this deployment history. `TestRunProductionMigrations_Postgres` already runs real migrations in a private schema, but currently checks only that the users table exists. CI explicitly executes this test with `POSTGRES_TEST_DSN` before its general backend checks.

**Recommended approach:** Add migration `004` with `CREATE INDEX IF NOT EXISTS`. It repairs older databases and safely accepts databases whose initial migration already created the index.

**Alternatives:** Editing `001` again does not repair deployed databases. A composite index or concurrent index build would require separate workload and migration-runner decisions; neither is necessary to deliver this issue's existing contract.

## Task 1: Deliver the index upgrade with PostgreSQL regression coverage

**User Story:** As a reader with existing saved annotations, I want scan-filtered retrieval to have the intended database index after an upgrade, so that older installations receive the same schema support as new installations.

**Files:**

- Create: `backend/migrations/004_add_annotations_scan_id_index.sql`.
- Modify: `backend/internal/storage/migrate_test.go`, extending `TestRunProductionMigrations_Postgres` and adding only focused test helpers where needed.
- Read: `backend/internal/storage/migrate.go` and `.github/workflows/ci-cd.yml`; no changes are expected in either file.

**Interfaces:** Consume `RunMigrations(db *sql.DB, migrationsDir string) error` and the existing optional `POSTGRES_TEST_DSN` test configuration. Produce a PostgreSQL index named `idx_annotations_scan_id` on `annotations(scan_id)` and a recorded migration named `004_add_annotations_scan_id_index.sql`. No Go or HTTP interface changes.

**Edge Cases:** Fresh database; upgrade with the index absent; upgrade with the index already present; repeated migration run; nullable `scan_id`; pre-existing annotation rows; another schema containing an index with the same name; unavailable test database.

**Definition of Done:** All three installation scenarios finish successfully, expose one valid non-unique single-column index on `annotations(scan_id)` in their own schema, preserve seeded annotation data, and record migration `004` exactly once. A repeat run leaves the migration count and index identity unchanged. The PostgreSQL command must execute without skipping, and backend tests, vet, and build must pass.

- [x] **1. Add the failing upgrade regression before creating migration `004`.** Keep the top-level test name unchanged so the existing CI selector runs every case. Use table-driven subtests for `fresh`, `existing_index`, and `missing_index`. Each subtest must create its own generated schema, set `db.SetMaxOpenConns(1)`, set `search_path`, and clean up the schema before closing its connection, following the existing PostgreSQL test pattern.
- [x] **2. Build an explicit pre-upgrade state.** For both upgrade cases, copy exactly migrations `001_initial_database_schema.sql`, `002_add_documents.sql`, and `003_prd_reader_memory.sql` into a temporary directory, then call `RunMigrations` on that directory. For `missing_index`, execute `DROP INDEX idx_annotations_scan_id` and assert it is absent before applying the full migration directory. This simulates a previously migrated database without modifying historical SQL or deleting its migration records. For `existing_index`, capture the index OID before the upgrade.
- [x] **3. Seed and preserve data.** Before upgrading each existing database, insert a user, a scan, an annotation referencing that scan, and an annotation with `scan_id = NULL`, using returned IDs. After the upgrade, assert the same annotation IDs, text, and scan associations remain. For the fresh case, verify the schema and insert the same sample records after applying migrations.
- [x] **4. Apply the complete production directory and inspect PostgreSQL catalogs.** Retain the users-table check. Verify the index's schema, table, column, validity, and non-unique definition through `pg_namespace`, `pg_class`, `pg_index`, and `pg_attribute`; filter by the generated schema to avoid accepting an index from another test. Check its recorded migration explicitly:

  ```sql
  SELECT COUNT(*) FROM schema_migrations
  WHERE name = '004_add_annotations_scan_id_index.sql';
  ```

  Expect `1`. Capture total migration count and index OID, call `RunMigrations` again, and require both to remain unchanged. In `existing_index`, also require the original index OID to survive the first upgrade. Do not assert that PostgreSQL always uses the index for tiny fixture queries.

- [x] **5. Run the regression against a disposable PostgreSQL database.** Set `POSTGRES_TEST_DSN` privately to that database, never to production. In the backend directory, run:

  ```sh
  rtk proxy go test -count=1 -v ./internal/storage -run '^TestRunProductionMigrations_Postgres$'
  ```

  Before the migration is added, the missing-index scenario must fail on the absent index; the fresh and existing-index scenarios must fail on missing migration `004` bookkeeping. An unavailable database or an unset DSN is a blocked check, not a useful regression failure.

- [x] **6. Add only the new migration.** Its complete SQL is:

  ```sql
  -- Ensure databases that applied migration 001 before the index was added receive it.
  CREATE INDEX IF NOT EXISTS idx_annotations_scan_id ON annotations(scan_id);
  ```

  Retain the existing index statement in `001` so fresh-install history remains unchanged.

- [x] **7. Run focused and required verification.** With a nonempty test DSN, run these commands from `backend/`:

  ```sh
  rtk proxy gofmt -w internal/storage/migrate_test.go
  rtk proxy go test -count=1 -v ./internal/storage -run '^TestRunProductionMigrations_Postgres$'
  rtk proxy go test ./...
  rtk proxy go vet ./...
  rtk proxy go build -o /tmp/annota-issue-15-server ./cmd/server
  rtk git diff --check
  ```

  Review the final diff to confirm only the new migration and focused PostgreSQL coverage implement the issue. Keep the existing SQLite tests unchanged. No browser screenshots or live AI calls are needed for this backend schema change.

- [x] **8. Commit the issue's implementation files after review.** Stage only this plan, the new migration, and the edited test file. Use `fix: apply annotation scan index to existing databases (#15)`. When opening a PR, target `dev` and include the executed verification results; close the issue only after integration.

## Operational risk and scope boundary

Ordinary `CREATE INDEX` runs within the migration runner's transaction and can block writes to `annotations` during the build. For a small beta database, deploy during a quiet window and check table size first. A large or heavily written deployment needs a separate concurrent-index migration design because `CREATE INDEX CONCURRENTLY` cannot run inside this transaction.

`IF NOT EXISTS` checks the relation name, not that a manually created index has the intended definition. The regression checks the definition for supported installation histories; unusual production catalog drift requires inspection before deployment. This task does not repair arbitrary manual drift or change the runner's separate schema/bookkeeping transactions, which belong to broader release work.

## Execution evidence (2026-10-20)

- Used the existing verified Go 1.27.3 toolchain at `/tmp/annota-runtime-upgrade/go/bin` because Go and Nix are absent from the shell PATH.
- Started a disposable PostgreSQL 17.10 cluster under a private temporary directory, using a local Unix socket. No production database or live AI provider was used.
- Before adding migration `004`, the focused PostgreSQL test failed as expected: `missing_index` found no valid index, and `fresh` / `existing_index` found no `004` migration record. Log: `/tmp/annota-issue-15-red.log`.
- After adding migration `004`, `rtk proxy go test -count=1 -v ./internal/storage -run '^TestRunProductionMigrations_Postgres$'` passed all three subtests without skipping. Log: `/tmp/annota-issue-15-green.log`.
- `rtk proxy go test ./...` passed with the disposable PostgreSQL DSN configured. Log: `/tmp/annota-issue-15-tests.log`.
- `rtk proxy go vet ./...`, `rtk proxy go build -o /tmp/annota-issue-15-server ./cmd/server`, and `rtk git diff --check` passed.
- Repeated-run checks compare migration totals before and after rerunning, rather than hardcoding the total number of repository migrations. This preserves the rerun invariant when later migrations are added.
- Existing migrations, runtime queries, the migration runner, and unrelated local changes remain untouched. Production rollout remains separate from this implementation.

- Independent read-only review found no blocking issues. The reviewer checked plan alignment, catalog assertions, data preservation, migration history, and cleanup ordering.
- Confirmed zero remaining test schemas, then stopped the disposable PostgreSQL cluster.
