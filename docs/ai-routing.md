# AI gateway routing

The backend supports independent OCR, annotation, and Japanese speech routes through OpenRouter and UCloud Global AstraFlow ModelVerse. This is protocol support, not a claim that every model, regional endpoint, or voice has been qualified.

## Enable routed mode

Set `AI_PROVIDER=routed` and `AI_ROUTING_CONFIG` to a JSON file. Absolute paths are recommended; relative paths resolve against the backend process working directory. Run the backend from `backend/` as documented. Keep credentials in backend environment variables (including `backend/.env`), and reference their names through `api_key_env`. The routing file must never contain API key values.

For containers, mount the file read-only, set its in-container absolute path, and inject keys through the operator's existing private environment mechanism. Do not bake a private file or keys into an image. No deployment change is made by this feature.

The following **synthetic example is for schema illustration only**. The `.test` endpoints and `test-*` models/voice will not work with real gateways. Replace them with exact, approved values after checking the chosen gateway/region/model's documentation and qualification evidence.

```json
{
  "version": 1,
  "profiles": {
    "active": {
      "kind": "openrouter",
      "region": "global",
      "base_url": "https://openrouter.test/api/v1",
      "api_key_env": "TEST_ACTIVE_GATEWAY_KEY",
      "timeout_seconds": 60,
      "max_attempts": 1,
      "models": {
        "test-vision": {
          "image_input": true,
          "text_input": true,
          "text_output": true,
          "json_mode": "json_schema",
          "ocr_output": "json",
          "max_output_tokens": 2048
        },
        "test-speech": {
          "text_input": true,
          "audio_output": true,
          "japanese_voices": [
            "test-ja"
          ],
          "audio_formats": [
            "mp3"
          ]
        }
      }
    },
    "unused": {
      "kind": "openrouter",
      "region": "global",
      "base_url": "https://openrouter.test/api/v1",
      "api_key_env": "UNUSED_KEY",
      "timeout_seconds": 60,
      "max_attempts": 1,
      "models": {
        "test-vision": {
          "image_input": true,
          "text_input": true,
          "text_output": true,
          "json_mode": "json_schema",
          "ocr_output": "json",
          "max_output_tokens": 2048
        },
        "test-speech": {
          "text_input": true,
          "audio_output": true,
          "japanese_voices": [
            "test-ja"
          ],
          "audio_formats": [
            "mp3"
          ]
        }
      }
    }
  },
  "tasks": {
    "ocr": {
      "profile": "active",
      "model": "test-vision"
    },
    "annotation": {
      "profile": "active",
      "model": "test-vision"
    },
    "speech": {
      "profile": "active",
      "model": "test-speech",
      "voice": "test-ja",
      "format": "mp3"
    }
  }
}
```

The test fixture includes an unused profile to demonstrate active-only key requirements. Real configurations can use one profile for all tasks or different profiles per task. For mixed gateways, set `kind` to `modelverse` on its profile, select its exact region/base URL, and declare that gateway's model IDs/capabilities. Do not reuse another gateway's model name or voice without verifying it.

## Configuration rules

- `version` must be `1`. All three task routes are required. Unknown fields, trailing JSON, malformed files, files over 1 MiB, and unsupported selections fail startup before database/Redis connections.
- Each named profile has `kind` (`openrouter` or `modelverse`), `region`, `base_url`, `api_key_env`, `models`, and optional `timeout_seconds`/`max_attempts`. Use a base URL ending in `/v1` or `/api/v1`, including OpenRouter's `https://openrouter.ai/api/v1`. ModelVerse regions use distinct hosts; choose the exact documented global endpoint. Endpoint paths are appended by adapters. URLs cannot contain credentials, query strings, or fragments. HTTP is accepted only for loopback fixtures.
- Only profiles referenced by tasks resolve a credential. Unused profiles still require valid declarations but need no key. Credential values are excluded from JSON serialization; do not log full configuration through other formatters.
- Models are keyed by their exact gateway-specific ID. Chat models require `text_input`, `text_output`, positive `max_output_tokens`, and `json_mode` (`prompt`, `json_object`, `json_schema`). OCR additionally requires `image_input` and `ocr_output` (`json` or `markdown`). A model ID is configurable without adding source constants.
- `json_mode=prompt` relies on task prompts and application validation; it does not claim provider schema enforcement. The other modes request the matching response format. OpenRouter requests disable provider failover and require supported parameters for structured modes; no request supplies alternate models. ModelVerse receives no OpenRouter-only fields.
- Use `ocr_output=markdown` for explicitly approved text/Markdown OCR models. This mode does not request structured JSON. JSON OCR cannot silently return malformed JSON as a successful transcript. Both modes produce the same application OCR response.
- Annotation output is normalized through existing compatibility helpers, then requires translation, contextual explanation, example, usage timing, word breakdown, alternative meanings, and kana. Incomplete/refused/truncated generations are errors. Reported models must match the selected ID or an exact documented entry in optional `reported_model_aliases`; absent reported IDs remain unknown.
- Speech models require `text_input`, `audio_output`, `japanese_voices`, and `audio_formats`. The route's `voice` must be in that model's approved Japanese list. This implementation supports explicit `format: mp3` only. Configured capability declarations are operator assertions that need real qualification, not automatic discovery or proof of quality.
- ModelVerse speech uses `/t2a_v2`, non-streaming JSON/hex audio, business-status checks, and Japanese language settings. OpenRouter speech uses `/audio/speech` and raw audio bytes. Both return the existing binary `audio/mpeg` API response. Context is never appended to the spoken text.

## Deadlines, attempts, and diagnostics

`timeout_seconds` defaults to 60 and must be 1–120. It covers the complete operation, including retries/backoff, and is shortened by the caller's deadline. `max_attempts` defaults to 1 and can be 1 or 2. An explicitly enabled second attempt is allowed only after HTTP 429/503. Valid `Retry-After` is honored within the remaining deadline; an absent/invalid header uses 500 ms. A delay that would exhaust the deadline prevents the retry. Connection/read failures, output validation failures, and truncation are never replayed automatically. Retrying may incur another charge.

Responses are bounded: 8 MiB chat JSON, 32 MiB binary speech, and 64 MiB ModelVerse speech JSON/hex. Redirects are rejected. HTTP 401/403 map to provider auth errors, 402 to quota, 429 to throttling; other failures remain upstream errors. Application API errors keep their existing shapes and status mapping.

Structured `AI gateway attempt` logs contain task, profile, gateway, region, selected/reported model, provider request ID when available, attempt, latency, HTTP status/error kind, and available usage/cost. Missing accounting values are `null`, not zero-cost claims. OpenRouter-reported chat cost is USD; no currency/price is invented for absent billing data. Raw provider error bodies and request payloads are excluded. Request IDs are provider-controlled metadata filtered for length/characters and the active credential; that filter is not general content redaction. Verify actual gateway ID formats and privacy behavior during live qualification. Existing request IDs correlate request-started AI work when present; this feature does not redesign detached OCR job tracing.

Existing route rate limits remain in place. They are not persisted per-user spending budgets. Application reservations/global quotas are tracked separately in #42, and model selection/quality qualification in #35. Gateway key caps are supplementary controls.

## Output provenance

Saved output records which configured producer generated it, so later quality differences can be explained (#35).

- **Fields:** each record holds identifiers only: version, provider mode, gateway, region, configured model and prompt version.
- **Excluded:** keys, prompts, request IDs, text and usage are never stored. Cost and usage belong to the quota ledger (#42).
- **OCR results:** stored in the nullable `scans.ocr_provenance` JSONB column (migration 007). A newer OCR result replaces the earlier provenance. A client that does not report a producer stores NULL.
- **Annotations:** the analyze response includes a `provenance` object. The client echoes it inside `nuance_data` when it saves the annotation. The server keeps it only if it exactly matches the server's own current annotation producer, so a client cannot store arbitrary values such as keys, request IDs or text. A malformed or mismatched value is dropped without rejecting the save. So is a producer change between analyze and save. In those cases the annotation's provenance is unknown.
- **Historical rows:** existing rows are not backfilled and stay unknown.
- **`reportedModel`** is the model the gateway reported back. It is recorded only for routed calls, where the adapter has already checked it against the selected model and the route's `reported_model_aliases`. The value comes from the last attempt for that task in #42's request-scoped `usage.Calls(ctx)`, and only when that attempt succeeded.
  - Legacy and Gemini clients, `AnalyzeWithLanguageAPI` (not metered), and attempts that report no model leave it unknown. The configured model is never substituted.
  - When an annotation is saved, an echoed `reportedModel` is kept only if the route accepts it. Otherwise only that field is dropped, and the configured producer is kept.

## Migrate an existing install

`AI_PROVIDER=minimax` remains the default and retains OpenRouter OCR plus direct MiniMax annotation/speech settings. `AI_PROVIDER=gemini` retains Gemini and the existing `GEMINI_API_KEY` before `GOOGLE_API_KEY` precedence. Neither legacy mode needs a routing file; supplying `AI_ROUTING_CONFIG` in either mode is an error so it cannot be silently ignored.

To migrate, prepare approved profiles/models/voices, set routed mode and the file path together, inject only active profile keys, and restart the backend. Changes take effect on restart, not through hot reload. Invalid routed configuration never falls through to a legacy mode. To intentionally return to a legacy mode, unset the routing path, select the legacy mode, restore its credentials, and restart.

`.envrc` loads developer environment values but leaves AI key validation to the backend, allowing all three modes. No frontend change is required.

## Qualification status

Local HTTP contract tests cover both protocols, task separation, validation, errors, and binary audio handling. They make no real inference requests. Before enabling a route, qualify exact region/model/voice access with one lawful representative Japanese page, one contextual annotation, and one short utterance. Record request ID, latency, usage/cost when supplied, and actual audio decoding/listening in `docs/evidence/issue-34/gateway-qualification.md`. Leave unavailable billing fields unknown. Global ModelVerse speech and Japanese voice availability remain unverified until those calls pass.

`go run ./cmd/qualify-routes` (from `backend/`) runs that procedure without the authenticated app, database, or Redis. Without `-live` it only validates the routing file and active keys. With `-live` it makes one bounded, possibly billed call per selected task. It prints a JSON report of route/attempt metadata and derived checks without sample text, output, audio, or keys. It also fails when a provider ID echoes sample or output text. The optional `-live -output-dir` writes OCR text and annotation JSON to a new private local directory for accuracy scoring; that directory holds source/model text, stays outside the report, and must never be committed. See the evidence file for usage. Candidate models, the fixed Japanese evaluation set, and acceptance targets for choosing routes are in [`evidence/issue-35/model-matrix-2026-10-20.md`](evidence/issue-35/model-matrix-2026-10-20.md) (#35); none is qualified yet.

References: [AstraFlow regional CLI](https://github.com/ucloud/astraflow-cli), [ModelVerse chat](https://github.com/UCloudDoc-Team/modelverse/blob/master/api_doc/text_api/openai_compatible.md), [global DeepSeek OCR](https://astraflow.ucloud-global.com/en-us/docs/modelverse/api_doc/text_api/deepseek-ocr.md), [ModelVerse speech](https://github.com/UCloudDoc-Team/modelverse/blob/master/api_doc/audio_api/minimax-speech.md), [OpenRouter speech](https://openrouter.ai/blog/tutorials/text-to-speech/), [OpenRouter model properties](https://openrouter.ai/docs/api/api-reference/models/list-all-models-and-their-properties).
