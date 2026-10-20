# Issue #41: Beta privacy, account isolation and data-deletion readiness

Base: `dev` @ `1a51fa8`. This includes Clerk (#39 / PR #59), atomic PG migrations (#43 / PR #55), the production
DB password guard (#12 / PR #60), and route qualification (#34 / PR #63).

## User story

As a reader, I want my documents and history accessible only to my account, and removable when I delete them, so
that I can trust ANNOTA with my learning material.

## What already holds (verified in source, not re-implemented)

- Every `/v1/*` route sits behind Clerk `AuthMiddleware`. Handlers use the stable numeric `users.id` from
  `middleware.GetUserID`.
- Scan, document, annotation, page, file, progress and AI-context reads all compare `UserID` with the caller.
  List queries filter by `user_id`. Mutations use `WHERE id = $1 AND user_id = $2`. A cross-account ID gets 403
  (`Access denied`). It never returns content.
- There is no public `/uploads/` route. Scan images are served only through `/v1/scans/{id}/image` after the
  owner check.
- `DELETE /v1/users/me` (#39) deletes files first, then the user row plus the tombstone in one transaction, then
  the Clerk user. A failure at the Clerk step returns 502, and the retry is idempotent. FK cascades remove scans,
  documents and annotations.
- The web app clears the React Query cache and `api-cache` on sign-out, account switch and deletion (#39).
  Late 401s are bound to the session that sent them.
- Background OCR only updates the scan row by ID and writes no files. If the scan or account is deleted first,
  the update matches no row, and nothing is recreated.
- Logs record the method, path, status, user ID and short JSON error bodies. They don't record request bodies,
  query strings, OCR text, images or tokens. Analytics events are allow-listed to `name` + `source`.

## Concrete gaps found

1. **Browser HTTP cache.** `/v1/scans/{id}/image` sends `Cache-Control: private, max-age=300`. The browser can
   serve account A's image to account B on the same browser via a copied URL, and keep a deleted image for up
   to 5 minutes. JSON API responses send no cache directive.
2. **Service-worker runtime cache.** `vite.config.ts` caches every `https://api.*` response in a
   URL-keyed `NetworkFirst` `api-cache`. That cache is not account-specific. Offline or slow networks fall back
   to the previous account's responses. Existing installs keep the cache until a sign-out clears it.
3. **Malformed-upload orphans.**
   - `POST /v1/documents` creates the row, then writes `documents/{id}.pdf`, then reads the page count. When the
     PDF is malformed, it returns 500 but leaves a row with an empty `file_url` plus the file on disk.
   - Account deletion skips documents whose `file_url` is empty, so that PDF survives account deletion.
   - `POST /v1/scans` has the same problem when the image write fails: the row stays stuck in `processing`.
4. **Partial file-deletion failures.**
   - `DELETE /v1/scans/{id}` logs a failed image removal and deletes the row anyway. The file then becomes
     unreachable, so account deletion can no longer find it.
   - It also skips image removal entirely when `image_url` is empty.
   - `DELETE /v1/documents/{id}` deletes the rows first and only warns if the PDF removal fails.
5. **Privacy page drift.** It names only Google Gemini as the AI processor, and says ANNOTA stores
   "pronunciation requests" (speech is not stored). It doesn't mention annotation retention after source
   deletion, backups, or logs.

6. **Raw provider text in logs (found during implementation).**
   - In legacy (non-routed) AI mode, `ProviderError.Message` held the raw OpenRouter/MiniMax HTTP error body,
     the Gemini SDK error text (which includes the response body), or MiniMax `status_msg`.
   - Handlers log `err.Error()`, so provider echoes of content or credentials could reach the logs.
   - API responses were already generic, and routed mode already used fixed messages.
   - Fix: at the existing `ProviderError` boundary, keep only kind and status (the Gemini cause is kept
     unlogged), with sentinel tests.

7. **Sol R1 findings (fixed).**
   - Images HTTP-cached by earlier builds: authenticated fetches use `cache: 'no-store'`.
   - A page scan inserted between the two delete statements: `DeleteDocumentAndPageScans` deletes in one
     transaction behind a `FOR UPDATE` row lock.
   - The PDF parser error quoted file bytes into logs: the log line is now fixed text.
   - An upload could publish its file after account deletion listed the files: a per-account advisory lock is
     taken shared by uploads and exclusive by deletion. This supersedes the earlier "residual race, no
     locking" decision.

8. **Sol R2 findings.**
   - The lock was released on request cancellation (`BeginTx(ctx)` auto-rollback). Fixed: the lock transaction
     uses `context.WithoutCancel`, while waiting for the lock still honors ctx.
   - Legacy HTTP cache entries could be opened by direct navigation. Fixed: `Clear-Site-Data: "cache"` on the
     profile response, which runs at each sign-in and account switch.
   - Multipart parser errors were logged. Fixed: fixed-text logs on both upload routes.
   - Pool starvation with capped `MaxOpenConns`: documented. Production leaves the pool unlimited.
     R4 assessed it as non-blocking-unresolved: with an uncapped pool, PostgreSQL's `max_connections`
     makes attempts fail fast, so there is no self-sustaining deadlock, but availability can drop under
     load. The runbook now says that a capped pool also needs admission limits on lock holders and waiters.

9. **Sol R3 findings.**
   - Cross-origin `Clear-Site-Data` was ignored on an uncredentialed fetch. Fixed: the profile request uses
     `credentials: 'include'`.
   - A single scan or document deletion could race an upload. Fixed: those deletions take the exclusive
     account lock.
   - Offset pagination in the account purge could skip files. Fixed: unpaginated `GetUserUploads`.
   - A MockDB map race in tests. Fixed: `DeleteUser` takes the mutex.
   - R2-4 (pool starvation) remains unresolved in code. R3 assessed it as non-blocking for the current
     uncapped pool:
     - the PostgreSQL `max_connections` limit makes extra connection attempts fail fast;
     - the upload then returns 500 and releases its lock;
     - a capped pool would need sizing per the runbook.

10. **Sol R4: APPROVE** at `07d96fa`.
    - All R1–R3 findings are resolved; R2-4 is non-blocking-unresolved.
    - The minor pool-sizing doc wording was corrected after approval.

## Scope (files)

Backend:

- `internal/middleware/cache.go` (new, small): `NoStore` sets `Cache-Control: no-store` on every `/v1/`
  response, including auth errors. Wired in `cmd/server/main.go` around the authenticated mux.
- `internal/handlers/scan.go`:
  - The image response uses `no-store`.
  - Create removes the scan row and any partial file when the image write fails.
  - Delete always removes the stored image candidates, and returns 500 while keeping the row when removal
    fails, so a retry can finish.
- `internal/handlers/document.go`:
  - Upload removes the PDF and the row when save, page count or the record update fails.
  - Delete removes the PDF first and returns 500, keeping the rows, on failure. It then deletes the page scans
    and the document.
  - It uses the deterministic `documents/{id}.pdf` path when `file_url` is empty.
- `internal/handlers/user.go`: account deletion uses the same deterministic PDF path for empty `file_url` rows.
- Tests:
  - A two-account handler matrix covering scans, images, documents, pages, page-scan, file, progress,
    annotations and AI context.
  - Orphan cleanup and file-first deletion/retry tests.
  - A `no-store` test.
  - A PostgreSQL test (`POSTGRES_TEST_DSN`) for account-deletion cascade versus the other account, and for
    annotation retention after scan/document deletion.
  - Uses the existing `openPostgresPoolInSchema` + `RunMigrations`. It doesn't define #6's
    `newMigratedPostgresStore`.

Frontend:

- `web/vite.config.ts`: remove the `api-cache` runtime route. Authenticated API responses are never stored by
  the service worker. The precache (app shell only) is unchanged.
- `web/src/lib/authSession.ts` + `web/src/main.tsx`: on startup, purge data left by earlier builds: the
  `api-cache` Cache Storage entry and the legacy `auth_token` / `savedAnnotations` localStorage keys. These hold
  account data that isn't scoped to anyone. `clearUserScopedCaches` keeps clearing the same entries.
- `web/src/pages/LegalPage.tsx` (+ test): describe the actual data, Clerk, the configurable AI providers
  (OpenRouter, UCloud AstraFlow ModelVerse, MiniMax, Google Gemini), logs, deletion semantics, annotation
  retention and backups. It makes no zero-retention, regional or compliance claims, and no invented contact.

Docs:

- `docs/commercial-ops.md` (Support And Data Requests) and `docs/clerk-auth.md` (Deletion):
  - a data map;
  - file-first deletion and retry for scans/documents;
  - orphan handling;
  - the annotation retention rule;
  - backup retention, which is an owner decision;
  - remaining owner actions.
- `docs/evidence/issue-41/privacy-verification.md`: commands, results and screenshots (redacted).

Out of scope:

- auth changes or migrations (no new migration is needed);
- quotas (#42);
- service-layer refactor (#14);
- general storage CRUD tests (#6);
- Nix;
- production config.

## Decisions

- **Annotations survive source deletion.**
  - The PRD lists deleting documents and deleting annotations as separate actions, and saved annotations carry
    their own source snapshot (selected text, context, explanation).
  - Deleting a scan sets `annotations.scan_id` to NULL. Deleting a document leaves `document_id` as a historical
    reference (there is no FK; BIGSERIAL IDs are never reused, so this can't point at another account's row).
  - The annotation is removed when the user deletes it or deletes their account.
  - The privacy page says this, and tests pin it.
- **Cross-account responses stay 403.** That's the existing contract and test. Content never leaks; the
  existence oracle on sequential IDs is accepted for beta.
- **Concurrent upload vs. account deletion.** A scan or document upload that starts before account deletion
  and writes its file after deletion has listed files can leave an orphan. This is a residual window of a few
  milliseconds. No locking is added.
- **Orphan sweep.**
  - Earlier builds already left orphans that no row can reach: malformed uploads and warn-and-continue
    deletes.
  - The runbook adds a read-only operator listing of numeric-named files under `UPLOAD_DIR` and
    `UPLOAD_DIR/documents` that have no matching `scans`/`documents` row.
  - The operator runs the listing and does the deleting; the deletion is not automated.
- **In-flight OCR.** The OCR call uses a detached 5-minute context, so content may already be with the AI
  provider when the user deletes. The result is discarded (the update matches no row), but processing isn't
  stopped. The runbook and privacy page say so.
- **Providers.** The privacy page leads with the issue-named providers:
  - UCloud AstraFlow ModelVerse;
  - OpenRouter, which forwards requests to the provider of the routed model.

  It then names MiniMax and Google Gemini as other supported configurations. Confirming the production
  `AI_PROVIDER` and routing profiles is an owner criterion.
- **Deletion order for a document:** the PDF file first, then the page scans (before the document, because
  `scans.document_id` is `ON DELETE SET NULL` and would otherwise orphan the extracted text), then the
  document. If a step fails, it returns 500, and a retry finishes the remaining steps.
- **Test proof.**
  - Deletion is proven by stable IDs and `os.Stat` on the exact file paths, not by disappearance from parent
    queries. Each cross-account attempt asserts the status, the absence of B's content in the body, and that
    B's row and file are intact.
  - Each production fix is reverted once to confirm its test fails.

## Edge cases covered

- Two accounts on one browser.
- Copied content URL.
- Cached data during an outage (no SW API cache; `no-store`).
- Deleted account (tombstone; only the `DELETE` retry is allowed, #39).
- Partially failed file deletion (fail closed, retryable).
- Malformed upload.
- Source deletion with a retained annotation snapshot.
- Late/stale token (#39 session binding).
- Provider policy differing by model (the disclosure is per configured provider; no guarantees).

## Acceptance criteria

1. Account A gets no B content from any scan/document/annotation/file/page/progress/AI-context route; tests
   prove it.
2. `/v1/*` responses are `no-store`. The SW has no API runtime cache. Legacy `api-cache` is deleted on startup.
   A real browser check confirms both.
3. Failed or malformed uploads leave no row or file behind. Account deletion removes PDFs even for empty
   `file_url` rows.
4. Scan/document deletion removes files first and is retryable on failure.
5. PostgreSQL: account deletion removes all of A's rows and leaves B's. Annotation retention after source
   deletion is as decided.
6. The privacy page and runbook match the actual behavior. Desktop and mobile screenshots of the privacy page
   are captured.
7. The built bundle contains no secret-key patterns. `backend/.env` / `web/.env` are ignored and untracked.

## Remaining owner / external criteria (kept open on the issue)

- Live two-account Clerk sign-in, switch and deletion on desktop and mobile, including Clerk-side deletion. No
  live keys are available here.
- Operator contact details and the confirmed backup/retention choices for the privacy page.
- Confirmation that the production `AI_PROVIDER` and routing profiles match the privacy page.
- Review of the selected provider and model policies.
- #12 revocation/rotation operator evidence.
- Production HTTPS/origin configuration evidence.

## Verification

- `cd backend && go test ./...`, plus `go vet ./...`.
- `POSTGRES_TEST_DSN` storage tests against local PostgreSQL.
- `cd web && bun run test`, `bun run lint`, `bun run build`.
- Grep `web/dist` for `sk_live_|sk_test_|API_KEY`. Inspect the generated `sw.js` for any API route.
- Real Chromium (built preview):
  - seed `api-cache`, reload, and confirm it's gone;
  - confirm no API runtime route exists;
  - screenshot the privacy page at 1280×800 and 390×844, through a temporary, uncommitted harness entry if
    Clerk keys block the route (the #16 precedent).
- Test-debug budget: 30 minutes total for this issue.
