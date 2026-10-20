# Issue #37: Go builder and toolchain reconciliation

**Issue:** [#37](https://github.com/ventodeco/annota-gemini-hackathon-3/issues/37). Most of the dependency work is already merged in PR #47 (`39349ec`). The earlier record is in `2026-10-20-dependency-runtime-upgrade.md`.

**Outcome:** The backend image builds with the official Go 1.27.3 image instead of a 1.27.2 image that downloads 1.27.3 at build time. This plan also records fresh evidence that the Go modules are current, and fixes the remaining stale Go prerequisite in the docs.

**User story:** As the operator, I want the builder image, CI and docs to use the same Go toolchain as `go.mod`, so that builds are reproducible without a toolchain download.

## Recheck on 2026-10-20 (base `origin/dev` 96ca6a0)

- Go: the [official index](https://go.dev/dl/?mode=json) lists go1.27.3 as the newest stable release. `backend/go.mod` declares `go 1.27.3`. CI reads the version from `go.mod` (`actions/setup-go` `go-version-file`).
- Docker: `golang:1.27.3-alpine3.24` was published on 2026-10-20.
  - Index digest: `sha256:f2c872bcee949b7be9c490efd02b48a8a2fbf947cdf533f1c58525ec9d8170fd`.
  - It covers linux/amd64 and linux/arm64, among other platforms.
  - In both the amd64 and arm64 image configs, `GOLANG_VERSION=1.27.3` and `GOTOOLCHAIN=local`.
  - The `ENV GOTOOLCHAIN=go1.27.3` bootstrap in `backend/Dockerfile` is no longer needed. `docs/github-workflow.md` says to remove it once the image is published.
- Runtime image: `alpine:3.24.2` (`sha256:294b683c…`) is still the latest Alpine release, so it stays unchanged.
- Modules, run with the checksum-verified go1.27.3 toolchain from #47 and `GOTOOLCHAIN=local`:
  - `go list -m -u all`: no direct module has an update.
  - 25 indirect modules have newer versions. None of them is listed in `go.mod`; they appear only in the module graph of dependencies' tests. That graph was already refreshed in #47 with `go get -u -t ./...`, so there is nothing to select.
  - `govulncheck` v1.9.0 (DB 2026-10-20): no vulnerabilities in called code. One module-level advisory remains: GO-2026-5932, `golang.org/x/crypto/openpgp`, which is unmaintained and has no fixed version. The code does not import that package.
- SDKs: `google.golang.org/genai` is still imported (`internal/ai/client.go`). `oauth2` and `jwt` are still used by `internal/auth/google.go`; #39 (Clerk) owns their removal. This PR does not touch `go.mod` or `go.sum`, so it cannot conflict with #39.

## Scope

- `backend/Dockerfile`: pin the builder to `golang:1.27.3-alpine3.24@sha256:f2c872bc…` and remove the `GOTOOLCHAIN` bootstrap and its comment.
- `docs/github-workflow.md`: replace the outdated `GOTOOLCHAIN` override note with the current rule. The builder image must provide at least the `go.mod` version, because the official image sets `GOTOOLCHAIN=local`.
- `RUN.md`: change the Go prerequisite from 1.25.x to 1.27.3, matching `README.md` and `backend/AGENTS.md`.

Out of scope:

- `flake.nix`, `flake.lock` and `docs/NIX.md`. Another team owns Nix alignment as a follow-up, and #37's acceptance does not need it.
- `go.mod` and `go.sum`: they are already current, and #39 is editing them.
- `docs/evidence/**` and the #47 plan, which are historical records.
- `docs/task.md` and `docs/add-pdf/task.md`, which are historical task specs.
- Live AI and auth provider qualification.

## Edge cases

- `GOTOOLCHAIN=local` in the official image: if `go.mod` is later raised above the image's Go version, the image build fails with a clear error instead of downloading a toolchain. Dependabot `golang` image bumps that are newer than `go.mod` remain valid.
- Multi-arch: the pinned index digest covers amd64 and arm64.
- `go mod download` runs before the source copy. It needs only `go.mod` and `go.sum`, which are unchanged.

## Acceptance criteria

- The builder image is `golang:1.27.3-alpine3.24`, pinned by index digest, with no `GOTOOLCHAIN` override.
- CI backend checks and the final-image build and smoke test pass on the PR head.
- `go mod tidy -diff` is empty, and `go mod verify`, `go vet ./...`, `go test ./...` and `go build ./cmd/server` pass with go1.27.3 and `GOTOOLCHAIN=local`.
- The Go versions documented in `README.md`, `RUN.md`, `backend/AGENTS.md` and `docs/github-workflow.md` agree with `go.mod`, CI and the Dockerfile.

## Verification

- Local: the module commands above, plus `CGO_ENABLED=0 GOOS=linux go build`. Docker is not installed locally, so the image build and smoke test are verified by CI only.
- The test-debugging budget is shared across #37 work. #47's evidence records no #37-specific debugging time, and none has been spent so far in this work.
