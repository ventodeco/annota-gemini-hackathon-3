# Issue #39 Clerk Authentication Migration Plan

> **For agentic workers:** Implemented inline by the #39 owner thread (A). The user's latest direction removes the separate plan review: A writes this plan, implements it, and GPT-6.1 Sol reviews the pull request only. Steps use checkbox (`- [ ]`) syntax.

**Goal:** Replace the bespoke Google OAuth and locally minted JWT flow with Clerk-managed sessions (Google and email) while the Go API keeps owning authorization and the numeric `users.id` ownership model.

**Architecture:** The React SPA uses `@clerk/react` prebuilt `<SignIn>`/`<SignUp>` inside the existing `BrowserRouter`. Every API call sends a fresh Clerk session token as `Authorization: Bearer`. Go verifies the token with `clerk-sdk-go/v2` (pinned issuer, explicit authorized parties, cached JWKS). It maps the Clerk subject to the local numeric user through an additive `users.clerk_user_id` column, provisioning or linking legacy Google rows idempotently under a PostgreSQL advisory lock. Account deletion writes a subject tombstone so stale tokens cannot recreate a profile.

**Tech Stack:** Go 1.27.3, `github.com/clerk/clerk-sdk-go/v2` v2.7.0, PostgreSQL migrations, React 19.3 + Vite 8, `@clerk/react` 6.x, TanStack Query, Vitest.

## Global Constraints

- Keep numeric `users.id` and all scan/document/annotation foreign keys unchanged. The migration is additive only.
- `CLERK_SECRET_KEY` is backend-only. `VITE_CLERK_PUBLISHABLE_KEY` / `CLERK_PUBLISHABLE_KEY` are public configuration. No real keys appear in the repo, issues, logs or screenshots.
- Never transfer legacy content based on an email address alone. A legacy link requires a Clerk-verified Google external account whose `provider_user_id` equals the legacy `users.provider_id` (`provider='google'`).
- All `/v1/*` auth failures use `httputil.WriteJSONError`.
- No added TypeScript `as` casts, no nested ternaries, and Bun only. Follow `vercel-react-best-practices`.
- Migrations are transaction-safe: no `CREATE INDEX CONCURRENTLY`. This matches the #43 runner contract (PostgreSQL-only, migration and ledger atomic under `pg_advisory_xact_lock`).
- Keep Redis as a startup dependency. It is shared infrastructure and the planned #43 `/readyz` checks DB and Redis. Its only code use (OAuth state) disappears, which is recorded in the hand-off note, not acted on.
- The 30-minute test-debugging budget per issue is shared with reviewers.

## Investigation summary (latest `dev` = `59500da`)

- No earlier PR implements any part of #39. PRs #46–#51 touched CI, dependencies, the index migration and AI routing only.
- Current legacy auth lives in `backend/internal/auth/{google,token}.go`, `backend/internal/handlers/auth.go` (+ `auth_redirect_test.go`), `backend/internal/middleware/auth.go` (`x-token` or Bearer HS256), `config.go` (`GOOGLE_OAUTH_*`, `JWT_SECRET`, `TOKEN_EXPIRY_MINUTES`), the frontend `AuthContext.tsx`, `useAuthApi.ts`, `LoginPage.tsx`, `OAuthCallbackPage.tsx`, and `api.ts` (`localStorage.auth_token`, `x-token`).
- `GET /v1/users/me` returns only `preferredLanguage`, yet `ProfilePage`/`AvatarMenu` render `user.email`/`provider`/`avatar_url`. Those fields are always empty today. After this change, identity display comes from Clerk's `useUser()`.
- `clerkhttp.WithHeaderAuthorization` is unsuitable as-is. It passes through when the token is missing, writes bare non-JSON 401/403 responses, and `jwt.Verify` only checks that `iss` looks like *some* Clerk issuer. We call `jwt.Verify` from our own middleware with `ProxyURL` set to the exact instance issuer, which makes the SDK compare `iss` for equality.
- No CSP exists (`web/nginx.conf`, `index.html`), so there is none to update. CI does not run Playwright. The backend image smoke passes Google/JWT fixtures that must become Clerk fixtures.

## Decisions

1. **Email method.** Prebuilt Clerk components make the email method a Clerk Dashboard setting with no method-specific screens in code. Proposed default: email verification code (OTP). This stays pending until the owner confirms it.
2. **Cutover.** This PR removes the legacy endpoints, token minting and `localStorage` token. Keeping both paths would double the auth test matrix and keep the known-flawed flow alive. "Verified cutover" means the owner's live Clerk qualification before any production deploy. Issue #39 stays open until that live evidence exists. Old legacy tokens receive a JSON 401 after deploy; users sign in once more through Clerk.
3. **Email-first legacy trap.** A legacy Google user who first signs in by email code would get a new empty profile, and Clerk could later attach Google to that same Clerk subject, stranding the legacy row. To prevent duplicate ownership, first-time provisioning without a Google match, where the Clerk-verified primary email equals an *unlinked* legacy Google row's email, returns `409` with message `legacy_google_link_required`. Nothing is transferred. The SPA explains: "continue with Google to restore your library". Clerk links Google to the same subject by verified email, and the next request links by Google subject. The operator runbook covers users who lost their Google account.
4. **Webhooks.** None. Provisioning is lazy on the first authenticated request. Clerk Dashboard user deletion is an operator reconciliation step in the runbook, and the Dashboard should disable user self-deletion so deletion always goes through ANNOTA. End-to-end deletion verification is shared with #41.

## Files

Backend:
- Create `backend/internal/auth/clerk.go`: issuer derivation, `SessionVerifier`, JWKS key cache, `IdentityDirectory` (Clerk Backend API).
- Create `backend/internal/auth/clerk_test.go`.
- Create `backend/internal/auth/identity.go`: `IdentityResolver` (lookup, tombstone, provisioning policy).
- Create `backend/internal/auth/identity_test.go`.
- Create `backend/migrations/005_clerk_identity.sql`.
- Modify `backend/internal/storage/db.go`, `backend/internal/models/user.go`, `backend/internal/testutil/mocks.go`, and `backend/internal/storage/migrate_test.go` (Postgres provisioning/concurrency coverage).
- Modify `backend/internal/middleware/auth.go` + `auth_test.go`, `middleware/cors.go` + `cors_test.go`.
- Modify `backend/internal/handlers/user.go` + `handlers_test.go` (deletion).
- Modify `backend/internal/config/config.go` + `config_test.go`, and `backend/cmd/server/main.go`.
- Delete `backend/internal/auth/google.go`, `auth/token.go`, `handlers/auth.go`, `handlers/auth_redirect_test.go`.
- Run `go mod tidy` in `backend/go.mod`/`go.sum`.

Frontend:
- Add `@clerk/react` to `web/package.json`, and update root `bun.lock`.
- Create `web/src/lib/authSession.ts`: token getter and unauthorized-handler registry, plus `clearUserScopedCaches`.
- Modify `web/src/lib/api.ts` (async Bearer header, `ApiError`, no `localStorage`), `web/src/lib/types.ts`, and `web/src/lib/authErrors.ts`.
- Rewrite `web/src/contexts/AuthContext.tsx`. Modify `web/src/contexts/useAuth.ts` and `web/src/hooks/useAuthApi.ts` (logout only).
- Modify `web/src/App.tsx` (Clerk provider inside the router, `/login/*`, `/sign-up/*`, deep-link return, config-error screen, legacy-link screen).
- Modify `web/src/pages/LoginPage.tsx`, `ProfilePage.tsx`, `WelcomePage.tsx`, `components/layout/AvatarMenu.tsx`, and `LegalPage.tsx` (one Clerk disclosure line; the full rewrite belongs to #41).
- Create `web/src/pages/SignUpPage.tsx`.
- Delete `web/src/pages/OAuthCallbackPage.tsx` and its test, plus `hooks/__tests__/useAuthApi.initiateLogin.test.tsx`.
- Tests: `web/src/lib/__tests__/api.test.ts`, new `web/src/__tests__/authRouting.test.tsx`, new `web/src/contexts/__tests__/AuthContext.test.tsx`, and adjusted page tests.

Ops/docs:
- `backend/.env.example`, `web/.env.example`, `web/Dockerfile`, `.github/workflows/ci-cd.yml` (build arg), `scripts/ops/smoke-images.sh` (Clerk fixtures).
- `README.md`, `RUN.md`, `backend/AGENTS.md` env list, `backend/docs/auth-flow.md`, `backend/docs/openapi.yaml`, Bruno collection, and a superseded note in `web/docs/FRONTEND_AUTH_PLAN.md`.
- New `docs/clerk-auth.md`: setup, cutover/user impact, inventory SQL, reconciliation and deletion policy, and the plan/cost review checklist.

## Task 1: Verify Clerk session tokens in Go

**User Story:** As a beta learner, I want the API to accept only my current ANNOTA Clerk session so that no other token can read my material.

**Interfaces (produced):**
- `auth.IssuerFromPublishableKey(pk string) (string, error)`: decodes `pk_test_`/`pk_live_` base64 `<frontend-api-host>$` into `https://<host>`.
- `auth.NewSessionVerifier(cfg SessionVerifierConfig) (*SessionVerifier, error)` with `SessionVerifierConfig{Issuer string; AuthorizedParties []string; Keys KeySource; Clock clerk.Clock; Leeway time.Duration}`.
- `(*SessionVerifier).Verify(ctx, token string) (subject string, err error)`.
- `auth.KeySource` interface `Key(ctx, kid string) (*clerk.JSONWebKey, error)`. Implementations are `NewJWKSKeySource(*jwks.Client, ttl)` (caches keys for 1h and refetches unknown `kid` at most once per minute) and `NewStaticKeySource(pem string)` (optional networkless `CLERK_JWT_KEY`).

**Edge Cases:** missing/garbled token; expired; `nbf` in the future; wrong signing key; HS256 legacy token; `none`/alg mismatch; issuer from another Clerk instance; missing or foreign `azp`; empty subject; unknown `kid`; JWKS fetch failure, which surfaces as `ErrVerifierUnavailable` → 503 rather than 401.

**Definition of Done:** Table-driven tests sign RS256 tokens with a local key (no network) and prove that each edge case is rejected and a valid token returns its subject. `IssuerFromPublishableKey` rejects malformed keys.

- [ ] Write the failing tests in `clerk_test.go`, implement `clerk.go`, then run `rtk proxy go test ./internal/auth`.

## Task 2: Durable Clerk-to-local mapping, provisioning and tombstones

**User Story:** As a returning legacy Google user, I want my first Clerk sign-in to land on my existing library. As a new user, I want exactly one profile even if the app fires several first requests at once.

**Migration `005_clerk_identity.sql`:**
```sql
ALTER TABLE users ADD COLUMN IF NOT EXISTS clerk_user_id TEXT;
CREATE UNIQUE INDEX IF NOT EXISTS idx_users_clerk_user_id ON users(clerk_user_id) WHERE clerk_user_id IS NOT NULL;
CREATE TABLE IF NOT EXISTS deleted_clerk_users (
    clerk_user_id TEXT PRIMARY KEY,
    deleted_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP
);
```

**Interfaces (produced):**
- `storage.DB` gains:
  - `GetUserByClerkID(ctx, clerkUserID string) (*models.User, error)`
  - `IsClerkUserDeleted(ctx, clerkUserID string) (bool, error)`
  - `ProvisionClerkUser(ctx, p storage.ClerkProvisionParams) (*models.User, error)`
  - `DeleteUserAndTombstoneClerk(ctx, userID int64, clerkUserID string) error`
  - `TombstoneClerkUser(ctx, clerkUserID string) error`
- `storage.ClerkProvisionParams{ClerkUserID, Email, AvatarURL string; GoogleSubject string}`. It returns `storage.ErrClerkUserDeleted` or `storage.ErrLegacyLinkRequired`.
- `ProvisionClerkUser` runs one transaction under `pg_advisory_xact_lock(hashtext('clerk:'||sub))`:
  1. re-select by `clerk_user_id`;
  2. check the tombstone;
  3. if `GoogleSubject` is set, `UPDATE users SET clerk_user_id=$sub … WHERE provider='google' AND provider_id=$g AND clerk_user_id IS NULL RETURNING …`;
  4. otherwise, if an unlinked `provider='google'` row has `lower(email)=lower($email)`, return `ErrLegacyLinkRequired`;
  5. otherwise insert with `provider='clerk'`, `provider_id=sub`, `clerk_user_id=sub`.
- `models.User` gains `ClerkUserID *string`.
- `auth.IdentityDirectory` interface:
  - `GetIdentity(ctx, clerkUserID) (*auth.ClerkIdentity, error)`, returning `ErrIdentityNotFound` or a wrapped unavailable error
  - `DeleteUser(ctx, clerkUserID) error`, which treats 404 as success
- `auth.ClerkIdentity{UserID, VerifiedPrimaryEmail, AvatarURL, GoogleSubject string}`. `GoogleSubject` is taken only from an `oauth_google` external account whose verification status is `verified`.
- `auth.IdentityResolver.Resolve(ctx, sub) (auth.Resolution, error)` where `Resolution{UserID int64; Deleted bool}`. Errors are `ErrIdentityUnavailable` (503), `ErrIdentityNotFound` (401), `ErrVerifiedEmailRequired` (403) and `storage.ErrLegacyLinkRequired` (409).

**Edge Cases:** two concurrent first requests (exactly one row); legacy Google subject match; Google then email for the same Clerk subject (same row); email-first legacy user (409, no transfer); duplicate legacy emails (409; no row picked); unverified Clerk email (403); tombstoned subject (never re-created); Clerk Backend API outage on first request (503, existing users unaffected).

**Definition of Done:**
- Unit tests cover the resolver policy with mocks.
- `TestClerkProvisioning_Postgres` (extends the `POSTGRES_TEST_DSN` suite) proves on real PostgreSQL:
  - migration 005 on fresh and upgraded schemas;
  - 8 concurrent provisions produce one row;
  - legacy linking keeps the numeric id and owned scans;
  - the email-only legacy collision returns `ErrLegacyLinkRequired`;
  - tombstoned subjects are refused.
- CI logs show these tests ran, not skipped. Locally they run against a disposable Homebrew PostgreSQL 17 cluster in `/tmp`.

- [ ] Write the migration, storage methods, mocks, resolver and tests, then run `rtk proxy go test ./internal/auth ./internal/storage` with `POSTGRES_TEST_DSN`.

## Task 3: Middleware, deletion, config and server wiring

**User Story:** As a learner, I want sign-out and account deletion to remove my access everywhere, and errors to stay consistent JSON.

**Behaviour:**
- `middleware.NewAuthMiddleware(verifier TokenVerifier, resolver UserResolver)`. It takes only `Authorization: Bearer`; `x-token` is no longer read. It verifies, then resolves.
- A tombstoned subject reaches the handler only for `DELETE /v1/users/me`, with `GetUserID==0` and `GetClerkUserID` set. Every other request gets 401 `Unauthorized: account deleted`.
- Context helpers: `GetClerkUserID(ctx) string`. `WithUserID` stays as is.
- `DELETE /v1/users/me`:
  1. purge owned files (existing behaviour);
  2. `DeleteUserAndTombstoneClerk` in one transaction;
  3. `directory.DeleteUser(sub)`.
  If Clerk deletion fails, the response is `502 Account data deleted; sign-in removal failed, retry`. A retry with the same session (now tombstoned) only re-attempts the Clerk deletion and returns 204.
- `NewUserHandlersWithStorage(db, fileStorage, cfg, directory auth.IdentityDirectory)`.
- Config:
  - Required: `CLERK_SECRET_KEY` (`sk_test_`/`sk_live_`) and `CLERK_PUBLISHABLE_KEY` (the issuer is derived from it).
  - Optional: `CLERK_JWT_KEY` (PEM).
  - `CLERK_AUTHORIZED_PARTIES` is a comma-separated list of origins, defaulting to `FRONTEND_BASE_URL`. It must contain absolute `http(s)` origins without a path, and `*` is rejected.
  - Remove `GOOGLE_OAUTH_CLIENT_ID`, `GOOGLE_OAUTH_CLIENT_SECRET`, `JWT_SECRET` and `TOKEN_EXPIRY_MINUTES`.
- `main.go`: remove `/v1/auth/google/*`. Keep the Redis connection at startup. Build the verifier with a `jwks.Client` that uses an instance `ClientConfig` (no global `clerk.SetKey`).
- CORS allowed headers become `Content-Type, Authorization`.

**Edge Cases:** missing header → 401 JSON; legacy `x-token` only → 401; legacy HS256 Bearer → 401; tombstoned subject on GET → 401; deletion when Clerk fails then retry; deletion with file purge failure (unchanged 500, nothing deleted); config with wildcard parties; config with malformed keys.

**Definition of Done:** `middleware`, `handlers` and `config` tests cover each edge case. `rtk proxy go test ./...`, `go vet ./...` and `go build ./cmd/server` pass. `go mod tidy` drops unused `golang.org/x/oauth2`/`jwt/v5` direct requirements where they are no longer imported.

## Task 4: Frontend Clerk session, routing and cache isolation

**User Story:** As a learner on mobile or desktop, I want to sign in with Google or email, return to the page I opened, and have sign-out or account switching leave no previous-account data on screen.

**Behaviour:**
- `main`/`App`: `QueryClientProvider > BrowserRouter > ClerkRoot`. `ClerkRoot` renders `<ClerkProvider publishableKey routerPush routerReplace afterSignOutUrl="/login">` and then `AuthProvider`. If `VITE_CLERK_PUBLISHABLE_KEY` is missing, an accessible "Sign-in is not configured" screen renders instead of a white screen.
- `authSession.ts`: `setSessionTokenGetter`, `getSessionToken`, `setUnauthorizedHandler`, `notifyUnauthorized`, and `clearUserScopedCaches(queryClient)` (`queryClient.clear()`, `caches.delete('api-cache')`, and removal of the leftover `auth_token` key).
- `api.ts`: every request awaits `getSessionToken()` and sends `Authorization: Bearer`. A 401 calls `notifyUnauthorized()`, so `AuthProvider` signs out through Clerk instead of hard-redirecting; this avoids a redirect loop. Non-OK responses throw `ApiError{status, message}`, parsed from the JSON `message` when present.
- `AuthProvider` reads Clerk `useAuth`/`useUser` and `useQuery(['user', userId], getUserProfile)`. Its value is `{ user: {email, avatarUrl, preferredLanguage} | null, isLoading, isAuthenticated, profileError, logout, refreshUser }`. When `userId` changes from one non-null value to another, or on sign-out, it clears user-scoped caches. This effect synchronises with the external Clerk session.
- Routes:
  - `/login/*` renders `<SignIn routing="path" path="/login" signUpUrl="/sign-up" fallbackRedirectUrl="/welcome">`, wrapped by the existing logo and legal copy.
  - `/sign-up/*` renders `<SignUp>`.
  - `ProtectedRoute` redirects to `/login?redirect_url=<path+search>`.
  - `PublicRoute` sends signed-in users to a sanitised same-origin relative `redirect_url`, or `/welcome`.
  - `/auth/callback` now redirects to `/login`.
- A profile 409 `legacy_google_link_required` shows a screen with "Continue with Google", which signs out and returns to `/login`. A 503 shows a retry state.
- `ProfilePage`/`AvatarMenu`/`WelcomePage` use the new `user` shape. Deletion calls `clearUserScopedCaches` and then signs out. `LegalPage` gains one sentence disclosing Clerk as the authentication processor.

**Edge Cases:** missing publishable key; deep link to `/documents/5?x=1`; malicious `redirect_url=//evil.example`; Clerk not yet loaded (busy state); expired session (Clerk refreshes the token; a 401 signs out); account switch A→B (caches cleared before B's queries); legacy-link 409; Clerk outage 503; sign-out from the avatar menu; delete account failure keeps the user signed in with an error.

**Definition of Done:** Vitest covers:
- the Bearer header and the absence of `x-token`/`localStorage`;
- 401 → handler;
- `ApiError` parsing;
- protected/public redirects, including the unsafe `redirect_url`;
- cache clearing on sign-out and on account switch;
- the legacy-link screen;
- the config-error screen.

`bun run test`, `bun run lint` and `bun run build` pass, and there are no new `as` casts (verified with `git diff | grep ' as '`).

## Task 5: Ops, docs and evidence

**User Story:** As the operator, I want accurate env/docs, cutover notes and a reconciliation runbook so that I can configure Clerk privately and qualify the cutover.

**Definition of Done:**
- The env examples, Dockerfile build arg, CI build arg, smoke fixtures, README/RUN/backend AGENTS, OpenAPI, Bruno and auth-flow docs reflect Clerk and the removed legacy endpoints.
- `docs/clerk-auth.md` documents:
  - Dashboard setup: Google, the email code default pending the owner, allowed origins, dev/prod instances, and disabled self-deletion;
  - cutover impact: one re-sign-in, legacy tokens rejected, and legacy users asked to use Google first;
  - the read-only inventory SQL;
  - the reconciliation runbook;
  - the deletion and retry policy;
  - a checklist for Clerk plan, email-delivery limits and cost review, unverified here.
- The CI backend image smoke passes with Clerk fixtures and no network.

**Browser evidence:**
- Desktop (1280×800) and mobile (390×844) screenshots of: `/login` with the config-error state (no key); the protected deep-link redirect; and, if the owner provides a development publishable key through `request_secret`, the real Clerk sign-in UI with Google and email.
- Paths are recorded below. Real Google/email sign-in, provisioning against a live Clerk instance, and deletion against Clerk are **pending owner credentials**. The PR and issue must state this.
- If screenshots contradict the agreed outcome, pause and ask the owner.

## Coordination notes (for #40/#41/#42/#7)

- **Auth contract:** `Authorization: Bearer <Clerk session JWT>` on every `/v1/*` call. `x-token`, `/v1/auth/google/state` and `/v1/auth/google/callback` are removed.
- **Ownership:** `middleware.GetUserID(ctx)` still returns the stable numeric `users.id`. The local profile is provisioned on the first authenticated request.
- **Status codes:** 401 means invalid or deleted session; 403 means verified email required; 409 means `legacy_google_link_required`; 503 means the identity service is unavailable.
- **Identity fields:** email and avatar come from Clerk on the client. `/v1/users/me` keeps preferences only.
- **Deletion:** a tombstone in `deleted_clerk_users`. Retry is idempotent through `DELETE /v1/users/me`. #41 owns the end-to-end deletion and service-worker cache scoping.
- **Redis:** unused by code after this change, but kept as a startup dependency for #43 `/readyz`.

## Verification log

(Filled in during execution.)
