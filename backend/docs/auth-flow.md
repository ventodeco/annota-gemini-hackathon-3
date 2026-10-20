# Authentication Flow

The bespoke Google OAuth endpoints (`/v1/auth/google/state`, `/v1/auth/google/callback`) and
locally minted JWTs were removed in #39. ANNOTA now uses Clerk sessions:

1. The web app signs users in with Clerk's prebuilt `<SignIn>`/`<SignUp>` (Google and email).
2. Every API call sends `Authorization: Bearer <Clerk session token>` (from Clerk `getToken()`).
3. `internal/middleware/auth.go` verifies the token (`internal/auth/clerk.go`) and maps the Clerk
   user ID to the local numeric `users.id` (`internal/auth/identity.go`, migration `005_clerk_identity.sql`).

Setup, identity mapping, legacy-user linking, deletion and reconciliation: [`docs/clerk-auth.md`](../../docs/clerk-auth.md).
