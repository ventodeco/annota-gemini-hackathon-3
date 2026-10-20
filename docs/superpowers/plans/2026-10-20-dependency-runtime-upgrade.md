# Annota Dependency and Runtime Upgrade Implementation Plan

> **For agentic workers:** Use superpowers:executing-plans to implement this plan task-by-task after owner approval. Steps use checkbox (`- [ ]`) syntax for tracking. Implementation authorized; apply the owner scope adjustment below.

**Goal:** Upgrade Annota to current supported stable dependencies and reproducible runtimes while preserving the reader, API and stored user data.

**Architecture:** Retain the Bun workspace, React/Vite SPA and Go JSON API. Separate compatible library upgrades, breaking frontend migrations, Go upgrades and stateful service migration so regressions have clear verification boundaries. Record incompatible newest versions explicitly.

**Tech Stack:** Bun, Node build tooling, React, Vite, TypeScript, Vitest, ESLint, PDF.js, Go, PostgreSQL, Redis, Nix and Docker.

**Checked:** 2026-10-20. Registry/release observations are candidates, not installed upgrades or proof of application compatibility. Recheck before implementation.

**Tracking:** Existing [#36](https://github.com/ventodeco/annota-gemini-hackathon-3/issues/36) covers frontend/Bun, [#37](https://github.com/ventodeco/annota-gemini-hackathon-3/issues/37) covers Go/toolchains, and [#43](https://github.com/ventodeco/annota-gemini-hackathon-3/issues/43) covers release/data recovery. No duplicate issue is needed.

## Global Constraints

- Implementation approved on 2026-10-20. Exclude all Nix changes and Nix CLI installation. Do not update/install Compose CLI. Stateful service migrations and production deployment remain separate under #43.
- Implement from `dev`, not frozen `master`; preserve unrelated local AGENTS.md, research and issue-43 plan changes.
- Use Bun for packages; root `bun.lock` is the workspace lockfile. No npm/yarn/pnpm.
- Follow root/web/backend AGENTS.md and Vercel React guidance. No added TypeScript casts, nested ternaries, unrelated refactors or UI redesign. Preserve accessibility checks.
- Preserve API/auth/data contracts. AI model/provider/Clerk migrations remain separate issues #34/#35/#39.
- Require supported runtime/peer combinations; no forced peer installs or hidden unsupported-version warnings.
- Limit debugging test/tooling failures to 30 minutes total per implementation task. Log exact command, first useful failure, suspected cause, attempts and next action in the issue. Missing tools/services block that check immediately.
- Browser-visible changes require desktop/mobile real-browser verification and saved screenshot paths. A screenshot mismatch against agreed behavior requires owner direction before proceeding.
- No live AI qualification is implied by mocks. No paid requests are needed for this planning task.

## Evidence and limits

All 45 direct npm packages were queried from official registry metadata. Current versions below come from root `bun.lock`, rather than manifest lower bounds. All 8 direct Go module candidates were queried through the Go module proxy, including minimum Go requirements from their module files.

Local tools: Node 24.18.1 and Bun 1.4.0. Go, Nix and Docker are not on PATH. No package installation, application test, container build, or service migration was performed for this plan. Full transitive Go inventory, Nix package evaluation, Docker tag/digest availability and CI runner compatibility remain execution checks.

### Runtime and service inventory

| Area | Baseline | Newest observed | Proposed target / decision | Primary source |
| --- | --- | --- | --- | --- |
| Bun | local 1.4.0; CI latest; Docker unpinned; Nix lock-derived | 1.4.3 | Pin 1.4.3 consistently | [release](https://github.com/oven-sh/bun/releases/tag/bun-v1.4.3) |
| Node.js | Nix/Docker 22; local 24.18.1 | Current 26.12.0; LTS 24.22.0 | 24.22.0 LTS; Current 26 is not yet LTS on check date | [index](https://nodejs.org/dist/index.json), [support policy](https://nodejs.org/en/about/previous-releases) |
| Go | go.mod 1.25.5; Nix/Docker/CI 1.25 | 1.27.3 | Align on 1.27.3 | [index](https://go.dev/dl/?mode=json) |
| PostgreSQL | Compose 13; CI 14; Nix 16 | Stable 18.6; 19 RC1 | 18.6 after separate data migration rehearsal under #43 | [version policy](https://www.postgresql.org/support/versioning/) |
| Redis server | Compose 7; Nix version unevaluated | 8.10.2 | Candidate 8.10.2; qualify persistence/client behavior first | [release](https://github.com/redis/redis/releases/tag/8.10.2) |
| Alpine | backend 3.19; other tags float | 3.24.2 | Candidate 3.24.2; verify image variants/digests | [index](https://alpinelinux.org/releases.json) |
| nginx | frontend nginx:alpine floats | Stable 1.30.6; mainline 1.31.7 | Stable 1.30.6 Alpine image candidate; verify tag/digest | [downloads](https://nginx.org/en/download.html) |
| Nix CLI | unavailable locally; no project CLI pin | official download lists 2.35.3 | Development prerequisite, not production runtime | [download](https://nixos.org/download/) |
| Docker Compose CLI | docs/CI docker-compose; Docker unavailable locally | 5.6.0 | Inventory only; qualify host command, do not remotely upgrade host | [release](https://github.com/docker/compose/releases/tag/v5.6.0) |

`flake.lock` pins nixpkgs revision `0726a0ecb6d4e08f6adced58726b95db924cef57`. Refresh/evaluate it during implementation; do not hand-edit its hash or assume an attribute supplies the exact target patch.

### Compatibility decisions

1. **TypeScript:** newest 7.0.2; target 6.0.3 because typescript-eslint 8.72.0 declares `>=4.8.4 <6.1.0`. Track TS7 in #36 until supported; do not add another compiler or replace lint tooling just to claim newest. [Parser support](https://typescript-eslint.io/users/dependency-versions/), [registry peers](https://registry.npmjs.org/typescript-eslint/latest).
2. **ESLint:** newest 10.13.0; target ESLint/@eslint/js 9.39.5 because latest jsx-a11y 6.10.2 declares support only through ESLint 9. Keep accessibility rules active and track the blocker in #36. [Plugin peers](https://registry.npmjs.org/eslint-plugin-jsx-a11y/latest).
3. **Node typings:** newest @types/node 26.6.5; target 24.19.2 for Node 24 LTS. Newer typings may permit APIs absent from the actual runtime. [Node 24 type metadata](https://registry.npmjs.org/@types%2fnode/24.19.2).
4. **Go floor:** go-redis, oauth2 and SQLite candidates require Go 1.26.0. Upgrade Go before these modules. Go 1.27.3 satisfies the observed floors, but still needs application verification.
5. **PDF assets:** `web/src/lib/pdf/initPdfWorker.ts` hardcodes CMaps at 5.5.207. Upgrade PDF.js, bundled worker, text-layer APIs and CMap assets together, using the installed library version for the CMap URL.
6. **Stateful services:** PostgreSQL 13 is unsupported; 14 reaches EOL on 2026-11-12. Major upgrades require dump/restore or pg_upgrade. PostgreSQL 18 also changes official-image PGDATA/mount layout. Do not reuse old-major raw data directories. [Policy](https://www.postgresql.org/support/versioning/), [image documentation](https://hub.docker.com/_/postgres).
7. **SDK ownership:** genai and oauth2 remain imported by `backend/internal/ai/client.go` and `backend/internal/auth/google.go`; do not remove them before the separate provider/auth migrations eliminate their use.

### Complete direct npm dependency inventory

Targets are proposed compatible versions, not verified installations. Unchanged rows are already at registry latest. Root shadcn is a CLI upgrade, not permission to regenerate all committed UI components.

| Package | Declared | Locked current | Registry latest | Proposed target | Source |
| --- | --- | --- | --- | --- | --- |
| `shadcn` | `^3.6.3` | 3.6.3 | 4.22.1 | 4.22.1 | [registry](https://registry.npmjs.org/shadcn/latest) |
| `@radix-ui/react-alert-dialog` | `^1.1.15` | 1.1.15 | 1.2.1 | 1.2.1 | [registry](https://registry.npmjs.org/@radix-ui%2freact-alert-dialog/latest) |
| `@radix-ui/react-dialog` | `^1.1.15` | 1.1.15 | 1.3.1 | 1.3.1 | [registry](https://registry.npmjs.org/@radix-ui%2freact-dialog/latest) |
| `@radix-ui/react-scroll-area` | `^1.2.10` | 1.2.10 | 1.4.0 | 1.4.0 | [registry](https://registry.npmjs.org/@radix-ui%2freact-scroll-area/latest) |
| `@radix-ui/react-separator` | `^1.1.8` | 1.1.8 | 1.2.0 | 1.2.0 | [registry](https://registry.npmjs.org/@radix-ui%2freact-separator/latest) |
| `@radix-ui/react-slot` | `^1.2.4` | 1.2.4 | 1.5.0 | 1.5.0 | [registry](https://registry.npmjs.org/@radix-ui%2freact-slot/latest) |
| `@tanstack/react-query` | `^5.90.16` | 5.90.16 | 5.104.5 | 5.104.5 | [registry](https://registry.npmjs.org/@tanstack%2freact-query/latest) |
| `class-variance-authority` | `^0.7.1` | 0.7.1 | 0.7.1 | 0.7.1 | [registry](https://registry.npmjs.org/class-variance-authority/latest) |
| `clsx` | `^2.1.1` | 2.1.1 | 2.1.1 | 2.1.1 | [registry](https://registry.npmjs.org/clsx/latest) |
| `framer-motion` | `^12.25.0` | 12.25.0 | 14.4.0 | 14.4.0 | [registry](https://registry.npmjs.org/framer-motion/latest) |
| `lucide-react` | `^0.562.0` | 0.562.0 | 1.59.0 | 1.59.0 | [registry](https://registry.npmjs.org/lucide-react/latest) |
| `next-themes` | `^0.4.6` | 0.4.6 | 0.4.6 | 0.4.6 | [registry](https://registry.npmjs.org/next-themes/latest) |
| `pdfjs-dist` | `^5.5.207` | 5.5.207 | 6.4.299 | 6.4.299 | [registry](https://registry.npmjs.org/pdfjs-dist/latest) |
| `radix-ui` | `^1.4.3` | 1.4.3 | 1.8.1 | 1.8.1 | [registry](https://registry.npmjs.org/radix-ui/latest) |
| `react` | `^19.2.0` | 19.2.3 | 19.3.1 | 19.3.1 | [registry](https://registry.npmjs.org/react/latest) |
| `react-dom` | `^19.2.0` | 19.2.3 | 19.3.1 | 19.3.1 | [registry](https://registry.npmjs.org/react-dom/latest) |
| `react-hook-form` | `^7.71.0` | 7.71.0 | 7.90.0 | 7.90.0 | [registry](https://registry.npmjs.org/react-hook-form/latest) |
| `react-router-dom` | `^7.12.0` | 7.12.0 | 7.18.4 | 7.18.4 | [registry](https://registry.npmjs.org/react-router-dom/latest) |
| `sonner` | `^2.0.7` | 2.0.7 | 2.0.8 | 2.0.8 | [registry](https://registry.npmjs.org/sonner/latest) |
| `tailwind-merge` | `^3.4.0` | 3.4.0 | 3.7.0 | 3.7.0 | [registry](https://registry.npmjs.org/tailwind-merge/latest) |
| `tw-animate-css` | `^1.4.0` | 1.4.0 | 1.4.0 | 1.4.0 | [registry](https://registry.npmjs.org/tw-animate-css/latest) |
| `@eslint/js` | `^9.39.1` | 9.39.2 | 10.0.1 | 9.39.5 (a11y peer limit) | [registry](https://registry.npmjs.org/@eslint%2fjs/latest) |
| `@playwright/test` | `^1.59.1` | 1.59.1 | 1.64.1 | 1.64.1 | [registry](https://registry.npmjs.org/@playwright%2ftest/latest) |
| `@tailwindcss/vite` | `^4.1.18` | 4.1.18 | 4.3.3 | 4.3.3 | [registry](https://registry.npmjs.org/@tailwindcss%2fvite/latest) |
| `@testing-library/jest-dom` | `^6.9.1` | 6.9.1 | 7.0.1 | 7.0.1 | [registry](https://registry.npmjs.org/@testing-library%2fjest-dom/latest) |
| `@testing-library/react` | `^16.3.1` | 16.3.1 | 16.3.3 | 16.3.3 | [registry](https://registry.npmjs.org/@testing-library%2freact/latest) |
| `@testing-library/user-event` | `^14.6.1` | 14.6.1 | 14.6.8 | 14.6.8 | [registry](https://registry.npmjs.org/@testing-library%2fuser-event/latest) |
| `@types/node` | `^24.10.1` | 24.10.7 | 26.6.5 | 24.19.2 (Node 24) | [registry](https://registry.npmjs.org/@types%2fnode/latest) |
| `@types/react` | `^19.2.5` | 19.2.8 | 19.3.0 | 19.3.0 | [registry](https://registry.npmjs.org/@types%2freact/latest) |
| `@types/react-dom` | `^19.2.3` | 19.2.3 | 19.3.0 | 19.3.0 | [registry](https://registry.npmjs.org/@types%2freact-dom/latest) |
| `@vitejs/plugin-react` | `^5.1.1` | 5.1.2 | 6.1.2 | 6.1.2 | [registry](https://registry.npmjs.org/@vitejs%2fplugin-react/latest) |
| `@vitest/coverage-v8` | `^4.0.16` | 4.0.16 | 5.0.5 | 5.0.5 | [registry](https://registry.npmjs.org/@vitest%2fcoverage-v8/latest) |
| `@vitest/ui` | `^4.0.16` | 4.0.16 | 5.0.5 | 5.0.5 | [registry](https://registry.npmjs.org/@vitest%2fui/latest) |
| `eslint` | `^9.39.1` | 9.39.2 | 10.13.0 | 9.39.5 (a11y peer limit) | [registry](https://registry.npmjs.org/eslint/latest) |
| `eslint-plugin-jsx-a11y` | `^6.10.2` | 6.10.2 | 6.10.2 | 6.10.2 | [registry](https://registry.npmjs.org/eslint-plugin-jsx-a11y/latest) |
| `eslint-plugin-react-hooks` | `^7.0.1` | 7.0.1 | 7.1.1 | 7.1.1 | [registry](https://registry.npmjs.org/eslint-plugin-react-hooks/latest) |
| `eslint-plugin-react-refresh` | `^0.4.24` | 0.4.26 | 0.5.7 | 0.5.7 | [registry](https://registry.npmjs.org/eslint-plugin-react-refresh/latest) |
| `globals` | `^16.5.0` | 16.5.0 | 17.13.0 | 17.13.0 | [registry](https://registry.npmjs.org/globals/latest) |
| `jsdom` | `^27.4.0` | 27.4.0 | 30.1.3 | 30.1.3 | [registry](https://registry.npmjs.org/jsdom/latest) |
| `tailwindcss` | `^4.1.18` | 4.1.18 | 4.3.3 | 4.3.3 | [registry](https://registry.npmjs.org/tailwindcss/latest) |
| `typescript` | `~5.9.3` | 5.9.3 | 7.0.2 | 6.0.3 (parser limit) | [registry](https://registry.npmjs.org/typescript/latest) |
| `typescript-eslint` | `^8.46.4` | 8.52.0 | 8.72.0 | 8.72.0 | [registry](https://registry.npmjs.org/typescript-eslint/latest) |
| `vite` | `^7.2.4` | 7.3.1 | 8.3.4 | 8.3.4 | [registry](https://registry.npmjs.org/vite/latest) |
| `vite-plugin-pwa` | `^1.2.0` | 1.2.0 | 2.0.0 | 2.0.0 | [registry](https://registry.npmjs.org/vite-plugin-pwa/latest) |
| `vitest` | `^4.0.16` | 4.0.16 | 5.0.5 | 5.0.5 | [registry](https://registry.npmjs.org/vitest/latest) |

### Direct Go module inventory

The PDF parser candidate is an exact pseudo-version, not a stable tagged API. Review its commit range before adoption. Other candidates are latest within existing module paths; major-path replacements need a separate decision. Transitive dependencies are resolved/reviewed with Go during execution.

| Module | Current | Latest candidate | Minimum Go | Source |
| --- | --- | --- | --- | --- |
| `github.com/alicebob/miniredis/v2` | `v2.37.0` | `v2.40.0` | 1.17 | [module proxy](https://proxy.golang.org/github.com/alicebob/miniredis/v2/@latest) |
| `github.com/golang-jwt/jwt/v5` | `v5.3.0` | `v5.3.1` | 1.21 | [module proxy](https://proxy.golang.org/github.com/golang-jwt/jwt/v5/@latest) |
| `github.com/ledongthuc/pdf` | `v0.0.0-20250511090121-5959a4027728` | `v0.0.0-20260907135840-6c8c28e0e8a0` | 1.24.1 | [module proxy](https://proxy.golang.org/github.com/ledongthuc/pdf/@latest) |
| `github.com/lib/pq` | `v1.10.9` | `v1.12.3` | 1.21 | [module proxy](https://proxy.golang.org/github.com/lib/pq/@latest) |
| `github.com/redis/go-redis/v9` | `v9.17.2` | `v9.23.0` | 1.26.0 | [module proxy](https://proxy.golang.org/github.com/redis/go-redis/v9/@latest) |
| `golang.org/x/oauth2` | `v0.23.0` | `v0.37.0` | 1.26.0 | [module proxy](https://proxy.golang.org/golang.org/x/oauth2/@latest) |
| `google.golang.org/genai` | `v1.54.0` | `v1.75.0` | 1.24 | [module proxy](https://proxy.golang.org/google.golang.org/genai/@latest) |
| `modernc.org/sqlite` | `v1.44.1` | `v1.60.1` | 1.26.0 | [module proxy](https://proxy.golang.org/modernc.org/sqlite/@latest) |

### CI action inventory

Publisher releases are candidates, not verified runner-compatible updates. Review release notes and required runner versions/inputs, then pin the reviewed release to its commit SHA. `ubuntu-latest` is a GitHub-managed runner label, not an application runtime version. Deployment-action changes belong with #43.

| Action | Current reference | Latest release | Source |
| --- | --- | --- | --- |
| `actions/checkout` | v3 | v7.0.1 | [release](https://github.com/actions/checkout/releases/tag/v7.0.1) |
| `actions/setup-go` | v5 | v7.0.0 | [release](https://github.com/actions/setup-go/releases/tag/v7.0.0) |
| `actions/cache` | v4 | v6.1.0 | [release](https://github.com/actions/cache/releases/tag/v6.1.0) |
| `oven-sh/setup-bun` | v2 | v2.2.0 | [release](https://github.com/oven-sh/setup-bun/releases/tag/v2.2.0) |
| `docker/setup-buildx-action` | v3 | v4.4.1 | [release](https://github.com/docker/setup-buildx-action/releases/tag/v4.4.1) |
| `docker/login-action` | v3 | v4.6.0 | [release](https://github.com/docker/login-action/releases/tag/v4.6.0) |
| `docker/build-push-action` | v6 | v7.4.0 | [release](https://github.com/docker/build-push-action/releases/tag/v7.4.0) |
| `appleboy/ssh-action` | v1.0.3 | v1.2.5 | [release](https://github.com/appleboy/ssh-action/releases/tag/v1.2.5) |

## Execution plan

Each task is independently reviewable. Upgrade one dependency family at a time within it. Reuse existing tests; add a regression test when required source adaptation changes behavior or fixes a demonstrated defect. Configuration-only version bumps do not need mirror-image tests.

### Task 1: Align runtime and build environments (#36/#37)

**User Story:** As a maintainer, I want matching supported runtimes, so that local, CI and container builds are reproducible.

**Files:** `package.json`, `backend/go.mod`, `flake.nix`, `flake.lock`, `web/Dockerfile`, `backend/Dockerfile`, `.github/workflows/ci-cd.yml`, `README.md`, `README.docker.md`, applicable version instructions in `backend/AGENTS.md`. Coordinate overlapping work with the existing issue-43 plan.

**Interfaces:** Consumes existing workspace/module; produces root-lockfile installation and Node 24.22.0/Bun 1.4.3/Go 1.27.3 build environments.

**Edge Cases:** Exact Nix patch unavailable; root/web lock mismatch; arm64 versus Linux builds; missing tools; floating CI Bun; publishing/deploying CI on PRs.

**Definition of Done:** Selected runtime versions are recorded from real execution and consistent across local/Nix/CI/image declarations. Frozen install and both builds pass, or exact tool/version blockers remain explicit.

- [ ] Establish baseline from `dev`: root `rtk proxy bun install --frozen-lockfile`; in web run `rtk proxy bun run lint`, `rtk proxy bun run build`, `rtk proxy bun run test`; in backend with Go available run `rtk proxy go test ./...`. Record pre-existing failures.
- [ ] When using Nix, enter `rtk proxy nix develop`; update nixpkgs with `rtk proxy nix flake update nixpkgs` and evaluate the selected Go/Node/Bun/PostgreSQL/Redis versions. Verify attributes before choosing them. Record exact-version availability blockers rather than assuming matching patches.
- [ ] Pin root `"packageManager": "bun@1.4.3"` and matching Node/Bun engine requirements. Align Go 1.27.3 in go.mod, Nix, image and CI; prefer setup-go's supported go-version-file input reading backend/go.mod. Select Node 24.22.0 and pin CI Bun 1.4.3. Document actual setup commands.
- [ ] Repair web Docker workspace installation: work at `/app`, copy root package.json/bun.lock and web/package.json, install from root using pinned Bun, then copy frontend source/config and build in `/app/web`. Exclude local node_modules and copy `/app/web/dist` into nginx. Verify versioned builder/runtime image tags and digests before pinning them.
- [ ] Build without pushing: root `rtk proxy docker build -f web/Dockerfile -t annota-web:upgrade-check .` and `rtk proxy docker build -t annota-api:upgrade-check backend`. Inspect builder runtime versions. Missing final-image migration assets/readiness are coordinated under #43.
- [ ] Review/pin compatible build/test actions from the inventory. Do not trigger the existing publishing/deployment workflow as an upgrade check: it currently permits publishing/deploying on PR events. Resolve remote execution gating under #43 first; no production deployment is authorized here.

### Task 2: Update compatible frontend libraries (#36)

**User Story:** As a reader, I want maintained UI/data dependencies, so that navigation, reading and saved work stay reliable.

**Files:** root package.json, web/package.json, bun.lock; existing source only where an API adaptation is required.

**Interfaces:** Preserves routes, API shapes, query/cache ownership and semantic controls.

**Edge Cases:** React/types mismatch; Radix focus; route transitions; Tailwind merging; mobile touch controls; broad shadcn regeneration.

**Definition of Done:** Compatible targets are locked and frozen install/lint/build/affected tests pass; desktop/mobile screenshots preserve existing navigation and reading behavior.

- [ ] Upgrade React/React DOM to 19.3.1 and their types to 19.3.0 together. Update query/router/forms/Radix/Sonner/style utilities to their table targets. Keep already-current packages; upgrade Tailwind and its Vite plugin together to 4.3.3.
- [ ] Upgrade root shadcn CLI to 4.22.1 without regenerating components. Run root `rtk proxy bun install`, followed by `rtk proxy bun install --frozen-lockfile` to verify the resulting lock.
- [ ] In web run `rtk proxy bun run test src/pages/__tests__/DocumentPage.test.tsx src/pages/__tests__/HistoryPage.test.tsx src/components/scanpage/__tests__/AnnotationDrawer.test.tsx`, then lint/build. Use the browser for route/dialog/focus/mobile smoke checks and record screenshot paths.

### Task 3: Migrate compiler/build/lint/test tools (#36)

**User Story:** As a maintainer, I want current compatible tooling, so that checks detect real defects without unsupported peer combinations.

**Files:** web/package.json, bun.lock, web/tsconfig.app.json, web/tsconfig.node.json, web/tsconfig.vitest.json, web/vite.config.ts, web/eslint.config.js, web/src/test/setup.ts, web/playwright.config.ts if required.

**Interfaces:** Retains the SPA and bun run lint/build/test/test:e2e entry points.

**Edge Cases:** Deprecated compiler options; Rolldown/Oxc changes; mocking/timer changes; jsdom APIs; required versus optional peers; unsupported TS7/ESLint10.

**Definition of Done:** Vite/plugin-react 8.3.4/6.1.2 and Vitest family 5.0.5 work. TypeScript 6.0.3 and ESLint 9.39.5 meet current peer limits. Accessibility checks stay active and unsupported-version warnings are not suppressed.

- [ ] Read [Vite migration](https://vite.dev/guide/migration), [TypeScript 6 changes](https://www.typescriptlang.org/docs/handbook/release-notes/typescript-6-0.html), [Vitest migration](https://vitest.dev/guide/migration/) and exact selected publisher release notes, covering every crossed major.
- [ ] Upgrade TypeScript to 6.0.3. Remove deprecated baseUrl from web/tsconfig.app.json while retaining `"paths": { "@/*": ["./src/*"] }`. Check rootDir/types behavior using web `rtk proxy bunx tsc -b` and `rtk proxy bunx tsc -p tsconfig.vitest.json --noEmit`. Do not hide deprecations.
- [ ] Upgrade Vite/plugin-react together, retaining React/Tailwind/PWA integration. Optional React compiler/Babel peers do not require adding new tooling. Run production build before changing test tooling.
- [ ] Set ESLint/@eslint/js to 9.39.5; update typescript-eslint/hooks/refresh/globals to the table targets. Retain jsx-a11y and all current accessibility rules. Record TS7/ESLint10 compatibility blockers in #36.
- [ ] Upgrade Vitest/UI/coverage together to 5.0.5; update jsdom/jest-dom/testing-library/Playwright to their targets. Verify required peer resolution. Install matching Chromium with web `rtk proxy bunx playwright install chromium`; do not install every optional peer.
- [ ] Run web lint/build and one full `rtk proxy bun run test`; run `rtk proxy bun run test:e2e`. Existing E2E covers public legal/auth redirect pages only, not PDF or annotation behavior.

### Task 4: Qualify PDF/PWA and visual-library majors (#36)

**User Story:** As a reader, I want Japanese PDF rendering and app updates to keep working, so that dependency upgrades preserve reading and selection.

**Files:** web/package.json, bun.lock, web/src/lib/pdf/initPdfWorker.ts, web/src/lib/pdf/renderPage.ts, web/src/components/documentpage/ContinuousPDFViewer.tsx, web/vite.config.ts, web/src/components/scanpage/AnnotationDrawer.tsx, required icon imports and existing tests.

**Interfaces:** Preserves ensurePdfjsWorker(), getPdfDocumentLoadOptions(url), canvas/text layers and current PWA registration behavior; no new update-prompt feature.

**Edge Cases:** Library/worker/CMap mismatch; vertical Japanese/furigana; textless PDF; render cancellation; mobile selection; old service worker; drawer focus/reduced motion; renamed icon exports.

**Definition of Done:** PDF.js 6.4.299 assets match and Japanese rendering/selection works in real desktop/mobile browsers. PWA 2.0.0 transitions from the old build to the new one on one origin without stale/blank assets. Motion 14.4.0 and Lucide 1.59.0 preserve existing behavior. Screenshot and console/network evidence is saved.

- [ ] Read [PDF.js releases](https://github.com/mozilla/pdf.js/releases), [PWA releases](https://github.com/vite-pwa/vite-plugin-pwa/releases), [Motion upgrade guide](https://motion.dev/docs/react-upgrade-guide), [Lucide releases](https://github.com/lucide-icons/lucide/releases).
- [ ] Upgrade PDF.js to 6.4.299. In initPdfWorker.ts derive the CMap URL from installed `pdfjsLib.version` instead of the fixed 5.5.207 string. Verify actual 6.x worker/distribution paths and adapt render/TextLayer calls only where required.
- [ ] Run web `rtk proxy bun run test src/components/documentpage/__tests__/ContinuousPDFViewer.test.tsx src/pages/__tests__/DocumentPage.test.tsx`. Check Japanese text PDF, textless PDF and mobile selection in a real browser; mocks do not verify worker/CMap network requests.
- [ ] Upgrade PWA to 2.0.0; resolve required Workbox peers without unrelated optional packages. Use baseline and upgraded production builds at one origin to check service-worker activation, reload, offline shell and return online. Preserve intended caching/update semantics. [Auto-update behavior](https://vite-pwa-org.netlify.app/guide/auto-update).
- [ ] Upgrade Motion to 14.4.0 then Lucide to 1.59.0 independently. Build to catch removed API/icon exports. Check drawer open/close, focus, reduced motion and mobile icon meaning without redesign.
- [ ] Use T3 preview tooling first; save desktop and mobile viewport screenshots for reader/selection/annotation/history and the upgrade-loaded app. If evidence contradicts agreed behavior, pause for owner direction on fixing the mismatch or showing the current result.

### Task 5: Update Go dependencies (#37)

**User Story:** As an operator, I want maintained Go libraries, so that the backend receives fixes without breaking auth, storage or provider contracts.

**Files:** backend/go.mod, backend/go.sum; current auth/AI/PDF/storage callers and existing tests only for required API adaptations.

**Interfaces:** Preserves JSON API, JWT/OAuth semantics, database queries, Redis use and provider selection. No model/auth-provider migration.

**Edge Cases:** New Go floors; JWT/OAuth errors; provider schema changes; Redis errors; SQLite versus PostgreSQL; untagged PDF parser; indirect module updates.

**Definition of Done:** Each direct target is adopted or explicitly blocked. Module verify/build/vet/tests pass on Go 1.27.3. Actual PostgreSQL coverage runs rather than skips in a suitable isolated environment. Mocks do not count as live AI qualification.

- [ ] Run backend `rtk proxy go list -m -u all` on the approved Go toolchain to inventory direct/transitive updates; review changelogs and PDF parser commit range.
- [ ] Upgrade storage/auth batch with `rtk proxy go get github.com/redis/go-redis/v9@v9.23.0 github.com/lib/pq@v1.12.3 github.com/golang-jwt/jwt/v5@v5.3.1 golang.org/x/oauth2@v0.37.0 github.com/alicebob/miniredis/v2@v2.40.0 modernc.org/sqlite@v1.60.1`. Run auth/storage focused tests.
- [ ] Upgrade parser and AI SDK independently with `rtk proxy go get github.com/ledongthuc/pdf@v0.0.0-20260907135840-6c8c28e0e8a0` and `rtk proxy go get google.golang.org/genai@v1.75.0`. Preserve currently used SDKs; run affected PDF/AI package tests after required API adaptations.
- [ ] Run backend `rtk proxy go mod tidy`, `rtk proxy go mod verify`, `rtk proxy go build ./cmd/server`, `rtk proxy go vet ./...`, `rtk proxy go test ./...`; review indirect module changes. Run existing TestRunProductionMigrations_Postgres with POSTGRES_TEST_DSN for a disposable isolated database coordinated under #43, never production. A skipped DB test is not migration evidence.
- [ ] Verify the CGO_ENABLED=0 GOOS=linux container build and final startup/assets. Add race checks only for concrete concurrency changes or observed races.

### Task 6: Migrate stateful service runtimes separately (#43)

**User Story:** As a reader, I want documents and annotations to survive runtime upgrades, so that newer services do not lose existing work.

**Files:** docker-compose.yml, flake.nix, flake.lock, CI PostgreSQL service, docs/commercial-ops.md. Coordinate with docs/superpowers/plans/2026-10-20-issue-43-beta-release-readiness.md instead of duplicating its work.

**Interfaces:** Copies of PostgreSQL data/uploads and Redis persistence enter an isolated rehearsal; existing source storage remains available until recovery is verified.

**Edge Cases:** Existing PostgreSQL 13 Docker or 16 Nix cluster; changed PGDATA mount; writes during backup; missing uploads; Redis persistence incompatibility; restored ownership.

**Definition of Done:** PostgreSQL 18.6 and Redis 8.10.2 pass isolated upgrade/recovery checks with representative data/files retained. Old/new majors have separate storage. Application dependency installation never silently migrates data or production services.

- [ ] Inventory actual server versions/storage first. Preserve old data and backups. Prefer logical dump/restore into a new PostgreSQL 18.6 volume for this small app; use pg_upgrade only when the separate #43 plan justifies and verifies it.
- [ ] Apply PostgreSQL 18 image mount at /var/lib/postgresql with version-specific PGDATA. Never reuse 13/16 raw data with 18. For Nix create a separate 18 state directory and restore existing 16 data as needed; do not delete .direnv/dev-services to bypass incompatibility.
- [ ] Restore a representative database and matching uploaded files in isolation. Verify ownership/counts, migration runner, reader/history and persistence across container replacement. Record exact backup/restore commands and evidence in #43.
- [ ] Rehearse Redis 7 -> 8.10.2 on copied RDB/AOF data; test expiry, OAuth state/cache, reconnect and persistence restart. Review publisher release notes and licensing/distribution terms before adopting the image. Preserve old state; do not assume Redis 7 reads newer persistence formats.
- [ ] Align local/Nix/CI service targets after rehearsal. Production execution needs the separate release/data approval and runbook; no host service upgrade is authorized by this plan.

### Task 7: Record adoption and verification (#36/#37)

**User Story:** As a maintainer, I want exact upgrade evidence, so that I know what is current, intentionally deferred or unverified.

**Files:** This plan's final adoption record, existing #36/#37/#43 issues, runtime setup docs listed above.

**Interfaces:** Real version/check results produce an honest maintenance record.

**Edge Cases:** A release appears mid-task; compatible-line deferral; pre-existing failures; missing tools/keys; unrun provider checks.

**Definition of Done:** Every selected upgrade has a version and evidence or concrete blocker. Relevant checks run after final related changes; unresolved release blockers remain open. Report matches manifests/locks.

- [ ] Recheck root/web `rtk proxy bun outdated`, run `rtk proxy bun audit`, and backend `rtk proxy go list -m -u all`. Separate intentional TS/ESLint/Node-type deferrals from failed migrations. Audit output is not a full security audit.
- [ ] Record frontend frozen-install/lint/build/tests/E2E, backend verify/build/vet/tests, image evidence and screenshot paths. Do not repeatedly rerun full suites without a new change or concrete concern.
- [ ] Document exact runtime versions/setup and reference #36/#37 in implementation commits. Keep #43 open until its release/data evidence exists; do not close provider/auth/beta work or deploy.

## Order and tradeoff

Recommended: Task 1 -> 2 -> 3 -> 4 -> 5 -> 7. Task 6 is separately gated under #43. Go work can be prepared independently once its toolchain exists. New service configs may be prepared earlier, but old volumes remain usable until migration rehearsal and approval.

All-at-once upgrades make build/lint/PDF/PWA/data failures harder to attribute. Patch-only upgrades leave requested major migrations unresolved. Use family batches and explicit ecosystem blockers rather than changing architecture to force every registry latest.

## Planning verification

Inventory includes every direct package from both manifests and all direct modules from go.mod. Sources are linked per row. Candidate Go minimums fit 1.27.3; TypeScript/ESLint deferrals match current declared peers. This planning turn creates documentation only and does not claim application/runtime compatibility.

## Owner scope adjustment and execution record

- User authorized implementation and explicitly excluded Nix CLI because this project no longer uses it for this work. Leave flake.nix/flake.lock unchanged; verify with temporary native toolchains.
- Compose CLI is a host workflow tool, not an application runtime requirement. Existing Compose workflows still use it, but this upgrade does not require a CLI update; leave it unchanged. No database/Redis server or production data migration in this task.
- Baseline: root frozen install, frontend lint/build pass; 40 test files pass, 155 tests pass, 2 tests skipped.


## Implementation result (2026-10-20)

Implemented locally on `chore/dependency-runtime-upgrade-36-37`. All 45 direct npm packages are at their proposed targets; the only newer direct releases intentionally deferred are TypeScript 7, ESLint 10/@eslint-js 10, and Node 26 types for the documented compatibility reasons. All 8 direct Go modules reached their inventory targets. Compatible npm transitives were refreshed with `bun update`, and imported Go/test dependencies with `go get -u -t ./...` followed by tidy. Not every module in the wider Go dependency/test graph is independently forced to latest; its upstream-selected graph remains visible in `go list -m -u all`.

- Native verification used temporary checksum-verified Node 24.22.0, Bun 1.4.3 and Go 1.27.3 toolchains under /tmp, without replacing global tools.
- Root packageManager/engines, .node-version, frontend builder and CI now agree on the selected Node/Bun versions. Root dev command no longer invokes the Nix-only dev-services command. Existing PostgreSQL/Redis must already be running.
- Go module/CI/build selection is 1.27.3. Docker Hub returned MANIFEST_UNKNOWN for the target Go Alpine image. The backend builder therefore pins a published 1.27.2 Alpine image by digest and sets GOTOOLCHAIN=go1.27.3, selecting the exact downloaded toolchain. Availability of Linux amd64/arm64 toolchain archives was verified. Replace the bootstrap image when the desired image is actually published.
- Fixed frontend Docker installation to consume root workspace package.json/bun.lock; pinned available Bun/Node/nginx/Alpine images by digest. Updated build-job actions to reviewed release SHAs; deployment job/actions and event triggers remain unchanged under #43. No remote workflow was triggered.
- Migrated TypeScript baseUrl and Vite import.meta.dirname. Updated auth initialization to publish asynchronously loaded profile results from the subscription callback, with cancellation on cleanup, satisfying the new hooks lint behavior. No lint rule was disabled.
- PDF CMap URL now follows installed PDF.js version. The new regression test failed against old 5.5.207 CMaps after upgrading the library, then passed after the fix.

### Passed verification

- Root Bun 1.4.3 frozen-lockfile install.
- Frontend bun run lint, bun run build and test TypeScript config checking (`bunx tsc -p tsconfig.vitest.json --noEmit`).
- Frontend full Vitest 5 suite: 41 files pass, 1 file skipped; 156 tests pass, 2 existing tests skipped.
- Playwright 1.64.1 mobile-chrome E2E: 2 tests pass (legal pages and unauthenticated redirects).
- Go 1.27.3: go mod tidy/verify, go vet ./..., go test ./..., native build and CGO_ENABLED=0 GOOS=linux GOARCH=amd64 build. Binary metadata confirms Go 1.27.3 and static Linux ELF output.
- Existing TestRunProductionMigrations_Postgres passed against an isolated temporary native PostgreSQL 17.10 cluster, which was then stopped. This is fresh-schema migration coverage, not a production data-upgrade or PostgreSQL 18 rehearsal.
- CI YAML parsed and explicit Go cache ownership verified. No remote publishing/deployment workflow ran.
- Independent code review found the unpublished Go image issue; the bootstrap correction resolved it. No outstanding concrete code-review finding remained.

### Browser evidence

Production frontend, real PDF.js worker/text layer/CMap requests, synthetic Japanese PDF/API data, desktop 1280x800 and Pixel 7 412x915. No real OAuth or AI provider was called. Japanese text rendered and selected; explanation drawer opened, annotation saved/reappeared in history, and synthetic speech request returned HTTP 200. The synthetic standard-font PDF produced a system-font warning; text still rendered and selected. Browser fixtures were temporary and were not added to the app.

Saved screenshots:
- Desktop PDF: /Users/naufaldisatriya/.t3/userdata/browser-artifacts/browser-screenshot-127-0-0-1-mvg3g2f8-2fc69bcc.png
- Mobile annotation drawer: /Users/naufaldisatriya/.t3/userdata/browser-artifacts/browser-screenshot-127-0-0-1-mvg3iehf-1229a4be.png
- Mobile saved history: /Users/naufaldisatriya/.t3/userdata/browser-artifacts/browser-screenshot-127-0-0-1-mvg3j9z7-59ac1dfe.png
- Offline upgraded PWA route: /Users/naufaldisatriya/.t3/userdata/browser-artifacts/browser-screenshot-127-0-0-1-mvg3jp38-8be1ac30.png

PWA rehearsal used the baseline 1.2.0-plugin build then the upgraded 2.0.0-plugin build on the same local origin. New worker activated, cache contained new index-CWWye50D.js and removed old index-CqOMIyZ6.js. With that fixture server stopped, navigating to /privacy rendered the cached new build under service-worker control. The server was restarted afterward.

### Remaining limitations and intentionally excluded work

- `bun audit` dropped from 56 advisories to 1 high advisory: unpatched braces 3.0.3 via the shadcn CLI (GHSA-vfj7-8cjw-p6xm). Registry latest is still 3.0.3 and the advisory has no first_patched_version. Do not claim a clean audit or force an incompatible override. Track the upstream fix in #36.
- Docker is unavailable, so final images/startup were not executed. Published image digests and action metadata were checked; they are not a substitute for container execution.
- Nix CLI, flake.nix/flake.lock, Compose CLI, docker-compose.yml and PostgreSQL/Redis server versions remain unchanged as requested/separately scoped. No production data migration/deployment happened.
- PostgreSQL 18 upgrade/recovery and full final-image release safety remain #43. Live OAuth/provider qualification needs separately configured credentials and remains pending.
- Existing large-chunk build warning remains; no unrelated bundle-splitting refactor was added.


### PR integration safety adjustment

The owner authorized pushing, subagent review, fixes and merging on 2026-10-20. Preflight found that opening/merging the upgrade PR would execute the old registry publication and SSH deployment jobs. The upgrade branch now makes ordinary CI verification-only: read-only repository permissions, no production environment, no registry login, push:false for image builds and no deployment job. This permits remote dependency/image verification without a production release. Broader final-image packaging, recovery and release operations remain #43; no unrelated PR was merged to obtain this guard.
