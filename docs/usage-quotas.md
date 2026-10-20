# Usage quotas and the beta allowance

ANNOTA meters its paid AI work against persisted daily allowances: one per user per feature, plus one global beta budget per feature. The backend enforces them (`backend/internal/usage`, `backend/internal/storage/usage.go`, migration `006_usage_quotas.sql`). The app shows the remaining allowance on the profile page, and the image-scan allowance on the home screen.

The limits are **operation caps** (pages, requests, characters), not a currency budget. Their values were **approved by the owner** on 2026-10-20. This is a beta cost control, not billing. There are no paid plans or prices, and the app makes no financial promises.

## What is metered

| Feature | Endpoint | Unit charged | Per user per day | Global per day |
|---|---|---|---|---|
| `ocr` | `POST /v1/scans` (image OCR) | 1 page per image | `USAGE_QUOTA_OCR_PAGES_PER_DAY` = 10 | `USAGE_BUDGET_GLOBAL_OCR_PAGES_PER_DAY` = 250 |
| `explanation` | `POST /v1/ai/analyze` | 1 request | `USAGE_QUOTA_EXPLANATIONS_PER_DAY` = 50 | `USAGE_BUDGET_GLOBAL_EXPLANATIONS_PER_DAY` = 1250 |
| `speech` | `POST /v1/ai/speech` | characters (Unicode code points) of the selected text | `USAGE_QUOTA_SPEECH_CHARS_PER_DAY` = 1500 | `USAGE_BUDGET_GLOBAL_SPEECH_CHARS_PER_DAY` = 37500 |

- The defaults are the owner-approved beta caps from issue #42: half of each value originally proposed. The global budget is roughly 25 users at full allowance. The operator can still override any of them through environment variables. `0` disables a feature, negative values fail startup validation, and there is no unlimited setting.
- A speech selection longer than `USAGE_SPEECH_MAX_CHARS_PER_REQUEST` (default 250), or longer than the per-user daily limit, is rejected as too large. A request larger than the global budget is reported as an exhausted budget, so the 413 never reveals the global value.
- These are not metered, because they make no AI call:
  - PDF pages, whose text is extracted locally;
  - re-opening stored scans or OCR text;
  - saving or reading annotations.
- Each feature has its own counters, so replaying speech never spends explanation allowance. Inside a signed-in session, replaying the same selection reuses the audio from the client query cache, which is cleared on sign-out and account switch.
- The in-memory `AI_RATE_LIMIT` burst throttle still applies separately.

## Reset and persistence

- **Days are UTC calendar days.** Allowances reset at 00:00 UTC. The app shows that moment in the reader's local time, and `resetAt` in the API is the next UTC midnight.
- **Counters live in PostgreSQL** (`usage_counters`, `usage_global_counters`). Restarts and additional replicas therefore share one allowance; they cannot reset it or multiply it.

## Admission, settlement and failures

1. **Reserve before the provider call.**
   - In one transaction, a zero row is upserted, then a conditional `UPDATE … WHERE used + units <= limit` runs: first on the user counter, then on the global counter. A `usage_reservations` row is then inserted.
   - Row locks make concurrent tabs, retries and replicas exact.
   - Lock order is always user, then global, so there is no deadlock cycle.
   - A rejection rolls the whole transaction back.
   - Validation and ownership checks run first, so a 400, 403 or 404 never touches counters.
2. **Settle exactly once.** Settlement runs on a detached, bounded context, so a client disconnect or an expired OCR deadline cannot strand a reservation.

   | Outcome | Status | User counter | Global counter |
   |---|---|---|---|
   | Success | `settled` | charged | charged |
   | No billable work: a storage failure before dispatch, or every observed provider attempt was a clean `config` / `auth` / `rate_limit` / `quota` rejection | `released` | refunded | refunded |
   | Work was dispatched and may be billed: upstream errors, timeouts, transport failures, unusable responses, OCR result not saved | `failed` | refunded | **kept** |

   - The outcome comes only from structured `ai.CallMetadata` attempts and `ai.ProviderError.Kind`, never from provider message text. Raw provider bodies are never stored.
   - The legacy `minimax`/`gemini` clients retry internally and report no attempts, so an earlier attempt may have been billed even when the final error is a clean rejection. For them, only a configuration error raised before any HTTP status counts as `released`; every other failure is `failed`.
   - Refunds apply to the reservation's own day, so a refund after midnight never credits the new day.
   - A second settlement is a no-op.
3. **Abandoned OCR reservations are refunded to the user.**
   - If the process dies mid-OCR (for example, a restart), its in-process settlement never runs.
   - An OCR reservation still `reserved` 15 minutes after admission (`handlers.OCRReservationStaleAfter`: the 5-minute OCR timeout plus a 10-minute margin) is settled as `failed`. The user's counter is refunded once, on the reservation's own day. The global charge is kept, because the provider may have billed. The cost is recorded as `missing`.
   - The cutoff cannot refund running work. OCR may only start within 9 minutes of admission (`ocrDispatchWindow` = 15 − 5 − a 1-minute settle margin). Its context then ends within 5 minutes, and the result write and settle fit in the remaining minute. If the upload steps (the account-lock wait, `CreateScan`, `SaveImage`) take longer than the dispatch window, OCR is not started: the scan is marked failed (`ocr_failed`) and the page is released.
   - Expiry runs lazily for the requesting user before an OCR admission and before the allowance is read. It needs no sweeper, and OCR is never re-dispatched.
   - **Relation to #40's `ocr_interrupted`.** #40 marks a processing image scan `ocr_interrupted` after 6 minutes (the 5-minute OCR timeout plus a 1-minute write margin). That threshold is intentionally separate from the 15-minute reservation cutoff. An interrupted scan does **not** refund immediately: its page can stay charged for up to about 9 more minutes, until the reservation passes 15 minutes. The page is then refunded once, never twice.
   - The OCR deadline is absolute: `created_at` plus 5 minutes, using the same timestamp that is stored on the scan. A dispatched job whose deadline has already passed (slow upload steps within the dispatch window) makes no provider call. The scan is marked failed (`ocr_timeout`) and the page is released.
   - A late settlement from the abandoned job is a no-op, and a deleted user's rows are never recreated.
   - Explanation and speech reservations stranded by a crash mid-request still stay charged until the reset. Operators can find them via `idx_usage_reservations_status_created`.

## Recorded usage and cost

Each reservation records:
- gateway, region and model;
- the number of attempts;
- input, output and total tokens, and characters;
- provider cost and currency;
- latency;
- `cost_status`, which takes one of these values:
  - `reported`: every attempt reported a cost in the same currency. The summed provider value is stored.
  - `missing`: a provider call happened, but cost metadata was absent, partial or mixed. This is always the case for the legacy `minimax`/`gemini` clients, which report no metadata.
  - `none`: no provider call was made.

ANNOTA keeps no price table and stores **no estimates**, so provider price changes cannot corrupt stored values. Provider-side key caps are a supplementary control and may enforce late. Reconciling against real provider charges stays pending until live keys are available.

## Errors

| Status | `code` | Meaning |
|---|---|---|
| 429 | `usage_quota_exceeded` | The caller's own allowance is used up. Includes `feature`, `limit`, `used`, `remaining` and `resetAt`. |
| 429 | `usage_budget_exhausted` | The global beta budget is used up. Includes only `feature` and `resetAt`; global totals are never exposed. |
| 413 | `usage_request_too_large` | A single request can never fit (`max` is included). Not retryable. |
| 503 | `usage_unavailable` | The usage store could not be reached. The request fails closed. |
| 429 | `rate_limited` | Burst throttling (`AI_RATE_LIMIT`). |
| 429 | `ai_provider_limited` | The upstream provider rejected the call for quota or rate limit reasons. |

Usage 429s carry `Retry-After`. None of these are 401, so a budget rejection never signs the user out.

## Accounts

Per-user counters and reservations are deleted with the account through the `users` foreign-key cascade. Global counters are not decremented, because the cost may already have been incurred. A provider call still in flight when the account is deleted settles as a no-op: it recreates no rows and refunds nothing globally.
