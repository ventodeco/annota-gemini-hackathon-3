# AGENTS.md

This document provides guidelines for AI agents working on the Gemini OCR+Annotation PWA project.

## Project Overview

A mobile-first PWA built with Go backend (JSON API) + React frontend that uses Gemini Flash for OCR and contextual annotations of Japanese text. See `docs/rfc.md` and `docs/prd.md` for detailed requirements.

## Build Commands

**Backend:**
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

**Frontend:**
```bash
# Install dependencies
cd web && bun install

# Run dev server
cd web && bun run dev

# Build for production
cd web && bun run build

# Run tests
cd web && bun test

# Run tests with coverage
cd web && bun run test:coverage

# Run tests in watch mode
cd web && bun run test:watch
```

## Environment Variables

Required:
- `GEMINI_API_KEY` - Google Gemini API key

Optional (with defaults):
- `APP_BASE_URL` - Base URL (default: `http://localhost:8080`)
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

- Use `fmt.Errorf("message: %w", err)` for error wrapping
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
- Return HTMX fragments for partial responses, full HTML for page loads

### Database Patterns

- Use interfaces for testability (see `storage/db.go`)
- Define repository methods on the DB interface
- Use context for cancellation support
- Store timestamps as `time.Time` in models

### Gemini API Integration

- Use `gemini.Client` interface for testability
- Pass `context.Context` for timeout/cancellation control
- Parse JSON responses carefully; handle malformed responses gracefully
- Store `model` and `prompt_version` with results for debugging

### Templates

- Templates are in `web/templates/` with subdirectories: `pages/`, `fragments/`, `components/`
- Use `.html` extension
- HTMX attributes (`hx-post`, `hx-target`, etc.) for progressive enhancement
- Tailwind CSS classes for styling (CDN for MVP)

### Tailwind CSS Guidelines

- Use utility classes for all styling; avoid custom CSS when possible
- Mobile-first: use `sm:`, `md:`, `lg:`, `xl:` breakpoints for responsive design
- Common utility patterns:
  - Spacing: `p-4`, `px-6`, `py-3`, `m-2`, `gap-4`
  - Flexbox: `flex`, `flex-col`, `justify-between`, `items-center`, `gap-4`
  - Typography: `text-lg`, `font-bold`, `text-gray-700`, `leading-relaxed`
  - Colors: `bg-white`, `text-gray-900`, `border-gray-200`, `bg-blue-500`
  - Interactive: `hover:bg-blue-600`, `focus:ring-2`, `active:bg-blue-700`
  - Touch targets: minimum `p-4` (16px) for buttons on mobile
  - Rounded: `rounded-lg`, `rounded-full`
  - Shadows: `shadow-md`, `shadow-lg`
- Use `class` attribute (not `className`) in templates
- Group related classes conceptually for readability

### Frontend Patterns

- **Progressive Enhancement**: Use HTMX for dynamic behavior; ensure basic functionality works without JS
- **Loading States**: Use HTMX `hx-indicator` and `opacity-50` for loading feedback
- **Error Handling**: Show user-friendly errors in inline fragments, not full page reloads
- **Mobile UX**:
  - Large touch targets (minimum 44x44px recommended)
  - Adequate spacing between interactive elements
  - Readable font sizes (`text-base` minimum, `text-lg` for body text)
- **JavaScript**: Keep to minimum; use only for HTMX extensions or mobile-specific UX
- **PWA**: Include `manifest.webmanifest` and service worker for installability
- **Icons**: Use inline SVGs or a consistent icon library

### Testing

- Use `net/http/httptest` for HTTP handler tests
- Mock external dependencies via interfaces (see `internal/testutil/mocks.go`)
- Name test files `*_test.go`
- Test file should be in same package as code under test
- Use table-driven tests for multiple test cases

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
web/
  src/               # React source code
    components/      # React components
    pages/           # Page components
    hooks/           # Custom React hooks
    lib/             # Utilities, API client, types
    test/            # Test setup and utilities
  public/            # Static assets
  dist/              # Build output (gitignored)
  package.json       # Frontend dependencies
  vite.config.ts     # Vite configuration
docs/
  prd.md             # Product requirements
  rfc.md             # Technical architecture
  task.md            # Implementation tasks
data/
  app.db             # SQLite database (created at runtime)
  uploads/           # Uploaded images
```

### Configuration

- Use `config.Load()` at startup to read environment variables
- Call `cfg.Validate()` to check required fields
- Provide sensible defaults in `Load()`

### Concurrency

- Use goroutines for background processing (e.g., OCR processing)
- Always pass context for cancellation
- Handle goroutine panics gracefully or let them crash the process in development

### Logging

- Use standard `log` package for now
- Include correlation IDs where applicable
- Log errors with context before returning
