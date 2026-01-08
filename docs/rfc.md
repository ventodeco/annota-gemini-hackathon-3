## RFC: Mobile-first PWA OCR+Annotation (Go + HTMX + Gemini)

### Status
Draft

### Summary
Build a **mobile-first PWA** that lets users **upload/take a photo of a Japanese book page**, runs **Gemini Flash OCR** to extract text, then lets users **highlight words/sentences** to get **contextual professional/work explanations** (meaning, usage example, when to use, word breakdown, alternative meanings).

### Goals
- **Fast OCR**: image → readable text preview (target OCR success rate ≥ 85% per PRD)
- **Core value loop**: highlight → annotation within ≤ 3 seconds on average (PRD)
- **Mobile UX first**: HTMX-driven UI optimized for touch selection and quick iteration
- **Persistence**: scans and annotations persist across sessions (session-based in Phase0)

### Non-goals (Phase0)
- Google OAuth (Phase1)
- Bookmark/history UI (Phase2)
- Offline OCR/annotation (PWA only caches shell/assets; API calls require network)
- Native apps, realtime live scanning, handwriting, quizzes (PRD out of scope)

### Scope
#### Phase0 (now): PRD 7.1.2 ImageCaptureUpload + 7.1.3 TextInteractionAnnotation
- **Input**: upload from gallery (and optionally capture via browser camera input)
- **OCR**: Gemini Flash vision OCR to extract Japanese text
- **Text preview**: show extracted text, allow selecting/highlighting
- **Annotation**: Gemini generates structured fields for highlighted text
- **Identity**: anonymous **session cookie** (no login)

#### Phase1 (next): PRD 7.1.2 AppEntryAuthentication
- Google OAuth sign-in
- Associate existing session data to a user upon login

#### Phase2 (next): PRD 7.1.4 BookmarkHistory
- Save/bookmark annotations
- Scan history + annotation history pages

### User experience (Phase0 happy flow)
- User opens app (PWA)
- User chooses **Upload** (or capture)
- App shows **OCR processing** state
- App shows **Text Preview**
- User highlights a word/sentence
- App shows **Annotation Result** with required fields
- User can highlight another phrase (loop)

### Architecture
#### Components
- **PWA client**: server-rendered HTML + HTMX partials, minimal JS only where required for mobile selection UX
- **Go web server**: routes for upload, OCR, scan view, annotation generation; renders templates and HTMX fragments
- **Gemini API client**: two calls
  - **OCR**: image → extracted text JSON
  - **Annotation**: extracted text + selection → structured annotation JSON
- **SQLite**: metadata + OCR text + annotations (Phase0); users/bookmarks later
- **File storage**: store uploaded images on disk (e.g. `data/uploads/`), DB stores path + hashes

#### PWA strategy
- `manifest.webmanifest` for installability
- Service worker caches the app shell + static assets for faster repeat loads
- OCR/annotation endpoints are **network-only** (no offline compute)

#### Gemini integration
- **Model**: Gemini Flash for OCR (vision) and annotation (text)
- **Output format**: JSON-first prompts so backend can store and render reliably
- **Prompt versioning**: store `prompt_version` with OCR/annotation for later iteration

### API surface (proposed)
- **GET /**: home/upload entry
- **POST /scans**: multipart upload, create scan, start OCR, redirect to scan page
- **GET /scans/{scanID}**: text preview + highlight UI (HTMX fragments for updates)
- **POST /scans/{scanID}/annotate**: accepts selection payload, returns annotation fragment
- **GET /healthz**: basic health check

### Data model
#### Diagrams
```mermaid
flowchart LR
  User[UserMobileBrowser] --> PWA[PWA_HTMX_UI]
  PWA --> GoServer[GoWebServer]
  GoServer --> SQLite[(SQLite)]
  GoServer --> FileStore[(LocalFileStorage)]
  GoServer --> Gemini[GeminiAPI]
```

```mermaid
sequenceDiagram
  participant U as User
  participant B as BrowserPWA
  participant S as GoServer
  participant G as GeminiAPI
  participant D as SQLite
  participant F as FileStore

  U->>B: SelectImageOrCapture
  B->>S: POST_/scans(multipart_image)
  S->>F: PersistImageFile
  S->>D: InsertScan+ImageMetadata
  S->>G: OCR(image,ocr_prompt)
  G-->>S: OCR_JSON(extracted_text)
  S->>D: UpsertOCRResult
  S-->>B: 302_/scans/{scanID}
  B->>S: GET_/scans/{scanID}
  S-->>B: TextPreviewHTML
```

```mermaid
sequenceDiagram
  participant U as User
  participant B as BrowserPWA
  participant S as GoServer
  participant G as GeminiAPI
  participant D as SQLite

  U->>B: HighlightText
  B->>S: POST_/scans/{scanID}/annotate(selection_payload)
  S->>D: ReadOCRText
  S->>G: Annotate(ocr_text,selection,annotation_prompt)
  G-->>S: Annotation_JSON
  S->>D: InsertAnnotation
  S-->>B: AnnotationFragmentHTML(HTMX)
```

```mermaid
erDiagram
  SESSIONS ||--o{ SCANS : owns
  USERS ||--o{ SCANS : owns
  SCANS ||--|| SCAN_IMAGES : has
  SCANS ||--|| OCR_RESULTS : has
  OCR_RESULTS ||--o{ ANNOTATIONS : produces
  USERS ||--o{ BOOKMARKS : saves
  ANNOTATIONS ||--o{ BOOKMARKS : saved_as

  SESSIONS {
    text id PK
    datetime created_at
    datetime last_seen_at
    text user_id FK
  }
  USERS {
    text id PK
    text provider
    text provider_subject
    text email
    text name
    datetime created_at
  }
  SCANS {
    text id PK
    text session_id FK
    text user_id FK
    text source
    text status
    datetime created_at
  }
  SCAN_IMAGES {
    text id PK
    text scan_id FK
    text storage_path
    text mime_type
    text sha256
    integer width
    integer height
    datetime created_at
  }
  OCR_RESULTS {
    text id PK
    text scan_id FK
    text model
    text language
    text raw_text
    text structured_json
    text prompt_version
    datetime created_at
  }
  ANNOTATIONS {
    text id PK
    text ocr_result_id FK
    text scan_id FK
    text selected_text
    integer selection_start
    integer selection_end
    text meaning
    text usage_example
    text when_to_use
    text word_breakdown
    text alternative_meanings
    text model
    text prompt_version
    datetime created_at
  }
  BOOKMARKS {
    text id PK
    text user_id FK
    text annotation_id FK
    datetime created_at
  }
```

#### SQLite schema (DDL)
```sql
PRAGMA foreign_keys = ON;

-- Phase0 identity: anonymous session (cookie) with optional future user association
CREATE TABLE IF NOT EXISTS sessions (
  id TEXT PRIMARY KEY,
  user_id TEXT NULL,
  created_at TEXT NOT NULL,
  last_seen_at TEXT NOT NULL,
  FOREIGN KEY (user_id) REFERENCES users(id)
);

-- Phase1: Google OAuth users
CREATE TABLE IF NOT EXISTS users (
  id TEXT PRIMARY KEY,
  provider TEXT NOT NULL,
  provider_subject TEXT NOT NULL,
  email TEXT NULL,
  name TEXT NULL,
  avatar_url TEXT NULL,
  created_at TEXT NOT NULL,
  UNIQUE(provider, provider_subject)
);

-- Phase0 scan records
CREATE TABLE IF NOT EXISTS scans (
  id TEXT PRIMARY KEY,
  session_id TEXT NOT NULL,
  user_id TEXT NULL,
  source TEXT NOT NULL,      -- camera|upload
  status TEXT NOT NULL,      -- uploaded|ocr_done|failed
  created_at TEXT NOT NULL,
  FOREIGN KEY (session_id) REFERENCES sessions(id),
  FOREIGN KEY (user_id) REFERENCES users(id)
);

CREATE TABLE IF NOT EXISTS scan_images (
  id TEXT PRIMARY KEY,
  scan_id TEXT NOT NULL UNIQUE,
  storage_path TEXT NOT NULL,
  mime_type TEXT NOT NULL,
  sha256 TEXT NULL,
  width INTEGER NULL,
  height INTEGER NULL,
  created_at TEXT NOT NULL,
  FOREIGN KEY (scan_id) REFERENCES scans(id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS ocr_results (
  id TEXT PRIMARY KEY,
  scan_id TEXT NOT NULL UNIQUE,
  model TEXT NOT NULL,
  language TEXT NULL,
  raw_text TEXT NOT NULL,
  structured_json TEXT NULL,
  prompt_version TEXT NOT NULL,
  created_at TEXT NOT NULL,
  FOREIGN KEY (scan_id) REFERENCES scans(id) ON DELETE CASCADE
);

-- Phase0 annotations (generated from highlight selections)
CREATE TABLE IF NOT EXISTS annotations (
  id TEXT PRIMARY KEY,
  scan_id TEXT NOT NULL,
  ocr_result_id TEXT NOT NULL,
  selected_text TEXT NOT NULL,
  selection_start INTEGER NULL,
  selection_end INTEGER NULL,
  meaning TEXT NOT NULL,
  usage_example TEXT NOT NULL,
  when_to_use TEXT NOT NULL,
  word_breakdown TEXT NOT NULL,
  alternative_meanings TEXT NOT NULL,
  model TEXT NOT NULL,
  prompt_version TEXT NOT NULL,
  created_at TEXT NOT NULL,
  FOREIGN KEY (scan_id) REFERENCES scans(id) ON DELETE CASCADE,
  FOREIGN KEY (ocr_result_id) REFERENCES ocr_results(id) ON DELETE CASCADE
);

-- Phase2 bookmarks
CREATE TABLE IF NOT EXISTS bookmarks (
  id TEXT PRIMARY KEY,
  user_id TEXT NOT NULL,
  annotation_id TEXT NOT NULL,
  created_at TEXT NOT NULL,
  FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE,
  FOREIGN KEY (annotation_id) REFERENCES annotations(id) ON DELETE CASCADE,
  UNIQUE(user_id, annotation_id)
);

CREATE INDEX IF NOT EXISTS idx_scans_session_id_created_at ON scans(session_id, created_at);
CREATE INDEX IF NOT EXISTS idx_scans_user_id_created_at ON scans(user_id, created_at);
CREATE INDEX IF NOT EXISTS idx_annotations_scan_id_created_at ON annotations(scan_id, created_at);
CREATE INDEX IF NOT EXISTS idx_bookmarks_user_id_created_at ON bookmarks(user_id, created_at);
```

### Tasks
#### Phase0: Core happy flow (7.1.2 ImageCaptureUpload + 7.1.3 Annotation)
- [ ] Add base Go web server (router, templates, static assets)
- [ ] Add HTMX-driven pages/fragments
  - [ ] Home/upload page
  - [ ] Scan page with text preview
  - [ ] Annotation fragment returned via HTMX
- [ ] Implement session cookie identity
  - [ ] Create/read session cookie
  - [ ] Update `sessions.last_seen_at`
- [ ] Implement image upload handling
  - [ ] Validate mime/size
  - [ ] Persist image to local storage
  - [ ] Create `scans` + `scan_images` rows
- [ ] Gemini Flash OCR
  - [ ] OCR prompt (JSON output with extracted text)
  - [ ] Store `ocr_results` (raw_text + structured_json + prompt_version)
  - [ ] OCR failure handling + retry UX
- [ ] Text preview UI
  - [ ] Render extracted text with selection guidance for mobile
  - [ ] Capture selection payload (selected_text, optional start/end offsets)
- [ ] Annotation generation
  - [ ] Annotation prompt constrained to PRD output fields
  - [ ] Parse + validate response
  - [ ] Store `annotations`
  - [ ] Render result (Meaning, Usage Example, When to Use, Word Breakdown, Alternative Meanings)
- [ ] Performance + safety
  - [ ] Timeouts and request size limits
  - [ ] Basic rate limiting per session (optional for MVP)
  - [ ] Logging + correlation IDs per scan

#### Phase1: Authentication (7.1.2 AppEntryAuthentication)
- [ ] Google OAuth flow (login, callback, logout)
- [ ] `users` table population + uniqueness rules (provider, subject)
- [ ] Session-to-user association (on login)
- [ ] Protect user-specific pages/endpoints where needed

#### Phase2: Bookmark & history (7.1.4 BookmarkHistory)
- [ ] Save bookmark action on annotation result
- [ ] Bookmarks page (list by date/time) + detail view
- [ ] Scan history page (list scans) + scan detail
- [ ] Annotation history (optional: per scan and global)