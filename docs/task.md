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