## Task List (Phase0 MVP): Upload → OCR → Text Preview → Highlight → Annotation

This is the execution checklist derived from `docs/rfc.md` (Phase0 only). Each task includes **user story**, **acceptance criteria**, and **unit tests** so we can implement quickly and safely.

### Tech preparation (do once)
#### Versions
- **Go**: **Go 1.25.x** (latest stable major is 1.25; choose latest patch from `go.dev/dl`)
- **HTMX**: **htmx 2.0.8** (official docs CDN snippet)
- **Tailwind CSS**: **Tailwind v4.1** (recommended for build/CLI); for fastest MVP styling, use Tailwind Play CDN
- **SQLite**: bundled with macOS; use a Go driver (recommend `modernc.org/sqlite` for pure-Go or `github.com/mattn/go-sqlite3` if CGO is OK)

#### Local setup checklist
- [ ] Install Go (1.25.x) and verify:
  - [ ] `go version`
  - [ ] `go env GOPATH GOMODCACHE`
- [ ] Initialize module:
  - [ ] `go mod init <module-path>`
- [ ] Decide SQLite driver:
  - [ ] If **pure Go**: use `modernc.org/sqlite`
  - [ ] If **CGO OK**: use `github.com/mattn/go-sqlite3`
- [ ] Add required env vars (local `.env` or shell export):
  - [ ] `GEMINI_API_KEY` (or the key name you’ll standardize on)
  - [ ] `APP_BASE_URL` (for redirects/absolute URLs if needed)
- [ ] Add a basic dev workflow:
  - [ ] `go test ./...`
  - [ ] `go test -race ./...` (optional but recommended)
  - [ ] `go vet ./...`

#### Frontend bootstrap
- [ ] Add HTMX to base layout (use one source consistently):
  - jsDelivr (minified):

```html
<script src="https://cdn.jsdelivr.net/npm/htmx.org@2.0.8/dist/htmx.min.js" integrity="sha384-/TgkGk7p307TH7EXJDuUlgG3Ce1UVolAOFopFekQkkXihi5u/6OCvVKyz1W+idaz" crossorigin="anonymous"></script>
```

- [ ] Add Tailwind CSS (fast MVP styling):
  - Play CDN (fastest, no build step):

```html
<script src="https://cdn.tailwindcss.com/"></script>
```

- [ ] Tailwind CSS build option (use when you want production CSS output):
  - Install:
    - `npm install tailwindcss@latest @tailwindcss/cli@latest`
  - Build:
    - `npx @tailwindcss/cli -i input.css -o output.css`

- [ ] PWA shell (minimal):
  - [ ] `manifest.webmanifest` (name, icons, start_url, display=standalone, theme_color)
  - [ ] `service-worker.js` (cache HTML shell + static assets)

#### Testing scaffolding
- [ ] Adopt interfaces to enable unit testing:
  - [ ] `type GeminiClient interface { OCR(ctx,...); Annotate(ctx,...) }`
  - [ ] `type Storage interface { SaveImage(...); OpenImage(...)}`
  - [ ] `type Clock interface { Now() time.Time }` (optional)
- [ ] Use `net/http/httptest` for handler tests.

---

### Phase0 tasks (implement in order)

#### T0. Project skeleton (Go server + templates + static)
- **User story**: As a user, I can open the app in mobile browser and see an upload screen.
- **Acceptance criteria**:
  - App serves `GET /` with an HTML page containing upload UI.
  - Base layout includes HTMX + Tailwind (CDN is OK for MVP).
  - Page uses Tailwind utility classes for a clean mobile layout (spacing, typography, buttons).
  - Static files served (`/static/...`) for any custom CSS/images/icons.
  - Basic health endpoint exists: `GET /healthz` returns 200.
- **Unit tests**:
  - `GET /healthz` returns 200 and body contains `ok`.
  - `GET /` returns 200 and contains `hx-` attributes (or a known marker).
  - `GET /` includes a Tailwind marker (e.g. `cdn.tailwindcss.com` or known Tailwind class usage).

#### T1. Anonymous session cookie
- **User story**: As a user, my scans are associated with my session without logging in.
- **Acceptance criteria**:
  - First visit sets a cookie (e.g. `sid`) with a new session ID.
  - Subsequent requests reuse the same session ID.
  - Session has `created_at` and `last_seen_at` updated.
- **Unit tests**:
  - First `GET /` sets `Set-Cookie: sid=...`.
  - Second request with cookie does not rotate `sid`.
  - Session persistence layer is called with expected ID and timestamps.

#### T2. SQLite schema + migration runner (Phase0 tables only)
- **User story**: As the system, I can persist scans, OCR results, and annotations reliably.
- **Acceptance criteria**:
  - DB file is created (e.g. `data/app.db`) and migrations run at startup (idempotent).
  - Tables exist: `sessions`, `scans`, `scan_images`, `ocr_results`, `annotations`.
- **Unit tests**:
  - Migration runner can run twice without error.
  - Smoke test inserts into `sessions` and reads back.

#### T3. Upload endpoint (multipart) + validation
- **User story**: As a user, I can upload a photo of a book page to start OCR.
- **Acceptance criteria**:
  - `POST /scans` accepts multipart file (field name standardized, e.g. `image`).
  - Validates: content type (jpg/png/webp), size limit, non-empty file.
  - On success creates `scan` + `scan_image` record and redirects to `/scans/{scanID}`.
  - On validation failure returns a user-friendly error.
- **Unit tests**:
  - Valid JPEG upload returns 302 to `/scans/{id}`.
  - Oversized file returns 413 (or a consistent 400) and does not write DB records.
  - Invalid mime returns 400 and does not write DB records.

#### T4. Local image storage
- **User story**: As the system, I can persist the uploaded image for OCR and audit/debugging.
- **Acceptance criteria**:
  - Image is stored on disk under a deterministic structure (e.g. `data/uploads/{scanID}.jpg`).
  - DB stores storage path + mime type (+ optional sha256).
  - If storage fails, the scan is marked failed and user sees retry guidance.
- **Unit tests**:
  - Storage `SaveImage` is called once for a successful upload.
  - Failure path returns a clean error and no orphaned DB rows (or marks status as failed consistently).

#### T5. Gemini Flash OCR integration
- **User story**: As a user, I see extracted Japanese text from my uploaded page.
- **Acceptance criteria**:
  - Server calls Gemini Flash OCR with the uploaded image.
  - OCR response is parsed into structured data and stored in `ocr_results`.
  - `raw_text` is rendered on scan page.
  - OCR failures show retry UI (re-upload is acceptable for MVP).
- **Unit tests**:
  - OCR handler uses a mocked `GeminiClient.OCR` and stores returned text.
  - Timeout from Gemini returns a friendly error and scan status becomes `failed` (or equivalent).
  - Bad JSON from Gemini is handled (no panic, error shown).

#### T6. Scan page (text preview UI)
- **User story**: As a user, I can read the OCR text and prepare to highlight.
- **Acceptance criteria**:
  - `GET /scans/{scanID}` renders:
    - image preview (optional)
    - OCR text preview
    - instructions for highlighting
  - Mobile-friendly layout (large tap targets, readable font).
- **Unit tests**:
  - `GET /scans/{scanID}` returns 200 for existing scan and includes OCR text.
  - Non-existent scan returns 404.

#### T7. Highlight capture (selection payload)
- **User story**: As a user, I can select a word/sentence and request an explanation.
- **Acceptance criteria**:
  - UI submits selected text to `POST /scans/{scanID}/annotate` via HTMX.
  - Payload includes:
    - `selected_text`
    - optional `selection_start`/`selection_end` (if implemented)
  - Server validates selection (non-empty, reasonable length).
- **Unit tests**:
  - Empty selection returns 400 with hint message.
  - Oversized selection returns 400 (or trims with warning, but be consistent).

#### T8. Gemini annotation generation (core value)
- **User story**: As a user, I get contextual explanation for highlighted text in work/pro context.
- **Acceptance criteria**:
  - Server sends OCR text + selected span to Gemini.
  - Response is stored in `annotations` with fields:
    - meaning
    - usage_example
    - when_to_use
    - word_breakdown
    - alternative_meanings
  - Response renders within HTMX target region (no full page reload required).
- **Unit tests**:
  - `Annotate` mocked returns JSON; handler persists and returns HTML fragment with all fields.
  - Gemini error returns HTMX fragment with retry suggestion.
  - Ensure annotation is tied to the correct `scan_id` + `ocr_result_id`.

#### T9. Happy-flow polish (loading + errors)
- **User story**: As a user, I understand what’s happening and what to do if something fails.
- **Acceptance criteria**:
  - OCR stage shows loading state; annotation shows spinner state.
  - Errors are displayed inline (HTMX swap) without losing context.
  - Basic logging and correlation id per scan.
- **Unit tests**:
  - Loading fragments render and swap targets correctly (handler-level HTML assertions).
  - Error fragments include actionable next step.

#### T10. Safety + performance guardrails (MVP)
- **User story**: As a user, the app feels fast and does not break under normal usage.
- **Acceptance criteria**:
  - Request size limit enforced for uploads.
  - Gemini calls have timeouts.
  - Rate limiting per session is optional but documented if deferred.
- **Unit tests**:
  - Upload size limit enforced.
  - Gemini client timeout path returns expected status and message.

---

### Notes for fast MVP execution
- Prefer **mock-first** Gemini client: implement the interface with a fake in tests before wiring real HTTP calls.
- Keep selection simple for Phase0: **send `selected_text` only**; offsets can be added later.

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