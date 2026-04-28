# Backend AGENTS.md

Guidelines for AI agents working on the Go backend of the Gemini OCR+Annotation PWA project.

## Project Overview

Go backend providing JSON API for a mobile-first PWA that uses OpenRouter OCR and MiniMax text/speech APIs for Japanese OCR, contextual annotations, and text-to-speech. See `../docs/rfc.md` and `../docs/prd.md` for detailed requirements.

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
- `AI_PROVIDER` - AI provider mode: `minimax` (default) or `gemini`
- `OPENROUTER_API_KEY` - OpenRouter API key for OCR when `AI_PROVIDER=minimax`
- `MINIMAX_API_KEY` - MiniMax API key for annotation and speech when `AI_PROVIDER=minimax`
- `GEMINI_API_KEY` or `GOOGLE_API_KEY` - Gemini API key when `AI_PROVIDER=gemini`
- `DB_CONNECTION_STRING` or the `POSTGRES_*` variables - PostgreSQL connection settings
- `GOOGLE_OAUTH_CLIENT_ID` - Google OAuth client ID
- `GOOGLE_OAUTH_CLIENT_SECRET` - Google OAuth client secret
- `JWT_SECRET` - JWT signing secret, at least 32 characters
- `FRONTEND_BASE_URL` - Frontend callback target for OAuth redirects

Optional (with defaults):
- `APP_ENV` - Application environment (default: `development`)
- `APP_BASE_URL` - Backend base URL (default: `http://localhost:8080`)
- `PORT` - Server port (default: `8080`)
- `UPLOAD_DIR` - Upload directory (default: `data/uploads`)
- `MAX_UPLOAD_SIZE` - Max upload size in bytes (default: `10485760` = 10MB)
- `SESSION_COOKIE_NAME` - Session cookie name (default: `sid`)
- `SESSION_SECURE` - Cookie secure flag (default: `false`)
- `REDIS_ADDR` - Redis address for OAuth state and caching (default: `localhost:6379`)
- `DEFAULT_PAGE_SIZE` - Default pagination size (default: `20`)
- `OPENROUTER_OCR_MODEL` - OCR model (default: `baidu/qianfan-ocr-fast:free`)
- `MINIMAX_TEXT_MODEL` - Annotation model (default: `MiniMax-M2.7`)
- `MINIMAX_TTS_MODEL` - Speech model (default: `speech-2.8-hd`)
- `MINIMAX_TTS_VOICE_ID` - Speech voice ID (default: `Japanese_Whisper_Belle`)
- `AI_RATE_LIMIT` - AI-backed route limit, set `0` to disable locally (default: `60`)
- `AI_RATE_LIMIT_WINDOW_SECONDS` - AI rate limit window (default: `3600`)

The flake provides developer tooling, local PostgreSQL/Redis service commands, and convenience env values. The running backend still requires OAuth, JWT, and the selected AI provider configuration from `backend/.env`.

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

### AI Provider Integration

- Use `ai.Client` interface for testability
- Pass `context.Context` for timeout/cancellation control
- Parse JSON responses carefully; handle malformed responses gracefully
- Store `model` and `prompt_version` with results for debugging

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
