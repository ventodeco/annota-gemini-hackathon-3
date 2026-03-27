## Task List (Phase0 MVP): Image OCR + PDF Reader + Annotation

This is the execution checklist derived from `docs/rfc.md` (Phase0 only). Each task includes **user story**, **acceptance criteria**, and **unit tests** so we can implement quickly and safely.

### Tech preparation (do once)
#### Versions
- **Go**: **Go 1.25.x** (latest stable major is 1.25; choose latest patch from `go.dev/dl`)
- **React**: React 18+ with TypeScript
- **Build**: Vite + Bun
- **UI**: shadcn/ui + Tailwind CSS v4
- **Database**: PostgreSQL
- **PDF**: Go PDF library (`github.com/ledongthuc/pdf` or `github.com/dslipak/pdf`)

#### Local setup checklist
- [ ] Install Go (1.25.x) and verify:
  - [ ] `go version`
  - [ ] `go env GOPATH GOMODCACHE`
- [ ] Initialize module:
  - [ ] `go mod init <module-path>`
- [ ] Frontend setup:
  - [ ] `cd web && bun install`
  - [ ] `cd web && bun run dev` (verify Vite dev server starts)
- [ ] Add required env vars (local `.env` or shell export):
  - [ ] `GEMINI_API_KEY`
  - [ ] `DATABASE_URL` (PostgreSQL connection string)
- [ ] Add a basic dev workflow:
  - [ ] Backend: `cd backend && go test ./...`
  - [ ] Frontend: `cd web && bun run test`
  - [ ] Backend lint: `cd backend && go vet ./...`
  - [ ] Frontend lint: `cd web && bun run lint`

#### Testing scaffolding
- [ ] Adopt interfaces to enable unit testing:
  - [ ] `type GeminiClient interface { OCR(ctx,...); Annotate(ctx,...) }`
  - [ ] `type Storage interface { SaveImage(...); OpenImage(...); SavePDF(...) }`
  - [ ] `type PDFExtractor interface { PageCount(path) int; ExtractText(path, page) string }`
- [ ] Use `net/http/httptest` for handler tests.
- [ ] Use Vitest + React Testing Library for frontend tests.

---

### Phase0 tasks — Image Path (implement in order)

#### T0. Project skeleton (Go server + React SPA)
- **User story**: As a user, I can open the app in mobile browser and see the welcome screen.
- **Acceptance criteria**:
  - App serves React SPA at `GET /`.
  - WelcomePage shows three options: Take Photo, Upload from Gallery, Upload PDF.
  - Basic health endpoint exists: `GET /healthz` returns 200.
- **Unit tests**:
  - `GET /healthz` returns 200 and body contains `ok`.
  - `GET /` returns 200 and serves React SPA HTML.

#### T1. Anonymous session cookie
- **User story**: As a user, my scans are associated with my session without logging in.
- **Acceptance criteria**:
  - First visit sets a cookie (e.g. `sid`) with a new session ID.
  - Subsequent requests reuse the same session ID.
  - Session has `created_at` and `last_seen_at` updated.
- **Unit tests**:
  - First request sets `Set-Cookie: sid=...`.
  - Second request with cookie does not rotate `sid`.
  - Session persistence layer is called with expected ID and timestamps.

#### T2. Database schema + migration runner (Phase0 tables)
- **User story**: As the system, I can persist scans, OCR results, documents, and annotations reliably.
- **Acceptance criteria**:
  - PostgreSQL migrations run at startup (idempotent).
  - Tables exist: `users`, `scans`, `documents`, `annotations`.
- **Unit tests**:
  - Migration runner can run twice without error.
  - Smoke test inserts into `scans` and reads back.

#### T3. Upload endpoint (multipart) + validation
- **User story**: As a user, I can upload a photo of a book page to start OCR.
- **Acceptance criteria**:
  - `POST /api/scans` accepts multipart file (field name `image`).
  - Validates: content type (jpg/png/webp), size limit (10MB), non-empty file.
  - On success creates `scan` record and returns JSON `{scanId, imageUrl}`.
  - On validation failure returns 400 with error message.
- **Unit tests**:
  - Valid JPEG upload returns 201 with scan ID.
  - Oversized file returns 413 and does not write DB records.
  - Invalid mime returns 400 and does not write DB records.

#### T4. Local image storage
- **User story**: As the system, I can persist the uploaded image for OCR and audit/debugging.
- **Acceptance criteria**:
  - Image is stored on disk under `data/uploads/{scanID}.{ext}`.
  - DB stores `image_url` path + SHA256 hash.
  - If storage fails, scan is marked failed.
- **Unit tests**:
  - Storage `SaveImage` is called once for a successful upload.
  - Failure path returns a clean error and no orphaned DB rows.

#### T5. Gemini Flash OCR integration
- **User story**: As a user, I see extracted Japanese text from my uploaded page.
- **Acceptance criteria**:
  - Server calls Gemini Flash OCR with the uploaded image (async goroutine).
  - OCR response parsed and stored in `scans.full_ocr_text`.
  - Frontend polls `GET /api/scans/{id}` until `full_ocr_text` is populated.
  - OCR failures set scan status and show retry UI.
- **Unit tests**:
  - OCR handler uses a mocked `GeminiClient.OCR` and stores returned text.
  - Timeout from Gemini returns a friendly error.
  - Bad JSON from Gemini is handled (no panic, error shown).

#### T6. Scan page (text preview UI)
- **User story**: As a user, I can read the OCR text and prepare to highlight.
- **Acceptance criteria**:
  - `ScanPage` renders image preview + OCR text preview.
  - Mobile-friendly layout (large tap targets, readable font).
  - Text is selectable for highlighting.
- **Unit tests**:
  - ScanPage renders with text content for valid scan.
  - Loading state shown while OCR is processing.
  - Non-existent scan shows error state.

#### T7. Highlight capture (selection payload)
- **User story**: As a user, I can select a word/sentence and request an explanation.
- **Acceptance criteria**:
  - UI captures selected text via touch/mouse selection.
  - Submits to `POST /api/scans/{scanID}/annotate` with `{selectedText}`.
  - Server validates selection (non-empty, reasonable length).
- **Unit tests**:
  - Empty selection returns 400 with hint message.
  - Oversized selection returns 400.

#### T8. Gemini annotation generation (core value)
- **User story**: As a user, I get contextual explanation for highlighted text in work/pro context.
- **Acceptance criteria**:
  - Server sends full OCR text + selected span to Gemini.
  - Response stored in `annotations` with fields: meaning, usage_example, when_to_use, word_breakdown, alternative_meanings.
  - Annotation card rendered in drawer/modal.
- **Unit tests**:
  - `Annotate` mocked returns JSON; handler persists and returns all fields.
  - Gemini error returns error state with retry suggestion.
  - Annotation is tied to correct `scan_id`.

#### T9. Happy-flow polish (loading + errors)
- **User story**: As a user, I understand what's happening and what to do if something fails.
- **Acceptance criteria**:
  - OCR stage shows loading spinner; annotation shows spinner.
  - Errors are displayed inline without losing context.
  - Basic logging and correlation id per scan.
- **Unit tests**:
  - Loading states render correctly.
  - Error states include actionable next step.

#### T10. Safety + performance guardrails (MVP)
- **User story**: As a user, the app feels fast and does not break under normal usage.
- **Acceptance criteria**:
  - Request size limit enforced for uploads (10MB).
  - Gemini calls have timeouts.
  - Rate limiting per session is optional but documented if deferred.
- **Unit tests**:
  - Upload size limit enforced.
  - Gemini client timeout path returns expected status and message.

---

### Phase0 tasks — PDF Path (implement in order)

#### T11. Database migration for documents
- **User story**: As the system, I can store PDF documents and link pages to scans.
- **Acceptance criteria**:
  - New migration `002_add_documents.sql` adds `documents` table.
  - Adds `document_id` and `page_number` columns to `scans` table.
  - Migration is idempotent.
- **Unit tests**:
  - Migration runs without error (can run twice).
  - Insert + read document record works.
  - Scan record can reference a document.

#### T12. PDF upload endpoint
- **User story**: As a user, I can upload a PDF to start reading.
- **Acceptance criteria**:
  - `POST /api/documents` accepts multipart PDF upload (field name `file`).
  - Validates: content type (`application/pdf`), size limit (10MB), non-empty.
  - Extracts page count using Go PDF library.
  - Stores PDF file on disk under `data/uploads/documents/`.
  - Creates `documents` record.
  - Returns JSON `{documentId, pageCount, filename}`.
- **Unit tests**:
  - Valid PDF returns 201 with document ID and page count.
  - Non-PDF file returns 400.
  - Oversized PDF returns 413.
  - Empty PDF returns 400.

#### T13. PDF text extraction endpoint
- **User story**: As a user, I can read the text from any page of my uploaded PDF.
- **Acceptance criteria**:
  - `GET /api/documents/{id}/pages/{n}` returns extracted text for page N.
  - Page number validated (1 ≤ n ≤ pageCount).
  - Text extraction is fast (no Gemini API call, < 1 second).
  - Returns JSON `{pageNumber, text, totalPages}`.
- **Unit tests**:
  - Valid page returns 200 with text content.
  - Out-of-range page (0, negative, > pageCount) returns 404.
  - Non-existent document returns 404.

#### T14. PDF-to-scan bridge endpoint
- **User story**: As a user, when I highlight text from a PDF page, it enters the same annotation flow as scanned images.
- **Acceptance criteria**:
  - `POST /api/documents/{id}/pages/{n}/scan` creates a scan record.
  - Scan has `full_ocr_text` populated immediately (no async OCR needed).
  - Scan linked via `document_id` and `page_number`.
  - If scan already exists for this document+page, returns existing scan.
  - Returns JSON `{scanId}`.
- **Unit tests**:
  - Creates scan with text populated and document_id set.
  - Duplicate request for same page returns same scan ID (idempotent).
  - Non-existent document returns 404.
  - Out-of-range page returns 404.

#### T15. WelcomePage — PDF upload button
- **User story**: As a user, I see an "Upload PDF" option on the home screen.
- **Acceptance criteria**:
  - Third button on WelcomePage: "Upload PDF".
  - File picker accepts `application/pdf` only.
  - Shows loading state during upload.
  - On success, navigates to `/document/{documentId}`.
  - On error, shows toast with error message.
- **Unit tests**:
  - Button renders on WelcomePage.
  - File input has `accept="application/pdf"`.
  - Successful upload navigates to document page.

#### T16. Document page — page list view
- **User story**: As a user, I can see all pages of my uploaded PDF and pick one to read.
- **Acceptance criteria**:
  - `/document/{id}` route shows document title and page list.
  - Simple text list: "Page 1", "Page 2", ..., "Page N".
  - Tapping a page navigates to the reader view for that page.
  - Shows document filename and total page count.
- **Unit tests**:
  - Page list renders correct number of page items.
  - Tapping a page item transitions to reader view.
  - Non-existent document shows error state.

#### T17. Document page — Kindle-like page reader
- **User story**: As a user, I can read my PDF page by page with swipe navigation.
- **Acceptance criteria**:
  - Reader view displays extracted text for current page.
  - Swipe left/right (touch) or tap edges to navigate between pages.
  - Page indicator shows current/total (e.g., "3 / 45").
  - Previous/next buttons visible for non-touch navigation.
  - Text is highlightable for annotation (reuses existing `useTextSelection` hook).
  - Highlight triggers annotation flow (same drawer/modal as ScanPage).
- **Unit tests**:
  - Reader renders page text content.
  - Navigation updates page number and fetches new text.
  - Page indicator shows correct current/total.
  - First page disables "previous" control.
  - Last page disables "next" control.
  - Text selection triggers annotation flow.

#### T18. useDocument hook
- **User story**: As a developer, I have a clean data-fetching interface for document operations.
- **Acceptance criteria**:
  - `useDocument(id)` fetches document metadata (filename, pageCount).
  - `useDocumentPage(id, pageNumber)` fetches page text with caching.
  - Page navigation state management (current page, loading states).
  - Integrates with TanStack Query for caching and refetching.
- **Unit tests**:
  - Hook returns document metadata on success.
  - Hook returns page text on success.
  - Loading and error states are correct.
  - Page text is cached (no refetch on revisit).

---

### Notes for fast MVP execution
- **Image path and PDF path share**: annotation generation (T8), text selection (T7), loading/error polish (T9), safety guardrails (T10).
- **PDF path does NOT use Gemini for text extraction** — only for annotation after highlight.
- Prefer **mock-first** Gemini client: implement the interface with a fake in tests before wiring real HTTP calls.
- Keep selection simple for Phase0: **send `selected_text` only**; offsets can be added later.
- PDF reader can start simple (prev/next buttons) and add swipe gestures as polish.
- **Scanned/image-only PDFs are out of scope** — only text-based PDFs with extractable text are supported in MVP. Do not attempt to OCR PDF page images.

---

## PDF feature epic (spec: [docs/add-pdf/prd.md](add-pdf/prd.md), [docs/add-pdf/rfc.md](add-pdf/rfc.md))

**Stack note:** The live app is **React + Go JSON API under `/v1`** (see [docs/rfc.md](rfc.md)). The Phase0 checklist above (HTMX, `POST /scans`) is legacy/reference — PDF work targets **`/v1`** and files under `backend/` and `web/`.

Derived from the PDF addendum RFC/PRD. Implement after core image scan flow is stable unless prioritized otherwise.

### Product / UX
- [ ] Home (or upload entry): **PDF file picker** (`application/pdf`), mobile-friendly; copy for limits and errors per [docs/add-pdf/prd.md](add-pdf/prd.md) §5–§6, §12–§13.
- [ ] **Processing UI**: uploading → **`status: processing`** with poll until **`ready`** / **`failed`** (use `status`, not only empty `fullText`); support optional `processingProgress` when backend exposes it.
- [ ] **Leave / return / stale tab**: refetch scan on focus if still `processing` (PRD §12).
- [ ] **Text preview + highlight**: reuse image flow when **`fullText`** is available and **`status === ready`**.
- [ ] **No image preview**: hide or skip image when `imageUrl` empty/null for PDF-only rows.

### Backend — API contract ([docs/add-pdf/rfc.md](add-pdf/rfc.md) §6)
- [ ] **`POST /v1/scans`**: accept multipart **`pdf`** OR **`image`** (mutually exclusive); `201` includes **`status`**, **`sourceType`**.
- [ ] **`GET /v1/scans/{id}`**: include **`status`**, **`sourceType`**, **`pageCount`**, optional **`processingProgress`**, optional **`failureReason`**.
- [ ] **`GET /v1/scans`** (list): include **`status`** / **`sourceType`** when known for history UI.
- [ ] **Annotation guard**: reject analyze (and/or annotation) with **4xx** + code **`scan_not_ready`** when `status !== ready` (RFC §6.4).
- [ ] **Keep v1 annotation flow**: **`POST /v1/ai/analyze`** + **`POST /v1/annotations`** — no new `POST /v1/scans/{id}/annotate` unless product explicitly expands scope.

### Backend — schema & migration (RFC §7)
- [ ] Migration: **`source_type`**, **`page_count`**, **`status`**, **`failure_reason`**, **`pdf_storage_path`** (nullable); **`image_url` nullable** for PDF-only rows.
- [ ] Backfill existing rows: `source_type = image`, `status = ready` where `full_ocr_text` present; otherwise align with current implicit behavior.
- [ ] **Image flow regression**: after migration, image uploads still set `image_url` and reach `ready` as today.

### Backend — ingestion (RFC §3–§5)
- [ ] **Validation**: PDF MIME/magic, max size (MB), max pages — config keys per PRD §14 / RFC §9.
- [ ] **Hard reject** over limits (no truncate, PRD §14).
- [ ] **Ingestion worker**: per-page extract → **§5.1** fallback → rasterize + `geminiClient.OCR`; merge with page delimiters; PRD §11 final gate before `ready`.
- [ ] **Whole-document failure**: any page failure → **`failed`** (PRD §10); no partial text in v1.
- [ ] **Password / corrupt / zero-page**: clear **4xx** or **`failed`** + **`failureReason`** per PRD §13.
- [ ] **Merged text cap** (optional safety): max stored characters/bytes; exceed → `failed` (RFC §9).

### Backend — analyze context (RFC §6.4)
- [ ] Implement **context window** (e.g. ≤ 12k Unicode scalars around selection) for **`POST /v1/ai/analyze`** when building `context` from PDF `fullText` (client and/or server — document which owns truncation).

### Backend — operations (RFC §11)
- [ ] **Stuck `processing`**: startup or periodic sweep marks scans stuck longer than **T** minutes as **`failed`** with `processing_timeout`, or document manual ops if deferred.

### Quality
- [ ] **Unit tests**: digital PDF extract fixture; scanned PDF OCR path (mock Gemini); §5.1 threshold cases; context window builder.
- [ ] **Integration test**: `POST /v1/scans` with `pdf` → poll `GET /v1/scans/{id}` until `ready` → `analyze` → `POST /v1/annotations`.
- [ ] **Negative tests**: password PDF; corrupt PDF; over size/pages; **`analyze` while `processing`** → 4xx.
- [ ] **Regression**: `POST /v1/scans` with `image` unchanged end-to-end.
- [ ] **Logging / metrics**: per-page `extract` vs `ocr`, failure stage/reason, duration (RFC §12).

### Docs
- [ ] After implementation, diff API against [docs/add-pdf/rfc.md](add-pdf/rfc.md) and [docs/add-pdf/prd.md](add-pdf/prd.md); update docs if env limits or field names change.
