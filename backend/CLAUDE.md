# Backend AGENTS.md

Guidelines for AI agents working on the Go backend of the Gemini OCR+Annotation PWA project.

## Project Overview

Go backend providing JSON API for a mobile-first PWA that uses Gemini Flash for OCR and contextual annotations of Japanese text. See `../docs/rfc.md` and `../docs/prd.md` for detailed requirements.

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

Required:
- `GEMINI_API_KEY` - Google Gemini API key

Optional (with defaults):
- `PORT` - Server port (default: `8080`)
- `DB_PATH` - SQLite database path (default: `data/app.db`)
- `UPLOAD_DIR` - Upload directory (default: `data/uploads`)
- `MAX_UPLOAD_SIZE` - Max upload size in bytes (default: `10485760` = 10MB)
- `SESSION_COOKIE_NAME` - Session cookie name (default: `sid`)
- `SESSION_SECURE` - Cookie secure flag (default: `false`)

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
    "github.com/gemini-hackathon/app/internal/gemini"
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

```go
func (h *Handlers) CreateScan(w http.ResponseWriter, r *http.Request) {
    if r.Method != http.MethodPost {
        http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
        return
    }

    if err := r.ParseMultipartForm(h.config.MaxUploadSize); err != nil {
        http.Error(w, "Failed to parse form", http.StatusBadRequest)
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

### Database Patterns

- Use interfaces for testability (see `storage/db.go`)
- Define repository methods on the DB interface
- Use `context.Context` for cancellation support
- Store timestamps as `time.Time` in models

### Gemini API Integration

- Use `gemini.Client` interface for testability
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
    gemini/          # Gemini API client (interface + implementation)
    handlers/        # HTTP handlers (JSON API)
    middleware/      # Session, logging middleware
    models/          # Data models
    storage/         # SQLite storage (interface + implementation)
    testutil/        # Test helpers and mocks
  migrations/        # SQLite schema migrations
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
