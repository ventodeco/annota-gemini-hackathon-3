# Issue #41 privacy, isolation and deletion verification

Date: 2026-10-20.

Scope:

- Repository changes on `backlog/issue-41`.
- Synthetic test data only: no real accounts, documents, keys or tokens.
- No live Clerk instance was available. Real sign-in, account switching and Clerk-side deletion are **not**
  verified here.

## Backend (Go 1.27.3)

| Check | Result |
| --- | --- |
| `go vet ./...` | clean |
| `go test ./...` | all packages pass |
| `POSTGRES_TEST_DSN=… go test ./internal/storage/ -run Postgres` (local PostgreSQL 17) | pass, including `TestAccountDeletionIsolationAndRetention_Postgres` |

The tests that cover #41 behavior:

- **`TestCrossAccountRequestsCannotReadOrChangeOwnerData`** (handlers).
  - Account A sends 18 requests against B's scan, image, document, file, page text, page scan, progress,
    annotations and AI context.
  - Every request is refused or returns an empty list, and no response body contains any of B's private
    markers.
  - Afterwards, B's rows are unchanged when read by ID, and B's image and PDF still exist on disk.
- **`TestScanUploadFailureLeavesNoRecordOrFile`, `TestMalformedPDFUploadLeavesNoRecordOrFile`.**
  - After a failed image write or a malformed PDF, both the row and the file are gone (`os.Stat`).
- **`TestScanDeleteKeepsRecordUntilImageIsRemoved`, `TestDocumentDeleteRemovesFileThenPageScansAndRetries`.**
  - When file removal fails, the API returns 500 and keeps the rows.
  - The retry returns 204 and removes the file, the page scans and the document, each checked by ID.
  - The saved annotation is retained.
- **`TestDocumentDeleteRemovesPDFOfRecordWithoutFileURL`, `TestDeleteAccountRemovesFilesOfIncompleteUploads`.**
  - PDFs and images of records without a stored URL are removed.
  - The other account's files and rows are untouched.
- **`TestScanImageResponseIsNeverStored`, `TestNoStore`, `TestNoStore_CoversAuthMiddlewareRejection`.**
  - The image route, normal responses and the auth middleware's 401 all send `Cache-Control: no-store`.
- **`TestAccountDeletionIsolationAndRetention_Postgres`.**
  - On the real migrated schema, document deletion removes its page scans by ID and keeps the annotation.
    `scan_id` becomes NULL, and `document_id` stays as a historical reference.
  - A non-owner's delete of each row type fails.
  - Account deletion removes every row of A by ID, writes the tombstone, and leaves B's user, scans, document
    and annotation present.
  - A repeated local deletion returns `sql.ErrNoRows`.
- **`TestLegacyHTTPErrorsDoNotKeepProviderBodies`, `TestMiniMaxStatusErrorsDoNotKeepProviderMessage`,
  `TestGeminiErrorsDoNotKeepProviderMessage`.**
  - Sentinel credential and content strings in the provider's error body never appear in
    `ProviderError.Error()` or `Message`.
  - Kind and status classification are preserved.

After the Sol R1 review, these tests were added:

- `TestAccountDeletionWaitsForInFlightUpload` (PDF and image, `-race`):
  - an upload is paused between creating its row and writing its file;
  - account deletion does not finish while the upload is paused;
  - after release, the upload completes, the deletion returns 204, and the file and row are gone.
- `TestMalformedPDFParseErrorIsNotLogged`: a sentinel inside a parser error never reaches the captured log or
  the response.
- `TestDeleteDocumentAndPageScans_ConcurrentPageScan_Postgres`:
  - a page-scan INSERT is open (uncommitted) while the document is deleted;
  - the deletion waits, then removes that scan by ID.
- `TestWithAccountLock_ExclusiveWaitsForShared_Postgres`:
  - deletion (exclusive) waits for an upload (shared);
  - shared holders don't block each other, and other accounts are unaffected.
- Web `api.test.ts`: every authenticated request uses `cache: 'no-store'`.

After the Sol R2 review, these were added:

- `TestWithAccountLock_HeldAfterRequestCancellation_Postgres`: account deletion can't take the lock while a
  cancelled upload is still running. It fails if the lock transaction uses the request context.
- `TestWithAccountLock_WaitHonorsCancellation_Postgres`.
- `TestMalformedMultipartUploadIsNotLogged` (PDF and image routes, with a sentinel in a malformed part
  header).
- `TestProfileResponseClearsBrowserHTTPCache`.
- A Chromium probe on a synthetic local server:
  - an image served with `private, max-age=300` was reopened by direct navigation from cache, with 1 server
    hit;
  - after a `fetch()` response carrying `Clear-Site-Data: "cache"`, the same navigation went back to the
    server, with 2 hits.
  - This was same-origin on 127.0.0.1. Production needs HTTPS.

After the Sol R3 review, these were added:

- `TestSourceDeletionWaitsForInFlightUpload` (document and scan). It fails if those deletions take the lock as
  shared.
- `TestGetUserUploads_ListsEveryRowOfOneUser_Postgres` (120 of each; the other account is excluded).
- `api.test.ts`: the profile request uses `credentials: 'include'`.
- Handlers pass `go test -race -count=5`.
- A cross-origin Chromium probe: page at 127.0.0.1:4745, synthetic API at localhost:4746.
  - The image was cached with `private, max-age=300`.
  - An uncredentialed `fetch` of `/me` carrying `Clear-Site-Data: "cache"` left it cached: the next direct
    navigation got 1 server hit.
  - The credentialed `fetch` cleared it: the next navigation got 2 server hits.

Mutation check: each production fix was reverted once, and its test failed. This includes the R1 fixes:

- removing `FOR UPDATE`;
- taking the account lock as shared in deletion;
- logging the parser error;
- dropping `cache: 'no-store'`.

The earlier fixes checked the same way:

- scan create/delete;
- document upload/delete;
- account PDF fallback;
- image `no-store`;
- the three legacy AI error paths.

### Raw provider text in logs (source evidence)

Before this change, the legacy (non-routed) client put provider text into `ProviderError.Message`:

- `internal/ai/client.go` `classifyHTTPError` stored `strings.TrimSpace(body)`, the raw OpenRouter/MiniMax HTTP
  error body;
- `wrapGeminiError` formatted the SDK error, whose `genai.APIError.Message` is the raw response body
  (`genai@v1.75.0/api_client.go` `newAPIError`);
- MiniMax TTS stored `base_resp.status_msg`.

`ProviderError.Error()` includes `Message`. Handlers log it through `logger.ErrorWithErr`, which writes
`err.Error()`.

API responses were already generic (`writeAIProviderError`), and routed mode already used fixed messages. Now
the legacy paths keep only the classification and status code. The Gemini SDK error stays available as the
unlogged `Cause`.

## Web

| Check | Result |
| --- | --- |
| `bun run test` | 44 files pass, 1 skipped (existing) |
| `bun run lint` | clean |
| `bun run build` | succeeds |

Built `dist/sw.js` registers only the navigation route. It contains no `api-cache` and no `NetworkFirst`.

Grepping `dist/` for `sk_live_`, `sk_test_`, `CLERK_SECRET`, `OPENROUTER_API_KEY`, `MINIMAX_API_KEY`,
`GEMINI_API_KEY` and private-key headers finds no matches.

### Real browser (Chromium, built `vite preview` on 127.0.0.1)

Without a Clerk key, the app shows "Sign-in is not configured". The service worker still registers and
controls the page.

1. With the service worker active, seed data that an older build could leave behind:
   - an `api-cache` entry for a synthetic `https://api.example.test/v1/documents` response;
   - `localStorage` keys `savedAnnotations` and `auth_token`, both with synthetic values.
2. Reload.
3. Caches are now only `workbox-precache-v2-…`, and `localStorage` is empty.
4. Fetch `/v1/documents` through the controlling service worker: no `/v1` entry exists in any cache.

### Privacy page screenshots

These come from a temporary, uncommitted Vite entry that renders the real `LegalPage` (the #16 precedent). The
real `/privacy` route sits behind Clerk configuration.

- [`privacy-desktop.png`](../../screenshots/issue-41/privacy-desktop.png) (1280×800).
- [`privacy-mobile-top.png`](../../screenshots/issue-41/privacy-mobile-top.png) and
  [`privacy-mobile-deletion.png`](../../screenshots/issue-41/privacy-mobile-deletion.png) (390×844). There is
  no horizontal overflow.
- The "Back to ANNOTA" link measures 127×44 px.

## Not verified here (owner or live credentials)

- Real Clerk two-account sign-in, account switch and deletion on desktop and mobile, including Clerk-side user
  removal.
- That production `AI_PROVIDER` and routing profiles match the Privacy Policy, plus a review of provider
  data-use and retention terms.
- Operator contact details and the backup retention period.
- #12 credential revocation/rotation evidence.
- Production HTTPS origin configuration.
- Repository check only: `backend/.env` and `web/.env` are ignored by git, and only the `.env.example` files
  are tracked. This does not prove any credential was rotated.
