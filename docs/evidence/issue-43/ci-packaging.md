# Issue 43 CI and Container Packaging Evidence

PR: https://github.com/ventodeco/annota-gemini-hackathon-3/pull/46

## Final Passing Verification

Both CI jobs passed on `149690bfac7f6bcb4adda117d8146d00962248c2`:
https://github.com/ventodeco/annota-gemini-hackathon-3/actions/runs/39362268833

- The real PostgreSQL migration test passed without skipping, followed by
  backend tests, vet, build, and final-image startup with the complete migration
  inventory. Backend image ID:
  `sha256:23d7bed40a2565f9ab9f52d43c592e5a0448793e4544995e688b86350046055d`.
- Frontend tests, lint, build, and final-image HTTP checks passed for HTML,
  bundled JS/CSS, manifest, PDF worker, and the direct client route. Frontend
  image ID:
  `sha256:75e6c937d46892f2b8c35fb6b4ecb936b5abb11497b2f71841adc6eba36e9fb4`.
- The probe reported one existing external stylesheet separately and made no
  third-party or paid AI requests. Services and probes used disposable resources
  on the internal Docker network.

The historical failures below explain how the checks and packaging were
corrected. Final-image acceptance for PR 1 is verified; the administrator and
remaining issue #43 checkpoints below are still outstanding.

## Baseline Container Failures

Run: https://github.com/ventodeco/annota-gemini-hackathon-3/actions/runs/39360925534

Source commit: `aa36f2e5490a0891637d68399bd0c4a02803344d`. This revision adds
verification-only CI and final-image checks while retaining the old Dockerfiles.

The backend job passed the explicit PostgreSQL migration test, backend tests,
vet, build, and Docker build. Its final-image smoke failed the directory/SQL
check for `/app/migrations`. The image ID was
`sha256:fdaee966b2178b2e81b80ad7ae1fe055f2dd5aafe2c36798b9ce8c9e95d0924c`.
The final Docker stage copied the binary but omitted runtime migrations.

The frontend job passed frozen root-workspace installation, tests, lint, and
the source build. The old Dockerfile then installed Bun 1.4.3 globally and
resolved 683 packages without the repository-root lockfile. Resolved versions
included React 19.3.1 and pdfjs-dist 5.7.284, differing from the committed graph.
The container build failed at `bun run build`:

```text
vite.config.ts(55,3): error TS2769: No overload matches this call.
Object literal may only specify known properties, and 'test' does not exist
in type 'UserConfigExport'.
```

The missing lockfile did not itself stop that Bun install. This is dependency
drift inside the container, not a reason to change the working Vite config.

## First Packaging Fix Run

Run: https://github.com/ventodeco/annota-gemini-hackathon-3/actions/runs/39361522532

Source commit: `675b249`. Both final images built successfully, so the original
packaging failures were resolved. Runtime acceptance remained blocked by a
defect in the new smoke harness: Docker did not publish host ports for
containers attached only to its internal network. The errors were
`No public port '80/tcp' published` and `No public port '8080/tcp' published`.

The correction runs the same Python HTTP assertions in disposable probe
containers on that network, with no published ports. It preserves external
network isolation and registers probes for cleanup before starting them.
PostgreSQL readiness explicitly probes TCP to avoid accepting the temporary
Unix-socket-only bootstrap server. Runtime acceptance was still pending at this
intermediate revision; the final passing run is recorded above.

## HTTP Probe Scope Correction

Run: https://github.com/ventodeco/annota-gemini-hackathon-3/actions/runs/39361977258

On `8db5217`, the frontend image built and its probe reached nginx. Requests for
the entry document and bundled JS succeeded. The checker then incorrectly
classified the existing Google Fonts stylesheet as a local generated asset:

```text
Expected a local generated asset path: https://fonts.googleapis.com/css2?family=Roboto:wght@400;500&display=swap
```

Reproduced this failure locally by running the same embedded HTTP checker
against the actual `web/dist` output served on loopback. The correction
classifies third-party stylesheet links separately and reports their count;
it still requires generated local CSS and checks its MIME type and body.
No third-party requests are made. The same local checker then passed HTML,
JS, CSS, manifest, PDF worker, and direct-route checks. This local HTTP fixture
isolates checker behavior; the final CI run above establishes nginx acceptance.

## Local Verification

These commands passed before submitting the packaging fixes:

- `bun install --frozen-lockfile` at the root, Bun 1.4.0.
- `bun run test` in `web/`: 155 passed, 2 existing skips.
- `bun run lint` and `bun run build` in `web/`.
- `go test ./...`, `go vet ./...`, and `go build -o /tmp/annota-issue-43-server ./cmd/server`
  in `backend/`, using a temporary checksum-verified Go 1.25.5 toolchain.
- actionlint 1.7.12 for `.github/workflows/ci-cd.yml`.
- shellcheck 0.11.0 and `bash -n` for `scripts/ops/smoke-images.sh`.
- Python AST parsing for all three embedded Python checks.

The local Go run skips the PostgreSQL integration test because it has no
`POSTGRES_TEST_DSN`. The baseline CI run above executes it successfully against
its PostgreSQL service. Local Docker is unavailable, so final-image runtime
verification uses GitHub's disposable runner. The final packaging-fix results
are recorded above from the completed run.

Existing frontend warnings about large chunks, stale Browserslist data, and
React test `act` usage were not changed by this infrastructure task.

## Remaining Operator Checkpoints

No production credentials, registry publication, host changes, or deployment
are used by the new workflow. This PR does not update the frozen `master`
workflow, protect `ENV_PRODUCTION`, or change the default branch.

The authenticated GitHub collaborator has write access, while repository
settings require admin access. The environment guard and approved eventual
default-branch switch remain explicit operator checkpoints.

Test-debugging budget started at `2026-10-20 02:48:15 UTC`; its shared deadline
is `03:18:15 UTC`. The first failure was a shellcheck unused-counter warning,
resolved by using an explicit counted retry loop. This budget includes CI
diagnosis and waits; container failures remain blockers until verified fixes.
