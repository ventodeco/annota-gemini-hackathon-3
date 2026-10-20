# Issue #44: actionable beta monitoring and support references

Base: `origin/dev` at 5488992 (includes #14 service layer, #39 Clerk, #40 reader, #41 privacy, #42 quotas, #35 provenance).

## User story

As a beta reader, when an operation fails I can see a short reference to give support, without sending my private page.
As the operator, I can look up that reference and find failure, latency, and AI usage/cost signals in the existing structured logs without reading page contents.

## Audit (current dev)

- `LoggingMiddleware` generates a request ID and logs it, but never returns it. Clients cannot show it, so support cannot look up a failed request.
- The request log stores `duration` as a Go duration string (`1.2s`). This cannot be aggregated.
- The routed AI `Observe` hook already logs `AI gateway attempt` per provider call, with the request ID. Each entry includes task, gateway, requested/reported model, attempt, duration, status, error kind, tokens, characters, and cost. The same hook feeds the #42 usage collector. It is reused as is, with no second observer.
- `ScanService.ProcessOCR` logs with `scan_id` only. Its terminal failure reason (`ocr_failed`, `ocr_timeout`, `no_text_found`) is persisted but never logged, so an OCR failure cannot be tied to the upload request.
- `/healthz` and `/readyz` already exist and are documented in `docs/commercial-ops.md`. They are reused unchanged.
- Analytics:
  - `reader_activation` is sent on every image/PDF upload from WelcomePage only. It always uses `source: image`, even for a gallery pick.
  - CameraPage uploads send nothing.
  - `first_annotation` is allow-listed but never sent. A client cannot know "first" reliably.
  - "First" therefore must be derived per `user_id` at query time, not claimed by the client.
  - Explanation, save, and reopen are already visible as authenticated request logs: `POST /v1/ai/analyze`, `POST /v1/annotations`, and document/scan GETs, plus #23's open route when it lands. No new events are needed for those.

## Scope

Backend
1. `middleware.LoggingMiddleware`:
   - Set `X-Request-ID` on every response before the handler runs.
   - Add a numeric `duration_ms` to the request log entry.
   - CORS: `Access-Control-Expose-Headers: X-Request-ID` wherever CORS headers are granted, so the cross-origin SPA can read it.
   - Never accept a client-supplied ID.
2. `ScanService.ProcessOCR`:
   - Log with the originating `request_id`. The context already carries it through `WithoutCancel(usageCtx)`.
   - Write one `OCR scan failed` warning per terminal failure, with `scan_id` and `reason`.
   - No other service changes.
3. Analytics allow-list:
   - Remove the unused, misleading `first_annotation`.
   - Keep `reader_activation` as an occurrence event (one per successful upload).
   - Document that first activation is the earliest event per `user_id`.
4. OpenAPI/api.md: document the `X-Request-ID` response header and the corrected event enum.

Frontend
5. `ApiError.requestId` from the `X-Request-ID` header (null when missing, e.g. a network failure, backend down, or offline).
6. Show the reference where a user sees a failed operation, using a small `ErrorReference` (text plus copy button, keyboard and screen-reader accessible). Placement:
   - WelcomePage and CameraPage upload errors.
   - LoadingPage "couldn't check this scan".
   - OCR failure states (the reference is the scan ID, which the logs carry as `scan_id`).
   - Explanation/save/regenerate/speech error toasts (`Reference …` description).
   - If there is no ID, nothing is shown. Monitoring never blocks reading.
7. Activation events:
   - Gallery uploads send `source: gallery`; camera uploads send `source: camera`.
   - Sending stays fire-and-forget, so delivery failure is ignored.

Docs
8. Short operator triage guide `docs/beta-triage.md`, linked from `docs/commercial-ops.md` Support:
   - Look up a reference.
   - Health/readiness, OCR failure/stuck queries, AI failure/latency/cost by task/model, and funnel derivation.
   - Proposed alert thresholds.
   - Owner/destination pending.

## Out of scope / pending

- **Report a problem submission:** pending #80 (Contact Us page, user decision) and a real configured destination. No invented inbox, email, `/contact` link, or log-as-inbox endpoint. The displayed reference is what a user will include in a report.
- **Alert destination and owner:** undecided. The guide lists thresholds and queries only; no alerting tool, no live sends.
- **No new tracking platform, no client log shipping, no new provider observer.**
- **No changes to quota/accounting, Clerk, privacy caches, or reader deadlines.**

## Edge cases

- **Backend down / offline:** no response or header, so no reference is shown. The existing error copy remains.
- **Missing header (old server or proxy):** `requestId` is null, so the reference is hidden.
- **Analytics delivery failure:** already swallowed. It never blocks navigation.
- **Sensitive data:** the reference is a random hex ID. No page text, model output, or token goes into the UI reference or the new log fields. The OCR failure log carries only the scan ID and reason code.
- **Model failure with HTTP success:** already logged per attempt with `error_kind` by the routed observer. The guide queries it.
- **Missing cost:** `cost` is null in the attempt log. The guide treats null as unknown, not zero.

## Acceptance criteria

- Every API response carries `X-Request-ID`, it is readable cross-origin, and it matches the `request_id` in the request log.
- A failed upload/explanation/speech shows the reference in the UI. Searching the logs for it finds the request log, the failure log, and AI attempt lines.
- A terminal OCR failure has one log line with `request_id`, `scan_id`, and `reason`.
- Request logs have `duration_ms`.
- Camera and gallery uploads report the correct activation source. No event claims "first".
- The triage guide exists and needs no page contents.

## Verification (proportional)

- **Go:** focused middleware test (header, CORS expose, `duration_ms`) and the analytics test update. Then `go test ./...`, vet, and build in CI.
- **Web:** one focused test (ApiError requestId plus reference rendering). Then lint/test/build in CI.
- **Browser:** desktop and mobile screenshots of one error-reference state. Results go in the PR/issue, not committed reports.
