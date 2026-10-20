# Issue #6: Storage layer tests for database operations

## Outcome

Storage behavior that the application depends on is checked against the real PostgreSQL schema, not just a handwritten SQLite schema. If a document, page-scan, annotation, ownership or account-deletion query regresses, a test that runs in the existing PostgreSQL CI step fails. This is not a coverage quota or a test-suite rewrite.

## Current state (latest `dev`, `607f75a`)

The issue text is stale. Storage tests already exist:

- `db_test.go`: user/scan/annotation CRUD against an in-memory **SQLite** schema that is handwritten to mirror the migrations. It is useful as a fast unit check and stays as is. It cannot catch PostgreSQL-only behavior: JSONB, `TIMESTAMPTZ`, real foreign keys and cascades, `COALESCE` ordering, or schema drift from the migrations.
- `migrate_test.go`: the PostgreSQL migration runner is covered: idempotent rerun, legacy upgrade, rollback, ledger failure, concurrent starters, lock bound and empty directory.
- `clerk_identity_test.go`: Clerk provisioning, tombstones and scan cascade on PostgreSQL.
- `redis_test.go`: state set/get/delete, key prefix, ping and context deadline (miniredis).
- `file_test.go`: image save/open/delete, MIME extension and SHA-256.
- `internal/testutil/helpers.go` (SQLite migration helpers) was deliberately removed by #43 and stays removed. `MockDB` is for handler tests. Storage tests exercise the real implementation, so the issue's "use MockDB/helpers" criterion is superseded.

The following production paths are **not tested anywhere**:

- Document storage: `CreateDocument`, `GetDocumentByID`, `GetDocumentsByUserID`, `UpdateDocumentFileInfo`, `UpdateDocumentProgress`, `DeleteScansByDocument`, `DeleteDocument`.
- PDF page scans: `CreateScanFromDocument`, `GetScanByDocumentPage`, `UpdateScanStatus`.
- `GetAnnotationsByUserIDAndDocumentPage`, which builds its SQL placeholders dynamically.
- Annotation JSONB `nuance_data` round trip on PostgreSQL.
- `SavePDF` / `OpenPDF` / `DeletePDF`.
- Redis state TTL expiry.

## User stories

- As a reader, my PDF documents, reading progress and page scans are stored and listed correctly, and another account can never change or delete them.
- As a reader, deleting a document removes the document and its page scans.

## Scope and files

1. `backend/internal/storage/db_postgres_test.go` (new). Every test name ends in `_Postgres`, so CI's no-skip PostgreSQL step runs it. Each test gets a fresh schema through the existing `openPostgresPoolInSchema` and `postgresTestDSN` helpers, and the real migrations are applied with `RunMigrations("../../migrations")`. Schemas are dropped on cleanup. Timestamps are explicit and distinct, so ordering is deterministic. Cases:
   - **Document lifecycle:**
     - Create applies the defaults (`last_page_number=1`, `updated_at=created_at`).
     - Get round-trips all fields; a missing document returns `nil, nil`.
     - `UpdateDocumentFileInfo` persists the new values.
     - `UpdateDocumentProgress` by the owner sets the page and `last_opened_at`. By another user it returns `sql.ErrNoRows` and changes nothing.
     - `GetDocumentsByUserID` is isolated per user, orders by `COALESCE(last_opened_at, created_at) DESC` (a recently opened older document moves first), and paginates.
   - **Page scans:**
     - `CreateScanFromDocument` defaults to `pdf`/`ready`.
     - `GetScanByDocumentPage` finds the page scan; a missing page returns `nil, nil`.
     - `UpdateScanStatus` with a failure reason persists. A later `UpdateScanOCR` sets `ready` and clears the failure reason.
   - **Document deletion CRUD contract:**
     - A non-owner `DeleteDocument` returns `sql.ErrNoRows` and the row stays.
     - The owner's sequence (as in the handler) removes the document and its page scans, leaves the owner's unrelated image scan, and a repeat delete returns `ErrNoRows`.
   - **Annotations:**
     - JSONB nuance data (Japanese text, nested arrays) round-trips.
     - `GetAnnotationsByUserIDAndDocumentPage` with a nil page returns all of the user's annotations for the document. With a page, it filters. Both paginate newest first. Another user's annotation on the same document ID is excluded.
   - User CRUD is not repeated here. `CreateUser` always writes every column, so the SQLite unit tests plus the PostgreSQL Clerk provisioning test already cover it.
2. `backend/internal/storage/file_test.go`: PDF save under `documents/`, read back, delete, idempotent delete of a missing file, and an error when opening a missing file.
3. `backend/internal/storage/redis_test.go`: a state written with a TTL expires (miniredis fast-forward) and `GetState` then returns `redis.Nil`.

Source changes happen only if a new test exposes a real bug. Any such fix stays minimal and comes with its regression test.

## Out of scope

- #41 (active) owns owner isolation, account and document deletion data retention, cache privacy and their production fixes. #6 does not test the account deletion cascade or whether annotations are kept after a document is deleted, and adds no isolation suites.
- Service-layer refactor (#14), quotas (#42) and new migrations.
- Resurrecting SQLite migration helpers.
- Rewriting the existing SQLite unit tests.
- Coverage targets.
- Redis features beyond the existing interface (after Clerk, Redis is only a readiness dependency).

## Edge cases

- `nil, nil` versus `sql.ErrNoRows` conventions differ per method (document/page-scan getters return `nil, nil`; annotation get and owner-scoped deletes return `ErrNoRows`). Tests assert each existing contract.
- Pagination with the dynamic placeholder path (page filter present or absent).
- Timezone: compare times with `Equal` after round-tripping through `TIMESTAMPTZ`.
- `annotations.document_id` has no foreign key, so after document deletion annotations keep pointing at the deleted document (and `scan_id` becomes NULL). This is left to #41's retention decision. #6 does not assert either way.

## Acceptance criteria

- [ ] PostgreSQL tests cover document, page-scan and annotation-query CRUD, including each method's owner-scoped contract, on the real migrated schema.
- [ ] PDF file lifecycle and Redis TTL expiry are tested.
- [ ] The migration runner (including idempotent rerun) is confirmed covered by the existing `migrate_test.go`; no duplicate tests.
- [ ] The CI PostgreSQL step (`-run 'Postgres$'`, no skips), `go test ./...`, `go vet ./...` and `gofmt` pass.

## Verification

- Local: Go 1.27.3 (official checksum-verified tarball in `/tmp`) and a disposable PostgreSQL 17 on `127.0.0.1:55436`, with a URL-form `POSTGRES_TEST_DSN` as in CI.
  - `go test -count=1 -v ./internal/storage -run 'Postgres$'` (no `SKIP`)
  - `go test -race ./internal/storage`
  - `go test ./...`
  - `go vet ./...`
  - `gofmt -l`
- CI: the Backend workflow (PostgreSQL 14 service).
