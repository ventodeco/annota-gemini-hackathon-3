# Issue #35 follow-up: opt-in local output for qualification scoring

Date: 2026-10-20. Base: `dev` at `1a51fa8` (#34's PR #63). Ownership: #34 agreed that #35 opens this as a separate PR.

## Problem

`qualify-routes` deliberately omits all output from its report. As a result, the #35 evaluation set cannot score OCR character error rate or annotation kana without running the full authenticated app.

## Change

Add `-output-dir <new dir>`, valid only with `-live`. Each run writes `ocr.txt` (OCR text) and `annotation.json` (the annotation response). Audio stays on `-audio-out`.

These invariants were set by #34 and are kept:

1. Stdout and stderr stay free of content and of the path. The existing leak tests are unchanged, and the report adds only `output_written`.
2. The directory is created with `os.Mkdir(path, 0o700)`, which fails on any existing path, including a symlink. Files are created with `O_EXCL` and mode 0600. Nothing is overwritten or merged.
3. An invalid or existing path, or the flag used without `-live`, exits with status 2 before any request and creates nothing. The error message is fixed and never contains the path.
4. A write failure after a paid call marks only that task failed, with a fixed message. The report is still printed.
5. `docs/evidence/issue-34/gateway-qualification.md` and `docs/ai-routing.md` state that the directory holds private text and must never be committed. They recommend a location outside the repo, so no `.gitignore` entry is needed.

## Verification

- `go test ./cmd/qualify-routes/`, including the new happy-path test (contents, permissions, and no report or path leak), the usage cases (no live, existing, symlink, missing parent), and a unit test for write failure.
- Full `go vet ./...` and `go test ./...`.
- No live calls.
