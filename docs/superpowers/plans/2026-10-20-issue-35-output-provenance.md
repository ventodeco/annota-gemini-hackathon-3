# Issue #35 follow-up: model and prompt provenance for saved AI output

Date: 2026-10-20. Base: `dev` at `3aa1675`. This is a key-independent requirement of #35: "Preserve existing OCR results and saved annotations. Track model metadata needed to explain future output differences."

## User story

As the beta operator, when a learner's OCR text or saved explanation changes in quality after a model or prompt change, I can see which producer configuration generated each saved output. Historical rows stay untouched and explicitly unknown.

## What exists today (inspected)

- `scans`: `full_ocr_text` and `detected_language` only. There is no metadata column, so a schema change is needed. This uses migration **007**, reserved for #35; 006 is #42's quota migration.
- `annotations.nuance_data JSONB`: the frontend builds `nuance_data` from the `/v1/ai/analyze` response object (`useAnnotationDrawerFlow.ts`) and sends it back unchanged in `POST /v1/annotations`. An extra field in the analyze response therefore round-trips without a frontend change, and the existing column can hold it.
- `ai.Client` exposes no producer information. Routed calls report `CallMetadata` (including `ReportedModel`) only through the `Observe` callback. #42 is adding the request-scoped `usage.Calls(ctx)` reader for that.

## Design

**`ai.Producer` and the optional `ai.ProducerReporter` interface** (new file `internal/ai/provenance.go`):
- `Producer{Provider, Gateway, Region, Model, PromptVersion}` describes the *configured* producer of a task.
- It is implemented by the legacy MiniMax/OpenRouter client, the Gemini client and the routed client.
- `ai.ProducerFor(client, task)` returns `ok=false` for clients that don't report, such as test mocks, so the result is unknown rather than guessed.
- `ai.Client` is unchanged.

**Prompt versions** are short identifiers, bumped when a prompt changes:

| Path | Prompt version |
| --- | --- |
| Shared annotation prompt (`buildEnhancedPrompt`) | `annotation/1` |
| OpenRouter legacy OCR | `ocr-openrouter-json/1` |
| Gemini OCR | `ocr-gemini-json/1` |
| Routed OCR, JSON output | `ocr-routed-json/1` |
| Routed OCR, Markdown output | `ocr-routed-markdown/1` |

A unit test pins each prompt text to its version, so a prompt edit without a version bump fails the test.

**`models.AIProvenance`**, the persisted JSON (camelCase, like `nuance_data`):

```
{
  "version": 1,
  "provider": "minimax|gemini|routed",
  "gateway": "...",
  "region": "...",
  "model": "<configured ID>",
  "reportedModel": "<provider-reported ID, only when known>",
  "promptVersion": "..."
}
```

- Identifiers only. There are no keys, prompts, request IDs, text or usage.
- `reportedModel` is filled only when the gateway reported it.

**OCR.** `processOCR` builds provenance from `ai.ProducerFor(aiClient, "ocr")` and saves it with the text. `UpdateScanOCR` gains a `provenance *models.AIProvenance` parameter and writes migration 007's nullable `scans.ocr_provenance JSONB` in the same UPDATE. Failed OCR writes nothing.

**Annotation.**
- `AnalyzeAPI` and `AnalyzeWithLanguageAPI` add `provenance` to the response.
- `NuanceData` gains `Provenance *AIProvenance` with `json:"provenance,omitempty"`.
- On save, the server decodes the echoed `provenance` leniently, as raw JSON. It keeps the value only if it **exactly equals the server's current annotation producer**. The handler gets this through `WithAIClient`, and `reportedModel` is absent until #42.
- Anything else is dropped, never rejected. That covers malformed values, a wrong type, token-shaped keys or request IDs, text, and a producer that changed between analyze and save.
- (Review round 1 changed this from a shape-only check, which could store a token-shaped secret, and fixed the 400 response on a malformed value.)
- The stored value is therefore always a server-configured identifier set, though whether the client really obtained it from analyze is still client-asserted.

**Historical rows.** Migration 007 only adds a nullable column: no backfill, and no rewrite of `full_ocr_text` or `nuance_data`. Old rows read as unknown (NULL, or no `provenance` key).

## Dependency on #42 (not blocking the rest)

The provider-*reported* model for routed calls needs #42's `usage.Calls(ctx context.Context) []ai.CallMetadata` (owner #42, branch `backlog/issue-42`, not merged).
- **This PR:** `reportedModel` stays omitted. That is correct: it is unknown, not the configured model.
- **After #42 merges:** a follow-up in `processOCR` and `AnalyzeAPI` sets `reportedModel` from the last successful, route-approved attempt in `usage.Calls(ctx)`. #42 merged as `d1200c6`.
  - On save, an echoed `reportedModel` is kept only if `ai.ReportedModelAllowed` accepts it, meaning it is the selected model or a documented alias. Otherwise only that field is dropped.
- **Legacy and Gemini clients** emit no metadata, so their `reportedModel` stays unknown.

## Out of scope

- No frontend change; provenance rides the existing pass-through.
- Speech: audio is not persisted.
- Cost and usage persistence belongs to #42.
- No API listing fields beyond what `nuance_data` already returns.
- No model or default selection; that needs keys.

## Edge cases

- A client that isn't a `ProducerReporter` (mocks) → NULL.
- Routed Markdown OCR → its own prompt version.
- A saved annotation carrying malformed or oversized provenance → dropped, not rejected.
- Old annotations without the field → unchanged.
- OCR retry or regenerate → each successful UPDATE writes the current producer with the text.
- A failure or status-only update (`UpdateScanStatus`) leaves provenance untouched.

## Acceptance criteria

1. Migration 007 is additive and idempotent. The real-PostgreSQL migration test passes, and existing rows keep their text with NULL provenance.
2. A successful OCR stores provenance matching the configured producer: unit test via handler + mock DB, and a real-PG round-trip test.
3. The analyze response includes provenance for reporting clients; a save round-trips it inside `nuance_data` (handler tests); invalid provenance is dropped.
4. Prompt versions are pinned by a test. No secret, prompt text or source text appears in provenance.
5. `go vet ./...`, `go test ./...`, and the PostgreSQL tests with `POSTGRES_TEST_DSN` (CI runs them) all pass.

## Verification

Focused Go tests, then full `go vet`/`go test`. Run the PostgreSQL migration and persistence tests locally if a PG is available, otherwise in CI. Then a fresh Sol PR review and CI. The 30-minute test-debugging budget is cumulative across all #35 PRs; 0 minutes used so far.
