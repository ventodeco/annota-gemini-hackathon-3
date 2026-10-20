# Issue 34 local implementation evidence

Date: 2026-10-20. Branch: `feat/issue-34-ai-gateway-routing`. Base: `origin/dev` at `39349ec`.

## Delivered

Explicit `AI_PROVIDER=routed` configuration validates named OpenRouter/ModelVerse profiles and independent OCR, annotation, and speech routes. Existing Gemini/MiniMax modes, public API responses, authentication, and rate-limit wiring remain intact. Configuration docs: [gateway routing](../../ai-routing.md).

Local HTTP tests cover both chat gateways, structured annotation/OCR and explicit Markdown OCR, task/credential separation, invalid config, upstream refusal/truncation, quota/auth/throttling errors including HTTP-200 business errors, bounded retries/deadlines/body reads, redirect rejection, missing accounting metadata, and safe errors/observations. Speech contracts cover ModelVerse JSON/hex and OpenRouter raw MP3, complete business status, format validation, and unchanged handler output. New tests were run before their implementations; additional review regressions demonstrated the embedded-quota and empty-audio defects before correction.

## Passing commands

Run from `backend/` using the existing `/tmp/annota-runtime-upgrade/go/bin/go` toolchain (`go version go1.27.3 darwin/arm64`):

- `rtk proxy /tmp/annota-runtime-upgrade/go/bin/go test -timeout 60s ./internal/handlers ./internal/config ./cmd/server ./internal/ai`
- `rtk proxy /tmp/annota-runtime-upgrade/go/bin/go test -race -timeout 60s ./internal/ai`
- `rtk proxy /tmp/annota-runtime-upgrade/go/bin/go test -count=1 -timeout 60s ./...`
- `rtk proxy /tmp/annota-runtime-upgrade/go/bin/go vet ./...`
- `rtk proxy /tmp/annota-runtime-upgrade/go/bin/go build -o /tmp/annota-issue-34-server ./cmd/server`

From the root, `rtk proxy git diff --check` passed. The synthetic MP3 fixture was decoded with `rtk proxy /usr/bin/afconvert backend/internal/ai/testdata/silence.mp3 /tmp/annota-issue-34-fixture.wav -f WAVE -d LEI16`, producing 4,608 PCM frames at 44,100 Hz, two channels.

The initial baseline AI/config tests passed. A cancellation test fixture initially blocked `httptest.Server.Close`; the stack trace located a fixture handler waiting on request cancellation with an unread POST body. Replaced that fixture synchronization with an explicit release channel while retaining the actual client-deadline/request-count assertions. The focused suite and race check then passed. No assertion was weakened or production error hidden.

## Scope adjustments

`buildAIClient` stays in `cmd/server/main.go` rather than a separate production file because the existing documented developer commands and root `dev` script run that single file. This preserves those commands without unrelated script/flake edits. Its tests remain in `cmd/server/ai_client_test.go`.

Kept existing prompt/normalization helpers and legacy transport behavior. Routed clients add safe cause unwrapping to `ProviderError` for cancellation/deadline checks. No dependency upgrades, schema migrations, frontend changes, or new external services were added.

## Pending checks

- No real provider calls were made. Account/model/voice availability, regional access, Japanese accuracy/pronunciation, latency, and actual cost are unqualified. Coordinate bounded calls with #35 after private credentials and explicit routes are supplied; record actual evidence in `gateway-qualification.md`. Synthetic declarations are not production defaults.
- `POSTGRES_TEST_DSN` was unset, so the existing opt-in production PostgreSQL migration test was skipped by the suite. This change does not alter database code/migrations.
- Docker is unavailable locally; final-image smoke was not run. Packaging inputs were not changed. No deployment or production configuration was changed.
- Source/API contracts are unchanged, so browser screenshots are not required for this backend-only delivery.

The original checkout's local documentation/research edits were preserved by using the separate T3 worktree. No issue comments or external messages were sent.
