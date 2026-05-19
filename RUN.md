# How to Run Backend and Web

Quick reference guide for running the application.

## Prerequisites

- **Go 1.25.x** or later
- **bun** (for frontend)
- **Nix dev shell** (for local PostgreSQL and Redis)
- **OpenRouter API Key** for OCR
- **MiniMax API Key** for annotation and text-to-speech

## Setup Steps

### 1. Environment Configuration

```bash
# Copy environment template
cp backend/.env.example backend/.env

# Edit backend/.env and add provider API keys
# OPENROUTER_API_KEY=your_openrouter_api_key_here
# MINIMAX_API_KEY=your_minimax_api_key_here

# Load the Nix flake dev shell and local backend env values
direnv allow
direnv reload
```

### 2. Start Local Database Services

```bash
# Start local PostgreSQL and Redis from the Nix dev shell
dev-services start

# Verify they are running
dev-services status

# View logs if needed
dev-services logs
```

### 3. Install Dependencies

**Backend:**
```bash
cd backend
go mod tidy
cd ..
```

**Frontend:**
```bash
cd web
bun install
cd ..
```

## Running the Application

### Option 1: Production Mode (Backend serves built frontend)

**Terminal 1 - Build Frontend:**
```bash
cd web
bun run build
cd ..
```

**Terminal 2 - Run Backend:**
```bash
cd backend
go run cmd/server/main.go
```

**Access:** `http://localhost:8080`

### Option 2: Development Mode (Separate servers)

Set backend callback target for split dev OAuth flow:
```bash
# backend/.env
APP_BASE_URL=http://localhost:8080
FRONTEND_BASE_URL=http://localhost:5173
```

**Terminal 1 - Backend:**
```bash
cd backend
go run cmd/server/main.go
```
Backend runs on: `http://localhost:8080`

**Terminal 2 - Frontend Dev Server:**
```bash
cd web
bun run dev
```
Frontend dev server runs on: `http://localhost:5173` (Vite default)

**Note:** In dev mode, frontend proxies API calls to backend at `http://localhost:8080`.
With `FRONTEND_BASE_URL=http://localhost:5173`, OAuth callback redirects to Vite without requiring `bun run build`.

## Quick Commands Reference

### Backend Commands

```bash
# Run server
cd backend && go run cmd/server/main.go

# Run tests
cd backend && go test ./...

# Run tests with race detector
cd backend && go test -race ./...

# Build binary
cd backend && go build -o ../server cmd/server/main.go

# Format code
cd backend && go fmt ./...

# Check code
cd backend && go vet ./...
```

### Frontend Commands

```bash
# Install dependencies
cd web && bun install

# Run dev server
cd web && bun run dev

# Build for production
cd web && bun run build

# Run tests
cd web && bun run test

# Run tests with coverage
cd web && bun run test:coverage

# Run tests in watch mode
cd web && bun run test:watch

# Preview production build
cd web && bun run preview
```

### Local Service Commands

```bash
# Start PostgreSQL and Redis locally
dev-services start

# Stop local services
dev-services stop

# Check status
dev-services status

# View logs
dev-services logs
```

## Environment Variables

Required:
- `OPENROUTER_API_KEY` - API key for OpenRouter OCR
- `MINIMAX_API_KEY` - API key for MiniMax annotation and speech
- `JWT_SECRET` - At least 32 random characters
- `GOOGLE_OAUTH_CLIENT_ID` - Google OAuth 2.0 client ID (from Google Cloud Console)
- `GOOGLE_OAUTH_CLIENT_SECRET` - Google OAuth 2.0 client secret

PostgreSQL:
- `POSTGRES_HOST` - Default: `localhost`
- `POSTGRES_PORT` - Default: `5432`
- `POSTGRES_USER` - Default: `postgres`
- `POSTGRES_PASSWORD` - Default: `password`
- `POSTGRES_DB` - Default: `gemini_db`

Optional:
- `APP_ENV` - `development` or `production` (production rejects wildcard CORS origins)
- `PORT` - Backend port (default: `8080`)
- `APP_BASE_URL` - Base URL (default: `http://localhost:8080`)
- `FRONTEND_BASE_URL` - Frontend callback URL for OAuth redirect (default: `APP_BASE_URL`)
- `UPLOAD_DIR` - Upload directory (default: `data/uploads`)
- `MAX_UPLOAD_SIZE` - Max upload size in bytes (default: `10485760` = 10MB)
- `SESSION_COOKIE_NAME` - Session cookie name (default: `sid`)
- `SESSION_SECURE` - Secure cookies (default: `false`)
- `OPENROUTER_OCR_MODEL` - OCR model (default: `baidu/qianfan-ocr-fast:free`)
- `MINIMAX_TEXT_MODEL` - Annotation model (default: `MiniMax-M2.7`)
- `MINIMAX_TTS_MODEL` - Speech model (default: `speech-2.8-hd`)
- `MINIMAX_TTS_VOICE_ID` - Speech voice ID (default: `Japanese_Whisper_Belle`)
- `AI_RATE_LIMIT` - AI-backed action limit per user/path/window (default: `60`, `0` disables)
- `AI_RATE_LIMIT_WINDOW_SECONDS` - Rate-limit window in seconds (default: `3600`)

## Troubleshooting

### Backend won't start

1. **Check PostgreSQL and Redis are running:**
   ```bash
   dev-services status
   ```
   PostgreSQL and Redis should both show as `running`.

2. **Check database connection:**
   ```bash
   psql -h localhost -p 5432 -U postgres -d gemini_db -c "SELECT 1;"
   ```

3. **Check Redis connection:**
   ```bash
   redis-cli -h localhost -p 6379 ping
   ```

4. **Check environment variables:**
   ```bash
   # Make sure .env file exists in backend/ and has all required keys
   cat backend/.env
   ```
   Required keys: `OPENROUTER_API_KEY`, `MINIMAX_API_KEY`, `JWT_SECRET` (32+ chars), `GOOGLE_OAUTH_CLIENT_ID`, `GOOGLE_OAUTH_CLIENT_SECRET`.

5. **Check Go dependencies:**
   ```bash
   cd backend && go mod tidy
   ```

### Frontend won't start

1. **Check bun is installed:**
   ```bash
   bun --version
   ```

2. **Reinstall dependencies:**
   ```bash
   cd web && rm -rf node_modules && bun install
   ```

3. **Check port 5173 is available:**
   ```bash
   lsof -i :5173
   ```

### Database migration errors

- Make sure PostgreSQL is running: `dev-services status`
- Check connection string in `.env`
- Verify database exists: `psql -h localhost -p 5432 -U postgres -l`

## Project Structure

```
hackathon-gemini-3/
├── backend/           # Go backend
│   ├── cmd/server/   # Main entry point
│   ├── internal/     # Internal packages
│   └── migrations/   # Database migrations
├── web/              # React frontend
│   ├── src/          # Source code
│   └── dist/         # Build output
├── docker-compose.yml # Optional Docker Compose services
└── .env              # Environment variables (create from .env.example)
```

## API Endpoints

- `GET /healthz` - Health check
- `POST /api/scans` - Upload image and create scan
- `GET /api/scans/{id}` - Get scan data with OCR result
- `POST /api/scans/{id}/annotate` - Generate annotation for selected text
- `GET /api/scans/{id}/image` - Get scan image file
- `GET /` - Serves React frontend (SPA)

## Next Steps

1. Start PostgreSQL and Redis: `dev-services start`
2. Set up `backend/.env` from `backend/.env.example`, filling in all required keys including `OPENROUTER_API_KEY`, `MINIMAX_API_KEY`, `GOOGLE_OAUTH_CLIENT_ID`, and `GOOGLE_OAUTH_CLIENT_SECRET`
3. Build frontend: `cd web && bun run build`
4. Run backend: `cd backend && go run cmd/server/main.go`
5. Open browser: `http://localhost:8080`

### Google OAuth Setup

To enable Google login:

1. Go to [Google Cloud Console](https://console.cloud.google.com/apis/credentials)
2. Create an OAuth 2.0 Client ID (Web application type)
3. Add authorized redirect URI: `http://localhost:8080/v1/auth/google/callback`
4. Copy Client ID and Client Secret into `backend/.env`
