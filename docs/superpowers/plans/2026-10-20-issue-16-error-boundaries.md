# Issue #16: React Error Boundaries reconciliation plan

Issue: https://github.com/ventodeco/annota-gemini-hackathon-3/issues/16
Base: `dev` @ `b3a4b29` (includes the Clerk migration, #39 / PR #59).

## Current state (verified on latest dev)

Most of the issue was already delivered by `c3fc736` ("feat: add route error boundary and refresh PRD"):

- `web/src/components/layout/ErrorBoundary.tsx` is a shared class boundary. It shows a full-page `role="alert"` fallback with a "Try again" button, logs through `componentDidCatch`, and resets when `resetKey` changes.
- In `web/src/App.tsx`, every route element is wrapped in `PageErrorBoundary` (keyed by `location.pathname`). That covers reader/scan (`/scans/:id`, `/documents/:id`), annotation (`/annotations/:id`, `/history`), auth (`/login`, `/sign-up`), and all other pages. The boundary sits outside `ProtectedRoute` / `PublicRoute`, so guard errors are contained too.
- `ErrorBoundary.test.tsx` covers the fallback rendering and the `onReset` callback.

## Remaining gaps (real, small)

1. **No in-app way out.** The page fallback only offers "Try again". If a page throws on every render, the user stays on the crashed view. An installed PWA has no browser back button to change the path and trigger the `resetKey` reset.
2. **Annotation components are not contained.** If `AnnotationDrawer` / `AnnotationContent` throws while rendering AI-derived data, the page fallback replaces the whole Scan or Document reader, which is the exact "critical gap" named in the issue.
3. **Focus is lost.** When the fallback replaces content after an interaction, focus falls to `<body>`.
4. **No root boundary.** `ClerkRoot` / `AuthNotConfigured` / `Toaster` render outside any boundary, so a throw there gives a blank screen.
5. **Recovery is untested.** No test shows that "Try again" renders the children again after a transient failure, or that navigating between params of the same route (`/scans/1` → `/scans/2`) clears the fallback.

## User story

As a reader, when part of the app fails to render, I see an understandable message. I can retry, go home, or (for an explanation panel) close it and keep reading. I don't get a blank screen or lose my page.

## Scope and files

- `web/src/components/layout/ErrorBoundary.tsx`
  - Add an optional `fallback` render prop (`(reset) => ReactNode`) for compact, context-specific fallbacks.
  - Default page fallback gains a secondary "Go to home" link. It is a plain `<a href="/">`, not a router `Link`, because the root boundary sits outside `BrowserRouter`. The full navigation also discards broken in-memory state. The link has a touch target of at least 44px.
  - Move focus to the fallback heading (`tabIndex={-1}`) when the fallback mounts.
- `web/src/components/layout/PageErrorBoundary.tsx` (new): move the existing `PageErrorBoundary` out of `App.tsx` unchanged, so the route-level reset can be tested.
- `web/src/App.tsx`: import `PageErrorBoundary`, and wrap the app body in a root `ErrorBoundary`.
- `web/src/components/scanpage/AnnotationDrawer.tsx`: wrap the drawer in an `ErrorBoundary` with a compact non-modal bottom panel fallback.
  - The panel has `role="alert"`, an `h2`, short copy, and a "Close" button that calls `onClose`.
  - `resetKey` is tied to the `isOpen` state, so closing clears the error and the next "Explain" retries cleanly.
  - The fallback renders nothing when the drawer is closed.
  - Both ScanPage and DocumentPage get the containment without page edits.
- Tests:
  - `ErrorBoundary.test.tsx`: retry re-renders the children after a transient failure, a `resetKey` change clears the fallback, the home link is present, focus moves to the heading, and a custom fallback works.
  - New `PageErrorBoundary.test.tsx`: navigating `/scans/1` → `/scans/2` under one route resets the boundary.
  - `AnnotationDrawer.test.tsx`: a throwing `AnnotationContent` shows the compact fallback, and Close calls `onClose`.

## Explicitly out of scope / verified, no change

- **Unhandled promise rejections.** Boundaries cannot catch async or event-handler errors, so I audited instead.
  - Every `mutateAsync` call site (`useAnnotationDrawerFlow`, CameraPage, HistoryPage, DocumentsPage, OCRHistoryPage, AnnotationDetailPage) is inside `try/catch` with a toast.
  - The `.then` chains in ScanImage and DocumentPage have `.catch`.
  - `clearUserScopedCaches` swallows its own failures.
  - `trackEvent` calls use `.catch`.
  - A global `unhandledrejection` handler would be new, undefined behavior, so I'm not adding one.
- **No `QueryErrorResetBoundary`.** Nothing uses `throwOnError` / `useSuspenseQuery`, so query errors never reach a boundary.
- **The class component is required.** React 19 still has no hook API for error boundaries. This is the one sanctioned exception to "no class components".
- **Files I won't touch.** `BottomActionBar`, `LoadingPage` and `HistoryPage` belong to PR #62 (#30), and there are no backend, auth or Clerk behavior changes.

## Edge cases

- An error that repeats deterministically: "Try again" re-throws into the same fallback, and "Go to home" is the escape.
- Param navigation within one route (same element instance): the `resetKey` on the pathname resets it.
- The drawer crashes while open: the reader stays mounted, and the selection and bottom action bar remain usable after Close.
- A crash at the root or outside the router: the root fallback works because the anchor needs no router context.

## Acceptance criteria

- A page render error shows the contained fallback with "Try again" and "Go to home". Retry recovers when the cause was transient, and focus lands on the fallback heading.
- Every route in `App.tsx` stays wrapped. Navigating between params resets the fallback.
- An annotation drawer render error leaves the reader page intact, with a closable inline alert.
- `bun run lint`, `bun run test`, and `bun run build` pass. There are no new `as` casts or nested ternaries.

## Verification

- Focused Vitest for the tests above, then the full `bun run test`, `bun run lint`, and `bun run build` in `web/`.
- Browser screenshots (desktop and mobile widths) of the page fallback and the drawer fallback.
  - Live Clerk keys are absent, so the real app shows `AuthNotConfigured` and no throwing route is reachable.
  - Screenshots come from a temporary, uncommitted Vite harness entry that renders the real components with a throwing child. The PR states this.
- Signed-in real-provider flows are not claimed as tested.

## Evidence (implementation on `backlog/issue-16`)

- The root `ErrorBoundary` is the outermost element in `App`, wrapping `QueryClientProvider` / `BrowserRouter`.
- Focused Vitest passed: `ErrorBoundary.test.tsx` (6), `PageErrorBoundary.test.tsx` (1), and `AnnotationDrawer.test.tsx` (4).
- Full `web/` run: `bun run lint` is clean, `bun run test` passes 188 with 2 skipped (pre-existing skips), and `bun run build` succeeds.
- Browser check: Playwright Chromium against a temporary, uncommitted Vite harness entry, deleted after capture.
  - The harness rendered the real `ErrorBoundary` with a throwing child, and the real `AnnotationDrawer` / `AnnotationContent` with a malformed AI annotation (`nuance_data: null`).
  - On load, focus landed on the fallback heading at both viewports. Tab reached "Try again" / "Close".
  - `docs/screenshots/issue-16/page-desktop.png`, `page-mobile.png` (1280×800, 390×844)
  - `docs/screenshots/issue-16/drawer-desktop.png`, `drawer-mobile.png`: the reader text stays visible with the closable "Explanation unavailable" panel.
- Not tested: signed-in real Clerk flows (no live keys), and nothing changed there.
