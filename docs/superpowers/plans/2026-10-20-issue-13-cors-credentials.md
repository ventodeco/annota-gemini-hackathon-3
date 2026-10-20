# CORS Credentials and Wildcard Origin Hardening Plan

> **For agentic workers:** Use superpowers:executing-plans to implement this plan after independent approval. Steps use checkbox syntax for tracking.

**Goal:** Close [issue #13](https://github.com/ventodeco/annota-gemini-hackathon-3/issues/13) by guaranteeing the CORS middleware never pairs `Access-Control-Allow-Credentials: true` with a wildcard or empty origin, and cover the remaining edge cases with focused tests.

**Architecture:** Change only `backend/internal/middleware/cors.go` and `backend/internal/middleware/cors_test.go`. Configuration parsing, production validation, session/auth behavior, and the frontend stay unchanged.

**Tech Stack:** Go 1.27.3, `net/http`, `httptest`.

**Status:** Plan written, awaiting independent review.

## Global Constraints

- Branch: `backlog/issue-13`, created from fetched `origin/dev` at `59500daf3443b95fa96f089c893ea162d33c3394`. PR targets `dev`.
- Scope is wildcard-origin and credential safety in the CORS middleware and its tests. No auth migration, cookie, session, or config-validation work.
- Prefix shell commands with `rtk`. Go and Nix are absent from the shell PATH; use the verified Go 1.27.3 toolchain at `/tmp/annota-runtime-upgrade/go/bin`. Use `rtk proxy` for `go` and `gh` because the filtered wrapper suppressed `gh issue view` output.
- Backend-only change: no screenshots.
- Test debugging is capped at 30 minutes, shared with review. Record any blocker honestly in issue #13.

## Evidence and reconciliation

The issue's primary suggested fix is already on `dev` and `master`. Commit `0003108` ("fix: harden cors credentials and add annotation scan index (#13, #15)", 2026-04-24, pushed without a PR) changed the wildcard branch so `ALLOWED_ORIGINS=*` emits `Access-Control-Allow-Origin: *` without `Access-Control-Allow-Credentials`. `TestCORSMiddleware_WildcardAllowsAnyOrigin` covers that for `GET`. Separately, `Config.Validate` rejects `ALLOWED_ORIGINS=*` when `APP_ENV=production` (`config_test.go`). The issue stayed open.

The current middleware (`cors.go`) reflects the request origin with credentials whenever `cfg.IsOriginAllowed(origin)` is true and `AllowedOrigins != "*"`. `GetAllowedOriginsList` splits on commas and trims entries without discarding `*` or empty entries. Two residual paths therefore still produce the unsafe pair:

1. **Literal `*` entry in an explicit list.** With `ALLOWED_ORIGINS="https://app.example,*"` and request header `Origin: *`, the middleware returns `Access-Control-Allow-Origin: *` with `Access-Control-Allow-Credentials: true`, the exact combination the issue forbids. Browsers never send `Origin: *`, so this is not a browser exploit, but the server should not emit it.
2. **Empty entry from a trailing or doubled comma.** With `ALLOWED_ORIGINS="https://app.example,"`, a request without an `Origin` header matches the `""` entry and gets an empty `Access-Control-Allow-Origin` plus `Access-Control-Allow-Credentials: true`.

A third gap concerns caching. The credentialed reflected response varies by `Origin`, but the middleware does not send `Vary: Origin`. A shared cache could reuse a response carrying one origin's `Access-Control-Allow-Origin` and credentials grant for a request from another origin, or reuse a no-CORS response for an allowed origin.

**Baseline (2026-10-20, before changes):** from `backend/`, `rtk proxy go test -count=1 ./internal/middleware ./internal/config` passed (`ok` for both packages) and `rtk proxy go vet ./...` passed.

## Task 1: Require an explicit concrete origin for credentialed CORS

**User Story:** As an Annota reader signed in with a session cookie, I want the API to grant credentialed cross-origin access only to the exact frontend origins the operator configured, so that a misconfigured or unusual origin value can never turn into a wildcard-with-credentials grant.

**Files:**

- Modify: `backend/internal/middleware/cors.go`
- Modify: `backend/internal/middleware/cors_test.go`

**Behavior after change:**

| `ALLOWED_ORIGINS` | Request `Origin` | `Access-Control-Allow-Origin` | `Access-Control-Allow-Credentials` | `Vary` includes `Origin` |
| --- | --- | --- | --- | --- |
| `*` | any or none | `*` | absent | no |
| explicit list | exact listed origin | request origin | `true` | yes |
| explicit list | unlisted origin | absent | absent | yes |
| explicit list containing `*` | `*` | absent | absent | yes |
| explicit list with empty entry | none | absent | absent | yes |

Methods, headers, and max-age headers keep their current values and are set only when an allow-origin header is set. `OPTIONS` keeps returning `204` without calling the next handler, regardless of origin, as today.

**Implementation sketch:**

```go
origin := r.Header.Get("Origin")
if m.cfg.AllowedOrigins == "*" {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	setCORSPolicyHeaders(w)
} else {
	w.Header().Add("Vary", "Origin")
	if origin != "" && origin != "*" && m.cfg.IsOriginAllowed(origin) {
		w.Header().Set("Access-Control-Allow-Origin", origin)
		w.Header().Set("Access-Control-Allow-Credentials", "true")
		setCORSPolicyHeaders(w)
	}
}
```

`setCORSPolicyHeaders` is a small unexported helper for the three shared headers, removing duplication between the two branches. Use `Header().Add` for `Vary` so values set by other handlers are preserved. The wildcard branch's current output stays exactly the same.

**Scope decision for the reviewer:** `Vary: Origin` is a credential-safety companion rather than a literal issue requirement. If the reviewer judges it out of scope, drop it and its test assertions; the rest of the plan stands.

**Focused tests (table-driven where natural):**

- [ ] Wildcard `OPTIONS` preflight returns `204`, `Access-Control-Allow-Origin: *`, and no `Access-Control-Allow-Credentials`. Only `GET` is covered today.
- [ ] Explicit list containing `*` with `Origin: *` sets neither `Access-Control-Allow-Origin` nor `Access-Control-Allow-Credentials` (both `GET` and `OPTIONS`).
- [ ] Trailing-comma list with no `Origin` header sets neither header.
- [ ] Explicit mode adds `Vary: Origin` on reflected and on disallowed responses; wildcard mode does not.
- [ ] A `Vary` value already set before the middleware runs is preserved alongside `Origin`.
- [ ] Existing four CORS tests stay green unchanged.

**Verification (from `backend/`):**

```bash
rtk proxy go test -count=1 -v ./internal/middleware -run 'TestCORS'
rtk proxy go test -count=1 ./...
rtk proxy go vet ./...
rtk proxy gofmt -l internal/middleware
```

The full `./...` run includes the PostgreSQL migration test, which skips locally without `POSTGRES_TEST_DSN`; CI runs it against a real database. Report that skip honestly rather than as a local pass.

## Definition of done

- The middleware cannot emit `Access-Control-Allow-Credentials: true` unless it also emits a concrete, non-empty, non-`*` origin that exactly matches a configured entry.
- Wildcard mode output is unchanged and covered for both `GET` and `OPTIONS`.
- Focused tests above pass; `go test ./...`, `go vet ./...`, and `gofmt` are clean locally and in CI.
- PR against `dev` references #13 and `0003108`, passes independent GPT-6.1 Sol review, and merges after green checks.
- Issue #13 closes with a comment citing `0003108` and the merged PR.

## Out of scope (follow-up candidates, not changed here)

- `Config.Validate` checks only `AllowedOrigins == "*"`; `" * "` or mixed lists containing `*` pass production validation. After this change those configurations fail closed in the middleware, so validation tightening can be a separate config issue.
- `GetAllowedOriginsList` keeps returning empty and `*` entries; the middleware guard makes them harmless.
- Whether the frontend needs credentialed CORS at all belongs to auth/session work, not this issue.

## Overlaps

- #33 (beta umbrella) tracks overall readiness; this PR does not duplicate any child implementation.
- #39 (Clerk migration) will later revisit CORS headers for Clerk origins, and #41 covers HTTPS/origin configuration for beta privacy. Both build on this guard; neither is implemented here. No open PR touches `cors.go`.
