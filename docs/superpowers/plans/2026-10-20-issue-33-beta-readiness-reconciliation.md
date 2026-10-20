# Issue #33: beta readiness reconciliation and reader performance slice

Base: `dev` at `5e3d4bc` (after #44's PR #82).

## Finish condition

- The original #33 acceptance is reconciled against landed child work. The issue body's stale
  status lines ("implementation has not started", "backlog only") are corrected. The remaining
  launch gates are listed with their owning issues and marked repo-complete, checks-passed or
  live-pending.
- Two item 5 criteria that no child issue owns are delivered: lazy loading that reduces the
  initial bundle, and long PDFs not keeping every page rendered.
- Existing operational docs no longer contradict landed behaviour.
- #33 stays open while real launch blockers remain. The app is not called release-ready.

## Gap confirmed in source (dev `5e3d4bc`)

- `web/src/App.tsx` imports every page eagerly; the repository has no `React.lazy` or `Suspense`.
  `pdfjs-dist` and all signed-in pages ship in the entry chunk.
- `web/src/components/documentpage/ContinuousPDFViewer.tsx` mounts a page wrapper and canvas for
  every page. It renders a page once that page becomes visible, but never releases it, so a long
  reading session keeps every visited page's canvas bitmap and text layer. Unrendered
  placeholders have no size, so they collapse to the canvas default (about 150px). Several
  pages then render together on load, and the layout shifts as they grow.

No child issue or PR claims either item; #40 covered reader correctness and identity.

## User story

As a mobile learner, I want the app to open quickly and long PDFs to scroll without the browser
running out of memory, so that I can read a whole book on my phone.

## Scope

1. `web/src/App.tsx`: `React.lazy` for the signed-in route pages (Welcome, Camera, Loading,
   History, OCR history, Scan, Document, Documents, Profile, Annotation detail). Each sits inside
   the existing `PageErrorBoundary`/`ProtectedRoute` with a `Suspense` fallback that has busy
   semantics. Intro, Login, SignUp, Legal and NotFound stay eager. Routes, guards, redirects and
   the Clerk root are unchanged.
2. `ContinuousPDFViewer.tsx`:
   - Placeholders take page 1's aspect ratio at the container width until the page itself has
     been measured. A page keeps its measured size after release, so the layout doesn't move.
   - A page is rendered while it is visible (over 30%) or within 2 pages of `currentPage`.
     Outside that window its canvas bitmap is freed and its text layer is cancelled and emptied.
   - A page whose text layer holds the active selection is not released. The selected text and
     page already reported through `onTextSelect` stay with `DocumentPage`; releasing a bitmap
     never clears them.
   - Unchanged: the dominant-page scroll logic, `onTextSelect` page identity, the single
     `selectionchange` listener, and #40's bridge scan cache and stale-result guard (those live
     in `DocumentPage`).
3. Docs (existing files only):
   - `docs/commercial-ops.md`: the public beta launch gate table, linked to owning issues.
     Billing text corrected to the #42 daily quotas. Stale "CI-only change" wording fixed.
   - `docs/rfc.md`: a status note that identity is Clerk and the session/Google OAuth phases are
     historical.

Out of scope: virtualization frameworks, benchmarks, E2E or browser matrices, Contact Us (#80),
other child acceptance criteria, Nix.

## Edge cases

- Fast scroll past the window: visible pages still render because visibility alone qualifies.
- Resume at a later page: the window centres on the initial `currentPage`.
- Selection on a page that then leaves the window: that page is kept rendered and its selection
  survives.
- A page released while a render is in flight: the release runs after the render settles.
- A lazy chunk fails to load (offline or a new deploy): the existing `PageErrorBoundary` shows
  its recoverable error.

## Acceptance and verification

- `bun run build` before and after: the entry chunk is materially smaller and `pdfjs-dist` is out
  of the entry chunk. Record the sizes in the PR.
- One focused Vitest regression in `ContinuousPDFViewer.test.tsx`:
  - a 10-page document renders only the window;
  - moving to a far page releases page 1's bitmap and keeps its measured height;
  - a page holding the selection is not released.
- Existing viewer, DocumentPage and route tests, lint and `tsc` pass. CI is green.
- Browser check of the document reader on desktop and mobile with existing tooling, if it is
  available without new harness work. Otherwise record it as unavailable.
- Sol PR review (fresh task per round), then merge into `dev`. The issue update states the gate
  status.
