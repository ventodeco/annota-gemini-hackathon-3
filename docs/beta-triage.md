# Beta Triage Guide

How the operator finds a failed operation, checks health, and reads failure, latency and AI cost signals without opening anyone's pages. Liveness, readiness and support policy live in [`commercial-ops.md`](commercial-ops.md).

The backend writes one JSON object per line to stdout. Collect it however the host keeps container logs. The examples below read from a file or pipe called `backend.log` and use `jq`. Startup lines are plain text; if they are in the input, use `jq -cR 'fromjson? | …'` instead of `jq -c '…'`.

## Privacy rules

- Logs and the queries below hold IDs, routes, status codes, timings, model metadata and usage counts. They never contain page text, OCR output, explanations, keys or tokens. Keep it that way: do not add `scans.full_ocr_text`, `annotations` content or uploaded files to a query.
- Open a user's page, image or PDF only when the user explicitly agrees to it for a specific report. A reference alone is not consent.

## Look up a reference

Users see a **Reference** under a failed upload, scan check, explanation, save, or speech request.

- **32 hex characters** (for example `3f2a…`): this is the `X-Request-ID` of the failed request. It appears in the request log, the `Request failed` line, any `AI gateway attempt` lines, and the OCR lines for a scan uploaded by that request.

  ```bash
  jq -c 'select(.request_id == "REFERENCE")' backend.log
  ```

- **`scan-<id>`**: shown when a scan failed or found no text. Look it up by scan ID:

  ```bash
  jq -c 'select(.fields.scan_id == ID)' backend.log
  ```

  The `OCR scan failed` line gives `fields.reason` (`ocr_failed`, `ocr_timeout`, `no_text_found`) and the upload's `request_id`. The `Marked interrupted OCR scan as failed` line means the job died and was recovered when the scan was read.

- **No reference**: the request never reached the server, for example when the user was offline or the backend was down. Check `/readyz` and the request log around the reported time.

## Health

- `/healthz` reports liveness and `/readyz` reports PostgreSQL and Redis readiness. Neither makes AI calls.
- Request failures:

  ```bash
  jq -c 'select(.message == "HTTP Request" and .status_code >= 500) | {timestamp, request_id, method, path, status_code, duration_ms}' backend.log
  ```

- Slow requests:

  ```bash
  jq -c 'select(.message == "HTTP Request" and .duration_ms > 10000) | {path, duration_ms, request_id}' backend.log
  ```

## OCR, explanation and speech

- OCR failures by reason:

  ```bash
  jq -r 'select(.message == "OCR scan failed") | .fields.reason' backend.log | sort | uniq -c
  ```

- Stuck OCR jobs. A scan should settle within 5 minutes. Any row here is stuck:

  ```sql
  SELECT id, user_id, created_at FROM scans
  WHERE status = 'processing' AND created_at < now() - interval '10 minutes';
  ```

- AI provider attempts (routed provider only). Each provider call logs task, gateway, model, attempt, duration, status, error kind, tokens and cost. A provider failure shows `error_kind` even when the HTTP transport succeeded.

  ```bash
  jq -c 'select(.message == "AI gateway attempt") | .fields.ai | {task, requested_model, reported_model, status_code, error_kind, duration_ms, total_tokens, cost, currency}' backend.log
  ```

- Latency, outcome and cost per feature for the day, from the durable usage ledger (#42). It holds no content:

  ```sql
  SELECT feature, status, count(*) AS operations,
         percentile_cont(0.95) WITHIN GROUP (ORDER BY latency_ms) AS p95_latency_ms,
         sum(provider_cost) FILTER (WHERE cost_status = 'reported') AS reported_cost,
         count(*) FILTER (WHERE cost_status = 'missing') AS cost_missing
  FROM usage_reservations
  WHERE usage_day = current_date
  GROUP BY feature, status ORDER BY feature, status;
  ```

  `cost_missing` means the provider sent no cost metadata. Treat it as unknown, not zero. Legacy `gemini`/`minimax` providers never report cost. Currency is per row, so do not add values across currencies.

## Activation funnel

Each funnel step is an occurrence. "First" is the earliest occurrence for each `user_id`, so repeat uploads are never counted as first.

| Step | Signal |
| --- | --- |
| Reader activation | `analytics_event` with `fields.event == "reader_activation"` and `fields.properties.source` set to `camera`, `gallery` or `pdf` (older clients sent `image` for gallery uploads) |
| First explanation | `HTTP Request` with `POST /v1/ai/analyze` and status 200 |
| Save | `HTTP Request` with `POST /v1/annotations` and status 201 |
| Reopen | `HTTP Request` with `GET /v1/documents/{id}` or `GET /v1/scans/{id}` at least a day after that user's activation |

To list first activation per user:

```bash
jq -sr '[.[] | select(.fields.event == "reader_activation")] | group_by(.user_id)[] | min_by(.timestamp) | "\(.user_id) \(.timestamp) \(.fields.properties.source)"' backend.log
```

Analytics delivery is best-effort. A failed event never blocks the user, so treat counts as a lower bound.

## Proposed alert thresholds

The alert tool, destination and owner are **not chosen yet**, so nothing sends alerts today. When they are chosen, start with these thresholds and record the owner here:

| Signal | Threshold |
| --- | --- |
| `/readyz` not 200 | 2 consecutive checks, 1 minute apart |
| `/healthz` not 200 | 1 check (restart the container) |
| 5xx responses on `/v1/*` | 5 or more in 15 minutes |
| Stuck OCR (query above) | any row |
| `OCR scan failed` with `ocr_failed` or `ocr_timeout` | 3 or more in 15 minutes |
| `AI gateway attempt` with `error_kind` `auth` or `config` | any (likely key or configuration problem) |
| `AI gateway attempt` with `error_kind` `upstream`, `rate_limit` or `quota` | 5 or more in 15 minutes |
| p95 explanation latency (usage query) | over 15 seconds for the day |

To avoid alert floods, group each signal into one alert per 15-minute window.

## Feedback

There is no in-app report form yet. The Contact Us page and its destination are tracked in #80. Until then, ask users for the reference they saw, the time, and what they were doing. Do not ask for page content.
