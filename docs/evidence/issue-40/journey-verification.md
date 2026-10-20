# Issue #40: Upload-to-saved-reading verification (2026-10-20)

Plan: [`docs/superpowers/plans/2026-10-20-issue-40-upload-to-saved-reading.md`](../../superpowers/plans/2026-10-20-issue-40-upload-to-saved-reading.md). Base: `dev` at 764c4c0, which includes #41, #42 (PR72 and PR73) and #35 (PR71).

## What changed and how it is proven

| Case | Behavior now | Proof |
| --- | --- | --- |
| Provider timeout | The final write uses a fresh 10-second context that starts when the write starts. The scan becomes `failed` / `ocr_timeout`. | `TestProcessOCRPersistsTerminalFailure`. Its DB rejects writes made with an expired context, as PostgreSQL does. |
| Slow upload before OCR | The OCR deadline is absolute: `created_at + 5m`. If that deadline has already passed, there is no provider call, the page is released and the scan reads `ocr_timeout`. | `TestOCRDeadlineIsAbsoluteFromScanCreation` (a slow `SaveImage`), `TestExpiredOCRDeadlineReleasesThePage` |
| Restart mid-OCR | `GET /v1/scans/{id}` and `GET /v1/scans` report a scan that is still `processing` after 6 minutes as `ocr_interrupted`. A conditional UPDATE never overwrites a concurrent `ready` result. | `TestInterruptedOCRScanIsReportedFailed`, `TestInterruptedOCRRecoveryKeepsConcurrentResult`, `TestFailStaleProcessingScanIsConditional_Postgres` |
| Empty OCR text | `failed` / `no_text_found`, instead of a `ready` scan with an endless spinner | Go table test, plus `ScanPage` tests for the "ready but empty" case |
| Two tabs creating one page scan | `GetOrCreateDocumentPageScan`: owner-checked `FOR KEY SHARE` on the document, then a per-page advisory transaction lock. Exactly one row is created, and none after the document is deleted. | `TestGetOrCreateDocumentPageScanConcurrent_Postgres` (8 concurrent creators, real PostgreSQL 17) |
| Malformed or zero-page PDF | 422 "This file couldn't be read as a PDF. Try another file."; PR67 still cleans up the row and the file | `privacy_test` assertions updated, plus the `WelcomePage` test |
| Slow OCR in the UI | `LoadingPage` polls until the scan is ready or failed, with no 30-second jump. `ScanPage` polls a processing scan opened from a direct link. Failure reasons map to recoverable copy, and an API outage offers Retry. | `LoadingPage` and `ScanPage` tests |
| Page identity | A selection carries its page. Explain and Save use the selected page after scrolling, and a new page never reuses the previous page's scan. | `DocumentPage` "explains against the page the text was selected on" |
| Page or view change while Explain is pending | A request-generation guard drops the result. Regenerate now also sends `scanId`. | `DocumentPage` "drops a pending explanation". A mutation check (guard removed) made this test fail. |
| Reader View | A request error shows Retry. A textless page states that scanned PDFs are unsupported. | `ExtractedTextReader` tests |
| Explanation language | The Profile copy now says explanations are in English. The preference is saved but not applied. | Screenshot |
| History keyboard access | Rows are buttons with a focus ring, and delete is a separate 44px control | `HistoryPage` Tab and Enter test, plus screenshot |

## Checks run on the final code

- **Backend:**
  - `go vet ./...` is clean.
  - `go test -count=1 ./...` passes 590 tests in 15 packages, with `POSTGRES_TEST_DSN` pointing at local PostgreSQL 17.
  - Go 1.27.3 binary, SHA-256 checked against the official checksum.
  - `gofmt` flags only `internal/httputil/response_test.go`, which this change does not touch.
- **Web:**
  - `bun run test`: 244 passed, 2 skipped.
  - `bun run lint` is clean.
  - `bun run build` succeeds.

## Screenshots

All screenshots are in `docs/screenshots/issue-40/`, at desktop 1280×800 and mobile 390×844 (Playwright Chromium; mobile uses the touch and isMobile flags):

- `scan-timeout`
- `loading-interrupted`
- `scan-processing`
- `reader-error`
- `reader-no-text`
- `welcome-pdf-rejected`
- `history-keyboard-focus`
- `profile-language-copy`

**How they were made:** a temporary, uncommitted Vite harness rendered the real pages. `@clerk/react` was stubbed, and Playwright supplied the `/v1` fixture responses. There is **no live Clerk session, live OCR, or live provider** in these screenshots.

## Still open (owner, live, or device)

- One fresh live user going upload → select → explain → listen → save → reopen → delete, with real Clerk keys and configured AI routes.
- Real **iOS Safari** and **Android Chrome** behavior: selection gestures, safe areas and audio. Only viewport emulation was checked here.
- Honoring the explanation language. That needs prompt and `PromptVersion` work in #35's area.
- OCR for image-only PDFs.
- An interrupted OCR page stays charged until #42's 15-minute reservation expiry. This is documented in `docs/usage-quotas.md`.
