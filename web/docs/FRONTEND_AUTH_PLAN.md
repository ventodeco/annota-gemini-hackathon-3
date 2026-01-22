# Frontend Auth Implementation Plan

## Overview

This document outlines the frontend implementation for the new authentication system using Google OAuth2 + JWT tokens, replacing the mock authentication currently in place.

## Architecture

### Authentication Flow
```
┌─────────┐     GET /v1/auth/google/state      ┌─────────┐
│  Front  │ ─────────────────────────────────▶  │ Backend │
│  End    │                                  │   API   │
└─────────┘                                  └─────────┘
          │                                        │
          │    {"ssoRedirection": "https://..."}  │
          │◀──────────────────────────────────────┘
          │
          │ Open URL in popup/redirect
          │ Redirect to Google
          │
          ▼
    ┌───────────┐
    │  Google   │
    │  OAuth    │
    └───────────┘
          │
          │ User signs in
          │
          ▼
    Redirect back to: APP_URL/auth/callback?state=XXX&code=YYY
```

### Token Management
- Token stored in `localStorage` with key `auth_token`
- Token passed via `x-token` header (or `Authorization: Bearer` header)
- Token expiry: 30 minutes

---

## API Changes

### Base URL
```env
VITE_API_BASE_URL=http://localhost:8080
VITE_APP_URL=http://localhost:5173  # Frontend URL for OAuth redirect
```

### New Types (web/src/lib/types.ts)

```typescript
// User
interface User {
  id: number
  email: string
  provider: 'google' | 'apple' | 'github'
  avatar_url?: string
  preferred_language: 'ID' | 'JP' | 'EN'
  created_at: string
}

// Auth
interface TokenResponse {
  token: string
  expirySeconds: number
  expiresAt: string
}

// Scans (new schema)
interface Scan {
  id: number
  user_id: number
  image_url: string
  full_ocr_text?: string
  detected_language?: string
  created_at: string
}

interface CreateScanResponse {
  scanId: number
  fullText?: string
  imageUrl: string
}

interface GetScanListItem {
  id: number
  imageUrl: string
  detectedLanguage?: string
  createdAt: string
}

interface GetScansResponse {
  data: GetScanListItem[]
  meta: {
    currentPage: number
    pageSize: number
    nextPage?: number
    previousPage?: number
  }
}

// Annotations
interface NuanceData {
  meaning: string
  usageExample: string
  usageTiming: string
  wordBreakdown: string
  alternativeMeaning: string
}

interface Annotation {
  id: number
  user_id: number
  scan_id?: number
  highlighted_text: string
  context_text?: string
  nuance_data: NuanceData
  is_bookmarked: boolean
  created_at: string
}

interface CreateAnnotationRequest {
  scanId: number
  highlightedText: string
  contextText?: string
  nuanceData: NuanceData
}

interface CreateAnnotationResponse {
  annotationId: number
  status: 'saved'
}

// AI
interface AnalyzeRequest {
  textToAnalyze: string
  context: string
}

interface AnalyzeResponse extends NuanceData {}

// Languages
interface Language {
  caption: string
  imageUrl: string
}

interface GetLanguagesResponse {
  languages: Language[]
}
```

---

## Implementation Tasks

### Phase 1: Foundation
- [ ] 1.1 Create environment variables (.env)
- [ ] 1.2 Update TypeScript types (types.ts)
- [ ] 1.3 Update API client (api.ts)

### Phase 2: Auth Context & Hooks
- [ ] 2.1 Create Auth Context (contexts/AuthContext.tsx)
- [ ] 2.2 Create Auth Hook (hooks/useAuth.ts)
- [ ] 2.3 Create Auth API Hooks (hooks/useAuthApi.ts)

### Phase 3: Components & Pages
- [ ] 3.1 Update App.tsx (routes, ProtectedRoute)
- [ ] 3.2 Create OAuth Callback Page (pages/OAuthCallbackPage.tsx)
- [ ] 3.3 Update LoginPage.tsx (real OAuth flow)

### Phase 4: Update Existing Pages
- [ ] 4.1 Update HistoryPage.tsx
- [ ] 4.2 Update CameraPage.tsx
- [ ] 4.3 Update ScanPage.tsx
- [ ] 4.4 Create data hooks (useScans.ts, useAnnotations.ts)

### Phase 5: Error Handling
- [ ] 5.1 Create auth error handler (lib/authErrors.ts)
- [ ] 5.2 Update query client with auth error handling

---

## Files to Create

| File | Purpose |
|------|---------|
| `web/src/contexts/AuthContext.tsx` | React context for auth state |
| `web/src/hooks/useAuth.ts` | Auth hook |
| `web/src/hooks/useAuthApi.ts` | Auth API mutations |
| `web/src/hooks/useScans.ts` | Scan data hooks |
| `web/src/hooks/useAnnotations.ts` | Annotation & AI hooks |
| `web/src/pages/OAuthCallbackPage.tsx` | OAuth callback handler |
| `web/src/lib/authErrors.ts` | Error handling utilities |

## Files to Modify

| File | Changes |
|------|---------|
| `web/.env` | Add VITE_APP_URL |
| `web/src/lib/types.ts` | Update all types for new schema |
| `web/src/lib/api.ts` | Add auth functions, update all calls |
| `web/src/App.tsx` | Update routes, ProtectedRoute, add callback route |
| `web/src/pages/LoginPage.tsx` | Implement real OAuth flow |
| `web/src/pages/HistoryPage.tsx` | Update for new API |
| `web/src/pages/CameraPage.tsx` | Update for new API |
| `web/src/pages/ScanPage.tsx` | Update for new API |

---

## API Endpoints Reference

### Auth (No Auth Required)
| Method | Endpoint | Description |
|--------|----------|-------------|
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

---

## Authentication Examples

### Login Flow (Popup Approach)
```typescript
// 1. Get OAuth URL
const { ssoRedirection } = await getGoogleAuthUrl()

// 2. Open popup
const popup = window.open(ssoRedirection, 'google-login', 'width=500,height=700')

// 3. Listen for popup close
const checkPopup = setInterval(() => {
  if (popup?.closed) {
    clearInterval(checkPopup)
    // Check if token exists
    if (localStorage.getItem('auth_token')) {
      navigate('/welcome')
    }
  }
}, 500)
```

### Authenticated Request
```typescript
const response = await fetch('http://localhost:8080/v1/scans', {
  method: 'GET',
  headers: {
    'x-token': localStorage.getItem('auth_token') || ''
  }
})
```

---

## Testing Checklist

- [ ] Login button opens Google OAuth
- [ ] Popup closes after authentication
- [ ] Token is stored in localStorage
- [ ] Authenticated requests include token
- [ ] Protected routes redirect to login when not authenticated
- [ ] 401 errors redirect to login
- [ ] Logout clears token and redirects
- [ ] Scans are fetched correctly
- [ ] Annotations are created and listed
- [ ] AI analyze works with user's preferred language

---

## Status

| Phase | Status |
|-------|--------|
| Phase 1: Foundation | Pending |
| Phase 2: Auth Context & Hooks | Pending |
| Phase 3: Components & Pages | Pending |
| Phase 4: Update Existing Pages | Pending |
| Phase 5: Error Handling | Pending |
