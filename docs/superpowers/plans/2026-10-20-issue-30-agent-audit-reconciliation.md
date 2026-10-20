# Issue #30: Agent improvement audit backlog reconciliation

Issue: https://github.com/ventodeco/annota-gemini-hackathon-3/issues/30
Base: `origin/dev` at `4b730e7`
Source checklist: `docs/agent-improvement-audit.md`

## Outcome

Close #30 with evidence that every audit item has been resolved or decided against the current `dev` source. Fix only the gaps that remain.

## Findings

Commit `441c340` ("refactor: address agent audit backlog (#30)") already implemented most of the audit. Later PRs closed further items: Postgres migration coverage in CI came with #55/#43. Each item was re-checked against `4b730e7`:

- **Production `as` assertions:** none remain. `eslint` with `consistent-type-assertions: never` finds 0 hits in production code and 31 in test-only files.
- **Nested ternaries:** 4 production hits remain, all added after `441c340`:
  - `web/src/components/layout/BottomActionBar.tsx:30` (speech button icon)
  - `web/src/pages/HistoryPage.tsx:47,49` (`headerTitle`)
  - `web/src/pages/HistoryPage.tsx:56` (`detailPath`)

  These break the AGENTS.md "Readable JSX" rule that #30 covers ("Remove ... JSX readability issues").
- **LoadingPage effect/ref mirroring (audit P2):** `441c340` never touched `web/src/pages/LoadingPage.tsx`.
  - `latestScanRef` is mirrored from `scan` through an effect, and the 30s timeout reads it.
  - That guard is redundant. When OCR is ready, the ready effect has already navigated and set `hasNavigatedRef`, so `navigateToScan()` returns early.
  - The id validation (`!id || !Number.isInteger(scanIdParsed) || scanIdParsed <= 0`) is repeated, although the derived `scanId` already encodes it.
- **Every other frontend, backend and documentation item:** resolved in source. The evidence table goes in the audit doc.

## Scope

1. `web/src/components/layout/BottomActionBar.tsx`: replace the nested icon ternary with a small named helper. The rendered output stays the same.
   Also give the icon-only speech button the same `aria-label` copy as `SelectionSpeechButton`. It currently has no accessible name, which breaks the AGENTS a11y baseline. The label isn't visible.
2. `web/src/pages/HistoryPage.tsx`: replace the `headerTitle` and `detailPath` nested ternaries with named helpers. Strings and paths stay the same.
3. `web/src/pages/LoadingPage.tsx`: remove `latestScanRef` and its mirroring effect, and validate through the derived `scanId`. Navigation behavior stays the same.
4. Tests: `BottomActionBar.test.tsx` covers the speech icon and label states, and `HistoryPage.test.tsx` covers the title and detail path for each filter. In `web/src/pages/__tests__/LoadingPage.test.tsx`, add regression tests for two cases:
   - OCR becomes ready, then the 30s timeout elapses: exactly one navigation, carrying `preloadedScan`.
   - An invalid id redirects to `/welcome`.
5. `docs/agent-improvement-audit.md`: add a reconciliation table at `4b730e7` with item → status → evidence. Record the deliberate decisions and the out-of-scope observations.

## Out of scope

- Production non-null `!` assertions (12 hits). The audit targeted `as` casts, so they're recorded as an observation only.
- Test-only `as` casts.
- Enabling new lint rules in `eslint.config.js`.
- Nix/flake changes, which another team owns.
- Auth, config and API-state files. Clerk PR #59 / #39 has these in flight.

## User story

As a maintainer, I can trust that the audit checklist matches the code. Future agents then start from accurate guidance and don't redo work that's already done.

## Edge cases

- `HistoryPage`:
  - A scan filter takes precedence over a document filter.
  - A document filter without a page number gets its own title.
  - `pageNumber` is appended to the detail path only when it's present.
- `LoadingPage`:
  - Under StrictMode, navigation happens exactly once.
  - If the scan never becomes ready, the timeout navigates without preloaded state.
  - An invalid id, non-positive id or NaN redirects to `/welcome`.
- `BottomActionBar`: the loading state takes precedence over the playing state.

## Acceptance criteria

- 0 production nested ternaries, checked with `bunx eslint src --rule '{"no-nested-ternary":"error"}'`.
- 0 production `as` assertions, checked with the `consistent-type-assertions` rule set to `never`.
- `LoadingPage` no longer has a ref mirrored from state, and its behavior is unchanged per the existing and new tests.
- The audit doc records per-item evidence at `4b730e7`.
- `bun run test`, `bun run lint`, `bun run build`, `go vet ./...` and `go test ./...` pass. Postgres migration tests skip locally without `POSTGRES_TEST_DSN` and run in CI.

## Verification

- Run the new LoadingPage tests on the unmodified source first. They should pass on both the old and the new code, which proves behavior is preserved.
- Run the focused tests for BottomActionBar, HistoryPage, LoadingPage and ScanPage, then the full frontend and backend suites.
- Screenshots aren't required because the rendered DOM is unchanged. Equivalence is covered by the existing text and role assertions.
