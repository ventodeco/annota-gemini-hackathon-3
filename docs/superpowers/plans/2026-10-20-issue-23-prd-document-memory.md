# Issue #23: PRD document memory and rich annotations (reconciliation)

Issue: <https://github.com/ventodeco/annota-gemini-hackathon-3/issues/23>
Base: `origin/dev` at `82c2f39`.

## Finish condition

Each item in the #23 slice has file and test evidence on `dev` or is a recorded dependency. The slice items are: scan lifecycle status, document library/progress/delete, page-aware annotation source, the rich pronunciation-first schema, and server-side analyze context.

Only genuinely unmet #23 acceptance owned by this issue is fixed. Items that belong in files #40 owns are handed off, not edited here. This is not a reimplementation: commit `2d09d38` ("feat: implement ANNOTA PRD reader memory (#23)") delivered most of the slice, and later PRs (#41 privacy and deletion, #35 provenance, #42 quotas) hardened it.

## User story

As a learner reading a Japanese PDF I want three things. First, to return to the document library, see my progress, and continue where I stopped. Second, to open the annotations I saved for a document. Third, for each saved annotation to show its reading (kana/romaji), translation and explanation, together with the document and page it came from.

## Per-item evidence (dev `82c2f39`)

| Slice item / PRD bullet | Evidence | Status |
| --- | --- | --- |
| Scan lifecycle `status` / `sourceType` / `failureReason` | Migration `backend/migrations/003_prd_reader_memory.sql` (columns, backfill, `idx_scans_user_status`). `handlers/scan.go` sets `processing` on create (L155) and `failed` plus a reason on OCR failure (L485). `statusOrDefault`/`sourceTypeOrDefault` apply in list and detail. The web polls on status. | Met. Stale-`processing` recovery is #40 (read-time `ocr_interrupted`). |
| Document library list with metadata (filename, pages, last opened, created) | `GET /v1/documents` (`handlers/document.go` `listDocumentsHandler`); `web/src/pages/DocumentsPage.tsx`. Tests: `TestListDocumentsSuccess`, `DocumentsPage.test.tsx`. | Met |
| Continue reading from the last page | `PATCH /v1/documents/{id}/progress` (`updateDocumentProgressHandler`, storage `UpdateDocumentProgress`). `DocumentsPage` opens `?page=lastPageNumber`; `DocumentPage` `initialPage` falls back to `lastPageNumber` and saves progress on page change. Test: `TestUpdateDocumentProgressSuccess`. | Met |
| Last opened time | `last_opened_at` is written only by the progress PATCH, and `DocumentPage.scheduleProgressSave` skips an unchanged page. Opening a document without changing page therefore does not update it. | **Gap, owned by #40** (`DocumentPage`/`document.go`); handed off |
| Delete documents / annotations | `DELETE /v1/documents/{id}` (file first, then the doc and its page scans, atomically; annotations kept, per #41). `DELETE /v1/annotations/{id}`. UI in `DocumentsPage`, `HistoryPage` and `AnnotationDetailPage`. Tests: `TestDeleteDocumentSuccess`, `TestDocumentDeleteRemovesFileThenPageScansAndRetries`, `TestCrossAccountRequestsCannotReadOrChangeOwnerData`. | Met |
| Page-aware source metadata (doc ID, page, selected text, context, created) | `CreateAnnotationAPI` derives `source_type`/`document_id`/`page_number`/`source_label` from the owned scan (`handlers/annotation.go` L138-176, `buildSourceLabel`). `annotations` columns come from migration 003. | Met server-side. Correctness depends on the client sending the scan of the selected page: `DocumentPage` reuses `bridgeScanId` across pages. **#40 dependency** (page identity) |
| View annotations scoped to a document/page | `GET /v1/annotations?documentId=&pageNumber=` (`GetAnnotationsByUserIDAndDocumentPage`). `HistoryPage` reads `documentId`/`pageNumber` (test: "Annotations for document #9, page 3"). | **Gap (#23): no UI entry point.** Nothing links to `/history?documentId=`. |
| Distinguish PDF vs image annotations | `source_label` is "<filename> page N" or "OCR Scan #N", shown in `HistoryPage` and `AnnotationDetailPage`. `OCRHistoryPage` branches on `sourceType`. | Met |
| Pronunciation-first schema (kana required, romaji optional) | `models.NuanceData.Pronunciation`. Provider schemas: `ai/client.go` L683-687 (Gemini, `kana` required) and `ai/chat_tasks.go` L172. Rendered via `lib/annotationFormatting.ts` in `AnnotationCard`, `AnnotationContent` and `AnnotationDetailPage`. The kana round trip is in `provenance_test.go`. | Met. Legacy-field removal is Phase 1.5, not #23. |
| Saved annotation shows translation, explanation, reading and source together, with TTS replay | `AnnotationDetailPage` (source L133, pronunciation L161, "Replay TTS"). | Met |
| Server-side analyze context loading | `AIHandlers.resolveAnalyzeContext` loads the owned scan's `full_ocr_text` and windows it to 12k runes around the selection (`contextWindow`). The client `context` is used only when there is no scanId or no text. No test asserts that the AI receives the server text. | Implemented; **unverified**. One focused test is added (below). Regenerate omits `scanId`, which is **owned by #40** (`useAnnotationDrawerFlow`); handed off. |

## Scope (this PR)

1. **`web/src/pages/DocumentsPage.tsx`:** add a per-document "Annotations" link to `/history?documentId={id}`.
   - Accessible name: "View annotations for {filename}".
   - Touch target of at least 44px, with a visible focus ring.
   - It is a separate control next to Open and Delete.
2. **`web/src/pages/__tests__/DocumentsPage.test.tsx`:** assert the link's accessible name and its `href`.
3. **`backend/internal/handlers/analyze_context_test.go` (new, test only):** one focused test.
   - The question: when analyze gets a `scanId`, does the AI client receive the server-stored scan text, windowed to 12k runes and containing the selection, instead of the client `context`?
   - Why it matters: if it fails, the slice item is unmet and `ai.go` needs a fix.
   - Why existing evidence can't answer it: the #41 privacy tests assert only ownership rejection, and the #35 provenance tests call analyze without a `scanId`.

Files are deliberately limited to these three. Not touched: `HistoryPage`, `DocumentPage`, `useAnnotationDrawerFlow`, `document.go`, `db.go`, `mocks.go`, all of which are #40-owned or in flight.

## Edge cases

- The library is empty: there is no list, so no link.
- A document with no annotations: `HistoryPage` shows its empty state under the document-scoped title.
- A long filename: the link label stays short ("Annotations"). The filename is in the accessible name only.
- Mobile at 375px: the row keeps Open (flexible), Annotations and Delete without overflow.
- Selection not found in the long text: `contextWindow` returns the first 12k runes. This is existing behavior and is not asserted.

## Acceptance criteria

- From the document library, a keyboard or touch user can open the annotation list scoped to that document.
- With a valid owned `scanId`, analyze passes server-stored context to the AI client, not the client context.
- No change to API contracts, schema, quotas (#42), provenance (#35) or privacy/deletion semantics (#41).

## Dependencies and observations (not fixed here)

- **#40 dependencies:**
  - `bridgeScanId` page identity, needed for page-aware source correctness.
  - Regenerate passing `scanId`.
  - `last_opened_at` updating on open.
  - Stale `processing` recovery.
  - Duplicate page scans.
- **Observation (nonblocking):** `POST /v1/ai/analyze` with a foreign `scanId` returns 403, while `GET /v1/scans/{id}` returns 404. This is recorded only; no #41 follow-up and no change to error semantics.
- **Out of scope:**
  - The `scan_not_ready` guard (add-pdf epic).
  - Legacy-field removal and language prompt wiring (Phase 1.5).
  - Existing touch targets, including the delete icon.

## Verification

- `go test ./internal/handlers -run TestAnalyzeLoadsServerScanContext`.
- `bun run test -- DocumentsPage`, `bun run lint` and `bun run build` in `web/`.
- Real browser screenshots of `/documents` at desktop (1280px) and mobile (375px), saved under `docs/screenshots/issue-23/`.
- CI is the full-suite gate. No repeated unchanged suites.

## Verification results

- **`web/`:**
  - `bun run test -- DocumentsPage` passes (2 tests).
  - `bun run lint` is clean.
  - `bun run build` succeeds.
- **Screenshots** in `docs/screenshots/issue-23/` (`documents-annotations-link-desktop.png` at 1280×800, `documents-annotations-link-mobile.png` at 375×812):
  - Captured with Playwright Chromium from a temporary, uncommitted Vite harness. It renders the real `DocumentsPage` with fixture library data and a keyboard-focused link. Live Clerk sign-in is not claimed.
  - The link measures 44px high and resolves to `/history?documentId=9`.
  - The first layout put the link inline, which truncated filenames to about 10 characters at 375px. The link now sits on its own line under the metadata.
- **Backend:** this workstation has no Go toolchain or Docker. That is a local tooling gap, not an app defect. `TestAnalyzeLoadsServerScanContext` runs in the CI Go job.
