# Issue #42: Persistent per-feature usage quotas and a visible beta allowance

## Outcome

Paid AI work (image OCR, explanations and speech) is admitted against persisted daily allowances: one per user per feature, plus a global beta budget per feature. Readers see what is left and when it resets before an operation fails. A rejected request returns an actionable quota error that is distinct from provider 429s and burst throttling, and it never signs the user out.

**User stories.**
- As a reader, I see my remaining allowance and reset time, so I understand the limits before an operation fails.
- As the operator, I have per-user and global per-feature caps that survive restarts and replicas, so beta traffic stays within an explicit budget.

## Investigation (latest `origin/dev` 4750008)

- The only paid AI calls are:
  - `POST /v1/scans`: image OCR, run asynchronously in `processOCR`.
  - `POST /v1/ai/analyze`: explanation.
  - `POST /v1/ai/speech`: speech.
- These calls do not use AI: PDF pages (`pdfExtractor.ExtractText`), page-scan creation (which is already idempotent per document page), viewing stored OCR, and saving annotations. They are not metered. `AnalyzeWithLanguageAPI` is not routed, so it is left untouched.
- `middleware.RateLimiter` is an in-memory, per-process sliding window. It is burst throttling, not a budget. It stays in place.
- `GET /v1/entitlements/me` returns only the configured limits, and nothing in `web/` calls it.
- Routed AI already reports `ai.CallMetadata` per attempt through `RoutedClientOptions.Observe`: gateway, region, model, tokens, characters, cost, currency and latency. Legacy clients report nothing.
- The frontend wipes the session only on 401 (`throwIfNotOk`), so a 429 does not sign the user out. Speech replay re-synthesizes every time; there is no audio cache.
- Clerk (#39) maps users to a stable numeric `users.id`. Account deletion runs `DELETE FROM users`. Migrations end at `005`; `006` is reserved for #42.
- #41 is changing `scan.go`, `document.go`, `user.go` and `main.go`. Admission therefore sits **before** the scan row insert, and the reservation does not depend on the scan row, because #41's discard paths delete it.

## Policy (explicit beta defaults; all configurable)

| Feature | Unit | Per-user/day | Global/day | Env |
|---|---|---|---|---|
| OCR | image page | 20 | 500 | `USAGE_QUOTA_OCR_PAGES_PER_DAY`, `USAGE_BUDGET_GLOBAL_OCR_PAGES_PER_DAY` |
| Explanation | request | 100 | 2500 | `USAGE_QUOTA_EXPLANATIONS_PER_DAY`, `USAGE_BUDGET_GLOBAL_EXPLANATIONS_PER_DAY` |
| Speech | character of selected text (Unicode code points) | 3000 | 75000 | `USAGE_QUOTA_SPEECH_CHARS_PER_DAY`, `USAGE_BUDGET_GLOBAL_SPEECH_CHARS_PER_DAY` |

- These are routine bounded beta defaults, chosen so the global budget is roughly 25 users at full allowance. The owner can change them by environment variable without a deploy-time code change. There is no unlimited tier. `0` disables a feature, and negative values fail config validation.
- **Reset:** daily at 00:00 UTC. `usage_day` is the UTC date at admission. `resetAt` is the next UTC midnight.
- **Charge timing:** units are reserved atomically before the provider call, and both the user and global counters include in-flight reservations.
- **Settlement:**
  - Success → `settled`; the units stay charged.
  - Failure where no billable provider work happened → `released`; both the user and global counters are refunded. This covers storage/DB failures before dispatch and provider `config`, `auth`, `rate_limit` and `quota` rejections.
  - Failure after dispatch where cost may have been incurred → `failed`. The user is refunded, because they received nothing, but the global budget keeps the charge, because the provider may still bill (the late-billed timeout case). This covers `upstream`, timeouts, unparseable responses and a failed OCR save.
- **Outcome classification** comes only from structured data, never from provider message strings, per the #41 sanitization contract:
  - the attempts in the context collector (`CallMetadata.StatusCode`/`ErrorKind`);
  - the final `*ai.ProviderError.Kind`;
  - context errors.
  - Any dispatched attempt that is not a clean rejection (`config`/`auth`/`rate_limit`/`quota`) means `failed`. Errors that are not `ProviderError` (deadline, cancel, network) also mean `failed`. The error kind alone is used only when no attempts were observed (legacy clients). Raw vendor bodies are never persisted or logged.
- **Admission order:** reserve only after method, auth, body and ownership validation. For analyze, that is after `resolveAnalyzeContext`; for scans, after the mime/size checks and before `CreateScan`.
- **Settlement** runs on a detached, bounded context (`WithoutCancel` + 5s), so client disconnects or the OCR deadline cannot strand a reservation. OCR settles via `defer` exactly once.
- Refunds always apply to the reservation's own `usage_day`, so a refund after midnight never credits the new day. Settlement is a single `reserved → *` transition, so a second settle is a no-op (no double refund).
- **Stale reservations** (the process dies mid-call) remain `reserved` and charged. This is conservative: no automatic sweeper in this issue, and it is documented.
- **Cost record per reservation:**
  - Recorded fields: gateway, region, model, attempts, input/output/total tokens, characters, provider cost and currency, `cost_status`, latency in ms.
  - `cost_status` values:
    - `reported`: every attempt reported a cost, all in the same currency.
    - `missing`: a call was made, but cost metadata is absent or mixed. This is always the case for the legacy providers.
    - `none`: no provider call.
  - No price table and **no estimates** are produced, so a provider price change cannot make stored estimates wrong. Provider key caps remain a supplementary, possibly-late control.
- **Account deletion:** the user's counters and reservations cascade with `users`. Global counters are not decremented, because cost was incurred.
- **Errors:**
  - Quota rejection: `429` with `code` set to `usage_quota_exceeded` (user scope: `feature`, `limit`, `used`, `remaining`, `resetAt`) or `usage_budget_exhausted` (global scope: `feature` and `resetAt` only; global totals are never exposed to users). Both send a `Retry-After` header. `remaining` is clamped at 0 when an operator lowers a limit mid-day.
  - Provider 429: `code: ai_provider_limited`.
  - Burst limiter: `code: rate_limited`, with the message changed from the misleading "Usage limit reached".
  - Accounting store unavailable: fail closed with `503 usage_unavailable`.
- **Caching:**
  - Stored OCR and PDF text are already reused without AI.
  - Speech replay inside a signed-in session reuses audio through a React Query cache keyed by `(text, context)`. That cache is cleared on sign-out or account switch under the #39 contract.
  - A server-side audio/result cache keyed by model is out of scope; it is noted as follow-up.

## Storage (migration `006_usage_quotas.sql`, transactional, additive)

- `usage_counters(user_id FK users ON DELETE CASCADE, feature, usage_day DATE, used_units BIGINT ≥ 0, PK(user_id, feature, usage_day))`
- `usage_global_counters(feature, usage_day, used_units ≥ 0, PK(feature, usage_day))`
- `usage_reservations(id, user_id FK CASCADE, feature, usage_day, units > 0, status CHECK reserved|settled|failed|released, created_at, settled_at, gateway, region, model, attempts, input_tokens, output_tokens, total_tokens, characters, provider_cost NUMERIC, currency, cost_status CHECK, latency_ms)` plus an index on `(status, created_at)` for operator reconciliation.
- **Reserve** runs in one transaction: upsert the zero row, then a conditional `UPDATE … SET used = used + n WHERE used + n <= limit RETURNING`, first on the user counter and then on the global counter, then insert the reservation. Rejection rolls back and reports the current usage. Row locks with READ COMMITTED re-evaluation make concurrent admissions exact. All transactions lock in the order user → global, so there is no deadlock cycle.
- **Settle** runs in one transaction: `UPDATE reservation … WHERE id AND status='reserved' RETURNING`, then the optional refunds, using `GREATEST(used - n, 0)`.

## Files

- **Backend:**
  - `migrations/006_usage_quotas.sql`
  - `internal/storage/usage.go` (+ `usage_postgres_test.go`)
  - `internal/usage/usage.go` (Meter, features, error, outcome classification, context call collector; + tests)
  - `internal/config/config.go` (+ test): the new limits
  - `internal/handlers/scan.go`, `ai.go`, `entitlements.go` (+ tests)
  - `internal/httputil` (optional `code`)
  - `internal/middleware/rate_limit.go`
  - `cmd/server/main.go`: wiring, plus `usage.RecordCall` in the routed observer
  - `internal/testutil`: in-memory usage store for handler tests
  - `docs/openapi.yaml`, `.env.example`, `backend/AGENTS.md`
- **Frontend:**
  - `lib/types.ts`, `lib/api.ts`: `getEntitlements`, `ApiError.code`/details
  - `lib/usage.ts`: parsing, labels, quota messages
  - `hooks/useEntitlements.ts`
  - `components/usage/AllowancePanel.tsx`
  - `ProfilePage`: full panel
  - `WelcomePage`: compact OCR line and quota message
  - `CameraPage`: quota message
  - `useAnnotationDrawerFlow` and `AnnotationDetailPage`: quota messages
  - `useAnnotations` and `useScans`: entitlement invalidation and the speech replay cache
  - tests for the above
- **Docs:** `docs/usage-quotas.md`.

## Edge cases → handling

- **Simultaneous tabs or replicas:** row-locked conditional increments. A PG test runs N concurrent reservations against limit L, and exactly L succeed.
- **Fresh day with limit 0, or n > limit:** must be rejected. The guard is the conditional UPDATE after a zero-row upsert, never `ON CONFLICT DO UPDATE` alone. PG test.
- **Per-user isolation and feature independence:** A's usage does not affect B, but the global counter is shared; speech never touches the explanation counter. PG test.
- **Restart:** counters live in PostgreSQL. A test reopens a fresh store and pool on the same schema and sees the same usage.
- **Midnight:** the day is fixed at reservation, and refunds go to that day. A test uses an injected clock.
- **Retries and duplicates:** each upload or explanation is new work and is charged. A duplicate settle is a no-op (tested). PDF page re-open is free.
- **Timed-out and later billed:** the global charge is kept and `cost_status=missing`.
- **Missing cost metadata:** `missing`, never estimated.
- **Zero allowance:** rejected with the quota error; the UI reads "Not available during beta".
- **Global exhausted first:** `usage_budget_exhausted` with the global `resetAt`; the UI shows a beta-capacity notice.
- **Large speech input:** a selection longer than `USAGE_SPEECH_MAX_CHARS_PER_REQUEST` (default 500), or than the per-user daily limit, returns a non-retryable `413 speech_text_too_long`, not a quota 429. Otherwise the requested characters must fit within the remaining allowance.
- **Budget rejection:** no logout (it is not a 401).

## Acceptance criteria

1. Limits are explicit, configurable, validated and persisted. Each feature has an independent user counter and global counter.
2. Concurrent admissions cannot exceed a limit. Failure settlement follows the policy above, and a double settle cannot double-refund.
3. Entitlements returns per-feature `limit/used/remaining/resetAt` plus global availability, and the UI shows exactly those values.
4. Quota errors are consistent, distinct from provider and burst 429s, and keep the user signed in.
5. A cost/usage record is stored per reservation, with an honest `cost_status`.
6. Desktop and mobile screenshots show the allowance, limit-reached and recovery states.
7. Live cost reconciliation remains pending provider keys.

## Verification (proportional)

- `go test ./...`, `go vet ./...`
- PG tests in `internal/storage`, named `*_Postgres` so CI runs them without SKIP: concurrency, fresh-day limit 0 and n > limit, global-first, isolation and feature independence, refund/day, double-settle, cascade on user delete, restart persistence.
- Handler tests (a testutil in-memory meter; a nil meter is never "unmetered"): quota 429 body, provider 429 code, pre-dispatch failure `released`, provider failure `failed`, speech too long 413.
- Web: `bun run test`, `bun run lint`, `bun run build`.
- Playwright screenshots (1280×800, 390×844) from a temporary uncommitted Vite harness that renders the real `AllowancePanel` and the quota message states with fixture data. States: allowance, empty (zero/disabled), exhausted (user and global), entitlements-fetch error, and recovery after reset. Live Clerk login is not claimed.
- The defaults are recorded on #42 as owner-adjustable. Owner confirmation of the exact numbers stays pending and is not recorded as agreed.
- Shared 30-minute test-debug budget for #42.

## Out of scope

Billing or paid plans, model selection (#35), a service layer (#14), auth changes, privacy/no-store/file cleanup (#41), a server-side audio cache, an automatic stale-reservation sweeper, and live provider cost reconciliation (pending keys).
