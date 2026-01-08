# Gemini Hackathon OCR App

A mobile-first Progressive Web App (PWA) for uploading book page images, extracting Japanese text via OCR, and getting contextual annotations. Built with Go, HTMX, and Gemini Flash API.

## Features

- **Image Upload**: Upload JPEG, PNG, or WebP images of book pages
- **OCR Processing**: Extract Japanese text using Gemini Flash vision API
- **Text Annotation**: Select text to get contextual explanations including:
  - Meaning
  - Usage examples in professional/work context
  - When to use
  - Word breakdown
  - Alternative meanings
- **Session-based**: Anonymous sessions via cookies (no login required for MVP)
- **PWA Support**: Installable as a Progressive Web App

## Prerequisites

- **Go 1.25.x** or later ([download](https://go.dev/dl/))
- **SQLite** (bundled with macOS, or install separately)
- **Gemini API Key** ([get one here](https://makersuite.google.com/app/apikey))

## Quick Start

1. **Clone and navigate to the project**:
   ```bash
   cd gemini-hackathon
   ```

2. **Set up environment variables**:
   ```bash
   cp .env.example .env
   # Edit .env and add your GEMINI_API_KEY
   ```

3. **Install dependencies**:
   ```bash
   go mod tidy
   ```

4. **Run the server**:
   ```bash
   go run cmd/server/main.go
   ```

5. **Open in browser**:
   Navigate to `http://localhost:8080`

## Environment Variables

See `.env.example` for all available configuration options:

- `GEMINI_API_KEY` (required): Your Gemini API key
- `APP_BASE_URL`: Base URL for the application (default: `http://localhost:8080`)
- `PORT`: Server port (default: `8080`)
- `DB_PATH`: Path to SQLite database file (default: `data/app.db`)
- `UPLOAD_DIR`: Directory for uploaded images (default: `data/uploads`)
- `MAX_UPLOAD_SIZE`: Maximum upload size in bytes (default: `10485760` = 10MB)
- `SESSION_COOKIE_NAME`: Session cookie name (default: `sid`)
- `SESSION_SECURE`: Use secure cookies (default: `false`)

## Development

### Running Tests

```bash
# Run all tests
go test ./...

# Run tests with race detection
go test -race ./...

# Run tests with verbose output
go test -v ./...
```

### Project Structure

```
gemini-hackathon/
├── cmd/server/          # Application entry point
├── internal/
│   ├── config/         # Configuration management
│   ├── handlers/       # HTTP route handlers
│   ├── middleware/     # HTTP middleware (session, logging)
│   ├── models/         # Data models
│   ├── services/       # Business logic (future)
│   ├── storage/        # Database and file storage interfaces
│   ├── gemini/         # Gemini API client
│   └── testutil/       # Test utilities and mocks
├── web/
│   ├── templates/      # HTML templates (Go templates)
│   └── static/         # Static assets (CSS, JS, images)
├── migrations/         # Database migration files
└── docs/              # Project documentation
```

### Code Style

- Follow standard Go formatting (`gofmt`, `goimports`)
- Use meaningful variable and function names
- Document exported functions and types
- Handle errors explicitly
- Use interfaces for testability

## API Endpoints

- `GET /` - Home page with upload form
- `POST /scans` - Upload image and create scan
- `GET /scans/{id}` - View scan with OCR text
- `POST /scans/{id}/annotate` - Generate annotation for selected text
- `GET /healthz` - Health check endpoint

## Database

The application uses SQLite for data persistence. The database file is created automatically at the path specified in `DB_PATH` (default: `data/app.db`).

Migrations are run automatically on startup. See `migrations/001_initial_schema.sql` for the schema.

## Frontend

- **HTMX 2.0.8**: For progressive enhancement and dynamic content updates
- **Tailwind CSS**: For styling (loaded via CDN for MVP)
- **Vanilla JavaScript**: Minimal JS for text selection handling

## Testing

The project includes test utilities and mocks for:
- Database operations
- File storage
- Gemini API client

See `internal/testutil/` for mock implementations and helper functions.

## Phase0 MVP Scope

This MVP (Phase0) includes:
- ✅ Image upload and storage
- ✅ OCR text extraction
- ✅ Text annotation
- ✅ Session-based identity

Future phases:
- **Phase1**: Google OAuth authentication
- **Phase2**: Bookmarks and history

## Troubleshooting

### Database errors
- Ensure the `data/` directory exists and is writable
- Check that SQLite is properly installed

### Upload errors
- Verify `UPLOAD_DIR` exists and is writable
- Check `MAX_UPLOAD_SIZE` is sufficient for your images

### Gemini API errors
- Verify `GEMINI_API_KEY` is set correctly
- Check API quota and rate limits

## License

See LICENSE file for details.

## References

- [Task List](docs/task.md) - Detailed implementation tasks
- [RFC](docs/rfc.md) - Technical architecture and design
- [PRD](docs/prd.md) - Product requirements and user stories
