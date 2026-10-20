# Issue #7: Basic API documentation

## Goal

Describe the API that ships on `dev` accurately, in one place, with runnable examples that are safe to share. The issue text predates #39: its Google OAuth routes (`/v1/auth/google/state`, `/v1/auth/google/callback`) no longer exist. Authentication is now a Clerk session JWT (`Authorization: Bearer`), mapped to a stable numeric local user ID.

User story: as a frontend or integration developer, I can read one spec to learn every route, its auth requirement, request and response shapes, and error codes, then call it with curl or Bruno/Postman without reading Go handlers.

## Inventory (dev at bb41f6c)

Routes are registered in `backend/cmd/server/main.go`. Everything under `/v1/` goes through `NoStore` and the Clerk `AuthMiddleware`; `/healthz` and `/readyz` are public.

| Route | Methods | In current spec? |
|-------|---------|------------------|
| `/healthz`, `/readyz` | GET | yes |
| `/v1/users/me` | GET, PATCH, DELETE | DELETE missing |
| `/v1/users/me/languages` | GET | yes |
| `/v1/entitlements/me` | GET | yes |
| `/v1/scans` | POST, GET | stale schemas |
| `/v1/scans/{id}` | GET, DELETE | stale schemas |
| `/v1/scans/{id}/image` | GET | missing |
| `/v1/ai/analyze`, `/v1/ai/speech` | POST | stale schemas, `text/plain` errors |
| `/v1/annotations` | POST, GET | missing filters and source fields |
| `/v1/annotations/{id}` | GET, DELETE | missing source fields |
| `/v1/documents` | POST, GET | missing |
| `/v1/documents/{id}` | GET, DELETE | missing |
| `/v1/documents/{id}/file` | GET | missing |
| `/v1/documents/{id}/pages/{n}` | GET | missing |
| `/v1/documents/{id}/pages/{n}/scan` | POST | missing |
| `/v1/documents/{id}/progress` | PATCH | missing |
| `/v1/events` | POST | missing |

Existing artifacts:

- `backend/docs/openapi.yaml`: OpenAPI 3.0.3, maintained by #39/#42/#43. This is the canonical spec and gets reconciled. No second spec is created.
- `bruno-collection/`: a Postman collection with 10 of 27 operations, a `.bru` file that is not valid Bruno syntax, and a README that gives two token lifetimes. These are replaced by importing the OpenAPI spec, which both Bruno and Postman support.
- The `README.md` "API Endpoints" list names `/api/scans/{id}/annotate`, which does not exist.
- `docs/rfc.md` and `backend/docs/IMPLEMENTATION_PLAN.md` are historical design documents and stay unchanged.

## Scope

Files:

1. `backend/docs/openapi.yaml`: reconcile with the handlers and structs.
   - Global `security: [BearerAuth]`, with `security: []` on the health routes. Shared auth error responses: 401 missing, invalid, expired or deleted session; 403 verified email required; 409 with message `legacy_google_link_required`, which is not a `code` field; 503 authentication service unavailable; 500 identity resolution failure.
   - All `/v1` errors are `application/json` `ErrorResponse`. Every `/v1` response sends `Cache-Control: no-store`.
   - Add the missing operations listed above. Fix the scan, analyze, speech, annotation and pagination schemas to match their structs, including `omitempty`: absent fields are omitted, not `null`.
   - Rate limiting: 429 `rate_limited` with `Retry-After`, on POST to scans, analyze and speech. Usage quotas: 429 `usage_quota_exceeded`/`usage_budget_exhausted`, 413 `usage_request_too_large`, 503 `usage_unavailable`. Upstream errors: 429 `ai_provider_limited`, 502.
   - Provider-neutral title and description, since AI providers are configurable.
2. `backend/docs/api.md` (new): a short guide covering the base URL, how to get a Clerk token, the error model, quotas, and runnable curl examples using placeholders only (`$ANNOTA_TOKEN`, `./page.jpg`, `./sample.pdf`). It links to the spec instead of repeating the route list.
3. `README.md`: replace the stale endpoint list with links to the spec and the guide.
4. `bruno-collection/`: delete the stale Postman and `.bru` files. Rewrite the README as "import `backend/docs/openapi.yaml`", keeping the Clerk token steps with the correct lifetime.

Out of scope: runtime API changes. These code issues are reported, not fixed:

- `summarizeNuance` truncates at 100 bytes, which can split a multi-byte character.
- `AnalyzeWithLanguageAPI` is unrouted.
- The Vite `/api` proxy is stale; the web app calls `VITE_API_BASE_URL` directly.

## Overlap with #40 (unmerged)

#40 will add `failureReason` values (`ocr_timeout`, `no_text_found`, `ocr_interrupted`), return 422 for malformed or zero-page PDFs, and make page scans race-free. The spec documents only what is on `dev`:

- `failureReason` is an open string, with `ocr_failed` as today's value; clients must handle unknown values.
- A PDF that cannot be parsed returns 500.

If #40 merges first, rebase and update the spec to the landed behavior. #40 does not touch the docs files.

## Edge cases to document

- A deleted (tombstoned) subject can reach only `DELETE /v1/users/me`. That route retries the Clerk deletion and returns 502 when sign-in removal fails.
- `POST /v1/scans` can return 201 with `status: failed` when OCR dispatch is skipped.
- Pagination coerces invalid `page`/`size` to defaults and caps `size` at 100. `nextPage` is present only when the page is full.
- `GET /v1/annotations`: `scanId` takes precedence over `documentId`, and `pageNumber` applies only together with `documentId`.
- Annotation DELETE returns 404 for missing or foreign IDs. GET returns 403 for foreign IDs.
- `POST /v1/documents/{id}/pages/{n}/scan` returns 200 with an existing scan and 201 when it creates one.
- Analyze with `scanId` uses the scan's OCR text (up to a 12,000-rune window) as context.
- Speech charges and limits by runes of `highlightedText`. `contextText` and `tone` are accepted but not spoken or used by the current providers. Audio is `audio/wav` or `audio/mpeg`.
- Events reject unknown fields and bodies over 2 KB.

## Acceptance criteria

- [ ] Every route and method registered in `main.go` appears in the spec (diffed against the router).
- [ ] Request and response schemas match the Go structs.
- [ ] Clerk Bearer authentication and its error responses are documented. No Google OAuth or `x-token` references remain in active API docs.
- [ ] Example requests are runnable and contain no credentials, tokens, private documents or personal data.
- [ ] The spec passes `redocly lint` with example validation.

## Verification

Docs-only change, so the application test suites are not rerun.

- `bunx @redocly/cli lint backend/docs/openapi.yaml`: structure and example validation.
- One uncommitted script diffing spec paths and methods against the routes and dispatchers.
- A grep of the changed docs for token-like strings and obsolete auth routes.
- CI handles the required gates.
