# Agent Improvement Audit

This backlog captures codebase improvement opportunities found while refreshing the agent guidance. At the time of that guidance refresh, no application code had been changed.

Implementation status: completed under GitHub issue #30. The findings below are retained as the original audit checklist for traceability.

## Frontend

### P1: Remove production TypeScript assertions

- `web/src/pages/ScanPage.tsx`: replace `location.state as ScanPageLocationState | null` with a navigation-state guard.
- `web/src/contexts/AuthContext.tsx`: align `getUserProfile` and `User` types instead of using `userData as unknown as User`.
- `web/src/pages/ProfilePage.tsx`: replace `Language` casts in state initialization, `Select` handling, and `Object.entries` with typed language options or guards.
- `web/src/lib/api.ts`: avoid `undefined as T` for `204` responses by modeling empty responses explicitly.
- `web/src/lib/storage.ts`: validate parsed annotation storage before returning it.
- `web/src/lib/logger.ts`: validate `VITE_LOG_LEVEL` against the allowed log levels.
- `web/src/components/ui/sonner.tsx`: remove theme/style assertions if shadcn updates or local typing can express them safely.

### P1: Improve JSX readability

- `web/src/components/scanpage/AnnotationDrawer.tsx`: replace nested ternaries for drawer height and ARIA values with helper functions.
- `web/src/pages/AnnotationDetailPage.tsx`: replace nested ternaries for `sourcePath` and TTS button copy with named helpers.

### P2: Reduce duplicated annotation formatting

- `web/src/components/scanpage/AnnotationContent.tsx`, `web/src/components/scanpage/AnnotationCard.tsx`, and `web/src/pages/AnnotationDetailPage.tsx` each format pronunciation/nuance data differently.
- Create one annotation formatting helper after normalizing whether UI reads `nuance_data` or `nuanceData`.

### P2: Tighten React effect usage

- `web/src/pages/LoadingPage.tsx`: review effect/ref mirroring around the current scan and clarify ownership of scan lifecycle side effects.
- `web/src/pages/DocumentPage.tsx` and `web/src/pages/ScanPage.tsx`: consider a shared speech cleanup hook if cleanup logic continues to duplicate.

### P2: Improve accessibility guardrails

- Add or configure `eslint-plugin-jsx-a11y` for automated checks.
- Review spinner/loading UI such as `web/src/pages/AnnotationDetailPage.tsx` for `role="status"`, `aria-busy`, or `aria-live` where updates are meaningful.
- Continue checking 44x44px touch targets and visible focus states for mobile-first flows.

## Backend

### P1: Align environment and database documentation with runtime

- `backend/internal/config/config.go` requires PostgreSQL connection details, JWT, Google OAuth, Redis, and Gemini configuration.
- `flake.nix` still exposes `DB_PATH`, which the backend does not read. Decide whether to update the flake env or document it as legacy.
- Keep `backend/.env.example` as the source of truth for backend env names.

### P1: Standardize API error responses

- `backend/internal/middleware/auth.go` uses `http.Error`, while most handlers use `httputil.WriteJSONError`.
- `backend/internal/middleware/rate_limit.go` writes a hand-rolled JSON error body.
- Move JSON API middleware toward one shared `ErrorResponse` shape.

### P1: Review request-started background work

- `backend/internal/handlers/scan.go` starts OCR processing with `context.Background()`.
- Decide whether OCR should be detached from the request with a shutdown-aware app context or bounded with a timeout.

### P2: Make helper contracts explicit

- `backend/internal/httputil/request.go`: `ParsePagination` ignores `strconv.Atoi` errors. Decide between coercing invalid input to defaults or returning `400`.
- `backend/internal/httputil/response.go`: `json.Encoder.Encode` errors are ignored. Decide whether helpers should log encode failures or intentionally ignore them with an explicit assignment.

### P2: Improve migration test fidelity

- `backend/internal/storage/migrate_test.go` uses SQLite while production migrations use PostgreSQL syntax (`BIGSERIAL`, `JSONB`, `NOW()`).
- Add Postgres-backed migration coverage for production DDL when infrastructure is available.

### P3: Logging middleware observability

- `backend/internal/middleware/logging.go` stores only the last write body in the response wrapper.
- If response body logging remains useful, append safely with a size cap and avoid logging secrets or large payloads.

## Documentation

### P1: Keep branch workflow consistent

- AGENTS guidance now standardizes on `dev`; ensure future edits to `docs/github-workflow.md` and any quick starts do not drift back to `main`.

### P1: Resolve database story drift

- README and older agent files previously described SQLite, while current backend runtime uses PostgreSQL and Redis.
- Keep README, `docs/NIX.md`, `flake.nix`, and backend env docs aligned.

### P2: Update stale CLAUDE guidance or point it to AGENTS

- `CLAUDE.md`, `web/CLAUDE.md`, and `backend/CLAUDE.md` contain older guidance and can miss newer AGENTS rules.
- Prefer reducing duplication by pointing Claude-specific docs to the canonical AGENTS files.
