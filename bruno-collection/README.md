# Bruno Collection - Gemini OCR+Annotation API

This is a Bruno collection for testing the Gemini OCR+Annotation API.

## Setup

1. Download and install [Bruno](https://www.usebruno.com/)
2. Open Bruno and import this collection
3. Set up the environment:
   - Click "Manage Environments"
   - Create a new environment (e.g., "Local")
   - Add variables:
     - `baseUrl`: `http://localhost:8080`
     - `token`: `` (empty initially)

## Authentication Flow

1. **Get Google OAuth URL**
   - Request: `GET /v1/auth/google/state`
   - Response: `{"ssoRedirection": "https://accounts.google.com/..."}`
   - Open the URL in browser to authenticate with Google

2. **Exchange Code for Token**
   - After Google redirects back, you'll receive `state` and `code` parameters
   - Request: `POST /v1/auth/google/callback`
   - Body: `{"code": "..."}`
   - Response: `{"token": "...", "expirySeconds": 1800, "expiresAt": "..."}`
   - Copy the token and set it in the environment variable `token`

## Environment Variables

| Variable | Description | Default |
|----------|-------------|---------|
| `baseUrl` | API base URL | `http://localhost:8080` |
| `token` | JWT token for authenticated requests | `` |

## API Endpoints

### Auth (No Auth Required)

| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/healthz` | Health check |
| GET | `/v1/auth/google/state` | Get Google OAuth redirect URL |
| GET | `/v1/auth/google/callback` | Handle Google OAuth redirect |
| POST | `/v1/auth/google/callback` | Exchange code for JWT token |

### User (JWT Required)

| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/v1/users/me` | Get user profile |
| GET | `/v1/users/me/languages` | Get available languages |
| PATCH | `/v1/users/me` | Update user preferences |

### Scans (JWT Required)

| Method | Endpoint | Description |
|--------|----------|-------------|
| POST | `/v1/scans` | Upload image, start OCR |
| GET | `/v1/scans` | Get scan history (paginated) |
| GET | `/v1/scans/{id}` | Get scan details |

### AI (JWT Required)

| Method | Endpoint | Description |
|--------|----------|-------------|
| POST | `/v1/ai/analyze` | Analyze text with AI |

### Annotations (JWT Required)

| Method | Endpoint | Description |
|--------|----------|-------------|
| POST | `/v1/annotations` | Create bookmark |
| GET | `/v1/annotations` | Get all annotations (paginated) |

## Testing Notes

1. **Token Expiry**: Tokens expire after 30 minutes (1800 seconds)
2. **Pagination**: Default page size is 20, max is 100
3. **Image Upload**: Use multipart/form-data with `image` field
4. **Content-Type**: Set `x-token` header for authenticated requests

## Response Examples

### Get Languages
```json
{
  "languages": [
    {"caption": "ID", "imageUrl": "https://flagcdn.com/w40/id.png"},
    {"caption": "JP", "imageUrl": "https://flagcdn.com/w40/jp.png"},
    {"caption": "EN", "imageUrl": "https://flagcdn.com/w40/gb.png"}
  ]
}
```

### Get Scans Response
```json
{
  "data": [
    {
      "id": 1,
      "imageUrl": "/uploads/1.jpg",
      "detectedLanguage": "JP",
      "createdAt": "2026-01-13T08:30:00Z"
    }
  ],
  "meta": {
    "currentPage": 1,
    "pageSize": 20,
    "nextPage": null,
    "previousPage": null
  }
}
```

### Analyze Response
```json
{
  "meaning": "Teknik penggilingan basah...",
  "usageExample": "Petani di Sumatra sering menggunakan...",
  "usageTiming": "Ketika kopi diolah...",
  "wordBreakdown": "Giling (Grind) + Basah (Wet)",
  "alternativeMeaning": "Dalam konteks metafora..."
}
```

### Get Annotations Response
```json
{
  "data": [
    {
      "id": 1,
      "highlightedText": "Giling Basah",
      "nuanceSummary": "Wet Hulled coffee processing...",
      "createdAt": "2026-01-13T08:35:00Z"
    }
  ],
  "meta": {
    "currentPage": 1,
    "pageSize": 20,
    "nextPage": null,
    "previousPage": null
  }
}
```
