# Issue #35 model matrix — checked 2026-10-20

**Status: candidate research only. No route is live-qualified.** No provider keys were available, so no inference call was made. Revisit by **2026-11-10**, before the Gemini TTS shutdown on 2026-11-17, or as soon as keys arrive.

Evidence levels, kept separate:

- **Listed** — the ID is in the gateway's public model catalog or official model documentation.
- **Served** — the gateway shows at least one active provider endpoint. Checked only where it is publicly readable.
- **Adapter-compatible** — documented inputs, outputs and formats match the request path ANNOTA actually uses. This is a documentation claim, not a test result.
- **Qualified** — passed the targets in `eval-set/README.md` on a keyed run. Pending for every row.

The raw catalog fields used here are in `catalog-snapshot-2026-10-20.json`, taken from public endpoints at 2026-10-20T05:15Z. Prices are list prices in USD exactly as each source states them. Billed usage must be compared live, because reasoning or thinking tokens and audio-token counts are not visible in list rates.

## Current defaults

| Mode / task | Configured ID | Where | Listed | Served / compatible | Finding |
| --- | --- | --- | --- | --- | --- |
| `minimax` OCR (default mode) | `baidu/qianfan-ocr-fast:free` (OpenRouter) | `config.go` `OPENROUTER_OCR_MODEL`, `client.go` `applyDefaults`, `.env.example`, README, RUN, `backend/AGENTS.md`, `backend/docs/IMPLEMENTATION_PLAN.md` | **No** (absent from `/api/v1/models`) | **No**: `/models/baidu/qianfan-ocr-fast:free/endpoints` returns `endpoints: []` | **Unavailable.** OCR fails with this default. Set a qualified `OPENROUTER_OCR_MODEL`. |
| `minimax` annotation | `MiniMax-M2.7` via `https://api.minimax.io/anthropic/v1/messages` | `config.go`, `client.go`, docs | Yes (MiniMax, OpenRouter, ModelVerse catalogs) | **Not documented on this path**: the [Anthropic-compatible API page](https://platform.minimax.io/docs/api-reference/text-anthropic-api) says it "currently only supports" `MiniMax-M3.1-Flash` and `MiniMax-M3`, and routes other models to the standard MiniMax API | **Unverified; likely to fail.** A keyed call is needed to confirm. |
| `minimax` speech | `speech-2.8-hd` with voice `Japanese_Whisper_Belle` | `config.go`, `client.go`, docs | Model yes ([MiniMax models](https://platform.minimax.io/docs/guides/models-intro)); voice **not in** the [published system voice list](https://platform.minimax.io/docs/faq/system-voice-id) (15 `Japanese_*` IDs) | Model documented. The voice can only be confirmed with the keyed Get Voice API. | **Voice unverified.** It is not in the published list, but it may still exist. |
| `gemini` annotation/OCR | `gemini-2.5-flash` (Google direct) | `client.go` | Yes | Google [deprecations](https://ai.google.dev/gemini-api/docs/deprecations): "No shutdown date announced" | Usable for now. OpenRouter's `google/gemini-2.5-flash` shows `expiration_date: 2026-10-20`, but that applies only to OpenRouter, not Gemini mode. |
| `gemini` speech | `gemini-2.5-flash-preview-tts` | `client.go` (hard-coded) | Yes | **Shutdown 2026-11-17.** Replacements: `gemini-3.8-flash-tts` / `gemini-3.8-flash-lite-tts` | **Migrated in this PR** to `gemini-3.8-flash-tts` (see below). |
| `routed` | none; operator file | `docs/ai-routing.md` | — | — | No defaults. Candidate file: `candidate-routing.json`. |

### Gemini-mode TTS migration (code change)

Per the [3.8 Flash TTS migration guide](https://ai.google.dev/gemini-api/docs/models/gemini-3.8-flash-tts):

- The model treats input as a verbatim transcript, and inline directions "may be spoken aloud". The previous prompt embedded instructions and up to 400 characters of the user's context, so a plain ID swap would have read them aloud. Only the selected text is now sent. Delivery style goes in `Part.SpeechMetadata.Style`, which is supported by the pinned `google.golang.org/genai v1.75.0`. `Kore` and `ja-JP` are kept.
- Unary output defaults to WAV with a RIFF header. The code now passes RIFF WAV through, and still wraps headerless `audio/l16`/`pcm` as WAV, using the declared `rate` when present.

Language support: Japanese is listed for both 3.8 tiers in the [speech guide](https://ai.google.dev/gemini-api/docs/speech-generation). The flash tier lists 130 languages and lite lists 101, and `Kore` is in the prebuilt voice list. Input limit: 8,192 tokens; output limit: 16,384.

Cost from [Gemini pricing](https://ai.google.dev/gemini-api/docs/pricing), paid tier:

| Model | Input | Output | Approx. per 10 s audio |
| --- | --- | --- | --- |
| `gemini-2.5-flash-preview-tts` (old) | $0.50/1M text tokens | $10/1M audio tokens | — |
| `gemini-3.8-flash-tts` until 2026-12-31 | $0.50/1M | $9/1M | ≈ $0.00225 |
| `gemini-3.8-flash-tts` from 2027-01-01 | $1.00/1M | $18/1M | ≈ $0.0045 |
| `gemini-3.8-flash-lite-tts` | $0.50/1M (then $1.00) | $6/1M (then $12) | — |

The free tier is marked "used to improve our products", so learner text must not be sent on the free tier.

Why flash, not lite: Google names flash for "difficult pronunciation" and positions lite for high-volume read-aloud. Lite is the cheaper alternative to compare in the listening pass. Status: adapter-compatible by documentation, **not qualified**.

## Candidates for live qualification

The selection rule: listed and served, a documented request shape that matches the adapter, stable rather than preview where possible, and low list cost. Release recency alone was not a criterion. Every row is **qualification pending**.

### OCR

| Route | Region | Listed / served | Inputs → outputs | Structured output | List price (USD) | Limits | Notes |
| --- | --- | --- | --- | --- | --- | --- | --- |
| OpenRouter `google/gemini-3.5-flash-lite` | global | Yes / 8 endpoints (Google, Google AI Studio) | text, image, video, file, audio → text | `structured_outputs` on all endpoints → `json_schema`, `ocr_output=json` | catalog $0.30/1M in, $2.50/1M out, $0.30/1M image tokens; endpoint list also shows a $0.15/$1.25 tier | 1,048,576 ctx; 65,536 max out | Google: released 2026-07-21, no shutdown. It is the named replacement for `gemini-3.1-flash-lite` (shutdown 2027-05-07). **First-pass candidate.** |
| OpenRouter `qwen/qwen3-vl-30b-a3b-instruct` | global | Yes / 3 endpoints (DeepInfra fp8, Novita bf16, SiliconFlow fp8) | text, image → text | `structured_outputs` on all three | $0.15/1M in, $0.60/1M out (catalog) | 262,144 ctx; 16,384 max out (catalog) | Cheapest served VLM candidate. Quantisation differs by provider, and failover is disabled in the adapter, so quality can vary with the provider chosen. |
| ModelVerse `deepseek-ai/DeepSeek-OCR` | `api-us-ca.umodelverse.ai` | **Documented** in the [global doc](https://astraflow.ucloud-global.com/en-us/docs/modelverse/api_doc/text_api/deepseek-ocr.md) (US-CA, "currently available for free", `max_tokens` ≤ 8192, base64 data-URL images only). **Not listed** in the unauthenticated `/v1/models` of Singapore, US-CA, Frankfurt or China. | image + prompt → markdown text | none documented → `json_mode=prompt`, `ocr_output=markdown` | free (documented; not permanent) | 8,192 max tokens | Benchmark required by the issue. Availability needs a keyed `/v1/models` check. Singapore is not assumed. Free capacity is not assumed. |

### Annotation (structured JSON explanation)

| Route | Region | Listed / served | Structured output | List price (USD) | Notes |
| --- | --- | --- | --- | --- | --- |
| OpenRouter `google/gemini-3.5-flash-lite` | global | Yes / 8 endpoints | `json_schema` | as above | **First-pass candidate.** The same model for OCR and annotation keeps one key and one bill. |
| ModelVerse `MiniMax-M3` | Singapore | Listed in the SG/US-CA/FRA/CN public catalogs | Not documented for ModelVerse → `prompt` | not captured: the [price page](https://astraflow.ucloud-global.com/en-us/modelverse/prices) renders client-side | On MiniMax direct, thinking is off by default ([API notes](https://platform.minimax.io/docs/api-reference/text-anthropic-api)). Thinking behaviour on ModelVerse is unknown. OpenRouter also serves `minimax/minimax-m3` (14 endpoints; only some advertise `structured_outputs`). |
| ModelVerse `MiniMax-M2.7` | Singapore | Listed | `prompt` | not captured | Continuity with the current legacy default. Its reasoning cannot be disabled (MiniMax docs), so billed output tokens can exceed the visible JSON. OpenRouter `minimax/minimax-m2.7` lacks `structured_outputs`. |

### Japanese speech (routed mode supports mp3 only)

| Route | Region | Listed / served | Japanese voice | Format | List price | Notes |
| --- | --- | --- | --- | --- | --- | --- |
| ModelVerse `speech-2.8-hd` | Singapore | Listed (all 4 regions) | Published MiniMax IDs, e.g. `Japanese_KindLady`, `Japanese_CalmLady`; `language_boost: Japanese` supported ([ModelVerse speech](https://github.com/UCloudDoc-Team/modelverse/blob/master/api_doc/audio_api/minimax-speech.md)) | mp3 (default), hex | not captured | **First-pass candidate.** Text limit < 10,000 chars. The `whisper` emotion is unsupported on 2.8. The voice ID must be confirmed on the keyed route. |
| ModelVerse `speech-2.8-turbo` | Singapore | Listed | as above | mp3 | not captured | Speed/cost alternative. |
| OpenRouter `google/gemini-3.8-flash-tts` | global | Yes / 2 endpoints | prebuilt `Kore` (language detected from text) | `/audio/speech` `response_format` `mp3` or `pcm` ([OpenRouter TTS](https://openrouter.ai/docs/guides/overview/multimodal/tts)) | $0.50/1M in, $9/1M out (catalog) | Same model as Gemini mode. Pronunciation can be compared across both paths. |
| OpenRouter `minimax/speech-2.8-hd` | global | Yes / 1 endpoint | Catalog lists 45 voices, **none Japanese**; the description says arbitrary MiniMax voice IDs are accepted | mp3 | $0.0001 per input character (OpenRouter prices most TTS per character) | Matrix only. Japanese voice support on this path is unproven. |

## Edge-case risks to watch during qualification

- **Regional catalog mismatch:** DeepSeek-OCR is documented but not publicly listed. Gemini 2.5 Flash expires on OpenRouter but not on Google. Record the exact host/region for every result.
- **Free quota and model removal:** `:free` and "currently free" routes can vanish, as `qianfan-ocr-fast:free` did. Never make a free-only route the sole default.
- **Truncation/empty output:** every chat candidate's max output is far above the dense sample. Blank-page hallucination is tested by ocr-06.
- **Text PDFs** need no OCR; PDF text extraction is outside this model selection.
- **Long speech:** sp-5 (~290 chars) is well under the documented limits (MiniMax < 10,000 chars; OpenRouter transcript ≤ 10,000; Gemini ≤ 8,192 input tokens).
- **Voice/pronunciation changes:** Gemini 3.8 no longer receives context, so heteronyms such as 今日 rely on the model alone. sp-4 records this.
- **Provider request IDs:** `qualify-routes` redacts and fails IDs that echo content. Record the ID format observed per route for #34.
- **Reasoning budget:** `MiniMax-M2.7` always thinks. The candidate file gives it `max_output_tokens: 16384` so that reasoning cannot truncate the JSON. Record billed output tokens, and treat truncation as a configuration question before disqualifying the route.
- **Gemini 3.8 request contract:** confirm on the first keyed Gemini-mode call that `generateContent` accepts `languageCode: ja-JP` together with `speechMetadata`, and that the returned MIME is `audio/wav`.

## Results

| Date | Task | Route | CER / schema / reading | p50 latency | Billed usage / cost | Request-ID format | Outcome |
| --- | --- | --- | --- | --- | --- | --- | --- |
| pending | — | — | — | — | — | — | Keys not yet provided |

## Remaining for #35 to close

1. Private keys for the chosen gateways. Then run the procedure in `eval-set/README.md` with one pass per candidate and fill **Results**.
2. Pick a route per task only from passing results, then update the routed operator configuration and the legacy defaults with that evidence. This includes replacing the unserved `qianfan` OCR default.
3. Listening notes for speech. Add user-approved blurry/skewed photo samples.
4. Follow-up: per-scan/annotation model + prompt-version metadata, which is a storage change that overlaps #43. Local output capture for scoring is available via `qualify-routes -live -output-dir`.
