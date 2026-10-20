# Backend AGENTS.md

Guidelines for AI agents working on the Go backend of the Gemini OCR+Annotation PWA project.

## Project Overview

Go 1.27.3 backend providing JSON API for a mobile-first PWA that uses OpenRouter OCR and MiniMax text/speech APIs for Japanese OCR, contextual annotations, and text-to-speech. See `../docs/rfc.md` and `../docs/prd.md` for detailed requirements.

## Agent-mandated rules (read first)

- **Superpowers workflow**: Before substantive backend work, read and follow the root `using-superpowers` workflow. Use process skills before implementation skills when they apply.
- **DRY / KISS**: Keep handlers and services small, explicit, and easy to test. Avoid drive-by refactors and extract helpers only when they remove real duplication or clarify a stable boundary.
- **Nix development**: If the user is working in the Nix flake environment, first run `nix develop` from the repository root, start local PostgreSQL/Redis with `dev-services start`, then run the documented `go` and `bun` commands inside that shell.

## Build Commands

```bash
# Run all tests
cd backend && go test ./...

# Run tests with race detector
cd backend && go test -race ./...

# Run a single test
cd backend && go test -run TestFunctionName ./internal/handlers

# Run tests with verbose output
cd backend && go test -v ./...

# Build the application
cd backend && go build -o ../server ./cmd/server

# Run the application (from root)
./server

# Run go vet
cd backend && go vet ./...

# Run go fmt
cd backend && go fmt ./...
```

## Environment Variables

Use `backend/.env.example` and `internal/config/config.go` as the source of truth.

Required:
- `AI_PROVIDER` - AI provider mode: `minimax` (legacy default), `gemini`, or `routed`
- `OPENROUTER_API_KEY` - OpenRouter API key for OCR when `AI_PROVIDER=minimax`
- `MINIMAX_API_KEY` - MiniMax API key for annotation and speech when `AI_PROVIDER=minimax`
- `GEMINI_API_KEY` or `GOOGLE_API_KEY` - Gemini API key when `AI_PROVIDER=gemini`
- `AI_ROUTING_CONFIG` - Required only in routed mode: JSON path, absolute or relative to backend cwd. See `../docs/ai-routing.md` for schema, active-key validation, timeout/retry rules, and qualification gates. Routed mode ignores legacy AI keys unless an active profile explicitly references them.
- `DB_CONNECTION_STRING` or the `POSTGRES_*` variables - PostgreSQL connection settings
- `CLERK_SECRET_KEY` - Clerk secret key (`sk_test_`/`sk_live_`), backend only
- `CLERK_PUBLISHABLE_KEY` - Public Clerk key; the accepted session-token issuer is derived from it
- `FRONTEND_BASE_URL` - Frontend origin, default Clerk authorized party

Optional (with defaults):
- `APP_ENV` - Application environment (default: `development`)
- `APP_BASE_URL` - Backend base URL (default: `http://localhost:8080`)
- `PORT` - Server port (default: `8080`)
- `UPLOAD_DIR` - Upload directory (default: `data/uploads`)
- `MAX_UPLOAD_SIZE` - Max upload size in bytes (default: `10485760` = 10MB)
- `SESSION_COOKIE_NAME` - Session cookie name (default: `sid`)
- `SESSION_SECURE` - Cookie secure flag (default: `false`)
- `CLERK_AUTHORIZED_PARTIES` - Comma-separated frontend origins accepted in the token `azp` claim (default: `FRONTEND_BASE_URL`)
- `CLERK_JWT_KEY` - Optional PEM public key for networkless session verification
- `REDIS_ADDR` - Redis address, required runtime dependency (default: `localhost:6379`)
- `DEFAULT_PAGE_SIZE` - Default pagination size (default: `20`)
- `OPENROUTER_OCR_MODEL` - OCR model (default: `baidu/qianfan-ocr-fast:free`; no active OpenRouter provider as of 2026-10-20 — set a qualified model, see [`../docs/evidence/issue-35/model-matrix-2026-10-20.md`](../docs/evidence/issue-35/model-matrix-2026-10-20.md))
- `MINIMAX_TEXT_MODEL` - Annotation model (default: `MiniMax-M2.7`; not documented on MiniMax's Anthropic-compatible API as of 2026-10-20)
- `MINIMAX_TTS_MODEL` - Speech model (default: `speech-2.8-hd`)
- `MINIMAX_TTS_VOICE_ID` - Speech voice ID (default: `Japanese_Whisper_Belle`; not in the published system voice list as of 2026-10-20)
- `AI_RATE_LIMIT` - AI-backed route limit, set `0` to disable locally (default: `60`)
- `AI_RATE_LIMIT_WINDOW_SECONDS` - AI rate limit window (default: `3600`)
- `USAGE_QUOTA_OCR_PAGES_PER_DAY` / `USAGE_QUOTA_EXPLANATIONS_PER_DAY` / `USAGE_QUOTA_SPEECH_CHARS_PER_DAY` - Persistent per-user daily operation caps (provisional defaults `20` / `100` / `3000`, pending owner approval; `0` disables)
- `USAGE_BUDGET_GLOBAL_OCR_PAGES_PER_DAY` / `USAGE_BUDGET_GLOBAL_EXPLANATIONS_PER_DAY` / `USAGE_BUDGET_GLOBAL_SPEECH_CHARS_PER_DAY` - Global daily operation caps (provisional defaults `500` / `2500` / `75000`)
- `USAGE_SPEECH_MAX_CHARS_PER_REQUEST` - Largest single speech selection (default: `500`)

Paid AI handlers must admit work through `usage.Meter` (reserve before the provider call, `Finish`/`Release` exactly once, classify from structured `ai` metadata only). See `../docs/usage-quotas.md`.

The flake provides developer tooling, local PostgreSQL/Redis service commands, and convenience env values. The running backend still requires Clerk keys and the selected AI provider configuration from `backend/.env`. See `../docs/clerk-auth.md` for authentication setup, identity mapping, and deletion rules.

## Code Style Guidelines

### Imports

Group imports in this order:
1. Standard library
2. Third-party packages
3. Internal packages (relative imports like `./internal/...`)

```go
import (
    "context"
    "fmt"
    "net/http"

    "github.com/gemini-hackathon/app/internal/config"
    "github.com/gemini-hackathon/app/internal/ai"
)
```

### Naming Conventions

- **Packages**: lowercase, single word or short phrase
- **Types/Structs**: PascalCase (e.g., `Scan`, `OCRResult`)
- **Variables/Fields**: camelCase (e.g., `scanID`, `createdAt`)
- **Constants**: PascalCase or SCREAMING_SNAKE_CASE for config values
- **Interfaces**: PascalCase, often with `er` suffix (e.g., `Client`, `Storage`)
- **Private fields**: leading underscore NOT used; use short descriptive names (e.g., `c *client`)

### Error Handling

- Use `fmt.Errorf("context: %w", err)` for error wrapping
- Use `errors.Is(err, targetErr)` and `errors.As(err, &target)` for error checking
- Return errors, don't panic (except unrecoverable situations)
- Check errors immediately, never ignore
- Return early on errors in handlers
- Log errors at the appropriate level before returning
- Validate inputs early and return user-friendly errors
- For `/v1/*` JSON APIs and middleware used by those APIs, prefer `httputil.WriteJSONError` over `http.Error` so clients receive a consistent `ErrorResponse`
- Shared JSON helpers should either handle `json.Encoder.Encode` errors or make intentional ignored errors explicit

```go
func (h *Handlers) CreateScan(w http.ResponseWriter, r *http.Request) {
    if r.Method != http.MethodPost {
        httputil.WriteJSONError(w, http.StatusMethodNotAllowed, "Method not allowed")
        return
    }

    if err := r.ParseMultipartForm(h.config.MaxUploadSize); err != nil {
        httputil.WriteJSONError(w, http.StatusBadRequest, "Failed to parse form")
        return
    }
    // ...
}
```

### HTTP Handlers

- Use method receivers on `*Handlers` struct
- Accept `http.ResponseWriter` and `*http.Request`
- Check request method explicitly
- Use `http.Status*` constants for status codes
- Return JSON responses for API endpoints
- Keep pagination behavior explicit: `ParsePagination` should either validate parse failures as `400` responses or document intentional coercion to defaults in the helper contract

### Database Patterns

- Use interfaces for testability (see `storage/db.go`)
- Define repository methods on the DB interface
- Use `context.Context` for cancellation support
- Store timestamps as `time.Time` in models
- Production migrations target PostgreSQL. Do not assume SQLite compatibility for `migrations/*.sql`.
- `RunMigrations` is PostgreSQL-only and runs each migration with its ledger row in one transaction under an advisory lock. Keep migrations transactional (no `CREATE INDEX CONCURRENTLY`) and test runner changes with `POSTGRES_TEST_DSN`.

### AI Provider Integration

- Keep `ai.Client` and legacy `NewClient` compatible. Use `ai.NewRoutedClient` for validated ModelVerse/OpenRouter task profiles. `internal/ai/routing.go` owns routing schema/validation; the AI package must not import application config.
- Routed profiles are immutable after construction. Only active profiles need secrets; never serialize resolved keys. Do not log raw upstream bodies, document text, images, or full config.
- Keep chat and speech protocols explicit, disable silent substitution, and observe each bounded attempt with usage fields left unknown when absent. See `../docs/ai-routing.md`.

- Use `ai.Client` interface for testability
- Pass `context.Context` for timeout/cancellation control
- Parse JSON responses carefully; handle malformed responses gracefully
- Saved OCR and annotation output carry `models.AIProvenance`: the configured provider, gateway, region, model and prompt version, taken from `ai.ProducerFor`. Bump the matching `PromptVersion*` constant in `internal/ai/provenance.go` whenever a prompt changes; `TestPromptVersionsArePinned` enforces this.

### Testing

- Use table-driven tests for multiple test cases
- Use `net/http/httptest` for HTTP handler tests
- Mock external dependencies via interfaces (see `internal/testutil/mocks.go`)
- Name test files `*_test.go`
- Test file should be in same package as code under test
- Test error scenarios explicitly

Example table-driven test:
```go
func TestAdd(t *testing.T) {
    tests := []struct {
        name     string
        a, b     int
        expected int
    }{
        {"positive numbers", 2, 3, 5},
        {"negative numbers", -2, -3, -5},
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            result := Add(tt.a, tt.b)
            if result != tt.expected {
                t.Errorf("Add(%d, %d) = %d; want %d", tt.a, tt.b, result, tt.expected)
            }
        })
    }
}
```

### Project Structure

```
backend/
  cmd/server/        # Main entry point
  internal/
    config/          # Configuration loading
    ai/              # OpenRouter/MiniMax AI client (interface + implementation)
    handlers/        # HTTP handlers (JSON API)
    middleware/      # Session, logging middleware
    models/          # Data models
    storage/         # PostgreSQL storage (interface + implementation)
    testutil/        # Test helpers and mocks
  migrations/        # PostgreSQL schema migrations
  go.mod             # Go module definition
  go.sum             # Go module checksums
```

### Configuration

- Use `config.Load()` at startup to read environment variables
- Call `cfg.Validate()` to check required fields
- Provide sensible defaults in `Load()`

### Concurrency

- Use goroutines for background processing (e.g., OCR processing)
- Always pass `context.Context` for cancellation
- Avoid raw `context.Background()` in request-started background work unless the lifecycle is intentionally detached and documented; prefer request, timeout, or shutdown-aware contexts
- Propagate errors from goroutines using channels
- Handle goroutine panics gracefully or let them crash the process in development

Example error channel pattern:
```go
errCh := make(chan error, 1)
go func() {
    errCh <- doSomething()
}()
if err := <-errCh; err != nil {
    log.Printf("Error in goroutine: %v", err)
}
```

### Logging

- Use standard `log` package for now
- Include correlation IDs where applicable
- Log errors with context before returning
- Middleware logs should preserve enough response information for debugging without leaking secrets or large payloads

## Issue Tracking Workflow

All work must be tracked via GitHub Issues using the GitHub CLI (`gh`).

See the complete workflow documentation: [`docs/github-workflow.md`](../docs/github-workflow.md)

### Quick Start

```bash
# Create issue for your work
gh issue create --label backend --title "[TASK] Description"

# Work on dev (or create feature branch - see workflow doc)
git checkout dev
git pull origin dev

# Commit with issue reference
git commit -m "feat: description (#42)"

# Push
git push origin dev

# Close issue when done
gh issue close #42
```

### Backend-Specific Labels

- `backend` - Backend related
- `api` - API changes
- `database` - Database/schema changes
- `ai` - AI provider integration
- `security` - Security-related
- `performance` - Performance optimization
