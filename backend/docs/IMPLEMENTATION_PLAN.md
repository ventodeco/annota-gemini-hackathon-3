# Backend Implementation Plan

## Overview

This document outlines the complete backend implementation plan for the Gemini OCR+Annotation PWA, covering authentication, user management, scans, annotations, and AI-powered features.

---

## 1. Database Schema

### 1.1 Final Schema Design

```sql
-- Users: Supports multiple OAuth providers (Google, Apple, GitHub)
CREATE TABLE users (
    id BIGSERIAL PRIMARY KEY,
    email VARCHAR(255) NOT NULL,
    provider VARCHAR(50) NOT NULL,  -- 'google', 'apple', 'github'
    provider_id VARCHAR(255) NOT NULL,  -- Unique 'sub' from OAuth provider
    avatar_url TEXT,
    preferred_language VARCHAR(10) DEFAULT 'ID',
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    UNIQUE (provider, provider_id)
);

-- Scans: User's scanned images with embedded OCR text
CREATE TABLE scans (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT REFERENCES users(id) ON DELETE CASCADE,
    image_url TEXT NOT NULL,  -- S3/Storage link
    full_ocr_text TEXT,  -- Raw text extracted from entire image
    detected_language VARCHAR(10),  -- Language detected in image
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

-- Annotations: User's saved highlights with AI-generated explanations
CREATE TABLE annotations (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT REFERENCES users(id) ON DELETE CASCADE,
    scan_id BIGINT REFERENCES scans(id) ON DELETE SET NULL,
    highlighted_text TEXT NOT NULL,  -- The specific word/sentence highlighted
    context_text TEXT,  -- Surrounding sentence/paragraph for LLM context
    nuance_data JSONB NOT NULL,  -- Structured LLM output
    is_bookmarked BOOLEAN DEFAULT TRUE,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

-- Indexes for fast retrieval
CREATE INDEX idx_scans_user_id ON scans(user_id);
CREATE INDEX idx_annotations_user_id ON annotations(user_id);
```

### 1.2 Migration Strategy

1. **Migration 001**: Drop existing tables (`sessions`, `scans`, `scan_images`, `ocr_results`, `annotations`) if they exist
2. **Migration 002**: Create new schema tables

---

## 2. Dependencies

```bash
go get github.com/redis/go-redis/v9
go get github.com/golang-jwt/jwt/v5
go get golang.org/x/oauth2/google
```

---

## 3. Configuration

### 3.1 Environment Variables

```bash
# Existing
GEMINI_API_KEY=...
APP_BASE_URL=http://localhost:8080
PORT=8080
POSTGRES_HOST=localhost
POSTGRES_PORT=5432
POSTGRES_USER=postgres
POSTGRES_PASSWORD=password
POSTGRES_DB=gemini_db
UPLOAD_DIR=data/uploads
MAX_UPLOAD_SIZE=10485760

# New - Authentication
GOOGLE_OAUTH_CLIENT_ID=...
GOOGLE_OAUTH_CLIENT_SECRET=...
REDIS_ADDR=localhost:6379
JWT_SECRET=your-256-bit-secret-key
TOKEN_EXPIRY_MINUTES=30
DEFAULT_PAGE_SIZE=20
```

### 3.2 Config Struct Updates

```go
// internal/config/config.go
type Config struct {
    GeminiAPIKey          string
    AppBaseURL            string
    Port                  string
    DBConnectionString    string
    UploadDir             string
    MaxUploadSize         int64

    // New fields
    GoogleOAuthClientID     string
    GoogleOAuthClientSecret string
    RedisAddr               string
    JWTSecret               string
    TokenExpiryMinutes      int
    DefaultPageSize         int
}
```

---

## 4. API Endpoints Summary

| Method | Endpoint | Description | Auth |
|--------|----------|-------------|------|
| GET | `/v1/auth/google/state` | Get Google OAuth redirect URL | No |
| GET | `/v1/auth/google/callback` | Handle Google OAuth redirect | No |
| POST | `/v1/auth/google/callback` | Exchange code for JWT token | No |
| GET | `/v1/users/me/languages` | Get available languages | JWT |
| GET | `/v1/users/me` | Get user profile | JWT |
| PATCH | `/v1/users/me` | Update user preferences | JWT |
| POST | `/v1/scans` | Upload and scan image | JWT |
| GET | `/v1/scans` | Get scan history (paginated) | JWT |
| GET | `/v1/scans/{id}` | Get scan details | JWT |
| POST | `/v1/ai/analyze` | Analyze text with AI | JWT |
| POST | `/v1/annotations` | Create bookmark | JWT |
| GET | `/v1/annotations` | Get all annotations (paginated) | JWT |

---

## 5. Task Breakdown

### Phase 1: Configuration & Models

#### Task 1.1: Update Config (`internal/config/config.go`)
- Add new fields for OAuth, Redis, JWT configuration
- Update `Load()` to read new env vars
- Update `Validate()` to check required OAuth fields

#### Task 1.2: Create User Model (`internal/models/user.go`)
```go
type User struct {
    ID                 int64
    Email              string
    Provider           string  // 'google', 'apple', 'github'
    ProviderID         string
    AvatarURL          *string
    PreferredLanguage  string  // 'ID', 'JP', 'EN'
    CreatedAt          time.Time
    UpdatedAt          time.Time
}
```

#### Task 1.3: Create Scan Model (`internal/models/scan.go`)
```go
type Scan struct {
    ID               int64
    UserID           int64
    ImageURL         string
    FullOCRText      *string
    DetectedLanguage *string  // 'ID', 'JP', 'EN'
    CreatedAt        time.Time
}
```

#### Task 1.4: Create Annotation Model (`internal/models/annotation.go`)
```go
type NuanceData struct {
    Meaning            string `json:"meaning"`
    UsageExample       string `json:"usageExample"`
    UsageTiming        string `json:"usageTiming"`
    WordBreakdown      string `json:"wordBreakdown"`
    AlternativeMeaning string `json:"alternativeMeaning"`
}

type Annotation struct {
    ID              int64
    UserID          int64
    ScanID          *int64
    HighlightedText string
    ContextText     *string
    NuanceData      NuanceData
    IsBookmarked    bool
    CreatedAt       time.Time
}
```

---

### Phase 2: Storage Layer

#### Task 2.1: Update `internal/storage/db.go`
- Remove session-related methods
- Add User, Scan, Annotation methods
- Update `postgresDB` implementations

#### Task 2.2: Create Redis Client (`internal/storage/redis.go`)
```go
type RedisClient interface {
    SetState(ctx context.Context, state, sessionID string, ttl time.Duration) error
    GetState(ctx context.Context, state string) (string, error)
    DeleteState(ctx context.Context, state string) error
    Close() error
}
```

---

### Phase 3: Authentication

#### Task 3.1: JWT Token Service (`internal/auth/token.go`)
```go
type TokenService struct {
    secret []byte
    expiry time.Duration
}

func NewTokenService(secret string, expiryMinutes int) *TokenService
func (s *TokenService) GenerateToken(userID int64) (string, time.Time, error)
func (s *TokenService) ValidateToken(tokenString string) (int64, error)
```

#### Task 3.2: Google OAuth Service (`internal/auth/google.go`)
```go
type GoogleOAuthService struct {
    config      *oauth2.Config
    redisClient storage.RedisClient
    appBaseURL  string
}

func NewGoogleOAuthService(cfg *config.Config, redis storage.RedisClient) *GoogleOAuthService
func (s *GoogleOAuthService) GetAuthURL(state string) string
func (s *GoogleOAuthService) ExchangeCode(ctx context.Context, code string) (*oauth2.Token, error)
func (s *GoogleOAuthService) GetUserInfo(ctx context.Context, token *oauth2.Token) (*UserInfo, error)

type UserInfo struct {
    Email      string
    ID         string
    Name       string
    PictureURL string
}
```

#### Task 3.3: Auth HTTP Handlers (`internal/handlers/auth.go`)
- GET `/v1/auth/google/state` - Returns OAuth redirect URL
- GET `/v1/auth/google/callback` - Redirects to FE with state/code
- POST `/v1/auth/google/callback` - Exchanges code for JWT

#### Task 3.4: Auth Middleware (`internal/middleware/auth.go`)
```go
type AuthMiddleware struct {
    jwtSecret []byte
}

func NewAuthMiddleware(jwtSecret string) *AuthMiddleware
func (m *AuthMiddleware) Handle(next http.Handler) http.Handler
func GetUserID(ctx context.Context) int64
```

---

### Phase 4: User API

#### Task 4.1: User HTTP Handlers (`internal/handlers/user.go`)
- GET `/v1/users/me/languages` - Returns hardcoded ID, JP, EN
- GET `/v1/users/me` - Returns user profile
- PATCH `/v1/users/me` - Updates user preferences

---

### Phase 5: Scan API

#### Task 5.1: Scan Handlers (`internal/handlers/scan.go`)
- POST `/v1/scans` - Upload image, start OCR
- GET `/v1/scans` - Get scan history (paginated)
- GET `/v1/scans/{id}` - Get scan details

#### Task 5.2: Update OCR Processing
- Store OCR results directly in `scans` table
- Remove dependency on separate `ocr_results` table

---

### Phase 6: AI Analyze API

#### Task 6.1: AI Analyze Handler (`internal/handlers/ai.go`)
- POST `/v1/ai/analyze` - Analyze text with Gemini
- Uses user's preferred language as target
- Returns structured JSON: {meaning, usageExample, usageTiming, wordBreakdown, alternativeMeaning}

---

### Phase 7: Annotation API

#### Task 7.1: Annotation Handlers (`internal/handlers/annotation.go`)
- POST `/v1/annotations` - Create bookmark
- GET `/v1/annotations` - Get all annotations (paginated)

---

### Phase 8: Cleanup

#### Task 8.1: Delete Session Middleware
- Delete `internal/middleware/session.go`

#### Task 8.2: Update Main Server
- Update `backend/cmd/server/main.go`
- Remove session middleware, add auth middleware
- Update routes to use `/v1/` prefix

#### Task 8.3: Create Migrations
- `migrations/001_drop_old_tables.sql`
- `migrations/002_new_schema.sql`

---

## 6. Implementation Order

1. Task 1.1: Update Config
2. Task 1.2: Create User Model
3. Task 1.3: Create Scan Model
4. Task 1.4: Create Annotation Model
5. Task 2.1: Update Storage Layer
6. Task 2.2: Create Redis Client
7. Task 3.1: JWT Token Service
8. Task 3.2: Google OAuth Service
9. Task 3.3: Auth Handlers
10. Task 3.4: Auth Middleware
11. Task 4.1: User Handlers
12. Task 5.1: Scan Handlers + OCR Update
13. Task 6.1: AI Analyze Handler
14. Task 7.1: Annotation Handlers
15. Task 8.1-8.3: Cleanup + Migrations

---

## 7. Notes

### 7.1 Pagination
- Page numbering: 1-based
- Default page size: 20 (configurable)
- Meta response includes currentPage, pageSize, nextPage, previousPage

### 7.2 JWT Claims
```go
type JWTClaims struct {
    UserID int64 `json:"user_id"`
    Exp    int64 `json:"exp"`
    Iat    int64 `json:"iat"`
}
```

### 7.3 Redis State Key Format
- Key: `oauth:state:{state}`
- Value: sessionID (for tracking)
- TTL: 10 minutes

### 7.4 Hardcoded Languages
```go
var Languages = []Language{
    {Caption: "ID", ImageURL: "..."},
    {Caption: "JP", ImageURL: "..."},
    {Caption: "EN", ImageURL: "..."},
}
```

---

## 8. Status

| Phase | Tasks | Status |
|-------|-------|--------|
| Phase 1: Config & Models | 4 | Pending |
| Phase 2: Storage | 2 | Pending |
| Phase 3: Authentication | 4 | Pending |
| Phase 4: User API | 1 | Pending |
| Phase 5: Scan API | 2 | Pending |
| Phase 6: AI Analyze | 1 | Pending |
| Phase 7: Annotation API | 1 | Pending |
| Phase 8: Cleanup | 3 | Pending |
