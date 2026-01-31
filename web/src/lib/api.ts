import type {
  // Auth types
  TokenResponse,
  // User types
  GetUserProfileResponse,
  UpdateUserPreferencesRequest,
  GetLanguagesResponse,
  // Scan types
  CreateScanResponse,
  GetScansResponse,
  Scan,
  // AI types
  AnalyzeRequest,
  AnalyzeResponse,
  // Annotation types
  CreateAnnotationRequest,
  CreateAnnotationResponse,
  GetAnnotationsResponse,
} from './types'
import { logger } from './logger'

const API_BASE_URL = import.meta.env.VITE_API_BASE_URL || 'http://localhost:8080'
const AUTH_TOKEN_KEY = 'auth_token'

// ============================================================================
// Auth Helpers
// ============================================================================

export function getAuthToken(): string | null {
  return localStorage.getItem(AUTH_TOKEN_KEY)
}

export function setAuthToken(token: string): void {
  localStorage.setItem(AUTH_TOKEN_KEY, token)
}

export function clearAuthToken(): void {
  localStorage.removeItem(AUTH_TOKEN_KEY)
}

function getAuthHeaders(): HeadersInit {
  const token = getAuthToken()
  return token ? { 'x-token': token } : {}
}

function isAuthError(status: number): boolean {
  return status === 401
}

async function handleResponse<T>(response: Response, method: string, url: string): Promise<T> {
  const startTime = Date.now()
  
  if (!response.ok) {
    if (isAuthError(response.status)) {
      clearAuthToken()
      window.location.href = '/login'
    }
    const error = await response.text().catch(() => 'An error occurred')
    logger.apiCall(method, url, response.status, Date.now() - startTime, new Error(error))
    throw new Error(error)
  }
  
  logger.apiCall(method, url, response.status, Date.now() - startTime)
  return response.json()
}

// ============================================================================
// Auth API
// ============================================================================

export async function getGoogleAuthUrl(): Promise<{ ssoRedirection: string }> {
  const url = `${API_BASE_URL}/v1/auth/google/state`
  const response = await fetch(url, {
    method: 'GET',
    headers: {
      'Content-Type': 'application/json',
    },
  })
  return handleResponse(response, 'GET', url)
}

export async function exchangeGoogleCode(code: string): Promise<TokenResponse> {
  const url = `${API_BASE_URL}/v1/auth/google/callback`
  const response = await fetch(url, {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
    },
    body: JSON.stringify({ code }),
  })
  return handleResponse(response, 'POST', url)
}

// ============================================================================
// User API
// ============================================================================

export async function getUserProfile(): Promise<GetUserProfileResponse> {
  const url = `${API_BASE_URL}/v1/users/me`
  const response = await fetch(url, {
    method: 'GET',
    headers: {
      ...getAuthHeaders(),
      'Content-Type': 'application/json',
    },
  })
  return handleResponse(response, 'GET', url)
}

export async function updateUserPreferences(
  preferences: UpdateUserPreferencesRequest,
): Promise<GetUserProfileResponse> {
  const url = `${API_BASE_URL}/v1/users/me`
  const response = await fetch(url, {
    method: 'PATCH',
    headers: {
      ...getAuthHeaders(),
      'Content-Type': 'application/json',
    },
    body: JSON.stringify(preferences),
  })
  return handleResponse(response, 'PATCH', url)
}

export async function getLanguages(): Promise<GetLanguagesResponse> {
  const url = `${API_BASE_URL}/v1/users/me/languages`
  const response = await fetch(url, {
    method: 'GET',
    headers: {
      ...getAuthHeaders(),
      'Content-Type': 'application/json',
    },
  })
  return handleResponse(response, 'GET', url)
}

// ============================================================================
// Scans API
// ============================================================================

export async function createScan(imageFile: File): Promise<CreateScanResponse> {
  const formData = new FormData()
  formData.append('image', imageFile)

  const url = `${API_BASE_URL}/v1/scans`
  logger.debug(`Uploading image: ${imageFile.name}, size: ${imageFile.size} bytes`)
  const response = await fetch(url, {
    method: 'POST',
    headers: getAuthHeaders(),
    body: formData,
  })
  return handleResponse(response, 'POST', url)
}

export async function getScans(page = 1, size = 20): Promise<GetScansResponse> {
  const url = `${API_BASE_URL}/v1/scans?page=${page}&size=${size}`
  const response = await fetch(url, {
    method: 'GET',
    headers: {
      ...getAuthHeaders(),
      'Content-Type': 'application/json',
    },
  })
  return handleResponse(response, 'GET', url)
}

export async function getScan(scanId: number): Promise<Scan> {
  const url = `${API_BASE_URL}/v1/scans/${scanId}`
  const response = await fetch(url, {
    method: 'GET',
    headers: {
      ...getAuthHeaders(),
      'Content-Type': 'application/json',
    },
  })
  return handleResponse(response, 'GET', url)
}

// ============================================================================
// AI API
// ============================================================================

export async function analyzeText(request: AnalyzeRequest): Promise<AnalyzeResponse> {
  const url = `${API_BASE_URL}/v1/ai/analyze`
  const response = await fetch(url, {
    method: 'POST',
    headers: {
      ...getAuthHeaders(),
      'Content-Type': 'application/json',
    },
    body: JSON.stringify(request),
  })
  return handleResponse(response, 'POST', url)
}

// ============================================================================
// Annotations API
// ============================================================================

export async function createAnnotation(
  request: CreateAnnotationRequest,
): Promise<CreateAnnotationResponse> {
  const url = `${API_BASE_URL}/v1/annotations`
  const response = await fetch(url, {
    method: 'POST',
    headers: {
      ...getAuthHeaders(),
      'Content-Type': 'application/json',
    },
    body: JSON.stringify(request),
  })
  return handleResponse(response, 'POST', url)
}

export async function getAnnotations(page = 1, size = 20): Promise<GetAnnotationsResponse> {
  const url = `${API_BASE_URL}/v1/annotations?page=${page}&size=${size}`
  const response = await fetch(url, {
    method: 'GET',
    headers: {
      ...getAuthHeaders(),
      'Content-Type': 'application/json',
    },
  })
  return handleResponse(response, 'GET', url)
}

// ============================================================================
// Utility Functions
// ============================================================================

export function getScanImageUrl(imageUrl: string): string {
  if (imageUrl.startsWith('/')) {
    return `${API_BASE_URL}${imageUrl}`
  }
  return imageUrl
}

export function formatDate(dateString: string): string {
  const date = new Date(dateString)
  return date.toLocaleDateString('en-US', {
    year: 'numeric',
    month: 'short',
    day: 'numeric',
    hour: '2-digit',
    minute: '2-digit',
  })
}
