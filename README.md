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
   cd backend
   go mod tidy
   cd ../web
   bun install
   cd ..
   ```

4. **Build the frontend**:
   ```bash
   cd web
   bun run build
   cd ..
   ```

5. **Run the backend server**:
   ```bash
   cd backend
   go run cmd/server/main.go
   ```

6. **Open in browser**:
   Navigate to `http://localhost:8080`

### Development Mode

Run backend and frontend separately for development:

**Terminal 1 - Backend:**
```bash
cd backend
go run cmd/server/main.go
```

**Terminal 2 - Frontend:**
```bash
cd web
bun run dev
```

Then access the frontend dev server at `http://localhost:5173`

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

**Backend tests:**
```bash
cd backend
go test ./...

# Run tests with race detection
go test -race ./...

# Run tests with verbose output
go test -v ./...
```

**Frontend tests:**
```bash
cd web
bun test

# Run with coverage
bun run test:coverage

# Run in watch mode
bun run test:watch
```

### Project Structure

```
gemini-hackathon/
├── backend/
│   ├── cmd/server/      # Application entry point
│   ├── internal/
│   │   ├── config/      # Configuration management
│   │   ├── handlers/    # HTTP route handlers (JSON API)
│   │   ├── middleware/  # HTTP middleware (session, logging)
│   │   ├── models/      # Data models
│   │   ├── storage/     # Database and file storage interfaces
│   │   ├── gemini/      # Gemini API client
│   │   └── testutil/    # Test utilities and mocks
│   ├── migrations/      # Database migration files
│   ├── go.mod           # Go module definition
│   └── go.sum           # Go module checksums
├── web/
│   ├── src/             # React source code
│   │   ├── components/  # React components
│   │   ├── pages/       # Page components
│   │   ├── hooks/       # Custom React hooks
│   │   ├── lib/         # Utilities, API client, types
│   │   └── test/        # Test setup and utilities
│   ├── public/          # Static assets
│   ├── dist/            # Build output (gitignored)
│   ├── package.json     # Frontend dependencies
│   └── vite.config.ts   # Vite configuration
└── docs/
    ├── prd.md           # Product requirements
    ├── rfc.md           # Technical architecture
    └── task.md          # Implementation tasks
```

### Code Style

- Follow standard Go formatting (`gofmt`, `goimports`)
- Use meaningful variable and function names
- Document exported functions and types
- Handle errors explicitly
- Use interfaces for testability

## API Endpoints

- `GET /healthz` - Health check endpoint
- `POST /api/scans` - Upload image and create scan
- `GET /api/scans/{id}` - Get scan data with OCR result
- `POST /api/scans/{id}/annotate` - Generate annotation for selected text
- `GET /api/scans/{id}/image` - Get scan image file
- `GET /` - Serves React frontend (SPA)

## Database

The application uses SQLite for data persistence. The database file is created automatically at the path specified in `DB_PATH` (default: `data/app.db`).

Migrations are run automatically on startup. See `migrations/001_initial_schema.sql` for the schema.

## Frontend

- **React 19**: Modern UI framework
- **TypeScript**: Type-safe development
- **Vite**: Fast build tool and dev server
- **Tailwind CSS v4**: Utility-first CSS framework
- **shadcn/ui**: Accessible component library
- **TanStack Query**: Data fetching and state management
- **Vitest**: Unit testing framework

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

## Build for Production

```bash
# Build frontend
cd web
bun run build

# Build backend
cd ../backend
go build -o ../server cmd/server/main.go

# Run from root
cd ..
./server
```

## References

- [Task List](docs/task.md) - Detailed implementation tasks
- [RFC](docs/rfc.md) - Technical architecture and design
- [PRD](docs/prd.md) - Product requirements and user stories
