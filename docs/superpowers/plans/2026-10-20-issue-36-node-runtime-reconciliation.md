# Issue #36: Node runtime reconciliation and dependency recheck

**Issue:** [#36](https://github.com/ventodeco/annota-gemini-hackathon-3/issues/36). The package upgrades are already merged in PR #47 (`39349ec`). The earlier record is in `2026-10-20-dependency-runtime-upgrade.md`.

**Outcome:** The Nix dev shell uses the same Node major as CI and the images (Node 24 LTS). This plan also records fresh evidence that every remaining direct-dependency deferral is still blocked upstream.

**User story:** As the maintainer, I want every documented development path to provide the same Node major, so that local Nix work does not run a toolchain that CI and the images no longer use.

## Recheck on 2026-10-20 (base `origin/dev` fd5bcc0)

- Root `bun outdated`: no outdated direct packages.
- `web/` `bun outdated`: only the four deferrals already documented in #47.
  - TypeScript 6.0.3 → 7.0.2: blocked. The latest `typescript-eslint` / `@typescript-eslint/parser` 8.72.0 declares the peer `typescript >=4.8.4 <6.1.0`.
  - ESLint and `@eslint/js` 9.39.5 → 10.x: blocked. The latest `eslint-plugin-jsx-a11y` 6.10.2 declares the peer `eslint ^3 … ^9`. The accessibility rules stay enabled.
  - `@types/node` 24.19.2 → 26.6.5: deferred on purpose so the typings match the Node 24 runtime.
- `bun audit`: 1 high advisory. `braces` 3.0.3 comes in through `shadcn > @shadcn/registry > fast-glob > micromatch` (GHSA-vfj7-8cjw-p6xm). The registry's latest is still 3.0.3, so no patched release exists.
- Node: the [official index](https://nodejs.org/dist/index.json) shows v24.22.0 (Krypton, released 2026-10-14) as the newest LTS release. v26.12.0 is Current and not yet LTS. `.node-version`, `engines`, CI and `web/Dockerfile` already use 24.22.0.
- Bun: the [latest release](https://github.com/oven-sh/bun/releases/latest) is bun-v1.4.3, which matches `packageManager`, CI `BUN_VERSION` and the Dockerfile.
- Nix: `flake.nix` still lists `nodejs_22`. The pinned nixpkgs `0726a0ec` defines `nodejs_24` (`pkgs/top-level/all-packages.nix:2766`), which resolves to 24.14.1. Its `bun` is 1.3.11. nixos-unstable `09063bd1` (2026-10-17) has `nodejs_24` 24.21.0 and `bun` 1.4.2. No nixpkgs revision provides Node 24.22.0 or Bun 1.4.3 yet.

There are no package manifest or lockfile changes, so this work does not overlap with #39's `@clerk/react` edits to `web/package.json` and `bun.lock`.

## Scope

- `flake.nix`: change `nodejs_22` to `nodejs_24`. `docs/github-workflow.md` lists `flake.nix` among the runtime pins to keep aligned. This brief reverses #47's "no Nix file changes" exclusion for this one attribute. Nix is still not installed.
- `docs/NIX.md`: update only the Node lines. Note that the Bun package at the locked revision is below the repository pin.

Out of scope:

- `flake.lock` refresh. A valid narHash needs the Nix CLI, a refresh also moves Go (#37), and even nixos-unstable lacks the exact pins.
- Go attributes and lines (#37).
- `docs/commercial-ops.md` Bun line (already changed in PR #55, #43).
- `docs/evidence/**` (historical record).
- `.github/dependabot.yml` (#50).
- Package upgrades blocked by the peers listed above.

## Edge cases

- The Nix shell at the locked revision provides Node 24.14.1 and Bun 1.3.11. Both are below the `engines` and `packageManager` floors. Whether Bun 1.3.11 can install from the 1.4.3 lockfile has not been verified, and the docs must not claim it can. Exact patches remain enforced by CI and the images.
- The flake is not evaluated locally because Nix is unavailable. Attribute existence is verified from nixpkgs source at the locked revision only.

## Acceptance criteria

- `flake.nix` uses `nodejs_24`. No other flake line changes.
- `docs/NIX.md` Node references say 24, with no Go edits.
- The PR and the issue record the deferral evidence above, the unevaluated-flake limitation and the remaining follow-ups.

## Verification

- Diff review and `git diff --check`. No app code, manifest or lockfile changes, so build, lint, tests and screenshots are not needed.
- Prior #36 test-debug time was not recorded in #47 or the issue. This change uses none of the budget.

## Remaining after merge (issue stays open)

- Adopt TypeScript 7 once typescript-eslint supports it.
- Adopt ESLint 10 once jsx-a11y supports it.
- Fix the `braces` advisory once an upstream patch exists.
- Refresh `flake.lock` (Node and Go together, by someone with Nix) when nixpkgs packages Node ≥24.22.0 and Bun ≥1.4.3.
- Run the desktop and mobile login browser check against real auth after Clerk (#39) lands with keys.
