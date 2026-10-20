# Agent Improvement Audit

This backlog captures codebase improvement opportunities found while refreshing the agent guidance. At the time of that guidance refresh, no application code had been changed.

Implementation status: completed under GitHub issue #30. The findings below are retained as the original audit checklist for traceability. See [Reconciliation at `b3a4b29`](#reconciliation-at-b3a4b29) for per-item evidence against the current `dev` source.

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

## Reconciliation at `b3a4b29`

Checked on 2026-10-20 against `origin/dev` at `4b730e7`, then re-checked at `b3a4b29` after the Clerk migration (#39/#59) landed. Most items were resolved by `441c340` ("refactor: address agent audit backlog (#30)"), and later PRs closed others. The issue #30 reconciliation PR resolved the gaps that were still open; those rows are marked "this PR". The plan is in `docs/superpowers/plans/2026-10-20-issue-30-agent-audit-reconciliation.md`.

### Frontend

| Item | Status | Evidence |
| --- | --- | --- |
| Production `as` assertions | Resolved | `bunx eslint src --rule '{"@typescript-eslint/consistent-type-assertions":["error",{"assertionStyle":"never"}]}'` reports 0 production hits; the remaining 28 are in test-only files. The guards are `ScanPage.tsx:54` `getPreloadedScan`, `ProfilePage.tsx:44` `isLanguage`, `api.ts:124` `handleEmptyResponse`, `storage.ts:20` `isAnnotation`, `logger.ts:9` `resolveLogLevel` and `sonner.tsx:17` `isSonnerTheme`. The `userData as unknown as User` cast in `AuthContext.tsx` is gone; since #39, the profile comes from Clerk's `useUser` and a React Query profile query, with no cast. |
| Nested ternaries | Resolved (this PR) | `AnnotationDrawer.tsx:24` and `AnnotationDetailPage.tsx:35` use named helpers from `441c340`. This PR replaces the later nested ternaries in `BottomActionBar.tsx` (`SpeechIcon`) and `HistoryPage.tsx` (`getHistoryTitle`, `getAnnotationDetailPath`). `bunx eslint src --rule '{"no-nested-ternary":"error"}'` reports 0 hits. |
| Annotation formatting duplication | Resolved | `web/src/lib/annotationFormatting.ts` is shared by `AnnotationContent.tsx`, `AnnotationCard.tsx` and `AnnotationDetailPage.tsx`. |
| LoadingPage effect/ref mirroring | Resolved (this PR) | `LoadingPage.tsx` no longer mirrors `scan` into a ref. The 30s fallback relies on the `hasNavigatedRef` guard in `navigateToScan`, and id validation uses the derived `scanId`. `LoadingPage.test.tsx` covers ready-then-timeout (navigates once) and invalid ids. |
| Shared speech cleanup | Resolved | `web/src/hooks/useSpeechPlayback.ts` is used by `ScanPage`, `DocumentPage` and `AnnotationDetailPage`. |
| `eslint-plugin-jsx-a11y` | Resolved | `web/eslint.config.js:3` and `web/package.json`. |
| Loading status semantics | Resolved | `AnnotationDetailPage.tsx:64`, `LoadingSpinner.tsx:9`, plus `role="status"` loaders in `ScanPage`, `DocumentPage` and `ExtractedTextReader`. |
| Accessible names for icon-only controls | Improved (this PR) | The `BottomActionBar` speech button now has the same `aria-label` copy as `SelectionSpeechButton`. |

### Backend

| Item | Status | Evidence |
| --- | --- | --- |
| Env and database docs vs runtime | Resolved | `backend/.env.example` is the source of truth for backend env names. `DB_PATH` no longer appears in `flake.nix`. Auth env names changed under #39 (Clerk); see that issue rather than this audit. |
| JSON API error shape | Resolved | `middleware/auth.go:49` (Clerk auth, #39) and `middleware/rate_limit.go:44` use `httputil.WriteJSONError`. The `http.Error` in `handlers/frontend.go:30` is intentional: it's the plain-text static-frontend fallback, not a JSON API route. |
| Request-started OCR work | Resolved | `handlers/scan.go:172` detaches OCR from request cancellation with `context.WithoutCancel` and bounds it with `ocrProcessingTimeout` (5 minutes). |
| `ParsePagination` contract | Resolved (decision: coerce) | `httputil/request.go:10` documents that invalid or non-positive values are coerced to defaults and that `size` is capped at 100. |
| JSON encode errors | Resolved (decision: ignore explicitly) | `httputil/response.go:18` uses explicit `_ =` assignments. |
| Postgres migration fidelity | Resolved | `storage/migrate_test.go` has PostgreSQL-backed tests gated on `POSTGRES_TEST_DSN`, and CI sets that variable (`.github/workflows/ci-cd.yml:39`, #43/#55). Locally the tests skip without a DSN. |
| Logging response body | Resolved | `middleware/logging.go:16` caps the captured body at 4096 bytes, and the body is logged only for responses with status 400 and above. |

### Documentation

| Item | Status | Evidence |
| --- | --- | --- |
| Branch workflow on `dev` | Resolved | `AGENTS.md` (Branching) and the quick start in `docs/github-workflow.md` use `dev`. |
| Database story drift | Resolved | The README describes PostgreSQL and Redis. `backend/AGENTS.md` notes that production migrations target PostgreSQL. Nix docs and the flake are maintained by another team. |
| CLAUDE guidance | Resolved | `CLAUDE.md`, `web/CLAUDE.md` and `backend/CLAUDE.md` point to the canonical AGENTS files. |

### Observations outside this audit

These came up during reconciliation. They're recorded here rather than expanded into #30:

- 12 production TypeScript non-null (`!`) assertions remain, for example in `HistoryPage.tsx`, `DocumentsPage.tsx`, `OCRHistoryPage.tsx`, `useDocument.ts` and `main.tsx`. The audit and the AGENTS rule target `as` casts.
- In `HistoryPage.tsx`, the clickable annotation summary is a `div` with `onClick`, so it can't be reached by keyboard. Fixing it changes the markup and needs its own visual check.
