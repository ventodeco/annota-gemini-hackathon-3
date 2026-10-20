# Dependabot Reconciliation Plan (Issue #50)

**Goal:** Reconcile [issue #50](https://github.com/ventodeco/annota-gemini-hackathon-3/issues/50) against what PR #49 already merged. Write down how maintainers handle Dependabot PRs, and list the repository-admin steps that still block the config from taking effect.

**Architecture:** Documentation only. Add a "Dependency Updates (Dependabot)" section to `docs/github-workflow.md`, plus this plan. `.github/dependabot.yml`, CI (#43), and build/auth contracts (#39) stay unchanged.

**Status:** Implemented under the owner's plan-then-execute authorization. Independent review covers the PR only.

## Global Constraints

- Branch: `backlog/issue-50`, fast-forwarded to `origin/dev` at `e8c8ed2`. The PR targets `dev`.
- Prefix shell commands with `rtk`. No application code, CI, or Dockerfile changes.
- Repository settings (default branch, Dependabot alerts, security updates) need admin rights. This session has `WRITE` permission, so it must not attempt or claim those changes.

## Evidence (2026-10-20)

| Issue #50 scope item | State on `dev` |
|---|---|
| `.github/dependabot.yml` with weekly checks against `dev` for `bun`, `gomod`, `docker` (`/backend`, `/web`), `github-actions` | Done in #49 (merge `0a94239`) |
| Minor and patch updates grouped per ecosystem | Done: `*-minor-patch` groups in all four entries |
| Majors ignored: TypeScript 7, ESLint and `@eslint/js` 10, `@types/node` and `node` image past 24, nginx 1.31 mainline | Done |
| PostgreSQL and Redis excluded (#43) | Done: the `docker-compose` ecosystem is not configured |
| Config validates | `uvx check-jsonschema --builtin-schema vendor.dependabot .github/dependabot.yml` passes (`ok -- validation done`) |
| Default branch switched to `dev` | **Not done.** `gh repo view` reports `defaultBranchRef: master`. Needs an admin. |
| First run parses every ecosystem, including the `web/` workspace under `bun` | **Blocked** until the default branch is switched |
| Dependabot alerts and security updates enabled | **Unknown.** `GET /repos/.../vulnerability-alerts` and `/automated-security-fixes` return 404 for a `WRITE` token, so an admin has to check. |

CI already runs `pull_request` for `dev`, so Dependabot PRs get the same checks as other PRs. No CI change is needed.

PR #49's review left four follow-ups, recorded only in an issue comment: the `GOTOOLCHAIN` override, manual runtime pin sync, the `-alpine3.24` tag suffix, and the alerts setting. They are reviewer guidance for future Dependabot PRs and belong in the repository docs. The `golang:1.27.3-alpine3.24` tag still returns 404 on Docker Hub, so `ENV GOTOOLCHAIN=go1.27.3` stays for now.

## Scope

1. Add a "Dependency Updates (Dependabot)" section to `docs/github-workflow.md`. It covers:
   - where the config lives and what it does (pointing to the file rather than copying the ignore list);
   - the admin prerequisites: default branch `dev`, plus alerts and security updates enabled;
   - a review checklist for Dependabot PRs: runtime pins Dependabot doesn't update (`packageManager`, `engines`, `.node-version`, `BUN_VERSION` in CI, `flake.nix`), removing `GOTOOLCHAIN`, the Alpine tag suffix, the next nginx mainline ignore entry, and keeping PostgreSQL/Redis out (#43).
2. Comment on issue #50 with this evidence and the remaining admin-only criteria. Keep the issue open.

Out of scope: changing `.github/dependabot.yml` (no gap found); `flake.nix` pin drift (`go_1_25`, `nodejs_22` against Go 1.27.3 / Node 24), which is only recorded in the checklist; CI jobs (#43); auth/build contracts (#39).

## User story

As a maintainer reviewing a Dependabot PR, I can find in the repository docs what Dependabot does not update and what to check before merging. As the repository admin, I can see which settings still have to change before Dependabot runs.

## Edge cases

- Dependabot reads its config from the default branch only. Until `dev` is the default, no version-update PRs open, even though the config is on `dev`.
- `oven/bun` or `node` image bumps arrive without matching `packageManager`, `engines`, `.node-version`, or `BUN_VERSION` changes. CI could then test a different runtime from the image.
- A `golang` image bump has no effect while `GOTOOLCHAIN` pins a newer toolchain.
- When upstream stops publishing `-alpine3.24` tags, Docker updates stop silently.

## Acceptance criteria

- [x] The `dependabot.yml` scope in the issue is verified as present on `dev` and schema-valid.
- [x] The Dependabot review checklist and admin prerequisites are in `docs/github-workflow.md`.
- [ ] (Admin) Default branch switched to `dev`.
- [ ] (Admin, after the first run) Insights → Dependency graph → Dependabot shows every ecosystem parsing, including `web/` under `bun`.
- [ ] (Admin) Dependabot alerts and security updates confirmed enabled.

Issue #50 stays open until an admin completes the unchecked items.

## Verification

- `uvx check-jsonschema --builtin-schema vendor.dependabot .github/dependabot.yml` passes.
- Docs-only diff: `git diff --stat origin/dev` lists only the two Markdown files.
- No screenshots (no UI change).
