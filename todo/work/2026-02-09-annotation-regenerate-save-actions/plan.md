# Add Regenerate + Save Actions in Annotation Drawer

Reference: `.agent/PLANS.md` (not present in this repository; this plan follows the `plan` skill contract as the governing format) | Tasks: `todo/work/2026-02-09-annotation-regenerate-save-actions/todos.md`

## Purpose

Users can currently view generated annotation content and save it, but they cannot request another explanation for the same highlighted text without closing the drawer and repeating selection/explain steps. This change adds two explicit actions in the annotation drawer: `Regenerate` as the primary action and `Save Annotation` as the secondary action (white background). It also adds a version indicator in the drawer header so users can see whether they are viewing the initial explanation or a regenerated one. The result should let users iterate explanation quality/context quickly while still being able to persist a chosen result.

### User Story

As a Japanese-learning user reviewing OCR text in the annotation drawer, I want a `Regenerate` button that fetches a fresh explanation for the same highlighted text and context, a clear `Version X/2` indicator next to the Annotation title, and a separate `Save Annotation` button styled as secondary, so that I can compare versions and know when I reached the current regeneration limit before saving.

### Acceptance Criteria

- [ ] Criterion 1: The drawer footer shows two side-by-side buttons: `Regenerate` (primary visual treatment) and `Save Annotation` (secondary visual treatment with white background and border/contrast that matches current design system).
- [ ] Criterion 2: The drawer header shows version text to the right of `Annotation` using regular 16px typography, formatted as `Version X/2` (`1/2` initially, `2/2` after one regenerate).
- [ ] Criterion 3: Pressing `Regenerate` calls Gemini analysis again using the current selected/highlighted text and context text, then replaces drawer content with the new `nuance_data` response without closing the drawer and increments version from `1/2` to `2/2`.
- [ ] Criterion 4: Regeneration is capped at version `2/2`; once version is `2/2`, the regenerate button is disabled (future re-enable behavior is explicitly out of scope for this task).
- [ ] Criterion 5: While regeneration is in-flight, UI prevents duplicate submissions (disabled button and/or spinner), and current content remains readable until new content arrives.
- [ ] Criterion 6: Pressing `Save Annotation` persists the currently displayed annotation payload to backend via existing create-annotation path and closes/clears state as it does today.
- [ ] Criterion 7: Automated tests cover the two-action rendering, version indicator, regeneration cap, and save behavior contracts, and all touched tests pass.

## Progress

- [x] (2026-02-09 07:42Z) Located annotation drawer and scan-page state orchestration files.
- [x] (2026-02-09 07:42Z) Drafted ExecPlan with user story, acceptance criteria, file targets, and validation commands.
- [x] (2026-02-09 07:56Z) Implemented drawer two-button footer, header version indicator, and regeneration cap (`2/2` disable behavior).
- [x] (2026-02-09 07:56Z) Added/updated tests for drawer header, drawer actions, and ScanPage regenerate-save integration.
- [x] (2026-02-09 07:57Z) Validation passed for `bun run test` and `bun run build`.

## Decision Log

Use `ScanPage` as the orchestration layer for regenerate behavior and keep `AnnotationDrawer` presentational plus event-callback driven. Rationale: `ScanPage` already owns `selectedText`, `contextText`, mutation hooks (`useAnalyzeText`, `useCreateAnnotation`), and the authoritative `currentAnnotation` object, so reusing that ownership avoids duplicating network logic in the drawer.

Retain save semantics as "save whatever is currently rendered." Rationale: once regeneration updates `currentAnnotation.nuance_data`, save should persist that updated data to make user intent explicit and predictable.

Add a dedicated drawer component test file for action controls, and keep page-level integration coverage in `ScanPage` tests with targeted mocks. Rationale: this splits fast UI assertions from flow orchestration assertions.

Apply a fixed version cap of `2` for now (displayed as `Version X/2`) and disable `Regenerate` at `2/2`. Rationale: this matches current product direction and UI copy; if limits change later, we can generalize with a configurable constant without changing UX behavior now.

## Surprises & Discoveries

Expected source file `.agent/PLANS.md` is missing in this repository root (`ls .agent` returned "No such file or directory"). This plan therefore follows the required section/format contract from the `plan` skill text directly.

The helper script `/Users/mac/.codex/skills/plan/scripts/scaffold-workdoc.sh` computes repo root relative to the skill directory and generated files under `/Users/mac/todo/...` for this environment. Workdocs were recreated at the project-local path `todo/work/2026-02-09-annotation-regenerate-save-actions/` to satisfy repository conventions.

## Outcomes & Retrospective

Implemented outcome: users can regenerate once from `Version 1/2` to `Version 2/2`, see version state in the header, and then save the currently displayed annotation. Regenerate is disabled at `2/2` per current product rule.

Retrospective notes:
- Regenerate orchestration fits cleanly in `ScanPage` because existing mutation/state ownership already lives there.
- `bun test` (Bun runner) is not suitable for this React/Vitest setup; `bun run test` is the correct command for reliable JS DOM tests in this repository.

## Context

Current flow overview:
- `web/src/pages/ScanPage.tsx` handles text selection, calls `useAnalyzeText` in `handleExplain`, stores result in `currentAnnotation`, and opens `AnnotationDrawer`.
- `web/src/components/scanpage/AnnotationDrawer.tsx` currently renders one bottom button (`Save Annotation`) and calls `onSave`.
- `web/src/components/scanpage/AnnotationContent.tsx` renders sections from `annotation.nuance_data` and highlighted/context text.

Terms used in this plan:
- Primary button: the strongest visual action style (dark/navy in current UI) used for the preferred next action.
- Secondary button: lower-emphasis action style with white background and visible border/text contrast.
- Regenerate: rerun analysis for the same `highlighted_text` + `context_text` and replace `nuance_data` in-memory.
- In-flight state: period while async mutation is pending; buttons should block duplicate submissions.
- Version indicator: header text shown to the right of `Annotation`, format `Version X/2`, 16px regular weight, used to communicate regeneration progress.

Primary files to edit and why:
- `web/src/pages/ScanPage.tsx`: add `handleRegenerate` and regeneration loading state wiring.
- `web/src/components/scanpage/AnnotationDrawer.tsx`: render two buttons, wire `onRegenerate`, `isRegenerating`, version display props, and keep `onSave` path.
- `web/src/components/scanpage/DrawerHeader.tsx`: add right-side version label (`Version X/2`) with 16px regular typography.
- `web/src/pages/__tests__/ScanPage.test.tsx`: extend/adjust page-level tests for regenerate flow contract.
- `web/src/components/scanpage/__tests__/AnnotationDrawer.test.tsx` (new): verify two-button layout, version label rendering, and disabled/loading states.

## Plan of Work

Implement the two-action footer in `AnnotationDrawer` with a responsive two-column layout that matches the provided mock (primary `Regenerate` on the left, secondary white `Save Annotation` on the right). Keep button labels and iconography aligned with current style tokens.

Add a version counter in the drawer header, positioned to the right of `Annotation`, using regular 16px text and format `Version X/2`.

In `ScanPage`, add a regenerate handler that reuses `analyzeText.mutateAsync` with the same selected/highlighted text and stored context, then updates only the mutable parts of `currentAnnotation` (`nuance_data`, optional refreshed `created_at`) while preserving `highlighted_text`, `scan_id`, and drawer open state. Track current version as `1` on initial explain and increment to `2` after regenerate. Guard against null state and show existing alert/error pattern on failure.

Ensure action-state behavior is explicit: `Regenerate` disabled when no current annotation, while analysis is pending, or when current version is `2/2`; `Save Annotation` disabled while save pending (and optionally during regenerate to avoid race writes). Keep content visible during regenerate instead of clearing.

Add tests that assert action rendering and callback behavior in drawer component, plus integration-level expectations that pressing regenerate causes a second analyze call and updates annotation state prior to save.

## Steps

Run from repository root `/Users/mac/WebApps/projects/hackathon-gemini-3`.

1) Inspect and update drawer action API and UI.

    cd web
    rg -n "onSave|Save Annotation|AnnotationDrawer|DrawerHeader|Version" src/components/scanpage/AnnotationDrawer.tsx src/components/scanpage/DrawerHeader.tsx src/pages/ScanPage.tsx

Expected observation: current drawer exposes only `onSave` action and one bottom button.

2) Implement regenerate orchestration in page state.

    cd web
    rg -n "handleExplain|currentAnnotation|useAnalyzeText" src/pages/ScanPage.tsx

Expected observation: existing `handleExplain` logic can be reused to call analyze and overwrite `currentAnnotation.nuance_data`.

3) Add/adjust tests.

    cd web
    bun test src/components/scanpage/__tests__/AnnotationDrawer.test.tsx src/pages/__tests__/ScanPage.test.tsx

Expected observation: tests verify two-button UI, version label (`Version 1/2` then `2/2`), disabled regenerate at `2/2`, and save behavior; both files pass.

4) Run full frontend confidence checks for touched scope.

    cd web
    bun test
    bun run build

Expected observation: no regressions in test suite and production build succeeds.

## Validation

Acceptance Criteria mapping:
- Criterion 1 validated by component test asserting both button labels, classes/variants, and layout presence in `AnnotationDrawer`.
- Criterion 2 validated by component/UI test asserting header version text is shown at `Version 1/2` initially and positioned to the right of title.
- Criterion 3 validated by page/integration test asserting `useAnalyzeText().mutateAsync` is called again on regenerate, state updates content without drawer close, and version increments to `2/2`.
- Criterion 4 validated by tests asserting regenerate button is disabled when version reaches `2/2`.
- Criterion 5 validated by tests asserting regenerate button disabled/spinner during pending, with prior content still rendered.
- Criterion 6 validated by test asserting save mutation uses current `currentAnnotation` (post-regeneration data) and close/clear logic remains intact.
- Criterion 7 validated by successful `bun test` and `bun run build` output.

Manual QA script:

    cd web
    bun run dev

Open scan detail page, select Japanese text, tap explain, then:
- verify footer shows `Regenerate` (dark primary) + `Save Annotation` (white secondary)
- verify header shows `Version 1/2` right of `Annotation` in 16px regular text
- tap `Regenerate` once and confirm version changes to `Version 2/2`
- confirm `Regenerate` is disabled at `Version 2/2`
- confirm annotation text sections update after response
- tap `Save Annotation` and confirm drawer closes and selection clears

## Artifacts

Evidence will be appended during implementation as indented command outputs and brief notes.
