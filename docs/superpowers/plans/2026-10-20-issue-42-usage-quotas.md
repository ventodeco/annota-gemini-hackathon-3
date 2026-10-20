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

## Policy (owner-approved operation caps; all configurable)

| Feature | Unit | Per-user/day | Global/day | Env |
|---|---|---|---|---|
| OCR | image page | 10 | 250 | `USAGE_QUOTA_OCR_PAGES_PER_DAY`, `USAGE_BUDGET_GLOBAL_OCR_PAGES_PER_DAY` |
| Explanation | request | 50 | 1250 | `USAGE_QUOTA_EXPLANATIONS_PER_DAY`, `USAGE_BUDGET_GLOBAL_EXPLANATIONS_PER_DAY` |
| Speech | character of selected text (Unicode code points) | 1500 | 37500 | `USAGE_QUOTA_SPEECH_CHARS_PER_DAY`, `USAGE_BUDGET_GLOBAL_SPEECH_CHARS_PER_DAY` |

- **Owner-approved caps (2026-10-20):** the owner approved half of each originally proposed value (originally 20/100/3000 per user, 500/2500/75000 global, 500 characters per speech request). Speech is also capped at 250 characters per request (`USAGE_SPEECH_MAX_CHARS_PER_REQUEST`). These are operation and character caps, not a currency spending cap. The global budget is still roughly 25 users at full allowance. The owner can change them by environment variable without a deploy-time code change. There is no unlimited tier. `0` disables a feature, and negative values fail config validation.
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
- **Large speech input:** a selection longer than `USAGE_SPEECH_MAX_CHARS_PER_REQUEST` (default 250), or than the per-user daily limit, returns a non-retryable `413 speech_text_too_long`, not a quota 429. Otherwise the requested characters must fit within the remaining allowance.
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
- The owner approved the caps above on 2026-10-20 (half of each original proposal). They are recorded on #42.
- Shared 30-minute test-debug budget for #42.

## Out of scope

Billing or paid plans, model selection (#35), a service layer (#14), auth changes, privacy/no-store/file cleanup (#41), a server-side audio cache, an automatic stale-reservation sweeper, and live provider cost reconciliation (pending keys).

## Delivery evidence (branch `backlog/issue-42`, rebased on dev 3aa1675)

- **Backend:**
  - `go vet ./...` is clean.
  - `go test -count=1 ./...` with the local `POSTGRES_TEST_DSN`: 470 passed in 15 packages.
  - `go test -v ./internal/storage -run 'Postgres$'`: 18 PASS, 0 SKIP, 0 FAIL. This includes `TestUsageReserveConcurrency_Postgres` (25 concurrent, limit 7: exactly 7 admitted), `TestUsageReserveGlobalBudgetConcurrency_Postgres`, `TestUsageReserveLimits_Postgres`, `TestUsageSettle_Postgres` and `TestUsagePersistenceAndAccountDeletion_Postgres` (restart persistence, cascade, and a late settle after deletion that is a no-op).
  - `go test -race ./internal/usage ./internal/handlers` is clean.
- **Web:** `bun run test` (all pass), `bun run lint` (clean) and `bun run build` (ok).
- **Screenshots** in `docs/screenshots/issue-42/`: allowance, exhausted, empty, error and recovery, each at desktop 1280×800 and mobile 390×844. They were captured with Playwright Chromium from a temporary, uncommitted Vite harness that renders the real `AllowanceView`, `ScanAllowanceLine` and `getUsageErrorMessage` with fixture data. This is not live Clerk sign-in.
- **Request-scoped call metadata** is shared with #35 provenance through `usage.Calls(ctx)`. There is no second observer.
- **Pending:**
  - Owner confirmation of the default numbers.
  - Live provider cost reconciliation, which needs keys.
  - LegalPage privacy wording for usage counts. It is coordinated with #41/PR #67 and goes in whichever PR merges second.

## Review round 1 (Sol, REQUEST_CHANGES on c9de071), resolved

1. **Legacy clients.** They retry internally and report no attempts, so a final clean rejection could hide an earlier billed attempt. Without observed attempts, only a configuration error raised before any HTTP status is now `released`; everything else is `failed`. Tests added: transport failure then 429, legacy 429, and legacy config error before and after a response.
2. **Hidden tab across the reset.** It could keep an exhausted allowance because the app disables focus refetching globally. Entitlements now opt in to `refetchOnWindowFocus`, and a regression test covers it (it fails without the fix).
3. **The 413 `max` exposed the global cap.** It is now derived from the user and per-request limits only. A request larger than the global budget gets the existing `usage_budget_exhausted` error instead. Test added.

## Review round 2 (Sol, REQUEST_CHANGES on ca07cae, one minor finding), resolved

- **A page refunded by failed background OCR could stay hidden until focus or midnight.** An entitlements snapshot taken while OCR still held the page stayed in use after the refund. `useEntitlements` now refreshes at least every 60 s while visible (5 s floor, and still just after the reset). A regression test covers reservation in flight → OCR fails → allowance restored, and fails without the fix.

## Follow-up: owner-approved caps (branch `backlog/issue-42-approved-caps`)

- **Scope:** apply the owner-approved caps as code defaults and update every doc, example and test fixture to match.
  - Per user per day: 10 OCR pages, 50 explanations, 1500 speech characters.
  - Global per day: 250 OCR pages, 1250 explanations, 37500 speech characters.
  - Speech per request: 250 characters.
  - Reset stays at 00:00 UTC.
  - Environment overrides and zero-disables semantics are unchanged.
  - No production configuration change and no deploy.
- **Files:**
  - `backend/internal/config/config.go` (+ test defaults)
  - `backend/.env.example`, `backend/AGENTS.md`
  - `docs/usage-quotas.md`, `backend/docs/openapi.yaml` (example)
  - web test fixtures and examples (`usage.test.ts`, `AllowancePanel.test.tsx`, `useEntitlements.test.ts`)
  - regenerated `docs/screenshots/issue-42/*`
  - this plan
- **Out of scope:** the source of truth is registry `scopeDecisions.quotaLimits`. Handler tests that set explicit custom limits keep them, because they test semantics, not defaults.
- **Verification:**
  - Backend: config tests, `go test ./...` and `go vet`.
  - Web: `bun run test`, `lint` and `build`.
  - Screenshots at desktop and mobile sizes with the approved values (temporary uncommitted harness).
  - Final-head CI.

## Follow-up: abandoned OCR reservations (branch `backlog/issue-42-stale-ocr`, no schema change)

**Problem (confirmed in source on dev b368099).** `processOCR` settles through an in-process `defer`. If the process restarts mid-OCR, that defer never runs: the reservation stays `reserved`, and both the user and global counters stay charged until 00:00 UTC. `usage_reservations` has no link to the scan.

**Outcome.** An OCR reservation still `reserved` after the cutoff (OCR processing timeout + a safety margin) is settled as `failed`. That refunds the user once and keeps the global charge, because the provider may have billed. No OCR is re-dispatched.

**Cutoff.** `handlers.OCRReservationStaleAfter = ocrProcessingTimeout (5m) + 10m margin = 15m`, measured from the reservation's `created_at`. It cannot refund OCR that is still running normally:
- the OCR context times out at 5m;
- `UpdateScanOCR` and the 5s detached settle run right after it;
- between reservation and dispatch there is only `CreateScan`, `SaveImage` and the shared account lock.

So a live process settles well within 15m, and anything older is abandoned. The 10m margin also absorbs lock waits and replica clock skew. #40's `ocr_interrupted` uses the same rule (no re-dispatch); a cutoff is agreed with A40.

**Mechanism.** Reuse the conditional settlement semantics, scoped per user and evaluated lazily:
- `storage.UsageStore.ExpireStaleUsage(ctx, userID, feature, cutoff, settledAt)` runs one statement:
  - lock the stale `reserved` rows `ORDER BY id FOR UPDATE`, re-checked after any wait;
  - mark them `failed`, with `cost_status=missing`;
  - refund `usage_counters` on each reservation's own `usage_day`, using `GREATEST(…, 0)`.
- Global counters are untouched.
- `usage.Meter` runs it for features configured with a stale timeout (OCR only), before `Reserve` and `Snapshot`, for that user only. An error is logged and never blocks admission.
- A late `Finish` uses `WHERE status='reserved'`, so it becomes a no-op: no double refund or charge.
- A deleted user has no reservations left (cascade) and `UPDATE` never inserts, so no rows are recreated.
- Lock order is reservation then counter, the same as `SettleUsage`. `Reserve` locks no existing reservations, so there is no deadlock cycle.

**Files.**
- `backend/internal/storage/usage.go` (+ `usage_postgres_test.go`)
- `backend/internal/usage/usage.go` (+ `usage_test.go`)
- `backend/internal/testutil/usage.go`
- `backend/internal/handlers/scan.go` (constant)
- `backend/cmd/server/main.go` (wiring)
- `docs/usage-quotas.md`
- this plan

**Out of scope.**
- Synchronous explanation and speech reservations stranded by a crash mid-request. They are noted as possible future work using the same mechanism.
- Schema or scan FK changes, sweeper goroutines, re-dispatch.

**Focused regression questions.**
1. PostgreSQL: is a stale OCR reservation refunded once, on its original day, with the global charge kept and recent reservations untouched? Is a late settle a no-op? Is nothing recreated after user deletion?
2. Meter: does a user at the limit with only a stale reservation get admitted again, and does `Snapshot` show the refund?

Required gates run once in CI.
