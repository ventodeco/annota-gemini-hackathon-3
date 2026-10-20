# Issue #40: Stabilize the mobile upload-to-saved-reading journey

Parent: #33. Base: `dev` at 60c59dd (includes #39 Clerk, #42 quotas, #35 provenance and #41/PR67 privacy and deletion cleanup).

**User story:** As a mobile learner, I want to upload a page or PDF, select text, get an explanation, hear it, save it and return later, so that I can study without losing progress.

## Already delivered elsewhere (not redone here)

- **Cleanup of failed uploads and deletions** (#41, PR67):
  - A failed image or PDF upload leaves no row and no file behind.
  - Scan and document deletes remove files first and can be retried.
  - Account locks are in place.
  - Annotations are kept after their source is deleted.
- **Clerk Bearer authentication** (#39) and **per-feature quotas** (#42).
- **OCR provenance** (#35). PR71 adds `usage.Calls(ctx)` to the `UpdateScanOCR` call, and that argument must be kept.

## Problems found on dev 60c59dd

1. `processOCR` writes the failure with the same 5-minute `ocrCtx`. After a provider timeout the context has already expired, so the write fails and the scan stays `processing` forever. A failure of `UpdateScanOCR` also leaves the scan in `processing`.
2. A restart during OCR leaves the scan in `processing` forever.
3. OCR that returns empty text produces a `ready` scan with no text. `ScanPage` then shows a spinner that never ends.
4. `LoadingPage` moves to `ScanPage` after 30 seconds, but `ScanPage` has polling turned off (`pollIntervalMs: 0`). A direct link to a processing scan also never refreshes. Failures show raw codes such as `Scan failed: ocr_failed`.
5. `DocumentPage` reuses `bridgeScanId` after a scroll-driven page change. Explain and Save can then attach to the previous page. The page is taken when Explain is pressed, not when the text was selected.
6. `createScanFromPageHandler` checks for an existing scan and then inserts without any guard. Two tabs, or a double tap, can create duplicate page scans. A database error during the check is also ignored, and the handler inserts anyway.
7. Reader View shows "No selectable text" when the page-text request fails, which mixes up request failures with genuinely textless pages. The app never says that image-only (scanned) PDFs are unsupported.
8. A malformed or zero-page PDF returns HTTP 500 "Failed to read PDF page count".
9. Profile says the preferred language is "Used for OCR and annotation responses", but every annotation prompt is fixed to English.
10. History rows open only on click: they are a `<div onClick>` with no keyboard access. The pagination buttons are smaller than 44px.

## Scope

### Backend

- **`handlers/scan.go` `processOCR`** (terminal writes only):
  - Use a separate bounded context for the final write: `context.WithTimeout(context.WithoutCancel(ctx), 10s)`.
  - Failure reasons:
    - `ocr_timeout` when `errors.Is(err, context.DeadlineExceeded)`.
    - `ocr_failed` for any other error, and when `UpdateScanOCR` itself fails.
    - `no_text_found` when the OCR text is empty after trimming. The provider did the work, so the reservation stays charged.
  - Keep the `provenanceFor(...)` argument, including PR71's `usage.Calls`.
- **Read-time recovery in `handlers/scan.go`:**
  - An image scan still in `processing` after `ocrProcessingTimeout` plus a 1-minute margin is persisted as `failed`/`ocr_interrupted` and returned that way.
  - This applies to `GET /v1/scans/{id}` and `GET /v1/scans`.
  - It needs no migration and no change to `main.go`. It is safe with several instances, because no job can still be running past its deadline.
- **`storage/db.go`:** add a new method, `GetOrCreateDocumentPageScan(ctx, scan) (id int64, created bool, err error)`.
  - One transaction takes `pg_advisory_xact_lock(hashtextextended('document-page:'||doc||':'||page, 0))`, returns the existing row if there is one, and inserts otherwise.
  - It is not nested inside `WithAccountLock`.
  - It can't deadlock with `DeleteDocumentAndPageScans`: the insert's foreign-key key-share lock waits behind the delete's `FOR UPDATE`, and an insert after the delete fails on the foreign key.
  - `MockDB` gets the same method.
- **`handlers/document.go`:**
  - The page-scan handler uses the new method and returns DB errors as 500 instead of ignoring them.
  - A malformed or zero-page PDF returns 422 "This file couldn't be read as a PDF. Try another file.", inside PR67's `lockedStepFailure`, which still cleans up the row and file.

### Frontend

- **`lib/scanStatus.ts`:** a new `getScanFailureMessage(reason)` that maps each reason code to a recoverable message.
- **`LoadingPage`:**
  - Drop the 30-second jump and keep polling until the scan is ready or failed.
  - Mark the processing state with `role="status"`.
  - On failure, show the mapped message and an "Upload another page" action.
  - When the request itself fails (API outage), show a Retry button.
- **`ScanPage`:**
  - Poll while the scan is processing.
  - A `ready` scan with empty text, or a `no_text_found` failure, shows a "no text" state.
  - Failures use the mapped messages.
- **`ContinuousPDFViewer` and `ExtractedTextReader`:**
  - `onTextSelect(text, pageNumber)` reports the page whose text layer contains the start of the selection.
  - Reader View gets `isError` and `onRetry`, so a failed request is shown separately from a page with no text.
  - The no-text message says that scanned (image-only) PDFs are not supported yet and suggests photographing the page instead.
- **`DocumentPage`:**
  - Keep `selectedPage` alongside the selection, and resolve the bridge scan for that page.
  - Cache the bridge scan only as `{page, scanId}`. The server call is idempotent anyway.
- **`useAnnotationDrawerFlow`:** use a request-generation guard. When the page or reader view changes, or the drawer is reset, a pending Explain result is dropped instead of opening the drawer for an old selection.
- **`WelcomePage`:**
  - Add a short note that PDFs need selectable text.
  - Give the error `role="alert"`.
  - Reset the file inputs so the same file can be chosen again after an error.
- **`ProfilePage`:** change the copy to "Saved for future use. Explanations are currently in English." This removes the false promise.
- **`HistoryPage`:**
  - The row content becomes a `<button type="button">` with a visible focus ring, and the delete button sits beside it as a separate control.
  - The pagination buttons get at least 44px touch targets and `type="button"`.

## Edge cases covered

- A direct link to a scan that is still processing.
- Provider timeout, and a restart in the middle of OCR.
- An empty OCR result.
- A malformed or zero-page PDF, and an image-only PDF page.
- Changing page or view while Explain is pending, and Explain after scrolling to another page.
- Two tabs creating the same page scan.
- API outage while polling, or while loading Reader View text.
- Opening the source after it was deleted: the existing "not found" page, with the annotation kept by PR67.

## Out of scope (recorded on the issue)

- **Honoring the explanation language.** It needs prompt changes and `PromptVersion` bumps in #35's area.
- **OCR for image-only PDFs.**
- **The usage reservation of an interrupted OCR job** stays charged until the daily reset. This is how #42 is designed.
- **Live Clerk and provider smoke checks**, and **real iOS Safari and Android Chrome device behavior.** These wait on the owner's keys and devices. Viewport-only screenshots will be labeled as such.
- **`AnalyzeWithLanguageAPI`**, which is not routed and has no metering. It is dead code and is reported to the coordinator.

## Acceptance criteria

1. A scan that times out, is interrupted, or finds no text reaches `failed` with a specific reason, without a reload. Polling pages show accurate, recoverable messages.
2. A direct link to a processing scan updates to ready or failed by itself.
3. After scrolling, Explain and Save use the page where the text was selected. A pending Explain never opens after a page or view change.
4. Concurrent page-scan creation produces one scan row. This is proven on real PostgreSQL.
5. A malformed PDF returns 422 and leaves no row or file. Reader View tells a request error apart from a textless page, and the PDF support scope is stated.
6. The Profile language copy is accurate. History rows work with the keyboard.

## Verification

- **Go:**
  - Focused handler tests for the timeout, interrupted, no-text and malformed-PDF cases.
  - A PostgreSQL concurrency test using `POSTGRES_TEST_DSN` (a local PostgreSQL 17 is running).
  - Then `go vet ./...` and `go test ./...`, and a re-run of PR67's privacy PostgreSQL tests.
- **Web:**
  - Focused Vitest tests for LoadingPage, ScanPage, DocumentPage, ExtractedTextReader, HistoryPage (Tab and Enter) and the drawer-flow guard.
  - Then `bun run test`, `bun run lint` and `bun run build`.
- **Screenshots:**
  - Playwright Chromium at 1280×800 and 390×844.
  - They come from a temporary, uncommitted Vite harness that renders the real pages with fixture data, as in #42. They are not live Clerk sessions.
  - Saved to `docs/screenshots/issue-40/`.
- **Test-debugging budget:** 30 minutes in total for this issue.
