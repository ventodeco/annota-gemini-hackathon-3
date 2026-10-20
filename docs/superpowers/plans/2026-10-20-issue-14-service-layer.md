# Issue #14: service layer between handlers and storage

Base: `origin/dev` at `e77cb9f` (after #40 PR75, #42 PR73, #23 PR74).

## User story

As a maintainer, I want scan lifecycle/OCR, annotation management and AI
analysis orchestration to live in services, so HTTP handlers only parse
requests and write responses, and the business rules can be tested and reused
without an `http.ResponseWriter`.

## Finish condition

`ScanHandlers`, `AnnotationHandlers` and the routed `AIHandlers` endpoints
(`AnalyzeAPI`, `SpeakAPI`) no longer call `storage.DB`, `storage.FileStorage`,
`usage.Meter` or `ai.Client` directly. They decode/validate HTTP input, call a
service, and map typed service errors to the exact existing status codes and
messages. Observable API behaviour is unchanged.

## Scope

New package `backend/internal/services`:

- `scan.go` - `ScanService`
  - `CreateImageScan(ctx, userID, image, mimeType)`: reserve OCR outside the
    account lock, create row + save file + set private image URL under the
    shared account lock, discard row/file on save failure, release exactly once
    when nothing was stored, 9m dispatch-window guard (fail scan `ocr_failed` +
    release), absolute OCR deadline `scan.CreatedAt + 5m` derived from
    `WithoutCancel(usageCtx)`, async `ProcessOCR`.
  - `ProcessOCR(ctx, reservation, scanID, image, mimeType)`: unchanged
    semantics - deferred single `Finish`, expired-ctx skip with `Release` and
    `ocr_timeout`, fresh 10s `WithoutCancel` terminal writes, `no_text_found`
    keeps the page charged, provenance from `usage.Calls(ctx)`. No retry.
  - `GetScan`, `ListScans`: owner checks and the 6m conditional
    `ocr_interrupted` read recovery.
  - `ScanImage`: owner check + stored-file lookup.
  - `DeleteScan`: owner check, then file-first delete and row delete under the
    exclusive account lock (row kept on file failure).
  - OCR timing constants and failure reasons move here (exported where tests or
    `main.go` need them). `handlers.OCRReservationStaleAfter` stays as an alias
    so `cmd/server/main.go` is untouched.
- `annotation.go` - `AnnotationService`: create (scan ownership, source
  type/label derivation, echoed provenance acceptance), get (ownership),
  owner-scoped delete, list (scan / document-page / all filter).
- `ai.go` - `AIService`: `Analyze` (user lookup, knowledge lookup, scan-context
  window with ownership, explanation reservation on the request-scoped usage
  context, single `Finish`, provenance with `reportedModel` from
  `usage.Calls(usageCtx)`) and `Speak` (per-rune speech reservation, single
  `Finish`).
- `provenance.go` - `ProvenanceFor` moved from `handlers/provenance.go`.
- `errors.go` - sentinels (`ErrScanNotFound`, `ErrScanForbidden`, ...) and
  `AdmissionError` wrapping usage admission failures for `writeUsageError`.

Handlers keep their constructors (`NewScanHandlers`, `NewAIHandlers`,
`NewAnnotationHandlers(...).WithAIClient(...)`) and build services internally,
so `main.go` wiring and other owners' test call sites do not change.

## Out of scope

- `document.go`, `user.go` (account/document deletion, PDF page scans) - not
  named by the issue; they keep `runWithAccountLock`/`lockedStepFailure`.
- Unrouted dead code (`AnalyzeWithLanguageAPI`, `createAnalyzePrompt`,
  `toNuanceData`) - left as is apart from the provenance helper rename.
- Any API, schema, quota, or frontend change.

## Preserved contracts / edge cases

- Clerk numeric `userID` from `middleware.GetUserID`; 401 still in handlers.
- Reserve before any scan row; never hold the account lock across `Reserve`,
  OCR, or AI calls; lock released before OCR goroutine starts.
- Upload shared lock / delete exclusive lock; file-first retryable deletion.
- Exact error status/messages: e.g. lock failure on upload -> 500
  "Failed to save uploaded image", create-row failure -> 500
  "Failed to initialize scan", foreign scan -> 403 "Access denied",
  analyze user lookup error -> 500 "Database error", usage errors via
  `writeUsageError`, provider errors via `writeAIProviderError`.
- Image responses keep `Cache-Control: no-store`.

## Verification

- Existing handler tests stay as behaviour-preservation regressions
  (OCR lifecycle/deadline, interrupted recovery, quota release/settlement,
  privacy locking, provenance round-trip); only internal call sites change
  from `h.processOCR` to the service.
- `go vet ./...`, `go test ./...` (race in CI) once on the final change; CI
  required gates.
