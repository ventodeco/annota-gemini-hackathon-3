# Issue #35 — Refresh and qualify AI models for the Japanese learning beta

Date: 2026-10-20. Branch: `backlog/issue-35` (from `origin/dev` at `c72c695`, refreshed to `1a51fa8` after #34's PR #63 merged).

## User story

As a Japanese learner, I want accurate reading assistance from supported models so that I can understand material reliably while beta costs remain controlled.

## What this delivery does and does not claim

Three evidence levels are kept separate throughout:

1. **Documented / catalog-listed** — a provider page or public catalog lists the ID.
2. **Served and adapter-compatible** — the gateway has an active endpoint, and the documented request/response shape matches the exact code path ANNOTA uses.
3. **Live-qualified** — a bounded, keyed run on the fixed Japanese evaluation set met the targets below, with latency/usage/cost recorded.

No API keys are available in this session. This delivery reaches levels 1–2 only. It must not describe any route as qualified, and #35 stays open for level 3.

## Findings that drive scope (checked 2026-10-20, primary sources)

Full matrix and source links: `docs/evidence/issue-35/model-matrix-2026-10-20.md`.

- **Legacy OCR default unserved.** `baidu/qianfan-ocr-fast:free` (`config.go`, `client.go`, `.env.example`, README, RUN, backend AGENTS, IMPLEMENTATION_PLAN) is absent from OpenRouter `/api/v1/models`, and `/models/{id}/endpoints` returns `endpoints: []`. It is listed-but-unserved, not usable.
- **Legacy annotation default undocumented on its path.** MiniMax's Anthropic-compatible API page says it "currently only supports" `MiniMax-M3.1-Flash` and `MiniMax-M3`, and points other models to the standard API. ANNOTA calls `MiniMax-M2.7` through `/anthropic/v1/messages`. A keyed call is needed to confirm actual rejection.
- **Legacy voice not in the published list.** `Japanese_Whisper_Belle` is absent from MiniMax's published system voice list (15 `Japanese_*` IDs). The page defers to the keyed Get Voice API, so this is "not published; keyed check pending", not "removed".
- **Gemini-mode TTS has a hard deadline.** Google's deprecations page (last updated 2026-10-20) lists the hard-coded `gemini-2.5-flash-preview-tts` shutdown as **2026-11-17**, with replacements `gemini-3.8-flash-tts` / `gemini-3.8-flash-lite-tts`. The 3.8 migration guide changes the request contract:
  - input text is a verbatim transcript, so inline directions "may be spoken aloud";
  - style belongs in `speech_metadata`;
  - unary output defaults to WAV with a RIFF header instead of raw PCM.

  The current Gemini prompt embeds instructions plus up to 400 characters of context. A plain ID swap would read them aloud, including user document context.
- **Gemini 2.5 Flash.** The Google API lists no shutdown for direct Gemini-mode `gemini-2.5-flash`. OpenRouter's catalog shows `expiration_date: 2026-10-20` for `google/gemini-2.5-flash`. That applies only to the OpenRouter route and does not affect Gemini mode.
- **DeepSeek-OCR.** The global AstraFlow doc names `deepseek-ai/DeepSeek-OCR` on `api-us-ca.umodelverse.ai` (free, `max_tokens` ≤ 8192). It is absent from the unauthenticated `/v1/models` lists of Singapore, US-CA, Frankfurt and China. A key-scoped listing may differ.
- **Model metadata.** Scans and annotations persist no model/prompt metadata (no columns in `migrations/`, no fields in `internal/models`). Adding it is a storage-contract change that overlaps #43 storage/migrations. It is flagged to the coordinator as a follow-up and is not done here.

## Scope (this PR)

1. **Dated model matrix** (`docs/evidence/issue-35/model-matrix-2026-10-20.md`):
   - per task, the current default plus two or three candidates;
   - gateway-specific ID, region, inputs/outputs, catalog status, active endpoints, structured-output support, listed price with its raw unit, limits, deprecation;
   - adapter compatibility and qualification status (all `pending`);
   - revisit date and links to primary sources.
2. **Fixed Japanese evaluation set and targets** (`docs/evidence/issue-35/eval-set/`):
   - original, self-written synthetic text, so there are no copyright or privacy concerns;
   - HTML sources rendered to PNG by a script using headless Chrome;
   - ground-truth transcripts, annotation selections, and speech utterances;
   - numeric targets fixed before any paid call;
   - real blurry/skewed photos are listed as pending user-approved captures, not faked.
3. **Candidate routing file** (`docs/evidence/issue-35/candidate-routing.json`). An exact, schema-valid routed configuration for the first qualification run. It is labelled candidate-only and is not a default, and a Go test guards it against schema drift.
4. **Gemini-mode TTS migration** (the only default/model code change):
   - `gemini-2.5-flash-preview-tts` → `gemini-3.8-flash-tts`;
   - send only the selected text as the transcript, with the delivery style in `Part.SpeechMetadata.Style` (supported in the pinned `google.golang.org/genai v1.75.0`);
   - keep `Kore` and `ja-JP`;
   - audio handling: pass RIFF/WAV through, and wrap `audio/l16` or `pcm` MIME types as WAV;
   - unit tests for request construction and MIME handling, with no network.

   Why the flash tier rather than lite: it is the direct successor named for the 2.5 TTS tier, and Google positions it for "difficult pronunciation". Lite is cheaper (OpenRouter lists $0.000006 vs $0.000009 per output unit) and goes into the listening comparison. Status: documented-compatible, not live-qualified.
5. **Documentation corrections.** Mark the legacy minimax-mode defaults as "unavailable/unverified as of 2026-10-20 — set a qualified model" in README, RUN, `.env.example` and backend AGENTS. Do not change those code defaults: no candidate has quality evidence, and the coordinator/issue forbid selecting defaults without it.

## Out of scope / follow-ups

- `backend/cmd/qualify-routes` belongs to #34 (PR #63). It is not edited here. Gap: the report deliberately omits output text, so transcription errors and pronunciation cannot be scored from it. Proposed follow-up after #63 merges, owned or agreed with #34: an opt-in local-only output directory (OCR text, annotation JSON, audio), never written into the report. Raised with #34.
- Persisting model and prompt-version metadata per scan/annotation (storage; overlaps #43).
- Changing the legacy OCR, annotation, or voice defaults. Change them only after live qualification picks a replacement.
- `docs/ai-routing.md` gets only a one-line link to the matrix. PR #63 merged as `1a51fa8` before this edit, so there is no conflict.
- No frontend, auth, Nix, or UX change, so no browser screenshots are needed. This PR does not change browser-visible output.

## Edge cases covered by the evaluation set

| Case | Coverage |
|---|---|
| Vertical text (`writing-mode: vertical-rl`) | Sample |
| Furigana (`<ruby>`) | Sample |
| Mixed scripts (kanji, kana, katakana, Latin, digits, full-width punctuation) | Sample |
| Dense multi-paragraph page | Sample |
| Empty/blank image (detects hallucination) | Sample |
| Long speech text | Speech utterance (near the documented limits) |
| Reading-ambiguity words for pronunciation observation | Speech utterance |
| Blurry/skewed photos | Pending user-approved real captures |
| Multipage content, text PDFs needing no OCR, free-quota exhaustion, model removal | Matrix risks and check procedure, not samples |

## Acceptance targets (fixed before paid calls)

**OCR** (per route, 6 rendered samples):
- character error rate (CER, edit distance against ground truth after NFKC normalisation and whitespace removal) ≤ 2% on clean horizontal text;
- CER ≤ 5% on vertical, mixed-script and dense samples;
- furigana: base-text CER ≤ 5%, and ruby must not be interleaved mid-word;
- the blank image must yield empty text or an explicit error, with no invented text;
- no truncation.

**Annotation** (5 selections):
- 5/5 schema-valid through the existing normaliser;
- 5/5 kana readings correct against ground truth;
- no refusal or truncation.

**Speech** (5 utterances):
- 5/5 decode as MP3 (routed) or WAV (Gemini mode);
- a human listening pass records the reading for each utterance, and must find no misreading of the unambiguous items;
- ambiguous items are recorded as observations only.

**Latency:** OCR p50 ≤ 15 s, annotation p50 ≤ 10 s, speech p50 ≤ 5 s on the beta region.

**Cost** (provider-reported billed usage, not list rate): ≤ US$0.005 per OCR page, ≤ US$0.002 per annotation, ≤ US$0.003 per short utterance. Record unreported cost as unknown, not zero.

**Reliability:** every call runs with `max_attempts=1`. A failure is recorded and is not retried blindly.

Paid calls are bounded: one pass of the set per candidate route, with no reruns of unchanged samples. A rerun needs a concrete question, such as a prompt change.

## Files

- `docs/superpowers/plans/2026-10-20-issue-35-model-qualification.md` (this plan)
- `docs/evidence/issue-35/model-matrix-2026-10-20.md`, `catalog-snapshot-2026-10-20.json` (trimmed public catalog fields)
- `docs/evidence/issue-35/candidate-routing.json`
- `docs/evidence/issue-35/eval-set/README.md`, `samples/*.html`, `ground-truth/*.txt`, `render.sh`, rendered `images/*.png`, `score_ocr.py` (stdlib CER)
- `backend/internal/ai/client.go` (Gemini TTS), `backend/internal/ai/client_test.go`, `backend/internal/ai/routing_test.go` (candidate file guard)
- `README.md`, `RUN.md`, `backend/.env.example`, `backend/AGENTS.md`, `docs/ai-routing.md` (default-status notes and matrix link)

## Verification

- `cd backend && go test ./internal/ai/... && go vet ./... && go build ./...` using a Go 1.27.3 toolchain, plus the CI workflow on the PR.
- Render script: confirm the PNGs exist and visually spot-check them (vertical and ruby rendered).
- Docs: diff/content review, and confirm every model ID in the matrix matches the captured catalog JSON.
- Live: none, because no keys are available. The exact keyed procedure is in `eval-set/README.md`.
- The shared 30-minute test-debugging budget applies; #43 already spent about 16.5 minutes of it.
