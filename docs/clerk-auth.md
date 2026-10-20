# Clerk Authentication

ANNOTA uses [Clerk](https://clerk.com) for Google and email sign-in (#39). Clerk owns identities and browser
sessions; the Go API still owns authorization and all learning data, keyed by the existing numeric `users.id`.

## How it works

- **Web app:** `@clerk/react` prebuilt `<SignIn>` (`/login/*`) and `<SignUp>` (`/sign-up/*`) inside the existing
  `BrowserRouter`. Every API call sends `Authorization: Bearer <session token>` from Clerk `getToken()`; nothing
  is stored in `localStorage`.
- **API:** `internal/auth/clerk.go` verifies RS256 session tokens with `clerk-sdk-go/v2`:
  - the issuer must equal the instance derived from `CLERK_PUBLISHABLE_KEY`, so tokens from another Clerk
    instance are rejected;
  - `azp` must be one of `CLERK_AUTHORIZED_PARTIES`;
  - `exp`/`nbf` are enforced with 5s leeway;
  - signing keys are cached for 1h, with refetch for unknown key IDs bounded to once a minute.
- **Mapping:** `users.clerk_user_id` (migration `005_clerk_identity.sql`, additive) links a Clerk user to one
  local user. The first authenticated request provisions or links the profile inside one transaction holding
  `pg_advisory_xact_lock` for that Clerk user, so concurrent first requests produce exactly one row.
- **Responses:**

  | Status | Meaning |
  | --- | --- |
  | 401 | Missing, invalid, expired or wrong-instance token, or a deleted account |
  | 403 | The Clerk user has no verified primary email |
  | 409 | `legacy_google_link_required` (see below) |
  | 503 | Clerk keys or Backend API unavailable for a first-time user; existing users keep working |

## Clerk Dashboard setup (owner, private)

Use separate **development** and **production** Clerk instances.

1. **Sign-in options:** enable Google and email.
   - The proposed default email method is a **one-time email verification code**. This still awaits owner
     confirmation; email + password with recovery is the alternative.
   - The prebuilt components follow whatever is enabled, so no code change is needed either way.
   - Require email verification at sign-up.
2. **Google:** production needs custom Google OAuth credentials in Clerk; development can use Clerk's shared
   credentials.
3. **Origins:** add the frontend origin(s) to allowed origins and redirect URLs. Set
   `CLERK_AUTHORIZED_PARTIES` to the same origins, or rely on `FRONTEND_BASE_URL`.
4. **Account deletion:** disable "Allow users to delete their accounts". Deletion must go through ANNOTA so
   local data, files and the tombstone stay consistent.
5. **Keys:**
   - `CLERK_SECRET_KEY` goes into backend secrets only.
   - `CLERK_PUBLISHABLE_KEY` (backend) and `VITE_CLERK_PUBLISHABLE_KEY` (web build arg) are public and must
     come from the same instance.
   - Optionally set `CLERK_JWT_KEY` (Dashboard > API keys > JWKS public key, PEM) for networkless
     verification.
6. **CSP/network:** ANNOTA ships no Content-Security-Policy today.
   - The browser loads Clerk's script from the instance Frontend API domain (and Google during OAuth).
   - The API calls `api.clerk.com` to fetch JWKS and, on a user's first request, that user's identity.
   - If a CSP or egress allow-list is added later, include those hosts.
7. **Plan and cost review (pending owner):**
   - Check the current Clerk plan's monthly-active-user allowance.
   - Check email-delivery limits for verification codes.
   - Check whether custom Google credentials or the production domain need a paid tier.
   - Record the expected beta cost.

   None of this was verified in the repository work.

## Cutover and user impact

- The legacy `/v1/auth/google/*` endpoints, HS256 tokens, the `x-token` header and the `auth_token`
  `localStorage` entry are removed. After deploy, every existing browser session ends; old tokens get a JSON
  401 and users sign in once through Clerk. Removed configuration: `GOOGLE_OAUTH_CLIENT_ID`,
  `GOOGLE_OAUTH_CLIENT_SECRET`, `JWT_SECRET`, `TOKEN_EXPIRY_MINUTES`.
- **Returning legacy (Google) users** should choose **Continue with Google** with the same Google account. The
  API links their existing profile only when Clerk reports a *verified* Google connection whose Google account
  ID equals the legacy `users.provider_id`. Documents, annotations, preferences and progress keep the same
  numeric `users.id`.
- **Email-first legacy users:** suppose a first-time Clerk user without that Google link has a verified
  primary email matching an unlinked legacy Google profile. The API then refuses to create a second library
  (409 `legacy_google_link_required`), and the app asks them to continue with Google. Clerk links Google to the
  same Clerk user by verified email, and the next request links the legacy profile. Content is never transferred
  on email alone.
- **Pending live check:** confirm on the development instance that Clerk's Google `provider_user_id` equals the
  legacy Google userinfo `id` stored in `users.provider_id`. Until this is confirmed, legacy linking is
  unverified.
- Suggested notice before deploy: "ANNOTA sign-in is moving to a new provider. You'll be asked to sign in again
  once. If you used Google before, choose Continue with Google to keep your library."

### Read-only legacy inventory (run before cutover)

```sql
-- Accounts by provider and how many are already linked to Clerk.
SELECT provider, COUNT(*) AS users, COUNT(clerk_user_id) AS linked FROM users GROUP BY provider;
-- Legacy emails shared by more than one account (need manual review).
SELECT lower(email) AS email, COUNT(*) FROM users GROUP BY lower(email) HAVING COUNT(*) > 1;
-- Legacy users with content, so the notice reaches the people with data at stake.
SELECT u.id, u.email,
       (SELECT COUNT(*) FROM documents d WHERE d.user_id = u.id) AS documents,
       (SELECT COUNT(*) FROM annotations a WHERE a.user_id = u.id) AS annotations
FROM users u WHERE u.provider = 'google' AND u.clerk_user_id IS NULL ORDER BY annotations DESC;
```

This inventory has **not** been run against any production database as part of #39.

## Deletion and reconciliation

`DELETE /v1/users/me` runs these steps in order:

1. Delete uploaded files. If this fails, the request returns 500 and nothing else changes.
2. In one transaction, delete the user row (cascading scans, documents and annotations) and insert the Clerk
   user ID into `deleted_clerk_users`.
3. Delete the Clerk user through the Backend API. A Clerk 404 counts as success.

The tombstone in `deleted_clerk_users` is written once and never removed by the application. A Clerk user ID
listed there is never provisioned again, so a still-valid token cannot recreate the profile.

If step 3 fails, the API returns 502 `Account data deleted; sign-in removal failed, please retry`. The deleted
account's session may still call **only** `DELETE /v1/users/me`, which repeats step 3 and returns 204; every
other route returns 401 `Unauthorized: account deleted`. The web app retries automatically on any such 401
before signing out.

Operator reconciliation:

- **Clerk user exists but the local profile is gone:** check that the ID is in `deleted_clerk_users`, then
  delete the user in the Clerk Dashboard.
- **Clerk user deleted in the Dashboard but local data remains:** delete the local row with the same SQL as
  step 2:
  ```sql
  BEGIN;
  INSERT INTO deleted_clerk_users (clerk_user_id) VALUES ($1) ON CONFLICT DO NOTHING;
  DELETE FROM users WHERE clerk_user_id = $1;
  COMMIT;
  ```
  Then remove that user's files under `UPLOAD_DIR`.
- **Legacy user who lost access to their Google account:**
  1. Verify their identity out of band.
  2. Have them sign up with email.
  3. Link the legacy row (`$1`, legacy user ID) to the new Clerk user ID (`$2`). A Clerk user ID can map to only
     one row. If the new account already has a local row (`provider='clerk'`), agree with the user what happens
     to any content in it before deleting that row in the same transaction:
     ```sql
     BEGIN;
     DELETE FROM users WHERE clerk_user_id = $2 AND provider = 'clerk';
     UPDATE users SET clerk_user_id = $2 WHERE id = $1 AND clerk_user_id IS NULL;
     COMMIT;
     ```
  4. Record the request.

No Clerk webhooks are used. End-to-end deletion and isolation verification across accounts is tracked in #41.

## Verification status

- **Automated:**
  - Go token verification: issuer, `azp`, `exp`/`nbf`, wrong key, HS256 legacy token, key cache.
  - Provisioning and resolver policy.
  - Middleware status mapping.
  - Deletion with failed and retried Clerk cleanup.
  - PostgreSQL migration 005 plus concurrent provisioning, legacy linking and tombstones
    (`TestClerkProvisioning_Postgres`, `POSTGRES_TEST_DSN`).
  - Web token header, 401 handling, cache clearing on sign-out and account switch, deep-link return,
    legacy-link screen.
- **Pending owner credentials:**
  - real Google and email sign-in on desktop and mobile;
  - live provisioning and legacy linking against a Clerk instance;
  - Clerk-side deletion;
  - the Clerk plan and email-limit review.
