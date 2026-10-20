# Secret Exposure Assessment and Default Database Password Guard Plan

> **For agentic workers:** Plan-then-execute is authorized by the owner. Independent review covers the PR only.

**Goal:** Resolve the repository side of [issue #12](https://github.com/ventodeco/annota-gemini-hackathon-3/issues/12). Record what the git evidence shows about the reported `.env` exposure. Stop production from starting with a known default PostgreSQL password, and document rotation and recovery steps. Operator-only actions stay as explicit open items.

**Architecture:** One production check in `backend/internal/config`, a local-only port binding change in `docker-compose.yml`, and documentation. Auth behavior, env templates, and Clerk-owned configuration do not change.

**Tech Stack:** Go 1.27.3 (`net/url`, `regexp`), Docker Compose YAML, Markdown.

## Global Constraints

- Branch: `backlog/issue-12`, fast-forwarded to `origin/dev` at `58deb43`. PR targets `dev`.
- Never print `.env` contents, key values, historical blob contents, or environment dumps. Evidence stays metadata-only or redacted.
- No credential rotation or revocation, no git history rewrite, and no production or VPS change. These need owner access and are listed as operator actions.
- Clerk migration #39 owns auth env and config: `GOOGLE_OAUTH_*`, `JWT_SECRET`, `TOKEN_EXPIRY_MINUTES`, `CLERK_*`, and `backend/.env.example`. This plan does not edit those entries. The only shared file is `config.go` (one call in `Validate`), and the new logic lives in a separate file to keep merge conflicts minimal.
- Prefix shell commands with `rtk`. Use `rtk proxy` for `go` and `gh`. Go is at `/tmp/annota-runtime-upgrade/go/bin`.
- Backend and docs only, so no screenshots. The test-debugging budget is shared with the rest of the issue's work (30 minutes total).

## Evidence (2026-10-20, metadata-only and redacted)

The issue says real keys were **committed** in `backend/.env`. The repository evidence does not support that:

1. `git ls-files` tracks no `.env` file. Tracked env-like files are `.envrc`, `backend/.env.example`, and `web/.env.example`, plus a plan document whose name contains "credentials".
2. All 22 remote branches and all 26 `refs/pull/*/head` refs were fetched. `git rev-list --all --objects` finds no path named `.env` in any commit. The only env-like paths ever recorded are `.env.example` (root, since removed), `backend/.env.example`, `web/.env.example`, and `.envrc`.
3. A redacted pattern scan of all 1,145 reachable blobs found no matches for Google API keys (`AIza…`), Google OAuth client secrets (`GOCSPX-…`), `sk-` style keys, Clerk secret keys, AWS access keys, GitHub tokens, or private-key blocks. Every non-placeholder-looking `*SECRET*/*PASSWORD*/*TOKEN*` assignment it flagged is a code identifier or a test fixture (`config_test.go`, values like `0123…` and `shor…`).
4. `.gitignore` (`.env`, `.env.local`, `.env.*.local`) and `backend/.gitignore` (`.env`) ignore `backend/.env`, `web/.env`, and `.env`. `.dockerignore` and `backend/.dockerignore` exclude `**/.env` and `**/.env.*`.
5. The templates use placeholders only.
6. The main checkout has no `backend/.env` right now (existence check only; nothing was read).

**Conclusion:** Within the scanned refs, path metadata, and provider-pattern coverage, there is no evidence of a committed `.env` or a committed provider key, so no history rewrite is indicated. The pattern scan covers only the listed formats and cannot prove that no secret was exposed. This assessment does not determine where the reported values were or whether they were exposed outside git. The rotation decision belongs to the owner, as an operator action.

**Residuals in the repository, confirmed in the source:**

- **R1, built-in default DB password:** `config.Load` builds the DSN with `POSTGRES_PASSWORD` defaulting to `gemini_password` when unset. The template and `docker-compose.yml` use `password`. `Validate` has no production check. A production start with an unset or template password would try a publicly known password, and nothing stops it.
- **R2, compose ports on all interfaces:** `docker-compose.yml` publishes PostgreSQL (password `password`) and Redis (no auth) on all host interfaces. Production deploys use separate compose files on the VPS (`origin/master` workflow: `~/project-vps/.../docker-compose.yml`), so this file is local-development only. Its ports are still reachable from the LAN.

## Task 1: Reject known default PostgreSQL passwords in production

**User Story:** As the Annota operator, I want the backend to refuse to start in production when its database password is a known default, so a missing or copied-from-template password cannot reach a deployed database.

**Files:**

- Create: `backend/internal/config/database_password.go`
- Modify: `backend/internal/config/config.go` (one call in `Validate`, after the DSN-required check)
- Modify: `backend/internal/config/config_test.go`

**Behavior:**

- Only when `AppEnv == "production"`: extract the password from `DBConnectionString` and fail with an error if it exactly matches `gemini_password`, `password`, or `postgres`. The error message must not include the value.
- URL DSNs (`postgres://`, `postgresql://`): password from `url.Parse(...).User.Password()`.
- Key/value DSNs: last `password=` value, with an optional single-quoted value and lib/pq-style `\'` and `\\` escapes.
- No password present: allowed (it may come from `PGPASSWORD` or `.pgpass`). A DSN that cannot be parsed is left to the driver.
- Development or any other `APP_ENV`: unchanged, so local defaults and the smoke script (`APP_ENV=development`) keep working.

**Edge cases tested:** URL and key/value forms, quoted key/value, an unset `POSTGRES_PASSWORD` in production through `Load` (built-in default), a strong password in production accepted, a default password in development accepted, and an error message that omits the password.

## Task 2: Bind local compose services to loopback

**Files:** Modify `docker-compose.yml`.

Change the ports to `127.0.0.1:5432:5432` and `127.0.0.1:6379:6379`, and add a comment that this file is for local development only and its credentials are not for deployed environments. Same-host access is unchanged and the default password stays, so existing local volumes keep working.

## Task 3: Exposure assessment and rotation runbook

**Files:**

- Create: `docs/evidence/issue-12/secret-exposure-assessment.md`: the redacted evidence above, the reproducible commands, and the scan method (rules only, no values).
- Modify: `docs/commercial-ops.md` "Secrets Rotation": where each credential lives (VPS host compose env, the GitHub `ENV_PRODUCTION` secrets `DOCKERHUB_TOKEN`/`VPS_*`, the developer's local `backend/.env`), the order "rotate or revoke at the provider first, then update the deploy, then verify, then retire the old value", how to verify without printing values, the effect of a git history rewrite (rewriting does not un-expose a credential; rotate first), the default-password guard, and a note that Clerk (#39) will replace `JWT_SECRET`/`GOOGLE_OAUTH_*`.

## Acceptance Criteria

- [ ] Production `Validate` rejects the three default passwords in URL and key/value DSNs, including the built-in default from `Load`. Development is unchanged. Focused tests pass.
- [ ] `go test ./internal/config`, `go vet ./...`, and `go build ./...` pass from `backend/`.
- [ ] The compose file binds to loopback and states it is local-only.
- [ ] The evidence and runbook are committed and contain no secret values.
- [ ] The PR links #12, distinguishes the unsupported "committed" premise from the verified residuals, and passes a fresh independent review and the required CI.
- [ ] Issue #12 stays open for operator actions, listed below.

## Operator Actions (not done here; need owner access)

1. Decide whether to rotate the credentials named in the issue (Gemini/OpenRouter/MiniMax keys, Google OAuth client secret, `JWT_SECRET`), wherever they were held. Record the date and provider, never the values.
2. On the production VPS, confirm without printing values: `APP_ENV=production`, a non-default database password, PostgreSQL and Redis not published publicly, and a `JWT_SECRET` that is not a template value (until Clerk replaces it).
3. GitHub `ENV_PRODUCTION` environment and credential-scope audit: already tracked as an administrator prerequisite in #43/#50. It is referenced here, not duplicated.

## Verification

From `backend/`: `rtk proxy go test -count=1 ./internal/config`, `rtk proxy go vet ./...`, `rtk proxy go build ./...`. Review the docs and compose diffs, and grep the diff for secret-like values.
