# Configurable AI Gateway Routing Implementation Plan

> **For agentic workers:** Use superpowers:executing-plans after owner approval. Execute inline; delegation requires a separate request. Track the steps below as they are completed.

**Status:** Implementation approved on 2026-10-20 and completed locally. Live provider qualification remains pending owner credentials and selected routes. See `docs/evidence/issue-34/implementation.md`.

**Issue:** [#34: Add configurable AstraFlow and OpenRouter task routing in Go](https://github.com/ventodeco/annota-gemini-hackathon-3/issues/34).

**Branch:** `feat/issue-34-ai-gateway-routing`, created from `origin/dev` at `39349ec0ab501bf4798547b8b81ec53a4d02f273`.

**Goal:** Let the operator independently select the gateway, model, and supported options for OCR, annotation, and Japanese speech without changing the frontend or public API.

**Architecture:** Preserve `ai.Client` and its response types. Add explicit, validated gateway profiles and task routes, share Chat Completions transport for OCR/annotation, and use separate speech protocol adapters. Keep the existing Gemini/MiniMax modes as an explicit migration path; routed mode never substitutes another provider or model after a failure.

**Tech Stack:** Existing Go backend, `net/http`, `encoding/json`, `httptest`, and structured logger. No new SDK or service is required.

## Global constraints

- Keep work in Go + React/Vite; this delivery changes Go/configuration/docs only. No Anvia dependency, TypeScript AI service, agent loops, or generic plugin framework.
- Preserve the four `ai.Client` methods, OCR/annotation response fields, `/v1` request/response shapes, and speech binary/MIME contract.
- Secrets stay in backend environment variables. Routing files contain environment variable names, never credential values. Never log full configuration, provider response bodies, source text, images, or keys.
- Keep model IDs scoped to their gateway and configurable without new source constants. Capabilities are explicit configuration supported by provider evidence, not proof of successful inference.
- Retain existing application authentication and `AI_RATE_LIMIT` controls. Persisted per-user/global budget reservations remain #42; gateway key caps do not replace those controls.
- Model/voice selection and Japanese quality qualification coordinate with #35. Do not change default models or claim live readiness from local contract tests.
- No production configuration changes, deployment, paid requests, push, or PR creation are authorized by this planning request.
- Follow the issue's 30-minute total test-debugging budget per implementation task. Stop earlier for unavailable tools/services/credentials. Record the command, first useful failure, suspected cause, attempts, and next action; posting to GitHub needs owner authorization.

## Why this issue

The dependency/runtime upgrades #36/#37 are already on `dev`. #13's wildcard credential fix is present. #15's index exists in the initial schema, and its existing-database repair is already in #43's release plan. #34 has a confirmed unimplemented boundary and can proceed without the Clerk migration #39 or access to production secrets.

## Current evidence

- `backend/internal/ai/client.go`: `Client` already isolates application operations. `AI_PROVIDER=minimax` couples OpenRouter OCR, MiniMax Anthropic annotation, and direct MiniMax speech. Gemini is a separate complete bundle.
- The same file mixes HTTP transport, prompts, normalization, retries, and audio decoding. `postJSON` uses the default HTTP client without its own deadline, retries ambiguous transport failures, reads unbounded bodies, and places upstream response text in `ProviderError.Message`.
- `backend/internal/config/config.go`: validation requires both OpenRouter and MiniMax keys for the current default bundle, rather than keys for independently selected routes.
- `backend/cmd/server/main.go`: constructs one `ai.Client` for all handlers. This is the integration point to preserve.
- `.envrc`: unconditionally requires OpenRouter and MiniMax credentials, including for Gemini mode.
- `backend/internal/ai/client_test.go`: existing HTTP contract tests cover OpenRouter vision, MiniMax Anthropic annotations, MiniMax hex speech, and legacy error behavior. Extend these contracts rather than replacing the test framework.
- `backend/internal/handlers/ai.go`: maps `ProviderError` kinds to existing JSON errors and writes speech bytes with their MIME type. Keep those contracts.
- `scripts/ops/smoke-images.sh`: uses explicit legacy provider settings and inert local endpoints. Legacy-mode compatibility must keep this smoke check working.

The earlier research remains untracked in the original checkout at `/Users/naufaldisatriya/WebApps/annota/docs/research/2026-10-20-ai-gateways.md`; it was read without modifying or copying the owner's files. This plan includes its required source links below so execution does not depend on that absolute path.

## Proposed design and tradeoffs

Use a small JSON routing file selected by `AI_ROUTING_CONFIG` with `AI_PROVIDER=routed`. A file keeps profiles, model capabilities, and task references readable. A large set of task-specific environment variables would duplicate profile details and make cross-field validation harder. A dynamic provider/plugin registry is unnecessary for two gateways.

Configuration ownership stays in `internal/ai/routing.go`. `config.Load()` reads the file through the AI package's loader, which parses and validates it once into server-owned state. `ai` must not import the application `config` package. Startup constructs the routed client before opening database/Redis connections so bad routing fails before infrastructure work.

Proposed public construction seam:

```go
func LoadRoutingConfig(path string, lookupEnv func(string) string) (RoutingConfig, error)
func ValidateRoutingConfig(cfg RoutingConfig) error
func NewRoutedClient(cfg RoutingConfig, opts RoutedClientOptions) (Client, error)
```

`RoutingConfig` contains `Version` (initially 1), `Profiles map[string]GatewayProfile`, and `Tasks TaskRoutes` with fixed fields `OCR`, `Annotation`, `Speech`. Each `TaskRoute` has `Profile`, `Model`, and optional speech `Voice`/`Format`. `GatewayProfile` contains `Kind` (`openrouter` or `modelverse`), `Region`, `BaseURL` including `/v1`, `APIKeyEnv`, a resolved `APIKey` excluded from JSON, `TimeoutSeconds`, `MaxAttempts`, and `Models map[string]ModelCapabilities`.

`ModelCapabilities` declares image/text input, text/audio output, `JSONMode` (`prompt`, `json_object`, or `json_schema`), `OCROutput` (`json` or `markdown`), positive `MaxOutputTokens` for chat models, and explicit Japanese speech voices/formats. An optional `ReportedModelAliases []string` records only documented canonical names a gateway may return for that selected model. These are gateway-scoped allowlists. A text-only model is illegal for OCR; a speech route without an approved Japanese voice/format is illegal. Unknown IDs/capability values fail instead of being guessed. Declared capabilities need documented qualification before enabling real traffic.

State boundary: file/env input -> parsed route configuration -> validated profiles/models -> bounded upstream attempt -> validated task result or `ProviderError`. Startup owns immutable routes; the operator changes them through configuration and restart. Upstream output becomes trusted application data only after task-specific parsing. No runtime discovery, automatic gateway switching, or implicit model selection.

Migration policy:

- `AI_PROVIDER=minimax` and `AI_PROVIDER=gemini` retain current settings, behavior, and defaults. Existing `GEMINI_API_KEY`/`GOOGLE_API_KEY` precedence remains unchanged.
- `AI_PROVIDER=routed` requires `AI_ROUTING_CONFIG` and all three explicit task routes. Validate keys only for profiles referenced by those routes; unrelated legacy keys are optional and ignored in this mode.
- A routing-file setting in a legacy mode is an actionable configuration error, preventing a routing file from being silently ignored. Missing/invalid routed configuration never falls through to a legacy client.
- Remove unconditional AI key requirements from `.envrc`; backend validation becomes the single authority. Document both modes together. No immediate removal of legacy adapters or the Gemini SDK.

Reliability policy for new routes: one total profile deadline covers all attempts and waits, shortened by the caller's deadline. Allow `TimeoutSeconds` from 1 to 120; default 60. `MaxAttempts` defaults to 1 and is restricted to 1 or 2. An explicitly enabled second attempt is limited to HTTP 429/503, honoring valid `Retry-After` within the remaining deadline; use a 500 ms delay when the header is absent/invalid. Do not retry when the required delay would exhaust the deadline. Never replay ambiguous connection/read timeouts, malformed output, or truncation. Every retry is observable and can incur cost; do not claim exactly-once billing. Keep legacy behavior separate in this delivery.

## Task 1: Define and validate gateway/task configuration

**User Story:** As the operator, I want invalid route/model/credential combinations rejected at startup, so that a configuration mistake cannot silently send requests elsewhere.

**Files:** Create `backend/internal/ai/routing.go`, `backend/internal/ai/routing_test.go`, and `backend/internal/ai/testdata/routing.json`. The fixture uses only synthetic IDs and keys and is never a production default.

**Interfaces:** Implement `LoadRoutingConfig` and `ValidateRoutingConfig` above. Define `RoutingConfig`, `GatewayProfile`, `TaskRoutes`, `TaskRoute`, and `ModelCapabilities` here. Validation is shared by the loader and constructor, not reimplemented in app configuration.

- [x] Add table-driven tests for valid single-gateway and mixed-gateway routes, same model ID in two profiles, and custom model IDs supplied entirely by configuration.
- [x] Add rejection cases for unknown fields/version, malformed or trailing JSON, missing task/profile/model, invalid capability values, missing active key, invalid timeout/attempt count, text-only OCR, unsupported annotation mode, and unsupported Japanese voice/format. Confirm an unused profile needs no key and does not trigger environment lookup. Parse errors must not quote secret-bearing input.
- [x] Decode a bounded file with `json.Decoder.DisallowUnknownFields`, require end of input, resolve only active `APIKeyEnv` values, and validate nonempty active region/model IDs. Accept HTTPS upstream URLs; permit HTTP only for loopback tests/local fixtures. Reject URL userinfo, query/fragment, unsupported schemes, and duplicated API-path configuration.
- [x] Run the focused tests red first, then implement the minimum parser/validator. Add one test proving resolved credentials are omitted when the resulting configuration is JSON encoded.

Representative fixture lookup and invariant assertions, inside tests using `testdata/routing.json`:

```go
cfg, err := LoadRoutingConfig("testdata/routing.json", func(name string) string {
    if name == "TEST_ACTIVE_GATEWAY_KEY" { return "fixture-key" }
    t.Fatalf("unexpected lookup for unused credential %q", name)
    return ""
})
if err != nil { t.Fatal(err) }
cfg.Tasks.OCR.Model = "not-in-this-profile"
if err := ValidateRoutingConfig(cfg); err == nil {
    t.Fatal("expected an unknown model to be rejected")
}
```

**Edge Cases:** Empty file/key, whitespace, inactive profiles, typos in options, different gateways using identical model strings, and region-specific routes. All configured profiles need structurally valid declarations; only active ones require credentials.

**Definition of Done:** Valid fixtures resolve deterministic task routes; invalid fixtures fail before any HTTP call with safe actionable errors. Adding an approved model through configuration requires no new source constant.

**Verification:** From `backend/`, `rtk proxy go test -count=1 ./internal/ai -run 'TestRouting'`. No external credentials or services.

## Task 2: Implement bounded chat transport and OCR/annotation adapters

**User Story:** As a reader, I want OCR and explanations to retain their current usable results when the operator changes gateway, so that provider selection does not alter the application contract.

**Files:** Create `backend/internal/ai/gateway_http.go`, `gateway_http_test.go`, `chat_tasks.go`, and `chat_tasks_test.go`. Reuse existing prompt/normalization helpers in `client.go`; modify a helper there only when a focused regression requires it.

**Interfaces:** `chatTasks` owns the validated OCR/annotation routes and implements the three existing OCR/annotation method signatures. Shared transport returns content, finish reason, provider/model/request ID, and optional usage/cost metadata. Define `CallMetadata` and `RoutedClientOptions` (`HTTPClient *http.Client`, `Observe func(context.Context, CallMetadata)`) for the final constructor. Internal adapters remain unwired to startup until Task 4.

- [x] Write `httptest` cases exercising each task against both gateway kinds. Capture URL, bearer key, exact configured model, image data URL, prompt/knowledge context, and response-format parameters. A mixed profile test must prove no request or credential reaches the other task's server.
- [x] Implement `/chat/completions` behind a shared transport. Keep gateway-specific fields in explicit adapter branches; never send OpenRouter-only `provider` fields to ModelVerse. For OpenRouter strict JSON modes require supported parameters and disable provider failover; send exactly one model and no model fallback list.
- [x] Enforce the total deadline/retry policy above, context cancellation, redirect rejection, and bounded response reads (8 MiB chat JSON). Use injected HTTP clients without replacing their transports or mutating shared instances. Preserve `errors.Is` for cancellation/deadlines while returning safe typed provider failures.
- [x] Inspect the provider envelope before task parsing: empty choices, HTTP-200 error payload, refusal, and token-limit/truncation finish reasons are failures. Reject a returned model that differs from the selected model, except documented gateway canonicalization explicitly represented in the model's configuration; never normalize IDs by guessing.
- [x] OCR: send base64 image content. For `json`, require usable `raw_text` and normalize the current `{raw_text, language}` shape; malformed JSON is an error. For explicitly configured `markdown`, accept nonempty text/Markdown, retain line breaks, and construct the same structured response. Do not let broken JSON masquerade as an OCR transcript.
- [x] Annotation: use `buildEnhancedPrompt`, including knowledge entries, then normalize legacy aliases and object `word_breakdown` through existing helpers. Require the application's nonempty translation/explanation/usage fields and kana reading after normalization. Null, empty objects, arrays, incomplete fields, invalid types, or truncated output fail clearly.
- [x] Classify HTTP 401/403 as auth, 402 as quota, 429 as throttling, and other upstream failures appropriately. Sanitize errors before they reach existing handler logs. Test that upstream echoes containing fixture document text or fixture credentials never appear in `Error()` or observation output.
- [x] Emit one `CallMetadata` observation per upstream attempt: task/profile/gateway, region, requested/reported model, provider request ID, attempt number, duration, status/error kind, and available token/character/cost fields. Missing usage/cost/model metadata stays explicitly unknown; zero usage/cost is not a substitute. Do not derive prices or bill users here.

**Edge Cases:** Missing usage/model fields, Markdown-only OCR, response refusal, stream-shaped response despite non-streaming request, invalid annotation types, oversized response, cancellation during backoff, long `Retry-After`, and ambiguous transport failures after a POST may have been accepted.

**Definition of Done:** Both gateways pass local OCR/annotation contract tests, preserve existing response types, and reject invalid output. Captured requests prove correct per-task routing; request counts/deadlines prove bounded attempts. No raw payload or credential appears in safe diagnostics.

**Verification:** `rtk proxy go test -count=1 ./internal/ai -run 'TestGatewayHTTP|TestChatTasks|TestClientOCR|TestClientAnnotate'` and `rtk proxy go test -race ./internal/ai` after the task is implemented. Use short local deadlines, not multi-minute waits.

## Task 3: Implement the two speech protocols

**User Story:** As a Japanese learner, I want the configured speech route to return valid playable audio, so that a gateway change does not break pronunciation playback.

**Files:** Create `backend/internal/ai/speech_tasks.go` and `speech_tasks_test.go`; reuse Task 2's bounded HTTP/error/observation machinery.

**Interfaces:** `speechTask.SynthesizeSpeech(context.Context, string, string) (*SpeechResponse, error)` consumes the validated speech route. Both protocol branches return the existing audio bytes/MIME fields. Selected text is spoken; context must never be appended as spoken content.

- [x] Add local contract tests for ModelVerse `/t2a_v2` JSON/hex and OpenRouter `/audio/speech` raw bytes. Assert gateway-specific request fields, selected model/voice, Japanese settings where supported, and explicitly requested MP3 output.
- [x] ModelVerse: send non-streaming `text`, `voice_setting`, `audio_setting`, and documented language options. Require successful business status and complete nonempty data, decode hex, and map a verified format to MIME. Capture `trace_id` and billed characters when present. Test HTTP-200 business errors, null data, invalid hex, empty/incomplete audio, and format mismatch.
- [x] OpenRouter: send `model`, `input`, `voice`, and `response_format: mp3`. Require successful HTTP status, MP3-compatible content type, and nonempty audio with a plausible MP3 header. Reject JSON/error bodies even on HTTP 200. Capture generation/request ID; unavailable usage/cost remains unknown.
- [x] Bound binary audio to 32 MiB and ModelVerse JSON/hex to 64 MiB, rejecting oversized responses rather than returning partial audio. Restrict this delivery to explicitly approved MP3 output rather than guessing PCM sample settings; keep existing Gemini PCM/WAV handling untouched.
- [x] Test the same timeout/cancellation/attempt rules as chat, without creating a second retry layer. A profile must not borrow a voice or model from another profile.

**Edge Cases:** Japanese voice unavailable in selected region, model/voice mismatch, missing MIME, unsupported output format, incomplete hex/audio, provider JSON with HTTP 200, and a request cancelled before audio arrives.

**Definition of Done:** Both local protocol contracts return valid fixture MP3 bytes and `audio/mpeg`, and material failures produce safe typed errors. Global ModelVerse availability and Japanese pronunciation remain live qualification gates; no new speech defaults are enabled from mainland examples alone.

**Verification:** `rtk proxy go test -count=1 ./internal/ai -run 'TestSpeechTasks|TestClientSynthesizeSpeech'`. Decode a small valid fixture with a local audio decoder during implementation; header checks alone do not prove an audio file is playable. No real synthesis request is needed for these tests.

## Task 4: Wire startup, document migration, and prove application compatibility

**User Story:** As the operator, I want to enable validated routes through server configuration while existing installs keep working, so that I can migrate without changing readers or leaking keys.

**Files:** Modify `backend/internal/config/config.go`, `config_test.go`, `backend/internal/ai/client.go`, `backend/cmd/server/main.go`, `backend/.env.example`, `.envrc`, `README.md`, and `backend/AGENTS.md`. Create `backend/internal/ai/routed_client.go`, `routed_client_test.go`, `backend/cmd/server/ai_client.go`, and `ai_client_test.go`. Add integration assertions to `backend/internal/handlers/ai_speech_test.go`; retain `scripts/ops/smoke-images.sh` unless its existing fixture actually needs a compatibility correction.

**Interfaces:** Add `Config.AIRoutingConfigPath string` and `Config.AIRouting *ai.RoutingConfig`. Add `buildAIClient(cfg *config.Config) (ai.Client, error)` in `cmd/server/ai_client.go`. Implement `NewRoutedClient` as a composite of the adapters above; keep `NewClient(ClientConfig) Client` intact for legacy callers.

- [x] Write config/startup tests for explicit routed mode, missing/invalid file, active-only credentials, routing file set in a legacy mode, independent task changes, and both existing legacy bundles. Isolate temporary cwd/env so developer `.env` files cannot affect tests; do not run environment-mutating tests in parallel.
- [x] Add routed startup selection before infrastructure connections. Reuse `ValidateRoutingConfig` when constructing the client. Wire the observation callback to the current structured logger using safe allowlisted fields; include correlation ID when available without introducing a second request-context convention.
- [x] Add a full `NewRoutedClient` test where OCR, annotation, and speech hit their configured local servers, then change a route/model in configuration and prove the next constructed client routes accordingly. Use a compile-time `var _ Client = (*routedClient)(nil)` assertion for interface compatibility.
- [x] Exercise the real routed client through existing AI handlers with `httptest` and mock storage. Assert unchanged annotation JSON and speech bytes/MIME, and existing public 429/502 error semantics. Confirm application rate limiting still wraps the same routes in `main.go`; do not rewrite quota accounting.
- [x] Update `.env.example`, `.envrc`, README, and backend guidance in the same delivery: explicit mode selection, routing file schema and absolute/working-directory path rules, supported adapter options, active-key rules, restart semantics, redacted diagnostics, and how to retain or leave legacy mode. Use clearly labeled synthetic JSON examples; no unqualified production model/voice defaults.
- [x] Review the existing final-image smoke fixture for legacy compatibility. Routed configuration is an operator-supplied file: document mounting it read-only and passing its path, without baking secrets or a machine-specific routing file into images. Production deployment stays outside this issue.
- [x] Run focused integration tests, then the required backend test/vet/build checks once. Inspect `git diff --check` and ensure no unrelated application changes, UI changes, API-schema changes, or dependency upgrades slipped into scope. Commit only after approval and verification with an issue-referencing message such as `feat: add configurable AI gateway routes (#34)`.

**Edge Cases:** Legacy Gemini install without MiniMax/OpenRouter keys, new mode with stale legacy variables, missing mounted config, invalid routing before DB startup, absent request ID, handler quota/auth errors, and synthetic CI startup without paid calls.

**Definition of Done:** A valid route file controls all three tasks with no frontend/handler contract changes; invalid routed config fails before infrastructure startup. Legacy config/tests and final-image smoke remain valid. Documentation and runtime validation agree. Local implementation completion is reported separately from live-provider readiness.

**Verification:** From `backend/`, run `rtk proxy go test -count=1 ./internal/config ./internal/ai ./cmd/server ./internal/handlers`, then `rtk proxy go test ./...`, `rtk proxy go vet ./...`, and `rtk proxy go build ./cmd/server`. Use the toolchain declared in `backend/go.mod`. When using the repository flake, enter `nix develop` before commands; stop and report an unavailable/incompatible toolchain instead of changing dependency versions. Existing CI should run the final backend image smoke; a local rerun is needed only when runtime packaging changes or CI cannot provide that evidence.

## Live qualification and completion gate

Local contract tests prove routing/protocol behavior, not account/region access, actual model support, pronunciation, or billing. After the owner privately supplies credentials and chooses the routes, coordinate bounded qualification with #35: one representative Japanese OCR page, one annotation with contextual/knowledge fields, and one short Japanese utterance per selected route. Use lawful synthetic/public material. Only repeat a request to resolve a concrete failure.

Record date, exact gateway/region/model/voice, provider request ID, latency, finish reason, usage/cost when supplied, and outcome in `docs/evidence/issue-34/gateway-qualification.md`. Keep missing cost explicitly unknown. Listen to/decode actual speech. Confirm global ModelVerse speech support before making it a default. Mark unsupported routes as blocked; do not automatically replace them. Do not mark the issue live-ready until this evidence exists.

No browser screenshot is required while API/UI behavior remains unchanged. A discovered browser-visible change requires updated scope and desktop/mobile browser checks with saved screenshot paths; pause for owner direction when screenshots contradict the agreed outcome.

## Primary references and uncertainty

Reviewed 2026-10-20, public documentation only; no authenticated/provider calls:

- [AstraFlow official CLI](https://github.com/ucloud/astraflow-cli): regional endpoints and model discovery. Actual account availability remains unknown.
- [ModelVerse Chat Completions protocol](https://github.com/UCloudDoc-Team/modelverse/blob/master/api_doc/text_api/openai_compatible.md): compatible transport; model-specific behavior still needs validation.
- [Global DeepSeek OCR documentation](https://astraflow.ucloud-global.com/en-us/docs/modelverse/api_doc/text_api/deepseek-ocr.md): base64-only example and Markdown output. Retrieved through public HTTP because the web reader could not access the Markdown page. Treat it as a candidate, not a new default.
- [ModelVerse MiniMax speech protocol](https://github.com/UCloudDoc-Team/modelverse/blob/master/api_doc/audio_api/minimax-speech.md): JSON/hex audio and business status. The example uses the mainland endpoint; global route/voice support is unverified.
- [OpenRouter speech tutorial](https://openrouter.ai/blog/tutorials/text-to-speech/): `/audio/speech`, model-specific voices, explicit output format, raw audio/error handling.
- [OpenRouter model properties](https://openrouter.ai/docs/api/api-reference/models/list-all-models-and-their-properties): modalities, supported parameters, and voices inform configuration. Catalog presence does not establish Japanese runtime quality.

## Planning verification

The planning-only turn created this document and verified branch/base and task contracts. Implementation was subsequently approved by the owner. Local implementation checks and pending live evidence are recorded in `docs/evidence/issue-34/implementation.md`.
